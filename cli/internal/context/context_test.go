package context

import (
	"errors"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewBudgetTracker(t *testing.T) {
	bt := NewBudgetTracker("test-session")

	if bt.SessionID != "test-session" {
		t.Errorf("expected SessionID test-session, got %s", bt.SessionID)
	}
	if bt.MaxTokens != DefaultMaxTokens {
		t.Errorf("expected MaxTokens %d, got %d", DefaultMaxTokens, bt.MaxTokens)
	}
	if bt.EstimatedUsage != 0 {
		t.Errorf("expected EstimatedUsage 0, got %d", bt.EstimatedUsage)
	}
}

func TestBudgetTrackerUsage(t *testing.T) {
	bt := NewBudgetTracker("test")
	bt.MaxTokens = 100000

	bt.UpdateUsage(40000)
	if bt.GetUsagePercent() != 0.4 {
		t.Errorf("expected 40%% usage, got %.2f", bt.GetUsagePercent())
	}

	bt.AddTokens(20000)
	if bt.EstimatedUsage != 60000 {
		t.Errorf("expected 60000 tokens, got %d", bt.EstimatedUsage)
	}
}

func TestBudgetTrackerStatus(t *testing.T) {
	bt := NewBudgetTracker("test")
	bt.MaxTokens = 100000

	// Optimal
	bt.UpdateUsage(30000)
	if bt.GetStatus() != StatusOptimal {
		t.Errorf("expected OPTIMAL at 30%%, got %s", bt.GetStatus())
	}

	// Warning
	bt.UpdateUsage(65000)
	if bt.GetStatus() != StatusWarning {
		t.Errorf("expected WARNING at 65%%, got %s", bt.GetStatus())
	}

	// Critical
	bt.UpdateUsage(85000)
	if bt.GetStatus() != StatusCritical {
		t.Errorf("expected CRITICAL at 85%%, got %s", bt.GetStatus())
	}
}

func TestBudgetTrackerNeedsSummarization(t *testing.T) {
	bt := NewBudgetTracker("test")
	bt.MaxTokens = 100000

	bt.UpdateUsage(70000)
	if bt.NeedsSummarization() {
		t.Error("should not need summarization at 70%")
	}

	bt.UpdateUsage(85000)
	if !bt.NeedsSummarization() {
		t.Error("should need summarization at 85%")
	}
}

func TestBudgetTrackerCheckpoints(t *testing.T) {
	bt := NewBudgetTracker("test")
	bt.MaxTokens = 100000
	bt.UpdateUsage(50000)

	cp := bt.CreateCheckpoint("cp1", "Completed feature X", []string{"file1.go"}, "passing")

	if cp.ID != "cp1" {
		t.Errorf("expected checkpoint ID cp1, got %s", cp.ID)
	}
	if cp.TokenUsage != 50000 {
		t.Errorf("expected TokenUsage 50000, got %d", cp.TokenUsage)
	}
	if len(bt.Checkpoints) != 1 {
		t.Errorf("expected 1 checkpoint, got %d", len(bt.Checkpoints))
	}

	last := bt.GetLastCheckpoint()
	if last == nil || last.ID != "cp1" {
		t.Error("GetLastCheckpoint failed")
	}
}

func TestBudgetTrackerRecordSummarization(t *testing.T) {
	bt := NewBudgetTracker("test")
	bt.MaxTokens = 100000
	bt.UpdateUsage(90000)

	bt.RecordSummarization(90000, 50000, []string{"file_changes", "failing_tests"})

	if bt.EstimatedUsage != 50000 {
		t.Errorf("expected usage updated to 50000, got %d", bt.EstimatedUsage)
	}
	if len(bt.SummarizationEvents) != 1 {
		t.Errorf("expected 1 summarization event, got %d", len(bt.SummarizationEvents))
	}

	event := bt.SummarizationEvents[0]
	if event.TokensSaved != 40000 {
		t.Errorf("expected 40000 tokens saved, got %d", event.TokensSaved)
	}
}

func TestBudgetTrackerReport(t *testing.T) {
	bt := NewBudgetTracker("test")
	bt.MaxTokens = 100000
	bt.UpdateUsage(60000)
	bt.CreateCheckpoint("cp1", "test", nil, "passing")

	report := bt.GetReport()

	if report.SessionID != "test" {
		t.Errorf("expected SessionID test, got %s", report.SessionID)
	}
	if report.UsagePercent != 60 {
		t.Errorf("expected UsagePercent 60, got %.2f", report.UsagePercent)
	}
	if report.TokensRemaining != 40000 {
		t.Errorf("expected TokensRemaining 40000, got %d", report.TokensRemaining)
	}
	if report.CheckpointCount != 1 {
		t.Errorf("expected CheckpointCount 1, got %d", report.CheckpointCount)
	}
}

func TestEstimateTokens(t *testing.T) {
	text := "This is a test string with some words"
	tokens := EstimateTokens(text)

	// Rough 4 chars per token
	expected := len(text) / 4
	if tokens != expected {
		t.Errorf("expected %d tokens, got %d", expected, tokens)
	}
}

func TestSummarizerClassifyItem(t *testing.T) {
	bt := NewBudgetTracker("test")
	s := NewSummarizer(bt)

	tests := []struct {
		itemType string
		expected SummaryPriority
	}{
		{"failing_test", PriorityCritical},
		{"file_change", PriorityCritical},
		{"critical_finding", PriorityCritical},
		{"high_finding", PriorityHigh},
		{"medium_finding", PriorityMedium},
		{"low_finding", PriorityLow},
		{"context", PriorityLow},
	}

	for _, tt := range tests {
		result := s.ClassifyItem(tt.itemType, "")
		if result != tt.expected {
			t.Errorf("ClassifyItem(%s) = %d, expected %d", tt.itemType, result, tt.expected)
		}
	}
}

func TestSummarizerCreateContextItem(t *testing.T) {
	bt := NewBudgetTracker("test")
	s := NewSummarizer(bt)

	item := s.CreateContextItem("failing_test", "Test xyz failed", map[string]string{"file": "test.go"})

	if item.Type != "failing_test" {
		t.Errorf("expected type failing_test, got %s", item.Type)
	}
	if item.Priority != PriorityCritical {
		t.Errorf("expected CRITICAL priority for failing test, got %d", item.Priority)
	}
	if item.TokenEstimate == 0 {
		t.Error("expected non-zero token estimate")
	}
}

func TestSummarizerSummarizeContext(t *testing.T) {
	bt := NewBudgetTracker("test")
	bt.MaxTokens = 1000
	bt.UpdateUsage(900) // 90% usage

	s := NewSummarizer(bt)
	s.Config.TargetUsage = 0.5

	items := []ContextItem{
		{Type: "critical_finding", Priority: PriorityCritical, Content: "Critical issue", TokenEstimate: 100},
		{Type: "high_finding", Priority: PriorityHigh, Content: "High priority issue", TokenEstimate: 100},
		{Type: "low_finding", Priority: PriorityLow, Content: "Low priority issue with lots of detail that could be dropped", TokenEstimate: 300},
	}

	result, event := s.SummarizeContext(items)

	if event.TokensBefore != 900 {
		t.Errorf("expected TokensBefore 900, got %d", event.TokensBefore)
	}
	if event.TokensSaved <= 0 {
		t.Error("expected tokens to be saved")
	}

	// Critical items should always be preserved
	hasCritical := false
	for _, item := range result {
		if item.Priority == PriorityCritical {
			hasCritical = true
			break
		}
	}
	if !hasCritical {
		t.Error("critical items should be preserved")
	}
}

func TestSummarizeState(t *testing.T) {
	state := SummarizeState{
		SessionID:    "test-session",
		Timestamp:    time.Now(),
		FilesChanged: []string{"file1.go", "file2.go"},
		TestStatus:   "failing",
		FailingTests: []string{"TestFoo", "TestBar"},
		CurrentTask:  "Implement feature X",
	}

	bt := NewBudgetTracker("test")
	s := NewSummarizer(bt)

	context := s.GenerateResumptionContext(state)

	if context == "" {
		t.Error("expected non-empty resumption context")
	}
	if !contains(context, "file1.go") {
		t.Error("expected files in context")
	}
	if !contains(context, "TestFoo") {
		t.Error("expected failing tests in context")
	}
}

func TestSummaryConfig(t *testing.T) {
	config := DefaultSummaryConfig()

	if config.TargetUsage != 0.5 {
		t.Errorf("expected TargetUsage 0.5, got %f", config.TargetUsage)
	}
	if !config.PreserveFailingTests {
		t.Error("expected PreserveFailingTests true")
	}
	if !config.PreserveFileChanges {
		t.Error("expected PreserveFileChanges true")
	}
}

func TestBudgetRecommendation(t *testing.T) {
	bt := NewBudgetTracker("test")
	bt.MaxTokens = 100000

	bt.UpdateUsage(30000)
	rec := bt.GetRecommendation()
	if !contains(rec, "OPTIMAL") {
		t.Error("expected OPTIMAL recommendation at 30%")
	}

	bt.UpdateUsage(65000)
	rec = bt.GetRecommendation()
	if !contains(rec, "MEDIUM") {
		t.Error("expected MEDIUM recommendation at 65%")
	}

	bt.UpdateUsage(85000)
	rec = bt.GetRecommendation()
	if !contains(rec, "HIGH") {
		t.Error("expected HIGH recommendation at 85%")
	}

	bt.UpdateUsage(95000)
	rec = bt.GetRecommendation()
	if !contains(rec, "CRITICAL") {
		t.Error("expected CRITICAL recommendation at 95%")
	}
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || contains(s[1:], substr)))
}

func TestGetUsagePercentZeroMax(t *testing.T) {
	bt := &BudgetTracker{MaxTokens: 0, EstimatedUsage: 100}
	if bt.GetUsagePercent() != 0 {
		t.Error("expected 0 when MaxTokens is 0")
	}
}

func TestNeedsCheckpoint(t *testing.T) {
	tests := []struct {
		name     string
		usage    int
		max      int
		expected bool
	}{
		{"below threshold", 50000, 100000, false},
		{"at threshold", 60000, 100000, true},
		{"above threshold", 75000, 100000, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bt := NewBudgetTracker("test")
			bt.MaxTokens = tt.max
			bt.UpdateUsage(tt.usage)
			if bt.NeedsCheckpoint() != tt.expected {
				t.Errorf("NeedsCheckpoint() = %v, want %v", bt.NeedsCheckpoint(), tt.expected)
			}
		})
	}
}

func TestGetLastCheckpointEmpty(t *testing.T) {
	bt := NewBudgetTracker("test")
	if bt.GetLastCheckpoint() != nil {
		t.Error("expected nil for empty checkpoints")
	}
}

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	bt := NewBudgetTracker("save-test")
	bt.MaxTokens = 100000
	bt.UpdateUsage(55000)
	bt.CreateCheckpoint("cp1", "test checkpoint", []string{"a.go"}, "passing")

	if err := bt.Save(dir); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := Load(dir, "save-test")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if loaded.SessionID != "save-test" {
		t.Errorf("expected SessionID save-test, got %s", loaded.SessionID)
	}
	if loaded.EstimatedUsage != 55000 {
		t.Errorf("expected EstimatedUsage 55000, got %d", loaded.EstimatedUsage)
	}
	if len(loaded.Checkpoints) != 1 {
		t.Errorf("expected 1 checkpoint, got %d", len(loaded.Checkpoints))
	}
}

func TestLoadNotFound(t *testing.T) {
	dir := t.TempDir()
	_, err := Load(dir, "nonexistent")
	if err == nil {
		t.Error("expected error loading nonexistent session")
	}
}

func TestEstimateFileTokens(t *testing.T) {
	// Create a temp file with known content
	dir := t.TempDir()
	path := dir + "/test.txt"
	content := "Hello, this is a test file with some content for estimation."
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	tokens := EstimateFileTokens(path)
	expected := len(content) / 4
	if tokens != expected {
		t.Errorf("EstimateFileTokens() = %d, want %d", tokens, expected)
	}
}

func TestEstimateFileTokensNotFound(t *testing.T) {
	tokens := EstimateFileTokens("/nonexistent/file.txt")
	if tokens != 1000 {
		t.Errorf("expected default 1000 for missing file, got %d", tokens)
	}
}

func TestClassifyItemAllTypes(t *testing.T) {
	bt := NewBudgetTracker("test")
	s := NewSummarizer(bt)

	tests := []struct {
		itemType string
		expected SummaryPriority
	}{
		{"failing_test", PriorityCritical},
		{"file_change", PriorityCritical},
		{"critical_finding", PriorityCritical},
		{"high_finding", PriorityHigh},
		{"medium_finding", PriorityMedium},
		{"low_finding", PriorityLow},
		{"context", PriorityLow},
		{"exploration", PriorityLow},
		{"unknown_type", PriorityMedium},
	}
	for _, tt := range tests {
		t.Run(tt.itemType, func(t *testing.T) {
			result := s.ClassifyItem(tt.itemType, "")
			if result != tt.expected {
				t.Errorf("ClassifyItem(%s) = %d, want %d", tt.itemType, result, tt.expected)
			}
		})
	}
}

func TestClassifyItemPreserveDisabled(t *testing.T) {
	bt := NewBudgetTracker("test")
	s := NewSummarizer(bt)
	s.Config.PreserveFailingTests = false
	s.Config.PreserveFileChanges = false
	s.Config.PreserveCriticalFindings = false

	tests := []struct {
		itemType string
		expected SummaryPriority
	}{
		{"failing_test", PriorityHigh},
		{"file_change", PriorityHigh},
		{"critical_finding", PriorityHigh},
	}
	for _, tt := range tests {
		t.Run(tt.itemType, func(t *testing.T) {
			result := s.ClassifyItem(tt.itemType, "")
			if result != tt.expected {
				t.Errorf("ClassifyItem(%s) with preserve=false = %d, want %d", tt.itemType, result, tt.expected)
			}
		})
	}
}

func TestSummarizeContextWithSummarizedItems(t *testing.T) {
	bt := NewBudgetTracker("test")
	bt.MaxTokens = 500
	bt.UpdateUsage(400)

	s := NewSummarizer(bt)
	s.Config.TargetUsage = 0.5

	// Item with a pre-existing summary (triggers summarizeItem with Summary set)
	items := []ContextItem{
		{Type: "critical_finding", Priority: PriorityCritical, Content: "Must keep", TokenEstimate: 50},
		{
			Type:          "medium_finding",
			Priority:      PriorityMedium,
			Content:       "This is a long medium priority finding that exceeds budget and needs summarization",
			Summary:       "Medium finding summarized",
			TokenEstimate: 300,
		},
	}

	result, event := s.SummarizeContext(items)
	if len(result) < 1 {
		t.Fatal("expected at least 1 item in result")
	}
	if event.TokensBefore != 400 {
		t.Errorf("expected TokensBefore 400, got %d", event.TokensBefore)
	}

	// Verify the medium item was summarized (uses Summary field)
	foundSummarized := false
	for _, item := range result {
		if item.Type == "medium_finding" && item.Content == "Medium finding summarized" {
			foundSummarized = true
		}
	}
	if !foundSummarized {
		t.Error("expected medium_finding to be summarized using its Summary field")
	}
}

func TestSummarizeContextTruncation(t *testing.T) {
	bt := NewBudgetTracker("test")
	bt.MaxTokens = 500
	bt.UpdateUsage(400)

	s := NewSummarizer(bt)
	s.Config.TargetUsage = 0.5
	s.Config.MaxSummaryLength = 20

	// Item without a summary that exceeds budget (triggers truncation path)
	longContent := "This is a very long content string that should be truncated during summarization to fit within budget"
	items := []ContextItem{
		{Type: "critical_finding", Priority: PriorityCritical, Content: "Keep", TokenEstimate: 50},
		{
			Type:          "medium_finding",
			Priority:      PriorityMedium,
			Content:       longContent,
			TokenEstimate: 300,
		},
	}

	result, _ := s.SummarizeContext(items)

	foundTruncated := false
	for _, item := range result {
		if item.Type == "medium_finding" && len(item.Content) < len(longContent) {
			foundTruncated = true
		}
	}
	if !foundTruncated {
		t.Error("expected medium_finding to be truncated")
	}
}

func TestSaveStateAndLoadState(t *testing.T) {
	dir := t.TempDir()
	bt := NewBudgetTracker("test")
	s := NewSummarizer(bt)

	state := SummarizeState{
		SessionID:        "state-test",
		Timestamp:        time.Now(),
		FilesChanged:     []string{"a.go", "b.go"},
		TestStatus:       "passing",
		CriticalFindings: []string{"finding1"},
		CurrentTask:      "task1",
		CompletedTasks:   []string{"task0"},
		Notes:            "some notes",
	}

	if err := s.SaveState(dir, state); err != nil {
		t.Fatalf("SaveState failed: %v", err)
	}

	loaded, err := LoadState(dir, "state-test")
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}
	if loaded.SessionID != "state-test" {
		t.Errorf("expected SessionID state-test, got %s", loaded.SessionID)
	}
	if len(loaded.FilesChanged) != 2 {
		t.Errorf("expected 2 files changed, got %d", len(loaded.FilesChanged))
	}
	if loaded.Notes != "some notes" {
		t.Errorf("expected Notes 'some notes', got %s", loaded.Notes)
	}
}

func TestLoadStateNotFound(t *testing.T) {
	dir := t.TempDir()
	_, err := LoadState(dir, "nonexistent")
	if err == nil {
		t.Error("expected error loading nonexistent state")
	}
}

func TestGenerateResumptionContextAllBranches(t *testing.T) {
	bt := NewBudgetTracker("test")
	s := NewSummarizer(bt)

	tests := []struct {
		name           string
		state          SummarizeState
		mustContain    []string
		mustNotContain []string
	}{
		{
			name: "no files changed",
			state: SummarizeState{
				TestStatus: "passing",
			},
			mustContain: []string{"No files changed yet."},
		},
		{
			name: "with completed tasks and notes",
			state: SummarizeState{
				FilesChanged:   []string{"x.go"},
				TestStatus:     "passing",
				CompletedTasks: []string{"did thing A", "did thing B"},
				Notes:          "Remember to check edge cases",
			},
			mustContain: []string{"Completed Tasks", "[x] did thing A", "[x] did thing B", "Notes", "Remember to check edge cases"},
		},
		{
			name: "with critical findings",
			state: SummarizeState{
				FilesChanged:     []string{"y.go"},
				TestStatus:       "failing",
				FailingTests:     []string{"TestBroken"},
				CriticalFindings: []string{"Memory leak in handler"},
			},
			mustContain: []string{"Critical Findings", "Memory leak in handler", "TestBroken"},
		},
		{
			name: "with current task",
			state: SummarizeState{
				FilesChanged: []string{"z.go"},
				TestStatus:   "passing",
				CurrentTask:  "Implement feature Y",
			},
			mustContain: []string{"Current Task", "Implement feature Y"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := s.GenerateResumptionContext(tt.state)
			for _, s := range tt.mustContain {
				if !contains(result, s) {
					t.Errorf("expected output to contain %q", s)
				}
			}
		})
	}
}

func TestBudgetTracker_Save_ReadOnlyDir(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root bypasses filesystem permissions")
	}
	tmpDir := t.TempDir()
	readOnly := filepath.Join(tmpDir, "readonly")
	if err := os.MkdirAll(readOnly, 0500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(readOnly, 0700) })

	bt := NewBudgetTracker("test-session")
	err := bt.Save(readOnly)
	if err == nil {
		t.Error("expected error when saving to read-only directory")
	}
}

func TestBudgetTracker_Load_MalformedJSON(t *testing.T) {
	tmpDir := t.TempDir()
	dir := filepath.Join(tmpDir, ".agents", "ao", "context")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}

	// Write a malformed JSON file
	path := filepath.Join(dir, "budget-bad-session.json")
	if err := os.WriteFile(path, []byte("{invalid json"), 0600); err != nil {
		t.Fatal(err)
	}

	_, err := Load(tmpDir, "bad-session")
	if err == nil {
		t.Error("expected error when loading malformed JSON budget file")
	}
}

func TestSummarizer_SaveState_ReadOnlyDir(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root bypasses filesystem permissions")
	}
	tmpDir := t.TempDir()
	readOnly := filepath.Join(tmpDir, "readonly")
	if err := os.MkdirAll(readOnly, 0500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(readOnly, 0700) })

	bt := NewBudgetTracker("test-session")
	s := NewSummarizer(bt)
	state := SummarizeState{SessionID: "test-session"}
	err := s.SaveState(readOnly, state)
	if err == nil {
		t.Error("expected error when saving state to read-only directory")
	}
}

func TestLoadState_MalformedJSON(t *testing.T) {
	tmpDir := t.TempDir()
	dir := filepath.Join(tmpDir, ".agents", "ao", "context")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}

	// Write a malformed JSON state file
	path := filepath.Join(dir, "state-bad-session.json")
	if err := os.WriteFile(path, []byte("{invalid json"), 0600); err != nil {
		t.Fatal(err)
	}

	_, err := LoadState(tmpDir, "bad-session")
	if err == nil {
		t.Error("expected error when loading malformed JSON state file")
	}
}

// --- Benchmarks ---

func BenchmarkEstimateTokens(b *testing.B) {
	text := "This is a sample string that represents typical content for token estimation benchmarking. "
	// Build ~1000 chars
	long := ""
	for range 10 {
		long += text
	}
	b.ResetTimer()
	for range b.N {
		EstimateTokens(long)
	}
}

func BenchmarkSummarizeContext(b *testing.B) {
	tracker := NewBudgetTracker("bench-session")
	summarizer := NewSummarizer(tracker)
	items := make([]ContextItem, 20)
	for i := range items {
		items[i] = ContextItem{
			Type:          "file_change",
			Priority:      PriorityHigh,
			Content:       "Modified cli/internal/context/summarize.go with refactoring changes",
			TokenEstimate: 15,
		}
	}
	b.ResetTimer()
	for range b.N {
		summarizer.SummarizeContext(items)
	}
}

func BenchmarkClassifyItem(b *testing.B) {
	tracker := NewBudgetTracker("bench-session")
	summarizer := NewSummarizer(tracker)
	b.ResetTimer()
	for range b.N {
		summarizer.ClassifyItem("failing_test", "TestFoo failed")
		summarizer.ClassifyItem("file_change", "modified foo.go")
		summarizer.ClassifyItem("low_finding", "minor style issue")
	}
}

// TestExtra_BudgetTracker_Save_MkdirError covers MkdirAll failure in Save.
func TestExtra_BudgetTracker_Save_MkdirError(t *testing.T) {
	bt := &BudgetTracker{SessionID: "test-session"}
	// Use /dev/null as base to force MkdirAll failure.
	err := bt.Save("/dev/null")
	if err == nil {
		t.Fatal("expected MkdirAll error, got nil")
	}
}

// TestExtra_BudgetTracker_Save_Success covers the happy path.
func TestExtra_BudgetTracker_Save_Success(t *testing.T) {
	bt := &BudgetTracker{SessionID: "test-save"}
	tmp := t.TempDir()
	if err := bt.Save(tmp); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	path := filepath.Join(tmp, ".agents", "ao", "context", "budget-test-save.json")
	if _, err := os.Stat(path); err != nil {
		t.Errorf("expected budget file to exist: %v", err)
	}
}

// TestExtra_Summarizer_SaveState_MkdirError covers MkdirAll failure in SaveState.
func TestExtra_Summarizer_SaveState_MkdirError(t *testing.T) {
	s := &Summarizer{}
	state := SummarizeState{SessionID: "test-session"}
	err := s.SaveState("/dev/null", state)
	if err == nil {
		t.Fatal("expected MkdirAll error, got nil")
	}
}

// TestExtra_Summarizer_SaveState_Success covers the happy path.
func TestExtra_Summarizer_SaveState_Success(t *testing.T) {
	s := &Summarizer{}
	state := SummarizeState{
		SessionID:      "test-state",
		CompletedTasks: []string{"task1"},
		Notes:          "some notes",
	}
	tmp := t.TempDir()
	if err := s.SaveState(tmp, state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	path := filepath.Join(tmp, ".agents", "ao", "context", "state-test-state.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("expected state file to exist: %v", err)
	}
	if len(data) == 0 {
		t.Error("expected non-empty state file")
	}
}

// TestBudgetTracker_Save_MarshalError covers the json.MarshalIndent error branch in Save.
// NaN in a float64 field is not representable in JSON, so MarshalIndent returns an error.
func TestBudgetTracker_Save_MarshalError(t *testing.T) {
	bt := NewBudgetTracker("marshal-fail")
	// Inject NaN into a checkpoint's PercentUsage to make json.MarshalIndent fail.
	bt.Checkpoints = append(bt.Checkpoints, Checkpoint{
		ID:           "bad",
		PercentUsage: math.NaN(),
	})

	tmp := t.TempDir()
	err := bt.Save(tmp)
	if err == nil {
		t.Fatal("expected json marshal error due to NaN, got nil")
	}
	expected := "json: unsupported value: NaN"
	if err.Error() != expected {
		t.Errorf("expected error %q, got %q", expected, err.Error())
	}
}

// TestSummarizer_SaveState_MarshalError covers the json.MarshalIndent error branch in SaveState.
// We inject a failing marshaler via the package-level marshalJSON variable.
func TestSummarizer_SaveState_MarshalError(t *testing.T) {
	original := marshalJSON
	t.Cleanup(func() { marshalJSON = original })

	marshalJSON = func(_ any, _ string, _ string) ([]byte, error) {
		return nil, errors.New("injected marshal error")
	}

	tmp := t.TempDir()
	s := &Summarizer{}
	state := SummarizeState{SessionID: "marshal-fail"}

	err := s.SaveState(tmp, state)
	if err == nil {
		t.Fatal("expected marshal error, got nil")
	}
	expected := "injected marshal error"
	if err.Error() != expected {
		t.Errorf("expected error %q, got %q", expected, err.Error())
	}
}

func BenchmarkGenerateResumptionContext(b *testing.B) {
	tracker := NewBudgetTracker("bench-session")
	summarizer := NewSummarizer(tracker)
	state := SummarizeState{
		FilesChanged:     []string{"file1.go", "file2.go", "file3.go"},
		TestStatus:       "3 passed, 1 failed",
		FailingTests:     []string{"TestFoo"},
		CriticalFindings: []string{"race condition in handler"},
		CurrentTask:      "Fix concurrency bug",
		CompletedTasks:   []string{"Refactored config", "Added tests"},
		Notes:            "Need to check edge cases",
	}
	b.ResetTimer()
	for range b.N {
		summarizer.GenerateResumptionContext(state)
	}
}
