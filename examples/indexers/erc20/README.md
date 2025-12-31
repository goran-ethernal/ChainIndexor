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
