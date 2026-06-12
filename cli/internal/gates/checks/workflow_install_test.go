package checks

import (
	"testing"

	"github.com/boshu2/agentops/cli/internal/gates"
)

// TestWorkflowInstallDriftRegistration asserts the exact registration shape of
// the workflow.install-drift gate (ag-wi9w1): always-run (no path globs),
// blocking, Fast|Full, backed by validate-workflow-install.sh.
func TestWorkflowInstallDriftRegistration(t *testing.T) {
	c, ok := gates.Default.Get("workflow.install-drift")
	if !ok {
		t.Fatal("workflow.install-drift is not registered in gates.Default")
	}
	if !c.Blocking {
		t.Error("workflow.install-drift must be Blocking")
	}
	if c.Backing != "validate-workflow-install.sh" {
		t.Errorf("workflow.install-drift Backing = %q, want validate-workflow-install.sh", c.Backing)
	}
	if !c.Tiers.Has(gates.Fast) {
		t.Errorf("workflow.install-drift Tiers = %v, want Fast included", c.Tiers)
	}
	if !c.Tiers.Has(gates.Full) {
		t.Errorf("workflow.install-drift Tiers = %v, want Full included", c.Tiers)
	}
	if len(c.Match) != 0 {
		t.Errorf("workflow.install-drift must be always-run (empty path globs, since the fixture mutation lives in $HOME); got %v", c.Match)
	}
}
