package sqlx

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/kcmvp/dvo/app"
	"github.com/spf13/viper"
)

// DB is the minimal database contract used by this package.
// It mirrors the methods we use from *sql.DB and can be backed by *sql.DB or a thin wrapper.
//
// This indirection lets us add cross-cutting features (SQL logging, tracing, metrics) without
// changing the higher-level query builder APIs.
type DB interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	PingContext(ctx context.Context) error
	Close() error
}

// stdDB adapts *sql.DB to the DB interface.
type stdDB struct{ *sql.DB }

func (d stdDB) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return d.DB.ExecContext(ctx, query, args...)
}

func (d stdDB) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return d.DB.QueryContext(ctx, query, args...)
}

func (d stdDB) PingContext(ctx context.Context) error { return d.DB.PingContext(ctx) }

// loggingDB is a thin wrapper around DB that logs SQL statements.
// It is intentionally minimal and does not attempt to pretty-print SQL.
// Use it in dev/test or when you need observability.
type loggingDB struct {
	inner  DB
	logger *log.Logger
}

func (d loggingDB) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	start := time.Now()
	res, err := d.inner.ExecContext(ctx, query, args...)
	d.logger.Printf("sqlx exec dur=%s err=%v sql=%q args=%v", time.Since(start), err, query, args)
	return res, err
}

func (d loggingDB) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	start := time.Now()
	rows, err := d.inner.QueryContext(ctx, query, args...)
	d.logger.Printf("sqlx query dur=%s err=%v sql=%q args=%v", time.Since(start), err, query, args)
	return rows, err
}

func (d loggingDB) PingContext(ctx context.Context) error {
	start := time.Now()
	err := d.inner.PingContext(ctx)
	d.logger.Printf("sqlx ping dur=%s err=%v", time.Since(start), err)
	return err
}

func (d loggingDB) Close() error {
	start := time.Now()
	err := d.inner.Close()
	d.logger.Printf("sqlx close dur=%s err=%v", time.Since(start), err)
	return err
}

// WithSQLLogger wraps db with a SQL logger if logger is not nil.
func WithSQLLogger(db DB, logger *log.Logger) DB {
	if logger == nil {
		return db
	}
	return loggingDB{inner: db, logger: logger}
}

var (
	defaultDS DB
	// registry holds named datasource
	dsRegistry = map[string]DB{}
	dsMu       sync.RWMutex

	initOnce sync.Once
	initErr  error

	// sqlLogger, when set, enables SQL logging for all registered datasources.
	sqlLogger *log.Logger
)

// SetSQLLogger enables SQL logging for all datasources registered after this call.
// Call this early (e.g., in main) before any DefaultDS/GetDS calls.
func SetSQLLogger(l *log.Logger) {
	sqlLogger = l
}

const (
	UserKey     = "${user}"
	PasswordKey = "${password}"
	HostKey     = "${host}"
	defaultDs   = "default"
)

type dataSource struct {
	DB       string   `mapstructure:"db" yaml:"db"`
	Driver   string   `mapstructure:"driver" yaml:"driver"`
	User     string   `mapstructure:"user" yaml:"user"`
	Password string   `mapstructure:"password" yaml:"password"`
	Host     string   `mapstructure:"host" yaml:"host"`
	URL      string   `mapstructure:"url" yaml:"url"`
	Scripts  []string `mapstructure:"scripts" yaml:"scripts"`
}

// DSNChecked returns the final connection string for sql.Open and validates placeholder usage.
//
// NOTE: Unlike Java JDBC, Go database drivers don't share a single standard DSN format.
// The `url` field is therefore required and should be a driver-specific DSN/URI (optionally
// containing placeholders).
//
// If ds.URL contains placeholders (${user}, ${password}, ${host}), the corresponding field must be
// non-empty, otherwise an error is returned. This is a fail-fast safety to prevent accidentally
// connecting with blank credentials/host due to misconfiguration.
func (ds dataSource) DSNChecked() (string, error) {
	if strings.TrimSpace(ds.URL) == "" {
		return "", fmt.Errorf("dsn requires url")
	}
	if strings.Contains(ds.URL, UserKey) && ds.User == "" {
		return "", fmt.Errorf("dsn requires user")
	}
	if strings.Contains(ds.URL, PasswordKey) && ds.Password == "" {
		return "", fmt.Errorf("dsn requires password")
	}
	if strings.Contains(ds.URL, HostKey) && ds.Host == "" {
		return "", fmt.Errorf("dsn requires host")
	}
	return ds.DSN(), nil
}

// DSN returns the final connection string for sql.Open.
//
// It performs *only* string substitution on ds.URL using these placeholders:
//   - ${user}
//   - ${password}
//   - ${host}
//
// If ds.URL contains no placeholders, it is returned as-is.
func (ds dataSource) DSN() string {
	dsn := strings.ReplaceAll(ds.URL, UserKey, ds.User)
	dsn = strings.ReplaceAll(dsn, PasswordKey, ds.Password)
	return strings.ReplaceAll(dsn, HostKey, ds.Host)
}

// registerDataSource opens a database connection from cfg and registers it under the provided name.
// If name is empty, default is used. The function will Ping the DB to validate the connection.
func registerDataSource(name string, cfg dataSource) error {
	if name == "" {
		name = defaultDs
	}
	if cfg.Driver == "" {
		return fmt.Errorf("driver is required to register datasource %q", name)
	}

	dsn, err := cfg.DSNChecked()
	if err != nil {
		return fmt.Errorf("invalid dsn for datasource %q: %w", name, err)
	}
	raw, err := sql.Open(cfg.Driver, dsn)
	if err != nil {
		return fmt.Errorf("open datasource %q: %w", name, err)
	}
	if err := raw.PingContext(context.Background()); err != nil {
		_ = raw.Close()
		return fmt.Errorf("ping datasource %q: %w", name, err)
	}

	var db DB = stdDB{DB: raw}
	if sqlLogger != nil {
		db = WithSQLLogger(db, sqlLogger)
	}

	dsMu.Lock()
	defer dsMu.Unlock()
	dsRegistry[name] = db
	if name == defaultDs && defaultDS == nil {
		defaultDS = db
	}
	return nil
}

func initDataSources() error {
	initOnce.Do(func() {
		res := app.Config()
		if res.IsError() {
			initErr = res.Error()
			return
		}
		cfg := res.MustGet()

		raw := cfg.GetStringMap("datasource")
		if len(raw) == 0 {
			return
		}

		for name, val := range raw {
			child := viper.New()
			if m, ok := val.(map[string]any); ok {
				if err := child.MergeConfigMap(m); err != nil {
					initErr = fmt.Errorf("merge datasource %s: %w", name, err)
					return
				}
			} else {
				child.Set("_", val)
			}

			var ds dataSource
			if err := child.Unmarshal(&ds); err != nil {
				initErr = fmt.Errorf("unmarshal datasource %s: %w", name, err)
				return
			}

			if err := registerDataSource(name, ds); err != nil {
				initErr = fmt.Errorf("register datasource %s: %w", name, err)
				return
			}
		}
	})
	return initErr
}

// GetDS returns a registered datasource by name.
func GetDS(name string) (DB, bool) {
	_ = initDataSources()
	if name == "" {
		name = defaultDs
	}
	dsMu.RLock()
	defer dsMu.RUnlock()
	db, ok := dsRegistry[name]
	return db, ok
}

// DefaultDS returns the default datasource if registered.
func DefaultDS() (DB, bool) {
	_ = initDataSources()
	dsMu.RLock()
	defer dsMu.RUnlock()
	if defaultDS == nil {
		db, ok := dsRegistry[defaultDs]
		return db, ok
	}
	return defaultDS, true
}

// CloseDataSource closes and removes the named datasource from the registry.
func CloseDataSource(name string) error {
	if name == "" {
		name = defaultDs
	}
	dsMu.Lock()
	defer dsMu.Unlock()
	if db, ok := dsRegistry[name]; ok {
		delete(dsRegistry, name)
		return db.Close()
	}
	return nil
}

// CloseAllDataSources closes and removes all registered datasources from the registry.
// It returns the first error encountered while closing any datasource, or nil on success.
func CloseAllDataSources() error {
	dsMu.Lock()
	defer dsMu.Unlock()
	var firstErr error
	for name, db := range dsRegistry {
		if err := db.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
		delete(dsRegistry, name)
	}
	defaultDS = nil
	return firstErr
}
