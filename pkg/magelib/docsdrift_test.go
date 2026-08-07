// Copyright (c) 2026 Steven Verhelle (enqack)
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.
//
// SPDX-License-Identifier: MPL-2.0

package magelib

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// generatorWriting builds a generator command that writes body to
// <root>/<rel>. `sh -c script root` makes root available as $0, which is
// exactly the appended-output-root contract DocsConfig.Generators
// declares.
func generatorWriting(rel, body string) []string {
	script := `mkdir -p "$(dirname "$0/` + rel + `")" && printf '%s' '` + body + `' > "$0/` + rel + `"`
	return []string{"sh", "-c", script}
}

// driftFixture sets up a temp repo whose generator writes one reference
// file, and returns the config.
func driftFixture(t *testing.T, body string) DocsConfig {
	t.Helper()
	t.Chdir(t.TempDir())
	cfg := testDocsConfig()
	cfg.Generators = [][]string{generatorWriting("docs/content/reference/cli.md", body)}
	cfg.Committed = []string{"docs/content/reference/cli.md"}
	return cfg
}

func TestCheckDocsDrift_CleanTreePasses(t *testing.T) {
	cfg := driftFixture(t, "generated")
	if err := DocsGenerate(cfg); err != nil {
		t.Fatalf("DocsGenerate: %v", err)
	}
	if err := CheckDocsDrift(cfg); err != nil {
		t.Fatalf("a freshly generated tree must pass, got %v", err)
	}
}

func TestCheckDocsDrift_MutatedArtifactIsStale(t *testing.T) {
	cfg := driftFixture(t, "generated")
	if err := DocsGenerate(cfg); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("docs/content/reference/cli.md", []byte("hand edited"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := CheckDocsDrift(cfg)
	if err == nil {
		t.Fatal("a hand-edited artifact must fail the drift gate")
	}
	var drift *DocsDriftError
	if !errors.As(err, &drift) {
		t.Fatalf("want a *DocsDriftError so a caller need not parse prose, got %T", err)
	}
	if len(drift.Stale) != 1 || drift.Stale[0] != "docs/content/reference/cli.md" {
		t.Errorf("stale set = %v, want the one edited path", drift.Stale)
	}
}

// THE PROPERTY THAT MAKES THE GATE WORTH HAVING. A check that
// regenerates in place repairs the drift it measures: the first run
// fails, the second passes, and the defect is gone before anyone reads
// the message.
func TestCheckDocsDrift_DoesNotWriteToTheWorkingTree(t *testing.T) {
	cfg := driftFixture(t, "generated")
	if err := DocsGenerate(cfg); err != nil {
		t.Fatal(err)
	}
	edited := []byte("hand edited")
	if err := os.WriteFile("docs/content/reference/cli.md", edited, 0o644); err != nil {
		t.Fatal(err)
	}
	before := snapshotTree(t, ".")

	if err := CheckDocsDrift(cfg); err == nil {
		t.Fatal("precondition: the gate must be failing here")
	}

	after := snapshotTree(t, ".")
	if before != after {
		t.Errorf("the drift gate modified the working tree\nbefore:\n%s\nafter:\n%s", before, after)
	}
	current, err := os.ReadFile("docs/content/reference/cli.md")
	if err != nil {
		t.Fatal(err)
	}
	if string(current) != string(edited) {
		t.Errorf("the gate rewrote the artifact it was measuring: %q", current)
	}
	if err := CheckDocsDrift(cfg); err == nil {
		t.Error("a second run went green, so the first run repaired the drift")
	}
}

// A path declared under drift control that generation does not produce
// is a configuration error pointing at a renamed output, not staleness.
func TestCheckDocsDrift_DeclaredButNotGeneratedIsMissing(t *testing.T) {
	cfg := driftFixture(t, "generated")
	cfg.Committed = append(cfg.Committed, "docs/content/reference/renamed.md")
	if err := DocsGenerate(cfg); err != nil {
		t.Fatal(err)
	}
	err := CheckDocsDrift(cfg)
	var drift *DocsDriftError
	if !errors.As(err, &drift) {
		t.Fatalf("want *DocsDriftError, got %v", err)
	}
	if len(drift.Missing) != 1 || drift.Missing[0] != "docs/content/reference/renamed.md" {
		t.Errorf("missing set = %v, want the undeliverable path", drift.Missing)
	}
	if len(drift.Stale) != 0 {
		t.Errorf("an ungenerated path must not be reported as stale, got %v", drift.Stale)
	}
}

// The quiet one: an artifact outside Committed can drift forever and the
// gate stays green, which is the failure mode the whole exercise is
// meant to remove.
func TestCheckDocsDrift_GeneratedButUndeclaredIsReported(t *testing.T) {
	cfg := driftFixture(t, "generated")
	cfg.Generators = append(cfg.Generators, generatorWriting("docs/content/reference/ungated.md", "nothing watches me"))
	if err := DocsGenerate(cfg); err != nil {
		t.Fatal(err)
	}
	err := CheckDocsDrift(cfg)
	var drift *DocsDriftError
	if !errors.As(err, &drift) {
		t.Fatalf("an ungated generated artifact must be reported, got %v", err)
	}
	if len(drift.Untracked) != 1 || drift.Untracked[0] != "docs/content/reference/ungated.md" {
		t.Errorf("untracked set = %v, want the ungated path", drift.Untracked)
	}
}

func TestCheckDocsDrift_EmptyCommittedIsRejected(t *testing.T) {
	cfg := driftFixture(t, "generated")
	cfg.Committed = nil
	err := CheckDocsDrift(cfg)
	if err == nil {
		t.Fatal("a drift gate over no paths is green and means nothing")
	}
	if !strings.Contains(err.Error(), "means nothing") {
		t.Errorf("error %q does not explain the rejection", err)
	}
}

// snapshotTree renders every file path and its contents under root,
// excluding the generated asset directory, as one comparable string.
func snapshotTree(t *testing.T, root string) string {
	t.Helper()
	var lines []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		lines = append(lines, path+" => "+string(data))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(lines)
	return strings.Join(lines, "\n")
}
