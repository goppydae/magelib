// Copyright (c) 2026 Steven Verhelle (enqack)
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.
//
// SPDX-License-Identifier: MPL-2.0

package magelib

import (
	"strings"
	"testing"
)

// The comparison is tested directly because the exported entry point
// shells out to `nix develop`, which no unit test can do. The probe
// stays thin and untested on purpose; everything that decides pass or
// fail lives here.
func TestCompareInventories_AgreementIsSilent(t *testing.T) {
	inv := []shellInventory{
		{name: "gapi", paths: map[string]string{"go": "/nix/store/aaa/bin/go"}},
		{name: "goblin", paths: map[string]string{"go": "/nix/store/aaa/bin/go"}},
	}
	if got := compareInventories(inv, []string{"go"}); len(got) != 0 {
		t.Errorf("identical store paths must produce no findings, got %v", got)
	}
}

func TestCompareInventories_DifferingPathsAreSkew(t *testing.T) {
	inv := []shellInventory{
		{name: "gapi", paths: map[string]string{"go": "/nix/store/aaa/bin/go"}},
		{name: "goblin", paths: map[string]string{"go": "/nix/store/bbb/bin/go"}},
	}
	got := compareInventories(inv, []string{"go"})
	if len(got) != 1 {
		t.Fatalf("want exactly one finding, got %d: %v", len(got), got)
	}
	for _, want := range []string{"go", "gapi", "goblin", "aaa", "bbb"} {
		if !strings.Contains(got[0], want) {
			t.Errorf("finding %q does not name %q", got[0], want)
		}
	}
}
