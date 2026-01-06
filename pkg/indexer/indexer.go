package indexer

import (
	"context"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

const defaultPageLimit = 100

// Indexer defines the interface that all indexers must implement.
// Indexers receive logs from the downloader and handle blockchain reorganizations.
type Indexer interface {
	// EventsToIndex returns a map of contract addresses to their event topic hashes.
	// This is used by the coordinator to determine which logs should be sent to this indexer.
	// The inner map is a set (using struct{} as values) of topic hashes for each address.
	EventsToIndex() map[common.Address]map[common.Hash]struct{}

	// HandleLogs processes a batch of logs received from the downloader.
	// Implementations should decode and persist the relevant events.
	HandleLogs(logs []types.Log) error

	// HandleReorg handles a blockchain reorganization starting from the given block number.
	// Implementations should roll back any data persisted at or after this block.
	HandleReorg(blockNum uint64) error

	// StartBlock returns the block number from which this indexer wants to start processing logs.
	// The downloader will use the minimum StartBlock across all registered indexers to determine
	// the earliest block to fetch. Each indexer will only receive logs from blocks >= its StartBlock.
	StartBlock() uint64

	// GetType returns the type identifier of the indexer (e.g., "erc20", "erc721").
	GetType() string

	// GetName returns the configured name of the indexer instance.
	GetName() string
}

// Queryable is an optional interface that indexers can implement to support API queries.
type Queryable interface {
	// QueryEvents retrieves events based on the provided query parameters.
	// Returns the events slice, total count, and any error.
	QueryEvents(ctx context.Context, params QueryParams) (interface{}, int, error)

	// GetStats returns statistics about the indexed data.
	// Returns a map of stat names to values.
	GetStats(ctx context.Context) (interface{}, error)

	// GetEventTypes returns the list of event type names this indexer handles.
	GetEventTypes() []string
}

// QueryParams represents common query parameters for event retrieval.
type QueryParams struct {
	// Event type to query (e.g., "Transfer", "Approval")
	EventType string

	// Pagination
	Limit  int
	Offset int

	// Block range filtering
	FromBlock *uint64
	ToBlock   *uint64

	// Address filtering
	Address string

	// Sorting
	SortBy    string
	SortOrder string // "asc" or "desc"
}

func NewDefaultQueryParams() *QueryParams {
	return &QueryParams{
		Limit:     defaultPageLimit,
		Offset:    0,
		SortOrder: "desc",
	}
}
