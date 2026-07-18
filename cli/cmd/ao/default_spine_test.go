package main

import (
	"sort"
	"strings"
	"testing"
)

// approvedDefaultSpine is the published root-command allowlist. The registered
// tree IS the production tree: there is no prune step, so drift here means a
// command was registered (or dropped) without changing this contract.
var approvedDefaultSpine = map[string]bool{
	"capabilities": true, "config": true, "doctor": true,
	"demo": true, "eval": true, "flywheel": true, "gate": true, "goals": true,
	"init": true, "provenance": true, "quick-start": true,
	"redact": true, "robot-docs": true, "session": true, "skills": true,
	"status": true, "version": true,
}

var approvedDefaultChildren = map[string]map[string]bool{
	"goals": {
		"drift": true, "export": true, "history": true, "measure": true,
		"meta": true, "render": true, "scenarios": true, "validate": true,
	},
	"session": {"bootstrap": true, "handoff": true, "rehydrate": true},
	"skills": {
		"check": true, "consumers": true, "find": true, "graph": true,
		"link": true, "list": true, "producers": true, "resolve": true,
		"unlink": true,
	},
}

func TestDefaultSpineMatchesCathedralCutAllowlist(t *testing.T) {
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

// TestRetiredVerbsAreNotRegistered proves the retired lifecycle verbs are
// genuinely absent from the command tree — not hidden, not pruned at runtime,
// not stubbed. The only surviving surface is the unknown-command hint data.
func TestRetiredVerbsAreNotRegistered(t *testing.T) {
	for name, tomb := range removedCommands {
		if tomb.use == "" {
			t.Errorf("removed verb %q has an empty replacement hint", name)
		}
		for _, command := range rootCmd.Commands() {
			if command.Name() == name || command.HasAlias(name) {
				t.Errorf("retired verb %q is still registered", name)
			}
		}
	}
	for parentName, children := range removedChildCommands {
		parent, _, err := rootCmd.Find([]string{parentName})
		if err != nil || parent == nil || parent == rootCmd {
			t.Fatalf("missing retained parent %q: %v", parentName, err)
		}
		for name, tomb := range children {
			if tomb.use == "" {
				t.Errorf("removed child %s %s has an empty replacement hint", parentName, name)
			}
			for _, command := range parent.Commands() {
				if command.Name() == name || command.HasAlias(name) {
					t.Errorf("retired child %s %s is still registered", parentName, name)
				}
			}
		}
	}
}

func TestDefaultChildSpineMatchesCathedralCutAllowlist(t *testing.T) {
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
