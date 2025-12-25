# sqlx — Consolidated Design Notes

This document consolidates the content of the following files in `sqlx/`:
- `join.md`
- `pure.md`
- `sqlx.md`
- `special_query.md`

It gathers the current design, goals, invariants, and prioritized TODOs in one place and flags items that may be outdated or need review. Use this file as the single reference for the package-level design; the original files remain in the repository for historical context.

---

## Table of Contents

- Summary
- Core goals and constraints (from `pure.md`)
- Single-table CRUD design (detailed)
- Where DSL and helpers
- Join design (2-table only) — consolidated
- Missing features / known gaps (from `sqlx.md`)
- Special queries and priority work (from `special_query.md`)
- Outdated / review-notes (items to re-check)
- Next steps (actionable checklist)

---

## Summary

`sqlx` aims to provide a small, generator-friendly SQL DSL and execution layer focused on deterministic SQL generation for simple CRUD and two-table joins. The package relies on generator-produced metadata (`meta.Schema`, `meta.Field`, `meta.ValueObject`) and emphasizes:

- Pure function-style SQL generation
- Small and predictable public API
- Deterministic column/alias naming (stable mapping to value objects)
- Safety rules for Update/Delete (require WHERE)

This consolidation gathers existing docs and priorities in one place and flags areas where designs have drifted or need confirmation.

---

## Core goals and constraints (extracted from `pure.md`)

- SQL generation is pure and deterministic; `sql()` returns SQL and is side-effect free.
- Public API surface is intentionally small: `Query`, `Delete`, `Update`, and join variants.
- The package uses `meta.Schema` (`[]meta.Field`) as the canonical projection description.
- `Where` is the only way to express predicates; combinators (`And`, `Or`) manage parentheses and precedence.
- Use `?` placeholders by default. Postgres-style `$1, $2` is a later enhancement.
- For safety, `Update` / `Delete` must be called with non-empty `Where`.

---

## Single-table CRUD design (detailed)

API shape (current/target):

- Query
  - `Query[T](schema meta.Schema) func(where Where) Executor`
  - Execution: `Executor.Execute(ctx, *sql.DB) -> mo.Either[[]meta.ValueObject, sql.Result]`
  - `selectSQL` generates `SELECT <cols> FROM <table> [WHERE ...]` using `schema` order and deterministic `table__column` aliases for mapping.

- Update
  - `Update[T](values meta.ValueObject) func(where Where) Executor`
  - Implementation reads schema from `meta.SchemaOf[T]()` (registered schema) at runtime.
  - `updateSQL` builds `UPDATE <table> SET col = ? ... WHERE <clause>` and uses the provided `meta.ValueObject` (or all placeholders when nil).
  - Safety: `where` required and must produce a non-empty clause.

- Delete
  - `Delete[T](where Where) Executor`
  - `deleteSQL` enforces non-empty `where` to prevent accidental full-table deletes.

- Count, Exists, CountDistinct (special-query helpers) — planned priorities in `special_query.md`.

Execution contract:
- Final executors accept `(context.Context, *sql.DB)`; results are either `[]meta.ValueObject` (select) or `sql.Result` (non-query).

Mapping rules:
- Projection columns are produced from `meta.Field.QualifiedName()` and aliased as `table__column` so `rowsToValueObjects` can reliably map results back to field names.
- Private fields (unexported struct fields) are not included in generation.

---

## Where DSL

- `Where` is an interface: `Build() (string, []any)`.
- Primitive predicates: `Eq/Ne/Gt/Gte/Lt/Lte/Like/In` take a `meta.Field` and value(s) and return a `Where`.
- Combinators: `And`, `Or` accept multiple `Where` and produce parenthesized expressions.
- `In` with empty values yields a safe `1=0` clause.

Implementation detail:
- `whereFunc` (function type) is used to adapt closures into `Where` values by providing a `Build` method.

---

## Join design (two-table only) — consolidated from `join.md`

Principles:
- `sqlx` intentionally supports only 2-table joins: `E1` (driven/base) and `E2` (joined).
- This keeps the API small and avoids complex join graphs and aliasing issues.
- For 3+ table joins or complex reporting queries, recommend using raw SQL.

Key points:
- `E1` is the driven/base entity (the table being primarily operated on). Joins can be expressed as `FROM E1 JOIN E2 ...` or as `WHERE EXISTS (SELECT 1 FROM E2 WHERE ...)` when the intention is filtering E1.
- Join predicates can be composite (multiple equality predicates combined with `AND`).
- Join predicates should reuse the `Where` DSL to get correct boolean composition and parentheses handling.

Public API (planned):
- `QueryJoin(schema meta.Schema) func(joinstmt string, where Where) Executor` — implemented in the codebase as a pragmatic approach where `joinstmt` is injected into the FROM clause; parameters must be provided via `Where`.
- `DeleteJoin` and `UpdateJoin` are implemented via `EXISTS`-style semantics: the join becomes an inner query used for filtering.

Constraints/requirements:
- `joinstmt` must not contain `?` placeholders; all parameters must be supplied through the `Where` value.
- The code currently expects a single `JOIN ... ON ...` pattern when translating `joinstmt` to `EXISTS(...)` for delete/update.

---

## Missing features / Known gaps (from `sqlx.md`)

These items were documented as missing in the original `sqlx.md` and should be prioritized or reviewed:

1. Core implementation completeness — some higher-level helpers were pending; now implemented for the main use-cases.
2. Transaction management — no explicit Tx API currently; design decisions required on how to expose Tx support in the fluent API.
3. Advanced query features (aggregation, GROUP BY, HAVING) — deferred.
4. Dialect support — currently `?` placeholders; Postgres placeholder style is a planned enhancement.
5. Connection management — `db.go` provides data source registry; `sqlx` executors currently accept `*sql.DB`.

---

## Special queries and priority work (from `special_query.md`)

Priority 1 (implement now):

- `Count[T](ctx, schema *Schema, where Where) (int64, error)` — `SELECT COUNT(1) FROM table [WHERE ...]`.
- `Exists[T](ctx, schema *Schema, where Where) (bool, error)` — `SELECT 1 FROM table WHERE ... LIMIT 1`.
- `CountDistinct[T](ctx, schema *Schema, field FieldProvider, where Where) (int64, error)` — `SELECT COUNT(DISTINCT <col>) FROM table [WHERE ...]`.

Priority 2 (soon):
- `First[T](...)` — single-row fetch (LIMIT 1) returning a `meta.ValueObject`.

Notes on implementation:
- Use deterministic field->column mapping: `db_column = lo.SnakeCase(providerName)` (or rely on `meta.Field.QualifiedName()` where available).
- Require fully-qualified provider tokens in `Where` to avoid ambiguous replacements; unqualified support can be added carefully later.

Tests to add:
- Table-driven tests for Count/Exists/CountDistinct using `testdata/sqlite_data.json`.

---

## Outdated / review notes (things to re-check)

While consolidating, I identified areas that likely need review or may be outdated relative to the current code and tests:

- Placeholder strategy: many docs assume `?`. If we decide to fully support Postgres, we must add a placeholder adapter.
- `pure.md` contains a few typed-generic `Where[T]` signatures; the implemented code uses `Where` without generics in places — verify the intended generic usage.
- The `sqlx.md` file described many unimplemented functions; most core builders are now implemented in `builder_helpers.go` — mark `sqlx.md` items as reviewed or removed.
- `join.md` suggests a `QueryJoint[E1,E2]` API; current `QueryJoin(schema)` uses a string `joinstmt`. Consider whether to change to a typed API in the future.
- `special_query.md` suggested `CountDistinct` and `Count` API signatures that include passing `schema *Schema` — current code often retrieves schema from `meta.SchemaOf[T]()`; pick one consistent approach and update docs.

Action: review the items above and decide which documentation lines should be amended or removed.

---

## Next steps (actionable checklist)

- [ ] Review and accept this consolidated document; remove or archive the older MD files if you want a single source-of-truth.
- [ ] Decide on schema lookup strategy: pass `schema` into public APIs or use `meta.SchemaOf[T]()` runtime registry — update docs and code for consistency.
- [ ] Implement/verify Priority 1 special queries (`Count`, `Exists`, `CountDistinct`) and add tests using `testdata/sqlite_data.json`.
- [ ] Decide whether to keep `QueryJoin` as string-based `joinstmt` or replace with a typed `QueryJoint[E1,E2]` generic API; update `join.md` accordingly.
- [ ] Add a short section on dialect strategy (placeholders) — finalize whether `sqlx` will adopt an adapter for `$1` vs `?`.
- [ ] Remove or mark out-of-date the original `*.md` files (optional) once you accept this consolidation.

---

### Document provenance

This file was generated by consolidating the `sqlx` package markdown files. It intentionally preserves the original design text where useful and adds flags for review. If you want, I can also:

- Produce a cleaned, canonical `sqlx/README.md` based on this file.
- Create a PR that renames old docs to `.md.bak` after you accept the consolidation.

Tell me which of the next steps you'd like me to execute, and I will proceed (implement tests, add Count/Exists/CountDistinct, or create README and archive old docs).
