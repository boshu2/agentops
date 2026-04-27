package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"
)

const (
	runtimeKindClaude   = "claude"
	runtimeKindCodex    = "codex"
	runtimeKindOpenCode = "opencode"
	runtimeKindUnknown  = "unknown"

	lifecycleModeHookCapable   = "hook-capable"
	lifecycleModeCodexHookless = "codex-hookless-fallback"
	lifecycleModeManual        = "manual"

	codexNativeHooksMinVersion = "0.115.0"
)

var codexArchivedSessionPattern = regexp.MustCompile(`([0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12})\.jsonl$`)

type lifecycleRuntimeProfile struct {
	Runtime          string `json:"runtime"`
	Mode             string `json:"mode"`
	HookCapable      bool   `json:"hook_capable"`
	HookConfigured   bool   `json:"hook_configured"`
	HookManifestPath string `json:"hook_manifest_path,omitempty"`
	SessionID        string `json:"session_id,omitempty"`
	ThreadName       string `json:"thread_name,omitempty"`
	Reason           string `json:"reason,omitempty"`
}

type codexHistoryEntry struct {
	SessionID string `json:"session_id"`
	TS        int64  `json:"ts"`
	Text      string `json:"text"`
}

type codexSessionIndexEntry struct {
	ID         string `json:"id"`
	ThreadName string `json:"thread_name"`
	UpdatedAt  string `json:"updated_at"`
}

type runtimeTranscriptCandidate struct {
	path    string
	modTime time.Time
}

func detectLifecycleRuntimeProfile() lifecycleRuntimeProfile {
	return detectLifecycleRuntimeProfileWithOptions(false)
}

func detectCodexLifecycleProfile() lifecycleRuntimeProfile {
	return detectLifecycleRuntimeProfileWithOptions(true)
}

func detectLifecycleRuntimeProfileWithOptions(forceCodex bool) lifecycleRuntimeProfile {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return lifecycleRuntimeProfile{
			Runtime: runtimeKindUnknown,
			Mode:    lifecycleModeManual,
			Reason:  fmt.Sprintf("cannot resolve home directory: %v", err),
		}
	}

	claudeManifest := filepath.Join(homeDir, ".agentops", "hooks.json")
	legacyClaudeManifest := filepath.Join(homeDir, ".claude", "hooks.json")
	codexConfig := filepath.Join(homeDir, ".codex", "config.toml")
	codexManifest := filepath.Join(homeDir, ".codex", "hooks.json")
	openCodeManifest := filepath.Join(homeDir, ".config", "opencode", "agentops", "hooks", "hooks.json")

	runtimeKind := detectRuntimeKind(forceCodex)
	profile := lifecycleRuntimeProfile{
		Runtime: runtimeKind,
		Mode:    lifecycleModeManual,
	}

	switch runtimeKind {
	case runtimeKindCodex:
		profile.Mode = lifecycleModeCodexHookless
		profile.SessionID = resolveCodexSessionID(homeDir)
		if entry, err := readCodexSessionIndexEntry(homeDir, profile.SessionID); err == nil && entry != nil {
			profile.ThreadName = strings.TrimSpace(entry.ThreadName)
		}
		profile.HookCapable = codexSupportsNativeHooks(homeDir)
		if profile.HookCapable {
			manifestConfigured, manifestReason := codexHooksManifestConfigured(codexManifest)
			featureEnabled := codexHooksFeatureEnabled(codexConfig)
			switch {
			case featureEnabled && manifestConfigured:
				profile.Mode = lifecycleModeHookCapable
				profile.HookConfigured = true
				profile.HookManifestPath = codexManifest
				profile.Reason = "Codex native hooks are configured via ~/.codex/hooks.json."
			case manifestConfigured && !featureEnabled:
				profile.Reason = "Codex native hooks are installed, but [features].codex_hooks is disabled in ~/.codex/config.toml; use ao codex start/stop until the feature flag is re-enabled."
			case featureEnabled && !manifestConfigured:
				profile.Reason = manifestReason
			default:
				profile.Reason = "Codex native hooks are supported but not configured; install them or use ao codex start/stop for explicit lifecycle handling."
			}
		} else {
			profile.Reason = "Detected Codex runtime without native hook support; use ao codex start/stop for explicit lifecycle handling."
		}
	case runtimeKindClaude:
		profile.HookCapable = true
		profile.SessionID = canonicalSessionID(strings.TrimSpace(os.Getenv("CLAUDE_SESSION_ID")))
		for _, candidate := range []string{claudeManifest, legacyClaudeManifest} {
			if fileExists(candidate) {
				profile.Mode = lifecycleModeHookCapable
				profile.HookConfigured = true
				profile.HookManifestPath = candidate
				break
			}
		}
		if !profile.HookConfigured {
			profile.Reason = "Claude runtime supports hooks, but no installed hook manifest was found."
		}
	case runtimeKindOpenCode:
		profile.HookCapable = true
		profile.SessionID = canonicalSessionID(strings.TrimSpace(os.Getenv("OPENCODE_SESSION_ID")))
		if fileExists(openCodeManifest) {
			profile.Mode = lifecycleModeHookCapable
			profile.HookConfigured = true
			profile.HookManifestPath = openCodeManifest
		} else {
			profile.Reason = "OpenCode runtime detected without an installed AgentOps hook manifest."
		}
	default:
		for _, candidate := range []string{openCodeManifest, claudeManifest, legacyClaudeManifest} {
			if fileExists(candidate) {
				profile.Mode = lifecycleModeHookCapable
				profile.HookCapable = true
				profile.HookConfigured = true
				profile.HookManifestPath = candidate
				profile.Reason = "Installed hook manifest detected; runtime can use hook-driven lifecycle."
				return profile
			}
		}
		profile.Reason = "No active runtime hook manifest detected."
	}

	return profile
}

func codexSupportsNativeHooks(homeDir string) bool {
	if version, ok := readCodexLatestVersion(homeDir); ok {
		return compareSemver(version, codexNativeHooksMinVersion) >= 0
	}
	return codexHooksFeatureEnabled(filepath.Join(homeDir, ".codex", "config.toml")) ||
		fileExists(filepath.Join(homeDir, ".codex", "hooks.json"))
}

func readCodexLatestVersion(homeDir string) (string, bool) {
	path := filepath.Join(homeDir, ".codex", "version.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}

	var payload struct {
		LatestVersion string `json:"latest_version"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return "", false
	}

	version := normalizeSemver(payload.LatestVersion)
	if version == "" {
		return "", false
	}
	return version, true
}

func codexHooksFeatureEnabled(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}

	inFeatures := false
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.HasPrefix(trimmed, "[") {
			inFeatures = trimmed == "[features]"
			continue
		}
		if inFeatures && strings.HasPrefix(trimmed, "codex_hooks") {
			parts := strings.SplitN(trimmed, "=", 2)
			if len(parts) != 2 {
				return false
			}
			return strings.EqualFold(strings.TrimSpace(parts[1]), "true")
		}
	}

	return false
}

func codexHooksManifestConfigured(path string) (bool, string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, "Codex native hooks are supported, but ~/.codex/hooks.json is missing; use ao codex start/stop until hooks are installed."
	}

	var payload struct {
		Hooks json.RawMessage `json:"hooks"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return false, fmt.Sprintf("~/.codex/hooks.json is invalid JSON: %v", err)
	}
	if len(payload.Hooks) == 0 {
		return false, "~/.codex/hooks.json is missing the hooks event map."
	}

	var hooksMap map[string]json.RawMessage
	if err := json.Unmarshal(payload.Hooks, &hooksMap); err != nil {
		return false, "~/.codex/hooks.json uses an unsupported hooks shape; expected an event map under hooks."
	}
	if len(hooksMap) == 0 {
		return false, "~/.codex/hooks.json has an empty hooks event map."
	}

	return true, ""
}

func normalizeSemver(raw string) string {
	version := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(raw), "v"))
	if version == "" {
		return ""
	}
	version = strings.SplitN(version, "-", 2)[0]
	if version == "" {
		return ""
	}
	return version
}

func detectRuntimeKind(forceCodex bool) string {
	if forceCodex {
		return runtimeKindCodex
	}
	if strings.TrimSpace(os.Getenv("CODEX_THREAD_ID")) != "" ||
		strings.Contains(strings.ToLower(os.Getenv("CODEX_INTERNAL_ORIGINATOR_OVERRIDE")), "codex") ||
		strings.TrimSpace(os.Getenv("CODEX_CI")) != "" {
		return runtimeKindCodex
	}
	if strings.TrimSpace(os.Getenv("CLAUDE_SESSION_ID")) != "" {
		return runtimeKindClaude
	}
	if strings.TrimSpace(os.Getenv("OPENCODE_SESSION_ID")) != "" {
		return runtimeKindOpenCode
	}
	return runtimeKindUnknown
}

func resolveCodexSessionID(homeDir string) string {
	if sessionID := strings.TrimSpace(os.Getenv("CODEX_THREAD_ID")); sessionID != "" {
		return sessionID
	}
	if latest, err := readLatestCodexSessionIndexEntry(homeDir); err == nil && latest != nil {
		return strings.TrimSpace(latest.ID)
	}
	entries, err := readAllCodexHistoryEntries(homeDir)
	if err != nil || len(entries) == 0 {
		return ""
	}
	return strings.TrimSpace(entries[len(entries)-1].SessionID)
}

func readLatestCodexSessionIndexEntry(homeDir string) (*codexSessionIndexEntry, error) {
	path := filepath.Join(homeDir, ".codex", "session_index.jsonl")
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	var latest *codexSessionIndexEntry
	var latestAt time.Time
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 128*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var entry codexSessionIndexEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}
		when, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(entry.UpdatedAt))
		if err != nil {
			continue
		}
		if latest == nil || when.After(latestAt) {
			copy := entry
			latest = &copy
			latestAt = when
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return latest, nil
}

func readCodexSessionIndexEntry(homeDir, sessionID string) (*codexSessionIndexEntry, error) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil, nil
	}
	path := filepath.Join(homeDir, ".codex", "session_index.jsonl")
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 128*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var entry codexSessionIndexEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}
		if strings.TrimSpace(entry.ID) == sessionID {
			copy := entry
			return &copy, nil
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return nil, nil
}

func readAllCodexHistoryEntries(homeDir string) ([]codexHistoryEntry, error) {
	path := filepath.Join(homeDir, ".codex", "history.jsonl")
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	var entries []codexHistoryEntry
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 128*1024), 2*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var entry codexHistoryEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}
		entries = append(entries, entry)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return entries, nil
}

func readCodexHistoryEntries(homeDir, sessionID string) ([]codexHistoryEntry, error) {
	all, err := readAllCodexHistoryEntries(homeDir)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(sessionID) == "" {
		return all, nil
	}
	var filtered []codexHistoryEntry
	for _, entry := range all {
		if strings.TrimSpace(entry.SessionID) == sessionID {
			filtered = append(filtered, entry)
		}
	}
	return filtered, nil
}

func compactCodexHistoryEntries(entries []codexHistoryEntry) []codexHistoryEntry {
	slices.SortFunc(entries, func(a, b codexHistoryEntry) int {
		switch {
		case a.TS < b.TS:
			return -1
		case a.TS > b.TS:
			return 1
		default:
			return 0
		}
	})

	result := make([]codexHistoryEntry, 0, len(entries))
	lastText := ""
	for _, entry := range entries {
		text := strings.TrimSpace(entry.Text)
		if shouldSkipCodexHistoryText(text) {
			continue
		}
		if len(result) > 0 && text == lastText {
			continue
		}
		entry.Text = text
		result = append(result, entry)
		lastText = text
	}
	return result
}

func shouldSkipCodexHistoryText(text string) bool {
	if text == "" {
		return true
	}
	lower := strings.ToLower(strings.TrimSpace(text))
	return lower == "exit" || strings.HasPrefix(lower, "/model ")
}

func synthesizeCodexHistoryTranscript(cwd, sessionID string) (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home directory: %w", err)
	}

	entries, err := readCodexHistoryEntries(homeDir, sessionID)
	if err != nil {
		return "", fmt.Errorf("read codex history: %w", err)
	}
	entries = compactCodexHistoryEntries(entries)
	if len(entries) == 0 {
		return "", fmt.Errorf("no codex history entries found for session %s", sessionID)
	}

	indexEntry, _ := readCodexSessionIndexEntry(homeDir, sessionID)

	transcriptDir := filepath.Join(cwd, ".agents", "ao", "codex", "transcripts")
	if err := os.MkdirAll(transcriptDir, 0o750); err != nil {
		return "", fmt.Errorf("create codex transcript dir: %w", err)
	}
	filename := fmt.Sprintf("history-%s.jsonl", sanitizePathComponent(sessionID))
	path := filepath.Join(transcriptDir, filename)

	lines := make([]string, 0, len(entries)+1)

	metaPayload := map[string]any{
		"id":        sessionID,
		"timestamp": time.Unix(entries[0].TS, 0).UTC().Format(time.RFC3339),
		"source": map[string]any{
			"history_fallback": true,
		},
	}
	if indexEntry != nil && strings.TrimSpace(indexEntry.ThreadName) != "" {
		metaPayload["thread_name"] = strings.TrimSpace(indexEntry.ThreadName)
	}
	metaLine, err := json.Marshal(map[string]any{
		"timestamp": time.Unix(entries[0].TS, 0).UTC().Format(time.RFC3339),
		"type":      "session_meta",
		"payload":   metaPayload,
	})
	if err != nil {
		return "", fmt.Errorf("marshal codex session meta: %w", err)
	}
	lines = append(lines, string(metaLine))

	for _, entry := range entries {
		line, err := json.Marshal(map[string]any{
			"timestamp": time.Unix(entry.TS, 0).UTC().Format(time.RFC3339),
			"type":      "event_msg",
			"payload": map[string]any{
				"type":    "user_message",
				"message": entry.Text,
			},
		})
		if err != nil {
			return "", fmt.Errorf("marshal codex history event: %w", err)
		}
		lines = append(lines, string(line))
	}

	content := strings.Join(lines, "\n") + "\n"
	if err := atomicWriteFile(path, []byte(content), 0o600); err != nil {
		return "", fmt.Errorf("write synthetic codex transcript: %w", err)
	}
	return path, nil
}

func findTranscriptBySessionID(sessionID string) (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home directory: %w", err)
	}

	aliases := sessionIDAliases(sessionID)
	for _, alias := range aliases {
		if alias == "" {
			continue
		}

		conversationsPattern := filepath.Join(homeDir, ".claude", "projects", "*", "conversations", alias+".jsonl")
		if matches, err := filepath.Glob(conversationsPattern); err == nil && len(matches) > 0 {
			return matches[0], nil
		}

		directPattern := filepath.Join(homeDir, ".claude", "projects", "*", alias+".jsonl")
		if matches, err := filepath.Glob(directPattern); err == nil && len(matches) > 0 {
			return matches[0], nil
		}

		codexPattern := filepath.Join(homeDir, ".codex", "archived_sessions", "*"+alias+"*.jsonl")
		if matches, err := filepath.Glob(codexPattern); err == nil && len(matches) > 0 {
			slices.Sort(matches)
			return matches[len(matches)-1], nil
		}
	}

	return "", fmt.Errorf("no transcript found for session %s", sessionID)
}

func findLastCodexArchivedTranscript() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home directory: %w", err)
	}
	return findLastTranscriptInDir(filepath.Join(homeDir, ".codex", "archived_sessions"), "no Codex archived sessions found")
}

func findLastSession() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home directory: %w", err)
	}

	var candidates []runtimeTranscriptCandidate

	if claudeCandidates, err := collectClaudeTranscriptCandidates(filepath.Join(homeDir, ".claude", "projects")); err == nil {
		candidates = append(candidates, claudeCandidates...)
	}
	if codexCandidates, err := collectFlatJSONLCandidates(filepath.Join(homeDir, ".codex", "archived_sessions")); err == nil {
		candidates = append(candidates, codexCandidates...)
	}

	if len(candidates) == 0 {
		return "", fmt.Errorf("no transcript files found in Claude or Codex runtime directories")
	}

	slices.SortFunc(candidates, func(a, b runtimeTranscriptCandidate) int {
		return b.modTime.Compare(a.modTime)
	})

	return candidates[0].path, nil
}

func collectClaudeTranscriptCandidates(projectsDir string) ([]runtimeTranscriptCandidate, error) {
	if !fileExists(projectsDir) {
		return nil, nil
	}

	var candidates []runtimeTranscriptCandidate
	err := filepath.Walk(projectsDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if info.IsDir() {
			if info.Name() == "subagents" {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".jsonl" || info.Size() <= 100 {
			return nil
		}
		candidates = append(candidates, runtimeTranscriptCandidate{
			path:    path,
			modTime: info.ModTime(),
		})
		return nil
	})
	return candidates, err
}

func collectFlatJSONLCandidates(dir string) ([]runtimeTranscriptCandidate, error) {
	if !fileExists(dir) {
		return nil, nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var candidates []runtimeTranscriptCandidate
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".jsonl" {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		candidates = append(candidates, runtimeTranscriptCandidate{
			path:    filepath.Join(dir, entry.Name()),
			modTime: info.ModTime(),
		})
	}
	return candidates, nil
}

func findLastTranscriptInDir(dir, emptyMessage string) (string, error) {
	candidates, err := collectFlatJSONLCandidates(dir)
	if err != nil {
		return "", err
	}
	if len(candidates) == 0 {
		return "", fmt.Errorf("%s", emptyMessage)
	}
	slices.SortFunc(candidates, func(a, b runtimeTranscriptCandidate) int {
		return b.modTime.Compare(a.modTime)
	})
	return candidates[0].path, nil
}
