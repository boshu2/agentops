package provenance

import (
	"fmt"

	"github.com/boshu2/agentops/cli/internal/clicontract"
	"github.com/boshu2/agentops/cli/internal/okfprofile"
	"github.com/spf13/cobra"
)

func (m *Module) checkOKFCommand() *cobra.Command {
	var file, profile string
	var localJSON bool
	cmd := &cobra.Command{
		Use: "check-okf", Short: "Check the pinned AgentOps OKF page profile without granting semantic approval", Args: cobra.NoArgs,
		SilenceUsage: true,
		Long: `Check one explicitly selected Markdown concept against agentops-okf-v0.2/v1,
pinned to OKF v0.2 SPEC commit ad30107c31c06aec8a7d5636e0d1058118604e6f.
Unknown profiles fail before any file read. Optional agentops_profile and
okf_version page declarations must match the selected pin. Missing status is
invalid; it never inherits stable. No page label grants clearance or admission.

Require type, title, description, explicit status, nonempty sources entries,
knowledge_use (maintained-reference or promotion-candidate), applicability,
claim or action, limitations, consumer, and retirement_condition. These five
text obligations may be string metadata or named Markdown ATX sections. Check standard
generated/verified actor and timestamp shapes without treating actor strings
as independent-review evidence. Other producer keys and unknown types are allowed.

Reads only --file, a regular .md concept of at most 1 MiB with at most 64 KiB
frontmatter. Rejects index.md/log.md, file symlinks, malformed/duplicate YAML,
aliases/merge inheritance and excessive nesting. Sources and links are opaque:
no source fetching, bundle search, Git, configuration lookup, memory write,
network, external process or semantic judgment. Native runtime authorization
must precede this read; the checker does not enforce owner/destination policy.

JSON by default, --json or -o json; -o yaml emits the same fields. The report
contains the exact input SHA-256, structural findings and the explicit assurance
boundary, without echoing page text or source locators. --dry-run is also read-only.
Exit 0 means the selected structure is valid. Exit 1 means findings, unsupported
profile, invalid input, read failure or output failure. Neither exit establishes
truth, disclosure permission, independent review, applicability or usefulness.`,
	}
	cmd.Flags().StringVar(&file, "file", "", "Explicit Markdown concept file (required; at most 1 MiB)")
	_ = cmd.MarkFlagRequired("file")
	cmd.Flags().StringVar(&profile, "profile", okfprofile.Profile, "Exact supported structural profile version")
	cmd.Flags().BoolVar(&localJSON, "json", false, "Emit JSON (the default)")
	cmd.RunE = func(cmd *cobra.Command, _ []string) error {
		result, err := okfprofile.CheckFile(file, profile)
		if err != nil {
			return err
		}
		if _, err := m.emitStructured(cmd.OutOrStdout(), true, result); err != nil {
			return err
		}
		if !result.StructurallyValid {
			return fmt.Errorf("OKF page profile has structural findings")
		}
		return nil
	}
	contract := m.Contract()
	contract.ID = "ao.provenance.check-okf"
	contract.Args = clicontract.ArgsPolicy{Name: "no-args", Validate: cobra.NoArgs}
	contract.Output = clicontract.OutputStructured
	contract.Effects = clicontract.EffectFilesystem
	contract.ExitClasses = map[int]clicontract.ExitClass{0: clicontract.ExitSuccess, 1: clicontract.ExitFailure}
	if err := clicontract.Attach(cmd, contract); err != nil {
		panic(err)
	}
	return cmd
}
