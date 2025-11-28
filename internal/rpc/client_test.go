package rpc

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	pkgrpc "github.com/goran-ethernal/ChainIndexor/pkg/rpc"
	"github.com/stretchr/testify/require"
)

// TestClientImplementsInterface verifies that Client implements the EthClient interface.
func TestClientImplementsInterface(t *testing.T) {
	// This test ensures compile-time interface compliance is maintained
	var _ pkgrpc.EthClient = (*Client)(nil)
}

func TestToBlockNumArg(t *testing.T) {
	tests := []struct {
		name     string
		blockNum uint64
		want     string
	}{
		{
			name:     "block 0",
			blockNum: 0,
			want:     "0x0",
		},
		{
			name:     "block 1",
			blockNum: 1,
			want:     "0x1",
		},
		{
			name:     "block 100",
			blockNum: 100,
			want:     "0x64",
		},
		{
			name:     "block 1000",
			blockNum: 1000,
			want:     "0x3e8",
		},
		{
			name:     "large block number",
			blockNum: 18000000,
			want:     "0x112a880",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := toBlockNumArg(tt.blockNum)
			require.Equal(t, tt.want, result)
		})
	}
}

func TestToFilterArg(t *testing.T) {
	addr1 := common.HexToAddress("0x1234567890123456789012345678901234567890")
	addr2 := common.HexToAddress("0xabcdefabcdefabcdefabcdefabcdefabcdefabcd")
	blockHash := common.HexToHash("0xdeadbeef")
	topic1 := common.HexToHash("0x1111111111111111111111111111111111111111111111111111111111111111")
	topic2 := common.HexToHash("0x2222222222222222222222222222222222222222222222222222222222222222")

	tests := []struct {
		name  string
		query ethereum.FilterQuery
		check func(t *testing.T, result any)
	}{
		{
			name: "query with single address and block range",
			query: ethereum.FilterQuery{
				FromBlock: big.NewInt(100),
				ToBlock:   big.NewInt(200),
				Addresses: []common.Address{addr1},
				Topics:    [][]common.Hash{{topic1}},
			},
			check: func(t *testing.T, result any) {
				t.Helper()
				m, ok := result.(map[string]any)
				require.True(t, ok, "result should be a map[string]any")
				require.Equal(t, "0x64", m["fromBlock"])
				require.Equal(t, "0xc8", m["toBlock"])
				require.Equal(t, addr1, m["address"])
				require.Equal(t, [][]common.Hash{{topic1}}, m["topics"])
				require.NotContains(t, m, "blockHash")
			},
		},
		{
			name: "query with multiple addresses",
			query: ethereum.FilterQuery{
				FromBlock: big.NewInt(1),
				ToBlock:   big.NewInt(10),
				Addresses: []common.Address{addr1, addr2},
				Topics:    [][]common.Hash{{topic1, topic2}},
			},
			check: func(t *testing.T, result any) {
				t.Helper()
				m, ok := result.(map[string]any)
				require.True(t, ok, "result should be a map[string]any")
				require.Equal(t, "0x1", m["fromBlock"])
				require.Equal(t, "0xa", m["toBlock"])
				require.Equal(t, []common.Address{addr1, addr2}, m["address"])
				require.Equal(t, [][]common.Hash{{topic1, topic2}}, m["topics"])
			},
		},
		{
			name: "query with block hash",
			query: ethereum.FilterQuery{
				BlockHash: &blockHash,
				Addresses: []common.Address{addr1},
				Topics:    [][]common.Hash{{topic1}},
			},
			check: func(t *testing.T, result any) {
				t.Helper()
				m, ok := result.(map[string]any)
				require.True(t, ok, "result should be a map[string]any")
				require.Equal(t, blockHash, m["blockHash"])
				require.Equal(t, addr1, m["address"])
				require.NotContains(t, m, "fromBlock")
				require.NotContains(t, m, "toBlock")
			},
		},
		{
			name: "query with no addresses",
			query: ethereum.FilterQuery{
				FromBlock: big.NewInt(50),
				ToBlock:   big.NewInt(100),
				Topics:    [][]common.Hash{{topic1}},
			},
			check: func(t *testing.T, result any) {
				t.Helper()
				m, ok := result.(map[string]any)
				require.True(t, ok, "result should be a map[string]any")
				require.Equal(t, "0x32", m["fromBlock"])
				require.Equal(t, "0x64", m["toBlock"])
				require.NotContains(t, m, "address")
				require.Equal(t, [][]common.Hash{{topic1}}, m["topics"])
			},
		},
		{
			name: "query with only fromBlock",
			query: ethereum.FilterQuery{
				FromBlock: big.NewInt(1000),
				Addresses: []common.Address{addr1},
				Topics:    [][]common.Hash{},
			},
			check: func(t *testing.T, result any) {
				t.Helper()
				m, ok := result.(map[string]any)
				require.True(t, ok, "result should be a map[string]any")
				require.Equal(t, "0x3e8", m["fromBlock"])
				require.NotContains(t, m, "toBlock")
				require.Equal(t, addr1, m["address"])
			},
		},
		{
			name: "query with only toBlock",
			query: ethereum.FilterQuery{
				ToBlock:   big.NewInt(500),
				Addresses: []common.Address{addr1},
				Topics:    [][]common.Hash{},
			},
			check: func(t *testing.T, result any) {
				t.Helper()
				m, ok := result.(map[string]any)
				require.True(t, ok, "result should be a map[string]any")
				require.NotContains(t, m, "fromBlock")
				require.Equal(t, "0x1f4", m["toBlock"])
				require.Equal(t, addr1, m["address"])
			},
		},
		{
			name: "minimal query with topics only",
			query: ethereum.FilterQuery{
				Topics: [][]common.Hash{{topic1}},
			},
			check: func(t *testing.T, result any) {
				t.Helper()
				m, ok := result.(map[string]any)
				require.True(t, ok, "result should be a map[string]any")
				require.Equal(t, [][]common.Hash{{topic1}}, m["topics"])
				require.NotContains(t, m, "fromBlock")
				require.NotContains(t, m, "toBlock")
				require.NotContains(t, m, "address")
				require.NotContains(t, m, "blockHash")
			},
		},
		{
			name: "query with empty topics",
			query: ethereum.FilterQuery{
				FromBlock: big.NewInt(1),
				ToBlock:   big.NewInt(10),
				Addresses: []common.Address{addr1},
				Topics:    [][]common.Hash{},
			},
			check: func(t *testing.T, result any) {
				t.Helper()
				m, ok := result.(map[string]any)
				require.True(t, ok, "result should be a map[string]any")
				require.Equal(t, "0x1", m["fromBlock"])
				require.Equal(t, "0xa", m["toBlock"])
				require.Equal(t, addr1, m["address"])
				require.Equal(t, [][]common.Hash{}, m["topics"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := toFilterArg(tt.query)
			tt.check(t, result)
		})
	}
}

func TestToFilterArg_AddressSingleVsMultiple(t *testing.T) {
	addr1 := common.HexToAddress("0x1111111111111111111111111111111111111111")
	addr2 := common.HexToAddress("0x2222222222222222222222222222222222222222")

	// Single address should be stored as a single address, not an array
	singleQuery := ethereum.FilterQuery{
		Addresses: []common.Address{addr1},
		Topics:    [][]common.Hash{},
	}
	singleResult := toFilterArg(singleQuery)
	singleMap, ok := singleResult.(map[string]any)
	require.True(t, ok, "result should be a map[string]any")

	// Should be a single address, not a slice
	require.IsType(t, common.Address{}, singleMap["address"])
	require.Equal(t, addr1, singleMap["address"])

	// Multiple addresses should be stored as an array
	multiQuery := ethereum.FilterQuery{
		Addresses: []common.Address{addr1, addr2},
		Topics:    [][]common.Hash{},
	}
	multiResult := toFilterArg(multiQuery)
	multiMap, ok := multiResult.(map[string]any)
	require.True(t, ok, "result should be a map[string]any")

	// Should be a slice of addresses
	require.IsType(t, []common.Address{}, multiMap["address"])
	require.Equal(t, []common.Address{addr1, addr2}, multiMap["address"])
}

func TestToFilterArg_BlockHashTakesPrecedence(t *testing.T) {
	blockHash := common.HexToHash("0xabcdef")
	addr := common.HexToAddress("0x1234567890123456789012345678901234567890")

	// When blockHash is set, fromBlock and toBlock should be ignored
	query := ethereum.FilterQuery{
		BlockHash: &blockHash,
		FromBlock: big.NewInt(100),
		ToBlock:   big.NewInt(200),
		Addresses: []common.Address{addr},
		Topics:    [][]common.Hash{},
	}

	result := toFilterArg(query)
	m, ok := result.(map[string]any)
	require.True(t, ok, "result should be a map[string]any")

	require.Equal(t, blockHash, m["blockHash"])
	require.NotContains(t, m, "fromBlock", "fromBlock should not be present when blockHash is set")
	require.NotContains(t, m, "toBlock", "toBlock should not be present when blockHash is set")
}
