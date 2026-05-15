package ports

import "context"

// IssueTracker is the driven port for epic/issue creation.
// Implementations: beads (when bd is available), tasklist (filesystem fallback).
type IssueTracker interface {
	Mode() string // "beads" | "tasklist"
	CreateEpic(ctx context.Context, title, body string) (epicID string, err error)
	CreateIssue(ctx context.Context, epicID, title, body string) (issueID string, err error)
}
