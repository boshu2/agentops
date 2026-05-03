package main

import (
	"testing"

	"github.com/boshu2/agentops/cli/internal/types"
)

// estimateTokens, truncateDisplay, hookMessageForStatus tests are in context_test.go

// ---------------------------------------------------------------------------
// applyMarkdownLine (temper.go:405)
// ---------------------------------------------------------------------------

func TestApplyMarkdownLine(t *testing.T) {
	tests := []struct {
		name    string
		line    string
		checkFn func(t *testing.T, meta *artifactMetadata)
	}{
		{
			name: "sets ID",
			line: "**ID**: learn-42",
			checkFn: func(t *testing.T, meta *artifactMetadata) {
				if meta.ID != "learn-42" {
					t.Errorf("ID = %q, want %q", meta.ID, "learn-42")
				}
			},
		},
		{
			name: "sets Maturity lowercase",
			line: "**Maturity**: Candidate",
			checkFn: func(t *testing.T, meta *artifactMetadata) {
				if meta.Maturity != types.MaturityCandidate {
					t.Errorf("Maturity = %q, want %q", meta.Maturity, types.MaturityCandidate)
				}
			},
		},
		{
			name: "sets Utility",
			line: "**Utility**: 0.75",
			checkFn: func(t *testing.T, meta *artifactMetadata) {
				if meta.Utility < 0.74 || meta.Utility > 0.76 {
					t.Errorf("Utility = %f, want ~0.75", meta.Utility)
				}
			},
		},
		{
			name: "sets Confidence",
			line: "**Confidence**: 0.92",
			checkFn: func(t *testing.T, meta *artifactMetadata) {
				if meta.Confidence < 0.91 || meta.Confidence > 0.93 {
					t.Errorf("Confidence = %f, want ~0.92", meta.Confidence)
				}
			},
		},
		{
			name: "sets SchemaVersion",
			line: "**Schema Version**: 3",
			checkFn: func(t *testing.T, meta *artifactMetadata) {
				if meta.SchemaVersion != 3 {
					t.Errorf("SchemaVersion = %d, want 3", meta.SchemaVersion)
				}
			},
		},
		{
			name: "sets Tempered for status tempered",
			line: "**Status**: tempered",
			checkFn: func(t *testing.T, meta *artifactMetadata) {
				if !meta.Tempered {
					t.Error("Tempered = false, want true")
				}
			},
		},
		{
			name: "sets Tempered for status locked",
			line: "**Status**: Locked",
			checkFn: func(t *testing.T, meta *artifactMetadata) {
				if !meta.Tempered {
					t.Error("Tempered = false, want true for Locked")
				}
			},
		},
		{
			name: "does not set Tempered for non-tempered status",
			line: "**Status**: draft",
			checkFn: func(t *testing.T, meta *artifactMetadata) {
				if meta.Tempered {
					t.Error("Tempered = true, want false for draft")
				}
			},
		},
		{
			name: "unrecognized line does not mutate",
			line: "Just some random text",
			checkFn: func(t *testing.T, meta *artifactMetadata) {
				if meta.ID != "" || meta.Maturity != "" || meta.Utility != 0 {
					t.Error("expected zero-value metadata for unrecognized line")
				}
			},
		},
		{
			name: "empty line does not mutate",
			line: "",
			checkFn: func(t *testing.T, meta *artifactMetadata) {
				if meta.ID != "" || meta.Maturity != "" || meta.Utility != 0 {
					t.Error("expected zero-value metadata for empty line")
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var meta artifactMetadata
			applyMarkdownLine(tt.line, &meta)
			tt.checkFn(t, &meta)
		})
	}
}

// ---------------------------------------------------------------------------
// severityEmoji (vibe_check.go:187)
// ---------------------------------------------------------------------------

func TestSeverityEmoji(t *testing.T) {
	tests := []struct {
		name     string
		severity string
		want     string
	}{
		{name: "error severity", severity: "error", want: "\u274c"},
		{name: "info severity", severity: "info", want: "\u2139\ufe0f"},
		{name: "warning (default)", severity: "warning", want: "\u26a0\ufe0f"},
		{name: "unknown falls to default", severity: "unknown", want: "\u26a0\ufe0f"},
		{name: "empty string falls to default", severity: "", want: "\u26a0\ufe0f"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := severityEmoji(tt.severity)
			if got != tt.want {
				t.Errorf("severityEmoji(%q) = %q, want %q", tt.severity, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// statusToMaturity (task_sync.go:373)
// ---------------------------------------------------------------------------

func TestStatusToMaturity(t *testing.T) {
	tests := []struct {
		name   string
		status string
		want   types.Maturity
	}{
		{name: "completed maps to established", status: "completed", want: types.MaturityEstablished},
		{name: "in_progress maps to candidate", status: "in_progress", want: types.MaturityCandidate},
		{name: "pending maps to provisional", status: "pending", want: types.MaturityProvisional},
		{name: "empty string defaults to provisional", status: "", want: types.MaturityProvisional},
		{name: "unknown status defaults to provisional", status: "cancelled", want: types.MaturityProvisional},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := statusToMaturity(tt.status)
			if got != tt.want {
				t.Errorf("statusToMaturity(%q) = %q, want %q", tt.status, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// filterProcessableTasks (task_sync.go:574)
// ---------------------------------------------------------------------------

func TestFilterProcessableTasks(t *testing.T) {
	tasks := []TaskEvent{
		{TaskID: "t1", Status: "completed", LearningID: "L1", SessionID: "s1"},
		{TaskID: "t2", Status: "completed", LearningID: "", SessionID: "s1"}, // no learning
		{TaskID: "t3", Status: "pending", LearningID: "L3", SessionID: "s1"}, // not completed
		{TaskID: "t4", Status: "completed", LearningID: "L4", SessionID: "s2"},
		{TaskID: "t5", Status: "in_progress", LearningID: "L5", SessionID: "s2"},
	}

	tests := []struct {
		name          string
		sessionFilter string
		wantIDs       []string
	}{
		{
			name:          "no session filter returns all completed with learning",
			sessionFilter: "",
			wantIDs:       []string{"t1", "t4"},
		},
		{
			name:          "session filter s1",
			sessionFilter: "s1",
			wantIDs:       []string{"t1"},
		},
		{
			name:          "session filter s2",
			sessionFilter: "s2",
			wantIDs:       []string{"t4"},
		},
		{
			name:          "session filter with no matches",
			sessionFilter: "s99",
			wantIDs:       nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filterProcessableTasks(tasks, tt.sessionFilter)
			gotIDs := make([]string, len(got))
			for i, task := range got {
				gotIDs[i] = task.TaskID
			}
			if len(gotIDs) != len(tt.wantIDs) {
				t.Fatalf("filterProcessableTasks() returned %d tasks %v, want %d tasks %v",
					len(gotIDs), gotIDs, len(tt.wantIDs), tt.wantIDs)
			}
			for i, id := range tt.wantIDs {
				if gotIDs[i] != id {
					t.Errorf("task[%d].TaskID = %q, want %q", i, gotIDs[i], id)
				}
			}
		})
	}

	t.Run("empty input returns nil", func(t *testing.T) {
		got := filterProcessableTasks(nil, "")
		if got != nil {
			t.Errorf("filterProcessableTasks(nil) = %v, want nil", got)
		}
	})
}

// ---------------------------------------------------------------------------
// assignMaturityAndUtility (task_sync.go:154)
// ---------------------------------------------------------------------------

func TestAssignMaturityAndUtility(t *testing.T) {
	t.Run("assigns maturity based on status", func(t *testing.T) {
		tasks := []TaskEvent{
			{TaskID: "t1", Status: "completed"},
			{TaskID: "t2", Status: "in_progress"},
			{TaskID: "t3", Status: "pending"},
		}
		assignMaturityAndUtility(tasks)

		if tasks[0].Maturity != types.MaturityEstablished {
			t.Errorf("tasks[0].Maturity = %q, want %q", tasks[0].Maturity, types.MaturityEstablished)
		}
		if tasks[1].Maturity != types.MaturityCandidate {
			t.Errorf("tasks[1].Maturity = %q, want %q", tasks[1].Maturity, types.MaturityCandidate)
		}
		if tasks[2].Maturity != types.MaturityProvisional {
			t.Errorf("tasks[2].Maturity = %q, want %q", tasks[2].Maturity, types.MaturityProvisional)
		}
	})

	t.Run("assigns default utility when zero", func(t *testing.T) {
		tasks := []TaskEvent{
			{TaskID: "t1", Status: "pending", Utility: 0},
		}
		assignMaturityAndUtility(tasks)

		if tasks[0].Utility != types.InitialUtility {
			t.Errorf("tasks[0].Utility = %f, want %f", tasks[0].Utility, types.InitialUtility)
		}
	})

	t.Run("preserves existing non-zero utility", func(t *testing.T) {
		tasks := []TaskEvent{
			{TaskID: "t1", Status: "completed", Utility: 0.9},
		}
		assignMaturityAndUtility(tasks)

		if tasks[0].Utility != 0.9 {
			t.Errorf("tasks[0].Utility = %f, want 0.9", tasks[0].Utility)
		}
	})

	t.Run("empty slice is no-op", func(t *testing.T) {
		assignMaturityAndUtility(nil)
		// no panic
	})
}
