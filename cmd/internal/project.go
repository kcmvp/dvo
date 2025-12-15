package internal

import (
	"os"
	"path/filepath"

	"github.com/fatih/color"
	"golang.org/x/mod/modfile"
)

var (
	// Home holds the path to the user's home directory.
	Home string
	// Project is the global project context, initialized at startup.
	Project *project
)

// project holds key information about the Go project being analyzed.
type project struct {
	Root    string
	Modules []string
	Mod     *modfile.File
}

// init automatically runs at program startup to initialize the project context.
// It enforces that the command is run from the go.mod root. Updated to search
// parent directories so the package is usable when tests are executed from
// subdirectories.
func init() {
	var err error
	Home, err = os.UserHomeDir()
	if err != nil {
		color.Red("Error: could not find user home directory: %v\n", err)
		os.Exit(1)
	}

	wd, err := os.Getwd()
	if err != nil {
		color.Red("Error: could not get working directory: %v\n", err)
		os.Exit(1)
	}

	// Walk up parent directories to find go.mod. This allows running from
	// subpackages (like during `go test ./cmd/internal`) without failing.
	modPath := ""
	cur := wd
	for {
		candidate := filepath.Join(cur, "go.mod")
		if _, err := os.Stat(candidate); err == nil {
			modPath = candidate
			wd = cur
			break
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			// reached filesystem root without finding go.mod
			break
		}
		cur = parent
	}

	if modPath == "" {
		color.Red("Error: 'gob' must be run from the module root (directory containing go.mod).\n")
		os.Exit(1)
	}

	modBytes, err := os.ReadFile(modPath)
	if err != nil {
		color.Red("Error: could not read go.mod file: %v\n", err)
		os.Exit(1)
	}

	modFile, err := modfile.Parse(modPath, modBytes, nil)
	if err != nil {
		color.Red("Error: could not parse go.mod file: %v\n", err)
		os.Exit(1)
	}

	Project = &project{
		Root:    wd,
		Modules: []string{modFile.Module.Mod.Path},
		Mod:     modFile,
	}
}

func (p *project) DependsOn(deps ...string) bool {
	if p == nil || p.Mod == nil || len(deps) == 0 {
		return false
	}

	targets := make(map[string]struct{}, len(deps))
	for _, d := range deps {
		targets[d] = struct{}{}
	}

	// Check the module path itself
	if p.Mod.Module != nil && p.Mod.Module.Mod.Path != "" {
		if _, ok := targets[p.Mod.Module.Mod.Path]; ok {
			return true
		}
	}

	// Check required modules
	for _, req := range p.Mod.Require {
		if _, ok := targets[req.Mod.Path]; ok {
			return true
		}
	}

	// Check replace directives (both old and new paths)
	for _, rep := range p.Mod.Replace {
		if rep.Old.Path != "" {
			if _, ok := targets[rep.Old.Path]; ok {
				return true
			}
		}
		if rep.New.Path != "" {
			if _, ok := targets[rep.New.Path]; ok {
				return true
			}
		}
	}

	return false
}
