package magelib

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Doctor's contract is its output: one line per check -- name, PASS or
// FAIL, and a one-line remedy on FAIL -- plus a non-nil error when
// anything failed. These tests assert that contract, nothing more.
//
// DETERMINISM BOUNDARY -- read before adding assertions here.
//
// Two of the six checks read the ambient shell rather than DoctorConfig:
// hermetic-resolution (every tool must resolve under /nix/store) and
// pin-agreement (shell 'go version' vs the toolchain directive in
// ./go.mod). Their PASS/FAIL depends on whether the suite runs inside
// 'nix develop', so nothing here may depend on their result. In
// particular:
//
//   - never assert the aggregate "doctor: N check(s) failed" count, which
//     those two checks contribute to;
//   - never assert Doctor returns nil, for the same reason;
//   - never assert CheckToolchainPin succeeds against a real go.mod.
//
// What is assertable is the config-driven checks (workspace-shape,
// codegen-reachability, binding-toolchain, required-environment), that a
// failure injected through DoctorConfig makes the returned error non-nil,
// and that each check name appears exactly once per run.

var doctorCheckNames = []string{
	"hermetic-resolution",
	"pin-agreement",
	"workspace-shape",
	"codegen-reachability",
	"binding-toolchain",
	"required-environment",
}

// runDoctor captures the report instead of letting it reach stdout.
func runDoctor(t *testing.T, cfg DoctorConfig) (string, error) {
	t.Helper()
	var buf bytes.Buffer
	cfg.Out = &buf
	err := Doctor(cfg)
	return buf.String(), err
}

// doctorLine returns the single report line for a check, failing if the
// check produced none or more than one.
func doctorLine(t *testing.T, output, name string) string {
	t.Helper()
	found := ""
	for _, line := range strings.Split(output, "\n") {
		if !strings.Contains(line, name) {
			continue
		}
		if found != "" {
			t.Fatalf("got 2+ lines for check %v, want 1; output:\n%s", name, output)
		}
		found = line
	}
	if found == "" {
		t.Fatalf("got no line for check %v, want 1; output:\n%s", name, output)
	}
	return found
}

func TestDoctorWorkspaceShapeFailsWhenAReplaceTargetIsMissing(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "gapi")
	output, err := runDoctor(t, DoctorConfig{ReplaceTargets: []string{missing}})
	if err == nil {
		t.Fatalf("got err = %v, want non-nil", err)
	}
	line := doctorLine(t, output, "workspace-shape")
	if !strings.HasPrefix(line, "FAIL") {
		t.Fatalf("got line = %v, want a FAIL line", line)
	}
	if !strings.Contains(line, missing) {
		t.Fatalf("got line = %v, want it to name %v", line, missing)
	}
	// A FAIL line without a remedy violates the output contract: the
	// operator is told something is wrong and not what to do about it.
	if !strings.Contains(line, "clone the sibling") {
		t.Fatalf("got line = %v, want a remedy telling the operator to clone the sibling", line)
	}
}

// TestDoctorWorkspaceShapeFailsWhenTheTargetIsAFile covers the second
// half of the stat check. "exists but is not a directory" is a distinct
// path through checkWorkspaceShape -- a stray file where a sibling
// checkout belongs -- and a test that only removes the target cannot tell
// the two apart.
func TestDoctorWorkspaceShapeFailsWhenTheTargetIsAFile(t *testing.T) {
	target := filepath.Join(t.TempDir(), "gapi")
	if err := os.WriteFile(target, []byte("not a checkout\n"), 0o600); err != nil {
		t.Fatalf("write %s: %v", target, err)
	}
	output, err := runDoctor(t, DoctorConfig{ReplaceTargets: []string{target}})
	if err == nil {
		t.Fatalf("got err = %v, want non-nil", err)
	}
	line := doctorLine(t, output, "workspace-shape")
	if !strings.HasPrefix(line, "FAIL") {
		t.Fatalf("got line = %v, want a FAIL line", line)
	}
	if !strings.Contains(line, target) {
		t.Fatalf("got line = %v, want it to name %v", line, target)
	}
	if !strings.Contains(line, "clone the sibling") {
		t.Fatalf("got line = %v, want a remedy telling the operator to clone the sibling", line)
	}
}

func TestDoctorCodegenReachabilityFailsForAnUnresolvablePlugin(t *testing.T) {
	plugin := "protoc-gen-magelib-doctor-test-absent"
	output, err := runDoctor(t, DoctorConfig{ProtoPlugins: []string{plugin}})
	if err == nil {
		t.Fatalf("got err = %v, want non-nil", err)
	}
	line := doctorLine(t, output, "codegen-reachability")
	if !strings.HasPrefix(line, "FAIL") {
		t.Fatalf("got line = %v, want a FAIL line", line)
	}
	if !strings.Contains(line, plugin) {
		t.Fatalf("got line = %v, want it to name %v", line, plugin)
	}
	if !strings.Contains(line, "nix develop") {
		t.Fatalf("got line = %v, want a remedy naming 'nix develop'", line)
	}
}

func TestDoctorRequiredEnvironmentFailsForAnUnsetVariable(t *testing.T) {
	name := "MAGELIB_DOCTOR_TEST_VAR"
	// t.Setenv registers restoration of the pre-test value; the Unsetenv
	// after it makes the variable genuinely absent no matter what the
	// ambient shell exported, so this test does not depend on the
	// environment it runs in.
	t.Setenv(name, "placeholder")
	if err := os.Unsetenv(name); err != nil {
		t.Fatalf("unsetenv %s: %v", name, err)
	}
	output, err := runDoctor(t, DoctorConfig{RequiredEnv: []string{name}})
	if err == nil {
		t.Fatalf("got err = %v, want non-nil", err)
	}
	line := doctorLine(t, output, "required-environment")
	if !strings.HasPrefix(line, "FAIL") {
		t.Fatalf("got line = %v, want a FAIL line", line)
	}
	if !strings.Contains(line, name) {
		t.Fatalf("got line = %v, want it to name %v", line, name)
	}
	if !strings.Contains(line, "nix develop") {
		t.Fatalf("got line = %v, want a remedy naming 'nix develop'", line)
	}
}

func TestDoctorRequiredEnvironmentPassesWhenTheVariableIsSet(t *testing.T) {
	name := "MAGELIB_DOCTOR_TEST_VAR"
	t.Setenv(name, "set")
	output, _ := runDoctor(t, DoctorConfig{RequiredEnv: []string{name}})
	line := doctorLine(t, output, "required-environment")
	if !strings.HasPrefix(line, "PASS") {
		t.Fatalf("got line = %v, want a PASS line", line)
	}
}

// TestDoctorBindingToolchainPassesWithANoteWithoutAGopyPin is the only
// exercise of report's pass-with-note branch. A repo with no FFI seam
// must still get a line saying so; silently dropping the check would look
// identical to a clean run on a repo that does have one.
func TestDoctorBindingToolchainPassesWithANoteWithoutAGopyPin(t *testing.T) {
	output, _ := runDoctor(t, DoctorConfig{})
	line := doctorLine(t, output, "binding-toolchain")
	if !strings.HasPrefix(line, "PASS") {
		t.Fatalf("got line = %v, want a PASS line", line)
	}
	if !strings.Contains(line, "no FFI codegen tool required") {
		t.Fatalf("got line = %v, want the 'no FFI codegen tool required' note", line)
	}
}

// TestDoctorReturnsAnErrorWhenAnInjectedCheckFails pins the exit-code
// half of the contract. Only the injected failure is asserted on; the
// error text carries a count this test deliberately does not read.
func TestDoctorReturnsAnErrorWhenAnInjectedCheckFails(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "gapi")
	_, err := runDoctor(t, DoctorConfig{ReplaceTargets: []string{missing}})
	if err == nil {
		t.Fatalf("got err = %v, want non-nil after an injected failure", err)
	}
}

// TestDoctorPrintsExactlyOneLinePerCheck is the "one line per check" half
// of the contract, and it holds whether a check passes or fails, so it
// says nothing about the ambient shell.
func TestDoctorPrintsExactlyOneLinePerCheck(t *testing.T) {
	output, _ := runDoctor(t, DoctorConfig{})
	for _, name := range doctorCheckNames {
		if got := strings.Count(output, name); got != 1 {
			t.Fatalf("got %d occurrences of %v, want 1; output:\n%s", got, name, output)
		}
	}
}

// writeGoMod materializes a go.mod under a fresh temp dir and returns its
// path, so CheckToolchainPin is exercised against known content rather
// than the repo's own manifest.
func writeGoMod(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "go.mod")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

func TestCheckToolchainPinErrorNamesAMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "go.mod")
	err := CheckToolchainPin(path)
	if err == nil {
		t.Fatalf("got err = %v, want non-nil for a missing go.mod", err)
	}
	if !strings.Contains(err.Error(), path) {
		t.Fatalf("got err = %v, want it to name %v", err, path)
	}
}

// TestCheckToolchainPinRequiresAToolchainDirective: a go.mod with no
// directive has nothing to compare against, and reporting that as a pass
// would silently disarm the in-band half of the double pin.
func TestCheckToolchainPinRequiresAToolchainDirective(t *testing.T) {
	path := writeGoMod(t, "module example.com/x\n\ngo 1.26\n")
	err := CheckToolchainPin(path)
	if err == nil {
		t.Fatalf("got err = %v, want non-nil for a go.mod with no toolchain directive", err)
	}
	if !strings.Contains(err.Error(), "add one") {
		t.Fatalf("got err = %v, want a remedy telling the operator to add a directive", err)
	}
}

// TestCheckToolchainPinReportsSkewNamingBothVersions uses a directive no
// real toolchain can match, so the skew branch is reached regardless of
// which go the suite runs under. The shell version is read out of the
// message rather than compared to a literal, for the same reason.
func TestCheckToolchainPinReportsSkewNamingBothVersions(t *testing.T) {
	path := writeGoMod(t, "module example.com/x\n\ngo 1.26\n\ntoolchain go0.0.1\n")
	err := CheckToolchainPin(path)
	if err == nil {
		t.Fatalf("got err = %v, want a skew error", err)
	}
	msg := err.Error()
	if !strings.Contains(msg, "skew") {
		t.Fatalf("got err = %v, want a skew error", err)
	}
	if !strings.Contains(msg, "go0.0.1") {
		t.Fatalf("got err = %v, want it to name the directive value go0.0.1", err)
	}
	const marker = "shell go is "
	i := strings.Index(msg, marker)
	if i < 0 {
		t.Fatalf("got err = %v, want it to name the shell version", err)
	}
	shell := strings.Fields(msg[i+len(marker):])
	if len(shell) == 0 || !strings.HasPrefix(shell[0], "go") || shell[0] == "go0.0.1" {
		t.Fatalf("got err = %v, want a real shell go version alongside the directive", err)
	}
}
