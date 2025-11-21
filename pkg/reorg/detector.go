package reorg

import (
	"database/sql"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/goran-ethernal/ChainIndexor/internal/db"
	"github.com/goran-ethernal/ChainIndexor/internal/logger"
	"github.com/goran-ethernal/ChainIndexor/pkg/reorg/migrations"
)

// ReorgDetector detects blockchain reorganizations by tracking block hashes.
type ReorgDetector struct {
	db  *sql.DB
	log *logger.Logger
}

// NewReorgDetector creates a new ReorgDetector with the given database connection.
func NewReorgDetector(dbPath string, log *logger.Logger) (*ReorgDetector, error) {
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
		log: log.WithComponent("reorg-detector"),
	}

	detector.log.Info("reorg detector initialized")

	return detector, nil
}

// VerifyAndRecordBlocks checks that logs and headers are from the same chain state
// and records the block hashes if verification passes.
// Returns an error if a reorg is detected during the fetch process.
func (r *ReorgDetector) VerifyAndRecordBlocks(logs []types.Log, headers []*types.Header) error {
	r.log.Debugw("verifying and recording blocks",
		"num_logs", len(logs),
		"num_headers", len(headers),
	)

	// 1. Build map of block hashes from logs
	logBlockHashes := make(map[uint64]common.Hash)
	for _, log := range logs {
		logBlockHashes[log.BlockNumber] = log.BlockHash
	}

	// 2. Build map of block hashes from headers
	headerBlockHashes := make(map[uint64]common.Hash)
	for _, header := range headers {
		headerBlockHashes[header.Number.Uint64()] = header.Hash()
	}

	// 3. Verify consistency for blocks that appear in both logs and headers
	for blockNum, logHash := range logBlockHashes {
		if headerHash, exists := headerBlockHashes[blockNum]; exists {
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

	// 4. Verify chain continuity (parent hashes form a valid chain)
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
				return fmt.Errorf("chain discontinuity detected between blocks %d and %d: expected_parent=%s actual_parent=%s",
					headers[i-1].Number.Uint64(),
					headers[i].Number.Uint64(),
					expectedParent.Hex(),
					actualParent.Hex())
			}
		}
	}

	// 5. All checks passed - safe to record blocks
	return r.recordBlocks(headers)
}

// recordBlocks persists block hashes to the database.
func (r *ReorgDetector) recordBlocks(headers []*types.Header) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil {
			r.log.Errorw("failed to rollback transaction", "error", err)
		}
	}()

	stmt, err := tx.Prepare(`
		INSERT OR REPLACE INTO block_hashes (block_number, block_hash, parent_hash)
		VALUES (?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, header := range headers {
		_, err := stmt.Exec(
			header.Number.Uint64(),
			header.Hash().Hex(),
			header.ParentHash.Hex(),
		)
		if err != nil {
			return fmt.Errorf("failed to insert block %d: %w", header.Number.Uint64(), err)
		}
	}

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

// CheckForReorg checks if a reorg has occurred at the given block number
// by comparing the cached hash with the current hash from the chain.
func (r *ReorgDetector) CheckForReorg(blockNum uint64, currentHash common.Hash) (bool, error) {
	var cachedHash string
	err := r.db.QueryRow(
		"SELECT block_hash FROM block_hashes WHERE block_number = ?",
		blockNum,
	).Scan(&cachedHash)

	if err == sql.ErrNoRows {
		// No cached hash for this block - can't detect reorg
		return false, nil
	}

	if err != nil {
		return false, fmt.Errorf("failed to query cached hash: %w", err)
	}

	// Compare hashes
	if currentHash.Hex() != cachedHash {
		// Reorg detected!
		r.log.Warnw("reorg detected",
			"block", blockNum,
			"cached_hash", cachedHash,
			"current_hash", currentHash.Hex(),
		)
		return true, nil
	}

	return false, nil
}

// PruneOldBlocks removes block hashes older than the given block number.
// This is useful to prevent the database from growing indefinitely.
func (r *ReorgDetector) PruneOldBlocks(keepFromBlock uint64) error {
	result, err := r.db.Exec(
		"DELETE FROM block_hashes WHERE block_number < ?",
		keepFromBlock,
	)
	if err != nil {
		return fmt.Errorf("failed to prune old blocks: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected > 0 {
		r.log.Infow("pruned old block hashes",
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
