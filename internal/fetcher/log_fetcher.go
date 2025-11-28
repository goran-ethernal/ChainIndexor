package fetcher

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/goran-ethernal/ChainIndexor/internal/logger"
	itypes "github.com/goran-ethernal/ChainIndexor/internal/types"
	"github.com/goran-ethernal/ChainIndexor/pkg/fetcher"
	"github.com/goran-ethernal/ChainIndexor/pkg/fetcher/store"
	"github.com/goran-ethernal/ChainIndexor/pkg/reorg"
	"github.com/goran-ethernal/ChainIndexor/pkg/rpc"
)

// Compile-time check to ensure LogFetcher implements fetcher.LogFetcher interface.
var _ fetcher.LogFetcher = (*LogFetcher)(nil)

const ethereumBlockTime = 12 * time.Second

// LogFetcherConfig contains configuration for the LogFetcher.
type LogFetcherConfig struct {
	// ChunkSize is the number of blocks to fetch per request
	ChunkSize uint64

	// Finality specifies the finality mode
	Finality itypes.BlockFinality

	// FinalizedLag is blocks behind head to consider finalized (only for "latest" mode)
	FinalizedLag uint64

	// Addresses are the contract addresses to filter
	Addresses []common.Address

	// Topics are the event topic filters
	Topics [][]common.Hash

	// AddressStartBlocks maps each address to its minimum start block
	AddressStartBlocks map[common.Address]uint64
}

// LogFetcher handles fetching logs and block headers from the blockchain.
type LogFetcher struct {
	cfg           LogFetcherConfig
	rpc           rpc.EthClient
	reorgDetector reorg.Detector
	logStore      store.LogStore
	log           *logger.Logger
	mode          fetcher.FetchMode
}

// NewLogFetcher creates a new LogFetcher instance.
func NewLogFetcher(
	cfg LogFetcherConfig,
	log *logger.Logger,
	rpcClient rpc.EthClient,
	reorgDetector reorg.Detector,
	logStore store.LogStore,
) *LogFetcher {
	return &LogFetcher{
		cfg:           cfg,
		rpc:           rpcClient,
		reorgDetector: reorgDetector,
		logStore:      logStore,
		log:           log.WithComponent("log-fetcher"),
		mode:          fetcher.ModeBackfill,
	}
}

// SetMode changes the fetcher's operating mode.
func (lf *LogFetcher) SetMode(mode fetcher.FetchMode) {
	lf.log.Infow("switching fetch mode", "from", lf.mode, "to", mode)
	lf.mode = mode
}

// GetMode returns the current operating mode.
func (lf *LogFetcher) GetMode() fetcher.FetchMode {
	return lf.mode
}

// FetchRange fetches logs and headers for a specific block range.
// It verifies consistency using the ReorgDetector and returns an error if a reorg is detected.
func (lf *LogFetcher) FetchRange(ctx context.Context, fromBlock, toBlock uint64) (*fetcher.FetchResult, error) {
	return lf.fetchRange(ctx, fromBlock, toBlock, lf.cfg.Addresses, lf.cfg.Topics)
}

func (lf *LogFetcher) fetchRange(
	ctx context.Context,
	fromBlock, toBlock uint64,
	addresses []common.Address,
	topics [][]common.Hash,
) (*fetcher.FetchResult, error) {
	lf.log.Debugw("fetching range",
		"from_block", fromBlock,
		"to_block", toBlock,
		"mode", lf.mode,
	)

	// Build dynamic filter with only addresses that have reached their start block
	activeAddresses := make([]common.Address, 0, len(addresses))
	activeTopics := make([][]common.Hash, 0, len(topics))

	for i, addr := range addresses {
		startBlock, exists := lf.cfg.AddressStartBlocks[addr]
		// Include address if:
		// 1. No start block is configured (shouldn't happen but be safe), OR
		// 2. We've reached or passed the start block
		if !exists || fromBlock >= startBlock {
			activeAddresses = append(activeAddresses, addr)
			activeTopics = append(activeTopics, lf.cfg.Topics[i])
		}
	}

	// Fetch block headers first (needed for reorg detection even if no logs)
	blockNumbers := make([]uint64, 0, toBlock-fromBlock+1)
	for blockNum := fromBlock; blockNum <= toBlock; blockNum++ {
		blockNumbers = append(blockNumbers, blockNum)
	}

	headers, err := lf.rpc.BatchGetBlockHeaders(ctx, blockNumbers)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch headers: %w", err)
	}

	var logs []types.Log

	// Only fetch logs if we have active addresses
	if len(activeAddresses) > 0 {
		// Build the filter query
		query := ethereum.FilterQuery{
			FromBlock: big.NewInt(int64(fromBlock)),
			ToBlock:   big.NewInt(int64(toBlock)),
			Addresses: activeAddresses,
			Topics:    activeTopics,
		}

		// Fetch logs
		logs, err = lf.rpc.GetLogs(ctx, query)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch logs: %w", err)
		}

		lf.log.Debugw("fetched logs",
			"from_block", fromBlock,
			"to_block", toBlock,
			"active_addresses", len(activeAddresses),
			"total_addresses", len(lf.cfg.Addresses),
			"logs_count", len(logs),
		)
	} else {
		// No active addresses yet - return empty logs
		logs = []types.Log{}
		lf.log.Debugw("skipped log fetch - no active addresses yet",
			"from_block", fromBlock,
			"to_block", toBlock,
			"total_addresses", len(lf.cfg.Addresses),
		)
	}

	// Verify consistency and record blocks
	// The reorg detector will verify headers and detect any reorgs
	if err := lf.reorgDetector.VerifyAndRecordBlocks(ctx, logs, fromBlock, toBlock); err != nil {
		// If reorg detected, invalidate cache
		var reorgErr *reorg.ReorgDetectedError
		if errors.As(err, &reorgErr) {
			lf.log.Warnw("reorg detected, invalidating cache",
				"from_block", reorgErr.FirstReorgBlock,
			)
			if storeErr := lf.logStore.HandleReorg(ctx, reorgErr.FirstReorgBlock); storeErr != nil {
				lf.log.Errorw("failed to handle reorg in log store",
					"error", storeErr,
				)
			}
		}
		return nil, fmt.Errorf("reorg detected: %w", err)
	}

	lf.log.Infow("fetched range",
		"from_block", fromBlock,
		"to_block", toBlock,
		"logs_count", len(logs),
		"blocks_count", len(headers),
	)

	return &fetcher.FetchResult{
		Logs:      logs,
		Headers:   headers,
		FromBlock: fromBlock,
		ToBlock:   toBlock,
	}, nil
}

// FetchNext fetches the next chunk of logs based on the current mode.
// For backfill mode, it fetches from the given block up to chunk_size.
// For live mode, it fetches new blocks since the last checkpoint.
func (lf *LogFetcher) FetchNext(
	ctx context.Context,
	lastIndexedBlock uint64,
	downloaderStartBlock uint64) (*fetcher.FetchResult, error) {
	switch lf.mode {
	case fetcher.ModeBackfill:
		return lf.fetchBackfill(ctx, lastIndexedBlock, downloaderStartBlock)
	case fetcher.ModeLive:
		return lf.fetchLive(ctx, lastIndexedBlock)
	default:
		return nil, fmt.Errorf("unknown fetch mode: %s", lf.mode)
	}
}

// fetchBackfill fetches historical blocks in chunks.
func (lf *LogFetcher) fetchBackfill(
	ctx context.Context,
	lastIndexedBlock uint64,
	downloaderStartBlock uint64,
) (*fetcher.FetchResult, error) {
	// check first if there are any unsynced logs
	// its the logs for indexers that just joined or want to backfill missed logs
	nonSyncedLogs, err := lf.logStore.GetUnsyncedTopics(ctx, lf.cfg.Addresses, lf.cfg.Topics, lastIndexedBlock)
	if err != nil {
		return nil, fmt.Errorf("failed to get unsynced topics: %w", err)
	}

	if !nonSyncedLogs.IsEmpty() {
		lf.log.Info("found unsynced logs, syncing them first")

		unsyncedAddresses, unsyncedTopics, lastCoveredBlock := nonSyncedLogs.GetAddressesAndTopics()
		// if we already synced past downloaderStartBlock, start from lastIndexedBlock+1
		fromBlock := max(downloaderStartBlock, lastCoveredBlock+1)
		toBlock := min(fromBlock+lf.cfg.ChunkSize-1, lastIndexedBlock) // Don't fetch beyond last indexed block
		return lf.fetchRange(
			ctx,
			fromBlock,
			toBlock,
			unsyncedAddresses,
			unsyncedTopics,
		)
	}

	// Get the current finalized block
	finalizedBlock, err := lf.getFinalizedBlock(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get finalized block: %w", err)
	}

	fromBlock := lastIndexedBlock + 1
	toBlock := min(fromBlock+lf.cfg.ChunkSize-1, finalizedBlock)

	// Check if we've caught up
	if fromBlock > finalizedBlock {
		lf.log.Info("backfill complete, switching to live mode")
		lf.mode = fetcher.ModeLive
		return lf.fetchLive(ctx, lastIndexedBlock)
	}

	return lf.FetchRange(ctx, fromBlock, toBlock)
}

// fetchLive tails new blocks as they become finalized.
func (lf *LogFetcher) fetchLive(ctx context.Context, lastIndexedBlock uint64) (*fetcher.FetchResult, error) {
	// Get the current finalized block
	finalizedBlock, err := lf.getFinalizedBlock(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get finalized block: %w", err)
	}

	fromBlock := lastIndexedBlock + 1

	// If we're caught up, wait for new blocks
	if fromBlock > finalizedBlock {
		lf.log.Debugw("waiting for new blocks",
			"last_indexed", lastIndexedBlock,
			"finalized", finalizedBlock,
		)

		// Wait for a short period before checking again
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(ethereumBlockTime):
			return lf.fetchLive(ctx, lastIndexedBlock)
		}
	}

	toBlock := finalizedBlock

	// In live mode, we still chunk to avoid huge fetches if we fall behind
	if toBlock-fromBlock+1 > lf.cfg.ChunkSize {
		toBlock = fromBlock + lf.cfg.ChunkSize - 1
	}

	return lf.FetchRange(ctx, fromBlock, toBlock)
}

// getFinalizedBlock gets the block number considered finalized based on config.
func (lf *LogFetcher) getFinalizedBlock(ctx context.Context) (uint64, error) {
	var header *types.Header
	var err error

	switch lf.cfg.Finality {
	case itypes.FinalityFinalized:
		header, err = lf.rpc.GetFinalizedBlockHeader(ctx)
	case itypes.FinalitySafe:
		header, err = lf.rpc.GetSafeBlockHeader(ctx)
	case itypes.FinalityLatest:
		header, err = lf.rpc.GetLatestBlockHeader(ctx)
		if err == nil && lf.cfg.FinalizedLag > 0 {
			// Apply lag to latest block
			finalizedNum := header.Number.Uint64()
			if finalizedNum > lf.cfg.FinalizedLag {
				finalizedNum -= lf.cfg.FinalizedLag
			} else {
				finalizedNum = 0
			}
			return finalizedNum, nil
		}
	default:
		return 0, fmt.Errorf("invalid finality mode: %s", lf.cfg.Finality)
	}

	if err != nil {
		return 0, err
	}

	return header.Number.Uint64(), nil
}
