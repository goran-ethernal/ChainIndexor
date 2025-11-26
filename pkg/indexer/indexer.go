package indexer

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

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
}
