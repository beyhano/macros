package main

import (
	"os"
	"path/filepath"
	"testing"
)

// pickLibraryRoot test doubles: never touch the network, never touch disk
// beyond the temp dirs created by the test itself.

// TestPickLibraryRoot_ExistingCheckout asserts that a pre-existing
// ClassicAssist-Macros checkout wins over the legacy template and that its
// inner Macros/ folder is selected as the library root.
func TestPickLibraryRoot_ExistingCheckout(t *testing.T) {
	base := t.TempDir()
	repo := filepath.Join(base, macroRepoDir)
	mustMkdir(t, filepath.Join(repo, "Macros", "Advanced"))
	legacy := filepath.Join(base, "Macros")
	mustMkdir(t, filepath.Join(legacy, "Crafting"))

	var cloned bool
	got := pickLibraryRoot(
		[]string{base},
		func(p string) bool { return p == repo },
		func(p string) bool { cloned = true; return true },
		func(string) bool { return true },
	)
	want := filepath.Join(repo, "Macros")
	if got != want {
		t.Fatalf("resolveMacrosRoot() = %q, want %q", got, want)
	}
	if cloned {
		t.Fatal("checkout existed; clone must not run")
	}
}

// TestPickLibraryRoot_CloneFallbackToLegacy asserts that when no checkout
// exists and cloning fails, a legacy Macros/ dir is used.
func TestPickLibraryRoot_CloneFallbackToLegacy(t *testing.T) {
	base := t.TempDir()
	legacy := filepath.Join(base, "Macros")
	mustMkdir(t, filepath.Join(legacy, "PVM"))

	var cloned string
	got := pickLibraryRoot(
		[]string{base},
		func(string) bool { return false },
		func(p string) bool { cloned = p; return false },
		func(string) bool { return true },
	)
	if got != legacy {
		t.Fatalf("pickLibraryRoot() = %q, want %q", got, legacy)
	}
	if cloned != filepath.Join(base, macroRepoDir) {
		t.Fatalf("clone target = %q, want %q", cloned, filepath.Join(base, macroRepoDir))
	}
}

// TestPickLibraryRoot_CloneSuccess asserts a successful clone wins over a
// legacy template. The repo dir does not exist up front (clone creates it).
func TestPickLibraryRoot_CloneSuccess(t *testing.T) {
	base := t.TempDir()
	legacy := filepath.Join(base, "Macros")
	mustMkdir(t, filepath.Join(legacy, "Crafting"))

	got := pickLibraryRoot(
		[]string{base},
		func(string) bool { return false },
		func(p string) bool {
			// Simulate a successful clone producing the inner Macros/ library.
			return os.MkdirAll(filepath.Join(p, "Macros", "Mining"), 0755) == nil
		},
		func(string) bool { return true },
	)
	want := filepath.Join(base, macroRepoDir, "Macros")
	if got != want {
		t.Fatalf("pickLibraryRoot() = %q, want %q", got, want)
	}
}

// TestPickLibraryRoot_ExtractFallback asserts extraction is the last resort
// when nothing exists.
func TestPickLibraryRoot_ExtractFallback(t *testing.T) {
	base := t.TempDir()

	var extracted string
	got := pickLibraryRoot(
		[]string{base},
		func(string) bool { return false },
		func(string) bool { return false },
		func(p string) bool { extracted = p; return true },
	)
	want := filepath.Join(base, "Macros")
	if got != want {
		t.Fatalf("pickLibraryRoot() = %q, want %q", got, want)
	}
	if extracted != want {
		t.Fatalf("extract target = %q, want %q", extracted, want)
	}
}

func mustMkdir(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
}