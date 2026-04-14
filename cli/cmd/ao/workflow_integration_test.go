package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/ratchet"
)

// =============================================================================
// Workflow Integration Test 1: RPI Ratchet Progression
//
// Simulates a full RPI lifecycle by recording each phase in order and verifying
// that computeNextStep returns the correct successor at every stage. Unlike the
// unit-level ratchet tests, this exercises the record-on-disk -> reload -> next
// pipeline end-to-end with realistic step outputs.
// =============================================================================

func TestWorkflow_RPIRatchetProgression(t *testing.T) {
	tmpDir := t.TempDir()

	// Create .agents/ao/ directory for chain storage
	chainDir := filepath.Join(tmpDir, ".agents", "ao")
	if err := os.MkdirAll(chainDir, 0700); err != nil {
		t.Fatal(err)
	}

	now := time.Now()
	chain := &ratchet.Chain{
		ID:      "workflow-rpi-test",
		Started: now,
		Entries: []ratchet.ChainEntry{},
	}
	chain.SetPath(filepath.Join(chainDir, "chain.jsonl"))

	if err := chain.Save(); err != nil {
		t.Fatalf("save initial chain: %v", err)
	}

	// Verify empty chain starts at "research"
	loaded, err := ratchet.LoadChain(tmpDir)
	if err != nil {
		t.Fatalf("load empty chain: %v", err)
	}
	result := computeNextStep(loaded)
	if result.Next != "research" {
		t.Fatalf("empty chain: Next = %q, want %q", result.Next, "research")
	}
	if result.Skill != "/research" {
		t.Fatalf("empty chain: Skill = %q, want %q", result.Skill, "/research")
	}

	// Define the full RPI progression with realistic outputs
	phases := []struct {
		step      ratchet.Step
		output    string
		wantNext  string
		wantSkill string
		wantDone  bool
	}{
		{
			step:      ratchet.StepResearch,
			output:    ".agents/research/2026-02-25-api-design.md",
			wantNext:  "pre-mortem",
			wantSkill: "/pre-mortem",
		},
		{
			step:      ratchet.StepPreMortem,
			output:    ".agents/council/2026-02-25-pre-mortem-api-design.md",
			wantNext:  "plan",
			wantSkill: "/plan",
		},
		{
			step:      ratchet.StepPlan,
			output:    ".agents/plans/2026-02-25-api-design-plan.md",
			wantNext:  "implement",
			wantSkill: "/implement or /crank",
		},
		{
			step:      ratchet.StepImplement,
			output:    "epic:ag-xyz implemented across 5 files",
			wantNext:  "vibe",
			wantSkill: "/vibe",
		},
		{
			step:      ratchet.StepVibe,
			output:    ".agents/council/2026-02-25-vibe-api-design.md",
			wantNext:  "post-mortem",
			wantSkill: "/post-mortem",
		},
		{
			step:     ratchet.StepPostMortem,
			output:   ".agents/council/2026-02-25-post-mortem-api-design.md",
			wantNext: "",
			wantDone: true,
		},
	}

	for _, p := range phases {
		// Record the step
		entry := ratchet.ChainEntry{
			Step:      p.step,
			Timestamp: time.Now(),
			Output:    p.output,
			Locked:    true,
		}
		if err := chain.Append(entry); err != nil {
			t.Fatalf("append %s: %v", p.step, err)
		}

		// Reload from disk (simulates a fresh CLI invocation)
		loaded, err := ratchet.LoadChain(tmpDir)
		if err != nil {
			t.Fatalf("reload after %s: %v", p.step, err)
		}

		result := computeNextStep(loaded)

		if result.Next != p.wantNext {
			t.Errorf("after %s: Next = %q, want %q", p.step, result.Next, p.wantNext)
		}
		if result.Complete != p.wantDone {
			t.Errorf("after %s: Complete = %v, want %v", p.step, result.Complete, p.wantDone)
		}
		if !p.wantDone && result.Skill != p.wantSkill {
			t.Errorf("after %s: Skill = %q, want %q", p.step, result.Skill, p.wantSkill)
		}
		// Verify last step and artifact are always populated after first step
		if result.LastStep != string(p.step) {
			t.Errorf("after %s: LastStep = %q, want %q", p.step, result.LastStep, string(p.step))
		}
		if result.LastArtifact != p.output {
			t.Errorf("after %s: LastArtifact = %q, want %q", p.step, result.LastArtifact, p.output)
		}
	}
}

// =============================================================================
// Workflow Integration Test 2: Session to Memory Sync
//
// Tests the cross-command workflow: forge creates sessions -> memory sync reads
// them and writes MEMORY.md -> second sync deduplicates. This exercises the
// session JSONL file format, syncMemory dedup, and managed block preservation.
// =============================================================================

func TestWorkflow_SessionToMemory(t *testing.T) {
	fixture := setupSessionToMemoryFixture(t)
	sessions := defaultSessionToMemorySessions()
	fixture.writeSessions(t, sessions)

	content := fixture.syncAndReadMemory(t, "first sync")
	assertSessionToMemoryInitialSync(t, content, sessions)

	content = fixture.syncAndReadMemory(t, "second sync")
	assertSessionToMemoryDeduped(t, content, sessions, "second sync")

	newSession := sessionToMemorySession{
		id:      "sess-ddd3456",
		date:    time.Date(2026, 2, 25, 16, 0, 0, 0, time.UTC),
		summary: "New session added after initial sync",
		name:    "2026-02-25-test-sess-ddd3456.jsonl",
	}
	fixture.writeSession(t, newSession)
	content = fixture.syncAndReadMemory(t, "third sync")
	assertSessionToMemoryThirdSync(t, content, newSession, sessions)
}

type sessionToMemorySession struct {
	id        string
	date      time.Time
	summary   string
	name      string
	decisions []string
	knowledge []string
}

type sessionToMemoryFixture struct {
	tmpDir      string
	sessionsDir string
	outputPath  string
}

func setupSessionToMemoryFixture(t *testing.T) sessionToMemoryFixture {
	t.Helper()

	tmpDir := t.TempDir()
	sessionsDir := filepath.Join(tmpDir, ".agents", "ao", "sessions")
	if err := os.MkdirAll(sessionsDir, 0755); err != nil {
		t.Fatal(err)
	}

	return sessionToMemoryFixture{
		tmpDir:      tmpDir,
		sessionsDir: sessionsDir,
		outputPath:  filepath.Join(tmpDir, "MEMORY.md"),
	}
}

func defaultSessionToMemorySessions() []sessionToMemorySession {
	return []sessionToMemorySession{
		{
			id:        "sess-aaa1234",
			date:      time.Date(2026, 2, 23, 10, 0, 0, 0, time.UTC),
			summary:   "Implemented pool ingest command with rubric scoring",
			name:      "2026-02-23-test-sess-aaa1234.jsonl",
			decisions: []string{"Decision for sess-aaa1234"},
			knowledge: []string{"Knowledge from sess-aaa1234"},
		},
		{
			id:        "sess-bbb5678",
			date:      time.Date(2026, 2, 24, 14, 0, 0, 0, time.UTC),
			summary:   "Fixed ratchet chain migration from YAML to JSONL",
			name:      "2026-02-24-test-sess-bbb5678.jsonl",
			decisions: []string{"Decision for sess-bbb5678"},
			knowledge: []string{"Knowledge from sess-bbb5678"},
		},
		{
			id:        "sess-ccc9012",
			date:      time.Date(2026, 2, 25, 9, 0, 0, 0, time.UTC),
			summary:   "Added notebook update with pruning and cursor dedup",
			name:      "2026-02-25-test-sess-ccc9012.jsonl",
			decisions: []string{"Decision for sess-ccc9012"},
			knowledge: []string{"Knowledge from sess-ccc9012"},
		},
	}
}

func (f sessionToMemoryFixture) writeSessions(t *testing.T, sessions []sessionToMemorySession) {
	t.Helper()

	for _, session := range sessions {
		f.writeSession(t, session)
	}
}

func (f sessionToMemoryFixture) writeSession(t *testing.T, session sessionToMemorySession) {
	t.Helper()

	entry := map[string]any{
		"session_id": session.id,
		"date":       session.date.Format(time.RFC3339),
		"summary":    session.summary,
	}
	if len(session.decisions) > 0 {
		entry["decisions"] = session.decisions
	}
	if len(session.knowledge) > 0 {
		entry["knowledge"] = session.knowledge
	}

	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(f.sessionsDir, session.name), data, 0644); err != nil {
		t.Fatal(err)
	}
}

func (f sessionToMemoryFixture) syncAndReadMemory(t *testing.T, phase string) string {
	t.Helper()

	if err := syncMemory(f.tmpDir, f.outputPath, 10, true); err != nil {
		t.Fatalf("%s: %v", phase, err)
	}

	data, err := os.ReadFile(f.outputPath)
	if err != nil {
		t.Fatalf("read after %s: %v", phase, err)
	}
	return string(data)
}

func assertSessionToMemoryInitialSync(t *testing.T, content string, sessions []sessionToMemorySession) {
	t.Helper()

	for _, session := range sessions {
		shortID := sessionToMemoryShortID(session)
		if !strings.Contains(content, shortID) {
			t.Errorf("first sync: missing session %s (short ID: %s)", session.id, shortID)
		}
	}

	if !strings.Contains(content, memoryBlockStart) {
		t.Error("first sync: missing managed block start marker")
	}
	if !strings.Contains(content, memoryBlockEnd) {
		t.Error("first sync: missing managed block end marker")
	}
}

func assertSessionToMemoryDeduped(t *testing.T, content string, sessions []sessionToMemorySession, phase string) {
	t.Helper()

	for _, session := range sessions {
		shortID := sessionToMemoryShortID(session)
		count := strings.Count(content, shortID)
		if count != 1 {
			t.Errorf("%s: session %s appears %d times (expected 1, dedup failed)", phase, shortID, count)
		}
	}
}

func assertSessionToMemoryThirdSync(
	t *testing.T,
	content string,
	newSession sessionToMemorySession,
	previousSessions []sessionToMemorySession,
) {
	t.Helper()

	if count := strings.Count(content, sessionToMemoryShortID(newSession)); count != 1 {
		t.Errorf("third sync: new session appears %d times (expected 1)", count)
	}
	for _, session := range previousSessions {
		shortID := sessionToMemoryShortID(session)
		if count := strings.Count(content, shortID); count != 1 {
			t.Errorf("third sync: old session %s appears %d times (expected 1)", shortID, count)
		}
	}
}

func sessionToMemoryShortID(session sessionToMemorySession) string {
	return session.id[:7]
}

// =============================================================================
// Workflow Integration Test 3: Notebook Update Cycle
//
// Tests the notebook update pipeline: create session data -> run notebook
// update logic -> verify MEMORY.md has "Last Session" section -> run again
// with the same session -> verify idempotency (cursor prevents replay).
// =============================================================================

func TestWorkflow_NotebookUpdateCycle(t *testing.T) {
	fixture := setupNotebookUpdateCycleFixture(t)
	fixture.writeSession(t, notebookUpdateCycleSession{
		id:      "workflow-nb-001",
		when:    time.Date(2026, 2, 25, 12, 0, 0, 0, time.UTC),
		summary: "Implemented cross-workflow integration tests",
		decisions: []string{
			"Used t.TempDir for isolation",
			"Called internal functions directly",
		},
		knowledge: []string{
			"Next: wire pool ingest into session close",
			"Success: all three test patterns validate end-to-end",
			"Ratchet chain reload works correctly across record cycles",
		},
	})

	entry := fixture.readLatestSessionEntry(t, "workflow-nb-001")
	content1 := fixture.renderLastSession(t, entry, "parse notebook")
	fixture.writeNotebook(t, content1, "write notebook")
	fixture.writeCursor(t, entry.SessionID)
	assertNotebookPhaseOneContent(t, content1)

	content2 := fixture.renderLastSession(t, entry, "parse notebook pass 2")
	assertNotebookIdempotent(t, content1, content2)
	assertNotebookCursor(t, fixture.cursorPath, "workflow-nb-001")

	fixture.writeSession(t, notebookUpdateCycleSession{
		id:        "workflow-nb-002",
		when:      time.Date(2026, 2, 25, 18, 0, 0, 0, time.UTC),
		summary:   "Fixed pool scoring edge cases",
		decisions: []string{"Bias pending learnings by confidence level"},
		knowledge: []string{"Success: rubric scores now match expected tiers"},
	})
	newEntry := fixture.readLatestSessionEntry(t, "workflow-nb-002")
	content3 := fixture.renderLastSession(t, newEntry, "parse notebook pass 3")
	fixture.writeNotebook(t, content3, "write notebook pass 3")
	assertNotebookPhaseFourContent(t, content3)
}

type notebookUpdateCycleSession struct {
	id        string
	when      time.Time
	summary   string
	decisions []string
	knowledge []string
}

type notebookUpdateCycleFixture struct {
	tmpDir      string
	sessionsDir string
	memoryFile  string
	cursorPath  string
}

func setupNotebookUpdateCycleFixture(t *testing.T) notebookUpdateCycleFixture {
	t.Helper()

	tmpDir := t.TempDir()
	aoDir := filepath.Join(tmpDir, ".agents", "ao")
	if err := os.MkdirAll(aoDir, 0755); err != nil {
		t.Fatal(err)
	}

	sessionsDir := filepath.Join(aoDir, "sessions")
	if err := os.MkdirAll(sessionsDir, 0755); err != nil {
		t.Fatal(err)
	}

	memoryFile := filepath.Join(tmpDir, "MEMORY.md")
	initialContent := "# AgentOps Nami Memory\n\n## Key Lessons\n- Always verify CLI flags before templating\n- Post-swarm constraint checklist catches violations\n"
	if err := os.WriteFile(memoryFile, []byte(initialContent), 0644); err != nil {
		t.Fatal(err)
	}

	return notebookUpdateCycleFixture{
		tmpDir:      tmpDir,
		sessionsDir: sessionsDir,
		memoryFile:  memoryFile,
		cursorPath:  filepath.Join(aoDir, "notebook-cursor.json"),
	}
}

func (f notebookUpdateCycleFixture) writeSession(t *testing.T, session notebookUpdateCycleSession) {
	t.Helper()

	data, err := json.Marshal(map[string]any{
		"session_id": session.id,
		"date":       session.when.Format(time.RFC3339),
		"summary":    session.summary,
		"decisions":  session.decisions,
		"knowledge":  session.knowledge,
	})
	if err != nil {
		t.Fatalf("marshal session %s: %v", session.id, err)
	}

	filename := fmt.Sprintf("%s-%s.jsonl", session.when.Format("2006-01-02"), session.id)
	if err := os.WriteFile(filepath.Join(f.sessionsDir, filename), data, 0644); err != nil {
		t.Fatal(err)
	}
}

func (f notebookUpdateCycleFixture) readLatestSessionEntry(t *testing.T, wantID string) *pendingEntry {
	t.Helper()

	entry, err := readLatestSessionEntry(f.tmpDir)
	if err != nil {
		t.Fatalf("read latest session: %v", err)
	}
	if entry == nil {
		t.Fatal("expected non-nil session entry")
	}
	if entry.SessionID != wantID {
		t.Fatalf("session ID = %q, want %q", entry.SessionID, wantID)
	}
	return entry
}

func (f notebookUpdateCycleFixture) renderLastSession(t *testing.T, entry *pendingEntry, parseLabel string) string {
	t.Helper()

	sections, err := parseNotebookSections(f.memoryFile)
	if err != nil {
		t.Fatalf("%s: %v", parseLabel, err)
	}

	lastSession := buildLastSessionSection(entry)
	sections = upsertLastSession(sections, lastSession)
	sections = pruneNotebook(sections, 190)
	return renderNotebook(sections)
}

func (f notebookUpdateCycleFixture) writeNotebook(t *testing.T, content string, label string) {
	t.Helper()

	if err := atomicWriteFile(f.memoryFile, []byte(content), 0644); err != nil {
		t.Fatalf("%s: %v", label, err)
	}
}

func (f notebookUpdateCycleFixture) writeCursor(t *testing.T, sessionID string) {
	t.Helper()

	if err := writeNotebookCursor(f.cursorPath, sessionID); err != nil {
		t.Fatalf("write cursor: %v", err)
	}
}

func assertNotebookPhaseOneContent(t *testing.T, content string) {
	t.Helper()

	assertNotebookContains(t, content, "## Last Session", "phase 1: missing Last Session heading")
	assertNotebookContains(t, content, "cross-workflow integration tests", "phase 1: missing session summary")
	assertNotebookContains(t, content, "Key decisions", "phase 1: missing decisions section")
	assertNotebookContains(t, content, "Next:", "phase 1: missing Next items")
	assertNotebookContains(t, content, "Key Lessons", "phase 1: existing Key Lessons section was lost")
	assertNotebookContains(t, content, "verify CLI flags", "phase 1: existing lesson content was lost")
}

func assertNotebookIdempotent(t *testing.T, content1 string, content2 string) {
	t.Helper()

	if content1 != content2 {
		t.Error("phase 2: notebook update is not idempotent")
		t.Logf("pass 1 (%d bytes):\n%s", len(content1), content1)
		t.Logf("pass 2 (%d bytes):\n%s", len(content2), content2)
	}
	assertNotebookHeadingCount(t, content2, "phase 2")
}

func assertNotebookCursor(t *testing.T, cursorPath string, wantID string) {
	t.Helper()

	lastID, err := readNotebookCursor(cursorPath)
	if err != nil {
		t.Fatalf("read cursor: %v", err)
	}
	if lastID != wantID {
		t.Errorf("cursor session_id = %q, want %q", lastID, wantID)
	}
}

func assertNotebookPhaseFourContent(t *testing.T, content string) {
	t.Helper()

	assertNotebookContains(t, content, "pool scoring edge cases", "phase 4: missing new session summary")
	if strings.Contains(content, "cross-workflow integration tests") {
		t.Error("phase 4: old Last Session summary was not replaced")
	}
	assertNotebookHeadingCount(t, content, "phase 4")
	assertNotebookContains(t, content, "Key Lessons", "phase 4: existing sections lost after new session update")
}

func assertNotebookContains(t *testing.T, content string, needle string, message string) {
	t.Helper()

	if !strings.Contains(content, needle) {
		t.Error(message)
	}
}

func assertNotebookHeadingCount(t *testing.T, content string, phase string) {
	t.Helper()

	if count := strings.Count(content, "## Last Session"); count != 1 {
		t.Errorf("%s: found %d Last Session sections (expected 1)", phase, count)
	}
}

// =============================================================================
// Workflow Integration Test 4: Ratchet-to-Memory Cross-Command
//
// Tests the cross-command data flow: ratchet records RPI phases -> sessions
// capture those phases -> memory sync picks them up. Verifies that the
// artifacts produced by one command are consumable by the next.
// =============================================================================

func TestWorkflow_RatchetToMemoryCrossCommand(t *testing.T) {
	tmpDir := t.TempDir()

	// Setup directories
	chainDir := filepath.Join(tmpDir, ".agents", "ao")
	sessionsDir := filepath.Join(chainDir, "sessions")
	if err := os.MkdirAll(sessionsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// ---- Step 1: Record ratchet progression (research -> plan) ----
	now := time.Now()
	chain := &ratchet.Chain{
		ID:      "cross-cmd-test",
		Started: now,
		Entries: []ratchet.ChainEntry{},
	}
	chain.SetPath(filepath.Join(chainDir, "chain.jsonl"))
	if err := chain.Save(); err != nil {
		t.Fatal(err)
	}

	// Record research
	if err := chain.Append(ratchet.ChainEntry{
		Step:      ratchet.StepResearch,
		Timestamp: now,
		Output:    ".agents/research/findings.md",
		Locked:    true,
	}); err != nil {
		t.Fatal(err)
	}

	// Record pre-mortem
	if err := chain.Append(ratchet.ChainEntry{
		Step:      ratchet.StepPreMortem,
		Timestamp: now.Add(30 * time.Minute),
		Output:    ".agents/council/pre-mortem.md",
		Locked:    true,
	}); err != nil {
		t.Fatal(err)
	}

	// Verify ratchet next says "plan"
	loaded, err := ratchet.LoadChain(tmpDir)
	if err != nil {
		t.Fatalf("load chain: %v", err)
	}
	nextResult := computeNextStep(loaded)
	if nextResult.Next != "plan" {
		t.Fatalf("after research+pre-mortem: next = %q, want plan", nextResult.Next)
	}

	// ---- Step 2: Create a session file representing the work done ----
	sessionEntry := map[string]any{
		"session_id": "cross-sess-001",
		"date":       now.Format(time.RFC3339),
		"summary":    "Completed research and pre-mortem phases",
		"decisions":  []string{"Research found 3 viable approaches", "Pre-mortem identified 5 risks"},
		"knowledge":  []string{"Next: create implementation plan"},
	}
	sessionData, _ := json.Marshal(sessionEntry)
	if err := os.WriteFile(
		filepath.Join(sessionsDir, fmt.Sprintf("%s-cross-sess-001.jsonl", now.Format("2006-01-02"))),
		sessionData, 0644,
	); err != nil {
		t.Fatal(err)
	}

	// ---- Step 3: Memory sync picks up the session ----
	outputPath := filepath.Join(tmpDir, "MEMORY.md")
	if err := syncMemory(tmpDir, outputPath, 10, true); err != nil {
		t.Fatalf("memory sync: %v", err)
	}

	memoryContent, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read MEMORY.md: %v", err)
	}

	// Verify session content appears in memory
	if !strings.Contains(string(memoryContent), "cross-s") {
		t.Error("memory sync: session ID not found in MEMORY.md")
	}
	if !strings.Contains(string(memoryContent), "research and pre-mortem") {
		t.Error("memory sync: session summary not found in MEMORY.md")
	}

	// ---- Step 4: Notebook update also works with the same session ----
	memoryFile := filepath.Join(tmpDir, "NOTEBOOK-MEMORY.md")
	if err := os.WriteFile(memoryFile, []byte("# Notebook\n"), 0644); err != nil {
		t.Fatal(err)
	}

	entry, err := readLatestSessionEntry(tmpDir)
	if err != nil {
		t.Fatalf("read session: %v", err)
	}

	sections, _ := parseNotebookSections(memoryFile)
	lastSession := buildLastSessionSection(entry)
	sections = upsertLastSession(sections, lastSession)
	nbContent := renderNotebook(sections)

	if !strings.Contains(nbContent, "## Last Session") {
		t.Error("notebook: missing Last Session")
	}
	if !strings.Contains(nbContent, "research and pre-mortem") {
		t.Error("notebook: session summary not found")
	}

	// ---- Step 5: Verify ratchet state is independent of memory ----
	// Ratchet should still say "plan" regardless of memory sync
	loaded2, _ := ratchet.LoadChain(tmpDir)
	nextResult2 := computeNextStep(loaded2)
	if nextResult2.Next != "plan" {
		t.Errorf("ratchet state changed after memory sync: next = %q, want plan", nextResult2.Next)
	}
}
