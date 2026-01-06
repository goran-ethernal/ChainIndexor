package codegen

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	mkdirPerm = 0755
	filePerm  = 0644
)

// Generator generates indexer code from event signatures.
type Generator struct {
	Name       string   // Indexer name (e.g., "ERC20Token")
	Package    string   // Go package name (e.g., "erc20token")
	Events     []string // Event signatures
	OutputDir  string   // Output directory path
	ImportPath string   // Go module import path
	Force      bool     // Overwrite existing files
	DryRun     bool     // Don't write files, just show what would be generated
}

// GeneratedFiles represents the files that were generated.
type GeneratedFiles struct {
	IndexerFile    string // Path to indexer.go
	ModelsFile     string // Path to models.go
	RegisterFile   string // Path to register.go
	APIFile        string // Path to api.go
	MigrationsFile string // Path to migrations/migrations.go
	ReadmeFile     string // Path to README.md
}

// Generate generates all indexer files.
func (g *Generator) Generate() (*GeneratedFiles, error) {
	// Validate inputs
	if err := g.validate(); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Parse event signatures
	events, err := g.parseEvents()
	if err != nil {
		return nil, fmt.Errorf("failed to parse events: %w", err)
	}

	// Determine package name if not provided
	if g.Package == "" {
		g.Package = strings.ToLower(g.Name)
	}

	// Determine output directory if not provided
	if g.OutputDir == "" {
		g.OutputDir = filepath.Join(".", "indexers", g.Package)
	}

	// Determine import path if not provided
	if g.ImportPath == "" {
		modulePath, err := getModulePath()
		if err != nil {
			g.ImportPath = "yourproject/indexers/" + g.Package
		} else {
			// Clean output path and convert to import path format
			cleanPath := filepath.Clean(g.OutputDir)
			cleanPath = strings.TrimPrefix(cleanPath, "./")
			cleanPath = filepath.ToSlash(cleanPath)
			g.ImportPath = modulePath + "/" + cleanPath
		}
	}

	// Prepare template data
	data := &TemplateData{
		Name:       g.Name,
		Package:    g.Package,
		ImportPath: g.ImportPath,
		Events:     events,
	}

	// Check if output directory exists
	if !g.Force {
		if _, err := os.Stat(g.OutputDir); err == nil {
			return nil, fmt.Errorf("output directory already exists: %s (use --force to overwrite)", g.OutputDir)
		}
	}

	// Create output directories
	if !g.DryRun {
		if err := os.MkdirAll(g.OutputDir, mkdirPerm); err != nil {
			return nil, fmt.Errorf("failed to create output directory: %w", err)
		}

		migrationsDir := filepath.Join(g.OutputDir, "migrations")
		if err := os.MkdirAll(migrationsDir, mkdirPerm); err != nil {
			return nil, fmt.Errorf("failed to create migrations directory: %w", err)
		}
	}

	// Generate all files
	type fileGen struct {
		path     *string
		render   func(*TemplateData) (string, error)
		filename string
		desc     string
	}

	files := &GeneratedFiles{}
	fileGens := []fileGen{
		{&files.ModelsFile, RenderModels, "models.go", "models"},
		{&files.IndexerFile, RenderIndexer, "indexer.go", "indexer"},
		{&files.RegisterFile, RenderRegister, "register.go", "register"},
		{&files.APIFile, RenderAPI, "api.go", "API"},
		{&files.MigrationsFile, RenderMigrations, "migrations/migrations.go", "migrations"},
		{nil, RenderInitialSQL, "migrations/001_initial.sql", "initial SQL"},
		{&files.ReadmeFile, RenderReadme, "README.md", "readme"},
	}

	for _, fg := range fileGens {
		content, err := fg.render(data)
		if err != nil {
			return nil, fmt.Errorf("failed to render %s: %w", fg.desc, err)
		}

		path := filepath.Join(g.OutputDir, fg.filename)
		if fg.path != nil {
			*fg.path = path
		}

		if err := g.writeFile(path, content); err != nil {
			return nil, err
		}
	}

	return files, nil
}

// validate validates the generator configuration.
func (g *Generator) validate() error {
	if g.Name == "" {
		return fmt.Errorf("indexer name is required")
	}

	if len(g.Events) == 0 {
		return fmt.Errorf("at least one event signature is required")
	}

	// Validate name format (should be PascalCase)
	if !strings.Contains(g.Name, " ") && len(g.Name) > 0 {
		firstChar := rune(g.Name[0])
		if firstChar < 'A' || firstChar > 'Z' {
			return fmt.Errorf("indexer name should start with an uppercase letter: %s", g.Name)
		}
	}

	return nil
}

// parseEvents parses event signature strings into EventSignature objects.
func (g *Generator) parseEvents() ([]*EventSignature, error) {
	events := make([]*EventSignature, 0, len(g.Events))
	eventNames := make(map[string]bool)

	for i, sig := range g.Events {
		event, err := ParseEventSignature(sig)
		if err != nil {
			return nil, fmt.Errorf("invalid event signature #%d '%s': %w", i+1, sig, err)
		}

		// Check for duplicate event names
		if eventNames[event.Name] {
			return nil, fmt.Errorf("duplicate event name: %s", event.Name)
		}
		eventNames[event.Name] = true

		events = append(events, event)
	}

	return events, nil
}

// writeFile writes content to a file, respecting DryRun and Force flags.
func (g *Generator) writeFile(path, content string) error {
	if g.DryRun {
		fmt.Printf("Would create: %s\n", path)
		return nil
	}

	// Create parent directory if it doesn't exist
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, mkdirPerm); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Check if file exists and Force is not set
	if !g.Force {
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("file already exists: %s (use --force to overwrite)", path)
		}
	}

	if err := os.WriteFile(path, []byte(content), filePerm); err != nil {
		return fmt.Errorf("failed to write file %s: %w", path, err)
	}

	fmt.Printf("Generated: %s\n", path)
	return nil
}

// getModulePath reads the module path from go.mod file.
func getModulePath() (string, error) {
	data, err := os.ReadFile("go.mod")
	if err != nil {
		return "", err
	}

	for line := range strings.SplitSeq(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module")), nil
		}
	}

	return "", fmt.Errorf("module directive not found in go.mod")
}

// PrintSummary prints a summary of what was generated.
func (g *Generator) PrintSummary(files *GeneratedFiles) {
	fmt.Println("\n✓ Successfully generated indexer!")
	fmt.Printf("\nIndexer: %s\n", g.Name)
	fmt.Printf("Package: %s\n", g.Package)
	fmt.Printf("Output:  %s\n", g.OutputDir)
	fmt.Printf("Events:  %d\n", len(g.Events))

	fmt.Println("\nGenerated files:")
	fmt.Printf("  • %s\n", files.IndexerFile)
	fmt.Printf("  • %s\n", files.ModelsFile)
	fmt.Printf("  • %s\n", files.RegisterFile)
	fmt.Printf("  • %s\n", files.APIFile)
	fmt.Printf("  • %s\n", files.MigrationsFile)
	fmt.Printf("  • %s\n", files.ReadmeFile)

	fmt.Println("\nNext steps:")
	fmt.Println("  1. Review the generated code")
	fmt.Println("  2. Add to your config.yaml:")
	fmt.Printf("     indexers:\n")
	fmt.Printf("       - name: \"%sIndexer\"\n", g.Name)
	fmt.Printf("         type: \"%s\"  # Indexer type for registry\n", strings.ToLower(g.Name))
	fmt.Printf("         start_block: 0\n")
	fmt.Printf("         db:\n")
	fmt.Printf("           path: \"./data/%s.sqlite\"\n", strings.ToLower(g.Name))
	fmt.Printf("         contracts:\n")
	fmt.Printf("           - address: \"0xYourContractAddress\"\n")
	fmt.Printf("             events:\n")
	for _, event := range g.Events {
		parsed, _ := ParseEventSignature(event)
		fmt.Printf("               - \"%s\"\n", parsed.CanonicalSignature())
	}
	fmt.Println("  3. Import in your main.go:")
	fmt.Printf("     import \"%s\"\n", g.ImportPath)
	fmt.Println("  4. Register the indexer with the orchestrator")
	fmt.Println("\nFor more information, see the generated README.md")
}
