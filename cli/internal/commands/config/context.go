package config

import (
	"context"
	"fmt"

	"github.com/boshu2/agentops/cli/internal/clicontract"
	configapp "github.com/boshu2/agentops/cli/internal/config"
	"github.com/spf13/cobra"
)

type ContextUseCases interface {
	Context(context.Context, configapp.ContextRequest) (configapp.ContextResult, error)
}

// NewContextCommand is a read-only consumer of configured routes. --field emits
// a checked root for an existing caller-selected evidence/staging operation.
func NewContextCommand(useCases ContextUseCases) *cobra.Command {
	var req configapp.ContextRequest
	var field string
	command := &cobra.Command{Use: "context", Short: "Resolve an authorized external context route without writing", Args: cobra.NoArgs,
		Long: `Resolve caller/home CDLC routes and read the selected native maintenance anchor.
Flags override AGENTOPS_CONTEXT_* variables, then project and home configuration.
AGENTOPS_CONFIG selects only that file. No roots or policies have defaults.
An existing schema_version 1 policy must bind the requested source, project,
owner, task, model, destination and exact storage roots. This selects policy;
it does not attest native access enforcement (T39).
Native BD 1.2.2 must support context schema 1, show --json and direct comments --json.
Missing, conflicting, malformed or incomplete inputs fail before any write.
--recover reads context.route.v1 from the same independently selected anchor;
it never creates replacement configuration, knowledge or work.`,
		RunE: func(command *cobra.Command, _ []string) error {
			if field != "" && field != "bundle_root" && field != "evidence_root" && field != "staging_root" {
				return fmt.Errorf("field must be bundle_root, evidence_root or staging_root")
			}
			result, err := useCases.Context(command.Context(), req)
			if err != nil {
				return err
			}
			switch field {
			case "bundle_root":
				_, err = fmt.Fprintln(command.OutOrStdout(), result.Route.BundleRoot)
			case "evidence_root":
				_, err = fmt.Fprintln(command.OutOrStdout(), result.Route.EvidenceRoot)
			case "staging_root":
				_, err = fmt.Fprintln(command.OutOrStdout(), result.Route.StagingRoot)
			default:
				return writeJSON(command, result, "context route")
			}
			return err
		}}
	flags := command.Flags()
	for _, item := range []struct {
		name string
		dst  *string
		help string
	}{
		{"source-id", &req.SourceID, "Expected canonical native beads_dir"}, {"project-id", &req.ProjectID, "Expected native project ID"}, {"owner-scope", &req.OwnerScope, "Expected separately authorized owner scope"},
		{"task-ref", &req.TaskRef, "Caller task identity"}, {"model-ref", &req.ModelRef, "Caller model/provider identity"}, {"destination-ref", &req.DestinationRef, "Caller destination identity"},
		{"consumer-root", &req.ConsumerRoot, "Existing consumer checkout to exclude"}, {"native-directory", &req.NativeDirectory, "Explicit native BD source directory"},
		{"bundle-root", &req.Overrides.BundleRoot, "Selected existing external bundle"}, {"evidence-root", &req.Overrides.EvidenceRoot, "Selected existing protected non-Git evidence"}, {"staging-root", &req.Overrides.StagingRoot, "Selected existing protected non-Git staging"},
		{"access-policy-ref", &req.Overrides.AccessPolicyRef, "Existing caller-owned access policy JSON"}, {"maintenance-work-ref", &req.Overrides.MaintenanceWorkRef, "Known native maintenance anchor"},
	} {
		flags.StringVar(item.dst, item.name, "", item.help)
	}
	flags.StringVar(&field, "field", "", "Emit one checked root for its caller-owned consumer")
	flags.BoolVar(&req.Recover, "recover", false, "Recover the same route from its native maintenance anchor")
	contract := clicontract.CommandContract{ID: "ao.config.context", Profiles: clicontract.ProfileDefault | clicontract.ProfileLegacy | clicontract.ProfileCombined, Args: clicontract.ArgsPolicy{Name: "none", Validate: cobra.NoArgs}, Output: clicontract.OutputStructured, Effects: clicontract.EffectFilesystem | clicontract.EffectEnvironment | clicontract.EffectProcess, ExitClasses: map[int]clicontract.ExitClass{0: clicontract.ExitSuccess, 1: clicontract.ExitFailure}}
	if err := clicontract.Attach(command, contract); err != nil {
		panic(err)
	}
	return command
}
