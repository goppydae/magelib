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
	"sort"
	"strings"
	"testing"
)

// testDocsConfig is the smallest configuration that validates.
func testDocsConfig() DocsConfig {
	return DocsConfig{
		Dir:     "docs",
		Title:   "magelib",
		BaseURL: "https://goppydae.github.io/magelib/",
		Repo:    "github.com/goppydae/magelib",
	}
}

func TestDocsConfig_ValidateRejectsUnusableConfigurations(t *testing.T) {
	cases := []struct {
		name string
		edit func(*DocsConfig)
		want string
	}{
		{"no dir", func(c *DocsConfig) { c.Dir = "" }, "Dir is empty"},
		{"absolute dir", func(c *DocsConfig) { c.Dir = "/docs" }, "repo-relative"},
		{"no title", func(c *DocsConfig) { c.Title = "" }, "Title is empty"},
		{"no base url", func(c *DocsConfig) { c.BaseURL = "" }, "BaseURL is empty"},
		{"no repo", func(c *DocsConfig) { c.Repo = "" }, "Repo is empty"},
		{"repo with scheme", func(c *DocsConfig) { c.Repo = "https://github.com/goppydae/magelib" }, "carries a scheme"},
		{"escaping committed path", func(c *DocsConfig) { c.Committed = []string{"../outside.md"} }, "inside the repo"},
		{"api package without title", func(c *DocsConfig) {
			c.APIPackages = []APIPackage{{Path: "./pkg/...", Out: "docs/x.md"}}
		}, "no Title"},
		{"empty generator", func(c *DocsConfig) { c.Generators = [][]string{{}} }, "run nothing and report success"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := testDocsConfig()
			tc.edit(&cfg)
			err := cfg.validate()
			if err == nil {
				t.Fatalf("%s must be rejected", tc.name)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %q does not explain the rejection (want %q)", err, tc.want)
			}
		})
	}
}

func TestDocsConfig_EditURLDerivesFromRepo(t *testing.T) {
	cfg := testDocsConfig()
	want := "https://github.com/goppydae/magelib/edit/main/docs/content/"
	if got := cfg.editURL(); got != want {
		t.Errorf("derived edit URL = %q, want %q", got, want)
	}
	cfg.EditURL = "https://example.com/edit/"
	if got := cfg.editURL(); got != cfg.EditURL {
		t.Errorf("explicit edit URL must win, got %q", got)
	}
}

// syncInto runs DocsSync inside a temporary working directory and
// returns the paths it produced, relative to docs/.magelib.
func syncInto(t *testing.T, cfg DocsConfig) (string, []string) {
	t.Helper()
	dir := t.TempDir()
	t.Chdir(dir)
	if err := DocsSync(cfg); err != nil {
		t.Fatalf("DocsSync: %v", err)
	}
	root := filepath.Join(cfg.Dir, magelibDir)
	var got []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		got = append(got, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(got)
	return dir, got
}

func TestGomarkdocArgs_PinsTheRepositoryRatherThanInferringIt(t *testing.T) {
	// gomarkdoc detects the repository from git state and emits source
	// links only when detection succeeds. A developer checkout and a CI
	// checkout detect differently, so the SAME source produced two
	// legitimate renderings - 212 lines apart, links against no links -
	// and the drift gate called one of them stale.
	//
	// Measured on magelib PR #33: green locally, red in CI, byte-identical
	// once these three flags are passed. Detection is the defect; pinning
	// is the fix.
	cfg := testDocsConfig()
	args := gomarkdocArgs(cfg, APIPackage{Path: "./pkg/magelib"}, "docs/content/reference/magelib.md")

	joined := strings.Join(args, " ")
	for _, want := range []string{
		"--repository.url https://github.com/goppydae/magelib",
		"--repository.default-branch main",
		"--repository.path /",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("gomarkdoc args missing %q\ngot: %s", want, joined)
		}
	}
}

func TestGomarkdocArgs_RepositoryURLFollowsRepo(t *testing.T) {
	// The URL is derived rather than configured, so a repo that sets Repo
	// correctly cannot also get the link host wrong. Repo is already
	// validated to carry no scheme, which is what makes the prefix safe.
	cfg := testDocsConfig()
	cfg.Repo = "github.com/goppydae/goblin"
	args := gomarkdocArgs(cfg, APIPackage{Path: "./pkg/x"}, "out.md")

	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "--repository.url https://github.com/goppydae/goblin") {
		t.Errorf("repository.url did not follow Repo\ngot: %s", joined)
	}
}

func TestDocsSync_MaterialisesTheExpectedFileSet(t *testing.T) {
	_, got := syncInto(t, testDocsConfig())
	want := []string{
		"assets/css/chroma-github-dark.css",
		"assets/css/theme-github-dark.css",
		"hugo-base.yaml",
		"layouts/partials/custom-header.html",
		"layouts/partials/menu-footer.html",
		"layouts/partials/sidebar/element/githublink.html",
		"layouts/partials/sidebar/element/homelink.html",
		"layouts/partials/sidebar/element/pkgsite.html",
		// layouts/reference/article.html is DELIBERATELY ABSENT
		// (MAGELIB-DIV-012). It was relearn's stock template with the
		// title h1 removed, scoped by Hugo lookup to the reference
		// section, and it is what made 75 generated pages render with no
		// heading at all. Its removal is the fix, so this list is the
		// assertion that it stays removed.
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("synced set:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// The template is rendered, not copied, so the per-repo values must
// actually reach the file Hugo reads.
func TestDocsSync_RendersConfigFromTheRepoValues(t *testing.T) {
	cfg := testDocsConfig()
	syncInto(t, cfg)
	data, err := os.ReadFile(filepath.Join(cfg.Dir, magelibDir, "hugo-base.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	body := string(data)
	for _, want := range []string{
		"title: magelib",
		"baseURL: https://goppydae.github.io/magelib/",
		"editURL: https://github.com/goppydae/magelib/edit/main/docs/content/",
		`url: "https://github.com/goppydae/magelib"`,
		`repo: "github.com/goppydae/magelib"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("rendered config does not carry %q", want)
		}
	}
	if strings.Contains(body, "<no value>") || strings.Contains(body, "{{") {
		t.Errorf("rendered config still carries template syntax:\n%s", body)
	}
}

// A renamed or deleted asset that survives in a consumer's tree keeps
// being served, from a file no version of magelib produces any more.
func TestDocsSync_RemovesStaleAssets(t *testing.T) {
	cfg := testDocsConfig()
	dir, _ := syncInto(t, cfg)
	stale := filepath.Join(dir, cfg.Dir, magelibDir, "layouts", "partials", "retired.html")
	if err := os.WriteFile(stale, []byte("<p>gone</p>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := DocsSync(cfg); err != nil {
		t.Fatalf("second DocsSync: %v", err)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Errorf("stale asset survived a re-sync (stat err = %v)", err)
	}
}

// The gate that follows compares bytes, so the sync must be a function
// of the config alone.
func TestDocsSync_IsDeterministic(t *testing.T) {
	cfg := testDocsConfig()
	dir, _ := syncInto(t, cfg)
	first, err := os.ReadFile(filepath.Join(dir, cfg.Dir, magelibDir, "hugo-base.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if err := DocsSync(cfg); err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadFile(filepath.Join(dir, cfg.Dir, magelibDir, "hugo-base.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Error("two syncs of one config produced different bytes")
	}
}

// Naming a missing file in --config is a hard Hugo error, so "I had
// nothing to override" must not look like a broken setup.
func TestHugoArgs_NamesConfigYamlOnlyWhenItExists(t *testing.T) {
	cfg := testDocsConfig()
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.MkdirAll(cfg.Dir, 0o755); err != nil {
		t.Fatal(err)
	}

	got := strings.Join(hugoArgs(cfg), " ")
	if strings.Contains(got, "config.yaml,") || strings.HasSuffix(got, ",config.yaml") {
		t.Errorf("absent config.yaml must not be named, got %q", got)
	}
	if !strings.Contains(got, filepath.Join(magelibDir, "hugo-base.yaml")) {
		t.Errorf("args %q do not name the shared base config", got)
	}

	if err := os.WriteFile(filepath.Join(cfg.Dir, "config.yaml"), []byte("title: x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got = strings.Join(hugoArgs(cfg), " ")
	if !strings.HasSuffix(got, ",config.yaml") {
		t.Errorf("present config.yaml must be named LAST so the repo overrides the base, got %q", got)
	}
}
