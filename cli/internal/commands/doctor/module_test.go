package doctor

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/spf13/cobra"

	doctorapp "github.com/boshu2/agentops/cli/internal/doctor"
	"github.com/boshu2/agentops/cli/internal/quality"
)

type fakeRead struct{}

func (fakeRead) Diagnose(context.Context, doctorapp.ReadRequest) (*doctorapp.Report, error) {
	return &doctorapp.Report{}, nil
}
func (fakeRead) Triage(context.Context, doctorapp.ReadRequest) (*doctorapp.RobotTriageResult, *doctorapp.Report, error) {
	return &doctorapp.RobotTriageResult{}, &doctorapp.Report{}, nil
}
func (fakeRead) Explain(context.Context, string) (*doctorapp.Finding, error) {
	return &doctorapp.Finding{}, nil
}
func (fakeRead) Capabilities(context.Context) *doctorapp.Capabilities {
	return &doctorapp.Capabilities{}
}
func (fakeRead) Health(context.Context) (string, *doctorapp.HealthResult, error) {
	return "ok", &doctorapp.HealthResult{}, nil
}
func (fakeRead) RobotDocs(context.Context) string                     { return "docs" }
func (fakeRead) List(context.Context) ([]doctorapp.RunSummary, error) { return nil, nil }
func (fakeRead) Diff(context.Context, doctorapp.ReadRequest) (*doctorapp.Report, error) {
	return &doctorapp.Report{}, nil
}

type fakeMutation struct{ request doctorapp.MutationRequest }

func (fake *fakeMutation) Fix(_ context.Context, request doctorapp.MutationRequest) (*doctorapp.Report, error) {
	fake.request = request
	return &doctorapp.Report{ExitCode: doctorapp.ExitHealthy}, nil
}

type fakeMaintenance struct {
	undo doctorapp.UndoRequest
	gc   doctorapp.GCRequest
}

func (fake *fakeMaintenance) Undo(_ context.Context, request doctorapp.UndoRequest) (*doctorapp.UndoResult, error) {
	fake.undo = request
	return &doctorapp.UndoResult{RunID: request.RunID}, nil
}
func (fake *fakeMaintenance) GC(_ context.Context, request doctorapp.GCRequest) (doctorapp.GCResult, error) {
	fake.gc = request
	return doctorapp.GCResult{Matched: 2, DryRun: request.DryRun}, nil
}

func testModule(mutation *fakeMutation, maintenance *fakeMaintenance, globals GlobalOptions) Module {
	return NewModule(UseCases{
		LegacyChecks: func(context.Context) []quality.Check { return nil },
		Read:         fakeRead{}, Mutation: mutation, Maintenance: maintenance,
		DetectorCount: func() int { return 0 },
	}, HostOptions{Globals: func(*cobra.Command) GlobalOptions { return globals }})
}

func TestInheritedDryRunProtectsFixGCAndUndo(t *testing.T) {
	mutation, maintenance := &fakeMutation{}, &fakeMaintenance{}
	for _, args := range [][]string{{"fix"}, {"gc", "--before", "2026-01-01", "--yes"}, {"undo", "latest"}} {
		command := testModule(mutation, maintenance, GlobalOptions{DryRun: true}).Command()
		command.SetArgs(args)
		if err := command.Execute(); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
	}
	if !mutation.request.DryRun || !maintenance.gc.DryRun || !maintenance.undo.DryRun {
		t.Fatalf("fix=%+v gc=%+v undo=%+v", mutation.request, maintenance.gc, maintenance.undo)
	}
}

func TestDryRunUnionProtectsEveryMutationEntryPoint(t *testing.T) {
	for _, test := range []struct {
		name    string
		args    []string
		globals GlobalOptions
		assert  func(*testing.T, *fakeMutation, *fakeMaintenance)
	}{
		{name: "root doctor fix and doctor-local dry-run", args: []string{"--fix", "--dry-run"}, assert: func(t *testing.T, mutation *fakeMutation, _ *fakeMaintenance) {
			if !mutation.request.DryRun {
				t.Fatalf("request = %+v", mutation.request)
			}
		}},
		{name: "fix child and root dry-run", args: []string{"fix"}, globals: GlobalOptions{DryRun: true}, assert: func(t *testing.T, mutation *fakeMutation, _ *fakeMaintenance) {
			if !mutation.request.DryRun {
				t.Fatalf("request = %+v", mutation.request)
			}
		}},
		{name: "gc child and root dry-run", args: []string{"gc", "--before", "2026-01-01", "--yes"}, globals: GlobalOptions{DryRun: true}, assert: func(t *testing.T, _ *fakeMutation, maintenance *fakeMaintenance) {
			if !maintenance.gc.DryRun {
				t.Fatalf("request = %+v", maintenance.gc)
			}
		}},
		{name: "undo child and undo-local dry-run", args: []string{"undo", "latest", "--dry-run"}, assert: func(t *testing.T, _ *fakeMutation, maintenance *fakeMaintenance) {
			if !maintenance.undo.DryRun {
				t.Fatalf("request = %+v", maintenance.undo)
			}
		}},
		{name: "undo child and root dry-run", args: []string{"undo", "latest"}, globals: GlobalOptions{DryRun: true}, assert: func(t *testing.T, _ *fakeMutation, maintenance *fakeMaintenance) {
			if !maintenance.undo.DryRun {
				t.Fatalf("request = %+v", maintenance.undo)
			}
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			mutation, maintenance := &fakeMutation{}, &fakeMaintenance{}
			command := testModule(mutation, maintenance, test.globals).Command()
			command.SetArgs(test.args)
			if err := command.Execute(); err != nil {
				t.Fatal(err)
			}
			test.assert(t, mutation, maintenance)
		})
	}
}

func TestJSONUnionReachesRootAndChildFix(t *testing.T) {
	for _, test := range []struct {
		name    string
		args    []string
		globals GlobalOptions
	}{
		{name: "doctor-local json", args: []string{"--fix", "--json"}},
		{name: "doctor robot alias", args: []string{"--fix", "--robot"}},
		{name: "root global json", args: []string{"fix"}, globals: GlobalOptions{JSON: true}},
		{name: "root output json", args: []string{"fix"}, globals: GlobalOptions{Output: "json"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			mutation, maintenance := &fakeMutation{}, &fakeMaintenance{}
			command := testModule(mutation, maintenance, test.globals).Command()
			var stdout bytes.Buffer
			command.SetOut(&stdout)
			command.SetArgs(test.args)
			if err := command.Execute(); err != nil {
				t.Fatal(err)
			}
			if !mutation.request.JSON {
				t.Fatalf("request = %+v", mutation.request)
			}
			var report doctorapp.Report
			if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
				t.Fatalf("structured output invalid: %v\n%s", err, stdout.String())
			}
		})
	}
}

func TestDoctorFlagOwnershipRemainsLocal(t *testing.T) {
	command := testModule(&fakeMutation{}, &fakeMaintenance{}, GlobalOptions{}).Command()
	if command.Flags().Lookup("dry-run") == nil || command.Flags().Lookup("json") == nil {
		t.Fatal("doctor local flags missing")
	}
	gc, _, _ := command.Find([]string{"gc"})
	if gc.Flags().Lookup("before") == nil || gc.Flags().Lookup("yes") == nil {
		t.Fatal("gc local flags missing")
	}
	if gc.Flags().Lookup("dry-run") != nil {
		t.Fatal("gc must inherit, not own, dry-run")
	}
	undo, _, _ := command.Find([]string{"undo"})
	if undo.Flags().Lookup("dry-run") == nil || undo.Flags().Lookup("strict") == nil {
		t.Fatal("undo local flags missing")
	}
}
