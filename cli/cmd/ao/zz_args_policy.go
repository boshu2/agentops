package main

import (
	"strings"

	"github.com/spf13/cobra"
)

// init runs after command declaration files (the zz prefix is deliberate) and
// closes the compatibility gap for the pre-existing tree. New declarations
// should still set Args adjacent to Use; this full-tree pass is the ratchet that
// prevents legacy nil policy from surviving indefinitely.
func init() {
	// Cobra otherwise synthesizes `help` lazily during execution. Initialize it
	// before the whole-tree policy pass so test order cannot expose a public
	// runnable with nil Args. Help accepts command paths of arbitrary depth.
	rootCmd.InitDefaultHelpCmd()
	for _, command := range rootCmd.Commands() {
		if command.Name() == "help" {
			command.Args = cobra.ArbitraryArgs
			break
		}
	}
	declareMissingArgsPolicies(rootCmd)
	requireKnownSubcommands(rootCmd)
}

func declareMissingArgsPolicies(parent *cobra.Command) {
	for _, command := range parent.Commands() {
		if (command.Run != nil || command.RunE != nil) && command.Args == nil {
			if command.DisableFlagParsing {
				command.Args = cobra.ArbitraryArgs
			} else {
				command.Args = compatibleArgsPolicy(command.Use)
			}
		}
		declareMissingArgsPolicies(command)
	}
}

func compatibleArgsPolicy(use string) cobra.PositionalArgs {
	fields := strings.Fields(use)
	required, optional := 0, 0
	variadic := false
	optionValue := false
	for _, field := range fields[1:] {
		if optionValue {
			optionValue = false
			continue
		}
		trimmed := strings.TrimLeft(field, "[")
		if strings.HasPrefix(trimmed, "-") {
			optionValue = !strings.Contains(field, "=")
			continue
		}
		if field == "[flags]" {
			continue
		}
		if strings.Contains(field, "...") {
			variadic = true
		}
		switch {
		case strings.HasPrefix(field, "<"):
			required++
		case strings.HasPrefix(field, "["):
			optional++
		}
	}
	if variadic {
		return cobra.MinimumNArgs(required)
	}
	if optional > 0 {
		return cobra.RangeArgs(required, required+optional)
	}
	if required > 0 {
		return cobra.ExactArgs(required)
	}
	return cobra.NoArgs
}

// requireKnownSubcommands walks the tree and installs the unknown-subcommand
// guard on every non-runnable parent, so `ao <parent> <typo>` fails with an
// unknown-command error (translated by removedCommandHint for retired
// children) instead of printing help with exit 0.
func requireKnownSubcommands(parent *cobra.Command) {
	for _, command := range parent.Commands() {
		if command.HasSubCommands() {
			if !command.Runnable() {
				requireKnownSubcommand(command)
			}
			requireKnownSubcommands(command)
		}
	}
}
