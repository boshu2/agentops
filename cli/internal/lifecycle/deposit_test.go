package lifecycle

import "testing"

func TestGuardDeposit_FailedNeverDeposits(t *testing.T) {
	// A failed verdict must be refused in BOTH modes — the anti-death-spiral
	// invariant: a gate that rejected the outcome cannot strengthen the trail.
	for _, mode := range []DepositMode{DepositModeWarn, DepositModeStrict} {
		allowed, reason := GuardDeposit(&GateVerdict{Passed: false, Source: "x"}, mode)
		if allowed {
			t.Errorf("mode %v: failed verdict was allowed to deposit (%q)", mode, reason)
		}
	}
}

func TestGuardDeposit_PassedDeposits(t *testing.T) {
	for _, mode := range []DepositMode{DepositModeWarn, DepositModeStrict} {
		allowed, reason := GuardDeposit(&GateVerdict{Passed: true, Source: "holdout"}, mode)
		if !allowed {
			t.Errorf("mode %v: passed verdict was refused (%q)", mode, reason)
		}
	}
}

func TestGuardDeposit_MissingVerdict_WarnAllowsStrictRefuses(t *testing.T) {
	allowedWarn, rWarn := GuardDeposit(nil, DepositModeWarn)
	if !allowedWarn {
		t.Errorf("warn mode: missing verdict should be allowed-with-warning, got refused (%q)", rWarn)
	}
	allowedStrict, rStrict := GuardDeposit(nil, DepositModeStrict)
	if allowedStrict {
		t.Errorf("strict mode: missing verdict should be refused, got allowed (%q)", rStrict)
	}
}

func TestResolveDepositMode(t *testing.T) {
	t.Setenv(depositGateEnv, "")
	if ResolveDepositMode() != DepositModeWarn {
		t.Error("default mode should be warn")
	}
	t.Setenv(depositGateEnv, "strict")
	if ResolveDepositMode() != DepositModeStrict {
		t.Error("AO_DEPOSIT_GATE=strict should select strict mode")
	}
	t.Setenv(depositGateEnv, "STRICT")
	if ResolveDepositMode() != DepositModeStrict {
		t.Error("mode resolution should be case-insensitive")
	}
}
