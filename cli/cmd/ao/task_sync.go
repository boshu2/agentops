package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/lifecycle"
	"github.com/boshu2/agentops/cli/internal/ratchet"
	"github.com/boshu2/agentops/cli/internal/types"
)

func cloneMap(input map[string]any) map[string]any {
	return lifecycle.CloneStringAnyMap(input)
}

// TaskEvent represents a captured task from Claude Code's Task tool.
type TaskEvent struct {
	// TaskID is the unique identifier from Claude Code's TaskCreate.
	TaskID string `json:"task_id"`

	// Subject is the task title.
	Subject string `json:"subject"`

	// Description is the detailed task description.
	Description string `json:"description,omitempty"`

	// Status is the task state: pending, in_progress, completed.
	Status string `json:"status"`

	// SessionID links to the Claude Code session.
	SessionID string `json:"session_id"`

	// CreatedAt is when the task was created.
	CreatedAt time.Time `json:"created_at"`

	// UpdatedAt is when the task was last modified.
	UpdatedAt time.Time `json:"updated_at"`

	// CompletedAt is when the task was marked completed (if applicable).
	CompletedAt time.Time `json:"completed_at,omitempty"`

	// LearningID links to any learning generated from this task.
	LearningID string `json:"learning_id,omitempty"`

	// Maturity is the CASS maturity level derived from task status.
	Maturity types.Maturity `json:"maturity,omitempty"`

	// Utility is the learned value from feedback signals.
	Utility float64 `json:"utility,omitempty"`

	// Owner is the agent/human assigned to the task.
	Owner string `json:"owner,omitempty"`

	// BlockedBy lists task IDs this task depends on.
	BlockedBy []string `json:"blocked_by,omitempty"`

	// Metadata contains additional task metadata.
	Metadata map[string]any `json:"metadata,omitempty"`
}

// TaskFilePath is the relative path to the task events log.
const TaskFilePath = ".agents/ao/tasks.jsonl"

var taskSyncCmd = &cobra.Command{
	Use:   "task-sync",
	Short: "Sync tasks from Claude Code sessions to CASS",
	Long: `Import and sync tasks from Claude Code's Task tool to CASS maturity tracking.

This command bridges Claude Code's built-in TaskCreate/TaskUpdate/TaskList tools
with the CASS (Contextual Agent Session Search) system, enabling:

  - Task → Learning promotion: Completed tasks can become learnings
  - Status → Maturity mapping: pending→provisional, in_progress→candidate, completed→established
  - Feedback loop closure: Task completion signals update learning utilities

The sync process:
  1. Reads Claude Code transcript for TaskCreate/TaskUpdate tool calls
  2. Extracts task events and stores them in .agents/ao/tasks.jsonl
  3. Maps task status to CASS maturity levels
  4. Links task completion to the feedback loop

Examples:
  ao task-sync                                # Sync from most recent transcript
  ao task-sync --transcript ~/.claude/projects/*/abc.jsonl
  ao task-sync --session session-20260125    # Filter by session
  ao task-sync --promote                     # Promote completed tasks to learnings`,
	RunE: runTaskSync,
}

var (
	taskSyncTranscript string
	taskSyncSessionID  string
	taskSyncPromote    bool
)

func init() {
	taskSyncCmd.Hidden = true
	taskSyncCmd.GroupID = "workflow"
	rootCmd.AddCommand(taskSyncCmd)
	taskSyncCmd.Flags().StringVar(&taskSyncTranscript, "transcript", "", "Path to Claude Code transcript")
	taskSyncCmd.Flags().StringVar(&taskSyncSessionID, "session", "", "Filter tasks by session ID")
	taskSyncCmd.Flags().BoolVar(&taskSyncPromote, "promote", false, "Promote completed tasks to learnings")
}

func runTaskSync(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	transcriptPath := resolveTranscriptPath(taskSyncTranscript)
	if transcriptPath == "" {
		return fmt.Errorf("no transcript found; use --transcript to specify")
	}

	if GetDryRun() {
		fmt.Printf("[dry-run] Would sync tasks from: %s\n", transcriptPath)
		return nil
	}

	tasks, err := extractTaskEvents(transcriptPath, taskSyncSessionID)
	if err != nil {
		return fmt.Errorf("extract tasks: %w", err)
	}

	if len(tasks) == 0 {
		fmt.Println("No task events found in transcript.")
		return nil
	}

	assignMaturityAndUtility(tasks)

	if err := writeTaskEvents(cwd, tasks); err != nil {
		return fmt.Errorf("write tasks: %w", err)
	}

	promoted := promoteCompletedTasks(cwd, tasks, taskSyncPromote)

	return printTaskSyncSummary(transcriptPath, tasks, promoted)
}

// resolveTranscriptPath returns the explicit path if provided, otherwise
// discovers the most recent transcript under ~/.claude/projects.
func resolveTranscriptPath(explicit string) string {
	if explicit != "" {
		return explicit
	}
	homeDir, _ := os.UserHomeDir()
	transcriptsDir := filepath.Join(homeDir, ".claude", "projects")
	return findMostRecentTranscript(transcriptsDir)
}

// assignMaturityAndUtility populates Maturity and default Utility on each task.
func assignMaturityAndUtility(tasks []TaskEvent) {
	for i := range tasks {
		tasks[i].Maturity = statusToMaturity(tasks[i].Status)
		if tasks[i].Utility == 0 {
			tasks[i].Utility = types.InitialUtility
		}
	}
}

// promoteCompletedTasks promotes completed tasks without learnings and returns
// the number of successful promotions. When promote is false it is a no-op.
func promoteCompletedTasks(cwd string, tasks []TaskEvent, promote bool) int {
	if !promote {
		return 0
	}
	promoted := 0
	for _, t := range tasks {
		if t.Status != "completed" || t.LearningID != "" {
			continue
		}
		if err := promoteTaskToLearning(cwd, &t); err != nil {
			VerbosePrintf("Warning: failed to promote task %s: %v\n", t.TaskID, err)
			continue
		}
		promoted++
	}
	return promoted
}

// printTaskSyncSummary renders the sync result as JSON or human-readable text.
func printTaskSyncSummary(transcriptPath string, tasks []TaskEvent, promoted int) error {
	if GetOutput() == "json" {
		result := map[string]any{
			"transcript": transcriptPath,
			"tasks":      tasks,
			"promoted":   promoted,
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}

	fmt.Printf("Task Sync Complete\n")
	fmt.Printf("==================\n")
	fmt.Printf("Transcript:  %s\n", transcriptPath)
	fmt.Printf("Tasks found: %d\n", len(tasks))

	statusCounts := make(map[string]int)
	for _, t := range tasks {
		statusCounts[t.Status]++
	}
	fmt.Printf("\nStatus breakdown:\n")
	for status, count := range statusCounts {
		maturity := statusToMaturity(status)
		fmt.Printf("  %-12s → %-12s: %d\n", status, maturity, count)
	}

	if promoted > 0 {
		fmt.Printf("\nPromoted to learnings: %d\n", promoted)
	}

	return nil
}

// extractTaskEvents parses a Claude Code transcript for Task tool calls.
func extractTaskEvents(transcriptPath, filterSession string) ([]TaskEvent, error) {
	f, err := os.Open(transcriptPath)
	if err != nil {
		return nil, fmt.Errorf("open transcript: %w", err)
	}
	defer func() {
		_ = f.Close() //nolint:errcheck // read-only transcript extraction, close error non-fatal
	}()

	taskMap := make(map[string]*TaskEvent) // Track by task ID for updates
	var currentSessionID string

	scanner := bufio.NewScanner(f)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		currentSessionID = processTranscriptLine(scanner.Text(), filterSession, currentSessionID, taskMap)
	}

	// Convert map to slice
	var tasks []TaskEvent
	for _, t := range taskMap {
		tasks = append(tasks, *t)
	}
	return tasks, nil
}

// processTranscriptLine parses one JSONL line from a transcript and applies
// any task tool calls found in it to taskMap. It returns the (possibly updated)
// session ID so the caller can thread it through successive lines.
func processTranscriptLine(line, filterSession, currentSessionID string, taskMap map[string]*TaskEvent) string {
	newSID, blocks := lifecycle.ProcessTranscriptLine(line, filterSession, currentSessionID)
	for _, block := range blocks {
		applyToolBlock(block, newSID, taskMap)
	}
	return newSID
}

// extractContentBlocks navigates data["message"]["content"] and returns only
// tool_use blocks as typed maps.
func extractContentBlocks(data map[string]any) []map[string]any {
	return lifecycle.ExtractContentBlocks(data)
}

// applyToolBlock dispatches a single tool_use content block to the appropriate
// task handler (TaskCreate or TaskUpdate).
func applyToolBlock(block map[string]any, sessionID string, taskMap map[string]*TaskEvent) {
	toolName, _ := block["name"].(string)
	input, _ := block["input"].(map[string]any)

	switch toolName {
	case "TaskCreate":
		task := parseTaskCreate(input, sessionID)
		if task != nil {
			taskMap[task.TaskID] = task
		}
	case "TaskUpdate":
		taskID, _ := input["taskId"].(string)
		if existing, ok := taskMap[taskID]; ok {
			updateTask(existing, input)
		}
	}
}

// parseTaskCreate extracts a TaskEvent from TaskCreate input.
func parseTaskCreate(input map[string]any, sessionID string) *TaskEvent {
	subject, description, activeForm, metadata := lifecycle.ParseTaskCreate(input)
	if subject == "" {
		return nil
	}

	task := &TaskEvent{
		TaskID:      generateTaskID(),
		Subject:     subject,
		Description: description,
		Status:      "pending",
		SessionID:   sessionID,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Utility:     types.InitialUtility,
		Metadata:    metadata,
	}

	if activeForm != "" {
		if task.Metadata == nil {
			task.Metadata = make(map[string]any)
		}
		task.Metadata["active_form"] = activeForm
	}

	return task
}

// updateTask applies updates from TaskUpdate to an existing task.
func updateTask(task *TaskEvent, input map[string]any) {
	task.UpdatedAt = time.Now()

	status, subject, description, owner := lifecycle.ApplyTaskUpdate(input)
	if status != "" {
		task.Status = status
		task.Maturity = statusToMaturity(status)
		if status == "completed" {
			task.CompletedAt = time.Now()
		}
	}
	if subject != "" {
		task.Subject = subject
	}
	if description != "" {
		task.Description = description
	}
	if owner != "" {
		task.Owner = owner
	}
}

// statusToMaturity maps Task status to CASS maturity.
func statusToMaturity(status string) types.Maturity {
	return lifecycle.StatusToMaturity(status)
}

// generateTaskID creates a unique task identifier.
func generateTaskID() string {
	return fmt.Sprintf("task-%s", time.Now().Format("20060102-150405"))
}

// writeTaskEvents appends task events to the task log.
func writeTaskEvents(baseDir string, tasks []TaskEvent) error {
	if len(tasks) == 0 {
		return nil
	}

	taskPath := filepath.Join(baseDir, TaskFilePath)

	// Create parent directory
	if err := os.MkdirAll(filepath.Dir(taskPath), 0750); err != nil {
		return fmt.Errorf("create task directory: %w", err)
	}

	// Load existing tasks to merge
	existing, _ := loadTaskEvents(baseDir)
	existingMap := make(map[string]bool)
	for _, t := range existing {
		existingMap[t.TaskID] = true
	}

	// Open for append
	f, err := os.OpenFile(taskPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("open task file: %w", err)
	}
	defer func() {
		_ = f.Close() //nolint:errcheck // write complete, close best-effort
	}()

	// Write only new tasks
	written := 0
	for _, task := range tasks {
		if existingMap[task.TaskID] {
			continue
		}
		data, err := json.Marshal(task)
		if err != nil {
			continue
		}
		if _, err := f.Write(append(data, '\n')); err != nil {
			return fmt.Errorf("write task event: %w", err)
		}
		written++
	}

	VerbosePrintf("Wrote %d new task events\n", written)
	return nil
}

// loadTaskEvents reads all task events from the log.
func loadTaskEvents(baseDir string) ([]TaskEvent, error) {
	taskPath := filepath.Join(baseDir, TaskFilePath)

	f, err := os.Open(taskPath)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = f.Close() //nolint:errcheck // read-only task load, close error non-fatal
	}()

	var tasks []TaskEvent
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var task TaskEvent
		if err := json.Unmarshal(scanner.Bytes(), &task); err != nil {
			continue
		}
		tasks = append(tasks, task)
	}

	return tasks, nil
}

// promoteTaskToLearning creates a learning from a completed task.
func promoteTaskToLearning(baseDir string, task *TaskEvent) error {
	learningsDir := filepath.Join(baseDir, ".agents", "learnings")
	if err := os.MkdirAll(learningsDir, 0750); err != nil {
		return fmt.Errorf("create learnings directory: %w", err)
	}

	// Create learning candidate
	learningID := fmt.Sprintf("L-%s", strings.TrimPrefix(task.TaskID, "task-"))
	learningPath := filepath.Join(learningsDir, learningID+".jsonl")

	learning := map[string]any{
		"id":           learningID,
		"type":         "learning",
		"content":      fmt.Sprintf("Task completed: %s", task.Subject),
		"context":      task.Description,
		"maturity":     string(types.MaturityEstablished),
		"utility":      task.Utility,
		"confidence":   0.7, // Initial confidence for promoted tasks
		"extracted_at": time.Now().Format(time.RFC3339),
		"source": map[string]any{
			"session_id": task.SessionID,
			"task_id":    task.TaskID,
			"type":       "task_promotion",
		},
	}

	data, err := json.Marshal(learning)
	if err != nil {
		return fmt.Errorf("marshal learning: %w", err)
	}

	if err := os.WriteFile(learningPath, append(data, '\n'), 0600); err != nil {
		return fmt.Errorf("write learning: %w", err)
	}

	// Update task with learning link
	task.LearningID = learningID

	VerbosePrintf("Promoted task %s to learning %s\n", task.TaskID, learningID)
	return nil
}

// taskFeedbackCmd applies feedback from task outcomes to learnings.
var taskFeedbackCmd = &cobra.Command{
	Use:   "task-feedback",
	Short: "Apply task completion signals to CASS feedback loop",
	Long: `Process task completion events and apply feedback to associated learnings.

This integrates with the CASS feedback loop by:
  - Finding tasks completed in the session
  - Computing reward from task completion signals
  - Updating utility of learnings associated with completed tasks
  - Triggering maturity transitions based on task outcomes

Examples:
  ao task-feedback --session session-20260125
  ao task-feedback --all                      # Process all pending tasks`,
	RunE: runTaskFeedback,
}

var (
	taskFeedbackSessionID string
	taskFeedbackAll       bool
)

func init() {
	taskFeedbackCmd.Hidden = true
	taskFeedbackCmd.GroupID = "workflow"
	rootCmd.AddCommand(taskFeedbackCmd)
	taskFeedbackCmd.Flags().StringVar(&taskFeedbackSessionID, "session", "", "Session ID to process")
	taskFeedbackCmd.Flags().BoolVar(&taskFeedbackAll, "all", false, "Process all tasks without feedback")
}

func runTaskFeedback(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	tasks, err := loadTaskEvents(cwd)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("No tasks found. Run 'ao task-sync' first.")
			return nil
		}
		return fmt.Errorf("load tasks: %w", err)
	}

	processable := filterProcessableTasks(tasks, taskFeedbackSessionID)
	if len(processable) == 0 {
		fmt.Println("No completed tasks with learnings found.")
		return nil
	}

	if GetDryRun() {
		fmt.Printf("[dry-run] Would process feedback for %d tasks\n", len(processable))
		return nil
	}

	processed := 0
	for _, task := range processable {
		if processSingleTaskFeedback(cwd, task) {
			processed++
		}
	}

	fmt.Printf("\nProcessed feedback for %d tasks\n", processed)
	return nil
}

// filterProcessableTasks returns completed tasks with learnings, optionally
// narrowed to a single session.
func filterProcessableTasks(tasks []TaskEvent, sessionFilter string) []TaskEvent {
	var out []TaskEvent
	for _, t := range tasks {
		if sessionFilter != "" && t.SessionID != sessionFilter {
			continue
		}
		if t.Status == "completed" && t.LearningID != "" {
			out = append(out, t)
		}
	}
	return out
}

// processSingleTaskFeedback applies the feedback reward for one completed task,
// updating its learning utility and checking for maturity transition.
// Returns true when feedback was successfully applied.
func processSingleTaskFeedback(cwd string, task TaskEvent) bool {
	const completionReward = 0.8

	learningPath, err := findLearningFile(cwd, task.LearningID)
	if err != nil {
		VerbosePrintf("Warning: learning not found for task %s: %v\n", task.TaskID, err)
		return false
	}

	oldUtility, newUtility, err := updateLearningUtility(learningPath, completionReward, types.DefaultAlpha)
	if err != nil {
		VerbosePrintf("Warning: failed to update %s: %v\n", learningPath, err)
		return false
	}

	result, err := ratchet.CheckMaturityTransition(learningPath)
	if err == nil && result.Transitioned {
		_, _ = ratchet.ApplyMaturityTransition(learningPath)
		VerbosePrintf("Maturity transition: %s → %s\n", result.OldMaturity, result.NewMaturity)
	}

	fmt.Printf("  ✓ %s: %.3f → %.3f (task: %s)\n",
		task.LearningID, oldUtility, newUtility, task.Subject)
	return true
}

// taskStatusCmd shows the status of tasks and their CASS maturity.
var taskStatusCmd = &cobra.Command{
	Use:   "task-status",
	Short: "Show task status and CASS maturity distribution",
	Long: `Display the status of tracked tasks and their CASS maturity levels.

Shows:
  - Task count by status (pending, in_progress, completed)
  - CASS maturity distribution
  - Tasks pending feedback
  - Tasks promoted to learnings

Examples:
  ao task-status
  ao task-status --session session-20260125
  ao task-status --format json`,
	RunE: runTaskStatus,
}

var taskStatusSessionID string

func init() {
	taskStatusCmd.Hidden = true
	taskStatusCmd.GroupID = "workflow"
	rootCmd.AddCommand(taskStatusCmd)
	taskStatusCmd.Flags().StringVar(&taskStatusSessionID, "session", "", "Filter by session ID")
}

func runTaskStatus(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	tasks, err := loadTaskEvents(cwd)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("No tasks found. Run 'ao task-sync' first.")
			return nil
		}
		return fmt.Errorf("load tasks: %w", err)
	}

	tasks = filterTasksBySession(tasks, taskStatusSessionID)

	if len(tasks) == 0 {
		fmt.Println("No tasks found.")
		return nil
	}

	statusCounts, maturityCounts, withLearnings := computeTaskDistributions(tasks)

	if GetOutput() == "json" {
		return outputTaskStatusJSON(tasks, statusCounts, maturityCounts, withLearnings)
	}

	printTaskStatusText(tasks, statusCounts, maturityCounts, withLearnings)
	return nil
}

// filterTasksBySession returns only tasks matching the given session, or all
// tasks when sessionID is empty.
func filterTasksBySession(tasks []TaskEvent, sessionID string) []TaskEvent {
	if sessionID == "" {
		return tasks
	}
	var filtered []TaskEvent
	for _, t := range tasks {
		if t.SessionID == sessionID {
			filtered = append(filtered, t)
		}
	}
	return filtered
}

// computeTaskDistributions tallies status, maturity, and learning counts.
func computeTaskDistributions(tasks []TaskEvent) (map[string]int, map[types.Maturity]int, int) {
	statuses := make([]string, len(tasks))
	maturities := make([]types.Maturity, len(tasks))
	learningIDs := make([]string, len(tasks))
	for i, t := range tasks {
		statuses[i] = t.Status
		maturities[i] = t.Maturity
		learningIDs[i] = t.LearningID
	}
	d := lifecycle.ComputeTaskDistributions(statuses, maturities, learningIDs)
	return d.StatusCounts, d.MaturityCounts, d.WithLearnings
}

// outputTaskStatusJSON writes task status as structured JSON.
func outputTaskStatusJSON(tasks []TaskEvent, statusCounts map[string]int, maturityCounts map[types.Maturity]int, withLearnings int) error {
	result := map[string]any{
		"total":           len(tasks),
		"status_counts":   statusCounts,
		"maturity_counts": maturityCounts,
		"with_learnings":  withLearnings,
		"tasks":           tasks,
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}

// printTaskStatusText renders task status as a human-readable table.
func printTaskStatusText(tasks []TaskEvent, statusCounts map[string]int, maturityCounts map[types.Maturity]int, withLearnings int) {
	fmt.Printf("Task Status\n")
	fmt.Printf("===========\n")
	fmt.Printf("Total tasks: %d\n\n", len(tasks))

	fmt.Printf("By Status:\n")
	for status, count := range statusCounts {
		fmt.Printf("  %-12s: %d\n", status, count)
	}

	fmt.Printf("\nBy CASS Maturity:\n")
	for _, m := range []types.Maturity{types.MaturityProvisional, types.MaturityCandidate, types.MaturityEstablished, types.MaturityAntiPattern} {
		if count, ok := maturityCounts[m]; ok {
			fmt.Printf("  %-12s: %d\n", m, count)
		}
	}

	fmt.Printf("\nPromoted to learnings: %d\n", withLearnings)
}
