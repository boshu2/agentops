//go:build flywheel

package main

import "testing"

func TestRefineryCmd_Registered(t *testing.T) {
	var found bool
	for _, c := range rootCmd.Commands() {
		if c.Name() == "refinery" {
			found = true
		}
	}
	if !found {
		t.Fatal("`ao refinery` not registered on root")
	}
}

func TestRefineryCmd_HasSubcommands(t *testing.T) {
	want := map[string]bool{"once": false, "run": false}
	for _, c := range refineryCmd.Commands() {
		if _, ok := want[c.Name()]; ok {
			want[c.Name()] = true
		}
	}
	for name, found := range want {
		if !found {
			t.Errorf("ao refinery missing subcommand %q", name)
		}
	}
}

func TestRefineryRun_HasIntervalFlag(t *testing.T) {
	if refineryRunCmd.Flags().Lookup("interval") == nil {
		t.Error("ao refinery run missing --interval flag")
	}
}