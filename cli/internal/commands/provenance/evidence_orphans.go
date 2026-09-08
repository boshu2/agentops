package provenance

import (
	"fmt"
	"strings"

	"github.com/boshu2/agentops/cli/internal/clicontract"
	"github.com/boshu2/agentops/cli/internal/evidence"
	"github.com/spf13/cobra"
)

func (m *Module) evidenceOrphansCommand() *cobra.Command {
	var root string
	var changed []string
	fail := func(err error) error {
		return &clicontract.ExitError{Code: 2, Message: "scan did not run to completion: " + err.Error(), Label: "evidence-orphans"}
	}
	args := func(cmd *cobra.Command, values []string) error {
		if err := cobra.NoArgs(cmd, values); err != nil {
			return fail(err)
		}
		if strings.TrimSpace(root) == "" {
			return fail(fmt.Errorf("required flag --root is missing"))
		}
		return nil
	}
	cmd := &cobra.Command{
		Use: "evidence-orphans", Short: "Report changed-file and digest exposure in existing evidence bindings",
		Long: `Read the established scorecard, fixture-set and capture-contract bindings
under an explicit repository root. --changed is repeatable; an empty changed
list still checks recorded digests. Emit the existing JSON receipt with changed,
orphaned, binding_count and artifact_count. Each orphan row identifies artifact,
binds, cause, recorded_sha256 and current_sha256 (null when unavailable).

Causes are changed_path, digest_drift, both and skill_changed. Exit 0 means the
scan completed, including when orphans exist. Exit 2 means malformed input or an
incomplete scan; no partial JSON receipt is emitted. This is advisory, not a gate.
There is no --strict mode, ledger, persistent index, Python or Git invocation.

The historical reader contract remains: absent scan directories are empty;
malformed shapes, evaluator read errors and directory-walk errors abort; canonical
skill resolution/read failures become null-digest orphan findings. Evaluator
paths retain their recorded meaning, including absolute and parent paths, and
artifact reads may follow file symlinks. Native authorization and filesystem
controls remain the caller's responsibility; this scanner is not confinement.
Only JSON output is supported. Explicit --output yaml/table is rejected.`,
		Args: args, SilenceErrors: true, SilenceUsage: true,
	}
	cmd.Flags().StringVar(&root, "root", "", "Explicit repository root to scan")
	_ = cmd.MarkFlagRequired("root")
	cmd.Flags().StringArrayVar(&changed, "changed", nil, "Changed bound path; repeat to preserve input order")
	cmd.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		if strings.HasPrefix(err.Error(), "unknown flag: ") {
			return fail(fmt.Errorf("unknown option %s", strings.TrimPrefix(err.Error(), "unknown flag: ")))
		}
		return fail(err)
	})
	cmd.RunE = func(cmd *cobra.Command, _ []string) error {
		format := cmd.Root().PersistentFlags().Lookup("output")
		if m.outputMode() == "yaml" || (format != nil && format.Changed && m.outputMode() != "json") {
			return fail(fmt.Errorf("JSON is the only evidence-orphans output format"))
		}
		result, err := evidence.OrphanBindings(root, changed)
		if err != nil {
			return fail(err)
		}
		if err = clicontract.WriteJSON(cmd.OutOrStdout(), result); err != nil {
			return fail(err)
		}
		return nil
	}
	contract := m.Contract()
	contract.ID = "ao.provenance.evidence-orphans"
	contract.Args = clicontract.ArgsPolicy{Name: "no-args", Validate: args}
	contract.Output = clicontract.OutputStructured
	contract.Effects = clicontract.EffectFilesystem
	contract.ExitClasses = map[int]clicontract.ExitClass{0: clicontract.ExitSuccess, 2: clicontract.ExitFailure}
	if err := clicontract.Attach(cmd, contract); err != nil {
		panic(err)
	}
	return cmd
}
