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
}

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
}

func runCompile(cmd *cobra.Command, _ []string) error {
	cwd, err := resolveProjectDir()
	if err != nil {
		return err
	}

	mode, err := resolveCompileMode()
	if err != nil {
		return err
	}

	runtime := strings.TrimSpace(compileRuntime)
	if runtime == "" {
		runtime = strings.TrimSpace(os.Getenv("AGENTOPS_COMPILE_RUNTIME"))
	}

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
		if !compileQuiet {
			fmt.Fprintln(progress, "Compile wiki: writing compiled knowledge")
		}
		opts := compileScriptOptions{
			Sources:     compileSourcesDir,
			Output:      compileOutputDir,
			Runtime:     runtime,
			Incremental: compileIncremental && !compileForce,
			Force:       compileForce,
			LintOnly:    mode == "lint-only",
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
