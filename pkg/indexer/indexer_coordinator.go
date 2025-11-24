package indexer

import (
	"fmt"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

// IndexerCoordinator manages multiple indexers and routes events to them based on address and topics.
type IndexerCoordinator struct {
	mu sync.RWMutex

	// addressTopics maps address -> topic -> indexers for specific topic filters
	addressTopics map[common.Address]map[common.Hash][]Indexer

	// addressAllTopics maps address -> indexers that want ALL topics from that address
	addressAllTopics map[common.Address][]Indexer

	// indexers holds all registered indexers
	indexers []Indexer
}

// NewIndexerCoordinator creates a new IndexerCoordinator.
func NewIndexerCoordinator() *IndexerCoordinator {
	return &IndexerCoordinator{
		indexers:         make([]Indexer, 0),
		addressTopics:    make(map[common.Address]map[common.Hash][]Indexer),
		addressAllTopics: make(map[common.Address][]Indexer),
	}
}

// RegisterIndexer registers a new indexer.
func (ic *IndexerCoordinator) RegisterIndexer(indexer Indexer) {
	ic.mu.Lock()
	defer ic.mu.Unlock()

	addressTopics := indexer.EventsToIndex()
	for addr, topics := range addressTopics {
		if len(topics) == 0 {
			// Empty topic set means indexer wants ALL events from this address
			ic.addressAllTopics[addr] = append(ic.addressAllTopics[addr], indexer)
		} else {
			// Specific topics - build routing map
			if _, exists := ic.addressTopics[addr]; !exists {
				ic.addressTopics[addr] = make(map[common.Hash][]Indexer)
			}
			for topic := range topics {
				ic.addressTopics[addr][topic] = append(ic.addressTopics[addr][topic], indexer)
			}
		}
	}

	ic.indexers = append(ic.indexers, indexer)
}

// HandleLogs processes a batch of logs and routes them to the appropriate indexers.
// Each log is sent to indexers that registered interest in both its address AND topic.
func (ic *IndexerCoordinator) HandleLogs(logs []types.Log) error {
	ic.mu.RLock()
	defer ic.mu.RUnlock()

	// Group logs by indexer to avoid duplicate processing
	indexerLogs := make(map[Indexer][]types.Log)

	for _, log := range logs {
		// Collect all indexers interested in this log
		interestedIndexers := make(map[Indexer]struct{})

		// Check indexers that want ALL topics from this address
		if indexers, exists := ic.addressAllTopics[log.Address]; exists {
			for _, idx := range indexers {
				interestedIndexers[idx] = struct{}{}
			}
		}

		// Check indexers that want specific topics from this address
		if len(log.Topics) > 0 {
			if topicsToIndexer, addrExists := ic.addressTopics[log.Address]; addrExists {
				eventTopic := log.Topics[0] // first topic is the event signature
				if indexers, topicExists := topicsToIndexer[eventTopic]; topicExists {
					for _, idx := range indexers {
						interestedIndexers[idx] = struct{}{}
					}
				}
			}
		}

		// Add this log to all interested indexers
		for idx := range interestedIndexers {
			indexerLogs[idx] = append(indexerLogs[idx], log)
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
