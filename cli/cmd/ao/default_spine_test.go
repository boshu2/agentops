package main

import (
	"sort"
	"strings"
	"testing"
)

var approvedDefaultSpine = map[string]bool{
	"capabilities": true, "config": true, "constraint": true, "doctor": true,
	"gate": true, "goals": true, "init": true,
	"flywheel":   true,
	"provenance": true, "quick-start": true, "robot-docs": true,
	"session": true, "skills": true, "status": true,
	"version": true,
	"pawl":    true, "plan-pawl": true, "land": true, "done": true,
	"close": true, "governor": true, "yield": true, "claim": true,
	"next-work": true, "state": true, "worktree": true, "validate": true,
	"converge": true, "reconcile": true, "membrane": true, "crank": true,
}

var approvedDefaultChildren = map[string]map[string]bool{
	"goals": {
		"drift": true, "export": true, "history": true, "measure": true,
		"meta": true, "render": true, "scenarios": true, "trace": true, "validate": true,
	},
	"session": {"bootstrap": true, "handoff": true, "memory": true, "rehydrate": true},
	"skills": {
		"check": true, "consumers": true, "edit": true, "find": true, "graph": true,
		"link": true, "list": true, "producers": true, "resolve": true,
		"unlink": true,
	},
}

func TestDefaultSpineMatchesCathedralCutAllowlist(t *testing.T) {
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

func TestDefaultChildSpineMatchesCathedralCutAllowlist(t *testing.T) {
	removed := pruneToDefaultSpine(rootCmd)
	t.Cleanup(func() { restorePrunedCommands(rootCmd, removed) })
	for parentName, expected := range approvedDefaultChildren {
		parent, _, err := rootCmd.Find([]string{parentName})
		if err != nil || parent == nil {
			t.Fatalf("missing retained parent %q: %v", parentName, err)
		}
		seen := map[string]bool{}
		for _, command := range parent.Commands() {
			if !command.Hidden && command.Name() != "help" {
				seen[command.Name()] = true
			}
		}
		for name := range expected {
			if !seen[name] {
				t.Errorf("%s missing child %s", parentName, name)
			}
		}
		for name := range seen {
			if !expected[name] {
				t.Errorf("%s exposes legacy child %s", parentName, name)
			}
		}
	}
}
