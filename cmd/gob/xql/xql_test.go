package xql

import (
	"testing"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"
)

func TestXqlCmd_EmbedDrivers(t *testing.T) {
	// Run the persistent pre-run to populate context
	err := XqlCmd.PersistentPreRunE(XqlCmd, []string{})
	require.NoError(t, err)

	v := XqlCmd.Context().Value(dbaAdapterKey)
	require.NotNil(t, v, "expected drivers value in command context")

	drivers, ok := v.([]string)
	require.True(t, ok, "expected context value to be []string")

	// Expect sqlite, mysql, and postgres to be detected
	require.Contains(t, drivers, "sqlite")
	require.Contains(t, drivers, "mysql")
	require.Contains(t, drivers, "postgres")
}
