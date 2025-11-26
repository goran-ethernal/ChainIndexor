package fetcher

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/goran-ethernal/ChainIndexor/internal/logger"
	"github.com/russross/meddler"
)

// LogStore defines the interface for storing and retrieving blockchain logs.
// It provides caching capabilities to avoid redundant RPC calls when multiple
// indexers need the same log data.
type LogStore interface {
	// GetLogs retrieves logs for the given address and block range.
	// Returns logs that have been previously stored and are still valid (not removed by reorg).
	// Also returns coverage information indicating which block ranges are available in the store.
	GetLogs(ctx context.Context, address common.Address, fromBlock, toBlock uint64) (logs []types.Log, coverage []CoverageRange, err error)

	// StoreLogs saves logs to the store for the given address and block range.
	// This should be called after fetching logs from the RPC node.
	// The store will track coverage to know which ranges have been downloaded.
	StoreLogs(ctx context.Context, address common.Address, fromBlock, toBlock uint64, logs []types.Log) error

	// HandleReorg marks logs as removed starting from the given block number.
	// This should be called when a reorg is detected to invalidate cached data.
	// Logs are not deleted but marked as removed so they won't be returned by GetLogs.
	HandleReorg(ctx context.Context, fromBlock uint64) error

	// PruneLogsBeforeBlock removes logs before the given block number from the store.
	// This is used to clean up old finalized data and save storage space.
	PruneLogsBeforeBlock(ctx context.Context, beforeBlock uint64) error

	// Close closes the log store and releases any resources.
	Close() error
}

// SQLiteLogStore implements LogStore using SQLite as the backend.
type SQLiteLogStore struct {
	db  *sql.DB
	log *logger.Logger
}

// dbLog represents a log entry in the database
type dbLog struct {
	ID          int64          `meddler:"id,pk"`
	Address     common.Address `meddler:"address,address"`
	BlockNumber uint64         `meddler:"block_number"`
	BlockHash   common.Hash    `meddler:"block_hash,hash"`
	TxHash      common.Hash    `meddler:"tx_hash,hash"`
	TxIndex     uint           `meddler:"tx_index"`
	LogIndex    uint           `meddler:"log_index"`
	Topic0      *common.Hash   `meddler:"topic0,hash"`
	Topic1      *common.Hash   `meddler:"topic1,hash"`
	Topic2      *common.Hash   `meddler:"topic2,hash"`
	Topic3      *common.Hash   `meddler:"topic3,hash"`
	Data        []byte         `meddler:"data"`
	Removed     bool           `meddler:"removed"`
	CreatedAt   string         `meddler:"created_at"`
}

// dbCoverage represents a coverage range in the database
type dbCoverage struct {
	ID        int64          `meddler:"id,pk"`
	Address   common.Address `meddler:"address,address"`
	FromBlock uint64         `meddler:"from_block"`
	ToBlock   uint64         `meddler:"to_block"`
	CreatedAt string         `meddler:"created_at"`
}

// NewSQLiteLogStore creates a new SQLite-backed LogStore.
func NewSQLiteLogStore(db *sql.DB, log *logger.Logger) *SQLiteLogStore {
	return &SQLiteLogStore{
		db:  db,
		log: log,
	}
}

// GetLogs retrieves logs for the given address and block range.
func (s *SQLiteLogStore) GetLogs(ctx context.Context, address common.Address, fromBlock, toBlock uint64) ([]types.Log, []CoverageRange, error) {
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

	coverage := make([]CoverageRange, len(dbCoverages))
	for i, c := range dbCoverages {
		coverage[i] = CoverageRange{
			FromBlock: c.FromBlock,
			ToBlock:   c.ToBlock,
		}
	}

	// Get logs for the requested range
	const logsQuery = `
		SELECT * FROM event_logs
		WHERE address = ? AND block_number >= ? AND block_number <= ? AND removed = 0
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

// StoreLogs saves logs to the store for the given address and block range.
func (s *SQLiteLogStore) StoreLogs(ctx context.Context, address common.Address, fromBlock, toBlock uint64, logs []types.Log) error {
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
	coverageInsert := `
		INSERT INTO log_coverage (address, from_block, to_block)
		VALUES (?, ?, ?)
		ON CONFLICT(address, from_block, to_block) DO NOTHING
	`

	_, err = tx.Exec(coverageInsert, address.Hex(), fromBlock, toBlock)
	if err != nil {
		return fmt.Errorf("failed to insert coverage: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	s.log.Debugf("Stored %d logs for address %s, blocks %d-%d", len(logs), address.Hex(), fromBlock, toBlock)

	return nil
}

// HandleReorg marks logs as removed starting from the given block number.
func (s *SQLiteLogStore) HandleReorg(ctx context.Context, fromBlock uint64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil {
			s.log.Errorf("failed to rollback transaction: %v", err)
		}
	}()

	// Mark logs as removed
	updateQuery := `
		UPDATE event_logs
		SET removed = 1
		WHERE block_number >= ?
	`

	result, err := tx.Exec(updateQuery, fromBlock)
	if err != nil {
		return fmt.Errorf("failed to mark logs as removed: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()

	// Delete coverage from the reorg point
	deleteCoverageQuery := `
		DELETE FROM log_coverage
		WHERE to_block >= ?
	`

	_, err = tx.Exec(deleteCoverageQuery, fromBlock)
	if err != nil {
		return fmt.Errorf("failed to delete coverage: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	s.log.Infof("Handled reorg from block %d, marked %d logs as removed", fromBlock, rowsAffected)

	return nil
}

// PruneLogsBeforeBlock removes logs before the given block number from the store.
func (s *SQLiteLogStore) PruneLogsBeforeBlock(ctx context.Context, beforeBlock uint64) error {
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
	deleteLogsQuery := `
		DELETE FROM event_logs
		WHERE block_number < ?
	`

	result, err := tx.Exec(deleteLogsQuery, beforeBlock)
	if err != nil {
		return fmt.Errorf("failed to delete logs: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()

	// Delete coverage
	deleteCoverageQuery := `
		DELETE FROM log_coverage
		WHERE to_block < ?
	`

	_, err = tx.Exec(deleteCoverageQuery, beforeBlock)
	if err != nil {
		return fmt.Errorf("failed to delete coverage: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	s.log.Infof("Pruned %d logs before block %d", rowsAffected, beforeBlock)

	return nil
}

// Close closes the log store.
func (s *SQLiteLogStore) Close() error {
	// The database connection is managed externally, so we don't close it here
	return nil
}

// ethLogToDbLog converts an Ethereum log to a database log.
func (s *SQLiteLogStore) ethLogToDbLog(log *types.Log) *dbLog {
	dbLog := &dbLog{
		Address:     log.Address,
		BlockNumber: log.BlockNumber,
		BlockHash:   log.BlockHash,
		TxHash:      log.TxHash,
		TxIndex:     log.TxIndex,
		LogIndex:    log.Index,
		Data:        log.Data,
		Removed:     log.Removed,
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
func (s *SQLiteLogStore) dbLogToEthLog(dbLog *dbLog) (types.Log, error) {
	log := types.Log{
		Address:     dbLog.Address,
		BlockNumber: dbLog.BlockNumber,
		BlockHash:   dbLog.BlockHash,
		TxHash:      dbLog.TxHash,
		TxIndex:     dbLog.TxIndex,
		Index:       dbLog.LogIndex,
		Data:        dbLog.Data,
		Removed:     dbLog.Removed,
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
