# Integration Tests

This package contains integration tests for ChainIndexor that use real blockchain simulation tools to verify functionality end-to-end.

## Overview

Integration tests validate the complete system behavior including:

- Blockchain reorganization detection and handling
- RPC client interactions with real Ethereum nodes
- Database operations and state management
- Log fetching and event processing

## Test Infrastructure

### Anvil Helpers (`helpers/anvil.go`)

Utilities for managing Anvil test nodes:

- **AnvilInstance**: Manages a running Anvil process with client, signer, and chain ID
- **StartAnvil()**: Starts Anvil with automatic cleanup on test completion
- **CreateSnapshot()**: Creates a snapshot of current chain state
- **RevertToSnapshot()**: Reverts chain to a previous snapshot
- **RevertToForkPoint()**: Convenience wrapper for simulating reorgs by reverting to a snapshot
- **Mine()**: Manually mines the specified number of blocks
- **GetBlockNumber()**: Returns the current block number
- **GetBlockHash()**: Returns the hash of a specific block
- **SkipIfAnvilNotAvailable()**: Helper to skip tests when Anvil is not installed

### Test Contract (`testdata/TestEmitter.sol`)

Simple Solidity contract for emitting test events:

```solidity
contract TestEmitter {
    event TestEvent(uint256 indexed id, address indexed sender, string data);
    
    function emitEvent(uint256 id, string memory data) public;
    function emitMultipleEvents(uint256 startId, uint256 count, string memory data) public;
}
```

Compiled with `solc` and Go bindings generated with `abigen`.

## Test Scenarios

### Reorg Integration Tests (`reorg_integration_test.go`)

#### TestReorg_SimpleBlockReplacement

- Tests basic 2-block reorganization
- Emits different events on original and reorg chains
- Verifies reorg detection and error reporting

#### TestReorg_DeepReorg

- Tests deep reorganization (15 blocks)
- Validates handling of large reorgs
- Ensures all blocks are properly detected

#### TestReorg_NoLogsOnReorgChain

- Tests reorg where new chain has no logs
- Verifies detection even without log differences
- Validates block hash comparison logic

#### TestReorg_NewLogsOnReorgChain

- Tests reorg where new chain has more events
- Emits multiple events per block on reorg chain
- Validates log count differences

## Prerequisites

### Install Foundry (includes Anvil)

```bash
curl -L https://foundry.paradigm.xyz | bash
foundryup
```

Verify installation:

```bash
anvil --version
```

## Running Tests

### Run All Integration Tests

```bash
go test -tags=integration -v ./tests/... -timeout 5m
```

### Run Specific Test

```bash
go test -tags=integration -v ./tests/... -run TestReorg_SimpleBlockReplacement
```

### Run with Detailed Logging

```bash
go test -tags=integration -v ./tests/... -timeout 5m 2>&1 | tee integration-test.log
```

## Test Execution Flow

1. **Setup**: Start Anvil, create database, initialize RPC client
2. **Deploy**: Deploy TestEmitter contract to Anvil
3. **Original Chain**: Mine blocks and emit events
4. **Record**: Use ReorgDetector to record block hashes
5. **Simulate Reorg**: Create snapshot, revert, mine alternative blocks
6. **Verify**: Attempt to record new blocks and verify reorg detection

## CI/CD Integration

Example GitHub Actions workflow:

```yaml
name: Integration Tests
on: [push, pull_request]

jobs:
  integration:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
      - name: Set up Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.21'
      
      - name: Install Foundry
        run: |
          curl -L https://foundry.paradigm.xyz | bash
          source ~/.bashrc
          foundryup
      
      - name: Run Integration Tests
        run: go test -tags=integration -v ./tests/... -timeout 5m
```

## Test Isolation

Each test:

- Starts its own Anvil instance
- Uses a temporary SQLite database
- Cleans up resources automatically via `t.Cleanup()`
- Is independent and can run in parallel

## Debugging

### Enable Verbose Anvil Output

Edit `helpers/anvil.go` to redirect Anvil output:

```go
cmd.Stdout = os.Stdout
cmd.Stderr = os.Stderr
```

### Inspect Test Database

Tests create temporary databases with pattern `reorg_*_*.db`. To inspect:

```bash
# Modify test to not delete database
# defer os.Remove(dbPath)  // Comment this line

# Then inspect
sqlite3 /tmp/reorg_integration_*.db
```

### Check Anvil Logs

Anvil logs are printed to stdout/stderr when tests run with `-v` flag.

## Adding New Test Contracts

To add a new Solidity contract for testing:

### 1. Create the Solidity Contract

Create your contract in `tests/testdata/`:

```solidity
// tests/testdata/MyContract.sol
// SPDX-License-Identifier: MIT
pragma solidity ^0.5.0;

contract MyContract {
    event MyEvent(uint256 indexed value, address indexed caller);
    
    function doSomething(uint256 value) public {
        emit MyEvent(value, msg.sender);
    }
}
```

### 2. Generate Go Bindings

Use the provided script to compile and generate Go bindings:

```bash
# From project root
./tests/scripts/generate-contract.sh MyContract

# Or from scripts directory
cd tests/scripts
./generate-contract.sh MyContract
```

The script will:

- Compile the Solidity contract with `solc`
- Generate Go bindings with `abigen`
- Clean up intermediate `.abi` and `.bin` files
- Output `MyContract.go` with embedded ABI and bytecode

### 3. Use in Tests

```go
import "github.com/goran-ethernal/ChainIndexor/tests/testdata"

func TestWithMyContract(t *testing.T) {
    helpers.SkipIfAnvilNotAvailable(t)
    anvil := helpers.StartAnvil(t)
    
    // Deploy contract
    address, tx, contract, err := testdata.DeployMyContract(anvil.Signer, anvil.Client)
    require.NoError(t, err)
    time.Sleep(2 * time.Second)
    
    // Interact with contract
    tx, err = contract.DoSomething(anvil.Signer, big.NewInt(42))
    require.NoError(t, err)
    time.Sleep(1 * time.Second)
    
    // Fetch logs and verify
    filter := ethereum.FilterQuery{
        FromBlock: big.NewInt(1),
        Addresses: []common.Address{address},
    }
    logs, err := anvil.Client.FilterLogs(context.Background(), filter)
    require.NoError(t, err)
    require.Len(t, logs, 1)
}
```

### Prerequisites for contracts gen

Ensure you have the required tools installed:

```bash
# Solidity compiler
solc --version

# Go Ethereum's abigen tool
go install github.com/ethereum/go-ethereum/cmd/abigen@latest
```

## Extending Tests

To add new integration tests:

1. Create test function with `Test` prefix
2. Add `//go:build integration` build tag
3. Use `SkipIfAnvilNotAvailable(t)` to skip if Anvil not installed
4. Start Anvil with `StartAnvil(t)`
5. Set up database and RPC client
6. Implement test scenario
7. Use `require` assertions for validation

Example:

```go
func TestReorg_MyNewScenario(t *testing.T) {
    helpers.SkipIfAnvilNotAvailable(t)
    anvil := helpers.StartAnvil(t)
    
    // Your test code here
}
```

## Performance

Integration tests typically take:

- Simple 2-block reorg: ~5-10 seconds
- Deep 15-block reorg: ~20-30 seconds
- Full test suite: ~1-2 minutes

Block time is set to 1 second in Anvil for faster tests.

## Troubleshooting

### Anvil not found

```bash
Tests are skipped if Anvil is not available. Install Foundry.
```

### Port already in use

```bash
Anvil starts on port 8545. Ensure no other process is using this port.
```

### Test timeout

```bash
Increase timeout: go test -tags=integration -timeout 10m ./tests/...
```

### Database locked

```bash
Ensure previous test processes are terminated. Each test uses unique temp files.
```
