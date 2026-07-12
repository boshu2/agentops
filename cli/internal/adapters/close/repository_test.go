package close

import (
	"context"
	"path/filepath"
	"testing"

	closeapp "github.com/boshu2/agentops/cli/internal/close"
)

func TestRepositoryCommitPublicWithoutPublicPathsIsNoop(t *testing.T) {
	dir := t.TempDir()
	_, err := (Repository{}).CommitPublic(context.Background(), closeapp.Snapshot{WorkDir: dir}, closeapp.Resolution{
		LedgerDir: filepath.Join(dir, "_beads"),
	}, "close bead", nil)
	if err != nil {
		t.Fatalf("CommitPublic() with no public paths = %v, want nil", err)
	}
}
