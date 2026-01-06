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

### API Test Contract (`testdata/TestERC20.sol`)

Minimal ERC20 implementation for testing REST API functionality:

```solidity
contract TestERC20 {
    event Transfer(address indexed from, address indexed to, uint256 value);
    event Approval(address indexed owner, address indexed spender, uint256 value);
    
    function transfer(address to, uint256 value) public returns (bool);
    function approve(address spender, uint256 value) public returns (bool);
    function transferFrom(address from, address to, uint256 value) public returns (bool);
}
```

Used in API integration tests to generate real Transfer and Approval events.

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

### API Integration Tests (`api_integration_test.go`)

Comprehensive REST API integration tests that validate all API endpoints using a real ERC20 contract deployment.

#### TestAPI_IntegrationWithERC20

End-to-end test covering all REST API functionality:

**Test Setup:**

- Deploys minimal ERC20 contract (`TestERC20.sol`) to Anvil
- Generates test transactions (3 transfers, 1 approval)
- Manually indexes events synchronously using `HandleLogs()`
- Starts API server and runs 14 test cases

**Test Coverage:**

1. **GET /health** - Health check returns status and indexer info
2. **GET /api/v1/indexers** - Lists all registered indexers with endpoints
3. **GET /api/v1/indexers/{name}/stats** - Returns event counts and block ranges
4. **Query all transfers** - Returns all Transfer events (4 total: 1 deployment + 3 test)
5. **Pagination** - Tests `limit` and `offset` parameters
6. **Block filtering** - Tests `from_block` and `to_block` filtering
7. **Address filtering** - Tests `address` parameter (participant filtering)
8. **Query approvals** - Returns Approval events separately
9. **Case-insensitive event_type** - Validates `TRANSFER` works like `transfer`
10. **Sorting** - Tests `sort_order=desc` for newest-first ordering
11. **Error: Invalid indexer** - Returns 404 for non-existent indexer
12. **Error: Missing event_type** - Validates required parameter handling
13. **Error: Invalid event_type** - Validates event type validation
14. **CORS headers** - Verifies CORS middleware functionality

**Key Features:**

- Synchronous indexing for deterministic results
- No async complexity or polling
- Uses mock coordinator for testing
- Validates JSON response structures
- Tests all query parameters
- Verifies error handling and status codes

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

### Run Reorg Tests Only

```bash
go test -tags=integration -v ./tests/... -run TestReorg -timeout 5m
```

### Run API Tests Only

```bash
go test -v ./tests/... -run TestAPI -timeout 2m
```

Note: API tests don't require the `integration` build tag as they use a simpler setup.

### Run Specific Test

```bash
go test -tags=integration -v ./tests/... -run TestReorg_SimpleBlockReplacement
go test -v ./tests/... -run TestAPI_IntegrationWithERC20
```

### Run with Detailed Logging

```bash
go test -tags=integration -v ./tests/... -timeout 5m 2>&1 | tee integration-test.log
```

## Test Execution Flow

### Reorg Tests

1. **Setup**: Start Anvil, create database, initialize RPC client
2. **Deploy**: Deploy TestEmitter contract to Anvil
3. **Original Chain**: Mine blocks and emit events
4. **Record**: Use ReorgDetector to record block hashes
5. **Simulate Reorg**: Create snapshot, revert, mine alternative blocks
6. **Verify**: Attempt to record new blocks and verify reorg detection

### API Tests

1. **Setup**: Start Anvil, create database, deploy ERC20 contract
2. **Generate Data**: Execute transactions (transfers and approvals)
3. **Index Events**: Manually fetch and index logs synchronously
4. **Start API Server**: Initialize REST API with mock coordinator
5. **Test Endpoints**: Run 14 test cases covering all API functionality
6. **Cleanup**: Gracefully shutdown API server and Anvil

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

- **Reorg Tests:**
  - Simple 2-block reorg: ~5-10 seconds
  - Deep 15-block reorg: ~20-30 seconds
  - Full reorg test suite: ~1-2 minutes

- **API Tests:**
  - API integration test: ~10 seconds
  - Includes contract deployment, transaction generation, and 14 test cases

Anvil is run with manual mining: blocks are mined when transactions are sent or when tests explicitly call `Mine()`, which keeps tests fast and deterministic.

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
