// practices: [in-toto-provenance, ai-assisted-dev]
package provenanceapp

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// mine runs MineSession into a captured stdout buffer, standing in for the
// former cmd/ao executeCommand harness. Events default to JSONL exactly as the
// command's --json flag default (true).
func mine(t *testing.T, opts MineOptions) (string, error) {
	t.Helper()
	opts.JSON = true
	var buf bytes.Buffer
	err := MineSession(opts, &buf)
	return buf.String(), err
}

func writeMineSession(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func parseMineEvents(t *testing.T, out string) []MineEvent {
	t.Helper()
	var evs []MineEvent
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var ev MineEvent
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			t.Fatalf("unparseable event line %q: %v", line, err)
		}
		evs = append(evs, ev)
	}
	return evs
}

func TestMineSession_ClaudeToolUses_FilterResults(t *testing.T) {
	dir := t.TempDir()
	// A tool_result block must NOT become a tool_call event (it is an output).
	sess := writeMineSession(t, dir, "claude.jsonl",
		`{"type":"user","message":{"role":"user","content":"go"}}
{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","name":"Read","input":{}},{"type":"tool_result","content":"x"}]}}
{"type":"tool_use","tool_name":"Bash","tool_input":{"command":"ls"}}
`)
	out, err := mine(t, MineOptions{File: sess})
	if err != nil {
		t.Fatalf("mine-session: %v\n%s", err, out)
	}
	evs := parseMineEvents(t, out)
	var tools []string
	for _, e := range evs {
		if e.Kind != "tool_call" {
			t.Errorf("kind = %q, want tool_call", e.Kind)
		}
		if e.SchemaVersion != MineEventSchemaVersion {
			t.Errorf("schema_version = %q, want %q", e.SchemaVersion, MineEventSchemaVersion)
		}
		if e.ID == "" {
			t.Errorf("event missing stable id: %+v", e)
		}
		tools = append(tools, e.Tool)
	}
	if got := strings.Join(tools, ","); got != "Read,Bash" {
		t.Fatalf("tools = %q, want Read,Bash (tool_result must be filtered, not emitted)", got)
	}
}

// compileMineEventSchemaForTest loads schemas/provenance-mine-event.v1.schema.json
// (cwd = cli/internal/provenanceapp, so three levels up to the repo root).
func compileMineEventSchemaForTest(t *testing.T) *jsonschema.Schema {
	t.Helper()
	const path = "../../../schemas/provenance-mine-event.v1.schema.json"
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read mine-event schema %s: %v", path, err)
	}
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("parse mine-event schema: %v", err)
	}
	c := jsonschema.NewCompiler()
	if err := c.AddResource("provenance-mine-event.v1.schema.json", doc); err != nil {
		t.Fatalf("add mine-event schema resource: %v", err)
	}
	schema, err := c.Compile("provenance-mine-event.v1.schema.json")
	if err != nil {
		t.Fatalf("compile mine-event schema: %v", err)
	}
	return schema
}

// TestMineSession_EventsValidateAgainstSchema (LOAD-BEARING): every emitted event
// must validate against the checked-in schema. This is what makes the ADR-0010
// "schema-validated events" claim real — it catches drift between the Go MineEvent
// struct and schemas/provenance-mine-event.v1.schema.json.
func TestMineSession_EventsValidateAgainstSchema(t *testing.T) {
	dir := t.TempDir()
	sess := writeMineSession(t, dir, "mix.jsonl",
		`{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","name":"Read","input":{}},{"type":"tool_result","content":"x"}]}}
{"type":"tool_use","tool_name":"Bash","tool_input":{"command":"ls"}}
{"timestamp":"t","type":"response_item","payload":{"type":"function_call","name":"exec_command","arguments":"{}"}}
`)
	out, err := mine(t, MineOptions{File: sess})
	if err != nil {
		t.Fatalf("mine-session: %v\n%s", err, out)
	}
	schema := compileMineEventSchemaForTest(t)
	n := 0
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		inst, perr := jsonschema.UnmarshalJSON(bytes.NewReader([]byte(line)))
		if perr != nil {
			t.Fatalf("parse emitted event %q: %v", line, perr)
		}
		if verr := schema.Validate(inst); verr != nil {
			t.Errorf("emitted event violates provenance-mine-event.v1.schema.json: %v\n%s", verr, line)
		}
		n++
	}
	if n != 3 {
		t.Fatalf("expected 3 schema-valid events (Read, Bash, exec_command), got %d", n)
	}
}

// TestMineSession_TopLevelToolResultNotEmitted (SOUNDNESS regression): a top-level
// Claude tool_result row carries the REAL tool name (e.g. "Bash"), not the literal
// "tool_result", so a name-only filter wrongly emitted it as a tool_call. The
// message Type is the correct family-agnostic discriminator. An OUTPUT row must
// emit zero events.
func TestMineSession_TopLevelToolResultNotEmitted(t *testing.T) {
	dir := t.TempDir()
	sess := writeMineSession(t, dir, "result.jsonl",
		`{"type":"tool_result","tool_name":"Bash","tool_input":{"command":"ls"},"tool_output":"done"}
`)
	out, err := mine(t, MineOptions{File: sess})
	if err != nil {
		t.Fatalf("mine-session: %v\n%s", err, out)
	}
	if evs := parseMineEvents(t, out); len(evs) != 0 {
		t.Fatalf("a top-level tool_result row produced %d events, want 0 (it is an output, not a call): %+v", len(evs), evs)
	}
}

// TestMineSession_RollbackOnContentRewrite (ROLLBACK regression): rewriting an
// already-mined line's tool ARGUMENTS while keeping the same tool name must
// invalidate the watermark and force a clean re-mine. Hashing tool names alone
// trusted the stale prefix.
func TestMineSession_RollbackOnContentRewrite(t *testing.T) {
	dir := t.TempDir()
	state := filepath.Join(dir, "state.json")
	sess := writeMineSession(t, dir, "s.jsonl",
		`{"type":"tool_use","tool_name":"Edit","tool_input":{"file_path":"a.txt"}}
{"type":"tool_use","tool_name":"Read","tool_input":{"file_path":"x.txt"}}
`)
	// Run 1: mine both lines.
	if _, err := mine(t, MineOptions{File: sess, State: state}); err != nil {
		t.Fatal(err)
	}
	rewritten := `{"type":"tool_use","tool_name":"Edit","tool_input":{"file_path":"b.txt"}}
{"type":"tool_use","tool_name":"Read","tool_input":{"file_path":"x.txt"}}
{"type":"tool_use","tool_name":"Bash","tool_input":{"command":"ls"}}
`
	if err := os.WriteFile(sess, []byte(rewritten), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := mine(t, MineOptions{File: sess, State: state})
	if err != nil {
		t.Fatal(err)
	}
	evs := parseMineEvents(t, out)
	if len(evs) != 3 {
		t.Fatalf("rewritten prefix: %d events, want 3 (clean re-mine after rollback): %+v", len(evs), evs)
	}
}

// TestMineSession_StateBoundToFile (DROP-CASE regression): a --state watermark is
// bound to ONE transcript. Reusing it against a DIFFERENT file (whose line-prefix
// happens to line up) must NOT trust the stale line watermark and silently drop
// the new file's events — it must re-mine the new file from the start.
func TestMineSession_StateBoundToFile(t *testing.T) {
	dir := t.TempDir()
	state := filepath.Join(dir, "state.json")
	fileA := writeMineSession(t, dir, "a.jsonl",
		`{"type":"tool_use","tool_name":"Read","tool_input":{"file_path":"x"}}
`)
	if _, err := mine(t, MineOptions{File: fileA, State: state}); err != nil {
		t.Fatal(err)
	}
	fileB := writeMineSession(t, dir, "b.jsonl",
		`{"type":"tool_use","tool_name":"Read","tool_input":{"file_path":"x"}}
`)
	out, err := mine(t, MineOptions{File: fileB, State: state})
	if err != nil {
		t.Fatal(err)
	}
	evs := parseMineEvents(t, out)
	if len(evs) != 1 || evs[0].SessionID != "b" {
		t.Fatalf("state reused across a different file: %d events %+v, want exactly one event for session \"b\" (no silent drop)", len(evs), evs)
	}
}

// TestMineSession_StateBindingIsCwdIndependent (DROP-CASE refinement): the File
// binding must be ABSOLUTE. Two physically different transcripts with the SAME
// relative name (./session.jsonl) run from DIFFERENT working dirs must not compare
// equal.
func TestMineSession_StateBindingIsCwdIndependent(t *testing.T) {
	root := t.TempDir()
	dirA := filepath.Join(root, "a")
	dirB := filepath.Join(root, "b")
	if err := os.MkdirAll(dirA, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dirB, 0o755); err != nil {
		t.Fatal(err)
	}
	const line = `{"type":"tool_use","tool_name":"Read","tool_input":{"file_path":"x"}}` + "\n"
	if err := os.WriteFile(filepath.Join(dirA, "session.jsonl"), []byte(line), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dirB, "session.jsonl"), []byte(line), 0o644); err != nil {
		t.Fatal(err)
	}
	state := filepath.Join(root, "shared.state")

	t.Chdir(dirA)
	if _, err := mine(t, MineOptions{File: "session.jsonl", State: state}); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dirB)
	out, err := mine(t, MineOptions{File: "session.jsonl", State: state})
	if err != nil {
		t.Fatal(err)
	}
	if evs := parseMineEvents(t, out); len(evs) != 1 {
		t.Fatalf("same relative name from a different cwd: %d events, want 1 (binding must be absolute, not cwd-relative)", len(evs))
	}
}

// TestMineSession_DuplicateSameToolOneLineDistinctIDs (UNIQUENESS regression): one
// assistant message can hold several tool_use blocks of the SAME tool (parallel
// Bash/Read calls). Each is a distinct real call and MUST get a distinct id.
func TestMineSession_DuplicateSameToolOneLineDistinctIDs(t *testing.T) {
	dir := t.TempDir()
	sess := writeMineSession(t, dir, "dup.jsonl",
		`{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","name":"Bash","input":{"command":"ls"}},{"type":"tool_use","name":"Bash","input":{"command":"pwd"}}]}}
`)
	out, err := mine(t, MineOptions{File: sess})
	if err != nil {
		t.Fatalf("mine-session: %v\n%s", err, out)
	}
	evs := parseMineEvents(t, out)
	if len(evs) != 2 {
		t.Fatalf("two parallel same-tool calls: %d events, want 2", len(evs))
	}
	if evs[0].ID == evs[1].ID {
		t.Fatalf("duplicate same-tool calls on one line share id %s — a real tool_call would be lost to dedup", evs[0].ID)
	}
	out2, err := mine(t, MineOptions{File: sess})
	if err != nil {
		t.Fatal(err)
	}
	evs2 := parseMineEvents(t, out2)
	ids := map[string]bool{evs[0].ID: true, evs[1].ID: true}
	for _, e := range evs2 {
		if !ids[e.ID] {
			t.Errorf("re-mine produced a different id %s (ordinal not stable across reruns)", e.ID)
		}
	}
}

func TestMineSession_CodexFunctionCalls(t *testing.T) {
	dir := t.TempDir()
	// Codex function_call -> tool_call; function_call_output is the result, skipped.
	sess := writeMineSession(t, dir, "codex.jsonl",
		`{"timestamp":"t","type":"response_item","payload":{"type":"function_call","name":"exec_command","arguments":"{}"}}
{"timestamp":"t","type":"response_item","payload":{"type":"function_call_output","output":"x"}}
`)
	out, err := mine(t, MineOptions{File: sess})
	if err != nil {
		t.Fatalf("mine-session: %v\n%s", err, out)
	}
	evs := parseMineEvents(t, out)
	if len(evs) != 1 || evs[0].Tool != "exec_command" || evs[0].Kind != "tool_call" {
		t.Fatalf("codex events = %+v, want exactly one exec_command tool_call", evs)
	}
}

func TestMineSession_IncrementalIdempotentRollback(t *testing.T) {
	dir := t.TempDir()
	state := filepath.Join(dir, "state.json")
	sess := writeMineSession(t, dir, "s.jsonl",
		`{"type":"tool_use","tool_name":"Read","tool_input":{}}
{"type":"tool_use","tool_name":"Bash","tool_input":{}}
`)

	// Run 1: full mine -> 2 events.
	out1, err := mine(t, MineOptions{File: sess, State: state})
	if err != nil {
		t.Fatal(err)
	}
	ev1 := parseMineEvents(t, out1)
	if len(ev1) != 2 {
		t.Fatalf("run1: %d events, want 2", len(ev1))
	}
	run1IDs := map[string]bool{ev1[0].ID: true, ev1[1].ID: true}

	// Run 2: same file + state -> idempotent, 0 new.
	out2, err := mine(t, MineOptions{File: sess, State: state})
	if err != nil {
		t.Fatal(err)
	}
	if ev2 := parseMineEvents(t, out2); len(ev2) != 0 {
		t.Fatalf("run2 (idempotent re-mine): %d events, want 0", len(ev2))
	}

	// Append a NEW tool line -> incremental: only the new one is mined.
	f, err := os.OpenFile(sess, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.WriteString(`{"type":"tool_use","tool_name":"Edit","tool_input":{}}` + "\n")
	_ = f.Close()
	out3, err := mine(t, MineOptions{File: sess, State: state})
	if err != nil {
		t.Fatal(err)
	}
	ev3 := parseMineEvents(t, out3)
	if len(ev3) != 1 || ev3[0].Tool != "Edit" {
		t.Fatalf("run3 (incremental): %+v, want exactly one Edit", ev3)
	}

	// Rollback: corrupt the state's prefix checksum -> the prior prefix is no
	// longer trusted, so the whole file is re-mined (now 3 events).
	var st mineState
	b, _ := os.ReadFile(state)
	_ = json.Unmarshal(b, &st)
	st.PrefixChecksum = "deadbeefdeadbeef"
	nb, _ := json.MarshalIndent(st, "", "  ")
	_ = os.WriteFile(state, nb, 0o644)
	out4, err := mine(t, MineOptions{File: sess, State: state})
	if err != nil {
		t.Fatal(err)
	}
	ev4 := parseMineEvents(t, out4)
	if len(ev4) != 3 {
		t.Fatalf("run4 (rollback re-mine): %d events, want 3", len(ev4))
	}

	// Stable IDs: re-mining the original two lines yields the SAME ids as run1.
	for _, e := range ev4 {
		if e.Tool == "Read" || e.Tool == "Bash" {
			if !run1IDs[e.ID] {
				t.Errorf("stable-id regression: %s/%s id %s differs from run1 (ids not deterministic)", e.Tool, e.Kind, e.ID)
			}
		}
	}
}
