package main

import (
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// defaultSpineCommands is the executable ADR-0012 membership boundary. The
// archive tags restore the complete registered tree; an untagged production
// build removes satellite registrations so those paths cannot execute.
var defaultSpineCommands = map[string]struct{}{
	"beads": {}, "capabilities": {}, "claim": {}, "close": {},
	"config": {}, "council-gate": {}, "doctor": {}, "done": {},
	"eval": {}, "gate": {}, "goals": {}, "governor": {},
	"init": {}, "land": {}, "membrane": {}, "pawl": {},
	"plan-pawl": {}, "provenance": {}, "quick-start": {}, "ready": {},
	"robot-docs": {}, "session": {}, "skills": {}, "status": {},
	"validate": {}, "verdict-gate": {}, "verify": {}, "version": {},
	"yield": {},
}

func init() {
	// The package test binary deliberately retains every registration so focused
	// tests for archived commands remain runnable without compiling the suite
	// repeatedly under every tag. Production/default binaries take the boundary.
	if len(archiveBuildTags) != 0 || strings.HasSuffix(os.Args[0], ".test") {
		return
	}
	for _, command := range append([]*cobra.Command(nil), rootCmd.Commands()...) {
		if _, retained := defaultSpineCommands[command.Name()]; retained || command.Hidden || command.Name() == "completion" {
			continue
		}
		rootCmd.RemoveCommand(command)
	}
}
