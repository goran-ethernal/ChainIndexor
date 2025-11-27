package reorg

import (
	"context"

	"github.com/ethereum/go-ethereum/core/types"
)

// Detector detects blockchain reorganizations by tracking block hashes.
type Detector interface {
	// VerifyAndRecordBlocks checks for reorgs and records blocks for the given range.
	// It follows these steps:
	// 1. Get the last finalized block and prune finalized blocks from DB
	// 2. Verify all non-finalized blocks in DB against current chain state
	// 3. Fetch headers for the new block range and verify consistency
	// 4. Record the new blocks to DB
	// All database operations are performed atomically within a single transaction.
	// Returns ErrReorgDetected if a reorg is detected.
	VerifyAndRecordBlocks(ctx context.Context, logs []types.Log, fromBlock, toBlock uint64) error

	// Close closes the detector and releases any resources.
	Close() error
}
