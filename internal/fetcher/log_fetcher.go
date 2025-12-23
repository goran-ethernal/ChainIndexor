package fetcher

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/goran-ethernal/ChainIndexor/internal/logger"
	irpc "github.com/goran-ethernal/ChainIndexor/internal/rpc"
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
	Addresses []ethcommon.Address

	// Topics are the event topic filters
	Topics [][]ethcommon.Hash

	// AddressStartBlocks maps each address to its minimum start block
	AddressStartBlocks map[ethcommon.Address]uint64
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
		log:           log,
		mode:          fetcher.ModeBackfill,
	}
}

// SetMode changes the fetcher's operating mode.
func (lf *LogFetcher) SetMode(mode fetcher.FetchMode) {
	lf.log.Infof("switching fetch mode from %v to %v", lf.mode, mode)
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
	addresses []ethcommon.Address,
	topics [][]ethcommon.Hash,
) (*fetcher.FetchResult, error) {
	lf.log.Debugf("fetching range from %d to %d in mode %v",
		fromBlock, toBlock, lf.mode,
	)

	// Build dynamic filter with only addresses that have reached their start block
	activeAddresses := make([]ethcommon.Address, 0, len(addresses))
	activeTopics := make([][]ethcommon.Hash, 0, len(topics))

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

	var (
		logs           []types.Log
		newFrom, newTo = fromBlock, toBlock
		err            error
	)

	// Only fetch logs if we have active addresses
	if len(activeAddresses) > 0 {
		// Fetch logs with automatic retry on "too many results" error
		logs, newFrom, newTo, err = lf.fetchLogsWithRetry(ctx, fromBlock, toBlock, activeAddresses, activeTopics)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch logs: %w", err)
		}

		lf.log.Debugf("fetched logs from %d to %d with %d active addresses (total %d addresses), logs count: %d",
			fromBlock,
			toBlock,
			len(activeAddresses),
			len(lf.cfg.Addresses),
			len(logs),
		)
	} else {
		// No active addresses yet - return empty logs
		logs = []types.Log{}
		lf.log.Debugf("skipped log fetch - no active addresses yet",
			fromBlock,
			toBlock,
			len(lf.cfg.Addresses),
		)
	}

	// Store fetched logs
	if err := lf.logStore.StoreLogs(ctx,
		activeAddresses, activeTopics, logs,
		fromBlock, toBlock); err != nil {
		return nil, fmt.Errorf("failed to store logs: %w", err)
	}

	// Verify consistency and record blocks
	// The reorg detector will verify headers and detect any reorgs
	headers, err := lf.reorgDetector.VerifyAndRecordBlocks(ctx, logs, fromBlock, toBlock)
	if err != nil {
		// If reorg detected, invalidate cache
		var reorgErr *reorg.ReorgDetectedError
		if errors.As(err, &reorgErr) {
			lf.log.Warnf("reorg detected, invalidating cache from block %d",
				reorgErr.FirstReorgBlock,
			)
			if storeErr := lf.logStore.HandleReorg(ctx, reorgErr.FirstReorgBlock); storeErr != nil {
				lf.log.Errorf("failed to handle reorg in log store: %v",
					storeErr,
				)
			}
		}
		return nil, fmt.Errorf("reorg detected: %w", err)
	}

	lf.log.Infof("fetched range from %d to %d with %d logs",
		fromBlock,
		toBlock,
		len(logs),
	)

	return &fetcher.FetchResult{
		Logs:      logs,
		Headers:   headers,
		FromBlock: newFrom,
		ToBlock:   newTo,
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

	if !nonSyncedLogs.IsEmpty() && nonSyncedLogs.ShouldCatchUp(lastIndexedBlock, downloaderStartBlock) {
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

	finalizedBlockNum := finalizedBlock.Number.Uint64()
	fromBlock := lastIndexedBlock + 1
	toBlock := min(fromBlock+lf.cfg.ChunkSize-1, finalizedBlockNum)

	// Check if we've caught up
	if fromBlock >= finalizedBlockNum {
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
	finalizedBlockNum := finalizedBlock.Number.Uint64()

	// If we're caught up, wait for new blocks
	if fromBlock > finalizedBlockNum {
		lf.log.Debugf("waiting for new blocks, last indexed: %d, finalized: %d",
			lastIndexedBlock,
			finalizedBlockNum,
		)

		// Wait for a short period before checking again
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(ethereumBlockTime):
			return lf.fetchLive(ctx, lastIndexedBlock)
		}
	}

	toBlock := finalizedBlockNum

	// In live mode, we still chunk to avoid huge fetches if we fall behind
	if toBlock-fromBlock+1 > lf.cfg.ChunkSize {
		toBlock = fromBlock + lf.cfg.ChunkSize - 1
	}

	return lf.FetchRange(ctx, fromBlock, toBlock)
}

// getFinalizedBlock gets the block number considered finalized based on config.
func (lf *LogFetcher) getFinalizedBlock(ctx context.Context) (*types.Header, error) {
	var (
		header *types.Header
		err    error
	)

	switch lf.cfg.Finality {
	case itypes.FinalityFinalized:
		header, err = lf.rpc.GetFinalizedBlockHeader(ctx)
	case itypes.FinalitySafe:
		header, err = lf.rpc.GetSafeBlockHeader(ctx)
	case itypes.FinalityLatest:
		header, err = lf.rpc.GetLatestBlockHeader(ctx)
		headerNum := header.Number.Uint64()
		if err == nil && lf.cfg.FinalizedLag > 0 && headerNum >= lf.cfg.FinalizedLag {
			// Apply lag to latest block
			header, err = lf.rpc.GetBlockHeader(ctx, headerNum-lf.cfg.FinalizedLag)
		} else {
			// If lag is zero or latest block number is less than lag, return genesis block
			header, err = lf.rpc.GetBlockHeader(ctx, 0)
		}
	default:
		return nil, fmt.Errorf("invalid finality mode: %s", lf.cfg.Finality)
	}

	if err != nil {
		return nil, err
	}

	return header, nil
}

// fetchLogsWithRetry fetches logs and automatically retries with a smaller range if too many results are returned.
// This function recursively splits the block range until a successful query is achieved.
func (lf *LogFetcher) fetchLogsWithRetry(
	ctx context.Context,
	fromBlock, toBlock uint64,
	addresses []ethcommon.Address,
	topics [][]ethcommon.Hash,
) ([]types.Log, uint64, uint64, error) {
	query := ethereum.FilterQuery{
		FromBlock: big.NewInt(int64(fromBlock)),
		ToBlock:   big.NewInt(int64(toBlock)),
		Addresses: addresses,
		Topics:    topics,
	}

	logs, err := lf.rpc.GetLogs(ctx, query)
	if err != nil {
		// Check if this is a "too many results" error
		ok, errData := irpc.IsTooManyResultsError(err)
		if !ok {
			return nil, 0, 0, err
		}

		// Try to parse suggested block range from error message
		var newFrom, newTo uint64
		if suggestedFrom, suggestedTo, ok := irpc.ParseSuggestedBlockRange(errData); ok {
			lf.log.Infof("too many logs, retrying with suggested block range from %d to %d (original range %d to %d)",
				suggestedFrom,
				suggestedTo,
				fromBlock,
				toBlock,
			)
			newFrom, newTo = suggestedFrom, suggestedTo
		} else {
			// No suggested range, split in half
			const splitBy = 2
			mid := (fromBlock + toBlock) / splitBy

			lf.log.Infof("too many logs, retrying with smaller block range (by splitting in half) from %d to %d"+
				"(original range %d to %d)",
				fromBlock,
				mid,
				fromBlock,
				toBlock,
			)

			if mid == fromBlock {
				// Can't split further (single block)
				return nil, 0, 0, fmt.Errorf("cannot split range further, single block %d has too many logs", fromBlock)
			}

			newFrom, newTo = fromBlock, mid
		}

		return lf.fetchLogsWithRetry(ctx, newFrom, newTo, addresses, topics)
	}

	return logs, fromBlock, toBlock, nil
}
