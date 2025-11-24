package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	pkgconfig "github.com/goran-ethernal/ChainIndexor/pkg/config"
	"gopkg.in/yaml.v3"
)

// LoadFromFile loads configuration from a file, auto-detecting the format by extension.
// Supported formats: .yaml, .yml, .json, .toml
func LoadFromFile(path string) (*pkgconfig.Config, error) {
	ext := strings.ToLower(filepath.Ext(path))

	switch ext {
	case ".yaml", ".yml":
		return LoadFromYAML(path)
	case ".json":
		return LoadFromJSON(path)
	case ".toml":
		return LoadFromTOML(path)
	default:
		return nil, fmt.Errorf("unsupported config file format: %s (supported: .yaml, .yml, .json, .toml)", ext)
	}
}

// LoadFromYAML loads configuration from a YAML file.
func LoadFromYAML(path string) (*pkgconfig.Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg pkgconfig.Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse YAML config: %w", err)
	}

	return processConfig(&cfg)
}

// LoadFromJSON loads configuration from a JSON file.
func LoadFromJSON(path string) (*pkgconfig.Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg pkgconfig.Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse JSON config: %w", err)
	}

	return processConfig(&cfg)
}

// LoadFromTOML loads configuration from a TOML file.
func LoadFromTOML(path string) (*pkgconfig.Config, error) {
	var cfg pkgconfig.Config
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse TOML config: %w", err)
	}

	return processConfig(&cfg)
}

// processConfig applies defaults and validates the configuration.
func processConfig(cfg *pkgconfig.Config) (*pkgconfig.Config, error) {
	// Apply defaults
	cfg.ApplyDefaults()

	// Validate
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return cfg, nil
}
