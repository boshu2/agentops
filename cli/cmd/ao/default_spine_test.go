package main

import (
	"sort"
	"strings"
	"testing"
)

var approvedDefaultSpine = map[string]bool{
	"beads": true, "capabilities": true, "claim": true, "close": true,
	"config": true, "council-gate": true, "doctor": true, "done": true,
	"eval": true, "gate": true, "goals": true, "governor": true,
	"init": true, "land": true, "membrane": true, "pawl": true,
	"plan-pawl": true, "provenance": true, "quick-start": true, "ready": true,
	"robot-docs": true, "session": true, "skills": true, "status": true,
	"validate": true, "verdict-gate": true, "verify": true, "version": true,
	"yield": true,
}

func TestDefaultSpineMatchesADR0012Allowlist(t *testing.T) {
	if len(archiveBuildTags) != 0 {
		t.Skip("restoration build")
	}
	var unexpected, missing []string
	seen := map[string]bool{}
	for _, command := range rootCmd.Commands() {
		if command.Hidden || command.Name() == "help" || command.Name() == "completion" {
			continue
		}
		seen[command.Name()] = true
	}
	for command := range approvedDefaultSpine {
		if !seen[command] {
			missing = append(missing, command)
		}
		if _, retained := defaultSpineCommands[command]; !retained {
			missing = append(missing, command+"(boundary)")
		}
	}
	for command := range defaultSpineCommands {
		if !approvedDefaultSpine[command] {
			unexpected = append(unexpected, command)
		}
	}
	sort.Strings(unexpected)
	sort.Strings(missing)
	if len(unexpected) != 0 || len(missing) != 0 {
		t.Fatalf("ADR-0012 default membership drift\nunexpected satellites: %s\nmissing spine: %s", strings.Join(unexpected, ", "), strings.Join(missing, ", "))
	}
}
