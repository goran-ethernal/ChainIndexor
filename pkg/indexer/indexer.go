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
	EventsToIndex() map[common.Address][]common.Hash

	// HandleLogs processes a batch of logs received from the downloader.
	// Implementations should decode and persist the relevant events.
	HandleLogs(logs []types.Log) error

	// HandleReorg handles a blockchain reorganization starting from the given block number.
	// Implementations should roll back any data persisted at or after this block.
	HandleReorg(blockNum uint64) error
}
