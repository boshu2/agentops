package main

import (
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// defaultSpineCommands is the published CLI membership boundary. Optional
// source packages do not become public commands merely by registering in tests.
var defaultSpineCommands = map[string]struct{}{
	"capabilities": {}, "config": {}, "constraint": {}, "doctor": {},
	"demo": {}, "flywheel": {}, "gate": {}, "goals": {},
	"init": {}, "provenance": {}, "quick-start": {},
	"redact": {}, "robot-docs": {}, "session": {}, "skills": {}, "status": {},
	"version": {},
	// One-release inert tombstones. These names do not restore lifecycle code.
	"pawl": {}, "plan-pawl": {}, "land": {}, "done": {}, "close": {},
	"governor": {}, "yield": {}, "claim": {}, "next-work": {}, "state": {},
	"worktree": {}, "validate": {}, "converge": {}, "reconcile": {},
	"membrane": {}, "crank": {},
}

// defaultChildSpineCommands removes legacy controller behavior nested under a
// retained read/evidence family. Source registrations remain testable, but the
// published binary exposes only this allowlist.
var defaultChildSpineCommands = map[string]map[string]struct{}{
	"goals": {
		"drift": {}, "export": {}, "history": {}, "measure": {}, "meta": {},
		"render": {}, "scenarios": {}, "validate": {},
	},
	"session": {
		"bootstrap": {}, "handoff": {}, "rehydrate": {},
	},
	"skills": {
		"check": {}, "consumers": {}, "find": {}, "graph": {}, "link": {},
		"list": {}, "producers": {}, "resolve": {}, "unlink": {},
	},
}

type prunedCommand struct {
	parent      *cobra.Command
	command     *cobra.Command
	replacement *cobra.Command
}

func init() {
	// The package test binary retains registrations so focused command tests can
	if strings.HasSuffix(os.Args[0], ".test") {
		// Constraint still has focused legacy unit tests. Production-spine tests
		// replace it temporarily through pruneToDefaultSpine.
		installRemovedCommandTombstones(rootCmd, "constraint")
		return
	}
	installRemovedCommandTombstones(rootCmd)
	pruneToDefaultSpine(rootCmd)
}

// pruneToDefaultSpine applies the production membership boundary and returns
// the commands it removed. Tests use the returned slice to restore the full
// registered test tree after checking the production view.
func pruneToDefaultSpine(root *cobra.Command) []prunedCommand {
	var removed []prunedCommand
	for name, tomb := range removedCommands {
		if _, cut := cathedralCutCommands[name]; !cut {
			continue
		}
		for _, command := range append([]*cobra.Command(nil), root.Commands()...) {
			if command.Name() != name && !command.HasAlias(name) {
				continue
			}
			if command.Short == "Removed in the AgentOps Cathedral Cut" {
				break
			}
			replacement := newRemovedCommand(name, tomb)
			root.RemoveCommand(command)
			root.AddCommand(replacement)
			removed = append(removed, prunedCommand{parent: root, command: command, replacement: replacement})
			break
		}
	}
	for _, command := range append([]*cobra.Command(nil), root.Commands()...) {
		if _, retained := defaultSpineCommands[command.Name()]; retained || command.Name() == "completion" || command.Name() == "help" {
			continue
		}
		root.RemoveCommand(command)
		removed = append(removed, prunedCommand{parent: root, command: command})
	}
	for parentName, allowed := range defaultChildSpineCommands {
		var parent *cobra.Command
		for _, command := range root.Commands() {
			if command.Name() == parentName {
				parent = command
				break
			}
		}
		if parent == nil {
			continue
		}
		for _, command := range append([]*cobra.Command(nil), parent.Commands()...) {
			if _, retained := allowed[command.Name()]; retained || command.Name() == "help" {
				continue
			}
			parent.RemoveCommand(command)
			entry := prunedCommand{parent: parent, command: command}
			if tomb, ok := removedChildCommands[parentName][command.Name()]; ok {
				entry.replacement = newRemovedChildCommand(parentName, command.Name(), tomb)
				parent.AddCommand(entry.replacement)
			}
			removed = append(removed, entry)
		}
	}
	return removed
}

func restorePrunedCommands(_ *cobra.Command, commands []prunedCommand) {
	for i := len(commands) - 1; i >= 0; i-- {
		removed := commands[i]
		if removed.replacement != nil {
			removed.parent.RemoveCommand(removed.replacement)
		}
		removed.parent.AddCommand(removed.command)
	}
}
