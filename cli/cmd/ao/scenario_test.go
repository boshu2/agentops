// practices: [property-based-testing, llm-eval-harness]
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestScenarioInit_CreatesDirectory(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	out, err := executeCommand("scenario", "init")
	if err != nil {
		t.Fatalf("scenario init failed: %v", err)
	}

	holdoutDir := filepath.Join(".agents", "holdout")
	if _, err := os.Stat(holdoutDir); os.IsNotExist(err) {
		t.Fatal("holdout directory not created")
	}
	if _, err := os.Stat(filepath.Join(holdoutDir, "README.md")); os.IsNotExist(err) {
		t.Fatal("README.md not created")
	}
	if !strings.Contains(out, "Initialized holdout directory") {
		t.Errorf("expected init confirmation, got: %s", out)
	}
}

func TestScenarioInit_Idempotent(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	// Run twice
	executeCommand("scenario", "init")
	_, err := executeCommand("scenario", "init")
	if err != nil {
		t.Fatalf("second init should not error: %v", err)
	}
}

func TestScenarioList_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	os.MkdirAll(filepath.Join(".agents", "holdout"), 0755)

	out, err := executeCommand("scenario", "list")
	if err != nil {
		t.Fatalf("list should not error on empty dir: %v", err)
	}
	if !strings.Contains(out, "No scenarios found") {
		t.Fatalf("expected 'No scenarios found', got: %s", out)
	}
}

func TestScenarioList_NoDirectory(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	out, err := executeCommand("scenario", "list")
	if err != nil {
		t.Fatalf("list should not error when dir missing: %v", err)
	}
	if !strings.Contains(out, "No holdout directory found") {
		t.Fatalf("expected missing-dir message, got: %s", out)
	}
}

func TestScenarioList_WithScenarios(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	holdoutDir := filepath.Join(".agents", "holdout")
	os.MkdirAll(holdoutDir, 0755)

	scenario := map[string]interface{}{
		"id": "s-2026-04-05-001", "version": 1, "date": "2026-04-05",
		"goal": "test goal", "narrative": "test narrative",
		"expected_outcome": "test outcome", "satisfaction_threshold": 0.8,
		"status": "active",
	}
	data, _ := json.Marshal(scenario)
	os.WriteFile(filepath.Join(holdoutDir, "s-2026-04-05-001.json"), data, 0644)

	out, err := executeCommand("scenario", "list")
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if !strings.Contains(out, "test goal") {
		t.Fatalf("expected scenario in output, got: %s", out)
	}
}

func TestScenarioList_FilterByStatus(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	holdoutDir := filepath.Join(".agents", "holdout")
	os.MkdirAll(holdoutDir, 0755)

	active := map[string]interface{}{
		"id": "s-2026-04-05-001", "version": 1, "date": "2026-04-05",
		"goal": "active goal", "narrative": "n", "expected_outcome": "o",
		"satisfaction_threshold": 0.8, "status": "active",
	}
	draft := map[string]interface{}{
		"id": "s-2026-04-05-002", "version": 1, "date": "2026-04-05",
		"goal": "draft goal", "narrative": "n", "expected_outcome": "o",
		"satisfaction_threshold": 0.5, "status": "draft",
	}
	d1, _ := json.Marshal(active)
	d2, _ := json.Marshal(draft)
	os.WriteFile(filepath.Join(holdoutDir, "s1.json"), d1, 0644)
	os.WriteFile(filepath.Join(holdoutDir, "s2.json"), d2, 0644)

	out, err := executeCommand("scenario", "list", "--status", "draft")
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if !strings.Contains(out, "draft goal") {
		t.Fatalf("expected draft scenario, got: %s", out)
	}
	if strings.Contains(out, "active goal") {
		t.Fatalf("should not contain active scenario when filtering by draft")
	}
}

func TestScenarioAdd_CreatesSchemaCompliantScenario(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	withScenarioClock(t, time.Date(2026, 4, 24, 10, 30, 0, 0, time.UTC))

	out, err := executeCommand(
		"scenario", "add", "CLI can author holdout scenarios",
		"--narrative", "Evaluator authors a scenario through the CLI.",
		"--expected-outcome", "A schema-compliant scenario file is written.",
		"--status", "active",
		"--source", "agent",
		"--threshold", "0.9",
		"--json",
	)
	if err != nil {
		t.Fatalf("scenario add failed: %v\noutput: %s", err, out)
	}

	var scenario scenarioFile
	if err := json.Unmarshal([]byte(out), &scenario); err != nil {
		t.Fatalf("parse scenario add JSON: %v\noutput: %s", err, out)
	}
	if scenario.ID != "s-2026-04-24-001" {
		t.Fatalf("id = %q, want s-2026-04-24-001", scenario.ID)
	}
	if scenario.Date != "2026-04-24" || scenario.Status != "active" || scenario.Source != "agent" {
		t.Fatalf("unexpected metadata: %+v", scenario)
	}
	if scenario.SatisfactionThreshold != 0.9 {
		t.Fatalf("threshold = %.2f, want 0.90", scenario.SatisfactionThreshold)
	}

	path := filepath.Join(".agents", "holdout", "s-2026-04-24-001.json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("scenario file not written: %v", err)
	}
	validateOut, err := executeCommand("scenario", "validate")
	if err != nil {
		t.Fatalf("written scenario should validate: %v\noutput: %s", err, validateOut)
	}
}

func TestScenarioAdd_IncrementsSameDayID(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	withScenarioClock(t, time.Date(2026, 4, 24, 10, 30, 0, 0, time.UTC))

	holdoutDir := filepath.Join(".agents", "holdout")
	if err := os.MkdirAll(holdoutDir, 0755); err != nil {
		t.Fatalf("mkdir holdout: %v", err)
	}
	existing := map[string]interface{}{
		"id": "s-2026-04-24-003", "version": 1, "date": "2026-04-24",
		"goal": "existing", "narrative": "existing", "expected_outcome": "existing",
		"satisfaction_threshold": 0.8, "status": "draft",
	}
	data, _ := json.Marshal(existing)
	if err := os.WriteFile(filepath.Join(holdoutDir, "legacy.json"), data, 0644); err != nil {
		t.Fatalf("write existing scenario: %v", err)
	}

	out, err := executeCommand("scenario", "add", "next same-day scenario", "--json")
	if err != nil {
		t.Fatalf("scenario add failed: %v\noutput: %s", err, out)
	}
	var scenario scenarioFile
	if err := json.Unmarshal([]byte(out), &scenario); err != nil {
		t.Fatalf("parse scenario add JSON: %v\noutput: %s", err, out)
	}
	if scenario.ID != "s-2026-04-24-004" {
		t.Fatalf("id = %q, want s-2026-04-24-004", scenario.ID)
	}
}

func TestScenarioAdd_RejectsInvalidFlags(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	if out, err := executeCommand("scenario", "add", "bad threshold", "--threshold", "1.2"); err == nil {
		t.Fatalf("scenario add should reject invalid threshold; output: %s", out)
	}
	if out, err := executeCommand("scenario", "add", "bad status", "--status", "blocked"); err == nil {
		t.Fatalf("scenario add should reject invalid status; output: %s", out)
	}
}

func TestScenarioValidate_ValidSchema(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	holdoutDir := filepath.Join(".agents", "holdout")
	os.MkdirAll(holdoutDir, 0755)

	scenario := map[string]interface{}{
		"id": "s-2026-04-05-001", "version": 1, "date": "2026-04-05",
		"goal": "test", "narrative": "test", "expected_outcome": "test",
		"satisfaction_threshold": 0.7, "status": "active", "source": "human",
	}
	data, _ := json.Marshal(scenario)
	os.WriteFile(filepath.Join(holdoutDir, "test.json"), data, 0644)

	out, err := executeCommand("scenario", "validate")
	if err != nil {
		t.Fatalf("validate should pass: %v", err)
	}
	if !strings.Contains(out, "all pass") {
		t.Fatalf("expected 'all pass', got: %s", out)
	}
}

func TestScenarioValidate_AcceptsAutoID(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	holdoutDir := filepath.Join(".agents", "holdout")
	os.MkdirAll(holdoutDir, 0755)

	scenario := map[string]interface{}{
		"id": "auto-agentops-core-cli", "version": 1, "date": "2026-04-24",
		"goal": "test", "narrative": "test", "expected_outcome": "test",
		"satisfaction_threshold": 0.7, "status": "draft", "source": "agent",
	}
	data, _ := json.Marshal(scenario)
	os.WriteFile(filepath.Join(holdoutDir, "auto.json"), data, 0644)

	out, err := executeCommand("scenario", "validate")
	if err != nil {
		t.Fatalf("validate should accept auto-* IDs: %v\noutput: %s", err, out)
	}
	if !strings.Contains(out, "all pass") {
		t.Fatalf("expected 'all pass', got: %s", out)
	}
}

func TestScenarioValidate_InvalidSchema(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	holdoutDir := filepath.Join(".agents", "holdout")
	os.MkdirAll(holdoutDir, 0755)

	// Missing required fields and bad id pattern
	scenario := map[string]interface{}{"id": "bad-id"}
	data, _ := json.Marshal(scenario)
	os.WriteFile(filepath.Join(holdoutDir, "bad.json"), data, 0644)

	_, err := executeCommand("scenario", "validate")
	if err == nil {
		t.Fatal("validate should fail for invalid schema")
	}
}

func TestScenarioValidate_MalformedJSON(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	holdoutDir := filepath.Join(".agents", "holdout")
	os.MkdirAll(holdoutDir, 0755)

	os.WriteFile(filepath.Join(holdoutDir, "bad.json"), []byte("{invalid"), 0644)

	_, err := executeCommand("scenario", "validate")
	if err == nil {
		t.Fatal("validate should fail for malformed JSON")
	}
}

func TestScenarioValidate_NoDirectory(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	out, err := executeCommand("scenario", "validate")
	if err != nil {
		t.Fatalf("validate should not error when dir missing: %v", err)
	}
	if !strings.Contains(out, "No holdout directory found") {
		t.Fatalf("expected missing-dir message, got: %s", out)
	}
}

func TestScenarioValidate_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	os.MkdirAll(filepath.Join(".agents", "holdout"), 0755)

	out, err := executeCommand("scenario", "validate")
	if err != nil {
		t.Fatalf("validate should not error on empty dir: %v", err)
	}
	if !strings.Contains(out, "No scenario files found") {
		t.Fatalf("expected empty message, got: %s", out)
	}
}

func withScenarioClock(t *testing.T, now time.Time) {
	t.Helper()
	orig := scenarioAddNow
	scenarioAddNow = func() time.Time { return now }
	t.Cleanup(func() {
		scenarioAddNow = orig
	})
}

// TestScenarioSchema_ExecutableSpecFields locks the scenario.v1 schema contract
// after F1.1: directive_id + given/when/then are accepted, additionalProperties
// stays false (so unknown keys are still rejected), and the satisfaction
// threshold default is reconciled with the CLI default (0.8).
func TestScenarioSchema_ExecutableSpecFields(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "schemas", "scenario.v1.schema.json"))
	if err != nil {
		t.Fatalf("reading scenario schema: %v", err)
	}
	var schema struct {
		AdditionalProperties bool `json:"additionalProperties"`
		Properties           map[string]struct {
			Type    string          `json:"type"`
			Pattern string          `json:"pattern"`
			Default json.RawMessage `json:"default"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatalf("parsing scenario schema: %v", err)
	}

	if schema.AdditionalProperties {
		t.Error("scenario schema must keep additionalProperties:false")
	}

	did, ok := schema.Properties["directive_id"]
	if !ok {
		t.Fatal("scenario schema missing directive_id property")
	}
	if did.Pattern != "^d-[a-z0-9][a-z0-9-]*$" {
		t.Errorf("directive_id pattern = %q, want ^d-[a-z0-9][a-z0-9-]*$", did.Pattern)
	}

	for _, key := range []string{"given", "when", "then"} {
		p, ok := schema.Properties[key]
		if !ok || p.Type != "array" {
			t.Errorf("scenario schema %q property missing or not an array (type=%q ok=%v)", key, p.Type, ok)
		}
	}

	st, ok := schema.Properties["satisfaction_threshold"]
	if !ok || string(st.Default) != "0.8" {
		t.Errorf("satisfaction_threshold default = %q, want 0.8", string(st.Default))
	}
}
