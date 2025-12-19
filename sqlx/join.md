# SQLX Join (2-table only)

This document explains the **join design** of the `sqlx` package.

> Scope: `sqlx` intentionally supports **joins between exactly two tables** at a time.

---

## Why only 2-table joins?

We intentionally keep joins limited to **two entities** (`E1` and `E2`) because:

1. **Multi-table joins are often for reporting**
   - Reporting queries (3+ tables, many aggregations, ad-hoc projections) tend to be highly application-specific.
   - The design space (join graphs, aliasing, conflict resolution, optimization) quickly becomes large.

2. **We want to keep the API small, predictable, and type-safe**
   - Two-table joins cover the most common domain use-cases (e.g. `Account` + `Profile`).
   - Limiting to 2 tables avoids complex join graphs and ambiguous ownership/ordering issues.

---

## Why we don't support 3+ table joins (and what to do instead)

### Why it's out of scope

We don’t support 3+ table joins on purpose.

Even if we add a `Joint3[E1,E2,E3]` type, real-world multi-table joins quickly require:

- **join graphs** (which table joins to which table, join order)
- **aliasing** (especially for repeated joins or self-joins)
- **column name collision strategies** (projection conflicts across many tables)
- **more complex testing/snapshot coverage**

This complexity tends to push the API away from the goals of `sqlx`: **simple, predictable, and type-guided** query building.

### How to handle 3+ table joins

For 3+ table joins, the recommended approach is to use **raw SQL**.

Typical workflow:

1. Write a SQL file (or a SQL string) with the exact join logic.
2. Execute it via your preferred database access layer (`database/sql`, `sqlx`, etc.).
3. Scan results into a DTO / struct tailored for the reporting use-case.

Example skeleton (conceptual):

```sql
SELECT
  a.id   AS account_id,
  p.id   AS profile_id,
  o.id   AS order_id
FROM accounts a
JOIN profiles p ON p.account_id = a.id
JOIN orders   o ON o.account_id = a.id
WHERE a.id = ?
```

If you want, you can still reuse **generated field names** from the `xql` generator to keep column usage consistent, while leaving the multi-table join structure to SQL.

---

## Core contract

### Driven entity (E1)

`E1` is always the **driven/base** entity:

- SQL shape is conceptually: `FROM E1 ... JOIN E2 ...`
- Join logic can also be expressed as `WHERE EXISTS (SELECT 1 FROM E2 WHERE ...)` when only filtering `E1`.

This removes ambiguity and makes join behavior consistent.

### Join on multiple columns (composite keys)

A join can be defined on **multiple column pairs**.

For example, the join relationship may consist of multiple equality predicates:

- `A`, `B`, `C`, `D` …

where each predicate is a column equality between `E1` and `E2`, and the join relation is:

- `(A AND B AND C AND D ...)`

In SQL this corresponds to:

```sql
... JOIN e2 ON (
  e1.a = e2.a
  AND e1.b = e2.b
  AND e1.c = e2.c
)
```

---

## Predicate-first design (reuse `Where`)

Instead of building a brand-new boolean expression system for joins, join relationships are treated as **boolean predicates** that can be wrapped into the existing `Where` model.

This is important because:

- It guarantees correct parentheses and precedence for expressions like:
  - `(A AND B) OR C`
  - `(A OR B) AND C`
- It keeps the join API consistent with the rest of `sqlx`.

Conceptually:

- A composite join key group is one **atomic predicate**: `(A AND B AND C ...)`
- More complex boolean composition is built by combining these predicates using:
  - `Where.And(...)`
  - `Where.Or(...)`

---

## Planned public API (high level)

### `Query[T]` (single table)

- `Query[T]` remains the single-table query API.

### `QueryJoint[E1,E2]` (two-table join query)

For join queries (selecting a projection across two entities), we plan to provide:

- `QueryJoint[E1, E2](...)`

This makes the join intent explicit and keeps the single-table API (`Query[T]`) simple.

> Notes:
> - `QueryJoint` returns a slice of value objects representing the projected view fields.
> - `View[E1,E2](...)` is used to define the projection and also enforces the “only these two tables” rule.

---

## Unsupported cases

The following are intentionally out of scope:

- 3+ table joins (join graphs)
- reporting-style queries with many joins + aggregations
- complex alias management across repeated joins

For these scenarios:

- use raw SQL (recommended)
- or build a dedicated reporting/query layer on top
