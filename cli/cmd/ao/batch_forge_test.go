package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/parser"
	"github.com/boshu2/agentops/cli/internal/storage"
)

func TestNormalizeForDedup(t *testing.T) {
	tests := []struct {
		name     string
		a, b     string
		wantSame bool
	}{
		{
			name:     "exact duplicates",
			a:        "Lead-only commit eliminates merge conflicts",
			b:        "Lead-only commit eliminates merge conflicts",
			wantSame: true,
		},
		{
			name:     "case difference",
			a:        "Lead-Only Commit Pattern",
			b:        "lead-only commit pattern",
			wantSame: true,
		},
		{
			name:     "whitespace difference",
			a:        "  Lead-only  commit   pattern  ",
			b:        "Lead-only commit pattern",
			wantSame: true,
		},
		{
			name:     "trailing ellipsis stripped",
			a:        "Workers write files but never commit...",
			b:        "Workers write files but never commit",
			wantSame: true,
		},
		{
			name:     "distinct content with same 80-char prefix",
			a:        "Topological wave decomposition extracts parallelism from dependency graphs by grouping leaves — this is about WAVES",
			b:        "Topological wave decomposition extracts parallelism from dependency graphs by grouping leaves — this is about SORTING",
			wantSame: false,
		},
		{
			name:     "short similar strings are distinct",
			a:        "Use content hashing for dedup",
			b:        "Use content hashing for dedup detection",
			wantSame: false,
		},
		{
			name:     "completely different",
			a:        "Workers should never commit",
			b:        "Wave sizing follows dependency graph",
			wantSame: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			keyA := normalizeForDedup(tt.a)
			keyB := normalizeForDedup(tt.b)
			gotSame := keyA == keyB
			if gotSame != tt.wantSame {
				t.Errorf("normalizeForDedup(%q) == normalizeForDedup(%q): got %v, want %v\n  keyA=%s\n  keyB=%s",
					tt.a, tt.b, gotSame, tt.wantSame, keyA, keyB)
			}
		})
	}
}

func TestDedupSimilar(t *testing.T) {
	tests := []struct {
		name      string
		input     []string
		wantCount int
	}{
		{
			name:      "nil input",
			input:     nil,
			wantCount: 0,
		},
		{
			name:      "empty input",
			input:     []string{},
			wantCount: 0,
		},
		{
			name:      "no duplicates",
			input:     []string{"alpha", "beta", "gamma"},
			wantCount: 3,
		},
		{
			name:      "exact duplicates removed",
			input:     []string{"alpha", "beta", "alpha", "gamma", "beta"},
			wantCount: 3,
		},
		{
			name: "case-insensitive dedup",
			input: []string{
				"Lead-only commit pattern",
				"lead-only commit pattern",
				"LEAD-ONLY COMMIT PATTERN",
			},
			wantCount: 1,
		},
		{
			name: "preserves distinct long strings",
			input: []string{
				"Topological wave decomposition extracts parallelism from dependency graphs by grouping leaves — approach A",
				"Topological wave decomposition extracts parallelism from dependency graphs by grouping leaves — approach B",
			},
			wantCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := dedupSimilar(tt.input)
			if len(result) != tt.wantCount {
				t.Errorf("dedupSimilar() returned %d items, want %d. Items: %v", len(result), tt.wantCount, result)
			}
		})
	}
}

func TestFindPendingTranscripts(t *testing.T) {
	// Create temp directory with test transcript files
	tmpDir, err := os.MkdirTemp("", "batch_forge_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create some test JSONL files (must be > 100 bytes to pass the size filter)
	for _, name := range []string{"session1.jsonl", "session2.jsonl"} {
		content := []byte(`{"role":"user","content":"hello world, this is a test message with enough content"}` + "\n" +
			`{"role":"assistant","content":"hi there, this is a sufficiently long response to exceed the 100 byte minimum"}` + "\n")
		if err := os.WriteFile(filepath.Join(tmpDir, name), content, 0644); err != nil {
			t.Fatal(err)
		}
	}

	// Create a file too small to be a transcript
	if err := os.WriteFile(filepath.Join(tmpDir, "tiny.jsonl"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a non-JSONL file
	if err := os.WriteFile(filepath.Join(tmpDir, "readme.md"), []byte("# Hello"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a subagents directory that should be skipped
	subDir := filepath.Join(tmpDir, "subagents")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "sub.jsonl"), []byte(`{"role":"user","content":"skip me please"}`+"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	candidates, err := findPendingTranscripts(tmpDir)
	if err != nil {
		t.Fatalf("findPendingTranscripts: %v", err)
	}

	if len(candidates) != 2 {
		t.Errorf("got %d candidates, want 2", len(candidates))
		for _, c := range candidates {
			t.Logf("  candidate: %s (size=%d)", c.path, c.size)
		}
	}

	// Verify sorted by mod time (oldest first)
	if len(candidates) >= 2 {
		if candidates[0].modTime.After(candidates[1].modTime) {
			t.Error("candidates not sorted by modification time (oldest first)")
		}
	}
}

func TestFindPendingTranscriptsEmptyDir(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "batch_forge_empty")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	candidates, err := findPendingTranscripts(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(candidates) != 0 {
		t.Errorf("got %d candidates, want 0", len(candidates))
	}
}

func TestHumanSize(t *testing.T) {
	tests := []struct {
		bytes int64
		want  string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
	}

	for _, tt := range tests {
		got := humanSize(tt.bytes)
		if got != tt.want {
			t.Errorf("humanSize(%d) = %q, want %q", tt.bytes, got, tt.want)
		}
	}
}

func TestTranscriptCandidateFields(t *testing.T) {
	now := time.Now()
	c := transcriptCandidate{
		path:    "/tmp/test.jsonl",
		modTime: now,
		size:    1234,
	}

	if c.path != "/tmp/test.jsonl" {
		t.Error("path mismatch")
	}
	if c.size != 1234 {
		t.Error("size mismatch")
	}
	if !c.modTime.Equal(now) {
		t.Error("modTime mismatch")
	}
}

func TestLoadForgedIndex(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()
	indexPath := filepath.Join(tmpDir, "forged.jsonl")

	// Test empty index (file doesn't exist)
	forgedSet, err := loadForgedIndex(indexPath)
	if err != nil {
		t.Fatalf("loadForgedIndex failed: %v", err)
	}
	if len(forgedSet) != 0 {
		t.Errorf("expected empty set, got %d entries", len(forgedSet))
	}

	// Write some test records
	records := []ForgedRecord{
		{Path: "/path/to/session1.jsonl", ForgedAt: time.Now(), Session: "session-1"},
		{Path: "/path/to/session2.jsonl", ForgedAt: time.Now(), Session: "session-2"},
		{Path: "/path/to/session3.jsonl", ForgedAt: time.Now(), Session: "session-3"},
	}

	f, err := os.Create(indexPath)
	if err != nil {
		t.Fatalf("create index file: %v", err)
	}
	for _, record := range records {
		data, err := json.Marshal(record)
		if err != nil {
			t.Fatalf("marshal record: %v", err)
		}
		_, _ = f.Write(append(data, '\n'))
	}
	_ = f.Close()

	// Load index
	forgedSet, err = loadForgedIndex(indexPath)
	if err != nil {
		t.Fatalf("loadForgedIndex failed: %v", err)
	}

	// Verify all paths are in set
	if len(forgedSet) != 3 {
		t.Errorf("expected 3 entries, got %d", len(forgedSet))
	}
	for _, record := range records {
		if !forgedSet[record.Path] {
			t.Errorf("expected path %s to be in set", record.Path)
		}
	}
}

func TestAppendForgedRecord(t *testing.T) {
	tmpDir := t.TempDir()
	indexPath := filepath.Join(tmpDir, "forged.jsonl")

	record1 := ForgedRecord{
		Path:     "/path/to/session1.jsonl",
		ForgedAt: time.Now(),
		Session:  "session-1",
	}

	// Append first record
	if err := appendForgedRecord(indexPath, record1); err != nil {
		t.Fatalf("appendForgedRecord failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(indexPath); os.IsNotExist(err) {
		t.Fatal("expected index file to exist")
	}

	// Append second record
	record2 := ForgedRecord{
		Path:     "/path/to/session2.jsonl",
		ForgedAt: time.Now(),
		Session:  "session-2",
	}
	if err := appendForgedRecord(indexPath, record2); err != nil {
		t.Fatalf("appendForgedRecord failed on second write: %v", err)
	}

	// Load and verify
	forgedSet, err := loadForgedIndex(indexPath)
	if err != nil {
		t.Fatalf("loadForgedIndex failed: %v", err)
	}

	if len(forgedSet) != 2 {
		t.Errorf("expected 2 entries, got %d", len(forgedSet))
	}
	if !forgedSet[record1.Path] {
		t.Errorf("expected path %s to be in set", record1.Path)
	}
	if !forgedSet[record2.Path] {
		t.Errorf("expected path %s to be in set", record2.Path)
	}
}

func TestBatchForgeSkipsAlreadyForged(t *testing.T) {
	tmpDir := t.TempDir()
	indexPath := filepath.Join(tmpDir, "forged.jsonl")

	// Create forged index with one entry
	record := ForgedRecord{
		Path:     "/already/forged.jsonl",
		ForgedAt: time.Now(),
		Session:  "session-old",
	}
	if err := appendForgedRecord(indexPath, record); err != nil {
		t.Fatalf("appendForgedRecord failed: %v", err)
	}

	// Load index
	forgedSet, err := loadForgedIndex(indexPath)
	if err != nil {
		t.Fatalf("loadForgedIndex failed: %v", err)
	}

	// Simulate filtering transcripts
	candidates := []transcriptCandidate{
		{path: "/already/forged.jsonl", modTime: time.Now(), size: 1000},
		{path: "/new/transcript.jsonl", modTime: time.Now(), size: 2000},
	}

	var unforged []transcriptCandidate
	for _, c := range candidates {
		if !forgedSet[c.path] {
			unforged = append(unforged, c)
		}
	}

	// Verify only new transcript remains
	if len(unforged) != 1 {
		t.Errorf("expected 1 unforged transcript, got %d", len(unforged))
	}
	if unforged[0].path != "/new/transcript.jsonl" {
		t.Errorf("expected /new/transcript.jsonl, got %s", unforged[0].path)
	}
}

func TestBatchForgeMaxFlag(t *testing.T) {
	// Simulate --max flag limiting transcripts
	candidates := []transcriptCandidate{
		{path: "/transcript1.jsonl", modTime: time.Now(), size: 1000},
		{path: "/transcript2.jsonl", modTime: time.Now(), size: 1000},
		{path: "/transcript3.jsonl", modTime: time.Now(), size: 1000},
		{path: "/transcript4.jsonl", modTime: time.Now(), size: 1000},
		{path: "/transcript5.jsonl", modTime: time.Now(), size: 1000},
	}

	maxLimit := 3
	var limited []transcriptCandidate
	if maxLimit > 0 && len(candidates) > maxLimit {
		limited = candidates[:maxLimit]
	} else {
		limited = candidates
	}

	if len(limited) != maxLimit {
		t.Errorf("expected %d transcripts after limit, got %d", maxLimit, len(limited))
	}

	// Verify we got the first 3
	for i := range maxLimit {
		if limited[i].path != candidates[i].path {
			t.Errorf("expected %s at position %d, got %s", candidates[i].path, i, limited[i].path)
		}
	}
}

func TestBatchForgeResult(t *testing.T) {
	// Test JSON marshaling of BatchForgeResult
	result := BatchForgeResult{
		Forged:    10,
		Skipped:   3,
		Failed:    1,
		Extracted: 8,
		Paths:     []string{"/path1.jsonl", "/path2.jsonl"},
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	// Unmarshal and verify
	var decoded BatchForgeResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if decoded.Forged != result.Forged {
		t.Errorf("expected Forged=%d, got %d", result.Forged, decoded.Forged)
	}
	if decoded.Skipped != result.Skipped {
		t.Errorf("expected Skipped=%d, got %d", result.Skipped, decoded.Skipped)
	}
	if decoded.Failed != result.Failed {
		t.Errorf("expected Failed=%d, got %d", result.Failed, decoded.Failed)
	}
	if decoded.Extracted != result.Extracted {
		t.Errorf("expected Extracted=%d, got %d", result.Extracted, decoded.Extracted)
	}
	if len(decoded.Paths) != len(result.Paths) {
		t.Errorf("expected %d paths, got %d", len(result.Paths), len(decoded.Paths))
	}
}

func TestLoadAndFilterTranscripts_RespectsForgedIndex(t *testing.T) {
	tmpDir := t.TempDir()
	transcriptDir := filepath.Join(tmpDir, "transcripts")
	if err := os.MkdirAll(transcriptDir, 0o755); err != nil {
		t.Fatal(err)
	}

	transcriptPath := filepath.Join(transcriptDir, "session.jsonl")
	content := `{"role":"user","content":"this transcript has enough content to exceed one hundred bytes for candidate detection"}
{"role":"assistant","content":"batch forging should discover this file and then skip it once indexed as forged"}`
	if err := os.WriteFile(transcriptPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	unforged, skipped, forgedIndexPath, err := loadAndFilterTranscripts(tmpDir, transcriptDir, 0)
	if err != nil {
		t.Fatalf("loadAndFilterTranscripts first run: %v", err)
	}
	if len(unforged) != 1 || skipped != 0 {
		t.Fatalf("expected one unforged transcript on first run, got len=%d skipped=%d", len(unforged), skipped)
	}
	if !strings.HasSuffix(forgedIndexPath, filepath.Join(".agents", "ao", "forged.jsonl")) {
		t.Fatalf("unexpected forged index path: %s", forgedIndexPath)
	}

	if err := appendForgedRecord(forgedIndexPath, ForgedRecord{
		Path:     transcriptPath,
		ForgedAt: time.Now(),
		Session:  "session-1",
	}); err != nil {
		t.Fatalf("appendForgedRecord: %v", err)
	}

	unforged, skipped, _, err = loadAndFilterTranscripts(tmpDir, transcriptDir, 0)
	if err != nil {
		t.Fatalf("loadAndFilterTranscripts second run: %v", err)
	}
	if len(unforged) != 0 || skipped != 1 {
		t.Fatalf("expected transcript to be skipped after forge index update, got len=%d skipped=%d", len(unforged), skipped)
	}
}

func TestRunForgeBatch_NoPendingTranscripts(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	oldBatchDir, oldBatchMax, oldBatchExtract := batchDir, batchMax, batchExtract
	oldOutput := output
	batchDir = filepath.Join(tmpDir, "empty-transcripts")
	batchMax = 0
	batchExtract = false
	output = "text"
	t.Cleanup(func() {
		batchDir, batchMax, batchExtract = oldBatchDir, oldBatchMax, oldBatchExtract
		output = oldOutput
	})

	if err := os.MkdirAll(batchDir, 0o755); err != nil {
		t.Fatalf("mkdir batch dir: %v", err)
	}

	origStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	err := runForgeBatch(nil, nil)
	w.Close()
	os.Stdout = origStdout
	if err != nil {
		t.Fatalf("runForgeBatch: %v", err)
	}

	var buf [1024]byte
	n, _ := r.Read(buf[:])
	r.Close()
	if !strings.Contains(string(buf[:n]), "No pending transcripts found.") {
		t.Fatalf("expected no-pending message, got: %s", string(buf[:n]))
	}
}

func TestRunForgeBatch_DryRunAppliesMaxLimit(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	transcriptDir := filepath.Join(tmpDir, "transcripts")
	if err := os.MkdirAll(transcriptDir, 0o755); err != nil {
		t.Fatalf("mkdir transcript dir: %v", err)
	}

	baseTime := time.Now().Add(-3 * time.Hour)
	for i := 1; i <= 3; i++ {
		path := filepath.Join(transcriptDir, fmt.Sprintf("session-%d.jsonl", i))
		content := strings.Repeat(
			fmt.Sprintf(`{"role":"user","content":"session %d has enough content to exceed the transcript size threshold for dry-run coverage"}
`, i),
			2,
		)
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write transcript %d: %v", i, err)
		}
		ts := baseTime.Add(time.Duration(i) * time.Minute)
		if err := os.Chtimes(path, ts, ts); err != nil {
			t.Fatalf("chtimes transcript %d: %v", i, err)
		}
	}

	oldBatchDir, oldBatchMax, oldBatchExtract := batchDir, batchMax, batchExtract
	oldDryRun, oldOutput := dryRun, output
	t.Cleanup(func() {
		batchDir, batchMax, batchExtract = oldBatchDir, oldBatchMax, oldBatchExtract
		dryRun, output = oldDryRun, oldOutput
	})

	batchDir = transcriptDir
	batchMax = 2
	batchExtract = false
	dryRun = true
	output = "table"

	stdout, err := captureStdout(t, func() error {
		return runForgeBatch(nil, nil)
	})
	if err != nil {
		t.Fatalf("runForgeBatch: %v", err)
	}

	if !strings.Contains(stdout, "[dry-run] Would process 2 transcript(s) (skipped 0):") {
		t.Fatalf("expected dry-run summary, got %q", stdout)
	}
	if !strings.Contains(stdout, "session-1.jsonl") || !strings.Contains(stdout, "session-2.jsonl") {
		t.Fatalf("expected first two transcripts in output, got %q", stdout)
	}
	if strings.Contains(stdout, "session-3.jsonl") {
		t.Fatalf("expected max limit to exclude session-3.jsonl, got %q", stdout)
	}
}

func TestRunForgeBatch_ProcessesTranscript(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	transcriptDir := filepath.Join(tmpDir, "transcripts")
	if err := os.MkdirAll(transcriptDir, 0o755); err != nil {
		t.Fatalf("mkdir transcript dir: %v", err)
	}

	transcriptPath := filepath.Join(transcriptDir, "session.jsonl")
	lines := strings.Join([]string{
		`{"type":"summary","sessionId":"sess-batch","timestamp":"2024-01-01T00:00:00Z","summary":"Batch forge summary"}`,
		`{"type":"assistant","role":"assistant","content":"Working on a task with enough detail to keep the transcript comfortably above the batch filter threshold.","sessionId":"sess-batch","timestamp":"2024-01-01T00:01:00Z"}`,
	}, "\n") + "\n"
	if err := os.WriteFile(transcriptPath, []byte(lines), 0o644); err != nil {
		t.Fatalf("write transcript: %v", err)
	}

	oldBatchDir, oldBatchMax, oldBatchExtract := batchDir, batchMax, batchExtract
	oldDryRun, oldOutput := dryRun, output
	t.Cleanup(func() {
		batchDir, batchMax, batchExtract = oldBatchDir, oldBatchMax, oldBatchExtract
		dryRun, output = oldDryRun, oldOutput
	})

	batchDir = transcriptDir
	batchMax = 0
	batchExtract = false
	dryRun = false
	output = "table"

	stdout, err := captureStdout(t, func() error {
		return runForgeBatch(nil, nil)
	})
	if err != nil {
		t.Fatalf("runForgeBatch: %v", err)
	}

	for _, want := range []string{
		"Found 1 transcript(s) to process (skipped 0 already forged).",
		"[1/1] Processing session.jsonl...",
		"--- Batch Forge Summary ---",
		"Transcripts processed: 1",
		"Failed:                0",
	} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected %q in output, got %q", want, stdout)
		}
	}

	forgedIndexPath := filepath.Join(tmpDir, storage.DefaultBaseDir, "forged.jsonl")
	if _, err := os.Stat(forgedIndexPath); err != nil {
		t.Fatalf("expected forged index to be created: %v", err)
	}
}

func TestRunBatchExtractionStep_DryRunPendingExtractions(t *testing.T) {
	tmpDir := t.TempDir()
	pendingPath := filepath.Join(tmpDir, storage.DefaultBaseDir, "pending.jsonl")
	entries := []PendingExtraction{
		{
			SessionID:      "session-1",
			Summary:        "Batch forge extraction dry run",
			TranscriptPath: "/tmp/transcript.jsonl",
			QueuedAt:       time.Now(),
		},
	}
	if err := writePendingFile(pendingPath, entries); err != nil {
		t.Fatalf("writePendingFile: %v", err)
	}

	origBatchExtract := batchExtract
	origDryRun := dryRun
	origOutput := output
	t.Cleanup(func() {
		batchExtract = origBatchExtract
		dryRun = origDryRun
		output = origOutput
	})

	batchExtract = true
	dryRun = true
	output = "json"

	stdout, err := captureStdout(t, func() error {
		extracted := runBatchExtractionStep(tmpDir, 2)
		if extracted != 1 {
			t.Fatalf("runBatchExtractionStep() = %d, want 1", extracted)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("captureStdout: %v", err)
	}
	if !strings.Contains(stdout, "Triggering extraction for 2 session(s)...") {
		t.Fatalf("expected extraction banner, got %q", stdout)
	}
	if !strings.Contains(stdout, "\"remaining\": 1") {
		t.Fatalf("expected dry-run JSON summary, got %q", stdout)
	}
	if !strings.Contains(stdout, "session-1") {
		t.Fatalf("expected session id in dry-run output, got %q", stdout)
	}
}

func TestOutputBatchForgeResult_JSON(t *testing.T) {
	origOutput := output
	t.Cleanup(func() { output = origOutput })
	output = "json"

	stdout, err := captureStdout(t, func() error {
		return outputBatchForgeResult(
			"/tmp/.agents/ao",
			3,
			2,
			1,
			7,
			9,
			4,
			2,
			[]string{"learning-1", "learning-2"},
			[]string{"decision-1"},
			[]string{"/tmp/a.jsonl", "/tmp/b.jsonl", "/tmp/c.jsonl"},
		)
	})
	if err != nil {
		t.Fatalf("outputBatchForgeResult: %v", err)
	}

	var got BatchForgeResult
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &got); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%q", err, stdout)
	}
	if got.Forged != 3 || got.Skipped != 2 || got.Failed != 1 || got.Extracted != 2 {
		t.Fatalf("unexpected result payload: %+v", got)
	}
	if len(got.Paths) != 3 {
		t.Fatalf("expected 3 paths, got %d", len(got.Paths))
	}
}

func TestOutputBatchForgeResult_Text(t *testing.T) {
	origOutput := output
	t.Cleanup(func() { output = origOutput })
	output = "table"

	stdout, err := captureStdout(t, func() error {
		return outputBatchForgeResult(
			"/tmp/.agents/ao",
			2,
			1,
			0,
			5,
			6,
			3,
			1,
			[]string{"learning-1", "learning-2"},
			[]string{"decision-1", "decision-2"},
			[]string{"/tmp/a.jsonl", "/tmp/b.jsonl"},
		)
	})
	if err != nil {
		t.Fatalf("outputBatchForgeResult: %v", err)
	}

	for _, want := range []string{
		"--- Batch Forge Summary ---",
		"Transcripts processed: 2",
		"Skipped (already):     1",
		"Failed:                0",
		"Decisions extracted:   5",
		"Learnings extracted:   6",
		"Duplicates removed:    3",
		"Unique decisions:      2",
		"Unique learnings:      2",
		"Extractions processed: 1",
		"Output:                /tmp/.agents/ao",
	} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected %q in output, got %q", want, stdout)
		}
	}
}

// ---------------------------------------------------------------------------
// batch_forge.go — dedupSimilar
// ---------------------------------------------------------------------------

func TestBatchForge_dedupSimilar_Deep(t *testing.T) {
	tests := []struct {
		name  string
		items []string
		want  int
	}{
		{
			name:  "nil input",
			items: nil,
			want:  0,
		},
		{
			name:  "empty slice",
			items: []string{},
			want:  0,
		},
		{
			name:  "no duplicates",
			items: []string{"alpha", "beta", "gamma"},
			want:  3,
		},
		{
			name:  "exact duplicates",
			items: []string{"alpha", "beta", "alpha", "gamma", "beta"},
			want:  3,
		},
		{
			name:  "case-insensitive duplicates",
			items: []string{"Hello World", "hello world", "HELLO WORLD"},
			want:  1,
		},
		{
			name:  "whitespace normalization",
			items: []string{"hello  world", "hello world", "  hello world  "},
			want:  1,
		},
		{
			name:  "ellipsis stripping",
			items: []string{"some long text...", "some long text"},
			want:  1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := dedupSimilar(tc.items)
			if tc.want == 0 && got != nil && len(got) != 0 {
				t.Errorf("expected nil/empty, got %d items", len(got))
			} else if tc.want > 0 && len(got) != tc.want {
				t.Errorf("dedupSimilar() returned %d items, want %d", len(got), tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// batch_forge.go — normalizeForDedup
// ---------------------------------------------------------------------------

func TestBatchForge_normalizeForDedup_Deep(t *testing.T) {
	// Same input should always produce same hash
	hash1 := normalizeForDedup("Hello World")
	hash2 := normalizeForDedup("hello world")
	hash3 := normalizeForDedup("  HELLO   WORLD  ")

	if hash1 != hash2 {
		t.Errorf("case-insensitive normalization failed: %q != %q", hash1, hash2)
	}
	if hash1 != hash3 {
		t.Errorf("whitespace normalization failed: %q != %q", hash1, hash3)
	}

	// Different content should produce different hashes
	hashA := normalizeForDedup("alpha content")
	hashB := normalizeForDedup("beta content")
	if hashA == hashB {
		t.Errorf("expected different hashes for different content")
	}

	// Ellipsis stripping
	hashWithEllipsis := normalizeForDedup("some text...")
	hashWithout := normalizeForDedup("some text")
	if hashWithEllipsis != hashWithout {
		t.Errorf("ellipsis stripping failed: %q != %q", hashWithEllipsis, hashWithout)
	}
}

// ---------------------------------------------------------------------------
// batch_forge.go — humanSize
// ---------------------------------------------------------------------------

func TestBatchForge_humanSize_Deep(t *testing.T) {
	tests := []struct {
		bytes int64
		want  string
	}{
		{0, "0 B"},
		{100, "100 B"},
		{1023, "1023 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{1073741824, "1.0 GB"},
	}

	for _, tc := range tests {
		got := humanSize(tc.bytes)
		if got != tc.want {
			t.Errorf("humanSize(%d) = %q, want %q", tc.bytes, got, tc.want)
		}
	}
}

// ---------------------------------------------------------------------------
// batch_forge.go — loadForgedIndex / appendForgedRecord
// ---------------------------------------------------------------------------

func TestBatchForge_loadForgedIndex_Deep(t *testing.T) {
	t.Run("file does not exist returns empty set", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "nonexistent.jsonl")

		set, err := loadForgedIndex(path)
		if err != nil {
			t.Fatalf("loadForgedIndex: %v", err)
		}
		if len(set) != 0 {
			t.Errorf("expected empty set, got %d entries", len(set))
		}
	})

	t.Run("reads valid records", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "forged.jsonl")

		records := []ForgedRecord{
			{Path: "/tmp/a.jsonl", ForgedAt: time.Now(), Session: "sess-1"},
			{Path: "/tmp/b.jsonl", ForgedAt: time.Now(), Session: "sess-2"},
		}

		var lines []byte
		for _, r := range records {
			data, _ := json.Marshal(r)
			lines = append(lines, data...)
			lines = append(lines, '\n')
		}
		if err := os.WriteFile(path, lines, 0644); err != nil {
			t.Fatalf("write: %v", err)
		}

		set, err := loadForgedIndex(path)
		if err != nil {
			t.Fatalf("loadForgedIndex: %v", err)
		}
		if len(set) != 2 {
			t.Errorf("expected 2 entries, got %d", len(set))
		}
		if !set["/tmp/a.jsonl"] {
			t.Error("expected /tmp/a.jsonl in set")
		}
	})

	t.Run("skips malformed lines", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "forged.jsonl")

		content := `{"path":"/good.jsonl","forged_at":"2026-01-01T00:00:00Z"}
not-json-line
{"path":"/also-good.jsonl","forged_at":"2026-01-02T00:00:00Z"}
`
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("write: %v", err)
		}

		set, err := loadForgedIndex(path)
		if err != nil {
			t.Fatalf("loadForgedIndex: %v", err)
		}
		if len(set) != 2 {
			t.Errorf("expected 2 valid entries, got %d", len(set))
		}
	})
}

func TestBatchForge_appendForgedRecord_Deep(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "subdir", "forged.jsonl")

	record := ForgedRecord{
		Path:     "/tmp/test.jsonl",
		ForgedAt: time.Now(),
		Session:  "sess-abc",
	}

	// Should create directory and file
	err := appendForgedRecord(path, record)
	if err != nil {
		t.Fatalf("appendForgedRecord: %v", err)
	}

	// Verify file exists and contains the record
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	if !strings.Contains(string(data), "/tmp/test.jsonl") {
		t.Error("expected record path in file")
	}

	// Append another record
	record2 := ForgedRecord{
		Path:     "/tmp/test2.jsonl",
		ForgedAt: time.Now(),
		Session:  "sess-def",
	}
	if err := appendForgedRecord(path, record2); err != nil {
		t.Fatalf("appendForgedRecord second: %v", err)
	}

	// Load back and verify both
	set, err := loadForgedIndex(path)
	if err != nil {
		t.Fatalf("loadForgedIndex: %v", err)
	}
	if len(set) != 2 {
		t.Errorf("expected 2 records, got %d", len(set))
	}
}

// ---------------------------------------------------------------------------
// batch_forge.go — batchForgeAccumulator
// ---------------------------------------------------------------------------

func TestBatchForge_accumulator(t *testing.T) {
	var acc batchForgeAccumulator

	// Accumulate a success
	acc.accumulate(true, []string{"dec1", "dec2"}, []string{"know1"}, "/tmp/a.jsonl")

	if acc.processed != 1 {
		t.Errorf("processed: got %d, want 1", acc.processed)
	}
	if acc.totalDecisions != 2 {
		t.Errorf("totalDecisions: got %d, want 2", acc.totalDecisions)
	}
	if acc.totalKnowledge != 1 {
		t.Errorf("totalKnowledge: got %d, want 1", acc.totalKnowledge)
	}

	// Accumulate a failure
	acc.accumulate(false, nil, nil, "")

	if acc.failed != 1 {
		t.Errorf("failed: got %d, want 1", acc.failed)
	}
	if acc.processed != 1 {
		t.Errorf("processed should still be 1, got %d", acc.processed)
	}

	// Accumulate another success
	acc.accumulate(true, []string{"dec3"}, []string{"know2", "know3"}, "/tmp/b.jsonl")

	if acc.processed != 2 {
		t.Errorf("processed: got %d, want 2", acc.processed)
	}
	if acc.totalDecisions != 3 {
		t.Errorf("totalDecisions: got %d, want 3", acc.totalDecisions)
	}
	if acc.totalKnowledge != 3 {
		t.Errorf("totalKnowledge: got %d, want 3", acc.totalKnowledge)
	}
	if len(acc.processedPaths) != 2 {
		t.Errorf("processedPaths: got %d, want 2", len(acc.processedPaths))
	}
}

// ---------------------------------------------------------------------------
// batch_forge.go — forgeSingleTranscript
// ---------------------------------------------------------------------------

func TestForgeSingleTranscript_nonexistentFile(t *testing.T) {
	tmp := t.TempDir()
	fs := storage.NewFileStorage(storage.WithBaseDir(filepath.Join(tmp, ".agents", "ao")))
	if err := fs.Init(); err != nil {
		t.Fatalf("fs.Init: %v", err)
	}
	p := parser.NewParser()
	extractor := parser.NewExtractor()

	candidate := transcriptCandidate{path: filepath.Join(tmp, "nonexistent.jsonl")}
	ok, decisions, knowledge, _ := forgeSingleTranscript(0, 1, candidate, fs, p, extractor, "")
	if ok {
		t.Error("expected false for nonexistent file")
	}
	if decisions != nil {
		t.Errorf("expected nil decisions, got %v", decisions)
	}
	if knowledge != nil {
		t.Errorf("expected nil knowledge, got %v", knowledge)
	}
}

func TestForgeSingleTranscript_happyPath(t *testing.T) {
	tmp := t.TempDir()
	baseDir := filepath.Join(tmp, ".agents", "ao")
	fs := storage.NewFileStorage(storage.WithBaseDir(baseDir))
	if err := fs.Init(); err != nil {
		t.Fatalf("fs.Init: %v", err)
	}
	p := parser.NewParser()
	p.MaxContentLength = 0
	extractor := parser.NewExtractor()

	// Minimal valid JSONL transcript (two newline-separated JSON objects)
	transcriptPath := filepath.Join(tmp, "session.jsonl")
	lines := strings.Join([]string{
		`{"type":"summary","sessionId":"sess-test","timestamp":"2024-01-01T00:00:00Z"}`,
		`{"type":"assistant","role":"assistant","content":"Working on a task","sessionId":"sess-test","timestamp":"2024-01-01T00:01:00Z"}`,
	}, "\n") + "\n"
	if err := os.WriteFile(transcriptPath, []byte(lines), 0644); err != nil {
		t.Fatal(err)
	}

	forgedIndexPath := filepath.Join(tmp, "forged.jsonl")
	candidate := transcriptCandidate{path: transcriptPath}
	ok, _, _, _ := forgeSingleTranscript(0, 1, candidate, fs, p, extractor, forgedIndexPath)
	if !ok {
		t.Error("expected true for valid transcript")
	}
}

func TestTriggerExtraction_emptyPendingFile(t *testing.T) {
	tmp := t.TempDir()
	pendingDir := filepath.Join(tmp, storage.DefaultBaseDir)
	if err := os.MkdirAll(pendingDir, 0755); err != nil {
		t.Fatal(err)
	}
	pendingPath := filepath.Join(pendingDir, "pending.jsonl")
	if err := os.WriteFile(pendingPath, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	count, err := triggerExtraction(tmp)
	if err != nil {
		t.Fatalf("triggerExtraction empty: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0, got %d", count)
	}
}
