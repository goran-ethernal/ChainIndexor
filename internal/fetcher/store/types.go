package store

import "github.com/ethereum/go-ethereum/common"

// dbLog represents a log entry in the database
type dbLog struct {
	ID          int64          `meddler:"id,pk"`
	Address     common.Address `meddler:"address,address"`
	BlockNumber uint64         `meddler:"block_number"`
	BlockHash   common.Hash    `meddler:"block_hash,hash"`
	TxHash      common.Hash    `meddler:"tx_hash,hash"`
	TxIndex     uint           `meddler:"tx_index"`
	LogIndex    uint           `meddler:"log_index"`
	Topic0      *common.Hash   `meddler:"topic0,hash"`
	Topic1      *common.Hash   `meddler:"topic1,hash"`
	Topic2      *common.Hash   `meddler:"topic2,hash"`
	Topic3      *common.Hash   `meddler:"topic3,hash"`
	Data        []byte         `meddler:"data"`
	CreatedAt   string         `meddler:"created_at"`
}

// dbCoverage represents a coverage range in the database
type dbCoverage struct {
	ID        int64          `meddler:"id,pk"`
	Address   common.Address `meddler:"address,address"`
	FromBlock uint64         `meddler:"from_block"`
	ToBlock   uint64         `meddler:"to_block"`
	CreatedAt string         `meddler:"created_at"`
}

// dbTopicCoverage represents a topic-specific coverage range in the database
type dbTopicCoverage struct {
	ID        int64          `meddler:"id,pk"`
	Address   common.Address `meddler:"address,address"`
	Topic0    common.Hash    `meddler:"topic0,hash"`
	FromBlock uint64         `meddler:"from_block"`
	ToBlock   uint64         `meddler:"to_block"`
	CreatedAt string         `meddler:"created_at"`
}
