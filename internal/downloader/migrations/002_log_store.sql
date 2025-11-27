-- +migrate Down
DROP INDEX IF EXISTS idx_topic_coverage_address_topic_range;
DROP INDEX IF EXISTS idx_topic_coverage_address_topic;
DROP TABLE IF EXISTS topic_coverage;
DROP INDEX IF EXISTS idx_log_coverage_address_range;
DROP INDEX IF EXISTS idx_log_coverage_address;
DROP TABLE IF EXISTS log_coverage;
DROP INDEX IF EXISTS idx_event_logs_tx_hash;
DROP INDEX IF EXISTS idx_event_logs_block_hash;
DROP INDEX IF EXISTS idx_event_logs_block_number;
DROP INDEX IF EXISTS idx_event_logs_address_block;
DROP TABLE IF EXISTS event_logs;

-- +migrate Up
CREATE TABLE IF NOT EXISTS event_logs (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	address TEXT NOT NULL,
	block_number INTEGER NOT NULL,
	block_hash TEXT NOT NULL,
	tx_hash TEXT NOT NULL,
	tx_index INTEGER NOT NULL,
	log_index INTEGER NOT NULL,
	topic0 TEXT,
	topic1 TEXT,
	topic2 TEXT,
	topic3 TEXT,
	data BLOB,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
	
	-- Composite unique constraint to prevent duplicates
	UNIQUE(address, block_number, tx_hash, log_index)
);

-- Indexes for efficient querying
CREATE INDEX IF NOT EXISTS idx_event_logs_address_block ON event_logs(address, block_number);
CREATE INDEX IF NOT EXISTS idx_event_logs_block_number ON event_logs(block_number);
CREATE INDEX IF NOT EXISTS idx_event_logs_block_hash ON event_logs(block_hash);
CREATE INDEX IF NOT EXISTS idx_event_logs_tx_hash ON event_logs(tx_hash);

CREATE TABLE IF NOT EXISTS log_coverage (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	address TEXT NOT NULL,
	from_block INTEGER NOT NULL,
	to_block INTEGER NOT NULL,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
	
	-- Ensure no overlapping ranges for the same address
	UNIQUE(address, from_block, to_block)
);

-- Indexes for efficient range queries
CREATE INDEX IF NOT EXISTS idx_log_coverage_address ON log_coverage(address);
CREATE INDEX IF NOT EXISTS idx_log_coverage_address_range ON log_coverage(address, from_block, to_block);

CREATE TABLE IF NOT EXISTS topic_coverage (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	address TEXT NOT NULL,
	topic0 TEXT NOT NULL,
	from_block INTEGER NOT NULL,
	to_block INTEGER NOT NULL,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
	
	-- Ensure no overlapping ranges for the same address and topic
	UNIQUE(address, topic0, from_block, to_block)
);

-- Indexes for efficient topic-based range queries
CREATE INDEX IF NOT EXISTS idx_topic_coverage_address_topic ON topic_coverage(address, topic0);
CREATE INDEX IF NOT EXISTS idx_topic_coverage_address_topic_range ON topic_coverage(address, topic0, from_block, to_block);
