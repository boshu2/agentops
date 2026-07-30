// Package gc owns Cobra presentation for the `ao gc` command family: the
// stock Gas City maintainer operations (prepare, check, recover-affinity)
// ported from scripts/gc-maintainer-ops.sh per ADR-0016. Every filesystem and
// process effect is delegated to internal/gcmaintainer; this package performs
// no direct effect.
package gc

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/clicontract"
	"github.com/boshu2/agentops/cli/internal/gcmaintainer"
)

// Module owns Cobra presentation for the gc command family.
type Module struct {
	host clicontract.HostOptions

	city         string
	rig          string
	gcBin        string
	packDir      string
	skillsSource string
	apply        bool
}

// NewModule constructs the gc command module from its host seams.
func NewModule(host clicontract.HostOptions) *Module {
	return &Module{host: host}
}

// Command builds the `ao gc` command tree.
func (m *Module) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "gc",
		GroupID: "workflow",
		Short:   "Gas City maintainer operations for stock rigs",
		Long: `Prepare and qualify the stock Gas City maintainer pack without owning a pack.

prepare verifies the official workflow and rig-role pins, snapshots upstream
validation assets unchanged under the rig's ignored .gc directory, installs
AgentOps-owned check wrappers, and links AgentOps skills into city/rig Codex
sinks. check is read-only. recover-affinity only considers ready formula beads
with gc.session_affinity=require and never re-slings work.`,
	}
	cmd.AddCommand(m.prepareCommand(), m.checkCommand(), m.recoverAffinityCommand())
	return cmd
}

func (m *Module) options() gcmaintainer.Options {
	return gcmaintainer.Options{
		City:         m.city,
		Rig:          m.rig,
		GCBin:        m.gcBin,
		PackDir:      m.packDir,
		SkillsSource: m.skillsSource,
		Apply:        m.apply,
	}
}

func (m *Module) dryRun() bool {
	return m.host.DryRun != nil && m.host.DryRun()
}

// effectiveApply resolves the recover-affinity write switch: --apply requests
// the recovery, but the global --dry-run always wins and forces the read-only
// dry run.
func (m *Module) effectiveApply() bool {
	return m.apply && !m.dryRun()
}

// addCommonFlags wires the flags shared by every maintainer operation.
func (m *Module) addCommonFlags(cmd *cobra.Command, withSkillsSource bool) {
	cmd.Flags().StringVar(&m.city, "city", "", "Gas City root directory (required)")
	cmd.Flags().StringVar(&m.rig, "rig", "", "rig directory inside the city (required)")
	cmd.Flags().StringVar(&m.gcBin, "gc-bin", "", "Gas City 1.4 binary (default: gc on PATH)")
	cmd.Flags().StringVar(&m.packDir, "pack-dir", "", "resolved official gascity pack root (normally auto-detected)")
	if withSkillsSource {
		cmd.Flags().StringVar(&m.skillsSource, "skills-source", "", "AgentOps skills directory to link from (default: enclosing checkout, then installed skills root)")
	}
	_ = cmd.MarkFlagRequired("city")
	_ = cmd.MarkFlagRequired("rig")
}

func (m *Module) prepareCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "prepare",
		Short: "Stage the contained maintainer runtime and skill links for a rig",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if m.dryRun() {
				return fmt.Errorf("prepare does not support --dry-run; use `ao gc check` for a read-only inspection")
			}
			opts := m.options()
			opts.Stdout = cmd.OutOrStdout()
			opts.Stderr = cmd.ErrOrStderr()
			return gcmaintainer.Prepare(opts)
		},
	}
	m.addCommonFlags(cmd, true)
	return cmd
}

func (m *Module) checkCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "check",
		Short: "Verify a prepared maintainer runtime read-only",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			opts := m.options()
			opts.Stdout = cmd.OutOrStdout()
			opts.Stderr = cmd.ErrOrStderr()
			return gcmaintainer.Check(opts)
		},
	}
	m.addCommonFlags(cmd, true)
	return cmd
}

func (m *Module) recoverAffinityCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "recover-affinity",
		Short: "Clear stale required session-affinity assignments (dry-run by default)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			opts := m.options()
			opts.Apply = m.effectiveApply()
			opts.Stdout = cmd.OutOrStdout()
			opts.Stderr = cmd.ErrOrStderr()
			return gcmaintainer.RecoverAffinity(opts)
		},
	}
	m.addCommonFlags(cmd, false)
	cmd.Flags().BoolVar(&m.apply, "apply", false, "apply the recovery; the default is a read-only dry run")
	return cmd
}
