// Package tracker_bd is a real adapter for the IssueTracker port, backed by the
// `bd` (beads) binary.
//
// It absorbs the issue-tracker coupling that the CLI previously reached by
// building its own exec.Command("bd", ...) at each callsite. The read paths map
// to:
//
//   - Ready  → `bd ready --json`
//   - List   → `bd list [--type T] [--status S] [--all] [--metadata-field K=V] [--limit N] --json`
//   - Show   → `bd show <id> --json`
//
// The create paths map to:
//
//   - CreateEpic  → `bd create <title> --type epic --description <body> --json`
//   - CreateIssue → `bd create <title> --description <body> [--deps parent-child:<epicID>] --json`
//
// Command construction is split from execution (see buildArgs/parse*) so the
// argv and parse logic can be tested directly without a live `bd` binary, per
// the repo's Go testing rules.
package tracker_bd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/boshu2/agentops/cli/internal/ports"
)

// Adapter satisfies ports.IssueTracker using the `bd` binary. WorkDir, when set,
// is the working directory for every bd invocation so the adapter operates
// against a known repository regardless of process cwd.
type Adapter struct {
	// WorkDir is the directory bd commands run in. Empty means the current
	// process working directory.
	WorkDir string
}

// New returns an Adapter rooted at workDir. Pass "" to run bd against the
// process working directory.
func New(workDir string) *Adapter {
	return &Adapter{WorkDir: workDir}
}

// Compile-time interface check.
var _ ports.IssueTracker = (*Adapter)(nil)

// Mode reports the backend identity.
func (a *Adapter) Mode() string { return "beads" }

// bdIssue mirrors the subset of `bd … --json` fields the port exposes. JSON tags
// match bd's snake_case output.
type bdIssue struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Status    string `json:"status"`
	Type      string `json:"type"`
	Priority  int    `json:"priority"`
	Assignee  string `json:"assignee"`
	UpdatedAt string `json:"updated_at"`
}

func (b bdIssue) toPort() ports.Issue {
	return ports.Issue{
		ID:        b.ID,
		Title:     b.Title,
		Status:    b.Status,
		Type:      b.Type,
		Priority:  b.Priority,
		Assignee:  b.Assignee,
		UpdatedAt: b.UpdatedAt,
	}
}

// readyArgs returns the argv for `bd ready --json`.
func readyArgs() []string { return []string{"ready", "--json"} }

// listArgs builds the argv for `bd list` from filter. Order is deterministic so
// it can be asserted in tests.
func listArgs(filter ports.IssueFilter) []string {
	args := []string{"list"}
	if filter.Type != "" {
		args = append(args, "--type", filter.Type)
	}
	if filter.Status != "" {
		args = append(args, "--status", filter.Status)
	}
	if filter.MetadataField != "" {
		args = append(args, "--metadata-field", filter.MetadataField)
	}
	if filter.All {
		args = append(args, "--all")
	}
	if filter.Limit > 0 {
		args = append(args, "--limit", strconv.Itoa(filter.Limit))
	}
	args = append(args, "--json")
	return args
}

// showArgs returns the argv for `bd show <id> --json`.
func showArgs(id string) []string { return []string{"show", id, "--json"} }

// createEpicArgs returns the argv for creating an epic.
func createEpicArgs(title, body string) []string {
	args := []string{"create", title, "--type", "epic"}
	if body != "" {
		args = append(args, "--description", body)
	}
	args = append(args, "--json")
	return args
}

// createIssueArgs returns the argv for creating an issue, optionally linked to a
// parent epic via a parent-child dependency.
func createIssueArgs(epicID, title, body string) []string {
	args := []string{"create", title}
	if body != "" {
		args = append(args, "--description", body)
	}
	if epicID != "" {
		args = append(args, "--deps", "parent-child:"+epicID)
	}
	args = append(args, "--json")
	return args
}

// parseIssueList decodes `bd list/ready --json` output, tolerating either a bare
// array or an {"issues": [...]} / {"beads": [...]} envelope (bd output has
// varied across versions). Empty input yields an empty slice.
func parseIssueList(data []byte) ([]ports.Issue, error) {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return []ports.Issue{}, nil
	}
	if strings.HasPrefix(trimmed, "[") {
		var arr []bdIssue
		if err := json.Unmarshal([]byte(trimmed), &arr); err != nil {
			return nil, fmt.Errorf("tracker_bd: parse list array: %w", err)
		}
		return toPortSlice(arr), nil
	}
	var env struct {
		Issues []bdIssue `json:"issues"`
		Beads  []bdIssue `json:"beads"`
	}
	if err := json.Unmarshal([]byte(trimmed), &env); err != nil {
		return nil, fmt.Errorf("tracker_bd: parse list envelope: %w", err)
	}
	if len(env.Issues) > 0 {
		return toPortSlice(env.Issues), nil
	}
	return toPortSlice(env.Beads), nil
}

// parseIssueShow decodes `bd show <id> --json`, which may emit a single object
// or a 1-element array. An empty array yields a not-found error.
func parseIssueShow(id string, data []byte) (ports.Issue, error) {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return ports.Issue{}, fmt.Errorf("tracker_bd: show %q: empty output", id)
	}
	if strings.HasPrefix(trimmed, "[") {
		var arr []bdIssue
		if err := json.Unmarshal([]byte(trimmed), &arr); err != nil {
			return ports.Issue{}, fmt.Errorf("tracker_bd: parse show array: %w", err)
		}
		if len(arr) == 0 {
			return ports.Issue{}, fmt.Errorf("tracker_bd: show %q: not found", id)
		}
		return arr[0].toPort(), nil
	}
	var rec bdIssue
	if err := json.Unmarshal([]byte(trimmed), &rec); err != nil {
		return ports.Issue{}, fmt.Errorf("tracker_bd: parse show object: %w", err)
	}
	return rec.toPort(), nil
}

// parseCreateID extracts the created id from `bd create … --json`, tolerating a
// bare object, a 1-element array, or a plain-id string fallback.
func parseCreateID(data []byte) (string, error) {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return "", errors.New("tracker_bd: create: empty output")
	}
	if strings.HasPrefix(trimmed, "[") {
		var arr []bdIssue
		if err := json.Unmarshal([]byte(trimmed), &arr); err == nil && len(arr) > 0 && arr[0].ID != "" {
			return arr[0].ID, nil
		}
	}
	if strings.HasPrefix(trimmed, "{") {
		var rec bdIssue
		if err := json.Unmarshal([]byte(trimmed), &rec); err == nil && rec.ID != "" {
			return rec.ID, nil
		}
	}
	// Fallback: bd may print just the id (possibly quoted).
	id := strings.Trim(trimmed, "\"")
	if id == "" {
		return "", fmt.Errorf("tracker_bd: create: no id in output %q", trimmed)
	}
	return id, nil
}

func toPortSlice(in []bdIssue) []ports.Issue {
	out := make([]ports.Issue, 0, len(in))
	for _, b := range in {
		out = append(out, b.toPort())
	}
	return out
}

// Ready returns ready (unblocked) issues via `bd ready --json`.
func (a *Adapter) Ready(ctx context.Context) ([]ports.Issue, error) {
	out, err := a.run(ctx, readyArgs()...)
	if err != nil {
		return nil, err
	}
	return parseIssueList(out)
}

// List returns issues matching filter via `bd list … --json`.
func (a *Adapter) List(ctx context.Context, filter ports.IssueFilter) ([]ports.Issue, error) {
	out, err := a.run(ctx, listArgs(filter)...)
	if err != nil {
		return nil, err
	}
	return parseIssueList(out)
}

// Show returns one issue via `bd show <id> --json`.
func (a *Adapter) Show(ctx context.Context, id string) (ports.Issue, error) {
	if strings.TrimSpace(id) == "" {
		return ports.Issue{}, errors.New("tracker_bd: Show id required")
	}
	out, err := a.run(ctx, showArgs(id)...)
	if err != nil {
		return ports.Issue{}, err
	}
	return parseIssueShow(id, out)
}

// CreateEpic creates an epic via `bd create … --type epic --json`.
func (a *Adapter) CreateEpic(ctx context.Context, title, body string) (string, error) {
	if strings.TrimSpace(title) == "" {
		return "", errors.New("tracker_bd: CreateEpic title required")
	}
	out, err := a.run(ctx, createEpicArgs(title, body)...)
	if err != nil {
		return "", err
	}
	return parseCreateID(out)
}

// CreateIssue creates an issue (optionally linked to epicID) via `bd create … --json`.
func (a *Adapter) CreateIssue(ctx context.Context, epicID, title, body string) (string, error) {
	if strings.TrimSpace(title) == "" {
		return "", errors.New("tracker_bd: CreateIssue title required")
	}
	out, err := a.run(ctx, createIssueArgs(epicID, title, body)...)
	if err != nil {
		return "", err
	}
	return parseCreateID(out)
}

// run invokes bd with args, returning stdout. The working directory is
// a.WorkDir when set. On failure it surfaces bd's stderr in the wrapped error.
func (a *Adapter) run(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "bd", args...)
	if a.WorkDir != "" {
		cmd.Dir = a.WorkDir
	}
	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return nil, fmt.Errorf("tracker_bd: bd %s: %s: %w", strings.Join(args, " "), strings.TrimSpace(string(exitErr.Stderr)), err)
		}
		return nil, fmt.Errorf("tracker_bd: bd %s: %w", strings.Join(args, " "), err)
	}
	return out, nil
}
