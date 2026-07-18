// practices: [ai-assisted-dev, pragmatic-programmer]
package main

import (
	"encoding/json"
	"sort"

	"github.com/spf13/cobra"
)

// groupCommandListing is the JSON an agent receives when it runs a parent
// command (one with subcommands but no action of its own) with --json.
// Without this, cobra prints human help text to a caller that asked for a
// machine-readable structure.
type groupCommandListing struct {
	Command     string         `json:"command"`
	Description string         `json:"description"`
	IsGroup     bool           `json:"is_group"`
	Subcommands []groupCommand `json:"subcommands"`
	Hint        string         `json:"hint"`
}

type groupCommand struct {
	Name  string `json:"name"`
	Short string `json:"short"`
}

// maybeEmitGroupJSON reports whether cmd is a non-runnable parent command
// invoked with --json, and if so writes a JSON listing of its subcommands.
// Returns true when it handled the command (the caller should then stop).
// groupGuardAnnotation marks a parent whose only Run function is the
// unknown-subcommand guard; for help purposes it is still a group.
const groupGuardAnnotation = "ao.unknown-subcommand-guard"

func maybeEmitGroupJSON(cmd *cobra.Command) bool {
	if cmd == nil || !cmd.HasAvailableSubCommands() {
		return false
	}
	if cmd.Runnable() && cmd.Annotations[groupGuardAnnotation] != "true" {
		return false
	}
	// PersistentPreRunE (which syncs --json into output) is skipped by cobra
	// for non-runnable commands, so check the raw --json flag too.
	if !jsonFlag && output != "json" {
		return false
	}
	var subs []groupCommand
	for _, c := range cmd.Commands() {
		if c.Hidden || c.Name() == "help" {
			continue
		}
		subs = append(subs, groupCommand{Name: c.Name(), Short: c.Short})
	}
	sort.Slice(subs, func(i, j int) bool { return subs[i].Name < subs[j].Name })
	listing := groupCommandListing{
		Command:     cmd.CommandPath(),
		Description: cmd.Short,
		IsGroup:     true,
		Subcommands: subs,
		Hint:        "run '" + cmd.CommandPath() + " <subcommand> --json' for a specific operation",
	}
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	_ = enc.Encode(listing)
	return true
}
