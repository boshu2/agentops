// practices: [pragmatic-programmer, twelve-factor-app]
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// helpPurityAllowlist lists full command labels (space-joined, e.g. "ao foo bar")
// whose --help output is PERMITTED to contain an absolute path. It is EMPTY by
// default: no command should leak a machine-specific path into its help.
//
// Add an entry ONLY with a comment explaining why an absolute path is intrinsic
// to that command's help and cannot be expressed as a placeholder (e.g. a fixed
// system path like /etc/... that is genuinely part of the contract). A leaked
// $REPO_ROOT / $HOME / temp path is NEVER a valid reason — fix the command to
// intercept -h/--help before printing the path, or print a placeholder. (age-je8h)
var helpPurityAllowlist = map[string]bool{}

// pathNeedle is one machine-specific absolute-path prefix that must never appear
// in a command's --help output.
type pathNeedle struct {
	name  string
	value string
}

// TestHelpOutputHasNoAbsolutePaths walks the WHOLE cobra command tree and invokes
// each command's --help, asserting the output carries no machine-specific ABSOLUTE
// path (the checkout root, the user's home, or a temp dir).
//
// This is the source-level half of the COMMANDS.md determinism class-fix (age-je8h).
// scripts/generate-cli-reference.sh now scrubs absolute paths out of the committed
// doc, so the doc stays byte-identical across worktrees; but a command that leaks
// $REPO_ROOT / $HOME / $TMPDIR THROUGH its --help (classically: a DisableFlagParsing
// command whose RunE prints a checkout path via an RCE/trust guard before it
// intercepts -h/--help) is a real UX defect that the scrub would otherwise silently
// mask. This catches the leak at its source.
//
// It reproduces the generator faithfully by EXECUTING each command with `--help`
// through rootCmd (via the shared executeCommand helper, which snapshots and
// restores all shared cobra flag state), so RunE-level leaks are caught — not just
// static help templates. executeCommand restores rootCmd's writers/args and every
// global flag on each call, satisfying the .claude/rules/go.md test-isolation rule.
func TestHelpOutputHasNoAbsolutePaths(t *testing.T) {
	// Resolve the checkout root from the ORIGINAL cwd before we chdir away: a RunE
	// leak would print this exact absolute path.
	repoRoot := findEnclosingRepoRoot(t)

	// Hermetic cwd: DisableFlagParsing commands run rootCmd's PersistentPreRunE
	// (git-env sanitize + shared-worktree-config repair) even on --help. Point it at
	// a non-repo temp dir so the repair no-ops and never mutates the real checkout.
	// t.Chdir auto-restores on cleanup and fails fast under parallel tests.
	t.Chdir(t.TempDir())

	needles := absolutePathNeedles(t, repoRoot)

	var labels []string
	var argvs [][]string
	collectCommandInvocations(rootCmd, nil, &labels, &argvs)
	if len(labels) == 0 {
		t.Fatal("walked cobra tree but collected no commands")
	}

	for i, label := range labels {
		args := append(append([]string{}, argvs[i]...), "--help")
		out, err := executeCommand(args...)
		if err != nil {
			t.Errorf("%s --help returned an error (a command's --help must never fail): %v", label, err)
			continue
		}
		if helpPurityAllowlist[label] {
			continue
		}
		for _, n := range needles {
			if n.value == "" {
				continue
			}
			if strings.Contains(out, n.value) {
				t.Errorf("%s --help leaks a machine-specific absolute path (%s = %q).\n"+
					"A DisableFlagParsing command must intercept -h/--help and return cmd.Help() BEFORE printing any path; "+
					"otherwise print a placeholder. If the path is genuinely intrinsic to this command's help, add %q to "+
					"helpPurityAllowlist with a justifying comment.",
					label, n.name, n.value, label)
			}
		}
	}
}

// collectCommandInvocations walks the cobra tree rooted at cmd, appending one
// (label, argv) pair per command — including cmd itself. label is the space-joined
// human path ("ao foo bar"); argv is the arg slice to pass to executeCommand
// (["foo", "bar"]). The root contributes label "ao" and an empty argv.
func collectCommandInvocations(cmd *cobra.Command, path []string, labels *[]string, argvs *[][]string) {
	label := "ao"
	if len(path) > 0 {
		label = "ao " + strings.Join(path, " ")
	}
	*labels = append(*labels, label)
	*argvs = append(*argvs, append([]string{}, path...))

	for _, child := range cmd.Commands() {
		collectCommandInvocations(child, append(path, child.Name()), labels, argvs)
	}
}

// absolutePathNeedles returns the machine-specific absolute-path prefixes a
// command's --help must never contain. Home roots ("/Users/", "/home/") are broad
// backstops; $HOME, the checkout root, and the temp dir are this run's concrete
// values (the exact strings a RunE leak would emit).
func absolutePathNeedles(t *testing.T, repoRoot string) []pathNeedle {
	t.Helper()
	needles := []pathNeedle{
		{name: "macOS home root", value: "/Users/"},
		{name: "Linux home root", value: "/home/"},
		{name: "temp root", value: "/tmp/"},
	}
	if home := os.Getenv("HOME"); home != "" {
		needles = append(needles, pathNeedle{name: "$HOME", value: home})
	}
	if repoRoot != "" {
		needles = append(needles, pathNeedle{name: "checkout root", value: repoRoot})
	}
	if tmp := os.Getenv("TMPDIR"); tmp != "" {
		trimmed := strings.TrimRight(tmp, "/")
		needles = append(needles, pathNeedle{name: "$TMPDIR", value: trimmed + "/"})
		// macOS canonicalizes $TMPDIR (/var/folders/...) to its /private-prefixed realpath
		// (/private/var/folders/...) via EvalSymlinks (the /var -> /private/var symlink); a
		// command that resolves its temp path leaks THAT form, which the raw-$TMPDIR needle
		// above would miss. Catch the canonicalized temp root at the source too. (age-i9ce)
		if canon, err := filepath.EvalSymlinks(tmp); err == nil && strings.TrimRight(canon, "/") != trimmed {
			needles = append(needles, pathNeedle{name: "canonicalized $TMPDIR", value: strings.TrimRight(canon, "/") + "/"})
		}
	}
	return needles
}

// findEnclosingRepoRoot walks up from the test's initial working directory to the
// nearest ancestor holding a `.git` entry (a linked worktree carries a `.git`
// FILE), returning that absolute path. Returns "" if none is found (the checkout-
// root needle is then simply skipped — the $HOME and home-root needles still catch
// any leaked path under the user's home).
func findEnclosingRepoRoot(t *testing.T) string {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	cur := cwd
	for {
		if _, statErr := os.Stat(filepath.Join(cur, ".git")); statErr == nil {
			return cur
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return ""
		}
		cur = parent
	}
}
