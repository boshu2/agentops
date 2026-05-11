package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/lifecycle"
	"github.com/boshu2/agentops/cli/internal/storage"
)

// agentsDirs are all .agents/ subdirectories ao init creates.
// Mirrors session-start.sh AGENTS_DIRS — keep in sync.
// Note: .agents/ao/{sessions,index,provenance} are created separately via storage.Init().
var agentsDirs = lifecycle.CoreAgentDirPaths()

var (
	initStealth      bool
	initHooks        bool
	initFull         bool
	initMinimalHooks bool
	initWithSchedule bool
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize AgentOps in the current repository",
	Long: `Set up a repository for AgentOps: directories, gitignore, and optional hooks.

This creates:
  .agents/research/       - Research findings
  .agents/packets/        - Domain/practice packets and task context inputs
  .agents/products/       - Product specs
  .agents/retro/          - Retrospectives
  .agents/learnings/      - Extracted learnings
  .agents/patterns/       - Reusable patterns
  .agents/council/        - Council verdicts
  .agents/knowledge/      - Knowledge artifacts
  .agents/plans/          - Implementation plans
  .agents/rpi/            - RPI orchestration state
  .agents/ao/sessions/    - Session files
  .agents/ao/index/       - Session index
  .agents/ao/provenance/  - Provenance graph

Git protection:
  .gitignore              - /.agents/ entry appended (or --stealth for .git/info/exclude)
  .agents/.gitignore      - Belt-and-suspenders deny-all

Run in your project root. Safe to run multiple times (idempotent).`,
	RunE: runInit,
}

func init() {
	initCmd.Flags().BoolVar(&initStealth, "stealth", false, "Use .git/info/exclude instead of .gitignore")
	initCmd.Flags().BoolVar(&initHooks, "hooks", false, "Also register hooks (full 12-event coverage by default; equivalent to ao hooks install --full)")
	initCmd.Flags().BoolVar(&initFull, "full", false, "With --hooks, explicitly request full coverage (legacy explicit flag)")
	initCmd.Flags().BoolVar(&initMinimalHooks, "minimal-hooks", false, "With --hooks, install SessionStart + SessionEnd + Stop hooks (lightweight)")
	initCmd.Flags().BoolVar(&initWithSchedule, "with-schedule", false,
		"Copy .agents/schedule.yaml.example to .agents/schedule.yaml (opt-in continuous-worker scheduling). "+
			"In a TTY, ao init prompts [Y/n] when this flag is unset. "+
			"In non-TTY runs, scheduling is silently skipped unless AGENTOPS_INIT_WITH_SCHEDULE=1 is set to opt-in.")
	initCmd.GroupID = "start"
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	isGitRepo := isGitRepository(cwd)

	if err := createAgentsDirs(cwd); err != nil {
		return err
	}

	if err := initStorage(cwd); err != nil {
		return err
	}

	if err := setupGitProtection(cwd, isGitRepo); err != nil {
		return err
	}

	if err := ensureNestedAgentsGitignore(cwd); err != nil {
		return err
	}

	if initHooks {
		if err := installInitHooks(cmd); err != nil {
			return err
		}
	}

	if err := maybeCopyScheduleExample(cmd, cwd); err != nil {
		return err
	}

	if !dryRun {
		printInitSummary(cwd, isGitRepo)
	}

	return nil
}

// shouldCopySchedule decides whether the schedule example should be copied
// based on the explicit flag, the AGENTOPS_INIT_WITH_SCHEDULE env var, and
// optional interactive prompting in a TTY.
//
// Returns (copy, prompted). prompted is true when the user was asked
// interactively (so callers can suppress duplicate logging).
func shouldCopySchedule(cmd *cobra.Command, flagSet bool) (bool, bool) {
	if flagSet {
		return true, false
	}
	if os.Getenv("AGENTOPS_INIT_WITH_SCHEDULE") == "1" {
		return true, false
	}
	// Detect `go test` binary by name suffix — skip prompt to avoid blocking
	// on stdin in test processes (where stdin may still be a TTY but tests
	// don't expect interactive prompts).
	if len(os.Args) > 0 && strings.HasSuffix(os.Args[0], ".test") {
		return false, false
	}
	if !isStdinTerminal() {
		// Silent skip in non-TTY runs unless the env var opts in.
		return false, false
	}
	in := cmd.InOrStdin()
	out := cmd.OutOrStdout()
	fmt.Fprint(out, "Enable continuous knowledge worker scheduling? [Y/n] ")
	reader := bufio.NewReader(in)
	response, _ := reader.ReadString('\n')
	response = strings.TrimSpace(strings.ToLower(response))
	if response == "" || response == "y" || response == "yes" {
		return true, true
	}
	return false, true
}

// maybeCopyScheduleExample copies .agents/schedule.yaml.example to
// .agents/schedule.yaml when the user has opted in. It honors --with-schedule,
// AGENTOPS_INIT_WITH_SCHEDULE=1, and the interactive TTY prompt. Pre-existing
// .agents/schedule.yaml is preserved (warn + skip).
func maybeCopyScheduleExample(cmd *cobra.Command, cwd string) error {
	copyIt, _ := shouldCopySchedule(cmd, initWithSchedule)
	if !copyIt {
		return nil
	}

	src := filepath.Join(cwd, ".agents", "schedule.yaml.example")
	dst := filepath.Join(cwd, ".agents", "schedule.yaml")

	if dryRun {
		fmt.Fprintf(cmd.OutOrStdout(), "[dry-run] Would copy %s to %s\n", src, dst)
		return nil
	}

	// Amendment B1: ensure .agents/ exists (fresh-repo case).
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("create .agents/ for schedule copy: %w", err)
	}

	if _, err := os.Stat(dst); err == nil {
		fmt.Fprintf(os.Stderr, "warning: %s already exists; skipping --with-schedule copy\n", dst)
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat %s: %w", dst, err)
	}

	srcContent, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("read %s: %w", src, err)
	}
	if err := os.WriteFile(dst, srcContent, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", dst, err)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Created %s\n", dst)
	return nil
}

// isStdinTerminal reports whether stdin is a TTY using stdlib only.
func isStdinTerminal() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// createAgentsDirs creates (or dry-run reports) all .agents/ subdirectories.
func createAgentsDirs(cwd string) error {
	for _, dir := range agentsDirs {
		target := filepath.Join(cwd, dir)
		if dryRun {
			if _, err := os.Stat(target); os.IsNotExist(err) {
				fmt.Printf("[dry-run] Would create %s\n", dir)
			}
			continue
		}
		if err := os.MkdirAll(target, 0700); err != nil {
			return fmt.Errorf("create directory %s: %w", dir, err)
		}
	}
	return nil
}

// initStorage initializes the .agents/ao/ storage subsystem.
func initStorage(cwd string) error {
	baseDir := filepath.Join(cwd, storage.DefaultBaseDir)
	if dryRun {
		fmt.Println("[dry-run] Would create .agents/ao/{sessions,index,provenance}")
		return nil
	}
	fs := storage.NewFileStorage(storage.WithBaseDir(baseDir))
	if err := fs.Init(); err != nil {
		return fmt.Errorf("initialize storage: %w", err)
	}
	return nil
}

// setupGitProtection configures gitignore and warns about tracked files.
func setupGitProtection(cwd string, isGitRepo bool) error {
	if !isGitRepo {
		VerbosePrintf("Not a git repo — skipping .gitignore setup\n")
		return nil
	}
	if err := setupGitignore(cwd, dryRun, initStealth); err != nil {
		return fmt.Errorf("setup gitignore: %w", err)
	}
	warnTrackedFiles(cwd)
	return nil
}

func ensureNestedAgentsGitignore(cwd string) error {
	nestedGitignore := filepath.Join(cwd, ".agents", ".gitignore")
	if dryRun {
		if _, err := os.Stat(nestedGitignore); os.IsNotExist(err) {
			fmt.Println("[dry-run] Would create .agents/.gitignore")
		}
		return nil
	}

	if _, err := os.Stat(nestedGitignore); os.IsNotExist(err) {
		content := "# Do not commit this directory — session artifacts, absolute paths, sensitive output.\n*\n!.gitignore\n"
		if err := os.WriteFile(nestedGitignore, []byte(content), 0600); err != nil {
			return fmt.Errorf("create .agents/.gitignore: %w", err)
		}
	}
	return nil
}

func installInitHooks(cmd *cobra.Command) error {
	if initFull && initMinimalHooks {
		return fmt.Errorf("--full and --minimal-hooks are mutually exclusive")
	}

	if dryRun {
		mode := "full"
		if initMinimalHooks {
			mode = "minimal"
		}
		fmt.Printf("[dry-run] Would install %s hooks\n", mode)
		return nil
	}

	// Delegate to existing hooks install logic.
	// Default to full coverage for `ao init --hooks`.
	hooksFull = true
	if initMinimalHooks {
		hooksFull = false
	}
	if initFull {
		hooksFull = true
	}
	hooksDryRun = false
	hooksForce = false
	if err := runHooksInstall(cmd, nil); err != nil {
		return fmt.Errorf("install hooks: %w", err)
	}
	return nil
}

func printInitSummary(cwd string, isGitRepo bool) {
	fmt.Printf("✓ Initialized AgentOps in %s\n", cwd)
	fmt.Println()
	fmt.Println("Created:")
	for _, dir := range agentsDirs {
		fmt.Printf("  %s/\n", dir)
	}
	fmt.Printf("  %s/{sessions,index,provenance}/\n", storage.DefaultBaseDir)
	if isGitRepo {
		if initStealth {
			fmt.Println("  .git/info/exclude (stealth)")
		} else {
			fmt.Println("  .gitignore (/.agents/ entry)")
		}
		fmt.Println("  .agents/.gitignore")
	}
	if initHooks {
		fmt.Println("  hooks registered")
	}
	fmt.Println()
	fmt.Println("Next steps:")
	if !initHooks {
		fmt.Println("  ao init --hooks        - Register session hooks")
	}
	if report, err := lifecycle.InspectRepoReadiness(cwd, lifecycle.ReadinessOptions{}); err == nil && !report.Ready {
		fmt.Println("  ao quick-start         - Finish core goals/instructions seed")
	}
	fmt.Println("  ao forge transcript <path.jsonl>  - Extract knowledge from transcript")
}

// isGitRepository checks if cwd is inside a git repo.
func isGitRepository(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil
}

// setupGitignore adds /.agents/ to .gitignore or .git/info/exclude.
func setupGitignore(cwd string, dryRun, stealth bool) error {
	var targetPath string
	var label string

	if stealth {
		targetPath = filepath.Join(cwd, ".git", "info", "exclude")
		label = ".git/info/exclude"
	} else {
		targetPath = filepath.Join(cwd, ".gitignore")
		label = ".gitignore"
	}

	// Check if the current repo-root .agents/ policy is already present.
	if fileContainsLine(targetPath, "/.agents/") {
		VerbosePrintf("%s already contains /.agents/\n", label)
		return nil
	}

	if dryRun {
		fmt.Printf("[dry-run] Would add /.agents/ to %s\n", label)
		return nil
	}

	// For stealth mode, ensure .git/info/ exists
	if stealth {
		if err := os.MkdirAll(filepath.Dir(targetPath), 0750); err != nil {
			return err
		}
	}

	// Append or create
	f, err := os.OpenFile(targetPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }() //nolint:errcheck // best-effort close

	// Check if file is non-empty and doesn't end with newline
	info, _ := f.Stat()
	if info.Size() > 0 {
		if err := appendNewlineIfMissing(f, targetPath); err != nil {
			return err
		}
	}

	_, err = f.WriteString("\n# AgentOps session artifacts (auto-added by ao init)\n/.agents/\n")
	return err
}

// appendNewlineIfMissing reads the last byte of targetPath and writes a newline
// to f if the file does not already end with one.
func appendNewlineIfMissing(f *os.File, targetPath string) error {
	rf, err := os.Open(targetPath)
	if err != nil {
		return nil // ignore open errors; non-critical
	}
	defer func() { _ = rf.Close() }()

	buf := make([]byte, 1)
	if _, err := rf.Seek(-1, 2); err != nil {
		return fmt.Errorf("seek %s: %w", targetPath, err)
	}
	if _, err := rf.Read(buf); err != nil {
		return fmt.Errorf("read last byte %s: %w", targetPath, err)
	}
	if buf[0] != '\n' {
		if _, err := f.WriteString("\n"); err != nil {
			return fmt.Errorf("write newline to %s: %w", targetPath, err)
		}
	}
	return nil
}

// fileContainsLine checks if a file contains a line matching the given text.
func fileContainsLine(path, text string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if strings.TrimSpace(scanner.Text()) == strings.TrimSpace(text) {
			return true
		}
	}
	return false
}

// warnTrackedFiles warns if .agents/ files are already tracked in git.
func warnTrackedFiles(cwd string) {
	cmd := exec.Command("git", "-C", cwd, "ls-files", ".agents/")
	out, err := cmd.Output()
	if err != nil {
		return
	}
	if len(strings.TrimSpace(string(out))) > 0 {
		fmt.Fprintln(os.Stderr, "Warning: .agents/ files are tracked in git. Run: git rm -r --cached .agents/")
	}
}
