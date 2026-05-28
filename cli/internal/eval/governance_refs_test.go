package eval

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// scriptPathPattern matches a whitespace-delimited shell field that is a
// relative path to a `.sh` script (e.g. `scripts/foo.sh`,
// `skills/doc/scripts/validate.sh`). Anchoring to the whole field avoids
// matching a `scripts/` substring inside a longer path.
var scriptPathPattern = regexp.MustCompile(`^[\w./-]+\.sh$`)

// TestDocsReleaseGovernanceEval_ReferencedPathsExist guards the
// docs-release-governance canary against dangling references: every
// `scripts/*.sh` token in a shell input and every artifact_contains target must
// resolve to a file that exists on disk. It is scoped to this one suite so it
// catches references that outlive the files they point at (e.g. the
// validate-hooks-doc-parity.sh canary removed when hooks left in 3.0) without
// flagging unrelated drift in other suites.
func TestDocsReleaseGovernanceEval_ReferencedPathsExist(t *testing.T) {
	suitePath := filepath.Join("..", "..", "..", "evals", "agentops-core", "docs-release-governance.json")
	suite, _, err := LoadSuite(suitePath)
	if err != nil {
		t.Fatalf("LoadSuite(%s): %v", suitePath, err)
	}
	suiteDir := filepath.Dir(suitePath)

	for _, c := range suite.Cases {
		// Shell command scripts resolve relative to the case cwd, which is
		// itself relative to the suite directory.
		if shell, ok := c.Inputs["shell"].(string); ok && shell != "" {
			cwd := "."
			if raw, ok := c.Inputs["cwd"].(string); ok && raw != "" {
				cwd = raw
			}
			scriptRoot := filepath.Join(suiteDir, cwd)
			for _, field := range strings.Fields(shell) {
				if !scriptPathPattern.MatchString(field) {
					continue
				}
				resolved := filepath.Join(scriptRoot, field)
				if _, err := os.Stat(resolved); err != nil {
					t.Errorf("case %q references missing script %q (resolved %q): %v", c.ID, field, resolved, err)
				}
			}
		}

		// artifact_contains targets resolve relative to the suite directory.
		for _, exp := range c.Expectations {
			if exp.Type != "artifact_contains" || exp.Target == "" {
				continue
			}
			resolved := filepath.Join(suiteDir, exp.Target)
			if _, err := os.Stat(resolved); err != nil {
				t.Errorf("case %q artifact_contains target %q (resolved %q) does not exist: %v", c.ID, exp.Target, resolved, err)
			}
		}
	}
}
