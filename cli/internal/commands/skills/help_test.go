// practices: [design-by-contract, code-complete]
package skills

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// linkLong / unlinkLong fetch the built command's Long help so the help
// contracts are asserted against the real module tree.
func subcommandLong(t *testing.T, name string) string {
	t.Helper()
	root := NewModule(HostOptions{DryRun: func() bool { return false }}).Command()
	var found *cobra.Command
	for _, c := range root.Commands() {
		if c.Name() == name {
			found = c
			break
		}
	}
	if found == nil {
		t.Fatalf("subcommand %q not registered", name)
	}
	return found.Long
}

// The command's --help is the user-facing contract for the optional track-main
// install path (age-4asp) and for its multi-runtime coverage (age-ivcba):
// documenting them there is the whole point, so guard against silent removal.
func TestSkillsLinkHelp_DocumentsTrackMainAndRuntimes(t *testing.T) {
	long := subcommandLong(t, "link")
	for _, want := range []string{"Track main", "git pull && ao skills link", "~/.agents/skills", "~/.codex/skills", "~/.gemini/skills", "no reinstall or plugin cache"} {
		if !strings.Contains(long, want) {
			t.Errorf("`ao skills link --help` no longer documents %q", want)
		}
	}
}

// The command's --help is the user-facing contract for the documented rollback
// path: it must name the inverse relationship, the multi-runtime coverage, and
// the dry-run rehearsal. Guard against silent removal.
func TestSkillsUnlinkHelp_DocumentsRollbackAndRuntimes(t *testing.T) {
	long := subcommandLong(t, "unlink")
	for _, want := range []string{"inverse", "track main", "~/.codex/skills", "~/.gemini/skills", "--dry-run"} {
		if !strings.Contains(long, want) {
			t.Errorf("`ao skills unlink --help` no longer documents %q", want)
		}
	}
}
