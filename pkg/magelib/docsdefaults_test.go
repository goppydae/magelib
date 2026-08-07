// Copyright (c) 2026 Steven Verhelle (enqack)
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.
//
// SPDX-License-Identifier: MPL-2.0

package magelib

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The address the hub quoted wrongly, and the value that motivated the
// whole gate.
const testAddr = "127.0.0.1:29979"

// defaultsFixture writes a docs tree with a defaults.json and the given
// content pages, and returns the config.
func defaultsFixture(t *testing.T, defaults string, pages map[string]string) DocsConfig {
	t.Helper()
	t.Chdir(t.TempDir())
	cfg := testDocsConfig()
	if err := os.MkdirAll(filepath.Join(cfg.Dir, "data"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfg.Dir, "data", "defaults.json"), []byte(defaults), 0o644); err != nil {
		t.Fatal(err)
	}
	for rel, body := range pages {
		full := filepath.Join(cfg.Dir, "content", rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return cfg
}

const oneAddress = `{"transport.address": {"value": "127.0.0.1:29979",
  "env": "GAPI_TRANSPORT_ADDRESS", "type": "string",
  "source": "core/config.Defaults"}}`

func TestCheckDocsDefaults_TranscribedValueIsReported(t *testing.T) {
	cfg := defaultsFixture(t, oneAddress, map[string]string{
		"user/configuration.md": "The daemon listens on " + testAddr + " by default.\n",
	})
	err := CheckDocsDefaults(cfg, nil)
	if err == nil {
		t.Fatal("prose spelling a default must be reported")
	}
	var found *DocsDefaultsError
	if !errors.As(err, &found) {
		t.Fatalf("want *DocsDefaultsError, got %T", err)
	}
	if len(found.Findings) != 1 {
		t.Fatalf("want one finding, got %v", found.Findings)
	}
	f := found.Findings[0]
	if f.Key != "transport.address" || f.Line != 1 {
		t.Errorf("finding = %+v, want transport.address at line 1", f)
	}
	if !strings.Contains(err.Error(), "user/configuration.md") {
		t.Errorf("error %q does not name the file", err)
	}
}

// A fenced example is showing a configuration file, which is the one
// context where writing the literal is the point.
func TestCheckDocsDefaults_FencedBlocksAreNotProse(t *testing.T) {
	cfg := defaultsFixture(t, oneAddress, map[string]string{
		"user/configuration.md": "Example:\n\n```yaml\ntransport:\n  address: " + testAddr + "\n```\n\nThat is all.\n",
	})
	if err := CheckDocsDefaults(cfg, nil); err != nil {
		t.Fatalf("a fenced example must not be reported, got %v", err)
	}
}

// A value in backticks mid-sentence is prose quoting a default, and it
// is the commonest form of the defect. Excluding inline code would
// exclude most real transcriptions.
func TestCheckDocsDefaults_InlineCodeIsStillProse(t *testing.T) {
	cfg := defaultsFixture(t, oneAddress, map[string]string{
		"user/index.md": "The default is `" + testAddr + "`.\n",
	})
	if err := CheckDocsDefaults(cfg, nil); err == nil {
		t.Fatal("a value in backticks is still a transcription")
	}
}

func TestCheckDocsDefaults_ShortcodeIsTheSanctionedForm(t *testing.T) {
	cfg := defaultsFixture(t, oneAddress, map[string]string{
		"user/index.md": `The default is {{< default "transport.address" >}}.` + "\n",
	})
	if err := CheckDocsDefaults(cfg, nil); err != nil {
		t.Fatalf("the shortcode is what prose is supposed to use, got %v", err)
	}
}

func TestCheckDocsDefaults_WaiverIsScopedToOneFileAndOneKey(t *testing.T) {
	cfg := defaultsFixture(t, oneAddress, map[string]string{
		"user/example.md": "Shown literally on purpose: " + testAddr + "\n",
		"user/other.md":   "Transcribed by accident: " + testAddr + "\n",
	})
	waiver := filepath.Join(cfg.Dir, "content", "user", "example.md") + ":transport.address"
	err := CheckDocsDefaults(cfg, []string{waiver})
	if err == nil {
		t.Fatal("a waiver for one file must not excuse another")
	}
	if strings.Contains(err.Error(), "example.md") {
		t.Errorf("the waived file must not be reported: %v", err)
	}
	if !strings.Contains(err.Error(), "other.md") {
		t.Errorf("the unwaived file must still be reported: %v", err)
	}
}

// A waiver whose subject is gone is debt that outlived the thing it
// excused, and left in place it starts reading as policy.
func TestCheckDocsDefaults_WaiverForUnknownKeyIsRejected(t *testing.T) {
	cfg := defaultsFixture(t, oneAddress, map[string]string{"user/index.md": "nothing here\n"})
	err := CheckDocsDefaults(cfg, []string{"docs/content/user/index.md:transport.retired"})
	if err == nil {
		t.Fatal("a waiver naming an undeclared key must be rejected")
	}
	if !strings.Contains(err.Error(), "drop the waiver") {
		t.Errorf("error %q does not say what to do", err)
	}
}

func TestCheckDocsDefaults_MalformedWaiverIsRejected(t *testing.T) {
	cfg := defaultsFixture(t, oneAddress, map[string]string{"user/index.md": "x\n"})
	if err := CheckDocsDefaults(cfg, []string{"no-colon-here"}); err == nil {
		t.Fatal("a waiver not in <path>:<key> form must be rejected")
	}
}

func TestCheckDocsDefaults_SkipPrunesTheWalk(t *testing.T) {
	cfg := defaultsFixture(t, oneAddress, map[string]string{
		"generated/cli.md": "Listens on " + testAddr + "\n",
	})
	skip := Skip{
		Name:   filepath.Join(cfg.Dir, "content", "generated"),
		Reason: "generated reference, produced from the same table the gate reads",
	}
	if err := CheckDocsDefaults(cfg, nil, skip); err != nil {
		t.Fatalf("a skipped directory must not be walked, got %v", err)
	}
}

// The gate cannot search prose for "info", "100" or "true" without
// reporting every page, so it does not - and it says which values it
// left out, because a gate that silently narrows its scope reads as
// full coverage.
func TestCheckDocsDefaults_IndistinctValuesAreExcludedNotSearched(t *testing.T) {
	defaults := `{
	  "logging.level":         {"value": "info",  "type": "string"},
	  "logging.file.maxSize":  {"value": "100",   "type": "int"},
	  "metrics.enabled":       {"value": "false", "type": "bool"},
	  "transport.address":     {"value": "127.0.0.1:29979", "type": "string"}
	}`
	cfg := defaultsFixture(t, defaults, map[string]string{
		"user/index.md": "We log at info level, keep 100 files, and false is false.\n",
	})
	if err := CheckDocsDefaults(cfg, nil); err != nil {
		t.Fatalf("ordinary prose must not trip the gate, got %v", err)
	}

	for _, v := range []struct {
		value string
		want  bool
	}{
		{"127.0.0.1:29979", true},
		{"/var/lib/gapi/gapid.log", true},
		{"info", false},
		{"100", false},
		{"false", false},
		{"true", false},
		{"", false},
		{"quic", false},
	} {
		got, _ := checkableValue(v.value)
		if got != v.want {
			t.Errorf("checkableValue(%q) = %v, want %v", v.value, got, v.want)
		}
	}
}

func TestCheckDocsDefaults_NothingCheckableIsAnError(t *testing.T) {
	cfg := defaultsFixture(t,
		`{"metrics.enabled": {"value": "false", "type": "bool"}}`,
		map[string]string{"user/index.md": "x\n"})
	err := CheckDocsDefaults(cfg, nil)
	if err == nil {
		t.Fatal("a gate with nothing to search for must not report success")
	}
	if !strings.Contains(err.Error(), "without checking anything") {
		t.Errorf("error %q does not explain the refusal", err)
	}
}

func TestCheckDocsDefaults_EmptyDefaultsIsAnError(t *testing.T) {
	cfg := defaultsFixture(t, `{}`, map[string]string{"user/index.md": "x\n"})
	if err := CheckDocsDefaults(cfg, nil); err == nil {
		t.Fatal("a gate over no values is green and means nothing")
	}
}
