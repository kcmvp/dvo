# Special Query TODOs for sqlx

Purpose: collect and prioritize the "special queries" (helpers) we want to provide on top of the generic `Query` API, document their API signatures, behavior, error modes, tests and implementation notes. This is a living TODO used to drive small, focused PRs.

Summary checklist (high-level)

- [x] Design and document the set of special queries we will implement
- [ ] Implement Priority 1 helpers: `Count`, `Exists`, `CountDistinct` (deterministic mapping: snake_case(provider) -> db column)
- [ ] Add unit/integration tests for Priority 1 helpers using `testdata/sqlite_data.json`
- [ ] Implement Priority 2 helper: `First` (single-row fetch)
- [ ] Add tests for `First`
- [ ] Optionally add metadata lookup for postgres/mysql (information_schema) if needed later
- [ ] Add an optional in-memory cache for table->column metadata as an enhancement (not required for deterministic mode)
- [ ] Iterate on Where rewriting to support safe replacements and unqualified names (optional)

Priority and what to implement now

- Priority 1 (implement now)
  - `Count[T](ctx, schema *Schema[T], where Where[T]) (int64, error)`
  - `Exists[T](ctx, schema *Schema[T], where Where[T]) (bool, error)`
  - `CountDistinct[T](ctx, schema *Schema[T], field FieldProvider[T], where Where[T]) (int64, error)`

- Priority 2 (soon)
  - `First[T](ctx, schema *Schema[T], where Where[T]) (*ValueObject[T], error)`

- Deferred (not in scope now)
  - Aggregates: Sum/Avg/Min/Max
  - Pagination: QueryPage / PageResult
  - Upsert / InsertReturning / BulkInsert
  - Composite/GetByPK helper (GetByPK can be composed via `First` + Eq)

Rationale (why these helpers?)

- Small, focused helpers (Count, Exists, First, CountDistinct) are common and reduce boilerplate in callers.
- They reuse existing `Schema` + `Where` semantics so behavior is consistent with `Query`.
- Keep initial implementation deterministic (snake_case mapping). Metadata lookup for database schemas is optional and can be added later if needed.

API signatures (finalized)

- Count

  ```go
  func Count[T entity.Entity](ctx context.Context, schema *Schema[T], where Where[T]) (int64, error)
  ```

  - Returns number of rows matching `where`. If `where` is nil or empty, returns total rows.
  - Errors: if `schema` is invalid, `DefaultDS()` missing, or DB error.

- Exists

  ```go
  func Exists[T entity.Entity](ctx context.Context, schema *Schema[T], where Where[T]) (bool, error)
  ```

  - Returns true if at least one row matches `where`. Uses `SELECT 1 FROM <table> WHERE ... LIMIT 1`.
  - Errors: same as `Count`.

- CountDistinct (single-field)

  ```go
  func CountDistinct[T entity.Entity](ctx context.Context, schema *Schema[T], field FieldProvider[T], where Where[T]) (int64, error)
  ```

  - Returns the number of distinct non-NULL values for the specified `field` among rows matching `where`.
  - Implementation: `SELECT COUNT(DISTINCT <table>.<db_column>) FROM <table> [WHERE ...]` using the same deterministic mapping logic as `Count`.
  - Errors: if `schema == nil`, `field` not found in `schema`, `DefaultDS()` missing, metadata/mapping fails, or DB error.
  - Notes: NULL values are not counted by SQL `COUNT(DISTINCT ...)`; this is the standard behavior.

- First (Priority 2)

  ```go
  func First[T entity.Entity](ctx context.Context, schema *Schema[T], where Where[T]) (*ValueObject[T], error)
  ```

  - Returns a single `ValueObject[T]` or `sql.ErrNoRows` if not found. Implemented by reusing `Query` but adding `LIMIT 1`.

Implementation notes (how to integrate with existing code)

1. Field->DB mapping
   - Use deterministic mapping: `db_column = lo.SnakeCase(providerName)`.
   - This avoids runtime DB metadata queries and keeps the implementation simple.

2. Where rewriting
   - `Where.Build()` yields a clause and args. Rewrite only fully-qualified provider tokens (e.g. `accounts.ID`) using the mapping to avoid accidental replacements inside literals.
   - Keep replacements simple (string.ReplaceAll) for the first pass. Document that callers should prefer fully-qualified tokens.

3. Metadata: optional
   - If needed later, we can add metadata lookups (PRAGMA / information_schema) as an optional enhancement and cache results. Not required for the deterministic initial mode.

4. Execution and scanning
   - `Count` and `CountDistinct` scan into `sql.NullInt64` and return the underlying `int64`.
   - `Exists` uses `LIMIT 1` and returns `true` if a row is returned.

5. Error handling
   - Fail-fast: if mapping indicates a provider referenced in `where` but no corresponding DB column found at exec time, return the DB error.
   - If `DefaultDS()` is not available, return a clear error.

6. Caching (later)
   - Metadata caching is optional and a future enhancement; not required now.

Edge cases and notes

- Unqualified field names in `Where`:
  - For now require qualified names (table.Field) in Where expressions to keep token replacement deterministic.
  - Optionally, later support unqualified names by scanning clause tokens and attempting to disambiguate them using the provided schema (danger: ambiguous matches).

- Composite primary keys:
  - `GetByPK` helper is deferred. Use `First` + `Eq` to fetch by arbitrary predicates including composite conditions.

- SQL dialect differences:
  - `COUNT(*)` and `LIMIT 1` are portable across sqlite/mysql/postgres for our use cases. Metadata queries differ and will be handled per-driver if/when we add them.

Tests to add

- Table-driven tests for `Count` using `testdata/sqlite_data.json`:
  - `Count(..., nil)` equals fixture length for each table.
  - `Count` with simple `Eq`/`And` conditions returns expected values.

- `Exists` tests:
  - True for existing value, false for impossible predicate.

- `CountDistinct` tests:
  - Single-field distinct: verify the number of unique non-null values for a column equals expectancy from `testdata` fixture.
  - Distinct with WHERE filter: `CountDistinct` with predicate matches expected unique counts over filtered set.

- Negative tests:
  - Where references unknown provider -> expect error.
  - No DefaultDS() -> expect error.

Files to change

- `sqlx/sqlx.go` — ensure deterministic mapping is used for Query/Count/Exists/CountDistinct.
- `sqlx/sqlx_test.go` — add table-driven tests for Count/Exists/CountDistinct.
- Add TODO: extend metadata queries for postgres/mysql in a follow-up.

Concrete next step (if you approve)

- I will implement Priority 1: add deterministic Count/Exists/CountDistinct using snake_case mapping and tests. I will not include PRAGMA-based metadata lookups.
- If you prefer to reintroduce optional metadata, we can add a cached metadata helper later.

If you approve, reply `implement` and I'll start coding the changes.
