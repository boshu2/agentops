package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/clicontract"
	skillscommand "github.com/boshu2/agentops/cli/internal/commands/skills"
)

func TestSkillsBuilderComposition(t *testing.T) {
	// Exercise ao skills build and ao skills check-source through their registered family.
	root := t.TempDir()
	root, _ = filepath.EvalSymlinks(root)
	if err := os.Mkdir(filepath.Join(root, "skills"), 0755); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) (string, error) {
		command := skillscommand.NewModule(clicontract.HostOptions{}).Command()
		var out bytes.Buffer
		command.SetOut(&out)
		command.SetErr(&out)
		command.SetArgs(args)
		err := command.Execute()
		return out.String(), err
	}
	out, err := run("build", "from-scratch", "sample", "--init-only", "--repo", root)
	if err != nil || !strings.Contains(out, `"authoring_state":"scaffold"`) {
		t.Fatalf("build: %s %v", out, err)
	}
	out, err = run("check-source", "skills/sample", "--strict", "--repo", root)
	if err == nil || !strings.Contains(out, "INCOMPLETE_SCAFFOLD") {
		t.Fatalf("check-source: %s %v", out, err)
	}
	for _, path := range []string{"ao skills build", "ao skills check-source", "ao skills audit"} {
		command := skillscommand.NewModule(clicontract.HostOptions{}).Command()
		child, remaining, err := command.Find(strings.Fields(strings.TrimPrefix(path, "ao skills ")))
		if err != nil || child == command || len(remaining) != 0 {
			t.Fatalf("missing %s", path)
		}
	}
}
