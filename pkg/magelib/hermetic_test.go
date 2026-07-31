package magelib

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// CheckHermetic's two halves are the tool set it decides to check and the
// per-tool store assertion. Both are reachable without a shell;
// CheckHermetic end to end is not.
//
// DETERMINISM BOUNDARY -- the same one doctor_test.go documents. Whether
// go, gcc, protoc and pandoc resolve under /nix/store, and whether the
// shell go matches the toolchain directive, depends on running inside
// 'nix develop'. So nothing here asserts CheckHermetic's result, nor
// checkStorePath's result on a real tool, nor that CheckToolchainPin
// succeeds. What is assertable is which names hermeticTools selects, and
// checkStorePath's two failure paths, both of which are constructed here
// rather than borrowed from the ambient shell.

func containsTool(tools []string, want string) bool {
	for _, tool := range tools {
		if tool == want {
			return true
		}
	}
	return false
}

func TestHermeticToolsIncludesEveryExtra(t *testing.T) {
	extra := []string{"buf", "golangci-lint", "gosec"}
	got := hermeticTools(extra)
	for _, want := range extra {
		if !containsTool(got, want) {
			t.Fatalf("got tools = %v, want it to include %v", got, want)
		}
	}
}

func TestHermeticToolsIncludesEachBaseTool(t *testing.T) {
	got := hermeticTools([]string{"buf"})
	for _, want := range baseTools {
		if !containsTool(got, want) {
			t.Fatalf("got tools = %v, want it to include base tool %v", got, want)
		}
	}
}

// TestHermeticToolsWithNoExtrasStillIncludesTheBaseTools covers the
// empty-variadic case, which is how gapi and goblin call CheckHermetic.
// An extras path that dropped the base set would leave those two repos
// checking nothing while still reporting success.
func TestHermeticToolsWithNoExtrasStillIncludesTheBaseTools(t *testing.T) {
	got := hermeticTools(nil)
	for _, want := range baseTools {
		if !containsTool(got, want) {
			t.Fatalf("got tools = %v, want it to include base tool %v", got, want)
		}
	}
}

// TestHermeticToolsKeepsExtrasInOrderAndDropsNone guards against an
// append that reuses the base slice's array: a caller passing several
// extras must get all of them, in the order given, not the last one or a
// truncated tail.
func TestHermeticToolsKeepsExtrasInOrderAndDropsNone(t *testing.T) {
	extra := []string{"buf", "golangci-lint", "gosec", "govulncheck", "mage", "goimports"}
	got := hermeticTools(extra)
	if len(got) < len(extra) {
		t.Fatalf("got tools = %v, want at least the %d extras", got, len(extra))
	}
	tail := got[len(got)-len(extra):]
	for i := range extra {
		if tail[i] != extra[i] {
			t.Fatalf("got trailing tools = %v, want %v", tail, extra)
		}
	}
}

func TestCheckStorePathErrorNamesAToolThatIsNotOnPath(t *testing.T) {
	tool := "magelib-hermetic-test-absent-tool"
	err := checkStorePath(tool)
	if err == nil {
		t.Fatalf("got err = %v, want non-nil for a tool absent from PATH", err)
	}
	if !strings.Contains(err.Error(), tool) {
		t.Fatalf("got err = %v, want it to name %v", err, tool)
	}
	if !strings.Contains(err.Error(), "nix develop") {
		t.Fatalf("got err = %v, want a remedy naming 'nix develop'", err)
	}
}

// TestCheckStorePathRejectsAToolResolvingOutsideTheStore is the assertion
// this file exists for: a host copy of a tool sitting ahead of the store
// one on PATH must fail. The shadow is planted as a real executable file,
// deliberately NOT a symlink into /nix/store -- checkStorePath calls
// filepath.EvalSymlinks, so a symlink would resolve back into the store
// and the test would pass while asserting nothing. A later reader
// simplifying this into a symlink would silently disarm it. Because the
// file is created here, the test gives the same result in or out of
// 'nix develop'.
func TestCheckStorePathRejectsAToolResolvingOutsideTheStore(t *testing.T) {
	dir := t.TempDir()
	tool := "magelib-hermetic-test-shadow"
	path := filepath.Join(dir, tool)
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	err := checkStorePath(tool)
	if err == nil {
		t.Fatalf("got err = %v, want non-nil for a tool resolving outside the store", err)
	}
	if !strings.Contains(err.Error(), "resolves outside the Nix store") {
		t.Fatalf("got err = %v, want it to say the tool resolves outside the Nix store", err)
	}
	// The message carries the post-EvalSymlinks path, and on hosts where
	// the temp root itself sits behind a symlink that is not the path
	// written above, so the expectation is resolved the same way.
	want := path
	if resolved, rerr := filepath.EvalSymlinks(path); rerr == nil {
		want = resolved
	}
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("got err = %v, want it to name the resolved path %v", err, want)
	}
}
