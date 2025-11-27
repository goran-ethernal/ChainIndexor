package store

import (
	"context"
	"os"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/goran-ethernal/ChainIndexor/internal/db"
	"github.com/goran-ethernal/ChainIndexor/internal/downloader/migrations"
	"github.com/goran-ethernal/ChainIndexor/internal/logger"
	"github.com/goran-ethernal/ChainIndexor/pkg/fetcher/store"
	"github.com/stretchr/testify/require"
)

func setupTestLogStore(t *testing.T) (*LogStore, func()) {
	t.Helper()

	// Create temporary database
	tmpFile, err := os.CreateTemp("", "logstore_test_*.db")
	require.NoError(t, err)
	tmpFile.Close()

	dbPath := tmpFile.Name()

	// Create database
	sqlDB, err := db.NewSQLiteDB(dbPath)
	require.NoError(t, err)

	// Run migrations
	err = migrations.RunMigrations(dbPath)
	require.NoError(t, err)

	// Create log store
	store := NewLogStore(sqlDB, logger.GetDefaultLogger())

	cleanup := func() {
		sqlDB.Close()
		os.Remove(dbPath)
	}

	return store, cleanup
}

func createTestLog(address common.Address, blockNumber uint64, txHash common.Hash, logIndex uint) types.Log {
	return types.Log{
		Address:     address,
		Topics:      []common.Hash{common.HexToHash("0x1234"), common.HexToHash("0x5678")},
		Data:        []byte{0x01, 0x02, 0x03},
		BlockNumber: blockNumber,
		BlockHash:   common.HexToHash("0xabcdef"),
		TxHash:      txHash,
		TxIndex:     0,
		Index:       logIndex,
	}
}

func TestLogStore_StoreLogs(t *testing.T) {
	store, cleanup := setupTestLogStore(t)
	defer cleanup()

	ctx := context.Background()
	address := common.HexToAddress("0x1234567890123456789012345678901234567890")

	logs := []types.Log{
		createTestLog(address, 100, common.HexToHash("0xaaa"), 0),
		createTestLog(address, 101, common.HexToHash("0xbbb"), 0),
		createTestLog(address, 102, common.HexToHash("0xccc"), 0),
	}

	topics := []common.Hash{common.HexToHash("0x1234")} // Extract topic0 from test logs
	err := store.StoreLogs(ctx, address, topics, 100, 102, logs)
	require.NoError(t, err)

	// Retrieve logs
	retrievedLogs, coverage, err := store.GetLogs(ctx, address, 100, 102)
	require.NoError(t, err)
	require.Len(t, retrievedLogs, 3)
	require.Len(t, coverage, 1)
	require.Equal(t, uint64(100), coverage[0].FromBlock)
	require.Equal(t, uint64(102), coverage[0].ToBlock)

	// Verify log content
	require.Equal(t, address, retrievedLogs[0].Address)
	require.Equal(t, uint64(100), retrievedLogs[0].BlockNumber)
	require.Equal(t, logs[0].Topics, retrievedLogs[0].Topics)
	require.Equal(t, logs[0].Data, retrievedLogs[0].Data)
}

func TestLogStore_GetLogs_PartialCoverage(t *testing.T) {
	store, cleanup := setupTestLogStore(t)
	defer cleanup()

	ctx := context.Background()
	address := common.HexToAddress("0x1234567890123456789012345678901234567890")

	// Store logs for blocks 100-102
	logs1 := []types.Log{
		createTestLog(address, 100, common.HexToHash("0xaaa"), 0),
		createTestLog(address, 101, common.HexToHash("0xbbb"), 0),
		createTestLog(address, 102, common.HexToHash("0xccc"), 0),
	}
	topics := []common.Hash{common.HexToHash("0x1234")}
	err := store.StoreLogs(ctx, address, topics, 100, 102, logs1)
	require.NoError(t, err)

	// Store logs for blocks 105-107 (gap between 102 and 105)
	logs2 := []types.Log{
		createTestLog(address, 105, common.HexToHash("0xddd"), 0),
		createTestLog(address, 106, common.HexToHash("0xeee"), 0),
		createTestLog(address, 107, common.HexToHash("0xfff"), 0),
	}
	err = store.StoreLogs(ctx, address, topics, 105, 107, logs2)
	require.NoError(t, err)

	// Query range 100-107
	retrievedLogs, coverage, err := store.GetLogs(ctx, address, 100, 107)
	require.NoError(t, err)
	require.Len(t, retrievedLogs, 6)
	require.Len(t, coverage, 2)

	// Verify coverage ranges
	require.Equal(t, uint64(100), coverage[0].FromBlock)
	require.Equal(t, uint64(102), coverage[0].ToBlock)
	require.Equal(t, uint64(105), coverage[1].FromBlock)
	require.Equal(t, uint64(107), coverage[1].ToBlock)
}

func TestLogStore_HandleReorg(t *testing.T) {
	store, cleanup := setupTestLogStore(t)
	defer cleanup()

	ctx := context.Background()
	address := common.HexToAddress("0x1234567890123456789012345678901234567890")

	// Store logs for blocks 100-105
	logs := []types.Log{
		createTestLog(address, 100, common.HexToHash("0xaaa"), 0),
		createTestLog(address, 101, common.HexToHash("0xbbb"), 0),
		createTestLog(address, 102, common.HexToHash("0xccc"), 0),
		createTestLog(address, 103, common.HexToHash("0xddd"), 0),
		createTestLog(address, 104, common.HexToHash("0xeee"), 0),
		createTestLog(address, 105, common.HexToHash("0xfff"), 0),
	}
	topics := []common.Hash{common.HexToHash("0x1234")}
	err := store.StoreLogs(ctx, address, topics, 100, 105, logs)
	require.NoError(t, err)

	// Handle reorg from block 103
	err = store.HandleReorg(ctx, 103)
	require.NoError(t, err)

	// Retrieve logs - should only get blocks 100-102 (103+ are removed)
	retrievedLogs, coverage, err := store.GetLogs(ctx, address, 100, 105)
	require.NoError(t, err)
	require.Len(t, retrievedLogs, 3, "should only have logs for blocks 100-102")
	require.Equal(t, uint64(100), retrievedLogs[0].BlockNumber)
	require.Equal(t, uint64(101), retrievedLogs[1].BlockNumber)
	require.Equal(t, uint64(102), retrievedLogs[2].BlockNumber)

	// Coverage should be truncated to blocks 100-102
	require.Len(t, coverage, 1, "coverage should be truncated to 100-102")
	require.Equal(t, uint64(100), coverage[0].FromBlock)
	require.Equal(t, uint64(102), coverage[0].ToBlock)
}

func TestLogStore_PruneLogsBeforeBlock(t *testing.T) {
	store, cleanup := setupTestLogStore(t)
	defer cleanup()

	ctx := context.Background()
	address := common.HexToAddress("0x1234567890123456789012345678901234567890")

	// Store logs for blocks 100-105
	logs := []types.Log{
		createTestLog(address, 100, common.HexToHash("0xaaa"), 0),
		createTestLog(address, 101, common.HexToHash("0xbbb"), 0),
		createTestLog(address, 102, common.HexToHash("0xccc"), 0),
		createTestLog(address, 103, common.HexToHash("0xddd"), 0),
		createTestLog(address, 104, common.HexToHash("0xeee"), 0),
		createTestLog(address, 105, common.HexToHash("0xfff"), 0),
	}
	topics := []common.Hash{common.HexToHash("0x1234")}
	err := store.StoreLogs(ctx, address, topics, 100, 105, logs)
	require.NoError(t, err)

	// Prune logs before block 103
	err = store.PruneLogsBeforeBlock(ctx, 103)
	require.NoError(t, err)

	// Retrieve logs - should only get blocks 103-105
	retrievedLogs, _, err := store.GetLogs(ctx, address, 100, 105)
	require.NoError(t, err)
	require.Len(t, retrievedLogs, 3, "should only have logs for blocks 103-105")
	require.Equal(t, uint64(103), retrievedLogs[0].BlockNumber)
	require.Equal(t, uint64(104), retrievedLogs[1].BlockNumber)
	require.Equal(t, uint64(105), retrievedLogs[2].BlockNumber)
}

func TestLogStore_MultipleAddresses(t *testing.T) {
	store, cleanup := setupTestLogStore(t)
	defer cleanup()

	ctx := context.Background()
	address1 := common.HexToAddress("0x1111111111111111111111111111111111111111")
	address2 := common.HexToAddress("0x2222222222222222222222222222222222222222")

	// Store logs for address1
	logs1 := []types.Log{
		createTestLog(address1, 100, common.HexToHash("0xaaa"), 0),
		createTestLog(address1, 101, common.HexToHash("0xbbb"), 0),
	}
	topics := []common.Hash{common.HexToHash("0x1234")}
	err := store.StoreLogs(ctx, address1, topics, 100, 101, logs1)
	require.NoError(t, err)

	// Store logs for address2
	logs2 := []types.Log{
		createTestLog(address2, 100, common.HexToHash("0xccc"), 0),
		createTestLog(address2, 101, common.HexToHash("0xddd"), 0),
	}
	err = store.StoreLogs(ctx, address2, topics, 100, 101, logs2)
	require.NoError(t, err)

	// Retrieve logs for address1
	retrievedLogs1, _, err := store.GetLogs(ctx, address1, 100, 101)
	require.NoError(t, err)
	require.Len(t, retrievedLogs1, 2)
	require.Equal(t, address1, retrievedLogs1[0].Address)

	// Retrieve logs for address2
	retrievedLogs2, _, err := store.GetLogs(ctx, address2, 100, 101)
	require.NoError(t, err)
	require.Len(t, retrievedLogs2, 2)
	require.Equal(t, address2, retrievedLogs2[0].Address)
}

func TestLogStore_GetUnsyncedTopics(t *testing.T) {
	store, cleanup := setupTestLogStore(t)
	defer cleanup()

	ctx := context.Background()
	address1 := common.HexToAddress("0x1111111111111111111111111111111111111111")
	address2 := common.HexToAddress("0x2222222222222222222222222222222222222222")
	topic1 := common.HexToHash("0x1234")
	topic2 := common.HexToHash("0x5678")
	topic3 := common.HexToHash("0x9abc")

	// Store logs for address1, topic1, blocks 0-100
	logs1 := []types.Log{
		createTestLog(address1, 50, common.HexToHash("0xaaa"), 0),
	}
	err := store.StoreLogs(ctx, address1, []common.Hash{topic1}, 0, 100, logs1)
	require.NoError(t, err)

	// Store logs for address1, topic2, blocks 0-50 (partial coverage)
	logs2 := []types.Log{
		createTestLog(address1, 25, common.HexToHash("0xbbb"), 0),
	}
	err = store.StoreLogs(ctx, address1, []common.Hash{topic2}, 0, 50, logs2)
	require.NoError(t, err)

	// Check unsynced topics for address1 up to block 100
	// topic1: fully synced (0-100)
	// topic2: partially synced (0-50, missing 51-100)
	// topic3: not synced at all
	addresses := []common.Address{address1, address2}
	topics := [][]common.Hash{
		{topic1, topic2, topic3},
		{topic1},
	}

	unsynced, err := store.GetUnsyncedTopics(ctx, addresses, topics, 100)
	require.NoError(t, err)

	// address1 should have topic2 and topic3 as unsynced
	require.True(t, unsynced.ContainsAddress(address1))
	require.True(t, unsynced.ContainsTopic(address1, topic2))
	require.True(t, unsynced.ContainsTopic(address1, topic3))

	// address2 should have topic1 as unsynced (nothing stored)
	require.True(t, unsynced.ContainsAddress(address2))
	require.True(t, unsynced.ContainsTopic(address2, topic1))
}

func TestLogStore_GetUnsyncedTopics_CompleteCoverage(t *testing.T) {
	store, cleanup := setupTestLogStore(t)
	defer cleanup()

	ctx := context.Background()
	address := common.HexToAddress("0x1111111111111111111111111111111111111111")
	topic := common.HexToHash("0x1234")

	// Store coverage in multiple ranges that together cover 0-100
	err := store.StoreLogs(ctx, address, []common.Hash{topic}, 0, 50, []types.Log{})
	require.NoError(t, err)

	err = store.StoreLogs(ctx, address, []common.Hash{topic}, 51, 100, []types.Log{})
	require.NoError(t, err)

	// Check unsynced topics - should be empty as we have complete coverage
	addresses := []common.Address{address}
	topics := [][]common.Hash{{topic}}

	unsynced, err := store.GetUnsyncedTopics(ctx, addresses, topics, 100)
	require.NoError(t, err)

	// Should not have any unsynced topics
	require.True(t, unsynced.IsEmpty(), "there should be no unsynced topics")
}

func TestLogStore_HandleReorg_ClearsTopicCoverage(t *testing.T) {
	store, cleanup := setupTestLogStore(t)
	defer cleanup()

	ctx := context.Background()
	address := common.HexToAddress("0x1111111111111111111111111111111111111111")
	topic := common.HexToHash("0x1234")

	// Store coverage for blocks 0-100
	logs := []types.Log{
		createTestLog(address, 50, common.HexToHash("0xaaa"), 0),
	}
	err := store.StoreLogs(ctx, address, []common.Hash{topic}, 0, 100, logs)
	require.NoError(t, err)

	// Verify topic is synced
	addresses := []common.Address{address}
	topics := [][]common.Hash{{topic}}
	unsynced, err := store.GetUnsyncedTopics(ctx, addresses, topics, 100)
	require.NoError(t, err)
	require.True(t, unsynced.IsEmpty(), "topic should be fully synced")

	// Handle reorg from block 50
	err = store.HandleReorg(ctx, 50)
	require.NoError(t, err)

	// Now topic should be unsynced from 50-100
	unsynced, err = store.GetUnsyncedTopics(ctx, addresses, topics, 100)
	require.NoError(t, err)
	require.True(t, unsynced.ContainsAddress(address), "should have unsynced topics for 150-200")
	require.True(t, unsynced.ContainsTopic(address, topic), "topic should be unsynced after reorg")
}

func TestLogStore_HandleReorg_TruncatesSpanningRanges(t *testing.T) {
	store, cleanup := setupTestLogStore(t)
	defer cleanup()

	ctx := context.Background()
	address := common.HexToAddress("0x1111111111111111111111111111111111111111")
	topic := common.HexToHash("0x1234")

	// Store coverage in two ranges: 0-100 and 101-200
	logs1 := []types.Log{
		createTestLog(address, 50, common.HexToHash("0xaaa"), 0),
	}
	err := store.StoreLogs(ctx, address, []common.Hash{topic}, 0, 100, logs1)
	require.NoError(t, err)

	logs2 := []types.Log{
		createTestLog(address, 150, common.HexToHash("0xbbb"), 0),
	}
	err = store.StoreLogs(ctx, address, []common.Hash{topic}, 101, 200, logs2)
	require.NoError(t, err)

	// Verify we have two coverage ranges
	_, coverage, err := store.GetLogs(ctx, address, 0, 200)
	require.NoError(t, err)
	require.Len(t, coverage, 2)
	require.Equal(t, uint64(0), coverage[0].FromBlock)
	require.Equal(t, uint64(100), coverage[0].ToBlock)
	require.Equal(t, uint64(101), coverage[1].FromBlock)
	require.Equal(t, uint64(200), coverage[1].ToBlock)

	// Handle reorg at block 150
	err = store.HandleReorg(ctx, 150)
	require.NoError(t, err)

	// After reorg, coverage should be:
	// - 0-100 (unchanged)
	// - 101-149 (truncated from 101-200)
	_, coverage, err = store.GetLogs(ctx, address, 0, 200)
	require.NoError(t, err)
	require.Len(t, coverage, 2, "should have two coverage ranges")
	require.Equal(t, uint64(0), coverage[0].FromBlock)
	require.Equal(t, uint64(100), coverage[0].ToBlock)
	require.Equal(t, uint64(101), coverage[1].FromBlock)
	require.Equal(t, uint64(149), coverage[1].ToBlock, "second range should be truncated to 149")

	// Topic coverage should also be truncated
	addresses := []common.Address{address}
	topics := [][]common.Hash{{topic}}
	unsynced, err := store.GetUnsyncedTopics(ctx, addresses, topics, 200)
	require.NoError(t, err)
	require.True(t, unsynced.ContainsAddress(address), "should have unsynced topics for 150-200")
	require.True(t, unsynced.ContainsTopic(address, topic), "topic should be unsynced after reorg")

	// Re-fetch blocks 150-200
	logs3 := []types.Log{
		createTestLog(address, 175, common.HexToHash("0xccc"), 0),
	}
	err = store.StoreLogs(ctx, address, []common.Hash{topic}, 150, 200, logs3)
	require.NoError(t, err)

	// Now we should have three coverage ranges: 0-100, 101-149, 150-200
	_, coverage, err = store.GetLogs(ctx, address, 0, 200)
	require.NoError(t, err)
	require.Len(t, coverage, 3, "should have three coverage ranges after re-fetch")
	require.Equal(t, uint64(0), coverage[0].FromBlock)
	require.Equal(t, uint64(100), coverage[0].ToBlock)
	require.Equal(t, uint64(101), coverage[1].FromBlock)
	require.Equal(t, uint64(149), coverage[1].ToBlock)
	require.Equal(t, uint64(150), coverage[2].FromBlock)
	require.Equal(t, uint64(200), coverage[2].ToBlock)

	// Topic coverage should now be complete
	unsynced, err = store.GetUnsyncedTopics(ctx, addresses, topics, 200)
	require.NoError(t, err)
	require.True(t, unsynced.IsEmpty(), "all topics should be synced after re-fetch")
}

func TestIsCovered(t *testing.T) {
	tests := []struct {
		name     string
		from     uint64
		to       uint64
		coverage []store.CoverageRange
		expected bool
	}{
		{
			name:     "empty coverage",
			from:     100,
			to:       110,
			coverage: []store.CoverageRange{},
			expected: false,
		},
		{
			name: "fully covered",
			from: 100,
			to:   110,
			coverage: []store.CoverageRange{
				{FromBlock: 90, ToBlock: 120},
			},
			expected: true,
		},
		{
			name: "not covered - range too small",
			from: 100,
			to:   110,
			coverage: []store.CoverageRange{
				{FromBlock: 100, ToBlock: 105},
			},
			expected: false,
		},
		{
			name: "partially covered",
			from: 100,
			to:   110,
			coverage: []store.CoverageRange{
				{FromBlock: 95, ToBlock: 105},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := store.IsCovered(tt.from, tt.to, tt.coverage)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestGetMissingRanges(t *testing.T) {
	tests := []struct {
		name     string
		from     uint64
		to       uint64
		coverage []store.CoverageRange
		expected []store.CoverageRange
	}{
		{
			name:     "empty coverage",
			from:     100,
			to:       110,
			coverage: []store.CoverageRange{},
			expected: []store.CoverageRange{{FromBlock: 100, ToBlock: 110}},
		},
		{
			name: "fully covered",
			from: 100,
			to:   110,
			coverage: []store.CoverageRange{
				{FromBlock: 90, ToBlock: 120},
			},
			expected: nil,
		},
		{
			name: "gap at beginning",
			from: 100,
			to:   110,
			coverage: []store.CoverageRange{
				{FromBlock: 105, ToBlock: 120},
			},
			expected: []store.CoverageRange{
				{FromBlock: 100, ToBlock: 104},
			},
		},
		{
			name: "gap at end",
			from: 100,
			to:   110,
			coverage: []store.CoverageRange{
				{FromBlock: 90, ToBlock: 105},
			},
			expected: []store.CoverageRange{
				{FromBlock: 106, ToBlock: 110},
			},
		},
		{
			name: "gap in middle",
			from: 100,
			to:   120,
			coverage: []store.CoverageRange{
				{FromBlock: 100, ToBlock: 105},
				{FromBlock: 115, ToBlock: 120},
			},
			expected: []store.CoverageRange{
				{FromBlock: 106, ToBlock: 114},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := store.GetMissingRanges(tt.from, tt.to, tt.coverage)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestLogStore_Close(t *testing.T) {
	store, cleanup := setupTestLogStore(t)
	defer cleanup()

	// Close should not return an error
	err := store.Close()
	require.NoError(t, err)
}

func TestLogStore_TopicConversion(t *testing.T) {
	store, cleanup := setupTestLogStore(t)
	defer cleanup()

	ctx := context.Background()
	address := common.HexToAddress("0x1234567890123456789012345678901234567890")

	tests := []struct {
		name   string
		topics []common.Hash
	}{
		{
			name:   "no topics",
			topics: []common.Hash{},
		},
		{
			name:   "one topic",
			topics: []common.Hash{common.HexToHash("0x1111")},
		},
		{
			name: "two topics",
			topics: []common.Hash{
				common.HexToHash("0x1111"),
				common.HexToHash("0x2222"),
			},
		},
		{
			name: "three topics",
			topics: []common.Hash{
				common.HexToHash("0x1111"),
				common.HexToHash("0x2222"),
				common.HexToHash("0x3333"),
			},
		},
		{
			name: "four topics (max)",
			topics: []common.Hash{
				common.HexToHash("0x1111"),
				common.HexToHash("0x2222"),
				common.HexToHash("0x3333"),
				common.HexToHash("0x4444"),
			},
		},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a log with the specified number of topics
			log := types.Log{
				Address:     address,
				Topics:      tt.topics,
				Data:        []byte{0x01, 0x02, 0x03},
				BlockNumber: 100 + uint64(i), // Use different block numbers to avoid conflicts
				BlockHash:   common.HexToHash("0xabcdef"),
				TxHash:      common.HexToHash("0xffffff"),
				TxIndex:     0,
				Index:       0,
			}

			// Store and retrieve the log
			topicFilter := []common.Hash{}
			if len(tt.topics) > 0 {
				topicFilter = []common.Hash{tt.topics[0]}
			}

			err := store.StoreLogs(ctx, address, topicFilter, log.BlockNumber, log.BlockNumber, []types.Log{log})
			require.NoError(t, err)

			// Retrieve and verify topics are preserved correctly
			retrievedLogs, _, err := store.GetLogs(ctx, address, log.BlockNumber, log.BlockNumber)
			require.NoError(t, err)
			require.Len(t, retrievedLogs, 1)
			require.Equal(t, tt.topics, retrievedLogs[0].Topics, "topics should be preserved")
		})
	}
}

func TestLogStore_StoreLogs_EmptyLogs(t *testing.T) {
	store, cleanup := setupTestLogStore(t)
	defer cleanup()

	ctx := context.Background()
	address := common.HexToAddress("0x1234567890123456789012345678901234567890")
	topic := common.HexToHash("0x1234")

	// Store empty logs (important for coverage tracking)
	err := store.StoreLogs(ctx, address, []common.Hash{topic}, 100, 105, []types.Log{})
	require.NoError(t, err)

	// Coverage should still be recorded
	_, coverage, err := store.GetLogs(ctx, address, 100, 105)
	require.NoError(t, err)
	require.Len(t, coverage, 1)
	require.Equal(t, uint64(100), coverage[0].FromBlock)
	require.Equal(t, uint64(105), coverage[0].ToBlock)
}

func TestLogStore_StoreLogs_DuplicateLogs(t *testing.T) {
	store, cleanup := setupTestLogStore(t)
	defer cleanup()

	ctx := context.Background()
	address := common.HexToAddress("0x1234567890123456789012345678901234567890")
	topic := common.HexToHash("0x1234")

	logs := []types.Log{
		createTestLog(address, 100, common.HexToHash("0xaaa"), 0),
		createTestLog(address, 101, common.HexToHash("0xbbb"), 0),
	}

	// Store logs first time
	err := store.StoreLogs(ctx, address, []common.Hash{topic}, 100, 101, logs)
	require.NoError(t, err)

	// Store same logs again (should be ignored due to UNIQUE constraint)
	err = store.StoreLogs(ctx, address, []common.Hash{topic}, 100, 101, logs)
	require.NoError(t, err)

	// Should still only have 2 logs
	retrievedLogs, _, err := store.GetLogs(ctx, address, 100, 101)
	require.NoError(t, err)
	require.Len(t, retrievedLogs, 2)
}

func TestLogStore_MultipleTopics(t *testing.T) {
	store, cleanup := setupTestLogStore(t)
	defer cleanup()

	ctx := context.Background()
	address := common.HexToAddress("0x1234567890123456789012345678901234567890")
	topic1 := common.HexToHash("0x1111")
	topic2 := common.HexToHash("0x2222")

	// Store logs with multiple topics in the filter
	logs := []types.Log{
		createTestLog(address, 100, common.HexToHash("0xaaa"), 0),
	}

	err := store.StoreLogs(ctx, address, []common.Hash{topic1, topic2}, 0, 100, logs)
	require.NoError(t, err)

	// Check that both topics are tracked in coverage
	addresses := []common.Address{address}
	topics := [][]common.Hash{{topic1, topic2}}

	unsynced, err := store.GetUnsyncedTopics(ctx, addresses, topics, 100)
	require.NoError(t, err)

	// Both topics should be synced now
	require.False(t, unsynced.ContainsTopic(address, topic1), "topic1 should be synced")
	require.False(t, unsynced.ContainsTopic(address, topic2), "topic2 should be synced")
}

func TestLogStore_GetLogs_NoCoverage(t *testing.T) {
	store, cleanup := setupTestLogStore(t)
	defer cleanup()

	ctx := context.Background()
	address := common.HexToAddress("0x1234567890123456789012345678901234567890")

	// Query without storing anything
	logs, coverage, err := store.GetLogs(ctx, address, 100, 110)
	require.NoError(t, err)
	require.Len(t, logs, 0)
	require.Len(t, coverage, 0)
}
