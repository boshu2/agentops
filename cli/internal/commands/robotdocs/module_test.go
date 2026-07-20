// practices: [twelve-factor-app, ai-assisted-dev, pragmatic-programmer]
package robotdocs

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/clicontract"
)

func TestModule_Contract(t *testing.T) {
	contract := NewModule().Contract()
	if contract.ID != "ao.robot-docs" {
		t.Fatalf("contract ID = %q, want ao.robot-docs", contract.ID)
	}
	if contract.Output != clicontract.OutputText {
		t.Fatalf("output = %v, want OutputText", contract.Output)
	}
	if contract.Effects != clicontract.EffectPure {
		t.Fatalf("effects = %v, want EffectPure", contract.Effects)
	}
	if contract.Args.Name != "arbitrary" {
		t.Fatalf("args = %q, want arbitrary", contract.Args.Name)
	}
}

func TestModule_CommandAttributes(t *testing.T) {
	command := NewModule().Command()
	if command.Use != "robot-docs" {
		t.Errorf("Use = %q, want robot-docs", command.Use)
	}
	if command.GroupID != "core" {
		t.Errorf("GroupID = %q, want core", command.GroupID)
	}
	if command.Short != "Print the paste-ready agent handbook for the ao CLI (Markdown)" {
		t.Errorf("Short = %q", command.Short)
	}
}

// newTestRoot builds a small root command tree so RenderHandbook has a grouped
// command surface to project, without depending on the full ao registration.
func newTestRoot(t *testing.T) *cobra.Command {
	t.Helper()
	root := &cobra.Command{Use: "ao"}
	root.AddGroup(&cobra.Group{ID: "core", Title: "Core:"})
	child := NewModule().Command()
	root.AddCommand(child)
	other := &cobra.Command{Use: "status", Short: "Show durable AgentOps loop evidence", GroupID: "core"}
	root.AddCommand(other)
	return root
}

func TestRenderHandbook_ContainsContractSections(t *testing.T) {
	out := RenderHandbook(newTestRoot(t))
	for _, want := range []string{
		"# ao — Agent Handbook",
		"## Output contract",
		"## Exit codes",
		"## Machine-readable surfaces",
		"## Canonical agent workflow",
		"## Command surface",
		"Run `ao <command> --help` for the flags and arguments of any command.",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("handbook missing %q", want)
		}
	}
}

func TestRenderHandbook_ProjectsLiveCommandSurface(t *testing.T) {
	out := RenderHandbook(newTestRoot(t))
	if !strings.Contains(out, "Core:") {
		t.Errorf("handbook missing group title, got:\n%s", out)
	}
	if !strings.Contains(out, "robot-docs") || !strings.Contains(out, "status") {
		t.Errorf("handbook did not project the live command surface, got:\n%s", out)
	}
}

func TestRenderHandbook_HidesHiddenCommands(t *testing.T) {
	root := newTestRoot(t)
	hidden := &cobra.Command{Use: "secret", Short: "hidden", GroupID: "core", Hidden: true}
	root.AddCommand(hidden)
	out := RenderHandbook(root)
	if strings.Contains(out, "secret") {
		t.Errorf("handbook leaked a hidden command, got:\n%s", out)
	}
}

func TestCommand_RunERendersToStdout(t *testing.T) {
	root := newTestRoot(t)
	var out bytes.Buffer
	cmd, _, err := root.Find([]string{"robot-docs"})
	if err != nil {
		t.Fatalf("find robot-docs: %v", err)
	}
	cmd.SetOut(&out)
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("RunE returned error: %v", err)
	}
	if !strings.Contains(out.String(), "# ao — Agent Handbook") {
		t.Errorf("RunE output missing handbook header, got:\n%s", out.String())
	}
}
