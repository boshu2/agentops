// practices: [hexagonal-architecture, ddd-bounded-context]
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/ports"
)

// citationCmd is the parent for BC1 CitationPort CLI surfaces. Built
// using the cycle-147 cli-wiring template.
var citationCmd = &cobra.Command{
	Use:   "citation",
	Short: "BC1 CitationPort operations (verify file/function/symbol citations)",
	Long:  `Verify citation freshness via the typed BC1 CitationPort. Useful for cross-repo citation auditing and dream-loop staleness checks.`,
}

var citationVerifyCmd = &cobra.Command{
	Use:   "verify --kind <file|function|symbol> --raw <text>",
	Short: "Verify a single citation via BC1 CitationPort",
	Long: `Verify a single citation (file, function, or symbol) against HEAD
via the typed BC1 CitationPort (productionCitationAdapter, cycle 83).

Emits a JSON CitationVerdict (Status: FRESH|STALE|UNKNOWN, Reason,
optional Resolved location).

Examples:
  ao citation verify --kind file --raw "cli/cmd/ao/beads.go"
  ao citation verify --kind file --raw "cli/cmd/ao/beads.go:227"
  ao citation verify --kind function --raw "verifyCitationInPlace"
  ao citation verify --kind symbol --raw "ProductionCitationAdapter"`,
	RunE: runCitationVerify,
}

type citationVerifyOptions struct {
	kind     string
	raw      string
	writer   io.Writer
	verifyFn func(ctx context.Context, opts citationVerifyOptions) (ports.CitationVerdict, error)
}

func init() {
	citationCmd.GroupID = "core"
	rootCmd.AddCommand(citationCmd)

	citationVerifyCmd.Flags().String("kind", "", "citation kind (required: file|function|symbol)")
	citationVerifyCmd.Flags().String("raw", "", "citation text to verify (required)")
	_ = citationVerifyCmd.MarkFlagRequired("kind")
	_ = citationVerifyCmd.MarkFlagRequired("raw")
	citationCmd.AddCommand(citationVerifyCmd)
}

func runCitationVerify(cmd *cobra.Command, _ []string) error {
	kind, _ := cmd.Flags().GetString("kind")
	raw, _ := cmd.Flags().GetString("raw")
	return citationVerifyRun(cmd.Context(), citationVerifyOptions{
		kind:   kind,
		raw:    raw,
		writer: cmd.OutOrStdout(),
	})
}

func citationVerifyRun(ctx context.Context, opts citationVerifyOptions) error {
	if opts.kind == "" {
		return errors.New("citation verify: --kind required")
	}
	if opts.raw == "" {
		return errors.New("citation verify: --raw required")
	}
	if opts.writer == nil {
		opts.writer = os.Stdout
	}
	fn := opts.verifyFn
	if fn == nil {
		fn = citationVerifyViaPort
	}
	verdict, err := fn(ctx, opts)
	if err != nil {
		return fmt.Errorf("citation verify: %w", err)
	}
	enc := json.NewEncoder(opts.writer)
	if err := enc.Encode(verdict); err != nil {
		return fmt.Errorf("citation verify encode: %w", err)
	}
	return nil
}

func citationVerifyViaPort(ctx context.Context, opts citationVerifyOptions) (ports.CitationVerdict, error) {
	cwd, err := resolveProjectDir()
	if err != nil {
		return ports.CitationVerdict{}, err
	}
	adapter := newProductionCitationAdapter()
	return adapter.Verify(ctx, ports.CitationRequest{
		Kind: ports.CitationKind(opts.kind),
		Raw:  opts.raw,
		Cwd:  cwd,
	})
}
