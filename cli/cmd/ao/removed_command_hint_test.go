// practices: [ai-assisted-dev, pragmatic-programmer]
package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// bareRoot returns a root command with no subcommands registered, so every
// tombstoned verb is genuinely unknown to it (the default-build shape).
func bareRoot() *cobra.Command {
	return &cobra.Command{Use: "ao"}
}

func TestRemovedCommandHint_TombstonedVerbs(t *testing.T) {
	cases := []struct {
		verb     string
		wantFrag []string // every fragment must appear in the hint
	}{
		{
			verb: "pawl",
			wantFrag: []string{
				`"pawl" was removed`,
				"docs/MIGRATION.md",
				"Validate skill",
			},
		},
		{
			verb: "land",
			wantFrag: []string{
				`"land" was removed`,
				"docs/MIGRATION.md",
				"Git or CI",
			},
		},
		{
			verb: "crank",
			wantFrag: []string{
				`"crank" was removed`,
				"docs/MIGRATION.md",
				"dispatch_once",
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.verb, func(t *testing.T) {
			err := fmt.Errorf("unknown command %q for %q", tc.verb, "ao")
			hint := removedCommandHint(bareRoot(), err)
			if hint == "" {
				t.Fatalf("removedCommandHint(%q) = empty; want a tombstone hint", tc.verb)
			}
			for _, frag := range tc.wantFrag {
				if !strings.Contains(hint, frag) {
					t.Errorf("hint for %q missing %q; got:\n%s", tc.verb, frag, hint)
				}
			}
		})
	}
}

func TestRemovedCommandHint_NotTombstoned(t *testing.T) {
	if got := removedCommandHint(bareRoot(), fmt.Errorf("unknown command %q for %q", "frobnicate", "ao")); got != "" {
		t.Errorf("non-tombstoned verb produced a hint: %q", got)
	}
	if got := removedCommandHint(bareRoot(), errors.New("some other failure")); got != "" {
		t.Errorf("non-unknown-command error produced a hint: %q", got)
	}
	if got := removedCommandHint(bareRoot(), nil); got != "" {
		t.Errorf("nil error produced a hint: %q", got)
	}
}

// A flywheel/legacy build registers some tombstoned verbs as real commands.
// The hint must stay silent for any verb the running binary actually serves,
// so a restored command never gets a "was removed" message on a usage error.
func TestRemovedCommandHint_RegisteredVerbSuppressed(t *testing.T) {
	root := bareRoot()
	root.AddCommand(&cobra.Command{Use: "pawl"})
	err := fmt.Errorf("unknown command %q for %q", "pawl", "ao")
	if got := removedCommandHint(root, err); got != "" {
		t.Errorf("registered verb produced a tombstone hint: %q", got)
	}
}

// Drift gate: every tombstoned verb must have a row (a literal mention) in
// docs/MIGRATION.md, so the point-of-failure hint and the migration map can
// never disagree about what was removed.
func TestRemovedVerbsHaveMigrationRows(t *testing.T) {
	path := findMigrationDoc(t)
	raw, err := os.ReadFile(path) // #nosec G304 -- repo-internal doc resolved by walking up from the test cwd
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	doc := string(raw)
	for verb := range removedCommands {
		if !strings.Contains(doc, verb) {
			t.Errorf("tombstoned verb %q has no mention in docs/MIGRATION.md — add a row or drop the tombstone", verb)
		}
	}
}

// The root help must never pitch a verb the default build does not serve
// (the pre-3.0 text sold `ao factory start` while `ao factory` errored).
func TestRootHelpNamesNoRemovedVerbs(t *testing.T) {
	help := rootCmd.Short + "\n" + rootCmd.Long
	for verb := range removedCommands {
		needle := "ao " + verb
		if strings.Contains(help, needle) {
			t.Errorf("root help still pitches removed verb %q — a dev following --help hits an unknown-command error", needle)
		}
	}
}

// findMigrationDoc walks up from the test's working directory to the repo
// root and returns the path to docs/MIGRATION.md.
func findMigrationDoc(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for i := 0; i < 8; i++ {
		candidate := filepath.Join(dir, "docs", "MIGRATION.md")
		if _, statErr := os.Stat(candidate); statErr == nil {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatal("docs/MIGRATION.md not found walking up from test cwd")
	return ""
}
