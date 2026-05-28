// practices: [snapshot-testing, llm-eval-harness]
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/boshu2/agentops/cli/internal/ratchet"
)

var updateGolden = flag.Bool("update-golden", false, "update golden files")

// goldenTestdataDir returns the absolute path to testdata/golden/ relative to
// this test file's source location. Using runtime.Caller ensures the path is
// correct even when chdirTemp has changed the working directory.
func goldenTestdataDir() string {
	_, thisFile, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(thisFile), "testdata", "golden")
}

// goldenTest compares got against a golden file. When -update-golden is set,
// the golden file is created/overwritten instead of compared.
//
// Usage:
//
//	go test -run TestGolden -update-golden   # create/refresh golden files
//	go test -run TestGolden                  # regression check
func goldenTest(t *testing.T, name string, got []byte) {
	t.Helper()
	golden := filepath.Join(goldenTestdataDir(), name)
	if *updateGolden {
		if err := os.MkdirAll(filepath.Dir(golden), 0755); err != nil {
			t.Fatalf("mkdir for golden: %v", err)
		}
		if err := os.WriteFile(golden, got, 0644); err != nil {
			t.Fatalf("update golden: %v", err)
		}
		return
	}
	expected, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("read golden file %s: %v (run with -update-golden to create)", golden, err)
	}
	if diff := cmp.Diff(string(expected), string(got)); diff != "" {
		t.Errorf("output mismatch (-want +got):\n%s", diff)
	}
}

// ---------------------------------------------------------------------------
// Test 1: Ratchet status JSON output (empty chain)
// ---------------------------------------------------------------------------

func TestGoldenRatchetStatusJSON_EmptyChain(t *testing.T) {
	data := &ratchetStatusOutput{
		ChainID: "golden-test-001",
		Started: "2025-06-15T08:00:00Z",
		EpicID:  "",
		Path:    "/tmp/test/.agents/ao/chain.jsonl",
		Steps:   make([]ratchetStepInfo, 0),
	}

	for _, step := range ratchet.AllSteps() {
		data.Steps = append(data.Steps, ratchetStepInfo{
			Step:   step,
			Status: ratchet.StatusPending,
		})
	}

	origOutput := output
	output = "json"
	defer func() { output = origOutput }()

	var buf bytes.Buffer
	if err := outputRatchetStatus(&buf, data); err != nil {
		t.Fatalf("outputRatchetStatus: %v", err)
	}

	goldenTest(t, "ratchet-status-empty.json", buf.Bytes())
}

// ---------------------------------------------------------------------------
// Test 2: Ratchet status JSON output (partially completed chain)
// ---------------------------------------------------------------------------

func TestGoldenRatchetStatusJSON_PartialChain(t *testing.T) {
	data := &ratchetStatusOutput{
		ChainID: "golden-test-002",
		Started: "2025-06-15T09:00:00Z",
		EpicID:  "ag-golden",
		Path:    "/tmp/test/.agents/ao/chain.jsonl",
		Steps: []ratchetStepInfo{
			{Step: ratchet.StepResearch, Status: ratchet.StatusLocked, Output: "findings.md", Time: "2025-06-15T09:10:00Z"},
			{Step: ratchet.StepPreMortem, Status: ratchet.StatusLocked, Output: "pre-mortem.md", Time: "2025-06-15T09:20:00Z"},
			{Step: ratchet.StepPlan, Status: ratchet.StatusInProgress, Input: "findings.md"},
			{Step: ratchet.StepImplement, Status: ratchet.StatusPending},
			{Step: ratchet.StepCrank, Status: ratchet.StatusPending},
			{Step: ratchet.StepVibe, Status: ratchet.StatusPending},
			{Step: ratchet.StepPostMortem, Status: ratchet.StatusPending},
		},
	}

	origOutput := output
	output = "json"
	defer func() { output = origOutput }()

	var buf bytes.Buffer
	if err := outputRatchetStatus(&buf, data); err != nil {
		t.Fatalf("outputRatchetStatus: %v", err)
	}

	goldenTest(t, "ratchet-status-partial.json", buf.Bytes())
}

// ---------------------------------------------------------------------------
// Test 3: Ratchet status table output
// ---------------------------------------------------------------------------

func TestGoldenRatchetStatusTable(t *testing.T) {
	data := &ratchetStatusOutput{
		ChainID: "golden-test-003",
		Started: "2025-06-15T10:00:00Z",
		EpicID:  "ag-table",
		Path:    "/tmp/test/.agents/ao/chain.jsonl",
		Steps: []ratchetStepInfo{
			{Step: ratchet.StepResearch, Status: ratchet.StatusLocked, Output: "findings.md"},
			{Step: ratchet.StepPreMortem, Status: ratchet.StatusSkipped},
			{Step: ratchet.StepPlan, Status: ratchet.StatusLocked, Output: "plan.md"},
			{Step: ratchet.StepImplement, Status: ratchet.StatusInProgress},
			{Step: ratchet.StepCrank, Status: ratchet.StatusPending},
			{Step: ratchet.StepVibe, Status: ratchet.StatusPending},
			{Step: ratchet.StepPostMortem, Status: ratchet.StatusPending},
		},
	}

	origOutput := output
	output = "table"
	defer func() { output = origOutput }()

	var buf bytes.Buffer
	if err := outputRatchetStatus(&buf, data); err != nil {
		t.Fatalf("outputRatchetStatus: %v", err)
	}

	goldenTest(t, "ratchet-status-table.txt", buf.Bytes())
}

// ---------------------------------------------------------------------------
// Test 4: Ratchet next JSON — no steps completed
// ---------------------------------------------------------------------------

func TestGoldenRatchetNextJSON_NoSteps(t *testing.T) {
	chain := &ratchet.Chain{
		ID:      "golden-next-001",
		Started: time.Date(2025, 6, 15, 8, 0, 0, 0, time.UTC),
		Entries: []ratchet.ChainEntry{},
	}

	result := computeNextStep(chain)

	origOutput := output
	output = "json"
	defer func() { output = origOutput }()

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	data = append(data, '\n')

	goldenTest(t, "ratchet-next-empty.json", data)
}

// ---------------------------------------------------------------------------
// Test 5: Ratchet next JSON — research locked
// ---------------------------------------------------------------------------

func TestGoldenRatchetNextJSON_ResearchLocked(t *testing.T) {
	chain := &ratchet.Chain{
		ID:      "golden-next-002",
		Started: time.Date(2025, 6, 15, 8, 0, 0, 0, time.UTC),
		Entries: []ratchet.ChainEntry{
			{
				Step:      ratchet.StepResearch,
				Timestamp: time.Date(2025, 6, 15, 8, 30, 0, 0, time.UTC),
				Output:    "research-findings.md",
				Locked:    true,
			},
		},
	}

	result := computeNextStep(chain)

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	data = append(data, '\n')

	goldenTest(t, "ratchet-next-research-locked.json", data)
}

// ---------------------------------------------------------------------------
// Test 6: Ratchet next JSON — all complete
// ---------------------------------------------------------------------------

func TestGoldenRatchetNextJSON_AllComplete(t *testing.T) {
	chain := &ratchet.Chain{
		ID:      "golden-next-003",
		Started: time.Date(2025, 6, 15, 8, 0, 0, 0, time.UTC),
		Entries: []ratchet.ChainEntry{
			{Step: ratchet.StepResearch, Timestamp: time.Date(2025, 6, 15, 8, 10, 0, 0, time.UTC), Output: "findings.md", Locked: true},
			{Step: ratchet.StepPreMortem, Timestamp: time.Date(2025, 6, 15, 8, 20, 0, 0, time.UTC), Output: "pre-mortem.md", Locked: true},
			{Step: ratchet.StepPlan, Timestamp: time.Date(2025, 6, 15, 8, 30, 0, 0, time.UTC), Output: "plan.md", Locked: true},
			{Step: ratchet.StepImplement, Timestamp: time.Date(2025, 6, 15, 9, 0, 0, 0, time.UTC), Output: "code-changes", Locked: true},
			{Step: ratchet.StepVibe, Timestamp: time.Date(2025, 6, 15, 9, 30, 0, 0, time.UTC), Output: "vibe-report.md", Locked: true},
			{Step: ratchet.StepPostMortem, Timestamp: time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC), Output: "retro.md", Locked: true},
		},
	}

	result := computeNextStep(chain)

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	data = append(data, '\n')

	goldenTest(t, "ratchet-next-all-complete.json", data)
}

// ---------------------------------------------------------------------------
// Test 7: Doctor output — all pass (synthetic checks)
// ---------------------------------------------------------------------------

func TestGoldenDoctorTableAllPass(t *testing.T) {
	checks := []doctorCheck{
		{Name: "ao CLI", Status: "pass", Detail: "v2.10.0", Required: true},
		{Name: "CLI Dependencies", Status: "pass", Detail: "gt and bd available", Required: false},
		{Name: "Hook Coverage", Status: "pass", Detail: "Full coverage: 5/5 events", Required: false},
		{Name: "Knowledge Base", Status: "pass", Detail: ".agents/ao initialized", Required: true},
		{Name: "Knowledge Freshness", Status: "pass", Detail: "Last session: 2h ago", Required: false},
		{Name: "Search Index", Status: "pass", Detail: "Index exists (1,247 terms)", Required: false},
		{Name: "Flywheel Health", Status: "pass", Detail: "12 learnings (3 established)", Required: false},
		{Name: "Plugin", Status: "pass", Detail: "18 skills found", Required: false},
	}
	result := computeResult(checks)

	var buf bytes.Buffer
	renderDoctorTable(&buf, result)

	goldenTest(t, "doctor-all-pass.txt", buf.Bytes())
}

// ---------------------------------------------------------------------------
// Test 8: Doctor output — mixed results
// ---------------------------------------------------------------------------

func TestGoldenDoctorTableMixed(t *testing.T) {
	checks := []doctorCheck{
		{Name: "ao CLI", Status: "pass", Detail: "v2.10.0", Required: true},
		{Name: "CLI Dependencies", Status: "warn", Detail: "bd not found — install with 'brew install beads'", Required: false},
		{Name: "Hook Coverage", Status: "warn", Detail: "Partial coverage: 3/5 events — run 'ao hooks install --force'", Required: false},
		{Name: "Knowledge Base", Status: "fail", Detail: ".agents/ao not initialized", Required: true},
		{Name: "Flywheel Health", Status: "warn", Detail: "No learnings found — the flywheel hasn't started", Required: false},
	}
	result := computeResult(checks)

	var buf bytes.Buffer
	renderDoctorTable(&buf, result)

	goldenTest(t, "doctor-mixed.txt", buf.Bytes())
}

// ---------------------------------------------------------------------------
// Test 9: Doctor JSON output
// ---------------------------------------------------------------------------

func TestGoldenDoctorJSON(t *testing.T) {
	checks := []doctorCheck{
		{Name: "ao CLI", Status: "pass", Detail: "v2.10.0", Required: true},
		{Name: "Knowledge Base", Status: "fail", Detail: ".agents/ao not initialized", Required: true},
	}
	result := computeResult(checks)

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	data = append(data, '\n')

	goldenTest(t, "doctor-output.json", data)
}

// ---------------------------------------------------------------------------
// Test 10: Ratchet status with cycle and parent epic
// ---------------------------------------------------------------------------

func TestGoldenRatchetStatusTable_WithCycle(t *testing.T) {
	data := &ratchetStatusOutput{
		ChainID: "golden-cycle-001",
		Started: "2025-06-15T10:00:00Z",
		EpicID:  "ag-cycle",
		Path:    "/tmp/test/.agents/ao/chain.jsonl",
		Steps: []ratchetStepInfo{
			{Step: ratchet.StepResearch, Status: ratchet.StatusLocked, Output: "findings.md", Cycle: 2, ParentEpic: "ag-parent"},
			{Step: ratchet.StepPreMortem, Status: ratchet.StatusLocked, Output: "pre-mortem.md", Cycle: 2, ParentEpic: "ag-parent"},
			{Step: ratchet.StepPlan, Status: ratchet.StatusPending},
			{Step: ratchet.StepImplement, Status: ratchet.StatusPending},
			{Step: ratchet.StepCrank, Status: ratchet.StatusPending},
			{Step: ratchet.StepVibe, Status: ratchet.StatusPending},
			{Step: ratchet.StepPostMortem, Status: ratchet.StatusPending},
		},
	}

	origOutput := output
	output = "table"
	defer func() { output = origOutput }()

	var buf bytes.Buffer
	if err := outputRatchetStatus(&buf, data); err != nil {
		t.Fatalf("outputRatchetStatus: %v", err)
	}

	goldenTest(t, "ratchet-status-cycle.txt", buf.Bytes())
}

// ---------------------------------------------------------------------------
// Test 11: Ratchet next JSON — plan locked (should suggest implement)
// ---------------------------------------------------------------------------

func TestGoldenRatchetNextJSON_PlanLocked(t *testing.T) {
	chain := &ratchet.Chain{
		ID:      "golden-next-004",
		Started: time.Date(2025, 6, 15, 8, 0, 0, 0, time.UTC),
		Entries: []ratchet.ChainEntry{
			{Step: ratchet.StepResearch, Timestamp: time.Date(2025, 6, 15, 8, 10, 0, 0, time.UTC), Output: "findings.md", Locked: true},
			{Step: ratchet.StepPreMortem, Timestamp: time.Date(2025, 6, 15, 8, 20, 0, 0, time.UTC), Output: "pre-mortem.md", Locked: true},
			{Step: ratchet.StepPlan, Timestamp: time.Date(2025, 6, 15, 8, 30, 0, 0, time.UTC), Output: "plan.md", Locked: true},
		},
	}

	result := computeNextStep(chain)

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	data = append(data, '\n')

	goldenTest(t, "ratchet-next-plan-locked.json", data)
}
