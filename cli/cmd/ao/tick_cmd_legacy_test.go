//go:build legacy

// practices: [tdd]
package main

import "testing"

// TestTickSubcommandSurfaceCovered asserts the `tick <sub>` surface that is only
// registered in the legacy build (tick_cmd_legacy.go). Split out of the untagged
// TestTickCommandSurfaceCovered when the `tick` command was archived (age-h4y3).
func TestTickSubcommandSurfaceCovered(t *testing.T) {
	covered := []string{
		"tick claim",
		"tick close",
		"tick council-gate",
		"tick guard-status",
		"tick install-guards",
		"tick next",
		"tick reopen",
		"tick smoke",
		"tick status",
		"tick verdict-gate",
	}
	registered := map[string]bool{}
	for _, cmd := range tickCmd.Commands() {
		registered["tick "+cmd.Name()] = true
	}
	for _, name := range covered {
		if !registered[name] {
			t.Fatalf("expected registered command %q", name)
		}
	}
}
