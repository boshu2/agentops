// Package lifecycle contains pure helpers for learning maturity, expiry,
// curation, eviction, and seeding lifecycle stages. These functions are
// extracted from cmd/ao for testability and reuse.
package lifecycle

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/boshu2/agentops/cli/internal/types"
)

// DefaultLearningMetadataFields returns the default frontmatter values used
// when normalizing learning files that lack metadata.
func DefaultLearningMetadataFields() map[string]string {
	return map[string]string{
		"utility":       fmt.Sprintf("%.4f", types.InitialUtility),
		"maturity":      "provisional",
		"confidence":    "0.0000",
		"reward_count":  "0",
		"helpful_count": "0",
		"harmful_count": "0",
	}
}

// ParseFrontmatterFields extracts specific fields from YAML frontmatter in a
// markdown file. Returns a map of field name to value for the requested fields.
func ParseFrontmatterFields(path string, fields ...string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	result := make(map[string]string)
	scanner := bufio.NewScanner(f)
	inFrontmatter := false
	dashCount := 0

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if trimmed == "---" {
			dashCount++
			if dashCount == 1 {
				inFrontmatter = true
				continue
			}
			if dashCount == 2 {
				break
			}
		}

		if inFrontmatter {
			for _, field := range fields {
				prefix := field + ":"
				if strings.HasPrefix(trimmed, prefix) {
					val := strings.TrimSpace(strings.TrimPrefix(trimmed, prefix))
					val = strings.Trim(val, "\"'")
					result[field] = val
				}
			}
		}
	}

	return result, scanner.Err()
}

// IsLowSignalLearningBody returns true if a learning body is too short or
// otherwise lacks meaningful signal.
func IsLowSignalLearningBody(body string) bool {
	trimmed := strings.TrimSpace(body)
	if trimmed == "" {
		return true
	}
	if len(trimmed) < 50 {
		return true
	}
	if len(strings.Fields(trimmed)) < 12 {
		return true
	}
	lower := strings.ToLower(trimmed)
	for _, prefix := range []string{
		"till ", "still ", "will ", "let me ",
		"and ", "but ", "or ", "however ", "therefore ",
		"additionally ", "furthermore ",
		"- ", "* ",
	} {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	sentenceEnders := 0
	runes := []rune(trimmed)
	for i, ch := range runes {
		if ch == '.' || ch == '!' || ch == '?' {
			if i == len(runes)-1 || runes[i+1] == ' ' || runes[i+1] == '\n' {
				sentenceEnders++
			}
		}
	}
	return sentenceEnders == 0
}

// StripLearningHeading removes the leading "# Title" from a learning body.
func StripLearningHeading(content string) string {
	lines := strings.Split(content, "\n")
	if len(lines) >= 3 && strings.HasPrefix(strings.TrimSpace(lines[0]), "# ") && strings.TrimSpace(lines[1]) == "" {
		return strings.TrimSpace(strings.Join(lines[2:], "\n"))
	}
	return strings.TrimSpace(content)
}

// IsEvictionEligible reports whether a learning's metrics make it a candidate
// for eviction.
func IsEvictionEligible(utility, confidence float64, maturity string) bool {
	if maturity == "established" {
		return false
	}
	if utility >= 0.3 {
		return false
	}
	return confidence < 0.3
}

// FloatValueFromData extracts a float64 from a generic map, returning the
// default if the key is missing or the wrong type.
func FloatValueFromData(data map[string]any, key string, defaultValue float64) float64 {
	value, ok := data[key].(float64)
	if !ok {
		return defaultValue
	}
	return value
}

// NonEmptyStringFromData extracts a non-empty string from a generic map,
// returning the default if missing or empty.
func NonEmptyStringFromData(data map[string]any, key, defaultValue string) string {
	value, ok := data[key].(string)
	if !ok || value == "" {
		return defaultValue
	}
	return value
}

// ExpiryCategory tracks how learning files are classified by expiry status.
type ExpiryCategory struct {
	Active          []string
	NeverExpiring   []string
	NewlyExpired    []string
	AlreadyArchived []string
}

// ExpiryClassification is the result for a single learning file.
type ExpiryClassification int

const (
	ExpiryNeverExpiring ExpiryClassification = iota
	ExpiryAlreadyArchived
	ExpiryActive
	ExpiryNewlyExpired
)

// ClassifyExpiryFields classifies a single learning by its frontmatter fields.
// now is injected for testability.
func ClassifyExpiryFields(fields map[string]string, now time.Time) ExpiryClassification {
	if fields["expiry_status"] == "archived" {
		return ExpiryAlreadyArchived
	}
	validUntil, hasExpiry := fields["valid_until"]
	if !hasExpiry || validUntil == "" {
		return ExpiryNeverExpiring
	}
	expiry, err := time.Parse("2006-01-02", validUntil)
	if err != nil {
		expiry, err = time.Parse(time.RFC3339, validUntil)
	}
	if err != nil {
		return ExpiryNeverExpiring
	}
	if now.After(expiry) {
		return ExpiryNewlyExpired
	}
	return ExpiryActive
}

// EvictionCitationStatus returns the formatted last-cited string for an
// eviction candidate, or false if the file was cited too recently to evict.
func EvictionCitationStatus(file string, lastCited map[string]time.Time, cutoff time.Time) (string, bool) {
	citedAt, ok := lastCited[file]
	if !ok {
		return "never", true
	}
	if citedAt.After(cutoff) {
		return "", false
	}
	return citedAt.Format("2006-01-02"), true
}

// ShouldArchiveUncitedLearning reports whether a learning is a curation
// candidate due to being uncited past the cutoff.
func ShouldArchiveUncitedLearning(maturity string, modTime time.Time, cited bool, cutoff time.Time) bool {
	switch maturity {
	case "established", "anti-pattern":
		return false
	}
	if modTime.After(cutoff) {
		return false
	}
	return !cited
}

// FormatLastCited returns the last-cited date string for display, or "never".
func FormatLastCited(citedAt time.Time, ok bool) string {
	if !ok {
		return "never"
	}
	return citedAt.Format("2006-01-02")
}

// ParseFrontmatterFloats parses frontmatter fields into a map[string]any
// where numeric values are converted to float64.
func ParseFrontmatterFloats(fields map[string]string) map[string]any {
	data := make(map[string]any, len(fields))
	for k, v := range fields {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			data[k] = f
		} else {
			data[k] = v
		}
	}
	return data
}

// DefaultLearningJSONLDefaults returns the default values applied to JSONL
// learning files when normalizing metadata.
func DefaultLearningJSONLDefaults() map[string]any {
	return map[string]any{
		"utility":       types.InitialUtility,
		"maturity":      "provisional",
		"confidence":    0.0,
		"reward_count":  0,
		"helpful_count": 0,
		"harmful_count": 0,
	}
}

// ApplyJSONLDefaults merges default fields into data, returning whether any
// changes were made.
func ApplyJSONLDefaults(data map[string]any) bool {
	changed := false
	for key, value := range DefaultLearningJSONLDefaults() {
		if existing, ok := data[key]; !ok || existing == nil || existing == "" {
			data[key] = value
			changed = true
		}
	}
	return changed
}

// LearningMetadataFieldOrder returns the canonical ordering of metadata fields
// when emitting frontmatter.
func LearningMetadataFieldOrder() []string {
	return []string{"utility", "maturity", "confidence", "reward_count", "helpful_count", "harmful_count"}
}

// MissingMetadataFields returns the subset of default fields that are absent
// (empty value) from existing.
func MissingMetadataFields(existing map[string]string) map[string]string {
	defaults := DefaultLearningMetadataFields()
	missing := make(map[string]string)
	for key, value := range defaults {
		if existing[key] == "" {
			missing[key] = value
		}
	}
	return missing
}

// BuildMarkdownFrontmatterPrefix builds a YAML frontmatter block populated
// with default learning metadata, suitable for prepending to a markdown file
// that has none.
func BuildMarkdownFrontmatterPrefix() string {
	defaults := DefaultLearningMetadataFields()
	var sb strings.Builder
	sb.WriteString("---\n")
	for _, key := range LearningMetadataFieldOrder() {
		fmt.Fprintf(&sb, "%s: %s\n", key, defaults[key])
	}
	sb.WriteString("---\n")
	return sb.String()
}

// FindFrontmatterEnd finds the index of the closing "---" of a YAML
// frontmatter block, given lines split on \n. Returns -1 if not found.
// The opening "---" is assumed at index 0.
func FindFrontmatterEnd(lines []string) int {
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			return i
		}
	}
	return -1
}

// HasYAMLFrontmatter reports whether text begins with a YAML frontmatter block.
func HasYAMLFrontmatter(text string) bool {
	return strings.HasPrefix(strings.TrimSpace(text), "---")
}

// NormalizeJSONLLine takes the first line of a JSONL learning file and
// returns the normalized line plus whether changes were made. Returns
// ("", false, nil) if no changes are needed.
func NormalizeJSONLLine(line string) (string, bool, error) {
	if strings.TrimSpace(line) == "" {
		return "", false, nil
	}
	var data map[string]any
	if err := json.Unmarshal([]byte(line), &data); err != nil {
		return "", false, err
	}
	if !ApplyJSONLDefaults(data) {
		return "", false, nil
	}
	encoded, err := json.Marshal(data)
	if err != nil {
		return "", false, err
	}
	return string(encoded), true, nil
}

// EvictionEntry is a backend-agnostic record fed to EvictUntilUnderBudget.
// Path is opaque to the helper; Size is the on-disk byte cost the eviction
// will reclaim; Utility drives the ordering (lowest evicted first).
type EvictionEntry struct {
	Path    string
	Size    int64
	Utility float64
}

// EvictUntilUnderBudget returns the prefix of candidates (after sorting by
// utility ascending) needed to bring currentSize below targetSize. If the hub
// is already under budget the slice is empty. If the candidates cannot
// collectively meet the budget, all candidates are returned (best-effort: the
// caller decides whether to log a warning).
func EvictUntilUnderBudget(candidates []EvictionEntry, currentSize, targetSize int64) []EvictionEntry {
	if currentSize < targetSize {
		return nil
	}
	sorted := make([]EvictionEntry, len(candidates))
	copy(sorted, candidates)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].Utility < sorted[j].Utility
	})
	projected := currentSize
	for i, c := range sorted {
		projected -= c.Size
		if projected < targetSize {
			return sorted[:i+1]
		}
	}
	return sorted
}

// ParseSizeBudget parses a human-readable byte budget like "250M", "1G",
// "1024K", "512B", or a plain byte count. Suffix is case-insensitive. Only
// integer values are accepted (no fractional sizes). Empty input, zero, and
// any unrecognized suffix return an error.
func ParseSizeBudget(s string) (int64, error) {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return 0, fmt.Errorf("size budget is empty")
	}
	mult := int64(1)
	last := trimmed[len(trimmed)-1]
	switch last {
	case 'B', 'b':
		mult = 1
		trimmed = trimmed[:len(trimmed)-1]
	case 'K', 'k':
		mult = 1024
		trimmed = trimmed[:len(trimmed)-1]
	case 'M', 'm':
		mult = 1024 * 1024
		trimmed = trimmed[:len(trimmed)-1]
	case 'G', 'g':
		mult = 1024 * 1024 * 1024
		trimmed = trimmed[:len(trimmed)-1]
	default:
		if last < '0' || last > '9' {
			return 0, fmt.Errorf("unknown size suffix %q in %q (want B/K/M/G)", string(last), s)
		}
	}
	if trimmed == "" {
		return 0, fmt.Errorf("size budget %q is missing a number", s)
	}
	n, err := strconv.ParseInt(trimmed, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse size budget %q: %w", s, err)
	}
	if n <= 0 {
		return 0, fmt.Errorf("size budget %q must be > 0", s)
	}
	return n * mult, nil
}

// ReadLearningJSONLData reads the first JSON line from a .jsonl learning file.
func ReadLearningJSONLData(file string) (map[string]any, bool) {
	content, err := os.ReadFile(file)
	if err != nil {
		return nil, false
	}

	lines := strings.Split(string(content), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) == "" {
		return nil, false
	}

	var data map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &data); err != nil {
		return nil, false
	}

	return data, true
}
