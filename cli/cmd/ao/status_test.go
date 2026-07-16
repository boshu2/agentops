// practices: [dora-metrics, sre]
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFormatDurationBrief(t *testing.T) {
	tests := []struct {
		input time.Duration
		want  string
	}{
		{30 * time.Second, "<1m"},
		{5 * time.Minute, "5m"},
		{2 * time.Hour, "2h"},
		{3 * 24 * time.Hour, "3d"},
		{45 * 24 * time.Hour, "6w"},
	}
	for _, test := range tests {
		if got := formatDurationBrief(test.input); got != test.want {
			t.Errorf("formatDurationBrief(%s) = %q, want %q", test.input, got, test.want)
		}
	}
}

func TestLoadLoopEvidence_ReportsOnlyValidatedArtifacts(t *testing.T) {
	tmp := t.TempDir()
	now := time.Date(2026, 7, 16, 1, 0, 0, 0, time.UTC)
	first := writeIntentArtifact(t, tmp, "first intent")
	second := writeIntentArtifact(t, tmp, "second intent")
	verdict := writeVerdictArtifact(t, tmp)
	setArtifactTime(t, first, now.Add(-20*time.Minute))
	setArtifactTime(t, second, now.Add(-5*time.Minute))
	setArtifactTime(t, verdict, now.Add(-15*time.Minute))

	got := loadLoopEvidence(tmp, now)
	if got.IntentArtifacts != 2 || got.VerdictArtifacts != 1 {
		t.Fatalf("unexpected counts: %+v", got)
	}
	if got.LatestKind != "intent" || got.State != "intent_is_latest_evidence" {
		t.Fatalf("unexpected latest evidence: %+v", got)
	}
	if got.LastEvidenceAt != "2026-07-16T00:55:00Z" || got.LastEvidenceAge != "5m" {
		t.Fatalf("unexpected evidence time: %+v", got)
	}
	if len(got.Corrupt) != 0 || len(got.Unavailable) != 0 {
		t.Fatalf("valid artifacts reported unhealthy: %+v", got)
	}
}

func TestLoadLoopEvidence_NoArtifactsIsExplicit(t *testing.T) {
	got := loadLoopEvidence(t.TempDir(), time.Now())
	if got == nil || got.State != "no_evidence" || got.IntentArtifacts != 0 || got.VerdictArtifacts != 0 {
		t.Fatalf("got %+v, want explicit no_evidence snapshot", got)
	}
	if len(got.Checked) != 2 || len(got.NotChecked) == 0 {
		t.Fatalf("missing disclosure: %+v", got)
	}
}

func TestLoadLoopEvidence_RejectsArbitraryAndCorruptFiles(t *testing.T) {
	tmp := t.TempDir()
	valid := writeIntentArtifact(t, tmp, "valid intent")
	intentDir := filepath.Dir(valid)
	verdictDir := filepath.Join(tmp, ".agents", "ao", "verdicts", "sha256")
	if err := os.MkdirAll(verdictDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(intentDir, "notes.txt"), []byte("not evidence"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(intentDir, strings.Repeat("a", 64)+".intent"), []byte("wrong digest"), 0o600); err != nil {
		t.Fatal(err)
	}
	writeRawVerdictArtifact(t, verdictDir, map[string]any{"schema_version": "verdict.v2"})

	got := loadLoopEvidence(tmp, time.Now())
	if got.IntentArtifacts != 1 || got.VerdictArtifacts != 0 {
		t.Fatalf("corrupt files affected counts: %+v", got)
	}
	if len(got.Corrupt) != 3 {
		t.Fatalf("corrupt = %+v, want three rejected files", got.Corrupt)
	}
}

func TestLoadLoopEvidence_ReportsUnavailableStore(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, ".agents", "ao", "intents", "sha256")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}

	got := loadLoopEvidence(tmp, time.Now())
	if got.State != "evidence_unavailable" || len(got.Unavailable) != 1 {
		t.Fatalf("expected explicit unavailable evidence, got %+v", got)
	}
}

func TestValidateVerdictArtifact_DetectsMutation(t *testing.T) {
	tmp := t.TempDir()
	path := writeVerdictArtifact(t, tmp)
	expected, ok := artifactDigestFromName(filepath.Base(path), ".json")
	if !ok {
		t.Fatal("generated verdict has invalid name")
	}
	if err := validateVerdictArtifact(path, expected); err != nil {
		t.Fatalf("valid verdict rejected: %v", err)
	}

	var value map[string]any
	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(payload, &value); err != nil {
		t.Fatal(err)
	}
	value["checked"] = []string{"mutated after storage"}
	payload, err = json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(payload, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := validateVerdictArtifact(path, expected); err == nil || !strings.Contains(err.Error(), "canonical content digest") {
		t.Fatalf("mutation error = %v, want canonical digest failure", err)
	}
}

func TestRunStatus_HumanOutputIsEvidenceOnly(t *testing.T) {
	resetCommandState(t)
	tmp := t.TempDir()
	writeIntentArtifact(t, tmp, "intent")
	t.Chdir(tmp)

	got, err := captureStdout(t, func() error { return runStatus(statusCmd, nil) })
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Loop Evidence", "intent_is_latest_evidence", "Checked:", "Not checked:"} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q:\n%s", want, got)
		}
	}
	for _, forbidden := range []string{"Sessions:", "Provenance:", "Flywheel", "Quality Signals", "Commands:", "ao init"} {
		if strings.Contains(got, forbidden) {
			t.Errorf("evidence-only output contains %q:\n%s", forbidden, got)
		}
	}
}

func TestRunStatus_JSONHasNoLegacySurfaces(t *testing.T) {
	resetCommandState(t)
	output = "json"
	t.Chdir(t.TempDir())

	got, err := captureStdout(t, func() error { return runStatus(statusCmd, nil) })
	if err != nil {
		t.Fatal(err)
	}
	var value map[string]any
	if err := json.Unmarshal([]byte(got), &value); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, got)
	}
	if len(value) != 1 || value["loop_evidence"] == nil {
		t.Fatalf("unexpected top-level status shape: %+v", value)
	}
	for _, forbidden := range []string{"initialized", "base_dir", "session_count", "recent_sessions", "provenance_stats", "flywheel", "quality_signals"} {
		if _, ok := value[forbidden]; ok {
			t.Errorf("JSON contains legacy field %q: %s", forbidden, got)
		}
	}
}

func writeIntentArtifact(t *testing.T, root, content string) string {
	t.Helper()
	digest := sha256.Sum256([]byte(content))
	directory := filepath.Join(root, ".agents", "ao", "intents", "sha256")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, hex.EncodeToString(digest[:])+".intent")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeVerdictArtifact(t *testing.T, root string) string {
	t.Helper()
	directory := filepath.Join(root, ".agents", "ao", "verdicts", "sha256")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	return writeRawVerdictArtifact(t, directory, map[string]any{
		"schema_version":          "verdict.v2",
		"acceptance_digest":       strings.Repeat("a", 64),
		"subject_manifest_digest": strings.Repeat("b", 64),
		"author_context_id":       "author",
		"validator_context_id":    "validator",
		"freshness_attestation": map[string]any{
			"source": "runtime", "attester_identity": "test-runtime",
		},
		"verdict": "FAIL",
		"criteria": []any{map[string]any{
			"id": "criterion", "result": "FAIL", "evidence_refs": []string{"test"},
		}},
		"findings":      []any{},
		"evidence_refs": []string{"test"},
		"checked":       []string{"test"},
		"not_checked":   []string{"live system"},
		"validated_at":  "2026-07-16T01:00:00Z",
	})
}

func writeRawVerdictArtifact(t *testing.T, directory string, value map[string]any) string {
	t.Helper()
	canonical, err := canonicalJSON(value)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(canonical)
	digestText := hex.EncodeToString(digest[:])
	value["artifact_digest"] = digestText
	payload, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, digestText+".json")
	if err := os.WriteFile(path, append(payload, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func setArtifactTime(t *testing.T, path string, timestamp time.Time) {
	t.Helper()
	if err := os.Chtimes(path, timestamp, timestamp); err != nil {
		t.Fatal(err)
	}
}
