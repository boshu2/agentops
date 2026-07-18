package checks

import (
	"testing"

	"github.com/boshu2/agentops/cli/internal/gates"
)

// TestGitConfigHygieneRegistration asserts the exact registration shape of the
// always.git-config-hygiene gate (age-gate-scripts-worktree-gitdir-p62wo):
// always-run (no path globs — the poisoning lives in .git/, invisible to
// changed-file routing), blocking, Fast|Full, backed by
// check-git-config-hygiene.sh.
func TestGitConfigHygieneRegistration(t *testing.T) {
	c, ok := gates.Default.Get("always.git-config-hygiene")
	if !ok {
		t.Fatal("always.git-config-hygiene is not registered in gates.Default")
	}
	if !c.Blocking {
		t.Error("always.git-config-hygiene must be Blocking")
	}
	if c.Backing != "check-git-config-hygiene.sh" {
		t.Errorf("always.git-config-hygiene Backing = %q, want check-git-config-hygiene.sh", c.Backing)
	}
	if !c.Tiers.Has(gates.Fast) {
		t.Errorf("always.git-config-hygiene Tiers = %v, want Fast included", c.Tiers)
	}
	if !c.Tiers.Has(gates.Full) {
		t.Errorf("always.git-config-hygiene Tiers = %v, want Full included", c.Tiers)
	}
	if len(c.Match) != 0 {
		t.Errorf("always.git-config-hygiene must be always-run (empty path globs — the mutation lives in .git/config); got %v", c.Match)
	}
}
