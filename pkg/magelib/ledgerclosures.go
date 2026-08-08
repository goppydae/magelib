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
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/magefile/mage/sh"
)

// The owed-closure gate: a fix that merged without its ledger entry.
//
// TWICE ON 2026-08-08, HOURS APART, A DIVERGENCE WAS FIXED AND MERGED
// WITH ITS ENTRY STILL READING `open` - GAPI-DIV-124 in gapi #143 and
// GAPI-DIV-107 in #140. Both closures were written later by somebody
// re-reading the ledger. That is a pattern rather than an oversight,
// because nothing in a green run distinguishes "closed" from "nobody
// got to it".
//
// THE OBVIOUS GATE DOES NOT WORK AND THE MEASUREMENT IS WHY. Flagging
// any commit whose message NAMES an entry that is still open catches 12
// of gapi's last 60 commits, and all twelve are legitimate: an entry
// being filed names itself, and a fix routinely cites a related open
// entry it does not close. Two real signals against twelve false ones
// is a gate that gets bypassed, which ends in the same place as not
// building it.
//
// So the claim has to be DECLARED rather than inferred. A `Closes:`
// footer means "the exit criterion of this entry is satisfied by this
// commit", and only that footer is read. A mention in prose is a
// reference and costs nothing.
//
// THE CLOSURE IS NOT REQUIRED IN THE SAME PULL REQUEST, deliberately.
// MAGELIB-DIV-012 closed on corpus counts that three consumers' CI
// produced only after its merge, so a closure written in the same diff
// would have had to cite evidence that did not exist yet. The gate
// instead reads the WHOLE history it is given: a footer whose entry is
// still open turns the NEXT pull request red. The debt is bounded to
// one merge instead of to whenever somebody re-reads the ledger.

// closesFooter matches a declared closure. Anchored to a whole line so
// that prose naming an entry - which is the common and legitimate case -
// cannot be mistaken for a claim about it.
var closesFooter = regexp.MustCompile(`(?m)^[ \t]*Closes:[ \t]*(\S+)[ \t]*$`)

// LedgerClosureConfig configures the owed-closure gate.
type LedgerClosureConfig struct {
	// Path is the divergence ledger, normally "divergence.jsonl".
	Path string

	// Ref is the history to walk. Empty means HEAD.
	Ref string
}

// closureClaim is one `Closes:` footer, with enough context to name the
// commit that made the claim in the failure message. An operator who
// sees the red needs the commit, not just the id.
type closureClaim struct {
	ID      string
	Commit  string
	Subject string
}

// CheckLedgerClosures fails when a commit declared an entry closed and
// the ledger still calls it open.
func CheckLedgerClosures(cfg LedgerClosureConfig) error {
	if strings.TrimSpace(cfg.Path) == "" {
		return fmt.Errorf("ledger closures: Path is required")
	}
	ref := strings.TrimSpace(cfg.Ref)
	if ref == "" {
		ref = "HEAD"
	}

	// A SHALLOW CLONE WOULD MAKE THIS GATE REPORT SUCCESS. actions/
	// checkout defaults to fetch-depth 1, so `git log` would see one
	// commit, find no footers, and print a clean zero - a green that
	// covers nothing. Refuse instead of answering from a truncated
	// history.
	shallow, err := sh.Output("git", "rev-parse", "--is-shallow-repository")
	if err != nil {
		return fmt.Errorf("ledger closures: reading repository depth: %w", err)
	}
	if strings.TrimSpace(shallow) == "true" {
		return fmt.Errorf(
			"ledger closures: the repository is shallow, so the history this " +
				"gate reads is truncated and a clean result would mean nothing; " +
				"check out with fetch-depth: 0")
	}

	statuses, err := ledgerStatuses(cfg.Path)
	if err != nil {
		return err
	}

	claims, commits, err := gitClosureClaims(ref)
	if err != nil {
		return err
	}

	problems := owedClosures(claims, statuses)
	if len(problems) > 0 {
		return fmt.Errorf(
			"%d declared closure(s) the ledger %s does not carry:\n  %s",
			len(problems), cfg.Path, strings.Join(problems, "\n  "))
	}

	// Report what was measured, not merely that nothing objected. A
	// count is something a gate that did not run cannot produce, and
	// `exit 0` is not.
	fmt.Printf(
		"Closure gate: %d Closes: footer(s) across %d commit(s) of %s, "+
			"%d entries in %s, 0 owed\n",
		len(claims), commits, ref, len(statuses), cfg.Path)
	return nil
}

// ledgerStatuses reads id -> status from a divergence ledger.
//
// Structural validity is CheckDivergence's job and is not repeated
// here; a line this cannot parse is skipped, because two gates
// reporting one malformed line names the same defect twice.
func ledgerStatuses(path string) (map[string]string, error) {
	raw, err := os.ReadFile(path) //nolint:gosec // caller-declared repo path
	if err != nil {
		return nil, fmt.Errorf("ledger closures: reading %s: %w", path, err)
	}
	statuses := map[string]string{}
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var entry struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		}
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}
		if entry.ID != "" {
			statuses[entry.ID] = entry.Status
		}
	}
	if len(statuses) == 0 {
		return nil, fmt.Errorf(
			"ledger closures: %s yielded no entries, which is a broken path "+
				"or a broken ledger rather than an empty one", path)
	}
	return statuses, nil
}

// gitClosureClaims collects every `Closes:` footer reachable from ref,
// and returns the number of commits walked alongside them.
//
// The record separator is a unit separator rather than a newline
// because a commit body is multi-line by construction, and splitting
// on newlines silently reads only a subject.
func gitClosureClaims(ref string) ([]closureClaim, int, error) {
	const sep = "\x1e"
	out, err := sh.Output("git", "log", ref, "--format=%H%x1f%s%x1f%B"+sep)
	if err != nil {
		return nil, 0, fmt.Errorf("ledger closures: reading history of %s: %w", ref, err)
	}

	var claims []closureClaim
	commits := 0
	for _, record := range strings.Split(out, sep) {
		if strings.TrimSpace(record) == "" {
			continue
		}
		fields := strings.SplitN(strings.TrimLeft(record, "\n"), "\x1f", 3)
		if len(fields) < 3 {
			continue
		}
		commits++
		sha, subject, body := fields[0], fields[1], fields[2]
		for _, m := range closesFooter.FindAllStringSubmatch(body, -1) {
			claims = append(claims, closureClaim{
				ID:      m[1],
				Commit:  sha,
				Subject: subject,
			})
		}
	}
	return claims, commits, nil
}

// owedClosures returns one message per claim the ledger does not honour.
//
// A claim naming an id THAT DOES NOT EXIST is a failure too, and a
// quiet one otherwise: a mistyped id closes nothing, matches nothing,
// and would let the gate pass while the entry it meant stayed open.
func owedClosures(claims []closureClaim, statuses map[string]string) []string {
	seen := map[string]bool{}
	var problems []string
	for _, c := range claims {
		status, known := statuses[c.ID]
		switch {
		case !known:
			problems = append(problems, fmt.Sprintf(
				"%s: %.60s\n    Closes: %s names no entry in the ledger",
				c.Commit[:8], c.Subject, c.ID))
		case status == "open":
			if seen[c.ID] {
				continue
			}
			seen[c.ID] = true
			problems = append(problems, fmt.Sprintf(
				"%s: %.60s\n    Closes: %s but that entry still reads open",
				c.Commit[:8], c.Subject, c.ID))
		}
	}
	sort.Strings(problems)
	return problems
}
