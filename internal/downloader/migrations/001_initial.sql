-- +migrate Down
DROP TABLE IF EXISTS sync_state;

-- +migrate Up
CREATE TABLE IF NOT EXISTS sync_state (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    last_indexed_block INTEGER NOT NULL DEFAULT 0,
    last_indexed_block_hash TEXT NOT NULL DEFAULT '',
    last_indexed_timestamp INTEGER NOT NULL,
    mode TEXT NOT NULL
);

-- Insert initial state
INSERT OR IGNORE INTO sync_state (id, last_indexed_block, last_indexed_block_hash, last_indexed_timestamp, mode)
VALUES (1, 0, '', 0, 'backfill');
