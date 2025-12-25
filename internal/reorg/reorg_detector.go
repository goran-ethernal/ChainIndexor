package reorg

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	internalcommon "github.com/goran-ethernal/ChainIndexor/internal/common"
	"github.com/goran-ethernal/ChainIndexor/internal/db"
	"github.com/goran-ethernal/ChainIndexor/internal/logger"
	"github.com/goran-ethernal/ChainIndexor/internal/metrics"
	"github.com/goran-ethernal/ChainIndexor/pkg/reorg"
	"github.com/goran-ethernal/ChainIndexor/pkg/rpc"
	"github.com/russross/meddler"
)

var _ reorg.Detector = (*ReorgDetector)(nil)

// ReorgDetector detects blockchain reorganizations by tracking block hashes.
type ReorgDetector struct {
	db                     *sql.DB
	log                    *logger.Logger
	rpc                    rpc.EthClient
	maintenanceCoordinator db.Maintenance
}

// NewReorgDetector creates a new ReorgDetector with the given database configuration.
func NewReorgDetector(
	db *sql.DB,
	rpcClient rpc.EthClient,
	log *logger.Logger,
	maintenanceCoordinator db.Maintenance,
) (*ReorgDetector, error) {
	detector := &ReorgDetector{
		db:                     db,
		rpc:                    rpcClient,
		log:                    log,
		maintenanceCoordinator: maintenanceCoordinator,
	}

	// Initialize component health
	metrics.ComponentHealthSet(internalcommon.ComponentReorgDetector, true)

	detector.log.Info("reorg detector initialized")

	return detector, nil
}

// VerifyAndRecordBlocks checks for reorgs and records blocks for the given range.
// It follows these steps:
// 1. Get the last finalized block and prune finalized blocks from DB
// 2. Verify all non-finalized blocks in DB against current chain state
// 3. Fetch headers for the new block range and verify consistency
// 4. Record the new blocks to DB
// All database operations are performed atomically within a single transaction.
func (r *ReorgDetector) VerifyAndRecordBlocks(
	ctx context.Context,
	logs []types.Log, fromBlock, toBlock uint64) ([]*types.Header, error) {
	// Acquire operation lock if maintenance coordinator is available
	unlock := r.maintenanceCoordinator.AcquireOperationLock()
	defer unlock()

	r.log.Debugf("verifying and recording blocks: num_logs=%d from_block=%d to_block=%d",
		len(logs),
		fromBlock,
		toBlock,
	)

	// Begin transaction for atomic operations
	tx, err := r.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			r.log.Errorf("failed to rollback transaction: %v", err)
		}
	}()

	// Step 1: Get last finalized block and prune finalized blocks from DB
	finalizedHeader, err := r.rpc.GetFinalizedBlockHeader(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get finalized block header: %w", err)
	}
	finalizedBlockNum := finalizedHeader.Number.Uint64()

	// Check if we have the finalized block in our DB
	cachedFinalizedBlock, err := r.getStoredBlockTx(tx, finalizedBlockNum)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("failed to query finalized block hash: %w", err)
	}

	// if we have the finalized block and it matches, prune all blocks up to and including it
	if cachedFinalizedBlock.BlockHash == finalizedHeader.Hash() {
		if err := r.pruneOldBlocksTx(tx, finalizedBlockNum+1); err != nil {
			return nil, fmt.Errorf("failed to prune finalized blocks: %w", err)
		}
		r.log.Debugf("pruned finalized blocks: finalized_block=%d", finalizedBlockNum)
	}

	// Step 2: Verify all non-finalized blocks in DB against current chain state
	nonFinalizedBlocks, err := r.getStoredBlocksAfterBlockTx(tx, finalizedBlockNum)
	if err != nil {
		return nil, fmt.Errorf("failed to get non-finalized blocks: %w", err)
	}

	if len(nonFinalizedBlocks) > 0 {
		r.log.Debugf("verifying non-finalized blocks: count=%d", len(nonFinalizedBlocks))

		// Get block numbers to fetch
		blockNums := make([]uint64, len(nonFinalizedBlocks))
		for i, block := range nonFinalizedBlocks {
			blockNums[i] = block.BlockNumber
		}

		// Fetch current headers from RPC
		currentHeaders, err := r.rpc.BatchGetBlockHeaders(ctx, blockNums)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch non-finalized headers: %w", err)
		}

		// Verify hashes match
		for i, header := range currentHeaders {
			cachedHash := nonFinalizedBlocks[i].BlockHash
			currentHash := header.Hash()

			if cachedHash != currentHash {
				// REORG DETECTED!
				r.log.Warnf("reorg detected in non-finalized blocks: block=%d cached_hash=%s current_hash=%s",
					header.Number.Uint64(),
					cachedHash.Hex(),
					currentHash.Hex(),
				)
				ReorgDetectedLog(uint64(len(nonFinalizedBlocks)-i), header.Number.Uint64())
				return nil, reorg.NewReorgError(header.Number.Uint64(),
					fmt.Sprintf("cached_hash=%s current_hash=%s", cachedHash.Hex(), currentHash.Hex()))
			}
		}

		r.log.Debugf("non-finalized blocks verified: count=%d", len(nonFinalizedBlocks))
	}

	// Step 3: Fetch headers for the new block range
	blockNums := make([]uint64, 0, toBlock-fromBlock+1)
	for blockNum := fromBlock; blockNum <= toBlock; blockNum++ {
		if blockNum > finalizedBlockNum {
			blockNums = append(blockNums, blockNum)
		}
	}

	if len(blockNums) == 0 {
		// All blocks are finalized and already verified
		return nil, nil
	}

	headers, err := r.rpc.BatchGetBlockHeaders(ctx, blockNums)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch headers for range: %w", err)
	}

	// Step 3a: Build map of block hashes from logs that are in non finalized blocks
	logBlockHashes := make(map[uint64]common.Hash)
	for _, log := range logs {
		if log.BlockNumber > finalizedBlockNum {
			logBlockHashes[log.BlockNumber] = log.BlockHash
		}
	}

	// Step 3b: Verify consistency between logs and headers
	for i, header := range headers {
		blockNum := header.Number.Uint64()
		headerHash := header.Hash()

		if logHash, exists := logBlockHashes[blockNum]; exists {
			if logHash != headerHash {
				// INCONSISTENCY! Reorg happened between the two RPC calls
				r.log.Warnf("reorg detected during fetch: block=%d log_hash=%s header_hash=%s",
					blockNum,
					logHash.Hex(),
					headerHash.Hex(),
				)
				ReorgDetectedLog(uint64(len(headers)-i), blockNum)
				return nil, reorg.NewReorgError(blockNum,
					fmt.Sprintf("log_hash=%s header_hash=%s", logHash.Hex(), headerHash.Hex()))
			}
		}
	}

	// Step 3c: Verify chain continuity (parent hashes form a valid chain)
	if len(headers) > 1 {
		for i := 1; i < len(headers); i++ {
			expectedParent := headers[i-1].Hash()
			actualParent := headers[i].ParentHash

			if actualParent != expectedParent {
				r.log.Warnf("chain discontinuity detected: block=%d prev_block=%d expected_parent=%s actual_parent=%s",
					headers[i].Number.Uint64(),
					headers[i-1].Number.Uint64(),
					expectedParent.Hex(),
					actualParent.Hex(),
				)
				ReorgDetectedLog(uint64(len(headers)-i), headers[i].Number.Uint64())
				return nil, reorg.NewReorgError(headers[i].Number.Uint64(),
					fmt.Sprintf("chain discontinuity between blocks %d and %d",
						headers[i-1].Number.Uint64(), headers[i].Number.Uint64()))
			}
		}
	}

	// Step 4: All checks passed - safe to record blocks
	if err := r.recordBlocksTx(tx, headers); err != nil {
		return nil, err
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	if len(headers) > 0 {
		r.log.Debugf("recorded block hashes: from_block=%d to_block=%d count=%d",
			headers[0].Number.Uint64(),
			headers[len(headers)-1].Number.Uint64(),
			len(headers),
		)
	}

	return headers, nil
}

// StoredBlock represents a block stored in the database.
// Uses meddler tags for automatic struct-to-db mapping.
type StoredBlock struct {
	BlockNumber uint64      `meddler:"block_number"`
	BlockHash   common.Hash `meddler:"block_hash,hash"`
	ParentHash  common.Hash `meddler:"parent_hash,hash"`
}

// getStoredBlockTx retrieves the cached block for a specific block number using a transaction.
func (r *ReorgDetector) getStoredBlockTx(tx *sql.Tx, blockNum uint64) (StoredBlock, error) {
	var block StoredBlock
	err := meddler.QueryRow(tx, &block, "SELECT * FROM block_hashes WHERE block_number = ?", blockNum)
	if err != nil {
		return StoredBlock{}, err
	}
	return block, nil
}

// getStoredBlocksAfterBlockTx retrieves all blocks from the DB
// that happened after the provided block using a transaction.
func (r *ReorgDetector) getStoredBlocksAfterBlockTx(tx *sql.Tx, finalizedBlockNum uint64) ([]*StoredBlock, error) {
	var blocks []*StoredBlock
	err := meddler.QueryAll(tx, &blocks,
		"SELECT * FROM block_hashes WHERE block_number > ? ORDER BY block_number ASC",
		finalizedBlockNum)
	if err != nil {
		return nil, err
	}
	return blocks, nil
}

// recordBlocksTx persists block hashes to the database using a transaction.
func (r *ReorgDetector) recordBlocksTx(tx *sql.Tx, headers []*types.Header) error {
	for _, header := range headers {
		block := &StoredBlock{
			BlockNumber: header.Number.Uint64(),
			BlockHash:   header.Hash(),
			ParentHash:  header.ParentHash,
		}

		if err := meddler.Save(tx, "block_hashes", block); err != nil {
			return fmt.Errorf("failed to insert block %d: %w", header.Number.Uint64(), err)
		}
	}

	return nil
}

// pruneOldBlocksTx removes block hashes older than the given block number using a transaction.
func (r *ReorgDetector) pruneOldBlocksTx(tx *sql.Tx, keepFromBlock uint64) error {
	result, err := tx.Exec(
		"DELETE FROM block_hashes WHERE block_number < ?",
		keepFromBlock,
	)
	if err != nil {
		return fmt.Errorf("failed to prune old blocks: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected > 0 {
		r.log.Debugf("pruned old block hashes in transaction: keep_from_block=%d deleted_count=%d",
			keepFromBlock,
			rowsAffected,
		)
	}

	return nil
}

// Close closes the database connection.
func (r *ReorgDetector) Close() error {
	metrics.ComponentHealthSet(internalcommon.ComponentReorgDetector, false)
	return r.db.Close()
}
