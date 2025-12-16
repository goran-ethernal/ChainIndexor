package config

import (
	"fmt"
	"time"
)

// Config represents the complete configuration for the ChainIndexor.
type Config struct {
	// Downloader contains the downloader configuration
	Downloader DownloaderConfig `yaml:"downloader" json:"downloader" toml:"downloader"`

	// Indexers contains the configuration for all indexers
	Indexers []IndexerConfig `yaml:"indexers" json:"indexers" toml:"indexers"`
}

// DownloaderConfig represents the configuration for the downloader.
type DownloaderConfig struct {
	// RPCURL is the Ethereum RPC endpoint URL
	RPCURL string `yaml:"rpc_url" json:"rpc_url" toml:"rpc_url"`

	// ChunkSize is the block range per eth_getLogs call
	ChunkSize uint64 `yaml:"chunk_size" json:"chunk_size" toml:"chunk_size"`

	// Finality specifies the finality mode: "finalized", "safe", or "latest"
	Finality string `yaml:"finality" json:"finality" toml:"finality"`

	// FinalizedLag is the number of blocks behind head to consider finalized
	// Only used when Finality is set to "latest"
	FinalizedLag uint64 `yaml:"finalized_lag" json:"finalized_lag" toml:"finalized_lag"`

	// DB contains database configuration for the downloader
	DB DatabaseConfig `yaml:"db" json:"db" toml:"db"`

	// RetentionPolicy contains optional database retention policy settings
	RetentionPolicy *RetentionPolicyConfig `yaml:"retention_policy,omitempty"`

	// Maintenance contains optional database maintenance settings
	Maintenance *MaintenanceConfig `yaml:"maintenance,omitempty"`
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

	if d.Maintenance != nil {
		d.Maintenance.ApplyDefaults()
	}

	// Apply database defaults
	d.DB.ApplyDefaults()
}

// DatabaseConfig represents database configuration.
type DatabaseConfig struct {
	// Path is the file path to the SQLite database
	Path string `yaml:"path" json:"path" toml:"path"`

	// JournalMode sets the SQLite journal mode (e.g., "WAL", "DELETE")
	// WAL mode is recommended for better concurrency
	JournalMode string `yaml:"journal_mode" json:"journal_mode" toml:"journal_mode"`

	// Synchronous sets the synchronization level ("FULL", "NORMAL", "OFF")
	// NORMAL provides a good balance between safety and performance
	Synchronous string `yaml:"synchronous" json:"synchronous" toml:"synchronous"`

	// BusyTimeout is the time in milliseconds to wait when the database is locked
	BusyTimeout int `yaml:"busy_timeout" json:"busy_timeout" toml:"busy_timeout"`

	// CacheSize is the size of the page cache (negative = KB, positive = pages)
	CacheSize int `yaml:"cache_size" json:"cache_size" toml:"cache_size"`

	// MaxOpenConnections is the maximum number of open database connections
	MaxOpenConnections int `yaml:"max_open_connections" json:"max_open_connections" toml:"max_open_connections"`

	// MaxIdleConnections is the maximum number of idle connections in the pool
	MaxIdleConnections int `yaml:"max_idle_connections" json:"max_idle_connections" toml:"max_idle_connections"`

	// EnableForeignKeys enables foreign key constraint enforcement
	EnableForeignKeys bool `yaml:"enable_foreign_keys" json:"enable_foreign_keys" toml:"enable_foreign_keys"`
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

// RetentionPolicyConfig represents database retention policy settings.
type RetentionPolicyConfig struct {
	// MaxDBSizeMB is the maximum database size in megabytes (0 = unlimited)
	MaxDBSizeMB uint64 `yaml:"max_db_size_mb"`

	// MaxBlocks is the maximum number of blocks to retain (0 = unlimited)
	MaxBlocks uint64 `yaml:"max_blocks"`
}

// IsEnabled returns true if retention policy should be applied
func (r *RetentionPolicyConfig) IsEnabled() bool {
	return r != nil && (r.MaxDBSizeMB > 0 || r.MaxBlocks > 0)
}

// MaintenanceConfig configures database maintenance behavior.
type MaintenanceConfig struct {
	// Enabled controls whether background maintenance runs
	Enabled bool `yaml:"enabled" json:"enabled" toml:"enabled"`

	// CheckInterval is how often to run maintenance (e.g., "30m", "1h")
	CheckInterval string `yaml:"check_interval" json:"check_interval" toml:"check_interval"`

	// VacuumOnStartup runs maintenance immediately on startup
	VacuumOnStartup bool `yaml:"vacuum_on_startup" json:"vacuum_on_startup" toml:"vacuum_on_startup"`

	// WALCheckpointMode controls the WAL checkpoint aggressiveness
	// Options: PASSIVE, FULL, RESTART, TRUNCATE
	// TRUNCATE is recommended for production (most aggressive space reclamation)
	WALCheckpointMode string `yaml:"wal_checkpoint_mode" json:"wal_checkpoint_mode" toml:"wal_checkpoint_mode"`
}

// ApplyDefaults sets default values for optional maintenance configuration fields.
func (m *MaintenanceConfig) ApplyDefaults() {
	if m.CheckInterval == "" {
		m.CheckInterval = "30m"
	}
	if m.WALCheckpointMode == "" {
		m.WALCheckpointMode = "TRUNCATE"
	}
	// Enabled defaults to false (zero value)
	// VacuumOnStartup defaults to false (zero value)
}

// Validate checks if the maintenance configuration is valid.
func (m *MaintenanceConfig) Validate() error {
	if m.CheckInterval != "" {
		// Try parsing the duration
		if _, err := time.ParseDuration(m.CheckInterval); err != nil {
			return fmt.Errorf("maintenance.check_interval: invalid duration format '%s'", m.CheckInterval)
		}
	}

	if m.WALCheckpointMode != "" {
		validModes := []string{"PASSIVE", "FULL", "RESTART", "TRUNCATE"}
		valid := false
		for _, mode := range validModes {
			if m.WALCheckpointMode == mode {
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf("maintenance.wal_checkpoint_mode: must be one of: PASSIVE, FULL, RESTART, TRUNCATE")
		}
	}

	return nil
}

// ParsedCheckInterval returns the parsed check interval duration.
func (m *MaintenanceConfig) ParsedCheckInterval() (time.Duration, error) {
	return time.ParseDuration(m.CheckInterval)
}

// IndexerConfig represents the configuration for a single indexer.
type IndexerConfig struct {
	// Name is a unique identifier for this indexer
	Name string `yaml:"name" json:"name" toml:"name"`

	// StartBlock is the block number to start indexing from
	StartBlock uint64 `yaml:"start_block" json:"start_block" toml:"start_block"`

	// DB contains database configuration for the indexer
	DB DatabaseConfig `yaml:"db" json:"db" toml:"db"`

	// Contracts contains the list of contracts to index
	Contracts []ContractConfig `yaml:"contracts" json:"contracts" toml:"contracts"`
}

// ApplyDefaults sets default values for optional indexer configuration fields.
func (i *IndexerConfig) ApplyDefaults() {
	// Apply database defaults
	i.DB.ApplyDefaults()
}

// ContractConfig represents a contract and its events to index.
type ContractConfig struct {
	// Address is the contract address to monitor
	Address string `yaml:"address" json:"address" toml:"address"`

	// Events is the list of event signatures to index
	// Format: "EventName(type1, type2, ...)"
	Events []string `yaml:"events" json:"events" toml:"events"`
}

// ApplyDefaults sets default values for optional configuration fields.
func (c *Config) ApplyDefaults() {
	// Apply downloader defaults (which includes DB defaults)
	c.Downloader.ApplyDefaults()

	// Apply indexer defaults
	for i := range c.Indexers {
		c.Indexers[i].ApplyDefaults()
	}
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

	if c.Downloader.Maintenance != nil {
		if err := c.Downloader.Maintenance.Validate(); err != nil {
			return fmt.Errorf("downloader.maintenance: %w", err)
		}
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

		if indexer.DB.Path == "" {
			return fmt.Errorf("indexer[%d] (%s): db.path is required", i, indexer.Name)
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
