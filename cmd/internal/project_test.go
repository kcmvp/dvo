package internal

import (
	"testing"

	"golang.org/x/mod/modfile"
	"golang.org/x/mod/module"
)

func TestDependsOn(t *testing.T) {
	old := Project
	defer func() { Project = old }()

	Project = &project{
		Root:    "/tmp",
		Modules: []string{"example.com/m"},
		Mod: &modfile.File{
			Module: &modfile.Module{
				Mod: module.Version{Path: "example.com/m"},
			},
			Require: []*modfile.Require{
				{Mod: module.Version{Path: "github.com/some/dependency"}},
			},
			Replace: []*modfile.Replace{
				{Old: module.Version{Path: "github.com/old/pkg"}, New: module.Version{Path: "github.com/new/pkg"}},
			},
		},
	}

	// Module path
	if !Project.DependsOn("example.com/m") {
		t.Fatalf("DependsOn should return true for the module path")
	}

	// Required dependency
	if !Project.DependsOn("github.com/some/dependency") {
		t.Fatalf("DependsOn should return true for a required module")
	}

	// Replace old path
	if !Project.DependsOn("github.com/old/pkg") {
		t.Fatalf("DependsOn should return true for replace old path")
	}

	// Replace new path
	if !Project.DependsOn("github.com/new/pkg") {
		t.Fatalf("DependsOn should return true for replace new path")
	}

	// Negative case
	if Project.DependsOn("does.not.exist") {
		t.Fatalf("DependsOn should return false for unknown module")
	}
}
