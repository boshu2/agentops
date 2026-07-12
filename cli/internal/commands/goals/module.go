// Package goals owns Cobra presentation for the goals command family.
package goals

import (
	"context"
	"fmt"
	"io/fs"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/clicontract"
	goalapp "github.com/boshu2/agentops/cli/internal/goals"
)

const defaultTimeoutSeconds = 240

type SimpleUseCases interface {
	Validate(context.Context, goalapp.ValidateOptions) error
	History(context.Context, goalapp.HistoryOptions) error
	Export(context.Context, goalapp.ExportOptions) error
	Drift(context.Context, goalapp.DriftOptions) error
	Meta(context.Context, goalapp.MetaOptions) error
}

type ManagementUseCases interface {
	Add(context.Context, goalapp.AddOptions) error
	Init(context.Context, goalapp.InitOptions) error
	Migrate(context.Context, goalapp.MigrateOptions) error
	Prune(context.Context, goalapp.PruneOptions) error
}

type ManualSteerUseCases interface {
	Add(context.Context, goalapp.SteerAddOptions) error
	Remove(context.Context, goalapp.SteerRemoveOptions) error
	Prioritize(context.Context, goalapp.SteerPrioritizeOptions) error
}

type UseCases struct {
	Simple      SimpleUseCases
	Management  ManagementUseCases
	ManualSteer ManualSteerUseCases
}

type HostOptions struct {
	OutputMode       func() string
	DryRun           func() bool
	ResolveGoalsPath func(string) string
	TemplateValues   func() []string
	TemplatesFS      fs.ReadFileFS
}

type Module struct {
	useCases UseCases
	host     HostOptions
}

func NewModule(useCases UseCases, host HostOptions) Module {
	return Module{useCases: useCases, host: host}
}

func (Module) Contract() clicontract.CommandContract {
	return clicontract.CommandContract{
		ID: "ao.goals",
		Profiles: clicontract.ProfileDefault | clicontract.ProfileFlywheel |
			clicontract.ProfileLegacy | clicontract.ProfileCombined,
		Args:    clicontract.ArgsPolicy{Name: "no-args", Validate: cobra.NoArgs},
		Output:  clicontract.OutputNone,
		Effects: clicontract.EffectFilesystem | clicontract.EffectProcess | clicontract.EffectEnvironment | clicontract.EffectClock,
		ExitClasses: map[int]clicontract.ExitClass{
			0: clicontract.ExitSuccess,
			1: clicontract.ExitFailure,
			2: clicontract.ExitUsage,
		},
	}
}

type rootOptions struct {
	file    string
	timeout int
}

func (module Module) Command() *cobra.Command {
	options := &rootOptions{timeout: defaultTimeoutSeconds}
	command := &cobra.Command{
		Use:   "goals",
		Short: "Fitness goal measurement and validation",
		Args:  cobra.NoArgs,
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

Management:
  init          Bootstrap a new GOALS.md interactively
  add (a)       Add a new goal
  steer         Manage directives (add/remove/prioritize)
  prune (p)     Remove stale gates
  migrate (mg)  Migrate between formats
  meta          Run and report meta-goals only`,
		GroupID: "workflow",
	}
	command.AddGroup(
		&cobra.Group{ID: "measurement", Title: "Measurement:"},
		&cobra.Group{ID: "analysis", Title: "Analysis:"},
		&cobra.Group{ID: "management", Title: "Management:"},
	)
	command.PersistentFlags().StringVar(&options.file, "file", "", "Path to goals file (auto-detects GOALS.md then GOALS.yaml)")
	command.PersistentFlags().IntVar(&options.timeout, "timeout", defaultTimeoutSeconds, "Check timeout in seconds")
	command.AddCommand(
		module.validateCommand(options),
		module.historyCommand(options),
		module.exportCommand(options),
		module.driftCommand(options),
		module.metaCommand(options),
		module.addCommand(options),
		module.initCommand(options),
		module.measureCommand(),
		module.migrateCommand(options),
		module.pruneCommand(options),
		module.renderCommand(),
		module.scenariosCommand(),
		module.steerCommand(options),
		module.traceCommand(),
	)
	return command
}

func (module Module) resolveGoalsPath(explicit string) string {
	if module.host.ResolveGoalsPath != nil {
		return module.host.ResolveGoalsPath(explicit)
	}
	if explicit != "" {
		return explicit
	}
	return "GOALS.md"
}

func (module Module) jsonOutput() bool {
	return module.host.OutputMode != nil && module.host.OutputMode() == "json"
}

func (module Module) dryRun() bool {
	return module.host.DryRun != nil && module.host.DryRun()
}

func (module Module) validateCommand(root *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use: "validate", Aliases: []string{"v"}, Short: "Validate GOALS.yaml structure and wiring", GroupID: "measurement",
		RunE: func(command *cobra.Command, _ []string) error {
			if module.useCases.Simple == nil {
				return missingUseCase("validate")
			}
			return module.useCases.Simple.Validate(command.Context(), goalapp.ValidateOptions{
				GoalsFile: module.resolveGoalsPath(root.file), JSON: module.jsonOutput(), Stdout: command.OutOrStdout(),
			})
		},
	}
}

func (module Module) historyCommand(_ *rootOptions) *cobra.Command {
	var goalID, since string
	command := &cobra.Command{
		Use: "history", Aliases: []string{"h"}, Short: "Show goal measurement history", GroupID: "analysis",
		RunE: func(command *cobra.Command, _ []string) error {
			if module.useCases.Simple == nil {
				return missingUseCase("history")
			}
			return module.useCases.Simple.History(command.Context(), goalapp.HistoryOptions{
				GoalID: goalID, Since: since, JSON: module.jsonOutput(), Stdout: command.OutOrStdout(),
			})
		},
	}
	command.Flags().StringVar(&goalID, "goal", "", "Filter history to a specific goal")
	command.Flags().StringVar(&since, "since", "", "Show entries since date (YYYY-MM-DD)")
	return command
}

func (module Module) exportCommand(root *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use: "export", Aliases: []string{"e"}, Short: "Export latest snapshot as JSON (for CI)", GroupID: "analysis",
		RunE: func(command *cobra.Command, _ []string) error {
			if module.useCases.Simple == nil {
				return missingUseCase("export")
			}
			return module.useCases.Simple.Export(command.Context(), goalapp.ExportOptions{
				GoalsFile: module.resolveGoalsPath(root.file), Timeout: time.Duration(root.timeout) * time.Second,
				Stdout: command.OutOrStdout(), Stderr: command.ErrOrStderr(),
			})
		},
	}
}

func (module Module) driftCommand(root *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use: "drift", Aliases: []string{"d"}, Short: "Compare snapshots for regressions", GroupID: "analysis",
		RunE: func(command *cobra.Command, _ []string) error {
			if module.useCases.Simple == nil {
				return missingUseCase("drift")
			}
			return module.useCases.Simple.Drift(command.Context(), goalapp.DriftOptions{
				GoalsFile: module.resolveGoalsPath(root.file), Timeout: time.Duration(root.timeout) * time.Second,
				JSON: module.jsonOutput(), Stdout: command.OutOrStdout(), Stderr: command.ErrOrStderr(),
			})
		},
	}
}

func (module Module) metaCommand(root *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use: "meta", Short: "Run and report meta-goals only", GroupID: "management",
		RunE: func(command *cobra.Command, _ []string) error {
			if module.useCases.Simple == nil {
				return missingUseCase("meta")
			}
			return module.useCases.Simple.Meta(command.Context(), goalapp.MetaOptions{
				GoalsFile: module.resolveGoalsPath(root.file), Timeout: time.Duration(root.timeout) * time.Second,
				JSON: module.jsonOutput(), Stdout: command.OutOrStdout(),
			})
		},
	}
}

func (module Module) addCommand(root *rootOptions) *cobra.Command {
	var weight int
	var goalType, description string
	command := &cobra.Command{
		Use: "add <id> <check-command>", Aliases: []string{"a"}, Short: "Add a new goal", GroupID: "management", Args: cobra.ExactArgs(2),
		RunE: func(command *cobra.Command, args []string) error {
			if module.useCases.Management == nil {
				return missingUseCase("add")
			}
			return module.useCases.Management.Add(command.Context(), goalapp.AddOptions{
				ID: args[0], Check: args[1], Weight: weight, Type: goalType, Description: description,
				GoalsFile: module.resolveGoalsPath(root.file), Timeout: time.Duration(root.timeout) * time.Second,
				DryRun: module.dryRun(), Stdout: command.OutOrStdout(),
			})
		},
	}
	command.Flags().IntVar(&weight, "weight", 5, "Goal weight (1-10)")
	command.Flags().StringVar(&goalType, "type", "", "Goal type (health, architecture, quality, meta)")
	command.Flags().StringVar(&description, "description", "", "Goal description")
	_ = command.RegisterFlagCompletionFunc("type", staticCompletion("health", "architecture", "quality", "meta"))
	return command
}

func (module Module) initCommand(root *rootOptions) *cobra.Command {
	var nonInteractive bool
	var template string
	command := &cobra.Command{
		Use: "init", Short: "Bootstrap a new GOALS.md file", GroupID: "management",
		RunE: func(command *cobra.Command, _ []string) error {
			if module.useCases.Management == nil {
				return missingUseCase("init")
			}
			return module.useCases.Management.Init(command.Context(), goalapp.InitOptions{
				NonInteractive: nonInteractive, Template: template, GoalsFile: module.resolveGoalsPath(root.file),
				JSON: module.jsonOutput(), DryRun: module.dryRun(), Stdin: command.InOrStdin(),
				Stdout: command.OutOrStdout(), TemplatesFS: module.host.TemplatesFS,
			})
		},
	}
	command.Flags().BoolVar(&nonInteractive, "non-interactive", false, "Use defaults without prompting")
	command.Flags().StringVar(&template, "template", "", "Goal template (go-cli, python-lib, web-app, rust-cli, generic)")
	if module.host.TemplateValues != nil {
		_ = command.RegisterFlagCompletionFunc("template", staticCompletion(module.host.TemplateValues()...))
	}
	return command
}

func (module Module) measureCommand() *cobra.Command {
	var goalID, excludeTag string
	var directives, scenariosOnly bool
	var totalTimeout int
	command := futureCommand("measure", "Run goal checks and produce a snapshot", "measurement", []string{"m"}, nil)
	command.Flags().StringVar(&goalID, "goal", "", "Measure a single goal by ID")
	command.Flags().BoolVar(&directives, "directives", false, "Output directives as JSON (skip gate checks)")
	command.Flags().StringVar(&excludeTag, "exclude-tag", "", "Skip goals whose Tags include this value (e.g. long-cycle)")
	command.Flags().IntVar(&totalTimeout, "total-timeout", 0, "Overall measurement timeout in seconds (0 disables)")
	command.Flags().BoolVar(&scenariosOnly, "scenarios-only", false, "Evaluate only executable-spec scenario satisfaction; skip shell gate-command execution")
	return command
}

func (module Module) migrateCommand(root *rootOptions) *cobra.Command {
	var toMD bool
	command := &cobra.Command{
		Use: "migrate", Short: "Migrate goals to latest format", GroupID: "management", Aliases: []string{"mg"},
		RunE: func(command *cobra.Command, _ []string) error {
			if module.useCases.Management == nil {
				return missingUseCase("migrate")
			}
			return module.useCases.Management.Migrate(command.Context(), goalapp.MigrateOptions{
				ToMD: toMD, GoalsFile: module.resolveGoalsPath(root.file), Stdout: command.OutOrStdout(),
			})
		},
	}
	command.Flags().BoolVar(&toMD, "to-md", false, "Convert GOALS.yaml to GOALS.md format")
	return command
}

func (module Module) pruneCommand(root *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use: "prune", Short: "Remove goals referencing nonexistent files", GroupID: "management", Aliases: []string{"p"},
		RunE: func(command *cobra.Command, _ []string) error {
			if module.useCases.Management == nil {
				return missingUseCase("prune")
			}
			return module.useCases.Management.Prune(command.Context(), goalapp.PruneOptions{
				GoalsFile: module.resolveGoalsPath(root.file), DryRun: module.dryRun(), JSON: module.jsonOutput(), Stdout: command.OutOrStdout(),
			})
		},
	}
}

func (module Module) renderCommand() *cobra.Command {
	var output string
	command := futureCommand("render", "Render GOALS.md directives and linked scenarios as BDD/Gherkin", "analysis", nil, cobra.NoArgs)
	command.Flags().StringVar(&output, "out", "", "Write Gherkin to this file instead of stdout")
	return command
}

func (module Module) scenariosCommand() *cobra.Command {
	var directive int
	var directiveID, create, status, source string
	var threshold float64
	var lint, strict bool
	command := futureCommand("scenarios", "List or create the holdout scenarios linked to GOALS.md directives", "analysis", nil, cobra.NoArgs)
	command.Flags().IntVar(&directive, "directive", 0, "Directive display number (filter when listing, target when creating)")
	command.Flags().StringVar(&directiveID, "directive-id", "", "Filter listing to one directive by stable Directive ID")
	command.Flags().StringVar(&create, "create", "", "Create a scenario from this goal description and link it to --directive")
	command.Flags().Float64Var(&threshold, "threshold", 0.8, "Satisfaction threshold for a created scenario")
	command.Flags().StringVar(&status, "status", "draft", "Status for a created scenario (active, draft, retired)")
	command.Flags().StringVar(&source, "source", "human", "Source for a created scenario (human, agent, prod-telemetry)")
	command.Flags().BoolVar(&lint, "lint", false, "Lint the directive↔scenario link graph instead of listing")
	command.Flags().BoolVar(&strict, "strict", false, "With --lint, exit non-zero on warnings as well as errors")
	_ = command.RegisterFlagCompletionFunc("status", staticCompletion("active", "draft", "retired"))
	_ = command.RegisterFlagCompletionFunc("source", staticCompletion("human", "agent", "prod-telemetry"))
	return command
}

func (module Module) steerCommand(root *rootOptions) *cobra.Command {
	command := &cobra.Command{Use: "steer", Short: "Manage directives", GroupID: "management"}
	var description, steer string
	add := &cobra.Command{
		Use: "add <title>", Short: "Add a new directive", Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			if module.useCases.ManualSteer == nil {
				return missingUseCase("steer add")
			}
			return module.useCases.ManualSteer.Add(command.Context(), goalapp.SteerAddOptions{
				Title: args[0], Description: description, Steer: steer, GoalsFile: module.resolveGoalsPath(root.file),
				JSON: module.jsonOutput(), DryRun: module.dryRun(), Stdout: command.OutOrStdout(),
			})
		},
	}
	add.Flags().StringVar(&description, "description", "", "Directive description (required)")
	_ = add.MarkFlagRequired("description")
	add.Flags().StringVar(&steer, "steer", "increase", "Steer direction (increase, decrease, hold, explore)")
	_ = add.RegisterFlagCompletionFunc("steer", staticCompletion("increase", "decrease", "hold", "explore"))
	remove := &cobra.Command{
		Use: "remove <number>", Short: "Remove a directive by number", Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			number, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("directive number must be an integer: %w", err)
			}
			if module.useCases.ManualSteer == nil {
				return missingUseCase("steer remove")
			}
			return module.useCases.ManualSteer.Remove(command.Context(), goalapp.SteerRemoveOptions{
				Number: number, GoalsFile: module.resolveGoalsPath(root.file), JSON: module.jsonOutput(),
				DryRun: module.dryRun(), Stdout: command.OutOrStdout(),
			})
		},
	}
	prioritize := &cobra.Command{
		Use: "prioritize <number> <new-position>", Short: "Move a directive to a new position", Args: cobra.ExactArgs(2),
		RunE: func(command *cobra.Command, args []string) error {
			number, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("directive number must be an integer: %w", err)
			}
			position, err := strconv.Atoi(args[1])
			if err != nil {
				return fmt.Errorf("new position must be an integer: %w", err)
			}
			if module.useCases.ManualSteer == nil {
				return missingUseCase("steer prioritize")
			}
			return module.useCases.ManualSteer.Prioritize(command.Context(), goalapp.SteerPrioritizeOptions{
				Number: number, NewPosition: position, GoalsFile: module.resolveGoalsPath(root.file),
				JSON: module.jsonOutput(), DryRun: module.dryRun(), Stdout: command.OutOrStdout(),
			})
		},
	}
	var policy string
	recommend := futureCommand("recommend", "Show re-steer recommendations without modifying GOALS.md", "", nil, cobra.NoArgs)
	recommend.Flags().StringVar(&policy, "policy", "", "Re-steer policy path (default: docs/re-steer-policy.json)")
	var yes, auto bool
	apply := futureCommand("apply", "Apply a re-steer recommendation to GOALS.md (human-gated)", "", nil, cobra.NoArgs)
	apply.Flags().BoolVar(&yes, "yes", false, "Pre-confirm the apply for non-interactive/scripted use (explicit consent)")
	apply.Flags().BoolVar(&auto, "auto", false, "Equivalent to --yes: explicit non-interactive consent to apply")
	apply.Flags().StringVar(&policy, "policy", "", "Re-steer policy path (default: docs/re-steer-policy.json)")
	command.AddCommand(add, apply, prioritize, recommend, remove)
	return command
}

func (module Module) traceCommand() *cobra.Command {
	var from string
	var orphans, strict bool
	command := futureCommand("trace", "Trace the directive→scenario→bead→artifact→learning executable-spec chain", "analysis", nil, cobra.NoArgs)
	command.Flags().StringVar(&from, "from", "", "Render the trace lineage rooted at this directive, scenario, or bead ID")
	command.Flags().BoolVar(&orphans, "orphans", false, "Audit the whole chain for broken references (errors) and missing yields (warnings)")
	command.Flags().BoolVar(&strict, "strict", false, "Escalate warning-class defects to a non-zero exit (ADR-0005 §4.2)")
	return command
}

func futureCommand(use, short, group string, aliases []string, args cobra.PositionalArgs) *cobra.Command {
	return &cobra.Command{
		Use: use, Short: short, GroupID: group, Aliases: aliases, Args: args,
		RunE: func(*cobra.Command, []string) error { return missingUseCase(use) },
	}
}

func missingUseCase(command string) error {
	return fmt.Errorf("goals %s: use case not configured", command)
}

func staticCompletion(values ...string) cobra.CompletionFunc {
	return func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return values, cobra.ShellCompDirectiveNoFileComp
	}
}
