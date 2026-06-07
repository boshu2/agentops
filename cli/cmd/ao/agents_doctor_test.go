// practices: [wiki-knowledge-surface, design-by-contract]
package main

import "testing"

func TestAgentsDoctorCmd_Registered(t *testing.T) {
	cmd, _, err := rootCmd.Find([]string{"agents", "doctor"})
	if err != nil {
		t.Fatalf("agents doctor command not registered: %v", err)
	}
	if cmd.Name() != "doctor" {
		t.Fatalf("found %q, want %q", cmd.Name(), "doctor")
	}
	for _, flag := range []string{"json", "strict", "contract", "script", "agents-dir", "skills-dir"} {
		if cmd.Flags().Lookup(flag) == nil {
			t.Errorf("expected --%s flag", flag)
		}
	}
}
