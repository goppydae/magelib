// Copyright (c) 2026 Steven Verhelle (enqack)
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.
//
// SPDX-License-Identifier: MPL-2.0

package magelib

import (
	"fmt"
	"os/exec"
	"strings"
)

// CheckShellUnification compares the tool inventory of sibling dev shells by
// resolving each tool's real store path inside every shell. Identical shells
// resolve identical paths; any skew is returned as an error, turning silent
// drift into a red check (ecosystem manifesto section 11, interim measure --
// the shared-nix-module refactor is the recorded target).
//
// shells maps a display name to a flake ref (e.g. "gapi" -> "../gapi").
// The check shells out to `nix develop <ref>`, so it is slower than the
// doctor and lives behind its own target.
// shellInventory pairs a shell's display name with the store paths it
// resolved, in probe order.
type shellInventory struct {
	name  string
	paths map[string]string
}

// compareInventories reports tools whose resolved store paths differ
// between shells. It is pure so it can be tested; the nix probe that
// feeds it is not.
func compareInventories(inv []shellInventory, tools []string) []string {
	var skew []string
	ref := inv[0]
	for _, other := range inv[1:] {
		for _, tool := range tools {
			a, b := ref.paths[tool], other.paths[tool]
			if a != b {
				skew = append(skew, fmt.Sprintf("%s: %s=%s vs %s=%s",
					tool, ref.name, a, other.name, b))
			}
		}
	}
	return skew
}

func CheckShellUnification(shells map[string]string, tools []string) error {
	var inventories []shellInventory

	for name, ref := range shells {
		script := "for t in " + strings.Join(tools, " ") + "; do printf '%s=' \"$t\"; readlink -f \"$(command -v \"$t\")\" || echo MISSING; done"
		out, err := exec.Command("nix", "develop", ref, "--command", "sh", "-c", script).Output()
		if err != nil {
			return fmt.Errorf("entering dev shell %s (%s): %w", name, ref, err)
		}
		paths := map[string]string{}
		for line := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
			if k, v, ok := strings.Cut(line, "="); ok {
				paths[k] = v
			}
		}
		inventories = append(inventories, shellInventory{name: name, paths: paths})
	}

	if len(inventories) < 2 {
		return fmt.Errorf("shell unification check needs at least two shells")
	}

	skew := compareInventories(inventories, tools)
	if len(skew) > 0 {
		return fmt.Errorf("dev shell tool inventories diverge (converge the flakes and locks):\n  %s", strings.Join(skew, "\n  "))
	}
	fmt.Printf("Shell inventories agree across %d shells for %d tools\n", len(inventories), len(tools))
	return nil
}
