// practices: [hexagonal-architecture, ddd-bounded-context]
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/claimproof"
	"github.com/boshu2/agentops/cli/internal/ports"
)

// claimCmd is the parent for BC2 ClaimEvidenceBinderPort CLI
// surfaces. Built using the cycle-147 cli-wiring template.
var claimCmd = &cobra.Command{
	Use:   "claim [id]",
	Short: "Claim a BR bead or manage claim-evidence bindings",
	Long: `Claim a BR bead for harness-neutral AgentOps loops, or bind/list claim evidence via the typed BC2 ClaimEvidenceBinderPort.

Examples:
  ao claim cp-123
  ao claim bind --claim AOP-CLAIM-X --path .agents/findings/x.md --level PG2
  ao claim list`,
	RunE: runClaim,
}

var claimBindCmd = &cobra.Command{
	Use:   "bind --claim <AOP-CLAIM-X> --path <evidence-path> [--level PG1|PG2|PG3|PG4] [--anchor ...] [--author-id <id> --judge-id <id>]",
	Short: "Bind a claim to an evidence file at a promotion level",
	Long: `Append (or upgrade) a claim→evidence binding via the typed BC2
ClaimEvidenceBinderPort (productionClaimEvidenceBinder, cycle 116).

The binder is append-only on disk; List replays the file and folds
to the latest per (Claim, Path). The Level can only go UP — attempting
to downgrade (e.g., PG3 → PG1) returns an error.

Examples:
  ao claim bind --claim AOP-CLAIM-X --path .agents/findings/x.md --level PG2
  ao claim bind --claim AOP-CLAIM-Y --path p.md --level PG4 --anchor L10 --anchor L20
  ao claim bind --claim AOP-CLAIM-Z --path verdict.md --level PG4 --author-id worker-1 --judge-id verifier-2`,
	RunE: runClaimBind,
}

var claimListCmd = &cobra.Command{
	Use:   "list",
	Short: "List recorded claim→evidence bindings (most-recent first)",
	Long:  `Emit all known claim→evidence bindings via the typed BC2 ClaimEvidenceBinderPort. Output is line-delimited JSON.`,
	RunE:  runClaimList,
}

var claimCheckCmd = &cobra.Command{
	Use:   "check --changed [--base <ref>]",
	Short: "Report proof cards for changed public claims",
	Long: `Report read-only proof cards for changed public claim markers.

The checker compares the current branch/worktree to a base ref, finds changed
files containing agentops:claim markers, then reports each claim's registry tier,
evidence citation status, verdict, and next action. It does not bind evidence,
promote tiers, mutate beads, or edit the claim registry.

Examples:
  ao claim check --changed
  ao claim check --changed --base origin/main --json`,
	RunE: runClaimCheck,
}

type claimOptions struct {
	claim    string
	path     string
	level    string
	anchors  []string
	authorID string
	judgeID  string
	writer   io.Writer
	bindFn   func(ctx context.Context, opts claimOptions) error
	listFn   func(ctx context.Context, opts claimOptions) ([]ports.EvidenceBinding, error)
}

type claimCheckOptions struct {
	repoRoot    string
	base        string
	changedOnly bool
	jsonOutput  bool
	writer      io.Writer
	checkFn     func(context.Context, claimproof.Options) (claimproof.Report, error)
}

func init() {
	claimCmd.GroupID = "core"
	rootCmd.AddCommand(claimCmd)

	claimBindCmd.Flags().String("claim", "", "claim ID (required, e.g. AOP-CLAIM-X)")
	claimBindCmd.Flags().String("path", "", "evidence file path (required, relative to repo root)")
	claimBindCmd.Flags().String("level", "PG1", "promotion level: PG1|PG2|PG3|PG4")
	claimBindCmd.Flags().StringArray("anchor", nil, "optional in-file anchors (repeatable)")
	claimBindCmd.Flags().String("author-id", "", "artifact author identity for reviewer separation checks")
	claimBindCmd.Flags().String("judge-id", "", "judge/verifier identity for reviewer separation checks")
	_ = claimBindCmd.MarkFlagRequired("claim")
	_ = claimBindCmd.MarkFlagRequired("path")
	claimCmd.AddCommand(claimBindCmd)

	claimCmd.AddCommand(claimListCmd)

	claimCheckCmd.Flags().Bool("changed", false, "check claim markers in files changed against --base plus worktree changes")
	claimCheckCmd.Flags().String("base", "origin/main", "base ref for --changed comparison")
	claimCmd.AddCommand(claimCheckCmd)
}

func runClaim(cmd *cobra.Command, args []string) error {
	if len(args) != 1 {
		return cmd.Help()
	}
	return tickPassthrough(newTickRuntime(cmd), "br", "update", args[0], "--claim")
}

func runClaimBind(cmd *cobra.Command, _ []string) error {
	claim, _ := cmd.Flags().GetString("claim")
	path, _ := cmd.Flags().GetString("path")
	level, _ := cmd.Flags().GetString("level")
	anchors, _ := cmd.Flags().GetStringArray("anchor")
	authorID, _ := cmd.Flags().GetString("author-id")
	judgeID, _ := cmd.Flags().GetString("judge-id")
	return claimBindRun(cmd.Context(), claimOptions{
		claim:    claim,
		path:     path,
		level:    level,
		anchors:  anchors,
		authorID: authorID,
		judgeID:  judgeID,
		writer:   cmd.OutOrStdout(),
	})
}

func runClaimList(cmd *cobra.Command, _ []string) error {
	return claimListRun(cmd.Context(), claimOptions{
		writer: cmd.OutOrStdout(),
	})
}

func runClaimCheck(cmd *cobra.Command, _ []string) error {
	changedOnly, _ := cmd.Flags().GetBool("changed")
	base, _ := cmd.Flags().GetString("base")
	repoRoot, err := resolveProjectDir()
	if err != nil {
		return err
	}
	return claimCheckRun(cmd.Context(), claimCheckOptions{
		repoRoot:    repoRoot,
		base:        base,
		changedOnly: changedOnly,
		jsonOutput:  GetOutput() == "json",
		writer:      cmd.OutOrStdout(),
	})
}

func claimBindRun(ctx context.Context, opts claimOptions) error {
	if opts.claim == "" {
		return errors.New("claim bind: --claim required\n  Example: ao claim bind --claim AOP-CLAIM-1 --path internal/foo.go")
	}
	if opts.path == "" {
		return errors.New("claim bind: --path required (evidence file, relative to repo root)\n  Example: ao claim bind --claim AOP-CLAIM-1 --path internal/foo.go")
	}
	if err := validateEvidenceLevelString(opts.level); err != nil {
		return fmt.Errorf("claim bind: %w", err)
	}
	fn := opts.bindFn
	if fn == nil {
		fn = claimBindViaPort
	}
	if err := fn(ctx, opts); err != nil {
		return fmt.Errorf("claim bind: %w", err)
	}
	if opts.writer == nil {
		opts.writer = os.Stdout
	}
	fmt.Fprintf(opts.writer, "bound claim=%q path=%q level=%s\n", opts.claim, opts.path, opts.level)
	return nil
}

func claimListRun(ctx context.Context, opts claimOptions) error {
	if opts.writer == nil {
		opts.writer = os.Stdout
	}
	fn := opts.listFn
	if fn == nil {
		fn = claimListViaPort
	}
	bindings, err := fn(ctx, opts)
	if err != nil {
		return fmt.Errorf("claim list: %w", err)
	}
	enc := json.NewEncoder(opts.writer)
	for _, b := range bindings {
		if err := enc.Encode(b); err != nil {
			return fmt.Errorf("claim list encode: %w", err)
		}
	}
	return nil
}

func claimCheckRun(ctx context.Context, opts claimCheckOptions) error {
	if !opts.changedOnly {
		return errors.New("claim check: --changed is required for this read-only MVP")
	}
	if opts.writer == nil {
		opts.writer = os.Stdout
	}
	fn := opts.checkFn
	if fn == nil {
		fn = claimproof.Check
	}
	report, err := fn(ctx, claimproof.Options{
		RepoRoot:    opts.repoRoot,
		Base:        opts.base,
		ChangedOnly: opts.changedOnly,
	})
	if err != nil {
		return fmt.Errorf("claim check: %w", err)
	}
	if opts.jsonOutput {
		enc := json.NewEncoder(opts.writer)
		enc.SetIndent("", "  ")
		return enc.Encode(report)
	}
	writeClaimCheckHuman(opts.writer, report)
	return nil
}

func writeClaimCheckHuman(w io.Writer, report claimproof.Report) {
	fmt.Fprintf(w, "claim check: %d changed claim(s) across %d surface(s) (base=%s)\n",
		report.Summary.Claims, report.Summary.ChangedSurfaces, report.Summary.Base)
	if len(report.Cards) == 0 {
		fmt.Fprintln(w, "No changed claim markers found.")
		return
	}
	for _, card := range report.Cards {
		fmt.Fprintf(w, "\n%s\n", card.ClaimID)
		fmt.Fprintf(w, "  surface: %s\n", card.Surface)
		fmt.Fprintf(w, "  tier: %s\n", card.Tier)
		fmt.Fprintf(w, "  verdict: %s\n", card.Verdict)
		if card.EvalBinding != "" {
			fmt.Fprintf(w, "  eval_binding: %s\n", card.EvalBinding)
		}
		if len(card.Evidence) == 0 {
			fmt.Fprintln(w, "  evidence: none")
		} else {
			fmt.Fprintln(w, "  evidence:")
			for _, evidence := range card.Evidence {
				if evidence.Reason == "" {
					fmt.Fprintf(w, "    - %s [%s]\n", evidence.Path, evidence.Status)
				} else {
					fmt.Fprintf(w, "    - %s [%s] %s\n", evidence.Path, evidence.Status, evidence.Reason)
				}
			}
		}
		fmt.Fprintf(w, "  next: %s\n", card.NextAction)
	}
}

// validateEvidenceLevelString accepts PG1, PG2, PG3, PG4 (or empty
// which becomes None at the port boundary).
func validateEvidenceLevelString(s string) error {
	switch strings.ToUpper(s) {
	case "", "PG1", "PG2", "PG3", "PG4":
		return nil
	}
	return fmt.Errorf("invalid --level %q (want PG1|PG2|PG3|PG4)", s)
}

func claimBindingsPath() (string, error) {
	cwd, err := resolveProjectDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cwd, ".agents", "findings", "evidence-bindings.jsonl"), nil
}

func claimBindViaPort(ctx context.Context, opts claimOptions) error {
	path, err := claimBindingsPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	b := newProductionClaimEvidenceBinder(path)
	return b.Bind(ctx, ports.EvidenceBinding{
		Claim:    ports.ClaimID(opts.claim),
		Path:     opts.path,
		Level:    ports.EvidenceLevel(strings.ToUpper(opts.level)),
		Anchors:  opts.anchors,
		AuthorID: opts.authorID,
		JudgeID:  opts.judgeID,
	})
}

func claimListViaPort(ctx context.Context, opts claimOptions) ([]ports.EvidenceBinding, error) {
	path, err := claimBindingsPath()
	if err != nil {
		return nil, err
	}
	b := newProductionClaimEvidenceBinder(path)
	return b.List(ctx)
}
