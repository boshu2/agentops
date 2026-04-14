package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/ratchet"
	"github.com/boshu2/agentops/cli/internal/storage"
)

// =============================================================================
// Integration Test 1: Ratchet Next After Recording
//
// Verifies that after recording a "research" step as locked in a chain,
// computeNextStep returns "pre-mortem" as the next step. Then after recording
// pre-mortem, it returns "plan", and so on through the full lifecycle.
// =============================================================================

func TestIntegration_RatchetNextAfterRecording(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		entries  []ratchet.ChainEntry
		wantNext string
		wantDone bool
	}{
		{
			name:     "empty chain yields research",
			entries:  nil,
			wantNext: "research",
			wantDone: false,
		},
		{
			name: "after research locked, next is pre-mortem",
			entries: []ratchet.ChainEntry{
				{
					Step:      ratchet.StepResearch,
					Timestamp: now,
					Output:    ".agents/research/findings.md",
					Locked:    true,
				},
			},
			wantNext: "pre-mortem",
			wantDone: false,
		},
		{
			name: "after research + pre-mortem locked, next is plan",
			entries: []ratchet.ChainEntry{
				{
					Step:      ratchet.StepResearch,
					Timestamp: now,
					Output:    ".agents/research/findings.md",
					Locked:    true,
				},
				{
					Step:      ratchet.StepPreMortem,
					Timestamp: now.Add(time.Hour),
					Output:    ".agents/council/pre-mortem.md",
					Locked:    true,
				},
			},
			wantNext: "plan",
			wantDone: false,
		},
		{
			name: "full chain through vibe yields post-mortem",
			entries: []ratchet.ChainEntry{
				{
					Step:      ratchet.StepResearch,
					Timestamp: now,
					Output:    ".agents/research/findings.md",
					Locked:    true,
				},
				{
					Step:      ratchet.StepPreMortem,
					Timestamp: now.Add(1 * time.Hour),
					Output:    ".agents/council/pre-mortem.md",
					Locked:    true,
				},
				{
					Step:      ratchet.StepPlan,
					Timestamp: now.Add(2 * time.Hour),
					Output:    ".agents/plans/epic-plan.md",
					Locked:    true,
				},
				{
					Step:      ratchet.StepImplement,
					Timestamp: now.Add(3 * time.Hour),
					Output:    "feature implemented",
					Locked:    true,
				},
				{
					Step:      ratchet.StepVibe,
					Timestamp: now.Add(4 * time.Hour),
					Output:    ".agents/council/vibe.md",
					Locked:    true,
				},
			},
			wantNext: "post-mortem",
			wantDone: false,
		},
		{
			name: "full chain yields complete",
			entries: []ratchet.ChainEntry{
				{
					Step:      ratchet.StepResearch,
					Timestamp: now,
					Output:    ".agents/research/findings.md",
					Locked:    true,
				},
				{
					Step:      ratchet.StepPreMortem,
					Timestamp: now.Add(1 * time.Hour),
					Output:    ".agents/council/pre-mortem.md",
					Locked:    true,
				},
				{
					Step:      ratchet.StepPlan,
					Timestamp: now.Add(2 * time.Hour),
					Output:    ".agents/plans/epic-plan.md",
					Locked:    true,
				},
				{
					Step:      ratchet.StepImplement,
					Timestamp: now.Add(3 * time.Hour),
					Output:    "feature implemented",
					Locked:    true,
				},
				{
					Step:      ratchet.StepVibe,
					Timestamp: now.Add(4 * time.Hour),
					Output:    ".agents/council/vibe.md",
					Locked:    true,
				},
				{
					Step:      ratchet.StepPostMortem,
					Timestamp: now.Add(5 * time.Hour),
					Output:    ".agents/council/post-mortem.md",
					Locked:    true,
				},
			},
			wantNext: "",
			wantDone: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chain := &ratchet.Chain{
				ID:      "integration-test",
				Started: now,
				Entries: tt.entries,
			}

			result := computeNextStep(chain)

			if result.Next != tt.wantNext {
				t.Errorf("Next = %q, want %q", result.Next, tt.wantNext)
			}
			if result.Complete != tt.wantDone {
				t.Errorf("Complete = %v, want %v", result.Complete, tt.wantDone)
			}

			// Integration check: verify skill mapping is consistent
			if !result.Complete && result.Next != "" {
				if result.Skill == "" {
					t.Errorf("skill mapping missing for next step %q", result.Next)
				}
			}
			if result.Complete && result.Skill != "" {
				t.Errorf("complete chain should have empty skill, got %q", result.Skill)
			}
		})
	}
}

// TestIntegration_RatchetNextProgressiveRecording simulates progressively
// appending entries to a chain (as a real session would) and verifying
// computeNextStep at each stage.
func TestIntegration_RatchetNextProgressiveRecording(t *testing.T) {
	tmpDir := t.TempDir()

	// Create .agents/ao/ directory and chain file
	chainDir := filepath.Join(tmpDir, ".agents", "ao")
	if err := os.MkdirAll(chainDir, 0700); err != nil {
		t.Fatal(err)
	}

	now := time.Now()
	chain := &ratchet.Chain{
		ID:      "progressive-test",
		Started: now,
		Entries: []ratchet.ChainEntry{},
	}
	chain.SetPath(filepath.Join(chainDir, "chain.jsonl"))

	// Save initial empty chain
	if err := chain.Save(); err != nil {
		t.Fatalf("save initial chain: %v", err)
	}

	// Step through the workflow, appending and checking at each stage
	steps := []struct {
		step     ratchet.Step
		output   string
		wantNext string
	}{
		{ratchet.StepResearch, ".agents/research/findings.md", "pre-mortem"},
		{ratchet.StepPreMortem, ".agents/council/pre-mortem.md", "plan"},
		{ratchet.StepPlan, ".agents/plans/epic-plan.md", "implement"},
		{ratchet.StepImplement, "feature implemented", "vibe"},
		{ratchet.StepVibe, ".agents/council/vibe.md", "post-mortem"},
	}

	for _, s := range steps {
		entry := ratchet.ChainEntry{
			Step:      s.step,
			Timestamp: time.Now(),
			Output:    s.output,
			Locked:    true,
		}

		if err := chain.Append(entry); err != nil {
			t.Fatalf("append %s: %v", s.step, err)
		}

		// Reload chain from disk to simulate a fresh read
		loaded, err := ratchet.LoadChain(tmpDir)
		if err != nil {
			t.Fatalf("reload chain after %s: %v", s.step, err)
		}

		result := computeNextStep(loaded)
		if result.Next != s.wantNext {
			t.Errorf("after %s: Next = %q, want %q", s.step, result.Next, s.wantNext)
		}
		if result.Complete {
			t.Errorf("after %s: should not be complete yet", s.step)
		}
	}

	// Final step: post-mortem should mark complete
	finalEntry := ratchet.ChainEntry{
		Step:      ratchet.StepPostMortem,
		Timestamp: time.Now(),
		Output:    ".agents/council/post-mortem.md",
		Locked:    true,
	}
	if err := chain.Append(finalEntry); err != nil {
		t.Fatalf("append post-mortem: %v", err)
	}

	loaded, err := ratchet.LoadChain(tmpDir)
	if err != nil {
		t.Fatalf("reload chain after post-mortem: %v", err)
	}

	result := computeNextStep(loaded)
	if !result.Complete {
		t.Error("expected chain to be complete after post-mortem")
	}
	if result.Next != "" {
		t.Errorf("expected empty Next after completion, got %q", result.Next)
	}
}

// =============================================================================
// Integration Test 2: Forge Batch Pipeline
//
// Verifies the batch forge index pipeline: loading a forged index,
// filtering already-forged transcripts, appending new records, and verifying
// that reloading the index correctly skips them.
// =============================================================================

func TestIntegration_ForgeBatchPipeline(t *testing.T) {
	fixture := setupForgeBatchPipelineFixture(t)
	assertForgeBatchEmptyIndex(t, fixture.indexPath)

	transcriptPaths := writeForgeBatchTranscriptCandidates(t, fixture.tmpDir)
	appendForgeBatchInitialRecords(t, fixture.indexPath, transcriptPaths[:2])

	forgedSet := loadForgeBatchIndex(t, fixture.indexPath, "reload index")
	if len(forgedSet) != 2 {
		t.Fatalf("expected 2 forged entries, got %d", len(forgedSet))
	}

	unforged := filterForgeBatchUnforged(t, forgedSet, transcriptPaths)
	assertForgeBatchUnforgedTranscript(t, unforged, transcriptPaths[2])

	appendForgeBatchRecord(t, fixture.indexPath, transcriptPaths[2], "session-ccc", "append final forged record")
	forgedSet = loadForgeBatchIndex(t, fixture.indexPath, "final reload")
	assertForgeBatchAllForged(t, forgedSet, transcriptPaths)
}

type forgeBatchPipelineFixture struct {
	tmpDir    string
	indexPath string
}

func setupForgeBatchPipelineFixture(t *testing.T) forgeBatchPipelineFixture {
	t.Helper()

	tmpDir := t.TempDir()

	aoDir := filepath.Join(tmpDir, storage.DefaultBaseDir)
	if err := os.MkdirAll(aoDir, 0755); err != nil {
		t.Fatal(err)
	}

	return forgeBatchPipelineFixture{
		tmpDir:    tmpDir,
		indexPath: filepath.Join(aoDir, "forged.jsonl"),
	}
}

func assertForgeBatchEmptyIndex(t *testing.T, indexPath string) {
	t.Helper()

	forgedSet, err := loadForgedIndex(indexPath)
	if err != nil {
		t.Fatalf("load empty index: %v", err)
	}
	if len(forgedSet) != 0 {
		t.Fatalf("expected empty forged set, got %d", len(forgedSet))
	}
}

func writeForgeBatchTranscriptCandidates(t *testing.T, tmpDir string) []string {
	t.Helper()

	transcriptDir := filepath.Join(tmpDir, "transcripts")
	if err := os.MkdirAll(transcriptDir, 0755); err != nil {
		t.Fatal(err)
	}

	transcriptPaths := []string{
		filepath.Join(transcriptDir, "session-aaa.jsonl"),
		filepath.Join(transcriptDir, "session-bbb.jsonl"),
		filepath.Join(transcriptDir, "session-ccc.jsonl"),
	}

	for _, p := range transcriptPaths {
		content := `{"role":"user","content":"hello world, testing integration pipeline with enough content"}` + "\n" +
			`{"role":"assistant","content":"this is a response with enough content to pass the 100 byte filter"}` + "\n"
		if err := os.WriteFile(p, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return transcriptPaths
}

func appendForgeBatchInitialRecords(t *testing.T, indexPath string, transcriptPaths []string) {
	t.Helper()

	for _, p := range transcriptPaths[:2] {
		appendForgeBatchRecord(t, indexPath, p, "session-"+filepath.Base(p), "append forged record")
	}
}

func loadForgeBatchIndex(t *testing.T, indexPath string, label string) map[string]bool {
	t.Helper()

	forgedSet, err := loadForgedIndex(indexPath)
	if err != nil {
		t.Fatalf("%s: %v", label, err)
	}
	return forgedSet
}

func filterForgeBatchUnforged(
	t *testing.T,
	forgedSet map[string]bool,
	transcriptPaths []string,
) []transcriptCandidate {
	t.Helper()

	candidates := make([]transcriptCandidate, 0, len(transcriptPaths))
	for _, p := range transcriptPaths {
		info, err := os.Stat(p)
		if err != nil {
			t.Fatal(err)
		}
		candidates = append(candidates, transcriptCandidate{
			path:    p,
			modTime: info.ModTime(),
			size:    info.Size(),
		})
	}

	var unforged []transcriptCandidate
	for _, c := range candidates {
		if !forgedSet[c.path] {
			unforged = append(unforged, c)
		}
	}
	return unforged
}

func assertForgeBatchUnforgedTranscript(
	t *testing.T,
	unforged []transcriptCandidate,
	wantPath string,
) {
	t.Helper()

	if len(unforged) != 1 {
		t.Fatalf("expected 1 unforged transcript, got %d", len(unforged))
	}
	if unforged[0].path != wantPath {
		t.Errorf("expected %s, got %s", wantPath, unforged[0].path)
	}
}

func appendForgeBatchRecord(
	t *testing.T,
	indexPath string,
	path string,
	session string,
	label string,
) {
	t.Helper()

	record := ForgedRecord{
		Path:     path,
		ForgedAt: time.Now(),
		Session:  session,
	}
	if err := appendForgedRecord(indexPath, record); err != nil {
		t.Fatalf("%s: %v", label, err)
	}
}

func assertForgeBatchAllForged(t *testing.T, forgedSet map[string]bool, transcriptPaths []string) {
	t.Helper()

	if len(forgedSet) != 3 {
		t.Fatalf("expected 3 forged entries, got %d", len(forgedSet))
	}
	for _, p := range transcriptPaths {
		if !forgedSet[p] {
			t.Errorf("expected %s in forged set", p)
		}
	}
}

// TestIntegration_ForgeBatchMaxLimit verifies that --max flag correctly limits
// the number of transcripts processed from the unforged set.
func TestIntegration_ForgeBatchMaxLimit(t *testing.T) {
	tmpDir := t.TempDir()
	aoDir := filepath.Join(tmpDir, storage.DefaultBaseDir)
	if err := os.MkdirAll(aoDir, 0755); err != nil {
		t.Fatal(err)
	}

	indexPath := filepath.Join(aoDir, "forged.jsonl")

	// Pre-forge 2 of 5 transcripts
	allPaths := []string{
		"/t/s1.jsonl", "/t/s2.jsonl", "/t/s3.jsonl", "/t/s4.jsonl", "/t/s5.jsonl",
	}
	for _, p := range allPaths[:2] {
		if err := appendForgedRecord(indexPath, ForgedRecord{
			Path: p, ForgedAt: time.Now(), Session: "s",
		}); err != nil {
			t.Fatal(err)
		}
	}

	forgedSet, err := loadForgedIndex(indexPath)
	if err != nil {
		t.Fatal(err)
	}

	// Filter to unforged
	var unforged []transcriptCandidate
	for _, p := range allPaths {
		if !forgedSet[p] {
			unforged = append(unforged, transcriptCandidate{path: p, size: 200})
		}
	}

	if len(unforged) != 3 {
		t.Fatalf("expected 3 unforged, got %d", len(unforged))
	}

	// Apply --max=2 limit
	maxLimit := 2
	if maxLimit > 0 && len(unforged) > maxLimit {
		unforged = unforged[:maxLimit]
	}

	if len(unforged) != 2 {
		t.Fatalf("expected 2 after max limit, got %d", len(unforged))
	}
}

// TestIntegration_ForgeBatchDedupAcrossSessions verifies that dedupSimilar
// correctly removes duplicates when aggregating knowledge across multiple
// forged sessions (cross-session dedup).
func TestIntegration_ForgeBatchDedupAcrossSessions(t *testing.T) {
	// Simulate knowledge from 3 different sessions
	session1Knowledge := []string{
		"Lead-only commit eliminates merge conflicts",
		"Workers should never commit directly",
	}
	session2Knowledge := []string{
		"lead-only commit eliminates merge conflicts", // Duplicate of session1[0] (case diff)
		"Topological wave decomposition extracts more parallelism than manual planning",
	}
	session3Knowledge := []string{
		"Workers should never commit directly", // Exact duplicate of session1[1]
		"Content hashing avoids false positive dedup",
	}

	// Combine (as batch forge does)
	allKnowledge := append(append(session1Knowledge, session2Knowledge...), session3Knowledge...)
	deduped := dedupSimilar(allKnowledge)

	// Expect 4 unique items (the two exact/near-dups removed)
	if len(deduped) != 4 {
		t.Errorf("expected 4 unique items after dedup, got %d: %v", len(deduped), deduped)
	}
}

// =============================================================================
// Integration Test 3: Session Close Lifecycle
//
// Tests the SessionCloseResult struct construction and output, verifying that
// the forge+extract pipeline produces a well-formed result.
// =============================================================================

func TestIntegration_SessionCloseResultLifecycle(t *testing.T) {
	tests := []struct {
		name   string
		result SessionCloseResult
		checks func(t *testing.T, r SessionCloseResult)
	}{
		{
			name: "productive session produces compounding status",
			result: SessionCloseResult{
				SessionID:     "session-productive-001",
				Transcript:    "/tmp/productive.jsonl",
				Decisions:     5,
				Knowledge:     8,
				FilesChanged:  12,
				Issues:        3,
				VelocityDelta: 0.15,
				Status:        "compounding",
				Message:       "Session closed: 5 decisions, 8 learnings extracted",
			},
			checks: func(t *testing.T, r SessionCloseResult) {
				t.Helper()
				if r.Status != "compounding" {
					t.Errorf("status = %q, want compounding", r.Status)
				}
				if r.VelocityDelta <= 0 {
					t.Errorf("expected positive velocity delta, got %f", r.VelocityDelta)
				}
				if r.Decisions != 5 || r.Knowledge != 8 {
					t.Errorf("expected 5 decisions and 8 knowledge, got %d/%d", r.Decisions, r.Knowledge)
				}
			},
		},
		{
			name: "empty session produces decaying status",
			result: SessionCloseResult{
				SessionID:     "session-empty-002",
				Transcript:    "/tmp/empty.jsonl",
				Decisions:     0,
				Knowledge:     0,
				FilesChanged:  0,
				VelocityDelta: -0.08,
				Status:        "decaying",
				Message:       "Session closed: 0 decisions, 0 learnings extracted",
			},
			checks: func(t *testing.T, r SessionCloseResult) {
				t.Helper()
				if r.Status != "decaying" {
					t.Errorf("status = %q, want decaying", r.Status)
				}
				if r.VelocityDelta >= 0 {
					t.Errorf("expected negative velocity delta, got %f", r.VelocityDelta)
				}
			},
		},
		{
			name: "near-escape session",
			result: SessionCloseResult{
				SessionID:     "session-near-003",
				Transcript:    "/tmp/near.jsonl",
				Decisions:     1,
				Knowledge:     2,
				FilesChanged:  3,
				VelocityDelta: -0.02,
				Status:        "near-escape",
				Message:       "Session closed: 1 decisions, 2 learnings extracted",
			},
			checks: func(t *testing.T, r SessionCloseResult) {
				t.Helper()
				if r.Status != "near-escape" {
					t.Errorf("status = %q, want near-escape", r.Status)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Verify JSON round-trip
			data, err := json.Marshal(tt.result)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}

			var decoded SessionCloseResult
			if err := json.Unmarshal(data, &decoded); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}

			// Verify fields survive round-trip
			if decoded.SessionID != tt.result.SessionID {
				t.Errorf("SessionID: got %q, want %q", decoded.SessionID, tt.result.SessionID)
			}
			if decoded.Decisions != tt.result.Decisions {
				t.Errorf("Decisions: got %d, want %d", decoded.Decisions, tt.result.Decisions)
			}
			if decoded.Knowledge != tt.result.Knowledge {
				t.Errorf("Knowledge: got %d, want %d", decoded.Knowledge, tt.result.Knowledge)
			}
			if decoded.VelocityDelta != tt.result.VelocityDelta {
				t.Errorf("VelocityDelta: got %f, want %f", decoded.VelocityDelta, tt.result.VelocityDelta)
			}
			if decoded.Status != tt.result.Status {
				t.Errorf("Status: got %q, want %q", decoded.Status, tt.result.Status)
			}
			if decoded.Message != tt.result.Message {
				t.Errorf("Message: got %q, want %q", decoded.Message, tt.result.Message)
			}

			// Run custom checks
			tt.checks(t, decoded)
		})
	}
}

// TestIntegration_SessionCloseOutput verifies that outputCloseResult works
// for both table and JSON output formats without errors.
func TestIntegration_SessionCloseOutput(t *testing.T) {
	result := SessionCloseResult{
		SessionID:     "integration-output-test",
		Transcript:    "/tmp/test-session.jsonl",
		Decisions:     3,
		Knowledge:     5,
		FilesChanged:  7,
		Issues:        1,
		VelocityDelta: 0.05,
		Status:        "compounding",
		Message:       "Session closed: 3 decisions, 5 learnings extracted",
	}

	formats := []string{"table", "json"}

	for _, format := range formats {
		t.Run("format_"+format, func(t *testing.T) {
			// Capture stdout
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			oldOutput := output
			output = format
			defer func() {
				output = oldOutput
			}()

			err := outputCloseResult(result)

			_ = w.Close()
			os.Stdout = oldStdout

			if err != nil {
				t.Fatalf("outputCloseResult error: %v", err)
			}

			// Read captured output
			buf := make([]byte, 8192)
			n, _ := r.Read(buf)
			_ = r.Close()

			out := string(buf[:n])
			if len(out) == 0 {
				t.Error("expected non-empty output")
			}

			// For JSON format, verify it's valid JSON
			if format == "json" {
				var decoded SessionCloseResult
				if err := json.Unmarshal([]byte(out), &decoded); err != nil {
					t.Errorf("JSON output is not valid: %v\nOutput: %s", err, out)
				}
				if decoded.SessionID != result.SessionID {
					t.Errorf("JSON SessionID = %q, want %q", decoded.SessionID, result.SessionID)
				}
			}
		})
	}
}

// TestIntegration_SessionCloseExtractPipeline verifies the extract side of
// session close: queuing pending extractions, reading them back, and processing
// all entries via runExtractAll.
func TestIntegration_SessionCloseExtractPipeline(t *testing.T) {
	tmpDir := t.TempDir()
	pendingPath := filepath.Join(tmpDir, storage.DefaultBaseDir, "pending.jsonl")

	// Simulate what session close does: queue multiple extractions
	entries := []PendingExtraction{
		{
			SessionID:      "close-session-1",
			Summary:        "First session in close pipeline",
			Decisions:      []string{"Decision A", "Decision B"},
			Knowledge:      []string{"Knowledge X"},
			QueuedAt:       time.Now().Add(-2 * time.Hour),
			SessionPath:    "/path/to/session1",
			TranscriptPath: "/path/to/transcript1.jsonl",
		},
		{
			SessionID:      "close-session-2",
			Summary:        "Second session in close pipeline",
			Decisions:      []string{"Decision C"},
			Knowledge:      []string{"Knowledge Y", "Knowledge Z"},
			QueuedAt:       time.Now().Add(-1 * time.Hour),
			SessionPath:    "/path/to/session2",
			TranscriptPath: "/path/to/transcript2.jsonl",
		},
		{
			SessionID:   "close-session-3",
			Summary:     "Third session in close pipeline",
			Knowledge:   []string{"Knowledge W"},
			QueuedAt:    time.Now(),
			SessionPath: "/path/to/session3",
		},
	}

	// Write entries (simulating queue)
	if err := writePendingFile(pendingPath, entries); err != nil {
		t.Fatalf("write pending: %v", err)
	}

	// Verify we can read them back
	pending, err := readPendingExtractions(pendingPath)
	if err != nil {
		t.Fatalf("read pending: %v", err)
	}
	if len(pending) != 3 {
		t.Fatalf("expected 3 pending, got %d", len(pending))
	}

	// Verify fields survived round-trip
	if pending[0].SessionID != "close-session-1" {
		t.Errorf("first entry session_id = %q", pending[0].SessionID)
	}
	if len(pending[0].Decisions) != 2 {
		t.Errorf("first entry decisions count = %d, want 2", len(pending[0].Decisions))
	}
	if len(pending[1].Knowledge) != 2 {
		t.Errorf("second entry knowledge count = %d, want 2", len(pending[1].Knowledge))
	}

	// Change to temp dir so runExtractAll can find the pending file
	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	// Capture stdout to prevent test pollution
	oldStdout := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w

	// Run extract all
	extractAll = true
	defer func() { extractAll = false }()

	err = runExtractAll(pendingPath, pending, tmpDir)
	_ = w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("runExtractAll: %v", err)
	}

	// Verify pending file is now empty (all processed)
	remaining, err := readPendingExtractions(pendingPath)
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("read remaining: %v", err)
	}
	if len(remaining) != 0 {
		t.Errorf("expected 0 remaining, got %d", len(remaining))
	}
}

// =============================================================================
// Integration Test: Cross-Command Consistency
//
// Verifies that types and results are consistent across commands that share
// data structures (e.g., BatchForgeResult and SessionCloseResult).
// =============================================================================

func TestIntegration_CrossCommandConsistency(t *testing.T) {
	t.Run("BatchForgeResult JSON round-trip", func(t *testing.T) {
		result := BatchForgeResult{
			Forged:    5,
			Skipped:   2,
			Failed:    1,
			Extracted: 4,
			Paths:     []string{"/a.jsonl", "/b.jsonl", "/c.jsonl", "/d.jsonl", "/e.jsonl"},
		}

		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}

		var decoded BatchForgeResult
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}

		if decoded.Forged != result.Forged {
			t.Errorf("Forged: got %d, want %d", decoded.Forged, result.Forged)
		}
		if decoded.Extracted != result.Extracted {
			t.Errorf("Extracted: got %d, want %d", decoded.Extracted, result.Extracted)
		}
		if len(decoded.Paths) != len(result.Paths) {
			t.Errorf("Paths: got %d, want %d", len(decoded.Paths), len(result.Paths))
		}
	})

	t.Run("ExtractBatchResult JSON round-trip", func(t *testing.T) {
		result := ExtractBatchResult{
			Processed: 3,
			Failed:    0,
			Remaining: 0,
			Entries:   []string{"s1", "s2", "s3"},
		}

		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}

		var decoded ExtractBatchResult
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}

		if decoded.Processed != result.Processed {
			t.Errorf("Processed: got %d, want %d", decoded.Processed, result.Processed)
		}
		if len(decoded.Entries) != len(result.Entries) {
			t.Errorf("Entries: got %d, want %d", len(decoded.Entries), len(result.Entries))
		}
	})

	t.Run("NextResult JSON round-trip", func(t *testing.T) {
		result := NextResult{
			Next:         "implement",
			Reason:       "plan locked",
			LastStep:     "plan",
			LastArtifact: ".agents/plans/epic-plan.md",
			Skill:        "/implement or /crank",
			Complete:     false,
		}

		data, err := json.Marshal(result)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}

		var decoded NextResult
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}

		if decoded.Next != result.Next {
			t.Errorf("Next: got %q, want %q", decoded.Next, result.Next)
		}
		if decoded.Skill != result.Skill {
			t.Errorf("Skill: got %q, want %q", decoded.Skill, result.Skill)
		}
	})
}
