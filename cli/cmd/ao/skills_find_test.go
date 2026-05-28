// practices: [design-by-contract, code-complete]
package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/skills"
)

// runFind invokes runSkillsFind with separate stdout/stderr buffers so tests
// can assert the stdout-as-data / stderr-as-diagnostics contract. It resolves
// skills/ from the live tree (test cwd walks up to the repo root).
func runFind(t *testing.T, jsonMode bool, limit int, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	prevJSON, prevLimit := skillsFindJSON, skillsFindLimit
	defer func() { skillsFindJSON, skillsFindLimit = prevJSON, prevLimit }()
	skillsFindJSON = jsonMode
	skillsFindLimit = limit

	var out, errb bytes.Buffer
	c := &cobra.Command{}
	c.SetOut(&out)
	c.SetErr(&errb)
	err = runSkillsFind(c, args)
	return out.String(), errb.String(), err
}

func TestSkillsFind_JSONIsSortedArrayOnStdout(t *testing.T) {
	stdout, stderr, err := runFind(t, true, 5, "close", "the", "loop")
	if err != nil {
		t.Fatalf("runSkillsFind: %v", err)
	}
	if stderr != "" {
		t.Errorf("expected no diagnostics on stderr in JSON mode, got %q", stderr)
	}
	var got []skills.Match
	if jerr := json.Unmarshal([]byte(stdout), &got); jerr != nil {
		t.Fatalf("stdout is not a JSON array: %v\noutput: %s", jerr, stdout)
	}
	if len(got) == 0 {
		t.Fatal("expected at least one match for 'close the loop' against the live tree")
	}
	if len(got) > 5 {
		t.Errorf("expected at most 5 results (default limit), got %d", len(got))
	}
	for i := 1; i < len(got); i++ {
		if got[i-1].Score < got[i].Score {
			t.Errorf("results not sorted descending at %d: %v < %v", i, got[i-1].Score, got[i].Score)
		}
	}
}

func TestSkillsFind_TextRanksOnStdout(t *testing.T) {
	stdout, _, err := runFind(t, false, 5, "close the loop")
	if err != nil {
		t.Fatalf("runSkillsFind: %v", err)
	}
	if !strings.Contains(stdout, "1.") {
		t.Errorf("expected a ranked list on stdout, got %q", stdout)
	}
}

func TestSkillsFind_UnmatchedReportsGracefully(t *testing.T) {
	stdout, stderr, err := runFind(t, false, 5, "zzqqxx-nonsense-token")
	if err != nil {
		t.Fatalf("unmatched intent must exit 0, got error: %v", err)
	}
	if strings.TrimSpace(stdout) != "" {
		t.Errorf("expected empty stdout for unmatched intent, got %q", stdout)
	}
	if !strings.Contains(stderr, "no strong matches") {
		t.Errorf("expected a 'no strong matches' note on stderr, got %q", stderr)
	}
}

func TestSkillsFind_LimitCapsResultCount(t *testing.T) {
	stdout, _, err := runFind(t, true, 3, "run")
	if err != nil {
		t.Fatalf("runSkillsFind: %v", err)
	}
	var got []skills.Match
	if jerr := json.Unmarshal([]byte(stdout), &got); jerr != nil {
		t.Fatalf("stdout is not a JSON array: %v", jerr)
	}
	if len(got) > 3 {
		t.Errorf("expected at most 3 results with --limit 3, got %d", len(got))
	}
}

func TestSkillsFind_InvalidLimitNamesCorrection(t *testing.T) {
	_, _, err := runFind(t, false, 0, "anything")
	if err == nil {
		t.Fatal("expected an error for --limit 0")
	}
	if !strings.Contains(err.Error(), "--limit") {
		t.Errorf("error should name the --limit flag, got %q", err.Error())
	}
}

func TestSkillsFind_RegisteredUnderSkills(t *testing.T) {
	found := false
	for _, c := range skillsCmd.Commands() {
		if c.Name() == "find" {
			found = true
			if !c.Flags().HasAvailableFlags() {
				t.Error("skills find has no flags; expected --json and --limit")
			}
		}
	}
	if !found {
		t.Error("`find` is not registered under `skills`")
	}
}
