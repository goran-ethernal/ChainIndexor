-- +migrate Down
DROP INDEX IF EXISTS idx_block_hash;
DROP TABLE IF EXISTS block_hashes;

-- +migrate Up
CREATE TABLE IF NOT EXISTS block_hashes (
    block_number INTEGER PRIMARY KEY,
    block_hash TEXT NOT NULL,
    parent_hash TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_block_hash ON block_hashes(block_hash);
