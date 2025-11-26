package store

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/goran-ethernal/ChainIndexor/internal/logger"
	"github.com/goran-ethernal/ChainIndexor/pkg/fetcher/store"
	"github.com/russross/meddler"
)

// LogStore implements LogStore interface using SQLite as the backend.
type LogStore struct {
	db  *sql.DB
	log *logger.Logger
}

// NewLogStore creates a new SQLite-backed LogStore.
func NewLogStore(db *sql.DB, log *logger.Logger) *LogStore {
	return &LogStore{
		db:  db,
		log: log,
	}
}

// GetLogs retrieves logs for the given address and block range.
func (s *LogStore) GetLogs(ctx context.Context, address common.Address, fromBlock, toBlock uint64) ([]types.Log, []store.CoverageRange, error) {
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
		log, err := s.dbLogToEthLog(dl)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to convert db log to eth log: %w", err)
		}
		logs[i] = log
	}

	return logs, coverage, nil
}

// GetUnsyncedTopics checks which address-topic combinations have not been fully synced up to the given block.
// For each address, it returns the list of topics that are missing coverage up to upToBlock.
func (s *LogStore) GetUnsyncedTopics(ctx context.Context, addresses []common.Address, topics [][]common.Hash, upToBlock uint64) (map[common.Address][]common.Hash, error) {
	result := make(map[common.Address][]common.Hash)

	// For each address-topic combination, check if there's complete coverage up to upToBlock
	for i, address := range addresses {
		addressTopics := topics[i]
		unsyncedTopics := []common.Hash{}

		for _, topic := range addressTopics {
			// Query topic coverage for this address-topic combination
			const topicCoverageQuery = `
				SELECT from_block, to_block FROM topic_coverage
				WHERE address = ? AND topic0 = ? AND to_block >= ? AND from_block <= ?
				ORDER BY from_block ASC
			`

			var dbCoverages []*dbTopicCoverage
			err := meddler.QueryAll(s.db, &dbCoverages, topicCoverageQuery, address.Hex(), topic.Hex(), 0, upToBlock)
			if err != nil {
				return nil, fmt.Errorf("failed to query topic coverage: %w", err)
			}

			// Check if there's a gap in coverage from 0 to upToBlock
			// We need continuous coverage from 0 to upToBlock
			if !s.hasCompleteCoverage(dbCoverages, 0, upToBlock) {
				unsyncedTopics = append(unsyncedTopics, topic)
			}
		}

		if len(unsyncedTopics) > 0 {
			result[address] = unsyncedTopics
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
func (s *LogStore) StoreLogs(ctx context.Context, address common.Address, topics []common.Hash, fromBlock, toBlock uint64, logs []types.Log) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil {
			s.log.Errorf("failed to rollback transaction: %v", err)
		}
	}()

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

	// Record coverage
	const coverageInsertQuery = `
		INSERT INTO log_coverage (address, from_block, to_block)
		VALUES (?, ?, ?)
		ON CONFLICT(address, from_block, to_block) DO NOTHING
	`

	_, err = tx.Exec(coverageInsertQuery, address.Hex(), fromBlock, toBlock)
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
		_, err = tx.Exec(topicCoverageInsertQuery, address.Hex(), topic.Hex(), fromBlock, toBlock)
		if err != nil {
			return fmt.Errorf("failed to insert topic coverage: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	s.log.Debugf("Stored %d logs for address %s, %d topics, blocks %d-%d", len(logs), address.Hex(), len(topics), fromBlock, toBlock)

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

	result, err := tx.Exec(deleteLogsQuery, fromBlock)
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

	_, err = tx.Exec(updateCoverageQuery, fromBlock-1, fromBlock, fromBlock)
	if err != nil {
		return fmt.Errorf("failed to update coverage: %w", err)
	}

	// Delete coverage ranges that are entirely >= fromBlock
	const deleteCoverageQuery = `
		DELETE FROM log_coverage
		WHERE from_block >= ?
	`

	_, err = tx.Exec(deleteCoverageQuery, fromBlock)
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
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil {
			s.log.Errorf("failed to rollback transaction: %v", err)
		}
	}()

	// Delete logs
	const deleteLogsQuery = `
		DELETE FROM event_logs
		WHERE block_number < ?
	`

	result, err := tx.Exec(deleteLogsQuery, beforeBlock)
	if err != nil {
		return fmt.Errorf("failed to delete logs: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()

	// Delete coverage
	const deleteCoverageQuery = `
		DELETE FROM log_coverage
		WHERE to_block < ?
	`

	_, err = tx.Exec(deleteCoverageQuery, beforeBlock)
	if err != nil {
		return fmt.Errorf("failed to delete coverage: %w", err)
	}

	// Delete topic coverage
	const deleteTopicCoverageQuery = `
		DELETE FROM topic_coverage
		WHERE to_block < ?
	`

	_, err = tx.Exec(deleteTopicCoverageQuery, beforeBlock)
	if err != nil {
		return fmt.Errorf("failed to delete topic coverage: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	s.log.Infof("Pruned %d logs before block %d", rowsAffected, beforeBlock)

	return nil
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
	if len(log.Topics) > 2 {
		topic2 := log.Topics[2]
		dbLog.Topic2 = &topic2
	}
	if len(log.Topics) > 3 {
		topic3 := log.Topics[3]
		dbLog.Topic3 = &topic3
	}

	return dbLog
}

// dbLogToEthLog converts a database log to an Ethereum log.
func (s *LogStore) dbLogToEthLog(dbLog *dbLog) (types.Log, error) {
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
	topics := []common.Hash{}
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

	return log, nil
}
