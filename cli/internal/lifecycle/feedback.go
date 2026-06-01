// Package lifecycle (feedback.go) provides pure helpers for the
// `ao feedback` MemRL EMA update flow and JSONL learning migration.
package lifecycle

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/boshu2/agentops/cli/internal/domain"
)

const (
	// ImpliedHelpfulRewardThreshold is the reward at/above which feedback is
	// considered helpful when --helpful is not explicitly set.
	ImpliedHelpfulRewardThreshold = 0.8
	// ImpliedHarmfulRewardThreshold is the reward at/below which feedback is
	// considered harmful when --harmful is not explicitly set.
	ImpliedHarmfulRewardThreshold = 0.2
)

// ResolveReward applies --helpful/--harmful shortcuts and validates reward and alpha.
func ResolveReward(helpful, harmful bool, reward, alpha float64) (float64, error) {
	if helpful && harmful {
		return 0, fmt.Errorf("cannot use both --helpful and --harmful")
	}
	if helpful {
		reward = 1.0
	} else if harmful {
		reward = 0.0
	}
	if reward < 0 {
		return 0, fmt.Errorf("must provide --reward, --helpful, or --harmful")
	}
	if reward > 1 {
		return 0, fmt.Errorf("reward must be between 0.0 and 1.0, got: %f", reward)
	}
	if alpha <= 0 || alpha > 1 {
		return 0, fmt.Errorf("alpha must be between 0 and 1 (exclusive 0), got: %f", alpha)
	}
	return reward, nil
}

// ClassifyFeedbackType returns a human-readable label for the feedback.
func ClassifyFeedbackType(helpful, harmful bool) string {
	if helpful {
		return "helpful"
	}
	if harmful {
		return "harmful"
	}
	return "custom"
}

// CounterDirectionFromFeedback decides whether to increment helpful or harmful counters.
func CounterDirectionFromFeedback(reward float64, explicitHelpful, explicitHarmful bool) (helpful bool, harmful bool) {
	if explicitHelpful {
		return true, false
	}
	if explicitHarmful {
		return false, true
	}
	if reward >= ImpliedHelpfulRewardThreshold {
		return true, false
	}
	if reward <= ImpliedHarmfulRewardThreshold {
		return false, true
	}
	return false, false
}

// ParseJSONLFirstLine reads a file and parses the first line as a JSON object.
func ParseJSONLFirstLine(path string) ([]string, map[string]any, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	lines := strings.Split(string(content), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) == "" {
		return nil, nil, fmt.Errorf("empty JSONL file")
	}
	var data map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &data); err != nil {
		return nil, nil, fmt.Errorf("parse JSONL: %w", err)
	}
	return lines, data, nil
}

// ApplyJSONLRewardFields updates reward fields and CASS counters in data.
func ApplyJSONLRewardFields(data map[string]any, oldUtility, newUtility, reward float64, explicitHelpful, explicitHarmful bool) {
	data["utility"] = newUtility
	data["last_reward"] = reward
	rewardCount := 0
	if rc, ok := data["reward_count"].(float64); ok {
		rewardCount = int(rc)
	}
	data["reward_count"] = rewardCount + 1
	data["last_reward_at"] = time.Now().Format(time.RFC3339)

	incrementHelpful, incrementHarmful := CounterDirectionFromFeedback(reward, explicitHelpful, explicitHarmful)
	if incrementHelpful {
		helpfulCount := 0
		if hc, ok := data["helpful_count"].(float64); ok {
			helpfulCount = int(hc)
		}
		data["helpful_count"] = helpfulCount + 1
	} else if incrementHarmful {
		harmfulCount := 0
		if hc, ok := data["harmful_count"].(float64); ok {
			harmfulCount = int(hc)
		}
		data["harmful_count"] = harmfulCount + 1
	}

	newRewardCount := rewardCount + 1
	confidence := 1.0 - (1.0 / (1.0 + float64(newRewardCount)/5.0))
	data["confidence"] = confidence
	data["last_decay_at"] = time.Now().Format(time.RFC3339)
}

// UpdateJSONLUtility updates utility in a JSONL file.
func UpdateJSONLUtility(path string, reward, alpha float64, explicitHelpful, explicitHarmful bool) (oldUtility, newUtility float64, err error) {
	lines, data, err := ParseJSONLFirstLine(path)
	if err != nil {
		return 0, 0, err
	}
	oldUtility = domain.InitialUtility
	if u, ok := data["utility"].(float64); ok && u > 0 {
		oldUtility = u
	}
	newUtility = (1-alpha)*oldUtility + alpha*reward
	ApplyJSONLRewardFields(data, oldUtility, newUtility, reward, explicitHelpful, explicitHarmful)
	newJSON, err := json.Marshal(data)
	if err != nil {
		return 0, 0, err
	}
	lines[0] = string(newJSON)
	return oldUtility, newUtility, os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0600)
}

// ParseFrontMatterUtility scans front matter lines for the utility value.
func ParseFrontMatterUtility(lines []string) (endIdx int, utility float64, err error) {
	utility = domain.InitialUtility
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			return i, utility, nil
		}
		if strings.HasPrefix(lines[i], "utility:") {
			_, _ = fmt.Sscanf(lines[i], "utility: %f", &utility) //nolint:errcheck
		}
	}
	return -1, 0, fmt.Errorf("malformed front matter: no closing ---")
}

// RebuildWithFrontMatter reconstructs a file with updated front matter and body.
func RebuildWithFrontMatter(updatedFM []string, bodyLines []string) string {
	var sb strings.Builder
	sb.WriteString("---\n")
	for _, line := range updatedFM {
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

// UpdateFrontMatterFields updates or adds fields in front matter lines.
func UpdateFrontMatterFields(lines []string, fields map[string]string) []string {
	result := make([]string, 0, len(lines)+len(fields))
	seen := make(map[string]bool)
	for _, line := range lines {
		updated := false
		for key, value := range fields {
			if strings.HasPrefix(line, key+":") {
				result = append(result, fmt.Sprintf("%s: %s", key, value))
				seen[key] = true
				updated = true
				break
			}
		}
		if !updated {
			result = append(result, line)
		}
	}
	for key, value := range fields {
		if !seen[key] {
			result = append(result, fmt.Sprintf("%s: %s", key, value))
		}
	}
	return result
}

// IncrementRewardCount parses and increments reward_count from front matter.
func IncrementRewardCount(lines []string) string {
	count := 0
	for _, line := range lines {
		if strings.HasPrefix(line, "reward_count:") {
			_, _ = fmt.Sscanf(line, "reward_count: %d", &count) //nolint:errcheck
			break
		}
	}
	return fmt.Sprintf("%d", count+1)
}

// ParseFrontMatterInt scans front matter lines for a named integer field.
func ParseFrontMatterInt(lines []string, field string) int {
	val := 0
	prefix := field + ":"
	for _, line := range lines {
		if strings.HasPrefix(line, prefix) {
			_, _ = fmt.Sscanf(line, field+": %d", &val) //nolint:errcheck
			break
		}
	}
	return val
}

// IncrementFMCount returns the incremented value of an int frontmatter field.
func IncrementFMCount(lines []string, field string) string {
	return fmt.Sprintf("%d", ParseFrontMatterInt(lines, field)+1)
}

// UpdateMarkdownUtility updates utility in a markdown file with front matter.
func UpdateMarkdownUtility(path string, reward, alpha float64, explicitHelpful, explicitHarmful bool) (oldUtility, newUtility float64, err error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return 0, 0, err
	}
	text := string(content)
	lines := strings.Split(text, "\n")
	hasFrontMatter := len(lines) > 0 && strings.TrimSpace(lines[0]) == "---"

	if hasFrontMatter {
		endIdx, oldU, fmErr := ParseFrontMatterUtility(lines)
		if fmErr != nil {
			return 0, 0, fmErr
		}
		oldUtility = oldU
		newUtility = (1-alpha)*oldUtility + alpha*reward

		fmLines := lines[1:endIdx]
		newRewardCountStr := IncrementRewardCount(fmLines)
		newRewardCount := ParseFrontMatterInt(fmLines, "reward_count") + 1
		confidence := 1.0 - (1.0 / (1.0 + float64(newRewardCount)/5.0))

		fields := map[string]string{
			"utility":        fmt.Sprintf("%.4f", newUtility),
			"last_reward":    fmt.Sprintf("%.2f", reward),
			"reward_count":   newRewardCountStr,
			"last_reward_at": time.Now().Format(time.RFC3339),
			"confidence":     fmt.Sprintf("%.4f", confidence),
			"last_decay_at":  time.Now().Format(time.RFC3339),
		}

		incrementHelpful, incrementHarmful := CounterDirectionFromFeedback(reward, explicitHelpful, explicitHarmful)
		if incrementHelpful {
			fields["helpful_count"] = IncrementFMCount(fmLines, "helpful_count")
		} else if incrementHarmful {
			fields["harmful_count"] = IncrementFMCount(fmLines, "harmful_count")
		}

		updatedFM := UpdateFrontMatterFields(fmLines, fields)
		rebuilt := RebuildWithFrontMatter(updatedFM, lines[endIdx+1:])
		return oldUtility, newUtility, os.WriteFile(path, []byte(rebuilt), 0600)
	}

	oldUtility = domain.InitialUtility
	newUtility = (1-alpha)*oldUtility + alpha*reward

	var sb strings.Builder
	sb.WriteString("---\n")
	fmt.Fprintf(&sb, "utility: %.4f\n", newUtility)
	fmt.Fprintf(&sb, "last_reward: %.2f\n", reward)
	sb.WriteString("reward_count: 1\n")
	fmt.Fprintf(&sb, "last_reward_at: %s\n", time.Now().Format(time.RFC3339))
	sb.WriteString("---\n")
	sb.WriteString(text)
	return oldUtility, newUtility, os.WriteFile(path, []byte(sb.String()), 0600)
}

// UpdateLearningUtility dispatches to JSONL or markdown updater based on extension.
func UpdateLearningUtility(path string, reward, alpha float64, explicitHelpful, explicitHarmful bool) (oldUtility, newUtility float64, err error) {
	if strings.HasSuffix(path, ".jsonl") {
		return UpdateJSONLUtility(path, reward, alpha, explicitHelpful, explicitHarmful)
	}
	return UpdateMarkdownUtility(path, reward, alpha, explicitHelpful, explicitHarmful)
}

// NeedsUtilityMigration checks whether a JSONL learning file is missing utility.
func NeedsUtilityMigration(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer func() { _ = f.Close() }() //nolint:errcheck

	scanner := bufio.NewScanner(f)
	if scanner.Scan() {
		var data map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &data); err != nil {
			return false, err
		}
		if u, ok := data["utility"].(float64); ok && u > 0 {
			return false, nil
		}
		return true, nil
	}
	return false, nil
}

// AddUtilityField adds the default utility value to a JSONL learning file.
func AddUtilityField(path string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	lines := strings.Split(string(content), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) == "" {
		return nil
	}
	var data map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &data); err != nil {
		return err
	}
	data["utility"] = domain.InitialUtility
	newJSON, err := json.Marshal(data)
	if err != nil {
		return err
	}
	lines[0] = string(newJSON)
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0600)
}
