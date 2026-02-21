package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/embedded"
)

func TestGenerateMinimalHooksConfig(t *testing.T) {
	hooks := generateMinimalHooksConfig()

	if len(hooks.SessionStart) == 0 {
		t.Error("expected SessionStart hooks, got none")
	}
	if len(hooks.Stop) == 0 {
		t.Error("expected Stop hooks, got none")
	}

	// Verify SessionStart contains ao inject
	found := false
	for _, g := range hooks.SessionStart {
		for _, h := range g.Hooks {
			if h.Type == "command" && h.Command == "ao inject --apply-decay --max-tokens 1500 2>/dev/null || true" {
				found = true
			}
		}
	}
	if !found {
		t.Error("expected ao inject command in SessionStart hooks")
	}

	// Verify Stop contains ao forge
	found = false
	for _, g := range hooks.Stop {
		for _, h := range g.Hooks {
			if h.Type == "command" && h.Command == "ao forge transcript --last-session --quiet --queue 2>/dev/null; ao task-sync --promote 2>/dev/null || true" {
				found = true
			}
		}
	}
	if !found {
		t.Error("expected ao forge command in Stop hooks")
	}
}

func TestAllEventNames(t *testing.T) {
	events := AllEventNames()
	if len(events) != 12 {
		t.Fatalf("expected 12 events, got %d", len(events))
	}
	expected := []string{
		"SessionStart", "SessionEnd",
		"PreToolUse", "PostToolUse",
		"UserPromptSubmit", "TaskCompleted",
		"Stop", "PreCompact",
		"SubagentStop", "WorktreeCreate",
		"WorktreeRemove", "ConfigChange",
	}
	for i, e := range expected {
		if events[i] != e {
			t.Errorf("event %d: expected %s, got %s", i, e, events[i])
		}
	}
}

func TestHooksConfigGetSetEventGroups(t *testing.T) {
	config := &HooksConfig{}
	groups := []HookGroup{
		{Hooks: []HookEntry{{Type: "command", Command: "test"}}},
	}

	for _, event := range AllEventNames() {
		config.SetEventGroups(event, groups)
		got := config.GetEventGroups(event)
		if len(got) != 1 {
			t.Errorf("event %s: expected 1 group after set, got %d", event, len(got))
		}
	}

	// Unknown event returns nil
	if got := config.GetEventGroups("Unknown"); got != nil {
		t.Error("expected nil for unknown event")
	}
}

func TestHookGroupToMapStringMatcher(t *testing.T) {
	g := HookGroup{
		Matcher: "Write|Edit",
		Hooks: []HookEntry{
			{Type: "command", Command: "echo hello"},
		},
	}

	m := hookGroupToMap(g)

	// Matcher should be a string
	matcher, ok := m["matcher"].(string)
	if !ok {
		t.Fatal("expected matcher to be a string")
	}
	if matcher != "Write|Edit" {
		t.Errorf("expected matcher 'Write|Edit', got '%s'", matcher)
	}

	hooks, ok := m["hooks"].([]map[string]interface{})
	if !ok {
		t.Fatal("expected hooks array in map")
	}
	if len(hooks) != 1 {
		t.Fatalf("expected 1 hook, got %d", len(hooks))
	}
}

func TestHookGroupToMapEmptyMatcher(t *testing.T) {
	g := HookGroup{
		Matcher: "",
		Hooks: []HookEntry{
			{Type: "command", Command: "echo hello"},
		},
	}

	m := hookGroupToMap(g)
	if _, exists := m["matcher"]; exists {
		t.Error("expected no matcher key when Matcher is empty string")
	}
}

func TestHookGroupToMapTimeout(t *testing.T) {
	g := HookGroup{
		Hooks: []HookEntry{
			{Type: "command", Command: "test", Timeout: 120},
		},
	}

	m := hookGroupToMap(g)
	hooks := m["hooks"].([]map[string]interface{})
	if hooks[0]["timeout"] != 120 {
		t.Errorf("expected timeout 120, got %v", hooks[0]["timeout"])
	}

	// Zero timeout should be omitted
	g2 := HookGroup{
		Hooks: []HookEntry{
			{Type: "command", Command: "test", Timeout: 0},
		},
	}
	m2 := hookGroupToMap(g2)
	hooks2 := m2["hooks"].([]map[string]interface{})
	if _, exists := hooks2[0]["timeout"]; exists {
		t.Error("expected no timeout key when Timeout is 0")
	}
}

func TestReadHooksManifest(t *testing.T) {
	manifest := `{
		"$schema": "test",
		"hooks": {
			"SessionStart": [{"hooks": [{"type": "command", "command": "test-start"}]}],
			"SessionEnd": [{"hooks": [{"type": "command", "command": "test-end"}]}],
			"PreToolUse": [{"matcher": "Write|Edit", "hooks": [{"type": "command", "command": "test-pre", "timeout": 2}]}],
			"PostToolUse": [{"matcher": "Bash", "hooks": [{"type": "command", "command": "test-post"}]}],
			"UserPromptSubmit": [{"hooks": [{"type": "command", "command": "test-prompt"}]}],
			"TaskCompleted": [{"hooks": [{"type": "command", "command": "test-task", "timeout": 120}]}],
			"Stop": [{"hooks": [{"type": "command", "command": "test-stop"}]}],
			"PreCompact": [{"hooks": [{"type": "command", "command": "test-compact"}]}],
			"SubagentStop": [{"hooks": [{"type": "command", "command": "test-subagent-stop"}]}],
			"WorktreeCreate": [{"hooks": [{"type": "command", "command": "test-worktree-create"}]}],
			"WorktreeRemove": [{"hooks": [{"type": "command", "command": "test-worktree-remove"}]}],
			"ConfigChange": [{"hooks": [{"type": "command", "command": "test-config-change"}]}]
		}
	}`

	config, err := ReadHooksManifest([]byte(manifest))
	if err != nil {
		t.Fatalf("ReadHooksManifest failed: %v", err)
	}

	// Verify all 12 events parsed
	for _, event := range AllEventNames() {
		groups := config.GetEventGroups(event)
		if len(groups) == 0 {
			t.Errorf("event %s: expected at least 1 group, got 0", event)
		}
	}

	// Verify PreToolUse has string matcher
	if len(config.PreToolUse) > 0 && config.PreToolUse[0].Matcher != "Write|Edit" {
		t.Errorf("PreToolUse matcher: expected 'Write|Edit', got '%s'", config.PreToolUse[0].Matcher)
	}

	// Verify timeout preserved
	if len(config.TaskCompleted) > 0 && len(config.TaskCompleted[0].Hooks) > 0 {
		if config.TaskCompleted[0].Hooks[0].Timeout != 120 {
			t.Errorf("TaskCompleted timeout: expected 120, got %d", config.TaskCompleted[0].Hooks[0].Timeout)
		}
	}

	// Verify PreToolUse hook timeout
	if len(config.PreToolUse) > 0 && len(config.PreToolUse[0].Hooks) > 0 {
		if config.PreToolUse[0].Hooks[0].Timeout != 2 {
			t.Errorf("PreToolUse timeout: expected 2, got %d", config.PreToolUse[0].Hooks[0].Timeout)
		}
	}
}

func TestReadHooksManifestInvalid(t *testing.T) {
	// Missing hooks key
	_, err := ReadHooksManifest([]byte(`{"other": "data"}`))
	if err == nil {
		t.Error("expected error for missing hooks key")
	}

	// Invalid JSON
	_, err = ReadHooksManifest([]byte(`not json`))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestReplacePluginRoot(t *testing.T) {
	config := &HooksConfig{
		PreToolUse: []HookGroup{
			{
				Matcher: "Write|Edit",
				Hooks: []HookEntry{
					{Type: "command", Command: "${CLAUDE_PLUGIN_ROOT}/hooks/standards-injector.sh"},
				},
			},
		},
		Stop: []HookGroup{
			{
				Hooks: []HookEntry{
					{Type: "command", Command: "${CLAUDE_PLUGIN_ROOT}/hooks/stop-team-guard.sh"},
				},
			},
		},
	}

	replacePluginRoot(config, "/home/user/.agentops")

	if config.PreToolUse[0].Hooks[0].Command != "/home/user/.agentops/hooks/standards-injector.sh" {
		t.Errorf("PreToolUse command not rewritten: %s", config.PreToolUse[0].Hooks[0].Command)
	}
	if config.Stop[0].Hooks[0].Command != "/home/user/.agentops/hooks/stop-team-guard.sh" {
		t.Errorf("Stop command not rewritten: %s", config.Stop[0].Hooks[0].Command)
	}
}

func TestFilterNonAoHookGroupsAllEvents(t *testing.T) {
	// Build a hooksMap with ao and non-ao groups for every event
	hooksMap := make(map[string]interface{})
	for _, event := range AllEventNames() {
		hooksMap[event] = []interface{}{
			map[string]interface{}{
				"hooks": []interface{}{
					map[string]interface{}{"type": "command", "command": "ao inject 2>/dev/null"},
				},
			},
			map[string]interface{}{
				"hooks": []interface{}{
					map[string]interface{}{"type": "command", "command": "my-custom-hook"},
				},
			},
		}
	}

	for _, event := range AllEventNames() {
		filtered := filterNonAoHookGroups(hooksMap, event)
		if len(filtered) != 1 {
			t.Errorf("event %s: expected 1 non-ao group, got %d", event, len(filtered))
		}
		if hooks, ok := filtered[0]["hooks"].([]interface{}); ok {
			if hook, ok := hooks[0].(map[string]interface{}); ok {
				if hook["command"] != "my-custom-hook" {
					t.Errorf("event %s: expected non-ao hook preserved, got %v", event, hook["command"])
				}
			}
		}
	}
}

func TestHookGroupContainsAoAllEvents(t *testing.T) {
	hooksMap := make(map[string]interface{})
	for _, event := range AllEventNames() {
		hooksMap[event] = []interface{}{
			map[string]interface{}{
				"hooks": []interface{}{
					map[string]interface{}{"type": "command", "command": "ao inject stuff"},
				},
			},
		}
	}

	for _, event := range AllEventNames() {
		if !hookGroupContainsAo(hooksMap, event) {
			t.Errorf("event %s: expected ao hook detected", event)
		}
	}
}

func TestHookGroupContainsAoForInstalledScriptPaths(t *testing.T) {
	hooksMap := map[string]interface{}{
		"SessionStart": []interface{}{
			map[string]interface{}{
				"hooks": []interface{}{
					map[string]interface{}{"type": "command", "command": "/Users/test/.agentops/hooks/session-start.sh"},
				},
			},
		},
	}
	if !hookGroupContainsAo(hooksMap, "SessionStart") {
		t.Fatal("expected .agentops hook script path to be treated as ao-managed")
	}

	filtered := filterNonAoHookGroups(hooksMap, "SessionStart")
	if len(filtered) != 0 {
		t.Fatalf("expected ao-managed script group to be filtered out, got %d group(s)", len(filtered))
	}
}

func TestBackwardsCompatDefaultInstall(t *testing.T) {
	// generateMinimalHooksConfig should ALWAYS return SessionStart + Stop
	hooks := generateMinimalHooksConfig()
	if len(hooks.SessionStart) == 0 {
		t.Error("minimal config missing SessionStart")
	}
	if len(hooks.Stop) == 0 {
		t.Error("minimal config missing Stop")
	}
	// Should NOT have other events
	if len(hooks.PreToolUse) > 0 {
		t.Error("minimal config should not have PreToolUse")
	}
	if len(hooks.TaskCompleted) > 0 {
		t.Error("minimal config should not have TaskCompleted")
	}
}

func TestReadEmbeddedHooks(t *testing.T) {
	// Verify embedded hooks.json is present and parseable
	if len(embedded.HooksJSON) == 0 {
		t.Fatal("embedded.HooksJSON is empty")
	}

	config, err := ReadHooksManifest(embedded.HooksJSON)
	if err != nil {
		t.Fatalf("failed to parse embedded hooks.json: %v", err)
	}

	// Verify all 12 events have at least one hook group
	for _, event := range AllEventNames() {
		groups := config.GetEventGroups(event)
		if len(groups) == 0 {
			t.Errorf("embedded hooks.json: event %s has no hook groups", event)
		}
	}
}

func TestGenerateFullHooksConfig(t *testing.T) {
	// generateFullHooksConfig should succeed (embedded fallback guarantees it)
	config, err := generateFullHooksConfig()
	if err != nil {
		t.Fatalf("generateFullHooksConfig failed: %v", err)
	}

	// Should have all 12 events populated
	for _, event := range AllEventNames() {
		groups := config.GetEventGroups(event)
		if len(groups) == 0 {
			t.Errorf("full config: event %s has no hook groups", event)
		}
	}
}

func TestEmbeddedAoCommandsHaveGuardrails(t *testing.T) {
	config, err := ReadHooksManifest(embedded.HooksJSON)
	if err != nil {
		t.Fatalf("failed to parse embedded hooks: %v", err)
	}

	foundBatchFeedback := false
	foundSessionEndMaintenance := false
	for _, event := range AllEventNames() {
		for _, group := range config.GetEventGroups(event) {
			for _, hook := range group.Hooks {
				if hook.Type != "command" {
					continue
				}

				cmd := strings.TrimSpace(hook.Command)
				if strings.Contains(cmd, "session-end-maintenance.sh") {
					foundSessionEndMaintenance = true
					if hook.Timeout <= 0 {
						t.Errorf("%s session-end-maintenance hook missing timeout: %q", event, hook.Command)
					}
				}
				isAOCommand := strings.HasPrefix(cmd, "ao ") || strings.Contains(cmd, "command -v ao") || strings.Contains(cmd, "; ao ")
				if !isAOCommand {
					continue
				}

				if hook.Timeout <= 0 {
					t.Errorf("%s hook has ao command without timeout: %q", event, hook.Command)
				}
				if strings.Contains(cmd, "command -v ao") && !strings.Contains(cmd, "AGENTOPS_HOOKS_DISABLED") {
					t.Errorf("%s inline ao command missing AGENTOPS_HOOKS_DISABLED guard: %q", event, hook.Command)
				}
				if strings.Contains(cmd, "ao batch-feedback") {
					foundBatchFeedback = true
					if !strings.Contains(cmd, "--max-sessions") || !strings.Contains(cmd, "--max-runtime") {
						t.Errorf("batch-feedback hook missing bounded flags: %q", hook.Command)
					}
				}
			}
		}
	}

	if !foundBatchFeedback && !foundSessionEndMaintenance {
		t.Error("expected embedded hooks to include bounded session-end heavy maintenance")
	}
}

func TestInstallFromEmbedded(t *testing.T) {
	// Extract embedded files to a temp directory
	tmpDir := t.TempDir()

	copied, err := installFullHooksFromEmbed(tmpDir)
	if err != nil {
		t.Fatalf("installFullHooksFromEmbed failed: %v", err)
	}

	if copied == 0 {
		t.Fatal("expected files to be extracted, got 0")
	}

	// Verify hooks.json was extracted
	hooksJSON := filepath.Join(tmpDir, "hooks", "hooks.json")
	if _, err := os.Stat(hooksJSON); err != nil {
		t.Errorf("hooks.json not extracted: %v", err)
	}

	// Verify shell scripts are executable
	entries, err := os.ReadDir(filepath.Join(tmpDir, "hooks"))
	if err != nil {
		t.Fatalf("read hooks dir: %v", err)
	}

	shCount := 0
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".sh" {
			shCount++
			info, err := e.Info()
			if err != nil {
				t.Errorf("stat %s: %v", e.Name(), err)
				continue
			}
			if info.Mode()&0111 == 0 {
				t.Errorf("%s is not executable (mode: %o)", e.Name(), info.Mode())
			}
		}
	}

	if shCount != 31 {
		t.Errorf("expected 31 shell scripts, got %d", shCount)
	}

	// Verify hook-helpers.sh was extracted
	helpers := filepath.Join(tmpDir, "lib", "hook-helpers.sh")
	if _, err := os.Stat(helpers); err != nil {
		t.Errorf("hook-helpers.sh not extracted: %v", err)
	}

	// Verify chain-parser.sh was extracted
	chainParser := filepath.Join(tmpDir, "lib", "chain-parser.sh")
	if _, err := os.Stat(chainParser); err != nil {
		t.Errorf("chain-parser.sh not extracted: %v", err)
	}
}

func TestMatcherJSONRoundTrip(t *testing.T) {
	original := HookGroup{
		Matcher: "Write|Edit",
		Hooks: []HookEntry{
			{Type: "command", Command: "test", Timeout: 5},
		},
	}

	// Marshal
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	// Unmarshal
	var roundTripped HookGroup
	if err := json.Unmarshal(data, &roundTripped); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if roundTripped.Matcher != "Write|Edit" {
		t.Errorf("matcher lost in round-trip: got '%s'", roundTripped.Matcher)
	}
	if roundTripped.Hooks[0].Timeout != 5 {
		t.Errorf("timeout lost in round-trip: got %d", roundTripped.Hooks[0].Timeout)
	}
}
