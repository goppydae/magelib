package magelib

import (
	"strings"
	"testing"
)

// goLines builds a Go-shaped file of exactly n lines. The first line is
// not a comment, so nothing in it can be mistaken for a generated
// marker.
func goLines(n int) string {
	if n == 0 {
		return ""
	}
	return "package p\n" + strings.Repeat("// filler\n", n-1)
}

func TestCheckFileLengthReportsAnOverLimitFile(t *testing.T) {
	writeTree(t, map[string]string{"big.go": goLines(maxFileLines + 1)})
	err := CheckFileLength(nil)
	if err == nil {
		t.Fatal("an over-limit file was reported clean")
	}
	if !strings.Contains(err.Error(), "big.go") {
		t.Fatalf("error does not name the file: %v", err)
	}
	// The count and the limit both belong in the message: "too long" on
	// its own does not tell the operator how much has to go.
	if !strings.Contains(err.Error(), "501 lines") {
		t.Fatalf("error does not carry the line count: %v", err)
	}
	if !strings.Contains(err.Error(), "limit 500") {
		t.Fatalf("error does not carry the limit: %v", err)
	}
}

// TestCheckFileLengthBoundary is the whole rule. An off-by-one here is
// invisible in normal use - every file is either comfortably under or
// grossly over - and would quietly move the ecosystem's limit by one.
func TestCheckFileLengthBoundary(t *testing.T) {
	t.Run("at the limit passes", func(t *testing.T) {
		writeTree(t, map[string]string{"exact.go": goLines(maxFileLines)})
		if err := CheckFileLength(nil); err != nil {
			t.Fatalf("a file of exactly %d lines failed: %v", maxFileLines, err)
		}
	})
	t.Run("one over fails", func(t *testing.T) {
		writeTree(t, map[string]string{"over.go": goLines(maxFileLines + 1)})
		if err := CheckFileLength(nil); err == nil {
			t.Fatalf("a file of %d lines passed", maxFileLines+1)
		}
	})
}

// TestCheckFileLengthReportsEveryViolation: a gate that stops at the
// first hit turns one fix into N build cycles.
func TestCheckFileLengthReportsEveryViolation(t *testing.T) {
	writeTree(t, map[string]string{
		"a.go":       goLines(maxFileLines + 1),
		"pkg/b.go":   goLines(maxFileLines + 100),
		"pkg/ok.go":  goLines(10),
		"pkg/c_x.go": goLines(maxFileLines + 2),
	})
	err := CheckFileLength(nil)
	if err == nil {
		t.Fatal("no error")
	}
	for _, want := range []string{"a.go", "pkg/b.go", "pkg/c_x.go"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error did not name %s: %v", want, err)
		}
	}
	if strings.Contains(err.Error(), "pkg/ok.go") {
		t.Fatalf("an under-limit file was reported: %v", err)
	}
}

func TestCheckFileLengthHonoursAWaiver(t *testing.T) {
	writeTree(t, map[string]string{"legacy.go": goLines(maxFileLines + 316)})
	if err := CheckFileLength([]string{"legacy.go"}); err != nil {
		t.Fatalf("a waived file failed the gate: %v", err)
	}
}

// TestCheckFileLengthFailsOnAStaleWaiver is what keeps the list finite.
// Without it a fix silently leaves its waiver behind, and the list rots
// into a permanent carve-out that no longer describes any debt.
func TestCheckFileLengthFailsOnAStaleWaiver(t *testing.T) {
	writeTree(t, map[string]string{"fixed.go": goLines(120)})
	err := CheckFileLength([]string{"fixed.go"})
	if err == nil {
		t.Fatal("a waiver on an under-limit file was accepted")
	}
	if !strings.Contains(err.Error(), `waiver "fixed.go"`) {
		t.Fatalf("error does not name the waiver to delete: %v", err)
	}
	if !strings.Contains(err.Error(), "delete the waiver") {
		t.Fatalf("error does not say what to do: %v", err)
	}
}

// TestCheckFileLengthFailsOnAWaiverForAMissingPath: a typo or a deleted
// file leaves the list claiming coverage it does not have.
func TestCheckFileLengthFailsOnAWaiverForAMissingPath(t *testing.T) {
	writeTree(t, map[string]string{"a.go": goLines(10)})
	err := CheckFileLength([]string{"pkg/typo.go"})
	if err == nil {
		t.Fatal("a waiver naming a nonexistent path was accepted")
	}
	if !strings.Contains(err.Error(), "pkg/typo.go") {
		t.Fatalf("error does not name the waiver: %v", err)
	}
	if !strings.Contains(err.Error(), "no such file") {
		t.Fatalf("error does not say the path is missing: %v", err)
	}
}

// TestCheckFileLengthSkipsAnExemptPath covers the mechanism for code
// the rule does not apply to. gapi's gopy-generated adk.go is the real
// case: 1312 lines, a non-standard generated header, and it can never
// come under the limit - so it is an exemption, not debt.
func TestCheckFileLengthSkipsAnExemptPath(t *testing.T) {
	t.Run("by path", func(t *testing.T) {
		writeTree(t, map[string]string{
			"native/adk.go": goLines(1312),
			"native/ok.go":  goLines(10),
		})
		if err := CheckFileLength(nil, "native/adk.go"); err != nil {
			t.Fatalf("a skipped file was measured: %v", err)
		}
	})
	t.Run("by base name at any depth", func(t *testing.T) {
		writeTree(t, map[string]string{
			"a/adk.go": goLines(maxFileLines + 1),
			"b/adk.go": goLines(maxFileLines + 1),
		})
		if err := CheckFileLength(nil, "adk.go"); err != nil {
			t.Fatalf("a base-name skip did not apply at every depth: %v", err)
		}
	})
	t.Run("by directory", func(t *testing.T) {
		writeTree(t, map[string]string{
			"third_party/dep/huge.go": goLines(maxFileLines + 800),
		})
		if err := CheckFileLength(nil, "third_party"); err != nil {
			t.Fatalf("a skipped directory was walked: %v", err)
		}
	})
}

// TestCheckFileLengthDoesNotStaleCheckASkip is the property that makes
// the two mechanisms distinct rather than two names for one thing. A
// skip is not debt: nobody promised it would ever come under the limit,
// so failing when it does would be nonsense, and an exempt file must be
// able to sit in the list forever without rotting it.
func TestCheckFileLengthDoesNotStaleCheckASkip(t *testing.T) {
	writeTree(t, map[string]string{"gen/small.go": goLines(12)})
	if err := CheckFileLength(nil, "gen/small.go"); err != nil {
		t.Fatalf("an under-limit skip was treated as a stale waiver: %v", err)
	}
	// Same for a skip that names nothing at all: unlike a waiver, a skip
	// makes no claim that the path exists today.
	if err := CheckFileLength(nil, "gen/never-existed.go"); err != nil {
		t.Fatalf("a skip on a missing path failed: %v", err)
	}
}

// TestCheckFileLengthRejectsAPathThatIsBothWaiverAndSkip: the two lists
// make opposite promises about the file's future, so claiming both is a
// contradiction about which one is in force. Resolving it silently
// would let the skip win and leave the waiver permanently
// unfalsifiable.
func TestCheckFileLengthRejectsAPathThatIsBothWaiverAndSkip(t *testing.T) {
	writeTree(t, map[string]string{"native/adk.go": goLines(maxFileLines + 1)})
	for _, skip := range []string{"native/adk.go", "adk.go"} {
		t.Run(skip, func(t *testing.T) {
			err := CheckFileLength([]string{"native/adk.go"}, skip)
			if err == nil {
				t.Fatalf("skip %q alongside the same waiver was accepted", skip)
			}
			if !strings.Contains(err.Error(), "both a waiver and a skip") {
				t.Fatalf("wrong error: %v", err)
			}
		})
	}
}

// TestCheckFileLengthSkipsGeneratedCode: the manifesto exempts
// generated code, and the standard marker is the only definition of
// "generated" the Go ecosystem agrees on.
func TestCheckFileLengthSkipsGeneratedCode(t *testing.T) {
	writeTree(t, map[string]string{
		"gen.go": "// Code generated by protoc-gen-go. DO NOT EDIT.\n" +
			goLines(maxFileLines+400),
	})
	if err := CheckFileLength(nil); err != nil {
		t.Fatalf("generated code was counted: %v", err)
	}
}

// TestCheckFileLengthCountsAFileThatOnlyMentionsTheMarker: the marker
// is only honoured above the first line of code. Scanning the whole
// file would let a doc comment that merely quotes the marker exempt
// hand-written source.
func TestCheckFileLengthCountsAFileThatOnlyMentionsTheMarker(t *testing.T) {
	writeTree(t, map[string]string{
		"doc.go": "package p\n" +
			strings.Repeat("// filler\n", maxFileLines) +
			"// Code generated by hand. DO NOT EDIT.\n",
	})
	if err := CheckFileLength(nil); err == nil {
		t.Fatal("a marker below the package clause exempted hand-written code")
	}
}

func TestCheckFileLengthSkipsVendorAndNonGoFiles(t *testing.T) {
	writeTree(t, map[string]string{
		"vendor/dep/huge.go": goLines(maxFileLines + 700),
		"data.json":          goLines(maxFileLines + 700),
		"README.md":          goLines(maxFileLines + 700),
	})
	if err := CheckFileLength(nil); err != nil {
		t.Fatalf("vendored or non-Go files were counted: %v", err)
	}
}

// TestCheckFileLengthCountsTestFiles pins a judgement call. The
// manifesto exempts generated, vendored and data files; a _test.go file
// is none of those, and gapi has a 539-line one. Exempting tests here
// would be a new carve-out invented by the gate rather than stated by
// the rule.
func TestCheckFileLengthCountsTestFiles(t *testing.T) {
	writeTree(t, map[string]string{"big_test.go": goLines(maxFileLines + 39)})
	err := CheckFileLength(nil)
	if err == nil {
		t.Fatal("an over-limit test file was not reported")
	}
	if !strings.Contains(err.Error(), "big_test.go") {
		t.Fatalf("error does not name the test file: %v", err)
	}
}

// TestCheckFileLengthRejectsUnusableWaivers: an absolute or escaping
// waiver can never match a walked path, so it is silently inert - and
// silence from a gate is indistinguishable from a clean tree.
func TestCheckFileLengthRejectsUnusableWaivers(t *testing.T) {
	writeTree(t, map[string]string{"a.go": goLines(maxFileLines + 1)})
	for _, bad := range []string{"/abs/pkg/a.go", "../gapi/core/a.go", "./"} {
		t.Run(bad, func(t *testing.T) {
			err := CheckFileLength([]string{bad})
			if err == nil {
				t.Fatalf("waiver %q was accepted", bad)
			}
			if strings.Contains(err.Error(), "file length check failed") {
				t.Fatalf("waiver %q was treated as a walk result, not a config error: %v", bad, err)
			}
		})
	}
}

// TestCheckFileLengthRejectsUnusableSkips: same argument as for
// waivers, plus the worst case - a skip of "." or "./" prunes the
// entire tree, and a gate that walks nothing reports clean.
func TestCheckFileLengthRejectsUnusableSkips(t *testing.T) {
	writeTree(t, map[string]string{"a.go": goLines(maxFileLines + 1)})
	for _, bad := range []string{"/abs/pkg/a.go", "../gapi/core/a.go", "./", "."} {
		t.Run(bad, func(t *testing.T) {
			err := CheckFileLength(nil, bad)
			if err == nil {
				t.Fatalf("skip %q was accepted", bad)
			}
			if strings.Contains(err.Error(), "file length check failed") {
				t.Fatalf("skip %q silently pruned instead of erroring: %v", bad, err)
			}
		})
	}
}
