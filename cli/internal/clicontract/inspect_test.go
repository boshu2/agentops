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
