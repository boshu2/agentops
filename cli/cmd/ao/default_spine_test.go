package main

import (
	"sort"
	"strings"
	"testing"
)

var approvedDefaultSpine = map[string]bool{
	"capabilities": true, "config": true, "constraint": true, "doctor": true,
	"gate": true, "goals": true, "init": true,
	"provenance": true, "quick-start": true, "robot-docs": true,
	"session": true, "skills": true, "status": true,
	"version": true,
	"pawl":    true, "plan-pawl": true, "land": true, "done": true,
	"close": true, "governor": true, "yield": true, "claim": true,
	"next-work": true, "state": true, "worktree": true, "validate": true,
	"converge": true, "reconcile": true, "membrane": true, "crank": true,
}

func TestDefaultSpineMatchesCathedralCutAllowlist(t *testing.T) {
	if len(archiveBuildTags) != 0 {
		t.Skip("restoration build")
	}
	removed := pruneToDefaultSpine(rootCmd)
	t.Cleanup(func() { restorePrunedCommands(rootCmd, removed) })
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
	for command := range seen {
		if !approvedDefaultSpine[command] {
			unexpected = append(unexpected, command+"(registered)")
		}
	}
	sort.Strings(unexpected)
	sort.Strings(missing)
	if len(unexpected) != 0 || len(missing) != 0 {
		t.Fatalf("Cathedral Cut default membership drift\nunexpected satellites: %s\nmissing spine: %s", strings.Join(unexpected, ", "), strings.Join(missing, ", "))
	}
}
