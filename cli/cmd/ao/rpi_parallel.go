package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"

	cliRPI "github.com/boshu2/agentops/cli/internal/rpi"
)

var (
	parallelManifest     string
	parallelMergeOrder   string
	parallelAutoMerge    bool
	parallelNoMerge      bool
	parallelGateScript   string
	parallelRuntimeCmd   string
	parallelPhaseTimeout time.Duration
	parallelTmux         bool
)

// parallelEpic describes one epic to run in parallel.
type parallelEpic struct {
	Name       string `json:"name" yaml:"name"`
	Goal       string `json:"goal" yaml:"goal"`
	MergeOrder int    `json:"merge_order" yaml:"merge_order"`
}

// parallelResult captures the outcome of one epic's execution.
type parallelResult struct {
	Epic      parallelEpic
	Worktree  string
	Branch    string
	Success   bool
	Error     error
	Duration  time.Duration
	CommitSHA string
	LogFile   string
}

// parallelManifestFile is the top-level manifest structure.
type parallelManifestFile struct {
	Epics []parallelEpic `json:"epics" yaml:"epics"`
}

// worktreeInfo holds the path and branch for a parallel worktree.
type worktreeInfo struct {
	path   string
	branch string
}

func init() {
	parallelCmd := &cobra.Command{
		Use:   "parallel [goals...]",
		Short: "Run legacy RPI epics in parallel worktrees",
		Long: `Run multiple RPI epics concurrently, each in an isolated git worktree.

Compatibility-only legacy command. Do not use this as the production
agentopsd control-plane path. Production parallel coding work must go through
daemon-managed worker slots, routing policy, validation gates, and manual merge
review.

Each epic gets:
  - Its own git worktree (branch: epic/<name>)
  - A fresh ao rpi phased session
  - Isolated file changes (no conflicts during execution)

By default this command preserves worker branches and worktrees for manual
review. Passing --auto-merge opts into the older behavior: successful branches
are merged back to the base branch in the specified order, then an optional
validation gate runs after all merges. Failed worktrees are retained.

Input: either a manifest file (--manifest) or inline goals as arguments.

Examples:
  # Inline goals (auto-named epic-1, epic-2, ...)
  ao rpi parallel "add evolve watchdog" "add CLI dashboard" "consolidate skills"

  # Manifest file with named epics and merge order
  ao rpi parallel --manifest epics.json

  # Skip auto-merge (leave worktrees for manual review)
  ao rpi parallel "goal 1" "goal 2"

  # Compatibility mode: explicitly opt in to legacy auto-merge behavior
  ao rpi parallel --auto-merge --manifest epics.json

Manifest format (JSON):
  {
    "epics": [
      {"name": "evolve", "goal": "Add evolve watchdog...", "merge_order": 1},
      {"name": "cli-obs", "goal": "Add CLI dashboard...", "merge_order": 2}
    ]
  }`,
		Args: cobra.ArbitraryArgs,
		RunE: runRPIParallel,
	}

	parallelCmd.Flags().StringVar(&parallelManifest, "manifest", "", "Path to epic manifest file (JSON)")
	parallelCmd.Flags().StringVar(&parallelMergeOrder, "merge-order", "", "Comma-separated epic names for merge order (default: manifest order or arg order)")
	parallelCmd.Flags().BoolVar(&parallelAutoMerge, "auto-merge", false, "Opt in to legacy merge/cleanup after successful epics")
	parallelCmd.Flags().BoolVar(&parallelNoMerge, "no-merge", false, "Compatibility alias for the default preserve-for-review behavior")
	parallelCmd.Flags().StringVar(&parallelGateScript, "gate-script", "", "Validation script to run after all merges (e.g., scripts/ci-local-release.sh)")
	parallelCmd.Flags().StringVar(&parallelRuntimeCmd, "runtime-cmd", "", "Runtime command for phased sessions (default: claude)")
	parallelCmd.Flags().DurationVar(&parallelPhaseTimeout, "phase-timeout", 90*time.Minute, "Timeout per epic (kills subprocess if exceeded)")
	parallelCmd.Flags().BoolVar(&parallelTmux, "tmux", false, "Spawn epics in tmux windows for interactive visibility")

	rpiCmd.AddCommand(parallelCmd)
}

func runRPIParallel(cmd *cobra.Command, args []string) error {
	shouldMerge, err := parallelShouldAutoMerge()
	if err != nil {
		return err
	}
	epics, baseCwd, runtimeCmd, err := validateParallelPrereqs(args)
	if err != nil {
		return err
	}

	if GetDryRun() {
		fmt.Printf("DRY RUN: would run %d epics in parallel worktrees\n", len(epics))
		if shouldMerge {
			fmt.Println("DRY RUN: --auto-merge requested; successful branches would be merged after completion")
		} else {
			fmt.Println("DRY RUN: worktrees would be preserved for manual review")
		}
		for i, e := range epics {
			fmt.Printf("  [%d] %s: %s\n", i+1, e.Name, truncateGoal(e.Goal, 80))
		}
		return nil
	}

	fmt.Printf("ao rpi parallel: %d epics\n", len(epics))
	for i, e := range epics {
		fmt.Printf("  [%d] %s: %s\n", i+1, e.Name, truncateGoal(e.Goal, 80))
	}
	fmt.Println()

	worktrees, err := createParallelWorktrees(baseCwd, epics, runtimeCmd)
	if err != nil {
		return err
	}

	logDir := filepath.Join(baseCwd, ".agents", "rpi", "parallel")
	_ = os.MkdirAll(logDir, 0o750)

	const tmuxSession = "rpi-parallel"
	tmuxCmd, err := setupTmuxSession(tmuxSession)
	if err != nil {
		return err
	}

	results := spawnParallelEpics(epics, worktrees, runtimeCmd, logDir, tmuxSession, tmuxCmd, parallelPhaseTimeout)

	allSuccess := reportParallelResults(results, epics, parallelTmux, tmuxSession, tmuxCmd)

	if !shouldMerge {
		fmt.Println("Manual merge required: worktrees left in place for review:")
		for i, wt := range worktrees {
			fmt.Printf("  %s: %s (branch: %s)\n", epics[i].Name, wt.path, wt.branch)
		}
		return nil
	}

	if !allSuccess {
		fmt.Println("Some epics failed. Merging only successful ones.")
	}

	mergedCount, err := mergeParallelWorktrees(epics, results, worktrees)
	if err != nil {
		return err
	}

	cleanupParallelWorktrees(worktrees, results)

	if err := runParallelGateScript(baseCwd, mergedCount); err != nil {
		return err
	}

	fmt.Printf("\nao rpi parallel: %d/%d epics merged successfully\n", mergedCount, len(epics))
	return nil
}

// cleanupParallelWorktrees removes worktrees and deletes branches for successful epics.
func cleanupParallelWorktrees(worktrees []worktreeInfo, results []parallelResult) {
	for i, wt := range worktrees {
		if i >= len(results) || !results[i].Success {
			fmt.Printf("PRESERVE failed worktree: %s (branch: %s)\n", wt.path, wt.branch)
			continue
		}
		_ = exec.Command("git", "worktree", "remove", "--force", wt.path).Run()
		_ = exec.Command("git", "branch", "-d", wt.branch).Run()
	}
}

// runParallelGateScript runs the optional gate validation script after merges.
func runParallelGateScript(baseCwd string, mergedCount int) error {
	if parallelGateScript == "" || mergedCount == 0 {
		return nil
	}
	fmt.Printf("Running gate: %s\n", parallelGateScript)
	gateCmd := exec.Command("bash", parallelGateScript)
	gateCmd.Dir = baseCwd
	gateCmd.Stdout = os.Stdout
	gateCmd.Stderr = os.Stderr
	if err := gateCmd.Run(); err != nil {
		return fmt.Errorf("gate script failed: %w", err)
	}
	fmt.Println("Gate: PASS")
	return nil
}

// validateParallelPrereqs resolves epics, validates git repo, and resolves the runtime command.
func validateParallelPrereqs(args []string) ([]parallelEpic, string, string, error) {
	epics, err := resolveParallelEpics(args)
	if err != nil {
		return nil, "", "", err
	}
	if len(epics) == 0 {
		return nil, "", "", fmt.Errorf("no epics to run (provide goals as arguments or use --manifest)")
	}

	baseCwd, err := os.Getwd()
	if err != nil {
		return nil, "", "", fmt.Errorf("get working directory: %w", err)
	}

	if _, err := exec.Command("git", "rev-parse", "--is-inside-work-tree").Output(); err != nil {
		return nil, "", "", fmt.Errorf("not a git repository (required for worktree isolation)")
	}

	runtimeCmd := "claude"
	if parallelRuntimeCmd != "" {
		runtimeCmd = parallelRuntimeCmd
	}
	if _, err := exec.LookPath(runtimeCmd); err != nil {
		return nil, "", "", fmt.Errorf("runtime command %q not found on PATH", runtimeCmd)
	}

	return epics, baseCwd, runtimeCmd, nil
}

func parallelShouldAutoMerge() (bool, error) {
	if parallelAutoMerge && parallelNoMerge {
		return false, fmt.Errorf("--auto-merge and --no-merge cannot be used together")
	}
	return parallelAutoMerge && !parallelNoMerge, nil
}

func parallelWorktreeRoot(baseCwd, runtimeCmd string) string {
	return cliRPI.ParallelWorktreeRoot(baseCwd, runtimeCmd)
}

// createParallelWorktrees creates git worktrees for each epic.
// On failure, cleans up previously created worktrees.
func createParallelWorktrees(baseCwd string, epics []parallelEpic, runtimeCmd string) ([]worktreeInfo, error) {
	worktrees := make([]worktreeInfo, len(epics))
	worktreeRoot := parallelWorktreeRoot(baseCwd, runtimeCmd)
	for i, e := range epics {
		branch := "epic/" + e.Name
		wtPath := filepath.Join(worktreeRoot, e.Name)

		if exists, err := pathExists(wtPath); err != nil {
			return nil, fmt.Errorf("inspect worktree path for %s: %w", e.Name, err)
		} else if exists {
			return nil, fmt.Errorf("refusing to remove existing worktree path for %s: %s", e.Name, wtPath)
		}
		if branchExists(branch) {
			return nil, fmt.Errorf("refusing to delete existing branch for %s: %s", e.Name, branch)
		}

		out, err := exec.Command("git", "worktree", "add", "-b", branch, wtPath).CombinedOutput()
		if err != nil {
			// Clean up any worktrees we already created.
			for j := 0; j < i; j++ {
				_ = exec.Command("git", "worktree", "remove", "--force", worktrees[j].path).Run()
				_ = exec.Command("git", "branch", "-D", worktrees[j].branch).Run()
			}
			return nil, fmt.Errorf("create worktree for %s: %s: %w", e.Name, string(out), err)
		}
		worktrees[i] = worktreeInfo{path: wtPath, branch: branch}
		fmt.Printf("  worktree: %s → %s\n", e.Name, branch)
	}
	fmt.Println()
	return worktrees, nil
}

func pathExists(path string) (bool, error) {
	if _, err := os.Stat(path); err == nil {
		return true, nil
	} else if os.IsNotExist(err) {
		return false, nil
	} else {
		return false, err
	}
}

func branchExists(branch string) bool {
	return exec.Command("git", "show-ref", "--verify", "--quiet", "refs/heads/"+branch).Run() == nil
}

// setupTmuxSession sets up a tmux session for parallel epic visibility.
// Returns the tmux command path (empty if tmux not requested) and any error.
func setupTmuxSession(tmuxSession string) (string, error) {
	if !parallelTmux {
		return "", nil
	}
	tmuxCmd := effectiveTmuxCommand("")
	if _, err := exec.LookPath(tmuxCmd); err != nil {
		return "", fmt.Errorf("tmux not found on PATH (required for --tmux)")
	}
	// Kill stale session.
	_ = exec.Command(tmuxCmd, "kill-session", "-t", tmuxSession).Run()
	// Create session with a temporary setup window.
	if out, err := exec.Command(tmuxCmd, "new-session", "-d", "-s", tmuxSession, "-n", "_setup").CombinedOutput(); err != nil {
		return "", fmt.Errorf("create tmux session: %s: %w", string(out), err)
	}
	// Keep panes visible after command exits.
	_ = exec.Command(tmuxCmd, "set-option", "-t", tmuxSession, "remain-on-exit", "on").Run()
	fmt.Printf("Attach: tmux attach -t %s\n\n", tmuxSession)
	return tmuxCmd, nil
}

// spawnParallelEpics dispatches all epics concurrently via goroutines, using tmux or direct mode.
// Returns a results slice indexed by epic position.
func spawnParallelEpics(epics []parallelEpic, worktrees []worktreeInfo, runtimeCmd, logDir, tmuxSession, tmuxCmd string, timeout time.Duration) []parallelResult {
	results := make([]parallelResult, len(epics))
	var wg sync.WaitGroup

	for i, e := range epics {
		wg.Add(1)
		go func(idx int, epic parallelEpic, wt worktreeInfo) {
			defer wg.Done()
			start := time.Now()

			logFile := filepath.Join(logDir, epic.Name+".log")
			var result parallelResult
			if parallelTmux {
				result = runParallelEpicTmux(epic, wt.path, wt.branch, runtimeCmd, logFile, logDir, tmuxSession, tmuxCmd, timeout)
			} else {
				result = runParallelEpic(epic, wt.path, wt.branch, runtimeCmd, logFile, timeout)
			}
			result.Duration = time.Since(start)
			result.LogFile = logFile
			results[idx] = result

			status := "DONE"
			if !result.Success {
				status = "FAIL"
			}
			fmt.Printf("  [%s] %s (%s) — %s\n", status, epic.Name, result.Duration.Round(time.Second), wt.branch)
		}(i, e, worktrees[i])
	}

	fmt.Println("Running epics...")
	wg.Wait()
	fmt.Println()

	// Clean up tmux setup window.
	if parallelTmux && tmuxCmd != "" {
		_ = exec.Command(tmuxCmd, "kill-window", "-t", tmuxSession+":_setup").Run()
	}

	return results
}

// reportParallelResults prints success/fail for each epic and handles tmux session message.
// Returns true if all epics succeeded.
func reportParallelResults(results []parallelResult, epics []parallelEpic, tmux bool, tmuxSession, tmuxCmd string) bool {
	allSuccess := true
	for _, r := range results {
		if !r.Success {
			allSuccess = false
			fmt.Printf("FAIL: %s — %v (log: %s)\n", r.Epic.Name, r.Error, r.LogFile)
		} else {
			fmt.Printf("DONE: %s — %s (%s)\n", r.Epic.Name, r.CommitSHA, r.Duration.Round(time.Second))
		}
	}
	fmt.Println()

	if tmux {
		fmt.Printf("Session %s still alive. Kill with: tmux kill-session -t %s\n\n", tmuxSession, tmuxSession)
	}

	return allSuccess
}

// mergeParallelWorktrees resolves merge order and merges successful epic branches.
// Returns merged count and any merge conflict error.
func mergeParallelWorktrees(epics []parallelEpic, results []parallelResult, worktrees []worktreeInfo) (int, error) {
	mergeIndices := resolveMergeOrder(epics, results)

	mergedCount := 0
	for _, idx := range mergeIndices {
		if !results[idx].Success {
			fmt.Printf("SKIP merge: %s (failed)\n", epics[idx].Name)
			continue
		}

		branch := worktrees[idx].branch
		msg := fmt.Sprintf("feat(%s): %s", epics[idx].Name, truncateGoal(epics[idx].Goal, 60))

		out, err := exec.Command("git", "merge", branch, "--no-ff", "-m", msg).CombinedOutput()
		if err != nil {
			fmt.Printf("MERGE CONFLICT: %s — %s\n", epics[idx].Name, string(out))
			fmt.Printf("  Resolve manually, then: git merge --continue\n")
			fmt.Printf("  Remaining branches: ")
			for _, j := range mergeIndices[mergedCount+1:] {
				fmt.Printf("%s ", worktrees[j].branch)
			}
			fmt.Println()
			return mergedCount, fmt.Errorf("merge conflict on %s: %w", branch, err)
		}
		mergedCount++
		fmt.Printf("MERGED: %s (%s)\n", epics[idx].Name, branch)
	}
	fmt.Println()

	return mergedCount, nil
}

// runParallelEpic executes one epic in its worktree via ao rpi phased subprocess.
func runParallelEpic(epic parallelEpic, worktreePath, branch, runtimeCmd, logFile string, timeout time.Duration) parallelResult {
	result := parallelResult{
		Epic:     epic,
		Worktree: worktreePath,
		Branch:   branch,
	}

	// Open log file.
	f, err := os.Create(logFile)
	if err != nil {
		result.Error = fmt.Errorf("create log file: %w", err)
		return result
	}
	defer func() { _ = f.Close() }()

	// Build the phased command. Use `ao rpi phased` if ao is on PATH,
	// otherwise fall back to spawning the runtime directly.
	var cmd *exec.Cmd
	aoPath, aoErr := exec.LookPath("ao")
	if aoErr == nil {
		// Use ao rpi phased (preferred — gets full orchestration).
		args := []string{"rpi", "phased", epic.Goal, "--no-worktree"}
		if runtimeCmd != "claude" {
			args = append(args, "--runtime-cmd", runtimeCmd)
		}
		cmd = exec.Command(aoPath, args...)
	} else {
		// Fallback: spawn runtime directly with a prompt.
		prompt := fmt.Sprintf("/rpi %q", epic.Goal)
		cmd = exec.Command(runtimeCmd, runtimeDirectCommandArgs(runtimeCmd, prompt)...)
	}

	cmd.Dir = worktreePath
	cmd.Stdout = f
	cmd.Stderr = f
	cmd.Env = append(os.Environ(),
		"AGENTOPS_RPI_NO_WORKTREE=1", // Prevent nested worktree creation.
	)

	// Start with timeout.
	if err := cmd.Start(); err != nil {
		result.Error = fmt.Errorf("start: %w", err)
		return result
	}

	// Wait with timeout.
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case err := <-done:
		if err != nil {
			result.Error = fmt.Errorf("epic %s failed: %w", epic.Name, err)
			return result
		}
	case <-time.After(timeout):
		_ = cmd.Process.Kill()
		result.Error = fmt.Errorf("epic %s timed out after %s", epic.Name, timeout)
		return result
	}

	// Get latest commit SHA from worktree.
	shaOut, err := exec.Command("git", "-C", worktreePath, "rev-parse", "--short", "HEAD").Output()
	if err == nil {
		result.CommitSHA = strings.TrimSpace(string(shaOut))
	}

	result.Success = true
	return result
}

// resolveParallelEpics builds the epic list from manifest file or CLI args.
func resolveParallelEpics(args []string) ([]parallelEpic, error) {
	if parallelManifest != "" {
		data, err := os.ReadFile(parallelManifest)
		if err != nil {
			return nil, fmt.Errorf("read manifest: %w", err)
		}
		var mf parallelManifestFile
		if err := json.Unmarshal(data, &mf); err != nil {
			return nil, fmt.Errorf("parse manifest: %w", err)
		}
		if len(mf.Epics) == 0 {
			return nil, fmt.Errorf("manifest has no epics")
		}
		// Validate names are unique.
		seen := make(map[string]bool)
		for i, e := range mf.Epics {
			if e.Name == "" {
				mf.Epics[i].Name = fmt.Sprintf("epic-%d", i+1)
			}
			if e.Goal == "" {
				return nil, fmt.Errorf("epic %q has no goal", mf.Epics[i].Name)
			}
			if seen[mf.Epics[i].Name] {
				return nil, fmt.Errorf("duplicate epic name: %s", mf.Epics[i].Name)
			}
			seen[mf.Epics[i].Name] = true
			if mf.Epics[i].MergeOrder == 0 {
				mf.Epics[i].MergeOrder = i + 1
			}
		}
		return mf.Epics, nil
	}

	if len(args) == 0 {
		return nil, nil
	}

	// Build from positional args.
	epics := make([]parallelEpic, len(args))
	for i, goal := range args {
		name := fmt.Sprintf("epic-%d", i+1)
		// Try to derive a short name from the goal.
		slug := goalSlug(goal)
		if slug != "" {
			name = slug
		}
		epics[i] = parallelEpic{
			Name:       name,
			Goal:       goal,
			MergeOrder: i + 1,
		}
	}
	return epics, nil
}

// resolveMergeOrder returns indices in the order they should be merged.
func resolveMergeOrder(epics []parallelEpic, results []parallelResult) []int {
	if parallelMergeOrder != "" {
		names := make([]string, len(epics))
		for i, e := range epics {
			names[i] = e.Name
		}
		return cliRPI.ResolveMergeOrderByNames(names, parallelMergeOrder)
	}
	orders := make([]int, len(epics))
	for i, e := range epics {
		orders[i] = e.MergeOrder
	}
	return cliRPI.ResolveMergeOrderByField(orders)
}

// goalSlug delegates to internal/rpi.GoalSlug.
func goalSlug(goal string) string { return cliRPI.GoalSlug(goal) }

// shellQuote delegates to internal/rpi.ShellQuote.
func shellQuote(s string) string { return cliRPI.ShellQuote(s) }

// tmuxPaneIsDead checks whether a tmux pane has exited.
// Returns (dead, exitCode). If the pane/window is gone, returns (true, 1).
func tmuxPaneIsDead(tmuxCmd, target string) (bool, int) {
	out, err := exec.Command(tmuxCmd, "list-panes", "-t", target, "-F", "#{pane_dead} #{pane_dead_status}").Output()
	if err != nil {
		return true, 1 // window gone = dead
	}
	fields := strings.Fields(strings.TrimSpace(string(out)))
	if len(fields) >= 1 && fields[0] == "1" {
		exitCode := 0
		if len(fields) >= 2 {
			_, _ = fmt.Sscanf(fields[1], "%d", &exitCode)
		}
		return true, exitCode
	}
	return false, 0
}

// runParallelEpicTmux executes one epic in a tmux window.
func runParallelEpicTmux(epic parallelEpic, worktreePath, branch, runtimeCmd, logFile, logDir, tmuxSession, tmuxCmd string, timeout time.Duration) parallelResult {
	result := parallelResult{
		Epic:     epic,
		Worktree: worktreePath,
		Branch:   branch,
	}

	// Write goal to file to avoid shell quoting issues.
	goalFile := filepath.Join(logDir, epic.Name+".goal")
	if err := os.WriteFile(goalFile, []byte(epic.Goal), 0o600); err != nil {
		result.Error = fmt.Errorf("write goal file: %w", err)
		return result
	}

	// Build runner script.
	aoPath, aoErr := exec.LookPath("ao")
	var cmdLine string
	if aoErr == nil {
		runtimeFlag := ""
		if runtimeCmd != "claude" {
			runtimeFlag = " --runtime-cmd " + shellQuote(runtimeCmd)
		}
		cmdLine = fmt.Sprintf("%s rpi phased \"$goal\" --no-worktree%s", shellQuote(aoPath), runtimeFlag)
	} else {
		cmdLine = fmt.Sprintf("%s -p \"/rpi $goal\"", shellQuote(runtimeCmd))
	}

	script := fmt.Sprintf(`#!/bin/bash
set -o pipefail
cd %s || exit 1
goal=$(cat %s)
%s 2>&1 | tee %s
`, shellQuote(worktreePath), shellQuote(goalFile), cmdLine, shellQuote(logFile))

	scriptFile := filepath.Join(logDir, epic.Name+".sh")
	if err := os.WriteFile(scriptFile, []byte(script), 0o700); err != nil { // #nosec G306
		result.Error = fmt.Errorf("write script: %w", err)
		return result
	}

	// Create tmux window.
	windowTarget := tmuxSession + ":" + epic.Name
	if out, err := exec.Command(tmuxCmd, "new-window", "-t", tmuxSession, "-n", epic.Name, "bash", scriptFile).CombinedOutput(); err != nil {
		result.Error = fmt.Errorf("create tmux window: %s: %w", string(out), err)
		return result
	}

	// Poll for completion.
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	deadline := time.After(timeout)

	for {
		select {
		case <-ticker.C:
			dead, exitCode := tmuxPaneIsDead(tmuxCmd, windowTarget)
			if !dead {
				continue
			}
			if exitCode != 0 {
				result.Error = fmt.Errorf("epic %s exited with code %d", epic.Name, exitCode)
			} else {
				result.Success = true
			}
			// Get latest commit SHA.
			shaOut, err := exec.Command("git", "-C", worktreePath, "rev-parse", "--short", "HEAD").Output()
			if err == nil {
				result.CommitSHA = strings.TrimSpace(string(shaOut))
			}
			// Clean up script (keep goal file for reference).
			_ = os.Remove(scriptFile)
			return result
		case <-deadline:
			_ = exec.Command(tmuxCmd, "kill-pane", "-t", windowTarget).Run()
			result.Error = fmt.Errorf("epic %s timed out after %s", epic.Name, timeout)
			_ = os.Remove(scriptFile)
			return result
		}
	}
}

// truncateGoal is defined in rpi_status.go — reused here.
