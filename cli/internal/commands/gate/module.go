package gate

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/clicontract"
	gateapp "github.com/boshu2/agentops/cli/internal/gate"
)

type CheckUseCases interface {
	Execute(context.Context, gateapp.CheckRequest) (gateapp.CheckResult, error)
}

type UseCases struct{ Check CheckUseCases }

// HostOptions is retained as the command-module host seam. Gate check does not
// use lifecycle or review state from the host.
type HostOptions struct {
	DryRun       func() bool
	OutputFormat func() string
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
		ID:       "ao.gate",
		Profiles: clicontract.ProfileDefault | clicontract.ProfileFlywheel | clicontract.ProfileLegacy | clicontract.ProfileCombined,
		Args:     clicontract.ArgsPolicy{Name: "no-args", Validate: cobra.NoArgs},
		Output:   clicontract.OutputNone,
		Effects:  clicontract.EffectFilesystem | clicontract.EffectProcess | clicontract.EffectEnvironment | clicontract.EffectClock,
		ExitClasses: map[int]clicontract.ExitClass{
			0: clicontract.ExitSuccess,
			1: clicontract.ExitFailure,
			2: clicontract.ExitUsage,
		},
	}
}

type ExitError struct {
	Code    int
	Message string
}

func (failure *ExitError) Error() string { return failure.Message }
func (failure *ExitError) ExitCode() int { return failure.Code }

func (module Module) Command() *cobra.Command {
	command := &cobra.Command{
		Use:     "gate",
		Short:   "Run deterministic repository checks",
		GroupID: "core",
		Long: `Run ordinary deterministic repository checks.

The result means only that the selected checks passed or failed. It is not a
semantic verdict and does not authorize Git, release, or delivery.`,
	}
	command.AddCommand(module.newCheckCommand())
	return command
}

func (module Module) newCheckCommand() *cobra.Command {
	var fast, full, jsonOutput, githubAnnotations, failFast bool
	var scope, workflowPath string
	var workflowCoverage, requireWorkflowParity bool
	command := &cobra.Command{
		Use:   "check",
		Short: "Run the fast changed-surface subset or the full deterministic suite",
		Long: `Run the declarative deterministic check registry.

  ao gate check            # checks selected for the changed surface
  ao gate check --full     # every registered deterministic check
  ao gate check --json     # machine-readable report`,
		Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			_ = fast
			if module.useCases.Check == nil {
				return &ExitError{Code: 2, Message: "gate check: use case not configured"}
			}
			result, err := module.useCases.Check.Execute(command.Context(), gateapp.CheckRequest{
				Full: full, Scope: scope, FailFast: failFast,
				WorkflowCoverage: workflowCoverage, RequireWorkflowParity: requireWorkflowParity, WorkflowPath: workflowPath,
			})
			if err != nil {
				return &ExitError{Code: 2, Message: err.Error()}
			}
			if jsonOutput {
				raw, jsonErr := result.Report.JSON()
				if jsonErr != nil {
					return &ExitError{Code: 2, Message: jsonErr.Error()}
				}
				fmt.Fprintln(command.OutOrStdout(), string(raw))
			} else {
				result.Report.Human(command.OutOrStdout())
			}
			if githubAnnotations {
				result.Report.GitHubAnnotations(command.ErrOrStderr())
			}
			if result.ExitCode != 0 {
				message := ""
				if result.WorkflowParityMissing > 0 {
					message = fmt.Sprintf("workflow parity missing %d blocking script(s)", result.WorkflowParityMissing)
				}
				return &ExitError{Code: result.ExitCode, Message: message}
			}
			return nil
		},
	}
	flags := command.Flags()
	flags.BoolVar(&fast, "fast", false, "explicitly select the default fast changed-surface subset")
	flags.BoolVar(&full, "full", false, "run every registered deterministic check")
	flags.BoolVar(&jsonOutput, "json", false, "emit the machine-readable JSON report")
	flags.BoolVar(&githubAnnotations, "github-annotations", false, "emit GitHub Actions annotations for check results")
	flags.BoolVar(&failFast, "fail-fast", false, "stop after the first blocking check failure")
	flags.StringVar(&scope, "scope", "head", "changed-file scope: head|staged|worktree|upstream|range:<base>..<head>")
	flags.BoolVar(&workflowCoverage, "workflow-coverage", false, "include workflow-to-registry coverage in the report")
	flags.BoolVar(&requireWorkflowParity, "require-workflow-parity", false, "fail if the workflow references unregistered blocking scripts")
	flags.StringVar(&workflowPath, "workflow-path", ".github/workflows/validate.yml", "workflow used for optional coverage comparison")
	return command
}
