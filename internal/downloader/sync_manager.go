package downloader

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/goran-ethernal/ChainIndexor/internal/db"
	"github.com/goran-ethernal/ChainIndexor/internal/downloader/migrations"
	"github.com/goran-ethernal/ChainIndexor/internal/logger"
	"github.com/russross/meddler"
)

// SyncManager manages the synchronization state and checkpoints.
type SyncManager struct {
	db  *sql.DB
	log *logger.Logger
}

// SyncState represents the current synchronization state.
// Uses meddler tags for automatic struct-to-db mapping.
type SyncState struct {
	ID                   int         `meddler:"id,pk" json:"-"`
	LastIndexedBlock     uint64      `meddler:"last_indexed_block" json:"last_indexed_block"`
	LastIndexedBlockHash common.Hash `meddler:"last_indexed_block_hash,hash" json:"last_indexed_block_hash"`
	LastIndexedTimestamp int64       `meddler:"last_indexed_timestamp" json:"last_indexed_timestamp"`
	Mode                 string      `meddler:"mode" json:"mode"`
}

// GetMode returns the Mode as a FetchMode type.
func (s *SyncState) GetMode() FetchMode {
	return FetchMode(s.Mode)
}

// SetMode sets the Mode from a FetchMode type.
func (s *SyncState) SetMode(mode FetchMode) {
	s.Mode = string(mode)
}

// NewSyncManager creates a new SyncManager instance.
func NewSyncManager(dbPath string, log *logger.Logger) (*SyncManager, error) {
	if err := migrations.RunMigrations(dbPath); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	// Open database connection first
	database, err := db.NewSQLiteDB(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	sm := &SyncManager{
		db:  database,
		log: log.WithComponent("sync-manager"),
	}

	sm.log.Info("sync manager initialized")

	return sm, nil
}

// GetLastIndexedBlock returns the last successfully indexed block number.
func (sm *SyncManager) GetLastIndexedBlock() (uint64, error) {
	var lastBlock uint64
	err := sm.db.QueryRow(`
		SELECT last_indexed_block FROM sync_state WHERE id = 1
	`).Scan(&lastBlock)

	if err != nil {
		return 0, fmt.Errorf("failed to get last indexed block: %w", err)
	}

	return lastBlock, nil
}

// GetState returns the current synchronization state.
func (sm *SyncManager) GetState() (*SyncState, error) {
	var state SyncState
	err := meddler.QueryRow(sm.db, &state, `SELECT * FROM sync_state WHERE id = 1`)
	if err != nil {
		return nil, fmt.Errorf("failed to get sync state: %w", err)
	}

	sm.log.Debugw("retrieved sync state",
		"last_block", state.LastIndexedBlock,
		"last_block_hash", state.LastIndexedBlockHash,
		"mode", state.Mode,
	)

	return &state, nil
}

// SaveCheckpoint saves a checkpoint with the given block number, hash, and mode.
func (sm *SyncManager) SaveCheckpoint(blockNum uint64, blockHash common.Hash, mode FetchMode) error {
	state := SyncState{
		ID:                   1,
		LastIndexedBlock:     blockNum,
		LastIndexedBlockHash: blockHash,
		LastIndexedTimestamp: time.Now().Unix(),
		Mode:                 string(mode),
	}

	err := meddler.Update(sm.db, "sync_state", &state)
	if err != nil {
		return fmt.Errorf("failed to save checkpoint: %w", err)
	}

	sm.log.Debugw("saved checkpoint",
		"block", blockNum,
		"block_hash", blockHash.Hex(),
		"mode", mode,
		"timestamp", state.LastIndexedTimestamp,
	)

	return nil
}

// SetMode updates the synchronization mode.
func (sm *SyncManager) SetMode(mode FetchMode) error {
	// First get current state to preserve other fields
	state, err := sm.GetState()
	if err != nil {
		return fmt.Errorf("failed to get current state: %w", err)
	}

	state.ID = 1
	state.Mode = string(mode)

	err = meddler.Update(sm.db, "sync_state", state)
	if err != nil {
		return fmt.Errorf("failed to set mode: %w", err)
	}

	sm.log.Infow("sync mode updated", "mode", mode)

	return nil
}

// Reset resets the sync state to the given starting block.
// This is useful for reindexing from a specific block.
func (sm *SyncManager) Reset(startBlock uint64) error {
	state := SyncState{
		ID:                   1,
		LastIndexedBlock:     startBlock,
		LastIndexedBlockHash: common.Hash{},
		LastIndexedTimestamp: time.Now().Unix(),
		Mode:                 string(ModeBackfill),
	}

	err := meddler.Update(sm.db, "sync_state", &state)
	if err != nil {
		return fmt.Errorf("failed to reset sync state: %w", err)
	}

	sm.log.Warnw("sync state reset",
		"start_block", startBlock,
		"mode", ModeBackfill,
	)

	return nil
}

// Close closes the database connection.
func (sm *SyncManager) Close() error {
	return sm.db.Close()
}
