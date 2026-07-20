// Package goals owns Cobra presentation for the `ao goals` command family. The
// module builds its command tree with constructor-scoped flag state and host
// seams, delegating every filesystem, process, and clock effect to
// internal/goals so the module itself performs no direct effect.
package goals

import (
	"bytes"
	"io"
	"time"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/clicontract"
	goalsapp "github.com/boshu2/agentops/cli/internal/goals"
)

// defaultGoalsTimeoutSeconds is the per-check measurement timeout. It is sized
// to cover the repository's own long-running race gate.
const defaultGoalsTimeoutSeconds = 240

// Module owns Cobra presentation for the goals command family.
type Module struct {
	host clicontract.HostOptions
}

// NewModule constructs the goals command module from its host seams.
func NewModule(host clicontract.HostOptions) Module {
	return Module{host: host}
}

// Contract declares the goals family's behavior. Goals reads GOALS.md/GOALS.yaml
// and runs gate checks (filesystem, process, clock), emitting a human table or
// JSON under the global -o/--output flag.
func (Module) Contract() clicontract.CommandContract {
	return clicontract.CommandContract{
		ID:       "ao.goals",
		Profiles: clicontract.ProfileDefault | clicontract.ProfileFlywheel | clicontract.ProfileLegacy | clicontract.ProfileCombined,
		Args:     clicontract.ArgsPolicy{Name: "no-args", Validate: cobra.NoArgs},
		Output:   clicontract.OutputNone,
		Effects:  clicontract.EffectFilesystem | clicontract.EffectProcess | clicontract.EffectClock,
		ExitClasses: map[int]clicontract.ExitClass{
			0: clicontract.ExitSuccess,
			1: clicontract.ExitFailure,
		},
	}
}

func (module Module) jsonOutput() bool { return module.host.OutputMode() == "json" }

// renderStructured runs a goalsapp entry point and honors the negotiated output
// mode. Under -o yaml it captures the app's JSON document (goalsapp emits one
// JSON document to Stdout in JSON mode) and re-emits it as YAML, so `-o yaml` is
// the same data yaml-marshaled instead of a silent human-table fallback. The
// run error (which may carry a non-zero exit, e.g. a drift regression) is still
// surfaced after the YAML is written, preserving json/yaml parity.
func (module Module) renderStructured(cmd *cobra.Command, run func(w io.Writer, asJSON bool) error) error {
	if module.host.OutputMode() == "yaml" {
		var buf bytes.Buffer
		runErr := run(&buf, true)
		if buf.Len() > 0 {
			if convErr := clicontract.JSONToYAML(cmd.OutOrStdout(), buf.Bytes()); convErr != nil {
				if runErr != nil {
					return runErr
				}
				return convErr
			}
		}
		return runErr
	}
	return run(cmd.OutOrStdout(), module.jsonOutput())
}

// Command builds the `ao goals` command tree with constructor-scoped flag
// state. No package-level command or flag variable is used.
func (module Module) Command() *cobra.Command {
	var (
		goalsFile    string
		goalsTimeout int
	)

	command := &cobra.Command{
		Use:   "goals",
		Short: "Fitness goal measurement and validation",
		Long: `Track, measure, and validate project fitness goals.

Supports both GOALS.yaml (versions 1-3) and GOALS.md (version 4) formats.
When both exist, GOALS.md takes precedence.

Measurement:
  measure (m)   Run goal checks and produce a snapshot
  validate (v)  Validate goals structure and wiring

Analysis:
  drift (d)     Compare snapshots for regressions
  history (h)   Show goal measurement history
  export (e)    Export latest snapshot as JSON
  meta          Run and report meta-goals only
  scenarios     Inspect or lint linked acceptance scenarios`,
		GroupID: "workflow",
	}
	command.AddGroup(
		&cobra.Group{ID: "measurement", Title: "Measurement:"},
		&cobra.Group{ID: "analysis", Title: "Analysis:"},
	)
	command.PersistentFlags().StringVar(&goalsFile, "file", "", "Path to goals file (auto-detects GOALS.md then GOALS.yaml)")
	command.PersistentFlags().IntVar(&goalsTimeout, "timeout", defaultGoalsTimeoutSeconds, "Check timeout in seconds")

	resolveFile := func() string { return goalsapp.ResolveGoalsFile(goalsFile) }
	timeout := func() time.Duration { return time.Duration(goalsTimeout) * time.Second }

	command.AddCommand(module.newMeasureCommand(resolveFile, timeout))
	command.AddCommand(module.newValidateCommand(resolveFile))
	command.AddCommand(module.newDriftCommand(resolveFile, timeout))
	command.AddCommand(module.newHistoryCommand())
	command.AddCommand(module.newExportCommand(resolveFile, timeout))
	command.AddCommand(module.newMetaCommand(resolveFile, timeout))
	command.AddCommand(module.newScenariosCommand(resolveFile))
	command.AddCommand(module.newRenderCommand(resolveFile))
	return command
}

func (module Module) newMeasureCommand(resolveFile func() string, timeout func() time.Duration) *cobra.Command {
	var (
		goalID        string
		directives    bool
		excludeTag    string
		totalTimeout  int
		scenariosOnly bool
	)
	command := &cobra.Command{
		Use:     "measure",
		Aliases: []string{"m"},
		Short:   "Run goal checks and produce a snapshot",
		GroupID: "measurement",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return module.renderStructured(cmd, func(w io.Writer, asJSON bool) error {
				return goalsapp.RunMeasureScenarios(goalsapp.MeasureScenariosOptions{
					GoalsFile:     resolveFile(),
					ProjectRoot:   module.host.ProjectRoot(),
					GoalID:        goalID,
					ExcludeTag:    excludeTag,
					Directives:    directives,
					Timeout:       timeout(),
					TotalTimeout:  time.Duration(totalTimeout) * time.Second,
					ScenariosOnly: scenariosOnly,
					JSON:          asJSON,
					Verbose:       module.host.Verbose(),
					Stdout:        w,
					Stderr:        cmd.ErrOrStderr(),
				})
			})
		},
	}
	command.Flags().StringVar(&goalID, "goal", "", "Measure a single goal by ID")
	command.Flags().BoolVar(&directives, "directives", false, "Output directives as JSON (skip gate checks)")
	command.Flags().StringVar(&excludeTag, "exclude-tag", "", "Skip goals whose Tags include this value (e.g. long-cycle)")
	command.Flags().IntVar(&totalTimeout, "total-timeout", 0, "Overall measurement timeout in seconds (0 disables)")
	command.Flags().BoolVar(&scenariosOnly, "scenarios-only", false, "Evaluate only executable-spec scenario satisfaction; skip shell gate-command execution")
	return command
}

func (module Module) newValidateCommand(resolveFile func() string) *cobra.Command {
	return &cobra.Command{
		Use:     "validate",
		Aliases: []string{"v"},
		Short:   "Validate GOALS.yaml structure and wiring",
		GroupID: "measurement",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return module.renderStructured(cmd, func(w io.Writer, asJSON bool) error {
				return goalsapp.RunValidate(goalsapp.ValidateOptions{
					GoalsFile: resolveFile(),
					JSON:      asJSON,
					Stdout:    w,
				})
			})
		},
	}
}

func (module Module) newDriftCommand(resolveFile func() string, timeout func() time.Duration) *cobra.Command {
	return &cobra.Command{
		Use:     "drift",
		Aliases: []string{"d"},
		Short:   "Compare snapshots for regressions",
		GroupID: "analysis",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return module.renderStructured(cmd, func(w io.Writer, asJSON bool) error {
				return goalsapp.RunDrift(goalsapp.DriftOptions{
					GoalsFile: resolveFile(),
					Timeout:   timeout(),
					JSON:      asJSON,
					Stdout:    w,
				})
			})
		},
	}
}

func (module Module) newHistoryCommand() *cobra.Command {
	var (
		goalID string
		since  string
	)
	command := &cobra.Command{
		Use:     "history",
		Aliases: []string{"h"},
		Short:   "Show goal measurement history",
		GroupID: "analysis",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return module.renderStructured(cmd, func(w io.Writer, asJSON bool) error {
				return goalsapp.RunHistory(goalsapp.HistoryOptions{
					GoalID: goalID,
					Since:  since,
					JSON:   asJSON,
					Stdout: w,
				})
			})
		},
	}
	command.Flags().StringVar(&goalID, "goal", "", "Filter history to a specific goal")
	command.Flags().StringVar(&since, "since", "", "Show entries since date (YYYY-MM-DD)")
	return command
}

func (module Module) newExportCommand(resolveFile func() string, timeout func() time.Duration) *cobra.Command {
	return &cobra.Command{
		Use:     "export",
		Aliases: []string{"e"},
		Short:   "Export latest snapshot as JSON (for CI)",
		GroupID: "analysis",
		RunE: func(cmd *cobra.Command, _ []string) error {
			// export is always structured (its default is JSON, not a human
			// table); honor -o yaml by capturing the JSON snapshot and re-emitting
			// it as YAML.
			return module.renderStructured(cmd, func(w io.Writer, _ bool) error {
				return goalsapp.RunExport(goalsapp.ExportOptions{
					GoalsFile: resolveFile(),
					Timeout:   timeout(),
					Stdout:    w,
				})
			})
		},
	}
}

func (module Module) newMetaCommand(resolveFile func() string, timeout func() time.Duration) *cobra.Command {
	return &cobra.Command{
		Use:     "meta",
		Short:   "Run and report meta-goals only",
		GroupID: "analysis",
		RunE: func(_ *cobra.Command, _ []string) error {
			return goalsapp.RunMeta(goalsapp.MetaOptions{
				GoalsFile: resolveFile(),
				Timeout:   timeout(),
				JSON:      module.jsonOutput(),
			})
		},
	}
}

func (module Module) newScenariosCommand(resolveFile func() string) *cobra.Command {
	var (
		directive   int
		directiveID string
		lint        bool
		strict      bool
	)
	command := &cobra.Command{
		Use:     "scenarios",
		Short:   "Inspect holdout scenarios linked to GOALS.md directives",
		GroupID: "analysis",
		Args:    cobra.NoArgs,
		Long: `Inspect the executable-spec scenarios linked to GOALS.md directives.

Directive membership comes from each directive's
"**Scenarios:**" attribute line; scenario content is resolved from
spec/scenarios/ then .agents/holdout/ (see docs/adr/ADR-0003).

  ao goals scenarios                       list every directive and its links
  ao goals scenarios --directive 2         filter to directive #2
  ao goals scenarios --directive-id d-foo  filter to a stable directive ID
  ao goals scenarios -o json               machine-readable directive→scenarios map
  ao goals scenarios --lint                report link-graph defects`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if lint {
				return module.renderStructured(cmd, func(w io.Writer, asJSON bool) error {
					return goalsapp.RunLint(goalsapp.LintOptions{
						GoalsFile: resolveFile(),
						Strict:    strict,
						JSON:      asJSON,
						Stdout:    w,
					})
				})
			}
			return module.renderStructured(cmd, func(w io.Writer, asJSON bool) error {
				return goalsapp.RunScenarios(goalsapp.ScenariosOptions{
					GoalsFile:    resolveFile(),
					DirectiveNum: directive,
					DirectiveID:  directiveID,
					JSON:         asJSON,
					Stdout:       w,
					Stderr:       cmd.ErrOrStderr(),
				})
			})
		},
	}
	command.Flags().IntVar(&directive, "directive", 0, "Filter by directive display number")
	command.Flags().StringVar(&directiveID, "directive-id", "", "Filter listing to one directive by stable Directive ID")
	command.Flags().BoolVar(&lint, "lint", false, "Lint the directive↔scenario link graph instead of listing")
	command.Flags().BoolVar(&strict, "strict", false, "With --lint, exit non-zero on warnings as well as errors")
	return command
}

func (module Module) newRenderCommand(resolveFile func() string) *cobra.Command {
	var out string
	command := &cobra.Command{
		Use:     "render",
		Short:   "Render GOALS.md directives and linked scenarios as BDD/Gherkin",
		GroupID: "analysis",
		Args:    cobra.NoArgs,
		Long: `Render the executable-spec layer as BDD/Gherkin text.

render is strictly READ-ONLY: it reads each GOALS.md directive's
"**Directive ID:**" and "**Scenarios:**" attributes plus the linked scenario
JSON files, and emits Gherkin. It never rewrites GOALS.md.

Each directive becomes a Gherkin "Feature" tagged with its stable directive ID
(@d-<id>). Each linked scenario becomes a "Scenario" tagged @<scenario-id>.
Gherkin steps come from the scenario's structured "given"/"when"/"then" arrays
when present; when a scenario lacks those, render emits a documented
best-effort step plus a comment noting the missing structured steps.

  ao goals render                 print Gherkin to stdout
  ao goals render --out spec.feature   write Gherkin to a file

Scenario content is resolved from spec/scenarios/<id>.json then
.agents/holdout/<id>.json (docs/adr/ADR-0003 resolution order).`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return goalsapp.RunRender(goalsapp.RenderOptions{
				GoalsFile:   resolveFile(),
				ProjectRoot: module.host.ProjectRoot(),
				Out:         out,
				Stdout:      cmd.OutOrStdout(),
			})
		},
	}
	command.Flags().StringVar(&out, "out", "", "Write Gherkin to this file instead of stdout")
	return command
}
