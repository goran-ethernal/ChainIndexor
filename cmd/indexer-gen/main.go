package main

import (
	"fmt"
	"os"

	"github.com/goran-ethernal/ChainIndexor/internal/codegen"
	"github.com/spf13/cobra"
)

const version = "0.1.0"

var (
	// Flags
	name        string
	events      []string
	output      string
	packageName string
	importPath  string
	force       bool
	dryRun      bool
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "indexer-gen",
	Short: "Generate blockchain event indexers from event signatures",
	Long: `indexer-gen is a code generator that creates production-ready indexer
implementations from Solidity event signatures. It generates models,
indexer logic, database migrations, and documentation automatically.`,
	Version: version,
	Example: `  # Generate ERC20 token indexer
  indexer-gen --name ERC20Token \
    --event "Transfer(address indexed from, address indexed to, uint256 value)" \
    --event "Approval(address indexed owner, address indexed spender, uint256 value)"

  # Generate NFT indexer with custom output
  indexer-gen --name ERC721 \
    --event "Transfer(address indexed from, address indexed to, uint256 indexed tokenId)" \
    --output ./examples/indexers/erc721

  # Preview generation without writing files
  indexer-gen --name MyToken \
    --event "Transfer(address,address,uint256)" \
    --dry-run`,
	RunE: runGenerate,
}

func init() {
	rootCmd.Flags().StringVarP(&name, "name", "n", "", "indexer name (required, PascalCase, e.g., 'ERC20Token')")
	rootCmd.Flags().StringArrayVarP(&events, "event", "e", []string{},
		"event signature (required, can be specified multiple times)")
	rootCmd.Flags().StringVarP(&output, "output", "o", "", "output directory (default: ./indexers/<name_lowercase>)")
	rootCmd.Flags().StringVarP(&packageName, "package", "p", "", "Go package name (default: derived from name)")
	rootCmd.Flags().StringVarP(&importPath, "import", "i", "", "Go import path (default: auto-detected from go.mod)")
	rootCmd.Flags().BoolVarP(&force, "force", "f", false, "overwrite existing files")
	rootCmd.Flags().BoolVar(&dryRun, "dry-run", false, "show what would be generated without writing files")

	// Mark required flags
	_ = rootCmd.MarkFlagRequired("name")
	_ = rootCmd.MarkFlagRequired("event")
}

func runGenerate(cmd *cobra.Command, args []string) error {
	// Create generator
	gen := &codegen.Generator{
		Name:       name,
		Package:    packageName,
		Events:     events,
		OutputDir:  output,
		ImportPath: importPath,
		Force:      force,
		DryRun:     dryRun,
	}

	// Generate indexer files
	files, err := gen.Generate()
	if err != nil {
		return err
	}

	// Print summary
	if !dryRun {
		gen.PrintSummary(files)
	} else {
		fmt.Println("\nDry run complete. No files were created.")
	}

	return nil
}
