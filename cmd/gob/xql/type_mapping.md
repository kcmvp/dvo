# XQL Type Mapping for Go Structs

This document outlines the rules for mapping Go struct fields to database table columns using the `xql` struct tag. The tool uses these tags to generate SQL schema definitions.

# Command-Line Usage

The `xql` command-line tool is used to work with your entity definitions. It is assumed that you have a main entry point for your application that registers the `xql` command group.

### `xql schema`

This command scans all Go files in the current project, finds all structs that implement the `entity.Entity` interface, and generates the appropriate SQL `CREATE TABLE` statements for them.

**Example:**
```bash
# Assuming your main.go is in ./cmd/gob/
go run ./cmd/gob xql schema
```

### `xql validate`

The `validate` command inspects all entity definitions to ensure that `xql` tags are correctly formatted and the mappings are valid, preventing errors during schema generation.

**Example:**
```bash
go run ./cmd/gob xql validate
```

### `xql index`

This command is intended to generate or update index files based on your entity definitions.

**Example:**
```bash
go run ./cmd/gob xql index
```

---

## Complete Example

Here is an example demonstrating various directives.

```go
package model

import "time"

// User implements the entity.Entity interface.
type User struct {
	ID        int64     `xql:"pk"`
	Email     string    `xql:"type:varchar(255);unique;not null"`
	Password  string    `xql:"-"` // Ignored field
	IsActive  bool      `xql:"default:true"`
	CreatedAt time.Time `xql:"name:creation_time"`
}

func (User) Table() string { return "users" }

// Post implements the entity.Entity interface.
type Post struct {
	ID        int64     `xql:"pk"`
	AuthorID  int64     `xql:"index;fk:users.id"`
	Title     string    `xql:"type:varchar(200);not null"`
	Content   string    // Defaults to TEXT type
	PublishedAt time.Time `xql:"not null;default:CURRENT_TIMESTAMP"`
}

func (Post) Table() string { return "posts" }
```

---

## Struct Tag Format

The basic format is a list of semicolon-separated directives within the `xql` tag.

`xql:"[directive1]; [directive2]; ..."`

---

## Directives

| Directive                       | Description                                                                                             |
|---------------------------------|---------------------------------------------------------------------------------------------------------|
| `name:<column_name>`            | Overrides the default column name (which is `snake_case` of the field name).                            |
| `type:<sql_type>`               | Overrides the default SQL type. Must include size/precision, e.g., `varchar(100)`.                      |
| `pk`                            | Marks the field as a primary key.                                                                       |
| `not null`                      | Adds a `NOT NULL` constraint.                                                                           |
| `unique`                        | Adds a `UNIQUE` constraint.                                                                             |
| `index`                         | Creates a non-unique index on the column.                                                               |
| `default:<value>`               | Sets a `DEFAULT` value for the column. For string literals, the value must be single-quoted.            |
| `fk:<reftable>.<refcolumn>`     | Creates a foreign key constraint referencing `refcolumn` in `reftable`.                                 |
| `-`                             | Instructs the generator to completely ignore this field.                                                |

---

## Naming and Field Handling

**Default Naming:**
If the `name` directive is not provided, the Go field name is converted from `CamelCase` to `snake_case` (e.g., `CreatedAt` becomes `created_at`).

**Ignoring Fields:**
To prevent a field from being mapped to a database column, use the ignore directive: `xql:"-"`.

---

## Keys and Relationships

**Primary Keys:**
For a numeric, auto-incrementing primary key, the recommended best practice is to use an `int64` in your Go struct with the `pk` flag.

- **Go Type:** `int64`
- **Tag:** `xql:"pk"`

This will generate the appropriate auto-incrementing primary key column in each database:

| Database     | Generated SQL Type                  |
|--------------|---------------------------------------|
| PostgreSQL   | `BIGSERIAL PRIMARY KEY`               |
| MySQL        | `BIGINT PRIMARY KEY AUTO_INCREMENT`   |
| SQLite       | `INTEGER PRIMARY KEY`                 |

**Foreign Keys:**
Use the `fk` directive to define a foreign key relationship. The value should be in the format `referenced_table.referenced_column`. It is good practice to also add an `index` on foreign key columns for performance.

- **Tag:** `xql:"index;fk:users.id"`

---

## Go Type to Database Type Mapping

The `xql` tool infers a default mapping from the Go field's type. You can override this with the `type` directive.

| Go Type     | Default PostgreSQL Type    | Default MySQL Type | Default SQLite Type | Notes                                                              |
|-------------|----------------------------|--------------------|---------------------|--------------------------------------------------------------------|
| `int64`     | `BIGINT`                   | `BIGINT`           | `INTEGER`           |                                                                    |
| `int`       | `BIGINT`                   | `BIGINT`           | `INTEGER`           | Assumes a 64-bit architecture.                                     |
| `int32`     | `INTEGER`                  | `INT`              | `INTEGER`           |                                                                    |
| `int16`     | `SMALLINT`                 | `SMALLINT`         | `INTEGER`           |                                                                    |
| `int8`      | `SMALLINT`                 | `TINYINT`          | `INTEGER`           | PostgreSQL lacks a `TINYINT`.                                      |
| `bool`      | `BOOLEAN`                  | `TINYINT(1)`       | `INTEGER`           | SQLite uses `0` for false and `1` for true.                        |
| `string`    | `TEXT`                     | `TEXT`             | `TEXT`              | **Safe default.** To enforce a length, use `xql:"type:varchar(N)"`. |
| `float32`   | `REAL`                     | `FLOAT`            | `REAL`              | For fixed-point numbers, use `xql:"type:decimal(P,S)"`.            |
| `float64`   | `DOUBLE PRECISION`         | `DOUBLE`           | `REAL`              | For fixed-point numbers, use `xql:"type:decimal(P,S)"`.            |
| `time.Time` | `TIMESTAMP WITH TIME ZONE` | `DATETIME`         | `TEXT`              | SQLite stores as an ISO-8601 string.                               |
| `[]byte`    | `BYTEA`                    | `BLOB`             | `BLOB`              |                                                                    |

---

## Column & Field Ordering

The generator applies a consistent ordering policy for both generated Go fields and database columns to ensure predictability. The order is determined as follows:

1.  **Primary Key Fields**: Any field marked with `xql:"pk"` is always placed first. If there are multiple primary keys, their relative order is preserved.
2.  **Host Struct Fields**: Non-primary key fields from the main struct are placed next, in the order they are defined in the struct.
3.  **Embedded Struct Fields**: Fields from embedded structs are placed last. The fields from each embedded struct are grouped together, and their original relative order is maintained.

### Example

Consider the following entity definition:

```go
package model

import "time"

// BaseModel contains common columns.
type BaseModel struct {
    ID        int64     `xql:"pk"`
    CreatedAt time.Time
    UpdatedAt time.Time
}

// User demonstrates field ordering.
type User struct {
    Email     string `xql:"type:varchar(255);unique"`
    BaseModel // Embed the common columns
    Nickname  string
}
```

The generated fields and columns for the `User` entity will be in this order:

1.  `ID` (from `BaseModel`, moved to the front because it's a primary key)
2.  `Email` (from `User` struct)
3.  `Nickname` (from `User` struct)
4.  `CreatedAt` (from `BaseModel`, embedded)
5.  `UpdatedAt` (from `BaseModel`, embedded)


## Reuse common database columns

To promote consistency and reduce duplication, you can define common columns in a separate struct and embed it in your entity models. This is particularly useful for fields like `ID`, `CreatedAt`, and `UpdatedAt` that appear in most tables.

When the generator encounters an embedded struct, it treats its fields as if they were part of the parent struct.

### Example

First, define a struct with the common columns. Note that this struct does not need to implement `entity.Entity` itself.

```go
package model

import "time"

// BaseModel contains common columns for all tables.
type BaseModel struct {
    ID        int64     `xql:"pk"`
    CreatedAt time.Time
    UpdatedAt time.Time
}
```

Next, embed `BaseModel` in your entity structs.

```go
package model

// User is a product in our application.
type User struct {
    BaseModel // Embed the common columns
    Email     string `xql:"type:varchar(255);unique;not null"`
    Nickname  string
}

func (User) Table() string { return "users" }
```

When `xql schema` is run, the generated `users` table will include `id`, `created_at`, `updated_at`, `email`, and `nickname` columns. The primary key and default naming rules apply to the embedded fields as well.


## TODO
### Customized data types
- We should update `FieldType` in `constraint/constraint.go` to support a wider range of types. The current definition is too restrictive and does not handle 
 custom string-based enums, slices (`[]string`, `[]byte`), or maps. By using `~string`, we can support any type with an underlying string type, and by adding slice and map types, we can correctly generate fields for more complex entities.
