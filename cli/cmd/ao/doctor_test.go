package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestComputeResult(t *testing.T) {
	tests := []struct {
		name       string
		checks     []doctorCheck
		wantResult string
		wantFails  bool
	}{
		{
			name: "all pass",
			checks: []doctorCheck{
				{Name: "a", Status: "pass", Required: true},
				{Name: "b", Status: "pass", Required: true},
			},
			wantResult: "HEALTHY",
			wantFails:  false,
		},
		{
			name: "one failure",
			checks: []doctorCheck{
				{Name: "a", Status: "pass", Required: true},
				{Name: "b", Status: "fail", Required: true},
			},
			wantResult: "UNHEALTHY",
			wantFails:  true,
		},
		{
			name: "warnings only",
			checks: []doctorCheck{
				{Name: "a", Status: "pass", Required: true},
				{Name: "b", Status: "warn", Required: false},
			},
			wantResult: "DEGRADED",
			wantFails:  false,
		},
		{
			name: "mixed failures and warnings",
			checks: []doctorCheck{
				{Name: "a", Status: "fail", Required: true},
				{Name: "b", Status: "warn", Required: false},
				{Name: "c", Status: "pass", Required: true},
			},
			wantResult: "UNHEALTHY",
			wantFails:  true,
		},
		{
			name:       "empty checks",
			checks:     []doctorCheck{},
			wantResult: "HEALTHY",
			wantFails:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := computeResult(tt.checks)
			if output.Result != tt.wantResult {
				t.Errorf("computeResult() result = %q, want %q", output.Result, tt.wantResult)
			}
			if tt.wantFails && output.Summary == "all checks passed" {
				t.Error("expected failure in summary")
			}
			if !tt.wantFails && len(tt.checks) > 0 && !hasWarns(tt.checks) {
				expected := fmt.Sprintf("%d/%d checks passed", len(tt.checks), len(tt.checks))
				if output.Summary != expected {
					t.Errorf("expected %q, got %q", expected, output.Summary)
				}
			}
		})
	}
}

func hasWarns(checks []doctorCheck) bool {
	for _, c := range checks {
		if c.Status == "warn" {
			return true
		}
	}
	return false
}

func TestCountFiles(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("empty directory", func(t *testing.T) {
		got := countFiles(tmpDir)
		if got != 0 {
			t.Errorf("countFiles(empty) = %d, want 0", got)
		}
	})

	t.Run("with files", func(t *testing.T) {
		if err := os.WriteFile(filepath.Join(tmpDir, "a.md"), []byte("test"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(tmpDir, "b.md"), []byte("test"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(tmpDir, "subdir"), 0755); err != nil {
			t.Fatal(err)
		}

		got := countFiles(tmpDir)
		if got != 2 {
			t.Errorf("countFiles() = %d, want 2 (should not count directories)", got)
		}
	})

	t.Run("nonexistent directory", func(t *testing.T) {
		got := countFiles(filepath.Join(tmpDir, "nonexistent"))
		if got != 0 {
			t.Errorf("countFiles(nonexistent) = %d, want 0", got)
		}
	})
}

// --- Integration tests for doctor check functions ---

// chdirTemp moved to testutil_test.go.

func TestCheckKnowledgeBase(t *testing.T) {
	t.Run("initialized", func(t *testing.T) {
		tmp := chdirTemp(t)
		if err := os.MkdirAll(filepath.Join(tmp, ".agents", "ao"), 0755); err != nil {
			t.Fatal(err)
		}
		result := checkKnowledgeBase()
		if result.Status != "pass" {
			t.Errorf("status=%q, want pass (detail: %s)", result.Status, result.Detail)
		}
	})

	t.Run("not initialized", func(t *testing.T) {
		chdirTemp(t)
		result := checkKnowledgeBase()
		if result.Status != "fail" {
			t.Errorf("status=%q, want fail (detail: %s)", result.Status, result.Detail)
		}
	})
}

func TestCheckKnowledgeFreshness(t *testing.T) {
	t.Run("recent session", func(t *testing.T) {
		tmp := chdirTemp(t)
		sessDir := filepath.Join(tmp, ".agents", "ao", "sessions")
		if err := os.MkdirAll(sessDir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(sessDir, "session-1.md"), []byte("recent"), 0644); err != nil {
			t.Fatal(err)
		}

		result := checkKnowledgeFreshness()
		if result.Status != "pass" {
			t.Errorf("status=%q, want pass (detail: %s)", result.Status, result.Detail)
		}
	})

	t.Run("no sessions", func(t *testing.T) {
		tmp := chdirTemp(t)
		if err := os.MkdirAll(filepath.Join(tmp, ".agents", "ao", "sessions"), 0755); err != nil {
			t.Fatal(err)
		}

		result := checkKnowledgeFreshness()
		if result.Status != "warn" {
			t.Errorf("status=%q, want warn (detail: %s)", result.Status, result.Detail)
		}
	})

	t.Run("no sessions dir", func(t *testing.T) {
		chdirTemp(t)
		result := checkKnowledgeFreshness()
		if result.Status != "warn" {
			t.Errorf("status=%q, want warn (detail: %s)", result.Status, result.Detail)
		}
	})
}

func TestCheckSearchIndex(t *testing.T) {
	t.Run("index exists with content", func(t *testing.T) {
		tmp := chdirTemp(t)
		indexDir := filepath.Join(tmp, IndexDir)
		if err := os.MkdirAll(indexDir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(indexDir, IndexFileName), []byte("{\"term\":\"hello\"}\n{\"term\":\"world\"}\n"), 0644); err != nil {
			t.Fatal(err)
		}

		result := checkSearchIndex()
		if result.Status != "pass" {
			t.Errorf("status=%q, want pass (detail: %s)", result.Status, result.Detail)
		}
	})

	t.Run("empty index", func(t *testing.T) {
		tmp := chdirTemp(t)
		indexDir := filepath.Join(tmp, IndexDir)
		if err := os.MkdirAll(indexDir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(indexDir, IndexFileName), []byte(""), 0644); err != nil {
			t.Fatal(err)
		}

		result := checkSearchIndex()
		if result.Status != "warn" {
			t.Errorf("status=%q, want warn (detail: %s)", result.Status, result.Detail)
		}
	})

	t.Run("no index", func(t *testing.T) {
		chdirTemp(t)
		result := checkSearchIndex()
		if result.Status != "warn" {
			t.Errorf("status=%q, want warn (detail: %s)", result.Status, result.Detail)
		}
	})
}

func TestCheckFlywheelHealth(t *testing.T) {
	t.Run("with learnings", func(t *testing.T) {
		tmp := t.TempDir()
		learningsDir := filepath.Join(tmp, ".agents", "ao", "learnings")
		if err := os.MkdirAll(learningsDir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(learningsDir, "L1.md"), []byte("learning 1"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(learningsDir, "L2.md"), []byte("learning 2"), 0644); err != nil {
			t.Fatal(err)
		}

		result := checkFlywheelHealth(tmp)
		if result.Status != "pass" {
			t.Errorf("status=%q, want pass (detail: %s)", result.Status, result.Detail)
		}
	})

	t.Run("no learnings", func(t *testing.T) {
		tmp := t.TempDir()
		result := checkFlywheelHealth(tmp)
		if result.Status != "warn" {
			t.Errorf("status=%q, want warn (detail: %s)", result.Status, result.Detail)
		}
	})

	t.Run("alt path learnings", func(t *testing.T) {
		tmp := t.TempDir()
		altDir := filepath.Join(tmp, ".agents", "learnings")
		if err := os.MkdirAll(altDir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(altDir, "L1.md"), []byte("learning"), 0644); err != nil {
			t.Fatal(err)
		}

		result := checkFlywheelHealth(tmp)
		if result.Status != "pass" {
			t.Errorf("status=%q, want pass (detail: %s)", result.Status, result.Detail)
		}
	})
}

func TestCountHooksInMap(t *testing.T) {
	tests := []struct {
		name string
		raw  any
		want int
	}{
		{
			name: "flat hook arrays",
			raw: map[string]any{
				"PreToolUse":  []any{"hook1", "hook2"},
				"PostToolUse": []any{"hook3"},
			},
			want: 3,
		},
		{
			name: "empty map",
			raw:  map[string]any{},
			want: 0,
		},
		{
			name: "nested hooks map",
			raw: map[string]any{
				"hooks": map[string]any{
					"PreToolUse": []any{"h1"},
				},
			},
			want: 1,
		},
		{
			name: "nil input",
			raw:  nil,
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := countHooksInMap(tt.raw)
			if got != tt.want {
				t.Errorf("countHooksInMap() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestCountFileLines(t *testing.T) {
	tmp := t.TempDir()

	t.Run("file with lines", func(t *testing.T) {
		path := filepath.Join(tmp, "test.jsonl")
		if err := os.WriteFile(path, []byte("{\"a\":1}\n{\"b\":2}\n{\"c\":3}\n"), 0644); err != nil {
			t.Fatal(err)
		}
		got := countFileLines(path)
		if got != 3 {
			t.Errorf("countFileLines() = %d, want 3", got)
		}
	})

	t.Run("empty file", func(t *testing.T) {
		path := filepath.Join(tmp, "empty.jsonl")
		if err := os.WriteFile(path, []byte(""), 0644); err != nil {
			t.Fatal(err)
		}
		got := countFileLines(path)
		if got != 0 {
			t.Errorf("countFileLines() = %d, want 0", got)
		}
	})

	t.Run("nonexistent file", func(t *testing.T) {
		got := countFileLines(filepath.Join(tmp, "nope"))
		if got != 0 {
			t.Errorf("countFileLines(nonexistent) = %d, want 0", got)
		}
	})

	t.Run("blank lines ignored", func(t *testing.T) {
		path := filepath.Join(tmp, "blanks.jsonl")
		if err := os.WriteFile(path, []byte("line1\n\n  \nline2\n"), 0644); err != nil {
			t.Fatal(err)
		}
		got := countFileLines(path)
		if got != 2 {
			t.Errorf("countFileLines() = %d, want 2", got)
		}
	})
}

func TestFormatNumber(t *testing.T) {
	tests := []struct {
		input int
		want  string
	}{
		{0, "0"},
		{42, "42"},
		{999, "999"},
		{1000, "1,000"},
		{1247, "1,247"},
		{1000000, "1,000,000"},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%d", tt.input), func(t *testing.T) {
			got := formatNumber(tt.input)
			if got != tt.want {
				t.Errorf("formatNumber(%d) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name  string
		input time.Duration
		want  string
	}{
		{"seconds", 30 * time.Second, "30s"},
		{"minutes", 5 * time.Minute, "5m"},
		{"hours", 3 * time.Hour, "3h"},
		{"days", 48 * time.Hour, "2d"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatDuration(tt.input)
			if got != tt.want {
				t.Errorf("formatDuration() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCountHealFindings(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   int
	}{
		{
			name:   "no findings",
			output: "All clean. No findings.",
			want:   0,
		},
		{
			name:   "report format findings",
			output: "[MISSING_NAME] skills/foo: No name field in frontmatter\n[DEAD_REF] skills/bar: SKILL.md references non-existent references/old.md\n\n2 finding(s) detected.\n",
			want:   2,
		},
		{
			name:   "summary only fallback",
			output: "some noise\n5 finding(s) detected.\n",
			want:   5,
		},
		{
			name:   "empty output",
			output: "",
			want:   0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := countHealFindings(tt.output)
			if got != tt.want {
				t.Errorf("countHealFindings() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestCheckSkillIntegrity_NoHealScript(t *testing.T) {
	// When run from a temp dir with no heal.sh, should warn gracefully.
	// Override HOME so findHealScript() can't find ~/.claude/skills/heal-skill/.
	chdirTemp(t)
	origHome := os.Getenv("HOME")
	t.Setenv("HOME", t.TempDir())
	t.Cleanup(func() { os.Setenv("HOME", origHome) })
	result := checkSkillIntegrity()
	if result.Status != "warn" {
		t.Errorf("status=%q, want warn (detail: %s)", result.Status, result.Detail)
	}
	if result.Required {
		t.Error("Skill Integrity should not be required")
	}
}

func TestCheckSkillIntegrity_WithHealScript(t *testing.T) {
	// Create a minimal heal.sh that exits 0 (all clean)
	tmp := chdirTemp(t)
	skillDir := filepath.Join(tmp, "skills", "heal-skill", "scripts")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatal(err)
	}
	healScript := filepath.Join(skillDir, "heal.sh")
	if err := os.WriteFile(healScript, []byte("#!/usr/bin/env bash\necho 'All clean. No findings.'\nexit 0\n"), 0755); err != nil {
		t.Fatal(err)
	}

	result := checkSkillIntegrity()
	if result.Status != "pass" {
		t.Errorf("status=%q, want pass (detail: %s)", result.Status, result.Detail)
	}
}

func TestSkillOverlapWarning_NoOverlap(t *testing.T) {
	base := map[string]struct{}{"a": {}, "b": {}}
	other := map[string]struct{}{"c": {}, "d": {}}
	result := skillOverlapWarning(base, 10, "primary", "%d found in %s; %d overlap: %s", other)
	if result != nil {
		t.Errorf("expected nil for non-overlapping sets, got %+v", result)
	}
}

func TestSkillOverlapWarning_WithOverlap(t *testing.T) {
	base := map[string]struct{}{"a": {}, "b": {}, "c": {}}
	other := map[string]struct{}{"b": {}, "c": {}, "d": {}}
	result := skillOverlapWarning(base, 10, "primary", "%d found in %s; %d overlap: %s", other)
	if result == nil {
		t.Fatal("expected warning for overlapping sets, got nil")
	}
	if result.Status != "warn" {
		t.Errorf("status=%q, want warn", result.Status)
	}
	if !strings.Contains(result.Detail, "2 overlap") {
		t.Errorf("detail=%q, want '2 overlap' substring", result.Detail)
	}
}

func TestScanSkillDir_NonexistentDir(t *testing.T) {
	result := scanSkillDir("/nonexistent/path/that/does/not/exist")
	if result != nil {
		t.Errorf("expected nil for nonexistent dir, got %v", result)
	}
}

func TestScanSkillDir_WithSkills(t *testing.T) {
	tmp := t.TempDir()
	// Create a skill directory with SKILL.md
	skillDir := filepath.Join(tmp, "my-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# skill"), 0644); err != nil {
		t.Fatal(err)
	}
	// Create a non-skill directory (no SKILL.md)
	if err := os.MkdirAll(filepath.Join(tmp, "not-a-skill"), 0755); err != nil {
		t.Fatal(err)
	}

	result := scanSkillDir(tmp)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if _, ok := result["my-skill"]; !ok {
		t.Error("expected 'my-skill' in result")
	}
	if _, ok := result["not-a-skill"]; ok {
		t.Error("did not expect 'not-a-skill' in result (no SKILL.md)")
	}
}

func TestCheckSkillIntegrity_WithFindings(t *testing.T) {
	// Create a heal.sh that reports findings and exits 1
	tmp := chdirTemp(t)
	skillDir := filepath.Join(tmp, "skills", "heal-skill", "scripts")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatal(err)
	}
	healScript := filepath.Join(skillDir, "heal.sh")
	script := `#!/usr/bin/env bash
echo "[MISSING_NAME] skills/foo: No name field"
echo "[DEAD_REF] skills/bar: references non-existent file"
echo ""
echo "2 finding(s) detected."
exit 1
`
	if err := os.WriteFile(healScript, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}

	result := checkSkillIntegrity()
	if result.Status != "warn" {
		t.Errorf("status=%q, want warn (detail: %s)", result.Status, result.Detail)
	}
}

func TestFileExists(t *testing.T) {
	tmp := t.TempDir()
	f := filepath.Join(tmp, "exists.txt")
	if err := os.WriteFile(f, []byte("hi"), 0644); err != nil {
		t.Fatal(err)
	}
	if !fileExists(f) {
		t.Error("expected fileExists to return true for existing file")
	}
	if fileExists(filepath.Join(tmp, "nope.txt")) {
		t.Error("expected fileExists to return false for non-existent file")
	}
}

func TestDoctorStaleReplacementsExist(t *testing.T) {
	// Every replacement (value) in deprecatedCommands should resolve to a
	// real Cobra command registered under rootCmd.
	for old, newCmd := range deprecatedCommands {
		parts := strings.Fields(newCmd)
		if len(parts) < 2 {
			t.Errorf("deprecatedCommands[%q] = %q — too few parts", old, newCmd)
			continue
		}
		// Walk rootCmd to find the command (skip "ao" prefix)
		cmd, _, err := rootCmd.Find(parts[1:])
		if err != nil {
			t.Errorf("deprecatedCommands[%q] = %q — command not found: %v", old, newCmd, err)
			continue
		}
		// rootCmd.Find returns rootCmd itself when the command doesn't
		// match any subcommand. That means the replacement is dead.
		if cmd == rootCmd {
			t.Errorf("deprecatedCommands[%q] = %q — resolved to root (command does not exist)", old, newCmd)
		}
	}
}

func TestRenderDoctorTable_Healthy(t *testing.T) {
	output := doctorOutput{
		Checks: []doctorCheck{
			{Name: "ao CLI", Status: "pass", Detail: "v2.0.0", Required: true},
			{Name: "Knowledge", Status: "pass", Detail: "ok", Required: true},
		},
		Result:  "HEALTHY",
		Summary: "2/2 checks passed",
	}
	var buf bytes.Buffer
	renderDoctorTable(&buf, output)
	rendered := buf.String()

	if !strings.Contains(rendered, "ao doctor") {
		t.Error("expected header 'ao doctor'")
	}
	if !strings.Contains(rendered, "ao CLI") {
		t.Error("expected check name 'ao CLI'")
	}
	if !strings.Contains(rendered, "2/2 checks passed") {
		t.Error("expected summary in output")
	}
}

func TestRenderDoctorTable_WithWarnings(t *testing.T) {
	output := doctorOutput{
		Checks: []doctorCheck{
			{Name: "Pass Check", Status: "pass", Detail: "ok", Required: true},
			{Name: "Warn Check", Status: "warn", Detail: "degraded", Required: false},
			{Name: "Fail Check", Status: "fail", Detail: "broken", Required: true},
			{Name: "Unknown", Status: "unknown", Detail: "???", Required: false},
		},
		Result:  "UNHEALTHY",
		Summary: "1/4 checks passed, 1 warning, 1 failed",
	}
	var buf bytes.Buffer
	renderDoctorTable(&buf, output)
	rendered := buf.String()

	// All status icons should appear
	if !strings.Contains(rendered, "\u2713") {
		t.Error("expected pass icon")
	}
	if !strings.Contains(rendered, "!") {
		t.Error("expected warn icon")
	}
	if !strings.Contains(rendered, "\u2717") {
		t.Error("expected fail icon")
	}
	if !strings.Contains(rendered, "?") {
		t.Error("expected unknown icon")
	}
}

func TestRenderDoctorTable_Empty(t *testing.T) {
	output := doctorOutput{
		Checks:  []doctorCheck{},
		Result:  "HEALTHY",
		Summary: "0/0 checks passed",
	}
	var buf bytes.Buffer
	renderDoctorTable(&buf, output)
	if buf.Len() == 0 {
		t.Error("expected non-empty output even with no checks")
	}
}

// ---------------------------------------------------------------------------
// doctorStatusIcon (already 100%, but test unknown branch explicitly)
// ---------------------------------------------------------------------------

func TestDoctorStatusIcon_Unknown(t *testing.T) {
	got := doctorStatusIcon("bogus")
	if got != "?" {
		t.Errorf("doctorStatusIcon(bogus) = %q, want ?", got)
	}
}

// ---------------------------------------------------------------------------
// checkOptionalCLI (0%)
// ---------------------------------------------------------------------------

func TestCheckOptionalCLI_Available(t *testing.T) {
	// "go" should be in PATH during tests
	result := checkOptionalCLI("go", "test builds")
	if result.Status != "pass" {
		t.Errorf("status=%q, want pass for 'go' CLI (detail: %s)", result.Status, result.Detail)
	}
	if result.Required {
		t.Error("optional CLI should not be required")
	}
	if !strings.Contains(result.Detail, "available") {
		t.Errorf("expected 'available' in detail, got %q", result.Detail)
	}
}

func TestCheckOptionalCLI_NotAvailable(t *testing.T) {
	result := checkOptionalCLI("nonexistent_cli_xyz_999", "test feature")
	if result.Status != "warn" {
		t.Errorf("status=%q, want warn for missing CLI (detail: %s)", result.Status, result.Detail)
	}
	if !strings.Contains(result.Detail, "not found") {
		t.Errorf("expected 'not found' in detail, got %q", result.Detail)
	}
	if !strings.Contains(result.Detail, "test feature") {
		t.Errorf("expected reason in detail, got %q", result.Detail)
	}
}

// ---------------------------------------------------------------------------
// checkCLIDependencies (0%)
// ---------------------------------------------------------------------------

func TestCheckCLIDependencies(t *testing.T) {
	// We can't guarantee gt/bd availability, but we can exercise the function.
	result := checkCLIDependencies()
	// It should return either pass or warn, never fail
	if result.Status != "pass" && result.Status != "warn" {
		t.Errorf("status=%q, expected pass or warn", result.Status)
	}
	if result.Name != "CLI Dependencies" {
		t.Errorf("name=%q, want 'CLI Dependencies'", result.Name)
	}
	if result.Required {
		t.Error("CLI Dependencies should not be required")
	}
}

// ---------------------------------------------------------------------------
// checkHookCoverage (0%) — needs a fake HOME with settings.json
// ---------------------------------------------------------------------------

func TestCheckHookCoverage_NoHooksFile(t *testing.T) {
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	result := checkHookCoverage()
	if result.Status != "warn" {
		t.Errorf("status=%q, want warn when no hooks files exist (detail: %s)", result.Status, result.Detail)
	}
}

func TestCheckHookCoverage_SettingsWithHooks(t *testing.T) {
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	claudeDir := filepath.Join(fakeHome, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Build a settings.json with hooks containing all events
	hooks := make(map[string]any)
	for _, event := range AllEventNames() {
		hooks[event] = []any{
			map[string]any{
				"matcher": "",
				"hooks": []any{
					map[string]any{"type": "command", "command": "ao hook-dispatch " + event},
				},
			},
		}
	}
	settings := map[string]any{"hooks": hooks}
	data, err := json.Marshal(settings)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(claudeDir, "settings.json"), data, 0644); err != nil {
		t.Fatal(err)
	}

	result := checkHookCoverage()
	// Should find hooks and report coverage
	if result.Status == "fail" {
		t.Errorf("status=fail unexpected (detail: %s)", result.Detail)
	}
}

func TestCheckHookCoverage_FallbackHooksJSON(t *testing.T) {
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	claudeDir := filepath.Join(fakeHome, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatal(err)
	}

	// hooks.json with a single event (partial coverage)
	hooks := map[string]any{
		"SessionStart": []any{
			map[string]any{
				"matcher": "",
				"hooks": []any{
					map[string]any{"type": "command", "command": "ao hook-dispatch SessionStart"},
				},
			},
		},
	}
	data, err := json.Marshal(hooks)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(claudeDir, "hooks.json"), data, 0644); err != nil {
		t.Fatal(err)
	}

	result := checkHookCoverage()
	// Should detect partial coverage
	if result.Status == "fail" {
		t.Errorf("status=fail unexpected for partial hooks.json (detail: %s)", result.Detail)
	}
}

func TestUsesRuntimeManifestContract(t *testing.T) {
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	claudeDir := filepath.Join(fakeHome, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatal(err)
	}
	settings := map[string]any{
		"hooks": map[string]any{
			"SessionStart": []any{
				map[string]any{
					"hooks": []any{
						map[string]any{"type": "command", "command": "ao inject --apply-decay"},
					},
				},
			},
		},
	}
	data, err := json.Marshal(settings)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(claudeDir, "settings.json"), data, 0644); err != nil {
		t.Fatal(err)
	}

	tmp := t.TempDir()
	hooksDir := filepath.Join(tmp, "hooks")
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		t.Fatal(err)
	}
	manifest := `{
		"hooks": {
			"SessionStart": [{"hooks": [{"type":"command","command":"ao inject --apply-decay"}]}]
		}
	}`
	if err := os.WriteFile(filepath.Join(hooksDir, "hooks.json"), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}

	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWD) })

	result := checkHookCoverage()
	if result.Status != "pass" {
		t.Fatalf("expected pass with 1/1 active manifest event, got %q (%s)", result.Status, result.Detail)
	}
	if !strings.Contains(result.Detail, "1/1") {
		t.Fatalf("expected active contract denominator in detail, got %q", result.Detail)
	}
}

func TestFallbackReasonSurfaced(t *testing.T) {
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	claudeDir := filepath.Join(fakeHome, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatal(err)
	}
	settings := map[string]any{
		"hooks": map[string]any{
			"SessionStart": []any{
				map[string]any{
					"hooks": []any{
						map[string]any{"type": "command", "command": "ao inject --apply-decay"},
					},
				},
			},
		},
	}
	data, err := json.Marshal(settings)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(claudeDir, "settings.json"), data, 0644); err != nil {
		t.Fatal(err)
	}

	tmp := t.TempDir()
	hooksDir := filepath.Join(tmp, "hooks")
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hooksDir, "hooks.json"), []byte("{invalid"), 0644); err != nil {
		t.Fatal(err)
	}

	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWD) })

	result := checkHookCoverage()
	if !strings.Contains(result.Detail, "coverage contract fallback:") {
		t.Fatalf("expected fallback reason in detail, got %q", result.Detail)
	}
	if !strings.Contains(result.Detail, "parse hooks manifest") {
		t.Fatalf("expected parse failure reason in detail, got %q", result.Detail)
	}
}

// ---------------------------------------------------------------------------
// checkSkills (0%) — needs a fake HOME
// ---------------------------------------------------------------------------

func TestCheckSkills_NoSkills(t *testing.T) {
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	result := checkSkills()
	if result.Status != "warn" {
		t.Errorf("status=%q, want warn when no skills installed (detail: %s)", result.Status, result.Detail)
	}
}

func TestCheckSkills_WithSkills(t *testing.T) {
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	// Create a fake skill directory with SKILL.md
	skillDir := filepath.Join(fakeHome, ".claude", "skills", "fake-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# Fake Skill"), 0644); err != nil {
		t.Fatal(err)
	}

	result := checkSkills()
	if result.Status != "pass" {
		t.Errorf("status=%q, want pass when skills are found (detail: %s)", result.Status, result.Detail)
	}
	if !strings.Contains(result.Detail, "1 skills found in ~/.claude/skills") {
		t.Errorf("expected install path in detail, got %q", result.Detail)
	}
}

func TestCheckSkills_WithNativeCodexPlugin(t *testing.T) {
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	skillsRoot := filepath.Join(fakeHome, ".codex", "plugins", "cache", "agentops-marketplace", "agentops", "local", "skills-codex")
	skillDir := filepath.Join(skillsRoot, "fake-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# Fake Skill"), 0644); err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(skillsRoot, ".agentops-manifest.json")
	if err := os.WriteFile(manifestPath, []byte(`{"skills":[{"name":"fake-skill"}]}`), 0644); err != nil {
		t.Fatal(err)
	}
	manifestHash, err := sha256File(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(fakeHome, ".codex"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fakeHome, ".codex", ".agentops-codex-install.json"), []byte(fmt.Sprintf(`{"install_mode":"native-plugin","plugin_root":"%s","manifest_hash":"%s","skill_count":1}`, filepath.Join(fakeHome, ".codex", "plugins", "cache", "agentops-marketplace", "agentops", "local"), manifestHash)), 0644); err != nil {
		t.Fatal(err)
	}

	result := checkSkills()
	if result.Status != "pass" {
		t.Errorf("status=%q, want pass when native plugin skills are found (detail: %s)", result.Status, result.Detail)
	}
	if !strings.Contains(result.Detail, "~/.codex/plugins/cache/agentops-marketplace/agentops/local/skills-codex") || !strings.Contains(result.Detail, "native manifest OK") {
		t.Errorf("expected native plugin path in detail, got %q", result.Detail)
	}
}

func TestCheckSkills_NativeCodexPluginMissingManifestWarns(t *testing.T) {
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	skillDir := filepath.Join(fakeHome, ".codex", "plugins", "cache", "agentops-marketplace", "agentops", "local", "skills-codex", "fake-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# Fake Skill"), 0644); err != nil {
		t.Fatal(err)
	}

	result := checkSkills()
	if result.Status != "warn" {
		t.Fatalf("status=%q, want warn when native plugin manifest is missing (detail: %s)", result.Status, result.Detail)
	}
	if !strings.Contains(result.Detail, ".agentops-manifest.json") {
		t.Fatalf("expected manifest warning in detail, got %q", result.Detail)
	}
}

func TestCheckSkills_NativeCodexPluginManifestMismatchWarns(t *testing.T) {
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	skillsRoot := filepath.Join(fakeHome, ".codex", "plugins", "cache", "agentops-marketplace", "agentops", "local", "skills-codex")
	skillDir := filepath.Join(skillsRoot, "fake-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# Fake Skill"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillsRoot, ".agentops-manifest.json"), []byte(`{"skills":[{"name":"fake-skill"}]}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(fakeHome, ".codex"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fakeHome, ".codex", ".agentops-codex-install.json"), []byte(fmt.Sprintf(`{"install_mode":"native-plugin","plugin_root":"%s","manifest_hash":"deadbeef","skill_count":1}`, filepath.Join(fakeHome, ".codex", "plugins", "cache", "agentops-marketplace", "agentops", "local"))), 0644); err != nil {
		t.Fatal(err)
	}

	result := checkSkills()
	if result.Status != "warn" {
		t.Fatalf("status=%q, want warn when native plugin manifest mismatches (detail: %s)", result.Status, result.Detail)
	}
	if !strings.Contains(result.Detail, "manifest hash does not match") {
		t.Fatalf("expected manifest mismatch warning, got %q", result.Detail)
	}
}

func TestCheckSkills_AltPath(t *testing.T) {
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	// Create skills under .agents/skills/ (alt path)
	skillDir := filepath.Join(fakeHome, ".agents", "skills", "alt-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# Alt Skill"), 0644); err != nil {
		t.Fatal(err)
	}

	result := checkSkills()
	if result.Status != "pass" {
		t.Errorf("status=%q, want pass for alt-path skills (detail: %s)", result.Status, result.Detail)
	}
}

func TestCheckSkills_UserSkillsOverlapWarnsWithoutPluginCache(t *testing.T) {
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	for _, dir := range []string{
		filepath.Join(fakeHome, ".codex", "skills", "research"),
		filepath.Join(fakeHome, ".codex", "skills", "vibe"),
		filepath.Join(fakeHome, ".agents", "skills", "research"),
	} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("# Skill"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	result := checkSkills()
	if result.Status != "warn" {
		t.Fatalf("status=%q, want warn for duplicate raw installs (detail: %s)", result.Status, result.Detail)
	}
	if !strings.Contains(result.Detail, "duplicate raw skill install") {
		t.Fatalf("expected duplicate raw install warning, got %q", result.Detail)
	}
	if !strings.Contains(result.Detail, "~/.agents/skills") {
		t.Fatalf("expected legacy path in detail, got %q", result.Detail)
	}
	if !strings.Contains(result.Detail, "research") {
		t.Fatalf("expected overlapping skill sample in detail, got %q", result.Detail)
	}
}

func TestCheckSkills_PluginCacheAndUserSkillsWarn(t *testing.T) {
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	for _, dir := range []string{
		filepath.Join(fakeHome, ".codex", "plugins", "cache", "agentops-marketplace", "agentops", "local", "skills-codex", "research"),
		filepath.Join(fakeHome, ".codex", "plugins", "cache", "agentops-marketplace", "agentops", "local", "skills-codex", "vibe"),
		filepath.Join(fakeHome, ".agents", "skills", "research"),
		filepath.Join(fakeHome, ".agents", "skills", "vibe"),
	} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("# Skill"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	pluginRoot := filepath.Join(fakeHome, ".codex", "plugins", "cache", "agentops-marketplace", "agentops", "local")
	manifestPath := filepath.Join(pluginRoot, "skills-codex", ".agentops-manifest.json")
	if err := os.WriteFile(manifestPath, []byte(`{"skills":[{"name":"research"},{"name":"vibe"}]}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pluginRoot, ".agentops-codex-state.json"), []byte(`{"manifest_hash":"abc123","skill_count":2}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(fakeHome, ".codex"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fakeHome, ".codex", ".agentops-codex-install.json"), []byte(`{"install_mode":"native-plugin","plugin_root":"`+pluginRoot+`","skill_count":2}`), 0644); err != nil {
		t.Fatal(err)
	}

	result := checkSkills()
	if result.Status != "warn" {
		t.Fatalf("status=%q, want warn when plugin cache and ~/.agents/skills overlap (detail: %s)", result.Status, result.Detail)
	}
	if !strings.Contains(result.Detail, "duplicate raw skill install") || !strings.Contains(result.Detail, "~/.agents/skills") {
		t.Fatalf("expected duplicate ~/.agents/skills warning, got %q", result.Detail)
	}
}

func TestCheckSkills_RawCodexOverlapWarns(t *testing.T) {
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	for _, dir := range []string{
		filepath.Join(fakeHome, ".codex", "plugins", "cache", "agentops-marketplace", "agentops", "local", "skills-codex", "research"),
		filepath.Join(fakeHome, ".codex", "skills", "research"),
	} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("# Skill"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	result := checkSkills()
	if result.Status != "warn" {
		t.Fatalf("status=%q, want warn for duplicate raw Codex installs (detail: %s)", result.Status, result.Detail)
	}
	if !strings.Contains(result.Detail, "duplicate raw Codex install") {
		t.Fatalf("expected duplicate raw Codex warning, got %q", result.Detail)
	}
}

func TestCheckCodexSync_PassWhenRepoMatchesInstall(t *testing.T) {
	repo := chdirTemp(t)
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	if err := exec.Command("git", "-C", repo, "init").Run(); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("git", "-C", repo, "config", "user.email", "test@example.com").Run(); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("git", "-C", repo, "config", "user.name", "Test").Run(); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(repo, "skills-codex"), 0755); err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(repo, "skills-codex", ".agentops-manifest.json")
	if err := os.WriteFile(manifestPath, []byte(`{"skills":[{"name":"research"}]}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("git", "-C", repo, "add", ".").Run(); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("git", "-C", repo, "commit", "-m", "fixture").Run(); err != nil {
		t.Fatal(err)
	}
	manifestHash, err := sha256File(manifestPath)
	if err != nil {
		t.Fatal(err)
	}

	versionOut, err := exec.Command("git", "-C", repo, "rev-parse", "--short", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	version := strings.TrimSpace(string(versionOut))

	metaDir := filepath.Join(fakeHome, ".codex")
	if err := os.MkdirAll(metaDir, 0755); err != nil {
		t.Fatal(err)
	}
	meta := fmt.Sprintf(`{"install_mode":"native-plugin","version":"%s","manifest_hash":"%s"}`, version, manifestHash)
	if err := os.WriteFile(filepath.Join(metaDir, ".agentops-codex-install.json"), []byte(meta), 0644); err != nil {
		t.Fatal(err)
	}

	result := checkCodexSync()
	if result.Status != "pass" {
		t.Fatalf("status=%q, want pass (detail: %s)", result.Status, result.Detail)
	}
	if !strings.Contains(result.Detail, "matches repo") {
		t.Fatalf("expected match detail, got %q", result.Detail)
	}
}

func TestCheckCodexSync_WarnsOnManifestDriftAtSameVersion(t *testing.T) {
	repo := chdirTemp(t)
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	if err := exec.Command("git", "-C", repo, "init").Run(); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("git", "-C", repo, "config", "user.email", "test@example.com").Run(); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("git", "-C", repo, "config", "user.name", "Test").Run(); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(repo, "skills-codex"), 0755); err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(repo, "skills-codex", ".agentops-manifest.json")
	if err := os.WriteFile(manifestPath, []byte(`{"skills":[{"name":"research"}]}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("git", "-C", repo, "add", ".").Run(); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("git", "-C", repo, "commit", "-m", "fixture").Run(); err != nil {
		t.Fatal(err)
	}

	versionOut, err := exec.Command("git", "-C", repo, "rev-parse", "--short", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	version := strings.TrimSpace(string(versionOut))

	metaDir := filepath.Join(fakeHome, ".codex")
	if err := os.MkdirAll(metaDir, 0755); err != nil {
		t.Fatal(err)
	}
	meta := fmt.Sprintf(`{"install_mode":"native-plugin","version":"%s","manifest_hash":"stale-hash"}`, version)
	if err := os.WriteFile(filepath.Join(metaDir, ".agentops-codex-install.json"), []byte(meta), 0644); err != nil {
		t.Fatal(err)
	}

	result := checkCodexSync()
	if result.Status != "warn" {
		t.Fatalf("status=%q, want warn (detail: %s)", result.Status, result.Detail)
	}
	if !strings.Contains(result.Detail, "manifest differs from repo") {
		t.Fatalf("expected manifest drift detail, got %q", result.Detail)
	}
	if strings.Contains(result.Detail, " -> ") {
		t.Fatalf("expected non-version drift detail, got %q", result.Detail)
	}
}

func TestCheckCodexSync_WarnsWhenRepoIsNewer(t *testing.T) {
	repo := chdirTemp(t)
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	if err := exec.Command("git", "-C", repo, "init").Run(); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("git", "-C", repo, "config", "user.email", "test@example.com").Run(); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("git", "-C", repo, "config", "user.name", "Test").Run(); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(repo, "skills-codex"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "skills-codex", ".agentops-manifest.json"), []byte(`{"skills":[{"name":"research"}]}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("git", "-C", repo, "add", ".").Run(); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("git", "-C", repo, "commit", "-m", "fixture").Run(); err != nil {
		t.Fatal(err)
	}

	metaDir := filepath.Join(fakeHome, ".codex")
	if err := os.MkdirAll(metaDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(metaDir, ".agentops-codex-install.json"), []byte(`{"install_mode":"native-plugin","version":"oldsha","manifest_hash":"deadbeef"}`), 0644); err != nil {
		t.Fatal(err)
	}

	result := checkCodexSync()
	if result.Status != "warn" {
		t.Fatalf("status=%q, want warn (detail: %s)", result.Status, result.Detail)
	}
	if !strings.Contains(result.Detail, "refresh-codex-local.sh") {
		t.Fatalf("expected repair command in detail, got %q", result.Detail)
	}
}

// ---------------------------------------------------------------------------
// runDoctor (0%) — exercise via cobra command
// ---------------------------------------------------------------------------

func TestRunDoctor_TableOutput(t *testing.T) {
	tmp := chdirTemp(t)
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	// Set up knowledge base so at least one check passes
	if err := os.MkdirAll(filepath.Join(tmp, ".agents", "ao"), 0755); err != nil {
		t.Fatal(err)
	}

	// Save and restore the global doctorJSON flag
	origJSON := doctorJSON
	doctorJSON = false
	t.Cleanup(func() { doctorJSON = origJSON })

	var buf bytes.Buffer
	doctorCmd.SetOut(&buf)
	doctorCmd.SetErr(&buf)
	t.Cleanup(func() { doctorCmd.SetOut(nil); doctorCmd.SetErr(nil) })

	// runDoctor may return error (required check failure) — we just ensure it runs
	_ = runDoctor(doctorCmd, nil)

	rendered := buf.String()
	if !strings.Contains(rendered, "ao doctor") {
		t.Error("expected 'ao doctor' header in table output")
	}
}

func TestRunDoctor_JSONOutput(t *testing.T) {
	tmp := chdirTemp(t)
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	if err := os.MkdirAll(filepath.Join(tmp, ".agents", "ao"), 0755); err != nil {
		t.Fatal(err)
	}

	origJSON := doctorJSON
	doctorJSON = true
	t.Cleanup(func() { doctorJSON = origJSON })

	var buf bytes.Buffer
	doctorCmd.SetOut(&buf)
	doctorCmd.SetErr(&buf)
	t.Cleanup(func() { doctorCmd.SetOut(nil); doctorCmd.SetErr(nil) })

	_ = runDoctor(doctorCmd, nil)

	// Should be valid JSON
	var result doctorOutput
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Errorf("expected valid JSON output, got error: %v\nOutput: %s", err, buf.String())
	}
	if len(result.Checks) == 0 {
		t.Error("expected at least one check in JSON output")
	}
	if result.Result == "" {
		t.Error("expected non-empty result field")
	}
}

// ---------------------------------------------------------------------------
// newestFileModTime — branch for no regular files
// ---------------------------------------------------------------------------

func TestNewestFileModTime_OnlyDirs(t *testing.T) {
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, "subdir"), 0755); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(tmp)
	if err != nil {
		t.Fatal(err)
	}
	newest := newestFileModTime(entries)
	if !newest.IsZero() {
		t.Error("expected zero time for directory-only entries")
	}
}

// ---------------------------------------------------------------------------
// countEstablished
// ---------------------------------------------------------------------------

func TestCountEstablished(t *testing.T) {
	tmp := t.TempDir()
	// Create some files with various names
	for _, name := range []string{
		"learning-established-001.md",
		"learning-promoted-002.md",
		"learning-provisional-003.md",
		"unrelated.jsonl",
	} {
		if err := os.WriteFile(filepath.Join(tmp, name), []byte("test"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	got := countEstablished(tmp)
	if got != 2 {
		t.Errorf("countEstablished() = %d, want 2", got)
	}
}

func TestCountEstablished_NonexistentDir(t *testing.T) {
	got := countEstablished("/nonexistent/path/xyz")
	if got != 0 {
		t.Errorf("countEstablished(nonexistent) = %d, want 0", got)
	}
}

// ---------------------------------------------------------------------------
// findHealScript — exercise all branches
// ---------------------------------------------------------------------------

func TestFindHealScript_NotFound(t *testing.T) {
	// Use a temp dir so in-repo heal.sh isn't found
	chdirTemp(t)
	t.Setenv("HOME", t.TempDir())

	path := findHealScript()
	if path != "" {
		t.Errorf("expected empty path, got %q", path)
	}
}

func TestFindHealScript_InClaude(t *testing.T) {
	chdirTemp(t)
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	// Create heal.sh in ~/.claude/skills/heal-skill/scripts/
	healDir := filepath.Join(fakeHome, ".claude", "skills", "heal-skill", "scripts")
	if err := os.MkdirAll(healDir, 0755); err != nil {
		t.Fatal(err)
	}
	healPath := filepath.Join(healDir, "heal.sh")
	if err := os.WriteFile(healPath, []byte("#!/bin/bash\nexit 0\n"), 0755); err != nil {
		t.Fatal(err)
	}

	got := findHealScript()
	if got != healPath {
		t.Errorf("findHealScript() = %q, want %q", got, healPath)
	}
}

func TestFindHealScript_InCodexPluginCache(t *testing.T) {
	chdirTemp(t)
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	healDir := filepath.Join(fakeHome, ".codex", "plugins", "cache", "agentops-marketplace", "agentops", "local", "skills-codex", "heal-skill", "scripts")
	if err := os.MkdirAll(healDir, 0755); err != nil {
		t.Fatal(err)
	}
	healPath := filepath.Join(healDir, "heal.sh")
	if err := os.WriteFile(healPath, []byte("#!/bin/bash\nexit 0\n"), 0755); err != nil {
		t.Fatal(err)
	}

	got := findHealScript()
	if got != healPath {
		t.Errorf("findHealScript() = %q, want %q", got, healPath)
	}
}

func TestFindHealScript_InAgents(t *testing.T) {
	chdirTemp(t)
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	// Create heal.sh in ~/.agents/skills/heal-skill/scripts/
	healDir := filepath.Join(fakeHome, ".agents", "skills", "heal-skill", "scripts")
	if err := os.MkdirAll(healDir, 0755); err != nil {
		t.Fatal(err)
	}
	healPath := filepath.Join(healDir, "heal.sh")
	if err := os.WriteFile(healPath, []byte("#!/bin/bash\nexit 0\n"), 0755); err != nil {
		t.Fatal(err)
	}

	got := findHealScript()
	if got != healPath {
		t.Errorf("findHealScript() = %q, want %q", got, healPath)
	}
}

// ---------------------------------------------------------------------------
// gatherDoctorChecks (0%) — smoke test that it returns checks
// ---------------------------------------------------------------------------

func TestGatherDoctorChecks(t *testing.T) {
	tmp := chdirTemp(t)
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	// Set up minimal environment
	if err := os.MkdirAll(filepath.Join(tmp, ".agents", "ao"), 0755); err != nil {
		t.Fatal(err)
	}

	checks := gatherDoctorChecks()
	if len(checks) == 0 {
		t.Error("expected at least one check from gatherDoctorChecks()")
	}

	// First check should be ao CLI
	if checks[0].Name != "ao CLI" {
		t.Errorf("first check name = %q, want 'ao CLI'", checks[0].Name)
	}
	if checks[0].Status != "pass" {
		t.Errorf("ao CLI check status = %q, want 'pass'", checks[0].Status)
	}
}

// ---------------------------------------------------------------------------
// extractHooksMap — test with invalid JSON
// ---------------------------------------------------------------------------

func TestExtractHooksMap_InvalidJSON(t *testing.T) {
	_, ok := extractHooksMap([]byte("not valid json"))
	if ok {
		t.Error("expected false for invalid JSON")
	}
}

func TestExtractHooksMap_NoHooksField(t *testing.T) {
	data := []byte(`{"foo": "bar"}`)
	_, ok := extractHooksMap(data)
	if ok {
		t.Error("expected false when no hooks field and no events found")
	}
}

// ---------------------------------------------------------------------------
// countHooksInMap — test with []any (array branch)
// ---------------------------------------------------------------------------

func TestCountHooksInMap_Array(t *testing.T) {
	got := countHooksInMap([]any{"a", "b", "c"})
	if got != 3 {
		t.Errorf("countHooksInMap([]any) = %d, want 3", got)
	}
}

// ---------------------------------------------------------------------------
// checkStaleReferences — stale command reference detector
// ---------------------------------------------------------------------------

func TestCheckStaleReferences_NoFiles(t *testing.T) {
	chdirTemp(t)
	result := checkStaleReferences()
	if result.Status != "pass" {
		t.Errorf("status=%q, want pass when no hooks/skills exist (detail: %s)", result.Status, result.Detail)
	}
	if result.Required {
		t.Error("Stale References should not be required")
	}
}

func TestCheckStaleReferences_CleanFiles(t *testing.T) {
	tmp := chdirTemp(t)

	// Create a hooks file with only current flat-style commands
	hooksDir := filepath.Join(tmp, "hooks")
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hooksDir, "dispatch.sh"), []byte("#!/bin/bash\nao forge transcript\nao rpi start\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a skill file with only current flat-style commands
	skillDir := filepath.Join(tmp, "skills", "test-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# Test Skill\nRun `ao inject` to load context.\n"), 0644); err != nil {
		t.Fatal(err)
	}

	result := checkStaleReferences()
	if result.Status != "pass" {
		t.Errorf("status=%q, want pass for clean files (detail: %s)", result.Status, result.Detail)
	}
}

func TestCheckStaleReferences_StaleInHooks(t *testing.T) {
	tmp := chdirTemp(t)

	hooksDir := filepath.Join(tmp, "hooks")
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		t.Fatal(err)
	}
	// Use old namespace-style "ao know forge" instead of flat "ao forge"
	if err := os.WriteFile(filepath.Join(hooksDir, "dispatch.sh"), []byte("#!/bin/bash\nao know forge transcript\n"), 0644); err != nil {
		t.Fatal(err)
	}

	result := checkStaleReferences()
	if result.Status != "warn" {
		t.Errorf("status=%q, want warn for stale hooks reference (detail: %s)", result.Status, result.Detail)
	}
	if !strings.Contains(result.Detail, "stale reference") {
		t.Errorf("expected 'stale reference' in detail, got %q", result.Detail)
	}
}

func TestCheckStaleReferences_StaleInSkills(t *testing.T) {
	tmp := chdirTemp(t)

	skillDir := filepath.Join(tmp, "skills", "test-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatal(err)
	}
	// Use old namespace-style "ao know inject" instead of flat "ao inject"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# Test\nRun `ao know inject` to load.\n"), 0644); err != nil {
		t.Fatal(err)
	}

	result := checkStaleReferences()
	if result.Status != "warn" {
		t.Errorf("status=%q, want warn for stale skill reference (detail: %s)", result.Status, result.Detail)
	}
}

func TestCheckStaleReferences_StaleInDocs(t *testing.T) {
	tmp := chdirTemp(t)

	docsDir := filepath.Join(tmp, "docs")
	if err := os.MkdirAll(docsDir, 0755); err != nil {
		t.Fatal(err)
	}
	// Use old namespace-style "ao work rpi" instead of flat "ao rpi"
	if err := os.WriteFile(filepath.Join(docsDir, "guide.md"), []byte("# Guide\nRun `ao work rpi status` to check.\n"), 0644); err != nil {
		t.Fatal(err)
	}

	result := checkStaleReferences()
	if result.Status != "warn" {
		t.Errorf("status=%q, want warn for stale docs reference (detail: %s)", result.Status, result.Detail)
	}
}

func TestCheckStaleReferences_StaleInScripts(t *testing.T) {
	tmp := chdirTemp(t)

	scriptsDir := filepath.Join(tmp, "scripts")
	if err := os.MkdirAll(scriptsDir, 0755); err != nil {
		t.Fatal(err)
	}
	// Use old namespace-style "ao quality pool list" instead of flat "ao pool list"
	if err := os.WriteFile(filepath.Join(scriptsDir, "smoke.sh"), []byte("#!/bin/bash\nao quality pool list\n"), 0644); err != nil {
		t.Fatal(err)
	}

	result := checkStaleReferences()
	if result.Status != "warn" {
		t.Errorf("status=%q, want warn for stale scripts reference (detail: %s)", result.Status, result.Detail)
	}
}

func TestCheckStaleReferences_SubdirsScanned(t *testing.T) {
	tmp := chdirTemp(t)

	// Create subdirectories that the expanded scan should cover
	contractsDir := filepath.Join(tmp, "docs", "contracts")
	if err := os.MkdirAll(contractsDir, 0755); err != nil {
		t.Fatal(err)
	}
	examplesDir := filepath.Join(tmp, "hooks", "examples")
	if err := os.MkdirAll(examplesDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Write a stale reference in docs/contracts/
	if err := os.WriteFile(
		filepath.Join(contractsDir, "test-contract.md"),
		[]byte("# Contract\nRun `ao know search` to find references.\n"),
		0644,
	); err != nil {
		t.Fatal(err)
	}

	// Write a stale reference in hooks/examples/
	if err := os.WriteFile(
		filepath.Join(examplesDir, "example.sh"),
		[]byte("#!/bin/bash\nao work rpi status\n"),
		0644,
	); err != nil {
		t.Fatal(err)
	}

	result := checkStaleReferences()
	if result.Status != "warn" {
		t.Errorf("status=%q, want warn for stale refs in subdirs (detail: %s)", result.Status, result.Detail)
	}
	if !strings.Contains(result.Detail, "stale reference") {
		t.Errorf("expected 'stale reference' in detail, got %q", result.Detail)
	}
}

func TestCheckStaleReferences_CodexAndReferencesScanned(t *testing.T) {
	tmp := chdirTemp(t)

	codexDir := filepath.Join(tmp, "skills-codex", "fixture")
	if err := os.MkdirAll(codexDir, 0755); err != nil {
		t.Fatal(err)
	}
	refDir := filepath.Join(tmp, "skills", "fixture", "references")
	if err := os.MkdirAll(refDir, 0755); err != nil {
		t.Fatal(err)
	}

	// skills-codex/ copies must stay on canonical surfaces
	if err := os.WriteFile(
		filepath.Join(codexDir, "SKILL.md"),
		[]byte("---\nname: fixture\n---\nRun `ao settings config` to review.\n"),
		0644,
	); err != nil {
		t.Fatal(err)
	}
	// skills/*/references/*.md is a first-class skill surface
	if err := os.WriteFile(
		filepath.Join(refDir, "overview.md"),
		[]byte("See `ao start seed` for bootstrap.\n"),
		0644,
	); err != nil {
		t.Fatal(err)
	}

	result := checkStaleReferences()
	if result.Status != "warn" {
		t.Errorf("status=%q, want warn for stale refs in skills-codex/ and references/ (detail: %s)", result.Status, result.Detail)
	}
	if !strings.Contains(result.Detail, "2 stale") {
		t.Errorf("expected both subdir hits counted, got %q", result.Detail)
	}
}

func TestCheckStaleReferences_NoFalsePositiveOnFlat(t *testing.T) {
	tmp := chdirTemp(t)

	// "ao forge" (flat, canonical) should NOT trigger a stale reference
	hooksDir := filepath.Join(tmp, "hooks")
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hooksDir, "test.sh"), []byte("ao forge transcript\n"), 0644); err != nil {
		t.Fatal(err)
	}

	result := checkStaleReferences()
	if result.Status != "pass" {
		t.Errorf("status=%q, want pass (should not trigger on flat 'ao forge') (detail: %s)", result.Status, result.Detail)
	}
}

func TestScanFileForDeprecatedCommands(t *testing.T) {
	tmp := t.TempDir()

	t.Run("nonexistent file returns nil", func(t *testing.T) {
		refs := scanFileForDeprecatedCommands(filepath.Join(tmp, "nope.sh"))
		if len(refs) != 0 {
			t.Errorf("expected 0 refs for nonexistent file, got %d", len(refs))
		}
	})

	t.Run("file with multiple deprecated commands", func(t *testing.T) {
		f := filepath.Join(tmp, "multi.sh")
		content := "ao know forge transcript\nao know inject --apply-decay\nao search query\n"
		if err := os.WriteFile(f, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
		refs := scanFileForDeprecatedCommands(f)
		if len(refs) < 2 {
			t.Errorf("expected at least 2 stale refs, got %d", len(refs))
		}
	})
}

func TestCountUniqueFiles(t *testing.T) {
	refs := []staleReference{
		{File: "a.sh", OldCommand: "ao know forge", NewCommand: "ao forge"},
		{File: "a.sh", OldCommand: "ao know inject", NewCommand: "ao inject"},
		{File: "b.md", OldCommand: "ao know forge", NewCommand: "ao forge"},
	}
	got := countUniqueFiles(refs)
	if got != 2 {
		t.Errorf("countUniqueFiles() = %d, want 2", got)
	}
}
