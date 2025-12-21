# Phrase 2 Refactor Design — Clean Architecture (Layered / Ports & Adapters) (meta / field / value / view)

This document captures the agreed design for Phrase‑2 refactoring: split responsibilities into small packages so we have a single source of truth for field identity while keeping validation (view) and persistence (value / sqlx) decoupled.

Status
- Packages created: `meta`, `field`, `value`, `view` (dependency chain: `meta <- field <- value <- view`).
- This doc records responsibilities, public contracts (shapes), ordering & PK rules, error modes, tests, migration steps and open decisions.

Goals
- Single source of truth for field identity and metadata.
- Keep validation (view) separated from persistence logic and SQL generation.
- Ensure deterministic code and schema generation with stable ordering and predictable templates.

## Principles

To keep the repository stable and maintain a clear, incremental migration path, follow these principles from now on:

- Single-source-of-change: Do not modify source code outside the `cmd` folder unless absolutely necessary. All refactor work (metadata emission, generator logic, scaffolding, and migration helpers) should be implemented in `cmd/*` (for example `cmd/gob/xql`). This keeps the project buildable at all times and makes generated artifacts the canonical source for schema/field metadata.

- Meta as authoritative metadata: Move runtime schema/field metadata into the `meta` layer (and emitted generated files). Runtime code should consume the metadata from generated artifacts rather than re-scanning entity structs.

- Keep table info stable: `entity` may expose `Table()` and `entity`-level helpers, but the generator should emit `Table` into generated `FieldMeta` when available. Prefer consuming generator-emitted table metadata for DDL and SQL generation to avoid re-invoking entity methods at runtime.

- Field factory signature and responsibility split:
  - The new canonical factory signature should be:

    `func Field[E entity.Entity, T constraint.FieldType](name string, col string, jsonName string, vfs ...constraint.ValidateFunc[T]) FieldProvider[E]`

    This signature records provider name, DB column name and JSON name explicitly.
  - Important separation: validator functions (`vfs ...constraint.ValidateFunc`) belong to the `view` layer. The `meta` and `field` layers must not depend on validator types or validation logic. Generated metadata should not embed validation logic; instead, `view` binds validators to providers at runtime or in view-specific generated wrappers.

- Functional style: Prefer a functional programming approach when implementing the core logic in `meta` and generator code:
  - Favor pure functions with no side-effects: functions should take inputs and return outputs (and errors) rather than mutating global state.
  - Use immutable data shapes (or treat data as immutable): `FieldMeta` and related structures should be treated as values, and transformations should produce new values.
  - Keep functions small and composable: build complex behavior by composing small, well-tested pure functions (ordering, classification, expansion, validation).
  - Return explicit errors instead of panics; avoid hidden global state and singletons for core metadata.
  - Use `context.Context` for cancellation/timeouts and pass configuration via explicit parameters or functional options rather than global variables.

- Backwards compatibility: If needed, create compatibility shims in `cmd` or a thin wrapper in `entity` that delegates to the new `field` factory to avoid breaking existing call sites. Migrate consumers to the new pattern incrementally.

- Generator-first approach: Treat the generator as the authoritative writer of schema and field metadata. Developers should re-run the generator when changing entity definitions; runtime components will consume generated metadata and thus avoid ad-hoc reflection/parsing.

Quick summary of responsibilities

- meta
  - Core, low-level primitives and helpers describing field identity and metadata.
  - No JSON parsing, no validators, no DB driver specifics.
  - Key responsibilities: field collection (expand embeds), deterministic naming (snake_case), ordering, type classification (buckets), lightweight validations (unsupported types), and stable small structs for codegen.

- field
  - Generated typed Field providers (one package per entity as produced by the generator).
  - Depends on `meta` for FieldMeta.
  - Exported variables/functions that callers (value/view) use as identifiers.

- value
  - Persistence-oriented data transfer objects and mapping helpers.
  - Scans DB rows into value containers, maps validated view input into persistence-ready values.
  - Uses `field` identifiers to map column names and types.

- view
  - All HTTP request parsing and validation logic.
  - Uses `field` identifiers (or `meta.FieldRef`) to build validation schemas.
  - Produces `value` objects for persistence and renders `value` objects to JSON for responses.

Public contracts (shapes and functions — signatures only)

- meta
  - type FieldRef interface { Name() string }
  - type FieldMeta struct { Name string; Column string; GoType string; IsExported bool; IsPK bool; IsIgnored bool; Tags map[string]string; OriginPkg string }
  - func ExpandAndCollectFields(pkgPath, structName string) ([]FieldMeta, error)
  - func OrderFields(fields []FieldMeta) []FieldMeta
  - func ClassifyGoType(goType string) (bucket string)
  - func ValidateFieldTypes(fields []FieldMeta) (warnings []string, errs []error)

- field
  - generator will produce per-entity providers like:
    - var ID = NewProvider[Entity, int64]("ID", meta.FieldMeta{...})
  - FieldProvider interface with: ProviderName(), ColumnName(), TypeName()

- value
  - type Value[T any] struct { /* map-backed or typed */ }
  - func NewValueFromRow[T any](cols []string, row []any) (*Value[T], error)
  - func (v *Value[T]) GetByProvider(p field.FieldProvider[T, any]) (any, bool)
  - func (v *Value[T]) ToEntity() (T, error)

- view
  - type Schema[T any] struct { Fields []field.FieldProvider[T, any]; rules ... }
  - func (s Schema[T]) ValidateJSON(json []byte) (*value.Value[T], error)
  - func (s Schema[T]) RenderJSON(v *value.Value[T]) ([]byte, error)
  - Middleware helpers (framework-level wrappers live in gin/echo/fiber vom packages)

Field & column ordering policy
- Single canonical `OrderFields` implementation in `meta`.
- Ordering rules (deterministic):
  1. Primary key fields (IsPK==true) first — preserve relative order if multiple PKs.
  2. Host struct exported fields in declaration order.
  3. Anonymous embedded structs appended afterwards; each embed preserves its internal declaration order.
- Embedded PKs: if PK appears in an anonymous embed, it must be promoted to be the first column overall.
- Private (unexported) fields are ignored and must not be generated.

PK and type mapping policy
- Only map PK auto-increment semantics to the `integer` bucket (Go int/int64) automatically.
- If user marks `pk` on a type outside the integer bucket (e.g., int8, string), generator behavior:
  - Default: fail-fast with a clear error explaining the type mismatch. (Recommended for safety.)
  - Alternative (not default): emit a warning and generate but without DB-specific auto-increment clause.

Naming & qualification
- Provider name: raw Go field name, e.g. `AccountID`.
- Column name: deterministic snake_case of provider name, e.g. `account_id`. Use existing helper (`lo.SnakeCase`) for consistency.
- Qualified names (when needed for where clauses): `table.column` or generator may emit `QualifiedColumn`.

Driver/adapters & generator
- `cmd/gob/xql` will embed a `drivers.json` resource and use `gjson` to discover driver names in go.mod. Use `gjson.Get(driversJSON, "#.drivers")` to list drivers.
- Driver detection logic: match go.mod module names to driver lists; return canonical adapter name (`sqlite`, `mysql`, `postgres`) for generating per-adapter DDL.
- Templates (fields.tmpl, schema.tmpl) must not hardcode the module import path; generator must obtain module path at runtime from `internal.Project.ToolModulePath()` and inject it into templates.

Error modes — generator vs runtime
- Generator: prefer fail-fast for ambiguous/unsupported cases (unsupported field type, duplicate exported provider names, illegal PK). Provide actionable errors.
- Runtime (view/value/sqlx): return explicit errors with context (entity, field) for parsing/validation/mapping failures.

Testing strategy
- Unit tests for `meta` (ordering, type classification, field expansion, unsupported types).
- Snapshot tests for generator output (generated `field` packages and SQL schema files) in `testdata/`.
- SQL builder snapshot tests in `sqlx/testdata/sql` and integration tests using `application_test.yml` + sqlite memory DB.
- `view` tests for validation and middleware; `value` tests for scanning and conversion.

Migration & implementation plan (atomic steps)
1. Doc & interface scaffolding
   - Commit `meta/README.md` and interface-only files to lock the contract.
2. Implement `meta.OrderFields`, `ClassifyGoType`, unit tests.
3. Adjust generator to use `meta` metadata shape, fix templates to accept module path, embed `drivers.json` and use `gjson`.
4. Implement `value` mapping helpers (NewValueFromRow, GetByProvider) and tests.
5. Implement `view` validation package (migrate `vo.go` logic into `view`, preserving `vo.go` façade for compatibility).
6. Run generator against `sample` and verify generated outputs with snapshot tests.

Docs to add or update
- `meta/README.md` — contract and ordering rules (first doc to add).
- `field/README.md` — generator output expectations and usage examples.
- `value/README.md` — mapping and type conversion rules.
- `view/README.md` — validation patterns and middleware examples.
- Update `cmd/gob/xql/type_mapping.md` and `xql_generator.md` to reflect new meta/field/value/view design.

Open questions requiring confirmation
1. Generator strictness on PK type mismatches — default to fail-fast? (recommended: yes)
2. Gen path behavior: tests should generate into `sample/gen/` while normal runs generate into `gen/` — confirm.
3. Anonymous embed semantics: confirm append-after-host ordering. (recommended: yes)

Next steps (recommended immediate actions)
- Confirm the three open questions above.
- Create `meta/README.md` and minimal interface files to lock contract.
- Implement `meta.OrderFields` + unit tests (small, high value).

Appendix — quick glossary
- FieldRef: minimal identifier used by view/value (`Name()`).
- FieldMeta: canonical metadata for a field used by generator and runtime (Name, Column, GoType, Tags, IsPK).
- FieldProvider: generated typed provider used by code and templates.


*End of document.*
