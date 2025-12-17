package internal

import (
	"runtime/debug"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
)

func TestDependsOn(t *testing.T) {
	// Ensure the package-initialized Current is available. If not, skip the test.
	if Current == nil || Current.Mod == nil {
		t.Skip("internal.Current not initialized or go.mod not found; skipping integration-style test")
	}

	// Check the module path itself (should always be present)
	if Current.Mod.Module != nil && Current.Mod.Module.Mod.Path != "" {
		if Current.DependsOn(Current.Mod.Module.Mod.Path).IsAbsent() {
			t.Fatalf("DependsOn should return true for the module path %s", Current.Mod.Module.Mod.Path)
		}
	}

	// Check a couple of known dependencies from the repository's go.mod
	if Current.DependsOn("github.com/fatih/color").IsAbsent() {
		t.Fatalf("DependsOn should return true for github.com/fatih/color")
	}
	if Current.DependsOn("golang.org/x/mod").IsAbsent() {
		t.Fatalf("DependsOn should return true for golang.org/x/mod")
	}

	// Negative case
	if Current.DependsOn("does.not.exist").IsPresent() {
		t.Fatalf("DependsOn should return false for unknown module")
	}
}

func TestToolModulePath(t *testing.T) {
	got := ToolModulePath()
	if got == "" {
		t.Fatalf("ToolModulePath returned empty string")
	}

	if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Path != "" {
		if got != bi.Main.Path {
			t.Fatalf("ToolModulePath = %q, want %q (from build info)", got, bi.Main.Path)
		}
	} else {
		// Fallback expectation when build info is unavailable.
		if got != "github.com/kcmvp/dvo" {
			t.Fatalf("ToolModulePath = %q, want %q (fallback)", got, "github.com/kcmvp/dvo")
		}
	}
}

func TestStructsImplementEntity(t *testing.T) {
	assert.NotNil(t, Current, "Current should be initialized")
	assert.NotEmpty(t, Current.Pkgs, "Current packages should be loaded")

	// Expected implementers from the sample package we added.
	expected := []string{"Account", "Order"}

	implementers := Current.StructsImplementEntity()
	implementerNames := lo.Map(implementers, func(info EntityInfo, _ int) string {
		return info.TypeSpec.Name.Name
	})

	for _, want := range expected {
		assert.Containsf(t, implementerNames, want, "StructsImplementEntity should contain %s; got %v", want, implementerNames)
	}
}
