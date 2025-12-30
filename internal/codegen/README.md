# Indexer Code Generator

The ChainIndexor code generator (`indexer-gen`) automatically generates complete, production-ready indexer implementations from Solidity event signatures. This eliminates boilerplate and ensures consistency across indexers.

## Overview

The code generator creates:

- **Event Models** - Go structs with proper types and database tags
- **Indexer Implementation** - Complete event handler with parsing logic
- **Database Migrations** - SQL schema with proper indexes
- **Documentation** - README with usage instructions and schema details

## Installation

Build the generator from source:

```bash
make build-codegen
```

Or use it directly with `go run`:

```bash
go run ./cmd/indexer-gen [flags]
```

## Quick Start

Generate an ERC20 token indexer:

```bash
./bin/indexer-gen \
  --name ERC20 \
  --event "Transfer(address indexed from, address indexed to, uint256 value)" \
  --event "Approval(address indexed owner, address indexed spender, uint256 value)" \
  --output ./indexers/erc20
```

This creates:

```text
indexers/erc20/
├── indexer.go                      # Main indexer implementation
├── models.go                       # Event struct definitions
├── register.go                     # Registry integration
├── migrations/
│   ├── migrations.go               # Migration runner
│   └── 001_initial.sql             # Database schema
└── README.md                       # Documentation
```

## Usage

### Basic Command

```bash
indexer-gen --name <IndexerName> --event "<EventSignature>" [options]
# Or use short flags:
indexer-gen -n <IndexerName> -e "<EventSignature>" [options]
```

### Flags

| Flag | Short | Required | Description | Example |
| ---- | ----- | -------- | ----------- | ------- |
| `--name` | `-n` | Yes | Indexer name (PascalCase) | `ERC20`, `UniswapV3Pool` |
| `--event` | `-e` | Yes | Event signature (can be repeated) | `Transfer(address,address,uint256)` |
| `--output` | `-o` | No | Output directory | `./indexers/erc20` |
| `--package` | `-p` | No | Go package name (defaults to lowercase name) | `erc20` |
| `--import` | `-i` | No | Go import path (auto-detected from go.mod) | `github.com/user/project/indexers/erc20` |
| `--force` | `-f` | No | Overwrite existing files | - |
| `--dry-run` | - | No | Show what would be generated | - |
| `--version` | `-v` | No | Show version information | - |
| `--help` | `-h` | No | Show help message | - |

### Event Signature Format

Event signatures follow Solidity syntax:

```text
EventName(type1 [indexed] [name1], type2 [indexed] [name2], ...)
```

**Supported Types:**

- Addresses: `address`
- Integers: `uint8`, `uint16`, `uint32`, `uint64`, `uint128`, `uint256`, `int8`, `int16`, `int32`, `int64`, `int128`, `int256`
- Bytes: `bytes`, `bytes1`, `bytes2`, ..., `bytes32`
- Other: `bool`, `string`
- Arrays: Any type followed by `[]` (e.g., `address[]`, `uint256[]`)

**Examples:**

```bash
# Simple transfer event
--event "Transfer(address from, address to, uint256 value)"

# With indexed parameters
--event "Transfer(address indexed from, address indexed to, uint256 value)"

# Complex event with arrays
--event "BatchTransfer(address indexed from, address[] to, uint256[] amounts)"

# Multiple events
--event "Transfer(address,address,uint256)" \
--event "Approval(address,address,uint256)"
```

## Examples

### ERC20 Token Indexer

```bash
./bin/indexer-gen \
  --name ERC20 \
  --event "Transfer(address indexed from, address indexed to, uint256 value)" \
  --event "Approval(address indexed owner, address indexed spender, uint256 value)" \
  --output ./indexers/erc20
```

### Uniswap V3 Pool Indexer

```bash
./bin/indexer-gen \
  --name UniswapV3Pool \
  --event "Swap(address indexed sender, address indexed recipient, int256 amount0, int256 amount1, uint160 sqrtPriceX96, uint128 liquidity, int24 tick)" \
  --event "Mint(address sender, address indexed owner, int24 indexed tickLower, int24 indexed tickUpper, uint128 amount, uint256 amount0, uint256 amount1)" \
  --event "Burn(address indexed owner, int24 indexed tickLower, int24 indexed tickUpper, uint128 amount, uint256 amount0, uint256 amount1)" \
  --output ./indexers/uniswapv3pool
```

### NFT Marketplace Indexer

```bash
./bin/indexer-gen \
  --name NFTMarketplace \
  --event "ItemListed(uint256 indexed itemId, address indexed seller, address indexed nftContract, uint256 tokenId, uint256 price)" \
  --event "ItemSold(uint256 indexed itemId, address indexed buyer, address indexed seller, uint256 price)" \
  --event "ItemCancelled(uint256 indexed itemId, address indexed seller)" \
  --output ./indexers/nftmarketplace
```

## Generated Code Structure

### models.go

Defines Go structs for each event with:

- Standard metadata fields (block number, transaction hash, etc.)
- Event-specific parameters with proper Go types
- Meddler tags for database mapping

```go
type Transfer struct {
    ID          int64       `meddler:"id,pk"`
    BlockNumber uint64      `meddler:"block_number"`
    BlockHash   common.Hash `meddler:"block_hash,hash"`
    TxHash      common.Hash `meddler:"tx_hash,hash"`
    TxIndex     uint        `meddler:"tx_index"`
    LogIndex    uint        `meddler:"log_index"`
    From        common.Address `meddler:"from_address,address"`
    To          common.Address `meddler:"to_address,address"`
    Value       string      `meddler:"value"`
}
```

### register.go

Automatically registers the indexer with ChainIndexor's registry system:

```go
func init() {
    indexer.Register("erc20", func(cfg config.IndexerConfig, log *logger.Logger) (indexer.Indexer, error) {
        return NewERC20Indexer(cfg, log)
    })
}
```

This allows the indexer to be:

- Used with the ChainIndexor binary (just add `type: "erc20"` in config)
- Discovered by `./bin/indexer --list-indexers`
- Created automatically from configuration

### indexer.go

Complete indexer implementation with:

- Event topic hash computation
- Log parsing and validation
- Database operations (insert, reorg handling)
- Error handling and logging

Key methods:

- `NewXXXIndexer()` - Constructor
- `Name()` - Returns indexer name
- `GetConfig()` - Returns configuration
- `StartBlock()` - Returns starting block number
- `GetAddresses()` - Returns monitored contract addresses
- `GetTopics()` - Returns event topic hashes
- `HandleLogs()` - Processes new logs
- `HandleReorg()` - Handles chain reorganizations

### migrations/

Database schema files that are automatically applied when the indexer starts:

**migrations.go:**

```go
//go:embed 001_initial.sql
var mig0001 string

func RunMigrations(dbConfig config.DatabaseConfig) error {
    migrations := []db.Migration{
        {ID: "001_initial.sql", SQL: mig0001},
    }
    return db.RunMigrations(dbConfig, migrations)
}
```

**001_initial.sql:**

```sql
-- +migrate Up
CREATE TABLE IF NOT EXISTS transfers (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    block_number INTEGER NOT NULL,
    block_hash TEXT NOT NULL,
    ...
);

CREATE INDEX IF NOT EXISTS idx_transfers_block_number ON transfers(block_number);
...

-- +migrate Down
DROP TABLE IF EXISTS transfers;
```

Migrations run automatically when the indexer is initialized, so no manual migration steps are needed.

### README.md

Comprehensive documentation including:

- Event signatures
- Database schema with all fields and indexes
- Configuration examples
- Usage instructions

## Type Mapping

| Solidity Type | Go Type | Database Type | Notes |
| ------------- | ------- | ------------- | ----- |
| `address` | `common.Address` | `TEXT` | 20-byte hex string |
| `uint8`-`uint64` | `uint8`-`uint64` | `INTEGER` | Native integers |
| `uint128`, `uint256` | `string` | `TEXT` | Stored as decimal string |
| `int8`-`int64` | `int8`-`int64` | `INTEGER` | Signed integers |
| `int128`, `int256` | `string` | `TEXT` | Stored as decimal string |
| `bool` | `bool` | `INTEGER` | 0 or 1 |
| `string` | `string` | `TEXT` | UTF-8 text |
| `bytes` | `[]byte` | `BLOB` | Raw bytes |
| `bytesN` | `[N]byte` | `TEXT` | Hex-encoded |
| `type[]` | `string` | `TEXT` | JSON-encoded array |

## Database Schema

Each event generates a table with:

**Standard Fields:**

- `id` - Auto-incrementing primary key
- `block_number` - Block containing the event
- `block_hash` - Hash of the block
- `tx_hash` - Transaction hash
- `tx_index` - Transaction index in block
- `log_index` - Log index in transaction

**Event Fields:**

- One column per event parameter
- Column names derived from parameter names (snake_case)
- Indexed parameters get `_address` suffix for addresses

**Indexes:**

- `block_number` - For block-based queries
- `tx_hash` - For transaction lookup
- Address fields - For filtering by addresses

## Integration

After generation, integrate the indexer into your application:

### 1. Add to Configuration

```yaml
indexers:
  - name: "ERC20Indexer"
    start_block: 0
    db:
      path: "./data/erc20.sqlite"
    contracts:
      - address: "0xYourContractAddress"
        events:
          - "Transfer(address,address,uint256)"
          - "Approval(address,address,uint256)"
```

### 2. Import and Register

```go
import (
    "github.com/yourproject/indexers/erc20"
)

func main() {
    // Load config
    cfg := loadConfig()
    
    // Create indexer (migrations run automatically)
    indexer, err := erc20.NewERC20Indexer(cfg.Indexers[0], db, logger)
    if err != nil {
        log.Fatal(err)
    }
    
    // Register with orchestrator
    orchestrator.RegisterIndexer(indexer)
}
```

The database schema is automatically created when the indexer is initialized - no manual migration steps required.

## Best Practices

1. **Naming Conventions**
   - Use PascalCase for indexer names (e.g., `ERC20`, `UniswapV3Pool`)
   - Package names will be automatically lowercased
   - Avoid special characters and spaces

2. **Event Signatures**
   - Include `indexed` keyword for indexed parameters
   - Use descriptive parameter names
   - Match the exact Solidity event signature

3. **Output Organization**
   - Group related indexers in subdirectories
   - Use meaningful directory names
   - Keep migrations with the indexer

4. **Version Control**
   - Commit generated code to version control
   - Mark files with `// Code generated by indexer-gen. DO NOT EDIT.`
   - Regenerate if event signatures change

5. **Testing**
   - Verify generated code compiles: `go build ./...`
   - Run linting: `golangci-lint run ./...`
   - Test with actual blockchain data

## Customization

While generated code is marked as auto-generated, you can:

1. **Extend Functionality** - Add helper methods to separate files
2. **Override Behavior** - Wrap the generated indexer with custom logic
3. **Add Validation** - Implement additional checks in wrapper functions

Example wrapper:

```go
// custom_erc20.go
package erc20

func (idx *ERC20Indexer) ValidateTransfer(t *Transfer) error {
    // Add custom validation
    if t.Value == "0" {
        return errors.New("zero value transfer")
    }
    return nil
}
```

## Troubleshooting

### Import Path Issues

If you get import path errors, explicitly set the import path:

```bash
--import "github.com/yourproject/indexers/erc20"
```

### Compilation Errors

Ensure all dependencies are installed:

```bash
go mod tidy
```

## Development

The code generator is implemented in `internal/codegen/`:

- `generator.go` - Main generator logic
- `parser.go` - Event signature parser
- `templates.go` - Code templates
- `types.go` - Type conversion helpers

To modify templates, edit the Go template strings in `templates.go` and rebuild the tool.

## License

The generated code follows the same license as the ChainIndexor project.
