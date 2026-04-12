// Package forge provides pure helpers for transcript and markdown knowledge extraction.
package forge

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	// SnippetMaxLength is the maximum length for extracted text snippets.
	SnippetMaxLength = 200
	// SummaryMaxLength is the maximum length for session summaries.
	SummaryMaxLength = 100
	// CharsPerToken is the rough estimate of characters per token.
	CharsPerToken = 4
)

// IssueIDPattern matches beads issue IDs like "ol-0001", "at-v123", "gt-abc-def".
var IssueIDPattern = regexp.MustCompile(`\b([a-z]{2,3})-([a-z0-9]{3,7}(?:-[a-z0-9]+)?)\b`)

// SessionUUIDPattern matches UUIDs in transcript file names.
var SessionUUIDPattern = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)

// SessionClaudeTranscriptPattern matches Claude session IDs like "ses_abc".
var SessionClaudeTranscriptPattern = regexp.MustCompile(`(ses_[A-Za-z0-9]+)`)

type sessionTypeRule struct {
	sessionType string
	keywords    []string
}

var sessionTypeRules = []sessionTypeRule{
	{sessionType: "career", keywords: []string{"career", "interview", "resume", "salary"}},
	{sessionType: "debug", keywords: []string{"debug", "stack trace", "broken", "error log"}},
	{sessionType: "brainstorm", keywords: []string{"brainstorm", "what if", "option a vs"}},
	{sessionType: "research", keywords: []string{"research", "explore"}},
	{sessionType: "implement", keywords: []string{"go test", "git commit", "implement", "feat("}},
}

// CollectFilesFromPatterns expands glob patterns into a file list, optionally filtered.
func CollectFilesFromPatterns(patterns []string, matchFilter func(string) bool) ([]string, error) {
	var files []string
	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return nil, &PatternError{Pattern: pattern, Err: err}
		}
		if len(matches) == 0 {
			if _, err := os.Stat(pattern); err == nil {
				files = append(files, pattern)
			}
			continue
		}
		for _, match := range matches {
			if matchFilter == nil || matchFilter(match) {
				files = append(files, match)
			}
		}
	}
	return files, nil
}

// PatternError wraps a glob pattern parse error.
type PatternError struct {
	Pattern string
	Err     error
}

func (e *PatternError) Error() string { return "invalid pattern " + e.Pattern + ": " + e.Err.Error() }
func (e *PatternError) Unwrap() error { return e.Err }

// DetectSessionTypeFromContent infers session type from forged content.
func DetectSessionTypeFromContent(summary string, knowledge, decisions []string) string {
	combined := combinedSessionContent(summary, knowledge, decisions)
	for _, rule := range sessionTypeRules {
		if containsAnyPhrase(combined, rule.keywords) {
			return rule.sessionType
		}
	}
	return "general"
}

func combinedSessionContent(summary string, knowledge, decisions []string) string {
	var combined strings.Builder
	combined.WriteString(summary)
	for _, text := range knowledge {
		combined.WriteString(" ")
		combined.WriteString(text)
	}
	for _, text := range decisions {
		combined.WriteString(" ")
		combined.WriteString(text)
	}
	return strings.ToLower(combined.String())
}

func containsAnyPhrase(content string, phrases []string) bool {
	for _, phrase := range phrases {
		if strings.Contains(content, phrase) {
			return true
		}
	}
	return false
}

// InferSessionIDFromPath extracts a session ID from a transcript file path.
func InferSessionIDFromPath(filePath string) string {
	base := filepath.Base(filePath)
	if match := SessionClaudeTranscriptPattern.FindStringSubmatch(base); len(match) > 1 {
		return match[1]
	}
	matches := SessionUUIDPattern.FindAllString(base, -1)
	if len(matches) == 0 {
		return ""
	}
	return matches[len(matches)-1]
}

// GenerateSummary creates a session summary from extracted content.
func GenerateSummary(decisions, knowledge []string, date time.Time) string {
	if len(decisions) > 0 {
		return TruncateString(decisions[0], SummaryMaxLength)
	}
	if len(knowledge) > 0 {
		return TruncateString(knowledge[0], SummaryMaxLength)
	}
	return "Session from " + date.Format("2006-01-02")
}

// CountLines quickly counts newlines in a file.
func CountLines(path string) int {
	f, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer func() { _ = f.Close() }()
	buf := make([]byte, 64*1024)
	count := 0
	for {
		n, err := f.Read(buf)
		if n > 0 {
			for _, b := range buf[:n] {
				if b == '\n' {
					count++
				}
			}
		}
		if err != nil {
			break
		}
	}
	return count
}

// ExtractSnippet extracts a text snippet around a match index.
func ExtractSnippet(content string, startIdx, maxLen int) string {
	if startIdx < 0 {
		startIdx = 0
	}
	if startIdx >= len(content) {
		return ""
	}
	end := startIdx + maxLen
	if end > len(content) {
		end = len(content)
	}
	snippet := content[startIdx:end]
	if end < len(content) {
		if idx := LastSpaceIndex(snippet); idx > maxLen/2 {
			snippet = snippet[:idx]
		}
		snippet += "..."
	}
	return snippet
}

// LastSpaceIndex returns the index of the last space in s, or -1.
func LastSpaceIndex(s string) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == ' ' {
			return i
		}
	}
	return -1
}

// ExtractIssueIDs finds issue IDs like "ol-0001" in content.
func ExtractIssueIDs(content string) []string {
	matches := IssueIDPattern.FindAllString(content, -1)
	if len(matches) == 0 {
		return nil
	}
	return matches
}

// TruncateString limits a string to maxLen characters with an ellipsis.
func TruncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// Dedup removes duplicate strings preserving order.
func Dedup(items []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(items))
	for _, item := range items {
		if !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}
	return result
}

// SplitMarkdownSections splits markdown content by heading boundaries.
func SplitMarkdownSections(content string) []string {
	lines := strings.Split(content, "\n")
	var sections []string
	var current []string
	for _, line := range lines {
		if (strings.HasPrefix(line, "# ") || strings.HasPrefix(line, "## ")) && len(current) > 0 {
			sections = append(sections, strings.Join(current, "\n"))
			current = nil
		}
		current = append(current, line)
	}
	if len(current) > 0 {
		sections = append(sections, strings.Join(current, "\n"))
	}
	if len(sections) == 0 {
		sections = []string{content}
	}
	return sections
}

// FileWithTime pairs a file path with its mod time.
type FileWithTime struct {
	Path    string
	ModTime time.Time
}

// IsTranscriptCandidate reports whether a file is a viable transcript file.
func IsTranscriptCandidate(path string, info os.FileInfo, projectsDir string) bool {
	if info.IsDir() || filepath.Ext(path) != ".jsonl" {
		return false
	}
	rel, _ := filepath.Rel(projectsDir, path)
	depth := len(filepath.SplitList(rel))
	return depth <= 3 && info.Size() > 100
}

// CollectTranscriptCandidates walks projectsDir and returns viable transcripts.
func CollectTranscriptCandidates(projectsDir string) ([]FileWithTime, error) {
	var candidates []FileWithTime
	err := filepath.Walk(projectsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() && info.Name() == "subagents" {
			return filepath.SkipDir
		}
		if IsTranscriptCandidate(path, info, projectsDir) {
			candidates = append(candidates, FileWithTime{Path: path, ModTime: info.ModTime()})
		}
		return nil
	})
	return candidates, err
}

// TranscriptState holds accumulated state during transcript processing.
type TranscriptState struct {
	Decisions    []string
	Knowledge    []string
	FilesChanged []string
	Issues       []string
	SeenFiles    map[string]bool
	SeenIssues   map[string]bool
	ChatMessages int
}

// NewTranscriptState creates a zero-value TranscriptState with initialized maps.
func NewTranscriptState() *TranscriptState {
	return &TranscriptState{
		SeenFiles:  make(map[string]bool),
		SeenIssues: make(map[string]bool),
	}
}

// IsConversationMessage reports whether a transcript message is user or assistant chat.
func IsConversationMessage(msgType, msgRole string) bool {
	return msgType == "user" || msgType == "assistant" || msgRole == "user" || msgRole == "assistant"
}

// ExtractFilePathsFromTool extracts file paths from a tool call's input parameters
// and appends any new paths to state.
func ExtractFilePathsFromTool(toolInput map[string]any, state *TranscriptState) {
	if toolInput == nil {
		return
	}
	for _, key := range []string{"file_path", "path", "filePath"} {
		if fp, ok := toolInput[key].(string); ok && !state.SeenFiles[fp] {
			state.FilesChanged = append(state.FilesChanged, fp)
			state.SeenFiles[fp] = true
		}
	}
}

// ExtractIssueRefs extracts issue IDs from content and appends new ones to state.
func ExtractIssueRefs(content string, state *TranscriptState) {
	ids := ExtractIssueIDs(content)
	for _, id := range ids {
		if !state.SeenIssues[id] {
			state.Issues = append(state.Issues, id)
			state.SeenIssues[id] = true
		}
	}
}

// FinalizeTranscriptSession populates session summary, decisions, knowledge,
// files, issues, tokens, and session type from the accumulated state.
func FinalizeTranscriptSession(
	summary *string,
	decisions *[]string,
	knowledge *[]string,
	filesChanged *[]string,
	issues *[]string,
	tokenTotal *int,
	tokenEstimated *bool,
	sessionType *string,
	state *TranscriptState,
	date time.Time,
	fileSize int64,
) {
	*summary = GenerateSummary(state.Decisions, state.Knowledge, date)
	*decisions = Dedup(state.Decisions)
	*knowledge = Dedup(state.Knowledge)
	*filesChanged = state.FilesChanged
	*issues = state.Issues
	*tokenTotal = int(fileSize / CharsPerToken)
	*tokenEstimated = true
	*sessionType = DetectSessionTypeFromContent(*summary, state.Knowledge, state.Decisions)
}
