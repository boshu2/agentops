package contextpacket

import (
	"bufio"
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	ReadinessGreen    = "GREEN"
	ReadinessAmber    = "AMBER"
	ReadinessRed      = "RED"
	ReadinessCritical = "CRITICAL"
)

const TranscriptTailMaxBytes int64 = 512 * 1024

var (
	filenameSanitizerRE = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)
	issueIDRE           = regexp.MustCompile(`(?i)\bag-[a-z0-9]+\b`)
)

func RemainingPercent(usagePercent float64) float64 {
	remaining := 1 - usagePercent
	switch {
	case remaining < 0:
		return 0
	case remaining > 1:
		return 1
	default:
		return remaining
	}
}

func ReadinessForUsage(usagePercent float64) string {
	remaining := RemainingPercent(usagePercent)
	switch {
	case remaining >= 0.75:
		return ReadinessGreen
	case remaining >= 0.60:
		return ReadinessAmber
	case remaining >= 0.40:
		return ReadinessRed
	default:
		return ReadinessCritical
	}
}

func ReadinessAction(readiness string) string {
	switch readiness {
	case ReadinessGreen:
		return "carry_on"
	case ReadinessAmber:
		return "finish_current_scope"
	case ReadinessRed:
		return "relief_on_station"
	default:
		return "immediate_relief"
	}
}

func ReadinessRank(readiness string) int {
	switch strings.TrimSpace(readiness) {
	case ReadinessCritical:
		return 0
	case ReadinessRed:
		return 1
	case ReadinessAmber:
		return 2
	case ReadinessGreen:
		return 3
	default:
		return 4
	}
}

func ActionForStatus(status string, stale bool, optimal, critical, warning string) string {
	if stale && status != optimal {
		return "recover_dead_session"
	}
	switch status {
	case critical:
		return "handoff_now"
	case warning:
		return "checkpoint_and_prepare_handoff"
	default:
		if stale {
			return "investigate_stale_session"
		}
		return "continue"
	}
}

func NonZeroOrDefault(value, fallback int) int {
	if value > 0 {
		return value
	}
	return fallback
}

func TruncateDisplay(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}
	return s[:max-3] + "..."
}

func DisplayOrDash(value string) string {
	return cmp.Or(strings.TrimSpace(value), "-")
}

func NormalizeLine(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	return strings.Join(strings.Fields(strings.TrimSpace(s)), " ")
}

func SanitizeForFilename(input string) string {
	return cmp.Or(strings.Trim(filenameSanitizerRE.ReplaceAllString(strings.TrimSpace(input), "-"), "-"), "session")
}

func ToRepoRelative(cwd, fullPath string) string {
	if fullPath == "" {
		return ""
	}
	rel, err := filepath.Rel(cwd, fullPath)
	if err != nil {
		return fullPath
	}
	return filepath.ToSlash(rel)
}

func ExtractIssueID(text string) string {
	m := issueIDRE.FindString(strings.TrimSpace(text))
	if m == "" {
		return ""
	}
	return strings.ToLower(m)
}

func EstimateTokensFromChars(text string, charsPerToken int) int {
	text = strings.TrimSpace(text)
	if text == "" {
		return 0
	}
	if charsPerToken <= 0 {
		charsPerToken = 4
	}
	n := len(text) / charsPerToken
	if n < 1 {
		return 1
	}
	return n
}

func ParseTimestamp(raw string) time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}
	}
	if ts, err := time.Parse(time.RFC3339Nano, raw); err == nil {
		return ts.UTC()
	}
	if ts, err := time.Parse(time.RFC3339, raw); err == nil {
		return ts.UTC()
	}
	return time.Time{}
}

func ExtractTextContent(raw json.RawMessage) string {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return ""
	}
	var plain string
	if err := json.Unmarshal(raw, &plain); err == nil {
		return NormalizeLine(plain)
	}
	var arr []map[string]any
	if err := json.Unmarshal(raw, &arr); err != nil {
		return ""
	}
	for _, item := range arr {
		txt, ok := item["text"].(string)
		if ok && strings.TrimSpace(txt) != "" {
			return NormalizeLine(txt)
		}
	}
	return ""
}

func ScanTailLines(data []byte) ([]string, error) {
	lines := make([]string, 0, 2048)
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}

func ReadFileTail(path string, maxBytes int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}
	size := fi.Size()
	if size == 0 {
		return []byte{}, nil
	}
	return SeekAndReadTail(f, size, maxBytes)
}

func SeekAndReadTail(f *os.File, size, maxBytes int64) ([]byte, error) {
	start := int64(0)
	if size > maxBytes {
		start = size - maxBytes
	}
	if _, err := f.Seek(start, io.SeekStart); err != nil {
		return nil, err
	}
	data, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}
	if start > 0 {
		if idx := bytes.IndexByte(data, '\n'); idx >= 0 && idx+1 < len(data) {
			data = data[idx+1:]
		}
	}
	return data, nil
}

func TmuxTargetFromPaneID(paneID string) string {
	paneID = strings.TrimSpace(paneID)
	if paneID == "" || paneID == "in-process" {
		return ""
	}
	if idx := strings.LastIndex(paneID, "."); idx > 0 {
		return paneID[:idx]
	}
	return paneID
}

func TmuxSessionFromTarget(target string) string {
	target = strings.TrimSpace(target)
	if target == "" {
		return ""
	}
	if idx := strings.Index(target, ":"); idx > 0 {
		return strings.TrimSpace(target[:idx])
	}
	return target
}

func TmuxTargetAlive(target string) bool {
	target = strings.TrimSpace(target)
	if target == "" {
		return false
	}
	ctx, cancel := WithTimeout(1200 * time.Millisecond)
	defer cancel()
	cmd := exec.CommandContext(ctx, "tmux", "has-session", "-t", target)
	return cmd.Run() == nil
}

func TmuxStartDetachedSession(sessionName string) error {
	sessionName = strings.TrimSpace(sessionName)
	if sessionName == "" {
		return errors.New("missing tmux session name")
	}
	ctx, cancel := WithTimeout(1200 * time.Millisecond)
	defer cancel()
	cmd := exec.CommandContext(ctx, "tmux", "new-session", "-d", "-s", sessionName)
	out, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}
	if strings.TrimSpace(string(out)) != "" {
		return errors.New(NormalizeLine(string(out)))
	}
	return err
}

func RunCommand(cwd string, timeout time.Duration, name string, args ...string) string {
	ctx, cancel := WithTimeout(timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = cwd
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func WithTimeout(timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		return context.WithCancel(context.Background())
	}
	return context.WithTimeout(context.Background(), timeout)
}

func GitChangedFiles(cwd string, limit int) []string {
	out := RunCommand(cwd, 1200*time.Millisecond, "git", "diff", "--name-only", "HEAD")
	if strings.TrimSpace(out) == "" {
		return nil
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) > limit {
		lines = lines[:limit]
	}
	trimmed := make([]string, 0, len(lines))
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l != "" {
			trimmed = append(trimmed, l)
		}
	}
	return trimmed
}

func InferAgentRole(agentName, explicitRole string) string {
	if strings.TrimSpace(explicitRole) != "" {
		return strings.TrimSpace(explicitRole)
	}
	agentName = strings.ToLower(strings.TrimSpace(agentName))
	switch {
	case agentName == "":
		return ""
	case strings.Contains(agentName, "admiral"),
		strings.Contains(agentName, "captain"),
		strings.Contains(agentName, "coordinator"),
		strings.Contains(agentName, "orchestrator"),
		strings.Contains(agentName, "quarterback"),
		strings.Contains(agentName, "mayor"),
		strings.Contains(agentName, "leader"),
		strings.Contains(agentName, "lead"):
		return "team-lead"
	case strings.Contains(agentName, "red-cell"),
		strings.Contains(agentName, "navigator"),
		strings.Contains(agentName, "judge"),
		strings.Contains(agentName, "reviewer"):
		return "review"
	case strings.Contains(agentName, "worker"),
		strings.Contains(agentName, "crew"),
		strings.Contains(agentName, "mate"):
		return "worker"
	default:
		return "agent"
	}
}

type HandoffInputs struct {
	Now                     time.Time
	SessionID               string
	Status                  string
	UsagePercent            float64
	Readiness               string
	RemainingPercent        float64
	Action                  string
	LastTask                string
	ActiveBead              string
	AgentName               string
	AgentRole               string
	TeamName                string
	IssueID                 string
	TmuxTarget              string
	ChangedFiles            []string
	IsStale                 bool
	Model                   string
	InputTokens             int
	CacheCreationInputToken int
	CacheReadInputToken     int
	EstimatedUsage          int
	MaxTokens               int
	Recommendation          string
}

func RenderHandoffMarkdown(in HandoffInputs) string {
	hull := cmp.Or(strings.TrimSpace(in.Readiness), ReadinessForUsage(in.UsagePercent))
	remaining := in.RemainingPercent
	if remaining <= 0 && in.UsagePercent > 0 {
		remaining = RemainingPercent(in.UsagePercent)
	}
	var b strings.Builder
	b.WriteString("# Auto-Handoff (Context Guard)\n\n")
	fmt.Fprintf(&b, "**Timestamp:** %s\n", in.Now.Format(time.RFC3339))
	fmt.Fprintf(&b, "**Session:** %s\n", in.SessionID)
	fmt.Fprintf(&b, "**Status:** %s (%.1f%%)\n", in.Status, in.UsagePercent*100)
	fmt.Fprintf(&b, "**Hull:** %s (%.1f%% remaining)\n", hull, remaining*100)
	fmt.Fprintf(&b, "**Action:** %s\n\n", in.Action)
	b.WriteString("## Last Task\n")
	b.WriteString(in.LastTask)
	b.WriteString("\n\n## Active Work\n")
	b.WriteString(in.ActiveBead)
	b.WriteString("\n\n## Assignment\n")
	fmt.Fprintf(&b, "- agent: %s\n", DisplayOrDash(in.AgentName))
	fmt.Fprintf(&b, "- role: %s\n", DisplayOrDash(in.AgentRole))
	fmt.Fprintf(&b, "- team: %s\n", DisplayOrDash(in.TeamName))
	fmt.Fprintf(&b, "- issue: %s\n", DisplayOrDash(in.IssueID))
	fmt.Fprintf(&b, "- tmux target: %s\n\n", DisplayOrDash(in.TmuxTarget))
	b.WriteString("## Next Action\nStart a fresh session, consume this handoff at startup, and continue from the listed task.\n\n")
	b.WriteString("## Modified Files\n")
	if len(in.ChangedFiles) == 0 {
		b.WriteString("none\n\n")
	} else {
		for _, f := range in.ChangedFiles {
			b.WriteString("- ")
			b.WriteString(f)
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}
	b.WriteString("## Blockers\n")
	if in.IsStale {
		b.WriteString("- Session appears stale; watchdog recovery recommended.\n\n")
	} else {
		b.WriteString("none detected\n\n")
	}
	b.WriteString("## Telemetry\n")
	fmt.Fprintf(&b, "- model: %s\n", in.Model)
	fmt.Fprintf(&b, "- input_tokens: %d\n", in.InputTokens)
	fmt.Fprintf(&b, "- cache_creation_input_tokens: %d\n", in.CacheCreationInputToken)
	fmt.Fprintf(&b, "- cache_read_input_tokens: %d\n", in.CacheReadInputToken)
	fmt.Fprintf(&b, "- estimated_usage: %d/%d\n", in.EstimatedUsage, in.MaxTokens)
	fmt.Fprintf(&b, "- recommendation: %s\n", in.Recommendation)
	return b.String()
}

type TeamConfigMember struct {
	Name      string `json:"name"`
	AgentType string `json:"agentType"`
	TmuxPane  string `json:"tmuxPaneId"`
}

type teamConfigFileInternal struct {
	Members []TeamConfigMember `json:"members"`
}

func FindTeamMemberByName(agentName string) (string, TeamConfigMember, bool) {
	agentName = strings.TrimSpace(agentName)
	if agentName == "" {
		return "", TeamConfigMember{}, false
	}
	homeDir := strings.TrimSpace(os.Getenv("HOME"))
	if homeDir == "" {
		return "", TeamConfigMember{}, false
	}
	teamsDir := filepath.Join(homeDir, ".claude", "teams")
	entries, err := os.ReadDir(teamsDir)
	if err != nil {
		return "", TeamConfigMember{}, false
	}
	sortDirEntries(entries)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if member, ok := SearchTeamConfigFile(filepath.Join(teamsDir, entry.Name(), "config.json"), agentName); ok {
			return entry.Name(), member, true
		}
	}
	return "", TeamConfigMember{}, false
}

func sortDirEntries(entries []os.DirEntry) {
	for i := 1; i < len(entries); i++ {
		for j := i; j > 0 && entries[j-1].Name() > entries[j].Name(); j-- {
			entries[j-1], entries[j] = entries[j], entries[j-1]
		}
	}
}

func SearchTeamConfigFile(cfgPath, agentName string) (TeamConfigMember, bool) {
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return TeamConfigMember{}, false
	}
	var config teamConfigFileInternal
	if err := json.Unmarshal(data, &config); err != nil {
		return TeamConfigMember{}, false
	}
	for _, member := range config.Members {
		if strings.EqualFold(strings.TrimSpace(member.Name), agentName) {
			return member, true
		}
	}
	return TeamConfigMember{}, false
}

type TranscriptUsage struct {
	InputTokens             int
	CacheCreationInputToken int
	CacheReadInputToken     int
	Model                   string
	Timestamp               time.Time
}

type TailLineEnvelope struct {
	Type      string `json:"type"`
	Timestamp string `json:"timestamp"`
	Message   struct {
		Role    string          `json:"role"`
		Model   string          `json:"model"`
		Usage   json.RawMessage `json:"usage"`
		Content json.RawMessage `json:"content"`
	} `json:"message"`
}

type tailUsageEnvelope struct {
	InputTokens             int `json:"input_tokens"`
	CacheCreationInputToken int `json:"cache_creation_input_tokens"`
	CacheReadInputToken     int `json:"cache_read_input_tokens"`
}

func ReadSessionTail(path string) (TranscriptUsage, string, time.Time, error) {
	tail, err := ReadFileTail(path, TranscriptTailMaxBytes)
	if err != nil {
		return TranscriptUsage{}, "", time.Time{}, err
	}
	lines, err := ScanTailLines(tail)
	if err != nil {
		return TranscriptUsage{}, "", time.Time{}, err
	}
	usage, lastTask, newestTS := ExtractTailUsageAndTask(lines)
	FixupTailTimestamps(path, &usage, &newestTS)
	return usage, lastTask, newestTS, nil
}

func ExtractTailUsageAndTask(lines []string) (TranscriptUsage, string, time.Time) {
	var usage TranscriptUsage
	var lastTask string
	var newestTS time.Time
	for i := len(lines) - 1; i >= 0; i-- {
		raw := strings.TrimSpace(lines[i])
		if raw == "" {
			continue
		}
		var entry TailLineEnvelope
		if err := json.Unmarshal([]byte(raw), &entry); err != nil {
			continue
		}
		ts := ParseTimestamp(entry.Timestamp)
		if UpdateTailState(entry, ts, &usage, &lastTask, &newestTS) {
			break
		}
	}
	return usage, lastTask, newestTS
}

func FixupTailTimestamps(path string, usage *TranscriptUsage, newestTS *time.Time) {
	if newestTS.IsZero() {
		if fi, err := os.Stat(path); err == nil {
			*newestTS = fi.ModTime().UTC()
		}
	}
	if usage.Timestamp.IsZero() {
		usage.Timestamp = *newestTS
	}
}

func ExtractUsageFromTailEntry(entry TailLineEnvelope, ts time.Time) TranscriptUsage {
	if len(entry.Message.Usage) == 0 {
		return TranscriptUsage{}
	}
	var u tailUsageEnvelope
	if err := json.Unmarshal(entry.Message.Usage, &u); err != nil {
		return TranscriptUsage{}
	}
	total := u.InputTokens + u.CacheCreationInputToken + u.CacheReadInputToken
	if total == 0 {
		return TranscriptUsage{}
	}
	return TranscriptUsage{
		InputTokens:             u.InputTokens,
		CacheCreationInputToken: u.CacheCreationInputToken,
		CacheReadInputToken:     u.CacheReadInputToken,
		Model:                   entry.Message.Model,
		Timestamp:               ts,
	}
}

func ExtractTaskFromTailEntry(entry TailLineEnvelope) string {
	if entry.Type != "user" || len(entry.Message.Content) == 0 {
		return ""
	}
	return ExtractTextContent(entry.Message.Content)
}

func UpdateTailState(entry TailLineEnvelope, ts time.Time, usage *TranscriptUsage, lastTask *string, newestTS *time.Time) bool {
	if newestTS.IsZero() && !ts.IsZero() {
		*newestTS = ts
	}
	if usage.Timestamp.IsZero() {
		*usage = ExtractUsageFromTailEntry(entry, ts)
	}
	if *lastTask == "" {
		*lastTask = ExtractTaskFromTailEntry(entry)
	}
	return !usage.Timestamp.IsZero() && *lastTask != ""
}

// HandoffMarker mirrors the .agents/handoff/pending/*.json schema.
type HandoffMarker struct {
	SchemaVersion int     `json:"schema_version"`
	ID            string  `json:"id"`
	CreatedAt     string  `json:"created_at"`
	SessionID     string  `json:"session_id"`
	Status        string  `json:"status"`
	UsagePercent  float64 `json:"usage_percent"`
	HandoffFile   string  `json:"handoff_file"`
	Consumed      bool    `json:"consumed"`
	ConsumedAt    string  `json:"consumed_at,omitempty"`
}

// FindPendingHandoffForSession scans .agents/handoff/pending for an unconsumed
// marker matching sessionID, returning its handoff path and marker path.
func FindPendingHandoffForSession(cwd, sessionID string) (string, string, bool, error) {
	pendingDir := filepath.Join(cwd, ".agents", "handoff", "pending")
	entries, err := os.ReadDir(pendingDir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", "", false, nil
		}
		return "", "", false, err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		if hp, mp, ok := MatchPendingHandoff(filepath.Join(pendingDir, e.Name()), cwd, sessionID); ok {
			return hp, mp, true, nil
		}
	}
	return "", "", false, nil
}

// MatchPendingHandoff returns the handoff and marker paths if path is an
// unconsumed marker for sessionID.
func MatchPendingHandoff(path, cwd, sessionID string) (string, string, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", "", false
	}
	var marker HandoffMarker
	if err := json.Unmarshal(data, &marker); err != nil {
		return "", "", false
	}
	if marker.SessionID != sessionID || marker.Consumed {
		return "", "", false
	}
	return marker.HandoffFile, ToRepoRelative(cwd, path), true
}
