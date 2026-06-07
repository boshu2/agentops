package main

import "testing"

func TestOrchestrateSelectCommandRegistered(t *testing.T) {
	cmd, _, err := rootCmd.Find([]string{"orchestrate", "select"})
	if err != nil {
		t.Fatalf("orchestrate select command not registered: %v", err)
	}
	if cmd == nil {
		t.Fatal("orchestrate select command not found")
	}
	if cmd.Flags().Lookup("json") == nil {
		t.Fatal("orchestrate select missing --json flag")
	}
	if cmd.Flags().Lookup("pin") == nil {
		t.Fatal("orchestrate select missing --pin flag")
	}
	if cmd.Flags().Lookup("opt-out") == nil {
		t.Fatal("orchestrate select missing --opt-out flag")
	}
}
