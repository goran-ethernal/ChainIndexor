package downloader

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/goran-ethernal/ChainIndexor/internal/db"
	"github.com/goran-ethernal/ChainIndexor/internal/logger"
	pkgdownloader "github.com/goran-ethernal/ChainIndexor/pkg/downloader"
	"github.com/goran-ethernal/ChainIndexor/pkg/fetcher"
	"github.com/russross/meddler"
)

// Compile-time check to ensure SyncManager implements pkgdownloader.SyncManager interface.
var _ pkgdownloader.SyncManager = (*SyncManager)(nil)

// SyncManager manages the synchronization state and checkpoints.
// It implements the pkgdownloader.SyncManager interface.
type SyncManager struct {
	db                     *sql.DB
	log                    *logger.Logger
	maintenanceCoordinator db.Maintenance
}

// SyncState is a type alias for the public SyncState type.
// Uses meddler tags for automatic struct-to-db mapping.
type SyncState = pkgdownloader.SyncState

// NewSyncManager creates a new SyncManager instance.
func NewSyncManager(db *sql.DB, log *logger.Logger,
	maintenanceCoordinator db.Maintenance) (*SyncManager, error) {
	sm := &SyncManager{
		db:                     db,
		log:                    log.WithComponent("sync-manager"),
		maintenanceCoordinator: maintenanceCoordinator,
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
	// Acquire operation lock if maintenance coordinator is available
	if sm.maintenanceCoordinator != nil {
		unlock := sm.maintenanceCoordinator.AcquireOperationLock()
		defer unlock()
	}

	var state SyncState
	err := meddler.QueryRow(sm.db, &state, `SELECT * FROM sync_state WHERE id = 1`)
	if err != nil {
		return nil, fmt.Errorf("failed to get sync state: %w", err)
	}

	sm.log.Debugf("retrieved sync state: last_block=%d, last_block_hash=%s, mode=%s",
		state.LastIndexedBlock,
		state.LastIndexedBlockHash.Hex(),
		state.Mode,
	)

	return &state, nil
}

// SaveCheckpoint saves a checkpoint with the given block number, hash, and mode.
func (sm *SyncManager) SaveCheckpoint(blockNum uint64, blockHash common.Hash, mode fetcher.FetchMode) error {
	// Acquire operation lock if maintenance coordinator is available
	if sm.maintenanceCoordinator != nil {
		unlock := sm.maintenanceCoordinator.AcquireOperationLock()
		defer unlock()
	}

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

	sm.log.Debugf("saved checkpoint: block=%d, block_hash=%s, mode=%s, timestamp=%d",
		blockNum,
		blockHash.Hex(),
		mode,
		state.LastIndexedTimestamp,
	)

	return nil
}

// SetMode updates the synchronization mode.
func (sm *SyncManager) SetMode(mode fetcher.FetchMode) error {
	// Acquire operation lock if maintenance coordinator is available
	if sm.maintenanceCoordinator != nil {
		unlock := sm.maintenanceCoordinator.AcquireOperationLock()
		defer unlock()
	}

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

	sm.log.Infof("sync mode updated: mode=%s", mode)

	return nil
}

// Reset resets the sync state to the given starting block.
// This is useful for reindexing from a specific block.
func (sm *SyncManager) Reset(startBlock uint64) error {
	// Acquire operation lock if maintenance coordinator is available
	if sm.maintenanceCoordinator != nil {
		unlock := sm.maintenanceCoordinator.AcquireOperationLock()
		defer unlock()
	}

	state := SyncState{
		ID:                   1,
		LastIndexedBlock:     startBlock,
		LastIndexedBlockHash: common.Hash{},
		LastIndexedTimestamp: time.Now().Unix(),
		Mode:                 string(fetcher.ModeBackfill),
	}

	err := meddler.Update(sm.db, "sync_state", &state)
	if err != nil {
		return fmt.Errorf("failed to reset sync state: %w", err)
	}

	sm.log.Warnf("sync state reset: start_block=%d, mode=%s",
		startBlock,
		fetcher.ModeBackfill,
	)

	return nil
}

// Close closes the database connection.
func (sm *SyncManager) Close() error {
	return sm.db.Close()
}

// DB returns the database connection for use by other components.
func (sm *SyncManager) DB() *sql.DB {
	return sm.db
}
