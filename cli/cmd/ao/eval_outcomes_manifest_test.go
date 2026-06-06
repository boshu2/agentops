package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/eval"
)

func fixedManifestTime() time.Time {
	return time.Date(2026, 6, 6, 12, 0, 0, 0, time.UTC)
}

// findEvalRunSchema walks up from the working directory to locate
// schemas/eval-run.v1.schema.json so the schema-linked assertions work whether
// the test runs from cli/cmd/ao or the repo root. Returns "" when not found.
func findEvalRunSchema(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	for i := 0; i < 8; i++ {
		cand := filepath.Join(dir, "schemas", "eval-run.v1.schema.json")
		if _, err := os.Stat(cand); err == nil {
			return cand
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

func TestBuildOutcomesRunManifest_BandMapping(t *testing.T) {
	cases := []struct {
		name       string
		band       string
		wantStatus eval.Status
		wantVerd   eval.Verdict
	}{
		{"pass", "PASS", eval.StatusPass, eval.VerdictPass},
		{"fail", "FAIL", eval.StatusFail, eval.VerdictFail},
		{"warn", "WARN", eval.StatusInconclusive, eval.VerdictAdvisory},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := outcomesScore{SourceTaskID: "task-1", Aggregate: 0.82}
			rec := buildOutcomesRunManifest(s, tc.band, "run-1", fixedManifestTime())
			if rec.Status != tc.wantStatus {
				t.Errorf("status = %q, want %q", rec.Status, tc.wantStatus)
			}
			if rec.Verdict != tc.wantVerd {
				t.Errorf("verdict = %q, want %q", rec.Verdict, tc.wantVerd)
			}
		})
	}
}

func TestBuildOutcomesRunManifest_RequiredFieldsAndStubs(t *testing.T) {
	s := outcomesScore{
		SourceTaskID:    "task-xyz",
		Aggregate:       0.9,
		Threshold:       0.8,
		Split:           "holdout",
		SuiteRef:        "suite-a",
		CriterionScores: map[string]float64{"correctness": 0.95, "safety": 0.88},
	}
	rec := buildOutcomesRunManifest(s, "PASS", "run-xyz", fixedManifestTime())

	if rec.SchemaVersion != 1 {
		t.Errorf("schema_version = %d, want 1", rec.SchemaVersion)
	}
	if rec.RunID != "run-xyz" {
		t.Errorf("run_id = %q, want run-xyz", rec.RunID)
	}
	if rec.Suite.ID != "suite-a" {
		t.Errorf("suite.id = %q, want suite-a", rec.Suite.ID)
	}
	if rec.Suite.Visibility != eval.VisibilityPrivateHoldout {
		t.Errorf("holdout split must map to private_holdout, got %q", rec.Suite.Visibility)
	}
	if rec.Suite.Path == "" || rec.Suite.Tier != eval.TierLive {
		t.Errorf("suite path/tier stubs wrong: path=%q tier=%q", rec.Suite.Path, rec.Suite.Tier)
	}
	if rec.AggregateScore != 0.9 {
		t.Errorf("aggregate_score = %v, want 0.9", rec.AggregateScore)
	}
	if len(rec.CaseResults) != 1 {
		t.Fatalf("case_results len = %d, want 1", len(rec.CaseResults))
	}
	if rec.CaseResults[0].ID != "task-xyz" {
		t.Errorf("case id = %q, want task-xyz", rec.CaseResults[0].ID)
	}
	if len(rec.DimensionScores) != 2 {
		t.Errorf("dimension_scores = %v, want 2 known dims", rec.DimensionScores)
	}
	// Honest stubs that must still be schema-valid.
	if rec.Git.CandidateSHA != "0000000" {
		t.Errorf("candidate_sha stub = %q, want 0000000", rec.Git.CandidateSHA)
	}
	if rec.Runtime.Name != eval.RuntimeManual {
		t.Errorf("runtime.name = %q, want manual", rec.Runtime.Name)
	}
	if rec.Environment.NetworkAccess != eval.NetworkUnknown {
		t.Errorf("network_access = %q, want unknown", rec.Environment.NetworkAccess)
	}
	if rec.Environment.ScrubbedEnvPrefixes == nil {
		t.Error("scrubbed_env_prefixes must be a non-nil slice (marshals to [] not null)")
	}
	if len(rec.Notes) == 0 {
		t.Error("expected a provenance note recording the out-of-process grade")
	}
}

func TestOutcomesDimensionScores_FiltersAndFallback(t *testing.T) {
	// Unknown keys are dropped; known keys (incl. clamping) are kept.
	got := outcomesDimensionScores(map[string]float64{
		"correctness":           1.4, // clamps to 1
		"made_up_axis":          0.5, // not a schema dimension → dropped
		"context_comprehension": 0.7, // valid Dimension but NOT an eval-run.v1 prop → dropped
	}, 0.6)
	if len(got) != 1 {
		t.Fatalf("expected only correctness kept, got %v", got)
	}
	if got[eval.DimensionCorrectness] != 1.0 {
		t.Errorf("correctness clamp = %v, want 1.0", got[eval.DimensionCorrectness])
	}

	// No known criterion → fallback synthesizes correctness from aggregate.
	fb := outcomesDimensionScores(map[string]float64{"nope": 0.3}, 0.42)
	if len(fb) != 1 || fb[eval.DimensionCorrectness] != 0.42 {
		t.Errorf("fallback = %v, want {correctness:0.42}", fb)
	}
}

func TestResolveOutcomesRunID(t *testing.T) {
	cases := []struct {
		name     string
		explicit string
		score    outcomesScore
		want     string
	}{
		{"explicit wins", "my-run", outcomesScore{RunID: "other", SourceTaskID: "task"}, "my-run"},
		{"falls back to run_id", "", outcomesScore{RunID: "from-payload", SourceTaskID: "task"}, "from-payload"},
		{"falls back to source_task_id", "", outcomesScore{SourceTaskID: "task-7"}, "task-7"},
		{"sanitizes invalid chars", "weird id/with spaces", outcomesScore{}, "weird-id-with-spaces"},
		{"trims leading invalid", "  -lead", outcomesScore{}, "lead"},
		{"all-empty fallback", "", outcomesScore{}, "outcomes-run"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := resolveOutcomesRunID(tc.explicit, tc.score); got != tc.want {
				t.Errorf("resolveOutcomesRunID(%q, %+v) = %q, want %q", tc.explicit, tc.score, got, tc.want)
			}
		})
	}
}

func TestWriteOutcomesRunManifest_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	s := outcomesScore{SourceTaskID: "task-rt", Aggregate: 0.77, Threshold: 0.7,
		CriterionScores: map[string]float64{"efficiency": 0.8}}
	rec := buildOutcomesRunManifest(s, "PASS", "run-rt", fixedManifestTime())

	path, err := writeOutcomesRunManifest(dir, "run-rt", rec)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	want := filepath.Join(dir, "run-rt", "manifest.json")
	if path != want {
		t.Errorf("path = %q, want %q", path, want)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	var back eval.RunRecord
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatalf("unmarshal round-trip: %v", err)
	}
	if back.RunID != "run-rt" || back.AggregateScore != 0.77 || back.Verdict != eval.VerdictPass {
		t.Errorf("round-trip mismatch: %+v", back)
	}
}

// TestOutcomesManifest_MatchesSchemaContract ties the manifest to the actual
// schema file: every top-level required key must be present and non-null in the
// marshaled manifest, and status/verdict must be members of the schema's enums.
// This is the Go-struct-roundtrip validation the bead permits in lieu of a
// vendored jsonschema validator.
func TestOutcomesManifest_MatchesSchemaContract(t *testing.T) {
	schemaPath := findEvalRunSchema(t)
	if schemaPath == "" {
		t.Skip("eval-run.v1 schema not found from cwd; skipping schema-contract check")
	}
	raw, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	var schema struct {
		Required []string `json:"required"`
		Defs     map[string]struct {
			Enum []string `json:"enum"`
		} `json:"$defs"`
	}
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatalf("parse schema: %v", err)
	}

	s := outcomesScore{SourceTaskID: "task-c", Aggregate: 0.5, Threshold: 0.8,
		CriterionScores: map[string]float64{"correctness": 0.5}}
	rec := buildOutcomesRunManifest(s, "WARN", "run-c", fixedManifestTime())
	blob, err := json.Marshal(rec)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	var asMap map[string]json.RawMessage
	if err := json.Unmarshal(blob, &asMap); err != nil {
		t.Fatalf("unmarshal manifest to map: %v", err)
	}

	for _, key := range schema.Required {
		v, ok := asMap[key]
		if !ok {
			t.Errorf("required key %q missing from manifest", key)
			continue
		}
		if string(v) == "null" {
			t.Errorf("required key %q is null", key)
		}
	}

	assertEnumMember(t, schema.Defs["status"].Enum, string(rec.Status), "status")
	assertEnumMember(t, schema.Defs["verdict"].Enum, string(rec.Verdict), "verdict")
}

func assertEnumMember(t *testing.T, enum []string, got, label string) {
	t.Helper()
	if len(enum) == 0 {
		t.Fatalf("%s enum not found in schema", label)
	}
	for _, e := range enum {
		if e == got {
			return
		}
	}
	t.Errorf("%s = %q is not a member of schema enum %v", label, got, enum)
}
