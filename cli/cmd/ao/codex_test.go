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
	repo := setupCodexStartupPolicyRepo(t)
	path := writeCodexStartupPolicyContext(t, repo)
	assertCodexStartupPolicyContext(t, path)
}

func setupCodexStartupPolicyRepo(t *testing.T) string {
	t.Helper()

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

	writeCodexStartupPolicyArtifacts(t, repo)
	writeKnowledgeCorpusFixtures(t, repo)
	if _, err := buildKnowledgeBeliefBook(filepath.Join(repo, ".agents")); err != nil {
		t.Fatalf("buildKnowledgeBeliefBook: %v", err)
	}
	if _, err := buildKnowledgePlaybooks(filepath.Join(repo, ".agents"), false); err != nil {
		t.Fatalf("buildKnowledgePlaybooks: %v", err)
	}

	return repo
}

func writeCodexStartupPolicyArtifacts(t *testing.T, repo string) {
	t.Helper()

	artifacts := []struct {
		rel     string
		content string
	}{
		{
			rel: filepath.Join(".agents", "findings", "f-startup-002.md"),
			content: `---
id: "f-startup-002"
title: "Prefer startup packet over recency dump"
status: "active"
severity: "high"
applicable_when: ["task","startup"]
scope_tags: ["startup","context"]
---
# Finding

Prefer startup packet over arbitrary recent artifacts.
`,
		},
		{
			rel: filepath.Join(".agents", "planning-rules", "f-startup-002.md"),
			content: `---
id: "f-startup-002"
---
# Planning Rule

- Ask: Did startup context use ranked rules before recent documents?
`,
		},
		{
			rel: filepath.Join(".agents", "pre-mortem-checks", "f-startup-002.md"),
			content: `---
id: "f-startup-002"
---
# Pre-Mortem Check

- Ask: Did startup context exclude discovery-only notes?
`,
		},
	}

	for _, artifact := range artifacts {
		if err := os.WriteFile(filepath.Join(repo, artifact.rel), []byte(artifact.content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func writeCodexStartupPolicyContext(t *testing.T, repo string) string {
	t.Helper()

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
		false,
	)
	if err != nil {
		t.Fatalf("writeCodexStartupContext: %v", err)
	}

	return path
}

func assertCodexStartupPolicyContext(t *testing.T, path string) {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read startup context: %v", err)
	}
	content := string(data)

	for _, expected := range []struct {
		needle  string
		failure string
	}{
		{needle: "## Briefings", failure: "expected briefings heading"},
		{needle: "Startup briefing", failure: "expected startup briefing in startup context"},
		{needle: "## Operator Model", failure: "expected operator model heading"},
		{needle: "## Startup Slots", failure: "expected startup slots heading"},
		{needle: "### Core Beliefs", failure: "expected core beliefs in startup context"},
		{needle: "### Relevant Playbook", failure: "expected relevant playbook in startup context"},
		{needle: "### Warnings / Blockers", failure: "expected warnings section in startup context"},
		{needle: "### Source Links", failure: "expected source links in startup context"},
		{needle: "## Degraded Mode", failure: "expected degraded mode section in startup context"},
		{needle: "## Excluded By Default", failure: "expected exclusion policy heading"},
		{needle: "Wire startup context to ranked packet", failure: "expected ranked next work in startup context"},
		{needle: "primary dynamic surface", failure: "expected briefing-first guidance in startup context"},
	} {
		if !strings.Contains(content, expected.needle) {
			t.Fatalf("%s, got:\n%s", expected.failure, content)
		}
	}
}

func TestCodexStartupBeliefsAndPlaybooksAreCapped(t *testing.T) {
	bundle := rankedContextBundle{
		Beliefs: []string{"one", "two", "three", "four"},
		Playbooks: []knowledgeContextPlaybook{
			{Title: "first", Summary: "first summary", Path: "first.md"},
			{Title: "second", Summary: "second summary", Path: "second.md"},
		},
	}

	beliefs := codexStartupBeliefs(bundle)
	playbooks := codexStartupPlaybooks(bundle)

	if len(beliefs) != 3 {
		t.Fatalf("belief count = %d, want 3", len(beliefs))
	}
	if beliefs[2] != "three" {
		t.Fatalf("belief cap kept %q as third item, want three", beliefs[2])
	}
	if len(playbooks) != 1 {
		t.Fatalf("playbook count = %d, want 1", len(playbooks))
	}
	if playbooks[0].Title != "first" {
		t.Fatalf("playbook cap kept %q, want first", playbooks[0].Title)
	}

	beliefs[0] = "mutated"
	playbooks[0].Title = "mutated"
	if bundle.Beliefs[0] == "mutated" {
		t.Fatal("belief cap result aliases bundle beliefs")
	}
	if bundle.Playbooks[0].Title == "mutated" {
		t.Fatal("playbook cap result aliases bundle playbooks")
	}
}

func TestWriteCodexStartupContextIncludesNewUserWelcome(t *testing.T) {
	repo := t.TempDir()
	profile := lifecycleRuntimeProfile{Runtime: runtimeKindCodex, Mode: lifecycleModeCodexHookless, ThreadName: "startup"}

	path, err := writeCodexStartupContext(
		repo,
		profile,
		"first run",
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		true,
	)
	if err != nil {
		t.Fatalf("writeCodexStartupContext: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read startup context: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "## New Here?") {
		t.Fatalf("expected new-user section in startup context, got:\n%s", content)
	}
	if !strings.Contains(content, "$research") || !strings.Contains(content, "$implement") || !strings.Contains(content, "$council") {
		t.Fatalf("expected startup commands in new-user section, got:\n%s", content)
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

func TestCodexEnsureStopRunsSessionEndMaintenanceParity(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CODEX_THREAD_ID", "019d1bf7-58ea-79e1-9f5d-02109d930082")
	t.Setenv("CODEX_INTERNAL_ORIGINATOR_OVERRIDE", "Codex Desktop")

	sessionID := "019d1bf7-58ea-79e1-9f5d-02109d930082"
	historyPath := filepath.Join(home, ".codex", "history.jsonl")
	indexPath := filepath.Join(home, ".codex", "session_index.jsonl")
	if err := os.MkdirAll(filepath.Dir(historyPath), 0o755); err != nil {
		t.Fatal(err)
	}
	history := []string{
		`{"session_id":"` + sessionID + `","ts":1766945655,"text":"Close out Codex session with maintenance parity"}`,
		`{"session_id":"` + sessionID + `","ts":1766945658,"text":"Archive stale uncited low-signal learnings automatically"}`,
	}
	if err := os.WriteFile(historyPath, []byte(strings.Join(history, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(indexPath, []byte(`{"id":"`+sessionID+`","thread_name":"Lifecycle maintenance parity","updated_at":"2026-03-23T12:00:00Z"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	repo := t.TempDir()
	learningsDir := filepath.Join(repo, ".agents", "learnings")
	if err := os.MkdirAll(learningsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	stalePath := filepath.Join(learningsDir, "stale-fragment.md")
	staleLearning := `---
id: stale-fragment
type: learning
date: 2026-01-01
source: codex-test
maturity: provisional
utility: 0.5000
confidence: 0.0000
reward_count: 0
helpful_count: 0
harmful_count: 0
---

# Stale Fragment

let me force the revert because the goals agent is still running and re-wrote it.
`
	if err := os.WriteFile(stalePath, []byte(staleLearning), 0o644); err != nil {
		t.Fatal(err)
	}
	oldTime := time.Now().AddDate(0, 0, -91)
	if err := os.Chtimes(stalePath, oldTime, oldTime); err != nil {
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

	out, err := executeCommand("codex", "ensure-stop", "--json")
	if err != nil {
		t.Fatalf("codex ensure-stop --json: %v\noutput: %s", err, out)
	}

	if _, err := os.Stat(filepath.Join(repo, ".agents", "archive", "learnings", "stale-fragment.md")); err != nil {
		t.Fatalf("expected stale learning archived by hookless closeout maintenance: %v", err)
	}
}

func TestCodexStatusJSONReflectsHooklessHealthAndSearchCitations(t *testing.T) {
	fixture := setupCodexStatusJSONFixture(t)

	startCodexStatusJSONLifecycle(t)
	recordSearchCitations(fixture.cwd, []searchResult{{Path: fixture.searchableLearningPath}}, resolveSessionID(""), "codex lifecycle status", "applied")
	assertCodexStatusJSONCitationLog(t, fixture)

	result := runCodexStatusJSON(t)
	assertCodexStatusJSONResult(t, result)
}

type codexStatusJSONFixture struct {
	repo                   string
	cwd                    string
	searchableLearningPath string
	sessionID              string
}

func setupCodexStatusJSONFixture(t *testing.T) codexStatusJSONFixture {
	t.Helper()

	home := t.TempDir()
	t.Setenv("HOME", home)
	sessionID := "019d1bf7-58ea-79e1-9f5d-02109d930081"
	t.Setenv("CODEX_THREAD_ID", sessionID)
	t.Setenv("CODEX_INTERNAL_ORIGINATOR_OVERRIDE", "Codex Desktop")

	indexPath := filepath.Join(home, ".codex", "session_index.jsonl")
	writeCodexStatusJSONFile(t, indexPath, `{"id":"`+sessionID+`","thread_name":"Codex lifecycle status","updated_at":"2026-03-23T12:00:00Z"}`+"\n")

	repo := t.TempDir()
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
	writeCodexStatusJSONFile(t, learningPath, learning)
	searchableLearningPath := filepath.Join(repo, ".agents", "learnings", "codex-status.jsonl")
	searchableLearning := `{"summary":"codex lifecycle status from explicit search citation","utility":0.8,"maturity":"provisional"}`
	writeCodexStatusJSONFile(t, searchableLearningPath, searchableLearning+"\n")
	writeCodexStatusJSONFile(t, filepath.Join(repo, ".agents", "knowledge", "pending", "queued-learning.md"), "# queued\n")
	writeCodexStatusJSONFile(t, filepath.Join(repo, ".agents", "knowledge", "pending", ".quarantine", "truncated-learning.md"), "# quarantined\n")

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	return codexStatusJSONFixture{
		repo:                   repo,
		cwd:                    cwd,
		searchableLearningPath: filepath.Join(cwd, ".agents", "learnings", "codex-status.jsonl"),
		sessionID:              sessionID,
	}
}

func writeCodexStatusJSONFile(t *testing.T, path string, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func startCodexStatusJSONLifecycle(t *testing.T) {
	t.Helper()

	if out, err := executeCommand("codex", "start", "--json", "--no-maintenance", "--query", "codex lifecycle status"); err != nil {
		t.Fatalf("codex start --json: %v\noutput: %s", err, out)
	}
}

func assertCodexStatusJSONCitationLog(t *testing.T, fixture codexStatusJSONFixture) {
	t.Helper()

	citationsPath := filepath.Join(fixture.repo, ".agents", "ao", "citations.jsonl")
	citations, err := os.ReadFile(citationsPath)
	if err != nil {
		t.Fatalf("read citations: %v", err)
	}
	citationsText := string(citations)
	if !strings.Contains(citationsText, `"session_id":"`+fixture.sessionID+`"`) {
		t.Fatalf("expected Codex session ID in citations: %s", citationsText)
	}
	if !strings.Contains(citationsText, `"citation_type":"applied"`) {
		t.Fatalf("expected applied citation in %s", citationsText)
	}
}

func runCodexStatusJSON(t *testing.T) codexStatusResult {
	t.Helper()

	out, err := executeCommand("codex", "status", "--json", "--days", "30")
	if err != nil {
		t.Fatalf("codex status --json: %v\noutput: %s", err, out)
	}

	var result codexStatusResult
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &result); err != nil {
		t.Fatalf("parse codex status json: %v\noutput: %s", err, out)
	}
	return result
}

func assertCodexStatusJSONResult(t *testing.T, result codexStatusResult) {
	t.Helper()

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

// --- validateCodexLifecycleState tests ---

func TestCodexValidateState_ValidEmpty(t *testing.T) {
	state := &codexLifecycleState{SchemaVersion: 1}
	if err := validateCodexLifecycleState(state); err != nil {
		t.Fatalf("expected no error for valid empty state, got: %v", err)
	}
}

func TestCodexValidateState_WrongSchemaVersion(t *testing.T) {
	state := &codexLifecycleState{SchemaVersion: 99}
	err := validateCodexLifecycleState(state)
	if err == nil {
		t.Fatal("expected error for unsupported schema version")
	}
	expected := "unsupported schema_version 99 (expected 1)"
	if err.Error() != expected {
		t.Fatalf("expected %q, got %q", expected, err.Error())
	}
}

func TestCodexValidateState_InvalidUpdatedAt(t *testing.T) {
	state := &codexLifecycleState{
		SchemaVersion: 1,
		UpdatedAt:     "not-a-timestamp",
	}
	err := validateCodexLifecycleState(state)
	if err == nil {
		t.Fatal("expected error for invalid updated_at")
	}
}

func TestCodexValidateState_InvalidStartTimestamp(t *testing.T) {
	state := &codexLifecycleState{
		SchemaVersion: 1,
		LastStart:     &codexLifecycleEvent{Timestamp: "2026-13-01T00:00:00Z"},
	}
	err := validateCodexLifecycleState(state)
	if err == nil {
		t.Fatal("expected error for invalid last_start timestamp")
	}
}

func TestCodexValidateState_InvalidStopTimestamp(t *testing.T) {
	state := &codexLifecycleState{
		SchemaVersion: 1,
		LastStop:      &codexLifecycleEvent{Timestamp: "garbage"},
	}
	err := validateCodexLifecycleState(state)
	if err == nil {
		t.Fatal("expected error for invalid last_stop timestamp")
	}
}

func TestCodexValidateState_StopBeforeStart(t *testing.T) {
	// Same session_id: stop before start is impossible and must be rejected.
	state := &codexLifecycleState{
		SchemaVersion: 1,
		LastStart:     &codexLifecycleEvent{SessionID: "sess-a", Timestamp: "2026-04-02T12:00:00Z"},
		LastStop:      &codexLifecycleEvent{SessionID: "sess-a", Timestamp: "2026-04-02T11:00:00Z"},
	}
	err := validateCodexLifecycleState(state)
	if err == nil {
		t.Fatal("expected error when last_stop is before last_start for same session")
	}
	expected := "last_stop (2026-04-02T11:00:00Z) is before last_start (2026-04-02T12:00:00Z)"
	if err.Error() != expected {
		t.Fatalf("expected %q, got %q", expected, err.Error())
	}
}

func TestCodexValidateState_StopBeforeStartSameSessionWithCloseoutEvidence(t *testing.T) {
	state := &codexLifecycleState{
		SchemaVersion: 1,
		LastStart: &codexLifecycleEvent{
			SessionID: "019d7cd3-d88a-7e80-b6fc-e0de3cf6f0fc",
			Timestamp: "2026-04-12T02:48:28Z",
		},
		LastStop: &codexLifecycleEvent{
			SessionID:      "019d7cd3-d88a-7e80-b6fc-e0de3cf6f0fc",
			Timestamp:      "2026-04-11T21:08:43Z",
			TranscriptPath: ".agents/ao/codex/transcripts/history-019d7cd3-d88a-7e80-b6fc-e0de3cf6f0fc.jsonl",
			HandoffPath:    ".agents/handoff/auto-019d7cd.json",
		},
	}
	if err := validateCodexLifecycleState(state); err != nil {
		t.Fatalf("expected no error for same-session restarted-thread closeout shape, got: %v", err)
	}
}

// TestCodexValidateState_ActiveSession_StopBeforeStartDifferentSession is the
// na-khx regression: during an active session, last_stop records the PREVIOUS
// (older) session and last_start records the CURRENT (newer) session. That
// shape must validate successfully so ensure-start/ensure-stop/status work
// without manual state repair.
func TestCodexValidateState_ActiveSession_StopBeforeStartDifferentSession(t *testing.T) {
	state := &codexLifecycleState{
		SchemaVersion: 1,
		LastStart: &codexLifecycleEvent{
			SessionID: "019d77ff-6609-77c3-8df3-9e2d433a9edc",
			Timestamp: "2026-04-10T15:26:31Z",
		},
		LastStop: &codexLifecycleEvent{
			SessionID: "019d77d9-7f74-7501-ae9a-83730531219d",
			Timestamp: "2026-04-10T15:20:51Z",
		},
	}
	if err := validateCodexLifecycleState(state); err != nil {
		t.Fatalf("expected no error for active-session shape (different session_ids, stop before start), got: %v", err)
	}
}

func TestCodexValidateState_StopAfterStart(t *testing.T) {
	state := &codexLifecycleState{
		SchemaVersion: 1,
		LastStart:     &codexLifecycleEvent{Timestamp: "2026-04-02T11:00:00Z"},
		LastStop:      &codexLifecycleEvent{Timestamp: "2026-04-02T12:00:00Z"},
	}
	if err := validateCodexLifecycleState(state); err != nil {
		t.Fatalf("expected no error when stop is after start, got: %v", err)
	}
}

func TestCodexValidateState_StopEqualStart(t *testing.T) {
	state := &codexLifecycleState{
		SchemaVersion: 1,
		LastStart:     &codexLifecycleEvent{Timestamp: "2026-04-02T12:00:00Z"},
		LastStop:      &codexLifecycleEvent{Timestamp: "2026-04-02T12:00:00Z"},
	}
	if err := validateCodexLifecycleState(state); err != nil {
		t.Fatalf("expected no error when stop equals start, got: %v", err)
	}
}

func TestCodexValidateState_FullValidState(t *testing.T) {
	state := &codexLifecycleState{
		SchemaVersion: 1,
		UpdatedAt:     "2026-04-02T12:30:00Z",
		LastStart:     &codexLifecycleEvent{Timestamp: "2026-04-02T11:00:00Z"},
		LastStop:      &codexLifecycleEvent{Timestamp: "2026-04-02T12:00:00Z"},
	}
	if err := validateCodexLifecycleState(state); err != nil {
		t.Fatalf("expected no error for fully valid state, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// printNamedItems
// ---------------------------------------------------------------------------

func TestPrintNamedItems_EmptySlice(t *testing.T) {
	out := captureJSONStdout(t, func() {
		printNamedItems("Briefings", []codexArtifactRef{}, func(item codexArtifactRef) string { return item.Title })
	})
	if !strings.Contains(out, "Briefings:") {
		t.Errorf("output missing heading, got: %q", out)
	}
	if !strings.Contains(out, "- none") {
		t.Errorf("output missing '- none', got: %q", out)
	}
}

func TestPrintNamedItems_WithItems(t *testing.T) {
	items := []codexArtifactRef{
		{Title: "First briefing", Path: "/a"},
		{Title: "Second briefing", Path: "/b"},
	}
	out := captureJSONStdout(t, func() {
		printNamedItems("Research", items, func(item codexArtifactRef) string { return item.Title })
	})
	if !strings.Contains(out, "Research:") {
		t.Errorf("output missing heading, got: %q", out)
	}
	if !strings.Contains(out, "- First briefing") {
		t.Errorf("output missing first item, got: %q", out)
	}
	if !strings.Contains(out, "- Second briefing") {
		t.Errorf("output missing second item, got: %q", out)
	}
}

// ---------------------------------------------------------------------------
// outputCodexStartResult (human mode)
// ---------------------------------------------------------------------------

func TestOutputCodexStartResult_Human(t *testing.T) {
	// Ensure we're in human/table mode
	origOutput := output
	output = "table"
	defer func() { output = origOutput }()

	result := codexStartResult{
		Runtime: lifecycleRuntimeProfile{
			Mode:    lifecycleModeCodexHookless,
			Runtime: runtimeKindCodex,
		},
		StartupContextPath: "/tmp/startup.md",
		MemoryPath:         "/tmp/MEMORY.md",
		Learnings: []learning{
			{Title: "Auth fix learning"},
		},
	}

	out, err := captureStdout(t, func() error {
		return outputCodexStartResult(result)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Codex Start") {
		t.Errorf("output missing header, got: %q", out)
	}
	if !strings.Contains(out, "codex-hookless") {
		t.Errorf("output missing mode, got: %q", out)
	}
	if !strings.Contains(out, "Startup context:") {
		t.Errorf("output missing startup context, got: %q", out)
	}
	if !strings.Contains(out, "Memory:") {
		t.Errorf("output missing memory, got: %q", out)
	}
	if !strings.Contains(out, "Auth fix learning") {
		t.Errorf("output missing learning title, got: %q", out)
	}
}

// ---------------------------------------------------------------------------
// outputCodexStartResult (JSON mode)
// ---------------------------------------------------------------------------

func TestOutputCodexStartResult_JSON(t *testing.T) {
	origOutput := output
	output = "json"
	defer func() { output = origOutput }()

	result := codexStartResult{
		Runtime: lifecycleRuntimeProfile{
			Mode:    lifecycleModeCodexHookless,
			Runtime: runtimeKindCodex,
		},
		StartupContextPath: "/tmp/startup.md",
	}

	out, err := captureStdout(t, func() error {
		return outputCodexStartResult(result)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed codexStartResult
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("invalid JSON output: %v\nraw: %s", err, out)
	}
	if parsed.StartupContextPath != "/tmp/startup.md" {
		t.Errorf("StartupContextPath = %q, want %q", parsed.StartupContextPath, "/tmp/startup.md")
	}
}

// ---------------------------------------------------------------------------
// outputCodexStopResult (human mode)
// ---------------------------------------------------------------------------

func TestOutputCodexStopResult_Human(t *testing.T) {
	origOutput := output
	output = "table"
	defer func() { output = origOutput }()

	result := codexStopResult{
		Runtime: lifecycleRuntimeProfile{
			Mode:    lifecycleModeCodexHookless,
			Runtime: runtimeKindCodex,
		},
		TranscriptPath:      "/tmp/transcript.jsonl",
		TranscriptSource:    "archived",
		SyntheticTranscript: true,
		Session: SessionCloseResult{
			SessionID:          "sess-1",
			LearningsExtracted: 3,
			LearningsRejected:  1,
			HandoffWritten:     "/tmp/handoff.md",
		},
	}

	out, err := captureStdout(t, func() error {
		return outputCodexStopResult(result)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Codex Stop") {
		t.Errorf("missing header, got: %q", out)
	}
	if !strings.Contains(out, "synthesized from Codex history.jsonl") {
		t.Errorf("missing synthetic notice, got: %q", out)
	}
	if !strings.Contains(out, "3 extracted") {
		t.Errorf("missing learnings count, got: %q", out)
	}
	if !strings.Contains(out, "Handoff:") {
		t.Errorf("missing handoff path, got: %q", out)
	}
}

// ---------------------------------------------------------------------------
// outputCodexEnsureStartResult (human mode)
// ---------------------------------------------------------------------------

func TestOutputCodexEnsureStartResult_Human(t *testing.T) {
	origOutput := output
	output = "table"
	defer func() { output = origOutput }()

	result := codexEnsureStartResult{
		Runtime: lifecycleRuntimeProfile{
			Mode:       lifecycleModeCodexHookless,
			Runtime:    runtimeKindCodex,
			ThreadName: "test-thread",
		},
		Performed:          true,
		SessionID:          "sess-42",
		Reason:             "first session",
		StartupContextPath: "/tmp/ctx.md",
		MemoryPath:         "/tmp/mem.md",
	}

	out, err := captureStdout(t, func() error {
		return outputCodexEnsureStartResult(result)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Codex Ensure Start") {
		t.Errorf("missing header, got: %q", out)
	}
	if !strings.Contains(out, "Thread: test-thread") {
		t.Errorf("missing thread name, got: %q", out)
	}
	if !strings.Contains(out, "Session: sess-42") {
		t.Errorf("missing session ID, got: %q", out)
	}
	if !strings.Contains(out, "Performed: true") {
		t.Errorf("missing performed flag, got: %q", out)
	}
	if !strings.Contains(out, "Reason: first session") {
		t.Errorf("missing reason, got: %q", out)
	}
}

// ---------------------------------------------------------------------------
// outputCodexEnsureStopResult (human mode)
// ---------------------------------------------------------------------------

func TestOutputCodexEnsureStopResult_Human(t *testing.T) {
	origOutput := output
	output = "table"
	defer func() { output = origOutput }()

	result := codexEnsureStopResult{
		Runtime: lifecycleRuntimeProfile{
			Mode:       lifecycleModeCodexHookless,
			Runtime:    runtimeKindCodex,
			ThreadName: "stop-thread",
		},
		Performed:           true,
		SessionID:           "sess-99",
		Reason:              "session ended",
		TranscriptPath:      "/tmp/t.jsonl",
		TranscriptSource:    "history-fallback",
		SyntheticTranscript: true,
		HandoffPath:         "/tmp/handoff.md",
		MemoryPath:          "/tmp/mem.md",
	}

	out, err := captureStdout(t, func() error {
		return outputCodexEnsureStopResult(result)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Codex Ensure Stop") {
		t.Errorf("missing header, got: %q", out)
	}
	if !strings.Contains(out, "Thread: stop-thread") {
		t.Errorf("missing thread, got: %q", out)
	}
	if !strings.Contains(out, "Performed: true") {
		t.Errorf("missing performed, got: %q", out)
	}
	if !strings.Contains(out, "synthesized from Codex history.jsonl") {
		t.Errorf("missing synthetic notice, got: %q", out)
	}
	if !strings.Contains(out, "Handoff:") {
		t.Errorf("missing handoff, got: %q", out)
	}
}

// ---------------------------------------------------------------------------
// outputCodexStatusResult (human mode)
// ---------------------------------------------------------------------------

func TestOutputCodexStatusResult_Human(t *testing.T) {
	origOutput := output
	output = "table"
	defer func() { output = origOutput }()

	result := codexStatusResult{
		Runtime: lifecycleRuntimeProfile{
			Mode:       lifecycleModeCodexHookless,
			Runtime:    runtimeKindCodex,
			ThreadName: "status-thread",
		},
		Capture: codexCaptureHealth{
			SessionsIndexed:  5,
			PendingKnowledge: 2,
			LastForgeAge:     "3h",
		},
		Retrieval: codexRetrievalHealth{
			Learnings: 10,
			Patterns:  5,
			Findings:  3,
			NextWork:  7,
			Briefings: 2,
			Research:  4,
		},
		Promotion: codexPromotionHealth{
			PendingPool: 3,
			StagedPool:  1,
		},
		Citations: codexCitationHealth{
			WindowDays:      30,
			Total:           50,
			UniqueArtifacts: 20,
			Retrieved:       30,
			Reference:       15,
			Applied:         5,
		},
		Flywheel: &flywheelBrief{
			Status:   "healthy",
			Velocity: 0.5,
		},
		State: &codexLifecycleState{
			LastStart: &codexLifecycleEvent{Timestamp: "2026-04-01T10:00:00Z"},
			LastStop:  &codexLifecycleEvent{Timestamp: "2026-04-01T11:00:00Z"},
		},
	}

	out, err := captureStdout(t, func() error {
		return outputCodexStatusResult(result)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Codex Lifecycle Status") {
		t.Errorf("missing header, got: %q", out)
	}
	if !strings.Contains(out, "Thread: status-thread") {
		t.Errorf("missing thread, got: %q", out)
	}
	if !strings.Contains(out, "sessions=5") {
		t.Errorf("missing sessions count, got: %q", out)
	}
	if !strings.Contains(out, "Last forge: 3h ago") {
		t.Errorf("missing forge age, got: %q", out)
	}
	if !strings.Contains(out, "learnings=10") {
		t.Errorf("missing learnings count, got: %q", out)
	}
	if !strings.Contains(out, "healthy") {
		t.Errorf("missing flywheel status, got: %q", out)
	}
	if !strings.Contains(out, "Last start: 2026-04-01T10:00:00Z") {
		t.Errorf("missing last start, got: %q", out)
	}
	if !strings.Contains(out, "Last stop: 2026-04-01T11:00:00Z") {
		t.Errorf("missing last stop, got: %q", out)
	}
}

// ---------------------------------------------------------------------------
// outputCodexStatusResult (JSON mode)
// ---------------------------------------------------------------------------

func TestOutputCodexStatusResult_JSON(t *testing.T) {
	origOutput := output
	output = "json"
	defer func() { output = origOutput }()

	result := codexStatusResult{
		Runtime: lifecycleRuntimeProfile{
			Mode:    lifecycleModeCodexHookless,
			Runtime: runtimeKindCodex,
		},
		Capture: codexCaptureHealth{SessionsIndexed: 3},
	}

	out, err := captureStdout(t, func() error {
		return outputCodexStatusResult(result)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed codexStatusResult
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v\nraw: %s", err, out)
	}
	if parsed.Capture.SessionsIndexed != 3 {
		t.Errorf("SessionsIndexed = %d, want 3", parsed.Capture.SessionsIndexed)
	}
}

// ---------------------------------------------------------------------------
// extractSessionIDFromCodexArchivedPath
// ---------------------------------------------------------------------------

func TestExtractSessionIDFromCodexArchivedPath_InCodexTest(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"/home/.codex/archived_sessions/a1b2c3d4-e5f6-7890-abcd-ef1234567890.jsonl", "a1b2c3d4-e5f6-7890-abcd-ef1234567890"},
		{"/home/.codex/foo.jsonl", ""},
		{"", ""},
	}
	for _, tt := range tests {
		got := extractSessionIDFromCodexArchivedPath(tt.path)
		if got != tt.want {
			t.Errorf("extractSessionIDFromCodexArchivedPath(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// normalizeCodexLifecyclePath
// ---------------------------------------------------------------------------

func TestNormalizeCodexLifecyclePath(t *testing.T) {
	tests := []struct {
		input string
		empty bool
	}{
		{"", true},
		{"  ", true},
		{"/tmp/foo", false},
		{"  /tmp/bar  ", false},
	}
	for _, tt := range tests {
		got := normalizeCodexLifecyclePath(tt.input)
		if tt.empty && got != "" {
			t.Errorf("normalizeCodexLifecyclePath(%q) = %q, want empty", tt.input, got)
		}
		if !tt.empty && got == "" {
			t.Errorf("normalizeCodexLifecyclePath(%q) = empty, want non-empty", tt.input)
		}
	}
}

// ---------------------------------------------------------------------------
// firstNonEmptyTrimmed
// ---------------------------------------------------------------------------

func TestFirstNonEmptyTrimmed(t *testing.T) {
	tests := []struct {
		values []string
		want   string
	}{
		{[]string{"", "  ", "hello"}, "hello"},
		{[]string{"first", "second"}, "first"},
		{[]string{"", "", ""}, ""},
		{[]string{" trimmed "}, "trimmed"},
		{nil, ""},
	}
	for _, tt := range tests {
		got := firstNonEmptyTrimmed(tt.values...)
		if got != tt.want {
			t.Errorf("firstNonEmptyTrimmed(%v) = %q, want %q", tt.values, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// codexLifecycleStatePath
// ---------------------------------------------------------------------------

func TestCodexLifecycleStatePath(t *testing.T) {
	got := codexLifecycleStatePath("/tmp/repo")
	want := filepath.Join("/tmp/repo", ".agents", "ao", "codex", "state.json")
	if got != want {
		t.Errorf("codexLifecycleStatePath = %q, want %q", got, want)
	}
}
