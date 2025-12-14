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
- **Recursive Log Fetching**: Automatically splits queries to handle RPC "too many results" errors.
- **Reorg Detection & Recovery**: Detects chain reorganizations and safely rolls back indexed data.
- **Configurable Database Backend**: Uses SQLite with connection pooling, PRAGMA tuning, and schema migrations.
- **Batch & Chunked Downloading**: Efficiently downloads logs in configurable block ranges.
- **Comprehensive Test Suite**: Includes unit and integration tests for all major components.
- **Example Indexers**: Production-grade ERC20 token indexer included as a template.

## ⚡ Performance

ChainIndexor is optimized for:

- Fast initial syncs and incremental updates.
- Minimal RPC calls via batching and chunking.
- Safe operation under RPC rate limits and large data volumes.
- Multi-indexer support with independent start blocks and schemas.

## 🛠️ Usage

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
```

### Downloader Configuration

The downloader is responsible for fetching logs from the blockchain and coordinating indexers.

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `rpc_url` | string | Yes | - | Ethereum RPC endpoint URL (HTTP/HTTPS/WebSocket) |
| `chunk_size` | uint64 | No | 5000 | Number of blocks to fetch per `eth_getLogs` call. Adjust based on RPC limits |
| `finality` | string | No | "finalized" | Block finality mode: `"finalized"`, `"safe"`, or `"latest"` |
| `finalized_lag` | uint64 | No | 0 | Blocks behind head to consider finalized (only used when `finality: "latest"`) |
| `db` | object | Yes | - | Database configuration for the downloader |
| `retention_policy` | object | No | - | Optional log retention policy configuration |

#### Database Configuration

SQLite database settings for optimal performance:

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
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

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `max_db_size_mb` | uint64 | No | 0 | Maximum database size in megabytes. `0` = unlimited. Triggers pruning when exceeded |
| `max_blocks` | uint64 | No | 0 | Maximum number of blocks to retain from finalized block. `0` = keep all blocks |

**How Retention Works:**

- When `max_blocks` is set, blocks older than `(newest_block - max_blocks)` are pruned
- When `max_db_size_mb` is set, oldest blocks are pruned when database exceeds the size limit
- Both policies can be used together; the more aggressive threshold applies
- Pruning runs automatically after log ingestion and includes WAL-aware vacuuming

### Indexer Configuration

Configure one or more indexers to process specific events:

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `name` | string | Yes | - | Unique identifier for this indexer |
| `start_block` | uint64 | No | 0 | Block number to start indexing from. `0` = genesis |
| `db` | object | Yes | - | Database configuration for the indexer (same format as downloader db) |
| `contracts` | array | Yes | - | List of contracts and events to index |

#### Contract Configuration

Each contract specifies which events to monitor:

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `address` | string | Yes | - | Ethereum contract address (hex format with `0x` prefix) |
| `events` | array | Yes | - | List of event signatures to index |

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

**Development Settings:**

- Use `finality: "latest"` for faster local testing
- Disable retention policy or set high limits to keep all data
- Use smaller `chunk_size` to test recursive splitting logic

**Multi-Indexer Best Practices:**

- Each indexer gets its own database for isolation
- Set appropriate `start_block` per indexer to avoid unnecessary syncing
- Use descriptive names for easier monitoring and debugging

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

## 📝 Documentation

- [Configuration Guide](docs/configuration.md)
- [Writing Custom Indexers](docs/indexers.md)
- [Database Schema](docs/database.md)

## 🧪 Testing

Run all tests and coverage:

```bash
make test
make test-coverage
```

## 🤝 Contributing

Contributions are welcome! Please open issues and pull requests for bug fixes, features, and documentation.

## 📄 License

ChainIndexor is Apache-2.0 licensed. See [LICENSE](LICENSE) for details.

## 🙏 Acknowledgements

Built on top of [go-ethereum](https://github.com/ethereum/go-ethereum), [testify](https://github.com/stretchr/testify), and other great open source projects.

---

For questions, support, or collaboration, open an issue or reach out via GitHub Discussions.
