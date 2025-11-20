package indexer

import (
	"fmt"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

// IndexerCoordinator manages multiple indexers and routes events to them based on topics.
type IndexerCoordinator struct {
	mu sync.RWMutex

	// topicToIndexers maps event topics to the indexers that are interested in them
	topicToIndexers map[common.Hash][]Indexer

	// indexers holds all registered indexers
	indexers []Indexer
}

// NewIndexerCoordinator creates a new IndexerCoordinator.
func NewIndexerCoordinator() *IndexerCoordinator {
	return &IndexerCoordinator{
		topicToIndexers: make(map[common.Hash][]Indexer),
		indexers:        make([]Indexer, 0),
	}
}

// RegisterIndexer registers a new indexer and associates it with the given topics.
// The indexer will receive logs that match any of the specified topics.
func (ic *IndexerCoordinator) RegisterIndexer(indexer Indexer, topics []common.Hash) {
	ic.mu.Lock()
	defer ic.mu.Unlock()

	// Add to the list of all indexers
	ic.indexers = append(ic.indexers, indexer)

	// Map topics to this indexer
	for _, topic := range topics {
		ic.topicToIndexers[topic] = append(ic.topicToIndexers[topic], indexer)
	}
}

// HandleLogs processes a batch of logs and routes them to the appropriate indexers.
// Each log is sent to all indexers that registered interest in its topic.
func (ic *IndexerCoordinator) HandleLogs(logs []types.Log) error {
	ic.mu.RLock()
	defer ic.mu.RUnlock()

	// Group logs by indexer to avoid duplicate processing
	indexerLogs := make(map[Indexer][]types.Log)

	for _, log := range logs {
		if len(log.Topics) == 0 {
			continue
		}

		// The first topic is the event signature
		eventTopic := log.Topics[0]

		// Find all indexers interested in this topic
		if indexers, ok := ic.topicToIndexers[eventTopic]; ok {
			for _, indexer := range indexers {
				indexerLogs[indexer] = append(indexerLogs[indexer], log)
			}
		}
	}

	// Call HandleLogs for each indexer with their relevant logs
	for indexer, relevantLogs := range indexerLogs {
		if err := indexer.HandleLogs(relevantLogs); err != nil {
			return fmt.Errorf("indexer failed to handle logs: %w", err)
		}
	}

	return nil
}

// HandleReorg notifies all registered indexers about a blockchain reorganization.
// All indexers are called sequentially to roll back their state.
func (ic *IndexerCoordinator) HandleReorg(blockNum uint64) error {
	ic.mu.RLock()
	defer ic.mu.RUnlock()

	for _, indexer := range ic.indexers {
		if err := indexer.HandleReorg(blockNum); err != nil {
			return fmt.Errorf("indexer failed to handle reorg at block %d: %w", blockNum, err)
		}
	}

	return nil
}
