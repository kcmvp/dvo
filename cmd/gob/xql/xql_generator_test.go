package xql

import (
	"context"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kcmvp/dvo/cmd/internal"
	"github.com/stretchr/testify/require"
)

func compareGoFileWithJSON(t *testing.T, goFilePath, jsonFilePath string) {
	// Read the generated Go file
	content, err := os.ReadFile(goFilePath)
	require.NoError(t, err)

	// Parse the Go file
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, "", content, parser.ParseComments)
	require.NoError(t, err)

	// Extract the fields from the var block
	fields := make(map[string]string)
	ast.Inspect(node, func(n ast.Node) bool {
		if vs, ok := n.(*ast.ValueSpec); ok {
			for _, name := range vs.Names {
				if len(vs.Values) > 0 {
					// Get the source code of the value expression
					start := vs.Values[0].Pos() - 1
					end := vs.Values[0].End() - 1
					if int(start) < len(content) && int(end) < len(content) {
						fields[name.Name] = string(content[start:end])
					}
				}
			}
		}
		return true
	})

	// Read the JSON file
	jsonContent, err := os.ReadFile(jsonFilePath)
	require.NoError(t, err)

	// Unmarshal the JSON file
	var expectedFields map[string]string
	err = json.Unmarshal(jsonContent, &expectedFields)
	require.NoError(t, err)

	// Compare the fields
	require.Equal(t, expectedFields, fields)
}

func compareFiles(t *testing.T, generatedFilePath, testDataFilePath string) {
	generatedContent, err := os.ReadFile(generatedFilePath)
	require.NoError(t, err)

	testDataContent, err := os.ReadFile(testDataFilePath)
	require.NoError(t, err)

	if cleanSQL(string(testDataContent)) != cleanSQL(string(generatedContent)) {
		t.Log("Generated file content:\n", string(generatedContent))
	}
	require.Equal(t, cleanSQL(string(testDataContent)), cleanSQL(string(generatedContent)))
}

func cleanSQL(content string) string {
	lines := strings.Split(content, "\n")
	var cleanedLines []string
	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		if trimmedLine != "" && !strings.HasPrefix(trimmedLine, "--") {
			cleanedLines = append(cleanedLines, trimmedLine)
		}
	}
	return strings.Join(cleanedLines, "\n")
}

func TestGeneration(t *testing.T) {
	// Ensure the project is initialized
	require.NotNil(t, internal.Current, "internal.Current should be initialized")

	// Create a context with database adapters
	ctx := context.WithValue(context.Background(), dbaAdapterKey, []string{"sqlite", "postgres", "mysql"})

	// Run the generator
	err := Generate(ctx)
	require.NoError(t, err)
	testDataDir := filepath.Join(internal.Current.Root, "testdata")

	// Verify the output for Account fields
	compareGoFileWithJSON(t,
		filepath.Join(internal.Current.GenPath(), "field", "account", "account.go"),
		filepath.Join(testDataDir, "account_fields.json"),
	)

	// Verify the output for Order fields
	compareGoFileWithJSON(t,
		filepath.Join(internal.Current.GenPath(), "field", "order", "order.go"),
		filepath.Join(testDataDir, "order_fields.json"),
	)

	// Verify the output for schemas
	for _, db := range []string{"sqlite", "postgres", "mysql"} {
		compareFiles(t,
			filepath.Join(internal.Current.GenPath(), "schemas", db, "account_schema.sql"),
			filepath.Join(testDataDir, db, "account_schema.sql"),
		)
		compareFiles(t,
			filepath.Join(internal.Current.GenPath(), "schemas", db, "order_schema.sql"),
			filepath.Join(testDataDir, db, "order_schema.sql"),
		)
	}
}
