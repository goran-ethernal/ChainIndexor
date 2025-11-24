package downloader

import (
	"github.com/ethereum/go-ethereum/core/types"
)

// FetchMode represents the operating mode of the log fetcher.
type FetchMode string

const (
	// ModeBackfill fetches historical blocks from start to finalized head
	ModeBackfill FetchMode = "backfill"

	// ModeLive tails new blocks as they arrive
	ModeLive FetchMode = "live"
)

// String returns the string representation of FetchMode.
func (m FetchMode) String() string {
	return string(m)
}

// FetchResult contains the results of a successful fetch operation.
type FetchResult struct {
	// Logs are the fetched logs
	Logs []types.Log

	// Headers are the block headers for the fetched range
	Headers []*types.Header

	// FromBlock is the starting block number of this fetch
	FromBlock uint64

	// ToBlock is the ending block number of this fetch
	ToBlock uint64
}
