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

	"github.com/boshu2/agentops/cli/internal/ports"
)

// claimCmd is the parent for BC2 ClaimEvidenceBinderPort CLI
// surfaces. Built using the cycle-147 cli-wiring template.
var claimCmd = &cobra.Command{
	Use:   "claim",
	Short: "BC2 ClaimEvidenceBinderPort operations (bind/list claim-evidence)",
	Long:  `Bind claims to evidence files at a promotion level (PG1-PG4) and list existing bindings, via the typed BC2 ClaimEvidenceBinderPort.`,
}

var claimBindCmd = &cobra.Command{
	Use:   "bind --claim <AOP-CLAIM-X> --path <evidence-path> [--level PG1|PG2|PG3|PG4] [--anchor ...]",
	Short: "Bind a claim to an evidence file at a promotion level",
	Long: `Append (or upgrade) a claim→evidence binding via the typed BC2
ClaimEvidenceBinderPort (productionClaimEvidenceBinder, cycle 116).

The binder is append-only on disk; List replays the file and folds
to the latest per (Claim, Path). The Level can only go UP — attempting
to downgrade (e.g., PG3 → PG1) returns an error.

Examples:
  ao claim bind --claim AOP-CLAIM-X --path .agents/findings/x.md --level PG2
  ao claim bind --claim AOP-CLAIM-Y --path p.md --level PG4 --anchor L10 --anchor L20`,
	RunE: runClaimBind,
}

var claimListCmd = &cobra.Command{
	Use:   "list",
	Short: "List recorded claim→evidence bindings (most-recent first)",
	Long:  `Emit all known claim→evidence bindings via the typed BC2 ClaimEvidenceBinderPort. Output is line-delimited JSON.`,
	RunE:  runClaimList,
}

type claimOptions struct {
	claim   string
	path    string
	level   string
	anchors []string
	writer  io.Writer
	bindFn  func(ctx context.Context, opts claimOptions) error
	listFn  func(ctx context.Context, opts claimOptions) ([]ports.EvidenceBinding, error)
}

func init() {
	claimCmd.GroupID = "core"
	rootCmd.AddCommand(claimCmd)

	claimBindCmd.Flags().String("claim", "", "claim ID (required, e.g. AOP-CLAIM-X)")
	claimBindCmd.Flags().String("path", "", "evidence file path (required, relative to repo root)")
	claimBindCmd.Flags().String("level", "PG1", "promotion level: PG1|PG2|PG3|PG4")
	claimBindCmd.Flags().StringArray("anchor", nil, "optional in-file anchors (repeatable)")
	_ = claimBindCmd.MarkFlagRequired("claim")
	_ = claimBindCmd.MarkFlagRequired("path")
	claimCmd.AddCommand(claimBindCmd)

	claimCmd.AddCommand(claimListCmd)
}

func runClaimBind(cmd *cobra.Command, _ []string) error {
	claim, _ := cmd.Flags().GetString("claim")
	path, _ := cmd.Flags().GetString("path")
	level, _ := cmd.Flags().GetString("level")
	anchors, _ := cmd.Flags().GetStringArray("anchor")
	return claimBindRun(cmd.Context(), claimOptions{
		claim:   claim,
		path:    path,
		level:   level,
		anchors: anchors,
		writer:  cmd.OutOrStdout(),
	})
}

func runClaimList(cmd *cobra.Command, _ []string) error {
	return claimListRun(cmd.Context(), claimOptions{
		writer: cmd.OutOrStdout(),
	})
}

func claimBindRun(ctx context.Context, opts claimOptions) error {
	if opts.claim == "" {
		return errors.New("claim bind: --claim required")
	}
	if opts.path == "" {
		return errors.New("claim bind: --path required")
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
		Claim:   ports.ClaimID(opts.claim),
		Path:    opts.path,
		Level:   ports.EvidenceLevel(strings.ToUpper(opts.level)),
		Anchors: opts.anchors,
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
