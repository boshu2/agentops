// practices: [dora-metrics, lean-startup]
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/lifecycle"
	"github.com/boshu2/agentops/cli/internal/ratchet"
	"github.com/boshu2/agentops/cli/internal/resolver"
	"github.com/boshu2/agentops/cli/internal/types"
)

var (
	feedbackReward  float64
	feedbackAlpha   float64
	feedbackHelpful bool
	feedbackHarmful bool
)

var feedbackCmd = &cobra.Command{
	Use:   "feedback <learning-id>",
	Short: "Record reward feedback for a learning",
	Long: `Record reward feedback for a learning to update its utility value.

This implements the MemRL EMA update rule:
  u_{t+1} = (1 - α) × u_t + α × r

Where:
  u_t = current utility value (default: 0.5)
  α   = learning rate (default: 0.1)
  r   = reward signal (0.0 = failure, 1.0 = success)

The utility value affects retrieval ranking in Two-Phase retrieval:
  Score = z_norm(freshness) + λ × z_norm(utility)

CASS Integration:
  - --helpful and --harmful are shortcuts for --reward 1.0 and --reward 0.0
  - Tracks helpful_count and harmful_count for maturity transitions
  - Repeated harmful feedback can promote to anti-pattern status

Examples:
  ao feedback L001 --helpful        # Learning was helpful (same as --reward 1.0)
  ao feedback L001 --harmful        # Learning was harmful (same as --reward 0.0)
  ao feedback L001 --reward 1.0     # Learning was helpful (success)
  ao feedback L001 --reward 0.0     # Learning was not helpful (failure)
  ao feedback L001 --reward 0.75    # Partial success
  ao feedback L001 --reward 1.0 --alpha 0.2   # Faster learning rate`,
	Args: cobra.ExactArgs(1),
	RunE: runFeedback,
}

func init() {
	feedbackCmd.Hidden = true
	feedbackCmd.GroupID = "knowledge"
	rootCmd.AddCommand(feedbackCmd)
	feedbackCmd.Flags().Float64Var(&feedbackReward, "reward", -1, "Reward value (0.0 to 1.0)")
	feedbackCmd.Flags().Float64Var(&feedbackAlpha, "alpha", types.DefaultAlpha, "EMA learning rate")
	feedbackCmd.Flags().BoolVar(&feedbackHelpful, "helpful", false, "Mark as helpful (shortcut for --reward 1.0)")
	feedbackCmd.Flags().BoolVar(&feedbackHarmful, "harmful", false, "Mark as harmful (shortcut for --reward 0.0)")
	// Note: reward is no longer required since --helpful/--harmful can be used instead
}

func resolveReward(helpful, harmful bool, reward, alpha float64) (float64, error) {
	return lifecycle.ResolveReward(helpful, harmful, reward, alpha)
}

func classifyFeedbackType(helpful, harmful bool) string {
	return lifecycle.ClassifyFeedbackType(helpful, harmful)
}

// printFeedbackJSON writes the result as indented JSON to stdout.
func printFeedbackJSON(learningID, learningPath, feedbackType string, oldUtility, newUtility, reward, alpha float64) error {
	result := map[string]any{
		"learning_id":   learningID,
		"path":          learningPath,
		"old_utility":   oldUtility,
		"new_utility":   newUtility,
		"reward":        reward,
		"feedback_type": feedbackType,
		"alpha":         alpha,
		"updated_at":    time.Now().Format(time.RFC3339),
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}

func runFeedback(cmd *cobra.Command, args []string) error {
	learningID := args[0]

	reward, err := resolveReward(feedbackHelpful, feedbackHarmful, feedbackReward, feedbackAlpha)
	if err != nil {
		return err
	}
	feedbackReward = reward

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	learningPath, err := findLearningFile(cwd, learningID)
	if err != nil {
		return fmt.Errorf("find learning: %w", err)
	}

	if GetDryRun() {
		fmt.Printf("[dry-run] Would update utility for %s:\n", learningID)
		fmt.Printf("  Reward: %.2f\n", feedbackReward)
		fmt.Printf("  Alpha: %.2f\n", feedbackAlpha)
		return nil
	}

	oldUtility, newUtility, err := updateLearningUtility(learningPath, feedbackReward, feedbackAlpha)
	if err != nil {
		return fmt.Errorf("update utility: %w", err)
	}

	feedbackType := classifyFeedbackType(feedbackHelpful, feedbackHarmful)

	if GetOutput() == "json" {
		return printFeedbackJSON(learningID, learningPath, feedbackType, oldUtility, newUtility, feedbackReward, feedbackAlpha)
	}

	fmt.Printf("Updated utility for %s\n", learningID)
	fmt.Printf("  Previous: %.3f\n", oldUtility)
	fmt.Printf("  Feedback: %s (reward=%.2f)\n", feedbackType, feedbackReward)
	fmt.Printf("  New:      %.3f\n", newUtility)
	return nil
}

// findLearningFile locates a learning file by ID.
// Delegates to the shared resolver package.
func findLearningFile(baseDir, learningID string) (string, error) {
	return resolver.NewFileResolver(baseDir).Resolve(learningID)
}

func updateLearningUtility(path string, reward, alpha float64) (oldUtility, newUtility float64, err error) {
	return lifecycle.UpdateLearningUtility(path, reward, alpha, feedbackHelpful, feedbackHarmful)
}

func parseJSONLFirstLine(path string) ([]string, map[string]any, error) {
	return lifecycle.ParseJSONLFirstLine(path)
}

func applyJSONLRewardFields(data map[string]any, oldUtility, newUtility, reward float64) {
	lifecycle.ApplyJSONLRewardFields(data, oldUtility, newUtility, reward, feedbackHelpful, feedbackHarmful)
}

func updateJSONLUtility(path string, reward, alpha float64) (oldUtility, newUtility float64, err error) {
	return lifecycle.UpdateJSONLUtility(path, reward, alpha, feedbackHelpful, feedbackHarmful)
}

func counterDirectionFromFeedback(reward float64, explicitHelpful, explicitHarmful bool) (helpful bool, harmful bool) {
	return lifecycle.CounterDirectionFromFeedback(reward, explicitHelpful, explicitHarmful)
}

func parseFrontMatterUtility(lines []string) (endIdx int, utility float64, err error) {
	return lifecycle.ParseFrontMatterUtility(lines)
}

func rebuildWithFrontMatter(updatedFM []string, bodyLines []string) string {
	return lifecycle.RebuildWithFrontMatter(updatedFM, bodyLines)
}

func updateMarkdownUtility(path string, reward, alpha float64) (oldUtility, newUtility float64, err error) {
	return lifecycle.UpdateMarkdownUtility(path, reward, alpha, feedbackHelpful, feedbackHarmful)
}

func updateFrontMatterFields(lines []string, fields map[string]string) []string {
	return lifecycle.UpdateFrontMatterFields(lines, fields)
}

func incrementRewardCount(lines []string) string {
	return lifecycle.IncrementRewardCount(lines)
}

func parseFrontMatterInt(lines []string, field string) int {
	return lifecycle.ParseFrontMatterInt(lines, field)
}

func incrementFMCount(lines []string, field string) string {
	return lifecycle.IncrementFMCount(lines, field)
}

// migrateCmd adds utility field to learnings without it.
var migrateCmd = &cobra.Command{
	Use:   "migrate memrl",
	Short: "Migrate learnings to include utility field",
	Long: `Migrate existing learnings to include MemRL utility field.

Scans .agents/learnings/ and adds utility: 0.5 to entries without it.
This prepares learnings for Two-Phase retrieval.

Examples:
  ao migrate memrl
  ao migrate memrl --dry-run`,
	Args: cobra.ExactArgs(1),
	RunE: runMigrate,
}

func init() {
	migrateCmd.Hidden = true
	migrateCmd.GroupID = "knowledge"
	rootCmd.AddCommand(migrateCmd)
}

// migrateJSONLFiles processes a list of JSONL files, migrating those that lack the utility field.
// Returns the number of files migrated and skipped.
func migrateJSONLFiles(files []string, dryRun bool) (migrated, skipped int) {
	for _, file := range files {
		needsMigration, err := needsUtilityMigration(file)
		if err != nil {
			VerbosePrintf("Warning: check %s: %v\n", filepath.Base(file), err)
			continue
		}
		if !needsMigration {
			skipped++
			continue
		}
		if dryRun {
			fmt.Printf("[dry-run] Would migrate: %s\n", filepath.Base(file))
			migrated++
			continue
		}
		if err := addUtilityField(file); err != nil {
			VerbosePrintf("Warning: migrate %s: %v\n", filepath.Base(file), err)
			continue
		}
		migrated++
	}
	return migrated, skipped
}

func runMigrate(cmd *cobra.Command, args []string) error {
	if args[0] != "memrl" {
		return fmt.Errorf("unknown migration: %s (supported: memrl)", args[0])
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	learningsDir := filepath.Join(cwd, ".agents", "learnings")
	if _, err := os.Stat(learningsDir); os.IsNotExist(err) {
		fmt.Println("No learnings directory found.")
		return nil
	}

	files, err := ratchet.GlobLearningFiles(learningsDir)
	if err != nil {
		return err
	}

	migrated, skipped := migrateJSONLFiles(files, GetDryRun())
	fmt.Printf("Migration complete: %d migrated, %d skipped (already have utility)\n", migrated, skipped)
	return nil
}

func needsUtilityMigration(path string) (bool, error) {
	return lifecycle.NeedsUtilityMigration(path)
}

func addUtilityField(path string) error {
	return lifecycle.AddUtilityField(path)
}
