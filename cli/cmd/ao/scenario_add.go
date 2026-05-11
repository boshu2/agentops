// practices: [property-based-testing, llm-eval-harness]
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var (
	scenarioIDPattern = regexp.MustCompile(`^(s-\d{4}-\d{2}-\d{2}-\d{3}|auto-.+)$`)
	scenarioAddNow    = time.Now
	scenarioAddFlags  = struct {
		Narrative       string
		ExpectedOutcome string
		Threshold       float64
		Status          string
		Source          string
	}{
		Threshold: 0.8,
		Status:    "draft",
		Source:    "human",
	}
)

type scenarioFile struct {
	ID                    string                     `json:"id"`
	Version               int                        `json:"version"`
	Date                  string                     `json:"date"`
	Goal                  string                     `json:"goal"`
	Narrative             string                     `json:"narrative"`
	ExpectedOutcome       string                     `json:"expected_outcome"`
	AcceptanceVectors     []scenarioAcceptanceVector `json:"acceptance_vectors,omitempty"`
	SatisfactionThreshold float64                    `json:"satisfaction_threshold"`
	Source                string                     `json:"source,omitempty"`
	Status                string                     `json:"status"`
}

type scenarioAcceptanceVector struct {
	Dimension string  `json:"dimension"`
	Threshold float64 `json:"threshold"`
	Check     string  `json:"check,omitempty"`
}

var scenarioAddCmd = &cobra.Command{
	Use:   "add <goal>",
	Short: "Author a holdout scenario from a goal description",
	Long: `Author a schema-compliant holdout scenario in .agents/holdout/.

The command infers narrative and expected-outcome text from the provided goal
unless explicit values are supplied. New scenarios default to draft so a human
or evaluator can review them before activation.`,
	Args: cobra.ExactArgs(1),
	RunE: runScenarioAdd,
}

func runScenarioAdd(cmd *cobra.Command, args []string) error {
	goal := strings.TrimSpace(args[0])
	if goal == "" {
		return fmt.Errorf("goal is required")
	}
	if err := validateScenarioAddFlags(); err != nil {
		return err
	}

	holdoutDir := filepath.Join(".agents", "holdout")
	if err := os.MkdirAll(holdoutDir, 0o755); err != nil {
		return fmt.Errorf("creating holdout directory: %w", err)
	}

	date := scenarioAddNow().UTC().Format("2006-01-02")
	id, err := nextScenarioID(holdoutDir, date)
	if err != nil {
		return err
	}
	scenario := scenarioFile{
		ID:                    id,
		Version:               1,
		Date:                  date,
		Goal:                  goal,
		Narrative:             scenarioNarrative(goal, scenarioAddFlags.Narrative),
		ExpectedOutcome:       scenarioExpectedOutcome(goal, scenarioAddFlags.ExpectedOutcome),
		SatisfactionThreshold: scenarioAddFlags.Threshold,
		Source:                scenarioAddFlags.Source,
		Status:                scenarioAddFlags.Status,
	}

	path := filepath.Join(holdoutDir, id+".json")
	data, err := json.MarshalIndent(scenario, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling scenario: %w", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("writing scenario %s: %w", path, err)
	}

	if GetOutput() == "json" {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(scenario)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Created scenario %s at %s\n", id, path)
	return nil
}

func validateScenarioAddFlags() error {
	if scenarioAddFlags.Threshold < 0 || scenarioAddFlags.Threshold > 1 {
		return fmt.Errorf("threshold %.2f out of range [0, 1]", scenarioAddFlags.Threshold)
	}
	if !validScenarioStatus(scenarioAddFlags.Status) {
		return fmt.Errorf("invalid status %q (must be active, draft, or retired)", scenarioAddFlags.Status)
	}
	if !validScenarioSource(scenarioAddFlags.Source) {
		return fmt.Errorf("invalid source %q (must be human, agent, or prod-telemetry)", scenarioAddFlags.Source)
	}
	return nil
}

func nextScenarioID(holdoutDir, date string) (string, error) {
	entries, err := os.ReadDir(holdoutDir)
	if err != nil {
		return "", fmt.Errorf("reading holdout directory: %w", err)
	}
	prefix := "s-" + date + "-"
	maxSeq := 0
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		for _, candidate := range scenarioIDCandidates(holdoutDir, entry.Name()) {
			if seq, ok := scenarioSequence(candidate, prefix); ok && seq > maxSeq {
				maxSeq = seq
			}
		}
	}
	return fmt.Sprintf("%s%03d", prefix, maxSeq+1), nil
}

func scenarioIDCandidates(holdoutDir, fileName string) []string {
	candidates := []string{strings.TrimSuffix(fileName, filepath.Ext(fileName))}
	data, err := os.ReadFile(filepath.Join(holdoutDir, fileName))
	if err != nil {
		return candidates
	}
	var decoded struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(data, &decoded); err == nil && strings.TrimSpace(decoded.ID) != "" {
		candidates = append(candidates, strings.TrimSpace(decoded.ID))
	}
	return candidates
}

func scenarioSequence(id, prefix string) (int, bool) {
	if !strings.HasPrefix(id, prefix) {
		return 0, false
	}
	seqText := strings.TrimPrefix(id, prefix)
	if len(seqText) != 3 {
		return 0, false
	}
	seq, err := strconv.Atoi(seqText)
	if err != nil {
		return 0, false
	}
	return seq, true
}

func scenarioNarrative(goal, override string) string {
	if value := strings.TrimSpace(override); value != "" {
		return value
	}
	return fmt.Sprintf("A user or evaluator exercises the system behavior for this goal: %s.", goal)
}

func scenarioExpectedOutcome(goal, override string) string {
	if value := strings.TrimSpace(override); value != "" {
		return value
	}
	return fmt.Sprintf("The implementation satisfies the goal in observable behavior: %s.", goal)
}

func validScenarioStatus(status string) bool {
	switch status {
	case "active", "draft", "retired":
		return true
	default:
		return false
	}
}

func validScenarioSource(source string) bool {
	switch source {
	case "human", "agent", "prod-telemetry":
		return true
	default:
		return false
	}
}

func init() {
	scenarioAddCmd.Flags().StringVar(&scenarioAddFlags.Narrative, "narrative", "", "Narrative description (default: inferred from goal)")
	scenarioAddCmd.Flags().StringVar(&scenarioAddFlags.ExpectedOutcome, "expected-outcome", "", "Expected observable outcome (default: inferred from goal)")
	scenarioAddCmd.Flags().Float64Var(&scenarioAddFlags.Threshold, "threshold", 0.8, "Satisfaction threshold in [0,1]")
	scenarioAddCmd.Flags().StringVar(&scenarioAddFlags.Status, "status", "draft", "Scenario status (active, draft, retired)")
	scenarioAddCmd.Flags().StringVar(&scenarioAddFlags.Source, "source", "human", "Scenario source (human, agent, prod-telemetry)")
	_ = scenarioAddCmd.RegisterFlagCompletionFunc("status", staticCompletionFunc("active", "draft", "retired"))
	_ = scenarioAddCmd.RegisterFlagCompletionFunc("source", staticCompletionFunc("human", "agent", "prod-telemetry"))
	scenarioCmd.AddCommand(scenarioAddCmd)
}
