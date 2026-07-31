package magelib

import "testing"

// The baseline the breaking check runs against is caller-supplied and
// environment-overridable, and BufGenerate itself shells out to buf, so
// breakingPlan is the only part of that gate a unit test can reach.
// MAGELIB-DIV-002 was a defect in exactly this selection that survived
// because nothing exercised it.

const defaultAgainst = ".git#ref=HEAD"

func TestBreakingPlanPassesTheParameterThroughByDefault(t *testing.T) {
	baseline, skip := breakingPlan(defaultAgainst)
	if skip {
		t.Fatalf("got skip = %v, want false", skip)
	}
	if baseline != defaultAgainst {
		t.Fatalf("got baseline = %v, want %v", baseline, defaultAgainst)
	}
}

// TestBreakingPlanEnvOverridesTheParameter pins the CI case: callers
// hardcode ".git#ref=HEAD", which a fresh checkout compares against
// itself and can never fail, so CI names its own baseline.
func TestBreakingPlanEnvOverridesTheParameter(t *testing.T) {
	t.Setenv("PROTO_BREAKING_AGAINST", ".git#ref=origin/main")
	baseline, skip := breakingPlan(defaultAgainst)
	if skip {
		t.Fatalf("got skip = %v, want false", skip)
	}
	if baseline != ".git#ref=origin/main" {
		t.Fatalf("got baseline = %v, want %v", baseline, ".git#ref=origin/main")
	}
}

// TestBreakingPlanSkipWinsOverAnExplicitBaseline is the precedence that
// matters. The skip is an operator escape hatch for an intentional
// schema reset; a baseline set elsewhere (a CI variable, a stale shell
// export) must not silently re-arm the gate the operator disarmed.
func TestBreakingPlanSkipWinsOverAnExplicitBaseline(t *testing.T) {
	t.Setenv("PROTO_BREAKING_AGAINST", ".git#ref=origin/main")
	t.Setenv("PROTO_BREAKING_SKIP", "1")
	_, skip := breakingPlan(defaultAgainst)
	if !skip {
		t.Fatalf("got skip = %v, want true", skip)
	}
}

func TestBreakingPlanSkipsWhenOnlySkipIsSet(t *testing.T) {
	t.Setenv("PROTO_BREAKING_SKIP", "1")
	_, skip := breakingPlan(defaultAgainst)
	if !skip {
		t.Fatalf("got skip = %v, want true", skip)
	}
}

// TestBreakingPlanDoesNotSkipOnANonOneValue exists because the check is
// string equality against "1", not a truthiness test. Any other value -
// including a plausible-looking "0", "true", or the empty string - must
// leave the gate armed, and a test that only ever sets "1" cannot tell
// string equality apart from truthiness. Silently disarming a gate is
// the failure mode this whole file exists to prevent.
func TestBreakingPlanDoesNotSkipOnANonOneValue(t *testing.T) {
	for _, value := range []string{"0", "", "true", "yes"} {
		t.Run(value, func(t *testing.T) {
			t.Setenv("PROTO_BREAKING_SKIP", value)
			baseline, skip := breakingPlan(defaultAgainst)
			if skip {
				t.Fatalf("got skip = %v for PROTO_BREAKING_SKIP=%q, want false", skip, value)
			}
			if baseline != defaultAgainst {
				t.Fatalf("got baseline = %v, want %v", baseline, defaultAgainst)
			}
		})
	}
}

// TestBreakingPlanTreatsAnEmptyOverrideAsAbsent: an exported-but-empty
// PROTO_BREAKING_AGAINST is what a CI job that computed no merge base
// leaves behind. Honouring it would hand buf an empty --against.
func TestBreakingPlanTreatsAnEmptyOverrideAsAbsent(t *testing.T) {
	t.Setenv("PROTO_BREAKING_AGAINST", "")
	baseline, skip := breakingPlan(defaultAgainst)
	if skip {
		t.Fatalf("got skip = %v, want false", skip)
	}
	if baseline != defaultAgainst {
		t.Fatalf("got baseline = %v, want %v", baseline, defaultAgainst)
	}
}
