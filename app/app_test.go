package app

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInit_LoadsApplicationTestYml(t *testing.T) {
	// Run from repo root so the search path '.' can find application_test.yml.
	cwd, err := os.Getwd()
	require.NoError(t, err)
	root := filepath.Dir(cwd) // cwd == .../dvo/app
	require.NoError(t, os.Chdir(root))
	t.Cleanup(func() { _ = os.Chdir(cwd) })

	// Reset singleton for this test.
	cfg = nil
	once = sync.Once{}

	res := Config()
	require.True(t, res.IsOk())
	v := res.MustGet()
	require.NotNil(t, v)

	// This value comes from application_test.yml
	require.Equal(t, "sqlite3", v.GetString("datasource.DefaultDS.driver"))
}
