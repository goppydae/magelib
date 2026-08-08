// Copyright (c) 2026 Steven Verhelle (enqack)
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.
//
// SPDX-License-Identifier: MPL-2.0

//go:build mage
// +build mage

package main

import (
	"fmt"

	"github.com/goppydae/magelib/pkg/magelib"
	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

// toolchain is this repo's single declaration of what the dev shell must
// provide. Doctor reports on it and checkHermetic gates on it, so the
// advisory check and the enforcing one read one value and cannot drift:
// MAGELIB-DIV-003 is exactly the drift that two declarations produce.
var toolchain = magelib.DoctorConfig{
	ProtoPlugins: []string{"buf", "protoc-gen-go", "protoc-gen-go-grpc"},
	RequiredEnv:  []string{"GOBIN"},
	SharedTools:  []string{"buf", "golangci-lint", "gosec", "govulncheck", "mage", "goimports", "hugo", "gomarkdoc"},
}

// checkHermetic ensures tools are running from Nix store.
//
// magelib is the repo that implements this check for the whole ecosystem,
// and it was the only repo not subject to it: gapi and goblin gate their
// targets on CheckHermetic while magelib's own Build, Test and Lint went
// straight to the tools. A build library that does not hold itself to its
// own rule is exactly the failure MAGELIB-DIV-001 names, so the gate now
// applies here first.
//
// Deliberately not applied to Doctor: doctor exists to diagnose a machine
// in the wrong gear, and refusing to run on that machine would make the
// diagnostic unavailable precisely when it is needed. Fmt and Tidy are
// likewise ungated - they are source hygiene, not builds.
func checkHermetic() error {
	return magelib.CheckHermetic(toolchain.SharedTools...)
}

// checkFileLength holds magelib to the 500-line rule it ships.
//
// magelib deliberately does NOT run CheckTerminology on its own tree -
// terminology.go and terminology_test.go spell the forbidden phrases in
// the clear, so the gate would go red against its own rule declaration.
// This gate has no such problem: it measures files rather than reading
// them for content, so the implementation can be subject to itself.
// Dogfooding the gate you ship is the difference between a library that
// enforces a rule and one that merely exports it, which is the finding
// MAGELIB-DIV-001 was about.
//
// Both lists are empty, and they are different lists. magelib has no
// file over the limit, so there is no debt to waive; and it has no
// generated or vendored Go outside the shared skipDirs, so there is
// nothing the rule does not reach. A repo with debt passes its
// violations as waivers, which must come back out as the files are
// split, and its exempt paths as skips, which never do.
func checkFileLength() error {
	return magelib.CheckFileLength(nil)
}

// licenseNotice is magelib's declaration for the header gate, and it is
// deliberately the whole of it: a holder and a year.
//
// The Exhibit A prose and the SPDX line live in magelib and are not
// spelled here, which is the lesson the terminology gate paid for.
// Moving that gate into this library moved the LOGIC while each
// consumer kept declaring the forbidden phrases in a Magefile that the
// gate itself walks. Four repos each hand-copying a licence notice
// would drift a word at a time, and the gate would certify each repo
// against its own copy - so every repo could pass while spelling the
// licence differently from the others.
//
// Year is 2026, magelib's year of FIRST PUBLICATION, and it does not
// advance. Per-repo years are gapi 2025, goblin 2025, magelib 2026,
// goppydae-docs 2026 (operator decision 16).
var licenseNotice = magelib.LicenseConfig{
	Holder: "Steven Verhelle (enqack)",
	Year:   2026,
}

// LicenseCheck reports every hand-written file missing the MPL notice.
//
// A dependency of Lint as of the sweep that headered this repo's 29
// files, which is the sequencing the license plan requires: wiring it
// BEFORE the sweep would have turned every CI run red, and CI minutes
// are the scarce resource the whole licence effort exists to obtain.
// Sweep and gate land together so the gate is never a mechanism with
// no caller - this ecosystem's most common defect.
//
// Local to magelib. Each repo has its own Lint target with its own
// mg.Deps; magelib.Lint is only the golangci-lint and gosec runner and
// carries no licence check, so this cannot turn gapi or goblin red.
// Those two are wired by their own sweeps.
func LicenseCheck() error {
	return magelib.CheckLicenseHeaders(licenseNotice)
}

// LicenseAdd inserts the notice into every file LicenseCheck reports,
// and prints each path it modified.
//
// Read that output; it is the diff you are about to review, and it is
// also what the `git add` line must be built from. `git add -A` is
// banned twice over in this silo - once because it sweeps up other
// sessions' concurrent work, once because it drags in generated churn -
// and a 400-file sweep is the exact situation that tempts it.
//
// Ungated on checkHermetic, like Fmt and Tidy: this is source hygiene
// that shells out to nothing.
func LicenseAdd() error {
	_, err := magelib.AddLicenseHeaders(licenseNotice)
	return err
}

// Build compiles the library (magelib ships no binaries).
func Build() error {
	mg.Deps(checkHermetic)
	fmt.Println("Building magelib...")
	return sh.RunV("go", "build", "./...")
}

// Test runs the test suite with the race detector.
//
// -v is load-bearing, not noise. Without it the only output is
// "ok <pkg> 1.0s", which a package whose tests all skipped prints
// identically to one where they all passed. MAGELIB-DIV-001 closes on a
// CI run showing a non-zero test count, and the package summary cannot
// show one. See the operator field guide, section 5 item 4.
func Test() error {
	mg.Deps(checkHermetic)
	return sh.RunV("go", "test", "-race", "-v", "./...")
}

// CI reproduces ci.yml's jobs locally, in CI's order.
//
// magelib has no All target to be confused with, but the same rule
// holds: nothing here REPAIRS the tree. Fmt and Tidy stay separate,
// because a target that fixes what CI checks cannot fail the way CI
// fails.
func CI() error {
	return magelib.RunCI(magelib.CIConfig{
		Steps: []magelib.Step{
			magelib.Target("doctor", Doctor),
			magelib.Target("lint", Lint),
			magelib.Target("build", Build),
			magelib.Target("test", Test),
			{Name: "docs:build", Run: Docs{}.Build},
			{Name: "nix flake check --all-systems", Run: magelib.NixFlakeCheckAllSystems},
		},
		Excluded: []string{
			"Consumer Gates - CI runs gapi's and goblin's lint, envcheck and " +
				"doctor against THIS magelib, which is the live-read surface " +
				"(.golangci.yml, the devShell, the pins). Reproduce with " +
				"`mage lint` in each sibling; go.work already points them here",
			"release-guard checkVersion - fires on a tag push, not a pull request",
		},
	})
}

// Fmt formats all Go code with gofmt.
func Fmt() error {
	return magelib.Fmt()
}

// Tidy runs go mod tidy.
func Tidy() error {
	return magelib.Tidy()
}

// Lint runs the shared lint gate.
//
// Rule-level carve-out (documented here, per the go manifesto section 12):
// magelib is a build-orchestration library whose purpose is launching
// ecosystem tools with caller-provided arguments and reading caller-named
// manifest files. gosec G204 (subprocess with variable) and G304 (file read
// via variable) flag that purpose itself, so they are excluded for this repo
// only. Consumer repos call magelib.Lint() with no excludes and stay strict.
func Lint() error {
	mg.Deps(checkHermetic, checkFileLength, LicenseCheck, Docs.Check, checkLedger)
	return magelib.Lint("G204", "G304")
}

// checkLedger gates this repo's ledgers' structure.
func checkLedger() error {
	if err := magelib.CheckDivergence("divergence.jsonl"); err != nil {
		return err
	}
	return magelib.CheckDeprecation("deprecation.jsonl")
}

// Doctor validates the dev shell against the ecosystem pins.
func Doctor() error {
	return magelib.Doctor(toolchain)
}

// docsConfig is this repo's documentation site.
//
// magelib publishes a generated API reference and a landing page, and no
// prose: the design reasoning lives in the ecosystem documentation as
// design/build-environment.md, and a second copy beside the code is a
// second thing to keep in step. There are no Generators here because
// magelib ships no binary and so has no command tree to walk.
var docsConfig = magelib.DocsConfig{
	Dir:     "docs",
	Title:   "magelib",
	BaseURL: "https://goppydae.github.io/magelib/",
	Repo:    "github.com/goppydae/magelib",
	// magelib is the one repo in the silo that exists ONLY to be
	// imported, so a sidebar without this was the strangest omission of
	// the four. MAGELIB-DIV-016 made the pkg.go.dev element opt-in and
	// no consumer opted in, which left every site - including this one -
	// with the link deleted rather than made correct, the outcome that
	// entry's exit explicitly warned against. github.com/goppydae/magelib
	// serves 200 on pkg.go.dev, measured 2026-08-08.
	ImportableModule: true,
	APIPackages: []magelib.APIPackage{
		{Path: "./pkg/magelib", Out: "docs/content/reference/magelib.md", Title: "magelib"},
	},
	Committed: []string{"docs/content/reference/magelib.md"},
}

// Docs groups the documentation targets.
//
// A namespace rather than the flat Docs()/DocsServe() pair the reference
// implementation uses: there are five of these and two are gates, so
// `mage docs:check` reads as a check while `mage docsCheck` reads as
// another way to build.
type Docs mg.Namespace

// Sync materialises the shared Hugo assets into docs/.magelib.
func (Docs) Sync() error {
	mg.Deps(checkHermetic)
	return magelib.DocsSync(docsConfig)
}

// Generate renders the API reference from source.
func (Docs) Generate() error {
	mg.Deps(checkHermetic)
	return magelib.DocsGenerate(docsConfig)
}

// Build renders the static site into docs/public.
func (Docs) Build() error {
	mg.Deps(checkHermetic)
	return magelib.DocsBuild(docsConfig)
}

// Serve runs Hugo's own server with live reload.
func (Docs) Serve() error {
	mg.Deps(checkHermetic)
	return magelib.DocsServe(docsConfig)
}

// Check fails when the committed reference no longer matches the source.
//
// Wired into Lint rather than left to a separate invocation, because a
// gate nobody runs is the state the whole documentation programme exists
// to end: docs/cli-reference.md in goblin was 312 hand-written lines
// nothing compared to anything.
func (Docs) Check() error {
	mg.Deps(checkHermetic)
	// EVERY documentation gate, through one call. Wired into Lint rather
	// than into the Documentation workflow because MAGELIB-DIV-012's exit
	// is explicit that a check running only in CI leaves local lint blind
	// and does not close it.
	return magelib.CheckDocs(docsConfig)
}

// CheckVersion gates the VERSION file against the tag being cut.
//
// Invoked by release-guard.yml on a tag ref, and by the operator before
// cutting one:
//
//	GITHUB_REF_TYPE=tag GITHUB_REF_NAME=v0.5.2 mage checkVersion
//
// Deliberately not a dependency of Lint or Build. It errors off a tag
// ref by design, so wiring it into a target that runs on every PR would
// make it either permanently red or - the likelier repair - silenced
// into the no-op it must never become. See MAGELIB-DIV-006.
//
// No mg.Deps(checkHermetic) either: this reads one file and two
// environment variables and depends on no ecosystem tool, so gating it
// on the hermetic check would let a dev-shell problem present itself as
// a version mismatch.
func CheckVersion() error {
	return magelib.CheckVersionAgainstTag()
}
