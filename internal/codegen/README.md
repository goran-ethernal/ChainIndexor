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

## API Integration (Optional)

The ChainIndexor framework includes an optional REST API for querying indexed events. To enable API support for your generated indexer, implement the `Queryable` interface.

### Queryable Interface

The `Queryable` interface is defined in `pkg/indexer/indexer.go`:

```go
type Queryable interface {
    QueryEvents(ctx context.Context, params QueryParams) ([]EventData, int64, error)
    GetStats(ctx context.Context) (*StatsResponse, error)
    GetEventTypes(ctx context.Context) ([]string, error)
    QueryEventsTimeseries(ctx context.Context, params TimeseriesParams) ([]TimeseriesDataPoint, error)
    GetMetrics(ctx context.Context) (*MetricsResponse, error)
}

type QueryParams struct {
    Limit     int
    Offset    int
    FromBlock *uint64
    ToBlock   *uint64
    Address   *string
    EventType *string
}

type TimeseriesParams struct {
    Interval  string    // "hour", "day", or "week"
    EventType *string
    FromBlock *uint64
    ToBlock   *uint64
}

type StatsResponse struct {
    TotalEvents   int64             `json:"total_events"`
    EventCounts   map[string]int64  `json:"event_counts"`
    EarliestBlock uint64            `json:"earliest_block"`
    LatestBlock   uint64            `json:"latest_block"`
}

type TimeseriesDataPoint struct {
    Period    string `json:"period"`
    EventType string `json:"event_type"`
    Count     int64  `json:"count"`
    MinBlock  uint64 `json:"min_block"`
    MaxBlock  uint64 `json:"max_block"`
}

type MetricsResponse struct {
    EventsPerBlock      float64 `json:"events_per_block"`
    AvgEventsPerDay     float64 `json:"avg_events_per_day"`
    RecentBlocksAnalyzed uint64 `json:"recent_blocks_analyzed"`
    RecentEventsCount   int64   `json:"recent_events_count"`
}
```

### Available API Endpoints

Once you implement the `Queryable` interface and enable the API, the following endpoints become available:

#### 1. Health Check

```text
GET /health
```

Check the health status of the API and all registered indexers.

**Response:**

```json
{
  "timestamp": "2024-01-15T10:30:00Z",
  "status": "healthy",
  "uptime": "1h30m45s",
  "indexers": [
    {
      "name": "erc20",
      "healthy": true,
      "latest_block": 19234567,
      "event_count": 1250000
    }
  ]
}
```

#### 2. List Indexers

```text
GET /indexers
```

Returns all registered indexers with their event types and available endpoints.

**Response:**

```json
[
  {
    "type": "erc20",
    "name": "erc20",
    "event_types": ["Transfer", "Approval"],
    "endpoints": [
      "/indexers/erc20/events",
      "/indexers/erc20/stats"
    ]
  }
]
```

#### 3. Query Events

```text
GET /indexers/{name}/events
```

Query events with filtering and pagination.

**Query Parameters:**

- `limit` (int, default: 100, max: 1000) - Events per page
- `offset` (int, default: 0) - Pagination offset
- `from_block` (uint64, optional) - Filter from block number
- `to_block` (uint64, optional) - Filter to block number
- `address` (string, optional) - Filter by address
- `event_type` (string, optional) - Filter by event type
- `sort_by` (string, optional) - Sort field
- `sort_order` (string, optional) - "asc" or "desc"

**Response:**

```json
{
  "events": [...],
  "pagination": {
    "total": 50000,
    "limit": 100,
    "offset": 0,
    "has_more": true
  }
}
```

#### 4. Get Stats

```text
GET /indexers/{name}/stats
```

Get indexer statistics.

**Response:**

```json
{
  "total_events": 1250000,
  "event_counts": {
    "Transfer": 1000000,
    "Approval": 250000
  },
  "earliest_block": 12373391,
  "latest_block": 19234567
}
```

#### 5. Get Timeseries Data

```text
GET /indexers/{name}/events/timeseries
```

Get time-series aggregated event data for analytics and charting.

**Query Parameters:**

- `interval` (string, optional, default: "day") - "hour", "day", or "week"
- `event_type` (string, optional) - Filter by event type
- `from_block` (uint64, optional) - From block
- `to_block` (uint64, optional) - To block

**Response:**

```json
[
  {
    "period": "2024-01-15T00:00:00Z",
    "event_type": "Transfer",
    "count": 5000,
    "min_block": 19230000,
    "max_block": 19234567
  }
]
```

#### 6. Get Metrics

```text
GET /indexers/{name}/metrics
```

Get performance and processing metrics.

**Response:**

```json
{
  "events_per_block": 12.5,
  "avg_events_per_day": 150000.25,
  "recent_blocks_analyzed": 1000,
  "recent_events_count": 12500
}
```

### Implementation Example

Here's how to implement API support for a generated ERC20 indexer:

#### 1. Implement QueryEvents

Add a method to your indexer struct that queries events from your database:

```go
func (idx *ERC20Indexer) QueryEvents(ctx context.Context, params indexer.QueryParams) ([]indexer.EventData, int64, error) {
    query := `
        SELECT block_number, transaction_hash, log_index, address, 
               event_type, event_data, block_timestamp
        FROM events
        WHERE 1=1
    `
    args := []interface{}{}
    argCount := 1

    // Apply filters
    if params.FromBlock != nil {
        query += fmt.Sprintf(" AND block_number >= $%d", argCount)
        args = append(args, *params.FromBlock)
        argCount++
    }
    if params.ToBlock != nil {
        query += fmt.Sprintf(" AND block_number <= $%d", argCount)
        args = append(args, *params.ToBlock)
        argCount++
    }
    if params.Address != nil {
        query += fmt.Sprintf(" AND LOWER(address) = LOWER($%d)", argCount)
        args = append(args, *params.Address)
        argCount++
    }
    if params.EventType != nil {
        query += fmt.Sprintf(" AND event_type = $%d", argCount)
        args = append(args, *params.EventType)
        argCount++
    }

    // Get total count for pagination
    countQuery := strings.Replace(query, "SELECT block_number, transaction_hash, log_index, address, event_type, event_data, block_timestamp", "SELECT COUNT(*)", 1)
    row := idx.db.QueryRowContext(ctx, countQuery, args...)
    var total int64
    if err := row.Scan(&total); err != nil {
        return nil, 0, fmt.Errorf("count events: %w", err)
    }

    // Add ordering and pagination
    query += " ORDER BY block_number DESC, log_index DESC"
    query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", argCount, argCount+1)
    args = append(args, params.Limit, params.Offset)

    rows, err := idx.db.QueryContext(ctx, query, args...)
    if err != nil {
        return nil, 0, fmt.Errorf("query events: %w", err)
    }
    defer rows.Close()

    var results []indexer.EventData
    for rows.Next() {
        var evt indexer.EventData
        var eventDataJSON []byte
        
        err := rows.Scan(
            &evt.BlockNumber,
            &evt.TransactionHash,
            &evt.LogIndex,
            &evt.Address,
            &evt.EventType,
            &eventDataJSON,
            &evt.BlockTimestamp,
        )
        if err != nil {
            return nil, 0, fmt.Errorf("scan event: %w", err)
        }

        // Unmarshal event data based on event type
        switch evt.EventType {
        case "Transfer":
            var transfer Transfer
            if err := json.Unmarshal(eventDataJSON, &transfer); err != nil {
                return nil, 0, fmt.Errorf("unmarshal Transfer: %w", err)
            }
            evt.EventData = transfer
        case "Approval":
            var approval Approval
            if err := json.Unmarshal(eventDataJSON, &approval); err != nil {
                return nil, 0, fmt.Errorf("unmarshal Approval: %w", err)
            }
            evt.EventData = approval
        }

        results = append(results, evt)
    }

    return results, total, rows.Err()
}
```

#### 2. Implement GetStats

```go
func (idx *ERC20Indexer) GetStats(ctx context.Context) (*indexer.StatsResponse, error) {
    stats := &indexer.StatsResponse{
        EventCounts: make(map[string]int64),
    }

    // Get total events and latest block
    row := idx.db.QueryRowContext(ctx, `
        SELECT COUNT(*), MIN(block_number), MAX(block_number)
        FROM events
    `)
    
    err := row.Scan(&stats.TotalEvents, &stats.EarliestBlock, &stats.LatestBlock)
    if err != nil {
        return nil, fmt.Errorf("get total stats: %w", err)
    }

    // Get event counts by type
    rows, err := idx.db.QueryContext(ctx, `
        SELECT event_type, COUNT(*)
        FROM events
        GROUP BY event_type
    `)
    if err != nil {
        return nil, fmt.Errorf("get event counts: %w", err)
    }
    defer rows.Close()

    for rows.Next() {
        var eventType string
        var count int64
        if err := rows.Scan(&eventType, &count); err != nil {
            return nil, fmt.Errorf("scan event count: %w", err)
        }
        stats.EventCounts[eventType] = count
    }

    return stats, rows.Err()
}
```

#### 3. Implement Other Interface Methods

Implement `GetEventTypes()`, `QueryEventsTimeseries()`, and `GetMetrics()` similarly:

```go
func (idx *ERC20Indexer) GetEventTypes(ctx context.Context) ([]string, error) {
    // Return list of event types
}

func (idx *ERC20Indexer) QueryEventsTimeseries(ctx context.Context, params indexer.TimeseriesParams) ([]indexer.TimeseriesDataPoint, error) {
    // Return time-aggregated event data
}

func (idx *ERC20Indexer) GetMetrics(ctx context.Context) (*indexer.MetricsResponse, error) {
    // Return performance metrics
}
```

### Database Schema Requirements

Ensure your database schema includes:

1. An `events` table with proper indexing:

   ```sql
   CREATE INDEX idx_events_block_number ON events(block_number);
   CREATE INDEX idx_events_address ON events(address);
   CREATE INDEX idx_events_type ON events(event_type);
   ```

### Enabling the API

Enable the API in your configuration:

```yaml
api:
  enabled: true
  listen_address: ":8080"
  cors:
    allowed_origins: ["*"]
```

### Interactive API Documentation

For detailed API documentation with live testing capabilities, visit:
**[http://localhost:8080/swagger/index.html](http://localhost:8080/swagger/index.html)**

See the [main README](../../README.md#-rest-api-configuration) for comprehensive API configuration options.

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
- Discovered by `./bin/indexer list`
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
