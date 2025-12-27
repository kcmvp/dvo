# Entity design & principles

This document defines what an **Entity** is in this project and the recommended rules for designing entity structs.

It is used as source material for the final `README.md`.

---

## What is an Entity?

An **Entity** is a plain Go struct that represents a table-like persistence model.

In code, an entity is any type that implements:

```go
// package entity

type Entity interface {
	Table() string
}
```

- `Table()` returns the table name used for schema generation and SQL building.
- Entities are intentionally **simple**: they are not an ORM, and they do not encode relationships.

---

## Core principles

### 1) Keep entities as plain data carriers

Entities should:
- be plain structs (no constructor required)
- have exported fields for persistence
- avoid business logic (pure domain logic should live elsewhere)

### 2) No relationship modeling inside structs

We do **not** model relationships through pointer/slice fields:

- Avoid `Orders []Order`, `Profile *Profile`, etc.
- Avoid `gorm:"foreignKey"`-style assumptions.

Instead, we model relationships **only through scalar key fields** such as `AccountID`, `OrderID`, `RoleID`, etc.

This keeps the design explicit and avoids hidden coupling.

### 3) No database foreign keys required

Even if an entity contains fields like `AccountID`, we do not require DB-level foreign key constraints.

- Joins are built purely from field providers (e.g. `Join(Account.ID, Order.AccountID)`).
- Schema generation focuses on column definitions and primary key constraints.

> You can still add FK constraints manually in your own migrations if you want them.

---

## How fields are discovered by generators

The generators (XQL) scan entity structs and treat fields under the following rules.

### Exported fields only

Only **exported** Go fields (capitalized) are considered persistence columns.

- `internalNotes string` is ignored.

### Ignoring fields

You can explicitly ignore a field using:

```go
Password string `xql:"-"`
```

### Embedded anonymous structs are supported

Anonymous embedded structs are expanded and their exported fields are treated as columns.

Recommended pattern:

```go
type BaseEntity struct {
	ID        int64 `xql:"pk"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Account struct {
	BaseEntity // embedded (anonymous)
	Email string
}
```

### Named fields that are structs are NOT expanded

A named struct field (non-anonymous) is treated as a normal field of a composite type.
Since composite types are not currently supported by field/schema generation, a named struct field should be used only for non-persistent information and should be ignored or kept out of entities.

Example (non-embedded):

```go
type Dummy struct { TestingName string }

type Account struct {
	Dummy Dummy // named -> NOT expanded
}
```

---

## Struct tag: `xql` directives

Entities can use the `xql` struct tag to control mapping.

Basic format:

```go
FieldName <type> `xql:"directive1;directive2;..."`
```

Common directives:

- `pk` — mark field as primary key
- `name:<column_name>` — override default column name (default uses snake_case)
- `type:<sql_type>` — override inferred SQL type
- `not null`, `unique`, `index`, `default:<value>`
- `-` — ignore field

> For full directive details and Go→DB type mapping, see `cmd/gob/xql/type_mapping.md`.

---

## Supported field types (current)

For persistence fields and XQL generation, the supported Go types are a subset defined by `constraint.FieldType`.
As of now, supported types are:

- numbers (`int`, `int8`, `int16`, `int32`, `int64`, `uint*`, `float32`, `float64`)
- `string`
- `bool`
- `time.Time`

If an entity contains an unsupported field type (e.g. `chan`, `map`, `[]T`, `func`), generation must **fail fast** with an error.

---

## Relationship patterns (without ORM)

You can express common relationship shapes without embedding relationships.

### One-to-one (1:1)

Use a foreign key-like scalar field:

```go
type Account struct {
	BaseEntity
	Email string
}

type Profile struct {
	BaseEntity
	AccountID int64
	Bio       string
}
```

Your join is explicit in the DSL:

- `Join(Account.ID, Profile.AccountID)`

### One-to-many (1:N)

```go
type Order struct {
	BaseEntity
	AccountID int64
	Amount    float64
}
```

Join:

- `Join(Account.ID, Order.AccountID)`

### Many-to-many (N:N)

Use an explicit join table entity:

```go
type AccountRole struct {
	BaseEntity
	AccountID int64
	RoleID    int64
}
```

Join:

- `Join(Account.ID, AccountRole.AccountID)`
- `Join(AccountRole.RoleID, Role.ID)`

---

## Recommendations

- Prefer `int64` for IDs.
- Use `xql:"pk"` for the primary key.
- Reuse a shared embedded struct (e.g. `BaseEntity`) for common columns.
- Keep entity structs stable and simple; build view-level validation in a dedicated layer (`view`) rather than here.

