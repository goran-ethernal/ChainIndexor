package store

import (
	"context"
	"math/big"
	"os"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/goran-ethernal/ChainIndexor/internal/db"
	"github.com/goran-ethernal/ChainIndexor/internal/logger"
	"github.com/goran-ethernal/ChainIndexor/internal/migrations"
	"github.com/goran-ethernal/ChainIndexor/pkg/config"
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

	// Create log store with proper dbConfig
	dbConfig := config.DatabaseConfig{Path: dbPath}
	store := NewLogStore(sqlDB, logger.GetDefaultLogger(), dbConfig, nil)

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
	err := store.StoreLogs(ctx, []common.Address{address}, [][]common.Hash{topics}, logs, 100, 102, nil)
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
	err := store.StoreLogs(ctx, []common.Address{address}, [][]common.Hash{topics}, logs1, 100, 102, nil)
	require.NoError(t, err)

	// Store logs for blocks 105-107 (gap between 102 and 105)
	logs2 := []types.Log{
		createTestLog(address, 105, common.HexToHash("0xddd"), 0),
		createTestLog(address, 106, common.HexToHash("0xeee"), 0),
		createTestLog(address, 107, common.HexToHash("0xfff"), 0),
	}
	err = store.StoreLogs(ctx, []common.Address{address}, [][]common.Hash{topics}, logs2, 105, 107, nil)
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
	err := store.StoreLogs(ctx, []common.Address{address}, [][]common.Hash{topics}, logs, 100, 105, nil)
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
	err := store.StoreLogs(ctx, []common.Address{address}, [][]common.Hash{topics}, logs, 100, 105, nil)
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
	err := store.StoreLogs(ctx, []common.Address{address1}, [][]common.Hash{topics}, logs1, 100, 101, nil)
	require.NoError(t, err)

	// Store logs for address2
	logs2 := []types.Log{
		createTestLog(address2, 100, common.HexToHash("0xccc"), 0),
		createTestLog(address2, 101, common.HexToHash("0xddd"), 0),
	}
	err = store.StoreLogs(ctx, []common.Address{address2}, [][]common.Hash{topics}, logs2, 100, 101, nil)
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
	err := store.StoreLogs(ctx, []common.Address{address1}, [][]common.Hash{{topic1}}, logs1, 0, 100, nil)
	require.NoError(t, err)

	// Store logs for address1, topic2, blocks 0-50 (partial coverage)
	logs2 := []types.Log{
		createTestLog(address1, 25, common.HexToHash("0xbbb"), 0),
	}
	err = store.StoreLogs(ctx, []common.Address{address1}, [][]common.Hash{{topic2}}, logs2, 0, 50, nil)
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
	err := store.StoreLogs(ctx, []common.Address{address}, [][]common.Hash{{topic}}, []types.Log{}, 0, 50, nil)
	require.NoError(t, err)

	err = store.StoreLogs(ctx, []common.Address{address}, [][]common.Hash{{topic}}, []types.Log{}, 51, 100, nil)
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
	err := store.StoreLogs(ctx, []common.Address{address}, [][]common.Hash{{topic}}, logs, 0, 100, nil)
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
	err := store.StoreLogs(ctx, []common.Address{address}, [][]common.Hash{{topic}}, logs1, 0, 100, nil)
	require.NoError(t, err)

	logs2 := []types.Log{
		createTestLog(address, 150, common.HexToHash("0xbbb"), 0),
	}
	err = store.StoreLogs(ctx, []common.Address{address}, [][]common.Hash{{topic}}, logs2, 101, 200, nil)
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
	err = store.StoreLogs(ctx, []common.Address{address}, [][]common.Hash{{topic}}, logs3, 150, 200, nil)
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

			err := store.StoreLogs(ctx, []common.Address{address}, [][]common.Hash{topicFilter}, []types.Log{log}, log.BlockNumber, log.BlockNumber, nil)
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
	err := store.StoreLogs(ctx, []common.Address{address}, [][]common.Hash{{topic}}, []types.Log{}, 100, 105, nil)
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
	err := store.StoreLogs(ctx, []common.Address{address}, [][]common.Hash{{topic}}, logs, 100, 101, nil)
	require.NoError(t, err)

	// Store same logs again (should be ignored due to UNIQUE constraint)
	err = store.StoreLogs(ctx, []common.Address{address}, [][]common.Hash{{topic}}, logs, 100, 101, nil)
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

	err := store.StoreLogs(ctx, []common.Address{address}, [][]common.Hash{{topic1, topic2}}, logs, 0, 100, nil)
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

func TestLogStore_CalculateBlocksToFreeSpace(t *testing.T) {
	store, cleanup := setupTestLogStore(t)
	defer cleanup()

	ctx := context.Background()
	address1 := common.HexToAddress("0x1111111111111111111111111111111111111111")
	address2 := common.HexToAddress("0x2222222222222222222222222222222222222222")
	topic1 := common.HexToHash("0xaaaa")
	topic2 := common.HexToHash("0xbbbb")

	// Create a substantial dataset with multiple addresses and topics
	// We'll create logs for many blocks with varying density
	const (
		startBlock = 1000
		endBlock   = 5000 // Increased from 2000 to create more data
		chunkSize  = 100
	)

	// Store logs in chunks to simulate realistic usage
	for blockStart := startBlock; blockStart < endBlock; blockStart += chunkSize {
		blockEnd := min(blockStart+chunkSize-1, endBlock)

		var logs []types.Log
		for block := blockStart; block <= blockEnd; block++ {
			// Create varying number of logs per block (1-11 logs)
			numLogs := int((block % 10) + 1)
			for i := range numLogs {
				// Alternate between addresses
				addr := address1
				if block%2 == 0 {
					addr = address2
				}

				// Create larger data payloads to increase DB size
				dataSize := 200 + (i * 50) // 200-700 bytes per log
				data := make([]byte, dataSize)
				for j := range dataSize {
					data[j] = byte(j % 256)
				}

				log := types.Log{
					Address:     addr,
					Topics:      []common.Hash{topic1, topic2},
					Data:        data,
					BlockNumber: uint64(block),
					BlockHash:   common.BigToHash(big.NewInt(int64(block))),
					TxHash:      common.BigToHash(big.NewInt(int64(block*100 + i))),
					TxIndex:     uint(i),
					Index:       uint(i),
				}
				logs = append(logs, log)
			}
		}

		// Store logs for both addresses with both topics
		err := store.StoreLogs(ctx,
			[]common.Address{address1, address2},
			[][]common.Hash{{topic1, topic2}, {topic1, topic2}},
			logs,
			uint64(blockStart),
			uint64(blockEnd),
			nil,
		)
		require.NoError(t, err)
	}

	// Get initial database size in bytes for more precision
	initialSizeBytes, err := db.DBTotalSize(store.db, store.dbConfig.Path)
	require.NoError(t, err)
	initialSize := uint64(initialSizeBytes) / (1024 * 1024) // Convert to MB
	t.Logf("Initial database size: %d MB (%d bytes)", initialSize, initialSizeBytes)
	require.Greater(t, initialSizeBytes, int64(0), "database should have measurable size")

	// Get counts for verification
	var eventLogCount, logCoverageCount, topicCoverageCount int64
	err = store.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM event_logs").Scan(&eventLogCount)
	require.NoError(t, err)
	err = store.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM log_coverage").Scan(&logCoverageCount)
	require.NoError(t, err)
	err = store.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM topic_coverage").Scan(&topicCoverageCount)
	require.NoError(t, err)

	t.Logf("Row counts - event_logs: %d, log_coverage: %d, topic_coverage: %d",
		eventLogCount, logCoverageCount, topicCoverageCount)

	// Test Case 1: Calculate blocks to free to reduce size by ~30%
	// For small databases, work with what we have
	targetSize := initialSize * 7 / 10 // 70% of current size
	if targetSize >= initialSize || initialSize == 0 {
		// If database is < 1 MB, use bytes for calculation
		targetSize = uint64(initialSizeBytes) * 7 / 10 / (1024 * 1024)
		if targetSize >= initialSize {
			targetSize = 0 // Force to free at least something
		}
	}

	pruneBlock, err := store.calculateBlocksToFreeSpace(ctx, initialSize, targetSize)
	require.NoError(t, err)
	require.Greater(t, pruneBlock, uint64(startBlock), "prune block should be greater than start block")
	require.Less(t, pruneBlock, uint64(endBlock), "prune block should be less than end block")

	t.Logf("To reduce from %d MB to %d MB, prune before block %d", initialSize, targetSize, pruneBlock)

	// Actually prune and verify the space freed
	sizeBefore := initialSize
	sizeBeforeBytes := initialSizeBytes

	t.Logf("Before prune - size: %d bytes", sizeBeforeBytes)

	blocksPruned, err := store.pruneLogsBeforeBlock(ctx, pruneBlock)
	require.NoError(t, err)
	require.Greater(t, blocksPruned, uint64(0), "should have pruned some blocks")

	// Wait a moment for filesystem to sync
	sizeAfterBytes, err := db.DBTotalSize(store.db, store.dbConfig.Path)
	require.NoError(t, err)
	sizeAfter := uint64(sizeAfterBytes) / (1024 * 1024)

	t.Logf("After prune - size: %d bytes (before: %d bytes)", sizeAfterBytes, sizeBeforeBytes)

	var spaceFreed, spaceFreedBytes int64
	if sizeBeforeBytes > sizeAfterBytes {
		spaceFreedBytes = sizeBeforeBytes - sizeAfterBytes
		if sizeBefore > sizeAfter {
			spaceFreed = int64(sizeBefore - sizeAfter)
		}
	}

	t.Logf("Pruned %d blocks, freed %d MB (%d bytes)", blocksPruned, spaceFreed, spaceFreedBytes)

	// Verify counts after pruning
	var eventLogCountAfter, logCoverageCountAfter, topicCoverageCountAfter int64
	err = store.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM event_logs").Scan(&eventLogCountAfter)
	require.NoError(t, err)
	err = store.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM log_coverage").Scan(&logCoverageCountAfter)
	require.NoError(t, err)
	err = store.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM topic_coverage").Scan(&topicCoverageCountAfter)
	require.NoError(t, err)

	t.Logf("After prune - event_logs: %d (-%d), log_coverage: %d (-%d), topic_coverage: %d (-%d)",
		eventLogCountAfter, eventLogCount-eventLogCountAfter,
		logCoverageCountAfter, logCoverageCount-logCoverageCountAfter,
		topicCoverageCountAfter, topicCoverageCount-topicCoverageCountAfter)

	// Verify that data was actually deleted from all tables
	require.Less(t, eventLogCountAfter, eventLogCount, "event_logs count should decrease")
	require.LessOrEqual(t, logCoverageCountAfter, logCoverageCount, "log_coverage count should decrease or stay same")
	require.LessOrEqual(t, topicCoverageCountAfter, topicCoverageCount, "topic_coverage count should decrease or stay same")

	// Verify we deleted the expected proportion of rows
	eventLogsDeleted := eventLogCount - eventLogCountAfter
	blocksToDelete := pruneBlock - uint64(startBlock)
	totalBlocks := uint64(endBlock - startBlock)
	expectedEventLogsDeleted := float64(eventLogCount) * float64(blocksToDelete) / float64(totalBlocks)
	deletionAccuracy := float64(eventLogsDeleted) / expectedEventLogsDeleted * 100
	t.Logf("Deletion accuracy: deleted %d event_logs out of %d, expected ~%.0f (%.1f%% accurate)",
		eventLogsDeleted, eventLogCount, expectedEventLogsDeleted, deletionAccuracy)

	// The deletion should be within reasonable bounds (within 100% of expected)
	require.Greater(t, eventLogsDeleted, int64(0), "should have deleted some event_logs")
	require.InDelta(t, expectedEventLogsDeleted, float64(eventLogsDeleted), expectedEventLogsDeleted,
		"deleted rows should be within 100%% of expected")

	// Note: In WAL mode, VACUUM doesn't immediately reclaim disk space.
	// The space is reused for future inserts, but the file doesn't shrink.
	// This is expected SQLite WAL behavior and doesn't indicate a bug.
	// The important thing is that rows are deleted and space will be reused.
	if spaceFreedBytes > 0 {
		t.Logf("Space freed: %d bytes", spaceFreedBytes)
	} else if sizeAfterBytes > sizeBeforeBytes {
		t.Logf("File size increased by %d bytes (expected in WAL mode during VACUUM - space will be reused)",
			sizeAfterBytes-sizeBeforeBytes)
	} else {
		t.Logf("File size unchanged (expected in WAL mode - deleted space will be reused for future writes)")
	}

	// Calculate accuracy metrics based on the calculation algorithm
	targetFreedBytes := sizeBeforeBytes - int64(targetSize*1024*1024)
	if targetFreedBytes > 0 {
		t.Logf("Target was to free %d bytes to reach %d MB target size", targetFreedBytes, targetSize)
	} // Test Case 2: Verify remaining data is correct
	// All logs before pruneBlock should be gone
	var logsBeforePrune int64
	err = store.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM event_logs WHERE block_number < ?",
		pruneBlock).Scan(&logsBeforePrune)
	require.NoError(t, err)
	require.Equal(t, int64(0), logsBeforePrune, "no logs should remain before prune block")

	// Some logs after pruneBlock should remain
	var logsAfterPrune int64
	err = store.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM event_logs WHERE block_number >= ?",
		pruneBlock).Scan(&logsAfterPrune)
	require.NoError(t, err)
	require.Greater(t, logsAfterPrune, int64(0), "logs should remain after prune block")

	// Test Case 3: Verify coverage tables were also pruned
	var coverageBeforePrune int64
	err = store.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM log_coverage WHERE to_block < ?",
		pruneBlock).Scan(&coverageBeforePrune)
	require.NoError(t, err)
	require.Equal(t, int64(0), coverageBeforePrune, "no log_coverage should remain before prune block")

	var topicCoverageBeforePrune int64
	err = store.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM topic_coverage WHERE to_block < ?",
		pruneBlock).Scan(&topicCoverageBeforePrune)
	require.NoError(t, err)
	require.Equal(t, int64(0), topicCoverageBeforePrune, "no topic_coverage should remain before prune block")

	// Test Case 4: Edge case - try to free more space than available (should prune most/all data)
	currentSize := uint64(sizeAfterBytes) / (1024 * 1024)
	pruneBlock2, err := store.calculateBlocksToFreeSpace(ctx, currentSize, 0)
	require.NoError(t, err)
	require.Greater(t, pruneBlock2, uint64(0), "should calculate a prune block even for full database deletion")
	t.Logf("To free entire database (%d MB), would prune before block: %d", currentSize, pruneBlock2)
}

func TestLogStore_RetentionPolicy(t *testing.T) {
	t.Run("MaxBlocksFromFinalized", func(t *testing.T) {
		// Setup store with retention policy
		tmpFile, err := os.CreateTemp("", "logstore_retention_blocks_*.db")
		require.NoError(t, err)
		tmpFile.Close()
		dbPath := tmpFile.Name()
		defer os.Remove(dbPath)

		sqlDB, err := db.NewSQLiteDB(dbPath)
		require.NoError(t, err)
		defer sqlDB.Close()

		err = migrations.RunMigrations(dbPath)
		require.NoError(t, err)

		// Retention policy: keep only 100 blocks from finalized
		retentionPolicy := &config.RetentionPolicyConfig{
			MaxBlocksFromFinalized: 100,
			MaxDBSizeMB:            0, // disabled
		}

		dbConfig := config.DatabaseConfig{Path: dbPath}
		store := NewLogStore(sqlDB, logger.GetDefaultLogger(), dbConfig, retentionPolicy)

		ctx := context.Background()
		address1 := common.HexToAddress("0x1111111111111111111111111111111111111111")
		address2 := common.HexToAddress("0x2222222222222222222222222222222222222222")
		topic1 := common.HexToHash("0xaaaa")
		topic2 := common.HexToHash("0xbbbb")

		// Store logs across a wide block range: 1000-1500 (500 blocks)
		var allLogs []types.Log
		for block := uint64(1000); block < 1500; block++ {
			// 2 addresses, each with 2 logs per block
			allLogs = append(allLogs,
				createTestLog(address1, block, common.BytesToHash([]byte{byte(block), 0x01}), 0),
				createTestLog(address1, block, common.BytesToHash([]byte{byte(block), 0x02}), 1),
				createTestLog(address2, block, common.BytesToHash([]byte{byte(block), 0x03}), 2),
				createTestLog(address2, block, common.BytesToHash([]byte{byte(block), 0x04}), 3),
			)
		}

		// Store in chunks to simulate real usage
		chunkSize := 100
		for i := 0; i < len(allLogs); i += chunkSize * 4 { // 4 logs per block
			end := i + chunkSize*4
			if end > len(allLogs) {
				end = len(allLogs)
			}
			chunk := allLogs[i:end]
			fromBlock := chunk[0].BlockNumber
			toBlock := chunk[len(chunk)-1].BlockNumber

			err = store.storeLogsInternal(ctx,
				[]common.Address{address1, address2},
				[][]common.Hash{{topic1}, {topic2}},
				chunk,
				fromBlock,
				toBlock,
			)
			require.NoError(t, err)
		}

		// Verify all logs are stored
		var totalLogsBefore int64
		err = sqlDB.QueryRow("SELECT COUNT(*) FROM event_logs").Scan(&totalLogsBefore)
		require.NoError(t, err)
		require.Equal(t, int64(2000), totalLogsBefore) // 500 blocks * 4 logs/block

		t.Logf("Initial logs stored: %d", totalLogsBefore)

		// Simulate finalized block at 1400
		// With MaxBlocksFromFinalized=100, we should prune everything before block 1300
		finalizedBlock := &types.Header{
			Number: big.NewInt(1400),
		}

		// Apply retention policy
		err = store.applyRetentionIfNeeded(ctx, finalizedBlock)
		require.NoError(t, err)

		// Verify pruning occurred
		var totalLogsAfter, minBlock, maxBlock int64
		err = sqlDB.QueryRow("SELECT COUNT(*), MIN(block_number), MAX(block_number) FROM event_logs").
			Scan(&totalLogsAfter, &minBlock, &maxBlock)
		require.NoError(t, err)

		t.Logf("After retention - logs: %d, block range: %d-%d", totalLogsAfter, minBlock, maxBlock)

		// Should have deleted blocks 1000-1299 (300 blocks * 4 logs = 1200 logs)
		// Should keep blocks 1300-1499 (200 blocks * 4 logs = 800 logs)
		require.Less(t, totalLogsAfter, totalLogsBefore, "should have pruned some logs")
		require.GreaterOrEqual(t, minBlock, int64(1300), "oldest block should be >= 1300")
		require.Equal(t, int64(1499), maxBlock, "newest block should still be 1499")

		// Verify coverage was also pruned
		var coverageCount int64
		err = sqlDB.QueryRow("SELECT COUNT(*) FROM log_coverage WHERE to_block < 1300").Scan(&coverageCount)
		require.NoError(t, err)
		require.Equal(t, int64(0), coverageCount, "old coverage should be deleted")

		var topicCoverageCount int64
		err = sqlDB.QueryRow("SELECT COUNT(*) FROM topic_coverage WHERE to_block < 1300").Scan(&topicCoverageCount)
		require.NoError(t, err)
		require.Equal(t, int64(0), topicCoverageCount, "old topic coverage should be deleted")
	})

	t.Run("MaxDBSizeMB", func(t *testing.T) {
		// Setup store with size-based retention policy
		tmpFile, err := os.CreateTemp("", "logstore_retention_size_*.db")
		require.NoError(t, err)
		tmpFile.Close()
		dbPath := tmpFile.Name()
		defer os.Remove(dbPath)

		sqlDB, err := db.NewSQLiteDB(dbPath)
		require.NoError(t, err)
		defer sqlDB.Close()

		err = migrations.RunMigrations(dbPath)
		require.NoError(t, err)

		// Retention policy: limit database to 5 MB
		retentionPolicy := &config.RetentionPolicyConfig{
			MaxBlocksFromFinalized: 0, // disabled
			MaxDBSizeMB:            5,
		}

		dbConfig := config.DatabaseConfig{Path: dbPath}
		store := NewLogStore(sqlDB, logger.GetDefaultLogger(), dbConfig, retentionPolicy)

		ctx := context.Background()
		address := common.HexToAddress("0x1111111111111111111111111111111111111111")
		topic := common.HexToHash("0xaaaa")

		// Store enough logs to exceed 5 MB
		// Each log is ~200-300 bytes, so ~20,000 logs should be ~4-6 MB
		var allLogs []types.Log
		for block := uint64(1000); block < 6000; block++ {
			for i := uint(0); i < 4; i++ {
				log := createTestLog(address, block, common.BytesToHash([]byte{byte(block), byte(i)}), i)
				// Add some data to make logs bigger
				log.Data = make([]byte, 100)
				for j := range log.Data {
					log.Data[j] = byte(block + uint64(i) + uint64(j))
				}
				allLogs = append(allLogs, log)
			}
		}

		// Store in chunks
		chunkSize := 500
		for i := 0; i < len(allLogs); i += chunkSize {
			end := i + chunkSize
			if end > len(allLogs) {
				end = len(allLogs)
			}
			chunk := allLogs[i:end]
			fromBlock := chunk[0].BlockNumber
			toBlock := chunk[len(chunk)-1].BlockNumber

			err = store.storeLogsInternal(ctx,
				[]common.Address{address},
				[][]common.Hash{{topic}},
				chunk,
				fromBlock,
				toBlock,
			)
			require.NoError(t, err)
		}

		// Get initial size
		sizeBefore, err := store.getDatabaseSizeMB()
		require.NoError(t, err)
		t.Logf("Initial database size: %d MB", sizeBefore)

		// Verify we exceeded the limit
		require.Greater(t, sizeBefore, uint64(5), "database should exceed 5 MB limit")

		// Simulate finalized block
		finalizedBlock := &types.Header{
			Number: big.NewInt(6000),
		}

		// Apply retention policy - should trigger size-based pruning
		err = store.applyRetentionIfNeeded(ctx, finalizedBlock)
		require.NoError(t, err)

		// Get size after pruning
		sizeAfter, err := store.getDatabaseSizeMB()
		require.NoError(t, err)
		t.Logf("After retention - database size: %d MB (before: %d MB)", sizeAfter, sizeBefore)

		// Should have reduced size (may not be exactly 5 MB due to estimation, but should be closer)
		require.Less(t, sizeAfter, sizeBefore, "database size should decrease after pruning")

		// Verify some logs were deleted
		var totalLogsAfter int64
		err = sqlDB.QueryRow("SELECT COUNT(*) FROM event_logs").Scan(&totalLogsAfter)
		require.NoError(t, err)
		require.Less(t, totalLogsAfter, int64(len(allLogs)), "should have pruned some logs")
		t.Logf("Logs after retention: %d (before: %d)", totalLogsAfter, len(allLogs))
	})

	t.Run("CombinedPolicy", func(t *testing.T) {
		// Test both policies active - should use whichever is more aggressive
		tmpFile, err := os.CreateTemp("", "logstore_retention_combined_*.db")
		require.NoError(t, err)
		tmpFile.Close()
		dbPath := tmpFile.Name()
		defer os.Remove(dbPath)

		sqlDB, err := db.NewSQLiteDB(dbPath)
		require.NoError(t, err)
		defer sqlDB.Close()

		err = migrations.RunMigrations(dbPath)
		require.NoError(t, err)

		retentionPolicy := &config.RetentionPolicyConfig{
			MaxBlocksFromFinalized: 200, // keep 200 blocks
			MaxDBSizeMB:            3,   // limit to 3 MB
		}

		dbConfig := config.DatabaseConfig{Path: dbPath}
		store := NewLogStore(sqlDB, logger.GetDefaultLogger(), dbConfig, retentionPolicy)

		ctx := context.Background()
		address := common.HexToAddress("0x1111111111111111111111111111111111111111")
		topic := common.HexToHash("0xaaaa")

		// Store logs with large data to quickly hit size limit
		var allLogs []types.Log
		for block := uint64(1000); block < 3000; block++ {
			for i := uint(0); i < 3; i++ {
				log := createTestLog(address, block, common.BytesToHash([]byte{byte(block), byte(i)}), i)
				log.Data = make([]byte, 200)
				allLogs = append(allLogs, log)
			}
		}

		// Store all logs
		chunkSize := 300
		for i := 0; i < len(allLogs); i += chunkSize {
			end := i + chunkSize
			if end > len(allLogs) {
				end = len(allLogs)
			}
			chunk := allLogs[i:end]
			fromBlock := chunk[0].BlockNumber
			toBlock := chunk[len(chunk)-1].BlockNumber

			err = store.storeLogsInternal(ctx,
				[]common.Address{address},
				[][]common.Hash{{topic}},
				chunk,
				fromBlock,
				toBlock,
			)
			require.NoError(t, err)
		}

		sizeBefore, err := store.getDatabaseSizeMB()
		require.NoError(t, err)
		t.Logf("Initial database size: %d MB", sizeBefore)

		// Finalized at block 2900
		// Block policy: keep from 2700+ (2900 - 200)
		// Size policy: likely more aggressive if DB > 3 MB
		finalizedBlock := &types.Header{
			Number: big.NewInt(2900),
		}

		err = store.applyRetentionIfNeeded(ctx, finalizedBlock)
		require.NoError(t, err)

		var minBlock, totalLogs int64
		err = sqlDB.QueryRow("SELECT MIN(block_number), COUNT(*) FROM event_logs").
			Scan(&minBlock, &totalLogs)
		require.NoError(t, err)

		sizeAfter, err := store.getDatabaseSizeMB()
		require.NoError(t, err)

		t.Logf("After retention - size: %d MB, logs: %d, min block: %d",
			sizeAfter, totalLogs, minBlock)

		// Should have applied whichever policy was more aggressive
		require.Less(t, sizeAfter, sizeBefore, "size should decrease")
		require.Less(t, totalLogs, int64(len(allLogs)), "should have pruned logs")

		// If size policy was more aggressive, minBlock will be > 2700
		// If block policy was more aggressive, minBlock should be around 2700
		require.Greater(t, minBlock, int64(1000), "should have pruned old blocks")
	})
}
