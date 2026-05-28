package goalsfitness

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/boshu2/agentops/cli/internal/scenarioresults"
)

// fixtureRoot stages a scenario-results fixture at the canonical runtime path
// under a fresh temp project root, so the F2.0 loader resolves it normally.
func fixtureRoot(t *testing.T, fixture string) string {
	t.Helper()
	src := filepath.Join("..", "..", "..", "tests", "fixtures", "scenario-results", fixture)
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("read fixture %s: %v", fixture, err)
	}
	root := t.TempDir()
	dst := filepath.Join(root, filepath.FromSlash(scenarioresults.ArtifactRelPath))
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return root
}

func TestAggregate_PerDirectiveScore(t *testing.T) {
	root := fixtureRoot(t, "all-pass.json")
	agg, err := NewAggregator(root, false)
	if err != nil {
		t.Fatalf("NewAggregator: %v", err)
	}

	cases := []struct {
		name         string
		link         DirectiveLink
		wantStatus   AggregationStatus
		wantScore    float64
		wantLinked   int
		wantEval     int
		wantMissing  int
		wantContribs []string
	}{
		{
			name:         "single linked scenario",
			link:         DirectiveLink{DirectiveID: "d-scenario-gate", ScenarioIDs: []string{"s-2026-05-17-001"}},
			wantStatus:   StatusOK,
			wantScore:    0.95,
			wantLinked:   1,
			wantEval:     1,
			wantContribs: []string{"s-2026-05-17-001"},
		},
		{
			name:         "two linked scenarios averaged",
			link:         DirectiveLink{DirectiveID: "d-multi", ScenarioIDs: []string{"s-2026-05-17-001", "s-2026-05-17-002"}},
			wantStatus:   StatusOK,
			wantScore:    0.915, // (0.95 + 0.88) / 2
			wantLinked:   2,
			wantEval:     2,
			wantContribs: []string{"s-2026-05-17-001", "s-2026-05-17-002"},
		},
		{
			name:        "linked scenario absent from artifact",
			link:        DirectiveLink{DirectiveID: "d-orphan", ScenarioIDs: []string{"s-2026-05-17-999"}},
			wantStatus:  StatusUnknown,
			wantScore:   0,
			wantLinked:  1,
			wantEval:    0,
			wantMissing: 1,
		},
		{
			name:       "directive with no linked scenarios",
			link:       DirectiveLink{DirectiveID: "d-empty", ScenarioIDs: nil},
			wantStatus: StatusUnknown,
			wantScore:  0,
			wantLinked: 0,
			wantEval:   0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := agg.Aggregate(tc.link)
			if got.Status != tc.wantStatus {
				t.Errorf("Status = %q, want %q", got.Status, tc.wantStatus)
			}
			if got.Score != tc.wantScore {
				t.Errorf("Score = %v, want %v", got.Score, tc.wantScore)
			}
			if got.Linked != tc.wantLinked {
				t.Errorf("Linked = %d, want %d", got.Linked, tc.wantLinked)
			}
			if got.Evaluated != tc.wantEval {
				t.Errorf("Evaluated = %d, want %d", got.Evaluated, tc.wantEval)
			}
			if got.Missing != tc.wantMissing {
				t.Errorf("Missing = %d, want %d", got.Missing, tc.wantMissing)
			}
			if !equalStrings(got.Contributing, tc.wantContribs) {
				t.Errorf("Contributing = %v, want %v", got.Contributing, tc.wantContribs)
			}
		})
	}
}

func TestAggregate_FailingScenarioCountsButLowersScore(t *testing.T) {
	root := fixtureRoot(t, "has-failing.json")
	agg, err := NewAggregator(root, false)
	if err != nil {
		t.Fatalf("NewAggregator: %v", err)
	}
	// d-resteer's linked scenario failed (score 0.41); still evaluated.
	got := agg.Aggregate(DirectiveLink{
		DirectiveID: "d-resteer",
		ScenarioIDs: []string{"s-2026-05-17-003"},
	})
	if got.Status != StatusOK {
		t.Fatalf("Status = %q, want ok", got.Status)
	}
	if got.Score != 0.41 {
		t.Errorf("Score = %v, want 0.41", got.Score)
	}
	if got.Evaluated != 1 {
		t.Errorf("Evaluated = %d, want 1", got.Evaluated)
	}
}

func TestAggregate_MissingArtifactIsUnknownNoCrash(t *testing.T) {
	root := t.TempDir() // no .agents/rpi/scenario-results.json
	agg, err := NewAggregator(root, false)
	if err != nil {
		t.Fatalf("NewAggregator with missing artifact errored: %v", err)
	}
	if agg.LoadStatus() != scenarioresults.StatusUnknown {
		t.Fatalf("LoadStatus = %q, want unknown", agg.LoadStatus())
	}
	got := agg.Aggregate(DirectiveLink{
		DirectiveID: "d-scenario-gate",
		ScenarioIDs: []string{"s-2026-05-17-001"},
	})
	if got.Status != StatusUnknown {
		t.Errorf("Status = %q, want unknown (no false pass)", got.Status)
	}
	if got.Score != 0 {
		t.Errorf("Score = %v, want 0", got.Score)
	}
	if got.Evaluated != 0 {
		t.Errorf("Evaluated = %d, want 0", got.Evaluated)
	}
	if got.Warning == "" {
		t.Errorf("Warning is empty, want missing-artifact message")
	}
}

func TestAggregate_AllSkipIsUnknownNotPass(t *testing.T) {
	root := fixtureRoot(t, "has-skip.json")
	agg, err := NewAggregator(root, false)
	if err != nil {
		t.Fatalf("NewAggregator: %v", err)
	}
	// d-domain-scope's only linked scenario has verdict skip.
	got := agg.Aggregate(DirectiveLink{
		DirectiveID: "d-domain-scope",
		ScenarioIDs: []string{"s-2026-05-17-004"},
	})
	if got.Status != StatusUnknown {
		t.Errorf("Status = %q, want unknown (skip is not a pass)", got.Status)
	}
	if got.Skipped != 1 {
		t.Errorf("Skipped = %d, want 1", got.Skipped)
	}
	if got.Evaluated != 0 {
		t.Errorf("Evaluated = %d, want 0", got.Evaluated)
	}
	if len(got.Contributing) != 0 {
		t.Errorf("Contributing = %v, want empty", got.Contributing)
	}
}

func TestAggregate_DuplicateScenarioLatestWins(t *testing.T) {
	root := fixtureRoot(t, "duplicate-scenario.json")
	agg, err := NewAggregator(root, false)
	if err != nil {
		t.Fatalf("NewAggregator: %v", err)
	}
	// s-2026-05-17-005 appears twice: stale fail (0.30) then latest pass (0.91).
	got := agg.Aggregate(DirectiveLink{
		DirectiveID: "d-scenario-gate",
		ScenarioIDs: []string{"s-2026-05-17-005"},
	})
	if got.Status != StatusOK {
		t.Fatalf("Status = %q, want ok", got.Status)
	}
	if got.Score != 0.91 {
		t.Errorf("Score = %v, want 0.91 (latest judged_at wins)", got.Score)
	}
	if got.Evaluated != 1 {
		t.Errorf("Evaluated = %d, want 1", got.Evaluated)
	}
}

func TestAggregate_MalformedArtifactStrictErrors(t *testing.T) {
	root := fixtureRoot(t, "malformed-directive-mismatch.json")
	_, err := NewAggregator(root, true)
	if err == nil {
		t.Fatal("NewAggregator(strict) on malformed artifact: want error, got nil")
	}
}

func TestAggregate_MalformedArtifactNonStrictUnknown(t *testing.T) {
	root := fixtureRoot(t, "malformed-directive-mismatch.json")
	agg, err := NewAggregator(root, false)
	if err != nil {
		t.Fatalf("NewAggregator(non-strict) on malformed artifact errored: %v", err)
	}
	if agg.LoadStatus() != scenarioresults.StatusMalformed {
		t.Fatalf("LoadStatus = %q, want malformed", agg.LoadStatus())
	}
	got := agg.Aggregate(DirectiveLink{
		DirectiveID: "d-scenario-gate",
		ScenarioIDs: []string{"s-2026-05-17-006"},
	})
	if got.Status != StatusUnknown {
		t.Errorf("Status = %q, want unknown (malformed must not pass)", got.Status)
	}
}

func TestAggregate_MixedMissingAndEvaluated(t *testing.T) {
	root := fixtureRoot(t, "all-pass.json")
	agg, err := NewAggregator(root, false)
	if err != nil {
		t.Fatalf("NewAggregator: %v", err)
	}
	// One present scenario, one absent: evaluated over present only.
	got := agg.Aggregate(DirectiveLink{
		DirectiveID: "d-mixed",
		ScenarioIDs: []string{"s-2026-05-17-002", "s-2026-05-17-999"},
	})
	if got.Status != StatusOK {
		t.Fatalf("Status = %q, want ok", got.Status)
	}
	if got.Score != 0.88 {
		t.Errorf("Score = %v, want 0.88", got.Score)
	}
	if got.Linked != 2 {
		t.Errorf("Linked = %d, want 2", got.Linked)
	}
	if got.Evaluated != 1 {
		t.Errorf("Evaluated = %d, want 1", got.Evaluated)
	}
	if got.Missing != 1 {
		t.Errorf("Missing = %d, want 1", got.Missing)
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
