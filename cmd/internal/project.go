package internal

import (
	"flag"
	"fmt"
	"go/ast"
	"go/types"
	"os"
	"path/filepath"
	"runtime/debug"

	"github.com/fatih/color"
	"github.com/samber/mo"
	"golang.org/x/mod/modfile"
	"golang.org/x/tools/go/packages"
)

// Project holds key information about the Go project being analyzed.
type Project struct {
	Root    string
	Modules []string
	Mod     *modfile.File
	Pkgs    []*packages.Package
}

var (
	// Current is the global project context, initialized at startup.
	Current *Project
)

// init automatically runs at program startup to initialize the project context.
func init() {
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
		// Don't exit if we can't find go.mod, just don't initialize the project.
		// This allows tests to run without a go.mod.
		return
	}

	Current, _ = NewProject(wd)
}

func NewProject(wd string) (*Project, error) {
	modPath := filepath.Join(wd, "go.mod")
	if _, err := os.Stat(modPath); err != nil {
		return nil, fmt.Errorf("go.mod not found in %s", wd)
	}

	modBytes, err := os.ReadFile(modPath)
	if err != nil {
		return nil, fmt.Errorf("could not read go.mod file: %w", err)
	}

	modFile, err := modfile.Parse(modPath, modBytes, nil)
	if err != nil {
		return nil, fmt.Errorf("could not parse go.mod file: %w", err)
	}

	cfg := &packages.Config{
		Mode:  packages.NeedName | packages.NeedFiles | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo | packages.NeedImports,
		Dir:   wd,
		Tests: false,
	}
	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		return nil, fmt.Errorf("could not load project packages: %w", err)
	}

	return &Project{
		Root:    wd,
		Modules: []string{modFile.Module.Mod.Path},
		Mod:     modFile,
		Pkgs:    pkgs,
	}, nil
}

func (p *Project) DependsOn(deps ...string) mo.Option[[]string] {
	// Return None for invalid inputs
	if p == nil || p.Mod == nil || len(deps) == 0 {
		return mo.None[[]string]()
	}

	// Build a set of available module paths from the project
	available := make(map[string]struct{})
	if p.Mod.Module != nil && p.Mod.Module.Mod.Path != "" {
		available[p.Mod.Module.Mod.Path] = struct{}{}
	}
	for _, req := range p.Mod.Require {
		available[req.Mod.Path] = struct{}{}
	}
	for _, rep := range p.Mod.Replace {
		if rep.Old.Path != "" {
			available[rep.Old.Path] = struct{}{}
		}
		if rep.New.Path != "" {
			available[rep.New.Path] = struct{}{}
		}
	}

	// Collect the deps from the input list that are present in available
	var matched []string
	for _, d := range deps {
		if _, ok := available[d]; ok {
			matched = append(matched, d)
		}
	}

	if len(matched) == 0 {
		return mo.None[[]string]()
	}
	return mo.Some(matched)
}

// ToolModulePath returns the current tool's module path inferred at runtime.
// Falls back to "github.com/kcmvp/dvo" if build info is unavailable.
func ToolModulePath() string {
	if bi, ok := debug.ReadBuildInfo(); ok {
		if bi.Main.Path != "" {
			return bi.Main.Path
		}
	}
	return "github.com/kcmvp/dvo"
}

// DependsOnTool reports whether the project depends on this tool's module path.
func (p *Project) DependsOnTool() bool {
	opt := p.DependsOn(ToolModulePath())
	return opt.IsPresent()
}

// ToolEntityInterface returns the Go import path for the Entity interface
// defined in this tool. The value is used to identify all structs that implement
// the Entity interface at runtime or via static analysis.
// Example: "github.com/kcmvp/dvo/entity".
func ToolEntityInterface() string {
	return ToolModulePath() + "/entity"
}

// EntityInfo holds the type spec and package path for a discovered entity.
type EntityInfo struct {
	TypeSpec *ast.TypeSpec
	PkgPath  string
}

// StructsImplementEntity finds all structs in the project that implement the
// entity.Entity interface. It works by finding the defining package of the
// interface and then scanning project packages for implementations.
func (p *Project) StructsImplementEntity() []EntityInfo {
	entityInterfacePath := ToolEntityInterface()
	var entityInterface *types.Interface

	// 1. Find the entity.Entity interface definition within the loaded packages.
	for _, pkg := range p.Pkgs {
		if pkg.PkgPath == entityInterfacePath {
			if obj := pkg.Types.Scope().Lookup("Entity"); obj != nil {
				if typ, ok := obj.Type().Underlying().(*types.Interface); ok {
					entityInterface = typ
					break
				}
			}
		}
	}

	if entityInterface == nil {
		fmt.Println("Warning: Could not find 'entity.Entity' interface definition.")
		return nil
	}

	var implementers []EntityInfo
	// 2. Scan all packages for types that implement the interface.
	for _, pkg := range p.Pkgs {
		for _, file := range pkg.Syntax {
			ast.Inspect(file, func(n ast.Node) bool {
				if ts, ok := n.(*ast.TypeSpec); ok {
					// Use types info to check if the type implements the interface
					obj := pkg.TypesInfo.Defs[ts.Name]
					if obj == nil {
						return true
					}
					if types.Implements(types.NewPointer(obj.Type()), entityInterface) || types.Implements(obj.Type(), entityInterface) {
						implementers = append(implementers, EntityInfo{TypeSpec: ts, PkgPath: pkg.PkgPath})
					}
				}
				return true
			})
		}
	}

	return implementers
}

// GenPath returns the root path for generated files. It returns
// `{project_root}/gen` by default. When running in a test, it returns
// `{project_root}/cmd/gob/gen` to avoid polluting the project root.
func (p *Project) GenPath() string {
	if flag.Lookup("test.v") != nil {
		return filepath.Join(p.Root, "cmd/gob/gen")
	}
	return filepath.Join(p.Root, "gen")
}
