package statusapp

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/verdictcheck"
)

func TestRun_ExplicitRootDoesNotFollowEvidenceSymlinks(t *testing.T) {
	outside := t.TempDir()
	intent := writeIntentArtifact(t, outside, "outside intent must not be read")
	verdict := writeVerdictArtifact(t, outside)
	for _, artifact := range []string{intent, verdict} {
		store := filepath.Base(filepath.Dir(filepath.Dir(artifact)))
		for _, level := range []string{"store", "sha256", "file"} {
			t.Run(store+"/"+level, func(t *testing.T) {
				root := t.TempDir()
				link, target := filepath.Join(root, store), filepath.Dir(filepath.Dir(artifact))
				if level == "sha256" {
					link, target = filepath.Join(link, "sha256"), filepath.Dir(artifact)
				}
				if level == "file" {
					link, target = filepath.Join(link, "sha256", filepath.Base(artifact)), artifact
				}
				if err := os.MkdirAll(filepath.Dir(link), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(target, link); err != nil {
					t.Fatal(err)
				}
				got := runExplicitEvidence(t, root)
				if got.IntentArtifacts != 0 || got.VerdictArtifacts != 0 || got.State != "evidence_unavailable" || len(got.Unavailable) != 1 {
					t.Fatalf("symlink inspection: %+v", got)
				}
				if !strings.Contains(got.Unavailable[0], "symlink excluded") {
					t.Fatalf("symlink not disclosed: %+v", got)
				}
			})
		}
	}
}

func TestRun_ExplicitRootIsReadOnly(t *testing.T) {
	base := t.TempDir()
	for _, name := range []string{"empty", "missing", "Git"} {
		t.Run(name, func(t *testing.T) {
			root := filepath.Join(base, name)
			if name != "missing" {
				if err := os.Mkdir(root, 0700); err != nil {
					t.Fatal(err)
				}
			}
			if name == "Git" {
				if err := os.Mkdir(filepath.Join(root, ".git"), 0700); err != nil {
					t.Fatal(err)
				}
			}
			before := treeEntries(t, base)
			var out bytes.Buffer
			err := Run(RunOptions{EvidenceRoot: &root, JSON: true, Stdout: &out})
			if name == "empty" {
				if err != nil {
					t.Fatal(err)
				}
				var got Output
				if err := json.Unmarshal(out.Bytes(), &got); err != nil || got.LoopEvidence.State != "no_evidence" {
					t.Fatalf("empty explicit root: %v, %s", err, out.String())
				}
			} else if err == nil || out.Len() != 0 {
				t.Fatalf("invalid explicit root: %v, %s", err, out.String())
			}
			if after := treeEntries(t, base); !reflect.DeepEqual(before, after) {
				t.Fatalf("inspection wrote directories/files: before %v, after %v", before, after)
			}
		})
	}
}

func TestRun_ExplicitRootHonorsActiveGitStorageBinding(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GIT_OBJECT_DIRECTORY", root)
	var out bytes.Buffer
	err := Run(RunOptions{EvidenceRoot: &root, JSON: true, Stdout: &out})
	if err == nil || !strings.Contains(err.Error(), "overlaps declared Git storage") || out.Len() != 0 {
		t.Fatalf("active Git storage guard: %v, %s", err, out.String())
	}
	// The environment guard is confined to explicit-root selection; no-flag
	// status retains the original cwd-based legacy behavior.
	t.Chdir(t.TempDir())
	if err := Run(RunOptions{JSON: true, Stdout: &out}); err != nil {
		t.Fatalf("legacy status changed: %v", err)
	}
}

func runExplicitEvidence(t *testing.T, root string) *LoopEvidenceStatus {
	t.Helper()
	var out bytes.Buffer
	if err := Run(RunOptions{EvidenceRoot: &root, JSON: true, Stdout: &out}); err != nil {
		t.Fatal(err)
	}
	var got Output
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	return got.LoopEvidence
}

func treeEntries(t *testing.T, root string) []string {
	t.Helper()
	var entries []string
	if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		entries = append(entries, path+":"+entry.Type().String())
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return entries
}

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
