// practices: [dora-metrics, lean-startup]
package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/boshu2/agentops/cli/internal/evolve"
	"github.com/spf13/cobra"
)

// cronSelfAdjust (soc-un0m) renders the next /evolve loop-mode cron prompt
// from the versioned template and emits a JSON spec the harness consumes to
// orchestrate CronCreate. The CLI never calls CronCreate itself — that boundary
// is intentional. See docs/plans/2026-05-21-evolve-loop-epic-design.md §A4.
//
// Audit trail: every render appends a row to .agents/evolve/cron-history.jsonl
// so operators can reconstruct the loop's prompt evolution.

const (
	cronSelfAdjustDefaultTemplate = ".agents/evolve/cron-template.md"
	cronSelfAdjustHistoryRel      = ".agents/evolve/cron-history.jsonl"
)

var (
	cronSelfAdjustOn         string
	cronSelfAdjustTemplate   string
	cronSelfAdjustShipped    string
	cronSelfAdjustNext       string
	cronSelfAdjustSubBeads   string
	cronSelfAdjustTestsDelta string
	cronSelfAdjustClock      func() time.Time
)

var cronSelfAdjustCmd = &cobra.Command{
	Use:   "self-adjust",
	Short: "Render the next loop-mode cron prompt and emit a CronCreate spec",
	Long: `Render the next /evolve loop-mode cron prompt and emit JSON for the harness.

This subcommand is the mechanical primitive the /evolve loop calls at the end
of every cycle. It:

  1. Reads the versioned cron template (default .agents/evolve/cron-template.md)
  2. Verifies VERBATIM-PRESERVE marker hashes (refuses on drift)
  3. Renders the template with the supplied shipped/next/sub-beads/tests-delta
  4. Appends one row to .agents/evolve/cron-history.jsonl
  5. Emits a JSON spec on stdout: {"new_cron_prompt": "<rendered>", "schedule_hint": "..."}

The CLI does NOT call CronCreate. The harness reads the JSON spec and
orchestrates CronList/Delete/Create itself; the CLI's responsibility ends at
emitting the spec.

--shipped accepts one or more "<commit-sha>:<bead-id>[#<scenario>]" entries,
comma-separated.

Example:
  ao cron self-adjust --on cycle-close \
    --template .agents/evolve/cron-template.md \
    --shipped abc123:soc-x,def456:soc-y#scen \
    --next soc-z --sub-beads soc-q,soc-r \
    --tests-delta "+3 passing, 0 new failures"`,
	Args: cobra.NoArgs,
	RunE: runCronSelfAdjust,
}

func init() {
	cronSelfAdjustClock = func() time.Time { return time.Now().UTC() }
	cronSelfAdjustCmd.Flags().StringVar(&cronSelfAdjustOn, "on", "cycle-close", "Trigger marker: 'cycle-close' for default loop usage")
	cronSelfAdjustCmd.Flags().StringVar(&cronSelfAdjustTemplate, "template", cronSelfAdjustDefaultTemplate, "Path to the cron-loop-mode template")
	cronSelfAdjustCmd.Flags().StringVar(&cronSelfAdjustShipped, "shipped", "", "Comma-separated commit:bead entries shipped this cycle")
	cronSelfAdjustCmd.Flags().StringVar(&cronSelfAdjustNext, "next", "", "Optional recommended next bead")
	cronSelfAdjustCmd.Flags().StringVar(&cronSelfAdjustSubBeads, "sub-beads", "", "Comma-separated bead ids filed this cycle")
	cronSelfAdjustCmd.Flags().StringVar(&cronSelfAdjustTestsDelta, "tests-delta", "", "Human-readable tests delta summary")
	cronCmd.AddCommand(cronSelfAdjustCmd)
}

// cronSelfAdjustSpec is the JSON spec written to stdout for the harness.
type cronSelfAdjustSpec struct {
	NewCronPrompt string `json:"new_cron_prompt"`
	ScheduleHint  string `json:"schedule_hint"`
}

// cronSelfAdjustHistoryRow is one row of cron-history.jsonl.
type cronSelfAdjustHistoryRow struct {
	Timestamp        string   `json:"timestamp"`
	CronIDBefore     string   `json:"cron_id_before"`
	CronIDAfter      string   `json:"cron_id_after"`
	Shipped          []string `json:"shipped"`
	Next             string   `json:"next,omitempty"`
	SubBeadsFiled    []string `json:"sub_beads_filed"`
	TestsDelta       string   `json:"tests_delta,omitempty"`
	RenderedTemplate string   `json:"rendered_template_path,omitempty"`
}

func runCronSelfAdjust(cmd *cobra.Command, _ []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}
	templatePath := cronSelfAdjustTemplate
	if !filepath.IsAbs(templatePath) {
		templatePath = filepath.Join(cwd, templatePath)
	}

	// Verify VERBATIM-PRESERVE markers before rendering. evolve.VerifyMarkers
	// returns a typed error listing each drifted marker; surface unchanged.
	if err := evolve.VerifyMarkers(templatePath); err != nil {
		return err
	}

	shipped := parseShippedCommits(cronSelfAdjustShipped)
	subBeads := splitCronCSV(cronSelfAdjustSubBeads)

	counter := countCronHistoryRows(filepath.Join(cwd, cronSelfAdjustHistoryRel)) + 1
	rendered, err := evolve.Render(templatePath, evolve.CronContext{
		ShippedCommits:         shipped,
		NextRecommendedBead:    cronSelfAdjustNext,
		SubBeadsFiledThisCycle: subBeads,
		TestsDelta:             cronSelfAdjustTestsDelta,
		CronSelfAdjustCounter:  counter,
	})
	if err != nil {
		return err
	}

	now := cronSelfAdjustClock()
	row := cronSelfAdjustHistoryRow{
		Timestamp:        now.Format(time.RFC3339),
		CronIDBefore:     "",
		CronIDAfter:      "",
		Shipped:          shippedAsStrings(shipped),
		Next:             cronSelfAdjustNext,
		SubBeadsFiled:    subBeads,
		TestsDelta:       cronSelfAdjustTestsDelta,
		RenderedTemplate: templatePath,
	}
	if err := appendCronHistoryRow(filepath.Join(cwd, cronSelfAdjustHistoryRel), row); err != nil {
		return err
	}

	spec := cronSelfAdjustSpec{
		NewCronPrompt: rendered,
		ScheduleHint:  cronSelfAdjustOn,
	}
	return writeCronSelfAdjustSpec(cmd.OutOrStdout(), spec)
}

// parseShippedCommits parses the comma-separated --shipped value into a slice
// of evolve.ShippedCommit. Each entry is "<sha>:<bead>[#<scenario>]".
func parseShippedCommits(in string) []evolve.ShippedCommit {
	if strings.TrimSpace(in) == "" {
		return nil
	}
	parts := strings.Split(in, ",")
	out := make([]evolve.ShippedCommit, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		sha, rest, ok := strings.Cut(p, ":")
		if !ok {
			// No colon — treat the whole token as the bead id.
			out = append(out, evolve.ShippedCommit{Bead: p})
			continue
		}
		bead, scenario, hasScen := strings.Cut(rest, "#")
		commit := evolve.ShippedCommit{Sha: sha, Bead: bead}
		if hasScen {
			commit.Scenario = scenario
		}
		out = append(out, commit)
	}
	return out
}

// shippedAsStrings rebuilds the canonical "<sha>:<bead>[#<scenario>]" form for
// logging.
func shippedAsStrings(in []evolve.ShippedCommit) []string {
	if len(in) == 0 {
		return []string{}
	}
	out := make([]string, 0, len(in))
	for _, c := range in {
		s := c.Sha + ":" + c.Bead
		if c.Scenario != "" {
			s += "#" + c.Scenario
		}
		out = append(out, s)
	}
	return out
}

// splitCronCSV is the local trim-aware splitter (the standard library's
// strings.Split keeps empty trailing tokens; we want a clean slice).
func splitCronCSV(in string) []string {
	if strings.TrimSpace(in) == "" {
		return []string{}
	}
	parts := strings.Split(in, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		out = append(out, p)
	}
	return out
}

// appendCronHistoryRow writes one JSONL row to path, creating the dir if
// needed.
func appendCronHistoryRow(path string, row cronSelfAdjustHistoryRow) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create history dir: %w", err)
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open history %s: %w", path, err)
	}
	defer f.Close()
	data, err := json.Marshal(row)
	if err != nil {
		return fmt.Errorf("marshal history row: %w", err)
	}
	if _, err := f.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("write history row: %w", err)
	}
	return nil
}

// writeCronSelfAdjustSpec emits the harness spec.
func writeCronSelfAdjustSpec(w io.Writer, spec cronSelfAdjustSpec) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(spec); err != nil {
		return fmt.Errorf("encode spec: %w", err)
	}
	return nil
}

// cronHistoryReadRows decodes path into typed rows. Missing file returns an
// empty slice. Exported for tests in this package only.
func cronHistoryReadRows(path string) ([]cronSelfAdjustHistoryRow, error) {
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	var rows []cronSelfAdjustHistoryRow
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var row cronSelfAdjustHistoryRow
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			return nil, fmt.Errorf("decode history row: %w", err)
		}
		rows = append(rows, row)
	}
	return rows, scanner.Err()
}
