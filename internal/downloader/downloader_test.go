package downloader

import (
	"fmt"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/goran-ethernal/ChainIndexor/internal/logger"
	"github.com/goran-ethernal/ChainIndexor/pkg/config"
	"github.com/goran-ethernal/ChainIndexor/pkg/indexer"
	"github.com/stretchr/testify/require"
)

// mockIndexer implements the indexer.Indexer interface for testing
type mockIndexer struct {
	eventsToIndex map[common.Address]map[common.Hash]struct{}
}

func (m *mockIndexer) EventsToIndex() map[common.Address]map[common.Hash]struct{} {
	return m.eventsToIndex
}

func (m *mockIndexer) HandleLogs(logs []types.Log) error {
	return nil
}

func (m *mockIndexer) HandleReorg(blockNum uint64) error {
	return nil
}

func TestDownloaderCreation(t *testing.T) {
	log, err := logger.NewLogger("info", true)
	require.NoError(t, err)

	// Create a temporary database for SyncManager
	tmpDB := t.TempDir() + "/test_downloader.db"
	sm, err := NewSyncManager(tmpDB, log)
	require.NoError(t, err)
	defer sm.Close()

	// We can't test with nil for these since New() checks them in order
	// Just verify the constructor validates required fields exist
	// In a real test, we'd use mocks for RPC and ReorgDetector
}

func TestIndexerRegistration(t *testing.T) {
	log, err := logger.NewLogger("info", true)
	require.NoError(t, err)

	// Create a temporary database for SyncManager
	tmpDB := t.TempDir() + "/test_downloader_indexer.db"
	sm, err := NewSyncManager(tmpDB, log)
	require.NoError(t, err)
	defer sm.Close()

	// Create downloader (without RPC/ReorgDetector for this unit test)
	cfg := config.DownloaderConfig{
		ChunkSize: 5000,
		Finality:  "finalized",
	}

	d := &Downloader{
		cfg:         cfg,
		syncManager: sm,
		log:         log.WithComponent("downloader"),
		coordinator: indexer.NewIndexerCoordinator(),
		addresses:   make([]common.Address, 0),
		topics:      make([][]common.Hash, 0),
	}

	// Create first mock indexer
	addr1 := common.HexToAddress("0x1234567890abcdef1234567890abcdef12345678")
	topic1 := common.HexToHash("0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef") // Transfer event

	mock1 := &mockIndexer{
		eventsToIndex: map[common.Address]map[common.Hash]struct{}{
			addr1: {topic1: {}},
		},
	}

	// Create second mock indexer with different address and topic
	addr2 := common.HexToAddress("0xabcdefabcdefabcdefabcdefabcdefabcdefabcd")
	topic2 := common.HexToHash("0x8c5be1e5ebec7d5bd14f71427d1e84f3dd0314c0f7b2291e5b200ac8c7c3b925") // Approval event

	mock2 := &mockIndexer{
		eventsToIndex: map[common.Address]map[common.Hash]struct{}{
			addr2: {topic2: {}},
		},
	}

	// Create third mock indexer with same address as first but different topic
	topic3 := common.HexToHash("0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef") // Custom event

	mock3 := &mockIndexer{
		eventsToIndex: map[common.Address]map[common.Hash]struct{}{
			addr1: {topic3: {}}, // Same address as mock1
		},
	}

	// Register all indexers
	d.RegisterIndexer(mock1)
	d.RegisterIndexer(mock2)
	d.RegisterIndexer(mock3)

	// Verify addresses were collected (should have 2 unique addresses)
	require.Len(t, d.addresses, 2, "should have 2 unique addresses")
	require.Contains(t, d.addresses, addr1)
	require.Contains(t, d.addresses, addr2)

	// Verify topics were accumulated per address
	// d.topics should have 2 arrays (one per address)
	require.Len(t, d.topics, 2, "should have two topic arrays (one per address)")

	// Find which index corresponds to each address
	addr1Index := -1
	addr2Index := -1
	for i, addr := range d.addresses {
		switch addr {
		case addr1:
			addr1Index = i
		case addr2:
			addr2Index = i
		}
	}

	// addr1 should have topics from mock1 and mock3 (topic1 and topic3)
	require.Len(t, d.topics[addr1Index], 2, "addr1 should have 2 topics")
	require.Contains(t, d.topics[addr1Index], topic1)
	require.Contains(t, d.topics[addr1Index], topic3)

	// addr2 should have topic from mock2 (topic2)
	require.Len(t, d.topics[addr2Index], 1, "addr2 should have 1 topic")
	require.Contains(t, d.topics[addr2Index], topic2)
}

func TestReorgErrorDetection(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		isReorg bool
	}{
		{
			name:    "nil error",
			err:     nil,
			isReorg: false,
		},
		{
			name:    "reorg detected error",
			err:     fmt.Errorf("reorg detected at block 100"),
			isReorg: true,
		},
		{
			name:    "chain discontinuity error",
			err:     fmt.Errorf("chain discontinuity detected"),
			isReorg: true,
		},
		{
			name:    "reorganization error",
			err:     fmt.Errorf("blockchain reorganization occurred"),
			isReorg: true,
		},
		{
			name:    "other error",
			err:     fmt.Errorf("connection timeout"),
			isReorg: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isReorgError(tt.err)
			require.Equal(t, tt.isReorg, result)
		})
	}
}
