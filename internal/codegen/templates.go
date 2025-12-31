package codegen

import (
	"bytes"
	_ "embed"
	"fmt"
	"strings"
	"text/template"
)

//go:embed templates/models.go.tmpl
var modelsTemplate string

//go:embed templates/indexer.go.tmpl
var indexerTemplate string

//go:embed templates/register.go.tmpl
var registerTemplate string

//go:embed templates/migrations.go.tmpl
var migrationsTemplate string

//go:embed templates/001_initial.sql.tmpl
var initialSQLTemplate string

//go:embed templates/README.md.tmpl
var readmeTemplate string

// TemplateData represents the data passed to templates.
type TemplateData struct {
	Name       string            // Indexer name (PascalCase, e.g., "ERC20Token")
	Package    string            // Go package name (lowercase, e.g., "erc20token")
	ImportPath string            // Full import path for the package
	Events     []*EventSignature // Events to generate code for
}

// RenderModels generates the models.go file content.
func RenderModels(data *TemplateData) (string, error) {
	return renderTemplate("models", modelsTemplate, data)
}

// RenderIndexer generates the indexer.go file content.
func RenderIndexer(data *TemplateData) (string, error) {
	return renderTemplate("indexer", indexerTemplate, data)
}

// RenderRegister generates the register.go file content.
func RenderRegister(data *TemplateData) (string, error) {
	return renderTemplate("register", registerTemplate, data)
}

// RenderMigrations generates the migrations/migrations.go file content.
func RenderMigrations(data *TemplateData) (string, error) {
	return renderTemplate("migrations", migrationsTemplate, data)
}

// RenderInitialSQL generates the migrations/001_initial.sql file content.
func RenderInitialSQL(data *TemplateData) (string, error) {
	return renderTemplate("initial_sql", initialSQLTemplate, data)
}

// RenderReadme generates the README.md file content.
func RenderReadme(data *TemplateData) (string, error) {
	return renderTemplate("readme", readmeTemplate, data)
}

// renderTemplate renders a template with the given data.
func renderTemplate(name, tmplStr string, data *TemplateData) (string, error) {
	tmpl, err := template.New(name).Funcs(templateFuncs()).Parse(tmplStr)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

// templateFuncs returns the functions available in templates.
func templateFuncs() template.FuncMap {
	return template.FuncMap{
		// Type conversion functions
		"GoTypeName":  GoTypeName,
		"DBTypeName":  DBTypeName,
		"DBFieldName": DBFieldName,
		"MeddlerTag":  MeddlerTag,

		// Case conversion functions
		"ToPascalCase":     ToPascalCase,
		"ToSnakeCase":      ToSnakeCase,
		"ToLowerCamelCase": ToLowerCamelCase,

		// String manipulation
		"Pluralize": Pluralize,
		"TableName": TableName,

		// Helper functions for templates
		"add":       func(a, b int) int { return a + b },
		"hasPrefix": strings.HasPrefix,
		"hasSuffix": strings.HasSuffix,
		"len": func(s any) int {
			switch v := s.(type) {
			case []EventParam:
				return len(v)
			case string:
				return len(v)
			default:
				return 0
			}
		},
	}
}
