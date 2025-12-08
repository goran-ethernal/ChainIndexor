package fetcher

import (
	"context"

	"github.com/ethereum/go-ethereum/core/types"
)

// LogFetcher defines the interface for fetching logs and block headers from the blockchain.
// This abstraction allows for easier testing and alternative implementations.
type LogFetcher interface {
	// SetMode changes the fetcher's operating mode.
	SetMode(mode FetchMode)

	// GetMode returns the current operating mode.
	GetMode() FetchMode

	// FetchRange fetches logs and headers for a specific block range.
	// It verifies consistency using the ReorgDetector and returns an error if a reorg is detected.
	FetchRange(ctx context.Context, fromBlock, toBlock uint64) (*FetchResult, error)

	// FetchNext fetches the next chunk of logs based on the current mode.
	// For backfill mode, it fetches from the given block up to chunk_size.
	// For live mode, it fetches new blocks since the last checkpoint.
	FetchNext(ctx context.Context, lastIndexedBlock uint64, downloaderStartBlock uint64) (*FetchResult, error)
}

// FetchMode represents the operating mode of the log fetcher.
type FetchMode string

const (
	// ModeBackfill fetches historical blocks in chunks
	ModeBackfill FetchMode = "backfill"
	// ModeLive tails new blocks as they arrive
	ModeLive FetchMode = "live"
)

// String returns the string representation of the mode.
func (m FetchMode) String() string {
	return string(m)
}

// FetchResult contains the results of a log fetch operation.
type FetchResult struct {
	Logs      []types.Log
	Headers   []*types.Header
	FromBlock uint64
	ToBlock   uint64
}
