// Copyright (c) 2026 Steven Verhelle (enqack)
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.
//
// SPDX-License-Identifier: MPL-2.0

package magelib_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/goppydae/magelib/pkg/magelib"
)

// The ledger every case below starts from: one resolved entry, one open.
const twoEntryLedger = `{"id": "GAPI-DIV-124", "status": "resolved"}
{"id": "GAPI-DIV-125", "status": "open"}
`

// gitRepo builds a repository whose history is exactly the given commit
// messages, and makes it the working directory.
//
// A real repository rather than a stub, because the parsing this gate
// does is of git's OUTPUT: a fake that emits what the code expects
// cannot disagree with git, and disagreeing with the real tool is the
// only thing a fixture is for.
func gitRepo(t *testing.T, ledger string, messages ...string) string {
	t.Helper()
	dir := t.TempDir()

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.invalid",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.invalid",
			"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}

	run("init", "--initial-branch=main")
	if err := os.WriteFile(filepath.Join(dir, "divergence.jsonl"), []byte(ledger), 0o600); err != nil {
		t.Fatalf("writing ledger: %v", err)
	}
	run("add", "divergence.jsonl")
	for i, msg := range messages {
		if i > 0 {
			name := filepath.Join(dir, "file")
			if err := os.WriteFile(name, []byte(strings.Repeat("x", i)), 0o600); err != nil {
				t.Fatalf("writing fixture file: %v", err)
			}
			run("add", "file")
		}
		run("commit", "--allow-empty", "-m", msg)
	}

	t.Chdir(dir)
	return dir
}

func check(t *testing.T) error {
	t.Helper()
	return magelib.CheckLedgerClosures(magelib.LedgerClosureConfig{Path: "divergence.jsonl"})
}

// A footer naming a resolved entry is the state the gate exists to
// permit.
func TestLedgerClosuresAcceptsAHonouredFooter(t *testing.T) {
	gitRepo(t, twoEntryLedger,
		"fix(client): the control client releases its peer\n\nCloses: GAPI-DIV-124\n")

	if err := check(t); err != nil {
		t.Fatalf("a footer naming a resolved entry must pass: %v", err)
	}
}

// The defect the gate was built for: the fix merged, the entry did not
// change. This is the fault injection - the same fixture as the case
// above with one status flipped - and it must go red.
func TestLedgerClosuresRefusesAnOwedClosure(t *testing.T) {
	gitRepo(t, twoEntryLedger,
		"fix(transport): a request is no longer lost\n\nCloses: GAPI-DIV-125\n")

	err := check(t)
	if err == nil {
		t.Fatal("a footer naming an open entry must fail")
	}
	if !strings.Contains(err.Error(), "GAPI-DIV-125") {
		t.Fatalf("the failure must name the owed entry, got: %v", err)
	}
	if !strings.Contains(err.Error(), "still reads open") {
		t.Fatalf("the failure must say what is wrong, got: %v", err)
	}
}

// THE CASE THAT MAKES THIS GATE DIFFERENT FROM THE NAIVE ONE. Twelve of
// gapi's last sixty commits name an entry that is still open, every one
// of them legitimately - a filing names itself, and a fix cites related
// work. If this case ever goes red the gate has collapsed back into the
// version that was measured unusable.
func TestLedgerClosuresIgnoresAMentionThatIsNotAFooter(t *testing.T) {
	gitRepo(t, twoEntryLedger,
		"docs(ledger): file three version-surface defects\n\n"+
			"Related to GAPI-DIV-125, which stays open. Closes: nothing here,\n"+
			"because this line is prose and not a footer.\n")

	if err := check(t); err != nil {
		t.Fatalf("a prose mention of an open entry must not fail: %v", err)
	}
}

// A mistyped id closes nothing and matches nothing, so without this the
// gate would pass while the entry it meant stayed open.
func TestLedgerClosuresRefusesAnUnknownID(t *testing.T) {
	gitRepo(t, twoEntryLedger,
		"fix(version): the runtime core row\n\nCloses: GAPI-DIV-999\n")

	err := check(t)
	if err == nil {
		t.Fatal("a footer naming no entry at all must fail")
	}
	if !strings.Contains(err.Error(), "names no entry") {
		t.Fatalf("the failure must say the id is unknown, got: %v", err)
	}
}

// A footer several commits back, honoured by a later commit, is the
// two-pull-request workflow this gate deliberately allows.
func TestLedgerClosuresReadsTheWholeHistory(t *testing.T) {
	gitRepo(t, twoEntryLedger,
		"fix(client): the control client releases its peer\n\nCloses: GAPI-DIV-124\n",
		"docs(user): unrelated\n",
		"chore: also unrelated\n")

	if err := check(t); err != nil {
		t.Fatalf("a footer honoured earlier in history must pass: %v", err)
	}
}

// A SHALLOW CLONE IS THE WAY THIS GATE FAILS SILENTLY. actions/checkout
// defaults to fetch-depth 1, which leaves one commit of history, no
// footers, and a clean report that covers nothing.
func TestLedgerClosuresRefusesAShallowRepository(t *testing.T) {
	origin := gitRepo(t, twoEntryLedger,
		"fix(transport): a request is no longer lost\n\nCloses: GAPI-DIV-125\n",
		"docs(user): the commit a depth-1 clone would see\n")

	shallow := t.TempDir()
	cmd := exec.Command("git", "clone", "--depth=1", "file://"+origin, shallow)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git clone --depth=1: %v\n%s", err, out)
	}
	t.Chdir(shallow)

	err := check(t)
	if err == nil {
		t.Fatal("a shallow repository must fail rather than report a clean history")
	}
	if !strings.Contains(err.Error(), "shallow") {
		t.Fatalf("the failure must name the truncated history, got: %v", err)
	}
}

// An unreadable or empty ledger is a broken path, not an empty ledger,
// and reporting zero owed closures against it would be a clean negative
// about nothing.
func TestLedgerClosuresRefusesAnEmptyLedger(t *testing.T) {
	gitRepo(t, "", "chore: nothing\n")

	if err := check(t); err == nil {
		t.Fatal("an empty ledger must fail rather than report a clean run")
	}
}
