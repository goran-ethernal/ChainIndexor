package tests

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/goran-ethernal/ChainIndexor/internal/db"
	"github.com/goran-ethernal/ChainIndexor/internal/logger"
	"github.com/goran-ethernal/ChainIndexor/internal/reorg"
	"github.com/goran-ethernal/ChainIndexor/internal/rpc"
	"github.com/goran-ethernal/ChainIndexor/pkg/config"
	pkgreorg "github.com/goran-ethernal/ChainIndexor/pkg/reorg"
	"github.com/goran-ethernal/ChainIndexor/tests/helpers"
	"github.com/goran-ethernal/ChainIndexor/tests/testdata"
	"github.com/stretchr/testify/require"
)

// TestReorg_SimpleBlockReplacement tests a simple reorg scenario where
// 2 blocks are replaced by 2 alternative blocks
func TestReorg_SimpleBlockReplacement(t *testing.T) {
	helpers.SkipIfAnvilNotAvailable(t)

	// Start Anvil
	anvil := helpers.StartAnvil(t)

	// Setup database
	database := helpers.NewTestDB(t, "reorg_integration.db")
	defer database.Close()

	ctx := context.Background()

	// Setup RPC client (with no retries for faster tests)
	retryConfig := config.RetryConfig{MaxAttempts: 1}
	rpcClient, err := rpc.NewClient(ctx, anvil.URL, &retryConfig)
	require.NoError(t, err)
	defer rpcClient.Close()

	// Setup logger
	log, err := logger.NewLogger("info", false)
	require.NoError(t, err)

	// Create ReorgDetector
	detector, err := reorg.NewReorgDetector(database, rpcClient, log, &db.NoOpMaintenance{})
	require.NoError(t, err)

	// Deploy test contract

	// Create the test contract using go-ethereum
	address, tx, contract, err := testdata.DeployTestEmitter(anvil.Signer, anvil.Client)
	require.NoError(t, err)
	require.NotNil(t, contract)

	// Wait for deployment transaction to be mined
	time.Sleep(2 * time.Second)

	// Verify contract is deployed
	code, err := anvil.Client.CodeAt(ctx, address, nil)
	require.NoError(t, err)
	require.NotEmpty(t, code, "contract not deployed")

	t.Logf("Contract deployed at: %s (tx: %s)", address.Hex(), tx.Hash().Hex())

	// Mine a few blocks to establish a base chain
	anvil.Mine(t, 3)

	forkPoint := anvil.GetBlockNumber(t)
	t.Logf("Fork point at block: %d", forkPoint)

	// Create snapshot at fork point
	snapshotID := anvil.CreateSnapshot(t)

	// Emit events on the original chain
	tx1, err := contract.EmitEvent(anvil.Signer, big.NewInt(1), "original-event-1")
	require.NoError(t, err)
	time.Sleep(1 * time.Second) // Wait for block

	tx2, err := contract.EmitEvent(anvil.Signer, big.NewInt(2), "original-event-2")
	require.NoError(t, err)
	time.Sleep(1 * time.Second) // Wait for block

	// Get the original blocks with logs
	originalBlock1 := forkPoint + 1
	originalBlock2 := forkPoint + 2

	t.Logf("Original chain: blocks %d-%d", originalBlock1, originalBlock2)
	t.Logf("Original tx1: %s, tx2: %s", tx1.Hash().Hex(), tx2.Hash().Hex())

	// Fetch logs from original blocks
	filter := ethereum.FilterQuery{
		FromBlock: big.NewInt(int64(originalBlock1)),
		ToBlock:   big.NewInt(int64(originalBlock2)),
		Addresses: []common.Address{address},
	}

	originalLogs, err := rpcClient.GetLogs(ctx, filter)
	require.NoError(t, err)
	require.Len(t, originalLogs, 2, "should have 2 logs on original chain")

	// Record the original blocks in the detector
	headers, err := detector.VerifyAndRecordBlocks(ctx, originalLogs, originalBlock1, originalBlock2)
	require.NoError(t, err)
	require.Len(t, headers, 2)

	originalHash1 := headers[0].Hash()
	originalHash2 := headers[1].Hash()
	t.Logf("Original block hashes: %s, %s", originalHash1.Hex(), originalHash2.Hex())

	// Verify database state after recording original blocks
	count, err := detector.GetStoredBlockCount()
	require.NoError(t, err)
	require.Equal(t, 2, count, "should have 2 blocks stored")

	stored1, err := detector.GetStoredBlock(originalBlock1)
	require.NoError(t, err)
	require.Equal(t, originalHash1, stored1.BlockHash, "stored hash should match original")

	stored2, err := detector.GetStoredBlock(originalBlock2)
	require.NoError(t, err)
	require.Equal(t, originalHash2, stored2.BlockHash, "stored hash should match original")

	t.Logf("✓ Database state verified: 2 blocks stored with correct hashes")

	// Now simulate a reorg - revert to fork point
	anvil.RevertToForkPoint(t, snapshotID)
	currentBlock := anvil.GetBlockNumber(t)
	require.Equal(t, forkPoint, currentBlock, "should be back at fork point")

	// Emit different events on the reorg chain (these will mine into blocks at same heights)
	tx3, err := contract.EmitEvent(anvil.Signer, big.NewInt(3), "reorg-event-1")
	require.NoError(t, err)
	time.Sleep(1 * time.Second)

	tx4, err := contract.EmitEvent(anvil.Signer, big.NewInt(4), "reorg-event-2")
	require.NoError(t, err)
	time.Sleep(1 * time.Second)

	t.Logf("Reorg tx3: %s, tx4: %s", tx3.Hash().Hex(), tx4.Hash().Hex())

	// Verify new block hashes are different
	reorgHash1 := anvil.GetBlockHash(t, originalBlock1)
	reorgHash2 := anvil.GetBlockHash(t, originalBlock2)
	t.Logf("Reorg block hashes: %s, %s", reorgHash1.Hex(), reorgHash2.Hex())
	require.NotEqual(t, originalHash1, reorgHash1, "block 1 hash should change after reorg")
	require.NotEqual(t, originalHash2, reorgHash2, "block 2 hash should change after reorg")

	// Fetch logs from reorg blocks
	reorgLogs, err := rpcClient.GetLogs(ctx, filter)
	require.NoError(t, err)
	require.Len(t, reorgLogs, 2, "should have 2 logs on reorg chain")

	// Verify logs are different
	require.NotEqual(t, originalLogs[0].TxHash, reorgLogs[0].TxHash, "log tx hashes should differ")

	// Now try to verify the reorg blocks - detector should detect the reorg
	_, err = detector.VerifyAndRecordBlocks(ctx, reorgLogs, originalBlock1, originalBlock2)
	require.Error(t, err, "should detect reorg")
	require.Contains(t, err.Error(), "reorg detected", "error should mention reorg")

	// Extract reorg info from error
	reorgErr, ok := err.(*pkgreorg.ReorgDetectedError)
	require.True(t, ok, "error should be ReorgDetectedError type")
	require.Equal(t, originalBlock1, reorgErr.FirstReorgBlock)

	t.Logf("Reorg detected at block %d: %s", reorgErr.FirstReorgBlock, reorgErr.Details)

	// Verify database state after reorg detection - old blocks should still be there
	// (they get invalidated but not removed until successful re-recording)
	count, err = detector.GetStoredBlockCount()
	require.NoError(t, err)
	require.Equal(t, 2, count, "should still have 2 blocks stored")

	stored1AfterReorg, err := detector.GetStoredBlock(originalBlock1)
	require.NoError(t, err)
	require.Equal(t, originalHash1, stored1AfterReorg.BlockHash, "old hash still in DB after reorg detection")

	t.Logf("✓ Database state after reorg: old blocks still present (not yet replaced)")
}

// TestReorg_DeepReorg tests a reorg with more than 10 blocks
func TestReorg_DeepReorg(t *testing.T) {
	helpers.SkipIfAnvilNotAvailable(t)

	anvil := helpers.StartAnvil(t)

	// Setup database
	database := helpers.NewTestDB(t, "reorg_deep.db")
	defer database.Close()

	ctx := context.Background()

	retryConfig := config.RetryConfig{MaxAttempts: 1}
	rpcClient, err := rpc.NewClient(ctx, anvil.URL, &retryConfig)
	require.NoError(t, err)
	defer rpcClient.Close()

	log, err := logger.NewLogger("info", false)
	require.NoError(t, err)

	detector, err := reorg.NewReorgDetector(database, rpcClient, log, &db.NoOpMaintenance{})
	require.NoError(t, err)

	// Deploy test contract
	address, _, contract, err := testdata.DeployTestEmitter(anvil.Signer, anvil.Client)
	require.NoError(t, err)
	time.Sleep(2 * time.Second)

	// Mine blocks and create snapshot at fork point
	anvil.Mine(t, 5)
	forkPoint := anvil.GetBlockNumber(t)
	snapshotID := anvil.CreateSnapshot(t)

	// Emit 15 events on original chain
	const numBlocks = 15
	var originalLogs []types.Log

	for i := 1; i <= numBlocks; i++ {
		_, err := contract.EmitEvent(anvil.Signer, big.NewInt(int64(i)), "original")
		require.NoError(t, err)
		time.Sleep(1 * time.Second)
	}

	// Fetch all logs
	filter := ethereum.FilterQuery{
		FromBlock: big.NewInt(int64(forkPoint + 1)),
		ToBlock:   big.NewInt(int64(forkPoint + numBlocks)),
		Addresses: []common.Address{address},
	}

	originalLogs, err = rpcClient.GetLogs(ctx, filter)
	require.NoError(t, err)
	require.Len(t, originalLogs, numBlocks)

	// Record original blocks
	_, err = detector.VerifyAndRecordBlocks(ctx, originalLogs, forkPoint+1, forkPoint+numBlocks)
	require.NoError(t, err)

	// Simulate deep reorg - revert to fork point
	anvil.RevertToForkPoint(t, snapshotID)
	t.Logf("Deep reorg: reverting %d blocks", numBlocks)

	// Emit different events on reorg chain (will mine into same block heights)
	for i := 1; i <= numBlocks; i++ {
		_, err := contract.EmitEvent(anvil.Signer, big.NewInt(int64(i+100)), "reorg")
		require.NoError(t, err)
		time.Sleep(1 * time.Second)
	}

	// Fetch reorg logs
	reorgLogs, err := rpcClient.GetLogs(ctx, filter)
	require.NoError(t, err)
	require.Len(t, reorgLogs, numBlocks)

	// Detector should detect the deep reorg
	_, err = detector.VerifyAndRecordBlocks(ctx, reorgLogs, forkPoint+1, forkPoint+numBlocks)
	require.Error(t, err)
	require.Contains(t, err.Error(), "reorg detected")

	reorgErr, ok := err.(*pkgreorg.ReorgDetectedError)
	require.True(t, ok)
	require.Equal(t, forkPoint+1, reorgErr.FirstReorgBlock)

	t.Logf("Deep reorg detected at block %d: %s (depth: %d blocks)",
		reorgErr.FirstReorgBlock, reorgErr.Details, numBlocks)
}

// TestReorg_NoLogsOnReorgChain tests reorg where new chain has no logs
func TestReorg_NoLogsOnReorgChain(t *testing.T) {
	helpers.SkipIfAnvilNotAvailable(t)

	anvil := helpers.StartAnvil(t)

	// Setup database
	database := helpers.NewTestDB(t, "reorg_nologs.db")
	defer database.Close()

	ctx := context.Background()

	retryConfig := config.RetryConfig{MaxAttempts: 1}
	rpcClient, err := rpc.NewClient(ctx, anvil.URL, &retryConfig)
	require.NoError(t, err)
	defer rpcClient.Close()

	log, err := logger.NewLogger("info", false)
	require.NoError(t, err)

	detector, err := reorg.NewReorgDetector(database, rpcClient, log, &db.NoOpMaintenance{})
	require.NoError(t, err)

	// Deploy contract
	address, _, contract, err := testdata.DeployTestEmitter(anvil.Signer, anvil.Client)
	require.NoError(t, err)
	time.Sleep(2 * time.Second)

	anvil.Mine(t, 3)
	forkPoint := anvil.GetBlockNumber(t)
	snapshotID := anvil.CreateSnapshot(t)

	// Emit events on original chain
	_, err = contract.EmitEvent(anvil.Signer, big.NewInt(1), "event-1")
	require.NoError(t, err)
	time.Sleep(1 * time.Second)

	_, err = contract.EmitEvent(anvil.Signer, big.NewInt(2), "event-2")
	require.NoError(t, err)
	time.Sleep(1 * time.Second)

	// Fetch original logs
	filter := ethereum.FilterQuery{
		FromBlock: big.NewInt(int64(forkPoint + 1)),
		ToBlock:   big.NewInt(int64(forkPoint + 2)),
		Addresses: []common.Address{address},
	}

	originalLogs, err := rpcClient.GetLogs(ctx, filter)
	require.NoError(t, err)
	require.Len(t, originalLogs, 2)

	// Record original blocks
	_, err = detector.VerifyAndRecordBlocks(ctx, originalLogs, forkPoint+1, forkPoint+2)
	require.NoError(t, err)

	// Simulate reorg - revert to fork point
	anvil.RevertToForkPoint(t, snapshotID)

	// Mine empty blocks on reorg chain (no transactions)
	anvil.Mine(t, 2)

	// Fetch logs from reorg chain (should be empty)
	reorgLogs, err := rpcClient.GetLogs(ctx, filter)
	require.NoError(t, err)
	require.Empty(t, reorgLogs, "reorg chain should have no logs")

	// Detector should still detect reorg even with empty logs
	_, err = detector.VerifyAndRecordBlocks(ctx, reorgLogs, forkPoint+1, forkPoint+2)
	require.Error(t, err)
	require.Contains(t, err.Error(), "reorg detected")

	t.Logf("Reorg detected even with no logs on new chain")
}

// TestReorg_NewLogsOnReorgChain tests reorg where new chain has MORE logs
func TestReorg_NewLogsOnReorgChain(t *testing.T) {
	helpers.SkipIfAnvilNotAvailable(t)

	anvil := helpers.StartAnvil(t)

	// Setup database
	database := helpers.NewTestDB(t, "reorg_morelogs.db")
	defer database.Close()

	ctx := context.Background()

	retryConfig := config.RetryConfig{MaxAttempts: 1}
	rpcClient, err := rpc.NewClient(ctx, anvil.URL, &retryConfig)
	require.NoError(t, err)
	defer rpcClient.Close()

	log, err := logger.NewLogger("info", false)
	require.NoError(t, err)

	detector, err := reorg.NewReorgDetector(database, rpcClient, log, &db.NoOpMaintenance{})
	require.NoError(t, err)

	// Deploy contract
	address, _, contract, err := testdata.DeployTestEmitter(anvil.Signer, anvil.Client)
	require.NoError(t, err)
	time.Sleep(2 * time.Second)

	anvil.Mine(t, 3)
	forkPoint := anvil.GetBlockNumber(t)
	snapshotID := anvil.CreateSnapshot(t)

	// Emit 2 events on original chain
	_, err = contract.EmitEvent(anvil.Signer, big.NewInt(1), "original-1")
	require.NoError(t, err)
	time.Sleep(1 * time.Second)

	_, err = contract.EmitEvent(anvil.Signer, big.NewInt(2), "original-2")
	require.NoError(t, err)
	time.Sleep(1 * time.Second)

	filter := ethereum.FilterQuery{
		FromBlock: big.NewInt(int64(forkPoint + 1)),
		ToBlock:   big.NewInt(int64(forkPoint + 2)),
		Addresses: []common.Address{address},
	}

	originalLogs, err := rpcClient.GetLogs(ctx, filter)
	require.NoError(t, err)
	require.Len(t, originalLogs, 2)

	// Record original blocks
	_, err = detector.VerifyAndRecordBlocks(ctx, originalLogs, forkPoint+1, forkPoint+2)
	require.NoError(t, err)

	// Simulate reorg - revert to fork point
	anvil.RevertToForkPoint(t, snapshotID)

	// Emit MULTIPLE events per block on reorg chain (will mine into same block heights)
	_, err = contract.EmitMultipleEvents(anvil.Signer, big.NewInt(10), big.NewInt(3), "reorg-batch-1")
	require.NoError(t, err)
	time.Sleep(1 * time.Second)

	_, err = contract.EmitMultipleEvents(anvil.Signer, big.NewInt(20), big.NewInt(5), "reorg-batch-2")
	require.NoError(t, err)
	time.Sleep(1 * time.Second)

	// Fetch logs from reorg chain (should have 8 total: 3 + 5)
	reorgLogs, err := rpcClient.GetLogs(ctx, filter)
	require.NoError(t, err)
	require.Len(t, reorgLogs, 8, "reorg chain should have more logs")

	t.Logf("Original chain had %d logs, reorg chain has %d logs",
		len(originalLogs), len(reorgLogs))

	// Detector should detect reorg
	_, err = detector.VerifyAndRecordBlocks(ctx, reorgLogs, forkPoint+1, forkPoint+2)
	require.Error(t, err)
	require.Contains(t, err.Error(), "reorg detected")
}

// TestReorg_Chaos_RapidMultipleReorgs simulates a highly unstable network
// where the chain rapidly reorgs between multiple competing forks.
// This tests the ReorgDetector's ability to handle rapid state changes
// without data corruption or panics.
func TestReorg_Chaos_RapidMultipleReorgs(t *testing.T) {
	helpers.SkipIfAnvilNotAvailable(t)

	anvil := helpers.StartAnvil(t)

	// Setup
	database := helpers.NewTestDB(t, "reorg_chaos.db")
	defer database.Close()

	ctx := context.Background()

	retryConfig := config.RetryConfig{MaxAttempts: 1}
	rpcClient, err := rpc.NewClient(ctx, anvil.URL, &retryConfig)
	require.NoError(t, err)
	defer rpcClient.Close()

	log, err := logger.NewLogger("info", false)
	require.NoError(t, err)

	detector, err := reorg.NewReorgDetector(database, rpcClient, log, &db.NoOpMaintenance{})
	require.NoError(t, err)

	// Deploy contract
	address, _, contract, err := testdata.DeployTestEmitter(anvil.Signer, anvil.Client)
	require.NoError(t, err)
	time.Sleep(2 * time.Second)

	// Build initial chain with several blocks
	anvil.Mine(t, 3)
	baseForkPoint := anvil.GetBlockNumber(t)
	baseSnapshot := anvil.CreateSnapshot(t)
	t.Logf("Base fork point at block: %d", baseForkPoint)

	// Build a longer chain on top
	for i := range 5 {
		anvil.Mine(t, 2)

		// Emit an event
		_, err := contract.EmitEvent(anvil.Signer, big.NewInt(int64(i*100)), "initial-"+string(rune('A'+i)))
		require.NoError(t, err)
		time.Sleep(1 * time.Second)
	}

	lastBlock := anvil.GetBlockNumber(t)
	t.Logf("Built initial chain up to block: %d", lastBlock)

	// Record the initial state
	filter := ethereum.FilterQuery{
		FromBlock: big.NewInt(int64(baseForkPoint + 1)),
		ToBlock:   big.NewInt(int64(lastBlock)),
		Addresses: []common.Address{address},
	}

	initialLogs, err := rpcClient.GetLogs(ctx, filter)
	require.NoError(t, err)
	t.Logf("Initial chain has %d logs from block %d to %d", len(initialLogs), baseForkPoint+1, lastBlock)

	// Record initial state in detector
	_, err = detector.VerifyAndRecordBlocks(ctx, initialLogs, baseForkPoint+1, lastBlock)
	require.NoError(t, err)

	// Now perform rapid reorgs - each time reverting to base and building a new alternative chain
	reorgCount := 15
	reorgsDetected := 0
	successfulRecords := 0

	t.Logf("Starting chaos test with %d rapid reorgs...", reorgCount)

	for i := range reorgCount {
		t.Logf("Reorg %d: Reverting to base fork point (block %d)", i+1, baseForkPoint)

		// Always revert to the base fork point
		anvil.RevertToForkPoint(t, baseSnapshot)

		// Recreate base snapshot since it's now invalidated
		baseSnapshot = anvil.CreateSnapshot(t)

		// Build a new alternative chain with varying length and content
		numNewBlocks := 3 + (i % 5) // 3-7 blocks
		for j := range numNewBlocks {
			anvil.Mine(t, 1)

			// Emit event with unique data
			_, err := contract.EmitEvent(anvil.Signer, big.NewInt(int64(1000+i*10+j)), "chaos-"+string(rune('0'+i%10)))
			require.NoError(t, err)
			time.Sleep(500 * time.Millisecond) // Faster than before
		}

		currentBlock := anvil.GetBlockNumber(t)
		t.Logf("  Built alternative chain up to block: %d (%d new blocks)", currentBlock, numNewBlocks)

		// Fetch logs from the new chain
		fetchFilter := ethereum.FilterQuery{
			FromBlock: big.NewInt(int64(baseForkPoint + 1)),
			ToBlock:   big.NewInt(int64(currentBlock)),
			Addresses: []common.Address{address},
		}

		logs, err := rpcClient.GetLogs(ctx, fetchFilter)
		require.NoError(t, err)

		// Attempt to verify and record
		_, err = detector.VerifyAndRecordBlocks(ctx, logs, baseForkPoint+1, currentBlock)

		if err != nil {
			// Reorg detected (expected after first iteration)
			require.Contains(t, err.Error(), "reorg detected", "Error should be a reorg error")

			reorgErr, ok := err.(*pkgreorg.ReorgDetectedError)
			require.True(t, ok, "Error should be ReorgDetectedError type")
			t.Logf("  ✓ Reorg detected at block %d: %s", reorgErr.FirstReorgBlock, reorgErr.Details)
			reorgsDetected++
		} else {
			// Successfully recorded
			t.Logf("  ✓ State recorded successfully (no conflict detected)")
			successfulRecords++
		}
	}

	t.Logf("Chaos test complete: %d reorgs detected, %d successful records", reorgsDetected, successfulRecords)

	// Since we recorded the initial state before the loop, and every iteration
	// builds a different alternative chain, we should detect reorgs on ALL iterations
	require.Equal(t, reorgCount, reorgsDetected, "Should detect reorg on every iteration")
	require.Equal(t, 0, successfulRecords, "No successful records expected since all iterations cause reorgs")

	// Verify database integrity after chaos test
	finalCount, err := detector.GetStoredBlockCount()
	require.NoError(t, err)
	t.Logf("Final database state: %d blocks stored", finalCount)
	require.Greater(t, finalCount, 0, "should have blocks stored after chaos test")

	// Verify we can read some blocks without errors (no corruption)
	for blockNum := baseForkPoint + 1; blockNum <= uint64(min(int(baseForkPoint+5), int(lastBlock))); blockNum++ {
		stored, err := detector.GetStoredBlock(blockNum)
		if err == nil {
			// Block exists - verify it has valid data
			require.NotEqual(t, common.Hash{}, stored.BlockHash, "stored block should have valid hash")
			t.Logf("  Block %d: hash=%s", blockNum, stored.BlockHash.Hex())
		}
	}

	// Most importantly: verify no panics or data corruption occurred
	// If we got here, the detector handled rapid state changes gracefully
	t.Logf("✓ Database integrity verified: no corruption after %d rapid reorgs", reorgCount)
}
