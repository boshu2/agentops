package close

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	closeapp "github.com/boshu2/agentops/cli/internal/close"
)

func TestCommitPublicWithoutPublicPathsIsNoop(t *testing.T) {
	directory := t.TempDir()
	snapshot := closeapp.Snapshot{WorkDir: directory, Env: os.Environ()}
	resolution := closeapp.Resolution{LedgerDir: filepath.Join(directory, "_beads")}

	ref, err := (Repository{}).CommitPublic(context.Background(), snapshot, resolution, "close bead", nil)
	if err != nil {
		t.Fatalf("CommitPublic() with no public paths = %v, want nil", err)
	}
	if ref != "none" {
		t.Fatalf("CommitPublic() ref = %q, want none", ref)
	}
}
