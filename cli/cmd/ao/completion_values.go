package main

import (
	"sort"

	"github.com/boshu2/agentops/cli/internal/lifecycle"
	"github.com/spf13/cobra"
)

// staticCompletionFunc returns a cobra flag-completion function that proposes a
// fixed, sorted list of values and suppresses file completion. Used for flags
// whose valid values are a known enumerated set.
func staticCompletionFunc(values ...string) func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	sorted := make([]string, len(values))
	copy(sorted, values)
	sort.Strings(sorted)
	return func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return sorted, cobra.ShellCompDirectiveNoFileComp
	}
}

// templateCompletionValues returns the sorted list of seed/goals-init template
// names. Derived from lifecycle.ValidTemplates so the CLI stays in lockstep
// with the validation source of truth.
func templateCompletionValues() []string {
	names := make([]string, 0, len(lifecycle.ValidTemplates))
	for name, enabled := range lifecycle.ValidTemplates {
		if enabled {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}
