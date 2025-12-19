package sqlx

import (
	"database/sql"
	"sync"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"
)

func TestDataSource_DSN_Substitution(t *testing.T) {
	ds := dataSource{
		User:     "u",
		Password: "p",
		Host:     "localhost:5432",
		URL:      "postgres://${user}:${password}@${host}/db?sslmode=disable",
	}
	require.Equal(t, "postgres://u:p@localhost:5432/db?sslmode=disable", ds.DSN())

	dsn, err := ds.DSNChecked()
	require.NoError(t, err)
	require.Equal(t, "postgres://u:p@localhost:5432/db?sslmode=disable", dsn)
}

func TestDataSource_DSN_NoPlaceholders(t *testing.T) {
	ds := dataSource{URL: "file::memory:?cache=shared"}
	require.Equal(t, "file::memory:?cache=shared", ds.DSN())

	dsn, err := ds.DSNChecked()
	require.NoError(t, err)
	require.Equal(t, "file::memory:?cache=shared", dsn)
}

func TestDataSource_DSNChecked_MissingUser(t *testing.T) {
	ds := dataSource{URL: "postgres://${user}@${host}/db", Host: "localhost"}
	_, err := ds.DSNChecked()
	require.Error(t, err)
}

func TestDataSource_DSNChecked_MissingPassword(t *testing.T) {
	ds := dataSource{URL: "postgres://${user}:${password}@${host}/db", User: "u", Host: "localhost"}
	_, err := ds.DSNChecked()
	require.Error(t, err)
}

func TestDataSource_DSNChecked_MissingHost(t *testing.T) {
	ds := dataSource{URL: "postgres://${user}@${host}/db", User: "u"}
	_, err := ds.DSNChecked()
	require.Error(t, err)
}

func TestDataSource_DSNChecked_RequiresURL(t *testing.T) {
	ds := dataSource{}
	_, err := ds.DSNChecked()
	require.Error(t, err)
}

func TestGetDS_DefaultDS_Close(t *testing.T) {
	// Reset init so this test is deterministic.
	initOnce = sync.Once{}
	initErr = nil

	dsMu.Lock()
	dsRegistry = map[string]DB{}
	defaultDS = nil
	dsMu.Unlock()

	// Use a real in-memory sqlite DB so Close() is safe.
	raw, err := sql.Open("sqlite3", "file::memory:?cache=shared")
	require.NoError(t, err)
	require.NoError(t, raw.Ping())
	wrapped := stdDB{DB: raw}

	dsMu.Lock()
	dsRegistry[defaultDSName] = wrapped
	dsRegistry["Other"] = wrapped
	defaultDS = wrapped
	dsMu.Unlock()

	got, ok := DefaultDS()
	require.True(t, ok)
	require.NotNil(t, got)

	got2, ok2 := GetDS("Other")
	require.True(t, ok2)
	require.NotNil(t, got2)

	require.NoError(t, CloseDataSource("Other"))
	_, ok3 := GetDS("Other")
	require.False(t, ok3)

	// DefaultDS is still present; close all should clear and close.
	require.NoError(t, CloseAllDataSources())
	_, ok4 := DefaultDS()
	require.False(t, ok4)
}
