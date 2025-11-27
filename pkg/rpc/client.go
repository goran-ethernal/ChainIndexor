package rpc

import (
	"context"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/core/types"
)

// EthClient defines the interface for Ethereum RPC operations.
// This abstraction allows for easier testing and alternative implementations.
type EthClient interface {
	// Close closes the RPC client connection.
	Close()

	// GetLogs retrieves logs matching the given filter query.
	GetLogs(ctx context.Context, query ethereum.FilterQuery) ([]types.Log, error)

	// GetBlockHeader retrieves the header for a specific block number.
	GetBlockHeader(ctx context.Context, blockNum uint64) (*types.Header, error)

	// GetLatestBlockHeader retrieves the latest block header.
	GetLatestBlockHeader(ctx context.Context) (*types.Header, error)

	// GetFinalizedBlockHeader retrieves the finalized block header.
	GetFinalizedBlockHeader(ctx context.Context) (*types.Header, error)

	// GetSafeBlockHeader retrieves the safe block header.
	GetSafeBlockHeader(ctx context.Context) (*types.Header, error)

	// BatchGetLogs retrieves logs for multiple filter queries in a single batch call.
	BatchGetLogs(ctx context.Context, queries []ethereum.FilterQuery) ([][]types.Log, error)

	// BatchGetBlockHeaders retrieves headers for multiple block numbers in a single batch call.
	BatchGetBlockHeaders(ctx context.Context, blockNums []uint64) ([]*types.Header, error)
}
