package skills

import (
	"encoding/json"
	"fmt"
	"github.com/boshu2/agentops/cli/internal/clicontract"
	"github.com/boshu2/agentops/cli/internal/skillsapp"
	"github.com/boshu2/agentops/cli/internal/skillshealth"
	"github.com/spf13/cobra"
)

func (m *Module) buildCommand() *cobra.Command {
	opts := skillsapp.BuildOptions{}
	cmd := &cobra.Command{Use: "build <from-scratch|from-template|absorb-external> <slug>", Short: "Create an incomplete skill scaffold and its owned projections", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		if m.host.DryRun != nil && m.host.DryRun() {
			return fmt.Errorf("skills build does not support dry-run; no files created")
		}
		opts.Mode = args[0]
		opts.Slug = args[1]
		report, err := skillsapp.Build(opts, cmd.ErrOrStderr())
		if report != nil {
			if e := json.NewEncoder(cmd.OutOrStdout()).Encode(report); e != nil {
				return e
			}
		}
		return err
	}}
	cmd.Flags().StringVar(&opts.Repo, "repo", ".", "Repository containing canonical skills")
	cmd.Flags().StringVar(&opts.Source, "source", "", "Template slug or external input path (structure only)")
	cmd.Flags().StringVar(&opts.Report, "report", "", "Optional new report in an existing external non-Git directory; default stdout only")
	cmd.Flags().BoolVar(&opts.InitOnly, "init-only", false, "Create the scaffold without projections")
	return cmd
}

func (m *Module) sourceCheckCommand() *cobra.Command {
	var repo string
	var strict bool
	cmd := &cobra.Command{Use: "check-source <skills/slug>...", Short: "Check explicit source packages without claiming semantic completeness", Args: cobra.MinimumNArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		findings, err := skillshealth.CheckSource(repo, args, false)
		if err != nil {
			return &clicontract.ExitError{Code: 2, Message: err.Error(), Label: "skills check-source"}
		}
		for _, f := range findings {
			fmt.Fprintf(cmd.OutOrStdout(), "[%s] %s: %s\n", f.Code, f.Path, f.Message)
		}
		if strict && len(findings) > 0 {
			return fmt.Errorf("source check found %d issue(s)", len(findings))
		}
		return nil
	}}
	cmd.Flags().StringVar(&repo, "repo", ".", "Repository containing canonical skills")
	cmd.Flags().BoolVar(&strict, "strict", false, "Return nonzero for structural or explicit scaffold findings")
	return cmd
}
