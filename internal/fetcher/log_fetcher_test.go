package fetcher

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	storemocks "github.com/goran-ethernal/ChainIndexor/internal/fetcher/store/mocks"
	"github.com/goran-ethernal/ChainIndexor/internal/logger"
	reorgmocks "github.com/goran-ethernal/ChainIndexor/internal/reorg/mocks"
	rpcmocks "github.com/goran-ethernal/ChainIndexor/internal/rpc/mocks"
	itypes "github.com/goran-ethernal/ChainIndexor/internal/types"
	"github.com/goran-ethernal/ChainIndexor/pkg/fetcher"
	"github.com/goran-ethernal/ChainIndexor/pkg/fetcher/store"
	"github.com/goran-ethernal/ChainIndexor/pkg/reorg"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func createTestHeader(blockNum uint64, parentHash common.Hash) *types.Header {
	return &types.Header{
		Number:     big.NewInt(int64(blockNum)),
		ParentHash: parentHash,
		Difficulty: big.NewInt(1),
		GasLimit:   8000000,
		GasUsed:    0,
		Time:       1000000 + blockNum,
	}
}

func setupTestLogFetcher(t *testing.T) (*LogFetcher, *rpcmocks.EthClient, *reorgmocks.Detector, *storemocks.LogStore) {
	t.Helper()

	mockRPC := rpcmocks.NewEthClient(t)
	mockReorg := reorgmocks.NewDetector(t)
	mockStore := storemocks.NewLogStore(t)

	log, err := logger.NewLogger("error", true)
	require.NoError(t, err)

	addr1 := common.HexToAddress("0x1111111111111111111111111111111111111111")
	topic1 := common.HexToHash("0xaaaa")

	cfg := LogFetcherConfig{
		ChunkSize:          100,
		Finality:           itypes.FinalityFinalized,
		FinalizedLag:       0,
		Addresses:          []common.Address{addr1},
		Topics:             [][]common.Hash{{topic1}},
		AddressStartBlocks: map[common.Address]uint64{addr1: 0},
	}

	lf := NewLogFetcher(cfg, log, mockRPC, mockReorg, mockStore)

	return lf, mockRPC, mockReorg, mockStore
}

func TestNewLogFetcher(t *testing.T) {
	lf, mockRPC, mockReorg, mockStore := setupTestLogFetcher(t)

	require.NotNil(t, lf)
	require.Equal(t, fetcher.ModeBackfill, lf.GetMode())
	require.NotNil(t, lf.cfg)
	require.NotNil(t, lf.rpc)
	require.NotNil(t, lf.reorgDetector)
	require.NotNil(t, lf.logStore)
	require.NotNil(t, lf.log)

	// Verify mocks are not nil
	require.NotNil(t, mockRPC)
	require.NotNil(t, mockReorg)
	require.NotNil(t, mockStore)
}

func TestLogFetcher_SetMode(t *testing.T) {
	lf, _, _, _ := setupTestLogFetcher(t) //nolint:dogsled

	require.Equal(t, fetcher.ModeBackfill, lf.GetMode())

	lf.SetMode(fetcher.ModeLive)
	require.Equal(t, fetcher.ModeLive, lf.GetMode())

	lf.SetMode(fetcher.ModeBackfill)
	require.Equal(t, fetcher.ModeBackfill, lf.GetMode())
}

func TestLogFetcher_FetchRange_Success(t *testing.T) {
	lf, mockRPC, mockReorg, mockStore := setupTestLogFetcher(t)
	ctx := context.Background()

	// Create test headers
	finalizedBlock := createTestHeader(99, common.HexToHash("0x99"))
	header100 := createTestHeader(100, finalizedBlock.Hash())
	header101 := createTestHeader(101, header100.Hash())
	header102 := createTestHeader(102, header101.Hash())

	testLogs := []types.Log{
		{
			BlockNumber: 100,
			BlockHash:   header100.Hash(),
			Address:     lf.cfg.Addresses[0],
			Topics:      []common.Hash{lf.cfg.Topics[0][0]},
		},
		{
			BlockNumber: 101,
			BlockHash:   header101.Hash(),
			Address:     lf.cfg.Addresses[0],
			Topics:      []common.Hash{lf.cfg.Topics[0][0]},
		},
	}

	mockRPC.EXPECT().GetLogs(ctx, mock.Anything).Return(testLogs, nil).Once()
	mockStore.EXPECT().StoreLogs(ctx, lf.cfg.Addresses, lf.cfg.Topics, testLogs, uint64(100), uint64(102), finalizedBlock).Return(nil).Once()
	mockReorg.EXPECT().VerifyAndRecordBlocks(ctx, testLogs, uint64(100), uint64(102)).Return(
		[]*types.Header{header100, header101, header102}, nil).Once()

	result, err := lf.FetchRange(ctx, finalizedBlock, 100, 102)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, uint64(100), result.FromBlock)
	require.Equal(t, uint64(102), result.ToBlock)
	require.Len(t, result.Logs, 2)
	require.Len(t, result.Headers, 3)
}

func TestLogFetcher_FetchRange_LogFetchError(t *testing.T) {
	lf, mockRPC, _, _ := setupTestLogFetcher(t)
	ctx := context.Background()

	mockRPC.EXPECT().GetLogs(ctx, mock.Anything).Return(nil, errors.New("log fetch error")).Once()

	finalizedBlock := createTestHeader(99, common.HexToHash("0x99"))
	result, err := lf.FetchRange(ctx, finalizedBlock, 100, 102)
	require.Error(t, err)
	require.Nil(t, result)
	require.Contains(t, err.Error(), "failed to fetch logs")
}

func TestLogFetcher_FetchRange_ReorgDetected(t *testing.T) {
	lf, mockRPC, mockReorg, mockStore := setupTestLogFetcher(t)
	ctx := context.Background()

	finalizedBlock := createTestHeader(99, common.HexToHash("0x99"))
	header100 := createTestHeader(100, finalizedBlock.Hash())

	testLogs := []types.Log{
		{BlockNumber: 100, BlockHash: header100.Hash()},
	}

	mockRPC.EXPECT().GetLogs(ctx, mock.Anything).Return(testLogs, nil).Once()

	reorgErr := &reorg.ReorgDetectedError{
		FirstReorgBlock: 101,
		Details:         "test reorg",
	}

	mockStore.EXPECT().StoreLogs(ctx, lf.cfg.Addresses, lf.cfg.Topics, testLogs, uint64(100), uint64(102), finalizedBlock).Return(nil).Once()
	mockReorg.EXPECT().VerifyAndRecordBlocks(ctx, testLogs, uint64(100), uint64(102)).
		Return(nil, reorgErr).Once()
	mockStore.EXPECT().HandleReorg(ctx, uint64(101)).Return(nil).Once()

	result, err := lf.FetchRange(ctx, finalizedBlock, 100, 102)
	require.Error(t, err)
	require.Nil(t, result)
	require.Contains(t, err.Error(), "reorg detected")
}

func TestLogFetcher_FetchRange_NoActiveAddresses(t *testing.T) {
	lf, _, mockReorg, mockStore := setupTestLogFetcher(t)
	ctx := context.Background()

	// Set start block beyond the range we're fetching
	lf.cfg.AddressStartBlocks[lf.cfg.Addresses[0]] = 200

	finalizedBlock := createTestHeader(99, common.HexToHash("0x99"))
	header100 := createTestHeader(100, finalizedBlock.Hash())
	header101 := createTestHeader(101, header100.Hash())

	// No GetLogs call should be made since no addresses are active
	emptyLogs := []types.Log{}
	mockStore.EXPECT().StoreLogs(ctx, []common.Address{}, [][]common.Hash{}, emptyLogs, uint64(100), uint64(101), finalizedBlock).Return(nil).Once()
	mockReorg.EXPECT().VerifyAndRecordBlocks(ctx, emptyLogs, uint64(100), uint64(101)).
		Return([]*types.Header{header100, header101}, nil).Once()

	result, err := lf.FetchRange(ctx, finalizedBlock, 100, 101)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, result.Logs, 0)
	require.Len(t, result.Headers, 2)
}

func TestLogFetcher_FetchBackfill_Success(t *testing.T) {
	lf, mockRPC, mockReorg, mockStore := setupTestLogFetcher(t)
	ctx := context.Background()

	// Mock unsynced topics - empty
	mockStore.EXPECT().GetUnsyncedTopics(ctx, lf.cfg.Addresses, lf.cfg.Topics, uint64(50)).
		Return(store.NewUnsyncedTopics(), nil).Once()

	// Mock finalized block at 150
	finalizedHeader := createTestHeader(150, common.HexToHash("0x149"))
	mockRPC.EXPECT().GetFinalizedBlockHeader(ctx).Return(finalizedHeader, nil).Once()

	// Mock headers for range 51-150 (capped by chunk size to 51-150)
	headers := make([]*types.Header, 100)
	for i := range 100 {
		headers[i] = createTestHeader(uint64(51+i), common.HexToHash("0x0"))
	}
	blockNums := make([]uint64, 100)
	for i := range 100 {
		blockNums[i] = uint64(51 + i)
	}

	testLogs := []types.Log{{BlockNumber: 51}}
	mockRPC.EXPECT().GetLogs(ctx, mock.Anything).Return(testLogs, nil).Once()
	mockStore.EXPECT().StoreLogs(ctx, lf.cfg.Addresses, lf.cfg.Topics, testLogs, uint64(51), uint64(150), finalizedHeader).Return(nil).Once()
	mockReorg.EXPECT().VerifyAndRecordBlocks(ctx, testLogs, uint64(51), uint64(150)).Return(headers, nil).Once()

	result, err := lf.FetchNext(ctx, 50, 0)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, uint64(51), result.FromBlock)
	require.Equal(t, uint64(150), result.ToBlock)
}

func TestLogFetcher_FetchBackfill_WithUnsyncedTopics(t *testing.T) {
	lf, mockRPC, mockReorg, mockStore := setupTestLogFetcher(t)
	ctx := context.Background()

	// Mock unsynced topics
	unsyncedTopics := store.NewUnsyncedTopics()
	unsyncedTopics.AddTopic(lf.cfg.Addresses[0], lf.cfg.Topics[0][0], store.CoverageRange{
		FromBlock: 0,
		ToBlock:   25,
	})

	mockStore.EXPECT().GetUnsyncedTopics(ctx, lf.cfg.Addresses, lf.cfg.Topics, uint64(50)).
		Return(unsyncedTopics, nil).Once()

	// Should fetch from lastCoveredBlock+1 (26) to min(26+chunkSize-1, lastIndexedBlock) = min(125, 50) = 50
	finalizedBlock := createTestHeader(99, common.HexToHash("0x99"))
	headers := make([]*types.Header, 25)
	blockNums := make([]uint64, 25)
	for i := range 25 {
		headers[i] = createTestHeader(uint64(26+i), common.HexToHash("0x0"))
		blockNums[i] = uint64(26 + i)
	}

	testLogs := []types.Log{{BlockNumber: 26}}
	mockRPC.EXPECT().GetFinalizedBlockHeader(ctx).Return(finalizedBlock, nil).Once()
	mockRPC.EXPECT().GetLogs(ctx, mock.Anything).Return(testLogs, nil).Once()
	mockStore.EXPECT().StoreLogs(ctx, lf.cfg.Addresses, lf.cfg.Topics, testLogs, uint64(26), uint64(50), finalizedBlock).Return(nil).Once()
	mockReorg.EXPECT().VerifyAndRecordBlocks(ctx, testLogs, uint64(26), uint64(50)).Return(headers, nil).Once()

	result, err := lf.FetchNext(ctx, 50, 0)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, uint64(26), result.FromBlock)
	require.Equal(t, uint64(50), result.ToBlock)
}

func TestLogFetcher_FetchBackfill_SwitchesToLive(t *testing.T) {
	lf, mockRPC, _, mockStore := setupTestLogFetcher(t)
	ctx := context.Background()

	mockStore.EXPECT().GetUnsyncedTopics(mock.Anything, lf.cfg.Addresses, lf.cfg.Topics, uint64(100)).
		Return(store.NewUnsyncedTopics(), nil).Once()

	// Finalized block is 100, last indexed is 100, so we're caught up
	finalizedHeader := createTestHeader(100, common.HexToHash("0x99"))
	mockRPC.EXPECT().GetFinalizedBlockHeader(mock.Anything).Return(finalizedHeader, nil).Times(2)

	// Create a context with cancellation to avoid infinite loop
	ctxWithCancel, cancel := context.WithCancel(ctx)
	cancel() // Cancel immediately

	result, err := lf.FetchNext(ctxWithCancel, 100, 0)
	require.Error(t, err)
	require.Nil(t, result)
	require.Equal(t, context.Canceled, err)
	require.Equal(t, fetcher.ModeLive, lf.GetMode()) // Should have switched to live mode
}

func TestLogFetcher_FetchLive_NewBlocks(t *testing.T) {
	lf, mockRPC, mockReorg, mockStore := setupTestLogFetcher(t)
	lf.SetMode(fetcher.ModeLive)
	ctx := context.Background()

	// Finalized block is 105, last indexed is 100
	finalizedHeader := createTestHeader(105, common.HexToHash("0x104"))
	mockRPC.EXPECT().GetFinalizedBlockHeader(ctx).Return(finalizedHeader, nil).Once()

	headers := make([]*types.Header, 5)
	blockNums := make([]uint64, 5)
	for i := range 5 {
		headers[i] = createTestHeader(uint64(101+i), common.HexToHash("0x0"))
		blockNums[i] = uint64(101 + i)
	}

	testLogs := []types.Log{{BlockNumber: 101}}
	mockRPC.EXPECT().GetLogs(ctx, mock.Anything).Return(testLogs, nil).Once()
	mockStore.EXPECT().StoreLogs(ctx, lf.cfg.Addresses, lf.cfg.Topics, testLogs, uint64(101), uint64(105), finalizedHeader).Return(nil).Once()
	mockReorg.EXPECT().VerifyAndRecordBlocks(ctx, testLogs, uint64(101), uint64(105)).Return(headers, nil).Once()

	result, err := lf.FetchNext(ctx, 100, 0)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, uint64(101), result.FromBlock)
	require.Equal(t, uint64(105), result.ToBlock)
}

func TestLogFetcher_FetchLive_ChunksLargeRanges(t *testing.T) {
	lf, mockRPC, mockReorg, mockStore := setupTestLogFetcher(t)
	lf.SetMode(fetcher.ModeLive)
	lf.cfg.ChunkSize = 10 // Small chunk for testing
	ctx := context.Background()

	// Finalized block is 200, last indexed is 100, should chunk
	finalizedHeader := createTestHeader(200, common.HexToHash("0x199"))
	mockRPC.EXPECT().GetFinalizedBlockHeader(ctx).Return(finalizedHeader, nil).Once()

	headers := make([]*types.Header, 10)
	blockNums := make([]uint64, 10)
	for i := range 10 {
		headers[i] = createTestHeader(uint64(101+i), common.HexToHash("0x0"))
		blockNums[i] = uint64(101 + i)
	}

	testLogs := []types.Log{{BlockNumber: 101}}
	mockRPC.EXPECT().GetLogs(ctx, mock.Anything).Return(testLogs, nil).Once()
	mockStore.EXPECT().StoreLogs(ctx, lf.cfg.Addresses, lf.cfg.Topics, testLogs, uint64(101), uint64(110), finalizedHeader).Return(nil).Once()
	mockReorg.EXPECT().VerifyAndRecordBlocks(ctx, testLogs, uint64(101), uint64(110)).Return(headers, nil).Once()

	result, err := lf.FetchNext(ctx, 100, 0)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, uint64(101), result.FromBlock)
	require.Equal(t, uint64(110), result.ToBlock) // Chunked to 10 blocks
}

func TestLogFetcher_GetFinalizedBlock_Finalized(t *testing.T) {
	lf, mockRPC, _, _ := setupTestLogFetcher(t)
	lf.cfg.Finality = itypes.FinalityFinalized
	ctx := context.Background()

	header := createTestHeader(100, common.HexToHash("0x99"))
	mockRPC.EXPECT().GetFinalizedBlockHeader(ctx).Return(header, nil).Once()

	finalizedBlock, err := lf.getFinalizedBlock(ctx)
	require.NoError(t, err)
	require.Equal(t, header, finalizedBlock)
}

func TestLogFetcher_GetFinalizedBlock_Safe(t *testing.T) {
	lf, mockRPC, _, _ := setupTestLogFetcher(t)
	lf.cfg.Finality = itypes.FinalitySafe
	ctx := context.Background()

	header := createTestHeader(98, common.HexToHash("0x97"))
	mockRPC.EXPECT().GetSafeBlockHeader(ctx).Return(header, nil).Once()

	finalizedBlock, err := lf.getFinalizedBlock(ctx)
	require.NoError(t, err)
	require.Equal(t, header, finalizedBlock)
}

func TestLogFetcher_GetFinalizedBlock_LatestWithLag(t *testing.T) {
	lf, mockRPC, _, _ := setupTestLogFetcher(t)
	lf.cfg.Finality = itypes.FinalityLatest
	lf.cfg.FinalizedLag = 10
	ctx := context.Background()

	header := createTestHeader(100, common.HexToHash("0x99"))
	mockRPC.EXPECT().GetLatestBlockHeader(ctx).Return(header, nil).Once()
	blockWithLag := createTestHeader(90, common.HexToHash("0x89"))
	mockRPC.EXPECT().GetBlockHeader(ctx, uint64(90)).Return(blockWithLag, nil).Once()

	finalizedBlock, err := lf.getFinalizedBlock(ctx)
	require.NoError(t, err)
	require.Equal(t, blockWithLag, finalizedBlock) // 100 - 10
}

func TestLogFetcher_GetFinalizedBlock_LatestWithLagBelowZero(t *testing.T) {
	lf, mockRPC, _, _ := setupTestLogFetcher(t)
	lf.cfg.Finality = itypes.FinalityLatest
	lf.cfg.FinalizedLag = 200
	ctx := context.Background()

	header := createTestHeader(100, common.HexToHash("0x99"))
	mockRPC.EXPECT().GetLatestBlockHeader(ctx).Return(header, nil).Once()
	genesisBlock := createTestHeader(0, common.Hash{})
	mockRPC.EXPECT().GetBlockHeader(ctx, uint64(0)).Return(genesisBlock, nil).Once()

	finalizedBlock, err := lf.getFinalizedBlock(ctx)
	require.NoError(t, err)
	require.Equal(t, genesisBlock, finalizedBlock) // Can't go below 0
}

func TestLogFetcher_GetFinalizedBlock_InvalidMode(t *testing.T) {
	lf, _, _, _ := setupTestLogFetcher(t) //nolint:dogsled
	lf.cfg.Finality = "invalid"
	ctx := context.Background()

	finalizedBlock, err := lf.getFinalizedBlock(ctx)
	require.Error(t, err)
	require.Nil(t, finalizedBlock)
	require.Contains(t, err.Error(), "invalid finality mode")
}
