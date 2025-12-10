package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/goran-ethernal/ChainIndexor/internal/common"
	"github.com/goran-ethernal/ChainIndexor/internal/db"
	"github.com/goran-ethernal/ChainIndexor/internal/logger"
	"github.com/goran-ethernal/ChainIndexor/pkg/config"
	"github.com/goran-ethernal/ChainIndexor/pkg/fetcher/store"
	"github.com/russross/meddler"
	"golang.org/x/sync/errgroup"
)

const maxConcurrency = 10

var _ store.LogStore = (*LogStore)(nil)

// LogStore implements LogStore interface using SQLite as the backend.
type LogStore struct {
	dbConfig config.DatabaseConfig
	db       *sql.DB
	log      *logger.Logger

	retentionPolicy *config.RetentionPolicyConfig
}

// NewLogStore creates a new SQLite-backed LogStore.
func NewLogStore(
	db *sql.DB,
	log *logger.Logger,
	dbConfig config.DatabaseConfig,
	retentionPolicy *config.RetentionPolicyConfig,
) *LogStore {
	return &LogStore{
		db:              db,
		log:             log,
		dbConfig:        dbConfig,
		retentionPolicy: retentionPolicy,
	}
}

// GetLogs retrieves logs for the given address and block range.
func (s *LogStore) GetLogs(
	ctx context.Context,
	address ethcommon.Address,
	fromBlock, toBlock uint64,
) ([]types.Log, []store.CoverageRange, error) {
	// Get coverage information
	const coverageQuery = `
		SELECT * FROM log_coverage
		WHERE address = ? AND from_block <= ? AND to_block >= ?
		ORDER BY from_block ASC
	`
	var dbCoverages []*dbCoverage
	err := meddler.QueryAll(s.db, &dbCoverages, coverageQuery, address.Hex(), toBlock, fromBlock)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to query coverage: %w", err)
	}

	coverage := make([]store.CoverageRange, len(dbCoverages))
	for i, c := range dbCoverages {
		coverage[i] = store.CoverageRange{
			FromBlock: c.FromBlock,
			ToBlock:   c.ToBlock,
		}
	}

	// Get logs for the requested range
	const logsQuery = `
		SELECT * FROM event_logs
		WHERE address = ? AND block_number >= ? AND block_number <= ?
		ORDER BY block_number ASC, log_index ASC
	`
	var dbLogs []*dbLog
	err = meddler.QueryAll(s.db, &dbLogs, logsQuery, address.Hex(), fromBlock, toBlock)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to query logs: %w", err)
	}

	logs := make([]types.Log, len(dbLogs))
	for i, dl := range dbLogs {
		logs[i] = s.dbLogToEthLog(dl)
	}

	return logs, coverage, nil
}

// GetUnsyncedTopics checks which address-topic combinations have not been fully synced up to the given block.
// For each address, it returns the list of topics that are missing coverage up to upToBlock.
func (s *LogStore) GetUnsyncedTopics(
	ctx context.Context,
	addresses []ethcommon.Address,
	topics [][]ethcommon.Hash,
	upToBlock uint64,
) (*store.UnsyncedTopics, error) {
	result := store.NewUnsyncedTopics()

	// For each address-topic combination, check if there's complete coverage up to upToBlock
	for i, address := range addresses {
		addressTopics := topics[i]

		// Get the oldest block still in database for this address-topic combination
		// This accounts for retention policy pruning - we don't want to re-sync pruned data
		var oldestBlock sql.NullInt64
		err := s.db.QueryRowContext(ctx,
			"SELECT MIN(from_block) FROM topic_coverage WHERE address = ?",
			address.Hex()).Scan(&oldestBlock)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("failed to get oldest block for address: %w", err)
		}

		// Determine the starting block for coverage check
		// If we have coverage data, start from oldest retained block
		// If no coverage exists, start from 0 (need full sync)
		startBlock := uint64(0)
		if oldestBlock.Valid && oldestBlock.Int64 > 0 {
			startBlock = uint64(oldestBlock.Int64)
		}

		for _, topic := range addressTopics {
			// Query topic coverage for this address-topic combination
			const topicCoverageQuery = `
				SELECT from_block, to_block FROM topic_coverage
				WHERE address = ? AND topic0 = ? AND to_block >= ? AND from_block <= ?
				ORDER BY from_block ASC
			`

			var dbCoverages []*dbTopicCoverage
			err := meddler.QueryAll(s.db, &dbCoverages, topicCoverageQuery,
				address.Hex(), topic.Hex(), startBlock, upToBlock)
			if err != nil {
				return nil, fmt.Errorf("failed to query topic coverage: %w", err)
			}

			// Check if there's a gap in coverage from startBlock to upToBlock
			// We need continuous coverage from startBlock (accounting for pruning) to upToBlock
			if !s.hasCompleteCoverage(dbCoverages, startBlock, upToBlock) {
				var coverage store.CoverageRange
				if len(dbCoverages) > 0 {
					coverage = store.CoverageRange{
						FromBlock: dbCoverages[0].FromBlock,
						ToBlock:   dbCoverages[len(dbCoverages)-1].ToBlock,
					}
				}

				result.AddTopic(address, topic, coverage)
			}
		}
	}

	return result, nil
}

// hasCompleteCoverage checks if the coverage ranges fully cover [fromBlock, toBlock]
func (s *LogStore) hasCompleteCoverage(coverages []*dbTopicCoverage, fromBlock, toBlock uint64) bool {
	if len(coverages) == 0 {
		return false
	}

	// Sort by from_block (should already be sorted from query)
	currentBlock := fromBlock

	for _, coverage := range coverages {
		// If there's a gap before this coverage
		if coverage.FromBlock > currentBlock {
			return false
		}

		// Move current block forward
		if coverage.ToBlock >= currentBlock {
			currentBlock = coverage.ToBlock + 1
		}

		// If we've covered up to toBlock, we're done
		if currentBlock > toBlock {
			return true
		}
	}

	// Check if we covered all the way to toBlock
	return currentBlock > toBlock
}

// StoreLogs saves logs to the store for the given address and block range.
func (s *LogStore) StoreLogs(
	ctx context.Context,
	addresses []ethcommon.Address,
	topics [][]ethcommon.Hash,
	logs []types.Log,
	fromBlock, toBlock uint64,
) error {
	if len(addresses) != len(topics) {
		return fmt.Errorf("addresses and topics length mismatch: %d vs %d", len(addresses), len(topics))
	}

	if len(addresses) == 0 {
		s.log.Debugf("No addresses to store logs for, skipping store operation")
		return nil
	}

	if err := s.storeLogsInternal(ctx, addresses, topics, logs, fromBlock, toBlock); err != nil {
		return err
	}

	// Apply retention policy if enabled
	if err := s.applyRetentionIfNeeded(ctx); err != nil {
		// Log warning but don't fail the store operation
		s.log.Warnf("failed to apply retention policy: %v", err)
	}

	return nil
}

// storeLogsInternal handles the actual log storage
func (s *LogStore) storeLogsInternal(
	ctx context.Context,
	addresses []ethcommon.Address,
	topics [][]ethcommon.Hash,
	logs []types.Log,
	fromBlock, toBlock uint64,
) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			s.log.Errorf("failed to rollback transaction: %v", err)
		}
	}()

	g, errCtx := errgroup.WithContext(ctx)
	g.SetLimit(maxConcurrency)

	g.Go(func() error {
		// Insert logs
		for _, log := range logs {
			dbLog := s.ethLogToDbLog(&log)

			err := meddler.Insert(tx, "event_logs", dbLog)
			if err != nil {
				// If insert fails due to unique constraint, ignore (log already exists)
				// This can happen when re-indexing the same range
				continue
			}
		}

		return nil
	})

	for i, address := range addresses {
		topics := topics[i]

		g.Go(func() error {
			// Record coverage
			const coverageInsertQuery = `
			INSERT INTO log_coverage (address, from_block, to_block)
			VALUES (?, ?, ?)
			ON CONFLICT(address, from_block, to_block) DO NOTHING
			`

			_, err = tx.ExecContext(errCtx, coverageInsertQuery, address.Hex(), fromBlock, toBlock)
			if err != nil {
				return fmt.Errorf("failed to insert coverage: %w", err)
			}

			// Record topic-specific coverage for each topic queried
			const topicCoverageInsertQuery = `
			INSERT INTO topic_coverage (address, topic0, from_block, to_block)
			VALUES (?, ?, ?, ?)
			ON CONFLICT(address, topic0, from_block, to_block) DO NOTHING
			`

			for _, topic := range topics {
				_, err = tx.ExecContext(errCtx, topicCoverageInsertQuery,
					address.Hex(), topic.Hex(), fromBlock, toBlock)
				if err != nil {
					return fmt.Errorf("failed to insert topic coverage: %w", err)
				}
			}

			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	s.log.Debugf("Stored %d logs for %d addresses, blocks %d-%d",
		len(logs), len(addresses), fromBlock, toBlock)

	return nil
}

// HandleReorg marks logs as removed starting from the given block number.
func (s *LogStore) HandleReorg(ctx context.Context, fromBlock uint64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil {
			s.log.Errorf("failed to rollback transaction: %v", err)
		}
	}()

	// Delete logs from the reorg point onwards
	const deleteLogsQuery = `
		DELETE FROM event_logs
		WHERE block_number >= ?
	`

	result, err := tx.ExecContext(ctx, deleteLogsQuery, fromBlock)
	if err != nil {
		return fmt.Errorf("failed to delete logs: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()

	// Handle coverage from the reorg point
	// For ranges that span the reorg point (from_block < fromBlock AND to_block >= fromBlock),
	// we need to truncate them to end at fromBlock-1
	const updateCoverageQuery = `
		UPDATE log_coverage
		SET to_block = ?
		WHERE from_block < ? AND to_block >= ?
	`

	_, err = tx.ExecContext(ctx, updateCoverageQuery, fromBlock-1, fromBlock, fromBlock)
	if err != nil {
		return fmt.Errorf("failed to update coverage: %w", err)
	}

	// Delete coverage ranges that are entirely >= fromBlock
	const deleteCoverageQuery = `
		DELETE FROM log_coverage
		WHERE from_block >= ?
	`

	_, err = tx.ExecContext(ctx, deleteCoverageQuery, fromBlock)
	if err != nil {
		return fmt.Errorf("failed to delete coverage: %w", err)
	}

	// Handle topic coverage - same logic
	const updateTopicCoverageQuery = `
		UPDATE topic_coverage
		SET to_block = ?
		WHERE from_block < ? AND to_block >= ?
	`

	_, err = tx.Exec(updateTopicCoverageQuery, fromBlock-1, fromBlock, fromBlock)
	if err != nil {
		return fmt.Errorf("failed to update topic coverage: %w", err)
	}

	// Delete topic coverage ranges that are entirely >= fromBlock
	const deleteTopicCoverageQuery = `
		DELETE FROM topic_coverage
		WHERE from_block >= ?
	`

	_, err = tx.Exec(deleteTopicCoverageQuery, fromBlock)
	if err != nil {
		return fmt.Errorf("failed to delete topic coverage: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	s.log.Infof("Handled reorg from block %d, deleted %d logs", fromBlock, rowsAffected)

	return nil
}

// PruneLogsBeforeBlock removes logs before the given block number from the store.
func (s *LogStore) PruneLogsBeforeBlock(ctx context.Context, beforeBlock uint64) error {
	_, err := s.pruneLogsBeforeBlock(ctx, beforeBlock)
	return err
}

func (s *LogStore) pruneLogsBeforeBlock(ctx context.Context, beforeBlock uint64) (uint64, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			s.log.Errorf("failed to rollback transaction: %v", err)
		}
	}()

	var blockCount uint64
	err = tx.QueryRowContext(ctx,
		"SELECT COUNT(DISTINCT block_number) FROM event_logs WHERE block_number < ?",
		beforeBlock).Scan(&blockCount)
	if err != nil {
		return 0, fmt.Errorf("failed to count blocks to prune: %w", err)
	}

	// Delete logs
	const deleteLogsQuery = `
		DELETE FROM event_logs
		WHERE block_number < ?
	`

	result, err := tx.ExecContext(ctx, deleteLogsQuery, beforeBlock)
	if err != nil {
		return 0, fmt.Errorf("failed to delete logs: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()

	// Delete coverage
	const deleteCoverageQuery = `
		DELETE FROM log_coverage
		WHERE to_block < ?
	`

	_, err = tx.ExecContext(ctx, deleteCoverageQuery, beforeBlock)
	if err != nil {
		return 0, fmt.Errorf("failed to delete coverage: %w", err)
	}

	// Delete topic coverage
	const deleteTopicCoverageQuery = `
		DELETE FROM topic_coverage
		WHERE to_block < ?
	`

	_, err = tx.ExecContext(ctx, deleteTopicCoverageQuery, beforeBlock)
	if err != nil {
		return 0, fmt.Errorf("failed to delete topic coverage: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Vacuum to reclaim space
	if err := db.Vacuum(s.db); err != nil {
		s.log.Warnf("failed to vacuum database: %v", err)
	}

	s.log.Infof("Pruned %d logs before block %d", rowsAffected, beforeBlock)

	return blockCount, nil
}

// Close closes the log store.
func (s *LogStore) Close() error {
	// The database connection is managed externally, so we don't close it here
	return nil
}

// ethLogToDbLog converts an Ethereum log to a database log.
func (s *LogStore) ethLogToDbLog(log *types.Log) *dbLog {
	dbLog := &dbLog{
		Address:     log.Address,
		BlockNumber: log.BlockNumber,
		BlockHash:   log.BlockHash,
		TxHash:      log.TxHash,
		TxIndex:     log.TxIndex,
		LogIndex:    log.Index,
		Data:        log.Data,
	}

	// Convert topics

	if len(log.Topics) > 0 {
		topic0 := log.Topics[0]
		dbLog.Topic0 = &topic0
	}
	if len(log.Topics) > 1 {
		topic1 := log.Topics[1]
		dbLog.Topic1 = &topic1
	}
	if len(log.Topics) > 2 { //nolint:mnd
		topic2 := log.Topics[2]
		dbLog.Topic2 = &topic2
	}
	if len(log.Topics) > 3 { //nolint:mnd
		topic3 := log.Topics[3]
		dbLog.Topic3 = &topic3
	}

	return dbLog
}

// dbLogToEthLog converts a database log to an Ethereum log.
func (s *LogStore) dbLogToEthLog(dbLog *dbLog) types.Log {
	log := types.Log{
		Address:     dbLog.Address,
		BlockNumber: dbLog.BlockNumber,
		BlockHash:   dbLog.BlockHash,
		TxHash:      dbLog.TxHash,
		TxIndex:     dbLog.TxIndex,
		Index:       dbLog.LogIndex,
		Data:        dbLog.Data,
	}

	// Convert topics
	topics := []ethcommon.Hash{}
	if dbLog.Topic0 != nil {
		topics = append(topics, *dbLog.Topic0)
	}
	if dbLog.Topic1 != nil {
		topics = append(topics, *dbLog.Topic1)
	}
	if dbLog.Topic2 != nil {
		topics = append(topics, *dbLog.Topic2)
	}
	if dbLog.Topic3 != nil {
		topics = append(topics, *dbLog.Topic3)
	}
	log.Topics = topics

	return log
}

// applyRetentionIfNeeded checks and applies retention policy if conditions are met
func (s *LogStore) applyRetentionIfNeeded(ctx context.Context) error {
	if !s.retentionPolicy.IsEnabled() {
		return nil
	}

	var pruneBeforeBlock uint64

	// Calculate prune threshold based on block age
	if s.retentionPolicy.MaxBlocks > 0 {
		// select min and max block numbers in the database
		var oldestBlock, newestBlock uint64

		err := s.db.QueryRowContext(ctx,
			"SELECT MIN(from_block), MAX(to_block) FROM log_coverage").
			Scan(&oldestBlock, &newestBlock)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("failed to get block range: %w", err)
		}

		if newestBlock > oldestBlock && newestBlock-oldestBlock > s.retentionPolicy.MaxBlocks {
			pruneBeforeBlock = newestBlock - s.retentionPolicy.MaxBlocks
		}
	}

	// Check database size and adjust if needed
	if s.retentionPolicy.MaxDBSizeMB > 0 {
		dbSize, err := s.getDatabaseSizeMB()
		if err != nil {
			return fmt.Errorf("failed to get database size: %w", err)
		}

		if dbSize >= s.retentionPolicy.MaxDBSizeMB {
			s.log.Warnf("Database size (%d MB) exceeds limit (%d MB), calculating blocks to prune",
				dbSize, s.retentionPolicy.MaxDBSizeMB)

			// Calculate how many blocks to prune based on size
			blockToPrune, err := s.calculateBlocksToFreeSpace(ctx, dbSize, s.retentionPolicy.MaxDBSizeMB)
			if err != nil {
				return fmt.Errorf("failed to calculate blocks to prune: %w", err)
			}

			// Use the more aggressive threshold
			if blockToPrune > pruneBeforeBlock {
				pruneBeforeBlock = blockToPrune
			}
		}
	}

	if pruneBeforeBlock == 0 {
		return nil
	}

	// Prune logs before the threshold
	blocksPruned, err := s.pruneLogsBeforeBlock(ctx, pruneBeforeBlock)
	if err != nil {
		return err
	}

	if blocksPruned > 0 {
		s.log.Infof("Applied retention policy: pruned %d blocks (before block %d)",
			blocksPruned, pruneBeforeBlock)
	}

	return nil
}

// getDatabaseSizeMB returns the current database size in megabytes
func (s *LogStore) getDatabaseSizeMB() (uint64, error) {
	sizeBytes, err := db.DBTotalSize(s.dbConfig.Path)
	if err != nil {
		return 0, err
	}
	return common.BytesToMB(uint64(sizeBytes)), nil
}

// calculateBlocksToFreeSpace estimates which block to prune to free the target space
func (s *LogStore) calculateBlocksToFreeSpace(ctx context.Context, currentMB, maxMB uint64) (uint64, error) {
	var oldestBlock, newestBlock uint64

	err := s.db.QueryRowContext(ctx,
		"SELECT MIN(from_block), MAX(to_block) FROM log_coverage").
		Scan(&oldestBlock, &newestBlock)
	if err != nil {
		return 0, fmt.Errorf("failed to get block range: %w", err)
	}

	if oldestBlock == 0 && newestBlock == 0 {
		return 0, nil
	}

	totalBlocks := newestBlock - oldestBlock + 1
	targetBytes := common.MBToBytes(currentMB - maxMB)
	totalBytes := common.MBToBytes(currentMB)

	// Get counts for all tables
	var eventLogCount, logCoverageCount, topicCoverageCount int64

	err = s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM event_logs").Scan(&eventLogCount)
	if err != nil {
		return 0, fmt.Errorf("failed to get event_logs count: %w", err)
	}

	err = s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM log_coverage").Scan(&logCoverageCount)
	if err != nil {
		return 0, fmt.Errorf("failed to get log_coverage count: %w", err)
	}

	err = s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM topic_coverage").Scan(&topicCoverageCount)
	if err != nil {
		return 0, fmt.Errorf("failed to get topic_coverage count: %w", err)
	}

	if eventLogCount == 0 {
		return 0, nil
	}

	// Estimate average bytes per row (weighted by table)
	// event_logs are typically larger (addresses, hashes, data)
	// coverage tables are smaller (just addresses and block numbers)
	// Use a rough weight: event_logs ~= 3x coverage row size
	const eventLogWeight = 3
	const coverageWeight = 1

	totalWeightedRows := (eventLogCount * eventLogWeight) +
		(logCoverageCount * coverageWeight) +
		(topicCoverageCount * coverageWeight)

	avgBytesPerWeightedRow := int64(totalBytes) / totalWeightedRows

	// Estimate rows per block for each table
	avgEventLogsPerBlock := float64(eventLogCount) / float64(totalBlocks)
	avgLogCoveragePerBlock := float64(logCoverageCount) / float64(totalBlocks)
	avgTopicCoveragePerBlock := float64(topicCoverageCount) / float64(totalBlocks)

	// Calculate how many blocks we need to delete to free targetBytes
	// For each block deleted, we free:
	// - avgEventLogsPerBlock * eventLogWeight * avgBytesPerWeightedRow
	// - avgLogCoveragePerBlock * coverageWeight * avgBytesPerWeightedRow
	// - avgTopicCoveragePerBlock * coverageWeight * avgBytesPerWeightedRow

	bytesFreedPerBlock := int64(
		(avgEventLogsPerBlock * eventLogWeight * float64(avgBytesPerWeightedRow)) +
			(avgLogCoveragePerBlock * coverageWeight * float64(avgBytesPerWeightedRow)) +
			(avgTopicCoveragePerBlock * coverageWeight * float64(avgBytesPerWeightedRow)),
	)

	if bytesFreedPerBlock <= 0 {
		bytesFreedPerBlock = 1 // avoid division by zero
	}

	blocksToDelete := uint64(int64(targetBytes) / bytesFreedPerBlock)
	const safetyMarginPercent = 10

	// Add safety margin (10%) since this is an estimate
	blocksToDelete += blocksToDelete / safetyMarginPercent

	if blocksToDelete == 0 {
		blocksToDelete = totalBlocks / safetyMarginPercent // fallback: delete 10% of blocks
	}

	// Ensure we don't try to delete more blocks than we have
	if blocksToDelete > totalBlocks {
		blocksToDelete = totalBlocks
	}

	pruneBeforeBlock := oldestBlock + blocksToDelete

	s.log.Infof("Calculated prune threshold: block %d (deleting ~%d blocks to free ~%d MB)",
		pruneBeforeBlock, blocksToDelete, currentMB-maxMB)
	s.log.Debugf("Size calculation - event_logs: %d, log_coverage: %d, topic_coverage: %d, total blocks: %d",
		eventLogCount, logCoverageCount, topicCoverageCount, totalBlocks)

	return pruneBeforeBlock, nil
}
