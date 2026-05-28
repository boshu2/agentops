package eval

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// docsReleaseGovernanceSuitePath is the suite under test, relative to this
// package directory (cli/internal/eval).
const docsReleaseGovernanceSuitePath = "../../../evals/agentops-core/docs-release-governance.json"

var shellScriptTokenRE = regexp.MustCompile(`[\w./-]+\.sh`)

// loadDocsReleaseGovernanceSuite loads the canary suite and returns it along
// with its directory (for resolving suite-relative reference paths).
func loadDocsReleaseGovernanceSuite(t *testing.T) (*Suite, string) {
	t.Helper()
	suite, _, err := LoadSuite(docsReleaseGovernanceSuitePath)
	if err != nil {
		t.Fatalf("load %s: %v", docsReleaseGovernanceSuitePath, err)
	}
	return suite, filepath.Dir(docsReleaseGovernanceSuitePath)
}

// TestDocsReleaseGovernanceEval_ReferencedPathsExist asserts that every script
// path invoked in a shell case and every artifact_contains target resolves to a
// file that exists on disk. A dangling reference (e.g. a script deleted by a
// prior cleanup) makes the canary fail when run, so it must fail this gate too.
func TestDocsReleaseGovernanceEval_ReferencedPathsExist(t *testing.T) {
	suite, suiteDir := loadDocsReleaseGovernanceSuite(t)

	for _, c := range suite.Cases {
		// Shell script tokens: resolve relative to the case cwd, which is
		// itself relative to the suite directory.
		if shell, ok := c.Inputs["shell"].(string); ok {
			cwd, _ := c.Inputs["cwd"].(string)
			base := filepath.Join(suiteDir, cwd)
			for _, tok := range shellScriptTokenRE.FindAllString(shell, -1) {
				p := filepath.Join(base, tok)
				if _, err := os.Stat(p); err != nil {
					t.Errorf("case %q: shell references missing script %q (resolved %q): %v", c.ID, tok, p, err)
				}
			}
		}

		// artifact_contains targets: resolve relative to the suite directory.
		for _, exp := range c.Expectations {
			if exp.Type != "artifact_contains" || exp.Target == "" {
				continue
			}
			p := filepath.Join(suiteDir, exp.Target)
			if _, err := os.Stat(p); err != nil {
				t.Errorf("case %q: artifact_contains target missing %q (resolved %q): %v", c.ID, exp.Target, p, err)
			}
		}
	}
}

// TestDocsReleaseGovernanceEval_NoDeletedHooksParityRefs asserts the canary no
// longer references the validate-hooks-doc-parity.sh script (deleted when hooks
// were removed in 3.0) nor expects its HOOKS_DOC_PARITY: PASS marker.
func TestDocsReleaseGovernanceEval_NoDeletedHooksParityRefs(t *testing.T) {
	suite, _ := loadDocsReleaseGovernanceSuite(t)

	for _, c := range suite.Cases {
		if shell, ok := c.Inputs["shell"].(string); ok {
			if strings.Contains(shell, "validate-hooks-doc-parity.sh") {
				t.Errorf("case %q: shell still references deleted validate-hooks-doc-parity.sh", c.ID)
			}
		}
		for _, exp := range c.Expectations {
			if strings.Contains(exp.Target, "validate-hooks-doc-parity.sh") {
				t.Errorf("case %q: artifact_contains target still references deleted validate-hooks-doc-parity.sh", c.ID)
			}
			if v, ok := exp.Value.(string); ok && strings.Contains(v, "HOOKS_DOC_PARITY") {
				t.Errorf("case %q: expectation still expects removed HOOKS_DOC_PARITY marker", c.ID)
			}
		}
	}
}
