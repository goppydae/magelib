// Copyright (c) 2026 Steven Verhelle (enqack)
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.
//
// SPDX-License-Identifier: MPL-2.0

package magelib

import (
	"strings"
	"testing"
)

// The lint gate shells out to gofmt, golangci-lint and gosec, so the two
// decisions it makes before any tool runs -- how the gosec carve-outs are
// spelled, and which shared config is resolved -- are the only parts a
// unit test can reach. Both are policy: getting either wrong silently
// changes what is enforced across every consumer repo.

func equalArgs(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func TestGosecArgsWithNoExcludes(t *testing.T) {
	want := []string{"-exclude-generated", "./..."}
	got := gosecArgs(nil)
	if !equalArgs(got, want) {
		t.Fatalf("got args = %v, want %v", got, want)
	}
}

func TestGosecArgsWithOneExclude(t *testing.T) {
	want := []string{"-exclude-generated", "-exclude=G204", "./..."}
	got := gosecArgs([]string{"G204"})
	if !equalArgs(got, want) {
		t.Fatalf("got args = %v, want %v", got, want)
	}
}

// TestGosecArgsJoinsMultipleExcludesIntoOneFlag is the one that matters.
// One -exclude flag per rule is a different command line, and gosec keeps
// only the last -exclude it is given, so the shape would silently drop
// every carve-out but one -- re-arming rules the operator disarmed with a
// documented justification at the Magefile call site.
func TestGosecArgsJoinsMultipleExcludesIntoOneFlag(t *testing.T) {
	want := []string{"-exclude-generated", "-exclude=G204,G304", "./..."}
	got := gosecArgs([]string{"G204", "G304"})
	if !equalArgs(got, want) {
		t.Fatalf("got args = %v, want %v", got, want)
	}
}

// TestGosecArgsKeepsExcludeGeneratedWhenExcludesArePresent: generated
// code is skipped separately from the rule carve-out. Folding the two
// together -- dropping -exclude-generated once -exclude is set -- would
// force repos to mute a rule in hand-written code just to silence
// protoc-gen-go output.
func TestGosecArgsKeepsExcludeGeneratedWhenExcludesArePresent(t *testing.T) {
	got := gosecArgs([]string{"G204", "G304"})
	if len(got) == 0 || got[0] != "-exclude-generated" {
		t.Fatalf("got args = %v, want -exclude-generated first", got)
	}
}

// TestGosecArgsAlwaysTargetsTheWholeModuleLast pins the target position:
// gosec reads its package pattern as a trailing operand, so a flag
// appended after it would be scanned as another path.
func TestGosecArgsAlwaysTargetsTheWholeModuleLast(t *testing.T) {
	for _, excludes := range [][]string{nil, {"G204"}, {"G204", "G304"}} {
		got := gosecArgs(excludes)
		if last := got[len(got)-1]; last != "./..." {
			t.Fatalf("got last arg = %v for excludes %v, want ./...", last, excludes)
		}
	}
}

// The SharedLintConfig tests use writeTree, which chdirs into a fresh
// temp dir and restores the working directory on cleanup. Keys starting
// with "../magelib/" land in a sibling of that dir, which is the sibling
// checkout layout the resolver models. The temp dir itself must not be
// named magelib, or the two candidates would name the same file and the
// precedence below would be untestable.

func TestSharedLintConfigPrefersTheLocalCopy(t *testing.T) {
	writeTree(t, map[string]string{
		".golangci.yml": "linters:\n",
	})
	got, err := SharedLintConfig()
	if err != nil {
		t.Fatalf("got err = %v, want nil", err)
	}
	if got != ".golangci.yml" {
		t.Fatalf("got config = %v, want .golangci.yml", got)
	}
}

func TestSharedLintConfigFallsBackToTheSibling(t *testing.T) {
	writeTree(t, map[string]string{
		"../magelib/.golangci.yml": "linters:\n",
	})
	got, err := SharedLintConfig()
	if err != nil {
		t.Fatalf("got err = %v, want nil", err)
	}
	if got != "../magelib/.golangci.yml" {
		t.Fatalf("got config = %v, want ../magelib/.golangci.yml", got)
	}
}

// TestSharedLintConfigLocalCopyWinsOverTheSibling is the precedence that
// matters. A repo's own checked-in config is the one its gate is pinned
// to; resolving the sibling first would lint it against magelib's rules
// instead. A test that only ever sets up one candidate cannot tell the
// two orderings apart.
func TestSharedLintConfigLocalCopyWinsOverTheSibling(t *testing.T) {
	writeTree(t, map[string]string{
		".golangci.yml":            "linters:\n",
		"../magelib/.golangci.yml": "linters:\n",
	})
	got, err := SharedLintConfig()
	if err != nil {
		t.Fatalf("got err = %v, want nil", err)
	}
	if got != ".golangci.yml" {
		t.Fatalf("got config = %v, want .golangci.yml", got)
	}
}

// TestSharedLintConfigErrorNamesBothLocations: a resolution failure that
// does not say where it looked sends the reader to the wrong repo. The
// fix is either "add the file here" or "check out the sibling", and the
// message has to distinguish them.
func TestSharedLintConfigErrorNamesBothLocations(t *testing.T) {
	writeTree(t, map[string]string{"README.md": "no config here\n"})
	got, err := SharedLintConfig()
	if err == nil {
		t.Fatalf("got config = %v, want an error", got)
	}
	for _, want := range []string{"looked in .", "../magelib"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error does not name %v: %v", want, err)
		}
	}
}
