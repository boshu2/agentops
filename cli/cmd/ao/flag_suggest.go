// practices: [agent-ergonomics]
package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// flagSuggestMaxDistance is the largest edit distance at which an unknown
// flag is treated as a typo of a known flag worth suggesting.
const flagSuggestMaxDistance = 2

// flagErrorWithSuggestion is a cobra FlagErrorFunc that turns an opaque
// "unknown flag: --jsno" into an actionable hint naming the exact flag the
// agent should have used. A single typo then teaches the correct spelling
// instead of wedging the caller. Wired on rootCmd (inherited by every
// subcommand) and on the doctor surface.
func flagErrorWithSuggestion(cmd *cobra.Command, err error) error {
	bad := parseUnknownFlag(err.Error())
	if bad == "" {
		return err
	}
	if guess := closestKnownFlag(cmd, bad); guess != "" {
		return fmt.Errorf("%w\n\nDid you mean --%s?\n  Run: %s --%s ...", err, guess, cmd.CommandPath(), guess)
	}
	return fmt.Errorf("%w\n\nRun '%s --help' to see available flags", err, cmd.CommandPath())
}

// parseUnknownFlag extracts the offending flag name from a pflag error string
// such as "unknown flag: --jsno". Returns "" when the error is not an
// unknown-flag error.
func parseUnknownFlag(msg string) string {
	const prefix = "unknown flag: --"
	idx := strings.Index(msg, prefix)
	if idx < 0 {
		return ""
	}
	rest := msg[idx+len(prefix):]
	if cut := strings.IndexAny(rest, " \t\n"); cut >= 0 {
		rest = rest[:cut]
	}
	return rest
}

// closestKnownFlag returns the known flag name nearest to bad within
// flagSuggestMaxDistance edits, or "" when nothing is close enough.
func closestKnownFlag(cmd *cobra.Command, bad string) string {
	best := ""
	bestDist := flagSuggestMaxDistance + 1
	consider := func(f *pflag.Flag) {
		if f.Hidden {
			return
		}
		if d := levenshtein(bad, f.Name); d < bestDist {
			bestDist, best = d, f.Name
		}
	}
	cmd.Flags().VisitAll(consider)
	cmd.InheritedFlags().VisitAll(consider)
	return best
}

// levenshtein computes the edit distance between two ASCII strings.
func levenshtein(a, b string) int {
	if a == b {
		return 0
	}
	prev := make([]int, len(b)+1)
	curr := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		curr[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			curr[j] = min3(curr[j-1]+1, prev[j]+1, prev[j-1]+cost)
		}
		prev, curr = curr, prev
	}
	return prev[len(b)]
}

// printRequiredFlagHint enriches cobra's terse "required flag(s) ... not set"
// error with the offending command's usage line and example, so an agent that
// forgot a flag sees exactly how to invoke the command correctly. Cobra has
// already printed the bare "Error: ..." line; this appends the actionable
// part to stderr. No-op for any other error.
func printRequiredFlagHint(cmd *cobra.Command, err error) {
	writeRequiredFlagHint(os.Stderr, cmd, err)
}

// writeRequiredFlagHint is printRequiredFlagHint with an explicit writer so the
// behavior is unit-testable.
func writeRequiredFlagHint(w io.Writer, cmd *cobra.Command, err error) {
	if cmd == nil || err == nil {
		return
	}
	if !strings.Contains(err.Error(), "required flag(s)") {
		return
	}
	fmt.Fprintf(w, "\nUsage: %s\n", cmd.UseLine())
	if ex := strings.TrimSpace(cmd.Example); ex != "" {
		fmt.Fprintf(w, "Example:\n%s\n", ex)
	} else {
		fmt.Fprintf(w, "Run '%s --help' for details.\n", cmd.CommandPath())
	}
}

func min3(a, b, c int) int {
	m := a
	if b < m {
		m = b
	}
	if c < m {
		m = c
	}
	return m
}
