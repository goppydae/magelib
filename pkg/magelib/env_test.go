// Copyright (c) 2026 Steven Verhelle (enqack)
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.
//
// SPDX-License-Identifier: MPL-2.0

package magelib

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The comparison is tested directly because the exported entry point
// shells out to `nix develop`, which no unit test can do. The probe
// stays thin and untested on purpose; everything that decides pass or
// fail lives here.
func TestCompareInventories_AgreementIsSilent(t *testing.T) {
	inv := []inventory{
		{name: "gapi", values: map[string]string{"go": "/nix/store/aaa/bin/go"}},
		{name: "goblin", values: map[string]string{"go": "/nix/store/aaa/bin/go"}},
	}
	got, unresolved := compareInventories(inv, []string{"go"}, "not resolvable")
	if len(got) != 0 {
		t.Errorf("identical store paths must produce no findings, got %v", got)
	}
	if len(unresolved) != 0 {
		t.Errorf("resolved tools must not be reported unresolved, got %v", unresolved)
	}
}

func TestCompareInventories_DifferingPathsAreSkew(t *testing.T) {
	inv := []inventory{
		{name: "gapi", values: map[string]string{"go": "/nix/store/aaa/bin/go"}},
		{name: "goblin", values: map[string]string{"go": "/nix/store/bbb/bin/go"}},
	}
	got, _ := compareInventories(inv, []string{"go"}, "not resolvable")
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
	inv := []inventory{
		{name: "gapi", values: map[string]string{"protoc-gen-go": "MISSING"}},
		{name: "goblin", values: map[string]string{"protoc-gen-go": "MISSING"}},
	}
	skew, unresolved := compareInventories(inv, []string{"protoc-gen-go"}, "not resolvable")
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
	inv := []inventory{
		{name: "gapi", values: map[string]string{"gosec": "/nix/store/aaa/bin/gosec"}},
		{name: "goblin", values: map[string]string{"gosec": "MISSING"}},
	}
	skew, unresolved := compareInventories(inv, []string{"gosec"}, "not resolvable")
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

// The phrase is a parameter because a require directive is not
// "resolvable" - it is declared or it is not, and a message using the
// shell vocabulary sends the reader to the flake instead of the go.mod.
func TestCompareInventories_AbsentPhraseNamesTheSourceType(t *testing.T) {
	inv := []inventory{
		{name: "gapi", values: map[string]string{"example.com/theme": "v1.0.0"}},
		{name: "goblin", values: map[string]string{}},
	}
	_, unresolved := compareInventories(inv, []string{"example.com/theme"}, "not declared")
	if len(unresolved) != 1 {
		t.Fatalf("want one finding, got %v", unresolved)
	}
	if !strings.Contains(unresolved[0], "not declared") {
		t.Errorf("finding %q does not carry the caller's phrase", unresolved[0])
	}
}

// Map iteration order decided which source became the reference, so one
// skew rendered two ways across runs. Both renderings were true and
// neither was reproducible.
func TestSortedNames_IsStable(t *testing.T) {
	m := map[string]string{"magelib": "c", "gapi": "a", "goblin": "b", "docs": "d"}
	want := []string{"docs", "gapi", "goblin", "magelib"}
	for range 20 {
		got := sortedNames(m)
		if len(got) != len(want) {
			t.Fatalf("got %v, want %v", got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("got %v, want %v", got, want)
			}
		}
	}
}

// writeModule creates <root>/<name>/docs/go.mod requiring one module at
// the given version, and returns the repo directory.
func writeModule(t *testing.T, root, name, module, version string) string {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Join(dir, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	body := "module example.com/" + name + "/docs\n\ngo 1.26.2\n"
	if module != "" {
		body += "\nrequire " + module + " " + version + " // indirect\n"
	}
	if err := os.WriteFile(filepath.Join(dir, "docs", "go.mod"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

const themeModule = "github.com/McShelby/hugo-theme-relearn"

func TestCheckModulePins_AgreementPasses(t *testing.T) {
	root := t.TempDir()
	pins := ModulePins{
		Dirs: map[string]string{
			"gapi":   writeModule(t, root, "gapi", themeModule, "v0.0.0-20260310200521-93d7f257d1a3"),
			"goblin": writeModule(t, root, "goblin", themeModule, "v0.0.0-20260310200521-93d7f257d1a3"),
		},
		File:    "docs/go.mod",
		Modules: []string{themeModule},
	}
	if err := checkModulePins(pins); err != nil {
		t.Fatalf("identical pins must pass, got %v", err)
	}
}

// This is the defect the gate exists for: four repos pin the theme
// independently and nothing compares them.
func TestCheckModulePins_DivergentVersionsAreSkew(t *testing.T) {
	root := t.TempDir()
	pins := ModulePins{
		Dirs: map[string]string{
			"gapi":   writeModule(t, root, "gapi", themeModule, "v0.0.0-20260310200521-93d7f257d1a3"),
			"goblin": writeModule(t, root, "goblin", themeModule, "v0.0.0-20250101000000-000000000000"),
		},
		File:    "docs/go.mod",
		Modules: []string{themeModule},
	}
	err := checkModulePins(pins)
	if err == nil {
		t.Fatal("divergent pins must fail")
	}
	for _, want := range []string{themeModule, "gapi", "goblin", "93d7f257d1a3", "docs/go.mod"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not name %q", err, want)
		}
	}
}

// An absent require and an agreeing require must not be the same
// silence. A repo that simply dropped the theme would otherwise read as
// unified with one that still pins it.
func TestCheckModulePins_AbsentRequireIsReported(t *testing.T) {
	root := t.TempDir()
	pins := ModulePins{
		Dirs: map[string]string{
			"gapi":   writeModule(t, root, "gapi", themeModule, "v0.0.0-20260310200521-93d7f257d1a3"),
			"goblin": writeModule(t, root, "goblin", "", ""),
		},
		File:    "docs/go.mod",
		Modules: []string{themeModule},
	}
	err := checkModulePins(pins)
	if err == nil {
		t.Fatal("a module absent from one repo must fail")
	}
	if !strings.Contains(err.Error(), "not declared") {
		t.Errorf("error %q does not report the absence as undeclared", err)
	}
	if strings.Contains(err.Error(), "diverge") {
		t.Errorf("error %q reports an absence as a version difference", err)
	}
}

func TestCheckModulePins_AbsentFromEveryRepoIsNotAgreement(t *testing.T) {
	root := t.TempDir()
	pins := ModulePins{
		Dirs: map[string]string{
			"gapi":   writeModule(t, root, "gapi", "", ""),
			"goblin": writeModule(t, root, "goblin", "", ""),
		},
		File:    "docs/go.mod",
		Modules: []string{themeModule},
	}
	if err := checkModulePins(pins); err == nil {
		t.Fatal("a module absent from every repo must fail, not read as unified")
	}
}

// A missing file is an error rather than an absent pin: "the repo has no
// docs module yet" and "the repo dropped the theme" want different
// responses, and only one of them is this gate's business.
func TestCheckModulePins_MissingFileIsAnError(t *testing.T) {
	root := t.TempDir()
	pins := ModulePins{
		Dirs: map[string]string{
			"gapi":   writeModule(t, root, "gapi", themeModule, "v1.0.0"),
			"goblin": filepath.Join(root, "goblin-with-no-docs-dir"),
		},
		File:    "docs/go.mod",
		Modules: []string{themeModule},
	}
	if err := checkModulePins(pins); err == nil {
		t.Fatal("an unreadable go.mod must fail loudly")
	}
}

// A configuration that compares nothing must not run at all. This is
// compileSkips' rule applied to the same failure: a gate whose scope is
// empty is green and means nothing.
func TestModulePins_ValidateRejectsEmptyConfigurations(t *testing.T) {
	dirs := map[string]string{"gapi": "a", "goblin": "b"}
	cases := []struct {
		name string
		pins ModulePins
		want string
	}{
		{"no file", ModulePins{Dirs: dirs, Modules: []string{themeModule}}, "File is empty"},
		{"absolute file", ModulePins{Dirs: dirs, File: "/docs/go.mod", Modules: []string{themeModule}}, "repo-relative"},
		{"no modules", ModulePins{Dirs: dirs, File: "docs/go.mod"}, "names no modules"},
		{"one repo", ModulePins{Dirs: map[string]string{"gapi": "a"}, File: "docs/go.mod", Modules: []string{themeModule}}, "at least two"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.pins.validate()
			if err == nil {
				t.Fatalf("%s must be rejected", tc.name)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %q does not explain the rejection (want %q)", err, tc.want)
			}
		})
	}
}
