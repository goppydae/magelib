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

// The comparison is tested directly because the exported entry point
// shells out to `nix develop`, which no unit test can do. The probe
// stays thin and untested on purpose; everything that decides pass or
// fail lives here.
func TestCompareInventories_AgreementIsSilent(t *testing.T) {
	inv := []shellInventory{
		{name: "gapi", paths: map[string]string{"go": "/nix/store/aaa/bin/go"}},
		{name: "goblin", paths: map[string]string{"go": "/nix/store/aaa/bin/go"}},
	}
	got, unresolved := compareInventories(inv, []string{"go"})
	if len(got) != 0 {
		t.Errorf("identical store paths must produce no findings, got %v", got)
	}
	if len(unresolved) != 0 {
		t.Errorf("resolved tools must not be reported unresolved, got %v", unresolved)
	}
}

func TestCompareInventories_DifferingPathsAreSkew(t *testing.T) {
	inv := []shellInventory{
		{name: "gapi", paths: map[string]string{"go": "/nix/store/aaa/bin/go"}},
		{name: "goblin", paths: map[string]string{"go": "/nix/store/bbb/bin/go"}},
	}
	got, _ := compareInventories(inv, []string{"go"})
	if len(got) != 1 {
		t.Fatalf("want exactly one finding, got %d: %v", len(got), got)
	}
	for _, want := range []string{"go", "gapi", "goblin", "aaa", "bbb"} {
		if !strings.Contains(got[0], want) {
			t.Errorf("finding %q does not name %q", got[0], want)
		}
	}
}

// The probe emits this literal when `command -v` finds nothing. It is a
// sentinel, not a path, and comparing two of them as equal is the defect
// this test exists for: a tool deleted from every flake would otherwise
// read as unified.
func TestCompareInventories_AllShellsMissingIsNotAgreement(t *testing.T) {
	inv := []shellInventory{
		{name: "gapi", paths: map[string]string{"protoc-gen-go": "MISSING"}},
		{name: "goblin", paths: map[string]string{"protoc-gen-go": "MISSING"}},
	}
	skew, unresolved := compareInventories(inv, []string{"protoc-gen-go"})
	if len(skew) != 0 {
		t.Errorf("two absences are not skew, got %v", skew)
	}
	if len(unresolved) != 1 {
		t.Fatalf("a tool absent from every shell must be reported, got %v", unresolved)
	}
	if !strings.Contains(unresolved[0], "protoc-gen-go") {
		t.Errorf("finding %q does not name the tool", unresolved[0])
	}
}

func TestCompareInventories_PartiallyMissingIsUnresolvedNotSkew(t *testing.T) {
	inv := []shellInventory{
		{name: "gapi", paths: map[string]string{"gosec": "/nix/store/aaa/bin/gosec"}},
		{name: "goblin", paths: map[string]string{"gosec": "MISSING"}},
	}
	skew, unresolved := compareInventories(inv, []string{"gosec"})
	if len(unresolved) != 1 {
		t.Fatalf("a tool missing from one shell is unresolved, got %v", unresolved)
	}
	if len(skew) != 0 {
		t.Errorf("an absence must not be reported as a store-path difference, got %v", skew)
	}
	if !strings.Contains(unresolved[0], "goblin") {
		t.Errorf("finding %q does not name the shell that lacks it", unresolved[0])
	}
}
