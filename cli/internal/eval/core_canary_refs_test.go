package eval

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var retiredMortemSkillPaths = []string{
	"skills/pre-mortem/",
	"skills/post-mortem/",
	"skills/pre_mortem/",
	"skills/post_mortem/",
	"skills-codex/pre-mortem/",
	"skills-codex/post-mortem/",
	"skills-codex/pre_mortem/",
	"skills-codex/post_mortem/",
}

// TestCoreCanaryMortemReferencesAreCanonical prevents deterministic public
// canaries from surviving the mortem rename while their executable commands or
// artifact targets point at redirect-only packages. Runtime compatibility
// pointers are invocable aliases, not ownership locations for implementation
// scripts and references; canaries use canonical paths.
func TestCoreCanaryMortemReferencesAreCanonical(t *testing.T) {
	suitePaths, err := filepath.Glob("../../../evals/agentops-core/*.json")
	if err != nil {
		t.Fatalf("glob core canaries: %v", err)
	}
	if len(suitePaths) == 0 {
		t.Fatal("no agentops-core canaries found")
	}

	for _, suitePath := range suitePaths {
		t.Run(filepath.Base(suitePath), func(t *testing.T) {
			suite, _, err := LoadSuite(suitePath)
			if err != nil {
				t.Fatalf("load suite: %v", err)
			}
			suiteDir := filepath.Dir(suitePath)

			for _, c := range suite.Cases {
				if shell, ok := c.Inputs["shell"].(string); ok {
					assertNoRetiredMortemPath(t, c.ID+" shell", shell)
				}

				for _, exp := range c.Expectations {
					if exp.Type != "artifact_contains" || exp.Target == "" {
						continue
					}
					assertNoRetiredMortemPath(t, c.ID+" artifact", exp.Target)
				}
			}
			_ = suiteDir
		})
	}

	for _, path := range []string{
		"../../../skills/premortem/SKILL.md",
		"../../../skills/premortem/scripts/validate.sh",
		"../../../skills/premortem/references/premortem.feature",
		"../../../skills/premortem/schemas/premortem-plan-review.v1.schema.json",
		"../../../skills-codex/premortem/SKILL.md",
		"../../../skills-codex/premortem/prompt.md",
		"../../../skills-codex/premortem/scripts/validate.sh",
		"../../../skills/postmortem/SKILL.md",
	} {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("canonical mortem canary dependency missing %q: %v", path, err)
		}
	}
}

func assertNoRetiredMortemPath(t *testing.T, field, value string) {
	t.Helper()
	for _, retired := range retiredMortemSkillPaths {
		if strings.Contains(value, retired) {
			t.Errorf("%s uses retired implementation path %q; use canonical premortem/postmortem ownership", field, retired)
		}
	}
}
