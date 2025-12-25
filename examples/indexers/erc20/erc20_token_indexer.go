package erc20

import (
	"database/sql"
	"errors"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/goran-ethernal/ChainIndexor/examples/indexers/erc20/migrations"
	"github.com/goran-ethernal/ChainIndexor/internal/db"
	"github.com/goran-ethernal/ChainIndexor/internal/logger"
	"github.com/goran-ethernal/ChainIndexor/pkg/config"
	"github.com/goran-ethernal/ChainIndexor/pkg/indexer"
	"github.com/russross/meddler"
)

const (
	// ERC20 event constants
	expectedTopicsCount = 3  // ERC20 Transfer and Approval events have 3 topics (event signature + 2 indexed params)
	expectedDataSize    = 32 // ERC20 events have 32 bytes of data (uint256 value)
)

// Compile-time check to ensure ERC20TokenIndexer implements indexer.Indexer interface.
var _ indexer.Indexer = (*ERC20TokenIndexer)(nil)

// Transfer represents an ERC20 Transfer event.
type Transfer struct {
	ID          int64          `meddler:"id,pk"`
	BlockNumber uint64         `meddler:"block_number"`
	BlockHash   common.Hash    `meddler:"block_hash,hash"`
	TxHash      common.Hash    `meddler:"tx_hash,hash"`
	TxIndex     uint           `meddler:"tx_index"`
	LogIndex    uint           `meddler:"log_index"`
	From        common.Address `meddler:"from_address,address"`
	To          common.Address `meddler:"to_address,address"`
	Value       string         `meddler:"value"` // Store as string to handle large uint256
}

// Approval represents an ERC20 Approval event.
type Approval struct {
	ID          int64          `meddler:"id,pk"`
	BlockNumber uint64         `meddler:"block_number"`
	BlockHash   common.Hash    `meddler:"block_hash,hash"`
	TxHash      common.Hash    `meddler:"tx_hash,hash"`
	TxIndex     uint           `meddler:"tx_index"`
	LogIndex    uint           `meddler:"log_index"`
	Owner       common.Address `meddler:"owner_address,address"`
	Spender     common.Address `meddler:"spender_address,address"`
	Value       string         `meddler:"value"` // Store as string to handle large uint256
}

// ERC20TokenIndexer indexes ERC20 token Transfer and Approval events.
type ERC20TokenIndexer struct {
	cfg config.IndexerConfig
	db  *sql.DB
	log *logger.Logger

	// Map of contract addresses to event topic hashes
	eventsToIndex map[common.Address]map[common.Hash]struct{}

	// Event signature hashes for quick lookup
	transferTopic common.Hash
	approvalTopic common.Hash
}

// NewERC20TokenIndexer creates a new ERC20 token indexer.
func NewERC20TokenIndexer(cfg config.IndexerConfig, log *logger.Logger) (*ERC20TokenIndexer, error) {
	// Run migrations to set up the database schema
	if err := migrations.RunMigrations(cfg.DB); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	// Create database connection from config
	database, err := db.NewSQLiteDBFromConfig(cfg.DB)
	if err != nil {
		return nil, fmt.Errorf("failed to create database: %w", err)
	}

	// Calculate event topic hashes
	transferTopic := crypto.Keccak256Hash([]byte("Transfer(address,address,uint256)"))
	approvalTopic := crypto.Keccak256Hash([]byte("Approval(address,address,uint256)"))

	// Build the events to index map
	eventsToIndex := make(map[common.Address]map[common.Hash]struct{})

	for _, contract := range cfg.Contracts {
		topics := make(map[common.Hash]struct{})

		for _, eventSig := range contract.Events {
			topic := crypto.Keccak256Hash([]byte(eventSig))
			topics[topic] = struct{}{}
		}

		// Parse contract address from string
		address := common.HexToAddress(contract.Address)
		eventsToIndex[address] = topics
	}

	return &ERC20TokenIndexer{
		cfg:           cfg,
		db:            database,
		log:           log,
		eventsToIndex: eventsToIndex,
		transferTopic: transferTopic,
		approvalTopic: approvalTopic,
	}, nil
}

// Name returns the name of the indexer.
func (idx *ERC20TokenIndexer) Name() string {
	return idx.cfg.Name
}

// EventsToIndex returns the map of contract addresses to event topic hashes.
func (idx *ERC20TokenIndexer) EventsToIndex() map[common.Address]map[common.Hash]struct{} {
	return idx.eventsToIndex
}

// HandleLogs processes a batch of logs and stores Transfer and Approval events.
func (idx *ERC20TokenIndexer) HandleLogs(logs []types.Log) error {
	if len(logs) == 0 {
		return nil
	}

	tx, err := idx.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			idx.log.Errorf("failed to rollback transaction: %v", err)
		}
	}()

	transferCount := 0
	approvalCount := 0

	for _, log := range logs {
		if len(log.Topics) == 0 {
			continue
		}

		topic := log.Topics[0]

		switch topic {
		case idx.transferTopic:
			transfer, err := idx.parseTransfer(&log)
			if err != nil {
				idx.log.Warnf("failed to parse Transfer event at block %d, tx %s: %v",
					log.BlockNumber, log.TxHash.Hex(), err)
				continue
			}

			if err := meddler.Insert(tx, "transfers", transfer); err != nil {
				return fmt.Errorf("failed to insert transfer: %w", err)
			}
			transferCount++

		case idx.approvalTopic:
			approval, err := idx.parseApproval(&log)
			if err != nil {
				idx.log.Warnf("failed to parse Approval event at block %d, tx %s: %v",
					log.BlockNumber, log.TxHash.Hex(), err)
				continue
			}

			if err := meddler.Insert(tx, "approvals", approval); err != nil {
				return fmt.Errorf("failed to insert approval: %w", err)
			}
			approvalCount++
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	idx.log.Infof("Indexed %d transfers and %d approvals", transferCount, approvalCount)

	return nil
}

// HandleReorg handles a blockchain reorganization by removing data from the reorg point.
func (idx *ERC20TokenIndexer) HandleReorg(blockNum uint64) error {
	tx, err := idx.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			idx.log.Errorf("failed to rollback transaction: %v", err)
		}
	}()

	// Delete transfers from the reorg point
	deleteTransfersQuery := `DELETE FROM transfers WHERE block_number >= ?`
	result, err := tx.Exec(deleteTransfersQuery, blockNum)
	if err != nil {
		return fmt.Errorf("failed to delete transfers: %w", err)
	}
	transfersDeleted, _ := result.RowsAffected()

	// Delete approvals from the reorg point
	deleteApprovalsQuery := `DELETE FROM approvals WHERE block_number >= ?`
	result, err = tx.Exec(deleteApprovalsQuery, blockNum)
	if err != nil {
		return fmt.Errorf("failed to delete approvals: %w", err)
	}
	approvalsDeleted, _ := result.RowsAffected()

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	idx.log.Infof("Handled reorg from block %d: deleted %d transfers and %d approvals",
		blockNum, transfersDeleted, approvalsDeleted)

	return nil
}

// StartBlock returns the block number from which this indexer should start.
func (idx *ERC20TokenIndexer) StartBlock() uint64 {
	return idx.cfg.StartBlock
}

// Close closes the database connection.
func (idx *ERC20TokenIndexer) Close() error {
	return idx.db.Close()
}

// parseTransfer parses a Transfer event from a log.
// Transfer event signature: Transfer(address indexed from, address indexed to, uint256 value)
func (idx *ERC20TokenIndexer) parseTransfer(log *types.Log) (*Transfer, error) {
	if len(log.Topics) != expectedTopicsCount {
		return nil, fmt.Errorf("invalid Transfer event: expected %d topics, got %d",
			expectedTopicsCount, len(log.Topics))
	}

	if len(log.Data) != expectedDataSize {
		return nil, fmt.Errorf("invalid Transfer event: expected %d bytes of data, got %d",
			expectedDataSize, len(log.Data))
	}

	from := common.BytesToAddress(log.Topics[1].Bytes())
	to := common.BytesToAddress(log.Topics[2].Bytes())
	value := new(big.Int).SetBytes(log.Data)

	return &Transfer{
		BlockNumber: log.BlockNumber,
		BlockHash:   log.BlockHash,
		TxHash:      log.TxHash,
		TxIndex:     log.TxIndex,
		LogIndex:    log.Index,
		From:        from,
		To:          to,
		Value:       value.String(),
	}, nil
}

// parseApproval parses an Approval event from a log.
// Approval event signature: Approval(address indexed owner, address indexed spender, uint256 value)
func (idx *ERC20TokenIndexer) parseApproval(log *types.Log) (*Approval, error) {
	if len(log.Topics) != expectedTopicsCount {
		return nil, fmt.Errorf("invalid Approval event: expected %d topics, got %d", expectedTopicsCount, len(log.Topics))
	}

	if len(log.Data) != expectedDataSize {
		return nil, fmt.Errorf("invalid Approval event: expected %d bytes of data, got %d", expectedDataSize, len(log.Data))
	}

	owner := common.BytesToAddress(log.Topics[1].Bytes())
	spender := common.BytesToAddress(log.Topics[2].Bytes())
	value := new(big.Int).SetBytes(log.Data)

	return &Approval{
		BlockNumber: log.BlockNumber,
		BlockHash:   log.BlockHash,
		TxHash:      log.TxHash,
		TxIndex:     log.TxIndex,
		LogIndex:    log.Index,
		Owner:       owner,
		Spender:     spender,
		Value:       value.String(),
	}, nil
}
