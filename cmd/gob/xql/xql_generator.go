package xql

import (
	"bytes"
	"context"
	"fmt"
	"go/ast"
	"go/format"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"text/template"
	"time"

	_ "embed"

	"github.com/kcmvp/dvo/cmd/internal"
	"github.com/samber/lo"
	"github.com/tidwall/gjson"
)

//go:embed resources/fields.tmpl
var fieldsTmpl string

//go:embed resources/schema.tmpl
var schemaTmpl string

// SchemaTemplateData holds the data passed to the schema template.
type SchemaTemplateData struct {
	TableName   string
	Fields      []Field
	GeneratedAt time.Time
}

// TemplateData holds the data passed to the template for execution.
type TemplateData struct {
	PackageName      string
	StructName       string
	Imports          []string
	Fields           []Field
	ModulePath       string
	EntityImportPath string
	GeneratedAt      time.Time
}

// Field represents a single column in a database table, derived from a Go struct field.
type Field struct {
	Name      string // The database column name (e.g., "creation_time").
	GoName    string // The original Go field name (e.g., "CreatedAt").
	GoType    string // The Go type of the field (e.g., "time.Time").
	DBType    string // The specific SQL type for the column (e.g., "TIMESTAMP WITH TIME ZONE").
	IsPK      bool   // True if this field is the primary key.
	IsNotNull bool   // True if the column has a NOT NULL constraint.
	IsUnique  bool   // True if the column has a UNIQUE constraint.
	IsIndexed bool   // True if an index should be created on this column.
	Default   string // The default value for the column, as a string.
	FKTable   string // The table referenced by a foreign key.
	FKColumn  string // The column referenced by a foreign key.
	Warning   string // A warning message associated with this field, e.g., for discouraged PK types.
}

// generate orchestrates field and schema generation.
func generate(ctx context.Context) error {
	if generateFields() == nil {
		return generateSchema(ctx)
	}
	return nil
}

// generateFields generates the entity field helpers.
func generateFields() error {
	project := internal.Current
	if project == nil {
		return fmt.Errorf("project context not initialized")
	}

	entities := project.StructsImplementEntity()
	if len(entities) == 0 {
		return fmt.Errorf("no entity structs found")
	}

	tmpl, err := template.New("fields").Parse(fieldsTmpl)
	if err != nil {
		return fmt.Errorf("failed to parse fields template: %w", err)
	}

	for _, entityInfo := range entities {
		structName := entityInfo.TypeSpec.Name.Name
		// For now, we assume a single adapter for parsing fields. This might need adjustment.
		fields := parseFields(entityInfo.TypeSpec, "")
		if len(fields) == 0 {
			continue
		}

		imports := lo.Uniq(lo.FilterMap(fields, func(f Field, _ int) (string, bool) {
			if strings.Contains(f.GoType, ".") {
				pkg := strings.Split(f.GoType, ".")[0]
				// This is a simple heuristic. A robust solution would inspect the package's imports.
				// For now, we assume standard library packages.
				switch pkg {
				case "time":
					return "time", true
				default:
					// We would need to resolve the full import path for external packages.
					// This is a complex task, so we'll omit it for now.
					return "", false
				}
			}
			return "", false
		}))

		data := TemplateData{
			PackageName:      strings.ToLower(structName),
			StructName:       structName,
			Imports:          imports,
			Fields:           fields,
			ModulePath:       internal.ToolModulePath(),
			EntityImportPath: entityInfo.PkgPath,
			GeneratedAt:      time.Now(),
		}

		// Define output path
		outputDir := filepath.Join(project.GenPath(), "field", data.PackageName)
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			return fmt.Errorf("failed to create output directory %s: %w", outputDir, err)
		}
		outputPath := filepath.Join(outputDir, fmt.Sprintf("%s.go", data.PackageName))

		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, data); err != nil {
			return fmt.Errorf("failed to execute template for %s: %w", structName, err)
		}

		formatted, err := format.Source(buf.Bytes())
		if err != nil {
			return fmt.Errorf("failed to format generated code for %s: %w", structName, err)
		}

		if err := os.WriteFile(outputPath, formatted, 0644); err != nil {
			return fmt.Errorf("failed to write generated file for %s: %w", structName, err)
		}
		fmt.Printf("Generated field helpers for %s at %s\n", structName, outputPath)
	}

	return nil
}

func generateSchema(ctx context.Context) error {
	project := internal.Current
	if project == nil {
		return fmt.Errorf("project context not initialized")
	}

	adapters, ok := ctx.Value(XqlDBAdapterKey).([]string)
	if !ok || len(adapters) == 0 {
		return fmt.Errorf("no database adapters are configured or detected")
	}

	entities := project.StructsImplementEntity()
	if len(entities) == 0 {
		return fmt.Errorf("no entity structs found")
	}

	funcMap := template.FuncMap{
		"plus1": func(i int) int {
			return i + 1
		},
	}

	tmpl, err := template.New("schema").Funcs(funcMap).Parse(schemaTmpl)
	if err != nil {
		return fmt.Errorf("failed to parse schema template: %w", err)
	}

	for _, adapter := range adapters {
		for _, entityInfo := range entities {
			structName := entityInfo.TypeSpec.Name.Name
			fields := parseFields(entityInfo.TypeSpec, adapter)
			if len(fields) == 0 {
				continue
			}

			tableName := lo.SnakeCase(structName)
			// A simplified way to get the table name from the Table() method.
			// This is fragile and assumes a specific method structure.
			for _, pkg := range project.Pkgs {
				if pkg.PkgPath == entityInfo.PkgPath {
					for _, file := range pkg.Syntax {
						ast.Inspect(file, func(n ast.Node) bool {
							if fn, ok := n.(*ast.FuncDecl); ok && fn.Name.Name == "Table" {
								if fn.Recv != nil && len(fn.Recv.List) > 0 {
									if starExpr, ok := fn.Recv.List[0].Type.(*ast.StarExpr); ok {
										if ident, ok := starExpr.X.(*ast.Ident); ok && ident.Name == structName {
											if len(fn.Body.List) > 0 {
												if ret, ok := fn.Body.List[0].(*ast.ReturnStmt); ok && len(ret.Results) > 0 {
													if lit, ok := ret.Results[0].(*ast.BasicLit); ok && lit.Kind == token.STRING {
														tableName = strings.Trim(lit.Value, `"`)
													}
												}
											}
										}
									} else if ident, ok := fn.Recv.List[0].Type.(*ast.Ident); ok && ident.Name == structName {
										if len(fn.Body.List) > 0 {
											if ret, ok := fn.Body.List[0].(*ast.ReturnStmt); ok && len(ret.Results) > 0 {
												if lit, ok := ret.Results[0].(*ast.BasicLit); ok && lit.Kind == token.STRING {
													tableName = strings.Trim(lit.Value, `"`)
												}
											}
										}
									}
								}
							}
							return true
						})
					}
				}
			}

			data := SchemaTemplateData{
				TableName:   tableName,
				Fields:      fields,
				GeneratedAt: time.Now(),
			}

			outputDir := filepath.Join(project.GenPath(), "schemas", adapter)
			if err := os.MkdirAll(outputDir, 0755); err != nil {
				return fmt.Errorf("failed to create output directory %s: %w", outputDir, err)
			}
			outputPath := filepath.Join(outputDir, fmt.Sprintf("%s_schema.sql", lo.SnakeCase(structName)))

			var buf bytes.Buffer
			if err := tmpl.Execute(&buf, data); err != nil {
				return fmt.Errorf("failed to execute schema template for %s: %w", structName, err)
			}

			if err := os.WriteFile(outputPath, buf.Bytes(), 0644); err != nil {
				return fmt.Errorf("failed to write generated schema for %s: %w", structName, err)
			}
			fmt.Printf("Generated schema for %s at %s\n", structName, outputPath)
		}
	}

	return nil
}

func parseFields(spec *ast.TypeSpec, adapter string) []Field {
	structType, ok := spec.Type.(*ast.StructType)
	if !ok {
		return nil
	}

	var fields []Field
	for _, field := range structType.Fields.List {
		if len(field.Names) == 0 || !field.Names[0].IsExported() {
			continue // Skip fields without names (e.g., embedded structs) or private fields
		}

		xqlTag := ""
		if field.Tag != nil {
			tag := reflect.StructTag(strings.Trim(field.Tag.Value, "`"))
			xqlTag = tag.Get("xql")
		}

		if xqlTag == "-" {
			continue // Skip ignored fields
		}

		goType := types.ExprString(field.Type)
		// For selector expressions like `time.Time`, we need to get the full type string.
		if se, ok := field.Type.(*ast.SelectorExpr); ok {
			if x, ok := se.X.(*ast.Ident); ok {
				goType = fmt.Sprintf("%s.%s", x.Name, se.Sel.Name)
			}
		}

		entityField := Field{
			GoName: field.Names[0].Name,
			GoType: goType,
			Name:   lo.SnakeCase(field.Names[0].Name),
		}

		parseDirectives(xqlTag, &entityField)

		if adapter != "" {
			if entityField.DBType == "" {
				entityField.DBType = sqlTypeFor(entityField.GoType, adapter, driversJSON)
			}

			if entityField.IsPK {
				_, warning := pkConstraintFor(entityField.GoType, entityField.DBType, adapter, driversJSON)
				entityField.Warning = warning
			}
		}

		fields = append(fields, entityField)
	}
	return fields
}

func parseDirectives(tag string, field *Field) {
	directives := strings.Split(tag, ";")
	for _, d := range directives {
		d = strings.TrimSpace(d)
		if d == "" {
			continue
		}

		parts := strings.SplitN(d, ":", 2)
		key := strings.ToLower(parts[0])
		var value string
		if len(parts) > 1 {
			value = parts[1]
		}

		switch key {
		case "pk":
			field.IsPK = true
		case "not null":
			field.IsNotNull = true
		case "unique":
			field.IsUnique = true
		case "index":
			field.IsIndexed = true
		case "name":
			field.Name = value
		case "type":
			field.DBType = value
		case "default":
			field.Default = value
		case "fk":
			fkParts := strings.SplitN(value, ".", 2)
			if len(fkParts) == 2 {
				field.FKTable = fkParts[0]
				field.FKColumn = fkParts[1]
			}
		}
	}
}

// sqlTypeFor returns the SQL type for a given Go type and adapter using the
// parsed drivers JSON (queried via gjson). If no mapping exists, it falls back
// to a sensible default.
func sqlTypeFor(goType string, adapter string, driversJSON []byte) string {
	if len(driversJSON) > 0 {
		path := fmt.Sprintf("%s.typeMapping.%s", adapter, goType)
		if res := gjson.GetBytes(driversJSON, path); res.Exists() {
			return res.String()
		}
	}
	// fallback defaults (conservative)
	switch goType {
	case "int64", "int":
		return "BIGINT"
	case "int32":
		return "INTEGER"
	case "int16":
		return "SMALLINT"
	case "int8":
		return "SMALLINT"
	case "bool":
		if adapter == "mysql" {
			return "TINYINT(1)"
		}
		return "BOOLEAN"
	case "string":
		return "TEXT"
	case "float32":
		return "REAL"
	case "float64":
		if adapter == "postgres" {
			return "DOUBLE PRECISION"
		}
		return "DOUBLE"
	case "time.Time":
		if adapter == "postgres" {
			return "TIMESTAMP WITH TIME ZONE"
		}
		return "DATETIME"
	case "[]byte":
		if adapter == "postgres" {
			return "BYTEA"
		}
		return "BLOB"
	default:
		return "TEXT"
	}
}

// pkConstraintFor returns the PK constraint clause for the given Go type and
// SQL type for the adapter. It normalizes the SQL type, tries exact and family
// fallbacks, and returns an optional warning if PK is used on a discouraged Go type.
func pkConstraintFor(goType string, sqlType string, adapter string, driversJSON []byte) (string, string) {
	if len(driversJSON) == 0 {
		return "", ""
	}
	infoPath := fmt.Sprintf("%s.pk", adapter)
	norm := strings.ToLower(strings.TrimSpace(sqlType))
	if i := strings.Index(norm, "("); i >= 0 {
		norm = strings.TrimSpace(norm[:i])
	}
	if res := gjson.GetBytes(driversJSON, infoPath+"."+norm); res.Exists() {
		v := res.String()
		if goType == "int8" {
			return v, "primary key defined on int8: small integer PKs are discouraged"
		}
		return v, ""
	}
	if res := gjson.GetBytes(driversJSON, infoPath+".integer"); res.Exists() {
		if strings.HasPrefix(norm, "int") || norm == "bigint" || norm == "smallint" || norm == "tinyint" {
			v := res.String()
			if goType == "int8" {
				return v, "primary key defined on int8: small integer PKs are discouraged"
			}
			return v, ""
		}
	}
	return "", ""
}
