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

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/ports"
)

// operatorCmd is the parent for BC4 OperatorPort CLI surfaces.
// Built using the cycle-147 cli-wiring template — parent noun + verb
// subcommands + injectable function field for tests.
var operatorCmd = &cobra.Command{
	Use:   "operator",
	Short: "BC4 OperatorPort operations (record/list operator intents)",
	Long:  `Read and write operator intents via the typed BC4 OperatorPort. Intents are durable records of operator decisions (halt, rescope, handoff) appended to .agents/operator/intents.jsonl.`,
}

var operatorRecordCmd = &cobra.Command{
	Use:   "record --kind <kind> [--subject S] [--note N]",
	Short: "Record an operator intent via BC4 OperatorPort",
	Long: `Append an OperatorIntent to .agents/operator/intents.jsonl via the
typed BC4 OperatorPort (productionOperator, cycle 110).

The Kind field is required; Subject and Note are optional.

Examples:
  ao operator record --kind halt --subject soc-y5vh --note "investigate before .1 work"
  ao operator record --kind rescope --note "split into 2 epics"
  ao operator record --kind handoff --subject "next session"`,
	RunE: runOperatorRecord,
}

var operatorListCmd = &cobra.Command{
	Use:   "list",
	Short: "List recorded operator intents (most-recent first)",
	Long: `Emit recorded OperatorIntents from .agents/operator/intents.jsonl
via the typed BC4 OperatorPort, most-recent first. Output is line-
delimited JSON.`,
	RunE: runOperatorList,
}

type operatorOptions struct {
	kind     string
	subject  string
	note     string
	writer   io.Writer
	recordFn func(ctx context.Context, opts operatorOptions) error
	listFn   func(ctx context.Context, opts operatorOptions) ([]ports.OperatorIntent, error)
}

func init() {
	operatorCmd.GroupID = "core"
	rootCmd.AddCommand(operatorCmd)

	operatorRecordCmd.Flags().String("kind", "", "intent kind (required: halt|rescope|handoff|other)")
	operatorRecordCmd.Flags().String("subject", "", "intent subject (e.g., bd ID, file path)")
	operatorRecordCmd.Flags().String("note", "", "free-text note")
	_ = operatorRecordCmd.MarkFlagRequired("kind")
	operatorCmd.AddCommand(operatorRecordCmd)

	operatorCmd.AddCommand(operatorListCmd)
}

func runOperatorRecord(cmd *cobra.Command, _ []string) error {
	kind, _ := cmd.Flags().GetString("kind")
	subject, _ := cmd.Flags().GetString("subject")
	note, _ := cmd.Flags().GetString("note")
	return operatorRecordRun(cmd.Context(), operatorOptions{
		kind:    kind,
		subject: subject,
		note:    note,
		writer:  cmd.OutOrStdout(),
	})
}

func runOperatorList(cmd *cobra.Command, _ []string) error {
	return operatorListRun(cmd.Context(), operatorOptions{
		writer: cmd.OutOrStdout(),
	})
}

func operatorRecordRun(ctx context.Context, opts operatorOptions) error {
	if opts.kind == "" {
		return errors.New("operator record: --kind required")
	}
	fn := opts.recordFn
	if fn == nil {
		fn = operatorRecordViaPort
	}
	if err := fn(ctx, opts); err != nil {
		return fmt.Errorf("operator record: %w", err)
	}
	if opts.writer == nil {
		opts.writer = os.Stdout
	}
	fmt.Fprintf(opts.writer, "recorded intent kind=%q subject=%q\n", opts.kind, opts.subject)
	return nil
}

func operatorListRun(ctx context.Context, opts operatorOptions) error {
	if opts.writer == nil {
		opts.writer = os.Stdout
	}
	fn := opts.listFn
	if fn == nil {
		fn = operatorListViaPort
	}
	intents, err := fn(ctx, opts)
	if err != nil {
		return fmt.Errorf("operator list: %w", err)
	}
	enc := json.NewEncoder(opts.writer)
	for _, intent := range intents {
		if err := enc.Encode(intent); err != nil {
			return fmt.Errorf("operator list encode: %w", err)
		}
	}
	return nil
}

func operatorIntentsPath() (string, error) {
	cwd, err := resolveProjectDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cwd, ".agents", "operator", "intents.jsonl"), nil
}

func operatorRecordViaPort(ctx context.Context, opts operatorOptions) error {
	path, err := operatorIntentsPath()
	if err != nil {
		return err
	}
	// Ensure parent dir exists; productionOperator's Record only
	// creates the file with O_CREATE, not the directory.
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	op := newProductionOperator(path)
	return op.Record(ctx, ports.OperatorIntent{
		Kind:    opts.kind,
		Subject: opts.subject,
		Note:    opts.note,
	})
}

func operatorListViaPort(ctx context.Context, opts operatorOptions) ([]ports.OperatorIntent, error) {
	path, err := operatorIntentsPath()
	if err != nil {
		return nil, err
	}
	op := newProductionOperator(path)
	return op.List(ctx)
}
