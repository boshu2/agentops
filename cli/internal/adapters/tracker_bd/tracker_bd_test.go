package tracker_bd

import (
	"reflect"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/ports"
)

func TestNew_Mode(t *testing.T) {
	a := New("/tmp/repo")
	if a.WorkDir != "/tmp/repo" {
		t.Errorf("WorkDir = %q, want %q", a.WorkDir, "/tmp/repo")
	}
	if got := a.Mode(); got != "beads" {
		t.Errorf("Mode() = %q, want %q", got, "beads")
	}
}

func TestReadyArgs(t *testing.T) {
	got := readyArgs()
	want := []string{"ready", "--json"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("readyArgs() = %v, want %v", got, want)
	}
}

func TestListArgs(t *testing.T) {
	tests := []struct {
		name   string
		filter ports.IssueFilter
		want   []string
	}{
		{
			name:   "empty filter",
			filter: ports.IssueFilter{},
			want:   []string{"list", "--json"},
		},
		{
			name:   "type only",
			filter: ports.IssueFilter{Type: "epic"},
			want:   []string{"list", "--type", "epic", "--json"},
		},
		{
			name:   "status with limit",
			filter: ports.IssueFilter{Status: "in_progress", Limit: 500},
			want:   []string{"list", "--status", "in_progress", "--limit", "500", "--json"},
		},
		{
			name:   "metadata field with all",
			filter: ports.IssueFilter{MetadataField: "dream_packet_id=p1", All: true, Limit: 10},
			want:   []string{"list", "--metadata-field", "dream_packet_id=p1", "--all", "--limit", "10", "--json"},
		},
		{
			name:   "zero and negative limit omitted",
			filter: ports.IssueFilter{Type: "task", Limit: -1},
			want:   []string{"list", "--type", "task", "--json"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := listArgs(tt.filter)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("listArgs(%+v) = %v, want %v", tt.filter, got, tt.want)
			}
		})
	}
}

func TestShowArgs(t *testing.T) {
	got := showArgs("soc-123")
	want := []string{"show", "soc-123", "--json"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("showArgs() = %v, want %v", got, want)
	}
}

func TestCreateEpicArgs(t *testing.T) {
	tests := []struct {
		name  string
		title string
		body  string
		want  []string
	}{
		{
			name:  "with body",
			title: "Big epic",
			body:  "context",
			want:  []string{"create", "Big epic", "--type", "epic", "--description", "context", "--json"},
		},
		{
			name:  "no body",
			title: "Bare epic",
			body:  "",
			want:  []string{"create", "Bare epic", "--type", "epic", "--json"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := createEpicArgs(tt.title, tt.body)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("createEpicArgs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCreateIssueArgs(t *testing.T) {
	tests := []struct {
		name   string
		epicID string
		title  string
		body   string
		want   []string
	}{
		{
			name:   "linked to epic",
			epicID: "epic-9",
			title:  "child",
			body:   "do it",
			want:   []string{"create", "child", "--description", "do it", "--deps", "parent-child:epic-9", "--json"},
		},
		{
			name:   "standalone, no body",
			epicID: "",
			title:  "loose",
			body:   "",
			want:   []string{"create", "loose", "--json"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := createIssueArgs(tt.epicID, tt.title, tt.body)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("createIssueArgs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseIssueList(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []ports.Issue
	}{
		{
			name: "empty input",
			in:   "",
			want: []ports.Issue{},
		},
		{
			name: "bare array",
			in:   `[{"id":"a-1","title":"T1","status":"open","type":"task","priority":2,"assignee":"bo","updated_at":"2026-05-23"}]`,
			want: []ports.Issue{{ID: "a-1", Title: "T1", Status: "open", Type: "task", Priority: 2, Assignee: "bo", UpdatedAt: "2026-05-23"}},
		},
		{
			name: "issues envelope",
			in:   `{"issues":[{"id":"a-2","status":"in_progress"}]}`,
			want: []ports.Issue{{ID: "a-2", Status: "in_progress"}},
		},
		{
			name: "beads envelope fallback",
			in:   `{"beads":[{"id":"a-3"}]}`,
			want: []ports.Issue{{ID: "a-3"}},
		},
		{
			name: "empty array",
			in:   `[]`,
			want: []ports.Issue{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseIssueList([]byte(tt.in))
			if err != nil {
				t.Fatalf("parseIssueList() error = %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseIssueList(%q) = %+v, want %+v", tt.in, got, tt.want)
			}
		})
	}
}

func TestParseIssueList_Invalid(t *testing.T) {
	if _, err := parseIssueList([]byte(`{not json`)); err == nil {
		t.Error("parseIssueList(invalid) expected error, got nil")
	}
}

func TestParseIssueShow(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want ports.Issue
	}{
		{
			name: "object",
			in:   `{"id":"s-1","title":"Show","status":"open"}`,
			want: ports.Issue{ID: "s-1", Title: "Show", Status: "open"},
		},
		{
			name: "single-element array",
			in:   `[{"id":"s-2","status":"closed"}]`,
			want: ports.Issue{ID: "s-2", Status: "closed"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseIssueShow(tt.want.ID, []byte(tt.in))
			if err != nil {
				t.Fatalf("parseIssueShow() error = %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseIssueShow(%q) = %+v, want %+v", tt.in, got, tt.want)
			}
		})
	}
}

func TestParseIssueShow_Errors(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		wantSub string
	}{
		{name: "empty", in: "", wantSub: "empty output"},
		{name: "empty array", in: `[]`, wantSub: "not found"},
		{name: "invalid object", in: `{bad`, wantSub: "parse show object"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseIssueShow("x-1", []byte(tt.in))
			if err == nil {
				t.Fatalf("parseIssueShow(%q) expected error", tt.in)
			}
			if !strings.Contains(err.Error(), tt.wantSub) {
				t.Errorf("error = %q, want substring %q", err.Error(), tt.wantSub)
			}
		})
	}
}

func TestParseCreateID(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "object", in: `{"id":"c-1"}`, want: "c-1"},
		{name: "array", in: `[{"id":"c-2"}]`, want: "c-2"},
		{name: "bare quoted id", in: `"c-3"`, want: "c-3"},
		{name: "bare unquoted id", in: `c-4`, want: "c-4"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseCreateID([]byte(tt.in))
			if err != nil {
				t.Fatalf("parseCreateID() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("parseCreateID(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestParseCreateID_Empty(t *testing.T) {
	if _, err := parseCreateID([]byte("")); err == nil {
		t.Error("parseCreateID(empty) expected error, got nil")
	}
}

func TestInterfaceSatisfied(t *testing.T) {
	var tracker ports.IssueTracker = New("")
	if tracker.Mode() != "beads" {
		t.Errorf("Mode() = %q, want %q", tracker.Mode(), "beads")
	}
}
