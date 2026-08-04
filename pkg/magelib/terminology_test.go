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

// writeTree materializes files under a temp dir and chdirs there, so the
// walk sees a repo-shaped tree rooted at ".".
func writeTree(t *testing.T, files map[string]string) {
	t.Helper()
	dir := t.TempDir()
	for name, body := range files {
		full := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			t.Fatalf("mkdir %s: %v", full, err)
		}
		if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
			t.Fatalf("write %s: %v", full, err)
		}
	}
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		if cerr := os.Chdir(wd); cerr != nil {
			t.Fatalf("restore wd: %v", cerr)
		}
	})
}

var expansionRule = TerminologyRule{
	Phrase:   "Agent Programming Interface",
	Reason:   "GAPI expands to 'Agent Process Infrastructure'",
	FoldCase: true,
}

var stylingRule = TerminologyRule{
	Phrase:       "Goppydae",
	Reason:       "canonical styling is 'GoPPydae'",
	WordBoundary: true,
}

func TestCheckTerminologyPassesOnCleanTree(t *testing.T) {
	writeTree(t, map[string]string{
		"README.md": "# GAPI (GoPPydae Agent Process Infrastructure)\n",
	})
	if err := CheckTerminology([]TerminologyRule{expansionRule, stylingRule}); err != nil {
		t.Fatalf("clean tree = %v, want nil", err)
	}
}

// TestCheckTerminologyCatchesAWrappedPhrase is the one that matters.
// The docs this gate polices are hard-wrapped at 77-83 columns and both
// forbidden phrases are longer than a typical wrap remainder, so a
// contiguous-bytes matcher misses the ACCIDENTAL path - someone writing
// the old phrase from memory into a wrapped paragraph - which is the
// entire threat model.
func TestCheckTerminologyCatchesAWrappedPhrase(t *testing.T) {
	writeTree(t, map[string]string{
		"docs/index.md": "GAPI, the Agent Programming\nInterface, supervises agents.\n",
	})
	err := CheckTerminology([]TerminologyRule{expansionRule})
	if err == nil {
		t.Fatal("a phrase split across a newline was not caught")
	}
	if !strings.Contains(err.Error(), "docs/index.md") {
		t.Fatalf("error does not name the file: %v", err)
	}
}

func TestCheckTerminologyFoldsCaseWhenAsked(t *testing.T) {
	writeTree(t, map[string]string{
		"README.md": "GAPI is not really an agent programming interface.\n",
	})
	if err := CheckTerminology([]TerminologyRule{expansionRule}); err == nil {
		t.Fatal("lowercase prose was not caught by a FoldCase rule")
	}
}

// TestCheckTerminologyDoesNotFoldCaseUnlessAsked pins the asymmetry.
// The styling rule must stay case-exact: folding it would make
// 'Goppydae' match the canonical 'GoPPydae' and the gate would fail on
// correct text, which is worse than not gating at all.
func TestCheckTerminologyDoesNotFoldCaseUnlessAsked(t *testing.T) {
	writeTree(t, map[string]string{
		"README.md": "# GAPI (GoPPydae Agent Process Infrastructure)\n",
	})
	if err := CheckTerminology([]TerminologyRule{stylingRule}); err != nil {
		t.Fatalf("canonical text tripped the case-exact styling rule: %v", err)
	}
}

// TestCheckTerminologyRespectsWordBoundaries keeps Go identifiers out of
// scope. goblin has namespaceGoppydae in internal/ident/named.go, which
// is an identifier and not prose; a substring rule would flag it and the
// only ways out would be renaming code or excluding the file.
func TestCheckTerminologyRespectsWordBoundaries(t *testing.T) {
	writeTree(t, map[string]string{
		"ident.go":   "var namespaceGoppydae = uuid.NewSHA1(ns, []byte(\"goppydae\"))\n",
		"manager.go": "type GoppydaeManager struct{}\n",
		"README.md":  "The Goppydae ecosystem.\n",
	})
	err := CheckTerminology([]TerminologyRule{stylingRule})
	if err == nil {
		t.Fatal("prose occurrence was not caught")
	}
	if strings.Contains(err.Error(), "ident.go") {
		t.Fatalf("word-boundary rule flagged a Go identifier: %v", err)
	}
	if strings.Contains(err.Error(), "manager.go") {
		t.Fatalf("word-boundary rule flagged an identifier suffix: %v", err)
	}
}

func TestCheckTerminologySkipsVendorAndExtras(t *testing.T) {
	writeTree(t, map[string]string{
		"vendor/dep/doc.md": "Agent Programming Interface\n",
		"divergence.jsonl":  "{\"violation\": \"Agent Programming Interface\"}\n",
	})
	if err := CheckTerminology([]TerminologyRule{expansionRule},
		Skip{Name: "divergence.jsonl", Reason: "the ledger quotes the phrases it is about"}); err != nil {
		t.Fatalf("skipped paths were walked: %v", err)
	}
}

// TestCheckTerminologyFailsLoudOnAnUnreadableFile: a gate that cannot
// read a file must not report it clean. An unchecked file reported as
// checked is the failure mode a gate exists to prevent.
func TestCheckTerminologyFailsLoudOnAnUnreadableFile(t *testing.T) {
	writeTree(t, map[string]string{"secret.md": "fine\n"})
	if err := os.Chmod("secret.md", 0o000); err != nil {
		t.Skipf("cannot chmod in this environment: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod("secret.md", 0o600) })
	if os.Geteuid() == 0 {
		t.Skip("running as root; mode bits do not deny reads")
	}
	if err := CheckTerminology([]TerminologyRule{expansionRule}); err == nil {
		t.Fatal("an unreadable file was silently reported clean")
	}
}

func TestCheckTerminologyReportsEveryHit(t *testing.T) {
	writeTree(t, map[string]string{
		"a.md": "Agent Programming Interface\n",
		"b.md": "Agent Programming Interface\n",
	})
	err := CheckTerminology([]TerminologyRule{expansionRule})
	if err == nil {
		t.Fatal("no error")
	}
	if !strings.Contains(err.Error(), "a.md") || !strings.Contains(err.Error(), "b.md") {
		t.Fatalf("error did not name both files: %v", err)
	}
}

// TestGoppydaeTerminologyRulesAreUsable pins the exported rule set as
// the consumer-facing surface: a Magefile wires this in and never
// spells the forbidden phrases itself.
func TestGoppydaeTerminologyRulesAreUsable(t *testing.T) {
	writeTree(t, map[string]string{
		"ok.md":  "# GAPI (GoPPydae Agent Process Infrastructure)\n",
		"bad.md": "GAPI, the Agent Programming Interface. The Goppydae ecosystem.\n",
	})
	err := CheckTerminology(GoppydaeTerminologyRules)
	if err == nil {
		t.Fatal("the exported rules caught nothing")
	}
	if !strings.Contains(err.Error(), "bad.md") {
		t.Fatalf("did not flag bad.md: %v", err)
	}
	if strings.Contains(err.Error(), "ok.md") {
		t.Fatalf("flagged canonical text in ok.md: %v", err)
	}
}

// TestCheckTerminologyRejectsAnUnmatchableWordBoundaryRule: a rule that
// compiles clean and can never fire is silent, and silence from a gate
// is indistinguishable from a clean tree.
func TestCheckTerminologyRejectsAnUnmatchableWordBoundaryRule(t *testing.T) {
	writeTree(t, map[string]string{"a.md": "anything\n"})
	err := CheckTerminology([]TerminologyRule{{
		Phrase:       "(deprecated)",
		Reason:       "unused",
		WordBoundary: true,
	}})
	if err == nil {
		t.Fatal("a rule that could never match was accepted")
	}
	if !strings.Contains(err.Error(), "would never match") {
		t.Fatalf("wrong error: %v", err)
	}
}

// TestCheckTerminologyAcceptsAOneSidedWordBoundaryRule pins the other
// half of the guard. Rejecting whenever EITHER end is non-word is
// strictly wrong: it would kill the rule shape the guard exists to
// enable, and the rejection test alone cannot tell the two apart.
func TestCheckTerminologyAcceptsAOneSidedWordBoundaryRule(t *testing.T) {
	writeTree(t, map[string]string{
		"a.md": "See GAPI (Agent Programming Interface) for details.\n",
	})
	err := CheckTerminology([]TerminologyRule{{
		Phrase:       "GAPI (Agent Programming Interface)",
		Reason:       "x",
		WordBoundary: true,
	}})
	if err == nil {
		t.Fatal("a one-sided word-boundary rule caught nothing")
	}
	if strings.Contains(err.Error(), "would never match") {
		t.Fatalf("a compilable rule was rejected: %v", err)
	}
	if !strings.Contains(err.Error(), "a.md") {
		t.Fatalf("did not flag a.md: %v", err)
	}
}

// TestCheckTerminologyIgnoresSurroundingWhitespaceInAPhrase: the
// boundary decision must read the normalized words, not the raw
// literal. Reading the raw literal made "Goppydae " lose its trailing
// \b and " Goppydae " get falsely rejected.
func TestCheckTerminologyIgnoresSurroundingWhitespaceInAPhrase(t *testing.T) {
	writeTree(t, map[string]string{
		"manager.go": "type GoppydaeManager struct{}\n",
		"prose.md":   "The Goppydae ecosystem.\n",
	})
	// A phrase with incidental surrounding whitespace must behave
	// exactly like the trimmed one: the boundary decision is about the
	// words, not about how the literal was typed.
	err := CheckTerminology([]TerminologyRule{{
		Phrase:       " Goppydae ",
		Reason:       "x",
		WordBoundary: true,
	}})
	if err == nil {
		t.Fatal("a padded phrase caught nothing")
	}
	if strings.Contains(err.Error(), "manager.go") {
		t.Fatalf("padded phrase lost its word boundary: %v", err)
	}
	if !strings.Contains(err.Error(), "prose.md") {
		t.Fatalf("padded phrase did not match prose: %v", err)
	}
}

// TestCheckTerminologyRejectsARuleContainingTheAllowMarker: such a rule
// waives itself on every line it could match, so it is silently dead.
func TestCheckTerminologyRejectsARuleContainingTheAllowMarker(t *testing.T) {
	writeTree(t, map[string]string{"a.md": "anything\n"})
	err := CheckTerminology([]TerminologyRule{{
		Phrase: "terminology:allow must not appear",
		Reason: "x",
	}})
	if err == nil {
		t.Fatal("a self-waiving rule was accepted")
	}
	if !strings.Contains(err.Error(), "could never fire") {
		t.Fatalf("wrong error: %v", err)
	}
}

// Unusable skips are NOT tested here. They are tested across every gate
// at once in skip_test.go, because a per-gate copy of that table is the
// same duplication one level up that let this gate's skip parsing drift
// away from CheckFileLength's in the first place (MAGELIB-DIV-004).

// TestCheckTerminologyDoesNotJoinListItems: '-' and '|' at line start
// are markdown bullets and table cells, not continuation prefixes.
// Treating them as continuations joined unrelated list items into one
// phantom phrase.
func TestCheckTerminologyDoesNotJoinListItems(t *testing.T) {
	writeTree(t, map[string]string{
		"list.md":  "- Agent\n- Programming\n- Interface\n",
		"table.md": "| Agent | Programming | Interface |\n",
		"cmt.md":   " * The Agent Programming\n * Interface is a misnomer.\n",
	})
	err := CheckTerminology([]TerminologyRule{{
		Phrase: "Agent Programming Interface", Reason: "x", FoldCase: true,
	}})
	if err == nil {
		t.Fatal("the C block comment continuation should still be caught")
	}
	if !strings.Contains(err.Error(), "cmt.md") {
		t.Fatalf("C block comment continuation was missed: %v", err)
	}
	for _, no := range []string{"list.md", "table.md"} {
		if strings.Contains(err.Error(), no) {
			t.Fatalf("%s was wrongly flagged as one phrase: %v", no, err)
		}
	}
}

// TestCheckTerminologyReportsTheWaivedCount: a waiver that shows up
// nowhere accumulates without review pressure. gosec prints "Nosec: N";
// this gate says so on both the failing and the clean path.
func TestCheckTerminologyReportsTheWaivedCount(t *testing.T) {
	writeTree(t, map[string]string{
		"lexicon.md": "GAPI is not an Agent Programming Interface. terminology:allow\n",
		"other.md":   "Agent Programming Interface\n",
	})
	err := CheckTerminology([]TerminologyRule{{Phrase: "Agent Programming Interface", Reason: "x"}})
	if err == nil {
		t.Fatal("other.md should have been flagged")
	}
	if !strings.Contains(err.Error(), "(1 waived)") {
		t.Fatalf("waived count missing from the failure: %v", err)
	}
}

// TestCheckTerminologyReportsTheLineOfAHit: suppression is line-scoped,
// so the report has to name the line the marker goes on.
func TestCheckTerminologyReportsTheLineOfAHit(t *testing.T) {
	writeTree(t, map[string]string{
		"doc.md": "one\ntwo\nGAPI, the Agent Programming Interface.\n",
	})
	err := CheckTerminology([]TerminologyRule{{Phrase: "Agent Programming Interface", Reason: "x"}})
	if err == nil {
		t.Fatal("no error")
	}
	if !strings.Contains(err.Error(), "doc.md:3:") {
		t.Fatalf("hit did not carry its 1-based line: %v", err)
	}
}

// TestCheckTerminologyCrossesACommentPrefix covers the likeliest real
// miss in a Go-first silo: a banned phrase written from memory into a
// hard-wrapped comment, where the wrap carries a continuation prefix
// that plain \s+ cannot cross.
func TestCheckTerminologyCrossesACommentPrefix(t *testing.T) {
	writeTree(t, map[string]string{
		"doc.go": "// GAPI, the Agent Programming\n// Interface, supervises agents.\n",
		"q.md":   "> The Agent Programming\n> Interface is a misnomer.\n",
	})
	err := CheckTerminology([]TerminologyRule{{
		Phrase:   "Agent Programming Interface",
		Reason:   "x",
		FoldCase: true,
	}})
	if err == nil {
		t.Fatal("a phrase wrapped across a comment prefix was not caught")
	}
	for _, want := range []string{"doc.go", "q.md"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("did not flag %s: %v", want, err)
		}
	}
}

// TestCheckTerminologySkipsByPathAndByName pins the two shapes of an
// Skip.Name: a bare base name is broad, a '/'-bearing entry
// is exactly one path. Before this, the natural "docs/legacy.md"
// matched nothing at all and said nothing about it.
//
// The second legacy.md, at a different depth, is what separates a path
// skip from a base-name skip: with one legacy.md the two are
// indistinguishable.
func TestCheckTerminologySkipsByPathAndByName(t *testing.T) {
	writeTree(t, map[string]string{
		"docs/legacy.md":     "Agent Programming Interface\n",
		"docs/old/legacy.md": "Agent Programming Interface\n",
		"docs/current.md":    "Agent Programming Interface\n",
	})
	err := CheckTerminology([]TerminologyRule{{Phrase: "Agent Programming Interface", Reason: "x"}},
		Skip{Name: "docs/legacy.md", Reason: "historical doc kept verbatim"})
	if err == nil {
		t.Fatal("current.md should still have been flagged")
	}
	if strings.Contains(err.Error(), "docs/legacy.md") {
		t.Fatalf("path skip did not take effect: %v", err)
	}
	if !strings.Contains(err.Error(), "docs/old/legacy.md") {
		t.Fatalf("path skip degenerated into a base-name skip: %v", err)
	}
	if !strings.Contains(err.Error(), "current.md") {
		t.Fatalf("path skip was too broad: %v", err)
	}
}

// TestCheckTerminologyHonoursAnInlineAllowMarker: prose that explains a
// rule has to be able to quote it. The alternative is unpolicing the
// whole lexicon file.
func TestCheckTerminologyHonoursAnInlineAllowMarker(t *testing.T) {
	writeTree(t, map[string]string{
		"lexicon.md": "GAPI does not stand for Agent Programming Interface. terminology:allow\n",
		"other.md":   "Agent Programming Interface\n",
	})
	err := CheckTerminology([]TerminologyRule{{Phrase: "Agent Programming Interface", Reason: "x"}})
	if err == nil {
		t.Fatal("other.md should have been flagged")
	}
	if strings.Contains(err.Error(), "lexicon.md") {
		t.Fatalf("inline allow marker was ignored: %v", err)
	}
}

// TestCheckTerminologyAllowMarkerDoesNotMaskALaterHit: waiving one line
// must not turn the rest of the file into a blind spot.
func TestCheckTerminologyAllowMarkerDoesNotMaskALaterHit(t *testing.T) {
	writeTree(t, map[string]string{
		"lexicon.md": "GAPI is not an Agent Programming Interface. terminology:allow\n" +
			"filler\n" +
			"But here someone wrote Agent Programming Interface for real.\n",
	})
	err := CheckTerminology([]TerminologyRule{{Phrase: "Agent Programming Interface", Reason: "x"}})
	if err == nil {
		t.Fatal("an unwaived hit after a waived one was missed")
	}
}
