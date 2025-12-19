# SQLX Package Design - Missing Features

This document outlines the key features and functionalities that are currently missing from the `sqlx` package, which are essential for a complete and production-ready data access library.

## 1. Core Implementation (SQL Generation and Execution)

*   **Current State**: All CRUD and Join functions (`Query`, `Insert`, `Update`, `Delete`, `JoinQuery`, `JoinUpdate`, `JoinDelete`) currently contain `panic("implement me")`.
*   **Missing**: The actual logic for generating the SQL queries based on the provided `Schema`, `Where` clauses, `Joint` definitions, and `ValueObject`/`Assignment` data, and then executing these queries against a database connection.

## 2. Transaction Management

*   **Current State**: There is no explicit API for managing database transactions.
*   **Missing**:
    *   Functions or methods to `BeginTx` (start a new transaction).
    *   Mechanisms to execute multiple database operations within the context of a single transaction.
    *   Functions or methods to `Commit` or `Rollback` a transaction.
    *   Integration with `context.Context` for transaction propagation.

## 3. Advanced Query Features (Aggregation and Grouping)

*   **Current State**: The `Query` and `JoinQuery` functions are designed to fetch full entity rows.
*   **Missing**:
    *   Support for SQL aggregate functions (e.g., `COUNT()`, `SUM()`, `AVG()`, `MAX()`, `MIN()`).
    *   Ability to specify `GROUP BY` clauses.
    *   Ability to specify `HAVING` clauses for filtering grouped results.

## 4. Database Dialect Support

*   **Current State**: The current design implicitly assumes a generic SQL syntax (e.g., `?` for placeholders).
*   **Missing**:
    *   A mechanism to abstract or configure SQL dialect differences (e.g., placeholder syntax like `?` for MySQL/SQLite vs. `$1`, `$2` for PostgreSQL).
    *   Potential support for database-specific functions or features.

## 5. Connection Management

*   **Current State**: The `ds.go` file provides basic data source configuration, but the `sqlx` functions do not yet integrate with it to obtain `*sql.DB` or `*sql.Tx` instances.
*   **Missing**:
    *   A clear strategy for how `sqlx` functions will acquire and release database connections or transaction handles.
    *   Integration with the `ds.go` configuration to manage multiple data sources if needed.

These missing components represent the next steps in evolving `sqlx` into a fully functional and robust data access layer.
