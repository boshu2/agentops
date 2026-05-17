package scenarioresults

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// fixtureBytes reads a tracked scenario-results fixture by name.
func fixtureBytes(t *testing.T, name string) []byte {
	t.Helper()
	// test file: cli/internal/scenarioresults/ -> repo root is three dirs up.
	path := filepath.Join("..", "..", "..", "tests", "fixtures", "scenario-results", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return data
}

// stageFixture writes a fixture into projectRoot's runtime artifact path.
func stageFixture(t *testing.T, projectRoot, fixture string) {
	t.Helper()
	dst := filepath.Join(projectRoot, filepath.FromSlash(ArtifactRelPath))
	if err := os.MkdirAll(filepath.Dir(dst), 0o750); err != nil {
		t.Fatalf("mkdir runtime dir: %v", err)
	}
	if err := os.WriteFile(dst, fixtureBytes(t, fixture), 0o600); err != nil {
		t.Fatalf("stage fixture %s: %v", fixture, err)
	}
}

func TestLoad_FixtureOutcomes(t *testing.T) {
	tests := []struct {
		name        string
		fixture     string
		wantStatus  Status
		wantResults int
		// per-scenario verdict expectations keyed by scenario_id.
		verdicts map[string]string
	}{
		{
			name:        "all-pass loads cleanly",
			fixture:     "all-pass.json",
			wantStatus:  StatusOK,
			wantResults: 2,
			verdicts: map[string]string{
				"s-2026-05-17-001": VerdictPass,
				"s-2026-05-17-002": VerdictPass,
			},
		},
		{
			name:        "has-failing loads cleanly with a fail",
			fixture:     "has-failing.json",
			wantStatus:  StatusOK,
			wantResults: 2,
			verdicts: map[string]string{
				"s-2026-05-17-001": VerdictPass,
				"s-2026-05-17-003": VerdictFail,
			},
		},
		{
			name:        "has-skip loads cleanly with a skip",
			fixture:     "has-skip.json",
			wantStatus:  StatusOK,
			wantResults: 2,
			verdicts: map[string]string{
				"s-2026-05-17-001": VerdictPass,
				"s-2026-05-17-004": VerdictSkip,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			stageFixture(t, root, tc.fixture)

			got, err := Load(root, true)
			if err != nil {
				t.Fatalf("Load returned error: %v", err)
			}
			if got.Status != tc.wantStatus {
				t.Fatalf("Status = %q, want %q", got.Status, tc.wantStatus)
			}
			if got.Artifact == nil {
				t.Fatalf("Artifact = nil, want non-nil")
			}
			if len(got.Artifact.Results) != tc.wantResults {
				t.Fatalf("len(Results) = %d, want %d", len(got.Artifact.Results), tc.wantResults)
			}
			if got.IsSkip() {
				t.Fatalf("IsSkip() = true, want false for StatusOK")
			}
			for _, r := range got.Artifact.Results {
				want, ok := tc.verdicts[r.ScenarioID]
				if !ok {
					t.Fatalf("unexpected scenario_id %q", r.ScenarioID)
				}
				if r.Verdict != want {
					t.Fatalf("verdict for %s = %q, want %q", r.ScenarioID, r.Verdict, want)
				}
			}
		})
	}
}

func TestLoad_ResultsCarryStableDirectiveID(t *testing.T) {
	root := t.TempDir()
	stageFixture(t, root, "all-pass.json")

	got, err := Load(root, true)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	want := map[string]string{
		"s-2026-05-17-001": "d-scenario-gate",
		"s-2026-05-17-002": "d-trace-chain",
	}
	for _, r := range got.Artifact.Results {
		if r.DirectiveID != want[r.ScenarioID] {
			t.Fatalf("directive_id for %s = %q, want %q", r.ScenarioID, r.DirectiveID, want[r.ScenarioID])
		}
		if !ValidDirectiveID(r.DirectiveID) {
			t.Fatalf("directive_id %q failed pattern check", r.DirectiveID)
		}
	}
}

func TestLoad_MissingArtifactIsUnknownSkip(t *testing.T) {
	root := t.TempDir() // no artifact staged

	got, err := Load(root, true)
	if err != nil {
		t.Fatalf("Load returned error for missing artifact: %v", err)
	}
	if got.Status != StatusUnknown {
		t.Fatalf("Status = %q, want %q", got.Status, StatusUnknown)
	}
	if got.Artifact != nil {
		t.Fatalf("Artifact = %+v, want nil", got.Artifact)
	}
	if !got.IsSkip() {
		t.Fatalf("IsSkip() = false, want true for missing artifact")
	}
	if got.Warning == "" {
		t.Fatalf("Warning = empty, want a not-found warning")
	}
}

func TestLoad_MalformedStrictVsNonStrict(t *testing.T) {
	root := t.TempDir()
	stageFixture(t, root, "malformed-directive-mismatch.json")

	// Strict mode: path-specific error.
	strictRes, strictErr := Load(root, true)
	if strictErr == nil {
		t.Fatalf("strict Load returned nil error, want a defect error")
	}
	if strictRes.Status != StatusMalformed {
		t.Fatalf("strict Status = %q, want %q", strictRes.Status, StatusMalformed)
	}

	// Non-strict mode: warning, no error.
	relaxedRes, relaxedErr := Load(root, false)
	if relaxedErr != nil {
		t.Fatalf("non-strict Load returned error: %v", relaxedErr)
	}
	if relaxedRes.Status != StatusMalformed {
		t.Fatalf("non-strict Status = %q, want %q", relaxedRes.Status, StatusMalformed)
	}
	if relaxedRes.Warning == "" {
		t.Fatalf("non-strict Warning = empty, want a malformed warning")
	}
	if !relaxedRes.IsSkip() {
		t.Fatalf("IsSkip() = false, want true for malformed artifact")
	}
}

func TestLoad_DuplicateScenarioLatestWins(t *testing.T) {
	root := t.TempDir()
	stageFixture(t, root, "duplicate-scenario.json")

	got, err := Load(root, true)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if len(got.Artifact.Results) != 1 {
		t.Fatalf("len(Results) = %d, want 1 after dedupe", len(got.Artifact.Results))
	}
	r := got.Artifact.Results[0]
	if r.ScenarioID != "s-2026-05-17-005" {
		t.Fatalf("scenario_id = %q, want s-2026-05-17-005", r.ScenarioID)
	}
	if r.Verdict != VerdictPass {
		t.Fatalf("verdict = %q, want %q (latest judged_at wins)", r.Verdict, VerdictPass)
	}
	if r.Score != 0.91 {
		t.Fatalf("score = %v, want 0.91 (latest judged_at wins)", r.Score)
	}
}

func TestWriter_AppendDoesNotBreakExistingArtifact(t *testing.T) {
	root := t.TempDir()
	stageFixture(t, root, "all-pass.json")

	clock := func() time.Time { return time.Date(2026, 5, 17, 14, 0, 0, 0, time.UTC) }
	w := Writer{Now: clock}

	newResults := []ScenarioResult{
		{
			ScenarioID:  "s-2026-05-17-009",
			DirectiveID: "d-resteer",
			Score:       0.7,
			Threshold:   0.8,
			Verdict:     VerdictFail,
			JudgedAt:    "2026-05-17T13:55:00Z",
			Evidence:    []string{".agents/rpi/phase-4-result.json"},
		},
	}
	artifact, err := w.Append(root, "run-2026-05-17-009", 3, newResults)
	if err != nil {
		t.Fatalf("Append returned error: %v", err)
	}
	if len(artifact.Results) != 3 {
		t.Fatalf("merged len(Results) = %d, want 3 (2 prior + 1 new)", len(artifact.Results))
	}
	if artifact.RunID != "run-2026-05-17-009" {
		t.Fatalf("RunID = %q, want run-2026-05-17-009", artifact.RunID)
	}
	if artifact.Iteration != 3 {
		t.Fatalf("Iteration = %d, want 3", artifact.Iteration)
	}
	if artifact.GeneratedAt != "2026-05-17T14:00:00Z" {
		t.Fatalf("GeneratedAt = %q, want 2026-05-17T14:00:00Z", artifact.GeneratedAt)
	}

	// Re-load from disk: the prior two scenarios survived the append.
	got, err := Load(root, true)
	if err != nil {
		t.Fatalf("Load after Append returned error: %v", err)
	}
	if len(got.Artifact.Results) != 3 {
		t.Fatalf("reloaded len(Results) = %d, want 3", len(got.Artifact.Results))
	}
	ids := map[string]bool{}
	for _, r := range got.Artifact.Results {
		ids[r.ScenarioID] = true
	}
	for _, want := range []string{"s-2026-05-17-001", "s-2026-05-17-002", "s-2026-05-17-009"} {
		if !ids[want] {
			t.Fatalf("scenario %q missing after append", want)
		}
	}
}

func TestWriter_AppendLatestWinsOnDuplicate(t *testing.T) {
	root := t.TempDir()
	clock := func() time.Time { return time.Date(2026, 5, 17, 15, 0, 0, 0, time.UTC) }
	w := Writer{Now: clock}

	first := []ScenarioResult{{
		ScenarioID:  "s-2026-05-17-010",
		DirectiveID: "d-scenario-gate",
		Score:       0.2,
		Threshold:   0.8,
		Verdict:     VerdictFail,
		JudgedAt:    "2026-05-17T14:00:00Z",
		Evidence:    []string{"stale"},
	}}
	if _, err := w.Append(root, "run-x", 0, first); err != nil {
		t.Fatalf("first Append error: %v", err)
	}

	second := []ScenarioResult{{
		ScenarioID:  "s-2026-05-17-010",
		DirectiveID: "d-scenario-gate",
		Score:       0.93,
		Threshold:   0.8,
		Verdict:     VerdictPass,
		JudgedAt:    "2026-05-17T14:30:00Z",
		Evidence:    []string{"fresh"},
	}}
	artifact, err := w.Append(root, "run-x", 1, second)
	if err != nil {
		t.Fatalf("second Append error: %v", err)
	}
	if len(artifact.Results) != 1 {
		t.Fatalf("len(Results) = %d, want 1 after latest-wins merge", len(artifact.Results))
	}
	if artifact.Results[0].Verdict != VerdictPass {
		t.Fatalf("verdict = %q, want %q (latest judged_at wins)", artifact.Results[0].Verdict, VerdictPass)
	}
	if artifact.Results[0].Score != 0.93 {
		t.Fatalf("score = %v, want 0.93", artifact.Results[0].Score)
	}
}

func TestValidVerdict_ExactCases(t *testing.T) {
	cases := map[string]bool{
		"pass": true, "fail": true, "skip": true,
		"PASS": false, "": false, "warn": false,
	}
	for in, want := range cases {
		if got := ValidVerdict(in); got != want {
			t.Fatalf("ValidVerdict(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestValidDirectiveID_ExactCases(t *testing.T) {
	cases := map[string]bool{
		"d-scenario-gate": true, "d-a": true, "d-0": true,
		"D-Scenario-Gate": false, "scenario-gate": false, "d-": false, "": false,
	}
	for in, want := range cases {
		if got := ValidDirectiveID(in); got != want {
			t.Fatalf("ValidDirectiveID(%q) = %v, want %v", in, got, want)
		}
	}
}
