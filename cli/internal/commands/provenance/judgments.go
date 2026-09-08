package provenance

import (
	"fmt"

	"github.com/boshu2/agentops/cli/internal/clicontract"
	"github.com/boshu2/agentops/cli/internal/evidence"
	"github.com/spf13/cobra"
)

func (m *Module) verifyJudgmentsCommand() *cobra.Command {
	var options evidence.JudgmentOptions
	var profiles, version string
	var localJSON bool
	cmd := &cobra.Command{
		Use: "verify-judgments", Short: "Check required judge profiles against native receipts and exact expected subject/intent", Args: cobra.NoArgs,
		Long: `Check caller-selected required judgment legs using existing verdict.v2 evidence_refs.

Supply independent profiles, provider authorization, immutable acceptance, subject
manifest and author identity. Receipt references use
judgment-receipt:/absolute/private/path.json#sha256=<exact-receipt-byte-hash>.
Referenced receipts, transcripts and verdicts must be private regular files in the
explicit non-Git --evidence-root. Each read is limited to 16 MiB. Native JSONL spans
use zero-based, end-exclusive byte offsets and must end on line boundaries.

Profiles cannot grant provider access: denied required providers fail before any
candidate or subject read. This local verifier never transmits content or starts a
judge; the invoking runtime must enforce task/owner/disclosure policy before launch.
Actual model/context comes only from supported native transcript envelope fields;
requested options and assistant self-descriptions cannot satisfy identity. Missing
identity, mismatches, omissions, incomplete termination and missing legs fail closed.

JSON (default) or YAML reports each unchanged verdict and mechanical satisfaction,
not a new semantic judgment. Exit 0 means all required supplied PASS legs match;
exit 1 means unsatisfied coverage, invalid input or a read error. Evidence helper
version 1 is required; no Python, Git, configuration lookup or tracker is invoked.`,
	}
	flag := func(target *string, name, help string) {
		cmd.Flags().StringVar(target, name, "", help)
		_ = cmd.MarkFlagRequired(name)
	}
	flag(&options.Root, "root", "Explicit subject directory")
	flag(&options.Manifest, "manifest", "Expected subject-manifest.v1 file")
	flag(&options.Intent, "intent", "Independent expected immutable acceptance file")
	flag(&options.EvidenceRoot, "evidence-root", "Explicit private non-Git root for receipts, transcripts and verdicts")
	flag(&options.AuthorContextID, "author-context-id", "Independent native author context identity")
	flag(&profiles, "required-profiles", "Independent JSON object containing the required profiles array")
	cmd.Flags().StringVar(&options.BaseManifest, "base-manifest", "", "Base manifest for deletions")
	cmd.Flags().StringArrayVar(&options.AllowedProviders, "allowed-provider", nil, "Independently authorized provider: openai or anthropic (repeatable)")
	_ = cmd.MarkFlagRequired("allowed-provider")
	cmd.Flags().StringArrayVar(&options.Verdicts, "verdict", nil, "Content-addressed verdict.v2 file inside evidence root (repeatable)")
	cmd.Flags().StringVar(&version, "helper-version", evidence.HelperVersion, "Required evidence helper version")
	cmd.Flags().BoolVar(&localJSON, "json", false, "Emit JSON (the default)")
	cmd.RunE = func(cmd *cobra.Command, _ []string) error {
		if err := m.checkEvidenceRequest(cmd, "verify-judgments", &evidenceOptions{localJSON: localJSON, version: version}); err != nil {
			return err
		}
		var err error
		options.Required, err = evidence.LoadJudgeProfiles(profiles)
		if err != nil {
			return err
		}
		result, err := evidence.VerifyJudgments(options)
		if err != nil {
			return err
		}
		if _, err = m.emitStructured(cmd.OutOrStdout(), true, result); err != nil {
			return err
		}
		if !result.Satisfied {
			return fmt.Errorf("required judgment coverage is unsatisfied")
		}
		return nil
	}
	contract := m.Contract()
	contract.ID = "ao.provenance.verify-judgments"
	contract.Args = clicontract.ArgsPolicy{Name: "no-args", Validate: cobra.NoArgs}
	contract.Output = clicontract.OutputStructured
	contract.Effects = clicontract.EffectFilesystem | clicontract.EffectEnvironment
	if err := clicontract.Attach(cmd, contract); err != nil {
		panic(err)
	}
	return cmd
}
