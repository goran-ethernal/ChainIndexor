package helpers

import (
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"net"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/stretchr/testify/require"
)

const (
	// Anvil default private key (first account)
	anvilPrivateKey = "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"
)

// getFreePort asks the kernel for a free open port that is ready to use
func getFreePort(t *testing.T) int {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err, "failed to get free port")

	port := listener.Addr().(*net.TCPAddr).Port

	err = listener.Close()
	require.NoError(t, err, "failed to close port listener")

	return port
}

// AnvilInstance manages an Anvil test node
type AnvilInstance struct {
	cmd        *exec.Cmd
	URL        string
	Client     *ethclient.Client
	PrivateKey *ecdsa.PrivateKey
	Signer     *bind.TransactOpts
	ChainID    *big.Int
}

// StartAnvil starts an Anvil instance for testing
func StartAnvil(t *testing.T) *AnvilInstance {
	t.Helper()

	// Get a random available port
	port := getFreePort(t)
	anvilURL := fmt.Sprintf("http://127.0.0.1:%d", port)

	// Start anvil with the allocated port
	// No --block-time flag = no auto-mining, blocks only mined when transactions arrive
	cmd := exec.Command("anvil",
		"--port", fmt.Sprintf("%d", port),
	)

	// Capture output for debugging
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Start()
	require.NoError(t, err, "failed to start anvil")

	// Wait for Anvil to be ready
	time.Sleep(2 * time.Second)

	// Connect to Anvil
	client, err := ethclient.Dial(anvilURL)
	require.NoError(t, err, "failed to connect to anvil")

	// Get chain ID
	ctx := t.Context()
	chainID, err := client.ChainID(ctx)
	require.NoError(t, err, "failed to get chain ID")

	// Setup signer with Anvil's default private key
	privateKey, err := crypto.HexToECDSA(anvilPrivateKey)
	require.NoError(t, err, "failed to parse private key")

	signer, err := bind.NewKeyedTransactorWithChainID(privateKey, chainID)
	require.NoError(t, err, "failed to create signer")

	instance := &AnvilInstance{
		cmd:        cmd,
		URL:        anvilURL,
		Client:     client,
		PrivateKey: privateKey,
		Signer:     signer,
		ChainID:    chainID,
	}

	// Cleanup on test completion
	t.Cleanup(func() {
		instance.Stop()
	})

	return instance
}

// Stop stops the Anvil instance
func (a *AnvilInstance) Stop() {
	if a.Client != nil {
		a.Client.Close()
	}
	if a.cmd != nil && a.cmd.Process != nil {
		_ = a.cmd.Process.Kill()
		_ = a.cmd.Wait()
	}
}

// CreateSnapshot creates a snapshot of the current chain state
func (a *AnvilInstance) CreateSnapshot(t *testing.T) string {
	t.Helper()

	var snapshotID string
	err := a.Client.Client().Call(&snapshotID, "evm_snapshot")
	require.NoError(t, err, "failed to create snapshot")

	return snapshotID
}

// RevertToSnapshot reverts the chain to a previous snapshot
func (a *AnvilInstance) RevertToSnapshot(t *testing.T, snapshotID string) {
	t.Helper()

	var success bool
	err := a.Client.Client().Call(&success, "evm_revert", snapshotID)
	require.NoError(t, err, "failed to revert to snapshot")
	require.True(t, success, "snapshot revert returned false")
}

// Mine mines the specified number of new blocks manually
func (a *AnvilInstance) Mine(t *testing.T, numBlocks int) {
	t.Helper()

	for range numBlocks {
		var blockHash string
		err := a.Client.Client().Call(&blockHash, "evm_mine")
		require.NoError(t, err, "failed to mine block")
	}
}

// GetBlockNumber returns the current block number
func (a *AnvilInstance) GetBlockNumber(t *testing.T) uint64 {
	t.Helper()

	ctx := t.Context()
	blockNumber, err := a.Client.BlockNumber(ctx)
	require.NoError(t, err, "failed to get block number")

	return blockNumber
}

// GetBlockHash returns the hash of a specific block
func (a *AnvilInstance) GetBlockHash(t *testing.T, blockNumber uint64) common.Hash {
	t.Helper()

	ctx := t.Context()
	block, err := a.Client.BlockByNumber(ctx, big.NewInt(int64(blockNumber)))
	require.NoError(t, err, "failed to get block")

	return block.Hash()
}

// RevertToForkPoint reverts the chain to a snapshot, simulating a blockchain reorg.
// Use this after creating a snapshot with CreateSnapshot() to fork the chain.
// After reverting, new transactions will create an alternative chain at the same block heights.
func (a *AnvilInstance) RevertToForkPoint(t *testing.T, snapshotID string) {
	t.Helper()
	a.RevertToSnapshot(t, snapshotID)
}

// SkipIfAnvilNotAvailable skips the test if Anvil is not available
func SkipIfAnvilNotAvailable(t *testing.T) {
	t.Helper()

	if _, err := exec.LookPath("anvil"); err != nil {
		t.Skip("anvil not found in PATH, skipping integration test")
	}
}
