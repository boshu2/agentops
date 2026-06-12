package main

import (
	"encoding/json"
	"os"
	"testing"
)

func TestCodexTaskPacketSchemaAndExample(t *testing.T) {
	schema, fixture := loadCodexContractPair(t,
		[]string{"schemas", "codex-task-packet.schema.json"},
		[]string{"docs", "contracts", "examples", "codex", "task-packet.json"},
	)

	assertNoTopLevelAdditionalProperties(t, schema)
	assertRequiredFieldsPresent(t, schema, fixture, []string{
		"schema_version",
		"packet_id",
		"created_at",
		"objective",
		"role",
		"cwd",
		"sandbox",
		"auth",
		"dispatch",
		"execution",
		"output",
		"evidence",
		"stop_condition",
	})

	assertEnumMemberAt(t, schema, fixtureString(t, fixture, "role"), "role", "$defs", "Role")
	assertEnumMemberAt(t, schema, fixtureString(t, fixture, "sandbox"), "sandbox", "$defs", "Sandbox")
	assertEnumMemberAt(t, schema, fixtureString(t, fixture, "auth", "required_mode"), "auth.required_mode", "$defs", "AuthGuard", "properties", "required_mode")
	assertEnumMemberAt(t, schema, fixtureString(t, fixture, "execution", "stdin", "mode"), "execution.stdin.mode", "$defs", "Stdin", "properties", "mode")
	assertEnumMemberAt(t, schema, fixtureString(t, fixture, "output", "capture_mode"), "output.capture_mode", "$defs", "OutputContract", "properties", "capture_mode")
	assertEnumMemberAt(t, schema, fixtureString(t, fixture, "resume", "policy"), "resume.policy", "$defs", "Resume", "properties", "policy")

	if got := fixtureString(t, fixture, "auth", "login_status_must_contain"); got != "Logged in using ChatGPT" {
		t.Fatalf("auth.login_status_must_contain = %q, want Logged in using ChatGPT", got)
	}
	if !fixtureStringSliceContains(t, fixture, "OPENAI_API_KEY", "auth", "reject_env") {
		t.Fatalf("auth.reject_env must include OPENAI_API_KEY")
	}
	if got := fixtureBool(t, fixture, "dispatch", "mutates_repo"); got {
		t.Fatalf("dispatch.mutates_repo = true, want false")
	}
	if got := fixtureBool(t, fixture, "execution", "stdin", "close_after_prompt"); !got {
		t.Fatalf("execution.stdin.close_after_prompt = false, want true")
	}
	if got := fixtureNumber(t, fixture, "execution", "timeout_seconds"); got <= 0 {
		t.Fatalf("execution.timeout_seconds = %v, want > 0", got)
	}
}

func TestCodexRunReceiptSchemaAndExample(t *testing.T) {
	schema, fixture := loadCodexContractPair(t,
		[]string{"schemas", "codex-run-receipt.schema.json"},
		[]string{"docs", "contracts", "examples", "codex", "run-receipt.json"},
	)

	assertNoTopLevelAdditionalProperties(t, schema)
	assertRequiredFieldsPresent(t, schema, fixture, []string{
		"schema_version",
		"receipt_id",
		"packet_id",
		"started_at",
		"ended_at",
		"cwd",
		"sandbox",
		"auth_mode",
		"auth_status",
		"command",
		"stdin",
		"timeout_seconds",
		"timed_out",
		"exit_code",
		"outputs",
		"changed_files",
		"commands_run",
		"verdict",
	})

	assertEnumMemberAt(t, schema, fixtureString(t, fixture, "sandbox"), "sandbox", "$defs", "Sandbox")
	assertEnumMemberAt(t, schema, fixtureString(t, fixture, "auth_mode"), "auth_mode", "$defs", "AuthMode")
	assertEnumMemberAt(t, schema, fixtureString(t, fixture, "stdin", "mode"), "stdin.mode", "$defs", "StdinReceipt", "properties", "mode")
	assertEnumMemberAt(t, schema, fixtureString(t, fixture, "verdict", "status"), "verdict.status", "$defs", "Verdict", "properties", "status")

	if got := fixtureString(t, fixture, "auth_status"); got != "Logged in using ChatGPT" {
		t.Fatalf("auth_status = %q, want Logged in using ChatGPT", got)
	}
	if got := fixtureNumber(t, fixture, "timeout_seconds"); got <= 0 {
		t.Fatalf("timeout_seconds = %v, want > 0", got)
	}
	if len(fixtureArray(t, fixture, "commands_run")) == 0 {
		t.Fatalf("commands_run must record acceptance evidence")
	}
	if got := fixtureString(t, fixture, "outputs", "receipt_path"); got == "" {
		t.Fatalf("outputs.receipt_path must be non-empty")
	}
}

func loadCodexContractPair(t *testing.T, schemaParts, fixtureParts []string) (map[string]any, map[string]any) {
	t.Helper()

	schema := readJSONMap(t, findRepoFileForTest(t, schemaParts...))
	fixture := readJSONMap(t, findRepoFileForTest(t, fixtureParts...))
	return schema, fixture
}

func readJSONMap(t *testing.T, path string) map[string]any {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal %s: %v", path, err)
	}
	return out
}

func assertNoTopLevelAdditionalProperties(t *testing.T, schema map[string]any) {
	t.Helper()

	if got, ok := schema["additionalProperties"].(bool); !ok || got {
		t.Fatalf("schema additionalProperties = %v, want false", schema["additionalProperties"])
	}
}

func assertRequiredFieldsPresent(t *testing.T, schema, fixture map[string]any, want []string) {
	t.Helper()

	required := fixtureStringSlice(t, schema, "required")
	for _, key := range want {
		if !containsCodexString(required, key) {
			t.Errorf("schema required missing %q", key)
		}
		value, ok := fixture[key]
		if !ok {
			t.Errorf("fixture missing required key %q", key)
			continue
		}
		if value == nil {
			t.Errorf("fixture required key %q is null", key)
		}
	}
}

func assertEnumMemberAt(t *testing.T, schema map[string]any, got, label string, path ...string) {
	t.Helper()

	node := nestedMap(t, schema, path...)
	enum := fixtureStringSlice(t, node, "enum")
	if len(enum) == 0 {
		t.Fatalf("%s enum not found at %v", label, path)
	}
	if !containsCodexString(enum, got) {
		t.Fatalf("%s = %q is not in schema enum %v", label, got, enum)
	}
}

func nestedMap(t *testing.T, root map[string]any, path ...string) map[string]any {
	t.Helper()

	cur := root
	for _, key := range path {
		next, ok := cur[key].(map[string]any)
		if !ok {
			t.Fatalf("path %v: key %q is %T, want object", path, key, cur[key])
		}
		cur = next
	}
	return cur
}

func fixtureString(t *testing.T, root map[string]any, path ...string) string {
	t.Helper()

	value := nestedValue(t, root, path...)
	got, ok := value.(string)
	if !ok {
		t.Fatalf("%v = %T, want string", path, value)
	}
	return got
}

func fixtureBool(t *testing.T, root map[string]any, path ...string) bool {
	t.Helper()

	value := nestedValue(t, root, path...)
	got, ok := value.(bool)
	if !ok {
		t.Fatalf("%v = %T, want bool", path, value)
	}
	return got
}

func fixtureNumber(t *testing.T, root map[string]any, path ...string) float64 {
	t.Helper()

	value := nestedValue(t, root, path...)
	got, ok := value.(float64)
	if !ok {
		t.Fatalf("%v = %T, want number", path, value)
	}
	return got
}

func fixtureArray(t *testing.T, root map[string]any, path ...string) []any {
	t.Helper()

	value := nestedValue(t, root, path...)
	got, ok := value.([]any)
	if !ok {
		t.Fatalf("%v = %T, want array", path, value)
	}
	return got
}

func fixtureStringSlice(t *testing.T, root map[string]any, path ...string) []string {
	t.Helper()

	raw := fixtureArray(t, root, path...)
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		s, ok := item.(string)
		if !ok {
			t.Fatalf("%v item = %T, want string", path, item)
		}
		out = append(out, s)
	}
	return out
}

func fixtureStringSliceContains(t *testing.T, root map[string]any, want string, path ...string) bool {
	t.Helper()
	return containsCodexString(fixtureStringSlice(t, root, path...), want)
}

func nestedValue(t *testing.T, root map[string]any, path ...string) any {
	t.Helper()

	var cur any = root
	for _, key := range path {
		obj, ok := cur.(map[string]any)
		if !ok {
			t.Fatalf("%v: parent for %q is %T, want object", path, key, cur)
		}
		var exists bool
		cur, exists = obj[key]
		if !exists {
			t.Fatalf("%v: missing key %q", path, key)
		}
	}
	return cur
}

func containsCodexString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
