// practices: [pragmatic-programmer]
package main

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

// advertisedAoInvocationRE extracts `ao <subcommand>...` invocations from
// user-facing text (help output, seeded CLAUDE.md sections, readiness
// actions). A token run ends at the first thing that is not a lowercase
// command word or a --flag, so placeholders like <topic> and prose punctuation
// never leak into the parsed invocation.
var advertisedAoInvocationRE = regexp.MustCompile(
	"(?:^|[\\s`\"'($])ao ([a-z][a-z0-9-]*(?: (?:[a-z][a-z0-9-]*|--[a-z][a-z0-9-]*(?:=\\S+)?))*)")

// extractAdvertisedAoInvocations returns every `ao ...` command string
// advertised in text, without the leading "ao ".
func extractAdvertisedAoInvocations(text string) []string {
	matches := advertisedAoInvocationRE.FindAllStringSubmatch(text, -1)
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		out = append(out, m[1])
	}
	return out
}

// validateAdvertisedAoInvocation checks that an advertised invocation (the
// part after "ao ") resolves against the live cobra command tree: every token
// must reach a registered command, group commands (no Run) must be followed
// by a real subcommand rather than a positional arg, and every --flag must be
// defined on the resolved command. This is the guard that keeps removed
// commands (ao factory, ao orchestrate, ao autodev, ...) from being
// advertised in fresh-install output again.
func validateAdvertisedAoInvocation(root *cobra.Command, invocation string) error {
	tokens := strings.Fields(invocation)
	cmd := root
	i := 0
	for ; i < len(tokens); i++ {
		tok := tokens[i]
		if strings.HasPrefix(tok, "-") {
			break
		}
		next := findAdvertisedSubcommand(cmd, tok)
		if next == nil {
			if isCommandGroup(cmd) {
				return fmt.Errorf("%q is not a subcommand of %q", tok, cmd.CommandPath())
			}
			// Runnable command: the remaining word tokens are positional args.
			break
		}
		cmd = next
	}
	for ; i < len(tokens); i++ {
		tok := tokens[i]
		if !strings.HasPrefix(tok, "--") {
			continue
		}
		name, _, _ := strings.Cut(strings.TrimPrefix(tok, "--"), "=")
		if cmd.Flags().Lookup(name) == nil && cmd.InheritedFlags().Lookup(name) == nil {
			return fmt.Errorf("flag --%s is not defined on %q", name, cmd.CommandPath())
		}
	}
	return nil
}

func findAdvertisedSubcommand(cmd *cobra.Command, name string) *cobra.Command {
	for _, c := range cmd.Commands() {
		if c.Name() == name || c.HasAlias(name) {
			return c
		}
	}
	return nil
}

// isCommandGroup reports whether cmd only routes to subcommands (it has no
// Run of its own), so a following token must be a real subcommand.
func isCommandGroup(cmd *cobra.Command) bool {
	return cmd.HasSubCommands() && cmd.Run == nil && cmd.RunE == nil
}
