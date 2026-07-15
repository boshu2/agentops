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
	"flywheel": {}, "gate": {}, "goals": {},
	"init": {}, "provenance": {}, "quick-start": {},
	"robot-docs": {}, "session": {}, "skills": {}, "status": {},
	"version": {},
	// One-release inert tombstones. These names do not restore lifecycle code.
	"pawl": {}, "plan-pawl": {}, "land": {}, "done": {}, "close": {},
	"governor": {}, "yield": {}, "claim": {}, "next-work": {}, "state": {},
	"worktree": {}, "validate": {}, "converge": {}, "reconcile": {},
	"membrane": {}, "crank": {},
}

func init() {
	installRemovedCommandTombstones(rootCmd)

	// The package test binary retains registrations so focused command tests can
	// exercise optional surfaces. Production binaries take the boundary.
	if strings.HasSuffix(os.Args[0], ".test") {
		return
	}
	pruneToDefaultSpine(rootCmd)
}

// pruneToDefaultSpine applies the production membership boundary and returns
// the commands it removed. Tests use the returned slice to restore the full
// registered test tree after checking the production view.
func pruneToDefaultSpine(root *cobra.Command) []*cobra.Command {
	var removed []*cobra.Command
	for _, command := range append([]*cobra.Command(nil), root.Commands()...) {
		if _, retained := defaultSpineCommands[command.Name()]; retained || command.Name() == "completion" || command.Name() == "help" {
			continue
		}
		root.RemoveCommand(command)
		removed = append(removed, command)
	}
	return removed
}

func restorePrunedCommands(root *cobra.Command, commands []*cobra.Command) {
	root.AddCommand(commands...)
}
