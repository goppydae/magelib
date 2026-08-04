package magelib

import (
	"go/build"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

// testLicense is the configuration every license test runs against.
// One value, so a test that passes cannot be passing against a notice
// only it uses.
func testLicense() LicenseConfig {
	return LicenseConfig{Holder: "Steven Verhelle (enqack)", Year: 2026}
}

// goNotice renders the Go-form notice or fails the test. Tests compare
// against the SAME string the inserter writes rather than a second copy
// pasted into the test file, because two copies of a notice is the
// exact drift this gate exists to prevent and a test holding its own
// copy would stop noticing.
func goNotice(t *testing.T) string {
	t.Helper()
	n, err := testLicense().Notice(".go")
	if err != nil {
		t.Fatalf("render notice: %v", err)
	}
	return n
}

// packageDoc parses src and returns the package doc comment go/doc and
// pkg.go.dev would show. Parsing rather than string-matching is the
// point: the question "did the licence become the package
// documentation" is answered by the same package that answers it for
// pkg.go.dev, not by our opinion of where a blank line goes.
func packageDoc(t *testing.T, src string) string {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), "doc.go", src, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse: %v\n%s", err, src)
	}
	if f.Doc == nil {
		return ""
	}
	return f.Doc.Text()
}

// docCommentFile has a comment block touching its package clause, which
// IS the package doc comment. MEASURED: 34 files in the silo have this
// shape (gapi 22, goblin 12).
const docCommentFile = `// Package widget does a thing.
//
// It has a second paragraph.
package widget

func F() {}
`

// TestNaivePrependAbsorbsTheDocComment is the CONTROL, and it asserts
// the failure rather than the fix.
//
// Without it, the test below could be passing because the hazard does
// not exist - a doc comment that was never at risk cannot be rescued,
// and an assertion that the licence is absent from the package doc
// would hold trivially against any inserter, including one that does
// nothing at all. This test proves the trap is real and that the
// assertion used to check for it CAN fail.
func TestNaivePrependAbsorbsTheDocComment(t *testing.T) {
	naive := goNotice(t) + docCommentFile // no blank line: the whole bug
	doc := packageDoc(t, naive)
	if !strings.Contains(doc, "Mozilla") {
		t.Fatal("naive prepend did NOT absorb the licence into the package doc; " +
			"the hazard this gate defends against no longer exists, so the " +
			"defence below is untested - re-derive the insertion rule")
	}
	if !strings.Contains(doc, "Package widget does a thing.") {
		t.Fatalf("control parsed the wrong comment as the package doc: %q", doc)
	}
}

// TestInsertNoticeDoesNotAbsorbTheDocComment is the property that
// matters. Prepend without a blank line after the notice and the MPL
// text becomes the package documentation, visible in go doc and on
// pkg.go.dev - across 34 files, applied by a tool, in one commit.
func TestInsertNoticeDoesNotAbsorbTheDocComment(t *testing.T) {
	got := string(insertNotice([]byte(docCommentFile), goNotice(t)))
	doc := packageDoc(t, got)
	if strings.Contains(doc, "Mozilla") {
		t.Fatalf("the licence became the package doc comment:\n%q\n\nfile:\n%s", doc, got)
	}
	// The original doc must SURVIVE. An inserter that destroyed the
	// doc comment would also pass the assertion above, and would be a
	// worse bug than the one being prevented.
	if !strings.Contains(doc, "Package widget does a thing.") {
		t.Fatalf("the original doc comment was lost:\n%q\n\nfile:\n%s", doc, got)
	}
	if !strings.HasPrefix(got, goNotice(t)) {
		t.Fatalf("notice is not at the top of the file:\n%s", got)
	}
}

// buildTagFile carries a constraint under a tag chosen not to collide
// with any real one, so the result cannot depend on the host's GOOS,
// GOARCH or the ambient tag set.
const buildTagFile = `//go:build magelibtesttag

package widget

func F() {}
`

const testBuildTag = "magelibtesttag"

// TestInsertNoticePreservesABuildConstraint covers the third trap. A
// notice inserted ABOVE a //go:build line is legal - the constraint may
// be preceded by blank lines and other line comments - while one
// inserted BELOW it, between the tag and the package clause, silently
// disables the constraint. Silently is the problem: the file still
// compiles, and it is simply built in configurations it was written to
// be excluded from.
//
// go/build answers this, because go/build is what the toolchain uses to
// decide.
func TestInsertNoticePreservesABuildConstraint(t *testing.T) {
	writeTree(t, map[string]string{"tagged.go": buildTagFile})

	// CONTROL FIRST: prove MatchFile is actually evaluating the
	// constraint on this file. If the tagged file matched with the tag
	// absent, MatchFile would be answering some other question and
	// every assertion below would be vacuous.
	off := build.Default
	off.BuildTags = nil
	if m, err := off.MatchFile(".", "tagged.go"); err != nil || m {
		t.Fatalf("control: file matched with the tag absent (match=%v err=%v); "+
			"MatchFile is not evaluating this constraint", m, err)
	}
	on := build.Default
	on.BuildTags = []string{testBuildTag}
	if m, err := on.MatchFile(".", "tagged.go"); err != nil || !m {
		t.Fatalf("control: file did not match with the tag set (match=%v err=%v)", m, err)
	}

	if err := os.WriteFile("tagged.go",
		insertNotice([]byte(buildTagFile), goNotice(t)), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	if m, err := off.MatchFile(".", "tagged.go"); err != nil || m {
		t.Fatalf("after insertion the constraint no longer excludes the file "+
			"(match=%v err=%v) - the notice disabled a build tag", m, err)
	}
	if m, err := on.MatchFile(".", "tagged.go"); err != nil || !m {
		t.Fatalf("after insertion the file no longer builds under its own tag "+
			"(match=%v err=%v)", m, err)
	}
}

// TestInsertNoticeKeepsAShebangOnLineOne covers the first trap. A "#!"
// is only a shebang on line 1; a notice prepended above it turns an
// executable script into a text file, and the failure appears at
// runtime rather than in any diff review.
func TestInsertNoticeKeepsAShebangOnLineOne(t *testing.T) {
	const script = "#!/usr/bin/env sh\nset -eu\necho hello\n"
	notice, err := testLicense().Notice(".sh")
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	got := string(insertNotice([]byte(script), notice))
	if !strings.HasPrefix(got, "#!/usr/bin/env sh\n") {
		t.Fatalf("shebang is no longer line 1:\n%s", got)
	}
	if !strings.Contains(got, notice) {
		t.Fatalf("notice missing:\n%s", got)
	}
	if !strings.Contains(got, "set -eu\n") {
		t.Fatalf("body lost:\n%s", got)
	}
	// The notice must come before the body, not be appended after it.
	if strings.Index(got, "SPDX-License-Identifier") > strings.Index(got, "set -eu") {
		t.Fatalf("notice landed below the body:\n%s", got)
	}
}

// TestAddLicenseHeadersPreservesTheExecutableBit: the .sh files in
// scope are executable. A sweep that dropped the bit would break every
// script it touched while reporting success, and 400 files is far too
// many for that to be caught by eye.
func TestAddLicenseHeadersPreservesTheExecutableBit(t *testing.T) {
	writeTree(t, map[string]string{"run.sh": "#!/bin/sh\necho hi\n"})
	if err := os.Chmod("run.sh", 0o755); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	if _, err := AddLicenseHeaders(testLicense()); err != nil {
		t.Fatalf("add: %v", err)
	}
	info, err := os.Stat("run.sh")
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm()&0o111 == 0 {
		t.Fatalf("executable bit lost: mode is %v", info.Mode().Perm())
	}
}
