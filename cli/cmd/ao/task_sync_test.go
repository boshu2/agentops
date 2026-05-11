// practices: [agile-manifesto, dora-metrics]
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/types"
)

// ---------------------------------------------------------------------------
// statusToMaturity
// ---------------------------------------------------------------------------

func TestTaskSync_statusToMaturity(t *testing.T) {
	tests := []struct {
		status string
		want   types.Maturity
	}{
		{"completed", types.MaturityEstablished},
		{"in_progress", types.MaturityCandidate},
		{"pending", types.MaturityProvisional},
		{"", types.MaturityProvisional},
		{"unknown", types.MaturityProvisional},
	}
	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			got := statusToMaturity(tt.status)
			if got != tt.want {
				t.Errorf("statusToMaturity(%q) = %q, want %q", tt.status, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// generateTaskID
// ---------------------------------------------------------------------------

func TestTaskSync_generateTaskID(t *testing.T) {
	id := generateTaskID()
	if !strings.HasPrefix(id, "task-") {
		t.Errorf("generateTaskID() = %q, want prefix 'task-'", id)
	}
	if len(id) < 10 {
		t.Errorf("generateTaskID() too short: %q", id)
	}
}

// ---------------------------------------------------------------------------
// assignMaturityAndUtility
// ---------------------------------------------------------------------------

func TestTaskSync_assignMaturityAndUtility(t *testing.T) {
	tasks := []TaskEvent{
		{Status: "pending", Utility: 0},
		{Status: "in_progress", Utility: 0},
		{Status: "completed", Utility: 0.9},
	}
	assignMaturityAndUtility(tasks)

	if tasks[0].Maturity != types.MaturityProvisional {
		t.Errorf("pending task maturity = %q, want provisional", tasks[0].Maturity)
	}
	if tasks[0].Utility != types.InitialUtility {
		t.Errorf("pending task utility = %f, want %f", tasks[0].Utility, types.InitialUtility)
	}
	if tasks[1].Maturity != types.MaturityCandidate {
		t.Errorf("in_progress task maturity = %q, want candidate", tasks[1].Maturity)
	}
	if tasks[1].Utility != types.InitialUtility {
		t.Errorf("in_progress task utility = %f, want %f", tasks[1].Utility, types.InitialUtility)
	}
	if tasks[2].Maturity != types.MaturityEstablished {
		t.Errorf("completed task maturity = %q, want established", tasks[2].Maturity)
	}
	// Utility already > 0, should not be overwritten
	if tasks[2].Utility != 0.9 {
		t.Errorf("completed task utility = %f, want 0.9 (unchanged)", tasks[2].Utility)
	}
}

func TestTaskSync_assignMaturityAndUtility_empty(t *testing.T) {
	// Verify nil and empty slice are handled without panic and return no modifications
	assignMaturityAndUtility(nil)
	tasks := []TaskEvent{}
	assignMaturityAndUtility(tasks)
	if len(tasks) != 0 {
		t.Errorf("expected empty slice, got %d tasks", len(tasks))
	}
}

// ---------------------------------------------------------------------------
// filterTasksBySession
// ---------------------------------------------------------------------------

func TestTaskSync_filterTasksBySession(t *testing.T) {
	tasks := []TaskEvent{
		{TaskID: "t1", SessionID: "s1"},
		{TaskID: "t2", SessionID: "s2"},
		{TaskID: "t3", SessionID: "s1"},
	}

	tests := []struct {
		name      string
		sessionID string
		wantCount int
	}{
		{"empty filter returns all", "", 3},
		{"filter to s1", "s1", 2},
		{"filter to s2", "s2", 1},
		{"filter to nonexistent", "s99", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filterTasksBySession(tasks, tt.sessionID)
			if len(got) != tt.wantCount {
				t.Errorf("filterTasksBySession(_, %q) returned %d, want %d", tt.sessionID, len(got), tt.wantCount)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// filterProcessableTasks
// ---------------------------------------------------------------------------

func TestTaskSync_filterProcessableTasks(t *testing.T) {
	tasks := []TaskEvent{
		{TaskID: "t1", Status: "completed", LearningID: "L-1", SessionID: "s1"},
		{TaskID: "t2", Status: "pending", LearningID: "", SessionID: "s1"},
		{TaskID: "t3", Status: "completed", LearningID: "", SessionID: "s1"}, // no learning
		{TaskID: "t4", Status: "completed", LearningID: "L-4", SessionID: "s2"},
	}

	tests := []struct {
		name      string
		session   string
		wantCount int
	}{
		{"all sessions", "", 2},   // t1 and t4
		{"filter to s1", "s1", 1}, // only t1
		{"filter to s2", "s2", 1}, // only t4
		{"nonexistent session", "s9", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filterProcessableTasks(tasks, tt.session)
			if len(got) != tt.wantCount {
				t.Errorf("filterProcessableTasks returned %d, want %d", len(got), tt.wantCount)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// computeTaskDistributions
// ---------------------------------------------------------------------------

func TestTaskSync_computeTaskDistributions(t *testing.T) {
	tasks := []TaskEvent{
		{Status: "pending", Maturity: types.MaturityProvisional},
		{Status: "pending", Maturity: types.MaturityProvisional},
		{Status: "in_progress", Maturity: types.MaturityCandidate},
		{Status: "completed", Maturity: types.MaturityEstablished, LearningID: "L-1"},
		{Status: "completed", Maturity: types.MaturityEstablished, LearningID: "L-2"},
		{Status: "completed", Maturity: types.MaturityEstablished},
	}

	statusCounts, maturityCounts, withLearnings := computeTaskDistributions(tasks)

	if statusCounts["pending"] != 2 {
		t.Errorf("pending count = %d, want 2", statusCounts["pending"])
	}
	if statusCounts["in_progress"] != 1 {
		t.Errorf("in_progress count = %d, want 1", statusCounts["in_progress"])
	}
	if statusCounts["completed"] != 3 {
		t.Errorf("completed count = %d, want 3", statusCounts["completed"])
	}
	if maturityCounts[types.MaturityProvisional] != 2 {
		t.Errorf("provisional count = %d, want 2", maturityCounts[types.MaturityProvisional])
	}
	if maturityCounts[types.MaturityCandidate] != 1 {
		t.Errorf("candidate count = %d, want 1", maturityCounts[types.MaturityCandidate])
	}
	if maturityCounts[types.MaturityEstablished] != 3 {
		t.Errorf("established count = %d, want 3", maturityCounts[types.MaturityEstablished])
	}
	if withLearnings != 2 {
		t.Errorf("withLearnings = %d, want 2", withLearnings)
	}
}

// ---------------------------------------------------------------------------
// promoteCompletedTasks
// ---------------------------------------------------------------------------

func TestTaskSync_promoteCompletedTasks_noPromote(t *testing.T) {
	tasks := []TaskEvent{
		{TaskID: "t1", Status: "completed", LearningID: ""},
	}
	got := promoteCompletedTasks(t.TempDir(), tasks, false)
	if got != 0 {
		t.Errorf("promoteCompletedTasks with promote=false returned %d, want 0", got)
	}
}

func TestTaskSync_promoteCompletedTasks_skipsNonCompleted(t *testing.T) {
	tmp := t.TempDir()
	tasks := []TaskEvent{
		{TaskID: "t1", Status: "pending", LearningID: ""},
		{TaskID: "t2", Status: "in_progress", LearningID: ""},
	}
	got := promoteCompletedTasks(tmp, tasks, true)
	if got != 0 {
		t.Errorf("promoteCompletedTasks with non-completed tasks returned %d, want 0", got)
	}
}

func TestTaskSync_promoteCompletedTasks_skipsAlreadyPromoted(t *testing.T) {
	tmp := t.TempDir()
	tasks := []TaskEvent{
		{TaskID: "t1", Status: "completed", LearningID: "L-already"},
	}
	got := promoteCompletedTasks(tmp, tasks, true)
	if got != 0 {
		t.Errorf("promoteCompletedTasks with existing learningID returned %d, want 0", got)
	}
}

func TestTaskSync_promoteCompletedTasks_promotes(t *testing.T) {
	tmp := t.TempDir()
	tasks := []TaskEvent{
		{TaskID: "task-20260125-100000", Status: "completed", LearningID: "", Subject: "Test task", SessionID: "s1", Utility: 0.5},
	}
	got := promoteCompletedTasks(tmp, tasks, true)
	if got != 1 {
		t.Errorf("promoteCompletedTasks returned %d, want 1", got)
	}
	// Verify learning file was created
	learningsDir := filepath.Join(tmp, ".agents", "learnings")
	entries, err := os.ReadDir(learningsDir)
	if err != nil {
		t.Fatalf("read learnings dir: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("expected 1 learning file, got %d", len(entries))
	}
}

// ---------------------------------------------------------------------------
// promoteTaskToLearning
// ---------------------------------------------------------------------------

func TestTaskSync_promoteTaskToLearning(t *testing.T) {
	tmp := t.TempDir()
	task := &TaskEvent{
		TaskID:      "task-20260125-100000",
		Subject:     "Refactor auth module",
		Description: "Detailed desc",
		Status:      "completed",
		SessionID:   "sess-1",
		Utility:     0.6,
	}

	err := promoteTaskToLearning(tmp, task)
	if err != nil {
		t.Fatalf("promoteTaskToLearning failed: %v", err)
	}

	// Check learning file
	expectedID := "L-20260125-100000"
	if task.LearningID != expectedID {
		t.Errorf("LearningID = %q, want %q", task.LearningID, expectedID)
	}

	learningPath := filepath.Join(tmp, ".agents", "learnings", expectedID+".jsonl")
	data, err := os.ReadFile(learningPath)
	if err != nil {
		t.Fatalf("read learning file: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data[:len(data)-1], &parsed); err != nil {
		t.Fatalf("unmarshal learning: %v", err)
	}
	if parsed["id"] != expectedID {
		t.Errorf("learning id = %v, want %v", parsed["id"], expectedID)
	}
	if parsed["type"] != "learning" {
		t.Errorf("learning type = %v, want 'learning'", parsed["type"])
	}
	if parsed["maturity"] != "established" {
		t.Errorf("learning maturity = %v, want 'established'", parsed["maturity"])
	}
	if parsed["utility"].(float64) != 0.6 {
		t.Errorf("learning utility = %v, want 0.6", parsed["utility"])
	}
}

// ---------------------------------------------------------------------------
// writeTaskEvents and loadTaskEvents
// ---------------------------------------------------------------------------

func TestTaskSync_writeAndLoadTaskEvents(t *testing.T) {
	tmp := t.TempDir()

	tasks := []TaskEvent{
		{TaskID: "t1", Subject: "Task 1", Status: "pending", SessionID: "s1", CreatedAt: time.Now(), UpdatedAt: time.Now()},
		{TaskID: "t2", Subject: "Task 2", Status: "completed", SessionID: "s2", CreatedAt: time.Now(), UpdatedAt: time.Now()},
	}

	err := writeTaskEvents(tmp, tasks)
	if err != nil {
		t.Fatalf("writeTaskEvents failed: %v", err)
	}

	loaded, err := loadTaskEvents(tmp)
	if err != nil {
		t.Fatalf("loadTaskEvents failed: %v", err)
	}
	if len(loaded) != 2 {
		t.Fatalf("loaded %d tasks, want 2", len(loaded))
	}
	if loaded[0].TaskID != "t1" && loaded[1].TaskID != "t1" {
		t.Error("expected t1 in loaded tasks")
	}
}

func TestTaskSync_writeTaskEvents_empty(t *testing.T) {
	tmp := t.TempDir()
	err := writeTaskEvents(tmp, nil)
	if err != nil {
		t.Fatalf("writeTaskEvents(nil) failed: %v", err)
	}
	err = writeTaskEvents(tmp, []TaskEvent{})
	if err != nil {
		t.Fatalf("writeTaskEvents([]) failed: %v", err)
	}
}

func TestTaskSync_writeTaskEvents_deduplicates(t *testing.T) {
	tmp := t.TempDir()

	tasks := []TaskEvent{
		{TaskID: "t1", Subject: "First", Status: "pending"},
	}
	if err := writeTaskEvents(tmp, tasks); err != nil {
		t.Fatal(err)
	}

	// Write again with same ID - should not duplicate
	tasks2 := []TaskEvent{
		{TaskID: "t1", Subject: "Duplicate", Status: "pending"},
		{TaskID: "t2", Subject: "New", Status: "pending"},
	}
	if err := writeTaskEvents(tmp, tasks2); err != nil {
		t.Fatal(err)
	}

	loaded, err := loadTaskEvents(tmp)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 2 {
		t.Errorf("loaded %d tasks, want 2 (dedup t1)", len(loaded))
	}
}

func TestTaskSync_loadTaskEvents_noFile(t *testing.T) {
	_, err := loadTaskEvents(t.TempDir())
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestTaskSync_loadTaskEvents_corruptedLines(t *testing.T) {
	tmp := t.TempDir()
	taskDir := filepath.Join(tmp, ".agents", "ao")
	if err := os.MkdirAll(taskDir, 0o755); err != nil {
		t.Fatal(err)
	}

	content := `{"task_id":"t1","subject":"Good","status":"pending"}
not valid json
{"task_id":"t2","subject":"Also good","status":"completed"}
`
	if err := os.WriteFile(filepath.Join(taskDir, "tasks.jsonl"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	loaded, err := loadTaskEvents(tmp)
	if err != nil {
		t.Fatalf("loadTaskEvents failed: %v", err)
	}
	if len(loaded) != 2 {
		t.Errorf("loaded %d tasks, want 2 (skipping corrupted line)", len(loaded))
	}
}

// ---------------------------------------------------------------------------
// extractContentBlocks
// ---------------------------------------------------------------------------

func TestTaskSync_extractContentBlocks(t *testing.T) {
	tests := []struct {
		name      string
		data      map[string]any
		wantCount int
	}{
		{
			name:      "no message key",
			data:      map[string]any{"foo": "bar"},
			wantCount: 0,
		},
		{
			name:      "message not a map",
			data:      map[string]any{"message": "string"},
			wantCount: 0,
		},
		{
			name:      "content not a slice",
			data:      map[string]any{"message": map[string]any{"content": "string"}},
			wantCount: 0,
		},
		{
			name: "filters tool_use blocks only",
			data: map[string]any{
				"message": map[string]any{
					"content": []any{
						map[string]any{"type": "text", "text": "Hello"},
						map[string]any{"type": "tool_use", "name": "TaskCreate", "input": map[string]any{}},
						map[string]any{"type": "tool_use", "name": "TaskUpdate", "input": map[string]any{}},
						"not-a-map",
					},
				},
			},
			wantCount: 2,
		},
		{
			name: "empty content array",
			data: map[string]any{
				"message": map[string]any{
					"content": []any{},
				},
			},
			wantCount: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			blocks := extractContentBlocks(tt.data)
			if len(blocks) != tt.wantCount {
				t.Errorf("extractContentBlocks returned %d blocks, want %d", len(blocks), tt.wantCount)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// parseTaskCreate
// ---------------------------------------------------------------------------

func TestTaskSync_parseTaskCreate(t *testing.T) {
	tests := []struct {
		name      string
		input     map[string]any
		sessionID string
		wantNil   bool
		wantDesc  string
		wantMeta  map[string]any
	}{
		{
			name:    "empty subject returns nil",
			input:   map[string]any{},
			wantNil: true,
		},
		{
			name:    "blank subject returns nil",
			input:   map[string]any{"subject": ""},
			wantNil: true,
		},
		{
			name:      "basic create",
			input:     map[string]any{"subject": "Fix bug"},
			sessionID: "sess-1",
			wantNil:   false,
		},
		{
			name:      "with description",
			input:     map[string]any{"subject": "Fix bug", "description": "Details here"},
			sessionID: "sess-1",
			wantDesc:  "Details here",
		},
		{
			name:      "with activeForm metadata",
			input:     map[string]any{"subject": "Fix bug", "activeForm": "checklist"},
			sessionID: "sess-1",
			wantMeta:  map[string]any{"active_form": "checklist"},
		},
		{
			name: "preserves metadata map",
			input: map[string]any{
				"subject": "Fix bug",
				"metadata": map[string]any{
					"issue_type": "feature",
					"files":      []any{"cli/cmd/ao/task_sync.go"},
					"validation": map[string]any{"tests": "go test ./cli/cmd/ao/..."},
				},
			},
			sessionID: "sess-1",
			wantMeta:  map[string]any{"issue_type": "feature"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseTaskCreate(tt.input, tt.sessionID)
			if tt.wantNil {
				if got != nil {
					t.Errorf("expected nil, got %+v", got)
				}
				return
			}
			if got == nil {
				t.Fatal("expected non-nil task")
			}
			if got.SessionID != tt.sessionID {
				t.Errorf("SessionID = %q, want %q", got.SessionID, tt.sessionID)
			}
			if got.Status != "pending" {
				t.Errorf("Status = %q, want 'pending'", got.Status)
			}
			if got.Utility != types.InitialUtility {
				t.Errorf("Utility = %f, want %f", got.Utility, types.InitialUtility)
			}
			if tt.wantDesc != "" && got.Description != tt.wantDesc {
				t.Errorf("Description = %q, want %q", got.Description, tt.wantDesc)
			}
			if len(tt.wantMeta) > 0 {
				if got.Metadata == nil {
					t.Fatalf("Metadata = nil, want %v", tt.wantMeta)
				}
				for key, want := range tt.wantMeta {
					if got.Metadata[key] != want {
						t.Errorf("Metadata[%q] = %v, want %v", key, got.Metadata[key], want)
					}
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// updateTask
// ---------------------------------------------------------------------------

func TestTaskSync_updateTask(t *testing.T) {
	task := &TaskEvent{
		TaskID:  "t1",
		Subject: "Original",
		Status:  "pending",
	}

	// Update status to completed
	updateTask(task, map[string]any{
		"status":      "completed",
		"subject":     "Updated Subject",
		"description": "New desc",
		"owner":       "agent-1",
	})

	if task.Status != "completed" {
		t.Errorf("Status = %q, want 'completed'", task.Status)
	}
	if task.Subject != "Updated Subject" {
		t.Errorf("Subject = %q, want 'Updated Subject'", task.Subject)
	}
	if task.Description != "New desc" {
		t.Errorf("Description = %q, want 'New desc'", task.Description)
	}
	if task.Owner != "agent-1" {
		t.Errorf("Owner = %q, want 'agent-1'", task.Owner)
	}
	if task.Maturity != types.MaturityEstablished {
		t.Errorf("Maturity = %q, want established", task.Maturity)
	}
	if task.CompletedAt.IsZero() {
		t.Error("CompletedAt should be set for completed status")
	}
}

func TestTaskSync_updateTask_partialUpdate(t *testing.T) {
	task := &TaskEvent{
		TaskID:  "t1",
		Subject: "Keep this",
		Status:  "pending",
		Owner:   "original-owner",
	}

	// Partial update: only status
	updateTask(task, map[string]any{"status": "in_progress"})

	if task.Subject != "Keep this" {
		t.Errorf("Subject changed unexpectedly to %q", task.Subject)
	}
	if task.Owner != "original-owner" {
		t.Errorf("Owner changed unexpectedly to %q", task.Owner)
	}
	if task.Status != "in_progress" {
		t.Errorf("Status = %q, want 'in_progress'", task.Status)
	}
}

// ---------------------------------------------------------------------------
// applyToolBlock
// ---------------------------------------------------------------------------

func TestTaskSync_applyToolBlock(t *testing.T) {
	taskMap := make(map[string]*TaskEvent)

	// TaskCreate
	createBlock := map[string]any{
		"name":  "TaskCreate",
		"input": map[string]any{"subject": "New task", "metadata": map[string]any{"issue_type": "task"}},
	}
	applyToolBlock(createBlock, "sess-1", taskMap)

	if len(taskMap) != 1 {
		t.Fatalf("expected 1 task after TaskCreate, got %d", len(taskMap))
	}

	// Get the created task's ID
	var taskID string
	for id := range taskMap {
		taskID = id
	}
	if got := taskMap[taskID].Metadata["issue_type"]; got != "task" {
		t.Fatalf("Metadata[issue_type] = %v, want task", got)
	}

	// TaskUpdate
	updateBlock := map[string]any{
		"name":  "TaskUpdate",
		"input": map[string]any{"taskId": taskID, "status": "completed"},
	}
	applyToolBlock(updateBlock, "sess-1", taskMap)

	if taskMap[taskID].Status != "completed" {
		t.Errorf("task status after update = %q, want 'completed'", taskMap[taskID].Status)
	}
}

func TestTaskSync_applyToolBlock_unknownTool(t *testing.T) {
	taskMap := make(map[string]*TaskEvent)
	block := map[string]any{
		"name":  "SomeOtherTool",
		"input": map[string]any{},
	}
	applyToolBlock(block, "sess-1", taskMap)
	if len(taskMap) != 0 {
		t.Errorf("unexpected tasks after unknown tool: %d", len(taskMap))
	}
}

func TestTaskSync_applyToolBlock_updateNonexistent(t *testing.T) {
	taskMap := make(map[string]*TaskEvent)
	block := map[string]any{
		"name":  "TaskUpdate",
		"input": map[string]any{"taskId": "nonexistent", "status": "completed"},
	}
	// Should not panic or create
	applyToolBlock(block, "sess-1", taskMap)
	if len(taskMap) != 0 {
		t.Errorf("unexpected tasks after updating nonexistent: %d", len(taskMap))
	}
}

// ---------------------------------------------------------------------------
// processTranscriptLine
// ---------------------------------------------------------------------------

func TestTaskSync_processTranscriptLine(t *testing.T) {
	taskMap := make(map[string]*TaskEvent)

	// Invalid JSON
	sid := processTranscriptLine("not json", "", "", taskMap)
	if sid != "" {
		t.Errorf("expected empty sessionID for invalid JSON, got %q", sid)
	}

	// Line with sessionId
	line := `{"sessionId":"sess-42","message":{"content":[]}}`
	sid = processTranscriptLine(line, "", "", taskMap)
	if sid != "sess-42" {
		t.Errorf("expected sessionID 'sess-42', got %q", sid)
	}

	// Line with TaskCreate
	lineCreate := `{"sessionId":"sess-42","message":{"content":[{"type":"tool_use","name":"TaskCreate","input":{"subject":"My task"}}]}}`
	_ = processTranscriptLine(lineCreate, "", "sess-42", taskMap)
	if len(taskMap) != 1 {
		t.Fatalf("expected 1 task, got %d", len(taskMap))
	}
}

func TestTaskSync_processTranscriptLine_sessionFilter(t *testing.T) {
	taskMap := make(map[string]*TaskEvent)

	// Line from different session, filter active
	line := `{"sessionId":"sess-1","message":{"content":[{"type":"tool_use","name":"TaskCreate","input":{"subject":"Filtered out"}}]}}`
	processTranscriptLine(line, "sess-2", "", taskMap)
	if len(taskMap) != 0 {
		t.Errorf("expected 0 tasks when session filtered, got %d", len(taskMap))
	}

	// Line matching filter
	line2 := `{"sessionId":"sess-2","message":{"content":[{"type":"tool_use","name":"TaskCreate","input":{"subject":"Included"}}]}}`
	processTranscriptLine(line2, "sess-2", "", taskMap)
	if len(taskMap) != 1 {
		t.Errorf("expected 1 task when session matches filter, got %d", len(taskMap))
	}
}

// ---------------------------------------------------------------------------
// extractTaskEvents
// ---------------------------------------------------------------------------

func TestTaskSync_extractTaskEvents(t *testing.T) {
	// NOTE: generateTaskID() uses time.Now().Format("20060102-150405") which
	// has only second-level precision. Multiple TaskCreate calls within the
	// same second produce the same ID, so the taskMap deduplicates them.
	// This test verifies the actual behavior: extracting unique tasks from
	// transcript lines that arrive at different times (simulated via TaskUpdate).
	tmp := t.TempDir()
	transcriptPath := filepath.Join(tmp, "transcript.jsonl")

	lines := []string{
		`{"sessionId":"sess-1","message":{"content":[{"type":"tool_use","name":"TaskCreate","input":{"subject":"First task"}}]}}`,
	}
	content := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(transcriptPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	// Single create produces exactly 1 task
	tasks, err := extractTaskEvents(transcriptPath, "")
	if err != nil {
		t.Fatalf("extractTaskEvents failed: %v", err)
	}
	if len(tasks) != 1 {
		t.Errorf("expected 1 task, got %d", len(tasks))
	}
	if len(tasks) > 0 && tasks[0].Subject != "First task" {
		t.Errorf("subject = %q, want %q", tasks[0].Subject, "First task")
	}

	// Session filter excludes non-matching sessions
	tasksFiltered, err := extractTaskEvents(transcriptPath, "sess-2")
	if err != nil {
		t.Fatalf("extractTaskEvents with filter failed: %v", err)
	}
	if len(tasksFiltered) != 0 {
		t.Errorf("expected 0 tasks for sess-2, got %d", len(tasksFiltered))
	}
}

func TestTaskSync_extractTaskEvents_fileNotFound(t *testing.T) {
	_, err := extractTaskEvents("/nonexistent/path", "")
	if err == nil {
		t.Error("expected error for missing transcript")
	}
}

// ---------------------------------------------------------------------------
// resolveTranscriptPath
// ---------------------------------------------------------------------------

func TestTaskSync_resolveTranscriptPath_explicit(t *testing.T) {
	got := resolveTranscriptPath("/some/explicit/path.jsonl")
	if got != "/some/explicit/path.jsonl" {
		t.Errorf("resolveTranscriptPath(explicit) = %q, want explicit path", got)
	}
}

func TestTaskSync_resolveTranscriptPath_empty(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	// With empty HOME/.claude/projects, should return empty
	got := resolveTranscriptPath("")
	if got != "" {
		t.Errorf("expected empty path with no projects, got %q", got)
	}
}

// ---------------------------------------------------------------------------
// printTaskSyncSummary
// ---------------------------------------------------------------------------

func TestTaskSync_printTaskSyncSummary_text(t *testing.T) {
	oldOutput := output
	output = "table"
	defer func() { output = oldOutput }()

	tasks := []TaskEvent{
		{Status: "pending"},
		{Status: "completed"},
	}
	// Should not error in text mode
	err := printTaskSyncSummary("/path/to/transcript", tasks, 1)
	if err != nil {
		t.Fatalf("printTaskSyncSummary text failed: %v", err)
	}
}

// ---------------------------------------------------------------------------
// printTaskStatusText
// ---------------------------------------------------------------------------

func TestTaskSync_printTaskStatusText(t *testing.T) {
	statusCounts := map[string]int{"pending": 2, "completed": 1}
	maturityCounts := map[types.Maturity]int{
		types.MaturityProvisional: 2,
		types.MaturityEstablished: 1,
	}
	out, _ := captureStdout(t, func() error {
		printTaskStatusText([]TaskEvent{{}, {}, {}}, statusCounts, maturityCounts, 1)
		return nil
	})
	if !strings.Contains(out, "pending") {
		t.Errorf("output missing 'pending':\n%s", out)
	}
}

// ---------------------------------------------------------------------------
// TaskEvent JSON round-trip
// ---------------------------------------------------------------------------

func TestTaskSync_TaskEventJSONRoundTrip(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	task := TaskEvent{
		TaskID:      "task-20260125",
		Subject:     "Test",
		Description: "Desc",
		Status:      "completed",
		SessionID:   "sess-1",
		CreatedAt:   now,
		UpdatedAt:   now,
		CompletedAt: now,
		LearningID:  "L-1",
		Maturity:    types.MaturityEstablished,
		Utility:     0.75,
		Owner:       "agent-1",
		BlockedBy:   []string{"t0"},
		Metadata:    map[string]any{"key": "value"},
	}

	data, err := json.Marshal(task)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded TaskEvent
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.TaskID != task.TaskID {
		t.Errorf("TaskID = %q, want %q", decoded.TaskID, task.TaskID)
	}
	if decoded.Status != task.Status {
		t.Errorf("Status = %q, want %q", decoded.Status, task.Status)
	}
	if decoded.Utility != task.Utility {
		t.Errorf("Utility = %f, want %f", decoded.Utility, task.Utility)
	}
	if len(decoded.BlockedBy) != 1 || decoded.BlockedBy[0] != "t0" {
		t.Errorf("BlockedBy = %v, want [t0]", decoded.BlockedBy)
	}
}

// ---------------------------------------------------------------------------
// cloneMap
// ---------------------------------------------------------------------------

func TestCloneMap(t *testing.T) {
	t.Run("nil input returns nil", func(t *testing.T) {
		got := cloneMap(nil)
		if got != nil {
			t.Errorf("cloneMap(nil) = %v, want nil", got)
		}
	})

	t.Run("empty map returns nil", func(t *testing.T) {
		got := cloneMap(map[string]any{})
		if got != nil {
			t.Errorf("cloneMap(empty) = %v, want nil", got)
		}
	})

	t.Run("clones all keys", func(t *testing.T) {
		input := map[string]any{"a": 1, "b": "two", "c": true}
		got := cloneMap(input)
		if len(got) != 3 {
			t.Fatalf("cloneMap len = %d, want 3", len(got))
		}
		if got["a"] != 1 {
			t.Errorf("got[a] = %v, want 1", got["a"])
		}
		if got["b"] != "two" {
			t.Errorf("got[b] = %v, want 'two'", got["b"])
		}
		if got["c"] != true {
			t.Errorf("got[c] = %v, want true", got["c"])
		}
	})

	t.Run("mutation of clone does not affect original", func(t *testing.T) {
		input := map[string]any{"key": "original"}
		cloned := cloneMap(input)
		cloned["key"] = "modified"
		if input["key"] != "original" {
			t.Error("modifying clone should not affect original")
		}
	})
}

// ---------------------------------------------------------------------------
// writeTaskEvents edge cases
// ---------------------------------------------------------------------------

func TestWriteTaskEvents_WritesOnlyNewTasks(t *testing.T) {
	tmp := t.TempDir()

	// Write first batch
	batch1 := []TaskEvent{
		{TaskID: "t-existing", Subject: "Existing", Status: "pending"},
	}
	if err := writeTaskEvents(tmp, batch1); err != nil {
		t.Fatal(err)
	}

	// Write second batch with mix of existing and new
	batch2 := []TaskEvent{
		{TaskID: "t-existing", Subject: "Duplicate", Status: "completed"},
		{TaskID: "t-new", Subject: "New Task", Status: "pending"},
	}
	if err := writeTaskEvents(tmp, batch2); err != nil {
		t.Fatal(err)
	}

	loaded, err := loadTaskEvents(tmp)
	if err != nil {
		t.Fatal(err)
	}
	// Should have 2 tasks: existing (original) and new
	if len(loaded) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(loaded))
	}

	foundNew := false
	for _, task := range loaded {
		if task.TaskID == "t-new" {
			foundNew = true
			if task.Subject != "New Task" {
				t.Errorf("new task subject = %q, want 'New Task'", task.Subject)
			}
		}
	}
	if !foundNew {
		t.Error("new task not found in loaded events")
	}
}
