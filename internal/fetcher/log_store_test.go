package fetcher

import (
	"context"
	"os"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/goran-ethernal/ChainIndexor/internal/db"
	"github.com/goran-ethernal/ChainIndexor/internal/downloader/migrations"
	"github.com/goran-ethernal/ChainIndexor/internal/logger"
	"github.com/stretchr/testify/require"
)

func setupTestLogStore(t *testing.T) (*SQLiteLogStore, func()) {
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
	store := NewSQLiteLogStore(sqlDB, logger.GetDefaultLogger())

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
		Removed:     false,
	}
}

func TestSQLiteLogStore_StoreLogs(t *testing.T) {
	store, cleanup := setupTestLogStore(t)
	defer cleanup()

	ctx := context.Background()
	address := common.HexToAddress("0x1234567890123456789012345678901234567890")

	logs := []types.Log{
		createTestLog(address, 100, common.HexToHash("0xaaa"), 0),
		createTestLog(address, 101, common.HexToHash("0xbbb"), 0),
		createTestLog(address, 102, common.HexToHash("0xccc"), 0),
	}

	err := store.StoreLogs(ctx, address, 100, 102, logs)
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

func TestSQLiteLogStore_GetLogs_PartialCoverage(t *testing.T) {
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
	err := store.StoreLogs(ctx, address, 100, 102, logs1)
	require.NoError(t, err)

	// Store logs for blocks 105-107 (gap between 102 and 105)
	logs2 := []types.Log{
		createTestLog(address, 105, common.HexToHash("0xddd"), 0),
		createTestLog(address, 106, common.HexToHash("0xeee"), 0),
		createTestLog(address, 107, common.HexToHash("0xfff"), 0),
	}
	err = store.StoreLogs(ctx, address, 105, 107, logs2)
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

func TestSQLiteLogStore_HandleReorg(t *testing.T) {
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
	err := store.StoreLogs(ctx, address, 100, 105, logs)
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

	// Coverage should be cleared for blocks >= 103
	require.Len(t, coverage, 0, "coverage should be empty after reorg")
}

func TestSQLiteLogStore_PruneLogsBeforeBlock(t *testing.T) {
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
	err := store.StoreLogs(ctx, address, 100, 105, logs)
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

func TestSQLiteLogStore_MultipleAddresses(t *testing.T) {
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
	err := store.StoreLogs(ctx, address1, 100, 101, logs1)
	require.NoError(t, err)

	// Store logs for address2
	logs2 := []types.Log{
		createTestLog(address2, 100, common.HexToHash("0xccc"), 0),
		createTestLog(address2, 101, common.HexToHash("0xddd"), 0),
	}
	err = store.StoreLogs(ctx, address2, 100, 101, logs2)
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

func TestIsCovered(t *testing.T) {
	tests := []struct {
		name     string
		from     uint64
		to       uint64
		coverage []CoverageRange
		expected bool
	}{
		{
			name:     "empty coverage",
			from:     100,
			to:       110,
			coverage: []CoverageRange{},
			expected: false,
		},
		{
			name: "fully covered",
			from: 100,
			to:   110,
			coverage: []CoverageRange{
				{FromBlock: 90, ToBlock: 120},
			},
			expected: true,
		},
		{
			name: "not covered - range too small",
			from: 100,
			to:   110,
			coverage: []CoverageRange{
				{FromBlock: 100, ToBlock: 105},
			},
			expected: false,
		},
		{
			name: "partially covered",
			from: 100,
			to:   110,
			coverage: []CoverageRange{
				{FromBlock: 95, ToBlock: 105},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsCovered(tt.from, tt.to, tt.coverage)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestGetMissingRanges(t *testing.T) {
	tests := []struct {
		name     string
		from     uint64
		to       uint64
		coverage []CoverageRange
		expected []CoverageRange
	}{
		{
			name:     "empty coverage",
			from:     100,
			to:       110,
			coverage: []CoverageRange{},
			expected: []CoverageRange{{FromBlock: 100, ToBlock: 110}},
		},
		{
			name: "fully covered",
			from: 100,
			to:   110,
			coverage: []CoverageRange{
				{FromBlock: 90, ToBlock: 120},
			},
			expected: nil,
		},
		{
			name: "gap at beginning",
			from: 100,
			to:   110,
			coverage: []CoverageRange{
				{FromBlock: 105, ToBlock: 120},
			},
			expected: []CoverageRange{
				{FromBlock: 100, ToBlock: 104},
			},
		},
		{
			name: "gap at end",
			from: 100,
			to:   110,
			coverage: []CoverageRange{
				{FromBlock: 90, ToBlock: 105},
			},
			expected: []CoverageRange{
				{FromBlock: 106, ToBlock: 110},
			},
		},
		{
			name: "gap in middle",
			from: 100,
			to:   120,
			coverage: []CoverageRange{
				{FromBlock: 100, ToBlock: 105},
				{FromBlock: 115, ToBlock: 120},
			},
			expected: []CoverageRange{
				{FromBlock: 106, ToBlock: 114},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetMissingRanges(tt.from, tt.to, tt.coverage)
			require.Equal(t, tt.expected, result)
		})
	}
}
