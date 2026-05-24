package ports

import (
	"context"
	"testing"
)

func TestInMemoryTracker_Mode(t *testing.T) {
	tr := NewInMemoryTracker()
	if got := tr.Mode(); got != "memory" {
		t.Errorf("Mode() = %q, want %q", got, "memory")
	}
}

func TestInMemoryTracker_CreateAndShow(t *testing.T) {
	tr := NewInMemoryTracker()
	ctx := context.Background()

	epicID, err := tr.CreateEpic(ctx, "Epic A", "body")
	if err != nil {
		t.Fatalf("CreateEpic() error = %v", err)
	}
	if epicID != "epic-1" {
		t.Errorf("epicID = %q, want %q", epicID, "epic-1")
	}

	issueID, err := tr.CreateIssue(ctx, epicID, "Task 1", "body")
	if err != nil {
		t.Fatalf("CreateIssue() error = %v", err)
	}
	if issueID != "issue-2" {
		t.Errorf("issueID = %q, want %q", issueID, "issue-2")
	}

	if len(tr.CreatedEpics) != 1 || len(tr.CreatedIssues) != 1 {
		t.Fatalf("recorded epics=%d issues=%d, want 1/1", len(tr.CreatedEpics), len(tr.CreatedIssues))
	}

	got, err := tr.Show(ctx, epicID)
	if err != nil {
		t.Fatalf("Show() error = %v", err)
	}
	if got.Title != "Epic A" || got.Type != "epic" {
		t.Errorf("Show() = %+v, want title=Epic A type=epic", got)
	}
}

func TestInMemoryTracker_CreateEmptyTitle(t *testing.T) {
	tr := NewInMemoryTracker()
	ctx := context.Background()
	if _, err := tr.CreateEpic(ctx, "  ", "body"); err == nil {
		t.Error("CreateEpic(empty title) expected error, got nil")
	}
	if _, err := tr.CreateIssue(ctx, "", "", "body"); err == nil {
		t.Error("CreateIssue(empty title) expected error, got nil")
	}
}

func TestInMemoryTracker_ShowNotFound(t *testing.T) {
	tr := NewInMemoryTracker()
	if _, err := tr.Show(context.Background(), "missing"); err == nil {
		t.Error("Show(missing) expected error, got nil")
	}
}

func TestInMemoryTracker_Ready(t *testing.T) {
	tr := NewInMemoryTracker()
	tr.Issues = []Issue{
		{ID: "a", Status: "open"},
		{ID: "b", Status: "blocked"},
		{ID: "c", Status: "open"},
	}
	tr.ReadyStatuses = map[string]bool{"open": true}

	got, err := tr.Ready(context.Background())
	if err != nil {
		t.Fatalf("Ready() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("Ready() returned %d, want 2", len(got))
	}
	if got[0].ID != "a" || got[1].ID != "c" {
		t.Errorf("Ready() ids = %q,%q, want a,c", got[0].ID, got[1].ID)
	}
}

func TestInMemoryTracker_ReadyAllWhenNoStatuses(t *testing.T) {
	tr := NewInMemoryTracker()
	tr.Issues = []Issue{{ID: "a", Status: "open"}, {ID: "b", Status: "x"}}
	got, err := tr.Ready(context.Background())
	if err != nil {
		t.Fatalf("Ready() error = %v", err)
	}
	if len(got) != 2 {
		t.Errorf("Ready() returned %d, want 2 (no status filter)", len(got))
	}
}

func TestInMemoryTracker_List(t *testing.T) {
	tr := NewInMemoryTracker()
	tr.Issues = []Issue{
		{ID: "e1", Type: "epic", Status: "open"},
		{ID: "t1", Type: "task", Status: "in_progress"},
		{ID: "t2", Type: "task", Status: "in_progress"},
	}

	tests := []struct {
		name    string
		filter  IssueFilter
		wantIDs []string
	}{
		{name: "by type", filter: IssueFilter{Type: "epic"}, wantIDs: []string{"e1"}},
		{name: "by status", filter: IssueFilter{Status: "in_progress"}, wantIDs: []string{"t1", "t2"}},
		{name: "with limit", filter: IssueFilter{Status: "in_progress", Limit: 1}, wantIDs: []string{"t1"}},
		{name: "no filter", filter: IssueFilter{}, wantIDs: []string{"e1", "t1", "t2"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tr.List(context.Background(), tt.filter)
			if err != nil {
				t.Fatalf("List() error = %v", err)
			}
			if len(got) != len(tt.wantIDs) {
				t.Fatalf("List() returned %d, want %d", len(got), len(tt.wantIDs))
			}
			for i, id := range tt.wantIDs {
				if got[i].ID != id {
					t.Errorf("List()[%d].ID = %q, want %q", i, got[i].ID, id)
				}
			}
		})
	}
}

func TestInMemoryTracker_ContextCancelled(t *testing.T) {
	tr := NewInMemoryTracker()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := tr.Ready(ctx); err == nil {
		t.Error("Ready(cancelled ctx) expected error, got nil")
	}
	if _, err := tr.List(ctx, IssueFilter{}); err == nil {
		t.Error("List(cancelled ctx) expected error, got nil")
	}
	if _, err := tr.Show(ctx, "x"); err == nil {
		t.Error("Show(cancelled ctx) expected error, got nil")
	}
}
