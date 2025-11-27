package reorg

import (
	"context"
	"database/sql"
	"errors"
	"math/big"
	"os"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/goran-ethernal/ChainIndexor/internal/logger"
	"github.com/goran-ethernal/ChainIndexor/internal/rpc/mocks"
	"github.com/stretchr/testify/require"
)

func setupTestReorgDetector(t *testing.T) (*ReorgDetector, *mocks.EthClient, func()) {
	t.Helper()

	// Create temporary database
	tmpFile, err := os.CreateTemp("", "reorg_test_*.db")
	require.NoError(t, err)
	tmpFile.Close()

	dbPath := tmpFile.Name()

	// Create mock RPC client
	mockRPC := mocks.NewEthClient(t)

	// Create reorg detector
	log, err := logger.NewLogger("error", true)
	require.NoError(t, err)

	detector, err := NewReorgDetector(dbPath, mockRPC, log)
	require.NoError(t, err)

	cleanup := func() {
		detector.Close()
		os.Remove(dbPath)
	}

	return detector, mockRPC, cleanup
}

func createTestHeader(blockNum uint64, parentHash common.Hash) *types.Header {
	header := &types.Header{
		Number:     big.NewInt(int64(blockNum)),
		ParentHash: parentHash,
		Difficulty: big.NewInt(1),
		GasLimit:   8000000,
		GasUsed:    0,
		Time:       1000000 + blockNum,
	}
	return header
}

func TestReorgDetector_NewReorgDetector(t *testing.T) {
	detector, _, cleanup := setupTestReorgDetector(t)
	defer cleanup()

	require.NotNil(t, detector)
	require.NotNil(t, detector.db)
	require.NotNil(t, detector.rpc)
	require.NotNil(t, detector.log)
}

func TestReorgDetector_VerifyAndRecordBlocks_FirstTime(t *testing.T) {
	detector, mockRPC, cleanup := setupTestReorgDetector(t)
	defer cleanup()

	ctx := context.Background()

	// Create test headers with proper chain continuity
	header100 := createTestHeader(100, common.HexToHash("0x99"))
	header101 := createTestHeader(101, header100.Hash())
	header102 := createTestHeader(102, header101.Hash())

	finalizedHeader := createTestHeader(50, common.HexToHash("0x49"))

	// Mock RPC calls
	mockRPC.EXPECT().GetFinalizedBlockHeader(ctx).Return(finalizedHeader, nil)
	mockRPC.EXPECT().BatchGetBlockHeaders(ctx, []uint64{100, 101, 102}).
		Return([]*types.Header{header100, header101, header102}, nil)

	// Create test logs
	logs := []types.Log{
		{
			BlockNumber: 100,
			BlockHash:   header100.Hash(),
		},
		{
			BlockNumber: 101,
			BlockHash:   header101.Hash(),
		},
		{
			BlockNumber: 102,
			BlockHash:   header102.Hash(),
		},
	}

	err := detector.VerifyAndRecordBlocks(ctx, logs, 100, 102)
	require.NoError(t, err)

	// Verify blocks were recorded
	tx, err := detector.db.Begin()
	require.NoError(t, err)
	defer func() {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			t.Errorf("failed to rollback transaction: %v", err)
		}
	}()

	block, err := detector.getStoredBlockTx(tx, 100)
	require.NoError(t, err)
	require.Equal(t, header100.Hash(), block.BlockHash)
	require.Equal(t, uint64(100), block.BlockNumber)
}

func TestReorgDetector_VerifyAndRecordBlocks_WithNonFinalizedBlocks(t *testing.T) {
	detector, mockRPC, cleanup := setupTestReorgDetector(t)
	defer cleanup()

	ctx := context.Background()

	// First, record some initial blocks
	header100 := createTestHeader(100, common.HexToHash("0x99"))
	header101 := createTestHeader(101, header100.Hash())
	finalizedHeader := createTestHeader(50, common.HexToHash("0x49"))

	mockRPC.EXPECT().GetFinalizedBlockHeader(ctx).Return(finalizedHeader, nil).Once()
	mockRPC.EXPECT().BatchGetBlockHeaders(ctx, []uint64{100, 101}).
		Return([]*types.Header{header100, header101}, nil).Once()

	logs := []types.Log{
		{BlockNumber: 100, BlockHash: header100.Hash()},
		{BlockNumber: 101, BlockHash: header101.Hash()},
	}

	err := detector.VerifyAndRecordBlocks(ctx, logs, 100, 101)
	require.NoError(t, err)

	// Now verify and record new blocks - should verify existing non-finalized blocks
	header102 := createTestHeader(102, header101.Hash())
	header103 := createTestHeader(103, header102.Hash())

	mockRPC.EXPECT().GetFinalizedBlockHeader(ctx).Return(finalizedHeader, nil).Once()
	// Should verify blocks 100 and 101 (non-finalized)
	mockRPC.EXPECT().BatchGetBlockHeaders(ctx, []uint64{100, 101}).
		Return([]*types.Header{header100, header101}, nil).Once()
	// Then fetch new blocks 102-103
	mockRPC.EXPECT().BatchGetBlockHeaders(ctx, []uint64{102, 103}).
		Return([]*types.Header{header102, header103}, nil).Once()

	logs2 := []types.Log{
		{BlockNumber: 102, BlockHash: header102.Hash()},
		{BlockNumber: 103, BlockHash: header103.Hash()},
	}

	err = detector.VerifyAndRecordBlocks(ctx, logs2, 102, 103)
	require.NoError(t, err)

	// Verify all blocks were recorded
	tx, err := detector.db.Begin()
	require.NoError(t, err)
	defer func() {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			t.Errorf("failed to rollback transaction: %v", err)
		}
	}()

	block103, err := detector.getStoredBlockTx(tx, 103)
	require.NoError(t, err)
	require.Equal(t, header103.Hash(), block103.BlockHash)
}

func TestReorgDetector_VerifyAndRecordBlocks_ReorgInNonFinalizedBlocks(t *testing.T) {
	detector, mockRPC, cleanup := setupTestReorgDetector(t)
	defer cleanup()

	ctx := context.Background()

	// First, record some initial blocks
	header100 := createTestHeader(100, common.HexToHash("0x99"))
	header101 := createTestHeader(101, header100.Hash())
	finalizedHeader := createTestHeader(50, common.HexToHash("0x49"))

	mockRPC.EXPECT().GetFinalizedBlockHeader(ctx).Return(finalizedHeader, nil).Once()
	mockRPC.EXPECT().BatchGetBlockHeaders(ctx, []uint64{100, 101}).
		Return([]*types.Header{header100, header101}, nil).Once()

	logs := []types.Log{
		{BlockNumber: 100, BlockHash: header100.Hash()},
		{BlockNumber: 101, BlockHash: header101.Hash()},
	}

	err := detector.VerifyAndRecordBlocks(ctx, logs, 100, 101)
	require.NoError(t, err)

	// Now simulate a reorg - block 101 has a different hash on chain
	header101Reorg := createTestHeader(101, header100.Hash())
	header101Reorg.GasUsed = 1000 // Make it different

	mockRPC.EXPECT().GetFinalizedBlockHeader(ctx).Return(finalizedHeader, nil).Once()
	// Should verify blocks 100 and 101, but 101 has changed!
	mockRPC.EXPECT().BatchGetBlockHeaders(ctx, []uint64{100, 101}).
		Return([]*types.Header{header100, header101Reorg}, nil).Once()

	logs2 := []types.Log{
		{BlockNumber: 102, BlockHash: common.HexToHash("0x102")},
	}

	err = detector.VerifyAndRecordBlocks(ctx, logs2, 102, 102)
	require.Error(t, err)

	// Should be a reorg error
	var reorgErr *ReorgDetectedError
	require.True(t, errors.As(err, &reorgErr))
	require.Equal(t, uint64(101), reorgErr.FirstReorgBlock)
}

func TestReorgDetector_VerifyAndRecordBlocks_ReorgBetweenRPCCalls(t *testing.T) {
	detector, mockRPC, cleanup := setupTestReorgDetector(t)
	defer cleanup()

	ctx := context.Background()

	// Create headers
	header100 := createTestHeader(100, common.HexToHash("0x99"))
	header101 := createTestHeader(101, header100.Hash())
	finalizedHeader := createTestHeader(50, common.HexToHash("0x49"))

	mockRPC.EXPECT().GetFinalizedBlockHeader(ctx).Return(finalizedHeader, nil).Once()
	mockRPC.EXPECT().BatchGetBlockHeaders(ctx, []uint64{100, 101}).
		Return([]*types.Header{header100, header101}, nil).Once()

	// Logs have different hash than headers (reorg happened between eth_getLogs and eth_getBlockByNumber)
	logs := []types.Log{
		{BlockNumber: 100, BlockHash: common.HexToHash("0xdifferent100")},
		{BlockNumber: 101, BlockHash: header101.Hash()},
	}

	err := detector.VerifyAndRecordBlocks(ctx, logs, 100, 101)
	require.Error(t, err)

	// Should be a reorg error
	var reorgErr *ReorgDetectedError
	require.True(t, errors.As(err, &reorgErr))
	require.Equal(t, uint64(100), reorgErr.FirstReorgBlock)
}

func TestReorgDetector_VerifyAndRecordBlocks_ChainDiscontinuity(t *testing.T) {
	detector, mockRPC, cleanup := setupTestReorgDetector(t)
	defer cleanup()

	ctx := context.Background()

	// Create headers with discontinuous chain (101 doesn't reference 100 as parent)
	header100 := createTestHeader(100, common.HexToHash("0x99"))
	header101 := createTestHeader(101, common.HexToHash("0xwrong")) // Wrong parent!
	finalizedHeader := createTestHeader(50, common.HexToHash("0x49"))

	mockRPC.EXPECT().GetFinalizedBlockHeader(ctx).Return(finalizedHeader, nil).Once()
	mockRPC.EXPECT().BatchGetBlockHeaders(ctx, []uint64{100, 101}).
		Return([]*types.Header{header100, header101}, nil).Once()

	logs := []types.Log{
		{BlockNumber: 100, BlockHash: header100.Hash()},
		{BlockNumber: 101, BlockHash: header101.Hash()},
	}

	err := detector.VerifyAndRecordBlocks(ctx, logs, 100, 101)
	require.Error(t, err)

	// Should be a reorg error
	var reorgErr *ReorgDetectedError
	require.True(t, errors.As(err, &reorgErr))
	require.Equal(t, uint64(101), reorgErr.FirstReorgBlock)
}

func TestReorgDetector_VerifyAndRecordBlocks_PrunesFinalizedBlocks(t *testing.T) {
	detector, mockRPC, cleanup := setupTestReorgDetector(t)
	defer cleanup()

	ctx := context.Background()

	// First, record blocks 50-52
	header50 := createTestHeader(50, common.HexToHash("0x49"))
	header51 := createTestHeader(51, header50.Hash())
	header52 := createTestHeader(52, header51.Hash())
	finalizedHeader40 := createTestHeader(40, common.HexToHash("0x39"))

	mockRPC.EXPECT().GetFinalizedBlockHeader(ctx).Return(finalizedHeader40, nil).Once()
	mockRPC.EXPECT().BatchGetBlockHeaders(ctx, []uint64{50, 51, 52}).
		Return([]*types.Header{header50, header51, header52}, nil).Once()

	logs := []types.Log{
		{BlockNumber: 50, BlockHash: header50.Hash()},
		{BlockNumber: 51, BlockHash: header51.Hash()},
		{BlockNumber: 52, BlockHash: header52.Hash()},
	}

	err := detector.VerifyAndRecordBlocks(ctx, logs, 50, 52)
	require.NoError(t, err)

	// Now finalized block is 51, should prune blocks <= 51
	header53 := createTestHeader(53, header52.Hash())

	mockRPC.EXPECT().GetFinalizedBlockHeader(ctx).Return(header51, nil).Once()
	// Should verify only block 52 (non-finalized)
	mockRPC.EXPECT().BatchGetBlockHeaders(ctx, []uint64{52}).
		Return([]*types.Header{header52}, nil).Once()
	// Then fetch new block 53
	mockRPC.EXPECT().BatchGetBlockHeaders(ctx, []uint64{53}).
		Return([]*types.Header{header53}, nil).Once()

	logs2 := []types.Log{
		{BlockNumber: 53, BlockHash: header53.Hash()},
	}

	err = detector.VerifyAndRecordBlocks(ctx, logs2, 53, 53)
	require.NoError(t, err)

	// Verify blocks 50 and 51 were pruned
	tx, err := detector.db.Begin()
	require.NoError(t, err)
	defer func() {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			t.Errorf("failed to rollback transaction: %v", err)
		}
	}()

	blocks, err := detector.getStoredBlocksAfterBlockTx(tx, 0)
	require.NoError(t, err)
	require.Len(t, blocks, 2) // Only 52 and 53 should remain
	require.Equal(t, uint64(52), blocks[0].BlockNumber)
	require.Equal(t, uint64(53), blocks[1].BlockNumber)
}

func TestReorgDetector_VerifyAndRecordBlocks_EmptyLogs(t *testing.T) {
	detector, mockRPC, cleanup := setupTestReorgDetector(t)
	defer cleanup()

	ctx := context.Background()

	// Create headers
	header100 := createTestHeader(100, common.HexToHash("0x99"))
	header101 := createTestHeader(101, header100.Hash())
	finalizedHeader := createTestHeader(50, common.HexToHash("0x49"))

	mockRPC.EXPECT().GetFinalizedBlockHeader(ctx).Return(finalizedHeader, nil).Once()
	mockRPC.EXPECT().BatchGetBlockHeaders(ctx, []uint64{100, 101}).
		Return([]*types.Header{header100, header101}, nil).Once()

	// Empty logs array (no logs in this range, but still need to verify blocks)
	var logs []types.Log

	err := detector.VerifyAndRecordBlocks(ctx, logs, 100, 101)
	require.NoError(t, err)

	// Verify blocks were recorded
	tx, err := detector.db.Begin()
	require.NoError(t, err)
	defer func() {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			t.Errorf("failed to rollback transaction: %v", err)
		}
	}()

	block, err := detector.getStoredBlockTx(tx, 100)
	require.NoError(t, err)
	require.Equal(t, header100.Hash(), block.BlockHash)
}

func TestReorgDetector_VerifyAndRecordBlocks_SingleBlock(t *testing.T) {
	detector, mockRPC, cleanup := setupTestReorgDetector(t)
	defer cleanup()

	ctx := context.Background()

	// Single block
	header100 := createTestHeader(100, common.HexToHash("0x99"))
	finalizedHeader := createTestHeader(50, common.HexToHash("0x49"))

	mockRPC.EXPECT().GetFinalizedBlockHeader(ctx).Return(finalizedHeader, nil).Once()
	mockRPC.EXPECT().BatchGetBlockHeaders(ctx, []uint64{100}).
		Return([]*types.Header{header100}, nil).Once()

	logs := []types.Log{
		{BlockNumber: 100, BlockHash: header100.Hash()},
	}

	err := detector.VerifyAndRecordBlocks(ctx, logs, 100, 100)
	require.NoError(t, err)

	// Verify block was recorded
	tx, err := detector.db.Begin()
	require.NoError(t, err)
	defer func() {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			t.Errorf("failed to rollback transaction: %v", err)
		}
	}()

	block, err := detector.getStoredBlockTx(tx, 100)
	require.NoError(t, err)
	require.Equal(t, header100.Hash(), block.BlockHash)
	require.Equal(t, header100.ParentHash, block.ParentHash)
}

func TestReorgDetector_Close(t *testing.T) {
	// Create temporary database
	tmpFile, err := os.CreateTemp("", "reorg_test_*.db")
	require.NoError(t, err)
	tmpFile.Close()
	dbPath := tmpFile.Name()
	defer os.Remove(dbPath)

	// Create mock RPC client
	mockRPC := mocks.NewEthClient(t)

	// Create reorg detector
	log, err := logger.NewLogger("error", true)
	require.NoError(t, err)

	detector, err := NewReorgDetector(dbPath, mockRPC, log)
	require.NoError(t, err)

	// Close should not return an error
	err = detector.Close()
	require.NoError(t, err)
}

func TestReorgDetector_StoredBlockOperations(t *testing.T) {
	detector, mockRPC, cleanup := setupTestReorgDetector(t)
	defer cleanup()

	ctx := context.Background()

	// Record some blocks first
	header100 := createTestHeader(100, common.HexToHash("0x99"))
	header101 := createTestHeader(101, header100.Hash())
	header102 := createTestHeader(102, header101.Hash())
	finalizedHeader := createTestHeader(50, common.HexToHash("0x49"))

	mockRPC.EXPECT().GetFinalizedBlockHeader(ctx).Return(finalizedHeader, nil).Once()
	mockRPC.EXPECT().BatchGetBlockHeaders(ctx, []uint64{100, 101, 102}).
		Return([]*types.Header{header100, header101, header102}, nil).Once()

	logs := []types.Log{
		{BlockNumber: 100, BlockHash: header100.Hash()},
		{BlockNumber: 101, BlockHash: header101.Hash()},
		{BlockNumber: 102, BlockHash: header102.Hash()},
	}

	err := detector.VerifyAndRecordBlocks(ctx, logs, 100, 102)
	require.NoError(t, err)

	// Test getStoredBlockTx
	tx, err := detector.db.Begin()
	require.NoError(t, err)
	defer func() {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			t.Errorf("failed to rollback transaction: %v", err)
		}
	}()

	block, err := detector.getStoredBlockTx(tx, 101)
	require.NoError(t, err)
	require.Equal(t, uint64(101), block.BlockNumber)
	require.Equal(t, header101.Hash(), block.BlockHash)
	require.Equal(t, header101.ParentHash, block.ParentHash)

	// Test getStoredBlocksAfterBlockTx
	blocks, err := detector.getStoredBlocksAfterBlockTx(tx, 100)
	require.NoError(t, err)
	require.Len(t, blocks, 2) // 101 and 102
	require.Equal(t, uint64(101), blocks[0].BlockNumber)
	require.Equal(t, uint64(102), blocks[1].BlockNumber)

	// Test with no results
	blocks, err = detector.getStoredBlocksAfterBlockTx(tx, 200)
	require.NoError(t, err)
	require.Len(t, blocks, 0)
}
