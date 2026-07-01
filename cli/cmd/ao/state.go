// practices: [hexagonal-architecture, design-by-contract]
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/aostate"
)

var (
	stateAdmitCandidate   string
	stateAdmitVerdict     string
	stateAdmitFinding     string
	stateAdmitDestination string
	stateAdmitMaxAgeDays  int
	stateVerifyAll        bool
)

var stateCmd = &cobra.Command{
	Use:   "state",
	Short: "Validate, admit, verify, and doctor AgentOps state memory",
	Long: `Manage the durable AgentOps state-memory contract.

State findings are schema-validated, fresh, non-self-reviewed artifacts. The
admission path rejects stale candidates, self-review, leaked secrets, and writes
accepted authority only under .ao/accepted with an admission ledger row.`,
}

var stateValidateCmd = &cobra.Command{
	Use:   "validate <file> [file...]",
	Short: "Validate state memory JSON files against their schemas",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runStateValidate,
}

var stateCandidateCmd = &cobra.Command{
	Use:   "candidate",
	Short: "Inspect inert .ao state candidates",
}

var stateCandidateValidateCmd = &cobra.Command{
	Use:   "validate <path>",
	Short: "Validate an inert .ao Finding candidate and print its digest",
	Args:  cobra.ExactArgs(1),
	RunE:  runStateCandidateValidate,
}

var stateReviewRequestCmd = &cobra.Command{
	Use:   "review-request <candidate>",
	Short: "Emit the digest-bound review request for a Finding candidate",
	Args:  cobra.ExactArgs(1),
	RunE:  runStateReviewRequest,
}

var stateAdmitCmd = &cobra.Command{
	Use:   "admit --candidate <path> --verdict <path>",
	Short: "Admit one independently reviewed Finding candidate into .ao/accepted",
	Args:  cobra.NoArgs,
	RunE:  runStateAdmit,
}

var stateVerifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "Verify .ao state schemas, fixtures, accepted findings, and ledger rows",
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
	// Folded under `ao session` (age-focus-membrane-bookkeeper-m1wg.17). No
	// back-compat alias — `ao state` had no external callers. The GroupID was
	// dropped because sessionCmd defines no command groups (cobra panics at
	// Execute if a child carries a GroupID its parent doesn't define).
	sessionCmd.AddCommand(stateCmd)
	stateCmd.AddCommand(stateValidateCmd, stateCandidateCmd, stateReviewRequestCmd, stateAdmitCmd, stateVerifyCmd, stateDoctorCmd)
	stateCandidateCmd.AddCommand(stateCandidateValidateCmd)

	stateAdmitCmd.Flags().StringVar(&stateAdmitCandidate, "candidate", "", "Path to an .ao Finding candidate JSON file (required)")
	stateAdmitCmd.Flags().StringVar(&stateAdmitVerdict, "verdict", "", "Path to an independent admission verdict JSON file (required)")
	stateAdmitCmd.Flags().StringVar(&stateAdmitFinding, "finding", "", "Deprecated alias for --candidate")
	stateAdmitCmd.Flags().StringVar(&stateAdmitDestination, "destination", "", "Destination under .ao/accepted/findings/ (default: finding id)")
	stateAdmitCmd.Flags().IntVar(&stateAdmitMaxAgeDays, "max-age-days", 30, "Maximum age for reviewed findings")
	_ = stateAdmitCmd.Flags().MarkHidden("finding")
	stateVerifyCmd.Flags().BoolVar(&stateVerifyAll, "all", false, "Verify all .ao state authority surfaces")
}

func runStateValidate(cmd *cobra.Command, args []string) error {
	root, err := repoRootOrCwd()
	if err != nil {
		return err
	}
	validated := 0
	for _, arg := range args {
		path := normalizeInputPath(arg)
		if err := aostate.ValidateStateFile(root, path); err != nil {
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

func runStateCandidateValidate(cmd *cobra.Command, args []string) error {
	root, err := repoRootOrCwd()
	if err != nil {
		return err
	}
	path := normalizeInputPath(args[0])
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read candidate: %w", err)
	}
	if err := aostate.ValidateCandidateFile(root, path); err != nil {
		return err
	}
	digest, err := aostate.CanonicalDigest(data)
	if err != nil {
		return err
	}
	if GetOutput() == "json" {
		return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]any{
			"verdict": "PASS",
			"path":    candidatePathFor(root, path),
			"digest":  digest,
		})
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Candidate valid: %s\n", candidatePathFor(root, path))
	fmt.Fprintf(cmd.OutOrStdout(), "Digest: %s\n", digest)
	return nil
}

func runStateReviewRequest(cmd *cobra.Command, args []string) error {
	root, err := repoRootOrCwd()
	if err != nil {
		return err
	}
	path := normalizeInputPath(args[0])
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read candidate: %w", err)
	}
	if err := aostate.ValidateCandidateFile(root, path); err != nil {
		return err
	}
	digest, err := aostate.CanonicalDigest(data)
	if err != nil {
		return err
	}
	payload := map[string]any{
		"kind":             "ao_state_review_request",
		"candidate_path":   candidatePathFor(root, path),
		"candidate_digest": digest,
		"required_verdict_fields": []string{
			"candidate_id",
			"candidate_digest",
			"reviewer_id",
			"reviewer_context_id",
			"reviewer_family",
			"evidence_ref",
			"proof_ref",
		},
	}
	if GetOutput() == "json" {
		return json.NewEncoder(cmd.OutOrStdout()).Encode(payload)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Review request for %s\n", payload["candidate_path"])
	fmt.Fprintf(cmd.OutOrStdout(), "Candidate digest: %s\n", digest)
	fmt.Fprintln(cmd.OutOrStdout(), "Verdict must bind reviewer identity, evidence_ref, and proof_ref.")
	return nil
}

func runStateAdmit(cmd *cobra.Command, _ []string) error {
	root, err := repoRootOrCwd()
	if err != nil {
		return err
	}
	candidateArg := stateAdmitCandidate
	if strings.TrimSpace(candidateArg) == "" {
		candidateArg = stateAdmitFinding
	}
	if strings.TrimSpace(candidateArg) == "" {
		return fmt.Errorf("ao state admit requires --candidate <path>")
	}
	if strings.TrimSpace(stateAdmitVerdict) == "" {
		return fmt.Errorf("ao state admit requires --verdict <path>")
	}
	candidatePath := normalizeInputPath(candidateArg)
	candidateBytes, err := os.ReadFile(candidatePath)
	if err != nil {
		return fmt.Errorf("read candidate: %w", err)
	}
	verdictPath := normalizeInputPath(stateAdmitVerdict)
	verdictBytes, err := os.ReadFile(verdictPath)
	if err != nil {
		return fmt.Errorf("read verdict: %w", err)
	}
	req := aostate.AdmissionRequest{
		CandidatePath:    candidatePathFor(root, candidatePath),
		VerdictPath:      candidatePathFor(root, verdictPath),
		Destination:      stateAdmitDestination,
		OperatorID:       GetCurrentUser(),
		Reason:           "ao state admit",
		ExecutionContext: admissionExecutionContext(root),
	}
	report, err := aostate.AdmitFinding(context.Background(), candidateBytes, verdictBytes, req, aostate.AdmissionOptions{
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
		fmt.Fprintf(cmd.OutOrStdout(), "Ledger: %s\n", report.LedgerPath)
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
	fmt.Fprintf(cmd.OutOrStdout(), "  accepted findings: %d\n", report.AcceptedFindings)
	fmt.Fprintf(cmd.OutOrStdout(), "  admission ledger rows: %d\n", report.LedgerRows)
	for _, failure := range report.Failures {
		fmt.Fprintf(cmd.OutOrStdout(), "  FAIL: %s\n", failure)
	}
	if report.Verdict != "PASS" {
		return fmt.Errorf("state doctor found %d failure(s)", len(report.Failures))
	}
	return nil
}

func runStateVerifyCore(ctx context.Context) (aostate.VerifyReport, error) {
	root, err := repoRootOrCwd()
	if err != nil {
		return aostate.VerifyReport{}, err
	}
	return aostate.VerifyRepo(ctx, root)
}

func emitStateVerifyReport(cmd *cobra.Command, report aostate.VerifyReport) error {
	if GetOutput() == "json" {
		return json.NewEncoder(cmd.OutOrStdout()).Encode(report)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "State verify: %s\n", report.Verdict)
	fmt.Fprintf(cmd.OutOrStdout(), "  schemas: %d\n", report.Schemas)
	fmt.Fprintf(cmd.OutOrStdout(), "  fixtures: %d valid, %d rejected bad\n", report.GoodFixtures, report.BadFixturesRejected)
	if report.AcceptedFindings > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "  accepted findings: %d\n", report.AcceptedFindings)
	}
	if report.LedgerRows > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "  admission ledger rows: %d\n", report.LedgerRows)
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

func admissionExecutionContext(root string) aostate.ExecutionContext {
	reservationID := firstNonEmptyEnv("AO_RESERVATION_ID", "AGENTOPS_RESERVATION_ID", "ATM_RESERVATION_ID")
	execution := aostate.ExecutionContext{
		WorktreePath:             root,
		Branch:                   gitStateOutput(root, "rev-parse", "--abbrev-ref", "HEAD"),
		HeadSHA:                  gitStateOutput(root, "rev-parse", "HEAD"),
		ReservationID:            reservationID,
		CanonicalBeadStateSource: "_beads/issues.jsonl",
	}
	if execution.Branch == "" {
		execution.Branch = "unknown"
	}
	if execution.HeadSHA == "" {
		execution.HeadSHA = "unknown"
	}
	if reservationID != "" {
		execution.PaneMode = "multi-pane"
		return execution
	}
	execution.PaneMode = "single-pane"
	execution.SinglePaneDowngradeReason = "no active ATM reservation metadata supplied to ao state admit"
	return execution
}

func gitStateOutput(root string, args ...string) string {
	fullArgs := append([]string{"-C", root}, args...)
	out, err := exec.Command("git", fullArgs...).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func firstNonEmptyEnv(names ...string) string {
	for _, name := range names {
		if value := strings.TrimSpace(os.Getenv(name)); value != "" {
			return value
		}
	}
	return ""
}
