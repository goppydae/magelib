package magelib

import (
	"strings"
	"testing"
)

// gates enumerates every gate that consumes Skip. The bad-skip table
// below runs against ALL of them, which is the whole point: writing it
// per gate is the same duplication one level up that let CheckTerminology
// and CheckFileLength drift apart, and a gate added with its own skip
// parsing must show up here as a compile error or a failure, not as a
// quiet second implementation.
//
// walkFailure is the prefix that gate uses when it is reporting on the
// TREE. A bad skip must never produce one: it means the configuration
// was accepted and the gate went on to walk.
var gates = []struct {
	name        string
	walkFailure string
	run         func(skips ...Skip) error
}{
	{
		name:        "CheckTerminology",
		walkFailure: "terminology check failed",
		run: func(s ...Skip) error {
			return CheckTerminology(GoppydaeTerminologyRules, s...)
		},
	},
	{
		name:        "CheckFileLength",
		walkFailure: "file length check failed",
		run: func(s ...Skip) error {
			return CheckFileLength(nil, s...)
		},
	},
}

// violatingTree materializes one file that violates EVERY gate: it is
// over the line limit and it contains a forbidden phrase. That is what
// makes an accepted bad skip detectable in two directions - a skip that
// prunes the tree returns nil, and a skip that is inert returns a walk
// failure. Both are failures of this test.
func violatingTree(t *testing.T) {
	t.Helper()
	writeTree(t, map[string]string{
		"over.go": "package p\n// Agent Programming Interface\n" +
			strings.Repeat("// filler\n", maxFileLines),
	})
}

// TestGatesRejectUnusableSkips is the gate on the gates. Each of these
// shapes cannot mean what a caller intended: it either matches nothing,
// in which case the skip is silently inert, or it matches the walk root,
// in which case the gate walks nothing and reports clean. Silence from a
// gate is indistinguishable from a clean tree, so all of them are
// configuration ERRORS.
func TestGatesRejectUnusableSkips(t *testing.T) {
	bad := []struct {
		name string
		skip Skip
	}{
		{"no reason", Skip{Name: "docs/legacy.md"}},
		{"blank reason", Skip{Name: "docs/legacy.md", Reason: "   "}},
		{"empty bare name", Skip{Name: "", Reason: "x"}},
		{"bare dot", Skip{Name: ".", Reason: "x"}},
		{"bare dotdot", Skip{Name: "..", Reason: "x"}},
		{"absolute path", Skip{Name: "/abs/docs/x.md", Reason: "x"}},
		{"escaping path", Skip{Name: "../escape", Reason: "x"}},
		{"path cleaning to dot", Skip{Name: "./", Reason: "x"}},
		{"path cleaning to dotdot", Skip{Name: "docs/../..", Reason: "x"}},
	}
	for _, g := range gates {
		for _, b := range bad {
			t.Run(g.name+"/"+b.name, func(t *testing.T) {
				violatingTree(t)
				err := g.run(b.skip)
				if err == nil {
					t.Fatalf("%s accepted skip %+v and reported clean", g.name, b.skip)
				}
				if strings.Contains(err.Error(), g.walkFailure) {
					t.Fatalf("%s took skip %+v as valid and walked: %v", g.name, b.skip, err)
				}
				if !strings.Contains(err.Error(), b.skip.Name) {
					t.Fatalf("%s rejected skip %+v without naming it: %v", g.name, b.skip, err)
				}
			})
		}
	}
}

// TestGatesNameThemselvesWhenRejectingASkip: a rejection the operator
// cannot trace to a configuration is a rejection they have to bisect.
func TestGatesNameThemselvesWhenRejectingASkip(t *testing.T) {
	for _, g := range gates {
		t.Run(g.name, func(t *testing.T) {
			violatingTree(t)
			err := g.run(Skip{Name: ".", Reason: "x"})
			if err == nil {
				t.Fatal("bare '.' skip was accepted")
			}
			if !strings.Contains(err.Error(), "check: skip") {
				t.Fatalf("error does not name the gate's configuration: %v", err)
			}
		})
	}
}

// TestGatesAcceptAValidSkip is the other half: the guards must reject
// the unusable shapes without rejecting the usable ones. A gate that
// refuses every skip passes the table above and is still broken.
func TestGatesAcceptAValidSkip(t *testing.T) {
	for _, g := range gates {
		t.Run(g.name, func(t *testing.T) {
			writeTree(t, map[string]string{
				"gen/over.go": "package p\n// Agent Programming Interface\n" +
					strings.Repeat("// filler\n", maxFileLines),
			})
			if err := g.run(Skip{Name: "gen", Reason: "generated output"}); err != nil {
				t.Fatalf("a valid skip was rejected: %v", err)
			}
		})
	}
}

// TestBareDotSkipDoesNotSilenceTerminology is the regression this all
// exists for, stated in the terms it was measured in: WalkDir's root
// entry is named ".", so a bare "." skip pruned the entire tree and
// CheckTerminology reported a violating tree clean. Against the code
// that shipped before MAGELIB-DIV-004 this test fails.
func TestBareDotSkipDoesNotSilenceTerminology(t *testing.T) {
	writeTree(t, map[string]string{"docs/a.md": "Agent Programming Interface\n"})
	rules := []TerminologyRule{{Phrase: "Agent Programming Interface", Reason: "x"}}
	if err := CheckTerminology(rules); err == nil {
		t.Fatal("precondition: the tree does not violate the rule")
	}
	err := CheckTerminology(rules, Skip{Name: ".", Reason: "x"})
	if err == nil {
		t.Fatal("a bare '.' skip silenced a violating tree")
	}
	if strings.Contains(err.Error(), "terminology check failed") {
		t.Fatalf("want a config error, got a walk result: %v", err)
	}
}
