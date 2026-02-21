package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/storage"
)

// agentsDirs are all .agents/ subdirectories ao init creates.
// Mirrors session-start.sh AGENTS_DIRS — keep in sync.
// Note: .agents/ao/{sessions,index,provenance} are created separately via storage.Init().
var agentsDirs = []string{
	".agents/research",
	".agents/products",
	".agents/retros",
	".agents/learnings",
	".agents/patterns",
	".agents/council",
	".agents/knowledge/pending",
	".agents/plans",
	".agents/rpi",
	".agents/ao",
}

var (
	initStealth      bool
	initHooks        bool
	initFull         bool
	initMinimalHooks bool
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize AgentOps in the current repository",
	Long: `Set up a repository for AgentOps: directories, gitignore, and optional hooks.

This creates:
  .agents/research/       - Research findings
  .agents/products/       - Product specs
  .agents/retros/         - Retrospectives
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
  .gitignore              - .agents/ entry appended (or --stealth for .git/info/exclude)
  .agents/.gitignore      - Belt-and-suspenders deny-all

Run in your project root. Safe to run multiple times (idempotent).`,
	RunE: runInit,
}

func init() {
	initCmd.Flags().BoolVar(&initStealth, "stealth", false, "Use .git/info/exclude instead of .gitignore")
	initCmd.Flags().BoolVar(&initHooks, "hooks", false, "Also register hooks (full 12-event coverage by default; equivalent to ao hooks install --full)")
	initCmd.Flags().BoolVar(&initFull, "full", false, "With --hooks, explicitly request full coverage (legacy explicit flag)")
	initCmd.Flags().BoolVar(&initMinimalHooks, "minimal-hooks", false, "With --hooks, install only SessionStart + Stop hooks")
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	isGitRepo := isGitRepository(cwd)

	// Phase 1: Create .agents/ subdirectories
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

	// Phase 1b: Create .agents/ao/ storage (sessions, index, provenance)
	baseDir := filepath.Join(cwd, storage.DefaultBaseDir)
	if dryRun {
		fmt.Println("[dry-run] Would create .agents/ao/{sessions,index,provenance}")
	} else {
		fs := storage.NewFileStorage(storage.WithBaseDir(baseDir))
		if err := fs.Init(); err != nil {
			return fmt.Errorf("initialize storage: %w", err)
		}
	}

	// Phase 2: Git protection
	if isGitRepo {
		if err := setupGitignore(cwd, dryRun, initStealth); err != nil {
			return fmt.Errorf("setup gitignore: %w", err)
		}
		warnTrackedFiles(cwd)
	} else {
		VerbosePrintf("Not a git repo — skipping .gitignore setup\n")
	}

	// Phase 2b: Nested .agents/.gitignore (belt-and-suspenders)
	if err := ensureNestedAgentsGitignore(cwd); err != nil {
		return err
	}

	// Phase 3: Hooks (optional)
	if initHooks {
		if err := installInitHooks(cmd); err != nil {
			return err
		}
	}

	// Summary
	if !dryRun {
		printInitSummary(cwd, isGitRepo)
	}

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
		content := "# Do not commit this directory — session artifacts, absolute paths, sensitive output.\n*\n!.gitignore\n!README.md\n"
		if err := os.WriteFile(nestedGitignore, []byte(content), 0644); err != nil {
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
			fmt.Println("  .gitignore (.agents/ entry)")
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
	fmt.Println("  ao forge transcript <path.jsonl>  - Extract knowledge from transcript")
}

// isGitRepository checks if cwd is inside a git repo.
func isGitRepository(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil
}

// setupGitignore adds .agents/ to .gitignore or .git/info/exclude.
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

	// Check if .agents/ already present
	if fileContainsLine(targetPath, ".agents/") {
		VerbosePrintf("%s already contains .agents/\n", label)
		return nil
	}

	if dryRun {
		fmt.Printf("[dry-run] Would add .agents/ to %s\n", label)
		return nil
	}

	// For stealth mode, ensure .git/info/ exists
	if stealth {
		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return err
		}
	}

	// Append or create
	f, err := os.OpenFile(targetPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	// Check if file is non-empty and doesn't end with newline
	info, _ := f.Stat()
	if info.Size() > 0 {
		// Read last byte to check for trailing newline
		rf, err := os.Open(targetPath)
		if err == nil {
			buf := make([]byte, 1)
			if _, err := rf.Seek(-1, 2); err != nil {
				rf.Close()
				return fmt.Errorf("seek %s: %w", targetPath, err)
			}
			if _, err := rf.Read(buf); err != nil {
				rf.Close()
				return fmt.Errorf("read last byte %s: %w", targetPath, err)
			}
			rf.Close()
			if buf[0] != '\n' {
				if _, err := f.WriteString("\n"); err != nil {
					return fmt.Errorf("write newline to %s: %w", targetPath, err)
				}
			}
		}
	}

	_, err = f.WriteString("\n# AgentOps session artifacts (auto-added by ao init)\n.agents/\n")
	return err
}

// fileContainsLine checks if a file contains a line matching the given text.
func fileContainsLine(path, text string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

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
