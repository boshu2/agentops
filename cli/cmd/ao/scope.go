// Package main: `ao scope` cobra subcommand for the /scope skill (issue
// soc-irg1.3). Manages .agents/scope.lock via cli/internal/scope.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/scope"
)

// defaultScopeLockPath returns `.agents/scope.lock` relative to the current
// working directory. Wave 1 hardcodes this path; Wave 2 (issue I5) routes via
// lib/ao-paths.sh / cli/internal/paths.
func defaultScopeLockPath() string {
	cwd, err := os.Getwd()
	if err != nil {
		return filepath.Join(".agents", "scope.lock")
	}
	return filepath.Join(cwd, ".agents", "scope.lock")
}

var (
	scopeJSON     bool
	scopeLockFlag string
)

var scopeCmd = &cobra.Command{
	Use:   "scope",
	Short: "Manage edit-scope guard (freeze, unfreeze, status)",
	Long: `Declare which directories are in scope for the current work session.

Edits outside the declared scope are hard-blocked by the
hooks/edit-scope-guard.sh PreToolUse hook. State lives in
.agents/scope.lock; mutations go through atomic temp+rename writes
(cli/internal/llmwiki.SafeAtomicWrite).`,
}

var scopeFreezeCmd = &cobra.Command{
	Use:   "freeze <dir> [<dir>...]",
	Short: "Freeze one or more directories (additive)",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := scopeLockPath()
		if err := scope.Freeze(path, args); err != nil {
			return err
		}
		l, err := scope.Read(path)
		if err != nil {
			return err
		}
		return printStatus(cmd.OutOrStdout(), l, scopeJSON)
	},
}

var scopeUnfreezeCmd = &cobra.Command{
	Use:   "unfreeze [<dir>...]",
	Short: "Unfreeze one (or all if no arg) directories",
	RunE: func(cmd *cobra.Command, args []string) error {
		path := scopeLockPath()
		if err := scope.Unfreeze(path, args); err != nil {
			return err
		}
		l, err := scope.Read(path)
		if err != nil {
			return err
		}
		return printStatus(cmd.OutOrStdout(), l, scopeJSON)
	},
}

var scopeStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current scope-lock state",
	RunE: func(cmd *cobra.Command, args []string) error {
		path := scopeLockPath()
		l, err := scope.Read(path)
		if err != nil {
			return err
		}
		return printStatus(cmd.OutOrStdout(), l, scopeJSON)
	},
}

func scopeLockPath() string {
	if scopeLockFlag != "" {
		return scopeLockFlag
	}
	if v := os.Getenv("AO_SCOPE_LOCK"); v != "" {
		return v
	}
	return defaultScopeLockPath()
}

func printStatus(w interface{ Write([]byte) (int, error) }, l *scope.Lock, asJSON bool) error {
	if l == nil {
		return errors.New("scope: nil lock")
	}
	if asJSON {
		enc := json.NewEncoder(w.(interface{ Write(p []byte) (n int, err error) }))
		enc.SetIndent("", "  ")
		return enc.Encode(l)
	}
	if len(l.FrozenDirs) == 0 {
		_, err := fmt.Fprintln(w, "scope: no frozen directories (enforcement off)")
		return err
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("scope: %d frozen director", len(l.FrozenDirs)))
	if len(l.FrozenDirs) == 1 {
		b.WriteString("y")
	} else {
		b.WriteString("ies")
	}
	b.WriteString(" (acquired_at=")
	if !l.AcquiredAt.IsZero() {
		b.WriteString(l.AcquiredAt.UTC().Format("2006-01-02T15:04:05Z"))
	} else {
		b.WriteString("unknown")
	}
	b.WriteString(", acquired_by=")
	if l.AcquiredBy != "" {
		b.WriteString(l.AcquiredBy)
	} else {
		b.WriteString("unknown")
	}
	b.WriteString(")\n")
	for _, d := range l.FrozenDirs {
		b.WriteString("  - ")
		b.WriteString(d)
		b.WriteString("\n")
	}
	_, err := fmt.Fprint(w, b.String())
	return err
}

func init() {
	scopeCmd.PersistentFlags().BoolVar(&scopeJSON, "json", false, "Emit JSON output")
	scopeCmd.PersistentFlags().StringVar(&scopeLockFlag, "lock", "", "Override scope-lock path (defaults to $AO_SCOPE_LOCK or .agents/scope.lock)")
	scopeCmd.AddCommand(scopeFreezeCmd)
	scopeCmd.AddCommand(scopeUnfreezeCmd)
	scopeCmd.AddCommand(scopeStatusCmd)
	rootCmd.AddCommand(scopeCmd)
}
