-- +migrate Down
DROP INDEX IF EXISTS idx_approvals_spender;
DROP INDEX IF EXISTS idx_approvals_owner;
DROP INDEX IF EXISTS idx_approvals_block_number;
DROP TABLE IF EXISTS approvals;

DROP INDEX IF EXISTS idx_transfers_to;
DROP INDEX IF EXISTS idx_transfers_from;
DROP INDEX IF EXISTS idx_transfers_block_number;
DROP TABLE IF EXISTS transfers;

-- +migrate Up
CREATE TABLE IF NOT EXISTS transfers (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    block_number INTEGER NOT NULL,
    block_hash TEXT NOT NULL,
    tx_hash TEXT NOT NULL,
    tx_index INTEGER NOT NULL,
    log_index INTEGER NOT NULL,
    from_address TEXT NOT NULL,
    to_address TEXT NOT NULL,
    value TEXT NOT NULL,
    UNIQUE(tx_hash, log_index)
);

CREATE INDEX IF NOT EXISTS idx_transfers_block_number ON transfers(block_number);
CREATE INDEX IF NOT EXISTS idx_transfers_from ON transfers(from_address);
CREATE INDEX IF NOT EXISTS idx_transfers_to ON transfers(to_address);

CREATE TABLE IF NOT EXISTS approvals (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    block_number INTEGER NOT NULL,
    block_hash TEXT NOT NULL,
    tx_hash TEXT NOT NULL,
    tx_index INTEGER NOT NULL,
    log_index INTEGER NOT NULL,
    owner_address TEXT NOT NULL,
    spender_address TEXT NOT NULL,
    value TEXT NOT NULL,
    UNIQUE(tx_hash, log_index)
);

CREATE INDEX IF NOT EXISTS idx_approvals_block_number ON approvals(block_number);
CREATE INDEX IF NOT EXISTS idx_approvals_owner ON approvals(owner_address);
CREATE INDEX IF NOT EXISTS idx_approvals_spender ON approvals(spender_address);
