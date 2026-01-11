# ChainIndexor

[![CI](https://github.com/goran-ethernal/ChainIndexor/actions/workflows/ci.yml/badge.svg)](https://github.com/goran-ethernal/ChainIndexor/actions/workflows/ci.yml)
[![Lint](https://github.com/goran-ethernal/ChainIndexor/actions/workflows/lint.yml/badge.svg)](https://github.com/goran-ethernal/ChainIndexor/actions/workflows/lint.yml)
[![Tests](https://github.com/goran-ethernal/ChainIndexor/actions/workflows/test.yml/badge.svg)](https://github.com/goran-ethernal/ChainIndexor/actions/workflows/test.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/goran-ethernal/ChainIndexor)](https://goreportcard.com/report/github.com/goran-ethernal/ChainIndexor)

ChainIndexor is a high-performance, modular blockchain log indexer and event processor for Ethereum and EVM-compatible chains. It enables fast, reliable, and scalable indexing of smart contract events, making it easy to build analytics, dashboards, and backend services on top of blockchain data.

## 🚀 Purpose & Overview

ChainIndexor is designed to:

- Efficiently fetch, filter, and store blockchain logs and events.
- Support custom indexers for any contract/event type.
- Handle large-scale data, reorgs, and RPC limitations robustly.
- Provide a flexible foundation for explorers, analytics, and DeFi backends.

## ✨ Features

- **Modular Indexer Framework**: Easily add custom indexers for any contract/event.
- **Code Generation**: Automatically generate production-ready indexers from event signatures. See [Code Generator Documentation](./internal/codegen/README.md).
- **Docker Support**: Production-ready Docker and docker-compose configurations. See [Docker Deployment Guide](./DOCKER.md).
- **Recursive Log Fetching**: Automatically splits queries to handle RPC "too many results" errors.
- **Reorg Detection & Recovery**: Detects chain reorganizations and safely rolls back indexed data.
- **Configurable Database Backend**: Uses SQLite with connection pooling, PRAGMA tuning, and schema migrations.
- **Batch & Chunked Downloading**: Efficiently downloads logs in configurable block ranges.
- **REST API**: Optional HTTP API for querying indexed events with pagination, filtering, CORS support, and comprehensive stats.
- **Prometheus Metrics**: Built-in metrics for monitoring indexing performance, RPC health, database operations, and system resources.
- **Comprehensive Test Suite**: Includes unit and integration tests for all major components.
- **Example Indexers**: Production-grade ERC20 token indexer included as a template.

## ⚡ Performance

ChainIndexor is optimized for:

- Fast initial syncs and incremental updates.
- Minimal RPC calls via batching and chunking.
- Safe operation under RPC rate limits and large data volumes.
- Multi-indexer support with independent start blocks and schemas.

## 🛠️ Usage

ChainIndexor can be used in two ways:

### 1. As a Library (For Custom Indexers) 🔧

Use ChainIndexor as a Go library to build your own custom indexers:

```bash
# Install the library
go get github.com/goran-ethernal/ChainIndexor
```

**Generate your indexer:**

```bash
# Install the code generator
go install github.com/goran-ethernal/ChainIndexor/cmd/indexer-gen@latest

# Generate a custom indexer
indexer-gen \
  --name MyContract \
  --event "MyEvent(address indexed user, uint256 amount)" \
  --output ./indexers/mycontract
```

**Create your main.go:**

```go
package main

import (
    "context"
    "github.com/goran-ethernal/ChainIndexor/pkg/downloader"
    "github.com/goran-ethernal/ChainIndexor/pkg/config"
    
    // Import your custom indexer
    "myproject/indexers/mycontract"
)

func main() {
    cfg, _ := config.LoadFromFile("config.yaml")
    
    dl, _ := downloader.New(cfg.Downloader, /* ... */)
    
    // Register your custom indexer
    idx, _ := mycontract.NewMyContractIndexer(cfg.Indexers[0], log)
    dl.RegisterIndexer(idx)
    
    dl.Download(context.Background(), *cfg)
}
```

**This approach is perfect for:**

- Custom contracts and events not covered by built-in indexers
- Full control over indexing logic and data processing
- Integration with existing Go applications
- Mixing custom indexers with built-in ones

### 2. As a Pre-built Binary (For Built-in Indexers) 📦

For common use cases (ERC20, ERC721, etc.), use the pre-built binary:

```bash
# Build from source
make build

# Or install directly
go install github.com/goran-ethernal/ChainIndexor/cmd/indexer@latest
```

**List available indexer types:**

```bash
./bin/indexer list
```

**Run with configuration:**

```bash
./bin/indexer --config config.yaml
```

**Example config.yaml:**

```yaml
indexers:
  - name: "MyERC20Indexer"
    type: "erc20"  # Built-in indexer type
    start_block: 0
    db:
      path: "./data/erc20.sqlite"
    contracts:
      - address: "0x..."
        events:
          - "Transfer(address,address,uint256)"
          - "Approval(address,address,uint256)"
```

**This approach is perfect for:**

- Standard ERC20/ERC721 token indexing
- Quick setup without writing code
- Running multiple built-in indexers
- Production deployments with common indexers

---

### Quick Start with Code Generator

Generate a custom indexer from event signatures:

```bash
# Build the generator
make build-codegen

# Generate an ERC20 indexer
./bin/indexer-gen \
  --name ERC20 \
  --event "Transfer(address indexed from, address indexed to, uint256 value)" \
  --event "Approval(address indexed owner, address indexed spender, uint256 value)" \
  --output ./indexers/erc20
```

This automatically creates all necessary files: models, indexer logic, migrations, and documentation.

📖 **[Full Code Generator Documentation](./internal/codegen/README.md)**

### 🐳 Docker Deployment

ChainIndexor can be easily deployed using Docker:

**Build the image:**

```bash
docker build -t chainindexor:latest .
```

**Run with docker-compose:**

```bash
# Copy and edit the example config
cp config.example.yaml config.yaml
# Edit config.yaml with your settings

# Start the service
docker-compose up -d

# View logs
docker-compose logs -f

# Stop the service
docker-compose down
```

**Run directly with Docker:**

```bash
docker run -d \
  --name chainindexor \
  -v $(pwd)/config.yaml:/app/config.yaml:ro \
  -v chainindexor-data:/app/data \
  -p 9090:9090 \
  chainindexor:latest
```

**Using the code generator in Docker:**

```bash
docker run --rm \
  -v $(pwd)/indexers:/app/indexers \
  chainindexor:latest \
  /app/indexer-gen \
    --name ERC20 \
    --event "Transfer(address,address,uint256)" \
    --output /app/indexers/erc20
```

The Docker image includes:

- Both `indexer` and `indexer-gen` binaries
- All example configuration files
- Non-root user for security
- Health checks and proper signal handling
- Optimized multi-stage build for minimal image size

### Manual Setup

1. **Configure**: Edit `config.example.yaml` to specify RPC endpoints, indexers, and database settings.
2. **Run Migrations**: Ensure database schemas are up-to-date (automatic on startup).
3. **Implement Indexers**: Use the provided interface to add custom event processors.
4. **Start Indexing**: Run the downloader to begin fetching and indexing logs.

Example:
Run the test in `examples/run_test.go` to test the ChainIndexor.

## ⚙️ Configuration

ChainIndexor supports YAML, JSON, and TOML configuration formats. Below is a comprehensive guide to all configuration options.

### Configuration File Structure

```yaml
downloader:
  # ... downloader settings
  
indexers:
  # ... indexer settings

metrics:
  # ... metrics settings

logging:
  # ... logging settings
```

### Downloader Configuration

The downloader is responsible for fetching logs from the blockchain and coordinating indexers.

| Parameter | Type | Required | Default | Description |
| ----------- | ------ | ---------- | --------- | ------------- |
| `rpc_url` | string | Yes | - | Ethereum RPC endpoint URL (HTTP/HTTPS/WebSocket) |
| `chunk_size` | uint64 | No | 5000 | Number of blocks to fetch per `eth_getLogs` call. Adjust based on RPC limits |
| `finality` | string | No | "finalized" | Block finality mode: `"finalized"`, `"safe"`, or `"latest"` |
| `finalized_lag` | uint64 | No | 0 | Blocks behind head to consider finalized (only used when `finality: "latest"`) |
| `retry` | object | No | - | Optional RPC retry configuration with exponential backoff |
| `db` | object | Yes | - | Database configuration for the downloader |
| `retention_policy` | object | No | - | Optional log retention policy configuration |

#### Retry Configuration

Optional configuration for automatic RPC retry logic with exponential backoff:

| Parameter | Type | Required | Default | Description |
| ----------- | ------ | ---------- | --------- | ------------- |
| `max_attempts` | int | No | 5 | Maximum number of attempts (including initial request) |
| `initial_backoff` | string | No | "1s" | Initial backoff duration before first retry (e.g., `"1s"`, `"500ms"`) |
| `max_backoff` | string | No | "30s" | Maximum backoff duration (cap for exponential growth) |
| `backoff_multiplier` | float | No | 2.0 | Multiplier for exponential backoff (e.g., 2.0 doubles each retry) |

**How Retry Works:**

- Automatically retries failed RPC requests with exponential backoff and jitter (±25%)
- Only retries transient errors: network timeouts, connection failures, rate limits (429), server errors (502/503/504)
- Non-retryable errors (invalid parameters, auth failures) fail immediately
- Respects context deadlines and cancellation during retry attempts
- Tracks retry attempts via `chainindexor_rpc_retries_total` Prometheus metric

**Backoff Example** (with 1s initial, 2.0 multiplier):

- Attempt 1: Immediate
- Attempt 2: ~1s wait
- Attempt 3: ~2s wait
- Attempt 4: ~4s wait
- Attempt 5: ~8s wait (capped at max_backoff)

#### Database Configuration

SQLite database settings for optimal performance:

| Parameter | Type | Required | Default | Description |
| ----------- | ------ | ---------- | --------- | ------------- |
| `path` | string | Yes | - | File path to the SQLite database |
| `journal_mode` | string | No | "WAL" | SQLite journal mode: `"WAL"`, `"DELETE"`, `"TRUNCATE"`, `"PERSIST"`, `"MEMORY"`. WAL recommended for concurrency |
| `synchronous` | string | No | "NORMAL" | Synchronization level: `"FULL"`, `"NORMAL"`, `"OFF"`. NORMAL balances safety and performance |
| `busy_timeout` | int | No | 5000 | Milliseconds to wait when database is locked |
| `cache_size` | int | No | 10000 | Page cache size (negative = KB, positive = pages). Higher values improve performance |
| `max_open_connections` | int | No | 25 | Maximum number of open database connections |
| `max_idle_connections` | int | No | 5 | Maximum number of idle connections in the pool |
| `enable_foreign_keys` | bool | No | false | Enable foreign key constraint enforcement |

#### Retention Policy Configuration

Optional configuration to automatically prune old logs and manage database size:

| Parameter         | Type   | Required | Default | Description                                                                                  |
|-------------------|--------|----------|---------|----------------------------------------------------------------------------------------------|
| `max_db_size_mb`  | uint64 | No       | 0       | Maximum database size in megabytes. `0` = unlimited. Triggers pruning when exceeded          |
| `max_blocks`      | uint64 | No       | 0       | Maximum number of blocks to retain from finalized block. `0` = keep all blocks               |

**How Retention Works:**

- When `max_blocks` is set, blocks older than `(newest_block - max_blocks)` are pruned
- When `max_db_size_mb` is set, oldest blocks are pruned when database exceeds the size limit
- Both policies can be used together; the more aggressive threshold applies
- Pruning runs automatically after log ingestion and includes WAL-aware vacuuming

#### Maintenance Configuration

Optional configuration for automated database maintenance tasks (WAL checkpoints and VACUUM operations):

| Parameter | Type | Required | Default | Description |
| ----------- | ------ | ---------- | --------- | ------------- |
| `enabled` | bool | No | false | Enable background maintenance tasks |
| `check_interval` | string | No | "30m" | How often to run maintenance (e.g., `"5m"`, `"30m"`, `"1h"`) |
| `vacuum_on_startup` | bool | No | false | Run maintenance immediately on startup before indexing begins |
| `wal_checkpoint_mode` | string | No | "TRUNCATE" | WAL checkpoint mode: `"PASSIVE"`, `"FULL"`, `"RESTART"`, `"TRUNCATE"` |

**Maintenance Operations:**

- **WAL Checkpoint**: Moves data from Write-Ahead Log (WAL) file back to main database file
- **VACUUM**: Reclaims fragmented space and optimizes database structure
- Both operations coordinate with active indexing operations to avoid conflicts

**Checkpoint Modes:**

- `PASSIVE`: Non-blocking, skips pages if busy (least aggressive)
- `FULL`: Waits for transactions, checkpoints all pages
- `RESTART`: Like FULL but also resets WAL file
- `TRUNCATE`: Most aggressive - resets and truncates WAL file (recommended for production)

**When to Enable:**

- Essential for long-running indexers to prevent WAL file growth
- Recommended for production deployments
- Disable for short-lived or test environments
- Works seamlessly with retention policies for optimal disk usage

### Indexer Configuration

Configure one or more indexers to process specific events:

| Parameter | Type | Required | Default | Description |
| ----------- | ------ | ---------- | --------- | ------------- |
| `name` | string | Yes | - | Unique identifier for this indexer |
| `start_block` | uint64 | No | 0 | Block number to start indexing from. `0` = genesis |
| `db` | object | Yes | - | Database configuration for the indexer (same format as downloader db) |
| `contracts` | array | Yes | - | List of contracts and events to index |

#### Contract Configuration

Each contract specifies which events to monitor:

| Parameter   | Type   | Required | Default | Description                                                    |
|-------------|--------|----------|---------|----------------------------------------------------------------|
| `address`   | string | Yes      | -       | Ethereum contract address (hex format with `0x` prefix)        |
| `events`    | array  | Yes      | -       | List of event signatures to index                              |

**Event Signature Format:**

```solidity
EventName(type1,type2,...)
```

Examples:

- `Transfer(address,address,uint256)` - ERC20 Transfer
- `Approval(address,address,uint256)` - ERC20 Approval
- `Swap(address,uint256,uint256,uint256,uint256,address)` - Uniswap Swap

### Complete Configuration Example

```yaml
# YAML anchor for reusable database config
common_db: &common_db
  journal_mode: WAL
  synchronous: NORMAL
  busy_timeout: 5000
  cache_size: 10000
  max_open_connections: 25
  max_idle_connections: 5
  enable_foreign_keys: true

downloader:
  rpc_url: "https://mainnet.infura.io/v3/YOUR_API_KEY"
  chunk_size: 5000
  finality: "finalized"
  db:
    <<: *common_db
    path: "./data/downloader.sqlite"
  retention_policy:
    max_db_size_mb: 1000  # Keep database under 1GB
    max_blocks: 10000     # Retain last 10k blocks
  maintenance:
    enabled: true
    check_interval: "30m"      # Run maintenance every 30 minutes
    vacuum_on_startup: true    # Clean database on startup
    wal_checkpoint_mode: "TRUNCATE"  # Aggressive WAL reclamation

indexers:
  - name: "ERC20Indexer"
    start_block: 12000000
    db:
      <<: *common_db
      path: "./data/erc20.sqlite"
    contracts:
      - address: "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48"
        events:
          - "Transfer(address,address,uint256)"
          - "Approval(address,address,uint256)"
      - address: "0xdAC17F958D2ee523a2206206994597C13D831ec7"
        events:
          - "Transfer(address,address,uint256)"

  - name: "UniswapV2Indexer"
    start_block: 10000835
    db:
      <<: *common_db
      path: "./data/uniswap.sqlite"
    contracts:
      - address: "0x5C69bEe701ef814a2B6a3EDD4B1652CB9cc5aA6f"
        events:
          - "PairCreated(address,address,address,uint256)"

metrics:
  enabled: true
  listen_address: ":9090"
  path: "/metrics"
```

### Configuration Tips

**Performance Tuning:**

- Increase `chunk_size` for faster syncing if RPC allows (watch for "query returned more than X results" errors)
- Use WAL mode (`journal_mode: WAL`) for better concurrent read/write performance
- Increase `cache_size` for memory-rich environments
- Use `finality: "latest"` with appropriate `finalized_lag` for faster indexing (less safe for reorgs)

**Production Settings:**

- Use `finality: "finalized"` for maximum safety against reorgs
- Enable `retention_policy` to prevent unbounded database growth
- Set reasonable `max_db_size_mb` based on available storage
- Monitor `max_blocks` to balance data retention needs with performance
- Enable `maintenance` with appropriate `check_interval` (e.g., `"30m"` or `"1h"`)
- Use `wal_checkpoint_mode: "TRUNCATE"` for maximum space reclamation
- Enable `vacuum_on_startup: true` for fresh starts after crashes
- Configure logging levels per component for production monitoring

**Development Settings:**

- Use `finality: "latest"` for faster local testing
- Disable retention policy or set high limits to keep all data
- Use smaller `chunk_size` to test recursive splitting logic
- Enable `logging.development: true` for detailed debug output with stack traces

**Multi-Indexer Best Practices:**

- Each indexer gets its own database for isolation
- Set appropriate `start_block` per indexer to avoid unnecessary syncing
- Use descriptive names for easier monitoring and debugging

## 📊 Metrics Configuration

ChainIndexor includes comprehensive Prometheus metrics for monitoring and observability. Metrics cover indexing progress, RPC operations, database performance, maintenance tasks, reorg detection, and system health.

### Metrics Parameters

| Parameter | Type | Required | Default | Description |
| ----------- | ------ | ---------- | --------- | ------------- |
| `enabled` | bool | No | false | Enable Prometheus metrics collection and HTTP endpoint |
| `listen_address` | string | No | ":9090" | Address and port for the metrics HTTP server |
| `path` | string | No | "/metrics" | HTTP path where metrics are exposed |

### Configuration Example

```yaml
metrics:
  enabled: true
  listen_address: ":9090"
  path: "/metrics"
```

### Accessing Metrics

Once enabled, metrics are available at:

- **Metrics endpoint**: `http://localhost:9090/metrics`
- **Health check**: `http://localhost:9090/health`

### Available Metrics Categories

ChainIndexor provides **33 metrics** across the following categories:

- **Indexing Metrics** (5): Block progress, logs indexed, processing time, indexing rate
- **Finalized Block** (1): Current finalized block from RPC
- **RPC Metrics** (5): Request counts, errors, latency, connections, retries
- **Database Metrics** (4): Query counts, query duration, errors, database size
- **Maintenance Metrics** (7): Maintenance runs, duration, space reclaimed, WAL checkpoints, VACUUM operations
- **Reorg Metrics** (4): Reorg detection, depth, blocks rolled back, timestamps
- **Retention Metrics** (2): Blocks pruned, logs pruned by retention policy
- **System Metrics** (5): Uptime, component health, goroutines, memory usage

### Prometheus Configuration

Add to your `prometheus.yml`:

```yaml
scrape_configs:
  - job_name: 'chainindexor'
    static_configs:
      - targets: ['localhost:9090']
    scrape_interval: 15s
```

### Grafana Dashboards

Use the exposed metrics to create Grafana dashboards for:

- Real-time indexing progress and lag monitoring
- RPC health and performance tracking
- Database query performance analysis
- System resource utilization
- Reorg detection and alerting

### Example Queries

```promql
# Indexing rate (blocks per second)
rate(chainindexor_blocks_processed_total[5m])

# Block lag from finalized
chainindexor_finalized_block - chainindexor_last_indexed_block

# RPC error rate
rate(chainindexor_rpc_errors_total[5m])

# Database query latency (95th percentile)
histogram_quantile(0.95, rate(chainindexor_db_query_duration_seconds_bucket[5m]))

# Component health status
chainindexor_component_health
```

For complete metrics documentation, see [internal/metrics/README.md](internal/metrics/README.md).

## 📊 Logging Configuration

ChainIndexor provides structured logging with per-component log level configuration, allowing you to fine-tune verbosity for different parts of the system.

### Logging Parameters

| Parameter | Type | Required | Default | Description |
| ----------- | ------ | ---------- | --------- | ------------- |
| `default_level` | string | No | "info" | Default log level for all components: `"debug"`, `"info"`, `"warn"`, `"error"` |
| `development` | bool | No | false | Enable development mode (stack traces, colored console output) |
| `component_levels` | map | No | {} | Per-component log level overrides |

### Available Components

| Component | Description |
| ----------- | ------------- |
| `downloader` | Main download orchestration and indexer coordination |
| `log-fetcher` | Blockchain log fetching and RPC interaction |
| `sync-manager` | Sync state management and checkpoint persistence |
| `reorg-detector` | Blockchain reorganization detection |
| `log-store` | Log storage layer and database operations |
| `maintenance` | Database maintenance operations (WAL checkpoint, VACUUM) |

### Configuration Examples

#### Basic Logging Configuration

```yaml
logging:
  default_level: "info"
  development: false
```

#### Per-Component Levels

```yaml
logging:
  default_level: "info"
  development: false
  component_levels:
    downloader: "info"
    log-fetcher: "debug"      # verbose RPC logging
    sync-manager: "info"
    reorg-detector: "warn"    # only warnings and errors
    log-store: "info"
    maintenance: "debug"      # detailed maintenance logs
```

#### Development Mode

```yaml
logging:
  default_level: "debug"
  development: true           # enables stack traces and colored output
  component_levels:
    log-fetcher: "debug"
    maintenance: "debug"
```

### Common Use Cases

**Production Monitoring:**

```yaml
logging:
  default_level: "info"
  development: false
  component_levels:
    reorg-detector: "warn"    # reduce noise from normal operations
    maintenance: "info"       # track maintenance operations
```

**Debugging RPC Issues:**

```yaml
logging:
  default_level: "info"
  component_levels:
    log-fetcher: "debug"      # detailed RPC request/response logging
```

**Debugging Performance:**

```yaml
logging:
  default_level: "info"
  component_levels:
    downloader: "debug"       # indexing throughput
    sync-manager: "debug"     # checkpoint frequency
    log-store: "debug"        # database operation timing
```

**Minimal Logging (High-Performance):**

```yaml
logging:
  default_level: "warn"       # only warnings and errors
  development: false
```

### Log Level Guidelines

- **debug**: Verbose output including internal state, timing, and detailed operations. Use for troubleshooting.
- **info**: Normal operational messages. Good default for production.
- **warn**: Unexpected conditions that don't prevent operation. Alerts for potential issues.
- **error**: Errors that require attention but may allow continued operation.

## 🌐 REST API Configuration

ChainIndexor includes an optional HTTP REST API for querying indexed events in real-time. The API provides endpoints for listing indexers, querying events with pagination and filtering, and retrieving indexer statistics.

### API Parameters

| Parameter | Type | Required | Default | Description |
| ----------- | ------ | ---------- | --------- | ------------- |
| `enabled` | bool | No | false | Enable the REST API HTTP server |
| `listen_address` | string | No | ":8080" | Address and port for the API HTTP server |
| `read_timeout` | string | No | "15s" | Maximum duration for reading the entire request |
| `write_timeout` | string | No | "15s" | Maximum duration before timing out writes of the response |
| `idle_timeout` | string | No | "60s" | Maximum amount of time to wait for the next request |
| `cors` | object | No | - | Optional CORS configuration for cross-origin requests |

#### CORS Configuration

| Parameter | Type | Required | Default | Description |
| ----------- | ------ | ---------- | --------- | ------------- |
| `allowed_origins` | []string | No | ["*"] | List of allowed origins. Use `["*"]` to allow all origins |
| `allowed_methods` | []string | No | ["GET", "POST", "OPTIONS"] | HTTP methods allowed for CORS requests |
| `allowed_headers` | []string | No | ["*"] | HTTP headers allowed in CORS requests |
| `allow_credentials` | bool | No | false | Whether to allow credentials (cookies, authorization headers) |
| `max_age` | int | No | 3600 | How long (in seconds) the results of a preflight request can be cached |

#### Basic API Configuration

```yaml
api:
  enabled: true
  listen_address: ":8080"
```

#### Full API Configuration with CORS

```yaml
api:
  enabled: true
  listen_address: ":8080"
  read_timeout: "30s"
  write_timeout: "30s"
  idle_timeout: "120s"
  cors:
    allowed_origins:
      - "https://mydapp.com"
      - "http://localhost:3000"
    allowed_methods:
      - "GET"
      - "POST"
      - "OPTIONS"
    allowed_headers:
      - "Content-Type"
      - "Authorization"
    allow_credentials: true
    max_age: 7200
```

#### Production CORS Configuration (Specific Origins)

```yaml
api:
  enabled: true
  listen_address: ":8080"
  cors:
    allowed_origins:
      - "https://app.example.com"
      - "https://dashboard.example.com"
    allowed_methods: ["GET", "OPTIONS"]
    allow_credentials: true
```

#### Development CORS Configuration (Permissive)

```yaml
api:
  enabled: true
  listen_address: ":8080"
  cors:
    allowed_origins: ["*"]
    allowed_methods: ["GET", "POST", "OPTIONS"]
    allowed_headers: ["*"]
```

### API Endpoints

Once enabled, the following endpoints are available. For interactive exploration and detailed schema information, visit the **[Swagger UI Documentation](http://localhost:8080/swagger/index.html)** once the API is running.

#### 1. Health Check

**Endpoint:** `GET /health`

**Description:** Check the health status of the API and all registered indexers.

**Response:**

```json
{
  "timestamp": "2024-01-15T10:30:00Z",
  "uptime": "1h30m45s",
  "status": "healthy",
  "indexers": [
    {
      "name": "erc20",
      "type": "erc20",
      "healthy": true,
      "latest_block": 19234567,
      "event_count": 1250000
    }
  ]
}
```

**Example:**

```bash
curl "http://localhost:8080/health"
```

---

#### 2. List All Indexers

**Endpoint:** `GET /indexers`

**Description:** Get a list of all registered indexers with their event types and available endpoints.

**Response:**

```json
[
  {
    "type": "erc20",
    "name": "erc20",
    "event_types": ["Transfer", "Approval"],
    "endpoints": [
      "/api/v1/indexers/erc20/events",
      "/api/v1/indexers/erc20/stats"
    ]
  }
]
```

**Example:**

```bash
curl "http://localhost:8080/indexers"
```

---

#### 3. Query Events

**Endpoint:** `GET /indexers/{name}/events`

**Description:** Retrieve events from a specific indexer with optional filtering, pagination, and sorting.

**Path Parameters:**

- `name` (string, required): Indexer name (e.g., "erc20")

**Query Parameters:**

- `limit` (int, default: 100, max: 1000): Maximum number of events to return
- `offset` (int, default: 0): Number of events to skip for pagination
- `from_block` (uint64, optional): Filter events from this block number
- `to_block` (uint64, optional): Filter events up to this block number
- `address` (string, optional): Filter by contract or participant address (lowercase hex with 0x prefix)
- `event_type` (string, optional): Filter by event type (e.g., "Transfer", "Approval")
- `sort_by` (string, optional): Field to sort by
- `sort_order` (string, optional): Sort order: "asc" or "desc"

**Response:**

```json
{
  "events": [
    {
      "block_number": 19234567,
      "transaction_hash": "0xabc...",
      "log_index": 0,
      "address": "0x123...",
      "event_type": "Transfer",
      "event_data": {
        "from": "0x...",
        "to": "0x...",
        "value": "1000000000000000000"
      },
      "block_timestamp": 1705315800
    }
  ],
  "pagination": {
    "total": 5000,
    "limit": 50,
    "offset": 0,
    "has_more": true
  }
}
```

**Examples:**

```bash
# Get latest 50 Transfer events
curl "http://localhost:8080/indexers/erc20/events?event_type=Transfer&limit=50"

# Get events for specific address with pagination
curl "http://localhost:8080/indexers/erc20/events?address=0x123...&limit=100&offset=100"

# Get events in block range
curl "http://localhost:8080/indexers/erc20/events?from_block=19000000&to_block=19100000"

# Get events sorted by block number in descending order
curl "http://localhost:8080/indexers/erc20/events?limit=50&sort_by=block_number&sort_order=desc"
```

---

#### 4. Get Indexer Statistics

**Endpoint:** `GET /indexers/{name}/stats`

**Description:** Retrieve statistics and status information for a specific indexer.

**Path Parameters:**

- `name` (string, required): Indexer name (e.g., "erc20")

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

**Example:**

```bash
curl "http://localhost:8080/indexers/erc20/stats"
```

---

#### 5. Get Timeseries Event Data

**Endpoint:** `GET /indexers/{name}/events/timeseries`

**Description:** Retrieve events aggregated by time periods (hour, day, or week) with event counts for time-series analysis.

**Path Parameters:**

- `name` (string, required): Indexer name (e.g., "erc20")

**Query Parameters:**

- `interval` (string, optional, default: "day"): Time period interval - "hour", "day", or "week"
- `event_type` (string, optional): Filter by specific event type
- `from_block` (uint64, optional): Filter events from this block number
- `to_block` (uint64, optional): Filter events up to this block number

**Response:**

```json
[
  {
    "period": "2024-01-15T00:00:00Z",
    "event_type": "Transfer",
    "count": 5000,
    "min_block": 19230000,
    "max_block": 19234567
  },
  {
    "period": "2024-01-14T00:00:00Z",
    "event_type": "Transfer",
    "count": 4850,
    "min_block": 19225000,
    "max_block": 19229999
  }
]
```

**Examples:**

```bash
# Get daily timeseries data
curl "http://localhost:8080/indexers/erc20/events/timeseries?interval=day"

# Get hourly timeseries data for Transfer events
curl "http://localhost:8080/indexers/erc20/events/timeseries?interval=hour&event_type=Transfer"

# Get weekly data for a specific block range
curl "http://localhost:8080/indexers/erc20/events/timeseries?interval=week&from_block=19000000&to_block=19234567"
```

---

#### 6. Get Indexer Metrics

**Endpoint:** `GET /indexers/{name}/metrics`

**Description:** Retrieve performance and processing metrics for a specific indexer.

**Path Parameters:**

- `name` (string, required): Indexer name (e.g., "erc20")

**Response:**

```json
{
  "events_per_block": 12.5,
  "avg_events_per_day": 150000.25,
  "recent_blocks_analyzed": 1000,
  "recent_events_count": 12500
}
```

**Example:**

```bash
curl "http://localhost:8080/indexers/erc20/metrics"
```

---

#### Swagger Documentation

For interactive API documentation with full schema information, request examples, and the ability to test endpoints directly:

1. Start the server with API enabled
2. Visit: **[http://localhost:8080/swagger/index.html](http://localhost:8080/swagger/index.html)**
3. All endpoints are documented with detailed parameters, response schemas, and example values

### Response Format

All API responses use JSON format:

```json
{
  "data": { ... },
  "error": null
}
```

Error responses:

```json
{
  "data": null,
  "error": "descriptive error message"
}
```

### Enabling API Support in Generated Indexers

To enable REST API support for a generated indexer, implement the `Queryable` interface. See the [Code Generator Documentation](./internal/codegen/README.md#api-integration-optional) for details.

### API Security Considerations

- **Authentication**: The API currently does not include authentication. Deploy behind a reverse proxy (nginx, Caddy) with authentication if needed.
- **Rate Limiting**: No built-in rate limiting. Use a reverse proxy or API gateway for rate limiting in production.
- **CORS**: Configure `allowed_origins` restrictively in production to prevent unauthorized cross-origin access.
- **Timeouts**: Adjust timeout values based on your query complexity and expected response times.

## 📦 Installation

Clone the repo and build:

```bash
git clone https://github.com/goran-ethernal/ChainIndexor.git
cd ChainIndexor
go build ./...
```

## 🧩 Extending

- Add new indexers in `examples/indexers/`.
- Use the ERC20 indexer as a template for custom event processing.
- Register indexers in your config and main application.

## 📝 Documentation (WIP)

- [Configuration Guide](docs/configuration.md)
- [Writing Custom Indexers](docs/indexers.md)
- [Database Schema](docs/database.md)

## 🧪 Testing

### Unit Tests

Run all unit tests with coverage:

```bash
make test
make test-coverage
```

### Integration Tests

ChainIndexor includes comprehensive integration tests for reorg handling using [Anvil](https://book.getfoundry.sh/anvil/), a local Ethereum test node. Integration tests are located in the `tests/` package.

#### Prerequisites

Install Foundry (which includes Anvil):

```bash
curl -L https://foundry.paradigm.xyz | bash
foundryup
```

Verify Anvil is installed:

```bash
anvil --version
```

#### Running Integration Tests

Integration tests simulate real blockchain reorganizations by:

- Starting a local Anvil test node
- Deploying test contracts and emitting events
- Creating blockchain forks and simulating reorgs
- Verifying reorg detection and handling

Run integration tests:

```bash
# Run all integration tests
go test -tags=integration -v ./tests/... -timeout 5m

# Run a specific integration test
go test -tags=integration -v ./tests/... -run TestReorg_SimpleBlockReplacement

# Run with more verbose output
go test -tags=integration -v ./tests/... -timeout 5m 2>&1 | tee integration-test.log
```

#### Integration Test Scenarios

The test suite covers:

1. **Simple Block Replacement** (`TestReorg_SimpleBlockReplacement`)
   - 2-block reorg with different events
   - Verifies basic reorg detection

2. **Deep Reorg** (`TestReorg_DeepReorg`)
   - 15-block reorg
   - Tests handling of large reorganizations

3. **No Logs on Reorg Chain** (`TestReorg_NoLogsOnReorgChain`)
   - Reorg where new chain has no events
   - Verifies detection even without log differences

4. **More Logs on Reorg Chain** (`TestReorg_NewLogsOnReorgChain`)
   - Reorg where new chain has additional events
   - Tests log count differences

#### CI/CD Integration

Integration tests are designed to run in CI/CD environments:

```yaml
# Example GitHub Actions workflow
- name: Run Integration Tests
  run: |
    curl -L https://foundry.paradigm.xyz | bash
    source ~/.bashrc
    foundryup
    go test -tags=integration -v ./tests/... -timeout 5m
```

#### Test Contract

Integration tests use a simple Solidity contract (`tests/testdata/TestEmitter.sol`) that emits indexed events:

```solidity
contract TestEmitter {
    event TestEvent(uint256 indexed id, address indexed sender, string data);
    
    function emitEvent(uint256 id, string memory data) public {
        emit TestEvent(id, msg.sender, data);
    }
}
```

The contract is compiled with `solc` and Go bindings are generated with `abigen` from go-ethereum.

## 🤝 Contributing

Contributions are welcome! Please open issues and pull requests for bug fixes, features, and documentation.

## 📄 License

ChainIndexor is Apache-2.0 licensed. See [LICENSE](LICENSE) for details.

## 🙏 Acknowledgements

Built on top of [go-ethereum](https://github.com/ethereum/go-ethereum), [testify](https://github.com/stretchr/testify), and other great open source projects.

---

For questions, support, or collaboration, open an issue or reach out via GitHub Discussions.
