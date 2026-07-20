package status

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/clicontract"
)

// newTestModule builds the status module with a fixed output mode, constructing
// the command directly instead of mutating any package-global command state.
func newTestModule(outputMode string) Module {
	return NewModule(clicontract.HostOptions{OutputMode: func() string { return outputMode }})
}

func TestModule_Contract(t *testing.T) {
	contract := newTestModule("text").Contract()
	if contract.ID != "ao.status" {
		t.Fatalf("contract ID = %q, want ao.status", contract.ID)
	}
	if contract.Output != clicontract.OutputText {
		t.Fatalf("output = %v, want OutputText", contract.Output)
	}
	if contract.Effects&clicontract.EffectFilesystem == 0 || contract.Effects&clicontract.EffectClock == 0 {
		t.Fatalf("effects = %v, want filesystem+clock", contract.Effects)
	}
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
