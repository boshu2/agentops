package doctor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/clicontract"
	doctorapp "github.com/boshu2/agentops/cli/internal/doctor"
	"gopkg.in/yaml.v3"
)

type fixOutcome struct {
	report *doctorapp.Report
	err    error
}

func (outcome fixOutcome) Fix(context.Context, doctorapp.MutationRequest) (*doctorapp.Report, error) {
	return outcome.report, outcome.err
}

func recoveryReport() *doctorapp.Report {
	return &doctorapp.Report{
		RunID: "abc123", RunDir: "/repo/.doctor/runs/2026-01-01T00-00-00Z__abc123",
		ActionsTaken: 2, ActionsPath: "/repo/.doctor/runs/2026-01-01T00-00-00Z__abc123/actions.jsonl",
		BackupsDir:  "/repo/.doctor/runs/2026-01-01T00-00-00Z__abc123/backups",
		UndoCommand: "ao doctor undo 2026-01-01T00-00-00Z__abc123",
		// A successful fixer result does not make a persistence failure successful.
		OK: true, ExitCode: doctorapp.ExitHealthy,
	}
}

func requireFixIOError(t *testing.T, err error, primary string) {
	t.Helper()
	var failure *clicontract.ExitError
	if !errors.As(err, &failure) || failure.ExitCode() != 74 || failure.Label != "ao doctor" || err.Error() != primary {
		t.Fatalf("error = %#v, want original %q with labeled exit 74", err, primary)
	}
}

func TestFixFailurePreservesRecovery(t *testing.T) {
	for _, test := range []struct {
		name    string
		args    []string
		globals GlobalOptions
		format  string
	}{
		{name: "human child", args: []string{"fix"}},
		{name: "human root", args: []string{"--fix"}},
		{name: "local json", args: []string{"--fix", "--json"}, format: "json"},
		{name: "robot alias", args: []string{"--fix", "--robot"}, format: "json"},
		{name: "global json", args: []string{"fix"}, globals: GlobalOptions{JSON: true}, format: "json"},
		{name: "output json", args: []string{"fix"}, globals: GlobalOptions{Output: "json"}, format: "json"},
		{name: "output yaml", args: []string{"fix"}, globals: GlobalOptions{Output: "yaml"}, format: "yaml"},
	} {
		t.Run(test.name, func(t *testing.T) {
			report := recoveryReport()
			primary := "doctor: atomic rename report.json: I/O failure"
			module := testModule(&fakeMutation{}, &fakeMaintenance{}, test.globals)
			module.useCases.Mutation = fixOutcome{report, errors.New(primary)}
			command := module.Command()
			var stdout, stderr bytes.Buffer
			command.SetOut(&stdout)
			command.SetErr(&stderr)
			command.SetArgs(test.args)
			requireFixIOError(t, command.Execute(), primary)

			if test.format == "" {
				if stdout.Len() != 0 {
					t.Fatalf("failure emitted a normal success summary: %s", stdout.String())
				}
				for _, want := range []string{report.RunID, report.RunDir, report.ActionsPath, report.BackupsDir, report.UndoCommand, "reported actions: 2"} {
					if !strings.Contains(stderr.String(), want) {
						t.Errorf("human recovery missing %q: %s", want, stderr.String())
					}
				}
			} else {
				var failure map[string]any
				var err error
				if test.format == "yaml" {
					err = yaml.Unmarshal(stdout.Bytes(), &failure)
				} else {
					err = json.Unmarshal(stdout.Bytes(), &failure)
				}
				if err != nil || failure["ok"] != false || failure["error"] != primary || failure["schema_version"] != nil {
					t.Fatalf("want distinct structured failure, got %s (decode: %v)", stdout.String(), err)
				}
				if code := failure["exit_code"]; code != float64(74) && code != 74 {
					t.Errorf("failure exit_code = %v, want 74", code)
				}
				recovery, ok := failure["recovery"].(map[string]any)
				if !ok {
					t.Fatalf("missing recovery context: %s", stdout.String())
				}
				for key, want := range map[string]string{"run_id": report.RunID, "run_dir": report.RunDir, "actions_jsonl_path": report.ActionsPath, "backups_dir": report.BackupsDir, "undo_command": report.UndoCommand} {
					if recovery[key] != want {
						t.Errorf("%s = %v, want %q", key, recovery[key], want)
					}
				}
				if count := recovery["actions_taken"]; count != float64(2) && count != 2 {
					t.Errorf("actions_taken = %v, want 2", count)
				}
			}
			combined := stdout.String() + stderr.String()
			for _, want := range []string{"may already have occurred", "in-memory report", "may be incomplete", "before retrying", "--dry-run --strict", "manual reconciliation"} {
				if !strings.Contains(combined, want) {
					t.Errorf("recovery advice missing %q: %s", want, combined)
				}
			}
			if strings.Contains(combined, "undo latest") {
				t.Errorf("unsafe latest-run advice: %s", combined)
			}
		})
	}
}

func TestFixFailureWithoutRecoveryIdentity(t *testing.T) {
	for _, report := range []*doctorapp.Report{nil, {}} {
		for _, structured := range []bool{false, true} {
			module := testModule(&fakeMutation{}, &fakeMaintenance{}, GlobalOptions{JSON: structured})
			module.useCases.Mutation = fixOutcome{report, errors.New("cannot open run artifacts")}
			command := module.Command()
			var stdout, stderr bytes.Buffer
			command.SetOut(&stdout)
			command.SetErr(&stderr)
			command.SetArgs([]string{"fix"})
			requireFixIOError(t, command.Execute(), "cannot open run artifacts")
			if strings.Contains(stdout.String()+stderr.String(), "ao doctor undo") {
				t.Fatal("missing recovery identity must not invent an undo command")
			}
			if structured {
				var failure map[string]any
				if err := json.Unmarshal(stdout.Bytes(), &failure); err != nil || failure["error"] != "cannot open run artifacts" || failure["ok"] != false {
					t.Fatalf("missing original structured failure: %s, %v", stdout.String(), err)
				}
				if report == nil && failure["recovery"] != nil {
					t.Fatalf("nil report invented recovery metadata: %s", stdout.String())
				}
			}
		}
	}
}

func TestFixSuccessOutputUnchanged(t *testing.T) {
	for _, structured := range []bool{false, true} {
		report := recoveryReport()
		module := testModule(&fakeMutation{}, &fakeMaintenance{}, GlobalOptions{JSON: structured})
		module.useCases.Mutation = fixOutcome{report, nil}
		command := module.Command()
		var stdout, stderr bytes.Buffer
		command.SetOut(&stdout)
		command.SetErr(&stderr)
		command.SetArgs([]string{"fix"})
		if err := command.Execute(); err != nil || stderr.Len() != 0 {
			t.Fatalf("successful fix = %v, stderr=%s", err, stderr.String())
		}
		if structured {
			var got doctorapp.Report
			if err := json.Unmarshal(stdout.Bytes(), &got); err != nil || got.RunID != report.RunID || !got.OK || got.ActionsTaken != 2 {
				t.Fatalf("success report changed: %s, %v", stdout.String(), err)
			}
		} else {
			want := "ao doctor fix — run abc123\n  findings: 0  actions: 2\n  undo: " + report.UndoCommand + "\n"
			if stdout.String() != want {
				t.Fatalf("success summary = %q, want %q", stdout.String(), want)
			}
		}
	}
}

type failedRecoveryWriter struct{}

func (failedRecoveryWriter) Write([]byte) (int, error) {
	return 0, errors.New("output unavailable")
}

func TestFixFailureKeepsPrimaryWhenRecoveryOutputFails(t *testing.T) {
	module := testModule(&fakeMutation{}, &fakeMaintenance{}, GlobalOptions{JSON: true})
	module.useCases.Mutation = fixOutcome{recoveryReport(), errors.New("primary persistence failure")}
	command := module.Command()
	var stderr bytes.Buffer
	command.SetOut(failedRecoveryWriter{})
	command.SetErr(&stderr)
	command.SetArgs([]string{"fix"})
	requireFixIOError(t, command.Execute(), "primary persistence failure")
	if !strings.Contains(stderr.String(), "output unavailable") {
		t.Fatalf("secondary output error missing: %s", stderr.String())
	}
}
