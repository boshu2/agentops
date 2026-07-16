// practices: [pragmatic-programmer, agile-manifesto]
package main

import (
	"sort"

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
