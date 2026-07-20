package gates

import (
	"os"
	"path/filepath"
	"strings"
)

// agentopsCLIModulePath is the module path declared by the agentops repo's own
// CLI module. Its presence at <root>/cli/go.mod is the deterministic
// "this IS the agentops repo" marker used by NotApplicableOutsideAgentOps.
const agentopsCLIModulePath = "github.com/boshu2/agentops/cli"

// NotApplicableReason is the first-class SKIP reason emitted when an
// agentops-internal check runs in a foreign repository whose tree does not
// contain the check's backing artifact. Report aggregation keys on this exact
// string, so it is a single shared constant.
const NotApplicableReason = "not applicable: agentops-repo check; backing artifact not present in this repository"

// IsAgentOpsRepo reports whether root is the agentops repository itself.
//
// Marker: <root>/cli/go.mod whose `module` directive is exactly
// github.com/boshu2/agentops/cli. This is unambiguous (module paths are
// globally unique by convention and this one names the agentops repo),
// deterministic (a plain tracked file, present in every checkout and linked
// worktree), and cheap (one bounded file read). It deliberately does NOT use
// git metadata (remotes can be renamed or absent) or the presence of scripts/
// (which is exactly what a foreign repo may or may not have).
//
// Inside the agentops repo a missing backing script stays UNKNOWN and
// fail-closed — that guard protects agentops CI from silently losing a gate.
// Outside it, the same condition is a first-class not-applicable SKIP
// (see ScriptRunner.Run).
func IsAgentOpsRepo(root string) bool {
	raw, err := os.ReadFile(filepath.Join(root, "cli", "go.mod"))
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if rest, ok := strings.CutPrefix(line, "module "); ok {
			return strings.TrimSpace(rest) == agentopsCLIModulePath
		}
	}
	return false
}
