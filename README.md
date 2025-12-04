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
