package store

import (
	"context"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

// LogStore defines the interface for storing and retrieving blockchain logs.
// It provides caching capabilities to avoid redundant RPC calls when multiple
// indexers need the same log data.
type LogStore interface {
	// GetLogs retrieves logs for the given address and block range.
	// Returns logs that have been previously stored.
	// Also returns coverage information indicating which block ranges are available in the store.
	GetLogs(
		ctx context.Context,
		address common.Address,
		fromBlock, toBlock uint64,
	) (logs []types.Log, coverage []CoverageRange, err error)

	// StoreLogs saves logs to the store for the given address and block range.
	// This should be called after fetching logs from the RPC node.
	// The store will track coverage to know which ranges have been downloaded.
	// topics parameter specifies which topics were queried (first element of each log's Topics array).
	StoreLogs(
		ctx context.Context,
		addresses []common.Address,
		topics [][]common.Hash,
		logs []types.Log,
		fromBlock, toBlock uint64,
		lastFinalizedBlock *types.Header,
	) error

	// HandleReorg deletes logs starting from the given block number.
	// This should be called when a reorg is detected to remove invalidated cached data.
	HandleReorg(ctx context.Context, fromBlock uint64) error

	// PruneLogsBeforeBlock removes logs before the given block number from the store.
	// This is used to clean up old finalized data and save storage space.
	PruneLogsBeforeBlock(ctx context.Context, beforeBlock uint64) error

	// GetUnsyncedTopics returns a map of addresses to topics that have not been synced up to the given block.
	// This is useful for determining which address-topic combinations need to be fetched.
	GetUnsyncedTopics(
		ctx context.Context,
		addresses []common.Address,
		topics [][]common.Hash,
		upToBlock uint64,
	) (*UnsyncedTopics, error)

	// Close closes the log store and releases any resources.
	Close() error
}
