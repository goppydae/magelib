package magelib

import (
	"errors"
	"testing"
)

// setRefEnv sets all three inputs CheckVersionAgainstTag can read, every
// time, including the ones a case wants absent.
//
// Leaving a variable unset is not the same as clearing it: CI runs these
// tests with GITHUB_REF_TYPE and GITHUB_REF_NAME already populated by the
// runner, so a case that established "not a tag ref" by omission would
// assert one thing locally and a different thing in the environment that
// matters. RELEASE_VERSION is cleared for the same reason - it is
// Version()'s highest-precedence input and an inherited value would make
// the override case pass for the wrong reason.
func setRefEnv(t *testing.T, refType, refName, releaseVersion string) {
	t.Helper()
	t.Setenv("GITHUB_REF_TYPE", refType)
	t.Setenv("GITHUB_REF_NAME", refName)
	t.Setenv("RELEASE_VERSION", releaseVersion)
}

// TestCheckVersionAgainstTag_Agreement covers the spellings that must be
// accepted. The tag ref carries a leading "v" and the VERSION file does
// not, so stripping exactly one is part of the contract rather than
// incidental cleanup.
func TestCheckVersionAgainstTag_Agreement(t *testing.T) {
	cases := []struct {
		name    string
		version string
		refName string
	}{
		{"plain release", "0.5.2", "v0.5.2"},
		{"prerelease letter suffix", "0.1.0-proto2f", "v0.1.0-proto2f"},
		{"trailing newline in file", "0.5.2\n", "v0.5.2"},
		{"surrounding whitespace in file", "  0.5.2\t\n", "v0.5.2"},
		{"ref name without leading v", "0.5.2", "0.5.2"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			writeTree(t, map[string]string{"VERSION": tc.version})
			setRefEnv(t, "tag", tc.refName, "")

			if err := CheckVersionAgainstTag(); err != nil {
				t.Fatalf("want nil, got %v", err)
			}
		})
	}
}

// TestCheckVersionAgainstTag_Mismatch is the property the gate exists
// for: MAGELIB-DIV-006, a VERSION file left stale by the tag being cut.
// Both values are read off the typed error rather than matched in the
// message, because a message assertion passes against a function that is
// right about the text and wrong about the comparison.
func TestCheckVersionAgainstTag_Mismatch(t *testing.T) {
	writeTree(t, map[string]string{"VERSION": "0.5.1\n"})
	setRefEnv(t, "tag", "v0.5.2", "")

	err := CheckVersionAgainstTag()

	var mismatch *VersionMismatchError
	if !errors.As(err, &mismatch) {
		t.Fatalf("want *VersionMismatchError, got %v", err)
	}
	if mismatch.File != "0.5.1" {
		t.Errorf("File: want %q, got %q", "0.5.1", mismatch.File)
	}
	if mismatch.Tag != "0.5.2" {
		t.Errorf("Tag: want %q, got %q", "0.5.2", mismatch.Tag)
	}
}

// TestCheckVersionAgainstTag_IgnoresReleaseVersionOverride is the case
// that disagrees with the obvious implementation.
//
// The tempting body for this gate is a comparison against Version().
// That body passes unconditionally on exactly the builds this gate
// polices, because Version() prefers RELEASE_VERSION and then
// GITHUB_REF_NAME - it would compare the tag to itself and measure the
// emitter instead of the property. Here the file is stale, the tag is
// correct, and RELEASE_VERSION agrees with the tag: every Version()-based
// implementation reports success and only a file-reading one fails.
func TestCheckVersionAgainstTag_IgnoresReleaseVersionOverride(t *testing.T) {
	writeTree(t, map[string]string{"VERSION": "0.5.1\n"})
	setRefEnv(t, "tag", "v0.5.2", "0.5.2")

	err := CheckVersionAgainstTag()

	var mismatch *VersionMismatchError
	if !errors.As(err, &mismatch) {
		t.Fatalf("want *VersionMismatchError despite RELEASE_VERSION agreeing with the tag, got %v", err)
	}
	if mismatch.File != "0.5.1" {
		t.Errorf("File: want the VERSION file contents %q, got %q", "0.5.1", mismatch.File)
	}
}

// TestCheckVersionAgainstTag_ErrorsOffTagRef holds the gate to refusing
// rather than passing when it has nothing to compare against.
//
// A check that returns nil when it cannot run reports success for its
// entire existence and is indistinguishable from one that works. This
// gate has exactly one caller - the tag-ref workflow - so reaching it off
// a tag ref means the wiring is wrong, and that must be loud.
func TestCheckVersionAgainstTag_ErrorsOffTagRef(t *testing.T) {
	cases := []struct {
		name    string
		refType string
		refName string
	}{
		{"branch ref", "branch", "main"},
		{"ref type unset", "", "main"},
		{"tag type with empty ref name", "tag", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			writeTree(t, map[string]string{"VERSION": "0.5.2\n"})
			setRefEnv(t, tc.refType, tc.refName, "")

			if err := CheckVersionAgainstTag(); err == nil {
				t.Fatal("want an error off a tag ref, got nil")
			}
		})
	}
}

// TestCheckVersionAgainstTag_ErrorsOnUnusableVersionFile keeps an absent
// or blank file from reading as agreement. An empty file trims to the
// empty string, which must not compare equal to a tag whose leading "v"
// was just stripped off a one-character ref name.
func TestCheckVersionAgainstTag_ErrorsOnUnusableVersionFile(t *testing.T) {
	cases := []struct {
		name  string
		files map[string]string
	}{
		{"file absent", map[string]string{}},
		{"file empty", map[string]string{"VERSION": ""}},
		{"file whitespace only", map[string]string{"VERSION": "\n\t \n"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			writeTree(t, tc.files)
			setRefEnv(t, "tag", "v0.5.2", "")

			err := CheckVersionAgainstTag()
			if err == nil {
				t.Fatal("want an error for an unusable VERSION file, got nil")
			}
			var mismatch *VersionMismatchError
			if errors.As(err, &mismatch) {
				t.Fatalf("want a read error, not a mismatch: %v", err)
			}
		})
	}
}
