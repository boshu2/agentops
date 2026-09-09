package session

import (
	"fmt"

	"github.com/boshu2/agentops/cli/internal/clicontract"
	"github.com/boshu2/agentops/cli/internal/sourceread"
	"github.com/spf13/cobra"
)

func (m Module) readSourceCommand() *cobra.Command {
	var options sourceread.Options
	var through int64
	var jsonOut bool
	command := &cobra.Command{
		Use: "read-source", Short: "Read authorized bounded source bytes with integrity facts",
		Long: `Read one explicit regular source file under independently selected T05 context
policy. Native BD 1.2.2 context and maintenance-anchor reads must succeed before
source bytes open. The selected task_policy_ref must contain source-read-policy.v1
with exact file permissions and a measured output profile. Missing, mismatched,
unavailable or malformed policy denies the read. Restricted sources remain
unavailable until native access enforcement is implemented; current policy checks
support only explicitly synthetic or already-cleared sources.

--start-byte and positive --max-bytes are required. The half-open result includes
raw base64 bytes, a separately labelled text view, full-prefix/span SHA-256,
file observations and next offset. --through-byte and --expect-prefix-sha256
must be paired for a frozen continuation; appends after that boundary are allowed.
Use --expect-file-identity from a previous file_before.identity to detect
replacement between invocations, including replacement with identical bytes.

Output is one compact source-read.v1 JSON document, including its trailing newline.
The selected profile bounds the actual serialized output before any bytes emit.
--allow-oversize explicitly bypasses only that size bound. Every result remains
host-delivery-unverified, semantically unprocessed and complete_reading=false:
emitted stdout cannot prove host delivery, absence of tool-result truncation,
or model comprehension. YAML is unavailable because this profile measures JSON.
No source, policy, tracker or evidence file is written; --dry-run changes nothing.
Exit 0 means JSON was written; exit 1 means denied access, invalid bounds,
integrity failure, unavailable source/profile, oversized output or output failure.`,
		Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			command.SilenceUsage = true
			if m.outputMode() == "yaml" {
				return fmt.Errorf("read-source: measured serialization supports JSON only")
			}
			if !command.Flags().Changed("start-byte") || !command.Flags().Changed("max-bytes") {
				return fmt.Errorf("read-source: explicit start-byte and max-bytes are required")
			}
			if command.Flags().Changed("through-byte") {
				options.ThroughByte = &through
			}
			return sourceread.Run(command.Context(), options, command.OutOrStdout())
		},
	}
	f := command.Flags()
	f.StringVar(&options.File, "file", "", "Exact absolute source file path (required)")
	f.StringVar(&options.Context.Overrides.AccessPolicyRef, "access-policy-ref", "", "Independently selected T05 access policy JSON (required)")
	f.StringVar(&options.Context.SourceID, "source-id", "", "Expected canonical native beads_dir (required)")
	f.StringVar(&options.Context.ProjectID, "project-id", "", "Expected native project identity (required)")
	f.StringVar(&options.Context.OwnerScope, "owner-scope", "", "Independently expected owner scope (required)")
	f.StringVar(&options.Context.TaskRef, "task-ref", "", "Expected caller task identity (required)")
	f.StringVar(&options.Context.ModelRef, "model-ref", "", "Expected model/provider identity (required)")
	f.StringVar(&options.Context.DestinationRef, "destination-ref", "", "Expected destination identity (required)")
	f.StringVar(&options.Context.ConsumerRoot, "consumer-root", "", "Existing consumer checkout for T05 route verification (required)")
	f.StringVar(&options.Context.NativeDirectory, "native-directory", "", "Explicit directory for native BD source verification (required)")
	f.Int64Var(&options.StartByte, "start-byte", 0, "First byte of the returned half-open range (required)")
	f.Int64Var(&options.MaxBytes, "max-bytes", 0, "Positive maximum returned source bytes (required)")
	f.Int64Var(&through, "through-byte", 0, "Frozen exclusive prefix boundary, paired with expect-prefix-sha256")
	f.StringVar(&options.ExpectPrefixSHA256, "expect-prefix-sha256", "", "Expected SHA-256 of all bytes before through-byte")
	f.StringVar(&options.ExpectFileIdentity, "expect-file-identity", "", "Expected prior file_before.identity, for replacement checks across calls")
	f.BoolVar(&options.AllowOversize, "allow-oversize", false, "Explicitly bypass serialized profile size bound; host delivery remains unverified")
	f.BoolVar(&jsonOut, "json", false, "Emit JSON (also the default; no text-only coverage view)")
	contract := clicontract.CommandContract{ID: "ao.session.read-source", Profiles: clicontract.ProfileDefault | clicontract.ProfileLegacy | clicontract.ProfileCombined, Args: clicontract.ArgsPolicy{Name: "none", Validate: cobra.NoArgs}, Output: clicontract.OutputStructured, Effects: clicontract.EffectFilesystem | clicontract.EffectEnvironment | clicontract.EffectProcess, ExitClasses: map[int]clicontract.ExitClass{0: clicontract.ExitSuccess, 1: clicontract.ExitFailure}}
	if err := clicontract.Attach(command, contract); err != nil {
		panic(err)
	}
	return command
}
