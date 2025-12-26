package rpc

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/goran-ethernal/ChainIndexor/pkg/config"
	pkgrpc "github.com/goran-ethernal/ChainIndexor/pkg/rpc"
)

// Compile-time check to ensure Client implements pkgrpc.EthClient interface.
var _ pkgrpc.EthClient = (*Client)(nil)

// Client wraps the Ethereum RPC client with convenience methods for indexing.
// It implements the pkgrpc.EthClient interface.
type Client struct {
	eth         *ethclient.Client
	rpc         *rpc.Client
	retryConfig *config.RetryConfig
}

// NewClient creates a new RPC client connected to the given endpoint.
func NewClient(ctx context.Context, endpoint string, retryConfig *config.RetryConfig) (*Client, error) {
	rpcClient, err := rpc.DialContext(ctx, endpoint)
	if err != nil {
		return nil, err
	}

	return &Client{
		eth:         ethclient.NewClient(rpcClient),
		rpc:         rpcClient,
		retryConfig: retryConfig,
	}, nil
}

// Close closes the RPC client connection.
func (c *Client) Close() {
	c.eth.Close()
}

// GetLogs retrieves logs matching the given filter query.
func (c *Client) GetLogs(ctx context.Context, query ethereum.FilterQuery) ([]types.Log, error) {
	start := time.Now()
	RPCMethodInc("eth_getLogs")
	defer func() {
		RPCMethodDuration("eth_getLogs", time.Since(start))
	}()

	var logs []types.Log
	err := retryWithBackoff(ctx, c.retryConfig, "eth_getLogs", func() error {
		var fetchErr error
		logs, fetchErr = c.eth.FilterLogs(ctx, query)
		return fetchErr
	})

	if err != nil {
		RPCMethodError("eth_getLogs", "error")
		return nil, err
	}

	return logs, nil
}

// GetBlockHeader retrieves the header for a specific block number.
func (c *Client) GetBlockHeader(ctx context.Context, blockNum uint64) (*types.Header, error) {
	start := time.Now()
	RPCMethodInc("eth_getBlockByNumber")
	defer func() {
		RPCMethodDuration("eth_getBlockByNumber", time.Since(start))
	}()

	var header *types.Header
	err := retryWithBackoff(ctx, c.retryConfig, "eth_getBlockByNumber", func() error {
		var fetchErr error
		header, fetchErr = c.eth.HeaderByNumber(ctx, big.NewInt(int64(blockNum)))
		return fetchErr
	})

	if err != nil {
		RPCMethodError("eth_getBlockByNumber", "error")
		return nil, err
	}

	return header, nil
}

// GetLatestBlockHeader retrieves the latest block header.
func (c *Client) GetLatestBlockHeader(ctx context.Context) (*types.Header, error) {
	start := time.Now()
	RPCMethodInc("eth_getBlockByNumber")
	defer func() {
		RPCMethodDuration("eth_getBlockByNumber", time.Since(start))
	}()

	var header *types.Header
	err := retryWithBackoff(ctx, c.retryConfig, "eth_getBlockByNumber", func() error {
		var fetchErr error
		header, fetchErr = c.eth.HeaderByNumber(ctx, nil)
		return fetchErr
	})

	if err != nil {
		RPCMethodError("eth_getBlockByNumber", "error")
		return nil, err
	}

	return header, nil
}

// GetFinalizedBlockHeader retrieves the finalized block header.
func (c *Client) GetFinalizedBlockHeader(ctx context.Context) (*types.Header, error) {
	start := time.Now()
	RPCMethodInc("eth_getBlockByNumber")
	defer func() {
		RPCMethodDuration("eth_getBlockByNumber", time.Since(start))
	}()

	var header *types.Header
	err := retryWithBackoff(ctx, c.retryConfig, "eth_getBlockByNumber", func() error {
		var fetchErr error
		header, fetchErr = c.eth.HeaderByNumber(ctx, big.NewInt(int64(rpc.FinalizedBlockNumber)))
		return fetchErr
	})

	if err != nil {
		RPCMethodError("eth_getBlockByNumber", "error")
		return nil, err
	}

	return header, nil
}

// GetSafeBlockHeader retrieves the safe block header.
func (c *Client) GetSafeBlockHeader(ctx context.Context) (*types.Header, error) {
	start := time.Now()
	RPCMethodInc("eth_getBlockByNumber")
	defer func() {
		RPCMethodDuration("eth_getBlockByNumber", time.Since(start))
	}()

	var header *types.Header
	err := retryWithBackoff(ctx, c.retryConfig, "eth_getBlockByNumber", func() error {
		var fetchErr error
		header, fetchErr = c.eth.HeaderByNumber(ctx, big.NewInt(int64(rpc.SafeBlockNumber)))
		return fetchErr
	})

	if err != nil {
		RPCMethodError("eth_getBlockByNumber", "error")
		return nil, err
	}

	return header, nil
}

// BatchGetLogs retrieves logs for multiple filter queries in a single batch call.
func (c *Client) BatchGetLogs(ctx context.Context, queries []ethereum.FilterQuery) ([][]types.Log, error) {
	start := time.Now()
	RPCMethodInc("eth_getLogs_batch")
	defer func() {
		RPCMethodDuration("eth_getLogs_batch", time.Since(start))
	}()

	var results [][]types.Log
	err := retryWithBackoff(ctx, c.retryConfig, "eth_getLogs_batch", func() error {
		batch := make([]rpc.BatchElem, len(queries))
		results = make([][]types.Log, len(queries))

		for i, query := range queries {
			batch[i] = rpc.BatchElem{
				Method: "eth_getLogs",
				Args:   []any{toFilterArg(query)},
				Result: &results[i],
			}
		}

		if err := c.rpc.BatchCallContext(ctx, batch); err != nil {
			return err
		}

		// Check for individual errors
		for _, elem := range batch {
			if elem.Error != nil {
				return elem.Error
			}
		}

		return nil
	})

	if err != nil {
		RPCMethodError("eth_getLogs_batch", "error")
		return nil, err
	}

	return results, nil
}

// BatchGetBlockHeaders retrieves headers for multiple block numbers in a single batch call.
func (c *Client) BatchGetBlockHeaders(ctx context.Context, blockNums []uint64) ([]*types.Header, error) {
	const maxBatch = 100
	var allResults []*types.Header

	start := time.Now()
	RPCMethodInc("eth_getBlockByNumber_batch")
	defer func() {
		RPCMethodDuration("eth_getBlockByNumber_batch", time.Since(start))
	}()

	for i := 0; i < len(blockNums); i += maxBatch {
		end := min(i+maxBatch, len(blockNums))
		chunk := blockNums[i:end]

		var chunkResults []*types.Header
		err := retryWithBackoff(ctx, c.retryConfig, "eth_getBlockByNumber_batch", func() error {
			batch := make([]rpc.BatchElem, len(chunk))
			chunkResults = make([]*types.Header, len(chunk))

			for j, blockNum := range chunk {
				batch[j] = rpc.BatchElem{
					Method: "eth_getBlockByNumber",
					Args:   []any{toBlockNumArg(blockNum), false}, // false = don't include transactions
					Result: &chunkResults[j],
				}
			}

			if err := c.rpc.BatchCallContext(ctx, batch); err != nil {
				return err
			}

			// Check for individual errors
			for _, elem := range batch {
				if elem.Error != nil {
					return elem.Error
				}
			}

			return nil
		})

		if err != nil {
			RPCMethodError("eth_getBlockByNumber_batch", "error")
			return nil, err
		}

		allResults = append(allResults, chunkResults...)
	}

	return allResults, nil
}

// toFilterArg converts ethereum.FilterQuery to the format expected by eth_getLogs.
func toFilterArg(q ethereum.FilterQuery) any {
	arg := map[string]any{
		"topics": q.Topics,
	}

	if q.BlockHash != nil {
		arg["blockHash"] = *q.BlockHash
	} else {
		if q.FromBlock != nil {
			arg["fromBlock"] = toBlockNumArg(q.FromBlock.Uint64())
		}
		if q.ToBlock != nil {
			arg["toBlock"] = toBlockNumArg(q.ToBlock.Uint64())
		}
	}

	if len(q.Addresses) > 0 {
		if len(q.Addresses) == 1 {
			arg["address"] = q.Addresses[0]
		} else {
			arg["address"] = q.Addresses
		}
	}

	return arg
}

// toBlockNumArg converts a block number to hex format.
func toBlockNumArg(blockNum uint64) string {
	return fmt.Sprintf("0x%x", blockNum)
}
