package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/wiki"
)

// OpenKB-style structural health surface (age-port-openkb-into-agentops-go-5qw.4):
// `ao wiki lint` (default = structural health check; --fix for safe repairs),
// `ao wiki list`, `ao wiki status`. They operate on the active OpenKB workspace
// (or --path), resolved via wikiResolveWorkspace, and delegate to internal/wiki
// health.go / repair.go. Accretive: the prior `ao wiki lint` pipeline-stage
// behavior is preserved under `--pipeline-stage`, and `ao wiki doctor`
// (corpus/index diagnostics) is untouched.

// wikiHealthExitError carries the verdict-as-exit-code for `ao wiki lint`:
// exit 1 means blocking structural defects were found. The report already went
// to stdout and the command silences cobra's error print, so this maps straight
// to the process exit code (mirrors beadsExitError / gateExitError).
type wikiHealthExitError struct {
	code int
}

func (e *wikiHealthExitError) Error() string { return "" }
func (e *wikiHealthExitError) ExitCode() int { return e.code }

var wikiListCmd = &cobra.Command{
	Use:   "list",
	Short: "List the pages in the active wiki workspace (optionally by type)",
	Long: `Enumerate every compiled wiki page (summaries, concepts, entities, ...) plus
the index/log in the active workspace. Filter to one frontmatter type with
--type (e.g. --type concept). Use --json for machine-readable output.

The workspace is the one selected by ao wiki init/use, or --path.`,
	RunE:         runWikiList,
	SilenceUsage: true,
}

var wikiStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Summarize page counts and structural health of the active wiki",
	Long: `Report per-type page counts and a structural-health summary (defect totals by
kind) for the active wiki workspace. Use --json for machine-readable output.

The workspace is the one selected by ao wiki init/use, or --path. This is a
read-only summary; run ao wiki lint for the per-page defect detail.`,
	RunE:         runWikiStatus,
	SilenceUsage: true,
}

func init() {
	wikiCmd.AddCommand(wikiListCmd)
	wikiCmd.AddCommand(wikiStatusCmd)

	wikiListCmd.Flags().String("path", "", "Workspace path (default: the active workspace)")
	wikiListCmd.Flags().String("type", "", "Filter to one frontmatter type (e.g. concept, entity, summary)")
	wikiListCmd.Flags().Bool("json", false, "Emit JSON")

	wikiStatusCmd.Flags().String("path", "", "Workspace path (default: the active workspace)")
	wikiStatusCmd.Flags().Bool("json", false, "Emit JSON")

	// Extend the existing `ao wiki lint` (defined in wiki.go) with the
	// structural-health surface. The default behavior becomes the structural
	// check over the resolved workspace; the prior pipeline-LINT-stage behavior
	// is preserved under --pipeline-stage.
	wikiLintCmd.Flags().String("path", "", "Workspace path (default: the active workspace)")
	wikiLintCmd.Flags().Bool("json", false, "Emit JSON")
	wikiLintCmd.Flags().Bool("fix", false, "Apply safe deterministic repairs (strip dangling wikilinks); read-only without this flag")
	wikiLintCmd.Flags().Bool("pipeline-stage", false, "Run the legacy WikiPipeline LINT stage instead of the structural health check")
}

// resolveWikiHealthWorkspace resolves the workspace for a health subcommand:
// --path wins, else the active workspace recorded by init/use.
func resolveWikiHealthWorkspace(cmd *cobra.Command) (string, error) {
	pathFlag, _ := cmd.Flags().GetString("path")
	stateDir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("resolve working directory: %w", err)
	}
	return wikiResolveWorkspace(stateDir, pathFlag)
}

// runWikiStructuralLint runs the structural health check (and optional --fix)
// over the resolved workspace. Exit 1 (via wikiHealthExitError) when blocking
// defects remain. This is the default body of `ao wiki lint`.
func runWikiStructuralLint(cmd *cobra.Command, _ []string) error {
	ws, err := resolveWikiHealthWorkspace(cmd)
	if err != nil {
		return err
	}
	out := cmd.OutOrStdout()
	asJSON, _ := cmd.Flags().GetBool("json")
	doFix, _ := cmd.Flags().GetBool("fix")

	var fixResult *wiki.FixResult
	if doFix {
		fr, ferr := wiki.FixWiki(ws)
		if ferr != nil {
			return fmt.Errorf("wiki lint --fix: %w", ferr)
		}
		fixResult = &fr
	}

	report, err := wiki.CheckWiki(ws)
	if err != nil {
		return fmt.Errorf("wiki lint: %w", err)
	}

	if asJSON {
		payload := struct {
			Report wiki.WikiHealthReport `json:"report"`
			Fix    *wiki.FixResult       `json:"fix,omitempty"`
		}{Report: report, Fix: fixResult}
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		if err := enc.Encode(payload); err != nil {
			return fmt.Errorf("encode json: %w", err)
		}
	} else {
		printWikiHealthReport(cmd, report, fixResult)
	}

	if report.HasBlocking() {
		return &wikiHealthExitError{code: 1}
	}
	return nil
}

// printWikiHealthReport renders the human-readable lint report.
func printWikiHealthReport(cmd *cobra.Command, report wiki.WikiHealthReport, fix *wiki.FixResult) {
	out := cmd.OutOrStdout()
	if fix != nil {
		fmt.Fprintf(out, "wiki lint --fix: stripped %d dangling link(s) across %d page(s)\n",
			fix.LinksStripped, fix.PagesModified)
		for _, r := range fix.Repairs {
			fmt.Fprintf(out, "  fixed %s: removed [[%s]]\n", r.Page, r.Target)
		}
	}
	fmt.Fprintf(out, "wiki lint: %d page(s), %d defect(s) (%d blocking)\n",
		report.PageCount, len(report.Defects), report.BlockingCount())
	for _, d := range report.Defects {
		marker := "warn"
		if d.Blocking {
			marker = "FAIL"
		}
		fmt.Fprintf(out, "  [%s] %s\n", marker, d.Message)
	}
	if len(report.Defects) == 0 {
		fmt.Fprintln(out, "  clean — no structural defects")
	}
}

// runWikiList enumerates wiki pages, optionally filtered by type.
func runWikiList(cmd *cobra.Command, _ []string) error {
	ws, err := resolveWikiHealthWorkspace(cmd)
	if err != nil {
		return err
	}
	typeFilter, _ := cmd.Flags().GetString("type")
	pages, err := wiki.ListPages(ws, typeFilter)
	if err != nil {
		return fmt.Errorf("wiki list: %w", err)
	}
	out := cmd.OutOrStdout()
	if asJSON, _ := cmd.Flags().GetBool("json"); asJSON {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		if err := enc.Encode(pages); err != nil {
			return fmt.Errorf("encode json: %w", err)
		}
		return nil
	}
	if len(pages) == 0 {
		fmt.Fprintln(out, "wiki list: no pages")
		return nil
	}
	fmt.Fprintf(out, "wiki list: %d page(s)\n", len(pages))
	for _, p := range pages {
		t := p.Type
		if t == "" {
			t = "-"
		}
		fmt.Fprintf(out, "  %-10s %s\n", t, p.Key)
	}
	return nil
}

// runWikiStatus prints page counts and a health summary.
func runWikiStatus(cmd *cobra.Command, _ []string) error {
	ws, err := resolveWikiHealthWorkspace(cmd)
	if err != nil {
		return err
	}
	st, err := wiki.Status(ws)
	if err != nil {
		return fmt.Errorf("wiki status: %w", err)
	}
	out := cmd.OutOrStdout()
	if asJSON, _ := cmd.Flags().GetBool("json"); asJSON {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		if err := enc.Encode(st); err != nil {
			return fmt.Errorf("encode json: %w", err)
		}
		return nil
	}
	fmt.Fprintf(out, "wiki status: %s\n", st.Workspace)
	fmt.Fprintf(out, "  total pages : %d\n", st.TotalPages)
	for _, t := range sortedKeys(st.ByType) {
		fmt.Fprintf(out, "    %-10s %d\n", t, st.ByType[t])
	}
	fmt.Fprintf(out, "  defects     : %d (%d blocking)\n", st.DefectCount, st.BlockingDefects)
	for _, k := range sortedKeys(st.DefectsByKind) {
		fmt.Fprintf(out, "    %-20s %d\n", k, st.DefectsByKind[k])
	}
	return nil
}

// sortedKeys returns the keys of a map[string]int in sorted order.
func sortedKeys(m map[string]int) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
