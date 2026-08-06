// Copyright (c) 2026 Steven Verhelle (enqack)
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.
//
// SPDX-License-Identifier: MPL-2.0

package magelib

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// openDiv is a structurally complete OPEN divergence. Tests mutate one
// key of a copy, so each failure differs from a passing ledger in
// exactly one way - which is what makes the assertion about that thing.
func openDiv() map[string]any {
	return map[string]any{
		"id": "GAPI-DIV-001", "scope": "the agent interface",
		"files":     []any{map[string]any{"path": "core/agentmgr/discovery.go", "symbol": "Agent"}},
		"violation": "v", "rule": "r", "owner": "supervisor", "status": "open",
		"exit": "closes on a gate", "closure": nil,
		"opened": "2026-08-06T15:27:28Z", "closed": nil, "amended": nil,
		"refs": []any{},
	}
}

func resolvedDiv() map[string]any {
	e := openDiv()
	e["id"] = "GAPI-DIV-002"
	e["status"] = "resolved"
	e["closure"] = "what happened"
	e["closed"] = "2026-08-07T09:00:00Z"
	return e
}

func write(t *testing.T, entries ...map[string]any) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "ledger.jsonl")
	var b strings.Builder
	for _, e := range entries {
		raw, err := json.Marshal(e)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		b.Write(raw)
		b.WriteByte('\n')
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	return path
}

func mustFail(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected a failure mentioning %q, got none", want)
	}
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("error does not mention %q: %v", want, err)
	}
}

func TestCheckDivergenceAcceptsOpenAndResolved(t *testing.T) {
	if err := CheckDivergence(write(t, openDiv(), resolvedDiv())); err != nil {
		t.Fatalf("well-formed ledger rejected: %v", err)
	}
}

// The gate DERIVES the requirement from status rather than trusting the
// author, so both directions are enforced.
func TestCheckDivergenceDerivesTerminalKeysFromStatus(t *testing.T) {
	t.Run("open entry may not carry a closing time", func(t *testing.T) {
		e := openDiv()
		e["closed"] = "2026-08-07T09:00:00Z"
		mustFail(t, CheckDivergence(write(t, e)), `closed must be null while status is not "resolved"`)
	})
	t.Run("resolved entry must carry one", func(t *testing.T) {
		e := resolvedDiv()
		e["closed"] = nil
		mustFail(t, CheckDivergence(write(t, e)), `closed is required when status is "resolved"`)
	})
	t.Run("resolved entry must carry a closure", func(t *testing.T) {
		e := resolvedDiv()
		e["closure"] = nil
		mustFail(t, CheckDivergence(write(t, e)), `closure is required when status is "resolved"`)
	})
}

// A reserved value is legal only where the schema permits it. Without
// that, "NA" is an escape hatch that turns any red gate green.
func TestCheckDivergenceRestrictsReservedValues(t *testing.T) {
	t.Run("NaN allowed on opened", func(t *testing.T) {
		e := openDiv()
		e["opened"] = "NaN"
		if err := CheckDivergence(write(t, e)); err != nil {
			t.Fatalf("NaN rejected on a key that permits it: %v", err)
		}
	})
	t.Run("NaN refused on owner", func(t *testing.T) {
		e := openDiv()
		e["owner"] = "NaN"
		mustFail(t, CheckDivergence(write(t, e)), `owner may not be "NaN"`)
	})
	t.Run("NA refused on opened", func(t *testing.T) {
		e := openDiv()
		e["opened"] = "NA"
		mustFail(t, CheckDivergence(write(t, e)), `opened may not be "NA"`)
	})
}

func TestCheckDivergenceRejectsAnUnknownOwner(t *testing.T) {
	e := openDiv()
	e["owner"] = "kernel" // the pre-rename spelling
	mustFail(t, CheckDivergence(write(t, e)), "owner kernel is not one of")
}

func TestCheckDivergenceRequiresRFC3339NotABareDate(t *testing.T) {
	e := openDiv()
	e["opened"] = "2026-08-06"
	mustFail(t, CheckDivergence(write(t, e)), "is not RFC3339")
}

func TestCheckDivergenceRejectsAMissingKey(t *testing.T) {
	e := openDiv()
	delete(e, "scope")
	mustFail(t, CheckDivergence(write(t, e)), "missing key scope")
}

func TestCheckDivergenceRejectsAnUnknownKey(t *testing.T) {
	e := openDiv()
	e["surface"] = "the pre-rename spelling"
	mustFail(t, CheckDivergence(write(t, e)), "unknown key surface")
}

func TestCheckDivergenceRejectsADuplicateID(t *testing.T) {
	mustFail(t, CheckDivergence(write(t, openDiv(), openDiv())), "duplicate id")
}

// An empty files array is a legal and meaningful answer: 30 entries name
// a package or a concept rather than a file.
func TestCheckDivergenceAcceptsEmptyFiles(t *testing.T) {
	e := openDiv()
	e["files"] = []any{}
	if err := CheckDivergence(write(t, e)); err != nil {
		t.Fatalf("empty files rejected: %v", err)
	}
}

func TestCheckDivergenceValidatesFileEntries(t *testing.T) {
	cases := map[string]struct {
		files any
		want  string
	}{
		"not an array":       {"core/x.go", "must be an array"},
		"element not object": {[]any{"core/x.go"}, "must be an object"},
		"missing path":       {[]any{map[string]any{"symbol": "Agent"}}, "needs a non-empty path"},
		"unknown key":        {[]any{map[string]any{"path": "x.go", "line": 3.0}}, "unknown key line"},
		"empty lines":        {[]any{map[string]any{"path": "x.go", "lines": []any{}}}, "non-empty array"},
		"zero line":          {[]any{map[string]any{"path": "x.go", "lines": []any{0.0}}}, "positive integers"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			e := openDiv()
			e["files"] = c.files
			mustFail(t, CheckDivergence(write(t, e)), c.want)
		})
	}
}

func TestCheckDivergenceAcceptsSymbolAndLinesTogether(t *testing.T) {
	e := openDiv()
	e["files"] = []any{map[string]any{
		"path": "core/agentmgr/discovery.go", "symbol": "Agent", "lines": []any{74.0}}}
	if err := CheckDivergence(write(t, e)); err != nil {
		t.Fatalf("path+symbol+lines rejected: %v", err)
	}
}

// A deprecation is announced then REMOVED AT A TAG; it has no exit.
func dep() map[string]any {
	return map[string]any{
		"id": "GAPI-DEP-001", "scope": "EventBus.SubscribeOnce",
		"files":     []any{},
		"rationale": "superseded",
		"owner":     "supervisor", "status": "complete",
		"replacement": "SubscribeCorrelated", "target_removal_tag": "v0.2.0",
		"announced": "2026-07-27T00:00:00Z", "removed": nil, "refs": []any{},
	}
}

func TestCheckDeprecationAcceptsAWellFormedEntry(t *testing.T) {
	if err := CheckDeprecation(write(t, dep())); err != nil {
		t.Fatalf("well-formed deprecation rejected: %v", err)
	}
}

// The distinction the old shape could not express: migration complete
// while the symbol is still present.
func TestCheckDeprecationSeparatesCompleteFromRemoved(t *testing.T) {
	e := dep()
	e["status"] = "removed"
	mustFail(t, CheckDeprecation(write(t, e)), `removed is required when status is "removed"`)

	e["removed"] = "2026-09-01T00:00:00Z"
	if err := CheckDeprecation(write(t, e)); err != nil {
		t.Fatalf("removed entry rejected: %v", err)
	}
}

// The two reserved values are not interchangeable, and this pins which
// key takes which. `target_removal_tag: "NA"` means removal is real but
// not tied to a tag - a closed question. `replacement: null` means there
// IS no replacement, which is not-applicable rather than untracked.
func TestCheckDeprecationReservedValuesAreNotInterchangeable(t *testing.T) {
	e := dep()
	e["target_removal_tag"] = "NA"
	e["replacement"] = nil
	if err := CheckDeprecation(write(t, e)); err != nil {
		t.Fatalf("correct reserved values rejected: %v", err)
	}

	swapped := dep()
	swapped["replacement"] = "NA"
	mustFail(t, CheckDeprecation(write(t, swapped)), `replacement may not be "NA"`)

	swapped2 := dep()
	swapped2["target_removal_tag"] = nil
	mustFail(t, CheckDeprecation(write(t, swapped2)), "target_removal_tag must not be null")
}

func TestCheckDeprecationRejectsADivergenceID(t *testing.T) {
	e := dep()
	e["id"] = "GAPI-DIV-001"
	mustFail(t, CheckDeprecation(write(t, e)), "id does not match")
}

func TestCheckLedgerReportsAMissingFile(t *testing.T) {
	mustFail(t, CheckDivergence(filepath.Join(t.TempDir(), "absent.jsonl")), "reading divergence ledger")
}

// A ref is a typed pointer and both halves are closed sets.
func TestCheckDivergenceValidatesRefs(t *testing.T) {
	t.Run("well formed", func(t *testing.T) {
		e := openDiv()
		e["refs"] = []any{
			map[string]any{"ref": "div:GAPI-DIV-083", "rel": "blocked-by"},
			map[string]any{"ref": "doc:goppydae-docs/content/design/adk-architecture.md", "rel": "governs"},
			map[string]any{"ref": "decision:53", "rel": "related"},
			map[string]any{"ref": "note:notes/close-on-the-gate.md", "rel": "governs"},
		}
		if err := CheckDivergence(write(t, e)); err != nil {
			t.Fatalf("well-formed refs rejected: %v", err)
		}
	})
	bad := map[string]struct {
		refs any
		want string
	}{
		"unknown scheme": {[]any{map[string]any{"ref": "pr:124", "rel": "related"}}, "must be scheme:target"},
		"bare target":    {[]any{map[string]any{"ref": "GAPI-DIV-083", "rel": "related"}}, "must be scheme:target"},
		"empty target":   {[]any{map[string]any{"ref": "div:", "rel": "related"}}, "must be scheme:target"},
		"unknown rel":    {[]any{map[string]any{"ref": "div:GAPI-DIV-083", "rel": "causes"}}, `rel "causes" is not one of`},
		"missing rel":    {[]any{map[string]any{"ref": "div:GAPI-DIV-083"}}, "is not one of"},
		"unknown key":    {[]any{map[string]any{"ref": "div:GAPI-DIV-083", "rel": "related", "why": "x"}}, "unknown key why"},
		"not an object":  {[]any{"div:GAPI-DIV-083"}, "must be an object"},
		"not an array":   {"div:GAPI-DIV-083", "must be an array"},
	}
	for name, c := range bad {
		t.Run(name, func(t *testing.T) {
			e := openDiv()
			e["refs"] = c.refs
			mustFail(t, CheckDivergence(write(t, e)), c.want)
		})
	}
}
