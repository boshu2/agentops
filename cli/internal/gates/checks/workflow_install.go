package checks

import "github.com/boshu2/agentops/cli/internal/gates"

// init registers the workflow-install freshness gate (ag-wi9w1): the installed
// ~/.claude/workflows/bdd-foundry.js must follow the repo canonical
// workflows/bdd-foundry.js (symlink installed via
// scripts/install-workflows.sh). Always-run — no path globs: the drift lives in
// $HOME, which changed-file routing can never see, so routing would orphan the
// check. Clean machines and CI stay green via the backing script's
// absent-file SKIP branch (validate-workflow-install.sh -> check-workflow-drift.sh).
func init() {
	gates.Register(gates.Check{
		ID:       "workflow.install-drift",
		Tiers:    gates.Fast | gates.Full,
		Blocking: true,
		Backing:  "validate-workflow-install.sh",
	})
}
