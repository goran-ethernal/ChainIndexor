# ERC20 Indexer

Auto-generated indexer for ERC20 events.

## Events

- `Transfer(address indexed from, address indexed to, uint256 value)`
- `Approval(address indexed owner, address indexed spender, uint256 value)`

## Database Schema

### transfers

| Column | Type | Description |
| ------ | ---- | ----------- |
| id | INTEGER | Primary key |
| block_number | INTEGER | Block number |
| block_hash | TEXT | Block hash |
| tx_hash | TEXT | Transaction hash |
| tx_index | INTEGER | Transaction index |
| log_index | INTEGER | Log index |
| from_address | TEXT | from (address) |
| to_address | TEXT | to (address) |
| value | TEXT | value (uint256) |

**Indexes:**

- `block_number`
- `tx_hash`
- `from_address`
- `to_address`

### approvals

| Column | Type | Description |
| ------ | ---- | ----------- |
| id | INTEGER | Primary key |
| block_number | INTEGER | Block number |
| block_hash | TEXT | Block hash |
| tx_hash | TEXT | Transaction hash |
| tx_index | INTEGER | Transaction index |
| log_index | INTEGER | Log index |
| owner_address | TEXT | owner (address) |
| spender_address | TEXT | spender (address) |
| value | TEXT | value (uint256) |

**Indexes:**

- `block_number`
- `tx_hash`
- `owner_address`
- `spender_address`

## Usage

### 1. Add to your config.yaml

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

### 2. Import in your main.go

```go
import "yourproject/indexers/erc20"

indexer, err := erc20.NewERC20Indexer(cfg, log)
if err != nil {
    log.Fatal(err)
}

orchestrator.RegisterIndexer(indexer)
```

### 3. Run your indexer

```bash
go run ./cmd/indexer
```

## REST API Endpoints

Once you implement the `Queryable` interface and enable the API in your configuration, the following endpoints become available:

### GET /indexers

List all registered indexers.

```bash
curl http://localhost:8080/indexers
```

### GET /indexers/erc20/events

Query ERC20 events with filtering and pagination.

**Query Parameters:**
- `limit` (int, default: 100, max: 1000)
- `offset` (int, default: 0)
- `from_block` (uint64, optional)
- `to_block` (uint64, optional)
- `address` (string, optional)
- `event_type` (string, optional)

**Example:**

```bash
# Get latest 50 events
curl "http://localhost:8080/indexers/erc20/events?limit=50"

# Query with filters
curl "http://localhost:8080/indexers/erc20/events?event_type=Transfer&limit=50"
```

### GET /indexers/erc20/stats

Get indexer statistics including total events and event counts by type.

```bash
curl "http://localhost:8080/indexers/erc20/stats"
```

### GET /indexers/erc20/events/timeseries

Get time-series aggregated event data for analytics.

**Query Parameters:**
- `interval` (string, optional: "hour", "day", "week", default: "day")
- `event_type` (string, optional)
- `from_block` (uint64, optional)
- `to_block` (uint64, optional)

```bash
curl "http://localhost:8080/indexers/erc20/events/timeseries?interval=day"
```

### GET /indexers/erc20/metrics

Get performance and processing metrics.

```bash
curl "http://localhost:8080/indexers/erc20/metrics"
```

### GET /health

Check API and indexer health status.

```bash
curl "http://localhost:8080/health"
```

### Swagger UI

For interactive API documentation, visit:
[http://localhost:8080/swagger/index.html](http://localhost:8080/swagger/index.html)

See the [Code Generator Documentation](../../internal/codegen/README.md#api-integration-optional) for instructions on implementing the `Queryable` interface.

## Generated Files

- `indexer.go` - Main indexer implementation
- `models.go` - Event struct definitions
- `register.go` - Registry integration (for using with ChainIndexor binary)
- `migrations/migrations.go` - Database schema and migrations

## Customization

This indexer was auto-generated. To add custom logic:

1. Create a new file (e.g., `indexer_custom.go`)
2. Add methods to the `ERC20Indexer` struct
3. The generated files won't be overwritten unless you regenerate with `--force`

## Regeneration

To regenerate this indexer after config changes:

```bash
indexer-gen \
  --name "ERC20" \
  --event "Transfer(address indexed from, address indexed to, uint256 value)" \
  --event "Approval(address indexed owner, address indexed spender, uint256 value)" \
  --output ./indexers/erc20 \
  --force
```
