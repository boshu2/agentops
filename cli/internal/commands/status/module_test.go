package status

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

	"github.com/boshu2/agentops/cli/internal/clicontract"
	"github.com/boshu2/agentops/cli/internal/evidence"
	"github.com/boshu2/agentops/cli/internal/statusapp"
	"gopkg.in/yaml.v3"
)

// newTestModule builds the status module with a fixed output mode, constructing
// the command directly instead of mutating any package-global command state.
func newTestModule(outputMode string) Module {
	return NewModule(clicontract.HostOptions{OutputMode: func() string { return outputMode }})
}

func TestModule_ExplicitEvidenceRootFromUnrelatedDirectory(t *testing.T) {
	root := t.TempDir()
	if _, err := evidence.SnapshotIntent(root, []byte("selected external intent")); err != nil {
		t.Fatal(err)
	}
	cwd := t.TempDir()
	writeIntentArtifact(t, cwd, "unrelated intent one")
	writeIntentArtifact(t, cwd, "unrelated intent two")
	t.Chdir(cwd)

	var buf bytes.Buffer
	command := newTestModule("json").Command()
	command.SetOut(&buf)
	command.SetErr(&buf)
	command.SetArgs([]string{"--evidence-root", root})
	if err := command.Execute(); err != nil {
		t.Fatalf("status --evidence-root: %v\n%s", err, buf.String())
	}
	var got statusapp.Output
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.LoopEvidence.IntentArtifacts != 1 || got.LoopEvidence.VerdictArtifacts != 0 {
		t.Fatalf("selected evidence counts: %+v", got.LoopEvidence)
	}
}

func TestModule_Contract(t *testing.T) {
	contract := newTestModule("text").Contract()
	if contract.ID != "ao.status" {
		t.Fatalf("contract ID = %q, want ao.status", contract.ID)
	}
	if contract.Output != clicontract.OutputText {
		t.Fatalf("output = %v, want OutputText", contract.Output)
	}
	if contract.Effects != clicontract.EffectFilesystem|clicontract.EffectEnvironment|clicontract.EffectClock {
		t.Fatalf("effects = %v, want filesystem+environment+clock", contract.Effects)
	}
}

func TestModule_ExplicitRootFormatsPreserveStructuralBoundary(t *testing.T) {
	root := t.TempDir()
	stored := storeExternalVerdict(t, root)
	// Unrelated files and nested legacy stores must not join the selected stores.
	writeIntentArtifact(t, root, "nested legacy intent")
	if err := os.WriteFile(filepath.Join(root, "notes.json"), []byte("not evidence"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(filepath.Dir(stored.Path), strings.Repeat("a", 64)+".json"), []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(filepath.Dir(stored.IntentRef), "notes.txt"), []byte("not an intent"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(t.TempDir())
	var reports []map[string]any
	for _, format := range []string{"text", "json", "yaml"} {
		var buf bytes.Buffer
		command := newTestModule(format).Command()
		command.SetOut(&buf)
		command.SetArgs([]string{"--evidence-root", root})
		if err := command.Execute(); err != nil {
			t.Fatalf("%s: %v", format, err)
		}
		if format == "text" {
			for _, want := range []string{"Artifacts: 1 intents, 1 verdicts", "Corrupt:", "Not checked:", "caller-supplied subject manifests"} {
				if !strings.Contains(buf.String(), want) {
					t.Errorf("text missing %q: %s", want, buf.String())
				}
			}
			continue
		}
		var report map[string]any
		if format == "yaml" {
			var value any
			if err := yaml.Unmarshal(buf.Bytes(), &value); err != nil {
				t.Fatal(err)
			}
			payload, err := json.Marshal(value)
			if err != nil {
				t.Fatal(err)
			}
			buf.Reset()
			buf.Write(payload)
		}
		if err := json.Unmarshal(buf.Bytes(), &report); err != nil {
			t.Fatal(err)
		}
		loop := report["loop_evidence"].(map[string]any)
		if loop["intent_artifacts"] != float64(1) || loop["verdict_artifacts"] != float64(1) || len(loop["corrupt"].([]any)) != 2 {
			t.Fatalf("%s counts or structural validation: %+v", format, loop)
		}
		if !reflect.DeepEqual(loop["checked"], []any{"intents/sha256", "verdicts/sha256"}) || len(loop["not_checked"].([]any)) != 5 {
			t.Fatalf("%s inspection boundary: %+v", format, loop)
		}
		reports = append(reports, report)
	}
	if !reflect.DeepEqual(reports[0], reports[1]) {
		t.Fatalf("JSON/YAML reports differ: %+v", reports)
	}
}

func TestModule_InvalidExplicitRootNeverFallsBack(t *testing.T) {
	cwd := t.TempDir()
	writeIntentArtifact(t, cwd, "fallback must not be inspected")
	t.Chdir(cwd)
	gitRoot := t.TempDir()
	if err := os.Mkdir(filepath.Join(gitRoot, ".git"), 0700); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, []byte("not a directory"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, root := range []string{"", " ", filepath.Join(t.TempDir(), "missing"), gitRoot, file} {
		for _, format := range []string{"text", "json", "yaml"} {
			var buf bytes.Buffer
			command := newTestModule(format).Command()
			command.SilenceUsage, command.SilenceErrors = true, true
			command.SetOut(&buf)
			command.SetErr(&buf)
			command.SetArgs([]string{"--evidence-root", root})
			if err := command.Execute(); err == nil || !strings.Contains(err.Error(), "invalid --evidence-root") {
				t.Fatalf("root %q, %s: %v", root, format, err)
			}
			if buf.Len() != 0 {
				t.Fatalf("invalid root emitted fallback report: %s", buf.String())
			}
		}
	}
}

func storeExternalVerdict(t *testing.T, root string) *evidence.StoreResult {
	t.Helper()
	subject := t.TempDir()
	if err := os.WriteFile(filepath.Join(subject, "value"), []byte("synthetic subject"), 0600); err != nil {
		t.Fatal(err)
	}
	manifest, err := evidence.BuildManifest(subject, []string{"value"}, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	manifestPath, err := evidence.StoreDocument(root, "manifest.json", manifest)
	if err != nil {
		t.Fatal(err)
	}
	draft := map[string]any{
		"verdict": "PASS", "criteria": []any{map[string]any{"id": "synthetic", "result": "PASS", "evidence_refs": []string{"unresolved:synthetic-check"}}},
		"findings": []any{}, "evidence_refs": []string{"unresolved:synthetic-check"}, "checked": []string{"value"}, "not_checked": []string{}, "validated_at": "2026-07-14T00:00:00Z",
	}
	draftPath, err := evidence.StoreDocument(root, "draft.json", draft)
	if err != nil {
		t.Fatal(err)
	}
	intent, err := evidence.SnapshotIntent(root, []byte("synthetic acceptance"))
	if err != nil {
		t.Fatal(err)
	}
	stored, err := evidence.StoreVerdict(evidence.StoreOptions{
		Root: subject, EvidenceRoot: root, SubjectManifest: manifestPath, Draft: draftPath, IntentSource: intent.IntentRef,
		Facts: evidence.RuntimeFacts{AuthorContextID: "synthetic-author", ValidatorContextID: "synthetic-judge", FreshnessSource: "runtime", FreshnessAttesterID: "synthetic-runtime", ScopeResult: "PASS"},
	})
	if err != nil {
		t.Fatal(err)
	}
	return stored
}

func TestModule_CommandAttributes(t *testing.T) {
	command := newTestModule("text").Command()
	if command.Use != "status" {
		t.Fatalf("Use = %q, want status", command.Use)
	}
	if command.GroupID != "core" {
		t.Fatalf("GroupID = %q, want core", command.GroupID)
	}
}

func TestModule_HumanOutputIsEvidenceOnly(t *testing.T) {
	tmp := t.TempDir()
	writeIntentArtifact(t, tmp, "intent")
	t.Chdir(tmp)

	var buf bytes.Buffer
	command := newTestModule("text").Command()
	command.SetOut(&buf)
	command.SetArgs(nil)
	if err := command.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	got := buf.String()
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

func TestModule_JSONHasNoLegacySurfaces(t *testing.T) {
	t.Chdir(t.TempDir())

	var buf bytes.Buffer
	command := newTestModule("json").Command()
	command.SetOut(&buf)
	command.SetArgs(nil)
	if err := command.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	var value map[string]any
	if err := json.Unmarshal(buf.Bytes(), &value); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, buf.String())
	}
	if len(value) != 1 || value["loop_evidence"] == nil {
		t.Fatalf("unexpected top-level status shape: %+v", value)
	}
	for _, forbidden := range []string{"initialized", "base_dir", "session_count", "recent_sessions", "provenance_stats", "flywheel", "quality_signals"} {
		if _, ok := value[forbidden]; ok {
			t.Errorf("JSON contains legacy field %q: %s", forbidden, buf.String())
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
