package provenance

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/clicontract"
)

func TestMineSessionExcerptViewPreservesLiteralTextAndInputs(t *testing.T) {
	dir := t.TempDir()
	source, target := filepath.Join(dir, "session.jsonl"), filepath.Join(dir, "prompt.md")
	text := strings.Repeat("Keep the evidence precise. ", 30)
	record, err := json.Marshal(map[string]any{
		"type": "event_msg", "payload": map[string]any{"type": "user_message", "message": text},
	})
	if err != nil {
		t.Fatal(err)
	}
	record = append(record, '\n')
	for path, data := range map[string][]byte{source: record, target: []byte(text)} {
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	out, err := execProv(t, testLedger(t), "mine-session", "--view", "excerpts", "--file", source, "--target", target)
	if err != nil {
		t.Fatalf("excerpt view: %v", err)
	}
	if !json.Valid([]byte(out)) || !strings.Contains(out, text) || strings.Contains(out, "[truncated]") {
		t.Fatalf("expected one JSON document containing full literal text: %s", out)
	}
	for path, want := range map[string][]byte{source: record, target: []byte(text)} {
		got, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(got, want) {
			t.Fatalf("input changed: %s: %v", path, err)
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 2 {
		t.Fatalf("unexpected state writes: entries=%v err=%v", entries, err)
	}
}

func TestMineSessionRejectsAmbiguousViewsBeforeSourceRead(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"unknown view", []string{"--view", "inference"}, "unknown mine-session view"},
		{"target on events", []string{"--target", "missing"}, "requires --view excerpts"},
		{"bounds on events", []string{"--max-bytes", "12"}, "requires --view excerpts"},
		{"state on excerpts", []string{"--view", "excerpts", "--state", "missing"}, "does not use --state"},
		{"text-only excerpts", []string{"--view", "excerpts", "--json=false"}, "requires JSON"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, err := execProv(t, testLedger(t), append([]string{"mine-session"}, tc.args...)...)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error=%v, want %q", err, tc.want)
			}
			if out != "" {
				t.Fatalf("invalid invocation emitted source output: %q", out)
			}
		})
	}
}

func TestMineSessionExcerptViewRejectsYAML(t *testing.T) {
	m := NewModule(clicontract.HostOptions{OutputMode: func() string { return "yaml" }})
	root := m.Command()
	var out, diagnostics bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&diagnostics)
	root.SetArgs([]string{"mine-session", "--view", "excerpts"})
	err := root.Execute()
	if err == nil || !strings.Contains(err.Error(), "requires JSON") || out.Len() != 0 {
		t.Fatalf("expected JSON-only rejection before reads; err=%v output=%q", err, out.String())
	}
}

func TestMineSessionDefaultRetainsEventsAndCheckpoint(t *testing.T) {
	dir := t.TempDir()
	source, state := filepath.Join(dir, "session.jsonl"), filepath.Join(dir, "cursor.json")
	data := []byte("{\"type\":\"response_item\",\"payload\":{\"type\":\"function_call\",\"name\":\"exec_command\",\"arguments\":\"{\\\"cmd\\\":\\\"true\\\"}\"}}\n")
	if err := os.WriteFile(source, data, 0o600); err != nil {
		t.Fatal(err)
	}
	out, err := execProv(t, testLedger(t), "mine-session", "--file", source, "--state", state)
	if err != nil {
		t.Fatal(err)
	}
	var event struct {
		Kind string `json:"kind"`
		Tool string `json:"tool"`
	}
	if err := json.Unmarshal([]byte(out), &event); err != nil {
		t.Fatal(err)
	}
	if event.Kind != "tool_call" || event.Tool != "exec_command" {
		t.Fatalf("legacy event changed: %+v", event)
	}
	out, err = execProv(t, testLedger(t), "mine-session", "--view", "events", "--file", source, "--state", state)
	if err != nil || out != "" {
		t.Fatalf("checkpoint replay: output=%q err=%v", out, err)
	}
}
