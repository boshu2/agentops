package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/boshu2/agentops/cli/internal/goals"
	"github.com/spf13/cobra"
)

const evolveBaselineEraHashLen = 12

var evolveCmd = &cobra.Command{
	Use:   "evolve [goal]",
	Short: "Run the autonomous code-improvement loop",
	Long: `Run the v2 autonomous improvement loop.

This is the top-level operator surface for the old /evolve flow. The v2 name is
still "evolve": it uses the same engine as "ao rpi loop" and defaults to
supervisor mode so each cycle gets lease locking, compile producer cadence,
quality gates, retries, and cleanup.

Operator cadence:
  post-mortem finished work, analyze repo state, select or create next work,
  run RPI planning/pre-mortem/implementation/validation, harvest follow-ups,
  and repeat until the queue is stable or a stop condition fires.

Examples:
  ao evolve                          # run until queue stable or stopped
  ao evolve --max-cycles 1           # one supervised autonomous cycle
  ao evolve "improve test coverage"  # run one explicit-goal cycle
  ao evolve --supervisor=false       # use raw rpi loop defaults`,
	Args: cobra.MaximumNArgs(1),
	RunE: runEvolve,
}

var (
	evolveDreamFirst   bool
	evolveDreamOnly    bool
	evolveDreamTimeout string
)

func init() {
	evolveCmd.GroupID = "workflow"
	addRPILoopFlags(evolveCmd)
	if flag := evolveCmd.Flags().Lookup("supervisor"); flag != nil {
		flag.DefValue = "true"
	}
	evolveCmd.Flags().BoolVar(&evolveDreamFirst, "dream-first", false, "Run Dream knowledge sub-cycle before code cycles")
	evolveCmd.Flags().BoolVar(&evolveDreamOnly, "dream-only", false, "Knowledge compounding only, no code cycles")
	evolveCmd.Flags().StringVar(&evolveDreamTimeout, "dream-timeout", "30m", "Timeout for the Dream sub-cycle")
	rootCmd.AddCommand(evolveCmd)
}

func runEvolve(cmd *cobra.Command, args []string) error {
	applyEvolveDefaults(cmd)
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}
	if _, err := resolveRPIToolchainDefaults(); err != nil {
		return err
	}
	if err := ensureEvolveEraBaseline(cwd); err != nil {
		return err
	}

	w := cmd.OutOrStdout()

	// Phase 1: Dream sub-cycle (knowledge compounding).
	if evolveDreamFirst || evolveDreamOnly {
		dreamTimeout, err := time.ParseDuration(evolveDreamTimeout)
		if err != nil {
			return fmt.Errorf("evolve: parse --dream-timeout: %w", err)
		}
		runID := time.Now().UTC().Format("20060102T150405Z")
		fmt.Fprintf(w, "=== Dream sub-cycle (timeout %s) ===\n", dreamTimeout)

		dreamResult, dreamErr := RunDreamSubCycle(cmd.Context(), DreamSubCycleOptions{
			Cwd:           cwd,
			OutputDir:     filepath.Join(cwd, ".agents", "evolve", "dream-"+runID),
			RunID:         "dream-" + runID,
			RunTimeout:    dreamTimeout,
			MaxIterations: 10,
			LogWriter:     w,
		})
		if dreamErr != nil {
			fmt.Fprintf(w, "Dream sub-cycle failed (degraded): %v\n", dreamErr)
		} else {
			fmt.Fprintf(w, "Dream: %d iterations", dreamResult.Iterations)
			if dreamResult.PlateauReason != "" {
				fmt.Fprintf(w, " (plateau: %s)", dreamResult.PlateauReason)
			}
			if dreamResult.Tier1Forge != nil {
				if dreamResult.Tier1Forge.Queued > 0 {
					fmt.Fprintf(w, ", tier1 forge: %d queued", dreamResult.Tier1Forge.Queued)
				} else {
					fmt.Fprintf(w, ", tier1 forge: %d sessions", dreamResult.Tier1Forge.SessionsWrote)
				}
			}
			fmt.Fprintln(w)
		}
	}

	if evolveDreamOnly {
		fmt.Fprintf(w, "=== Dream-only mode, skipping code cycles ===\n")
		return nil
	}

	// Phase 2: Daytime code cycles via the RPI loop.
	return runRPILoop(cmd, args)
}

func applyEvolveDefaults(cmd *cobra.Command) {
	if cmd != nil && cmd.Flags().Changed("supervisor") {
		return
	}
	rpiSupervisor = true
}

func ensureEvolveEraBaseline(cwd string) error {
	if GetDryRun() {
		return nil
	}

	goalsPath, err := resolveEvolveGoalsFile(cwd)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	eraID, err := evolveGoalsEraID(goalsPath)
	if err != nil {
		return err
	}

	baselineDir := evolveEraBaselineDir(cwd, eraID)
	exists, err := hasExistingEvolveEraBaseline(baselineDir)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	timeout := time.Duration(goalsTimeout) * time.Second
	if timeout <= 0 {
		timeout = 120 * time.Second
	}
	if err := goals.RunMeasure(goals.MeasureOptions{
		GoalsFile: goalsPath,
		Timeout:   timeout,
		SnapDir:   baselineDir,
		Stdout:    io.Discard,
		Stderr:    os.Stderr,
	}); err != nil {
		return fmt.Errorf("capture evolve baseline for %s: %w", eraID, err)
	}

	fmt.Printf("Evolve baseline captured: %s\n", baselineDir)
	return nil
}

func resolveEvolveGoalsFile(cwd string) (string, error) {
	for _, name := range []string{"GOALS.md", "GOALS.yaml"} {
		path := filepath.Join(cwd, name)
		info, err := os.Stat(path)
		if err == nil && !info.IsDir() {
			return path, nil
		}
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("stat goals file %s: %w", path, err)
		}
	}
	return "", os.ErrNotExist
}

func evolveGoalsEraID(goalsPath string) (string, error) {
	data, err := os.ReadFile(goalsPath)
	if err != nil {
		return "", fmt.Errorf("read goals file %s: %w", goalsPath, err)
	}
	sum := sha256.Sum256(data)
	return "goals-" + hex.EncodeToString(sum[:])[:evolveBaselineEraHashLen], nil
}

func evolveEraBaselineDir(cwd, eraID string) string {
	return filepath.Join(cwd, ".agents", "evolve", "fitness-baselines", eraID)
}

func hasExistingEvolveEraBaseline(dir string) (bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("read evolve baseline dir %s: %w", dir, err)
	}
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
			return true, nil
		}
	}
	return false, nil
}
