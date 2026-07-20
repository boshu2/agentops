package doctor

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/boshu2/agentops/cli/internal/clicontract"
	doctorapp "github.com/boshu2/agentops/cli/internal/doctor"
	"github.com/boshu2/agentops/cli/internal/quality"
)

// GlobalOptions is the doctor test's stand-in for the root's resolved global
// flags. testModule translates it into the shared clicontract.HostOptions seams
// the module actually reads (DryRun and OutputMode); the module derives its
// JSON intent from OutputMode == "json", so a JSON-true row maps to "json".
type GlobalOptions struct {
	DryRun bool
	JSON   bool
	Output string
}

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

// stubRead overrides the read seams a rendering test needs while inheriting
// the inert fakeRead behavior for everything else.
type stubRead struct {
	fakeRead
	diff    *doctorapp.Report
	explain *doctorapp.Finding
}

func (stub stubRead) Diff(context.Context, doctorapp.ReadRequest) (*doctorapp.Report, error) {
	return stub.diff, nil
}
func (stub stubRead) Explain(context.Context, string) (*doctorapp.Finding, error) {
	return stub.explain, nil
}

func testModule(mutation *fakeMutation, maintenance *fakeMaintenance, globals GlobalOptions) Module {
	return testModuleWithRead(fakeRead{}, mutation, maintenance, globals)
}

func testModuleWithRead(read ReadUseCases, mutation *fakeMutation, maintenance *fakeMaintenance, globals GlobalOptions) Module {
	return NewModule(UseCases{
		LegacyChecks: func(context.Context) []quality.Check { return nil },
		Read:         read, Mutation: mutation, Maintenance: maintenance,
		DetectorCount: func() int { return 0 },
	}, clicontract.HostOptions{
		DryRun: func() bool { return globals.DryRun },
		OutputMode: func() string {
			if globals.JSON {
				return "json"
			}
			return globals.Output
		},
	})
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

// TestDiffRendersFixPlan guards novice edge 4c: the human `ao doctor diff`
// output must frame itself as the --fix PLAN — per finding it states whether
// --fix would act (with the estimated action count) and prints an explicit
// "no automatic fix; manual action: ..." line for non-fixable findings —
// instead of the plain findings list triage already renders. The exit code
// deliberately stays 0 with findings present: diff is a read-only preview of
// the plan, not a health verdict — `ao doctor` / `ao doctor health` own the
// failing exit for findings.
func TestDiffRendersFixPlan(t *testing.T) {
	for _, test := range []struct {
		name    string
		report  *doctorapp.Report
		want    []string
		notWant []string
	}{
		{
			name: "findings render as plan with fixability per finding",
			report: &doctorapp.Report{ExitCode: doctorapp.ExitFindings, Findings: []doctorapp.Finding{
				{ID: "fm-manual-only", Severity: "P1", Title: "needs a human",
					Remediation: doctorapp.Remediation{Command: "install skills manually: run `ao skills link`", AutoFixable: false, EstimatedActions: 0}},
				{ID: "fm-auto", Severity: "P2", Title: "machine can fix",
					Remediation: doctorapp.Remediation{Command: "ao doctor --fix --only fm-auto", AutoFixable: true, EstimatedActions: 3}},
			}},
			want: []string{
				"Fix plan — what --fix would do (2 finding(s), read-only preview):",
				"[P1] fm-manual-only — needs a human",
				"no automatic fix; manual action: install skills manually: run `ao skills link`",
				"[P2] fm-auto — machine can fix",
				"would auto-fix: 3 estimated action(s) via ao doctor --fix --only fm-auto",
			},
		},
		{
			name:   "clean report renders clean-diff line",
			report: &doctorapp.Report{},
			want:   []string{"clean diff: --fix would change nothing"},
			notWant: []string{
				"Fix plan",
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			command := testModuleWithRead(stubRead{diff: test.report}, &fakeMutation{}, &fakeMaintenance{}, GlobalOptions{}).Command()
			var stdout bytes.Buffer
			command.SetOut(&stdout)
			command.SetArgs([]string{"diff"})
			if err := command.Execute(); err != nil {
				t.Fatalf("diff must exit 0 (read-only plan preview), got %v", err)
			}
			for _, want := range test.want {
				if !bytes.Contains(stdout.Bytes(), []byte(want)) {
					t.Fatalf("diff output missing %q:\n%s", want, stdout.String())
				}
			}
			for _, notWant := range test.notWant {
				if bytes.Contains(stdout.Bytes(), []byte(notWant)) {
					t.Fatalf("diff output unexpectedly contains %q:\n%s", notWant, stdout.String())
				}
			}
		})
	}
}

// TestExplainRendersSupersetOfTriageFields guards novice edge 4d: the human
// `ao doctor explain <id>` output must be a superset of the per-finding fields
// robot-triage emits — identity, confidence, every evidence field,
// remediation, and fixability — not just title plus remediation command.
func TestExplainRendersSupersetOfTriageFields(t *testing.T) {
	finding := &doctorapp.Finding{
		ID: "fm-skills-missing", Severity: "P1", Subsystem: "skills",
		Title:      "no installed skills found in any known install location",
		Confidence: 1.0,
		Evidence: doctorapp.Evidence{
			File: ".claude/skills", Lines: []int{7, 9},
			Query: "scan of 4 SkillInstallDirs found 0 SKILL.md subdirs",
			Hash:  "deadbeef",
		},
		Remediation: doctorapp.Remediation{
			Command:        "install skills manually: run `ao skills link`",
			ExplainCommand: "ao doctor explain fm-skills-missing",
			AutoFixable:    false, EstimatedActions: 0,
		},
	}
	command := testModuleWithRead(stubRead{explain: finding}, &fakeMutation{}, &fakeMaintenance{}, GlobalOptions{}).Command()
	var stdout bytes.Buffer
	command.SetOut(&stdout)
	command.SetArgs([]string{"explain", "fm-skills-missing"})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"fm-skills-missing [P1] (skills)",
		"no installed skills found in any known install location",
		"Confidence: 1.00",
		"file: .claude/skills",
		"lines: [7 9]",
		"query: scan of 4 SkillInstallDirs found 0 SKILL.md subdirs",
		"hash: deadbeef",
		"Remediation: install skills manually: run `ao skills link`",
		"Auto-fixable: false (estimated actions: 0)",
		"Explain: ao doctor explain fm-skills-missing",
	} {
		if !bytes.Contains(stdout.Bytes(), []byte(want)) {
			t.Fatalf("explain output missing %q:\n%s", want, stdout.String())
		}
	}
}
