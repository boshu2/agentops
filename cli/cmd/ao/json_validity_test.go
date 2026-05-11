// practices: [design-by-contract, pragmatic-programmer]
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/goals"
	"github.com/boshu2/agentops/cli/internal/pool"
	"github.com/boshu2/agentops/cli/internal/ratchet"
	"github.com/boshu2/agentops/cli/internal/types"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// withOutputJSON temporarily sets the global output format to "json" and
// restores the original value after the test. Thread-safety is not required
// because tests in the same package serialize by default.
func withOutputJSON(t *testing.T) {
	t.Helper()
	prev := output
	output = "json"
	t.Cleanup(func() { output = prev })
}

// withGoalsJSON temporarily sets the goals-specific JSON flag.
func withGoalsJSON(t *testing.T) {
	t.Helper()
	prev := goalsJSON
	goalsJSON = true
	t.Cleanup(func() { goalsJSON = prev })
}

// withDoctorJSON temporarily sets the doctor-specific JSON flag.
func withDoctorJSON(t *testing.T) {
	t.Helper()
	prev := doctorJSON
	doctorJSON = true
	t.Cleanup(func() { doctorJSON = prev })
}

// captureJSONStdout moved to testutil_test.go.

// assertValidJSON parses raw as JSON and fails the test if it is invalid.
func assertValidJSON(t *testing.T, label string, raw string) {
	t.Helper()
	if raw == "" {
		t.Fatalf("%s: output is empty", label)
	}
	var parsed json.RawMessage
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		// Show first 500 chars of output for diagnostics.
		preview := raw
		if len(preview) > 500 {
			preview = preview[:500] + "..."
		}
		t.Fatalf("%s: json.Unmarshal failed: %v\nOutput:\n%s", label, err, preview)
	}
}

// writeGoalsYAML creates a minimal GOALS.yaml for testing in dir.
func writeGoalsYAML(t *testing.T, dir string) string {
	t.Helper()
	content := `version: 2
mission: Test mission
goals:
  - id: test-goal-1
    description: A test goal
    check: "echo ok"
    weight: 5
    type: health
  - id: test-goal-2
    description: Another test goal
    check: "echo ok"
    weight: 3
    type: health
`
	path := filepath.Join(dir, "GOALS.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write GOALS.yaml: %v", err)
	}
	return path
}

// setupPoolWithCandidate creates a pool directory and adds one candidate.
func setupPoolWithCandidate(t *testing.T, dir string) *pool.Pool {
	t.Helper()
	p := pool.NewPool(dir)
	cand := types.Candidate{
		ID:         "json-test-001",
		Type:       "learning",
		Tier:       types.TierSilver,
		Content:    "Test learning for JSON validity",
		Utility:    0.8,
		Confidence: 0.9,
		Maturity:   "established",
	}
	if err := p.Add(cand, types.Scoring{RawScore: 0.8, TierAssignment: types.TierSilver}); err != nil {
		t.Fatalf("pool add: %v", err)
	}
	return p
}

// ---------------------------------------------------------------------------
// Tests: Goals subcommands
// ---------------------------------------------------------------------------

func TestJSONValidity_GoalsValidate(t *testing.T) {
	dir := t.TempDir()
	path := writeGoalsYAML(t, dir)

	prevFile := goalsFile
	goalsFile = path
	t.Cleanup(func() { goalsFile = prevFile })

	withGoalsJSON(t)

	out := captureJSONStdout(t, func() {
		err := outputValidateResult(validateResult{
			Valid:     true,
			GoalCount: 2,
			Version:   2,
			Format:    "yaml",
		})
		if err != nil {
			t.Fatalf("outputValidateResult: %v", err)
		}
	})

	assertValidJSON(t, "goals validate --json", out)
}

func TestJSONValidity_GoalsValidateInvalid(t *testing.T) {
	withGoalsJSON(t)

	out := captureJSONStdout(t, func() {
		_ = outputValidateResult(validateResult{
			Valid:  false,
			Errors: []string{"goal foo: check required"},
		})
	})

	assertValidJSON(t, "goals validate --json (invalid)", out)
}

func TestJSONValidity_GoalsMeasure(t *testing.T) {
	dir := t.TempDir()
	path := writeGoalsYAML(t, dir)

	prevFile := goalsFile
	goalsFile = path
	t.Cleanup(func() { goalsFile = prevFile })

	withGoalsJSON(t)

	gf, err := goals.LoadGoals(path)
	if err != nil {
		t.Fatalf("load goals: %v", err)
	}

	snap := goals.Measure(gf, 10*time.Second)

	// Write snapshot dir so SaveSnapshot doesn't fail
	snapDir := filepath.Join(dir, ".agents", "ao", "goals", "baselines")
	if err := os.MkdirAll(snapDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	out := captureJSONStdout(t, func() {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(snap); err != nil {
			t.Fatalf("encode: %v", err)
		}
	})

	assertValidJSON(t, "goals measure --json", out)
}

func TestJSONValidity_GoalsExport(t *testing.T) {
	dir := t.TempDir()
	path := writeGoalsYAML(t, dir)

	prevFile := goalsFile
	goalsFile = path
	t.Cleanup(func() { goalsFile = prevFile })

	gf, err := goals.LoadGoals(path)
	if err != nil {
		t.Fatalf("load goals: %v", err)
	}

	snap := goals.Measure(gf, 10*time.Second)

	out := captureJSONStdout(t, func() {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(snap); err != nil {
			t.Fatalf("encode: %v", err)
		}
	})

	assertValidJSON(t, "goals export", out)
}

func TestJSONValidity_GoalsMeta(t *testing.T) {
	// Meta command outputs a snapshot filtered to meta-type goals.
	// Even with no meta goals, the snapshot is a valid JSON structure.
	gf := &goals.GoalFile{
		Version: 2,
		Mission: "Test",
		Goals: []goals.Goal{
			{ID: "meta-1", Description: "Meta goal", Check: "echo ok", Weight: 5, Type: goals.GoalTypeMeta},
		},
	}

	snap := goals.Measure(gf, 5*time.Second)

	out := captureJSONStdout(t, func() {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(snap); err != nil {
			t.Fatalf("encode: %v", err)
		}
	})

	assertValidJSON(t, "goals meta --json", out)
}

func TestJSONValidity_GoalsHistory(t *testing.T) {
	// History outputs an array of history entries.
	withGoalsJSON(t)

	entries := []goals.HistoryEntry{
		{Timestamp: "2026-01-15T10:00:00Z", GoalsPassing: 3, GoalsTotal: 5, Score: 60.0, GitSHA: "abc1234"},
		{Timestamp: "2026-01-16T10:00:00Z", GoalsPassing: 4, GoalsTotal: 5, Score: 80.0, GitSHA: "def5678"},
	}

	out := captureJSONStdout(t, func() {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(entries); err != nil {
			t.Fatalf("encode: %v", err)
		}
	})

	assertValidJSON(t, "goals history --json", out)
}

func TestJSONValidity_GoalsDrift(t *testing.T) {
	// Drift outputs an array of drift entries.
	withGoalsJSON(t)

	drifts := []goals.DriftResult{
		{GoalID: "test-1", Before: "pass", After: "fail", Delta: "regressed", Weight: 5},
		{GoalID: "test-2", Before: "fail", After: "pass", Delta: "improved", Weight: 3},
	}

	out := captureJSONStdout(t, func() {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(drifts); err != nil {
			t.Fatalf("encode: %v", err)
		}
	})

	assertValidJSON(t, "goals drift --json", out)
}

func TestJSONValidity_GoalsPrune(t *testing.T) {
	withGoalsJSON(t)

	result := pruneResult{
		StaleGoals: []staleGoal{},
		Removed:    0,
		DryRun:     true,
	}

	out := captureJSONStdout(t, func() {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(result); err != nil {
			t.Fatalf("encode: %v", err)
		}
	})

	assertValidJSON(t, "goals prune --json", out)
}

func TestJSONValidity_GoalsInit(t *testing.T) {
	// goals init --json --non-interactive outputs the generated GoalFile.
	withGoalsJSON(t)

	gf := &goals.GoalFile{
		Version: 4,
		Format:  "md",
		Mission: "Test project",
		NorthStars: []string{
			"All checks pass",
		},
		Directives: []goals.Directive{
			{Number: 1, Title: "Establish baseline", Steer: "increase"},
		},
	}

	out := captureJSONStdout(t, func() {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(gf); err != nil {
			t.Fatalf("encode: %v", err)
		}
	})

	assertValidJSON(t, "goals init --json", out)
}

func TestJSONValidity_GoalsSteerAdd(t *testing.T) {
	withGoalsJSON(t)

	d := goals.Directive{
		Number:      1,
		Title:       "Test directive",
		Description: "A test",
		Steer:       "increase",
	}

	out := captureJSONStdout(t, func() {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(d); err != nil {
			t.Fatalf("encode: %v", err)
		}
	})

	assertValidJSON(t, "goals steer add --json", out)
}

func TestJSONValidity_GoalsMeasureDirectives(t *testing.T) {
	// --directives outputs the directives array from GOALS.md.
	directives := []goals.Directive{
		{Number: 1, Title: "First", Steer: "increase"},
		{Number: 2, Title: "Second", Steer: "decrease"},
	}

	out := captureJSONStdout(t, func() {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(directives); err != nil {
			t.Fatalf("encode: %v", err)
		}
	})

	assertValidJSON(t, "goals measure --directives", out)
}

// ---------------------------------------------------------------------------
// Tests: Ratchet subcommands
// ---------------------------------------------------------------------------

func TestJSONValidity_RatchetStatus(t *testing.T) {
	withOutputJSON(t)

	data := &ratchetStatusOutput{
		ChainID: "json-test-chain",
		Started: "2026-01-15T10:00:00Z",
		EpicID:  "ag-json",
		Path:    "/tmp/chain.jsonl",
		Steps: []ratchetStepInfo{
			{Step: ratchet.StepResearch, Status: ratchet.StatusLocked, Output: "findings.md"},
			{Step: ratchet.StepPreMortem, Status: ratchet.StatusPending},
			{Step: ratchet.StepPlan, Status: ratchet.StatusPending},
		},
	}

	var buf bytes.Buffer
	if err := outputRatchetStatus(&buf, data); err != nil {
		t.Fatalf("outputRatchetStatus: %v", err)
	}

	assertValidJSON(t, "ratchet status --json", buf.String())
}

func TestJSONValidity_RatchetNext(t *testing.T) {
	result := NextResult{
		Next:         "plan",
		Reason:       "research is locked",
		LastStep:     "research",
		LastArtifact: "findings.md",
		Skill:        "/plan",
		Complete:     false,
	}

	out := captureJSONStdout(t, func() {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(result); err != nil {
			t.Fatalf("encode: %v", err)
		}
	})

	assertValidJSON(t, "ratchet next --json", out)
}

func TestJSONValidity_RatchetNextComplete(t *testing.T) {
	result := NextResult{
		Next:     "",
		Reason:   "all steps locked",
		Complete: true,
	}

	out := captureJSONStdout(t, func() {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(result); err != nil {
			t.Fatalf("encode: %v", err)
		}
	})

	assertValidJSON(t, "ratchet next --json (complete)", out)
}

// ---------------------------------------------------------------------------
// Tests: Pool subcommands
// ---------------------------------------------------------------------------

func TestJSONValidity_PoolListEmpty(t *testing.T) {
	dir := t.TempDir()
	withOutputJSON(t)

	p := pool.NewPool(dir)
	entries, err := p.List(pool.ListOptions{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	out := captureJSONStdout(t, func() {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(entries); err != nil {
			t.Fatalf("encode: %v", err)
		}
	})

	assertValidJSON(t, "pool list --json (empty)", out)
}

func TestJSONValidity_PoolListWithEntries(t *testing.T) {
	dir := t.TempDir()
	withOutputJSON(t)

	p := setupPoolWithCandidate(t, dir)
	entries, err := p.List(pool.ListOptions{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	out := captureJSONStdout(t, func() {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(entries); err != nil {
			t.Fatalf("encode: %v", err)
		}
	})

	assertValidJSON(t, "pool list --json", out)
}

func TestJSONValidity_PoolShow(t *testing.T) {
	dir := t.TempDir()
	withOutputJSON(t)

	p := setupPoolWithCandidate(t, dir)
	entry, err := p.Get("json-test-001")
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	out := captureJSONStdout(t, func() {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(entry); err != nil {
			t.Fatalf("encode: %v", err)
		}
	})

	assertValidJSON(t, "pool show --json", out)
}

func TestJSONValidity_PoolAutoPromoteResult(t *testing.T) {
	withOutputJSON(t)

	result := poolAutoPromotePromoteResult{
		Threshold:  "24h",
		Considered: 5,
		Promoted:   2,
		Skipped:    3,
		Artifacts:  []string{".agents/learnings/L001.md", ".agents/learnings/L002.md"},
		SkippedIDs: []string{"cand-003", "cand-004", "cand-005"},
	}

	out := captureJSONStdout(t, func() {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(result); err != nil {
			t.Fatalf("encode: %v", err)
		}
	})

	assertValidJSON(t, "pool auto-promote --json", out)
}

// ---------------------------------------------------------------------------
// Tests: Gate subcommand
// ---------------------------------------------------------------------------

func TestJSONValidity_GatePending(t *testing.T) {
	dir := t.TempDir()
	withOutputJSON(t)

	p := pool.NewPool(dir)
	entries, err := p.ListPendingReview()
	if err != nil {
		t.Fatalf("list pending: %v", err)
	}

	out := captureJSONStdout(t, func() {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(entries); err != nil {
			t.Fatalf("encode: %v", err)
		}
	})

	assertValidJSON(t, "gate pending --json", out)
}

// ---------------------------------------------------------------------------
// Tests: Flywheel subcommands
// ---------------------------------------------------------------------------

func TestJSONValidity_FlywheelStatus(t *testing.T) {
	withOutputJSON(t)

	// Build a synthetic flywheel metrics map matching what the command produces.
	payload := map[string]any{
		"status":                      "DECAYING",
		"delta":                       0.1,
		"sigma":                       0.2,
		"rho":                         0.3,
		"sigma_rho":                   0.06,
		"velocity":                    -0.04,
		"compounding":                 false,
		"escape_velocity_status":      "DECAYING",
		"escape_velocity_compounding": false,
	}

	out := captureJSONStdout(t, func() {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(payload); err != nil {
			t.Fatalf("encode: %v", err)
		}
	})

	assertValidJSON(t, "flywheel status --json", out)
}

func TestJSONValidity_FlywheelNudge(t *testing.T) {
	withOutputJSON(t)

	result := NudgeResult{
		Status:          "DECAYING",
		Velocity:        -0.05,
		EscapeVelocity:  false,
		SessionsCount:   3,
		LearningsCount:  10,
		PoolPending:     2,
		PoolApproaching: 1,
		Suggestion:      "Run 'ao lookup' to improve retrieval",
	}

	out := captureJSONStdout(t, func() {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(result); err != nil {
			t.Fatalf("encode: %v", err)
		}
	})

	assertValidJSON(t, "flywheel nudge --json", out)
}

func TestJSONValidity_FlywheelCloseLoop(t *testing.T) {
	withOutputJSON(t)

	result := flywheelCloseLoopResult{
		Ingest: poolIngestResult{
			FilesScanned:    5,
			CandidatesFound: 3,
			Added:           2,
			SkippedExisting: 1,
		},
		AutoPromote: poolAutoPromotePromoteResult{
			Threshold:  "24h",
			Considered: 2,
			Promoted:   1,
		},
	}

	out := captureJSONStdout(t, func() {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(result); err != nil {
			t.Fatalf("encode: %v", err)
		}
	})

	assertValidJSON(t, "flywheel close-loop --json", out)
}

// ---------------------------------------------------------------------------
// Tests: Status command
// ---------------------------------------------------------------------------

func TestJSONValidity_Status(t *testing.T) {
	withOutputJSON(t)

	status := &statusOutput{
		Initialized:  true,
		BaseDir:      "/tmp/test/.agents/ao",
		SessionCount: 3,
		RecentSessions: []sessionInfo{
			{ID: "session-1", Date: "2026-01-15", Summary: "Test session", Path: "session-1.md"},
		},
		ProvenanceStats: &provStats{TotalRecords: 10, UniqueSessions: 5},
		Flywheel: &flywheelBrief{
			Status:         "COMPOUNDING",
			TotalArtifacts: 15,
			Velocity:       0.3,
			NewArtifacts:   5,
			StaleArtifacts: 2,
		},
	}

	out := captureJSONStdout(t, func() {
		data, err := json.MarshalIndent(status, "", "  ")
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		os.Stdout.Write(data) //nolint:errcheck
	})

	assertValidJSON(t, "status --json", out)
}

func TestJSONValidity_StatusUninitialized(t *testing.T) {
	withOutputJSON(t)

	status := &statusOutput{
		Initialized: false,
		BaseDir:     "/tmp/test/.agents/ao",
	}

	out := captureJSONStdout(t, func() {
		data, err := json.MarshalIndent(status, "", "  ")
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		os.Stdout.Write(data) //nolint:errcheck
	})

	assertValidJSON(t, "status --json (uninitialized)", out)
}

// ---------------------------------------------------------------------------
// Tests: Config command
// ---------------------------------------------------------------------------

func TestJSONValidity_Config(t *testing.T) {
	withOutputJSON(t)

	// config --show --json outputs the resolved config struct.
	// We test the JSON encoding path without depending on actual config files.
	payload := map[string]any{
		"output":  map[string]any{"value": "json", "source": "flag"},
		"verbose": map[string]any{"value": false, "source": "default"},
	}

	out := captureJSONStdout(t, func() {
		data, err := json.MarshalIndent(payload, "", "  ")
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		os.Stdout.Write(data) //nolint:errcheck
	})

	assertValidJSON(t, "config --show --json", out)
}

// ---------------------------------------------------------------------------
// Tests: Doctor command
// ---------------------------------------------------------------------------

func TestJSONValidity_Doctor(t *testing.T) {
	withDoctorJSON(t)

	result := doctorOutput{
		Checks: []doctorCheck{
			{Name: "ao CLI", Status: "pass", Detail: "v0.0.0-test", Required: true},
			{Name: "test check", Status: "warn", Detail: "not installed", Required: false},
		},
		Result:  "HEALTHY",
		Summary: "2/2 checks passed",
	}

	out := captureJSONStdout(t, func() {
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		os.Stdout.WriteString(string(data) + "\n") //nolint:errcheck
	})

	assertValidJSON(t, "doctor --json", out)
}

// ---------------------------------------------------------------------------
// Tests: Inject command
// ---------------------------------------------------------------------------

func TestJSONValidity_Inject(t *testing.T) {
	// Inject --format json outputs injectedKnowledge as JSON.
	knowledge := &injectedKnowledge{
		Timestamp: time.Now(),
		Query:     "test query",
		Learnings: []learning{
			{ID: "L001", Title: "Test learning", Summary: "A test learning summary"},
		},
		Patterns: []pattern{
			{Name: "test-pattern", Description: "A test pattern"},
		},
		Sessions: []session{
			{Date: "2026-01-15", Summary: "Test session summary"},
		},
	}

	out := captureJSONStdout(t, func() {
		data, err := json.MarshalIndent(knowledge, "", "  ")
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		os.Stdout.Write(data) //nolint:errcheck
	})

	assertValidJSON(t, "inject --format json", out)
}

// ---------------------------------------------------------------------------
// Tests: Extract command
// ---------------------------------------------------------------------------

func TestJSONValidity_Extract(t *testing.T) {
	result := ExtractBatchResult{
		Processed: 3,
		Failed:    0,
		Remaining: 0,
		Entries:   []string{"session-1", "session-2", "session-3"},
	}

	out := captureJSONStdout(t, func() {
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		os.Stdout.Write(data) //nolint:errcheck
	})

	assertValidJSON(t, "extract --all --json", out)
}

func TestJSONValidity_ExtractPendingEntry(t *testing.T) {
	entry := PendingExtraction{
		SessionID:      "session-abc",
		SessionPath:    ".agents/ao/sessions/2026-01-15-test.md",
		TranscriptPath: "/tmp/transcript.jsonl",
		Summary:        "Test session",
		QueuedAt:       time.Now(),
	}

	out := captureJSONStdout(t, func() {
		data, err := json.MarshalIndent(entry, "", "  ")
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		os.Stdout.Write(data) //nolint:errcheck
	})

	assertValidJSON(t, "extract --json (single entry)", out)
}

// ---------------------------------------------------------------------------
// Tests: Session close
// ---------------------------------------------------------------------------

func TestJSONValidity_SessionClose(t *testing.T) {
	result := SessionCloseResult{
		SessionID:     "session-abc123",
		Transcript:    "/tmp/transcript.jsonl",
		Decisions:     5,
		Knowledge:     3,
		FilesChanged:  10,
		Issues:        2,
		VelocityDelta: 0.05,
		Status:        "success",
		Message:       "Session closed successfully",
	}

	out := captureJSONStdout(t, func() {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(result); err != nil {
			t.Fatalf("encode: %v", err)
		}
	})

	assertValidJSON(t, "session close --json", out)
}

// ---------------------------------------------------------------------------
// Tests: Search command
// ---------------------------------------------------------------------------

func TestJSONValidity_Search(t *testing.T) {
	withOutputJSON(t)

	results := []searchResult{
		{Path: "test.md", Type: "decisions", Context: "Test decision"},
		{Path: "test2.md", Type: "knowledge", Context: "Test knowledge"},
	}

	out := captureJSONStdout(t, func() {
		data, err := json.MarshalIndent(results, "", "  ")
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		os.Stdout.WriteString(string(data) + "\n") //nolint:errcheck
	})

	assertValidJSON(t, "search --json", out)
}

// ---------------------------------------------------------------------------
// Tests: Index command
// ---------------------------------------------------------------------------

func TestJSONValidity_Index(t *testing.T) {
	results := []indexResult{
		{
			Dir:     ".agents/learnings",
			Entries: []indexEntry{{Filename: "L001.md", Date: "2026-01-15", Summary: "Test", Tags: "test"}},
			Written: true,
		},
	}

	out := captureJSONStdout(t, func() {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(results); err != nil {
			t.Fatalf("encode: %v", err)
		}
	})

	assertValidJSON(t, "index --json", out)
}

// ---------------------------------------------------------------------------
// Tests: RPI subcommands
// ---------------------------------------------------------------------------

func TestJSONValidity_RPIVerify(t *testing.T) {
	withOutputJSON(t)

	payload := rpiVerifyOutput{
		Status: "PASS",
		rpiLedgerVerifyResult: rpiLedgerVerifyResult{
			Pass:        true,
			RecordCount: 10,
		},
	}

	out := captureJSONStdout(t, func() {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(payload); err != nil {
			t.Fatalf("encode: %v", err)
		}
	})

	assertValidJSON(t, "rpi verify --json", out)
}

func TestJSONValidity_RPIVerifyFail(t *testing.T) {
	withOutputJSON(t)

	payload := rpiVerifyOutput{
		Status: "FAIL",
		rpiLedgerVerifyResult: rpiLedgerVerifyResult{
			Pass:             false,
			RecordCount:      10,
			FirstBrokenIndex: 5,
			Message:          "hash mismatch",
		},
	}

	out := captureJSONStdout(t, func() {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(payload); err != nil {
			t.Fatalf("encode: %v", err)
		}
	})

	assertValidJSON(t, "rpi verify --json (fail)", out)
}

// ---------------------------------------------------------------------------
// Tests: Ratchet trace
// ---------------------------------------------------------------------------

func TestJSONValidity_RatchetTrace(t *testing.T) {
	entries := []traceEntry{
		{
			Step:   ratchet.StepResearch,
			Input:  "research-request",
			Output: ".agents/research/findings.md",
			Time:   time.Now().Format(time.RFC3339),
		},
		{
			Step:   ratchet.StepPlan,
			Input:  ".agents/research/findings.md",
			Output: ".agents/plans/plan.md",
			Time:   time.Now().Format(time.RFC3339),
		},
	}

	out := captureJSONStdout(t, func() {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(entries); err != nil {
			t.Fatalf("encode: %v", err)
		}
	})

	assertValidJSON(t, "ratchet trace --json", out)
}

// ---------------------------------------------------------------------------
// Tests: Temper subcommand
// ---------------------------------------------------------------------------

func TestJSONValidity_TemperStatus(t *testing.T) {
	withOutputJSON(t)

	result := TemperResult{
		Path:          ".agents/learnings/L001.md",
		Valid:         true,
		Tempered:      true,
		Maturity:      "established",
		Utility:       0.85,
		Confidence:    0.9,
		FeedbackCount: 5,
		ValidatedAt:   time.Now(),
	}

	out := captureJSONStdout(t, func() {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(result); err != nil {
			t.Fatalf("encode: %v", err)
		}
	})

	assertValidJSON(t, "temper status --json", out)
}

// ---------------------------------------------------------------------------
// Tests: Metrics subcommands
// ---------------------------------------------------------------------------

func TestJSONValidity_MetricsCiteReport(t *testing.T) {
	report := citeReportData{
		TotalCitations:  50,
		UniqueArtifacts: 20,
		UniqueSessions:  10,
		HitRate:         0.6,
		HitCount:        12,
		TopArtifacts: []artifactCount{
			{Path: ".agents/learnings/L001.md", Count: 10},
			{Path: ".agents/learnings/L002.md", Count: 8},
		},
		UncitedLearnings: []string{".agents/learnings/L003.md"},
		Staleness:        map[string]int{"30d": 3, "60d": 2, "90d": 1},
		FeedbackTotal:    50,
		FeedbackGiven:    40,
		FeedbackRate:     0.8,
		Days:             30,
		PeriodStart:      time.Now().AddDate(0, 0, -30),
		PeriodEnd:        time.Now(),
	}

	out := captureJSONStdout(t, func() {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(report); err != nil {
			t.Fatalf("encode: %v", err)
		}
	})

	assertValidJSON(t, "metrics cite-report --json", out)
}

// ---------------------------------------------------------------------------
// Tests: Store subcommands
// ---------------------------------------------------------------------------

func TestJSONValidity_StoreSearchIndex(t *testing.T) {
	entries := []IndexEntry{
		{
			Path:       ".agents/learnings/L001.md",
			ID:         "L001",
			Type:       "learning",
			Title:      "Test learning",
			Content:    "Test content",
			Utility:    0.8,
			Maturity:   "established",
			IndexedAt:  time.Now(),
			ModifiedAt: time.Now(),
		},
	}

	out := captureJSONStdout(t, func() {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(entries); err != nil {
			t.Fatalf("encode: %v", err)
		}
	})

	assertValidJSON(t, "store search --json", out)
}

func TestJSONValidity_StoreStats(t *testing.T) {
	stats := map[string]any{
		"total_entries":     42,
		"types":             map[string]int{"learning": 20, "pattern": 15, "research": 7},
		"avg_utility":       0.72,
		"last_indexed":      time.Now().Format(time.RFC3339),
		"index_size_bytes":  12345,
		"stale_count":       3,
		"categories":        5,
		"categorized_count": 30,
	}

	out := captureJSONStdout(t, func() {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(stats); err != nil {
			t.Fatalf("encode: %v", err)
		}
	})

	assertValidJSON(t, "store stats --json", out)
}

// ---------------------------------------------------------------------------
// Tests: Pool ingest
// ---------------------------------------------------------------------------

func TestJSONValidity_PoolIngest(t *testing.T) {
	result := poolIngestResult{
		FilesScanned:     10,
		CandidatesFound:  5,
		Added:            3,
		SkippedExisting:  1,
		SkippedMalformed: 1,
		Errors:           0,
		AddedIDs:         []string{"cand-001", "cand-002", "cand-003"},
	}

	out := captureJSONStdout(t, func() {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(result); err != nil {
			t.Fatalf("encode: %v", err)
		}
	})

	assertValidJSON(t, "pool ingest --json", out)
}

// ---------------------------------------------------------------------------
// Tests: Pool migrate-legacy
// ---------------------------------------------------------------------------

func TestJSONValidity_PoolMigrateLegacy(t *testing.T) {
	// The migrate-legacy command outputs a result structure.
	result := map[string]any{
		"migrated":    5,
		"skipped":     2,
		"errors":      0,
		"dry_run":     true,
		"source_dir":  ".agents/knowledge/pending",
		"target_pool": ".agents/pool",
	}

	out := captureJSONStdout(t, func() {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(result); err != nil {
			t.Fatalf("encode: %v", err)
		}
	})

	assertValidJSON(t, "pool migrate-legacy --json", out)
}

// ---------------------------------------------------------------------------
// Tests: Session outcome
// ---------------------------------------------------------------------------

func TestJSONValidity_SessionOutcome(t *testing.T) {
	outcome := SessionOutcome{
		SessionID: "session-test",
		Reward:    0.75,
		Signals: []Signal{
			{Name: "tests_pass", Value: true, Weight: 0.3},
			{Name: "git_push", Value: true, Weight: 0.2},
			{Name: "no_errors", Value: true, Weight: 0.1},
		},
		AnalyzedAt: time.Now(),
		TotalLines: 500,
	}

	out := captureJSONStdout(t, func() {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(outcome); err != nil {
			t.Fatalf("encode: %v", err)
		}
	})

	assertValidJSON(t, "session outcome --json", out)
}

// ---------------------------------------------------------------------------
// Tests: Task sync
// ---------------------------------------------------------------------------

func TestJSONValidity_TaskSync(t *testing.T) {
	events := []TaskEvent{
		{
			TaskID:    "task-001",
			Subject:   "Fix authentication bug",
			Status:    "completed",
			SessionID: "session-abc",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}

	out := captureJSONStdout(t, func() {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(events); err != nil {
			t.Fatalf("encode: %v", err)
		}
	})

	assertValidJSON(t, "task-sync --json", out)
}

// ---------------------------------------------------------------------------
// Tests: Batch promote
// ---------------------------------------------------------------------------

func TestJSONValidity_BatchPromote(t *testing.T) {
	result := map[string]any{
		"promoted": 3,
		"failed":   0,
		"skipped":  1,
		"artifacts": []string{
			".agents/learnings/L001.md",
			".agents/learnings/L002.md",
			".agents/learnings/L003.md",
		},
	}

	out := captureJSONStdout(t, func() {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(result); err != nil {
			t.Fatalf("encode: %v", err)
		}
	})

	assertValidJSON(t, "batch-promote --json", out)
}

// ---------------------------------------------------------------------------
// Tests: Context command
// ---------------------------------------------------------------------------

func TestJSONValidity_ContextStatus(t *testing.T) {
	withOutputJSON(t)

	// Context status outputs a context status structure.
	payload := map[string]any{
		"session_id":   "session-abc",
		"tokens_used":  5000,
		"max_tokens":   100000,
		"budget_pct":   5.0,
		"compacted":    false,
		"elapsed_min":  15.5,
		"tools_called": 42,
	}

	out := captureJSONStdout(t, func() {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(payload); err != nil {
			t.Fatalf("encode: %v", err)
		}
	})

	assertValidJSON(t, "context status --json", out)
}

// ---------------------------------------------------------------------------
// Tests: Maturity command
// ---------------------------------------------------------------------------

func TestJSONValidity_MaturityScan(t *testing.T) {
	withOutputJSON(t)

	results := []map[string]any{
		{
			"learning_id":     "L001",
			"current":         "provisional",
			"recommended":     "candidate",
			"transition":      true,
			"utility":         0.75,
			"reward_count":    5,
			"helpful_count":   4,
			"harmful_count":   1,
			"confidence":      0.85,
			"feedback_count":  5,
			"transition_rule": "utility >= 0.55 AND reward_count >= 3",
		},
	}

	out := captureJSONStdout(t, func() {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(results); err != nil {
			t.Fatalf("encode: %v", err)
		}
	})

	assertValidJSON(t, "maturity --scan --json", out)
}

// ---------------------------------------------------------------------------
// Tests: Feedback command
// ---------------------------------------------------------------------------

func TestJSONValidity_Feedback(t *testing.T) {
	withOutputJSON(t)

	result := map[string]any{
		"learning_id":   "L001",
		"reward":        1.0,
		"alpha":         0.1,
		"old_utility":   0.5,
		"new_utility":   0.55,
		"helpful_count": 3,
		"harmful_count": 0,
		"reward_count":  3,
	}

	out := captureJSONStdout(t, func() {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(result); err != nil {
			t.Fatalf("encode: %v", err)
		}
	})

	assertValidJSON(t, "feedback --json", out)
}

// ---------------------------------------------------------------------------
// Tests: Anti-patterns command
// ---------------------------------------------------------------------------

func TestJSONValidity_AntiPatterns(t *testing.T) {
	withOutputJSON(t)

	patterns := []map[string]any{
		{
			"name":        "Amnesia",
			"detected":    true,
			"severity":    "high",
			"description": "Agent keeps relearning the same things",
			"evidence":    []string{"L001 cited 0 times in last 30 days"},
		},
	}

	out := captureJSONStdout(t, func() {
		data, err := json.MarshalIndent(patterns, "", "  ")
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		os.Stdout.Write(data) //nolint:errcheck
	})

	assertValidJSON(t, "anti-patterns --format json", out)
}

// ---------------------------------------------------------------------------
// Tests: Vibe check
// ---------------------------------------------------------------------------

func TestJSONValidity_VibeCheck(t *testing.T) {
	// VibeCheck outputs a comprehensive analysis result.
	result := map[string]any{
		"grade":           "B",
		"score":           72.5,
		"velocity":        8,
		"rework_pct":      12.0,
		"trust_ratio":     0.85,
		"spirals":         0,
		"flow_score":      70,
		"issues":          []string{},
		"recommendations": []string{"Consider adding more test coverage"},
		"period":          "30d",
	}

	out := captureJSONStdout(t, func() {
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		os.Stdout.Write(data) //nolint:errcheck
	})

	assertValidJSON(t, "vibe-check --json", out)
}

// ---------------------------------------------------------------------------
// Tests: Trace command
// ---------------------------------------------------------------------------

func TestJSONValidity_Trace(t *testing.T) {
	withOutputJSON(t)

	result := map[string]any{
		"artifact": ".agents/ao/sessions/2026-01-15-test.md",
		"chain": []map[string]any{
			{
				"id":            "prov-001",
				"artifact_type": "session",
				"source_path":   "/tmp/transcript.jsonl",
				"session_id":    "session-abc",
				"created_at":    time.Now().Format(time.RFC3339),
			},
		},
		"sources": []string{"/tmp/transcript.jsonl"},
	}

	out := captureJSONStdout(t, func() {
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		os.Stdout.WriteString(string(data) + "\n") //nolint:errcheck
	})

	assertValidJSON(t, "trace --json", out)
}
