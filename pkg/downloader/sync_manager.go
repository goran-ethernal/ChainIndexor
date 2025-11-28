package downloader

import (
	"database/sql"

	"github.com/ethereum/go-ethereum/common"
	"github.com/goran-ethernal/ChainIndexor/pkg/fetcher"
)

// SyncManager defines the interface for managing synchronization state and checkpoints.
// This abstraction allows for easier testing and alternative implementations.
type SyncManager interface {
	// GetLastIndexedBlock returns the last successfully indexed block number.
	GetLastIndexedBlock() (uint64, error)

	// GetState returns the current synchronization state.
	GetState() (*SyncState, error)

	// SaveCheckpoint saves a checkpoint with the given block number, hash, and mode.
	SaveCheckpoint(blockNum uint64, blockHash common.Hash, mode fetcher.FetchMode) error

	// SetMode updates the synchronization mode.
	SetMode(mode fetcher.FetchMode) error

	// Reset resets the sync state to the given starting block.
	// This is useful for reindexing from a specific block.
	Reset(startBlock uint64) error

	// Close closes the sync manager and releases any resources.
	Close() error

	// DB returns the database connection for use by other components.
	DB() *sql.DB
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

// GetMode returns the Mode as a fetcher.FetchMode type.
func (s *SyncState) GetMode() fetcher.FetchMode {
	return fetcher.FetchMode(s.Mode)
}
