package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDetectCodexLifecycleProfile_Hookless(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CODEX_THREAD_ID", "019d1bf7-58ea-79e1-9f5d-02109d930081")
	t.Setenv("CODEX_INTERNAL_ORIGINATOR_OVERRIDE", "Codex Desktop")

	indexPath := filepath.Join(home, ".codex", "session_index.jsonl")
	if err := os.MkdirAll(filepath.Dir(indexPath), 0o755); err != nil {
		t.Fatal(err)
	}
	line := `{"id":"019d1bf7-58ea-79e1-9f5d-02109d930081","thread_name":"Codex lifecycle fallback","updated_at":"2026-03-23T12:00:00Z"}`
	if err := os.WriteFile(indexPath, []byte(line+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	profile := detectCodexLifecycleProfile()
	if profile.Runtime != runtimeKindCodex {
		t.Fatalf("runtime = %q, want %q", profile.Runtime, runtimeKindCodex)
	}
	if profile.Mode != lifecycleModeCodexHookless {
		t.Fatalf("mode = %q, want %q", profile.Mode, lifecycleModeCodexHookless)
	}
	if profile.HookCapable {
		t.Fatal("hook_capable = true, want false for Codex")
	}
	if profile.ThreadName != "Codex lifecycle fallback" {
		t.Fatalf("thread_name = %q, want %q", profile.ThreadName, "Codex lifecycle fallback")
	}
}

func TestFindLastSession_PrefersNewestCodexArchivedTranscript(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	claudePath := filepath.Join(home, ".claude", "projects", "repo", "conversations", "claude.jsonl")
	if err := os.MkdirAll(filepath.Dir(claudePath), 0o755); err != nil {
		t.Fatal(err)
	}
	claudeContent := strings.Repeat(`{"type":"user","message":{"content":"hello"}}`+"\n", 4)
	if err := os.WriteFile(claudePath, []byte(claudeContent), 0o644); err != nil {
		t.Fatal(err)
	}
	older := time.Date(2026, 3, 20, 12, 0, 0, 0, time.UTC)
	if err := os.Chtimes(claudePath, older, older); err != nil {
		t.Fatal(err)
	}

	codexPath := filepath.Join(home, ".codex", "archived_sessions", "rollout-2026-03-23T12-00-00-019d1bf7-58ea-79e1-9f5d-02109d930081.jsonl")
	if err := os.MkdirAll(filepath.Dir(codexPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(codexPath, []byte(`{"type":"session_meta","payload":{"id":"019d1bf7-58ea-79e1-9f5d-02109d930081"}}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	newer := older.Add(24 * time.Hour)
	if err := os.Chtimes(codexPath, newer, newer); err != nil {
		t.Fatal(err)
	}

	got, err := findLastSession()
	if err != nil {
		t.Fatalf("findLastSession: %v", err)
	}
	if got != codexPath {
		t.Fatalf("findLastSession = %q, want %q", got, codexPath)
	}
}

func TestFindTranscriptBySessionID_FindsCodexArchivedTranscript(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	sessionID := "019d1bf7-58ea-79e1-9f5d-02109d930081"
	path := filepath.Join(home, ".codex", "archived_sessions", "rollout-2026-03-23T12-00-00-"+sessionID+".jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"type":"session_meta","payload":{"id":"`+sessionID+`"}}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := findTranscriptBySessionID(sessionID)
	if err != nil {
		t.Fatalf("findTranscriptBySessionID: %v", err)
	}
	if got != path {
		t.Fatalf("findTranscriptBySessionID = %q, want %q", got, path)
	}
}

func TestSynthesizeCodexHistoryTranscript(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	sessionID := "019d1bf7-58ea-79e1-9f5d-02109d930081"
	historyPath := filepath.Join(home, ".codex", "history.jsonl")
	indexPath := filepath.Join(home, ".codex", "session_index.jsonl")
	if err := os.MkdirAll(filepath.Dir(historyPath), 0o755); err != nil {
		t.Fatal(err)
	}

	history := []string{
		`{"session_id":"` + sessionID + `","ts":1766945654,"text":"exit"}`,
		`{"session_id":"` + sessionID + `","ts":1766945655,"text":"Investigate lifecycle fallback"}`,
		`{"session_id":"` + sessionID + `","ts":1766945656,"text":"Investigate lifecycle fallback"}`,
		`{"session_id":"` + sessionID + `","ts":1766945657,"text":"/model default"}`,
		`{"session_id":"` + sessionID + `","ts":1766945658,"text":"Record explicit Codex closeout"}`,
	}
	if err := os.WriteFile(historyPath, []byte(strings.Join(history, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(indexPath, []byte(`{"id":"`+sessionID+`","thread_name":"Lifecycle fallback","updated_at":"2026-03-23T12:00:00Z"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	repo := t.TempDir()
	path, err := synthesizeCodexHistoryTranscript(repo, sessionID)
	if err != nil {
		t.Fatalf("synthesizeCodexHistoryTranscript: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, `"type":"session_meta"`) {
		t.Fatalf("expected session_meta line in %s", content)
	}
	if !strings.Contains(content, `"thread_name":"Lifecycle fallback"`) {
		t.Fatalf("expected thread_name in synthesized transcript: %s", content)
	}
	if strings.Contains(content, `"message":"exit"`) {
		t.Fatalf("control message should be filtered out: %s", content)
	}
	if strings.Count(content, `"type":"event_msg"`) != 2 {
		t.Fatalf("expected 2 event messages after compaction, got %d in %s", strings.Count(content, `"type":"event_msg"`), content)
	}
}

func TestCodexStartJSONWritesStateAndCitations(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CODEX_THREAD_ID", "019d1bf7-58ea-79e1-9f5d-02109d930081")
	t.Setenv("CODEX_INTERNAL_ORIGINATOR_OVERRIDE", "Codex Desktop")

	indexPath := filepath.Join(home, ".codex", "session_index.jsonl")
	if err := os.MkdirAll(filepath.Dir(indexPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(indexPath, []byte(`{"id":"019d1bf7-58ea-79e1-9f5d-02109d930081","thread_name":"explicit lifecycle","updated_at":"2026-03-23T12:00:00Z"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".agents", "learnings"), 0o755); err != nil {
		t.Fatal(err)
	}
	learningPath := filepath.Join(repo, ".agents", "learnings", "codex-lifecycle.md")
	learning := `---
id: codex-lifecycle
type: learning
date: 2026-03-23
source: codex-test
maturity: provisional
utility: 0.9
---

# Explicit Codex lifecycle

Use ao codex start and ao codex stop when runtime hooks are unavailable.
`
	if err := os.WriteFile(learningPath, []byte(learning), 0o644); err != nil {
		t.Fatal(err)
	}

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	out, err := executeCommand("codex", "start", "--json", "--query", "explicit codex lifecycle")
	if err != nil {
		t.Fatalf("codex start --json: %v\noutput: %s", err, out)
	}

	var result codexStartResult
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &result); err != nil {
		t.Fatalf("parse codex start json: %v\noutput: %s", err, out)
	}
	if result.Runtime.Mode != lifecycleModeCodexHookless {
		t.Fatalf("runtime mode = %q, want %q", result.Runtime.Mode, lifecycleModeCodexHookless)
	}
	if result.StartupContextPath == "" || !fileExists(result.StartupContextPath) {
		t.Fatalf("startup context path missing or unreadable: %q", result.StartupContextPath)
	}
	if result.StatePath == "" || !fileExists(result.StatePath) {
		t.Fatalf("state path missing or unreadable: %q", result.StatePath)
	}
	citationsPath := filepath.Join(repo, ".agents", "ao", "citations.jsonl")
	data, err := os.ReadFile(citationsPath)
	if err != nil {
		t.Fatalf("read citations: %v", err)
	}
	if !strings.Contains(string(data), `"citation_type":"retrieved"`) {
		t.Fatalf("expected retrieved citation in %s", string(data))
	}
}

func TestWriteCodexStartupContextUsesRankedSectionsAndPolicy(t *testing.T) {
	repo := t.TempDir()
	for _, rel := range []string{
		filepath.Join(".agents", "briefings"),
		filepath.Join(".agents", "findings"),
		filepath.Join(".agents", "planning-rules"),
		filepath.Join(".agents", "pre-mortem-checks"),
		filepath.Join(".agents", "ao", "codex"),
	} {
		if err := os.MkdirAll(filepath.Join(repo, rel), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	finding := `---
id: "f-startup-002"
title: "Prefer startup packet over recency dump"
status: "active"
severity: "high"
applicable_when: ["task","startup"]
scope_tags: ["startup","context"]
---
# Finding

Prefer startup packet over arbitrary recent artifacts.
`
	if err := os.WriteFile(filepath.Join(repo, ".agents", "findings", "f-startup-002.md"), []byte(finding), 0o644); err != nil {
		t.Fatal(err)
	}

	rule := `---
id: "f-startup-002"
---
# Planning Rule

- Ask: Did startup context use ranked rules before recent documents?
`
	if err := os.WriteFile(filepath.Join(repo, ".agents", "planning-rules", "f-startup-002.md"), []byte(rule), 0o644); err != nil {
		t.Fatal(err)
	}

	check := `---
id: "f-startup-002"
---
# Pre-Mortem Check

- Ask: Did startup context exclude discovery-only notes?
`
	if err := os.WriteFile(filepath.Join(repo, ".agents", "pre-mortem-checks", "f-startup-002.md"), []byte(check), 0o644); err != nil {
		t.Fatal(err)
	}

	profile := lifecycleRuntimeProfile{Runtime: runtimeKindCodex, Mode: lifecycleModeCodexHookless, ThreadName: "startup"}
	path, err := writeCodexStartupContext(
		repo,
		profile,
		"startup packet",
		[]codexArtifactRef{{Title: "Startup briefing", ModifiedAt: "2026-04-01T21:00:00Z"}},
		[]learning{{Title: "Use ranked startup packet", Summary: "Prefer rules and findings over recency."}},
		[]pattern{{Name: "Small startup payloads", Description: "Keep startup context concise and high trust."}},
		[]knowledgeFinding{{ID: "f-startup-002", Title: "Prefer startup packet over recency dump", Summary: "Prefer ranked startup packet."}},
		[]session{{Date: "2026-04-01", Summary: "Reworked startup context."}},
		[]nextWorkItem{{Title: "Wire startup context to ranked packet", Severity: "high", Description: "Replace low-signal startup dump"}},
		[]codexArtifactRef{{Title: "Session intelligence research", ModifiedAt: "2026-04-01T22:00:00Z"}},
	)
	if err != nil {
		t.Fatalf("writeCodexStartupContext: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read startup context: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "## Briefings") {
		t.Fatalf("expected briefings heading, got:\n%s", content)
	}
	if !strings.Contains(content, "Startup briefing") {
		t.Fatalf("expected startup briefing in startup context, got:\n%s", content)
	}
	if !strings.Contains(content, "## Selected Context") {
		t.Fatalf("expected ranked startup heading, got:\n%s", content)
	}
	if !strings.Contains(content, "### Planning Rules") {
		t.Fatalf("expected planning rules in startup context, got:\n%s", content)
	}
	if !strings.Contains(content, "## Excluded By Default") {
		t.Fatalf("expected exclusion policy heading, got:\n%s", content)
	}
	if !strings.Contains(content, "Wire startup context to ranked packet") {
		t.Fatalf("expected ranked next work in startup context, got:\n%s", content)
	}
}

func TestCodexStartJSONSurfacesMatchingBriefings(t *testing.T) {
	normalize := func(path string) string {
		if resolved, err := filepath.EvalSymlinks(path); err == nil && resolved != "" {
			return filepath.Clean(resolved)
		}
		if abs, err := filepath.Abs(path); err == nil {
			return filepath.Clean(abs)
		}
		return filepath.Clean(path)
	}

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CODEX_THREAD_ID", "019d1bf7-58ea-79e1-9f5d-02109d930081")
	t.Setenv("CODEX_INTERNAL_ORIGINATOR_OVERRIDE", "Codex Desktop")

	indexPath := filepath.Join(home, ".codex", "session_index.jsonl")
	if err := os.MkdirAll(filepath.Dir(indexPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(indexPath, []byte(`{"id":"019d1bf7-58ea-79e1-9f5d-02109d930081","thread_name":"knowledge briefing startup","updated_at":"2026-03-23T12:00:00Z"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	repo := t.TempDir()
	briefingsDir := filepath.Join(repo, ".agents", "briefings")
	if err := os.MkdirAll(briefingsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	briefingPath := filepath.Join(briefingsDir, "2026-04-01-fix-auth-startup.md")
	if err := os.WriteFile(briefingPath, []byte("# Briefing: Fix auth startup\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	out, err := executeCommand("codex", "start", "--json", "--no-maintenance", "--query", "fix auth startup")
	if err != nil {
		t.Fatalf("codex start --json: %v\noutput: %s", err, out)
	}

	var result codexStartResult
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &result); err != nil {
		t.Fatalf("parse codex start json: %v\noutput: %s", err, out)
	}
	if len(result.Briefings) != 1 {
		t.Fatalf("briefings = %+v, want exactly one matching briefing", result.Briefings)
	}
	if got, want := normalize(result.Briefings[0].Path), normalize(briefingPath); got != want {
		t.Fatalf("briefing path = %q, want %q", got, want)
	}

	startupContext, err := os.ReadFile(result.StartupContextPath)
	if err != nil {
		t.Fatalf("read startup context: %v", err)
	}
	content := string(startupContext)
	if !strings.Contains(content, "## Briefings") {
		t.Fatalf("startup context missing briefings section:\n%s", content)
	}
	if !strings.Contains(content, "2026-04-01-fix-auth-startup") {
		t.Fatalf("startup context missing matched briefing title:\n%s", content)
	}
}

func TestCodexEnsureStartJSONSkipsDuplicateStartupForSameSession(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CODEX_THREAD_ID", "019d1bf7-58ea-79e1-9f5d-02109d930081")
	t.Setenv("CODEX_INTERNAL_ORIGINATOR_OVERRIDE", "Codex Desktop")

	indexPath := filepath.Join(home, ".codex", "session_index.jsonl")
	if err := os.MkdirAll(filepath.Dir(indexPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(indexPath, []byte(`{"id":"019d1bf7-58ea-79e1-9f5d-02109d930081","thread_name":"explicit lifecycle","updated_at":"2026-03-23T12:00:00Z"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".agents", "learnings"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".agents", "learnings", "codex-lifecycle.md"), []byte(`---
id: codex-lifecycle
type: learning
date: 2026-03-23
source: codex-test
maturity: provisional
utility: 0.9
---

# Explicit Codex lifecycle

Use ao codex ensure-start and ao codex ensure-stop when runtime hooks are unavailable.
`), 0o644); err != nil {
		t.Fatal(err)
	}

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	firstOut, err := executeCommand("codex", "ensure-start", "--json", "--query", "explicit codex lifecycle")
	if err != nil {
		t.Fatalf("first codex ensure-start --json: %v\noutput: %s", err, firstOut)
	}
	var first codexEnsureStartResult
	if err := json.Unmarshal([]byte(strings.TrimSpace(firstOut)), &first); err != nil {
		t.Fatalf("parse first codex ensure-start json: %v\noutput: %s", err, firstOut)
	}
	if !first.Performed {
		t.Fatalf("first ensure-start performed = false, want true: %+v", first)
	}

	statePath := filepath.Join(repo, ".agents", "ao", "codex", "state.json")
	before, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("read state before second ensure-start: %v", err)
	}

	secondOut, err := executeCommand("codex", "ensure-start", "--json", "--query", "explicit codex lifecycle")
	if err != nil {
		t.Fatalf("second codex ensure-start --json: %v\noutput: %s", err, secondOut)
	}
	var second codexEnsureStartResult
	if err := json.Unmarshal([]byte(strings.TrimSpace(secondOut)), &second); err != nil {
		t.Fatalf("parse second codex ensure-start json: %v\noutput: %s", err, secondOut)
	}
	if second.Performed {
		t.Fatalf("second ensure-start performed = true, want false: %+v", second)
	}
	if !strings.Contains(second.Reason, "already recorded") {
		t.Fatalf("second reason = %q, want already-recorded hint", second.Reason)
	}

	after, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("read state after second ensure-start: %v", err)
	}
	if string(after) != string(before) {
		t.Fatalf("expected idempotent ensure-start to leave state unchanged\nbefore:\n%s\nafter:\n%s", string(before), string(after))
	}
}

func TestCodexStopJSONUsesHistoryFallback(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CODEX_THREAD_ID", "019d1bf7-58ea-79e1-9f5d-02109d930081")
	t.Setenv("CODEX_INTERNAL_ORIGINATOR_OVERRIDE", "Codex Desktop")

	sessionID := "019d1bf7-58ea-79e1-9f5d-02109d930081"
	historyPath := filepath.Join(home, ".codex", "history.jsonl")
	indexPath := filepath.Join(home, ".codex", "session_index.jsonl")
	if err := os.MkdirAll(filepath.Dir(historyPath), 0o755); err != nil {
		t.Fatal(err)
	}
	history := []string{
		`{"session_id":"` + sessionID + `","ts":1766945655,"text":"Design Codex fallback lifecycle"}`,
		`{"session_id":"` + sessionID + `","ts":1766945658,"text":"Implement explicit ao codex stop closeout"}`,
	}
	if err := os.WriteFile(historyPath, []byte(strings.Join(history, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(indexPath, []byte(`{"id":"`+sessionID+`","thread_name":"Lifecycle fallback stop","updated_at":"2026-03-23T12:00:00Z"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	repo := t.TempDir()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	out, err := executeCommand("codex", "stop", "--json")
	if err != nil {
		t.Fatalf("codex stop --json: %v\noutput: %s", err, out)
	}

	var result codexStopResult
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &result); err != nil {
		t.Fatalf("parse codex stop json: %v\noutput: %s", err, out)
	}
	if result.TranscriptSource != "history-fallback" {
		t.Fatalf("transcript_source = %q, want history-fallback", result.TranscriptSource)
	}
	if !result.SyntheticTranscript {
		t.Fatal("synthetic_transcript = false, want true")
	}
	if result.Session.SessionID == "" {
		t.Fatal("expected session close result to include a session ID")
	}
	if !fileExists(result.TranscriptPath) {
		t.Fatalf("synthetic transcript path missing: %s", result.TranscriptPath)
	}
}

func TestCodexStopJSONSkipsDuplicateCloseoutForSameSession(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CODEX_THREAD_ID", "019d1bf7-58ea-79e1-9f5d-02109d930081")
	t.Setenv("CODEX_INTERNAL_ORIGINATOR_OVERRIDE", "Codex Desktop")

	sessionID := "019d1bf7-58ea-79e1-9f5d-02109d930081"
	historyPath := filepath.Join(home, ".codex", "history.jsonl")
	indexPath := filepath.Join(home, ".codex", "session_index.jsonl")
	if err := os.MkdirAll(filepath.Dir(historyPath), 0o755); err != nil {
		t.Fatal(err)
	}
	history := []string{
		`{"session_id":"` + sessionID + `","ts":1766945655,"text":"Design Codex fallback lifecycle"}`,
		`{"session_id":"` + sessionID + `","ts":1766945658,"text":"Implement explicit ao codex stop closeout"}`,
	}
	if err := os.WriteFile(historyPath, []byte(strings.Join(history, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(indexPath, []byte(`{"id":"`+sessionID+`","thread_name":"Lifecycle fallback stop","updated_at":"2026-03-23T12:00:00Z"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	repo := t.TempDir()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	firstOut, err := executeCommand("codex", "stop", "--json")
	if err != nil {
		t.Fatalf("first codex stop --json: %v\noutput: %s", err, firstOut)
	}

	var first codexStopResult
	if err := json.Unmarshal([]byte(strings.TrimSpace(firstOut)), &first); err != nil {
		t.Fatalf("parse first codex stop json: %v\noutput: %s", err, firstOut)
	}

	statePath := filepath.Join(repo, ".agents", "ao", "codex", "state.json")
	before, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("read state before second stop: %v", err)
	}

	secondOut, err := executeCommand("codex", "stop", "--json")
	if err != nil {
		t.Fatalf("second codex stop --json: %v\noutput: %s", err, secondOut)
	}

	var second codexStopResult
	if err := json.Unmarshal([]byte(strings.TrimSpace(secondOut)), &second); err != nil {
		t.Fatalf("parse second codex stop json: %v\noutput: %s", err, secondOut)
	}
	if second.Session.Status != "already_closed" {
		t.Fatalf("second status = %q, want already_closed", second.Session.Status)
	}
	if !strings.Contains(second.Session.Message, "already recorded") {
		t.Fatalf("second message = %q, want already-recorded hint", second.Session.Message)
	}
	if second.CloseLoop != nil {
		t.Fatalf("expected no duplicate close-loop on second stop, got %+v", second.CloseLoop)
	}
	if second.TranscriptPath != first.TranscriptPath {
		t.Fatalf("second transcript path = %q, want %q", second.TranscriptPath, first.TranscriptPath)
	}

	after, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("read state after second stop: %v", err)
	}
	if string(after) != string(before) {
		t.Fatalf("expected idempotent codex stop to leave state unchanged\nbefore:\n%s\nafter:\n%s", string(before), string(after))
	}
}

func TestCodexEnsureStopJSONSkipsDuplicateCloseoutForSameSession(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CODEX_THREAD_ID", "019d1bf7-58ea-79e1-9f5d-02109d930081")
	t.Setenv("CODEX_INTERNAL_ORIGINATOR_OVERRIDE", "Codex Desktop")

	sessionID := "019d1bf7-58ea-79e1-9f5d-02109d930081"
	historyPath := filepath.Join(home, ".codex", "history.jsonl")
	indexPath := filepath.Join(home, ".codex", "session_index.jsonl")
	if err := os.MkdirAll(filepath.Dir(historyPath), 0o755); err != nil {
		t.Fatal(err)
	}
	history := []string{
		`{"session_id":"` + sessionID + `","ts":1766945655,"text":"Design Codex fallback lifecycle"}`,
		`{"session_id":"` + sessionID + `","ts":1766945658,"text":"Implement explicit ao codex stop closeout"}`,
	}
	if err := os.WriteFile(historyPath, []byte(strings.Join(history, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(indexPath, []byte(`{"id":"`+sessionID+`","thread_name":"Lifecycle fallback stop","updated_at":"2026-03-23T12:00:00Z"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	repo := t.TempDir()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	firstOut, err := executeCommand("codex", "ensure-stop", "--json")
	if err != nil {
		t.Fatalf("first codex ensure-stop --json: %v\noutput: %s", err, firstOut)
	}
	var first codexEnsureStopResult
	if err := json.Unmarshal([]byte(strings.TrimSpace(firstOut)), &first); err != nil {
		t.Fatalf("parse first codex ensure-stop json: %v\noutput: %s", err, firstOut)
	}
	if !first.Performed {
		t.Fatalf("first ensure-stop performed = false, want true: %+v", first)
	}

	statePath := filepath.Join(repo, ".agents", "ao", "codex", "state.json")
	before, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("read state before second ensure-stop: %v", err)
	}

	secondOut, err := executeCommand("codex", "ensure-stop", "--json")
	if err != nil {
		t.Fatalf("second codex ensure-stop --json: %v\noutput: %s", err, secondOut)
	}
	var second codexEnsureStopResult
	if err := json.Unmarshal([]byte(strings.TrimSpace(secondOut)), &second); err != nil {
		t.Fatalf("parse second codex ensure-stop json: %v\noutput: %s", err, secondOut)
	}
	if second.Performed {
		t.Fatalf("second ensure-stop performed = true, want false: %+v", second)
	}
	if !strings.Contains(second.Reason, "already recorded") {
		t.Fatalf("second reason = %q, want already-recorded hint", second.Reason)
	}

	after, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("read state after second ensure-stop: %v", err)
	}
	if string(after) != string(before) {
		t.Fatalf("expected idempotent ensure-stop to leave state unchanged\nbefore:\n%s\nafter:\n%s", string(before), string(after))
	}
}

func TestCodexStatusJSONReflectsHooklessHealthAndSearchCitations(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CODEX_THREAD_ID", "019d1bf7-58ea-79e1-9f5d-02109d930081")
	t.Setenv("CODEX_INTERNAL_ORIGINATOR_OVERRIDE", "Codex Desktop")

	indexPath := filepath.Join(home, ".codex", "session_index.jsonl")
	if err := os.MkdirAll(filepath.Dir(indexPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(indexPath, []byte(`{"id":"019d1bf7-58ea-79e1-9f5d-02109d930081","thread_name":"Codex lifecycle status","updated_at":"2026-03-23T12:00:00Z"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".agents", "learnings"), 0o755); err != nil {
		t.Fatal(err)
	}
	learningPath := filepath.Join(repo, ".agents", "learnings", "codex-status.md")
	learning := `---
id: codex-status
type: learning
date: 2026-03-23
source: codex-test
maturity: provisional
utility: 0.9
---

# Codex lifecycle status

Use ao codex start, ao search --cite applied, and ao codex stop when runtime hooks are unavailable.
`
	if err := os.WriteFile(learningPath, []byte(learning), 0o644); err != nil {
		t.Fatal(err)
	}
	searchableLearningPath := filepath.Join(repo, ".agents", "learnings", "codex-status.jsonl")
	searchableLearning := `{"summary":"codex lifecycle status from explicit search citation","utility":0.8,"maturity":"provisional"}`
	if err := os.WriteFile(searchableLearningPath, []byte(searchableLearning+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(repo, ".agents", "knowledge", "pending"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".agents", "knowledge", "pending", "queued-learning.md"), []byte("# queued\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(repo, ".agents", "knowledge", "pending", ".quarantine"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".agents", "knowledge", "pending", ".quarantine", "truncated-learning.md"), []byte("# quarantined\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origDir) }()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	if out, err := executeCommand("codex", "start", "--json", "--no-maintenance", "--query", "codex lifecycle status"); err != nil {
		t.Fatalf("codex start --json: %v\noutput: %s", err, out)
	}

	recordSearchCitations(cwd, []searchResult{{Path: filepath.Join(cwd, ".agents", "learnings", "codex-status.jsonl")}}, resolveSessionID(""), "codex lifecycle status", "applied")

	citationsPath := filepath.Join(repo, ".agents", "ao", "citations.jsonl")
	citations, err := os.ReadFile(citationsPath)
	if err != nil {
		t.Fatalf("read citations: %v", err)
	}
	citationsText := string(citations)
	if !strings.Contains(citationsText, `"session_id":"019d1bf7-58ea-79e1-9f5d-02109d930081"`) {
		t.Fatalf("expected Codex session ID in citations: %s", citationsText)
	}
	if !strings.Contains(citationsText, `"citation_type":"applied"`) {
		t.Fatalf("expected applied citation in %s", citationsText)
	}

	out, err := executeCommand("codex", "status", "--json", "--days", "30")
	if err != nil {
		t.Fatalf("codex status --json: %v\noutput: %s", err, out)
	}

	var result codexStatusResult
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &result); err != nil {
		t.Fatalf("parse codex status json: %v\noutput: %s", err, out)
	}
	if result.Runtime.Mode != lifecycleModeCodexHookless {
		t.Fatalf("runtime mode = %q, want %q", result.Runtime.Mode, lifecycleModeCodexHookless)
	}
	if result.Capture.PendingKnowledge != 1 {
		t.Fatalf("pending knowledge = %d, want 1", result.Capture.PendingKnowledge)
	}
	if result.Capture.PendingQuarantine != 1 {
		t.Fatalf("pending quarantine = %d, want 1", result.Capture.PendingQuarantine)
	}
	if result.Citations.Total != 3 {
		t.Fatalf("citation total = %d, want 3", result.Citations.Total)
	}
	if result.Citations.UniqueArtifacts != 2 {
		t.Fatalf("unique artifacts = %d, want 2", result.Citations.UniqueArtifacts)
	}
	if result.Citations.Retrieved != 2 {
		t.Fatalf("retrieved citations = %d, want 2", result.Citations.Retrieved)
	}
	if result.Citations.Applied != 1 {
		t.Fatalf("applied citations = %d, want 1", result.Citations.Applied)
	}
	if result.State == nil || result.State.LastStart == nil {
		t.Fatalf("expected persisted codex lifecycle state, got %+v", result.State)
	}
}
