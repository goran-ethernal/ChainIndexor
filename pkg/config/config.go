package config

import "fmt"

// Config represents the complete configuration for the ChainIndexor.
type Config struct {
	// Downloader contains the downloader configuration
	Downloader DownloaderConfig `yaml:"downloader"`

	// Indexers contains the configuration for all indexers
	Indexers []IndexerConfig `yaml:"indexers"`
}

// DownloaderConfig represents the configuration for the downloader.
type DownloaderConfig struct {
	// RPCURL is the Ethereum RPC endpoint URL
	RPCURL string `yaml:"rpc_url"`

	// StartBlock is the block number to start backfilling from
	StartBlock uint64 `yaml:"start_block"`

	// ChunkSize is the block range per eth_getLogs call
	ChunkSize uint64 `yaml:"chunk_size"`

	// Finality specifies the finality mode: "finalized", "safe", or "latest"
	Finality string `yaml:"finality"`

	// FinalizedLag is the number of blocks behind head to consider finalized
	// Only used when Finality is set to "latest"
	FinalizedLag uint64 `yaml:"finalized_lag"`

	// DB contains database configuration for the downloader
	DB DatabaseConfig `yaml:"db"`
}

// ApplyDefaults sets default values for optional downloader configuration fields.
func (d *DownloaderConfig) ApplyDefaults() {
	// Apply downloader defaults
	if d.ChunkSize == 0 {
		d.ChunkSize = 5000
	}
	if d.Finality == "" {
		d.Finality = "finalized"
	}

	// Apply database defaults
	d.DB.ApplyDefaults()
}

// DatabaseConfig represents database configuration.
type DatabaseConfig struct {
	// Path is the file path to the SQLite database
	Path string `yaml:"path"`

	// JournalMode sets the SQLite journal mode (e.g., "WAL", "DELETE")
	// WAL mode is recommended for better concurrency
	JournalMode string `yaml:"journal_mode"`

	// Synchronous sets the synchronization level ("FULL", "NORMAL", "OFF")
	// NORMAL provides a good balance between safety and performance
	Synchronous string `yaml:"synchronous"`

	// BusyTimeout is the time in milliseconds to wait when the database is locked
	BusyTimeout int `yaml:"busy_timeout"`

	// CacheSize is the size of the page cache (negative = KB, positive = pages)
	CacheSize int `yaml:"cache_size"`

	// MaxOpenConnections is the maximum number of open database connections
	MaxOpenConnections int `yaml:"max_open_connections"`

	// MaxIdleConnections is the maximum number of idle connections in the pool
	MaxIdleConnections int `yaml:"max_idle_connections"`

	// EnableForeignKeys enables foreign key constraint enforcement
	EnableForeignKeys bool `yaml:"enable_foreign_keys"`
}

// ApplyDefaults sets default values for optional database configuration fields.
func (d *DatabaseConfig) ApplyDefaults() {
	if d.JournalMode == "" {
		d.JournalMode = "WAL"
	}
	if d.Synchronous == "" {
		d.Synchronous = "NORMAL"
	}
	if d.BusyTimeout == 0 {
		d.BusyTimeout = 5000
	}
	if d.CacheSize == 0 {
		d.CacheSize = 10000
	}
	if d.MaxOpenConnections == 0 {
		d.MaxOpenConnections = 25
	}
	if d.MaxIdleConnections == 0 {
		d.MaxIdleConnections = 5
	}
	// EnableForeignKeys defaults to false (zero value)
}

// IndexerConfig represents the configuration for a single indexer.
type IndexerConfig struct {
	// Name is a unique identifier for this indexer
	Name string `yaml:"name"`

	// DBPath is the file path to this indexer's SQLite database
	DBPath string `yaml:"db_path"`

	// Contracts contains the list of contracts to index
	Contracts []ContractConfig `yaml:"contracts"`
}

// ContractConfig represents a contract and its events to index.
type ContractConfig struct {
	// Address is the contract address to monitor
	Address string `yaml:"address"`

	// Events is the list of event signatures to index
	// Format: "EventName(type1, type2, ...)"
	Events []string `yaml:"events"`
}

// ApplyDefaults sets default values for optional configuration fields.
func (c *Config) ApplyDefaults() {
	// Apply downloader defaults (which includes DB defaults)
	c.Downloader.ApplyDefaults()
}

// Validate checks if the configuration is valid.
func (c *Config) Validate() error {
	// Validate downloader configuration
	if c.Downloader.RPCURL == "" {
		return fmt.Errorf("downloader.rpc_url is required")
	}

	if c.Downloader.Finality != "finalized" && c.Downloader.Finality != "safe" && c.Downloader.Finality != "latest" {
		return fmt.Errorf("downloader.finality must be one of: 'finalized', 'safe', or 'latest'")
	}

	if c.Downloader.DB.Path == "" {
		return fmt.Errorf("downloader.db.path is required")
	}

	// Validate database settings with defaults
	if c.Downloader.DB.JournalMode != "" && c.Downloader.DB.JournalMode != "WAL" &&
		c.Downloader.DB.JournalMode != "DELETE" && c.Downloader.DB.JournalMode != "TRUNCATE" &&
		c.Downloader.DB.JournalMode != "PERSIST" && c.Downloader.DB.JournalMode != "MEMORY" {
		return fmt.Errorf("downloader.db.journal_mode must be one of: WAL, DELETE, TRUNCATE, PERSIST, MEMORY")
	}

	if c.Downloader.DB.Synchronous != "" && c.Downloader.DB.Synchronous != "FULL" &&
		c.Downloader.DB.Synchronous != "NORMAL" && c.Downloader.DB.Synchronous != "OFF" {
		return fmt.Errorf("downloader.db.synchronous must be one of: FULL, NORMAL, OFF")
	}

	if len(c.Indexers) == 0 {
		return fmt.Errorf("at least one indexer must be configured")
	}

	indexerNames := make(map[string]bool)
	for i, indexer := range c.Indexers {
		if indexer.Name == "" {
			return fmt.Errorf("indexer[%d]: name is required", i)
		}

		if indexerNames[indexer.Name] {
			return fmt.Errorf("indexer[%d]: duplicate indexer name '%s'", i, indexer.Name)
		}
		indexerNames[indexer.Name] = true

		if indexer.DBPath == "" {
			return fmt.Errorf("indexer[%d] (%s): db_path is required", i, indexer.Name)
		}

		if len(indexer.Contracts) == 0 {
			return fmt.Errorf("indexer[%d] (%s): at least one contract must be configured", i, indexer.Name)
		}

		for j, contract := range indexer.Contracts {
			if contract.Address == "" {
				return fmt.Errorf("indexer[%d] (%s), contract[%d]: address is required", i, indexer.Name, j)
			}

			if len(contract.Events) == 0 {
				return fmt.Errorf("indexer[%d] (%s), contract[%d]: at least one event must be configured", i, indexer.Name, j)
			}
		}
	}

	return nil
}
