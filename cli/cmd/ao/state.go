// practices: [hexagonal-architecture, design-by-contract]
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/statememory"
)

var (
	stateAdmitFinding     string
	stateAdmitDestination string
	stateAdmitMaxAgeDays  int
)

var stateCmd = &cobra.Command{
	Use:     "state",
	GroupID: "knowledge",
	Short:   "Validate, admit, verify, and doctor AgentOps state memory",
	Long: `Manage the durable AgentOps state-memory contract.

State findings are schema-validated, fresh, non-self-reviewed artifacts. The
admission path rejects stale findings, self-review, leaked secrets, and writes
outside .agents/state/findings/.`,
}

var stateValidateCmd = &cobra.Command{
	Use:   "validate <file> [file...]",
	Short: "Validate state memory JSON files against their schemas",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runStateValidate,
}

var stateAdmitCmd = &cobra.Command{
	Use:   "admit --finding <path>",
	Short: "Admit one confirmed state finding into .agents/state/findings",
	Args:  cobra.NoArgs,
	RunE:  runStateAdmit,
}

var stateVerifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "Verify state memory schemas, fixtures, and admitted findings",
	Args:  cobra.NoArgs,
	RunE:  runStateVerify,
}

var stateDoctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Diagnose state memory health",
	Args:  cobra.NoArgs,
	RunE:  runStateDoctor,
}

func init() {
	rootCmd.AddCommand(stateCmd)
	stateCmd.AddCommand(stateValidateCmd, stateAdmitCmd, stateVerifyCmd, stateDoctorCmd)

	stateAdmitCmd.Flags().StringVar(&stateAdmitFinding, "finding", "", "Path to a state_finding JSON file (required)")
	stateAdmitCmd.Flags().StringVar(&stateAdmitDestination, "destination", "", "Destination under .agents/state/findings/ (default: finding id)")
	stateAdmitCmd.Flags().IntVar(&stateAdmitMaxAgeDays, "max-age-days", 30, "Maximum age for reviewed findings")
	_ = stateAdmitCmd.MarkFlagRequired("finding")
}

func runStateValidate(cmd *cobra.Command, args []string) error {
	root, err := repoRootOrCwd()
	if err != nil {
		return err
	}
	validated := 0
	for _, arg := range args {
		path := normalizeInputPath(arg)
		if err := statememory.ValidateStateFile(root, path); err != nil {
			return err
		}
		validated++
	}
	if GetOutput() == "json" {
		return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]any{
			"verdict":   "PASS",
			"validated": validated,
		})
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Validated %d state file(s): all pass\n", validated)
	return nil
}

func runStateAdmit(cmd *cobra.Command, _ []string) error {
	root, err := repoRootOrCwd()
	if err != nil {
		return err
	}
	findingPath := normalizeInputPath(stateAdmitFinding)
	data, err := os.ReadFile(findingPath)
	if err != nil {
		return fmt.Errorf("read finding: %w", err)
	}
	req := statememory.AdmissionRequest{
		SchemaVersion: 1,
		Kind:          statememory.AdmissionKind,
		CandidatePath: candidatePathFor(root, findingPath),
		Destination:   stateAdmitDestination,
		OperatorID:    GetCurrentUser(),
		Reason:        "ao state admit",
	}
	report, err := statememory.AdmitFinding(context.Background(), data, req, statememory.AdmissionOptions{
		Root:   root,
		Now:    time.Now().UTC(),
		MaxAge: time.Duration(stateAdmitMaxAgeDays) * 24 * time.Hour,
		Write:  !GetDryRun(),
	})
	if err != nil {
		return err
	}
	if GetOutput() == "json" {
		return json.NewEncoder(cmd.OutOrStdout()).Encode(report)
	}
	if report.Wrote {
		fmt.Fprintf(cmd.OutOrStdout(), "Admitted %s -> %s\n", report.FindingID, report.Destination)
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "Would admit %s -> %s\n", report.FindingID, report.Destination)
	}
	return nil
}

func runStateVerify(cmd *cobra.Command, _ []string) error {
	report, err := runStateVerifyCore(cmd.Context())
	if err != nil {
		return err
	}
	if err := emitStateVerifyReport(cmd, report); err != nil {
		return err
	}
	if report.Verdict != "PASS" {
		return fmt.Errorf("state verify failed with %d failure(s)", len(report.Failures))
	}
	return nil
}

func runStateDoctor(cmd *cobra.Command, _ []string) error {
	report, err := runStateVerifyCore(cmd.Context())
	if err != nil {
		return err
	}
	if GetOutput() == "json" {
		return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]any{
			"component": "state-memory",
			"report":    report,
		})
	}
	fmt.Fprintf(cmd.OutOrStdout(), "State memory: %s\n", report.Verdict)
	fmt.Fprintf(cmd.OutOrStdout(), "  schemas: %d\n", report.Schemas)
	fmt.Fprintf(cmd.OutOrStdout(), "  good fixtures: %d\n", report.GoodFixtures)
	fmt.Fprintf(cmd.OutOrStdout(), "  bad fixtures rejected: %d\n", report.BadFixturesRejected)
	fmt.Fprintf(cmd.OutOrStdout(), "  admitted findings: %d\n", report.StateFindings)
	for _, failure := range report.Failures {
		fmt.Fprintf(cmd.OutOrStdout(), "  FAIL: %s\n", failure)
	}
	if report.Verdict != "PASS" {
		return fmt.Errorf("state doctor found %d failure(s)", len(report.Failures))
	}
	return nil
}

func runStateVerifyCore(ctx context.Context) (statememory.VerifyReport, error) {
	root, err := repoRootOrCwd()
	if err != nil {
		return statememory.VerifyReport{}, err
	}
	return statememory.VerifyRepo(ctx, root)
}

func emitStateVerifyReport(cmd *cobra.Command, report statememory.VerifyReport) error {
	if GetOutput() == "json" {
		return json.NewEncoder(cmd.OutOrStdout()).Encode(report)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "State verify: %s\n", report.Verdict)
	fmt.Fprintf(cmd.OutOrStdout(), "  schemas: %d\n", report.Schemas)
	fmt.Fprintf(cmd.OutOrStdout(), "  fixtures: %d valid, %d rejected bad\n", report.GoodFixtures, report.BadFixturesRejected)
	if report.StateFindings > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "  admitted findings: %d\n", report.StateFindings)
	}
	for _, failure := range report.Failures {
		fmt.Fprintf(cmd.OutOrStdout(), "  FAIL: %s\n", failure)
	}
	return nil
}

func normalizeInputPath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Clean(path)
}

func candidatePathFor(root, path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	rel, err := filepath.Rel(root, abs)
	if err != nil || rel == "." || rel == "" || rel == ".." || strings.HasPrefix(rel, "../") {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(rel)
}
