// Package ratchet implements the Brownian Ratchet workflow tracking.
// This file implements CASS (Contextual Agent Session Search) maturity transitions.
package ratchet

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/boshu2/agentops/cli/internal/types"
	"github.com/boshu2/agentops/cli/internal/wiki"
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
//   - provisional → candidate: utility >= threshold AND reward_count >= 3
//   - candidate → established: utility >= threshold AND reward_count >= 5 AND (helpful_count > harmful_count OR reward_count >= 10)
//   - any → anti-pattern: utility <= 0.2 AND harmful_count >= 3
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

func readLearningData(learningPath string) (map[string]any, error) {
	content, err := os.ReadFile(learningPath)
	if err != nil {
		return nil, fmt.Errorf("read learning: %w", err)
	}

	lines := strings.Split(string(content), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) == "" {
		return nil, ErrEmptyLearningFile
	}

	// Try JSONL first (existing behavior)
	var data map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &data); err == nil {
		return data, nil
	}

	// Fallback: YAML front matter (--- delimited)
	if strings.TrimSpace(lines[0]) == "---" {
		return parseYAMLFrontMatter(lines)
	}

	return nil, fmt.Errorf("parse learning: unsupported format")
}

// parseYAMLFrontMatter extracts the YAML front matter block from lines.
// Unlike the strict decoder, a missing closing --- is tolerated: all lines
// after the opening delimiter are treated as the YAML body. An empty body is
// an error.
//
// The YAML unmarshalling is delegated to wiki.FrontmatterCodec; this wrapper
// keeps the tolerant no-closing-delimiter and empty-body semantics that the
// ratchet maturity reader depends on.
func parseYAMLFrontMatter(lines []string) (map[string]any, error) {
	var yamlLines []string
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			break
		}
		yamlLines = append(yamlLines, lines[i])
	}
	if len(yamlLines) == 0 {
		return nil, fmt.Errorf("parse YAML front matter: empty")
	}
	// Re-frame as a closed block so the codec parses the YAML body. The
	// codec performs the same yaml.Unmarshal the legacy parser did.
	framed := append([]string{"---"}, yamlLines...)
	framed = append(framed, "---")
	doc := wiki.FrontmatterCodec{}.DecodeLines(framed)
	if !doc.HasFrontmatter {
		return nil, fmt.Errorf("parse YAML front matter: invalid YAML body")
	}
	return doc.Fields, nil
}

func buildMaturityTransitionResult(learningPath string, data map[string]any) *MaturityTransitionResult {
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
	helpfulSignal := result.HelpfulCount > result.HarmfulCount
	implicitHelpful := result.RewardCount >= 10
	if result.Utility >= types.MaturityPromotionThreshold && result.RewardCount >= 5 && (helpfulSignal || implicitHelpful) {
		result.NewMaturity = types.MaturityEstablished
		result.Transitioned = true
		if implicitHelpful && !helpfulSignal {
			result.Reason = fmt.Sprintf("utility %.2f >= %.2f, reward_count %d >= 10 (implicit helpful signal)",
				result.Utility, types.MaturityPromotionThreshold, result.RewardCount)
		} else {
			result.Reason = fmt.Sprintf("utility %.2f >= %.2f, reward_count %d >= 5, helpful > harmful (%d > %d)",
				result.Utility, types.MaturityPromotionThreshold, result.RewardCount, result.HelpfulCount, result.HarmfulCount)
		}
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

func stringFromData(data map[string]any, key, defaultValue string, requireNonEmpty bool) string {
	value, ok := data[key].(string)
	if !ok {
		return defaultValue
	}
	if requireNonEmpty && value == "" {
		return defaultValue
	}
	return value
}

func floatFromData(data map[string]any, key string, defaultValue float64) float64 {
	switch v := data[key].(type) {
	case float64:
		return v
	case int:
		return float64(v)
	default:
		return defaultValue
	}
}

func intFromData(data map[string]any, key string) int {
	switch v := data[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	default:
		return 0
	}
}

// GlobLearningFiles returns all .jsonl and .md files in the given directory.
func GlobLearningFiles(dir string) ([]string, error) {
	jsonl, err := filepath.Glob(filepath.Join(dir, "*.jsonl"))
	if err != nil {
		return nil, fmt.Errorf("glob jsonl in %s: %w", dir, err)
	}
	md, err := filepath.Glob(filepath.Join(dir, "*.md"))
	if err != nil {
		return nil, fmt.Errorf("glob md in %s: %w", dir, err)
	}
	return append(jsonl, md...), nil
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

	updates := map[string]any{
		"maturity":            string(result.NewMaturity),
		"maturity_changed_at": time.Now().Format(time.RFC3339),
		"maturity_reason":     result.Reason,
	}

	if strings.HasSuffix(learningPath, ".md") {
		return result, updateMarkdownFrontMatter(learningPath, updates)
	}
	return result, updateJSONLFirstLine(learningPath, updates)
}

// mergeJSONData unmarshals firstLine as JSON, merges updates into it, and returns re-marshaled JSON.
func mergeJSONData(firstLine string, updates map[string]any) ([]byte, error) {
	var data map[string]any
	if err := json.Unmarshal([]byte(firstLine), &data); err != nil {
		return nil, fmt.Errorf("parse learning for update: %w", err)
	}
	for k, v := range updates {
		data[k] = v
	}
	newJSON, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("marshal updated learning: %w", err)
	}
	return newJSON, nil
}

// updateJSONLFirstLine reads a JSONL file, merges fields into the first
// JSON line, and writes the file back.
func updateJSONLFirstLine(path string, updates map[string]any) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read learning for update: %w", err)
	}

	if len(content) == 0 {
		return ErrEmptyFile
	}
	lines := strings.Split(string(content), "\n")

	newJSON, err := mergeJSONData(lines[0], updates)
	if err != nil {
		return err
	}

	lines[0] = string(newJSON)
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o600); err != nil {
		return fmt.Errorf("write updated learning: %w", err)
	}
	return nil
}

// parseFrontMatterBounds locates the opening and closing --- delimiters in a
// set of lines. It returns the index of the closing delimiter or an error if
// the front matter is missing or malformed.
func parseFrontMatterBounds(lines []string) (int, error) {
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return -1, fmt.Errorf("no front matter found")
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			return i, nil
		}
	}
	return -1, fmt.Errorf("malformed front matter: no closing ---")
}

// updateOrAddFrontMatterFields updates existing fields or appends new ones to
// the given front matter lines.
func updateOrAddFrontMatterFields(fmLines []string, updates map[string]any) []string {
	for key, value := range updates {
		found := false
		for i, line := range fmLines {
			if strings.HasPrefix(line, key+":") {
				fmLines[i] = fmt.Sprintf("%s: %v", key, value)
				found = true
				break
			}
		}
		if !found {
			fmLines = append(fmLines, fmt.Sprintf("%s: %v", key, value))
		}
	}
	return fmLines
}

// rebuildMarkdownFile reassembles a markdown file from front matter lines and
// the body lines that follow the closing delimiter.
func rebuildMarkdownFile(fmLines []string, bodyLines []string) string {
	var sb strings.Builder
	sb.WriteString("---\n")
	for _, line := range fmLines {
		sb.WriteString(line)
		sb.WriteString("\n")
	}
	sb.WriteString("---\n")
	for i, line := range bodyLines {
		sb.WriteString(line)
		if i < len(bodyLines)-1 {
			sb.WriteString("\n")
		}
	}
	return sb.String()
}

// updateMarkdownFrontMatter reads a .md file, finds YAML front matter boundaries,
// updates/adds fields, and writes the file back.
func updateMarkdownFrontMatter(path string, updates map[string]any) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read learning for update: %w", err)
	}

	lines := strings.Split(string(content), "\n")
	endIdx, err := parseFrontMatterBounds(lines)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}

	fmLines := make([]string, endIdx-1)
	copy(fmLines, lines[1:endIdx])
	fmLines = updateOrAddFrontMatterFields(fmLines, updates)

	result := rebuildMarkdownFile(fmLines, lines[endIdx+1:])
	if err := os.WriteFile(path, []byte(result), 0o600); err != nil {
		return fmt.Errorf("write updated learning: %w", err)
	}
	return nil
}

// ScanForMaturityTransitions scans a learnings directory and returns all pending transitions.
func ScanForMaturityTransitions(learningsDir string) ([]*MaturityTransitionResult, error) {
	files, err := GlobLearningFiles(learningsDir)
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

// filterLearningsByMaturity returns learning file paths (JSONL or MD) whose
// metadata has the specified maturity value.
func filterLearningsByMaturity(learningsDir string, target types.Maturity) ([]string, error) {
	files, err := GlobLearningFiles(learningsDir)
	if err != nil {
		return nil, fmt.Errorf("glob learnings: %w", err)
	}

	var result []string
	for _, file := range files {
		if readFirstLineMaturity(file) == string(target) {
			result = append(result, file)
		}
	}
	return result, nil
}

// readFirstLineMaturity reads the metadata of a learning file (JSONL or MD)
// and returns its "maturity" field value, or "" on any error.
func readFirstLineMaturity(path string) string {
	data, err := readLearningData(path)
	if err != nil {
		return ""
	}
	maturity, _ := data["maturity"].(string)
	return maturity
}

// GetAntiPatterns returns all learnings marked as anti-patterns.
func GetAntiPatterns(learningsDir string) ([]string, error) {
	return filterLearningsByMaturity(learningsDir, types.MaturityAntiPattern)
}

// GetEstablishedLearnings returns all learnings with established maturity.
func GetEstablishedLearnings(learningsDir string) ([]string, error) {
	return filterLearningsByMaturity(learningsDir, types.MaturityEstablished)
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
// Counts both .jsonl and .md files with proper metadata parsing.
func GetMaturityDistribution(learningsDir string) (*MaturityDistribution, error) {
	files, err := GlobLearningFiles(learningsDir)
	if err != nil {
		return nil, fmt.Errorf("glob learnings: %w", err)
	}

	dist := &MaturityDistribution{}
	for _, file := range files {
		classifyLearningFile(file, dist)
	}

	return dist, nil
}

// classifyLearningFile reads the metadata of a learning file (JSONL or MD) and
// updates the distribution.
func classifyLearningFile(file string, dist *MaturityDistribution) {
	data, err := readLearningData(file)
	if err != nil {
		// Can't parse = unknown
		dist.Unknown++
		dist.Total++
		return
	}

	maturity, ok := data["maturity"].(string)
	if !ok || maturity == "" {
		maturity = string(types.MaturityProvisional)
	}

	incrementMaturity(dist, types.Maturity(maturity))
	dist.Total++
}

// incrementMaturity increments the appropriate maturity counter in the distribution.
func incrementMaturity(dist *MaturityDistribution, m types.Maturity) {
	switch m {
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
}
