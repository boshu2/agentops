package checks

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/gates"
	"github.com/boshu2/agentops/cli/internal/ports"
)

func TestRunChangelogSync_ReadFailureFailsClosed(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "CHANGELOG.md"), []byte("current\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	verdict, err := runChangelogSync(context.Background(), gates.RunContext{RepoRoot: root})
	if err != nil {
		t.Fatalf("runChangelogSync: %v", err)
	}
	if verdict.Status != ports.GateStatusFail {
		t.Fatalf("status = %s, want FAIL for missing applicable evidence", verdict.Status)
	}
	if !strings.Contains(verdict.Reason, "docs/CHANGELOG.md") {
		t.Fatalf("reason = %q, want missing evidence path", verdict.Reason)
	}
}
