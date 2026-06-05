// practices: [pragmatic-programmer, twelve-factor-app]
package main

import (
	"fmt"
	"os"
	"sort"
	"testing"
)

// TestDumpRegisteredTopLevelCommands is the machine source-of-truth for the
// command-landing regenerator (scripts/regen-command-surfaces.sh, bead ag-jy12).
//
// It walks the LIVE cobra command tree (rootCmd.Commands()) — the same set the
// production CLI registers — and, when AO_DUMP_REGISTERED_CMDS=1 is set, prints
// every registered top-level command name (one per line, sorted, "help"
// excluded) to stdout. This is exactly the membership the two expectedCmds
// literals in cobra_commands_test.go must match, so the regenerator can derive
// them deterministically instead of hand-editing N copies.
//
// Why a test and not a CLI subcommand: emitting the list from a real subcommand
// would itself add a registered command (circular — it would need adding to
// expectedCmds). A test reuses the live rootCmd without touching the surface.
//
// When AO_DUMP_REGISTERED_CMDS is unset the test is a no-op assertion that the
// command set is non-empty, so it stays cheap in the normal `go test ./...` run.
func TestDumpRegisteredTopLevelCommands(t *testing.T) {
	var names []string
	for _, c := range rootCmd.Commands() {
		if c.Name() == "help" {
			// cobra auto-adds `help`; expectedCmds excludes it by contract.
			continue
		}
		names = append(names, c.Name())
	}
	sort.Strings(names)

	if len(names) == 0 {
		t.Fatal("no top-level commands registered on rootCmd")
	}

	if os.Getenv("AO_DUMP_REGISTERED_CMDS") == "1" {
		for _, n := range names {
			fmt.Println(n)
		}
	}
}
