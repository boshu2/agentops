package clicontract

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestInspectRecursiveStableAndPure(t *testing.T) {
	root := &cobra.Command{Use: "ao"}
	group := &cobra.Command{Use: "group"}
	leaf := &cobra.Command{Use: "leaf", RunE: func(*cobra.Command, []string) error { t.Fatal("inspector ran handler"); return nil }}
	root.AddCommand(group)
	group.AddCommand(leaf)
	got := Inspect(root, nil)
	if len(got) != 2 || got[1].ID != "ao.group.leaf" || got[1].Path != "ao group leaf" {
		t.Fatalf("unexpected recursive contract: %+v", got)
	}
}

func TestInspectProjectsAttachedContract(t *testing.T) {
	root := &cobra.Command{Use: "ao"}
	command := &cobra.Command{Use: "config", RunE: func(*cobra.Command, []string) error { return nil }}
	contract := CommandContract{
		ID:       "ao.config",
		Profiles: ProfileDefault,
		Args:     ArgsPolicy{Name: "none", Validate: cobra.NoArgs},
		Output:   OutputText,
		Effects:  EffectFilesystem | EffectEnvironment,
		ExitClasses: map[int]ExitClass{
			0: ExitSuccess,
			1: ExitFailure,
		},
	}
	if err := Attach(command, contract); err != nil {
		t.Fatal(err)
	}
	root.AddCommand(command)

	got := Inspect(root, nil)
	if len(got) != 1 {
		t.Fatalf("Inspect() returned %d commands, want 1", len(got))
	}
	if got[0].Args != "none" || got[0].Output != "text" || got[0].Effects != "filesystem,environment" {
		t.Fatalf("Inspect() ignored attached contract: %+v", got[0])
	}
}
