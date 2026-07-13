package beads

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	beadsapp "github.com/boshu2/agentops/cli/internal/beads"
	"github.com/boshu2/agentops/cli/internal/scenarios"
)

func TestModuleBuildsFreshCompleteTrees(t *testing.T) {
	module := NewModule(nil, nil, nil, nil, nil, nil, nil, nil)
	first := module.Command()
	second := module.Command()
	if first == second {
		t.Fatal("Command reused a Cobra tree")
	}
	want := []string{
		"audit", "cluster", "dir", "epic-status", "exec", "harvest", "lint",
		"resume", "scenarios", "stale-claims", "tracker", "verify", "verify-acceptance",
	}
	var got []string
	for _, command := range first.Commands() {
		got = append(got, command.Name())
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("commands = %v, want %v", got, want)
	}
}

type fakeTrackerPorts struct {
	resolution beadsapp.TrackerResolution
	ledger     beadsapp.LedgerResolution
	snapshot   beadsapp.LedgerSnapshot
	override   bool
	executed   bool
	execErr    error
	listOutput []byte
	shown      []beadsapp.StaleBeadRecord
	showIndex  int
	calls      []string
	now        time.Time
	actor      string
	appended   any
	readOutput []byte
}

type fakeKnowledge struct {
	available bool
	verified  *beadsapp.VerifyReport
	linted    *beadsapp.LintReport
	harvested beadsapp.HarvestResult
}

type fakeHygiene struct {
	audit   *beadsapp.AuditReport
	cluster *beadsapp.ClusterReport
}

type fakeScenario struct {
	extraction beadsapp.ScenarioExtraction
	validation beadsapp.ScenarioValidation
	applied    int
}

func (fake *fakeScenario) Available() bool { return true }
func (fake *fakeScenario) PrepareScenarios(string, bool) (beadsapp.ScenarioExtraction, error) {
	return fake.extraction, nil
}
func (fake *fakeScenario) ApplyScenarios(beadsapp.ScenarioExtraction) error {
	fake.applied++
	return nil
}
func (fake *fakeScenario) ValidateScenarios(string) (beadsapp.ScenarioValidation, error) {
	return fake.validation, nil
}

type fakeAcceptance struct {
	results []beadsapp.AcceptanceResult
	nonPass bool
}

func (fake fakeAcceptance) VerifyAcceptance([]string) ([]beadsapp.AcceptanceResult, bool, error) {
	return fake.results, fake.nonPass, nil
}

func (fake fakeHygiene) Audit(bool) (*beadsapp.AuditReport, error)     { return fake.audit, nil }
func (fake fakeHygiene) Cluster(bool) (*beadsapp.ClusterReport, error) { return fake.cluster, nil }

func (fake fakeKnowledge) Available() bool { return fake.available }
func (fake fakeKnowledge) Verify(context.Context, string) (*beadsapp.VerifyReport, error) {
	return fake.verified, nil
}
func (fake fakeKnowledge) Lint(context.Context, string) (*beadsapp.LintReport, error) {
	return fake.linted, nil
}
func (fake fakeKnowledge) Harvest(context.Context, string, string, bool) (beadsapp.HarvestResult, error) {
	return fake.harvested, nil
}

func (fake *fakeTrackerPorts) Resolve() (beadsapp.TrackerResolution, error) {
	return fake.resolution, nil
}
func (fake *fakeTrackerPorts) BRLedger() (beadsapp.LedgerResolution, error) {
	return fake.ledger, nil
}
func (fake *fakeTrackerPorts) BeadsDirOverride() bool { return fake.override }
func (fake *fakeTrackerPorts) InspectLedger(string) beadsapp.LedgerSnapshot {
	return fake.snapshot
}
func (fake *fakeTrackerPorts) Execute(_ context.Context, _ []string, streams beadsapp.ExecStreams) error {
	fake.executed = true
	_, _ = streams.Stdout.Write([]byte("executed\n"))
	return fake.execErr
}
func (fake *fakeTrackerPorts) ListInProgress(context.Context) ([]byte, error) {
	return fake.listOutput, nil
}
func (fake *fakeTrackerPorts) Show(_ context.Context, beadID string) (beadsapp.StaleBeadRecord, error) {
	fake.calls = append(fake.calls, "show:"+beadID)
	if fake.showIndex >= len(fake.shown) {
		return beadsapp.StaleBeadRecord{}, errors.New("missing shown record")
	}
	record := fake.shown[fake.showIndex]
	fake.showIndex++
	return record, nil
}
func (fake *fakeTrackerPorts) Claim(_ context.Context, beadID, agent string) error {
	fake.calls = append(fake.calls, "claim:"+beadID+":"+agent)
	return nil
}
func (fake *fakeTrackerPorts) Now() time.Time { return fake.now }
func (fake *fakeTrackerPorts) Actor() string  { return fake.actor }
func (fake *fakeTrackerPorts) ResolveRepoPath(path string) (string, error) {
	fake.calls = append(fake.calls, "resolve:"+path)
	return "/repo/" + path, nil
}
func (fake *fakeTrackerPorts) AppendEvent(path string, event any) error {
	fake.calls = append(fake.calls, "append:"+path)
	fake.appended = event
	return nil
}
func (fake *fakeTrackerPorts) ReadFile(path string) ([]byte, error) {
	fake.calls = append(fake.calls, "read:"+path)
	return fake.readOutput, nil
}

func TestModuleOwnsDirectoryTrackerAndExecHandlers(t *testing.T) {
	ports := &fakeTrackerPorts{
		resolution: beadsapp.TrackerResolution{Tracker: beadsapp.TrackerBD, Binary: "/bin/bd", LedgerDir: "/repo/.beads", Source: "ledger"},
		snapshot:   beadsapp.LedgerSnapshot{Exists: true, Directory: true, Readable: true, Entries: []string{"beads.db"}},
	}

	t.Run("directory JSON", func(t *testing.T) {
		var output bytes.Buffer
		root := NewModule(ports, ports, beadsapp.DirectoryService{Resolver: ports, Inspector: ports}, nil, nil, nil, nil, nil).Command()
		root.SetOut(&output)
		root.SetArgs([]string{"dir", "--require", "--json"})
		if err := root.Execute(); err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
		if got := output.String(); got != "{\"beads_dir\":\"/repo/.beads\",\"source\":\"ledger\"}\n" {
			t.Fatalf("output = %q", got)
		}
	})

	t.Run("tracker text", func(t *testing.T) {
		var output bytes.Buffer
		root := NewModule(ports, ports, beadsapp.DirectoryService{Resolver: ports, Inspector: ports}, nil, nil, nil, nil, nil).Command()
		root.SetOut(&output)
		root.SetArgs([]string{"tracker"})
		if err := root.Execute(); err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
		if got := output.String(); got != "tracker     bd\nbinary      /bin/bd\nledger_dir  /repo/.beads\nsource      ledger\n" {
			t.Fatalf("output = %q", got)
		}
	})

	t.Run("exec help precedes resolution", func(t *testing.T) {
		ports.executed = false
		root := NewModule(ports, ports, beadsapp.DirectoryService{Resolver: ports, Inspector: ports}, nil, nil, nil, nil, nil).Command()
		root.SetArgs([]string{"exec", "--help"})
		if err := root.Execute(); err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
		if ports.executed {
			t.Fatal("executor called for static help")
		}
	})

	t.Run("exec preserves typed exit", func(t *testing.T) {
		ports.executed = false
		ports.execErr = &beadsapp.ExitError{Code: 7}
		root := NewModule(ports, ports, beadsapp.DirectoryService{Resolver: ports, Inspector: ports}, nil, nil, nil, nil, nil).Command()
		root.SetArgs([]string{"exec", "close", "age-x"})
		err := root.Execute()
		var exitError *beadsapp.ExitError
		if !errors.As(err, &exitError) || exitError.ExitCode() != 7 {
			t.Fatalf("error = %v, want ExitError(7)", err)
		}
	})
}

func TestModuleOwnsRecoveryHandlers(t *testing.T) {
	now := time.Date(2026, 7, 11, 18, 0, 0, 0, time.UTC)
	ports := &fakeTrackerPorts{
		ledger: beadsapp.LedgerResolution{Path: "/ledger", Source: "env"},
		now:    now,
		actor:  "codex",
	}

	t.Run("stale claims", func(t *testing.T) {
		ports.listOutput = []byte(`[{"id":"age-old","status":"in_progress","assignee":"bo","updated_at":"2026-07-11T10:00:00Z"}]`)
		var output bytes.Buffer
		root := NewModule(ports, ports, beadsapp.DirectoryService{Resolver: ports, Inspector: ports}, beadsapp.RecoveryService{StaleSource: ports, Claims: ports, Runtime: ports, Resolver: ports, Reader: ports}, nil, nil, nil, nil).Command()
		root.SetOut(&output)
		root.SetArgs([]string{"stale-claims", "--threshold", "4", "--json"})
		if err := root.Execute(); err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
		if !bytes.Contains(output.Bytes(), []byte(`"bead_id":"age-old"`)) {
			t.Fatalf("output = %q", output.String())
		}
	})

	t.Run("resume ordered effects", func(t *testing.T) {
		ports.calls, ports.showIndex = nil, 0
		ports.shown = []beadsapp.StaleBeadRecord{
			{ID: "age-x", Status: "in_progress", Assignee: "old", UpdatedAt: "2026-07-11T10:00:00Z"},
			{ID: "age-x", Status: "in_progress", Assignee: "codex", UpdatedAt: "2026-07-11T18:00:00Z"},
		}
		root := NewModule(ports, ports, beadsapp.DirectoryService{Resolver: ports, Inspector: ports}, beadsapp.RecoveryService{StaleSource: ports, Claims: ports, Runtime: ports, Resolver: ports, Reader: ports}, nil, nil, nil, nil).Command()
		root.SetArgs([]string{"resume", "age-x", "--json"})
		if err := root.Execute(); err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
		want := []string{"show:age-x", "claim:age-x:codex", "show:age-x", "resolve:docs/provenance/ledger.jsonl", "append:/repo/docs/provenance/ledger.jsonl"}
		if !reflect.DeepEqual(ports.calls, want) {
			t.Fatalf("calls = %v, want %v", ports.calls, want)
		}
	})

	t.Run("epic terminal exit", func(t *testing.T) {
		ports.readOutput = []byte("{\"id\":\"age-e\",\"status\":\"open\",\"issue_type\":\"epic\"}\n{\"id\":\"age-e.1\",\"status\":\"open\",\"issue_type\":\"task\"}\n")
		root := NewModule(ports, ports, beadsapp.DirectoryService{Resolver: ports, Inspector: ports}, beadsapp.RecoveryService{StaleSource: ports, Claims: ports, Runtime: ports, Resolver: ports, Reader: ports}, nil, nil, nil, nil).Command()
		root.SetArgs([]string{"epic-status", "age-e", "--terminal"})
		err := root.Execute()
		var exitError *beadsapp.ExitError
		if !errors.As(err, &exitError) || exitError.ExitCode() != 2 {
			t.Fatalf("error = %v, want ExitError(2)", err)
		}
	})
}

func TestModuleOwnsKnowledgeHandlers(t *testing.T) {
	knowledge := fakeKnowledge{
		available: true,
		verified: &beadsapp.VerifyReport{
			BeadID: "age-x", Title: "stale", Status: "OPEN", TotalCount: 1, StaleCount: 1, BDAvailable: true,
			Citations: []beadsapp.Citation{{Kind: "file", Raw: "gone.go", Status: beadsapp.CitationStale, Reason: "missing"}},
		},
		linted:    &beadsapp.LintReport{StatusFilter: "open", TotalBeads: 1, StaleBeads: 1},
		harvested: beadsapp.HarvestResult{Body: "learning", Target: ".agents/learnings/age-x.md"},
	}

	t.Run("verify maps stale verdict", func(t *testing.T) {
		var output bytes.Buffer
		root := NewModule(nil, nil, nil, nil, knowledge, nil, nil, nil).Command()
		root.SetOut(&output)
		root.SetArgs([]string{"verify", "age-x"})
		err := root.Execute()
		var exitError *beadsapp.ExitError
		if !errors.As(err, &exitError) || exitError.ExitCode() != 1 {
			t.Fatalf("error = %v, want ExitError(1)", err)
		}
		if !bytes.Contains(output.Bytes(), []byte("[STALE] gone.go")) {
			t.Fatalf("output = %q", output.String())
		}
	})

	t.Run("lint maps stale verdict", func(t *testing.T) {
		root := NewModule(nil, nil, nil, nil, knowledge, nil, nil, nil).Command()
		root.SetArgs([]string{"lint"})
		err := root.Execute()
		var exitError *beadsapp.ExitError
		if !errors.As(err, &exitError) || exitError.ExitCode() != 1 {
			t.Fatalf("error = %v, want ExitError(1)", err)
		}
	})

	t.Run("harvest renders result", func(t *testing.T) {
		var output bytes.Buffer
		root := NewModule(nil, nil, nil, nil, knowledge, nil, nil, nil).Command()
		root.SetOut(&output)
		root.SetArgs([]string{"harvest", "age-x"})
		if err := root.Execute(); err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
		if got := output.String(); got != "harvested bead age-x → .agents/learnings/age-x.md\n" {
			t.Fatalf("output = %q", got)
		}
	})
}

func TestModuleOwnsHygieneHandlers(t *testing.T) {
	hygiene := fakeHygiene{
		audit: &beadsapp.AuditReport{
			BDAvailable: true,
			LikelyFixed: []beadsapp.AuditFinding{{ID: "age-fixed", Title: "fixed", Reason: "commit_match"}},
			Summary:     beadsapp.AuditSummary{Total: 1, LikelyFixed: 1},
		},
		cluster: &beadsapp.ClusterReport{
			BDAvailable: true,
			Clusters: []beadsapp.BeadCluster{{
				Representative: "age-e",
				SharedKeywords: []string{"tracker"},
				Beads:          []beadsapp.ClusterBead{{ID: "age-e", Title: "tracker epic", IsEpic: true}, {ID: "age-x", Title: "tracker task"}},
			}},
		},
	}

	t.Run("strict audit maps findings", func(t *testing.T) {
		var output bytes.Buffer
		root := NewModule(nil, nil, nil, nil, nil, hygiene, nil, nil).Command()
		root.SetOut(&output)
		root.SetArgs([]string{"audit", "--strict"})
		err := root.Execute()
		var exitError *beadsapp.ExitError
		if !errors.As(err, &exitError) || exitError.ExitCode() != 1 {
			t.Fatalf("error = %v, want ExitError(1)", err)
		}
		if !bytes.Contains(output.Bytes(), []byte("Likely fixed: age-fixed")) {
			t.Fatalf("output = %q", output.String())
		}
	})

	t.Run("cluster renders representative", func(t *testing.T) {
		var output bytes.Buffer
		root := NewModule(nil, nil, nil, nil, nil, hygiene, nil, nil).Command()
		root.SetOut(&output)
		root.SetArgs([]string{"cluster"})
		if err := root.Execute(); err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
		if !bytes.Contains(output.Bytes(), []byte("Consolidate under age-e (existing epic)")) {
			t.Fatalf("output = %q", output.String())
		}
	})
}

func TestModuleOwnsScenarioAndAcceptanceHandlers(t *testing.T) {
	scenarioUseCases := &fakeScenario{
		extraction: beadsapp.ScenarioExtraction{
			BeadID:    "age-x",
			Scenarios: []scenarios.Scenario{{Name: "works", Given: "state", When: "action", Then: "result"}},
		},
		validation: beadsapp.ScenarioValidation{BeadID: "age-x", Valid: true, Scenarios: []scenarios.Scenario{{Name: "works"}}},
	}

	t.Run("write prompts before update and decline is safe", func(t *testing.T) {
		scenarioUseCases.applied = 0
		var prompt bytes.Buffer
		root := NewModule(nil, nil, nil, nil, nil, nil, scenarioUseCases, nil).Command()
		root.SetErr(&prompt)
		root.SetIn(strings.NewReader("n\n"))
		root.SetArgs([]string{"scenarios", "extract", "age-x", "--write"})
		if err := root.Execute(); err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
		if scenarioUseCases.applied != 0 || !bytes.Contains(prompt.Bytes(), []byte("Proceed? [y/N]")) {
			t.Fatalf("applied=%d prompt=%q", scenarioUseCases.applied, prompt.String())
		}
	})

	t.Run("write confirms update", func(t *testing.T) {
		scenarioUseCases.applied = 0
		root := NewModule(nil, nil, nil, nil, nil, nil, scenarioUseCases, nil).Command()
		root.SetIn(strings.NewReader("yes\n"))
		root.SetArgs([]string{"scenarios", "extract", "age-x", "--write"})
		if err := root.Execute(); err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
		if scenarioUseCases.applied != 1 {
			t.Fatalf("applied = %d, want 1", scenarioUseCases.applied)
		}
	})

	t.Run("strict acceptance maps non-pass", func(t *testing.T) {
		acceptance := fakeAcceptance{
			results: []beadsapp.AcceptanceResult{{BeadID: "age-x", IssueType: "feature", Verdict: beadsapp.AcceptanceFail, Missing: []string{"TDD signal"}}},
			nonPass: true,
		}
		root := NewModule(nil, nil, nil, nil, nil, nil, nil, acceptance).Command()
		root.SetArgs([]string{"verify-acceptance", "age-x", "--strict"})
		err := root.Execute()
		var exitError *beadsapp.ExitError
		if !errors.As(err, &exitError) || exitError.ExitCode() != 1 {
			t.Fatalf("error = %v, want ExitError(1)", err)
		}
	})
}

func TestLegacyProductionOwnersAreAbsent(t *testing.T) {
	commandDirectory := filepath.Join("..", "..", "..", "cmd", "ao")
	entries, err := os.ReadDir(commandDirectory)
	if err != nil {
		t.Fatal(err)
	}
	allowed := map[string]bool{
		"beads_composition.go":     true,
		"beads_citation_compat.go": true, // frozen citation-family adapter bridge
		"beads_exec.go":            true, // yield-family compatibility delegates
		"beads_json_compat.go":     true, // lookup compatibility helper
		"beads_module.go":          true,
	}
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, "beads") || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		if !allowed[name] {
			t.Errorf("unexpected production beads owner remains: %s", name)
		}
		content, readErr := os.ReadFile(filepath.Join(commandDirectory, name))
		if readErr != nil {
			t.Fatal(readErr)
		}
		for _, forbidden := range []string{"executeBeads", "beadsModuleRunner", "beadsTrackerOutput", "beadsTrackerAvailable", "beadsResumeAgentID", "beadsAuditJSON"} {
			if strings.Contains(string(content), forbidden) {
				t.Errorf("%s retains forbidden legacy declaration %q", name, forbidden)
			}
		}
	}
}
