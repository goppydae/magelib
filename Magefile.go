//go:build mage
// +build mage

package main

import (
	"fmt"

	"github.com/goppydae/magelib/pkg/magelib"
	"github.com/magefile/mage/sh"
)

// Build compiles the library (magelib ships no binaries).
func Build() error {
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
