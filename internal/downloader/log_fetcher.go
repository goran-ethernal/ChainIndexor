package downloader

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/goran-ethernal/ChainIndexor/internal/logger"
	"github.com/goran-ethernal/ChainIndexor/internal/rpc"
	itypes "github.com/goran-ethernal/ChainIndexor/internal/types"
	"github.com/goran-ethernal/ChainIndexor/pkg/reorg"
)

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
}

// LogFetcher handles fetching logs and block headers from the blockchain.
type LogFetcher struct {
	cfg           LogFetcherConfig
	rpc           *rpc.Client
	reorgDetector *reorg.ReorgDetector
	log           *logger.Logger
	mode          FetchMode
}

// NewLogFetcher creates a new LogFetcher instance.
func NewLogFetcher(cfg LogFetcherConfig, rpcClient *rpc.Client, reorgDetector *reorg.ReorgDetector, log *logger.Logger) *LogFetcher {
	return &LogFetcher{
		cfg:           cfg,
		rpc:           rpcClient,
		reorgDetector: reorgDetector,
		log:           log.WithComponent("log-fetcher"),
		mode:          ModeBackfill,
	}
}

// SetMode changes the fetcher's operating mode.
func (lf *LogFetcher) SetMode(mode FetchMode) {
	lf.log.Infow("switching fetch mode", "from", lf.mode, "to", mode)
	lf.mode = mode
}

// GetMode returns the current operating mode.
func (lf *LogFetcher) GetMode() FetchMode {
	return lf.mode
}

// FetchRange fetches logs and headers for a specific block range.
// It verifies consistency using the ReorgDetector and returns an error if a reorg is detected.
func (lf *LogFetcher) FetchRange(ctx context.Context, fromBlock, toBlock uint64) (*FetchResult, error) {
	lf.log.Debugw("fetching range",
		"from_block", fromBlock,
		"to_block", toBlock,
		"mode", lf.mode,
	)

	// Build the filter query
	query := ethereum.FilterQuery{
		FromBlock: big.NewInt(int64(fromBlock)),
		ToBlock:   big.NewInt(int64(toBlock)),
		Addresses: lf.cfg.Addresses,
		Topics:    lf.cfg.Topics,
	}

	// Fetch logs
	logs, err := lf.rpc.GetLogs(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch logs: %w", err)
	}

	// Verify consistency and record blocks
	// The reorg detector will fetch headers itself and verify everything
	if err := lf.reorgDetector.VerifyAndRecordBlocks(ctx, logs, fromBlock, toBlock); err != nil {
		return nil, fmt.Errorf("reorg detected: %w", err)
	}

	// Fetch block headers for the result (after reorg verification passed)
	blockNumbers := make([]uint64, 0, toBlock-fromBlock+1)
	for blockNum := fromBlock; blockNum <= toBlock; blockNum++ {
		blockNumbers = append(blockNumbers, blockNum)
	}

	headers, err := lf.rpc.BatchGetBlockHeaders(ctx, blockNumbers)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch headers: %w", err)
	}

	lf.log.Infow("fetched range",
		"from_block", fromBlock,
		"to_block", toBlock,
		"logs_count", len(logs),
		"blocks_count", len(headers),
	)

	return &FetchResult{
		Logs:      logs,
		Headers:   headers,
		FromBlock: fromBlock,
		ToBlock:   toBlock,
	}, nil
}

// FetchNext fetches the next chunk of logs based on the current mode.
// For backfill mode, it fetches from the given block up to chunk_size.
// For live mode, it fetches new blocks since the last checkpoint.
func (lf *LogFetcher) FetchNext(ctx context.Context, lastIndexedBlock uint64) (*FetchResult, error) {
	switch lf.mode {
	case ModeBackfill:
		return lf.fetchBackfill(ctx, lastIndexedBlock)
	case ModeLive:
		return lf.fetchLive(ctx, lastIndexedBlock)
	default:
		return nil, fmt.Errorf("unknown fetch mode: %s", lf.mode)
	}
}

// fetchBackfill fetches historical blocks in chunks.
func (lf *LogFetcher) fetchBackfill(ctx context.Context, lastIndexedBlock uint64) (*FetchResult, error) {
	// Get the current finalized block
	finalizedBlock, err := lf.getFinalizedBlock(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get finalized block: %w", err)
	}

	fromBlock := lastIndexedBlock + 1
	// Don't fetch beyond finalized block
	toBlock := min(fromBlock+lf.cfg.ChunkSize-1, finalizedBlock)

	// Check if we've caught up
	if fromBlock > finalizedBlock {
		lf.log.Info("backfill complete, switching to live mode")
		lf.mode = ModeLive
		return lf.fetchLive(ctx, lastIndexedBlock)
	}

	return lf.FetchRange(ctx, fromBlock, toBlock)
}

// fetchLive tails new blocks as they become finalized.
func (lf *LogFetcher) fetchLive(ctx context.Context, lastIndexedBlock uint64) (*FetchResult, error) {
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
		case <-time.After(12 * time.Second): // Ethereum block time
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
