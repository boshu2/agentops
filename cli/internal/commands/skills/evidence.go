package skills

import (
	"encoding/json"
	"fmt"

	"github.com/boshu2/agentops/cli/internal/clicontract"
	"github.com/boshu2/agentops/cli/internal/skillsapp"
	"github.com/boshu2/agentops/cli/internal/skillshealth"
	"github.com/spf13/cobra"
)

func (m *Module) evidenceCommand() *cobra.Command {
	var repo, profile, reportPath string
	var strict bool
	cmd := &cobra.Command{Use: "audit <package>", Short: "Separate static conformance, effect observations and unmeasured behavior", Args: func(cmd *cobra.Command, args []string) error {
		if len(args) != 1 {
			return &clicontract.ExitError{Code: 2, Message: "exactly one skill package is required", Label: "skills audit"}
		}
		return nil
	}, RunE: func(cmd *cobra.Command, args []string) error {
		report, err := skillshealth.AuditEvidence(repo, args[0], profile)
		if err != nil {
			return &clicontract.ExitError{Code: 2, Message: err.Error(), Label: "skills audit"}
		}
		if reportPath != "" {
			if m.host.DryRun != nil && m.host.DryRun() {
				return fmt.Errorf("audit --json does not support dry-run; no report written")
			}
			if err = skillsapp.WriteAuditEvidence(repo, reportPath, report); err != nil {
				return &clicontract.ExitError{Code: 2, Message: err.Error(), Label: "skills audit report"}
			}
		} else {
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			if err = enc.Encode(report); err != nil {
				return err
			}
		}
		fmt.Fprintf(cmd.ErrOrStderr(), "Conformance (%s): %s. Effects: NOT_PROVEN. Behavior: NOT_PROVEN. Authoring suspicions are non-gating.\n", report.Conformance.Profile, report.Conformance.Status)
		if report.Conformance.Status == "FAIL" {
			return &clicontract.ExitError{Code: 1, Message: "static conformance failed", Label: "skills audit"}
		}
		return nil
	}}
	cmd.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return &clicontract.ExitError{Code: 2, Message: err.Error(), Label: "skills audit"}
	})
	cmd.Flags().StringVar(&repo, "repo", ".", "Repository used to select canonical or portable applicability")
	cmd.Flags().StringVar(&profile, "profile", "", "Static profile: canonical, portable or external-observation; inferred from target location")
	cmd.Flags().StringVar(&reportPath, "json", "", "Write a new report in an existing external non-Git directory instead of stdout")
	cmd.Flags().BoolVar(&strict, "strict", false, "Compatibility flag; only concrete conformance failures gate, never authoring suspicions")
	return cmd
}
