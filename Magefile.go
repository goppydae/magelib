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
	SharedTools:  []string{"buf", "golangci-lint", "gosec", "govulncheck", "mage", "goimports"},
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

// Fmt formats all Go code with goimports.
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
	mg.Deps(checkHermetic, checkFileLength)
	return magelib.Lint("G204", "G304")
}

// Doctor validates the dev shell against the ecosystem pins.
func Doctor() error {
	return magelib.Doctor(toolchain)
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
