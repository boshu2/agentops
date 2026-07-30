package gc

import (
	"bytes"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/clicontract"
)

func executeCommand(t *testing.T, host clicontract.HostOptions, args ...string) (string, error) {
	t.Helper()
	cmd := NewModule(host).Command()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

func TestCommandTreeShape(t *testing.T) {
	cmd := NewModule(clicontract.HostOptions{}).Command()
	if cmd.Name() != "gc" {
		t.Fatalf("root name = %q, want gc", cmd.Name())
	}
	want := map[string]bool{"prepare": false, "check": false, "recover-affinity": false}
	for _, sub := range cmd.Commands() {
		if _, ok := want[sub.Name()]; ok {
			want[sub.Name()] = true
		}
	}
	for name, seen := range want {
		if !seen {
			t.Errorf("missing subcommand %q", name)
		}
	}
}

func TestSubcommandsRequireCityAndRig(t *testing.T) {
	for _, sub := range []string{"prepare", "check", "recover-affinity"} {
		t.Run(sub, func(t *testing.T) {
			_, err := executeCommand(t, clicontract.HostOptions{}, sub)
			if err == nil {
				t.Fatalf("%s without --city/--rig succeeded", sub)
			}
			if !strings.Contains(err.Error(), "city") || !strings.Contains(err.Error(), "rig") {
				t.Fatalf("%s error %q does not name the required city/rig flags", sub, err)
			}
		})
	}
}

func TestPrepareRefusesGlobalDryRun(t *testing.T) {
	host := clicontract.HostOptions{DryRun: func() bool { return true }}
	_, err := executeCommand(t, host, "prepare", "--city", t.TempDir(), "--rig", t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "does not support --dry-run") {
		t.Fatalf("prepare under --dry-run = %v, want a does-not-support refusal", err)
	}
}

func TestApplyFlagOnlyOnRecoverAffinity(t *testing.T) {
	cmd := NewModule(clicontract.HostOptions{}).Command()
	for _, sub := range cmd.Commands() {
		hasApply := sub.Flags().Lookup("apply") != nil
		wantApply := sub.Name() == "recover-affinity"
		if hasApply != wantApply {
			t.Errorf("%s --apply presence = %v, want %v", sub.Name(), hasApply, wantApply)
		}
	}
}

// TestRecoverAffinityGlobalDryRunOverridesApply pins the safety mapping RunE
// delegates through: --apply requests the write, but the global --dry-run
// always forces the read-only dry run.
func TestRecoverAffinityGlobalDryRunOverridesApply(t *testing.T) {
	cases := []struct {
		name   string
		apply  bool
		dryRun bool
		want   bool
	}{
		{"apply alone applies", true, false, true},
		{"dry-run overrides apply", true, true, false},
		{"neither stays dry", false, false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			module := NewModule(clicontract.HostOptions{DryRun: func() bool { return tc.dryRun }})
			cmd := module.recoverAffinityCommand()
			if tc.apply {
				if err := cmd.Flags().Set("apply", "true"); err != nil {
					t.Fatalf("set --apply: %v", err)
				}
			}
			if got := module.effectiveApply(); got != tc.want {
				t.Fatalf("effectiveApply() = %v, want %v", got, tc.want)
			}
		})
	}
}
