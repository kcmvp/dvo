app package — configuration and datasource design

Purpose

- The `app` package centralizes application configuration discovery and loading.
- Packages such as `sqlx` call `app.Config()` to obtain configuration values (for example, datasource definitions) and initialize runtime resources.

Goals

- Provide a stable, predictable way to load YAML-based configuration files during both development and runtime.
- Make test-time behavior easy to control (use `application_test.yml` during `go test`).
- Keep discovery robust across IDE runs, `go test` from package folders, and running a built binary.

Files

- `app.go` — the implementation of the config loader (public API: `Config()`).
- `testdb.go` — optional convenience helper (for tests) to initialize a temporary SQLite DB for tests (`InitTestSQLiteDB()`).
- `app.md` — this document.

Design overview

1) Config discovery

- The `app` package exposes a single entrypoint for configuration: `Config() mo.Result[*viper.Viper]`.
- `Config()` is lazy: the first call runs the loader once (via `sync.Once`) and caches the `*viper.Viper` instance.
- Search paths (ordered):
  1. Project root (nearest parent directory containing `go.mod`).
     - Also add `${projectRoot}/config`.
  2. Current working directory (CWD).
     - Also add `${CWD}/config`.
  3. If CWD cannot be determined, fallbacks `.` and `./config` are used.

- Config file names used:
  - `application.yml` (default runtime config)
  - `application_test.yml` (used while running `go test`)

- The loader will prefer `application_test.yml` when it detects a test process (see below).

2) Test detection — `isTestProcess()`

- The loader uses a best-effort heuristic to detect `go test` runs:
  - It first inspects `os.Args` for flags starting with `-test.` (e.g., `-test.v`, `-test.run`). This works for the executed test binary.
  - As a fallback it scans stack frames for a filename ending with `_test.go`.
- This heuristic is intentionally conservative: it gives `go test` precedence when detected, but it is safe if it returns false in edge cases.

Notes about reliability

- The `-test.` flag approach is reliable for the test binary invocation.
- Stack-based detection is best-effort (useful during library init from test code). If code needs 100% deterministic behavior during init, prefer explicit environment control.

3) Configuration format (datasource example)

- `sqlx/db.go` expects a top-level `datasource` mapping in the config. Each key under `datasource` becomes a named datasource.
- Example `application_test.yml` snippet:

```yaml
datasource:
  DefaultDS:
    driver: sqlite3
    url: file::memory:?cache=shared
  reporting:
    driver: postgres
    url: "postgres://${user}:${password}@${host}/reportdb?sslmode=disable"
    user: reporting
    password: s3cr3t
    host: localhost:5432
```

- Each datasource maps to the `sqlx.dataSource` structure fields:
  - `driver` (required) — the Go SQL driver name (e.g., `sqlite3`, `mysql`, `postgres`).
  - `url` (required) — driver-specific DSN/URI. If it contains placeholders `${user}`, `${password}`, or `${host}`, the corresponding config fields must not be empty.
  - `user`, `password`, `host` — used for simple string substitution into `url` if placeholders are present.
  - `scripts` — optional array of SQL script filenames to run when the datasource is registered (not mandatory; `sqlx` may extend support for this).

4) How configuration and initialization interact

- The `app` package is only responsible for discovering and exposing configuration via `Config()`.
- Actual datasource initialization is performed by `sqlx/db.go` which calls `app.Config()` inside its `initDataSources()` function.
  - `sqlx/db.go` unmarshals the `datasource` map and calls its own `registerDataSource()` to open connections, validate via `Ping()`, and register DBs in the package registry.
- This separation keeps configuration discovery (app) and runtime resource lifecycle (sqlx) decoupled.

5) Testing helpers and recommended test pattern

- For unit / integration tests, there are two options:
  1) Provide an `application_test.yml` in the project (repo root or `./config`) pointing to a test DB. Then `initDataSources()` in `sqlx` will load and register datasources automatically via `DefaultDS()`/`GetDS()`.
  2) Use the convenience helper `app.InitTestSQLiteDB()` (in `app/testdb.go`) to create a temporary SQLite DB, apply SQL schema files and load fixtures. This returns `(*sql.DB, cleanup func(), error)`.
     - If using this helper, register the returned `*sql.DB` into `sqlx`'s registry (wrap into `stdDB{DB: raw}`) under the `DefaultDS` key so the library picks it up.
     - Call the returned cleanup function (which closes the DB and removes the temp file) when the test finishes.

- Note: `app.InitTestSQLiteDB()` is a convenience helper for the test suite and is not part of runtime initialization. The canonical datasource initialization path for applications is via configuration + `sqlx/initDataSources()`.

6) Lifecycle and concurrency

- `app.Config()` is safe for concurrent use and will only initialize viper once.
- `sqlx` uses `sync.Once` and mutexes to guard datasource initialization and access; `GetDS()`, `DefaultDS()` and registration are concurrency-safe.

7) Logging and observability

- `sqlx.SetSQLLogger(logger)` enables SQL logging for datasources registered after the call.
- `WithSQLLogger` is a lightweight wrapper that logs exec/query/ping/close durations.

8) Troubleshooting

- If `DefaultDS()` returns `(nil, false)`:
  - Check `application_test.yml` or `application.yml` locations (project root or `./config`).
  - Verify `isTestProcess()` detection if you expect `application_test.yml` to be used.
- If a datasource fails to open or ping:
  - `registerDataSource()` returns a descriptive error; ensure `url`, `driver`, and any placeholder values (`user`, `password`, `host`) are correct.
- If tests fail with missing tables or validation errors:
  - Ensure `app.InitTestSQLiteDB()` (or your `application_test.yml` DB) has the expected schema applied and fixtures loaded.

9) Extension points (TODOs)

- Add optional support for running named `scripts` listed in datasource config automatically after registering the datasource.
- Provide a documented mechanism to register existing `*sql.DB` programmatically (helper wrapper) so consumers can wire non-file-based DBs without using config files.
- Add richer examples showing `application.yml` and `application_test.yml` with multiple datasources.

References

- See `app/app.go` for config loader details.
- See `sqlx/db.go` for datasource lifecycle and registry semantics.
- See `app/testdb.go` for the test DB helper used by the test suite.

Contact

For changes to this design or more examples, update this document and corresponding tests in `sqlx`/`cmd` packages.
