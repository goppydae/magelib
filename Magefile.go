//go:build mage
// +build mage

package main

import (
	"fmt"

	"github.com/goppydae/magelib/pkg/magelib"
	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

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
	return magelib.CheckHermetic()
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
	mg.Deps(checkHermetic)
	return magelib.Lint("G204", "G304")
}

// Doctor validates the dev shell against the ecosystem pins.
func Doctor() error {
	return magelib.Doctor(magelib.DoctorConfig{
		ProtoPlugins: []string{"buf", "protoc-gen-go", "protoc-gen-go-grpc"},
		RequiredEnv:  []string{"GOBIN"},
		SharedTools:  []string{"buf", "golangci-lint", "gosec", "govulncheck", "mage", "goimports"},
	})
}
