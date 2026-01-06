package indexer

import (
	"errors"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/goran-ethernal/ChainIndexor/internal/indexer/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func newTestLog(addr common.Address, topic common.Hash, block uint64) types.Log {
	return types.Log{
		Address:     addr,
		Topics:      []common.Hash{topic},
		BlockNumber: block,
	}
}

// captureHandledLogs is a helper to capture logs passed to HandleLogs with type-safe assertions.
func captureHandledLogs(captured *[]types.Log) func(mock.Arguments) {
	return func(args mock.Arguments) {
		if logs, ok := args[0].([]types.Log); ok {
			*captured = logs
		}
	}
}

func TestIndexerCoordinator_RegisterIndexer(t *testing.T) {
	t.Parallel()

	coord := NewIndexerCoordinator()
	addr := common.HexToAddress("0x1234")
	topic := common.HexToHash("0xabcd")

	idx := mocks.NewIndexer(t)
	idx.EXPECT().StartBlock().Return(uint64(100))
	idx.EXPECT().EventsToIndex().Return(map[common.Address]map[common.Hash]struct{}{
		addr: {topic: {}},
	})

	coord.RegisterIndexer(idx)

	startBlocks := coord.IndexerStartBlocks()
	assert.Equal(t, []uint64{100}, startBlocks)
}

func TestIndexerCoordinator_HandleLogsRoutesByAddressAndTopic(t *testing.T) {
	t.Parallel()

	coord := NewIndexerCoordinator()
	addr := common.HexToAddress("0xdeadbeef")
	topic := common.HexToHash("0xfeedface")
	logEntry := newTestLog(addr, topic, 1)

	idx := mocks.NewIndexer(t)
	idx.EXPECT().GetName().Return("testIndexer")
	idx.EXPECT().StartBlock().Return(uint64(0))
	idx.EXPECT().EventsToIndex().Return(map[common.Address]map[common.Hash]struct{}{
		addr: {topic: {}},
	})

	var handled []types.Log
	idx.On("HandleLogs", mock.Anything).Return(nil).Run(captureHandledLogs(&handled))

	coord.RegisterIndexer(idx)

	err := coord.HandleLogs([]types.Log{logEntry}, 0, 1)
	require.NoError(t, err)
	assert.Equal(t, []types.Log{logEntry}, handled)
}

func TestIndexerCoordinator_HandleLogsIgnoresLogsBeforeStartBlock(t *testing.T) {
	t.Parallel()

	coord := NewIndexerCoordinator()
	addr := common.HexToAddress("0xbeefdead")
	topic := common.HexToHash("0xcafebabe")
	logEntry := newTestLog(addr, topic, 5)

	idx := mocks.NewIndexer(t)
	idx.EXPECT().GetName().Return("testIndexer")
	idx.EXPECT().StartBlock().Return(uint64(10))
	idx.EXPECT().EventsToIndex().Return(map[common.Address]map[common.Hash]struct{}{
		addr: {topic: {}},
	})

	coord.RegisterIndexer(idx)

	err := coord.HandleLogs([]types.Log{logEntry}, 0, 5)
	require.NoError(t, err)
	idx.AssertNotCalled(t, "HandleLogs", mock.Anything)
}

func TestIndexerCoordinator_HandleLogsFiltersLogsAtExactStartBlock(t *testing.T) {
	t.Parallel()

	coord := NewIndexerCoordinator()
	addr := common.HexToAddress("0xaabbccdd")
	topic := common.HexToHash("0x11223344")
	logEntry := newTestLog(addr, topic, 10)

	idx := mocks.NewIndexer(t)
	idx.EXPECT().GetName().Return("testIndexer")
	idx.EXPECT().StartBlock().Return(uint64(10))
	idx.EXPECT().EventsToIndex().Return(map[common.Address]map[common.Hash]struct{}{
		addr: {topic: {}},
	})

	var handled []types.Log
	idx.On("HandleLogs", mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		if logs, ok := args[0].([]types.Log); ok {
			handled = logs
		}
	})

	coord.RegisterIndexer(idx)

	err := coord.HandleLogs([]types.Log{logEntry}, 0, 10)
	require.NoError(t, err)
	assert.Equal(t, []types.Log{logEntry}, handled)
}

func TestIndexerCoordinator_HandleLogsSupportsAllTopics(t *testing.T) {
	t.Parallel()

	coord := NewIndexerCoordinator()
	addr := common.HexToAddress("0xabcdef12")
	topic := common.HexToHash("0x12345678")
	logEntry := newTestLog(addr, topic, 1)

	idx := mocks.NewIndexer(t)
	idx.EXPECT().GetName().Return("testIndexer")
	idx.EXPECT().StartBlock().Return(uint64(0))
	idx.EXPECT().EventsToIndex().Return(map[common.Address]map[common.Hash]struct{}{
		addr: {}, // Empty topic set means all topics
	})

	var handled []types.Log
	idx.On("HandleLogs", mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		if logs, ok := args[0].([]types.Log); ok {
			handled = logs
		}
	})

	coord.RegisterIndexer(idx)

	err := coord.HandleLogs([]types.Log{logEntry}, 0, 1)
	require.NoError(t, err)
	assert.Equal(t, []types.Log{logEntry}, handled)
}

func TestIndexerCoordinator_HandleLogsRoutesToMultipleIndexers(t *testing.T) {
	t.Parallel()

	coord := NewIndexerCoordinator()
	addr := common.HexToAddress("0x1111")
	topic := common.HexToHash("0x2222")
	logEntry := newTestLog(addr, topic, 1)

	idx1 := mocks.NewIndexer(t)
	idx1.EXPECT().GetName().Return("testIndexer1")
	idx1.EXPECT().StartBlock().Return(uint64(0))
	idx1.EXPECT().EventsToIndex().Return(map[common.Address]map[common.Hash]struct{}{
		addr: {topic: {}},
	})
	var handled1 []types.Log
	idx1.On("HandleLogs", mock.Anything).Return(nil).Run(captureHandledLogs(&handled1))

	idx2 := mocks.NewIndexer(t)
	idx2.EXPECT().GetName().Return("testIndexer2")
	idx2.EXPECT().StartBlock().Return(uint64(0))
	idx2.EXPECT().EventsToIndex().Return(map[common.Address]map[common.Hash]struct{}{
		addr: {topic: {}},
	})
	var handled2 []types.Log
	idx2.On("HandleLogs", mock.Anything).Return(nil).Run(captureHandledLogs(&handled2))

	coord.RegisterIndexer(idx1)
	coord.RegisterIndexer(idx2)

	err := coord.HandleLogs([]types.Log{logEntry}, 0, 1)
	require.NoError(t, err)
	assert.Equal(t, []types.Log{logEntry}, handled1)
	assert.Equal(t, []types.Log{logEntry}, handled2)
}

func TestIndexerCoordinator_HandleLogsIgnoresUnmatchedAddress(t *testing.T) {
	t.Parallel()

	coord := NewIndexerCoordinator()
	addr1 := common.HexToAddress("0x1111")
	addr2 := common.HexToAddress("0x2222")
	topic := common.HexToHash("0x3333")
	logEntry := newTestLog(addr2, topic, 1)

	idx := mocks.NewIndexer(t)
	idx.EXPECT().StartBlock().Return(uint64(0))
	idx.EXPECT().EventsToIndex().Return(map[common.Address]map[common.Hash]struct{}{
		addr1: {topic: {}},
	})

	coord.RegisterIndexer(idx)

	err := coord.HandleLogs([]types.Log{logEntry}, 0, 1)
	require.NoError(t, err)
	idx.AssertNotCalled(t, "HandleLogs", mock.Anything)
}

func TestIndexerCoordinator_HandleLogsIgnoresUnmatchedTopic(t *testing.T) {
	t.Parallel()

	coord := NewIndexerCoordinator()
	addr := common.HexToAddress("0x1111")
	topic1 := common.HexToHash("0x2222")
	topic2 := common.HexToHash("0x3333")
	logEntry := newTestLog(addr, topic2, 1)

	idx := mocks.NewIndexer(t)
	idx.EXPECT().StartBlock().Return(uint64(0))
	idx.EXPECT().EventsToIndex().Return(map[common.Address]map[common.Hash]struct{}{
		addr: {topic1: {}},
	})

	coord.RegisterIndexer(idx)

	err := coord.HandleLogs([]types.Log{logEntry}, 0, 1)
	require.NoError(t, err)
	idx.AssertNotCalled(t, "HandleLogs", mock.Anything)
}

func TestIndexerCoordinator_HandleLogsPropagatesErrors(t *testing.T) {
	t.Parallel()

	coord := NewIndexerCoordinator()
	addr := common.HexToAddress("0x11110000")
	topic := common.HexToHash("0x22220000")
	logEntry := newTestLog(addr, topic, 1)

	idx := mocks.NewIndexer(t)
	idx.EXPECT().GetName().Return("testIndexer")
	idx.EXPECT().StartBlock().Return(uint64(0))
	idx.EXPECT().EventsToIndex().Return(map[common.Address]map[common.Hash]struct{}{
		addr: {topic: {}},
	})
	expectedErr := errors.New("boom")
	idx.EXPECT().HandleLogs(mock.Anything).Return(expectedErr)

	coord.RegisterIndexer(idx)

	err := coord.HandleLogs([]types.Log{logEntry}, 0, 1)
	require.Error(t, err)
	assert.ErrorContains(t, err, expectedErr.Error())
}

func TestIndexerCoordinator_HandleLogsWithMultipleLogs(t *testing.T) {
	t.Parallel()

	coord := NewIndexerCoordinator()
	addr := common.HexToAddress("0x1234")
	topic := common.HexToHash("0x5678")
	log1 := newTestLog(addr, topic, 1)
	log2 := newTestLog(addr, topic, 2)
	log3 := newTestLog(addr, topic, 3)

	idx := mocks.NewIndexer(t)
	idx.EXPECT().GetName().Return("testIndexer")
	idx.EXPECT().StartBlock().Return(uint64(0))
	idx.EXPECT().EventsToIndex().Return(map[common.Address]map[common.Hash]struct{}{
		addr: {topic: {}},
	})

	var handled []types.Log
	idx.On("HandleLogs", mock.Anything).Return(nil).Run(captureHandledLogs(&handled))

	coord.RegisterIndexer(idx)

	err := coord.HandleLogs([]types.Log{log1, log2, log3}, 0, 3)
	require.NoError(t, err)
	assert.Len(t, handled, 3)
	assert.Contains(t, handled, log1)
	assert.Contains(t, handled, log2)
	assert.Contains(t, handled, log3)
}

func TestIndexerCoordinator_HandleLogsWithEmptyLogList(t *testing.T) {
	t.Parallel()

	coord := NewIndexerCoordinator()
	addr := common.HexToAddress("0x1234")
	topic := common.HexToHash("0x5678")

	idx := mocks.NewIndexer(t)
	idx.EXPECT().StartBlock().Return(uint64(0))
	idx.EXPECT().EventsToIndex().Return(map[common.Address]map[common.Hash]struct{}{
		addr: {topic: {}},
	})

	coord.RegisterIndexer(idx)

	err := coord.HandleLogs([]types.Log{}, 0, 0)
	require.NoError(t, err)
	idx.AssertNotCalled(t, "HandleLogs", mock.Anything)
}

func TestIndexerCoordinator_HandleReorgSuccess(t *testing.T) {
	t.Parallel()

	coord := NewIndexerCoordinator()

	idx1 := mocks.NewIndexer(t)
	idx1.EXPECT().StartBlock().Return(uint64(0))
	idx1.EXPECT().EventsToIndex().Return(nil)
	idx1.EXPECT().HandleReorg(uint64(100)).Return(nil)

	idx2 := mocks.NewIndexer(t)
	idx2.EXPECT().StartBlock().Return(uint64(0))
	idx2.EXPECT().EventsToIndex().Return(nil)
	idx2.EXPECT().HandleReorg(uint64(100)).Return(nil)

	coord.RegisterIndexer(idx1)
	coord.RegisterIndexer(idx2)

	err := coord.HandleReorg(100)
	require.NoError(t, err)

	idx1.AssertCalled(t, "HandleReorg", uint64(100))
	idx2.AssertCalled(t, "HandleReorg", uint64(100))
}

func TestIndexerCoordinator_HandleReorgPropagatesErrors(t *testing.T) {
	t.Parallel()

	coord := NewIndexerCoordinator()

	idx1 := mocks.NewIndexer(t)
	idx1.EXPECT().StartBlock().Return(uint64(0))
	idx1.EXPECT().EventsToIndex().Return(nil)
	idx1.EXPECT().HandleReorg(uint64(100)).Return(nil)

	idx2 := mocks.NewIndexer(t)
	idx2.EXPECT().StartBlock().Return(uint64(0))
	idx2.EXPECT().EventsToIndex().Return(nil)
	reorgErr := errors.New("reorg fail")
	idx2.EXPECT().HandleReorg(uint64(100)).Return(reorgErr)

	coord.RegisterIndexer(idx1)
	coord.RegisterIndexer(idx2)

	err := coord.HandleReorg(100)
	require.Error(t, err)
	assert.ErrorContains(t, err, reorgErr.Error())
	assert.ErrorContains(t, err, "block 100")
}

func TestIndexerCoordinator_IndexerStartBlocks(t *testing.T) {
	t.Parallel()

	coord := NewIndexerCoordinator()

	idx1 := mocks.NewIndexer(t)
	idx1.EXPECT().StartBlock().Return(uint64(5))
	idx1.EXPECT().EventsToIndex().Return(nil)

	idx2 := mocks.NewIndexer(t)
	idx2.EXPECT().StartBlock().Return(uint64(10))
	idx2.EXPECT().EventsToIndex().Return(nil)

	idx3 := mocks.NewIndexer(t)
	idx3.EXPECT().StartBlock().Return(uint64(15))
	idx3.EXPECT().EventsToIndex().Return(nil)

	coord.RegisterIndexer(idx1)
	coord.RegisterIndexer(idx2)
	coord.RegisterIndexer(idx3)

	assert.Equal(t, []uint64{5, 10, 15}, coord.IndexerStartBlocks())
}

func TestIndexerCoordinator_IndexerStartBlocksEmpty(t *testing.T) {
	t.Parallel()

	coord := NewIndexerCoordinator()
	assert.Equal(t, []uint64{}, coord.IndexerStartBlocks())
}

func TestIndexerCoordinator_HandleLogsWithMixedStartBlocks(t *testing.T) {
	t.Parallel()

	coord := NewIndexerCoordinator()
	addr := common.HexToAddress("0x1234")
	topic := common.HexToHash("0x5678")
	log1 := newTestLog(addr, topic, 5)
	log2 := newTestLog(addr, topic, 15)
	log3 := newTestLog(addr, topic, 25)

	// Indexer 1 starts at block 10
	idx1 := mocks.NewIndexer(t)
	idx1.EXPECT().GetName().Return("testIndexer1")
	idx1.EXPECT().StartBlock().Return(uint64(10))
	idx1.EXPECT().EventsToIndex().Return(map[common.Address]map[common.Hash]struct{}{
		addr: {topic: {}},
	})
	var handled1 []types.Log
	idx1.On("HandleLogs", mock.Anything).Return(nil).Run(captureHandledLogs(&handled1))

	// Indexer 2 starts at block 20
	idx2 := mocks.NewIndexer(t)
	idx2.EXPECT().GetName().Return("testIndexer2")
	idx2.EXPECT().StartBlock().Return(uint64(20))
	idx2.EXPECT().EventsToIndex().Return(map[common.Address]map[common.Hash]struct{}{
		addr: {topic: {}},
	})
	var handled2 []types.Log
	idx2.On("HandleLogs", mock.Anything).Return(nil).Run(captureHandledLogs(&handled2))

	coord.RegisterIndexer(idx1)
	coord.RegisterIndexer(idx2)

	err := coord.HandleLogs([]types.Log{log1, log2, log3}, 0, 30)
	require.NoError(t, err)

	// idx1 should get logs from blocks 15 and 25
	assert.Len(t, handled1, 2)
	assert.Contains(t, handled1, log2)
	assert.Contains(t, handled1, log3)

	// idx2 should only get log from block 25
	assert.Len(t, handled2, 1)
	assert.Contains(t, handled2, log3)
}

func TestIndexerCoordinator_HandleLogsDeduplicatesLogPerIndexer(t *testing.T) {
	t.Parallel()

	coord := NewIndexerCoordinator()
	addr := common.HexToAddress("0x1234")
	topic1 := common.HexToHash("0x5678")
	topic2 := common.HexToHash("0x9abc")

	// Log with multiple topics
	logEntry := types.Log{
		Address:     addr,
		Topics:      []common.Hash{topic1, topic2},
		BlockNumber: 1,
	}

	// Indexer interested in the same address with all topics
	idx := mocks.NewIndexer(t)
	idx.EXPECT().GetName().Return("testIndexer")
	idx.EXPECT().StartBlock().Return(uint64(0))
	idx.EXPECT().EventsToIndex().Return(map[common.Address]map[common.Hash]struct{}{
		addr: {}, // All topics
	})

	var handled []types.Log
	callCount := 0
	idx.On("HandleLogs", mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		if logs, ok := args[0].([]types.Log); ok {
			handled = logs
		}
		callCount++
	})

	coord.RegisterIndexer(idx)

	err := coord.HandleLogs([]types.Log{logEntry}, 0, 10)
	require.NoError(t, err)

	// Should only be called once despite matching multiple criteria
	assert.Equal(t, 1, callCount)
	assert.Len(t, handled, 1)
}
