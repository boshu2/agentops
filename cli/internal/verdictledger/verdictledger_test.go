package verdictledger

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// repoRoot returns the absolute repo root so tests can reference tracked
// fixtures and the schema without embedding duplicates.
func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// file = .../cli/internal/verdictledger/verdictledger_test.go
	// climb: verdictledger/ -> internal/ -> cli/ -> repo root
	return filepath.Join(filepath.Dir(file), "..", "..", "..")
}

// fixturePath resolves a tracked fixture under tests/fixtures/verdict-ledger.
func fixturePath(t *testing.T, name string) string {
	t.Helper()
	return filepath.Join(repoRoot(t), "tests", "fixtures", "verdict-ledger", name)
}

// TestLoadPath_Fixtures verifies every tracked fixture loads through the
// structural validator without error and carries the expected record count.
func TestLoadPath_Fixtures(t *testing.T) {
	cases := []struct {
		fixture     string
		wantRecords int
	}{
		{"failure-streak.json", 3},
		{"pass-resets-streak.json", 4},
		{"cooldown.json", 6},
		{"multi-iteration.json", 6},
	}
	for _, tc := range cases {
		t.Run(tc.fixture, func(t *testing.T) {
			ledger, err := LoadPath(fixturePath(t, tc.fixture))
			if err != nil {
				t.Fatalf("LoadPath(%s) error: %v", tc.fixture, err)
			}
			if ledger.SchemaVersion != SchemaVersion {
				t.Errorf("SchemaVersion = %q, want %q", ledger.SchemaVersion, SchemaVersion)
			}
			if len(ledger.Records) != tc.wantRecords {
				t.Errorf("len(Records) = %d, want %d", len(ledger.Records), tc.wantRecords)
			}
		})
	}
}

// TestFailureStreak verifies streak accounting across fixtures: a 3-fail run,
// a pass that resets the streak, and an unknown that breaks a streak.
func TestFailureStreak(t *testing.T) {
	cases := []struct {
		fixture     string
		directiveID string
		wantStreak  int
	}{
		{"failure-streak.json", "d-reduce-flaky-tests", 3},
		{"pass-resets-streak.json", "d-improve-coverage", 1},
		{"cooldown.json", "d-cut-build-time", 1},
		{"multi-iteration.json", "d-harden-auth", 3},
		{"multi-iteration.json", "d-reduce-latency", 0},
	}
	for _, tc := range cases {
		t.Run(tc.fixture+"/"+tc.directiveID, func(t *testing.T) {
			ledger, err := LoadPath(fixturePath(t, tc.fixture))
			if err != nil {
				t.Fatalf("LoadPath error: %v", err)
			}
			got := ledger.FailureStreak(tc.directiveID)
			if got != tc.wantStreak {
				t.Errorf("FailureStreak(%q) = %d, want %d", tc.directiveID, got, tc.wantStreak)
			}
		})
	}
}

// TestIterationCount verifies cooldown records are excluded from the evidence
// count and that per-directive counts are exact.
func TestIterationCount(t *testing.T) {
	cases := []struct {
		fixture     string
		directiveID string
		wantCount   int
	}{
		{"cooldown.json", "d-cut-build-time", 5},
		{"multi-iteration.json", "d-reduce-latency", 3},
		{"multi-iteration.json", "d-harden-auth", 3},
		{"failure-streak.json", "d-missing", 0},
	}
	for _, tc := range cases {
		t.Run(tc.fixture+"/"+tc.directiveID, func(t *testing.T) {
			ledger, err := LoadPath(fixturePath(t, tc.fixture))
			if err != nil {
				t.Fatalf("LoadPath error: %v", err)
			}
			got := ledger.IterationCount(tc.directiveID)
			if got != tc.wantCount {
				t.Errorf("IterationCount(%q) = %d, want %d", tc.directiveID, got, tc.wantCount)
			}
		})
	}
}

// TestInCooldown verifies the cooldown window is measured in iteration records.
func TestInCooldown(t *testing.T) {
	ledger, err := LoadPath(fixturePath(t, "cooldown.json"))
	if err != nil {
		t.Fatalf("LoadPath error: %v", err)
	}
	// The proposal sits between the 3rd and 4th iteration; window of 3 covers
	// iterations 3,4,5 (the proposal precedes iteration 4) -> in cooldown.
	if !ledger.InCooldown("d-cut-build-time", 3) {
		t.Error("InCooldown(d-cut-build-time, 3) = false, want true")
	}
	// Window of 1 covers only the last iteration (12:00), which is after the
	// proposal (10:05) -> proposal is outside the window.
	if ledger.InCooldown("d-cut-build-time", 1) {
		t.Error("InCooldown(d-cut-build-time, 1) = true, want false")
	}
	// A directive with no cooldown records is never in cooldown.
	if ledger.InCooldown("d-missing", 5) {
		t.Error("InCooldown(d-missing, 5) = true, want false")
	}
}

// TestIterationsFor verifies records are returned in append (oldest-first)
// order and filtered to the directive.
func TestIterationsFor(t *testing.T) {
	ledger, err := LoadPath(fixturePath(t, "multi-iteration.json"))
	if err != nil {
		t.Fatalf("LoadPath error: %v", err)
	}
	got := ledger.IterationsFor("d-reduce-latency")
	wantTimes := []string{
		"2026-05-17T06:00:00Z",
		"2026-05-17T07:00:00Z",
		"2026-05-17T08:00:00Z",
	}
	if len(got) != len(wantTimes) {
		t.Fatalf("len(IterationsFor) = %d, want %d", len(got), len(wantTimes))
	}
	for i, want := range wantTimes {
		if got[i].RunTime != want {
			t.Errorf("record[%d].RunTime = %q, want %q", i, got[i].RunTime, want)
		}
		if got[i].DirectiveID != "d-reduce-latency" {
			t.Errorf("record[%d].DirectiveID = %q, want d-reduce-latency", i, got[i].DirectiveID)
		}
	}
}

// TestValidateRecord verifies structural rejection of malformed records.
func TestValidateRecord(t *testing.T) {
	good := 0.7
	cnt := 3
	cases := []struct {
		name    string
		record  Record
		wantBad bool
	}{
		{
			name: "valid iteration",
			record: Record{RecordType: RecordIteration, DirectiveID: "d-x",
				RunTime: "2026-05-17T12:00:00Z", ScenarioVerdict: VerdictFail,
				ScenarioSatisfaction: &good, ScenarioCount: &cnt, EvaluatedCount: &cnt},
			wantBad: false,
		},
		{
			name: "valid cooldown",
			record: Record{RecordType: RecordCooldown, DirectiveID: "d-x",
				RunTime: "2026-05-17T12:00:00Z", CooldownKind: CooldownApplied,
				MutationType: MutationPriorityBump},
			wantBad: false,
		},
		{
			name:    "bad directive id",
			record:  Record{RecordType: RecordIteration, DirectiveID: "X", RunTime: "2026-05-17T12:00:00Z"},
			wantBad: true,
		},
		{
			name: "bad verdict",
			record: Record{RecordType: RecordIteration, DirectiveID: "d-x",
				RunTime: "2026-05-17T12:00:00Z", ScenarioVerdict: "maybe",
				ScenarioSatisfaction: &good, ScenarioCount: &cnt, EvaluatedCount: &cnt},
			wantBad: true,
		},
		{
			name: "missing scenario_count",
			record: Record{RecordType: RecordIteration, DirectiveID: "d-x",
				RunTime: "2026-05-17T12:00:00Z", ScenarioVerdict: VerdictPass,
				ScenarioSatisfaction: &good, EvaluatedCount: &cnt},
			wantBad: true,
		},
		{
			name:    "unknown record type",
			record:  Record{RecordType: "bogus", DirectiveID: "d-x", RunTime: "2026-05-17T12:00:00Z"},
			wantBad: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			defect := validateRecord(tc.record)
			if tc.wantBad && defect == "" {
				t.Error("validateRecord accepted a malformed record, want defect")
			}
			if !tc.wantBad && defect != "" {
				t.Errorf("validateRecord rejected a valid record: %s", defect)
			}
		})
	}
}

// TestWriterAppendsWithoutClobber verifies the writer preserves prior records
// and appends in order, writing atomically.
func TestWriterAppendsWithoutClobber(t *testing.T) {
	root := t.TempDir()
	at := func(h int) time.Time {
		return time.Date(2026, 5, 17, h, 0, 0, 0, time.UTC)
	}
	w := Writer{Now: func() time.Time { return at(20) }}

	first, err := w.AppendIteration(root, IterationInput{
		DirectiveID: "d-x", RunTime: at(10), ScenarioVerdict: VerdictFail,
		ScenarioSatisfaction: 0.4, ScenarioCount: 3, EvaluatedCount: 3, RunID: "r1",
	})
	if err != nil {
		t.Fatalf("AppendIteration first: %v", err)
	}
	if len(first.Records) != 1 {
		t.Fatalf("after first append len(Records) = %d, want 1", len(first.Records))
	}

	second, err := w.AppendIteration(root, IterationInput{
		DirectiveID: "d-x", RunTime: at(11), ScenarioVerdict: VerdictFail,
		ScenarioSatisfaction: 0.42, ScenarioCount: 3, EvaluatedCount: 3, RunID: "r2",
	})
	if err != nil {
		t.Fatalf("AppendIteration second: %v", err)
	}
	if len(second.Records) != 2 {
		t.Fatalf("after second append len(Records) = %d, want 2", len(second.Records))
	}

	third, err := w.AppendCooldown(root, CooldownInput{
		DirectiveID: "d-x", RunTime: at(12), CooldownKind: CooldownProposed,
		MutationType: MutationSetpointLoosen, Note: "streak of 2",
	})
	if err != nil {
		t.Fatalf("AppendCooldown: %v", err)
	}
	if len(third.Records) != 3 {
		t.Fatalf("after cooldown append len(Records) = %d, want 3", len(third.Records))
	}

	// Reload from disk: prior records must survive every append.
	loaded, err := Load(root)
	if err != nil {
		t.Fatalf("Load after appends: %v", err)
	}
	if len(loaded.Records) != 3 {
		t.Fatalf("reloaded len(Records) = %d, want 3", len(loaded.Records))
	}
	if loaded.Records[0].RunTime != "2026-05-17T10:00:00Z" {
		t.Errorf("Records[0].RunTime = %q, want 2026-05-17T10:00:00Z", loaded.Records[0].RunTime)
	}
	if loaded.Records[2].RecordType != RecordCooldown {
		t.Errorf("Records[2].RecordType = %q, want cooldown", loaded.Records[2].RecordType)
	}
	if loaded.FailureStreak("d-x") != 2 {
		t.Errorf("FailureStreak(d-x) = %d, want 2", loaded.FailureStreak("d-x"))
	}
	if !loaded.InCooldown("d-x", 5) {
		t.Error("InCooldown(d-x, 5) = false, want true after cooldown append")
	}

	// No leftover tmp file from atomic write.
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(ArtifactRelPath)) + ".tmp"); !os.IsNotExist(err) {
		t.Error("leftover .tmp file after atomic write")
	}
}

// TestLoadMissingLedger verifies a missing ledger is an empty ledger, no error.
func TestLoadMissingLedger(t *testing.T) {
	ledger, err := Load(t.TempDir())
	if err != nil {
		t.Fatalf("Load(missing) error: %v", err)
	}
	if len(ledger.Records) != 0 {
		t.Errorf("len(Records) = %d, want 0", len(ledger.Records))
	}
	if ledger.FailureStreak("d-x") != 0 {
		t.Errorf("FailureStreak on empty ledger = %d, want 0", ledger.FailureStreak("d-x"))
	}
}

// TestSchemaDeclaresADRFields verifies the tracked schema enumerates the exact
// ADR-0006 required fields for an iteration record, guarding against drift
// between the schema contract and the Go struct.
func TestSchemaDeclaresADRFields(t *testing.T) {
	schemaPath := filepath.Join(repoRoot(t), "schemas", "verdict-ledger.v1.schema.json")
	data, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	var schema struct {
		Defs map[string]struct {
			Required             []string                   `json:"required"`
			AdditionalProperties *bool                      `json:"additionalProperties"`
			Properties           map[string]json.RawMessage `json:"properties"`
		} `json:"$defs"`
	}
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatalf("parse schema: %v", err)
	}
	iter, ok := schema.Defs["iteration_record"]
	if !ok {
		t.Fatal("schema missing $defs.iteration_record")
	}
	if iter.AdditionalProperties == nil || *iter.AdditionalProperties {
		t.Error("iteration_record must set additionalProperties:false")
	}
	wantRequired := map[string]bool{
		"record_type": true, "directive_id": true, "run_timestamp": true,
		"scenario_verdict": true, "scenario_satisfaction": true,
		"scenario_count": true, "evaluated_count": true,
	}
	gotRequired := map[string]bool{}
	for _, f := range iter.Required {
		gotRequired[f] = true
	}
	for f := range wantRequired {
		if !gotRequired[f] {
			t.Errorf("iteration_record schema missing required field %q", f)
		}
	}
	if len(gotRequired) != len(wantRequired) {
		t.Errorf("iteration_record required count = %d, want %d", len(gotRequired), len(wantRequired))
	}
}
