package codegen

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerator_Validate(t *testing.T) {
	tests := []struct {
		name    string
		gen     *Generator
		wantErr bool
	}{
		{
			name: "valid configuration",
			gen: &Generator{
				Name:   "MyToken",
				Events: []string{"Transfer(address,address,uint256)"},
			},
			wantErr: false,
		},
		{
			name: "missing name",
			gen: &Generator{
				Events: []string{"Transfer(address,address,uint256)"},
			},
			wantErr: true,
		},
		{
			name: "missing events",
			gen: &Generator{
				Name: "MyToken",
			},
			wantErr: true,
		},
		{
			name: "invalid name - lowercase",
			gen: &Generator{
				Name:   "myToken",
				Events: []string{"Transfer(address,address,uint256)"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.gen.validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGenerator_ParseEvents(t *testing.T) {
	tests := []struct {
		name      string
		events    []string
		wantCount int
		wantErr   bool
	}{
		{
			name: "single event",
			events: []string{
				"Transfer(address indexed from, address indexed to, uint256 value)",
			},
			wantCount: 1,
			wantErr:   false,
		},
		{
			name: "multiple events",
			events: []string{
				"Transfer(address indexed from, address indexed to, uint256 value)",
				"Approval(address indexed owner, address indexed spender, uint256 value)",
			},
			wantCount: 2,
			wantErr:   false,
		},
		{
			name: "duplicate event names",
			events: []string{
				"Transfer(address indexed from, address indexed to, uint256 value)",
				"Transfer(address sender, address recipient, uint256 amount)",
			},
			wantErr: true,
		},
		{
			name: "invalid event signature",
			events: []string{
				"Invalid Event Signature",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gen := &Generator{Events: tt.events}
			parsed, err := gen.parseEvents()

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Len(t, parsed, tt.wantCount)
		})
	}
}

func TestGenerator_Generate(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir := t.TempDir()

	gen := &Generator{
		Name: "TestToken",
		Events: []string{
			"Transfer(address indexed from, address indexed to, uint256 value)",
			"Approval(address indexed owner, address indexed spender, uint256 value)",
		},
		OutputDir:  filepath.Join(tmpDir, "testtoken"),
		ImportPath: "github.com/test/indexers/testtoken",
		Force:      true,
	}

	files, err := gen.Generate()
	require.NoError(t, err)
	require.NotNil(t, files)

	// Verify all files were created
	assert.FileExists(t, files.IndexerFile)
	assert.FileExists(t, files.ModelsFile)
	assert.FileExists(t, files.MigrationsFile)
	assert.FileExists(t, files.ReadmeFile)

	// Verify file contents contain expected strings
	modelsContent, err := os.ReadFile(files.ModelsFile)
	require.NoError(t, err)
	assert.Contains(t, string(modelsContent), "type Transfer struct")
	assert.Contains(t, string(modelsContent), "type Approval struct")

	indexerContent, err := os.ReadFile(files.IndexerFile)
	require.NoError(t, err)
	assert.Contains(t, string(indexerContent), "type TestTokenIndexer struct")
	assert.Contains(t, string(indexerContent), "func NewTestTokenIndexer")
	assert.Contains(t, string(indexerContent), "func (idx *TestTokenIndexer) HandleLogs")
	assert.Contains(t, string(indexerContent), "func (idx *TestTokenIndexer) HandleReorg")

	migrationsContent, err := os.ReadFile(files.MigrationsFile)
	require.NoError(t, err)
	assert.Contains(t, string(migrationsContent), "//go:embed 001_initial.sql")
	assert.Contains(t, string(migrationsContent), "func RunMigrations")

	// Check the SQL file exists and has the expected content
	sqlFile := filepath.Join(filepath.Dir(files.MigrationsFile), "001_initial.sql")
	assert.FileExists(t, sqlFile)
	sqlContent, err := os.ReadFile(sqlFile)
	require.NoError(t, err)
	assert.Contains(t, string(sqlContent), "CREATE TABLE IF NOT EXISTS transfers")
	assert.Contains(t, string(sqlContent), "CREATE TABLE IF NOT EXISTS approvals")

	readmeContent, err := os.ReadFile(files.ReadmeFile)
	require.NoError(t, err)
	assert.Contains(t, string(readmeContent), "# TestToken Indexer")
	assert.Contains(t, string(readmeContent), "Transfer(address,address,uint256)")
	assert.Contains(t, string(readmeContent), "Approval(address,address,uint256)")

	// Verify package declarations are correct
	assert.Contains(t, string(modelsContent), "package testtoken")
	assert.Contains(t, string(indexerContent), "package testtoken")

	// Verify imports are present
	assert.Contains(t, string(modelsContent), "github.com/ethereum/go-ethereum/common")
	assert.Contains(t, string(indexerContent), "github.com/ethereum/go-ethereum/common")
	assert.Contains(t, string(indexerContent), "github.com/goran-ethernal/ChainIndexor/pkg/config")
}

func TestGenerator_GenerateDryRun(t *testing.T) {
	tmpDir := t.TempDir()

	gen := &Generator{
		Name:      "TestToken",
		Events:    []string{"Transfer(address,address,uint256)"},
		OutputDir: filepath.Join(tmpDir, "testtoken"),
		DryRun:    true,
	}

	files, err := gen.Generate()
	require.NoError(t, err)
	require.NotNil(t, files)

	// In dry-run mode, files should not be created
	assert.NoFileExists(t, files.IndexerFile)
	assert.NoFileExists(t, files.ModelsFile)
	assert.NoFileExists(t, files.MigrationsFile)
	assert.NoFileExists(t, files.ReadmeFile)
}

func TestGenerator_GenerateWithoutForce(t *testing.T) {
	tmpDir := t.TempDir()
	outputDir := filepath.Join(tmpDir, "testtoken")

	// Create the directory first
	err := os.MkdirAll(outputDir, 0755)
	require.NoError(t, err)

	gen := &Generator{
		Name:      "TestToken",
		Events:    []string{"Transfer(address,address,uint256)"},
		OutputDir: outputDir,
		Force:     false,
	}

	// Should fail because directory already exists
	_, err = gen.Generate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestGenerator_DefaultValues(t *testing.T) {
	tmpDir := t.TempDir()

	// Change to temp directory to test default output path
	oldWd, err := os.Getwd()
	require.NoError(t, err)
	defer os.Chdir(oldWd) //nolint:errcheck
	require.NoError(t, os.Chdir(tmpDir))

	gen := &Generator{
		Name:   "MyToken",
		Events: []string{"Transfer(address,address,uint256)"},
		Force:  true,
	}

	files, err := gen.Generate()
	require.NoError(t, err)

	// Should use default package name (lowercase of Name)
	assert.Equal(t, "mytoken", gen.Package)

	// Should use default output directory
	assert.Contains(t, gen.OutputDir, "indexers/mytoken")

	// Files should be created
	assert.FileExists(t, files.IndexerFile)
}

func TestGetModulePath(t *testing.T) {
	// Create a temporary directory with a go.mod file
	tmpDir := t.TempDir()
	goModContent := `module github.com/test/project

go 1.21

require (
	github.com/ethereum/go-ethereum v1.13.0
)
`
	goModPath := filepath.Join(tmpDir, "go.mod")
	err := os.WriteFile(goModPath, []byte(goModContent), 0644)
	require.NoError(t, err)

	// Change to temp directory
	oldWd, err := os.Getwd()
	require.NoError(t, err)
	defer os.Chdir(oldWd) //nolint:errcheck
	require.NoError(t, os.Chdir(tmpDir))

	modulePath, err := getModulePath()
	require.NoError(t, err)
	assert.Equal(t, "github.com/test/project", modulePath)
}
