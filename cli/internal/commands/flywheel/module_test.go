// practices: [dora-metrics, sre]
package flywheel

import (
	"testing"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/clicontract"
)

func newTestModule(outputMode string) Module {
	return NewModule(HostOptions{
		OutputMode: func() string { return outputMode },
		Verbosef:   func(string, ...any) {},
	})
}

func TestModule_Contract(t *testing.T) {
	contract := newTestModule("table").Contract()
	if contract.ID != "ao.flywheel" {
		t.Fatalf("contract ID = %q, want ao.flywheel", contract.ID)
	}
	if contract.Args.Name != "arbitrary" {
		t.Fatalf("args = %q, want arbitrary", contract.Args.Name)
	}
	if contract.Output != clicontract.OutputText {
		t.Fatalf("output = %v, want OutputText", contract.Output)
	}
	if contract.Effects != (clicontract.EffectFilesystem | clicontract.EffectClock) {
		t.Fatalf("effects = %v, want Filesystem|Clock", contract.Effects)
	}
}

func TestModule_CommandTree(t *testing.T) {
	command := newTestModule("table").Command()
	if command.Use != "flywheel" {
		t.Errorf("Use = %q, want flywheel", command.Use)
	}
	if command.GroupID != "experimental" {
		t.Errorf("GroupID = %q, want experimental", command.GroupID)
	}
	names := map[string]bool{}
	for _, c := range command.Commands() {
		names[c.Name()] = true
	}
	if !names["status"] || !names["compare"] {
		t.Errorf("flywheel subcommands = %v, want status + compare", names)
	}
}

func TestModule_StatusFlags(t *testing.T) {
	command := newTestModule("table").Command()
	var statusCmd *cobra.Command
	for _, c := range command.Commands() {
		if c.Name() == "status" {
			statusCmd = c
		}
	}
	if statusCmd == nil {
		t.Fatal("status subcommand not registered")
	}
	if f := statusCmd.Flags().Lookup("days"); f == nil || f.DefValue != "7" {
		t.Errorf("status --days default = %v, want 7", f)
	}
	if f := statusCmd.Flags().Lookup("namespace"); f == nil || f.DefValue != "primary" {
		t.Errorf("status --namespace default = %v, want primary", f)
	}
	if f := statusCmd.Flags().Lookup("golden"); f == nil || !f.Hidden {
		t.Errorf("status --golden should exist and be hidden, got %v", f)
	}
}
