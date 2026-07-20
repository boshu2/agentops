package initapp

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/storage"
)

// A fresh `ao init` must satisfy the CLI's own diagnostics: the knowledge-store
// substructure doctor enforces (storage sessions/index/provenance contract) and
// both loop-evidence stores status reads (intents, verdicts). Regression guard
// for the 3.3 audit finding where doctor flagged a just-initialized store.
func TestRun_CreatesLayoutSatisfyingDoctorAndStatusContracts(t *testing.T) {
	t.Chdir(t.TempDir())
	var out bytes.Buffer

	if err := Run(RunOptions{Stdout: &out}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	mustBeDir := func(rel string) {
		t.Helper()
		info, err := os.Stat(rel)
		if err != nil || !info.IsDir() {
			t.Errorf("init did not create directory %s (err=%v)", rel, err)
		}
	}

	// Doctor's knowledge-store contract: sessions/index/provenance under the
	// storage base (fm-knowledge-missing-substructure fires on any missing one).
	for _, sub := range []string{storage.SessionsDir, storage.IndexDir, storage.ProvenanceDir} {
		mustBeDir(filepath.Join(storage.DefaultBaseDir, sub))
	}
	// Status's loop-evidence stores: intents and verdicts content stores.
	mustBeDir(filepath.Join(storage.DefaultBaseDir, "intents", "sha256"))
	mustBeDir(filepath.Join(storage.DefaultBaseDir, "verdicts", "sha256"))
	// Session handoff evidence directory.
	mustBeDir(filepath.Join(".agents", "handoff"))

	if got := strings.Count(out.String(), "created "); got != len(evidenceDirs) {
		t.Errorf("expected %d 'created' lines, got %d:\n%s", len(evidenceDirs), got, out.String())
	}
}

func TestRun_DryRunCreatesNothing(t *testing.T) {
	t.Chdir(t.TempDir())
	var out bytes.Buffer

	if err := Run(RunOptions{DryRun: true, Stdout: &out}); err != nil {
		t.Fatalf("Run dry-run: %v", err)
	}

	if got := strings.Count(out.String(), "would create "); got != len(evidenceDirs) {
		t.Errorf("expected %d 'would create' lines, got %d:\n%s", len(evidenceDirs), got, out.String())
	}
	if _, err := os.Stat(".agents"); !os.IsNotExist(err) {
		t.Errorf("dry-run created .agents (stat err=%v); want nothing on disk", err)
	}
}
