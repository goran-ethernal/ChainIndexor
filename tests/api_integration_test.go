package tests

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"path"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	_ "github.com/goran-ethernal/ChainIndexor/examples/indexers/erc20"
	commonpkg "github.com/goran-ethernal/ChainIndexor/internal/common"
	"github.com/goran-ethernal/ChainIndexor/internal/logger"
	"github.com/goran-ethernal/ChainIndexor/pkg/api"
	"github.com/goran-ethernal/ChainIndexor/pkg/config"
	"github.com/goran-ethernal/ChainIndexor/pkg/indexer"
	"github.com/goran-ethernal/ChainIndexor/tests/helpers"
	"github.com/goran-ethernal/ChainIndexor/tests/testdata"
	"github.com/stretchr/testify/require"
)

// mockCoordinator implements a simple coordinator for testing
type mockCoordinator struct {
	indexers []indexer.Indexer
}

func (m *mockCoordinator) GetByName(name string) indexer.Indexer {
	for _, idx := range m.indexers {
		if idx.GetName() == name {
			return idx
		}
	}
	return nil
}

func (m *mockCoordinator) ListAll() []indexer.Indexer {
	return m.indexers
}

// TestAPI_IntegrationWithERC20 tests the complete flow: contract deployment → transactions → indexing → API queries
func TestAPI_IntegrationWithERC20(t *testing.T) {
	helpers.SkipIfAnvilNotAvailable(t)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	// ========================================
	// 1. SETUP PHASE
	// ========================================

	// Start Anvil
	anvil := helpers.StartAnvil(t)

	// Deploy ERC20 token with 1,000,000 tokens initial supply
	initialSupply := new(big.Int).Mul(big.NewInt(1000000), big.NewInt(1e18))
	tokenAddress, tx, token, err := testdata.DeployTestERC20(anvil.Signer, anvil.Client, initialSupply)
	require.NoError(t, err)
	require.NotNil(t, token)

	// Wait for deployment
	time.Sleep(2 * time.Second)

	// Verify deployment
	code, err := anvil.Client.CodeAt(ctx, tokenAddress, nil)
	require.NoError(t, err)
	require.NotEmpty(t, code)

	deployBlock := anvil.GetBlockNumber(t)
	t.Logf("✓ ERC20 token deployed at %s (block %d, tx: %s)", tokenAddress.Hex(), deployBlock, tx.Hash().Hex())

	// Create additional test accounts - use Anvil's pre-funded accounts
	// Bob = Anvil account #1, Charlie = Anvil account #2
	bobKey, err := crypto.HexToECDSA("59c6995e998f97a5a0044966f0945389dc9e86dae88c7a8412f4603b6b78690d")
	require.NoError(t, err)
	bobAddress := crypto.PubkeyToAddress(bobKey.PublicKey)

	charlieKey, err := crypto.HexToECDSA("5de4111afa1a4b94908f83103eb1f1706367c2e68ca870fc3fb9a804cdab365a")
	require.NoError(t, err)
	charlieAddress := crypto.PubkeyToAddress(charlieKey.PublicKey)

	aliceAddress := anvil.Signer.From
	t.Logf("Test accounts - Alice: %s, Bob: %s, Charlie: %s", aliceAddress.Hex(), bobAddress.Hex(), charlieAddress.Hex())

	// Setup database
	tmpDir := t.TempDir()
	indexerDBPath := path.Join(tmpDir, "api_test_erc20.db")

	// Setup logger
	log, err := logger.NewLogger("info", false)
	require.NoError(t, err)

	// Create and initialize ERC20 indexer
	indexerDBConfig := config.DatabaseConfig{Path: indexerDBPath}
	indexerDBConfig.ApplyDefaults()

	indexerConfig := config.IndexerConfig{
		Name:       "TestERC20Indexer",
		Type:       "erc20",
		StartBlock: 0,
		DB:         indexerDBConfig,
		Contracts: []config.ContractConfig{
			{
				Address: tokenAddress.Hex(),
				Events: []string{
					"Transfer(address indexed from, address indexed to, uint256 value)",
					"Approval(address indexed owner, address indexed spender, uint256 value)",
				},
			},
		},
	}

	idx, err := indexer.Create("erc20", indexerConfig, log)
	require.NoError(t, err)
	t.Logf("✓ ERC20 indexer created")

	// Start API server with a mock coordinator that contains our single indexer
	coordinator := &mockCoordinator{indexers: []indexer.Indexer{idx}}

	apiConfig := &config.APIConfig{
		Enabled:       true,
		ListenAddress: ":18080",
		ReadTimeout:   commonpkg.NewDuration(30 * time.Second),
		WriteTimeout:  commonpkg.NewDuration(30 * time.Second),
		IdleTimeout:   commonpkg.NewDuration(120 * time.Second),
		CORS: config.CORSConfig{
			Enabled:        true,
			AllowedOrigins: []string{"*"},
		},
	}
	apiServer := api.NewServer(apiConfig, coordinator, log)
	go func() {
		if err := apiServer.Start(ctx); err != nil {
			t.Logf("API server error: %v", err)
		}
	}()

	// Wait for API server to start
	time.Sleep(1 * time.Second)
	t.Logf("✓ API server started on %s", apiConfig.ListenAddress)

	// ========================================
	// 2. GENERATE TEST DATA PHASE
	// ========================================

	// Mine an empty block for spacing
	anvil.Mine(t, 1)
	time.Sleep(1 * time.Second)

	// Block 3: Two transfers
	amount100 := new(big.Int).Mul(big.NewInt(100), big.NewInt(1e18))
	_, err = token.Transfer(anvil.Signer, bobAddress, amount100)
	require.NoError(t, err)
	time.Sleep(1 * time.Second)

	amount50 := new(big.Int).Mul(big.NewInt(50), big.NewInt(1e18))
	_, err = token.Transfer(anvil.Signer, charlieAddress, amount50)
	require.NoError(t, err)
	time.Sleep(1 * time.Second)

	block3 := anvil.GetBlockNumber(t)
	t.Logf("✓ Block %d: Transfer Alice→Bob (100), Transfer Alice→Charlie (50)", block3)

	// Block 4: One approval
	amount200 := new(big.Int).Mul(big.NewInt(200), big.NewInt(1e18))
	_, err = token.Approve(anvil.Signer, bobAddress, amount200)
	require.NoError(t, err)
	time.Sleep(1 * time.Second)

	block4 := anvil.GetBlockNumber(t)
	t.Logf("✓ Block %d: Approval Alice→Bob (200)", block4)

	// Block 5: Transfer from Bob (who is now a pre-funded Anvil account)
	bobSigner, err := bind.NewKeyedTransactorWithChainID(bobKey, anvil.ChainID)
	require.NoError(t, err)

	amount25 := new(big.Int).Mul(big.NewInt(25), big.NewInt(1e18))
	_, err = token.Transfer(bobSigner, charlieAddress, amount25)
	require.NoError(t, err)
	time.Sleep(1 * time.Second)

	block5 := anvil.GetBlockNumber(t)
	t.Logf("✓ Block %d: Transfer Bob→Charlie (25)", block5)

	t.Logf("✓ Test data generated: 3 transfers, 1 approval across blocks %d-%d", block3, block5)

	// ========================================
	// 3. INDEXING PHASE - Manual synchronous indexing
	// ========================================

	t.Log("Manually indexing events...")

	// Fetch logs from the RPC
	filter := ethereum.FilterQuery{
		FromBlock: big.NewInt(0),
		ToBlock:   big.NewInt(int64(block5)),
		Addresses: []common.Address{tokenAddress},
	}

	logs, err := anvil.Client.FilterLogs(ctx, filter)
	require.NoError(t, err)
	t.Logf("Fetched %d logs from blocks 0-%d", len(logs), block5)

	// Manually call HandleLogs to index the events synchronously
	err = idx.HandleLogs(logs)
	require.NoError(t, err)

	t.Logf("✓ Manually indexed %d events", len(logs))

	// ========================================
	// 4. API TESTING PHASE
	// ========================================

	baseURL := fmt.Sprintf("http://localhost%s", apiConfig.ListenAddress)

	// Test 1: Health check
	t.Run("GET /health", func(t *testing.T) {
		resp, err := http.Get(baseURL + "/health")
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)

		var result struct {
			Status   string `json:"status"`
			Indexers []any  `json:"indexers"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
		require.Equal(t, "ok", result.Status)
		require.Len(t, result.Indexers, 1)
		t.Log("✓ Health check passed")
	})

	// Test 2: List indexers
	t.Run("GET /api/v1/indexers", func(t *testing.T) {
		resp, err := http.Get(baseURL + "/api/v1/indexers")
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)

		var indexers []any
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&indexers))
		require.Len(t, indexers, 1)

		indexerInfo, ok := indexers[0].(map[string]any)
		require.True(t, ok)
		require.Equal(t, "TestERC20Indexer", indexerInfo["name"])
		require.Equal(t, "erc20", indexerInfo["type"])

		eventTypes := indexerInfo["event_types"].([]any)
		require.Len(t, eventTypes, 2)
		require.Contains(t, []string{eventTypes[0].(string), eventTypes[1].(string)}, "Transfer")
		require.Contains(t, []string{eventTypes[0].(string), eventTypes[1].(string)}, "Approval")

		t.Log("✓ Indexer list correct")
	})

	// Test 3: Get stats
	t.Run("GET /api/v1/indexers/{name}/stats", func(t *testing.T) {
		resp, err := http.Get(baseURL + "/api/v1/indexers/TestERC20Indexer/stats")
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)

		var result map[string]any
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))

		eventCounts, ok := result["event_counts"].(map[string]any)
		require.True(t, ok)

		// Check Transfer count (deployment creates 1, test creates 3 more = 4 total)
		require.Equal(t, float64(4), eventCounts["Transfer"])

		// Check Approval count
		require.Equal(t, float64(1), eventCounts["Approval"])

		t.Logf("✓ Stats correct: 4 transfers (1 deployment + 3 test), 1 approval")
	})

	// Test 4: Query all transfers
	t.Run("GET /api/v1/indexers/{name}/events?event_type=transfer", func(t *testing.T) {
		resp, err := http.Get(baseURL + "/api/v1/indexers/TestERC20Indexer/events?event_type=transfer")
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)

		var result map[string]any
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))

		events, ok := result["events"].([]any)
		require.True(t, ok)
		require.Len(t, events, 4) // 1 deployment + 3 test transfers

		pagination, ok := result["pagination"].(map[string]any)
		require.True(t, ok)
		require.Equal(t, float64(4), pagination["total"])

		t.Log("✓ Query all transfers: 3 transfer events returned and 1 deployment")
	})

	// Test 5: Pagination
	t.Run("GET /api/v1/indexers/{name}/events?event_type=transfer&limit=2", func(t *testing.T) {
		resp, err := http.Get(baseURL + "/api/v1/indexers/TestERC20Indexer/events?event_type=transfer&limit=2")
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)

		var result map[string]any
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))

		events, ok := result["events"].([]any)
		require.True(t, ok)
		require.Len(t, events, 2)

		pagination, ok := result["pagination"].(map[string]any)
		require.True(t, ok)
		require.Equal(t, float64(4), pagination["total"])
		require.Equal(t, float64(2), pagination["limit"])
		require.Equal(t, float64(0), pagination["offset"])

		t.Log("✓ Pagination works: 2 of 4 events returned")
	})

	// Test 6: Block filtering
	t.Run("GET /api/v1/indexers/{name}/events?event_type=transfer&from_block=N", func(t *testing.T) {
		resp, err := http.Get(fmt.Sprintf("%s/api/v1/indexers/TestERC20Indexer/events?event_type=transfer&from_block=%d", baseURL, block5))
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)

		var result map[string]any
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))

		events, ok := result["events"].([]any)
		require.True(t, ok)
		require.Len(t, events, 1, "should only return transfer from block 5")

		event, ok := events[0].(map[string]any)
		require.True(t, ok)
		require.Equal(t, float64(block5), event["BlockNumber"])

		t.Log("✓ Block filtering works: 1 event from block 5")
	})

	// Test 7: Address filtering
	t.Run("GET /api/v1/indexers/{name}/events?event_type=transfer&address=bob", func(t *testing.T) {
		resp, err := http.Get(fmt.Sprintf("%s/api/v1/indexers/TestERC20Indexer/events?event_type=transfer&address=%s", baseURL, bobAddress.Hex()))
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)

		var result map[string]any
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))

		events, ok := result["events"].([]any)
		require.True(t, ok)
		require.Len(t, events, 2, "Bob involved in 2 transfers")

		t.Log("✓ Address filtering works: 2 transfers involving Bob")
	})

	// Test 8: Query approvals
	t.Run("GET /api/v1/indexers/{name}/events?event_type=approval", func(t *testing.T) {
		resp, err := http.Get(baseURL + "/api/v1/indexers/TestERC20Indexer/events?event_type=approval")
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)

		var result map[string]any
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))

		events, ok := result["events"].([]any)
		require.True(t, ok)
		require.Len(t, events, 1)

		approval, ok := events[0].(map[string]any)
		require.True(t, ok)
		require.Equal(t, float64(block4), approval["BlockNumber"])

		t.Log("✓ Query approvals: 1 event returned")
	})

	// Test 9: Case-insensitive event_type
	t.Run("GET /api/v1/indexers/{name}/events?event_type=TRANSFER", func(t *testing.T) {
		resp, err := http.Get(baseURL + "/api/v1/indexers/TestERC20Indexer/events?event_type=TRANSFER")
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)

		var result map[string]any
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))

		events, ok := result["events"].([]any)
		require.True(t, ok)
		require.Len(t, events, 4) // 1 deployment + 3 test transfers

		t.Log("✓ Case-insensitive event_type works")
	})

	// Test 10: Sorting
	t.Run("GET /api/v1/indexers/{name}/events?event_type=transfer&sort_order=desc", func(t *testing.T) {
		resp, err := http.Get(baseURL + "/api/v1/indexers/TestERC20Indexer/events?event_type=transfer&sort_by=block_number&sort_order=desc")
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)

		var result map[string]any
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))

		events := result["events"].([]any)
		require.Len(t, events, 4) // 1 deployment + 3 test transfers

		// Verify descending order
		event1, ok := events[0].(map[string]any)
		require.True(t, ok)
		event3, ok := events[2].(map[string]any)
		require.True(t, ok)
		require.Greater(t, event1["BlockNumber"].(float64), event3["BlockNumber"].(float64))

		t.Log("✓ Sorting works: events in descending order")
	})

	// Test 11: Error - Invalid indexer name
	t.Run("GET /api/v1/indexers/NonExistent/events - 404", func(t *testing.T) {
		resp, err := http.Get(baseURL + "/api/v1/indexers/NonExistent/events?event_type=transfer")
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusNotFound, resp.StatusCode)

		t.Log("✓ Invalid indexer returns 404")
	})

	// Test 12: Error - Missing event_type
	t.Run("GET /api/v1/indexers/{name}/events - 500", func(t *testing.T) {
		resp, err := http.Get(baseURL + "/api/v1/indexers/TestERC20Indexer/events")
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusInternalServerError, resp.StatusCode) // Current API returns 500 for query errors

		t.Log("✓ Missing event_type returns 500")
	})

	// Test 13: Error - Invalid event_type
	t.Run("GET /api/v1/indexers/{name}/events?event_type=invalid - 500", func(t *testing.T) {
		resp, err := http.Get(baseURL + "/api/v1/indexers/TestERC20Indexer/events?event_type=invalid")
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusInternalServerError, resp.StatusCode) // Current API returns 500 for query errors

		t.Log("✓ Invalid event_type returns 500")
	})

	// Test 14: CORS headers
	t.Run("CORS headers", func(t *testing.T) {
		req, err := http.NewRequest("OPTIONS", baseURL+"/api/v1/indexers", nil)
		require.NoError(t, err)
		req.Header.Set("Origin", "http://example.com")
		req.Header.Set("Access-Control-Request-Method", "GET")

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		require.NotEmpty(t, resp.Header.Get("Access-Control-Allow-Origin"))
		t.Log("✓ CORS headers present")
	})

	t.Log("✅ All API integration tests passed!")
}
