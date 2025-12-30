-- +migrate Down
DROP INDEX IF EXISTS idx_transfers_from_address;
DROP INDEX IF EXISTS idx_transfers_to_address;
DROP INDEX IF EXISTS idx_transfers_tx_hash;
DROP INDEX IF EXISTS idx_transfers_block_number;
DROP TABLE IF EXISTS transfers;


DROP INDEX IF EXISTS idx_approvals_owner_address;
DROP INDEX IF EXISTS idx_approvals_spender_address;
DROP INDEX IF EXISTS idx_approvals_tx_hash;
DROP INDEX IF EXISTS idx_approvals_block_number;
DROP TABLE IF EXISTS approvals;

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
CREATE INDEX IF NOT EXISTS idx_transfers_tx_hash ON transfers(tx_hash);
CREATE INDEX IF NOT EXISTS idx_transfers_from_address ON transfers(from_address);
CREATE INDEX IF NOT EXISTS idx_transfers_to_address ON transfers(to_address);


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
CREATE INDEX IF NOT EXISTS idx_approvals_tx_hash ON approvals(tx_hash);
CREATE INDEX IF NOT EXISTS idx_approvals_owner_address ON approvals(owner_address);
CREATE INDEX IF NOT EXISTS idx_approvals_spender_address ON approvals(spender_address);


