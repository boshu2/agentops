// Package verdict parses independent-review verdict artifacts without owning
// filesystem or command concerns.
package verdict

import (
	"bufio"
	"regexp"
	"strings"
)

// LineCap bounds a single verdict line. Artifacts beyond the cap fail closed.
const LineCap = 1 << 20 // 1 MiB

var (
	commandsHeader = regexp.MustCompile(`(?i)commands[ _-]*run`)
	reasonsHeader  = regexp.MustCompile(`^\s*reasons`)
	passToken      = regexp.MustCompile(`(?i)^\s*VERDICT:\s*PASS\b`)
	failToken      = regexp.MustCompile(`(?i)^\s*VERDICT:\s*FAIL\b`)
)

// IdentityInfo is the normalized identity tuple carried by a verdict.
type IdentityInfo struct {
	Author           string
	JudgeName        string
	JudgeProgram     string
	JudgeModelFamily string
	ContextID        string
	AuthorContextID  string
}

// ScanLines returns every artifact line or an error when a line exceeds LineCap.
func ScanLines(text string) ([]string, error) {
	scanner := bufio.NewScanner(strings.NewReader(text))
	scanner.Buffer(make([]byte, 0, 64*1024), LineCap)
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return lines, nil
}

// HasCommandsRun reports whether the artifact cites at least one command.
func HasCommandsRun(text string) bool {
	lines, err := ScanLines(text)
	if err != nil {
		return false
	}
	inBlock := false
	for _, line := range lines {
		lower := strings.ToLower(line)
		if commandsHeader.MatchString(line) {
			inBlock = true
			if index := strings.Index(line, ":"); index >= 0 && strings.TrimSpace(line[index+1:]) != "" {
				return true
			}
			continue
		}
		if inBlock && reasonsHeader.MatchString(lower) {
			inBlock = false
		}
		if inBlock && strings.TrimSpace(line) != "" {
			return true
		}
	}
	return false
}

// Identity returns the normalized tuple and every reason it cannot establish
// an independent judge.
func Identity(text string) (IdentityInfo, []string) {
	info := IdentityInfo{
		Author:           NormalizeIdentityValue(MetadataValue(text, "author", "author_id", "author-id", "author id", "author_name", "author-name", "author name")),
		JudgeName:        NormalizeIdentityValue(MetadataValue(text, "judge", "judge_name", "judge-name", "judge name", "judge_id", "judge-id", "judge id")),
		JudgeProgram:     NormalizeIdentityValue(MetadataValue(text, "judge_program", "judge-program", "judge program", "program", "validator_program", "validator-program", "validator program")),
		JudgeModelFamily: NormalizeModelFamily(MetadataValue(text, "judge_model_family", "judge-model-family", "judge model family", "model_family", "model-family", "model family", "family")),
		ContextID:        NormalizeIdentityValue(MetadataValue(text, "context_id", "context-id", "context id", "validator_session", "validator-session", "validator session")),
		AuthorContextID:  NormalizeIdentityValue(MetadataValue(text, "author_context_id", "author-context-id", "author context id")),
	}
	var gaps []string
	if info.Author == "" {
		gaps = append(gaps, "missing author")
	}
	if info.JudgeName == "" {
		gaps = append(gaps, "missing judge.name")
	}
	if info.JudgeProgram == "" {
		gaps = append(gaps, "missing judge.program")
	}
	if info.JudgeModelFamily == "" {
		gaps = append(gaps, "missing judge.model_family")
	} else if UnknownModelFamily(info.JudgeModelFamily) {
		gaps = append(gaps, "judge.model_family is unknown")
	}
	if info.ContextID == "" {
		gaps = append(gaps, "missing judge.context_id")
	}
	if info.Author != "" && info.JudgeName != "" && info.Author == info.JudgeName {
		gaps = append(gaps, "judge.name equals author")
	}
	if info.AuthorContextID != "" && info.ContextID != "" && info.AuthorContextID == info.ContextID {
		gaps = append(gaps, "judge.context_id equals author context")
	}
	if MetadataValue(text, "allow_self", "allow-self", "allow self", "self_waiver", "self-waiver", "self waiver") != "" {
		gaps = append(gaps, "self-judge waiver must be external and principal-logged, not verdict-authored")
	}
	return info, gaps
}

// MetadataValue returns the first matching metadata field.
func MetadataValue(text string, keys ...string) string {
	wanted := map[string]bool{}
	for _, key := range keys {
		wanted[NormalizeMetadataKey(key)] = true
	}
	lines, err := ScanLines(text)
	if err != nil {
		return ""
	}
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		line = strings.TrimPrefix(line, "-")
		line = strings.TrimPrefix(strings.TrimSpace(line), "*")
		line = strings.TrimSpace(line)
		index := strings.Index(line, ":")
		if index < 0 || !wanted[NormalizeMetadataKey(line[:index])] {
			continue
		}
		return strings.Trim(strings.TrimSpace(line[index+1:]), "`\"'")
	}
	return ""
}

func NormalizeMetadataKey(key string) string {
	key = strings.ToLower(strings.TrimSpace(key))
	key = strings.ReplaceAll(key, "-", "_")
	return strings.ReplaceAll(key, " ", "_")
}

func NormalizeIdentityValue(value string) string { return strings.TrimSpace(value) }
func NormalizeModelFamily(value string) string   { return strings.ToLower(strings.TrimSpace(value)) }

func UnknownModelFamily(family string) bool {
	switch strings.ToLower(strings.TrimSpace(family)) {
	case "", "unknown", "none", "n/a", "na", "unset":
		return true
	default:
		return false
	}
}

// TokenCounts counts exact PASS and FAIL verdict tokens. Unscannable artifacts
// return zero counts so callers fail closed.
func TokenCounts(text string) (pass, fail int) {
	lines, err := ScanLines(text)
	if err != nil {
		return 0, 0
	}
	for _, line := range lines {
		if passToken.MatchString(line) {
			pass++
		}
		if failToken.MatchString(line) {
			fail++
		}
	}
	return pass, fail
}
