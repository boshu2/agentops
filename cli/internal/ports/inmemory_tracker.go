// practices: [hexagonal-architecture, ddd-bounded-context]
package ports

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// InMemoryTracker is an IssueTracker test double. It records create calls and
// serves a fixed set of issues for the read paths so consumers can be exercised
// without a real `bd` binary. It is not safe for concurrent use.
type InMemoryTracker struct {
	// Issues backs Ready/List/Show. ReadyStatuses gates which issues Ready
	// returns; an empty set means every issue is ready.
	Issues        []Issue
	ReadyStatuses map[string]bool

	// CreatedEpics and CreatedIssues record create calls in order.
	CreatedEpics  []Issue
	CreatedIssues []Issue

	nextID int
}

// NewInMemoryTracker returns an empty in-memory tracker.
func NewInMemoryTracker() *InMemoryTracker {
	return &InMemoryTracker{
		ReadyStatuses: map[string]bool{},
	}
}

// Mode reports the in-memory backend identity.
func (t *InMemoryTracker) Mode() string { return "memory" }

// CreateEpic records an epic and returns a synthetic id.
func (t *InMemoryTracker) CreateEpic(ctx context.Context, title, body string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if strings.TrimSpace(title) == "" {
		return "", errors.New("ports: InMemoryTracker.CreateEpic title required")
	}
	t.nextID++
	id := fmt.Sprintf("epic-%d", t.nextID)
	issue := Issue{ID: id, Title: title, Type: "epic", Status: "open"}
	t.CreatedEpics = append(t.CreatedEpics, issue)
	t.Issues = append(t.Issues, issue)
	return id, nil
}

// CreateIssue records an issue under epicID and returns a synthetic id.
func (t *InMemoryTracker) CreateIssue(ctx context.Context, epicID, title, body string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if strings.TrimSpace(title) == "" {
		return "", errors.New("ports: InMemoryTracker.CreateIssue title required")
	}
	t.nextID++
	id := fmt.Sprintf("issue-%d", t.nextID)
	issue := Issue{ID: id, Title: title, Type: "task", Status: "open"}
	t.CreatedIssues = append(t.CreatedIssues, issue)
	t.Issues = append(t.Issues, issue)
	return id, nil
}

// Ready returns the recorded issues whose status is in ReadyStatuses (or all
// issues when ReadyStatuses is empty).
func (t *InMemoryTracker) Ready(ctx context.Context) ([]Issue, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	out := make([]Issue, 0, len(t.Issues))
	for _, issue := range t.Issues {
		if len(t.ReadyStatuses) == 0 || t.ReadyStatuses[issue.Status] {
			out = append(out, issue)
		}
	}
	return out, nil
}

// List returns the recorded issues that match filter.
func (t *InMemoryTracker) List(ctx context.Context, filter IssueFilter) ([]Issue, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	out := make([]Issue, 0, len(t.Issues))
	for _, issue := range t.Issues {
		if filter.Type != "" && issue.Type != filter.Type {
			continue
		}
		if filter.Status != "" && issue.Status != filter.Status {
			continue
		}
		out = append(out, issue)
		if filter.Limit > 0 && len(out) >= filter.Limit {
			break
		}
	}
	return out, nil
}

// Show returns the recorded issue with the given id, or an error when absent.
func (t *InMemoryTracker) Show(ctx context.Context, id string) (Issue, error) {
	if err := ctx.Err(); err != nil {
		return Issue{}, err
	}
	for _, issue := range t.Issues {
		if issue.ID == id {
			return issue, nil
		}
	}
	return Issue{}, fmt.Errorf("ports: InMemoryTracker.Show: issue %q not found", id)
}

var _ IssueTracker = (*InMemoryTracker)(nil)
