// practices: [ddd-bounded-context, knowledge-flywheel]

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/boshu2/agentops/cli/internal/canon"
	"github.com/spf13/cobra"
)

// canon CLI surface: the team-knowledge canon — learnings earned into a shared
// trusted set by independent attestation, never self-certified.
//
// .agents/canon/
//   citations.jsonl       — who USED each learning (proves useful)
//   verifications.jsonl   — who independently CHECKED each learning (proves true)
//   learnings/            — promoted, earned canon entries

const (
	canonDir         = ".agents/canon"
	canonCitations   = "citations.jsonl"
	canonVerifs      = "verifications.jsonl"
	canonLearnings   = "learnings"
	canonAuthorGuard = "an attestation by the entry's own author never counts toward promotion"
)

func canonLedgers(cwd string) (*canon.CitationLedger, *canon.VerificationLedger) {
	base := filepath.Join(cwd, canonDir)
	return canon.NewCitationLedger(filepath.Join(base, canonCitations)),
		canon.NewVerificationLedger(filepath.Join(base, canonVerifs))
}

var canonCmd = &cobra.Command{
	Use:   "canon",
	Short: "Team-knowledge canon: learnings earned by independent attestation",
	Long: `The team-knowledge canon is the trusted set of learnings a team shares.

A learning is *earned* into canon, not self-asserted. Promotion requires two
independent signals, and ` + canonAuthorGuard + `:

  cite     a learning was USED by another engineer   (proves useful)
  verify   a learning was CHECKED by another engineer (proves true)

Promotion succeeds only when an entry has both a cross-engineer citation and an
independent confirmation, and has not been independently refuted.`,
}

var canonCiteCmd = &cobra.Command{
	Use:   "cite <entry-id>",
	Short: "Record that you used a learning (the 'useful' signal)",
	Args:  cobra.ExactArgs(1),
	RunE:  runCanonCite,
}

var canonVerifyCmd = &cobra.Command{
	Use:   "verify <entry-id>",
	Short: "Record an independent check of a learning (the 'true' signal)",
	Long: `Record that you independently checked a learning and whether it holds.

A verification is only independent if you GATHERED YOUR OWN EVIDENCE — pass
--receipt with the gate log, file:line, or hash you actually observed, rather
than trusting a summary. Receiptless verifications are recorded but the
promotion gate can be configured to ignore them.`,
	Args: cobra.ExactArgs(1),
	RunE: runCanonVerify,
}

var canonStatusCmd = &cobra.Command{
	Use:   "status [entry-id]",
	Short: "Show promotion eligibility for one or all tracked entries",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runCanonStatus,
}

var canonPromoteCmd = &cobra.Command{
	Use:   "promote <entry-id>",
	Short: "Promote an earned learning into the team canon",
	Args:  cobra.ExactArgs(1),
	RunE:  runCanonPromote,
}

func init() {
	canonCmd.GroupID = "core"
	rootCmd.AddCommand(canonCmd)

	canonCiteCmd.Flags().String("path", "", "path to the learning file (required)")
	canonCiteCmd.Flags().String("query", "", "search query that surfaced the learning")
	canonCiteCmd.Flags().String("session", "", "session ID")
	canonCiteCmd.Flags().String("as", "", "acting actor override (\"Name\" or \"Name <email>\"); else AGENTOPS_ACTOR / git")
	_ = canonCiteCmd.MarkFlagRequired("path")
	canonCmd.AddCommand(canonCiteCmd)

	canonVerifyCmd.Flags().String("path", "", "path to the learning file (required)")
	canonVerifyCmd.Flags().String("verdict", "confirmed", "confirmed|refuted")
	canonVerifyCmd.Flags().String("method", "manual", "how it was checked: manual|ao-verify|council|cross-model")
	canonVerifyCmd.Flags().String("receipt", "", "evidence you independently gathered (gate log, file:line, hash)")
	canonVerifyCmd.Flags().String("as", "", "acting actor override (\"Name\" or \"Name <email>\"); else AGENTOPS_ACTOR / git")
	canonVerifyCmd.Flags().Bool("council", false, "obtain the verdict from an independent cross-vendor judge (cmd: AGENTOPS_CANON_VERIFIER_CMD) instead of asserting it")
	_ = canonVerifyCmd.MarkFlagRequired("path")
	canonCmd.AddCommand(canonVerifyCmd)

	canonCmd.AddCommand(canonStatusCmd)

	canonPromoteCmd.Flags().String("path", "", "path to the learning file (required)")
	canonPromoteCmd.Flags().Bool("force", false, "promote despite an unmet gate (records the override loudly)")
	_ = canonPromoteCmd.MarkFlagRequired("path")
	canonCmd.AddCommand(canonPromoteCmd)
}

func runCanonCite(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	path, _ := cmd.Flags().GetString("path")
	query, _ := cmd.Flags().GetString("query")
	session, _ := cmd.Flags().GetString("session")
	as, _ := cmd.Flags().GetString("as")
	cl, _ := canonLedgers(cwd)

	by, src := canon.ResolveIdentity(as)
	c, err := cl.Record(args[0], path, query, session, by, time.Now())
	if err != nil {
		return fmt.Errorf("record citation: %w", err)
	}
	return emitCanonAttestation(cmd.OutOrStdout(), "cite", args[0], c.By, src, c.Self)
}

func runCanonVerify(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	path, _ := cmd.Flags().GetString("path")
	method, _ := cmd.Flags().GetString("method")
	receipt, _ := cmd.Flags().GetString("receipt")
	verdictStr, _ := cmd.Flags().GetString("verdict")
	as, _ := cmd.Flags().GetString("as")
	council, _ := cmd.Flags().GetBool("council")
	_, vl := canonLedgers(cwd)

	if council {
		return runCanonCouncilVerify(cmd, args[0], path, vl)
	}

	verdict := canon.Verdict(verdictStr)
	if verdict != canon.VerdictConfirmed && verdict != canon.VerdictRefuted {
		return fmt.Errorf("invalid --verdict %q: want confirmed|refuted", verdictStr)
	}

	by, src := canon.ResolveIdentity(as)
	v, err := vl.Record(args[0], path, method, receipt, verdict, by, time.Now())
	if err != nil {
		return fmt.Errorf("record verification: %w", err)
	}
	if err := emitCanonAttestation(cmd.OutOrStdout(), "verify("+verdictStr+")", args[0], v.By, src, v.Self); err != nil {
		return err
	}
	if v.Receipt == "" && GetOutput() != "json" {
		fmt.Fprintln(cmd.OutOrStdout(), "  note: no --receipt recorded; this is a weak (unreceipted) verification")
	}
	return nil
}

// runCanonCouncilVerify obtains the verdict from an independent cross-vendor
// judge instead of an operator assertion. The verdict is attributed to the
// judge (not whoever ran the command), so it counts toward promotion even when
// the operator authored the learning — independence by construction.
func runCanonCouncilVerify(cmd *cobra.Command, entryID, path string, vl *canon.VerificationLedger) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read learning %s: %w", path, err)
	}
	judgeName := os.Getenv("AGENTOPS_CANON_JUDGE")
	if judgeName == "" {
		judgeName = "codex" // the cross-vendor lane in this fleet
	}
	verifier := canon.CommandVerifier{
		Command: os.Getenv("AGENTOPS_CANON_VERIFIER_CMD"),
		JudgeID: canon.Identity{Name: judgeName},
	}
	verdict, judge, evidence, err := verifier.Judge(canon.Claim{EntryID: entryID, Path: path, Content: string(content)})
	if err != nil {
		return fmt.Errorf("council verify: %w", err)
	}

	v, err := vl.Record(entryID, path, "council", truncateReceipt(evidence, 4000), verdict, judge, time.Now())
	if err != nil {
		return fmt.Errorf("record verification: %w", err)
	}

	if GetOutput() == "json" {
		return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]any{
			"kind": "verify", "method": "council", "entry_id": entryID,
			"verdict": verdict, "judge": judge, "self": v.Self,
		})
	}
	fmt.Fprintf(cmd.OutOrStdout(), "recorded council verify(%s) for %s by judge %s [independent cross-vendor]\n", verdict, entryID, judge.Name)
	return nil
}

func truncateReceipt(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "\n…[truncated]"
}

func emitCanonAttestation(w io.Writer, kind, entryID string, by canon.Identity, src canon.IdentitySource, self bool) error {
	if GetOutput() == "json" {
		return json.NewEncoder(w).Encode(map[string]any{
			"kind": kind, "entry_id": entryID, "by": by, "source": src, "self": self,
		})
	}
	flag := ""
	if self {
		flag = "  [self — does not count toward promotion]"
	}
	fmt.Fprintf(w, "recorded %s for %s by %s (%s)%s\n", kind, entryID, by.Name, src, flag)
	return nil
}

func runCanonStatus(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	cl, vl := canonLedgers(cwd)

	var ids []string
	if len(args) == 1 {
		ids = []string{args[0]}
	} else {
		ids, err = canonTrackedEntries(cwd)
		if err != nil {
			return err
		}
	}

	decisions := make([]canon.Decision, 0, len(ids))
	for _, id := range ids {
		d, err := canon.EvaluateEntry(id, canon.EntryPath(id, cl, vl), cl, vl)
		if err != nil {
			return err
		}
		decisions = append(decisions, d)
	}

	if GetOutput() == "json" {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(decisions)
	}
	if len(decisions) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "no tracked entries (record a cite or verify first)")
		return nil
	}
	for _, d := range decisions {
		status := "PENDING"
		if d.Eligible {
			status = "EARNED "
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%s %-24s [%s] cites=%d verifs=%d\n", status, d.EntryID, d.Tier, d.Citations, d.Verifications)
		for _, u := range d.Unmet {
			fmt.Fprintf(cmd.OutOrStdout(), "          - %s\n", u)
		}
	}
	return nil
}

func runCanonPromote(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	entryID := args[0]
	path, _ := cmd.Flags().GetString("path")
	force, _ := cmd.Flags().GetBool("force")

	cl, vl := canonLedgers(cwd)
	d, err := canon.EvaluateEntry(entryID, path, cl, vl)
	if err != nil {
		return err
	}

	if !d.Eligible && !force {
		if GetOutput() == "json" {
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			_ = enc.Encode(map[string]any{"promoted": false, "decision": d})
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "BLOCKED %s is not earned:\n", entryID)
			for _, u := range d.Unmet {
				fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", u)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "(%s)\n", canonAuthorGuard)
		}
		return fmt.Errorf("entry %s not earned", entryID)
	}

	dest := filepath.Join(cwd, canonDir, canonLearnings, filepath.Base(path))
	if err := copyFileInto(path, dest); err != nil {
		return fmt.Errorf("promote into canon: %w", err)
	}

	forced := !d.Eligible && force
	if GetOutput() == "json" {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(map[string]any{"promoted": true, "forced": forced, "dest": dest, "decision": d})
	}
	if forced {
		fmt.Fprintf(cmd.OutOrStdout(), "FORCED  promoted %s despite unmet gate (override recorded)\n", entryID)
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "EARNED  promoted %s → %s\n", entryID, dest)
	}
	return nil
}

// canonTrackedEntries returns the distinct entry IDs that appear in either
// ledger, sorted for stable output.
func canonTrackedEntries(cwd string) ([]string, error) {
	cl, vl := canonLedgers(cwd)
	seen := map[string]struct{}{}
	cites, err := cl.AllEntryIDs()
	if err != nil {
		return nil, err
	}
	verifs, err := vl.AllEntryIDs()
	if err != nil {
		return nil, err
	}
	for _, id := range append(cites, verifs...) {
		seen[id] = struct{}{}
	}
	ids := make([]string, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids, nil
}

func copyFileInto(src, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dest, data, 0o644)
}
