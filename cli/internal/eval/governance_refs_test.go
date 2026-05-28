package eval

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

// shellScriptToken matches repo-relative shell script paths (foo/bar.sh) inside
// a case's shell command string, so we can assert each referenced script exists.
var shellScriptToken = regexp.MustCompile(`[A-Za-z0-9_./-]+\.sh`)

// TestDocsReleaseGovernanceEval_ReferencedPathsExist guards the public
// docs-release-governance canary against dangling references: every shell
// script it invokes and every artifact_contains target it checks must resolve
// to a file on disk. Hooks were removed in 3.0 and
// scripts/validate-hooks-doc-parity.sh was deleted; this test fails until the
// suite stops referencing it.
func TestDocsReleaseGovernanceEval_ReferencedPathsExist(t *testing.T) {
	const suiteRel = "../../../evals/agentops-core/docs-release-governance.json"
	suiteDir := filepath.Dir(suiteRel)

	suite, _, err := LoadSuite(suiteRel)
	if err != nil {
		t.Fatalf("load suite %s: %v", suiteRel, err)
	}

	var missing []string
	check := func(caseID, field, resolved, raw string) {
		if _, statErr := os.Stat(resolved); statErr != nil {
			missing = append(missing, fmt.Sprintf("case %q %s references %q (resolved %q): %v", caseID, field, raw, resolved, statErr))
		}
	}

	for _, c := range suite.Cases {
		// Shell commands resolve script tokens relative to the case cwd.
		cwd := suiteDir
		if rel, ok := c.Inputs["cwd"].(string); ok && rel != "" {
			cwd = filepath.Join(suiteDir, rel)
		}
		if shell, ok := c.Inputs["shell"].(string); ok {
			for _, tok := range shellScriptToken.FindAllString(shell, -1) {
				check(c.ID, "shell script", filepath.Join(cwd, tok), tok)
			}
		}
		// artifact_contains targets resolve relative to the suite directory.
		for _, e := range c.Expectations {
			if e.Type == "artifact_contains" && e.Target != "" {
				check(c.ID, "artifact_contains target", filepath.Join(suiteDir, e.Target), e.Target)
			}
		}
	}

	if len(missing) > 0 {
		t.Fatalf("docs-release-governance canary references %d missing path(s):\n%s", len(missing), joinLines(missing))
	}
}

func joinLines(s []string) string {
	out := ""
	for _, line := range s {
		out += "  - " + line + "\n"
	}
	return out
}
