package bridge

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestNormalizeCodexLifecyclePath(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{"empty string", "", ""},
		{"whitespace only", "   ", ""},
		{"cleans path", "/tmp/../tmp/file.txt", "/tmp/file.txt"},
		{"trims whitespace", "  /tmp/file.txt  ", "/tmp/file.txt"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeCodexLifecyclePath(tt.path)
			if tt.want != "" && got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
			if tt.want == "" && got != "" {
				t.Errorf("got %q, want empty", got)
			}
		})
	}
}

func TestFirstNonEmptyTrimmed(t *testing.T) {
	tests := []struct {
		name   string
		values []string
		want   string
	}{
		{"first non-empty", []string{"", "  ", "hello", "world"}, "hello"},
		{"all empty", []string{"", "  "}, ""},
		{"no values", nil, ""},
		{"first is valid", []string{"first", "second"}, "first"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FirstNonEmptyTrimmed(tt.values...)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCodexStopAlreadyClosed(t *testing.T) {
	tests := []struct {
		name                  string
		lastStopSessionID     string
		lastStopTranscript    string
		sessionID             string
		transcriptPath        string
		want                  bool
	}{
		{
			name:               "matching transcripts same session",
			lastStopSessionID:  "s1",
			lastStopTranscript: "/tmp/transcript.md",
			sessionID:          "s1",
			transcriptPath:     "/tmp/transcript.md",
			want:               true,
		},
		{
			name:               "matching transcripts no session ID",
			lastStopSessionID:  "",
			lastStopTranscript: "/tmp/transcript.md",
			sessionID:          "",
			transcriptPath:     "/tmp/transcript.md",
			want:               true,
		},
		{
			name:               "different transcripts",
			lastStopSessionID:  "s1",
			lastStopTranscript: "/tmp/old.md",
			sessionID:          "s1",
			transcriptPath:     "/tmp/new.md",
			want:               false,
		},
		{
			name:               "session ID match no transcripts",
			lastStopSessionID:  "s1",
			lastStopTranscript: "",
			sessionID:          "s1",
			transcriptPath:     "",
			want:               true,
		},
		{
			name:               "different session IDs no transcripts",
			lastStopSessionID:  "s1",
			lastStopTranscript: "",
			sessionID:          "s2",
			transcriptPath:     "",
			want:               false,
		},
		{
			name:               "no session IDs no transcripts",
			lastStopSessionID:  "",
			lastStopTranscript: "",
			sessionID:          "",
			transcriptPath:     "",
			want:               false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CodexStopAlreadyClosed(tt.lastStopSessionID, tt.lastStopTranscript, tt.sessionID, tt.transcriptPath)
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEnsureStopReason(t *testing.T) {
	if got := EnsureStopReason("already_closed"); !strings.Contains(got, "already recorded") {
		t.Errorf("unexpected reason for already_closed: %q", got)
	}
	if got := EnsureStopReason("new_close"); !strings.Contains(got, "recorded") {
		t.Errorf("unexpected reason for new_close: %q", got)
	}
}

func TestFactoryRecommendedCommands(t *testing.T) {
	noGoal := FactoryRecommendedCommands("")
	if len(noGoal) == 0 {
		t.Fatal("expected commands for empty goal")
	}
	if !strings.Contains(noGoal[0], "Set a concrete goal") {
		t.Errorf("first command should suggest setting a goal, got %q", noGoal[0])
	}

	withGoal := FactoryRecommendedCommands("ship v3")
	if len(withGoal) == 0 {
		t.Fatal("expected commands with goal")
	}
	if !strings.Contains(withGoal[0], "ship v3") {
		t.Errorf("first command should contain goal, got %q", withGoal[0])
	}
}

func TestParseSemverParts(t *testing.T) {
	tests := []struct {
		version string
		want    [3]int
	}{
		{"1.2.3", [3]int{1, 2, 3}},
		{"v1.2.3", [3]int{1, 2, 3}},
		{"0.13.0", [3]int{0, 13, 0}},
		{"1.0.0-beta.1", [3]int{1, 0, 0}},
		{"2.0", [3]int{2, 0, 0}},
		{"", [3]int{0, 0, 0}},
	}
	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			got := ParseSemverParts(tt.version)
			if got != tt.want {
				t.Errorf("ParseSemverParts(%q) = %v, want %v", tt.version, got, tt.want)
			}
		})
	}
}

func TestCompareSemver(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"1.0.0", "1.0.0", 0},
		{"1.0.0", "2.0.0", -1},
		{"2.0.0", "1.0.0", 1},
		{"0.13.0", "0.12.0", 1},
		{"0.12.0", "0.13.0", -1},
		{"v1.0.0", "1.0.0", 0},
	}
	for _, tt := range tests {
		t.Run(tt.a+"_vs_"+tt.b, func(t *testing.T) {
			got := CompareSemver(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("CompareSemver(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestGCBridgeCompatible(t *testing.T) {
	if !GCBridgeCompatible("0.13.0") {
		t.Error("min version should be compatible")
	}
	if !GCBridgeCompatible("1.0.0") {
		t.Error("higher version should be compatible")
	}
	if GCBridgeCompatible("0.12.0") {
		t.Error("lower version should not be compatible")
	}
}

func TestParseGCStatus(t *testing.T) {
	valid := `{
		"city": "test-city",
		"controller": {"running": true, "pid": 1234, "mode": "active"},
		"agents": [{"name": "worker", "running": true, "state": "running"}],
		"summary": {"running": 1, "stopped": 0, "total": 1}
	}`
	status, err := ParseGCStatus([]byte(valid))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.City != "test-city" {
		t.Errorf("city: got %q", status.City)
	}
	if !status.Controller.Running {
		t.Error("controller should be running")
	}
	if len(status.Agents) != 1 {
		t.Errorf("agents: got %d", len(status.Agents))
	}
	if status.Summary.Running != 1 {
		t.Errorf("summary running: got %d", status.Summary.Running)
	}

	_, err = ParseGCStatus([]byte(`{"controller": null, "agents": [], "summary": {}}`))
	if err == nil {
		t.Error("expected error for null controller")
	}

	_, err = ParseGCStatus([]byte(`not json`))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestParseGCSessions(t *testing.T) {
	valid := `[
		{"alias": "main", "state": "running", "id": "s1", "template": "worker"},
		{"Alias": "backup", "State": "stopped", "ID": "s2", "Template": "helper", "Closed": true}
	]`
	sessions, err := ParseGCSessions([]byte(valid))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("got %d sessions", len(sessions))
	}
	if sessions[0].Alias != "main" {
		t.Errorf("session 0 alias: got %q", sessions[0].Alias)
	}
	if sessions[1].Alias != "backup" {
		t.Errorf("session 1 alias (uppercase): got %q", sessions[1].Alias)
	}
	if !sessions[1].Closed {
		t.Error("session 1 should be closed")
	}

	_, err = ParseGCSessions([]byte(`[{"state": "running"}]`))
	if err == nil {
		t.Error("expected error for missing alias field")
	}
}

func TestGCStatusSummaryUnmarshal(t *testing.T) {
	oldShape := `{"running": 2, "stopped": 1, "total": 3}`
	var s1 GCStatusSummary
	if err := json.Unmarshal([]byte(oldShape), &s1); err != nil {
		t.Fatal(err)
	}
	if s1.Running != 2 || s1.Stopped != 1 || s1.Total != 3 {
		t.Errorf("old shape: got %+v", s1)
	}

	newShape := `{"running_agents": 4, "stopped_agents": 2, "total_agents": 6}`
	var s2 GCStatusSummary
	if err := json.Unmarshal([]byte(newShape), &s2); err != nil {
		t.Fatal(err)
	}
	if s2.Running != 4 || s2.Stopped != 2 || s2.Total != 6 {
		t.Errorf("new shape: got %+v", s2)
	}
}

func TestGCSessionNewArgs(t *testing.T) {
	args := GCSessionNewArgs("worker", "my-session")
	if len(args) != 6 {
		t.Fatalf("got %d args: %v", len(args), args)
	}
	if args[0] != "session" || args[1] != "new" || args[2] != "worker" {
		t.Errorf("prefix: %v", args[:3])
	}
	if args[3] != "--alias" || args[4] != "my-session" {
		t.Errorf("alias: %v", args[3:5])
	}
	if args[5] != "--no-attach" {
		t.Errorf("missing --no-attach: %v", args)
	}

	noAlias := GCSessionNewArgs("worker", "")
	if len(noAlias) != 4 {
		t.Fatalf("no-alias got %d args: %v", len(noAlias), noAlias)
	}
}

func TestGCNudgeArgs(t *testing.T) {
	args := GCNudgeArgs("agent-1", "wake up")
	expected := []string{"session", "nudge", "agent-1", "--delivery", "immediate", "wake up"}
	if len(args) != len(expected) {
		t.Fatalf("got %v, want %v", args, expected)
	}
	for i, a := range args {
		if a != expected[i] {
			t.Errorf("arg %d: got %q, want %q", i, a, expected[i])
		}
	}
}

func TestGCPeekArgs(t *testing.T) {
	args := GCPeekArgs("agent-1", 50)
	if args[0] != "session" || args[1] != "peek" || args[2] != "agent-1" || args[4] != "50" {
		t.Errorf("unexpected args: %v", args)
	}
}

func TestGCEventEmitArgs(t *testing.T) {
	args := GCEventEmitArgs("test.event", `{"key":"val"}`)
	if args[0] != "event" || args[1] != "emit" || args[2] != "test.event" {
		t.Errorf("prefix: %v", args[:3])
	}
	if args[3] != "--payload" {
		t.Errorf("missing --payload: %v", args)
	}

	noPayload := GCEventEmitArgs("test.event", "")
	if len(noPayload) != 3 {
		t.Errorf("expected 3 args without payload, got %d: %v", len(noPayload), noPayload)
	}
}

func TestGCEventEmitArgsWithFields(t *testing.T) {
	args := GCEventEmitArgsWithFields("test.event", "actor1", "subject1", "hello", `{"k":"v"}`)
	if len(args) != 11 {
		t.Fatalf("got %d args: %v", len(args), args)
	}

	minimal := GCEventEmitArgsWithFields("test.event", "", "", "", "")
	if len(minimal) != 3 {
		t.Errorf("minimal got %d args: %v", len(minimal), minimal)
	}
}

func TestGCEventsArgs(t *testing.T) {
	full := GCEventsArgs(GCEventsArgsConfig{
		Type:        "rpi.cycle",
		Since:       "1h",
		After:       "evt-123",
		AfterCursor: "cursor-456",
		Watch:       true,
		Follow:      true,
	})
	joined := strings.Join(full, " ")
	for _, want := range []string{"--type", "rpi.cycle", "--since", "1h", "--after", "evt-123", "--after-cursor", "cursor-456", "--watch", "--follow"} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing %q in %v", want, full)
		}
	}

	empty := GCEventsArgs(GCEventsArgsConfig{})
	if len(empty) != 1 || empty[0] != "events" {
		t.Errorf("empty config: got %v", empty)
	}
}

func TestGCSessionListArgs(t *testing.T) {
	args := GCSessionListArgs()
	if len(args) != 3 || args[0] != "session" || args[2] != "--json" {
		t.Errorf("unexpected: %v", args)
	}
}

func TestGCStatusArgs(t *testing.T) {
	noCity := GCStatusArgs("")
	if len(noCity) != 2 || noCity[0] != "status" {
		t.Errorf("no city: %v", noCity)
	}

	withCity := GCStatusArgs("/path/to/city")
	if withCity[0] != "--city" || withCity[1] != "/path/to/city" {
		t.Errorf("with city: %v", withCity)
	}
}
