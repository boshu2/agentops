// Package ratchet implements the Brownian Ratchet workflow tracking.
// This file implements CASS (Contextual Agent Session Search) maturity transitions.
package ratchet

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/boshu2/agentops/cli/internal/types"
)

// MaturityTransitionResult contains the result of a maturity transition check.
type MaturityTransitionResult struct {
	// LearningID is the identifier of the learning.
	LearningID string `json:"learning_id"`

	// OldMaturity is the maturity before transition.
	OldMaturity types.Maturity `json:"old_maturity"`

	// NewMaturity is the maturity after transition.
	NewMaturity types.Maturity `json:"new_maturity"`

	// Transitioned indicates if a transition occurred.
	Transitioned bool `json:"transitioned"`

	// Reason explains why the transition did or didn't occur.
	Reason string `json:"reason"`

	// Utility is the current utility value.
	Utility float64 `json:"utility"`

	// Confidence is the current confidence value.
	Confidence float64 `json:"confidence"`

	// HelpfulCount is the number of helpful feedback events.
	HelpfulCount int `json:"helpful_count"`

	// HarmfulCount is the number of harmful feedback events.
	HarmfulCount int `json:"harmful_count"`

	// RewardCount is the total number of feedback events.
	RewardCount int `json:"reward_count"`
}

// CheckMaturityTransition evaluates if a learning should transition to a new maturity level.
// Transition rules:
//   - provisional → candidate: utility >= 0.7 AND reward_count >= 3
//   - candidate → established: utility >= 0.7 AND reward_count >= 5 AND helpful_count > harmful_count
//   - any → anti-pattern: utility <= 0.2 AND harmful_count >= 5
//   - established → candidate: utility < 0.5 (demotion)
//   - candidate → provisional: utility < 0.3 (demotion)
func CheckMaturityTransition(learningPath string) (*MaturityTransitionResult, error) {
	data, err := readLearningData(learningPath)
	if err != nil {
		return nil, err
	}

	result := buildMaturityTransitionResult(learningPath, data)
	if applyAntiPatternTransition(result) {
		return result, nil
	}

	applyMaturitySpecificTransition(result)
	return result, nil
}

func readLearningData(learningPath string) (map[string]interface{}, error) {
	content, err := os.ReadFile(learningPath)
	if err != nil {
		return nil, fmt.Errorf("read learning: %w", err)
	}

	lines := strings.Split(string(content), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) == "" {
		return nil, fmt.Errorf("empty learning file")
	}

	var data map[string]interface{}
	if err := json.Unmarshal([]byte(lines[0]), &data); err != nil {
		return nil, fmt.Errorf("parse learning: %w", err)
	}

	return data, nil
}

func buildMaturityTransitionResult(learningPath string, data map[string]interface{}) *MaturityTransitionResult {
	learningID := stringFromData(data, "id", filepath.Base(learningPath), false)
	currentMaturity := types.Maturity(stringFromData(data, "maturity", string(types.MaturityProvisional), true))

	return &MaturityTransitionResult{
		LearningID:   learningID,
		OldMaturity:  currentMaturity,
		NewMaturity:  currentMaturity,
		Transitioned: false,
		Utility:      floatFromData(data, "utility", types.InitialUtility),
		Confidence:   floatFromData(data, "confidence", 0.5),
		HelpfulCount: intFromData(data, "helpful_count"),
		HarmfulCount: intFromData(data, "harmful_count"),
		RewardCount:  intFromData(data, "reward_count"),
	}
}

func applyAntiPatternTransition(result *MaturityTransitionResult) bool {
	if result.Utility > types.MaturityAntiPatternThreshold || result.HarmfulCount < types.MinFeedbackForAntiPattern {
		return false
	}

	result.NewMaturity = types.MaturityAntiPattern
	result.Transitioned = result.OldMaturity != types.MaturityAntiPattern
	result.Reason = fmt.Sprintf("utility %.2f <= %.2f and harmful_count %d >= %d",
		result.Utility, types.MaturityAntiPatternThreshold, result.HarmfulCount, types.MinFeedbackForAntiPattern)
	return true
}

func applyMaturitySpecificTransition(result *MaturityTransitionResult) {
	switch result.OldMaturity {
	case types.MaturityProvisional:
		applyProvisionalTransition(result)
	case types.MaturityCandidate:
		applyCandidateTransition(result)
	case types.MaturityEstablished:
		applyEstablishedTransition(result)
	case types.MaturityAntiPattern:
		applyAntiPatternRehabilitationTransition(result)
	}
}

func applyProvisionalTransition(result *MaturityTransitionResult) {
	if result.Utility >= types.MaturityPromotionThreshold && result.RewardCount >= types.MinFeedbackForPromotion {
		result.NewMaturity = types.MaturityCandidate
		result.Transitioned = true
		result.Reason = fmt.Sprintf("utility %.2f >= %.2f and reward_count %d >= %d",
			result.Utility, types.MaturityPromotionThreshold, result.RewardCount, types.MinFeedbackForPromotion)
		return
	}

	result.Reason = "not enough positive feedback for promotion"
}

func applyCandidateTransition(result *MaturityTransitionResult) {
	if result.Utility >= types.MaturityPromotionThreshold && result.RewardCount >= 5 && result.HelpfulCount > result.HarmfulCount {
		result.NewMaturity = types.MaturityEstablished
		result.Transitioned = true
		result.Reason = fmt.Sprintf("utility %.2f >= %.2f, reward_count %d >= 5, helpful > harmful (%d > %d)",
			result.Utility, types.MaturityPromotionThreshold, result.RewardCount, result.HelpfulCount, result.HarmfulCount)
		return
	}

	if result.Utility < types.MaturityDemotionThreshold {
		result.NewMaturity = types.MaturityProvisional
		result.Transitioned = true
		result.Reason = fmt.Sprintf("utility %.2f < %.2f (demotion)",
			result.Utility, types.MaturityDemotionThreshold)
		return
	}

	result.Reason = "maintaining candidate status"
}

func applyEstablishedTransition(result *MaturityTransitionResult) {
	if result.Utility < 0.5 {
		result.NewMaturity = types.MaturityCandidate
		result.Transitioned = true
		result.Reason = fmt.Sprintf("utility %.2f < 0.5 (demotion from established)",
			result.Utility)
		return
	}

	result.Reason = "maintaining established status"
}

func applyAntiPatternRehabilitationTransition(result *MaturityTransitionResult) {
	if result.Utility >= 0.6 && result.HelpfulCount > result.HarmfulCount*2 {
		result.NewMaturity = types.MaturityProvisional
		result.Transitioned = true
		result.Reason = fmt.Sprintf("utility %.2f >= 0.6 and helpful > 2*harmful (%d > %d) - rehabilitation",
			result.Utility, result.HelpfulCount, result.HarmfulCount*2)
		return
	}

	result.Reason = "maintaining anti-pattern status"
}

func stringFromData(data map[string]interface{}, key, defaultValue string, requireNonEmpty bool) string {
	value, ok := data[key].(string)
	if !ok {
		return defaultValue
	}
	if requireNonEmpty && value == "" {
		return defaultValue
	}
	return value
}

func floatFromData(data map[string]interface{}, key string, defaultValue float64) float64 {
	value, ok := data[key].(float64)
	if !ok {
		return defaultValue
	}
	return value
}

func intFromData(data map[string]interface{}, key string) int {
	value, ok := data[key].(float64)
	if !ok {
		return 0
	}
	return int(value)
}

// ApplyMaturityTransition checks and applies a maturity transition to a learning file.
// Returns the transition result and updates the file if a transition occurred.
func ApplyMaturityTransition(learningPath string) (*MaturityTransitionResult, error) {
	result, err := CheckMaturityTransition(learningPath)
	if err != nil {
		return nil, err
	}

	if !result.Transitioned {
		return result, nil
	}

	// Read and update the file
	content, err := os.ReadFile(learningPath)
	if err != nil {
		return nil, fmt.Errorf("read learning for update: %w", err)
	}

	lines := strings.Split(string(content), "\n")
	if len(lines) == 0 {
		return nil, fmt.Errorf("empty file")
	}

	var data map[string]interface{}
	if err := json.Unmarshal([]byte(lines[0]), &data); err != nil {
		return nil, fmt.Errorf("parse learning for update: %w", err)
	}

	// Update maturity and timestamp
	data["maturity"] = string(result.NewMaturity)
	data["maturity_changed_at"] = time.Now().Format(time.RFC3339)
	data["maturity_reason"] = result.Reason

	// Write back
	newJSON, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("marshal updated learning: %w", err)
	}

	lines[0] = string(newJSON)
	if err := os.WriteFile(learningPath, []byte(strings.Join(lines, "\n")), 0644); err != nil {
		return nil, fmt.Errorf("write updated learning: %w", err)
	}

	return result, nil
}

// ScanForMaturityTransitions scans a learnings directory and returns all pending transitions.
func ScanForMaturityTransitions(learningsDir string) ([]*MaturityTransitionResult, error) {
	files, err := filepath.Glob(filepath.Join(learningsDir, "*.jsonl"))
	if err != nil {
		return nil, fmt.Errorf("glob learnings: %w", err)
	}

	var results []*MaturityTransitionResult

	for _, file := range files {
		result, err := CheckMaturityTransition(file)
		if err != nil {
			continue // Skip files that can't be parsed
		}

		// Only include learnings that would transition
		if result.Transitioned {
			results = append(results, result)
		}
	}

	return results, nil
}

// GetAntiPatterns returns all learnings marked as anti-patterns.
func GetAntiPatterns(learningsDir string) ([]string, error) {
	files, err := filepath.Glob(filepath.Join(learningsDir, "*.jsonl"))
	if err != nil {
		return nil, fmt.Errorf("glob learnings: %w", err)
	}

	var antiPatterns []string

	for _, file := range files {
		f, err := os.Open(file)
		if err != nil {
			continue
		}

		scanner := bufio.NewScanner(f)
		if scanner.Scan() {
			var data map[string]interface{}
			if err := json.Unmarshal(scanner.Bytes(), &data); err != nil {
				_ = f.Close() //nolint:errcheck // read-only, moving to next file
				continue
			}

			if maturity, ok := data["maturity"].(string); ok && maturity == string(types.MaturityAntiPattern) {
				antiPatterns = append(antiPatterns, file)
			}
		}
		_ = f.Close() //nolint:errcheck // read-only, moving to next file
	}

	return antiPatterns, nil
}

// GetEstablishedLearnings returns all learnings with established maturity.
func GetEstablishedLearnings(learningsDir string) ([]string, error) {
	files, err := filepath.Glob(filepath.Join(learningsDir, "*.jsonl"))
	if err != nil {
		return nil, fmt.Errorf("glob learnings: %w", err)
	}

	var established []string

	for _, file := range files {
		f, err := os.Open(file)
		if err != nil {
			continue
		}

		scanner := bufio.NewScanner(f)
		if scanner.Scan() {
			var data map[string]interface{}
			if err := json.Unmarshal(scanner.Bytes(), &data); err != nil {
				_ = f.Close() //nolint:errcheck // read-only, moving to next file
				continue
			}

			if maturity, ok := data["maturity"].(string); ok && maturity == string(types.MaturityEstablished) {
				established = append(established, file)
			}
		}
		_ = f.Close() //nolint:errcheck // read-only, moving to next file
	}

	return established, nil
}

// MaturityDistribution represents the count of learnings at each maturity level.
type MaturityDistribution struct {
	Provisional int `json:"provisional"`
	Candidate   int `json:"candidate"`
	Established int `json:"established"`
	AntiPattern int `json:"anti_pattern"`
	Unknown     int `json:"unknown"`
	Total       int `json:"total"`
}

// GetMaturityDistribution returns the distribution of learnings across maturity levels.
func GetMaturityDistribution(learningsDir string) (*MaturityDistribution, error) {
	files, err := filepath.Glob(filepath.Join(learningsDir, "*.jsonl"))
	if err != nil {
		return nil, fmt.Errorf("glob learnings: %w", err)
	}

	dist := &MaturityDistribution{}

	for _, file := range files {
		f, err := os.Open(file)
		if err != nil {
			continue
		}

		scanner := bufio.NewScanner(f)
		if scanner.Scan() {
			var data map[string]interface{}
			if err := json.Unmarshal(scanner.Bytes(), &data); err != nil {
				_ = f.Close() //nolint:errcheck // read-only, moving to next file
				dist.Unknown++
				dist.Total++
				continue
			}

			maturity, ok := data["maturity"].(string)
			if !ok || maturity == "" {
				maturity = string(types.MaturityProvisional)
			}

			switch types.Maturity(maturity) {
			case types.MaturityProvisional:
				dist.Provisional++
			case types.MaturityCandidate:
				dist.Candidate++
			case types.MaturityEstablished:
				dist.Established++
			case types.MaturityAntiPattern:
				dist.AntiPattern++
			default:
				dist.Unknown++
			}
			dist.Total++
		}
		_ = f.Close() //nolint:errcheck // read-only, moving to next file
	}

	return dist, nil
}
