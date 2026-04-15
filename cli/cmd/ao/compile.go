package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/boshu2/agentops/cli/embedded"
	"github.com/boshu2/agentops/cli/internal/config"
	"github.com/boshu2/agentops/cli/internal/lifecycle"
	minePkg "github.com/boshu2/agentops/cli/internal/mine"
	"github.com/spf13/cobra"
)

var (
	compileSourcesDir  string
	compileOutputDir   string
	compileSince       string
	compileRuntime     string
	compileIncremental bool
	compileForce       bool
	compileOnly        bool
	compileLintOnly    bool
	compileDefragOnly  bool
	compileMineOnly    bool
	compileFull        bool
	compileQuiet       bool
	compileBatchSize   int
	compileMaxBatches  int
	compileReset       bool
	compileRepair      bool
	compileRepairForce bool
)

var (
	runCompileScriptFn = runCompileScript
	runCompileMineFn   = runCompileMine
	runCompileDefragFn = runCompileDefrag
)

type compileReport struct {
	Mode        string               `json:"mode"`
	Sources     string               `json:"sources"`
	Output      string               `json:"output"`
	Runtime     string               `json:"runtime,omitempty"`
	Incremental bool                 `json:"incremental"`
	Force       bool                 `json:"force"`
	DryRun      bool                 `json:"dry_run"`
	Phases      []compilePhaseResult `json:"phases"`
}

type compilePhaseResult struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Detail string `json:"detail,omitempty"`
}

type compileScriptOptions struct {
	Sources     string
	Output      string
	Runtime     string
	Incremental bool
	Force       bool
	LintOnly    bool
	BatchSize   int
	MaxBatches  int
}

// lookPathFn is a seam for tests to stub PATH lookup.
var lookPathFn = exec.LookPath

var compileCmd = &cobra.Command{
	Use:   "compile",
	Short: "Compile .agents knowledge into an interlinked wiki",
	Long: `Compile makes the existing AgentOps knowledge compiler available through the ao CLI.

By default it runs the repo-local compiled knowledge cycle:
  mine signal from .agents/git/code
  compile changed .agents artifacts into .agents/compiled/
  lint the compiled wiki
  defrag stale or duplicate learnings

Headless compilation uses skills/compile/scripts/compile.sh. Set
AGENTOPS_COMPILE_RUNTIME or pass --runtime to choose an LLM backend
(ollama, claude, or openai).`,
	Args: cobra.NoArgs,
	RunE: runCompile,
}

func init() {
	compileCmd.GroupID = "knowledge"
	rootCmd.AddCommand(compileCmd)

	compileCmd.Flags().StringVar(&compileSourcesDir, "sources", ".agents", "Source .agents root to compile")
	compileCmd.Flags().StringVar(&compileOutputDir, "output-dir", ".agents/compiled", "Compiled wiki output directory")
	compileCmd.Flags().StringVar(&compileSince, "since", "26h", "Mine lookback window for full and mine-only modes")
	compileCmd.Flags().StringVar(&compileRuntime, "runtime", "", "LLM runtime override for headless compilation (ollama, claude, openai)")
	compileCmd.Flags().BoolVar(&compileIncremental, "incremental", true, "Compile only changed source artifacts")
	compileCmd.Flags().BoolVar(&compileForce, "force", false, "Recompile all source artifacts regardless of hashes")
	compileCmd.Flags().BoolVar(&compileOnly, "compile-only", false, "Skip mine and defrag; run compile plus lint")
	compileCmd.Flags().BoolVar(&compileLintOnly, "lint-only", false, "Only lint the existing compiled wiki")
	compileCmd.Flags().BoolVar(&compileDefragOnly, "defrag-only", false, "Only run mechanical defrag cleanup")
	compileCmd.Flags().BoolVar(&compileMineOnly, "mine-only", false, "Only mine new knowledge signal")
	compileCmd.Flags().BoolVar(&compileFull, "full", false, "Run the full mine, compile, lint, and defrag cycle")
	compileCmd.Flags().BoolVar(&compileQuiet, "quiet", false, "Suppress human progress output")
	compileCmd.Flags().IntVar(&compileBatchSize, "batch-size", 25, "Max changed files per LLM prompt (prevents single-giant-prompt on large corpora)")
	compileCmd.Flags().IntVar(&compileMaxBatches, "max-batches", 0, "Cap number of compile batches per invocation (0 = unlimited)")
	compileCmd.Flags().BoolVar(&compileReset, "reset", false, "Delete .agents/compiled/ and .hashes.json before compiling (force full rebuild)")
	compileCmd.Flags().BoolVar(&compileRepair, "repair", false, "Remove orphaned fallback stubs from .agents/compiled/ (files with no inbound wikilink traffic)")
	compileCmd.Flags().BoolVar(&compileRepairForce, "force-repair", false, "Actually delete orphans during --repair. Without --force-repair, --repair runs dry.")
}

func runCompile(cmd *cobra.Command, _ []string) error {
	cwd, err := resolveProjectDir()
	if err != nil {
		return err
	}

	// --reset and --repair run BEFORE mode resolution so they work
	// standalone (no LLM runtime needed) and compose with other flags
	// (e.g. --reset --full rebuilds from scratch).
	if compileReset {
		if err := resetCompileOutput(cwd, compileOutputDir, cmd.OutOrStdout()); err != nil {
			return fmt.Errorf("compile reset: %w", err)
		}
		if !compileFull && !compileOnly && !compileRepair {
			// standalone --reset is a complete action
			return nil
		}
	}
	if compileRepair {
		if err := repairCompileOutput(cwd, compileOutputDir, cmd.OutOrStdout()); err != nil {
			return fmt.Errorf("compile repair: %w", err)
		}
		if !compileFull && !compileOnly {
			return nil
		}
	}

	mode, err := resolveCompileMode()
	if err != nil {
		return err
	}

	runtime := resolveCompileRuntime(compileRuntime)

	report := compileReport{
		Mode:        mode,
		Sources:     compileSourcesDir,
		Output:      compileOutputDir,
		Runtime:     runtime,
		Incremental: compileIncremental && !compileForce,
		Force:       compileForce,
		DryRun:      GetDryRun(),
	}

	if GetDryRun() {
		report.Phases = plannedCompilePhases(mode)
		return printCompileReport(cmd.OutOrStdout(), report)
	}

	progress := cmd.OutOrStdout()
	if GetOutput() == "json" {
		progress = cmd.ErrOrStderr()
	}

	if shouldRunCompileMine(mode) {
		if !compileQuiet {
			fmt.Fprintln(progress, "Compile mine: extracting knowledge signal")
		}
		if err := runCompileMineFn(cwd, compileSince, compileQuiet); err != nil {
			return fmt.Errorf("compile mine: %w", err)
		}
		report.Phases = append(report.Phases, compilePhaseResult{Name: "mine", Status: "ok", Detail: "knowledge signal extracted"})
	}

	if shouldRunCompileScript(mode) {
		// Preflight the runtime before doing any expensive work. lint-only
		// does not call an LLM, so skip the runtime check there.
		if mode != "lint-only" {
			if err := preflightCompileRuntime(runtime); err != nil {
				return fmt.Errorf("compile wiki: %w", err)
			}
		}
		if !compileQuiet {
			fmt.Fprintln(progress, "Compile wiki: writing compiled knowledge")
			if runtime != "" && mode != "lint-only" {
				fmt.Fprintf(progress, "  runtime: %s\n", runtime)
			}
		}
		opts := compileScriptOptions{
			Sources:     compileSourcesDir,
			Output:      compileOutputDir,
			Runtime:     runtime,
			Incremental: compileIncremental && !compileForce,
			Force:       compileForce,
			LintOnly:    mode == "lint-only",
			BatchSize:   compileBatchSize,
			MaxBatches:  compileMaxBatches,
		}
		if err := runCompileScriptFn(cmd.Context(), cwd, opts, cmd.OutOrStdout(), cmd.ErrOrStderr()); err != nil {
			return fmt.Errorf("compile wiki: %w", err)
		}
		phase := "compile"
		detail := "compiled wiki and lint report updated"
		if mode == "lint-only" {
			phase = "lint"
			detail = "lint report updated"
		}
		report.Phases = append(report.Phases, compilePhaseResult{Name: phase, Status: "ok", Detail: detail})
	}

	if shouldRunCompileDefrag(mode) {
		if !compileQuiet {
			fmt.Fprintln(progress, "Compile defrag: cleaning knowledge store")
		}
		if err := runCompileDefragFn(cwd, GetDryRun()); err != nil {
			return fmt.Errorf("compile defrag: %w", err)
		}
		report.Phases = append(report.Phases, compilePhaseResult{Name: "defrag", Status: "ok", Detail: "mechanical cleanup completed"})
	}

	return printCompileReport(cmd.OutOrStdout(), report)
}

func resolveCompileMode() (string, error) {
	modes := []struct {
		name string
		set  bool
	}{
		{"full", compileFull},
		{"compile-only", compileOnly},
		{"lint-only", compileLintOnly},
		{"defrag-only", compileDefragOnly},
		{"mine-only", compileMineOnly},
	}

	selected := ""
	for _, mode := range modes {
		if !mode.set {
			continue
		}
		if selected != "" {
			return "", fmt.Errorf("choose only one compile mode flag")
		}
		selected = mode.name
	}
	if selected == "" {
		selected = "full"
	}
	if compileForce && !compileIncremental {
		return "", fmt.Errorf("--force and --incremental=false both request full recompilation; use only --force")
	}
	return selected, nil
}

func shouldRunCompileMine(mode string) bool {
	return mode == "full" || mode == "mine-only"
}

func shouldRunCompileScript(mode string) bool {
	return mode == "full" || mode == "compile-only" || mode == "lint-only"
}

func shouldRunCompileDefrag(mode string) bool {
	return mode == "full" || mode == "defrag-only"
}

func plannedCompilePhases(mode string) []compilePhaseResult {
	var phases []compilePhaseResult
	if shouldRunCompileMine(mode) {
		phases = append(phases, compilePhaseResult{Name: "mine", Status: "planned"})
	}
	if shouldRunCompileScript(mode) {
		name := "compile"
		if mode == "lint-only" {
			name = "lint"
		}
		phases = append(phases, compilePhaseResult{Name: name, Status: "planned"})
	}
	if shouldRunCompileDefrag(mode) {
		phases = append(phases, compilePhaseResult{Name: "defrag", Status: "planned"})
	}
	return phases
}

func runCompileMine(cwd, since string, quiet bool) error {
	window, err := minePkg.ParseWindow(since)
	if err != nil {
		return fmt.Errorf("parse --since: %w", err)
	}
	_, err = minePkg.Run(cwd, minePkg.RunOpts{
		Sources:      []string{"git", "agents", "code"},
		Window:       window,
		OutputDir:    filepath.Join(".agents", "mine"),
		Quiet:        quiet,
		MineEventsFn: mineEvents,
	})
	return err
}

func runCompileDefrag(cwd string, dryRun bool) error {
	if _, err := lifecycle.ExecutePrune(cwd, dryRun, 30); err != nil {
		return err
	}
	if _, err := lifecycle.ExecuteDedup(cwd, dryRun); err != nil {
		return err
	}
	if _, err := lifecycle.SweepOscillatingGoals(cwd); err != nil {
		return err
	}
	return nil
}

func runCompileScript(ctx context.Context, cwd string, opts compileScriptOptions, stdout, stderr io.Writer) error {
	scriptPath, cleanup, err := materializeCompileScript(cwd)
	if err != nil {
		return err
	}
	defer cleanup()

	args := []string{
		scriptPath,
		"--sources", opts.Sources,
		"--output", opts.Output,
	}
	if opts.LintOnly {
		args = append(args, "--lint-only")
	} else if opts.Force || !opts.Incremental {
		args = append(args, "--force")
	} else {
		args = append(args, "--incremental")
	}
	if opts.BatchSize > 0 {
		args = append(args, "--batch-size", fmt.Sprintf("%d", opts.BatchSize))
	}
	if opts.MaxBatches > 0 {
		args = append(args, "--max-batches", fmt.Sprintf("%d", opts.MaxBatches))
	}

	execCmd := exec.CommandContext(ctx, "bash", args...)
	execCmd.Dir = cwd
	execCmd.Stdout = stdout
	execCmd.Stderr = stderr
	execCmd.Stdin = os.Stdin
	execCmd.Env = os.Environ()
	if opts.Runtime != "" {
		execCmd.Env = append(execCmd.Env, "AGENTOPS_COMPILE_RUNTIME="+opts.Runtime)
	}

	if err := execCmd.Run(); err != nil {
		return fmt.Errorf("run %s: %w", filepath.Base(scriptPath), err)
	}
	return nil
}

func materializeCompileScript(cwd string) (string, func(), error) {
	data, err := loadCompileScript(cwd)
	if err != nil {
		return "", func() {}, err
	}
	data = normalizeShellScript(data)

	tmp, err := os.CreateTemp("", "ao-compile-*.sh")
	if err != nil {
		return "", func() {}, fmt.Errorf("create temp compile script: %w", err)
	}
	path := tmp.Name()
	cleanup := func() { _ = os.Remove(path) }

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		cleanup()
		return "", func() {}, fmt.Errorf("write temp compile script: %w", err)
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return "", func() {}, fmt.Errorf("close temp compile script: %w", err)
	}
	if err := os.Chmod(path, 0o700); err != nil {
		cleanup()
		return "", func() {}, fmt.Errorf("chmod temp compile script: %w", err)
	}

	return path, cleanup, nil
}

func loadCompileScript(cwd string) ([]byte, error) {
	local := filepath.Join(cwd, "skills", "compile", "scripts", "compile.sh")
	if data, err := os.ReadFile(local); err == nil {
		return data, nil
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("read local compile script: %w", err)
	}

	data, err := fs.ReadFile(embedded.HooksFS, "skills/compile/scripts/compile.sh")
	if err != nil {
		return nil, fmt.Errorf("read embedded compile script: %w", err)
	}
	return data, nil
}

func normalizeShellScript(data []byte) []byte {
	return []byte(strings.ReplaceAll(string(data), "\r\n", "\n"))
}

// resetCompileOutput removes the compiled wiki directory and the incremental
// hash file so the next compile run starts from scratch. Safe to call when
// the dir does not exist.
func resetCompileOutput(cwd, outputDir string, stdout io.Writer) error {
	target := outputDir
	if !filepath.IsAbs(target) {
		target = filepath.Join(cwd, target)
	}
	if GetDryRun() {
		fmt.Fprintf(stdout, "[dry-run] would rm -rf %s\n", target)
		return nil
	}
	removed := 0
	info, err := os.Stat(target)
	if err == nil && info.IsDir() {
		entries, _ := os.ReadDir(target)
		removed = len(entries)
		if err := os.RemoveAll(target); err != nil {
			return fmt.Errorf("remove %s: %w", target, err)
		}
	} else if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("stat %s: %w", target, err)
	}
	if !compileQuiet {
		fmt.Fprintf(stdout, "Compile reset: removed %s (%d entries)\n", target, removed)
	}
	return nil
}

// repairCompileOutput removes orphaned fallback stubs from the compiled
// wiki. An orphan is an article file with zero inbound [[wikilinks]] from
// any other article AND no matching source .md file in .agents/*/. Today's
// fallback stubs (index.md, log.md, lint-report.md from a failed run) are
// always preserved — removing those would delete the user's history.
// Returns the number of files removed.
func repairCompileOutput(cwd, outputDir string, stdout io.Writer) error {
	target := outputDir
	if !filepath.IsAbs(target) {
		target = filepath.Join(cwd, target)
	}
	entries, err := os.ReadDir(target)
	if err != nil {
		if os.IsNotExist(err) {
			if !compileQuiet {
				fmt.Fprintf(stdout, "Compile repair: %s does not exist, nothing to do\n", target)
			}
			return nil
		}
		return fmt.Errorf("read %s: %w", target, err)
	}

	// Collect candidate articles (skip infrastructure files we never remove).
	preserved := map[string]bool{
		"index.md":        true,
		"log.md":          true,
		"lint-report.md":  true,
		".hashes.json":    true,
	}
	type article struct {
		name     string
		slug     string
		content  string
		fullPath string
	}
	var articles []article
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		if preserved[e.Name()] {
			continue
		}
		full := filepath.Join(target, e.Name())
		data, err := os.ReadFile(full)
		if err != nil {
			continue
		}
		articles = append(articles, article{
			name:     e.Name(),
			slug:     strings.TrimSuffix(e.Name(), ".md"),
			content:  string(data),
			fullPath: full,
		})
	}

	// Build the set of all inbound [[wikilinks]] across the compiled set.
	inboundCount := make(map[string]int)
	for _, a := range articles {
		for _, b := range articles {
			if a.slug == b.slug {
				continue
			}
			if strings.Contains(b.content, "[["+a.slug+"]]") {
				inboundCount[a.slug]++
			}
		}
	}

	removed := 0
	// Safety: --repair defaults to dry-run. Actual deletion requires
	// --force-repair. The global --dry-run flag always wins (even if
	// --force-repair is also passed). This prevents a regex bug or stray
	// [[...]] pattern in prose from silently nuking user wiki content.
	globalDryRun := GetDryRun()
	dryRun := globalDryRun || !compileRepairForce
	for _, a := range articles {
		if inboundCount[a.slug] > 0 {
			continue
		}
		if dryRun {
			fmt.Fprintf(stdout, "[dry-run] would remove orphan: %s\n", a.fullPath)
			removed++
			continue
		}
		if err := os.Remove(a.fullPath); err != nil {
			return fmt.Errorf("remove orphan %s: %w", a.name, err)
		}
		removed++
	}

	if dryRun && removed > 0 && !globalDryRun {
		fmt.Fprintln(stdout, "Use --force-repair to actually delete.")
	}

	if !compileQuiet {
		if dryRun {
			fmt.Fprintf(stdout, "Compile repair: scanned %d article(s), would remove %d orphan(s)\n", len(articles), removed)
		} else {
			fmt.Fprintf(stdout, "Compile repair: scanned %d article(s), removed %d orphan(s)\n", len(articles), removed)
		}
	}
	return nil
}

// loadCompileConfigFn is a seam for tests to stub config loading.
var loadCompileConfigFn = func() (string, error) {
	cfg, err := config.Load(nil)
	if err != nil || cfg == nil {
		return "", err
	}
	return cfg.Compile.PreferredRuntime, nil
}

// resolveCompileRuntime picks the LLM runtime for headless compilation in this
// order of precedence:
//  1. explicit --runtime flag
//  2. AGENTOPS_COMPILE_RUNTIME env var
//  3. compile.preferred_runtime in ~/.agentops/config.yaml or
//     .agents/config.yaml (so privacy-preferring users can force Ollama
//     even when `claude` is installed)
//  4. auto-detect: if 'claude' binary is on PATH, use claude-cli
//  5. empty (preflight will fail with an actionable error)
func resolveCompileRuntime(flagValue string) string {
	if v := strings.TrimSpace(flagValue); v != "" {
		return v
	}
	if v := strings.TrimSpace(os.Getenv("AGENTOPS_COMPILE_RUNTIME")); v != "" {
		return v
	}
	if v, _ := loadCompileConfigFn(); strings.TrimSpace(v) != "" {
		return strings.TrimSpace(v)
	}
	// Auto-detect local Claude Code CLI as a zero-config backend.
	if _, err := lookPathFn("claude"); err == nil {
		return "claude-cli"
	}
	return ""
}

// preflightCompileRuntime verifies the selected runtime has the credentials or
// binary it needs, and returns an actionable error if not. Kept in Go rather
// than only in compile.sh so ao can fail fast before materializing the temp
// script.
func preflightCompileRuntime(runtime string) error {
	switch runtime {
	case "":
		return fmt.Errorf(`no LLM runtime configured for headless compile.

Pick one:
  export AGENTOPS_COMPILE_RUNTIME=claude-cli   # uses local 'claude' binary, no API key needed
  export AGENTOPS_COMPILE_RUNTIME=ollama       # needs OLLAMA_HOST (default http://localhost:11434)
  export AGENTOPS_COMPILE_RUNTIME=claude       # needs ANTHROPIC_API_KEY
  export AGENTOPS_COMPILE_RUNTIME=openai       # needs OPENAI_API_KEY

Or pass --runtime=<name> on this command.
Or invoke /compile interactively inside a Claude Code session.`)
	case "claude-cli":
		if _, err := lookPathFn("claude"); err != nil {
			return fmt.Errorf("runtime=claude-cli but 'claude' binary is not on PATH; install Claude Code or switch: --runtime=ollama")
		}
	case "claude":
		if strings.TrimSpace(os.Getenv("ANTHROPIC_API_KEY")) == "" {
			return fmt.Errorf("runtime=claude but ANTHROPIC_API_KEY is not set; export it or switch: --runtime=claude-cli (uses local claude binary, no key needed)")
		}
	case "openai":
		if strings.TrimSpace(os.Getenv("OPENAI_API_KEY")) == "" {
			return fmt.Errorf("runtime=openai but OPENAI_API_KEY is not set")
		}
	case "ollama":
		// curl/ollama connectivity is best-effort; compile.sh will warn.
	default:
		return fmt.Errorf("unknown runtime %q; expected one of: ollama, claude, claude-cli, openai", runtime)
	}
	return nil
}

func printCompileReport(w io.Writer, report compileReport) error {
	if GetOutput() == "json" {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(report)
	}

	if report.DryRun {
		fmt.Fprintln(w, "[dry-run] ao compile")
	}
	fmt.Fprintf(w, "Compile %s complete.\n", report.Mode)
	fmt.Fprintf(w, "  sources: %s\n", report.Sources)
	fmt.Fprintf(w, "  output:  %s\n", report.Output)
	if report.Runtime != "" {
		fmt.Fprintf(w, "  runtime: %s\n", report.Runtime)
	}
	fmt.Fprintf(w, "  incremental: %t\n", report.Incremental)
	for _, phase := range report.Phases {
		fmt.Fprintf(w, "  %s: %s", phase.Name, phase.Status)
		if phase.Detail != "" {
			fmt.Fprintf(w, " (%s)", phase.Detail)
		}
		fmt.Fprintln(w)
	}
	if len(report.Phases) == 0 {
		fmt.Fprintln(w, "  phases: none")
	}
	fmt.Fprintf(w, "  finished: %s\n", time.Now().UTC().Format(time.RFC3339))
	return nil
}
