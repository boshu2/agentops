package statusapp

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/verdictcheck"
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
		if got := FormatDurationBrief(test.input); got != test.want {
			t.Errorf("FormatDurationBrief(%s) = %q, want %q", test.input, got, test.want)
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

	got := LoadLoopEvidence(tmp, now)
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
	got := LoadLoopEvidence(t.TempDir(), time.Now())
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

	got := LoadLoopEvidence(tmp, time.Now())
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

	got := LoadLoopEvidence(tmp, time.Now())
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

func TestLoadLoopEvidence_AcceptsLegacyV2AndCompleteV3(t *testing.T) {
	tmp := t.TempDir()
	legacyPath := writeVerdictArtifact(t, tmp)
	currentPath := writeVerdictV3Artifact(t, tmp)

	got := LoadLoopEvidence(tmp, time.Now())
	if got.VerdictArtifacts != 2 || len(got.Corrupt) != 0 {
		t.Fatalf("mixed version evidence not accepted: %+v", got)
	}

	payload, err := os.ReadFile(currentPath)
	if err != nil {
		t.Fatal(err)
	}
	expected, ok := artifactDigestFromName(filepath.Base(currentPath), ".json")
	if !ok {
		t.Fatal("generated verdict.v3 has invalid name")
	}
	verified, err := verdictcheck.ReadArtifact(payload, expected)
	if err != nil {
		t.Fatalf("read stored verdict.v3: %v", err)
	}
	if verified.V3 == nil ||
		verified.V3.InvocationID != "invocation:status-test" ||
		verified.V3.ProofIdentity.Epoch.String() != "1" ||
		verified.V3.Criteria[0].ID != "criterion:status-test" {
		t.Fatalf("stored verdict.v3 semantic fields were lost: %+v", verified)
	}
	if _, err := os.Stat(legacyPath); err != nil {
		t.Fatalf("legacy verdict.v2 was not preserved: %v", err)
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

func writeVerdictV3Artifact(t *testing.T, root string) string {
	t.Helper()
	directory := filepath.Join(root, ".agents", "ao", "verdicts", "sha256")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	return writeRawVerdictArtifact(t, directory, map[string]any{
		"schema_version": "verdict.v3",
		"invocation_id":  "invocation:status-test",
		"judgment_id":    "judgment:status-test",
		"intent_ref":     ".agents/ao/intents/sha256/status.intent",
		"intent_digest":  strings.Repeat("a", 64),
		"proof_identity": map[string]any{
			"epoch":                        1,
			"contract_ref":                 "docs/contracts/proof-contracts/epoch-1.json",
			"contract_digest":              strings.Repeat("b", 64),
			"activation_transition_digest": strings.Repeat("c", 64),
		},
		"schema_digests": map[string]any{
			"verdict":          strings.Repeat("a", 64),
			"rpi_report":       strings.Repeat("b", 64),
			"subject_manifest": strings.Repeat("c", 64),
			"scope_index":      strings.Repeat("d", 64),
			"effect_receipt":   strings.Repeat("e", 64),
			"check_receipt":    strings.Repeat("f", 64),
		},
		"before_manifest_ref":    "proof/before.json",
		"before_manifest_digest": strings.Repeat("d", 64),
		"final_manifest_ref":     "proof/final.json",
		"final_manifest_digest":  strings.Repeat("e", 64),
		"scope_index_ref":        "proof/scope.json",
		"scope_index_digest":     strings.Repeat("f", 64),
		"effect_receipt_ref":     "proof/effect.json",
		"effect_receipt_digest":  strings.Repeat("0", 64),
		"author_context_id":      "author:status-test",
		"validator_context_id":   "validator:status-test",
		"freshness_attestation": map[string]any{
			"source": "runtime", "attester_identity": "runtime:status-test",
		},
		"verdict": "PASS",
		"criteria": []any{map[string]any{
			"id": "criterion:status-test", "result": "PASS",
			"evidence_receipt_digests": []string{strings.Repeat("1", 64)},
			"reason":                   "checked",
		}},
		"findings":     []any{},
		"checked":      []string{"go test ./internal/statusapp"},
		"not_checked":  []any{},
		"validated_at": "2026-07-24T12:00:00Z",
	})
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
	canonical, err := verdictcheck.CanonicalJSON(value)
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
