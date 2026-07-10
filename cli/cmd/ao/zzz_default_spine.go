package main

import "github.com/spf13/cobra"

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
	if len(archiveBuildTags) != 0 {
		return
	}
	for _, command := range append([]*cobra.Command(nil), rootCmd.Commands()...) {
		if _, retained := defaultSpineCommands[command.Name()]; retained || command.Hidden || command.Name() == "completion" {
			continue
		}
		rootCmd.RemoveCommand(command)
	}
}
