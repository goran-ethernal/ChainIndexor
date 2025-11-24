package reorg

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/goran-ethernal/ChainIndexor/internal/db"
	"github.com/goran-ethernal/ChainIndexor/internal/logger"
	"github.com/goran-ethernal/ChainIndexor/internal/rpc"
	"github.com/goran-ethernal/ChainIndexor/pkg/reorg/migrations"
	"github.com/russross/meddler"
)

// ReorgDetector detects blockchain reorganizations by tracking block hashes.
type ReorgDetector struct {
	db  *sql.DB
	rpc *rpc.Client
	log *logger.Logger
}

// NewReorgDetector creates a new ReorgDetector with the given database connection.
func NewReorgDetector(dbPath string, rpcClient *rpc.Client, log *logger.Logger) (*ReorgDetector, error) {
	// Run migrations to set up the database schema
	err := migrations.RunMigrations(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}
	database, err := db.NewSQLiteDB(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	detector := &ReorgDetector{
		db:  database,
		rpc: rpcClient,
		log: log.WithComponent("reorg-detector"),
	}

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
func (r *ReorgDetector) VerifyAndRecordBlocks(ctx context.Context, logs []types.Log, fromBlock, toBlock uint64) error {
	r.log.Debugw("verifying and recording blocks",
		"num_logs", len(logs),
		"from_block", fromBlock,
		"to_block", toBlock,
	)

	// Begin transaction for atomic operations
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
			r.log.Errorw("failed to rollback transaction", "error", err)
		}
	}()

	// Step 1: Get last finalized block and prune finalized blocks from DB
	finalizedHeader, err := r.rpc.GetFinalizedBlockHeader(ctx)
	if err != nil {
		return fmt.Errorf("failed to get finalized block header: %w", err)
	}
	finalizedBlockNum := finalizedHeader.Number.Uint64()

	// Check if we have the finalized block in our DB
	cachedFinalizedBlock, err := r.getStoredBlockTx(tx, finalizedBlockNum)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("failed to query finalized block hash: %w", err)
	}

	// if we have the finalized block and it matches, prune all blocks up to and including it
	if cachedFinalizedBlock.BlockHash == finalizedHeader.Hash() {
		if err := r.pruneOldBlocksTx(tx, finalizedBlockNum+1); err != nil {
			return fmt.Errorf("failed to prune finalized blocks: %w", err)
		}
		r.log.Debugw("pruned finalized blocks", "finalized_block", finalizedBlockNum)
	}

	// Step 2: Verify all non-finalized blocks in DB against current chain state
	nonFinalizedBlocks, err := r.getStoredBlocksAfterBlockTx(tx, finalizedBlockNum)
	if err != nil {
		return fmt.Errorf("failed to get non-finalized blocks: %w", err)
	}

	if len(nonFinalizedBlocks) > 0 {
		r.log.Debugw("verifying non-finalized blocks", "count", len(nonFinalizedBlocks))

		// Get block numbers to fetch
		blockNums := make([]uint64, len(nonFinalizedBlocks))
		for i, block := range nonFinalizedBlocks {
			blockNums[i] = block.BlockNumber
		}

		// Fetch current headers from RPC
		currentHeaders, err := r.rpc.BatchGetBlockHeaders(ctx, blockNums)
		if err != nil {
			return fmt.Errorf("failed to fetch non-finalized headers: %w", err)
		}

		// Verify hashes match
		for i, header := range currentHeaders {
			cachedHash := nonFinalizedBlocks[i].BlockHash
			currentHash := header.Hash()

			if cachedHash != currentHash {
				// REORG DETECTED!
				r.log.Warnw("reorg detected in non-finalized blocks",
					"block", header.Number.Uint64(),
					"cached_hash", cachedHash.Hex(),
					"current_hash", currentHash.Hex(),
				)
				return fmt.Errorf("reorg detected at block %d: cached_hash=%s current_hash=%s",
					header.Number.Uint64(), cachedHash.Hex(), currentHash.Hex())
			}
		}

		r.log.Debugw("non-finalized blocks verified", "count", len(nonFinalizedBlocks))
	}

	// Step 3: Fetch headers for the new block range
	blockNums := make([]uint64, 0, toBlock-fromBlock+1)
	for blockNum := fromBlock; blockNum <= toBlock; blockNum++ {
		blockNums = append(blockNums, blockNum)
	}

	headers, err := r.rpc.BatchGetBlockHeaders(ctx, blockNums)
	if err != nil {
		return fmt.Errorf("failed to fetch headers for range: %w", err)
	}

	// Step 3a: Build map of block hashes from logs
	logBlockHashes := make(map[uint64]common.Hash)
	for _, log := range logs {
		logBlockHashes[log.BlockNumber] = log.BlockHash
	}

	// Step 3b: Verify consistency between logs and headers
	for _, header := range headers {
		blockNum := header.Number.Uint64()
		headerHash := header.Hash()

		if logHash, exists := logBlockHashes[blockNum]; exists {
			if logHash != headerHash {
				// INCONSISTENCY! Reorg happened between the two RPC calls
				r.log.Warnw("reorg detected during fetch",
					"block", blockNum,
					"log_hash", logHash.Hex(),
					"header_hash", headerHash.Hex(),
				)
				return fmt.Errorf("reorg detected during fetch at block %d: log_hash=%s header_hash=%s",
					blockNum, logHash.Hex(), headerHash.Hex())
			}
		}
	}

	// Step 3c: Verify chain continuity (parent hashes form a valid chain)
	if len(headers) > 1 {
		for i := 1; i < len(headers); i++ {
			expectedParent := headers[i-1].Hash()
			actualParent := headers[i].ParentHash

			if actualParent != expectedParent {
				r.log.Warnw("chain discontinuity detected",
					"block", headers[i].Number.Uint64(),
					"prev_block", headers[i-1].Number.Uint64(),
					"expected_parent", expectedParent.Hex(),
					"actual_parent", actualParent.Hex(),
				)
				return fmt.Errorf("chain discontinuity detected between blocks %d and %d",
					headers[i-1].Number.Uint64(),
					headers[i].Number.Uint64())
			}
		}
	}

	// Step 4: All checks passed - safe to record blocks
	if err := r.recordBlocksTx(tx, headers); err != nil {
		return err
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	if len(headers) > 0 {
		r.log.Debugw("recorded block hashes",
			"from_block", headers[0].Number.Uint64(),
			"to_block", headers[len(headers)-1].Number.Uint64(),
			"count", len(headers),
		)
	}

	return nil
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

// getStoredBlocksAfterBlockTx retrieves all blocks from the DB that happened after the provided block using a transaction.
func (r *ReorgDetector) getStoredBlocksAfterBlockTx(tx *sql.Tx, finalizedBlockNum uint64) ([]StoredBlock, error) {
	var blocks []StoredBlock
	err := meddler.QueryAll(tx, &blocks, "SELECT * FROM block_hashes WHERE block_number > ? ORDER BY block_number ASC", finalizedBlockNum)
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
		r.log.Debugw("pruned old block hashes in transaction",
			"keep_from_block", keepFromBlock,
			"deleted_count", rowsAffected,
		)
	}

	return nil
}

// Close closes the database connection.
func (r *ReorgDetector) Close() error {
	return r.db.Close()
}
