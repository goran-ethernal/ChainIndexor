package indexer

import (
	"fmt"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/goran-ethernal/ChainIndexor/internal/metrics"
	"github.com/goran-ethernal/ChainIndexor/pkg/indexer"
	"golang.org/x/sync/errgroup"
)

// IndexerCoordinator manages multiple indexers and routes events to them based on address and topics.
type IndexerCoordinator struct {
	mu sync.RWMutex

	// addressTopics maps address -> topic -> indexers for specific topic filters
	addressTopics map[common.Address]map[common.Hash][]indexer.Indexer

	// addressAllTopics maps address -> indexers that want ALL topics from that address
	addressAllTopics map[common.Address][]indexer.Indexer

	// indexers holds all registered indexers
	indexers []indexer.Indexer

	// startBlocks maps each indexer to its start block
	startBlocks map[indexer.Indexer]uint64
}

// NewIndexerCoordinator creates a new IndexerCoordinator.
func NewIndexerCoordinator() *IndexerCoordinator {
	return &IndexerCoordinator{
		indexers:         make([]indexer.Indexer, 0),
		addressTopics:    make(map[common.Address]map[common.Hash][]indexer.Indexer),
		addressAllTopics: make(map[common.Address][]indexer.Indexer),
		startBlocks:      make(map[indexer.Indexer]uint64),
	}
}

// RegisterIndexer registers a new indexer.
func (ic *IndexerCoordinator) RegisterIndexer(idx indexer.Indexer) {
	ic.mu.Lock()
	defer ic.mu.Unlock()

	// Store the indexer's start block
	ic.startBlocks[idx] = idx.StartBlock()

	addressTopics := idx.EventsToIndex()
	for addr, topics := range addressTopics {
		if len(topics) == 0 {
			// Empty topic set means indexer wants ALL events from this address
			ic.addressAllTopics[addr] = append(ic.addressAllTopics[addr], idx)
		} else {
			// Specific topics - build routing map
			if _, exists := ic.addressTopics[addr]; !exists {
				ic.addressTopics[addr] = make(map[common.Hash][]indexer.Indexer)
			}
			for topic := range topics {
				ic.addressTopics[addr][topic] = append(ic.addressTopics[addr][topic], idx)
			}
		}
	}

	ic.indexers = append(ic.indexers, idx)
}

// HandleLogs processes a batch of logs and routes them to the appropriate indexers.
// Each log is sent to indexers that registered interest in both its address AND topic.
func (ic *IndexerCoordinator) HandleLogs(logs []types.Log, from, to uint64) error {
	ic.mu.RLock()
	defer ic.mu.RUnlock()

	// Group logs by indexer to avoid duplicate processing
	indexerLogs := make(map[indexer.Indexer][]types.Log)

	for _, log := range logs {
		// Collect all indexers interested in this log
		interestedIndexers := make(map[indexer.Indexer]struct{})
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

	// Call HandleLogs for each indexer with their relevant logs concurrently
	var g errgroup.Group
	for idx, relevantLogs := range indexerLogs {
		// Capture loop variables
		indexer := idx
		indexerName := indexer.Name()
		logs := relevantLogs

		g.Go(func() error {
			start := time.Now()
			defer func() {
				metrics.BlockProcessingTimeLog(indexerName, time.Since(start))
			}()

			// Filter logs based on the indexer's start block
			startBlock := ic.startBlocks[indexer]
			filteredLogs := make([]types.Log, 0, len(logs))
			for _, log := range logs {
				if log.BlockNumber >= startBlock {
					filteredLogs = append(filteredLogs, log)
				}
			}

			// Only call HandleLogs if there are logs to process
			if len(filteredLogs) > 0 {
				if err := indexer.HandleLogs(filteredLogs); err != nil {
					return fmt.Errorf("indexer failed to handle logs: %w", err)
				}
			}

			logMetrics(indexerName, len(filteredLogs), start, from, to)

			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return err
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

// IndexerStartBlocks returns a slice of start blocks for all registered indexers.
func (ic *IndexerCoordinator) IndexerStartBlocks() []uint64 {
	ic.mu.RLock()
	defer ic.mu.RUnlock()

	startBlocks := make([]uint64, 0, len(ic.indexers))
	for _, idx := range ic.indexers {
		startBlocks = append(startBlocks, ic.startBlocks[idx])
	}
	return startBlocks
}

// logMetrics records metrics for the indexing operation.
func logMetrics(indexer string, numOfLogsIndexed int, processingStart time.Time, fromBlock, toBlock uint64) {
	blocksProcessed := toBlock - fromBlock + 1
	metrics.LogsIndexedInc(indexer, numOfLogsIndexed)
	metrics.BlocksProcessedInc(indexer, blocksProcessed)
	metrics.LastIndexedBlockInc(indexer, toBlock)

	elapsed := time.Since(processingStart).Seconds()
	if elapsed == 0 {
		elapsed = 1 // prevent division by zero
	}

	metrics.IndexingRateLog(indexer, float64(blocksProcessed)/elapsed)
}
