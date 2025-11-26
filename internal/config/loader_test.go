package config

import (
	"testing"

	"github.com/goran-ethernal/ChainIndexor/pkg/config"
	"github.com/stretchr/testify/require"
)

func TestLoadFromYAML(t *testing.T) {
	cfg, err := LoadFromYAML("../../config.example.yaml")
	if err != nil {
		t.Fatalf("failed to load YAML config: %v", err)
	}

	validateConfig(t, cfg, "YAML")
}

func TestLoadFromJSON(t *testing.T) {
	cfg, err := LoadFromJSON("../../config.example.json")
	if err != nil {
		t.Fatalf("failed to load JSON config: %v", err)
	}

	validateConfig(t, cfg, "JSON")
}

func TestLoadFromTOML(t *testing.T) {
	cfg, err := LoadFromTOML("../../config.example.toml")
	if err != nil {
		t.Fatalf("failed to load TOML config: %v", err)
	}

	validateConfig(t, cfg, "TOML")
}

func TestLoadFromFile_YAML(t *testing.T) {
	cfg, err := LoadFromFile("../../config.example.yaml")
	if err != nil {
		t.Fatalf("failed to auto-load YAML config: %v", err)
	}

	validateConfig(t, cfg, "auto-detected YAML")
}

func TestLoadFromFile_JSON(t *testing.T) {
	cfg, err := LoadFromFile("../../config.example.json")
	if err != nil {
		t.Fatalf("failed to auto-load JSON config: %v", err)
	}

	validateConfig(t, cfg, "auto-detected JSON")
}

func TestLoadFromFile_TOML(t *testing.T) {
	cfg, err := LoadFromFile("../../config.example.toml")
	if err != nil {
		t.Fatalf("failed to auto-load TOML config: %v", err)
	}

	validateConfig(t, cfg, "auto-detected TOML")
}

func TestLoadFromFile_UnsupportedFormat(t *testing.T) {
	_, err := LoadFromFile("config.txt")
	require.Contains(t, err.Error(), "unsupported config file format")
}

// validateConfig checks that the loaded config has expected values
func validateConfig(t *testing.T, cfg *config.Config, format string) {
	t.Helper()

	// Test downloader config
	require.NotEmpty(t, cfg.Downloader.RPCURL, "[%s] downloader.rpc_url should not be empty", format)

	// Test defaults applied
	require.NotZero(t, cfg.Downloader.ChunkSize, "[%s] downloader.chunk_size should not be zero")
	require.NotEmpty(t, cfg.Downloader.Finality, "[%s] finality should have default value applied", format)

	// Test database config
	require.NotEmpty(t, cfg.Downloader.DB.Path, "[%s] db.path should not be empty", format)

	// Check defaults were applied
	require.NotEmpty(t, cfg.Downloader.DB.JournalMode, "[%s] db.journal_mode should have default value", format)
	require.NotEmpty(t, cfg.Downloader.DB.Synchronous, "[%s] db.synchronous should have default value", format)

	// Test indexers
	require.NotEmpty(t, cfg.Indexers, "[%s] there should be at least one indexer configured", format)

	for i, indexer := range cfg.Indexers {
		require.NotEmpty(t, indexer.Name, "[%s] indexer[%d].name should not be empty", format, i)
		require.NotEmpty(t, indexer.DB.Path, "[%s] indexer[%d].db.path should not be empty", format, i)
		require.NotEmpty(t, indexer.Contracts, "[%s] indexer[%d] should have at least one contract", format, i)

		// Check indexer DB defaults were applied
		require.NotEmpty(t, indexer.DB.JournalMode, "[%s] indexer[%d].db.journal_mode should have default value", format, i)
		require.NotEmpty(t, indexer.DB.Synchronous, "[%s] indexer[%d].db.synchronous should have default value", format, i)

		for j, contract := range indexer.Contracts {
			require.NotEmpty(t, contract.Address, "[%s] indexer[%d].contract[%d].address should not be empty", format, i, j)
			require.NotEmpty(t, contract.Events, "[%s] indexer[%d].contract[%d] should have at least one event", format, i, j)
		}
	}
}

func TestConfigDefaults(t *testing.T) {
	cfg := &config.Config{
		Downloader: config.DownloaderConfig{
			RPCURL: "https://test.com",
			DB: config.DatabaseConfig{
				Path: "./test.db",
			},
		},
		Indexers: []config.IndexerConfig{
			{
				Name: "test",
				DB: config.DatabaseConfig{
					Path: "./test-indexer.db",
				},
				Contracts: []config.ContractConfig{
					{
						Address: "0x1234",
						Events:  []string{"Transfer(address,address,uint256)"},
					},
				},
			},
		},
	}

	// Apply defaults
	cfg.ApplyDefaults()

	// Check defaults were applied
	if cfg.Downloader.ChunkSize != 5000 {
		t.Errorf("expected default chunk_size=5000, got %d", cfg.Downloader.ChunkSize)
	}

	if cfg.Downloader.Finality != "finalized" {
		t.Errorf("expected default finality=finalized, got %s", cfg.Downloader.Finality)
	}

	if cfg.Downloader.DB.JournalMode != "WAL" {
		t.Errorf("expected default journal_mode=WAL, got %s", cfg.Downloader.DB.JournalMode)
	}

	if cfg.Downloader.DB.Synchronous != "NORMAL" {
		t.Errorf("expected default synchronous=NORMAL, got %s", cfg.Downloader.DB.Synchronous)
	}

	if cfg.Downloader.DB.BusyTimeout != 5000 {
		t.Errorf("expected default busy_timeout=5000, got %d", cfg.Downloader.DB.BusyTimeout)
	}

	if cfg.Downloader.DB.MaxOpenConnections != 25 {
		t.Errorf("expected default max_open_connections=25, got %d", cfg.Downloader.DB.MaxOpenConnections)
	}

	// Check indexer DB defaults were applied
	if len(cfg.Indexers) > 0 {
		if cfg.Indexers[0].DB.JournalMode != "WAL" {
			t.Errorf("expected default indexer journal_mode=WAL, got %s", cfg.Indexers[0].DB.JournalMode)
		}

		if cfg.Indexers[0].DB.Synchronous != "NORMAL" {
			t.Errorf("expected default indexer synchronous=NORMAL, got %s", cfg.Indexers[0].DB.Synchronous)
		}

		if cfg.Indexers[0].DB.BusyTimeout != 5000 {
			t.Errorf("expected default indexer busy_timeout=5000, got %d", cfg.Indexers[0].DB.BusyTimeout)
		}

		if cfg.Indexers[0].DB.MaxOpenConnections != 25 {
			t.Errorf("expected default indexer max_open_connections=25, got %d", cfg.Indexers[0].DB.MaxOpenConnections)
		}
	}
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *config.Config
		wantErr bool
	}{
		{
			name: "valid config",
			cfg: &config.Config{
				Downloader: config.DownloaderConfig{
					RPCURL:   "https://test.com",
					Finality: "finalized",
					DB: config.DatabaseConfig{
						Path: "./test.db",
					},
				},
				Indexers: []config.IndexerConfig{
					{
						Name: "test",
						DB: config.DatabaseConfig{
							Path: "./test.db",
						},
						Contracts: []config.ContractConfig{
							{
								Address: "0x1234",
								Events:  []string{"Transfer(address,address,uint256)"},
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "missing rpc_url",
			cfg: &config.Config{
				Downloader: config.DownloaderConfig{
					DB: config.DatabaseConfig{
						Path: "./test.db",
					},
				},
				Indexers: []config.IndexerConfig{
					{
						Name: "test",
						DB: config.DatabaseConfig{
							Path: "./test.db",
						},
						Contracts: []config.ContractConfig{
							{
								Address: "0x1234",
								Events:  []string{"Transfer(address,address,uint256)"},
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid finality",
			cfg: &config.Config{
				Downloader: config.DownloaderConfig{
					RPCURL:   "https://test.com",
					Finality: "invalid",
					DB: config.DatabaseConfig{
						Path: "./test.db",
					},
				},
				Indexers: []config.IndexerConfig{
					{
						Name: "test",
						DB: config.DatabaseConfig{
							Path: "./test.db",
						},
						Contracts: []config.ContractConfig{
							{
								Address: "0x1234",
								Events:  []string{"Transfer(address,address,uint256)"},
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "no indexers",
			cfg: &config.Config{
				Downloader: config.DownloaderConfig{
					RPCURL: "https://test.com",
					DB: config.DatabaseConfig{
						Path: "./test.db",
					},
				},
				Indexers: []config.IndexerConfig{},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.cfg.ApplyDefaults()
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
