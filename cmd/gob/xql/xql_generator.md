# XQL Generator Implementation Notes

## Purpose
- Translate entity structs (implementing `entity.Entity`) found in the working project into generated helpers under `gen/`.
- Keep the generator idempotent so repeated runs only update changed structs or schemas.
- Work with runtime driver detection (via `drivers.json` + go.mod) to produce database-specific artifacts.

## Entity Field Generation
1. **Discovery**: scan all Go packages (excluding `vendor`, `gen`, and tool-internal folders) and collect structs that satisfy `entity.Entity`.
2. **Output layout**:
   - Package: `{project_root}/gen/field/{strings.ToLower(structName)}`.
   - File: `lower_{structName}.go`.
   - Contents: `entity.Field[{Struct}, {FieldType}]("{FieldName}")` declarations for every exported field that survives `xql:"-"`.
3. **Idempotence**: re-generate the file completely each run; formatter (`gofmt`) ensures stable diffs.
4. **Imports**: only import `github.com/kcmvp/dvo/entity` and any scalar types that need package references (e.g., `time`).

Example snippet:
```go
var (
    ProductName  = entity.Field[Product, string]("Name")
    ProductPrice = entity.Field[Product, float64]("Price")
    UserCreatedAt = entity.Field[Product, time.Time]("CreatedAt")
)
```

## Schema Generation
1. **Adapter detection**: `xql` discovers drivers by matching go.mod deps against `cmd/gob/xql/drivers.json`. Each match yields a canonical adapter name (`sqlite`, `mysql`, `postgres`).
2. **Folder layout**:
   - Root: `{project_root}/gen/schema/{adapter}`.
   - One file per entity: `strings.ToLower(structName).sql`.
3. **DDL contents**:
   - Table name from `Entity.Table()` if implemented, otherwise snake_case(struct).
   - Column definitions derived from field tags + default mapping.
   - Only emit PK clauses for fields mapped to the `integer` bucket per adapter rules (per drivers.json `typeMapping.integer`). Warn when a user specifies `pk` on smaller ints (`int8`).
4. **Multiple adapters**: repeat generation per adapter; shared entities appear under each folder but adapt SQL types per adapter rules.
5. **Constraints / indexes**: honor directives parsed from `xql` tags (pk, not null, unique, index, fk, default, type override, ignore).

## CLI Flow (cmd/gob/xql/xql.go)
1. `xql schema` invokes:
   - project scan via `internal.Project` (already populated by root command).
   - driver inference; store adapter list in context (key `xql.dbAdapter`).
   - generator orchestrator in `xql_generator.go` to emit fields + schemas.
2. `xql validate` should reuse the parser to ensure tags + mappings are legal without writing files.
3. `xql index` remains a placeholder for future index helpers (document assumption for now).

## Outstanding Tasks
- Implement the actual generator in `cmd/gob/xql/xql_generator.go` using the above layout.
- Add tests that exercise discovery, driver detection, and file emission (likely under `cmd/gob/xql/xql_generator_test.go`).
- Wire `xql schema` to call the generator once the scaffolding is ready.
- Extend documentation once actual CLI flags (e.g., output dir overrides) are finalized.
