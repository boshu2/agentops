package checks

import "github.com/boshu2/agentops/cli/internal/gates"

// init registers the workflow-install freshness gate. Retired installed
// workflows must match their fail-closed source tombstones, and the source
// tombstones must refuse before dispatch. Always-run: installed copies under
// $HOME are outside changed-file routing. Absent installs are safe to skip.
func init() {
	gates.Register(gates.Check{
		ID:       "workflow.install-drift",
		Tiers:    gates.Fast | gates.Full,
		Blocking: true,
		Backing:  "validate-workflow-install.sh",
	})
}
