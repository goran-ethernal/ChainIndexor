package downloader

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/goran-ethernal/ChainIndexor/internal/fetcher"
	"github.com/goran-ethernal/ChainIndexor/internal/fetcher/store"
	"github.com/goran-ethernal/ChainIndexor/internal/indexer"
	"github.com/goran-ethernal/ChainIndexor/internal/logger"
	"github.com/goran-ethernal/ChainIndexor/internal/reorg"
	"github.com/goran-ethernal/ChainIndexor/internal/rpc"
	"github.com/goran-ethernal/ChainIndexor/internal/types"
	"github.com/goran-ethernal/ChainIndexor/pkg/config"
	idx "github.com/goran-ethernal/ChainIndexor/pkg/indexer"
)

// Downloader orchestrates the log downloading process.
// It coordinates LogFetcher, SyncManager, and IndexerCoordinator to stream
// blockchain logs to registered indexers.
type Downloader struct {
	cfg           config.DownloaderConfig
	rpc           *rpc.Client
	reorgDetector *reorg.ReorgDetector
	syncManager   *SyncManager
	log           *logger.Logger
	coordinator   *indexer.IndexerCoordinator
	logFetcher    *fetcher.LogFetcher

	// Filter configuration built from registered indexers
	mu        sync.RWMutex
	addresses []common.Address
	topics    [][]common.Hash

	// Per-address start blocks (minimum across all indexers for that address)
	addressStartBlocks map[common.Address]uint64
}

// New creates a new Downloader instance.
func New(
	cfg config.DownloaderConfig,
	rpcClient *rpc.Client,
	reorgDetector *reorg.ReorgDetector,
	syncManager *SyncManager,
	log *logger.Logger,
) (*Downloader, error) {
	if rpcClient == nil {
		return nil, errors.New("RPC client is required")
	}
	if reorgDetector == nil {
		return nil, errors.New("ReorgDetector is required")
	}
	if syncManager == nil {
		return nil, errors.New("SyncManager is required")
	}
	if log == nil {
		return nil, errors.New("Logger is required")
	}

	d := &Downloader{
		cfg:                cfg,
		rpc:                rpcClient,
		reorgDetector:      reorgDetector,
		syncManager:        syncManager,
		log:                log.WithComponent("downloader"),
		coordinator:        indexer.NewIndexerCoordinator(),
		addresses:          make([]common.Address, 0),
		topics:             make([][]common.Hash, 0),
		addressStartBlocks: make(map[common.Address]uint64),
	}

	d.log.Info("downloader initialized")

	return d, nil
}

// RegisterIndexer registers an indexer to receive logs.
// The downloader will use the indexer's EventsToIndex method to determine
// which logs to fetch and forward.
func (d *Downloader) RegisterIndexer(idx idx.Indexer) {
	d.log.Infow("registering indexer", "indexer", fmt.Sprintf("%T", idx))

	// Get the events this indexer wants
	eventsToIndex := idx.EventsToIndex()

	// Get the start block for this indexer
	startBlock := idx.StartBlock()

	// Extract addresses and topics
	allTopics := make([]common.Hash, 0)

	d.mu.Lock()
	defer d.mu.Unlock()

	addressesIndex := make(map[common.Address]int, len(eventsToIndex))
	for addr, topicSet := range eventsToIndex {
		// Update the minimum start block for this address
		if existingStartBlock, exists := d.addressStartBlocks[addr]; !exists || startBlock < existingStartBlock {
			d.addressStartBlocks[addr] = startBlock
		}

		// Add address to filter (avoid duplicates)
		index := d.indexOfAddressLocked(addr)
		if index == -1 {
			// Address not found, add it to the downloader's addresses slice
			// Also initialize corresponding topics slice
			d.addresses = append(d.addresses, addr)
			d.topics = append(d.topics, make([]common.Hash, 0))
			index = len(d.addresses) - 1
		}
		addressesIndex[addr] = index

		// Get existing topics for this address
		addressTopics := make(map[common.Hash]struct{})
		for _, t := range d.topics[index] {
			addressTopics[t] = struct{}{}
		}

		// Add new topics from this indexer's topic set
		for topic := range topicSet {
			if _, exists := addressTopics[topic]; !exists {
				d.topics[index] = append(d.topics[index], topic)
				allTopics = append(allTopics, topic)
			}
		}
	}

	// Register with coordinator (outside of lock to avoid potential deadlock)
	d.coordinator.RegisterIndexer(idx)

	d.log.Infow("indexer registered",
		"indexer", fmt.Sprintf("%T", idx),
		"start_block", startBlock,
		"total_addresses", len(d.addresses),
		"total_topics", len(allTopics),
	)
}

func (d *Downloader) getDownloaderStartBlock() uint64 {
	d.mu.RLock()
	defer d.mu.RUnlock()

	// Determine the minimum start block from all registered indexers
	minStartBlock := uint64(0)
	indexerStartBlocks := d.coordinator.IndexerStartBlocks()
	if len(indexerStartBlocks) > 0 {
		minStartBlock = ^uint64(0) // Max uint64
		for _, startBlock := range indexerStartBlocks {
			if startBlock < minStartBlock {
				minStartBlock = startBlock
			}
		}
	}

	return minStartBlock
}

// Download starts the download process, streaming logs to registered indexers.
// It continues until the context is cancelled or an error occurs.
func (d *Downloader) Download(ctx context.Context) error {
	d.log.Info("starting download process")

	// Parse finality from config string
	finality, err := types.ParseBlockFinality(d.cfg.Finality)
	if err != nil {
		return fmt.Errorf("invalid finality configuration: %w", err)
	}

	// Initialize LogFetcher with filter configuration
	d.mu.RLock()

	addresses := make([]common.Address, len(d.addresses))
	copy(addresses, d.addresses)
	topics := make([][]common.Hash, len(d.topics))
	for i, topicSlice := range d.topics {
		topics[i] = make([]common.Hash, len(topicSlice))
		copy(topics[i], topicSlice)
	}
	// Copy addressStartBlocks map
	addressStartBlocks := make(map[common.Address]uint64, len(d.addressStartBlocks))
	maps.Copy(addressStartBlocks, d.addressStartBlocks)

	d.mu.RUnlock()

	// Create LogStore using the sync manager's database connection
	logStore := store.NewLogStore(d.syncManager.DB(), d.log)

	d.logFetcher = fetcher.NewLogFetcher(fetcher.LogFetcherConfig{
		ChunkSize:          d.cfg.ChunkSize,
		Finality:           finality,
		FinalizedLag:       d.cfg.FinalizedLag,
		Addresses:          addresses,
		Topics:             topics,
		AddressStartBlocks: addressStartBlocks,
	}, d.log, d.rpc, d.reorgDetector, logStore)

	// Get current sync state
	state, err := d.syncManager.GetState()
	if err != nil {
		return fmt.Errorf("failed to get sync state: %w", err)
	}

	// Initialize from saved state or start from the earliest indexer start block
	lastIndexedBlock := state.LastIndexedBlock
	downloaderStartBlock := d.getDownloaderStartBlock()
	if lastIndexedBlock == 0 {
		lastIndexedBlock = downloaderStartBlock - 1
		d.log.Infow("starting fresh download", "start_block", lastIndexedBlock)
	} else {
		d.log.Infow("resuming download", "last_indexed_block", lastIndexedBlock)
	}

	d.logFetcher.SetMode(fetcher.ModeBackfill) // Always start in backfill mode

	// Main download loop
	for {
		select {
		case <-ctx.Done():
			d.log.Info("download cancelled")
			return ctx.Err()
		default:
		}

		// Fetch next chunk
		result, err := d.logFetcher.FetchNext(ctx, lastIndexedBlock, downloaderStartBlock)
		if err != nil {
			// Check if this is a reorg error
			var reorgErr *reorg.ErrReorgDetected
			if errors.As(err, &reorgErr) {
				d.log.Warnw("reorg detected, initiating rollback",
					"block", reorgErr.FirstReorgBlock,
					"details", reorgErr.Details,
				)
				if err := d.handleReorg(reorgErr.FirstReorgBlock); err != nil {
					return fmt.Errorf("failed to handle reorg: %w", err)
				}
				// Continue from rolled-back position
				state, err := d.syncManager.GetState()
				if err != nil {
					return fmt.Errorf("failed to get state after reorg: %w", err)
				}
				lastIndexedBlock = state.LastIndexedBlock
				continue
			}

			// Not a reorg error, it's a real failure
			d.log.Errorw("failed to fetch logs", "error", err, "last_block", lastIndexedBlock)
			return fmt.Errorf("failed to fetch logs: %w", err)
		}

		// Route logs to indexers
		if len(result.Logs) > 0 {
			d.log.Debugw("processing logs",
				"count", len(result.Logs),
				"from_block", result.FromBlock,
				"to_block", result.ToBlock,
			)

			if err := d.coordinator.HandleLogs(result.Logs); err != nil {
				return fmt.Errorf("failed to handle logs: %w", err)
			}
		}

		// Save checkpoint with the last block's hash
		// Only update if we've progressed past the last saved block
		// We can receive blocks from already indexed ranges
		// due to new indexers being added with earlier start blocks
		if state.LastIndexedBlock <= result.FromBlock {
			lastHeader := result.Headers[len(result.Headers)-1]
			blockHash := lastHeader.Hash()

			if err := d.syncManager.SaveCheckpoint(
				result.ToBlock,
				blockHash,
				d.logFetcher.GetMode(),
			); err != nil {
				return fmt.Errorf("failed to save checkpoint: %w", err)
			}

			lastIndexedBlock = result.ToBlock

			d.log.Infow("checkpoint saved",
				"block", lastIndexedBlock,
				"block_hash", blockHash.Hex(),
				"mode", d.logFetcher.GetMode(),
				"logs_processed", len(result.Logs),
			)
		}

		// Get new state
		state, err = d.syncManager.GetState()
		if err != nil {
			return fmt.Errorf("failed to get sync state: %w", err)
		}
	}
}

// handleReorg handles a blockchain reorganization by rolling back indexers
// and adjusting the sync state.
func (d *Downloader) handleReorg(firstReorgBlock uint64) error {
	d.log.Warnw("handling reorg", "first_reorg_block", firstReorgBlock)

	// Notify all indexers to roll back
	if err := d.coordinator.HandleReorg(firstReorgBlock); err != nil {
		return fmt.Errorf("failed to notify indexers of reorg: %w", err)
	}

	// Reset sync state to rollback point
	rollbackTo := firstReorgBlock - 1
	if err := d.syncManager.Reset(rollbackTo); err != nil {
		return fmt.Errorf("failed to reset sync state: %w", err)
	}

	// Switch back to backfill mode to re-process the affected range
	if err := d.syncManager.SetMode(fetcher.ModeBackfill); err != nil {
		return fmt.Errorf("failed to set mode after reorg: %w", err)
	}

	d.logFetcher.SetMode(fetcher.ModeBackfill)

	d.log.Infow("reorg handled, resuming from safe block", "block", rollbackTo)

	return nil
}

// Close closes the downloader and releases resources.
func (d *Downloader) Close() error {
	d.log.Info("closing downloader")

	if d.syncManager != nil {
		if err := d.syncManager.Close(); err != nil {
			d.log.Errorw("failed to close sync manager", "error", err)
		}
	}

	if d.reorgDetector != nil {
		if err := d.reorgDetector.Close(); err != nil {
			d.log.Errorw("failed to close reorg detector", "error", err)
		}
	}

	if d.rpc != nil {
		d.rpc.Close()
	}

	return nil
}

// indexOfAddressLocked returns the index of an address in the downloader's addresses slice.
// Must be called with d.mu held (either read or write lock).
func (d *Downloader) indexOfAddressLocked(addr common.Address) int {
	for index, a := range d.addresses {
		if a == addr {
			return index
		}
	}
	return -1
}
