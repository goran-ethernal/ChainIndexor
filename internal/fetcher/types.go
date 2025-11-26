package fetcher

import "github.com/ethereum/go-ethereum/core/types"

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
