package xql

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/format"
	"go/token"
	"go/types"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"strings"
	"text/template"
	"time"

	_ "embed"

	"github.com/kcmvp/xql/cmd/internal"
	"github.com/samber/lo"
	"github.com/tidwall/gjson"
	"golang.org/x/tools/go/packages"
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
	ModulePkgName    string
	EntityImportPath string
	GeneratedAt      time.Time
}

// Field represents a single column in a database table, derived from a Go struct field.
type Field struct {
	Name       string // The database column name (e.g., "creation_time").
	GoName     string // The original Go field name (e.g., "CreatedAt").
	GoType     string // The Go type of the field (e.g., "time.Time").
	DBType     string // The specific SQL type for the column (e.g., "TIMESTAMP WITH TIME ZONE").
	IsPK       bool   // True if this field is the primary key.
	IsNotNull  bool   // True if the column has a NOT NULL constraint.
	IsUnique   bool   // True if the column has a UNIQUE constraint.
	IsIndexed  bool   // True if an index should be created on this column.
	Default    string // The default value for the column, as a string.
	FKTable    string // The table referenced by a foreign key.
	FKColumn   string // The column referenced by a foreign key.
	Warning    string // A warning message associated with this field, e.g., for discouraged PK types.
	IsEmbedded bool
}

// isSupportedType checks if a field type is valid.
func isSupportedType(typ types.Type) bool {
	// Check for named types like time.Time
	if named, ok := typ.(*types.Named); ok {
		if named.Obj().Pkg() != nil && named.Obj().Pkg().Path() == "time" && named.Obj().Name() == "Time" {
			return true
		}
	}

	// Check for basic types
	basic, ok := typ.Underlying().(*types.Basic)
	if !ok {
		return false
	}

	// Explicit kind set instead of a switch to avoid IDE warnings about missing iota cases.
	allowed := map[types.BasicKind]struct{}{
		types.Bool:    {},
		types.Int:     {},
		types.Int8:    {},
		types.Int16:   {},
		types.Int32:   {},
		types.Int64:   {},
		types.Uint:    {},
		types.Uint8:   {},
		types.Uint16:  {},
		types.Uint32:  {},
		types.Uint64:  {},
		types.Float32: {},
		types.Float64: {},
		types.String:  {},
	}
	_, ok = allowed[basic.Kind()]
	return ok
}

// applyOrderPolicy reorders a slice of fields based on the defined ordering policy:
// 1. Primary key fields
// 2. Host struct fields
// 3. Embedded struct fields
func applyOrderPolicy(fields []Field) []Field {
	var pkFields []Field
	var hostFields []Field
	var embeddedFields []Field

	for _, f := range fields {
		if f.IsPK {
			pkFields = append(pkFields, f)
		} else if f.IsEmbedded {
			embeddedFields = append(embeddedFields, f)
		} else {
			hostFields = append(hostFields, f)
		}
	}

	return append(append(pkFields, hostFields...), embeddedFields...)
}

// EntityMeta holds all the derived metadata needed to generate both field helpers
// and database schemas for one entity.
//
// Fields are ordered using applyOrderPolicy.
type EntityMeta struct {
	StructName string
	PkgPath    string
	Pkg        *packages.Package
	TypeSpec   *ast.TypeSpec
	TableName  string
	Fields     []Field // adapter-agnostic field info (no DBType)
}

// generate is the single entrypoint for this package's generation workflow.
// It builds entity metadata once, then generates both field helpers and schemas.
func generate(ctx context.Context) error {
	meta, err := generateMeta(ctx)
	if err != nil {
		return err
	}
	if err := generateFieldsFromMeta(meta); err != nil {
		return err
	}
	return generateSchemaFromMeta(ctx, meta)
}

// generateMeta builds a consistent metadata model from source code exactly once.
// Both field helpers and schema generation should consume this output to avoid
// drift and duplicated parsing logic.
func generateMeta(ctx context.Context) ([]EntityMeta, error) {
	project := internal.Current
	if project == nil {
		return nil, fmt.Errorf("project context not initialized")
	}

	entities := project.StructsImplementEntity()
	// Optional entity filtering:
	// - []string: explicit allow-list of struct names
	// - func(internal.EntityInfo) bool: advanced/internal filtering
	if v := ctx.Value(entityFilterKey); v != nil {
		switch vv := v.(type) {
		case []string:
			allow := make(map[string]struct{}, len(vv))
			for _, n := range vv {
				n = strings.TrimSpace(n)
				if n != "" {
					allow[n] = struct{}{}
				}
			}
			if len(allow) > 0 {
				entities = lo.Filter(entities, func(e internal.EntityInfo, _ int) bool {
					if e.TypeSpec == nil || e.TypeSpec.Name == nil {
						return false
					}
					_, ok := allow[e.TypeSpec.Name.Name]
					return ok
				})
			}
		case func(internal.EntityInfo) bool:
			entities = lo.Filter(entities, func(e internal.EntityInfo, _ int) bool {
				return vv(e)
			})
		}
	}

	if len(entities) == 0 {
		return nil, fmt.Errorf("no entity structs found")
	}

	metas := make([]EntityMeta, 0, len(entities))
	for _, entityInfo := range entities {
		structName := entityInfo.TypeSpec.Name.Name

		fields, err := parseFields(entityInfo.Pkg, entityInfo.TypeSpec, "")
		if err != nil {
			return nil, err
		}
		if len(fields) == 0 {
			return nil, fmt.Errorf("no supported fields found for entity %s", structName)
		}
		fields = applyOrderPolicy(fields)

		tableName, err := resolveTableName(project, entityInfo.PkgPath, structName)
		if err != nil {
			return nil, err
		}

		metas = append(metas, EntityMeta{
			StructName: structName,
			PkgPath:    entityInfo.PkgPath,
			Pkg:        entityInfo.Pkg,
			TypeSpec:   entityInfo.TypeSpec,
			TableName:  tableName,
			Fields:     fields,
		})
	}

	if len(metas) == 0 {
		return nil, fmt.Errorf("no entity structs found")
	}
	return metas, nil
}

func resolveTableName(project *internal.Project, pkgPath, structName string) (string, error) {
	// default fallback
	tableName := lo.SnakeCase(structName)

	// Find Table() method receiver matching structName in that package.
	for _, pkg := range project.Pkgs {
		if pkg.PkgPath != pkgPath {
			continue
		}
		for _, file := range pkg.Syntax {
			ast.Inspect(file, func(n ast.Node) bool {
				fn, ok := n.(*ast.FuncDecl)
				if !ok || fn.Name == nil || fn.Name.Name != "Table" {
					return true
				}
				if fn.Recv == nil || len(fn.Recv.List) == 0 {
					return true
				}

				recvMatches := func(t ast.Expr) bool {
					switch rt := t.(type) {
					case *ast.StarExpr:
						if ident, ok := rt.X.(*ast.Ident); ok {
							return ident.Name == structName
						}
					case *ast.Ident:
						return rt.Name == structName
					}
					return false
				}

				if !recvMatches(fn.Recv.List[0].Type) {
					return true
				}

				// We only support: `return "..."` to keep it deterministic.
				if fn.Body == nil || len(fn.Body.List) == 0 {
					return true
				}
				ret, ok := fn.Body.List[0].(*ast.ReturnStmt)
				if !ok || len(ret.Results) == 0 {
					return true
				}
				lit, ok := ret.Results[0].(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					return true
				}
				tableName = strings.Trim(lit.Value, `"`)
				return false
			})
		}
	}

	return tableName, nil
}

// generateFieldsFromMeta generates field helpers from the precomputed entity metadata.
func generateFieldsFromMeta(metas []EntityMeta) error {
	project := internal.Current
	if project == nil {
		return fmt.Errorf("project context not initialized")
	}

	tmpl, err := template.New("fields").Parse(fieldsTmpl)
	if err != nil {
		return fmt.Errorf("failed to parse fields template: %w", err)
	}

	for _, meta := range metas {
		imports := lo.Uniq(lo.FilterMap(meta.Fields, func(f Field, _ int) (string, bool) {
			if strings.Contains(f.GoType, ".") {
				pkg := strings.Split(f.GoType, ".")[0]
				switch pkg {
				case "time":
					return "time", true
				default:
					return "", false
				}
			}
			return "", false
		}))

		// compute module package name heuristically: try to load package to get declared name,
		// fall back to last path element if load fails.
		modulePkgName := path.Base(internal.ToolModulePath())
		if pkgs, _ := packages.Load(&packages.Config{Mode: packages.NeedName}, internal.ToolModulePath()); len(pkgs) > 0 {
			if pkgs[0].Name != "" {
				modulePkgName = pkgs[0].Name
			}
		}

		data := TemplateData{
			PackageName:      strings.ToLower(meta.StructName),
			StructName:       meta.StructName,
			Imports:          imports,
			Fields:           meta.Fields,
			ModulePath:       internal.ToolModulePath(),
			ModulePkgName:    modulePkgName,
			EntityImportPath: meta.PkgPath,
			GeneratedAt:      time.Now(),
		}

		outputDir := filepath.Join(project.GenPath(), "field", data.PackageName)
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			return fmt.Errorf("failed to create output directory %s: %w", outputDir, err)
		}
		outputPath := filepath.Join(outputDir, fmt.Sprintf("%s_gen.go", data.PackageName))

		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, data); err != nil {
			return fmt.Errorf("failed to execute template for %s: %w", meta.StructName, err)
		}

		formatted, err := format.Source(buf.Bytes())
		if err != nil {
			return fmt.Errorf("failed to format generated code for %s: %w", meta.StructName, err)
		}

		if err := os.WriteFile(outputPath, formatted, 0644); err != nil {
			return fmt.Errorf("failed to write generated file for %s: %w", meta.StructName, err)
		}
		// generation info suppressed in non-verbose mode
	}
	return nil
}

// generateSchemaFromMeta generates schemas from the precomputed entity metadata.
func generateSchemaFromMeta(ctx context.Context, metas []EntityMeta) error {
	project := internal.Current
	if project == nil {
		return fmt.Errorf("project context not initialized")
	}

	adapters, ok := ctx.Value(dbaAdapterKey).([]string)
	if !ok || len(adapters) == 0 {
		return fmt.Errorf("no database adapters are configured or detected")
	}

	funcMap := template.FuncMap{
		"plus1": func(i int) int { return i + 1 },
	}

	tmpl, err := template.New("schema").Funcs(funcMap).Parse(schemaTmpl)
	if err != nil {
		return fmt.Errorf("failed to parse schema template: %w", err)
	}

	for _, adapter := range adapters {
		for _, meta := range metas {
			fields := enrichFieldsForAdapter(meta.Fields, adapter)
			if len(fields) == 0 {
				continue
			}

			data := SchemaTemplateData{
				TableName:   meta.TableName,
				Fields:      fields,
				GeneratedAt: time.Now(),
			}

			outputDir := filepath.Join(project.GenPath(), "schemas", adapter)
			if err := os.MkdirAll(outputDir, 0755); err != nil {
				return fmt.Errorf("failed to create output directory %s: %w", outputDir, err)
			}
			outputPath := filepath.Join(outputDir, fmt.Sprintf("%s_schema.sql", lo.SnakeCase(meta.StructName)))

			var buf bytes.Buffer
			if err := tmpl.Execute(&buf, data); err != nil {
				return fmt.Errorf("failed to execute schema template for %s: %w", meta.StructName, err)
			}
			if err := os.WriteFile(outputPath, buf.Bytes(), 0644); err != nil {
				return fmt.Errorf("failed to write generated schema for %s: %w", meta.StructName, err)
			}
			// generation info suppressed in non-verbose mode
		}
	}

	return nil
}

// enrichFieldsForAdapter clones the base fields and fills DBType/PK warnings for the given adapter.
// This avoids re-parsing AST/types multiple times.
func enrichFieldsForAdapter(base []Field, adapter string) []Field {
	fields := make([]Field, len(base))
	copy(fields, base)
	for i := range fields {
		if fields[i].DBType == "" {
			fields[i].DBType = sqlTypeFor(fields[i].GoType, adapter, driversJSON)
		}
		if fields[i].IsPK {
			_, warning := pkConstraintFor(fields[i].GoType, fields[i].DBType, adapter, driversJSON)
			fields[i].Warning = warning
		}
	}
	return fields
}

func parseFields(pkg *packages.Package, spec *ast.TypeSpec, adapter string) ([]Field, error) {
	// NOTE: adapter is intentionally ignored now; adapter-specific typing happens in enrichFieldsForAdapter.
	_ = adapter
	structType, ok := spec.Type.(*ast.StructType)
	if !ok {
		return nil, nil
	}

	var fields []Field
	for _, field := range structType.Fields.List {
		if len(field.Names) == 0 { // Embedded struct
			var ident *ast.Ident
			switch t := field.Type.(type) {
			case *ast.Ident:
				ident = t
			case *ast.SelectorExpr:
				ident = t.Sel
			}

			if ident != nil && ident.Obj != nil && ident.Obj.Kind == ast.Typ {
				if embeddedSpec, ok := ident.Obj.Decl.(*ast.TypeSpec); ok {
					embeddedFields, err := parseFields(pkg, embeddedSpec, adapter)
					if err != nil {
						return nil, err
					}
					for i := range embeddedFields {
						embeddedFields[i].IsEmbedded = true
					}
					fields = append(fields, embeddedFields...)
				}
			}
			continue
		}

		if !field.Names[0].IsExported() {
			continue // Skip private fields
		}

		// Check if the field is a struct type that should be skipped
		if tv, ok := pkg.TypesInfo.Types[field.Type]; ok {
			if !isSupportedType(tv.Type) {
				if _, ok := tv.Type.Underlying().(*types.Struct); !ok {
					return nil, fmt.Errorf("unsupported field type %s for field %s", tv.Type.String(), field.Names[0].Name)
				}
			}
			if _, ok := tv.Type.Underlying().(*types.Struct); ok {
				// Allow time.Time, but skip other structs
				if tv.Type.String() != "time.Time" {
					continue
				}
			}
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

		fields = append(fields, entityField)
	}
	return fields, nil
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

var _ = errors.New
