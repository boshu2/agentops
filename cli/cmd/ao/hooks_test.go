// practices: [design-by-contract, gitops]
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"bytes"
	"fmt"

	"github.com/boshu2/agentops/cli/embedded"
)

func TestGenerateMinimalHooksConfig(t *testing.T) {
	hooks := generateMinimalHooksConfig()

	if len(hooks.SessionStart) == 0 {
		t.Error("expected SessionStart hooks, got none")
	}
	if len(hooks.SessionEnd) == 0 {
		t.Error("expected SessionEnd hooks, got none")
	}
	if len(hooks.Stop) == 0 {
		t.Error("expected Stop hooks, got none")
	}

	// Verify SessionStart uses script path
	found := false
	for _, g := range hooks.SessionStart {
		for _, h := range g.Hooks {
			if h.Type == "command" && strings.Contains(h.Command, "session-start.sh") {
				found = true
			}
		}
	}
	if !found {
		t.Error("expected session-start.sh script path in SessionStart hooks")
	}

	// Verify SessionEnd uses script path
	found = false
	for _, g := range hooks.SessionEnd {
		for _, h := range g.Hooks {
			if h.Type == "command" && strings.Contains(h.Command, "session-end-maintenance.sh") {
				found = true
			}
		}
	}
	if !found {
		t.Error("expected session-end-maintenance.sh script path in SessionEnd hooks")
	}

	// Verify Stop uses script path
	found = false
	for _, g := range hooks.Stop {
		for _, h := range g.Hooks {
			if h.Type == "command" && strings.Contains(h.Command, "ao-flywheel-close.sh") {
				found = true
			}
		}
	}
	if !found {
		t.Error("expected ao-flywheel-close.sh script path in Stop hooks")
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

func TestHooksCommandCopyMentionsCodexNativeHooks(t *testing.T) {
	if !strings.Contains(hooksCmd.Long, "scripts/install-codex-plugin.sh") {
		t.Fatalf("expected hooks command help to mention native Codex hook install, got:\n%s", hooksCmd.Long)
	}
	if !strings.Contains(hooksCmd.Long, "ao codex start/stop") {
		t.Fatalf("expected hooks command help to mention explicit Codex fallback, got:\n%s", hooksCmd.Long)
	}
	if !strings.Contains(hooksInstallCmd.Short, "Codex native hooks install separately") {
		t.Fatalf("expected hooks install short help to mention separate Codex native hook install, got %q", hooksInstallCmd.Short)
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

	hooks, ok := m["hooks"].([]map[string]any)
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
	hooks := m["hooks"].([]map[string]any)
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
	hooks2 := m2["hooks"].([]map[string]any)
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
	hooksMap := make(map[string]any)
	for _, event := range AllEventNames() {
		hooksMap[event] = []any{
			map[string]any{
				"hooks": []any{
					map[string]any{"type": "command", "command": "ao inject 2>/dev/null"},
				},
			},
			map[string]any{
				"hooks": []any{
					map[string]any{"type": "command", "command": "my-custom-hook"},
				},
			},
		}
	}

	for _, event := range AllEventNames() {
		filtered := filterNonAoHookGroups(hooksMap, event)
		if len(filtered) != 1 {
			t.Errorf("event %s: expected 1 non-ao group, got %d", event, len(filtered))
		}
		if hooks, ok := filtered[0]["hooks"].([]any); ok {
			if hook, ok := hooks[0].(map[string]any); ok {
				if hook["command"] != "my-custom-hook" {
					t.Errorf("event %s: expected non-ao hook preserved, got %v", event, hook["command"])
				}
			}
		}
	}
}

func TestHookGroupContainsAoAllEvents(t *testing.T) {
	hooksMap := make(map[string]any)
	for _, event := range AllEventNames() {
		hooksMap[event] = []any{
			map[string]any{
				"hooks": []any{
					map[string]any{"type": "command", "command": "ao inject stuff"},
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
	hooksMap := map[string]any{
		"SessionStart": []any{
			map[string]any{
				"hooks": []any{
					map[string]any{"type": "command", "command": "/Users/test/.agentops/hooks/session-start.sh"},
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
	// generateMinimalHooksConfig should ALWAYS return SessionStart + SessionEnd + Stop
	hooks := generateMinimalHooksConfig()
	if len(hooks.SessionStart) == 0 {
		t.Error("minimal config missing SessionStart")
	}
	if len(hooks.SessionEnd) == 0 {
		t.Error("minimal config missing SessionEnd")
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

	// Verify core flywheel events have hooks registered
	coreEvents := []string{"SessionStart", "SessionEnd", "Stop"}
	for _, event := range coreEvents {
		groups := config.GetEventGroups(event)
		if len(groups) == 0 {
			t.Errorf("embedded hooks.json: core event %s has no hook groups", event)
		}
	}
}

func TestGenerateFullHooksConfig(t *testing.T) {
	// generateFullHooksConfig should succeed (embedded fallback guarantees it)
	config, err := generateFullHooksConfig()
	if err != nil {
		t.Fatalf("generateFullHooksConfig failed: %v", err)
	}

	// Should have core flywheel events populated
	coreEvents := []string{"SessionStart", "SessionEnd", "Stop"}
	for _, event := range coreEvents {
		groups := config.GetEventGroups(event)
		if len(groups) == 0 {
			t.Errorf("full config: core event %s has no hook groups", event)
		}
	}
}

func TestEmbeddedAoCommandsHaveGuardrails(t *testing.T) {
	config, err := ReadHooksManifest(embedded.HooksJSON)
	if err != nil {
		t.Fatalf("failed to parse embedded hooks: %v", err)
	}

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
			}
		}
	}

	if !foundSessionEndMaintenance {
		t.Error("expected embedded hooks to include session-end-maintenance")
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

	if shCount < 30 {
		t.Errorf("expected at least 30 shell scripts, got %d", shCount)
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

func TestCollectScriptNames(t *testing.T) {
	tmp := t.TempDir()

	// Create some scripts
	for _, name := range []string{"session-start.sh", "stop.sh", "readme.txt"} {
		if err := os.WriteFile(filepath.Join(tmp, name), []byte("#!/bin/bash"), 0755); err != nil {
			t.Fatal(err)
		}
	}

	names := collectScriptNames(tmp)
	if len(names) != 2 {
		t.Errorf("expected 2 scripts, got %d", len(names))
	}
	if !names["session-start.sh"] {
		t.Error("expected session-start.sh in script names")
	}
	if !names["stop.sh"] {
		t.Error("expected stop.sh in script names")
	}
	if names["readme.txt"] {
		t.Error("did not expect readme.txt in script names")
	}
}

func TestCollectScriptNamesEmptyDir(t *testing.T) {
	tmp := t.TempDir()
	names := collectScriptNames(tmp)
	if len(names) != 0 {
		t.Errorf("expected 0 scripts, got %d", len(names))
	}
}

func TestCollectScriptNamesNonexistent(t *testing.T) {
	names := collectScriptNames("/nonexistent/path")
	if len(names) != 0 {
		t.Errorf("expected 0 scripts for nonexistent path, got %d", len(names))
	}
}

func TestCollectWiredScripts(t *testing.T) {
	hooksMap := map[string]any{
		"SessionStart": []any{
			map[string]any{
				"hooks": []any{
					map[string]any{"type": "command", "command": "/home/user/.agentops/hooks/session-start.sh"},
				},
			},
		},
		"Stop": []any{
			map[string]any{
				"hooks": []any{
					map[string]any{"type": "command", "command": "/home/user/.agentops/hooks/stop-team-guard.sh"},
					map[string]any{"type": "command", "command": "/home/user/.agentops/hooks/session-close.sh"},
				},
			},
		},
		"PreToolUse": []any{
			map[string]any{
				"matcher": "Write|Edit",
				"hooks": []any{
					map[string]any{"type": "command", "command": "/home/user/.agentops/hooks/standards-injector.sh"},
				},
			},
		},
	}

	eventScriptCount, wiredScripts := collectWiredScripts(hooksMap)

	if len(eventScriptCount) != 3 {
		t.Errorf("expected 3 events with scripts, got %d", len(eventScriptCount))
	}
	if eventScriptCount["SessionStart"] != 1 {
		t.Errorf("SessionStart: expected 1, got %d", eventScriptCount["SessionStart"])
	}
	if eventScriptCount["Stop"] != 2 {
		t.Errorf("Stop: expected 2, got %d", eventScriptCount["Stop"])
	}
	if eventScriptCount["PreToolUse"] != 1 {
		t.Errorf("PreToolUse: expected 1, got %d", eventScriptCount["PreToolUse"])
	}

	if len(wiredScripts) != 4 {
		t.Errorf("expected 4 unique wired scripts, got %d", len(wiredScripts))
	}
	for _, name := range []string{"session-start.sh", "stop-team-guard.sh", "session-close.sh", "standards-injector.sh"} {
		if !wiredScripts[name] {
			t.Errorf("expected %s in wired scripts", name)
		}
	}
}

func TestCollectWiredScriptsEmpty(t *testing.T) {
	hooksMap := map[string]any{}
	eventScriptCount, wiredScripts := collectWiredScripts(hooksMap)
	if len(eventScriptCount) != 0 {
		t.Errorf("expected 0 events, got %d", len(eventScriptCount))
	}
	if len(wiredScripts) != 0 {
		t.Errorf("expected 0 wired scripts, got %d", len(wiredScripts))
	}
}

func TestCollectWiredScriptsInlineCommands(t *testing.T) {
	// Hooks with inline ao commands (no .sh scripts)
	hooksMap := map[string]any{
		"SessionStart": []any{
			map[string]any{
				"hooks": []any{
					map[string]any{"type": "command", "command": "ao inject --max-tokens 1500 2>/dev/null || true"},
				},
			},
		},
	}

	eventScriptCount, wiredScripts := collectWiredScripts(hooksMap)
	// Inline ao commands don't reference .sh files
	if len(eventScriptCount) != 0 {
		t.Errorf("expected 0 events with scripts, got %d", len(eventScriptCount))
	}
	if len(wiredScripts) != 0 {
		t.Errorf("expected 0 wired scripts, got %d", len(wiredScripts))
	}
}

// --- Cobra execution tests for `ao hooks` commands ---

func TestHooksCommand_RootListsSubcommands(t *testing.T) {
	out, err := executeCommand("hooks")
	if err != nil {
		t.Fatalf("ao hooks failed: %v", err)
	}
	for _, sub := range []string{"init", "install", "show", "test"} {
		if !strings.Contains(out, sub) {
			t.Errorf("expected subcommand %q in output, got: %s", sub, out)
		}
	}
}

func TestHooksCommand_InitProducesValidJSON(t *testing.T) {
	// Reset format to default in case prior test changed it
	hooksOutputFormat = "json"

	out, err := executeCommand("hooks", "init")
	if err != nil {
		t.Fatalf("ao hooks init failed: %v", err)
	}

	var parsed map[string]any
	if jsonErr := json.Unmarshal([]byte(out), &parsed); jsonErr != nil {
		t.Fatalf("ao hooks init did not produce valid JSON: %v\noutput: %s", jsonErr, out)
	}

	hooksSection, ok := parsed["hooks"].(map[string]any)
	if !ok {
		t.Fatal("expected top-level 'hooks' key in JSON output")
	}
	if _, ok := hooksSection["SessionStart"]; !ok {
		t.Error("expected 'SessionStart' key in hooks JSON output")
	}
}

func TestHooksCommand_InitShellFormat(t *testing.T) {
	out, err := executeCommand("hooks", "init", "--format=shell")
	if err != nil {
		t.Fatalf("ao hooks init --format=shell failed: %v", err)
	}

	// Shell format should contain comment lines, not be valid JSON
	if !strings.Contains(out, "#") {
		t.Errorf("expected shell comments (#) in output, got: %s", out)
	}
	// Verify it is NOT valid JSON
	var probe map[string]any
	if json.Unmarshal([]byte(out), &probe) == nil {
		t.Errorf("shell format should not be valid JSON, but it parsed successfully")
	}
}

func TestHooksCommand_ShowHandlesNoSettings(t *testing.T) {
	tmp := chdirTemp(t)
	t.Setenv("HOME", tmp)

	out, err := executeCommand("hooks", "show")
	if err != nil {
		t.Fatalf("ao hooks show failed: %v", err)
	}
	// With no settings.json, should print guidance about installing hooks
	if !strings.Contains(out, "install") && !strings.Contains(out, "No") {
		t.Errorf("expected guidance about missing settings, got: %s", out)
	}
}

func TestHooksCommand_ShowWithSettings(t *testing.T) {
	tmp := chdirTemp(t)
	t.Setenv("HOME", tmp)

	// Create a minimal settings.json with hooks
	claudeDir := filepath.Join(tmp, ".claude")
	if err := os.MkdirAll(claudeDir, 0750); err != nil {
		t.Fatal(err)
	}
	settings := map[string]any{
		"hooks": map[string]any{
			"SessionStart": []any{
				map[string]any{
					"hooks": []any{
						map[string]any{"type": "command", "command": "ao inject 2>/dev/null"},
					},
				},
			},
		},
	}
	data, _ := json.Marshal(settings)
	if err := os.WriteFile(filepath.Join(claudeDir, "settings.json"), data, 0600); err != nil {
		t.Fatal(err)
	}

	out, err := executeCommand("hooks", "show")
	if err != nil {
		t.Fatalf("ao hooks show failed: %v", err)
	}
	if !strings.Contains(out, "SessionStart") {
		t.Errorf("expected SessionStart in show output, got: %s", out)
	}
}

func TestHooksCommand_InstallDryRun(t *testing.T) {
	tmp := chdirTemp(t)
	t.Setenv("HOME", tmp)

	out, err := executeCommand("hooks", "install", "--dry-run")
	if err != nil {
		t.Fatalf("ao hooks install --dry-run failed: %v", err)
	}
	if !strings.Contains(out, "dry-run") && !strings.Contains(out, "Would") {
		t.Errorf("expected dry-run indication in output, got: %s", out)
	}

	// Verify no settings.json was created
	settingsPath := filepath.Join(tmp, ".claude", "settings.json")
	if _, statErr := os.Stat(settingsPath); statErr == nil {
		t.Error("dry-run should NOT create settings.json, but it exists")
	}
}

func TestHooksCommand_TestDryRun(t *testing.T) {
	tmp := chdirTemp(t)
	t.Setenv("HOME", tmp)

	out, err := executeCommand("hooks", "test", "--dry-run")
	if err != nil {
		t.Fatalf("ao hooks test --dry-run failed: %v", err)
	}
	// Should show test steps header
	if !strings.Contains(out, "Testing") {
		t.Errorf("expected 'Testing' header in output, got: %s", out)
	}
}

// ---------------------------------------------------------------------------
// HooksConfig.eventGroupPtrs / eventGroupPtr
// ---------------------------------------------------------------------------

func TestHooksCoverage_eventGroupPtrs_ReturnsAllEvents(t *testing.T) {
	config := &HooksConfig{}
	// Verify all 12 canonical events are addressable via the public API
	events := AllEventNames()
	if len(events) != 12 {
		t.Errorf("expected 12 event names, got %d", len(events))
	}
	for _, event := range events {
		config.SetEventGroups(event, []HookGroup{{Hooks: []HookEntry{{Type: "command", Command: "test"}}}})
		groups := config.GetEventGroups(event)
		if len(groups) != 1 {
			t.Errorf("event %q: expected 1 group after set, got %d", event, len(groups))
		}
	}
}

// ---------------------------------------------------------------------------
// generateMinimalHooksConfig
// ---------------------------------------------------------------------------

func TestHooksCoverage_generateMinimalHooksConfig_PluginRootPlaceholder(t *testing.T) {
	config := generateMinimalHooksConfig()
	// All commands should contain ${CLAUDE_PLUGIN_ROOT} before replacement
	for _, g := range config.SessionStart {
		for _, h := range g.Hooks {
			if !strings.Contains(h.Command, "${CLAUDE_PLUGIN_ROOT}") {
				t.Errorf("SessionStart command missing placeholder: %s", h.Command)
			}
		}
	}
	for _, g := range config.SessionEnd {
		for _, h := range g.Hooks {
			if !strings.Contains(h.Command, "${CLAUDE_PLUGIN_ROOT}") {
				t.Errorf("SessionEnd command missing placeholder: %s", h.Command)
			}
		}
	}
	for _, g := range config.Stop {
		for _, h := range g.Hooks {
			if !strings.Contains(h.Command, "${CLAUDE_PLUGIN_ROOT}") {
				t.Errorf("Stop command missing placeholder: %s", h.Command)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// generateHooksConfig (falls back to minimal when full fails)
// ---------------------------------------------------------------------------

func TestHooksCoverage_generateHooksConfig_ReturnsNonNil(t *testing.T) {
	// generateHooksConfig always returns a non-nil config (falls back to minimal)
	config := generateHooksConfig()
	if config == nil {
		t.Fatal("expected non-nil config from generateHooksConfig")
	}
	// Should have at least the core events
	if len(config.SessionStart) == 0 {
		t.Error("expected SessionStart hooks")
	}
	if len(config.Stop) == 0 {
		t.Error("expected Stop hooks")
	}
}

// ---------------------------------------------------------------------------
// replacePluginRoot
// ---------------------------------------------------------------------------

func TestHooksCoverage_replacePluginRoot_EmptyBasePath(t *testing.T) {
	config := generateMinimalHooksConfig()
	replacePluginRoot(config, "")
	// Placeholder should be removed (replaced with empty string)
	for _, g := range config.SessionStart {
		for _, h := range g.Hooks {
			if strings.Contains(h.Command, "${CLAUDE_PLUGIN_ROOT}") {
				t.Errorf("placeholder should be removed, got: %s", h.Command)
			}
		}
	}
}

func TestHooksCoverage_replacePluginRoot_AllEvents(t *testing.T) {
	config := &HooksConfig{}
	for _, event := range AllEventNames() {
		config.SetEventGroups(event, []HookGroup{
			{Hooks: []HookEntry{{Type: "command", Command: "${CLAUDE_PLUGIN_ROOT}/hooks/test.sh"}}},
		})
	}
	replacePluginRoot(config, "/home/user/.agentops")
	for _, event := range AllEventNames() {
		groups := config.GetEventGroups(event)
		for _, g := range groups {
			for _, h := range g.Hooks {
				if strings.Contains(h.Command, "${CLAUDE_PLUGIN_ROOT}") {
					t.Errorf("event %s: placeholder not replaced in %s", event, h.Command)
				}
				if !strings.Contains(h.Command, "/home/user/.agentops/hooks/test.sh") {
					t.Errorf("event %s: expected replaced path, got %s", event, h.Command)
				}
			}
		}
	}
}

// ---------------------------------------------------------------------------
// ReadHooksManifest
// ---------------------------------------------------------------------------

func TestHooksCoverage_ReadHooksManifest_EmptyHooksObject(t *testing.T) {
	data := []byte(`{"hooks": {}}`)
	config, err := ReadHooksManifest(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should return config with no events populated
	for _, event := range AllEventNames() {
		if groups := config.GetEventGroups(event); len(groups) > 0 {
			t.Errorf("expected empty groups for %s, got %d", event, len(groups))
		}
	}
}

// ---------------------------------------------------------------------------
// hookGroupToMap
// ---------------------------------------------------------------------------

func TestHooksCoverage_hookGroupToMap_EmptyHooks(t *testing.T) {
	g := HookGroup{Hooks: []HookEntry{}}
	m := hookGroupToMap(g)
	hooks, ok := m["hooks"].([]map[string]any)
	if !ok {
		t.Fatal("expected hooks key in map")
	}
	if len(hooks) != 0 {
		t.Errorf("expected 0 hooks, got %d", len(hooks))
	}
	if _, exists := m["matcher"]; exists {
		t.Error("expected no matcher key for empty Matcher")
	}
}

func TestHooksCoverage_hookGroupToMap_ZeroTimeout(t *testing.T) {
	g := HookGroup{
		Hooks: []HookEntry{
			{Type: "command", Command: "test", Timeout: 0},
		},
	}
	m := hookGroupToMap(g)
	hooks := m["hooks"].([]map[string]any)
	if _, exists := hooks[0]["timeout"]; exists {
		t.Error("expected no timeout key when Timeout is 0")
	}
}

// ---------------------------------------------------------------------------
// loadHooksSettings
// ---------------------------------------------------------------------------

func TestHooksCoverage_loadHooksSettings_FileNotExist(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "nonexistent.json")
	result, err := loadHooksSettings(path)
	if err != nil {
		t.Fatalf("expected nil error for nonexistent file, got %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected empty map, got %v", result)
	}
}

func TestHooksCoverage_loadHooksSettings_ValidJSON(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "settings.json")
	content := `{"hooks": {"SessionStart": []}, "other": "value"}`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	result, err := loadHooksSettings(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := result["hooks"]; !ok {
		t.Error("expected hooks key in result")
	}
	if result["other"] != "value" {
		t.Error("expected other key preserved")
	}
}

func TestHooksCoverage_loadHooksSettings_InvalidJSON(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "settings.json")
	if err := os.WriteFile(path, []byte("not json{"), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := loadHooksSettings(path)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

// ---------------------------------------------------------------------------
// writeHooksSettings
// ---------------------------------------------------------------------------

func TestHooksCoverage_writeHooksSettings_CreatesDir(t *testing.T) {
	tmp := t.TempDir()
	settingsPath := filepath.Join(tmp, "subdir", "settings.json")
	rawSettings := map[string]any{"hooks": map[string]any{}}
	if err := writeHooksSettings(settingsPath, rawSettings); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Verify file exists and is valid JSON
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("file not created: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("written file is not valid JSON: %v", err)
	}
}

func TestHooksCoverage_backupHooksSettings_CreatesBackup(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "settings.json")
	content := `{"hooks":{}}`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	if err := backupHooksSettings(path); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Verify backup file exists
	entries, err := os.ReadDir(tmp)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "settings.json.backup.") {
			found = true
			// Verify backup content matches original
			backupData, err := os.ReadFile(filepath.Join(tmp, e.Name()))
			if err != nil {
				t.Fatal(err)
			}
			if string(backupData) != content {
				t.Error("backup content doesn't match original")
			}
		}
	}
	if !found {
		t.Error("expected backup file to be created")
	}
}

func TestHooksCoverage_cloneHooksMap_WithExistingHooks(t *testing.T) {
	rawSettings := map[string]any{
		"hooks": map[string]any{
			"SessionStart": []any{},
			"custom":       "value",
		},
	}
	result := cloneHooksMap(rawSettings)
	if len(result) != 2 {
		t.Errorf("expected 2 keys, got %d", len(result))
	}
	if result["custom"] != "value" {
		t.Error("expected custom key preserved")
	}
}

// ---------------------------------------------------------------------------
// existingAoHooksBlock
// ---------------------------------------------------------------------------

func TestHooksCoverage_existingAoHooksBlock_NoHooks(t *testing.T) {
	oldForce := hooksForce
	defer func() { hooksForce = oldForce }()
	hooksForce = false

	rawSettings := map[string]any{}
	if existingAoHooksBlock(rawSettings) {
		t.Error("expected false when no hooks present")
	}
}

func TestHooksCoverage_existingAoHooksBlock_WithForce(t *testing.T) {
	oldForce := hooksForce
	defer func() { hooksForce = oldForce }()
	hooksForce = true

	rawSettings := map[string]any{
		"hooks": map[string]any{
			"SessionStart": []any{
				map[string]any{
					"hooks": []any{
						map[string]any{"command": "ao inject"},
					},
				},
			},
		},
	}
	if existingAoHooksBlock(rawSettings) {
		t.Error("expected false when --force is set")
	}
}

func TestHooksCoverage_existingAoHooksBlock_WithAoHooks(t *testing.T) {
	oldForce := hooksForce
	defer func() { hooksForce = oldForce }()
	hooksForce = false

	rawSettings := map[string]any{
		"hooks": map[string]any{
			"SessionStart": []any{
				map[string]any{
					"hooks": []any{
						map[string]any{"command": "ao inject"},
					},
				},
			},
		},
	}
	if !existingAoHooksBlock(rawSettings) {
		t.Error("expected true when ao hooks are present and force is false")
	}
}

func TestHooksCoverage_existingAoHooksBlock_NonAoHooks(t *testing.T) {
	oldForce := hooksForce
	defer func() { hooksForce = oldForce }()
	hooksForce = false

	rawSettings := map[string]any{
		"hooks": map[string]any{
			"SessionStart": []any{
				map[string]any{
					"hooks": []any{
						map[string]any{"command": "echo hello"},
					},
				},
			},
		},
	}
	if existingAoHooksBlock(rawSettings) {
		t.Error("expected false when no ao hooks are present")
	}
}

// ---------------------------------------------------------------------------
// dryRunPrintSettings
// ---------------------------------------------------------------------------

func TestHooksCoverage_dryRunPrintSettings_NotDryRun(t *testing.T) {
	oldDryRun := hooksDryRun
	defer func() { hooksDryRun = oldDryRun }()
	hooksDryRun = false

	done, err := dryRunPrintSettings("/fake/path", map[string]any{})
	if done {
		t.Error("expected done=false when not in dry-run mode")
	}
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}

func TestHooksCoverage_dryRunPrintSettings_DryRun(t *testing.T) {
	oldDryRun := hooksDryRun
	defer func() { hooksDryRun = oldDryRun }()
	hooksDryRun = true

	rawSettings := map[string]any{
		"hooks": map[string]any{
			"SessionStart": []any{},
		},
	}
	done, err := dryRunPrintSettings("/fake/path", rawSettings)
	if !done {
		t.Error("expected done=true when in dry-run mode")
	}
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// generateHooksForInstall
// ---------------------------------------------------------------------------

func TestHooksCoverage_generateHooksForInstall_Minimal(t *testing.T) {
	oldFull := hooksFull
	defer func() { hooksFull = oldFull }()
	hooksFull = false

	config, events, err := generateHooksForInstall("/home/user/.agentops")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if config == nil {
		t.Fatal("expected non-nil config")
	}
	if len(events) != 3 {
		t.Errorf("expected 3 events for minimal install, got %d", len(events))
	}
	// Verify placeholder was replaced
	for _, g := range config.SessionStart {
		for _, h := range g.Hooks {
			if strings.Contains(h.Command, "${CLAUDE_PLUGIN_ROOT}") {
				t.Error("placeholder not replaced in minimal config")
			}
			if !strings.Contains(h.Command, "/home/user/.agentops") {
				t.Errorf("expected install base in command, got %s", h.Command)
			}
		}
	}
}

func TestHooksCoverage_generateHooksForInstall_Full(t *testing.T) {
	oldFull := hooksFull
	defer func() { hooksFull = oldFull }()
	hooksFull = true

	// This should succeed because embedded hooks are always available
	config, events, err := generateHooksForInstall("/home/user/.agentops")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if config == nil {
		t.Fatal("expected non-nil config")
	}
	// Dynamically count expected events from embedded manifest
	manifestData, mErr := findHooksManifest()
	if mErr != nil {
		t.Fatalf("could not read embedded manifest: %v", mErr)
	}
	manifestConfig, mErr := ReadHooksManifest(manifestData)
	if mErr != nil {
		t.Fatalf("could not parse embedded manifest: %v", mErr)
	}
	expectedEvents := activeEventNamesFromConfig(manifestConfig)
	if len(events) != len(expectedEvents) {
		t.Errorf("expected %d active manifest events for full install, got %d (events: %v)", len(expectedEvents), len(events), events)
	}
}

func TestHooksCoverage_UsesManifestEventCount(t *testing.T) {
	tmp := t.TempDir()
	hooksDir := filepath.Join(tmp, "hooks")
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		t.Fatal(err)
	}
	manifest := `{
		"hooks": {
			"SessionStart": [{"hooks": [{"type": "command", "command": "ao inject"}]}],
			"Stop": [{"hooks": [{"type": "command", "command": "ao forge"}]}]
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

	contract := resolveHookCoverageContract()
	if len(contract.ActiveEvents) != 2 {
		t.Fatalf("expected manifest-derived active event denominator 2, got %d", len(contract.ActiveEvents))
	}
	if contract.FallbackReason != "" {
		t.Fatalf("unexpected fallback reason: %s", contract.FallbackReason)
	}

	hooksMap := map[string]any{
		"SessionStart": []any{
			map[string]any{"hooks": []any{map[string]any{"command": "ao inject"}}},
		},
		"Stop": []any{
			map[string]any{"hooks": []any{map[string]any{"command": "ao forge"}}},
		},
	}
	got := countInstalledEventsForList(hooksMap, contract.ActiveEvents)
	if got != 2 {
		t.Errorf("expected 2/2 active events installed, got %d", got)
	}
}

func TestHooksCoverage_Legacy12EventSettingsMigration(t *testing.T) {
	hooksMap := make(map[string]any)
	for _, event := range AllEventNames() {
		hooksMap[event] = []any{
			map[string]any{
				"hooks": []any{
					map[string]any{"type": "command", "command": "ao legacy " + event},
				},
			},
		}
	}

	newHooks := generateMinimalHooksConfig()
	replacePluginRoot(newHooks, "/home/user/.agentops")
	eventsToInstall := []string{"SessionStart", "SessionEnd", "Stop"}

	installed := mergeHookEvents(hooksMap, newHooks, eventsToInstall)
	if installed != len(eventsToInstall) {
		t.Fatalf("installed events = %d, want %d", installed, len(eventsToInstall))
	}

	// Preserve+report migration policy: legacy ao-managed non-active events remain.
	if !hookGroupContainsAo(hooksMap, "PreToolUse") {
		t.Error("expected PreToolUse legacy ao hook to be preserved")
	}
	if !hookGroupContainsAo(hooksMap, "ConfigChange") {
		t.Error("expected ConfigChange legacy ao hook to be preserved")
	}
}

func TestHooksCoverage_PreservedLegacyEventsAreReported(t *testing.T) {
	hooksMap := map[string]any{
		"SessionStart": []any{
			map[string]any{"hooks": []any{map[string]any{"command": "ao inject"}}},
		},
		"PreToolUse": []any{
			map[string]any{"hooks": []any{map[string]any{"command": "ao legacy pre"}}},
		},
		"ConfigChange": []any{
			map[string]any{"hooks": []any{map[string]any{"command": "ao legacy config"}}},
		},
	}

	legacy := collectLegacyAoManagedEvents(hooksMap, []string{"SessionStart", "SessionEnd", "Stop"})
	if len(legacy) != 2 {
		t.Fatalf("expected 2 preserved legacy events, got %d", len(legacy))
	}
	if legacy[0] != "PreToolUse" || legacy[1] != "ConfigChange" {
		t.Fatalf("expected deterministic event order [PreToolUse ConfigChange], got %v", legacy)
	}

	report := formatLegacyPreservationReport(legacy)
	if !strings.Contains(report, "Preserved legacy ao-managed hooks outside active contract (2)") {
		t.Fatalf("unexpected report prefix: %s", report)
	}
	if !strings.Contains(report, "PreToolUse") || !strings.Contains(report, "ConfigChange") {
		t.Fatalf("report should include preserved events, got: %s", report)
	}
}

// ---------------------------------------------------------------------------
// commitHooksSettings
// ---------------------------------------------------------------------------

func TestHooksCoverage_commitHooksSettings(t *testing.T) {
	tmp := t.TempDir()
	settingsPath := filepath.Join(tmp, "settings.json")

	rawSettings := map[string]any{
		"hooks": map[string]any{
			"SessionStart": []any{},
		},
	}
	newHooks := generateMinimalHooksConfig()
	replacePluginRoot(newHooks, "/test")
	eventsToInstall := []string{"SessionStart", "SessionEnd", "Stop"}

	err := commitHooksSettings(settingsPath, rawSettings, newHooks, eventsToInstall, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify file was written
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("settings file not created: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("written file is not valid JSON: %v", err)
	}
}

// ---------------------------------------------------------------------------
// loadHooksMap
// ---------------------------------------------------------------------------

func TestHooksCoverage_loadHooksMap_FileNotExist(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "nonexistent.json")
	result, err := loadHooksMap(path)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if result != nil {
		t.Error("expected nil result for nonexistent file")
	}
}

func TestHooksCoverage_loadHooksMap_NoHooksKey(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "settings.json")
	if err := os.WriteFile(path, []byte(`{"other": "value"}`), 0644); err != nil {
		t.Fatal(err)
	}
	result, err := loadHooksMap(path)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if result != nil {
		t.Error("expected nil result when no hooks key")
	}
}

func TestHooksCoverage_loadHooksMap_ValidHooks(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "settings.json")
	content := `{"hooks": {"SessionStart": []}}`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	result, err := loadHooksMap(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil hooks map")
	}
}

func TestHooksCoverage_loadHooksMap_InvalidJSON(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "settings.json")
	if err := os.WriteFile(path, []byte("not-json{"), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := loadHooksMap(path)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestHooksCoverage_loadHooksMap_HooksNotMap(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "settings.json")
	if err := os.WriteFile(path, []byte(`{"hooks": "not-a-map"}`), 0644); err != nil {
		t.Fatal(err)
	}
	result, err := loadHooksMap(path)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if result != nil {
		t.Error("expected nil result when hooks is not a map")
	}
}

// ---------------------------------------------------------------------------
// countRawGroupHooks
// ---------------------------------------------------------------------------

func TestHooksCoverage_countRawGroupHooks(t *testing.T) {
	tests := []struct {
		name   string
		groups []any
		want   int
	}{
		{
			name:   "empty groups",
			groups: []any{},
			want:   0,
		},
		{
			name: "single group single hook",
			groups: []any{
				map[string]any{
					"hooks": []any{
						map[string]any{"command": "test"},
					},
				},
			},
			want: 1,
		},
		{
			name: "multiple groups multiple hooks",
			groups: []any{
				map[string]any{
					"hooks": []any{
						map[string]any{"command": "test1"},
						map[string]any{"command": "test2"},
					},
				},
				map[string]any{
					"hooks": []any{
						map[string]any{"command": "test3"},
					},
				},
			},
			want: 3,
		},
		{
			name: "non-map group skipped",
			groups: []any{
				"not-a-map",
				map[string]any{
					"hooks": []any{
						map[string]any{"command": "test"},
					},
				},
			},
			want: 1,
		},
		{
			name: "group without hooks key",
			groups: []any{
				map[string]any{
					"matcher": "test",
				},
			},
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := countRawGroupHooks(tt.groups)
			if got != tt.want {
				t.Errorf("countRawGroupHooks() = %d, want %d", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// printEventCoverage
// ---------------------------------------------------------------------------

func TestHooksCoverage_printEventCoverage_AllInstalled(t *testing.T) {
	hooksMap := make(map[string]any)
	for _, event := range AllEventNames() {
		hooksMap[event] = []any{
			map[string]any{
				"hooks": []any{
					map[string]any{"command": "test"},
				},
			},
		}
	}
	count := printEventCoverage(hooksMap)
	if count != 12 {
		t.Errorf("expected 12 installed events, got %d", count)
	}
}

func TestHooksCoverage_printEventCoverage_NoneInstalled(t *testing.T) {
	hooksMap := make(map[string]any)
	count := printEventCoverage(hooksMap)
	if count != 0 {
		t.Errorf("expected 0 installed events, got %d", count)
	}
}

func TestHooksCoverage_printEventCoverage_Partial(t *testing.T) {
	hooksMap := map[string]any{
		"SessionStart": []any{
			map[string]any{
				"hooks": []any{map[string]any{"command": "test"}},
			},
		},
		"Stop": []any{
			map[string]any{
				"hooks": []any{map[string]any{"command": "test"}},
			},
		},
	}
	count := printEventCoverage(hooksMap)
	if count != 2 {
		t.Errorf("expected 2 installed events, got %d", count)
	}
}

// ---------------------------------------------------------------------------
// countInstalledHookEvents
// ---------------------------------------------------------------------------

func TestHooksCoverage_countInstalledHookEvents(t *testing.T) {
	tests := []struct {
		name     string
		hooksMap map[string]any
		want     int
	}{
		{
			name:     "empty map",
			hooksMap: map[string]any{},
			want:     0,
		},
		{
			name: "one event with groups",
			hooksMap: map[string]any{
				"SessionStart": []any{map[string]any{}},
			},
			want: 1,
		},
		{
			name: "event with empty array",
			hooksMap: map[string]any{
				"SessionStart": []any{},
			},
			want: 0,
		},
		{
			name: "non-standard event ignored",
			hooksMap: map[string]any{
				"CustomEvent": []any{map[string]any{}},
			},
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := countInstalledHookEvents(tt.hooksMap)
			if got != tt.want {
				t.Errorf("countInstalledHookEvents() = %d, want %d", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// readSettingsHooksMap
// ---------------------------------------------------------------------------

func TestHooksCoverage_readSettingsHooksMap_FileNotExist(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "nonexistent.json")
	result, err := readSettingsHooksMap(path)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if result != nil {
		t.Error("expected nil result for nonexistent file")
	}
}

func TestHooksCoverage_readSettingsHooksMap_InvalidJSON(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "settings.json")
	if err := os.WriteFile(path, []byte("invalid"), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := readSettingsHooksMap(path)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestHooksCoverage_readSettingsHooksMap_NoHooksKey(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "settings.json")
	if err := os.WriteFile(path, []byte(`{"other": "value"}`), 0644); err != nil {
		t.Fatal(err)
	}
	result, err := readSettingsHooksMap(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Error("expected nil result when no hooks key")
	}
}

func TestHooksCoverage_readSettingsHooksMap_HooksNotMap(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "settings.json")
	if err := os.WriteFile(path, []byte(`{"hooks": "string-value"}`), 0644); err != nil {
		t.Fatal(err)
	}
	result, err := readSettingsHooksMap(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Error("expected nil result when hooks is not a map")
	}
}

func TestHooksCoverage_readSettingsHooksMap_Valid(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "settings.json")
	content := `{"hooks": {"SessionStart": [{"hooks": [{"command": "test"}]}]}}`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	result, err := readSettingsHooksMap(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil hooks map")
	}
	if _, ok := result["SessionStart"]; !ok {
		t.Error("expected SessionStart key in hooks map")
	}
}

// ---------------------------------------------------------------------------
// rawGroupIsAoManaged / rawGroupHooksContainAo / rawGroupLegacyContainsAo
// ---------------------------------------------------------------------------

func TestHooksCoverage_rawGroupIsAoManaged(t *testing.T) {
	tests := []struct {
		name  string
		group map[string]any
		want  bool
	}{
		{
			name: "new format ao command",
			group: map[string]any{
				"hooks": []any{
					map[string]any{"command": "ao inject"},
				},
			},
			want: true,
		},
		{
			name: "new format agentops script",
			group: map[string]any{
				"hooks": []any{
					map[string]any{"command": "/home/user/.agentops/hooks/test.sh"},
				},
			},
			want: true,
		},
		{
			name: "legacy format ao command",
			group: map[string]any{
				"command": []any{"bash", "ao flywheel"},
			},
			want: true,
		},
		{
			name: "non-ao group",
			group: map[string]any{
				"hooks": []any{
					map[string]any{"command": "echo hello"},
				},
			},
			want: false,
		},
		{
			name:  "empty group",
			group: map[string]any{},
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := rawGroupIsAoManaged(tt.group)
			if got != tt.want {
				t.Errorf("rawGroupIsAoManaged() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHooksCoverage_rawGroupHooksContainAo(t *testing.T) {
	tests := []struct {
		name  string
		group map[string]any
		want  bool
	}{
		{
			name:  "no hooks key",
			group: map[string]any{},
			want:  false,
		},
		{
			name: "hooks not array",
			group: map[string]any{
				"hooks": "not-array",
			},
			want: false,
		},
		{
			name: "hook entries not maps",
			group: map[string]any{
				"hooks": []any{"not-a-map"},
			},
			want: false,
		},
		{
			name: "command not string",
			group: map[string]any{
				"hooks": []any{
					map[string]any{"command": 123},
				},
			},
			want: false,
		},
		{
			name: "ao command present",
			group: map[string]any{
				"hooks": []any{
					map[string]any{"command": "ao inject"},
				},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := rawGroupHooksContainAo(tt.group)
			if got != tt.want {
				t.Errorf("rawGroupHooksContainAo() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHooksCoverage_rawGroupLegacyContainsAo(t *testing.T) {
	tests := []struct {
		name  string
		group map[string]any
		want  bool
	}{
		{
			name:  "no command key",
			group: map[string]any{},
			want:  false,
		},
		{
			name: "command not array",
			group: map[string]any{
				"command": "string",
			},
			want: false,
		},
		{
			name: "command array too short",
			group: map[string]any{
				"command": []any{"bash"},
			},
			want: false,
		},
		{
			name: "second element not string",
			group: map[string]any{
				"command": []any{"bash", 123},
			},
			want: false,
		},
		{
			name: "legacy ao command",
			group: map[string]any{
				"command": []any{"bash", "ao flywheel close-loop"},
			},
			want: true,
		},
		{
			name: "legacy non-ao command",
			group: map[string]any{
				"command": []any{"bash", "echo hello"},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := rawGroupLegacyContainsAo(tt.group)
			if got != tt.want {
				t.Errorf("rawGroupLegacyContainsAo() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHooksCoverage_filterNonAoHookGroups_NonMapEntries(t *testing.T) {
	hooksMap := map[string]any{
		"SessionStart": []any{
			"not-a-map",
			map[string]any{
				"hooks": []any{
					map[string]any{"command": "echo hello"},
				},
			},
		},
	}
	result := filterNonAoHookGroups(hooksMap, "SessionStart")
	if len(result) != 1 {
		t.Errorf("expected 1 group (non-map skipped), got %d", len(result))
	}
}

// ---------------------------------------------------------------------------
// hooksCopyFile
// ---------------------------------------------------------------------------

func TestHooksCoverage_hooksCopyFile(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "source.sh")
	dst := filepath.Join(tmp, "subdir", "dest.sh")

	content := "#!/bin/bash\necho hello"
	if err := os.WriteFile(src, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	if err := hooksCopyFile(src, dst); err != nil {
		t.Fatalf("hooksCopyFile failed: %v", err)
	}

	data, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("destination not created: %v", err)
	}
	if string(data) != content {
		t.Error("content mismatch")
	}
}

// ---------------------------------------------------------------------------
// copyDir
// ---------------------------------------------------------------------------

func TestHooksCoverage_copyDir(t *testing.T) {
	tmp := t.TempDir()
	srcDir := filepath.Join(tmp, "src")
	dstDir := filepath.Join(tmp, "dst")
	if err := os.MkdirAll(filepath.Join(srcDir, "sub"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "a.txt"), []byte("a"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "sub", "b.txt"), []byte("b"), 0644); err != nil {
		t.Fatal(err)
	}

	count, err := copyDir(srcDir, dstDir)
	if err != nil {
		t.Fatalf("copyDir failed: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 files copied, got %d", count)
	}

	// Verify files exist
	if _, err := os.Stat(filepath.Join(dstDir, "a.txt")); err != nil {
		t.Error("a.txt not copied")
	}
	if _, err := os.Stat(filepath.Join(dstDir, "sub", "b.txt")); err != nil {
		t.Error("sub/b.txt not copied")
	}
}

// ---------------------------------------------------------------------------
// copyShellScripts
// ---------------------------------------------------------------------------

func TestHooksCoverage_copyShellScripts(t *testing.T) {
	tmp := t.TempDir()
	srcDir := filepath.Join(tmp, "src")
	dstDir := filepath.Join(tmp, "dst")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create mixed files
	for _, name := range []string{"hook1.sh", "hook2.sh", "readme.md"} {
		if err := os.WriteFile(filepath.Join(srcDir, name), []byte("content"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	// Create a subdirectory (should be skipped)
	if err := os.MkdirAll(filepath.Join(srcDir, "subdir"), 0755); err != nil {
		t.Fatal(err)
	}

	count, err := copyShellScripts(srcDir, dstDir)
	if err != nil {
		t.Fatalf("copyShellScripts failed: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 scripts copied, got %d", count)
	}

	// Verify scripts are executable
	for _, name := range []string{"hook1.sh", "hook2.sh"} {
		info, err := os.Stat(filepath.Join(dstDir, name))
		if err != nil {
			t.Fatalf("script %s not copied: %v", name, err)
		}
		if info.Mode()&0111 == 0 {
			t.Errorf("script %s should be executable", name)
		}
	}

	// readme.md should not be copied
	if _, err := os.Stat(filepath.Join(dstDir, "readme.md")); !os.IsNotExist(err) {
		t.Error("readme.md should not be copied")
	}
}

func TestHooksCoverage_copyOptionalFile_Present(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "source.txt")
	dst := filepath.Join(tmp, "subdir", "dest.txt")
	if err := os.WriteFile(src, []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}
	count, err := copyOptionalFile(src, dst, "test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 copied, got %d", count)
	}
	data, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal("destination not created")
	}
	if string(data) != "content" {
		t.Error("content mismatch")
	}
}

func TestHooksCoverage_copyOptionalDir_Present(t *testing.T) {
	tmp := t.TempDir()
	srcDir := filepath.Join(tmp, "src")
	dstDir := filepath.Join(tmp, "dst")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "file.txt"), []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}
	count, err := copyOptionalDir(srcDir, dstDir, "test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 file copied, got %d", count)
	}
}

// ---------------------------------------------------------------------------
// installFullHooks
// ---------------------------------------------------------------------------

func TestHooksCoverage_installFullHooks_NoGitDir(t *testing.T) {
	tmp := t.TempDir()
	// Source dir without .git - should fail
	srcDir := filepath.Join(tmp, "src")
	if err := os.MkdirAll(filepath.Join(srcDir, "hooks"), 0755); err != nil {
		t.Fatal(err)
	}
	installBase := filepath.Join(tmp, "install")

	_, err := installFullHooks(srcDir, installBase)
	if err == nil {
		t.Error("expected error when .git directory is missing")
	}
	if !strings.Contains(err.Error(), "not a git root") {
		t.Errorf("expected 'not a git root' error, got: %v", err)
	}
}

func TestHooksCoverage_installFullHooks_WithGitDir(t *testing.T) {
	tmp := t.TempDir()
	srcDir := filepath.Join(tmp, "src")
	installBase := filepath.Join(tmp, "install")

	// Create source dir with .git and hooks
	if err := os.MkdirAll(filepath.Join(srcDir, ".git"), 0755); err != nil {
		t.Fatal(err)
	}
	hooksDir := filepath.Join(srcDir, "hooks")
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hooksDir, "test.sh"), []byte("#!/bin/bash"), 0644); err != nil {
		t.Fatal(err)
	}

	count, err := installFullHooks(srcDir, installBase)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count < 1 {
		t.Errorf("expected at least 1 file copied, got %d", count)
	}
}

// ---------------------------------------------------------------------------
// installFullHooksFromEmbed
// ---------------------------------------------------------------------------

func TestHooksCoverage_installFullHooksFromEmbed(t *testing.T) {
	tmp := t.TempDir()
	count, err := installFullHooksFromEmbed(tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count == 0 {
		t.Error("expected at least 1 file extracted")
	}

	// Verify hooks.json was extracted
	hooksJSON := filepath.Join(tmp, "hooks", "hooks.json")
	if _, err := os.Stat(hooksJSON); err != nil {
		t.Errorf("hooks.json not extracted: %v", err)
	}
}

// ---------------------------------------------------------------------------
// printHooksInstallSummary (just verify no panic)
// ---------------------------------------------------------------------------

func TestHooksCoverage_printHooksInstallSummary_Minimal(t *testing.T) {
	oldFull := hooksFull
	defer func() { hooksFull = oldFull }()
	hooksFull = false

	newHooks := generateMinimalHooksConfig()
	out, _ := captureStdout(t, func() error {
		printHooksInstallSummary("/fake/settings.json", newHooks, []string{"SessionStart", "SessionEnd", "Stop"}, 3)
		return nil
	})
	if !strings.Contains(out, "settings.json") {
		t.Errorf("output missing settings path:\n%s", out)
	}
}

func TestHooksCoverage_printHooksInstallSummary_Full(t *testing.T) {
	oldFull := hooksFull
	defer func() { hooksFull = oldFull }()
	hooksFull = true

	newHooks := generateMinimalHooksConfig()
	out, _ := captureStdout(t, func() error {
		printHooksInstallSummary("/fake/settings.json", newHooks, AllEventNames(), 3)
		return nil
	})
	if !strings.Contains(out, "settings.json") {
		t.Errorf("output missing settings path:\n%s", out)
	}
}

// ---------------------------------------------------------------------------
// runSettingsCoverageTest
// ---------------------------------------------------------------------------

func TestHooksCoverage_runSettingsCoverageTest_NoSettings(t *testing.T) {
	tmp := t.TempDir()
	allPassed := true
	out, _ := captureStdout(t, func() error {
		runSettingsCoverageTest(1, tmp, &allPassed)
		return nil
	})
	// Missing settings should produce diagnostic output
	if out == "" {
		t.Error("expected non-empty output for missing settings")
	}
}

func TestHooksCoverage_runSettingsCoverageTest_WithSettings(t *testing.T) {
	tmp := t.TempDir()
	claudeDir := filepath.Join(tmp, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatal(err)
	}
	content := `{"hooks": {"SessionStart": [{"hooks": [{"command": "test"}]}]}}`
	if err := os.WriteFile(filepath.Join(claudeDir, "settings.json"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	allPassed := true
	runSettingsCoverageTest(1, tmp, &allPassed)
	// Should still pass
	if !allPassed {
		t.Error("expected allPassed to remain true")
	}
}

func TestHooksCoverage_runSettingsCoverageTest_InvalidJSON(t *testing.T) {
	tmp := t.TempDir()
	claudeDir := filepath.Join(tmp, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(claudeDir, "settings.json"), []byte("invalid"), 0644); err != nil {
		t.Fatal(err)
	}
	allPassed := true
	runSettingsCoverageTest(1, tmp, &allPassed)
	if allPassed {
		t.Error("expected allPassed to be false for invalid JSON")
	}
}

// ---------------------------------------------------------------------------
// runHookScriptsAccessTest
// ---------------------------------------------------------------------------

func TestHooksCoverage_runHookScriptsAccessTest_NoAgentopsDir(t *testing.T) {
	tmp := t.TempDir()
	out, _ := captureStdout(t, func() error {
		runHookScriptsAccessTest(1, tmp)
		return nil
	})
	if out == "" {
		t.Error("expected non-empty output for missing agentops dir")
	}
}

func TestHooksCoverage_runHookScriptsAccessTest_WithScripts(t *testing.T) {
	tmp := t.TempDir()
	hooksDir := filepath.Join(tmp, ".agentops", "hooks")
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		t.Fatal(err)
	}
	// Create executable scripts
	for _, name := range []string{"test1.sh", "test2.sh"} {
		if err := os.WriteFile(filepath.Join(hooksDir, name), []byte("#!/bin/bash"), 0755); err != nil {
			t.Fatal(err)
		}
	}
	// Should not panic
	runHookScriptsAccessTest(1, tmp)
}

func TestHooksCoverage_runHookScriptsAccessTest_NonExecutableScripts(t *testing.T) {
	tmp := t.TempDir()
	hooksDir := filepath.Join(tmp, ".agentops", "hooks")
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		t.Fatal(err)
	}
	// Create non-executable script
	if err := os.WriteFile(filepath.Join(hooksDir, "test.sh"), []byte("#!/bin/bash"), 0644); err != nil {
		t.Fatal(err)
	}
	// Should not panic, prints warning
	runHookScriptsAccessTest(1, tmp)
}

func TestHooksCoverage_runHookScriptsAccessTest_WithSettingsAndScripts(t *testing.T) {
	tmp := t.TempDir()

	// Create hooks scripts
	hooksDir := filepath.Join(tmp, ".agentops", "hooks")
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"session-start.sh", "orphan.sh"} {
		if err := os.WriteFile(filepath.Join(hooksDir, name), []byte("#!/bin/bash"), 0755); err != nil {
			t.Fatal(err)
		}
	}

	// Create settings with wired hooks
	claudeDir := filepath.Join(tmp, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatal(err)
	}
	settings := map[string]any{
		"hooks": map[string]any{
			"SessionStart": []any{
				map[string]any{
					"hooks": []any{
						map[string]any{
							"type":    "command",
							"command": fmt.Sprintf("%s/session-start.sh", hooksDir),
						},
					},
				},
			},
		},
	}
	data, _ := json.MarshalIndent(settings, "", "  ")
	if err := os.WriteFile(filepath.Join(claudeDir, "settings.json"), data, 0644); err != nil {
		t.Fatal(err)
	}

	// Should not panic, should report scripts and unwired count
	runHookScriptsAccessTest(1, tmp)
}

// ---------------------------------------------------------------------------
// runForgeTranscriptAccessTest
// ---------------------------------------------------------------------------

func TestHooksCoverage_runForgeTranscriptAccessTest_DryRun(t *testing.T) {
	oldDryRun := hooksDryRun
	defer func() { hooksDryRun = oldDryRun }()
	hooksDryRun = true

	out, _ := captureStdout(t, func() error {
		runForgeTranscriptAccessTest(1, t.TempDir())
		return nil
	})
	// Dry run should indicate skipping, produce minimal output, or be empty (no projects dir)
	if out != "" && !strings.Contains(out, "SKIP") && !strings.Contains(out, "skip") && !strings.Contains(out, "dry") && !strings.Contains(out, "forge") {
		t.Errorf("unexpected dry-run output: %s", out)
	}
}

func TestHooksCoverage_runForgeTranscriptAccessTest_NoProjectsDir(t *testing.T) {
	oldDryRun := hooksDryRun
	defer func() { hooksDryRun = oldDryRun }()
	hooksDryRun = false

	tmp := t.TempDir()
	out, _ := captureStdout(t, func() error {
		runForgeTranscriptAccessTest(1, tmp)
		return nil
	})
	if out == "" {
		t.Error("expected non-empty output for missing projects dir")
	}
}

// ---------------------------------------------------------------------------
// runInjectCommandTest (dry-run only)
// ---------------------------------------------------------------------------

func TestHooksCoverage_runInjectCommandTest_DryRun(t *testing.T) {
	oldDryRun := hooksDryRun
	defer func() { hooksDryRun = oldDryRun }()
	hooksDryRun = true

	allPassed := true
	// Should skip
	runInjectCommandTest(1, &allPassed)
	if !allPassed {
		t.Error("expected allPassed to remain true in dry-run mode")
	}
}

// ---------------------------------------------------------------------------
// resolveSourceDir
// ---------------------------------------------------------------------------

func TestHooksCoverage_resolveSourceDir_ExplicitValid(t *testing.T) {
	tmp := t.TempDir()
	hooksDir := filepath.Join(tmp, "hooks")
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hooksDir, "hooks.json"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}

	oldSourceDir := hooksSourceDir
	defer func() { hooksSourceDir = oldSourceDir }()
	hooksSourceDir = tmp

	result, err := resolveSourceDir()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != tmp {
		t.Errorf("expected %s, got %s", tmp, result)
	}
}

// ---------------------------------------------------------------------------
// collectScriptNames
// ---------------------------------------------------------------------------

func TestHooksCoverage_collectScriptNames_WithMixedFiles(t *testing.T) {
	tmp := t.TempDir()
	for _, name := range []string{"a.sh", "b.sh", "c.txt", "d.py"} {
		if err := os.WriteFile(filepath.Join(tmp, name), []byte(""), 0644); err != nil {
			t.Fatal(err)
		}
	}
	names := collectScriptNames(tmp)
	if len(names) != 2 {
		t.Errorf("expected 2, got %d", len(names))
	}
	if !names["a.sh"] || !names["b.sh"] {
		t.Error("expected a.sh and b.sh")
	}
}

// ---------------------------------------------------------------------------
// collectWiredScripts
// ---------------------------------------------------------------------------

func TestHooksCoverage_collectWiredScripts_MalformedEntries(t *testing.T) {
	hooksMap := map[string]any{
		"SessionStart": []any{
			"not-a-map", // should be skipped
			map[string]any{
				"hooks": "not-an-array", // should be skipped
			},
			map[string]any{
				"hooks": []any{
					"not-a-map-hook", // should be skipped
					map[string]any{
						"command": 123, // not a string, should be skipped
					},
				},
			},
		},
	}
	eventScriptCount, wiredScripts := collectWiredScripts(hooksMap)
	if len(eventScriptCount) != 0 {
		t.Errorf("expected 0 events, got %d", len(eventScriptCount))
	}
	if len(wiredScripts) != 0 {
		t.Errorf("expected 0 wired scripts, got %d", len(wiredScripts))
	}
}

// ---------------------------------------------------------------------------
// isAoManagedHookCommand
// ---------------------------------------------------------------------------

func TestHooksCoverage_isAoManagedHookCommand(t *testing.T) {
	tests := []struct {
		cmd  string
		want bool
	}{
		{"ao inject --max-tokens 100", true},
		{"/home/user/.agentops/hooks/session-start.sh", true},
		{"echo hello", false},
		{"", false},
		{"/opt/tools/ao-runner.sh", false}, // "ao" not followed by space
		{"ao ", true},                      // "ao " is detected
	}
	for _, tt := range tests {
		got := isAoManagedHookCommand(tt.cmd)
		if got != tt.want {
			t.Errorf("isAoManagedHookCommand(%q) = %v, want %v", tt.cmd, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// HookEntry / HookGroup JSON round-trip
// ---------------------------------------------------------------------------

func TestHooksCoverage_HookEntry_JSONRoundTrip(t *testing.T) {
	entry := HookEntry{
		Type:    "command",
		Command: "ao inject",
		Timeout: 30,
	}
	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatal(err)
	}
	var decoded HookEntry
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Type != entry.Type || decoded.Command != entry.Command || decoded.Timeout != entry.Timeout {
		t.Error("round-trip mismatch")
	}
}

func TestHooksCoverage_HookEntry_ZeroTimeout_OmittedInJSON(t *testing.T) {
	entry := HookEntry{Type: "command", Command: "test", Timeout: 0}
	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte("timeout")) {
		t.Error("expected timeout to be omitted when 0")
	}
}

func TestHooksCoverage_HookGroup_JSONRoundTrip(t *testing.T) {
	group := HookGroup{
		Matcher: "Write|Edit",
		Hooks: []HookEntry{
			{Type: "command", Command: "test1", Timeout: 5},
			{Type: "command", Command: "test2"},
		},
	}
	data, err := json.Marshal(group)
	if err != nil {
		t.Fatal(err)
	}
	var decoded HookGroup
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Matcher != group.Matcher {
		t.Errorf("matcher mismatch: got %q, want %q", decoded.Matcher, group.Matcher)
	}
	if len(decoded.Hooks) != 2 {
		t.Errorf("expected 2 hooks, got %d", len(decoded.Hooks))
	}
}

// ---------------------------------------------------------------------------
// HooksConfig JSON round-trip
// ---------------------------------------------------------------------------

func TestHooksCoverage_HooksConfig_JSONRoundTrip(t *testing.T) {
	config := &HooksConfig{}
	for _, event := range AllEventNames() {
		config.SetEventGroups(event, []HookGroup{
			{
				Matcher: "test-matcher",
				Hooks: []HookEntry{
					{Type: "command", Command: fmt.Sprintf("test-%s", event), Timeout: 10},
				},
			},
		})
	}

	data, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}

	var decoded HooksConfig
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}

	for _, event := range AllEventNames() {
		groups := decoded.GetEventGroups(event)
		if len(groups) != 1 {
			t.Errorf("event %s: expected 1 group after round-trip, got %d", event, len(groups))
		}
	}
}

// ---------------------------------------------------------------------------
// hooksManifest type
// ---------------------------------------------------------------------------

func TestHooksCoverage_hooksManifest_Unmarshal(t *testing.T) {
	data := []byte(`{"hooks": {"SessionStart": [{"hooks": [{"type": "command", "command": "test"}]}]}}`)
	config, err := ReadHooksManifest(data)
	if err != nil {
		t.Fatal(err)
	}
	if config == nil {
		t.Fatal("expected non-nil config")
	}
	if len(config.SessionStart) != 1 {
		t.Errorf("expected 1 SessionStart group, got %d", len(config.SessionStart))
	}
}

// ---------------------------------------------------------------------------
// mergeHookEvents edge cases
// ---------------------------------------------------------------------------

func TestHooksCoverage_mergeHookEvents_MultipleEvents(t *testing.T) {
	hooksMap := map[string]any{}
	config := &HooksConfig{}
	config.SetEventGroups("SessionStart", []HookGroup{
		{Hooks: []HookEntry{{Command: "ao start"}}},
	})
	config.SetEventGroups("Stop", []HookGroup{
		{Hooks: []HookEntry{{Command: "ao stop"}}},
	})
	config.SetEventGroups("PreToolUse", []HookGroup{
		{Matcher: "Write", Hooks: []HookEntry{{Command: "ao pre"}}},
	})

	count := mergeHookEvents(hooksMap, config, []string{"SessionStart", "Stop", "PreToolUse", "PostToolUse"})
	if count != 3 {
		t.Errorf("expected 3 installed events (PostToolUse empty), got %d", count)
	}
}

// ---------------------------------------------------------------------------
// runHooksInit output format validation
// ---------------------------------------------------------------------------

func TestHooksCoverage_runHooksInit_UnknownFormat(t *testing.T) {
	oldFormat := hooksOutputFormat
	defer func() { hooksOutputFormat = oldFormat }()
	hooksOutputFormat = "xml"

	err := runHooksInit(nil, nil)
	if err == nil {
		t.Error("expected error for unknown format")
	}
	if !strings.Contains(err.Error(), "unknown format") {
		t.Errorf("expected 'unknown format' error, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// fallbackHookCoverageContract
// ---------------------------------------------------------------------------

func TestFallbackContract_Basic(t *testing.T) {
	c := fallbackHookCoverageContract("test reason")
	if c.ActiveEvents == nil {
		t.Fatal("expected non-nil ActiveEvents")
	}
	if c.FallbackReason != "test reason" {
		t.Errorf("FallbackReason = %q, want %q", c.FallbackReason, "test reason")
	}
	if len(c.ActiveEvents) != 12 {
		t.Errorf("expected 12 active events (all), got %d", len(c.ActiveEvents))
	}
}

func TestFallbackContract_EmptyReason(t *testing.T) {
	c := fallbackHookCoverageContract("")
	if c.FallbackReason != "" {
		t.Errorf("expected empty FallbackReason, got %q", c.FallbackReason)
	}
}

// ---------------------------------------------------------------------------
// printLegacyPreservationReport
// ---------------------------------------------------------------------------

func TestPrintLegacyReport_NonEmpty(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	printLegacyPreservationReport([]string{"PreToolUse", "PostToolUse"})
	_ = w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	if !strings.Contains(buf.String(), "PreToolUse") {
		t.Error("expected PreToolUse in report")
	}
}

func TestPrintLegacyReport_Empty(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	printLegacyPreservationReport([]string{})
	_ = w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	if buf.Len() > 0 {
		t.Error("expected no output for empty slice")
	}
}

// ---------------------------------------------------------------------------
// SetEventGroups / activeEventNamesFromConfig
// ---------------------------------------------------------------------------

func TestSetEventGroups_Unknown(t *testing.T) {
	config := &HooksConfig{}
	config.SetEventGroups("FakeEvent", []HookGroup{{Matcher: "", Hooks: []HookEntry{{Type: "command", Command: "echo"}}}})
	groups := config.GetEventGroups("FakeEvent")
	if len(groups) != 0 {
		t.Error("expected unknown event to be ignored")
	}
}

func TestActiveEventNames_Empty(t *testing.T) {
	names := activeEventNamesFromConfig(&HooksConfig{})
	if len(names) != 0 {
		t.Errorf("expected 0, got %d", len(names))
	}
}

func TestActiveEventNames_Partial(t *testing.T) {
	config := &HooksConfig{
		SessionStart: []HookGroup{{Matcher: "", Hooks: []HookEntry{{Type: "command", Command: "echo"}}}},
	}
	names := activeEventNamesFromConfig(config)
	if len(names) != 1 || names[0] != "SessionStart" {
		t.Errorf("expected [SessionStart], got %v", names)
	}
}

// ---------------------------------------------------------------------------
// resolveHookCoverageContract
// ---------------------------------------------------------------------------

func TestResolveContract(t *testing.T) {
	c := resolveHookCoverageContract()
	if len(c.ActiveEvents) == 0 {
		t.Error("expected at least one active event")
	}
}

// ---------------------------------------------------------------------------
// resolveSourceDir
// ---------------------------------------------------------------------------

func TestResolveSourceDir_InvalidExplicit(t *testing.T) {
	old := hooksSourceDir
	hooksSourceDir = "/nonexistent/repo"
	defer func() { hooksSourceDir = old }()
	_, err := resolveSourceDir()
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestResolveSourceDir_CwdFallback(t *testing.T) {
	old := hooksSourceDir
	hooksSourceDir = ""
	defer func() { hooksSourceDir = old }()
	tmpDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmpDir, "hooks"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "hooks", "hooks.json"), []byte(`{}`), 0644); err != nil {
		t.Fatal(err)
	}
	origDir, _ := os.Getwd()
	_ = os.Chdir(tmpDir)
	defer func() { _ = os.Chdir(origDir) }()
	result, err := resolveSourceDir()
	if err != nil {
		t.Fatalf("resolveSourceDir: %v", err)
	}
	expectedDir, _ := filepath.EvalSymlinks(tmpDir)
	resultResolved, _ := filepath.EvalSymlinks(result)
	if resultResolved != expectedDir {
		t.Errorf("got %q, want %q", resultResolved, expectedDir)
	}
}

func TestResolveSourceDir_NoSource(t *testing.T) {
	old := hooksSourceDir
	hooksSourceDir = ""
	defer func() { hooksSourceDir = old }()
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	_ = os.Chdir(tmpDir)
	defer func() { _ = os.Chdir(origDir) }()
	_, err := resolveSourceDir()
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---------------------------------------------------------------------------
// cloneHooksMap
// ---------------------------------------------------------------------------

func TestCloneHooksMap_NoKey(t *testing.T) {
	result := cloneHooksMap(map[string]any{"other": "val"})
	if len(result) != 0 {
		t.Errorf("expected empty, got %d", len(result))
	}
}

func TestCloneHooksMap_NotMap(t *testing.T) {
	result := cloneHooksMap(map[string]any{"hooks": "string"})
	if len(result) != 0 {
		t.Errorf("expected empty, got %d", len(result))
	}
}

// ---------------------------------------------------------------------------
// backupHooksSettings
// ---------------------------------------------------------------------------

func TestBackupSettings_Nonexistent(t *testing.T) {
	if err := backupHooksSettings("/nonexistent/settings.json"); err != nil {
		t.Fatalf("expected no-op, got: %v", err)
	}
}

func TestBackupSettings_Existing(t *testing.T) {
	dir := t.TempDir()
	sp := filepath.Join(dir, "settings.json")
	os.WriteFile(sp, []byte(`{}`), 0644)
	if err := backupHooksSettings(sp); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(dir)
	found := false
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "settings.json.backup.") {
			found = true
		}
	}
	if !found {
		t.Error("expected backup")
	}
}

// ---------------------------------------------------------------------------
// copyOptionalFile / copyOptionalDir / copyShellScripts / hooksCopyFile
// ---------------------------------------------------------------------------

func TestCopyOptionalFile_Missing(t *testing.T) {
	n, err := copyOptionalFile("/nonexistent", "/dst", "test")
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("expected 0, got %d", n)
	}
}

func TestCopyOptionalFile_Exists(t *testing.T) {
	src := filepath.Join(t.TempDir(), "f.sh")
	dst := filepath.Join(t.TempDir(), "f.sh")
	os.WriteFile(src, []byte("#!/bin/bash"), 0755)
	n, err := copyOptionalFile(src, dst, "test")
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("expected 1, got %d", n)
	}
}

func TestCopyOptionalDir_Missing(t *testing.T) {
	n, err := copyOptionalDir("/nonexistent", "/dst", "test")
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("expected 0, got %d", n)
	}
}

func TestCopyOptionalDir_Exists(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := filepath.Join(t.TempDir(), "dest")
	os.WriteFile(filepath.Join(srcDir, "a.md"), []byte("a"), 0644)
	n, err := copyOptionalDir(srcDir, dstDir, "test")
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("expected 1, got %d", n)
	}
}

func TestCopyShellScripts_Nonexistent(t *testing.T) {
	_, err := copyShellScripts("/nonexistent", "/dst")
	if err == nil {
		t.Fatal("expected error for nonexistent source dir")
	}
}

func TestCopyShellScripts_WithFiles(t *testing.T) {
	srcDir := t.TempDir()
	os.WriteFile(filepath.Join(srcDir, "a.sh"), []byte("#!/bin/bash\n"), 0755)
	os.WriteFile(filepath.Join(srcDir, "b.sh"), []byte("#!/bin/bash\n"), 0755)
	os.WriteFile(filepath.Join(srcDir, "c.txt"), []byte(""), 0644)
	n, err := copyShellScripts(srcDir, filepath.Join(t.TempDir(), "dst"))
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Errorf("expected 2, got %d", n)
	}
}

func TestHooksCopyFile_Missing(t *testing.T) {
	err := hooksCopyFile("/nonexistent", "/dst")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestHooksCopyFile_OK(t *testing.T) {
	src := filepath.Join(t.TempDir(), "f.sh")
	os.WriteFile(src, []byte("data"), 0755)
	dst := filepath.Join(t.TempDir(), "sub", "f.sh")
	if err := hooksCopyFile(src, dst); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(dst)
	if string(data) != "data" {
		t.Error("content mismatch")
	}
}

// ---------------------------------------------------------------------------
// installFullHooks / installFullHooksFromEmbed / installFullHookScripts
// ---------------------------------------------------------------------------

func TestInstallFullHooks_NoGit(t *testing.T) {
	_, err := installFullHooks(t.TempDir(), t.TempDir())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestInstallFullHooks_WithGit(t *testing.T) {
	srcDir := t.TempDir()
	os.MkdirAll(filepath.Join(srcDir, ".git"), 0755)
	hDir := filepath.Join(srcDir, "hooks")
	os.MkdirAll(hDir, 0755)
	os.WriteFile(filepath.Join(hDir, "test.sh"), []byte("#!/bin/bash\n"), 0755)
	n, err := installFullHooks(srcDir, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if n < 1 {
		t.Errorf("expected >= 1, got %d", n)
	}
}

func TestInstallFromEmbed(t *testing.T) {
	n, err := installFullHooksFromEmbed(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if n == 0 {
		t.Error("expected > 0")
	}
}

func TestInstallFullHookScripts_Embedded(t *testing.T) {
	old := hooksSourceDir
	oldDry := hooksDryRun
	hooksSourceDir = ""
	hooksDryRun = false
	defer func() { hooksSourceDir = old; hooksDryRun = oldDry }()
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	_ = os.Chdir(tmpDir)
	defer func() { _ = os.Chdir(origDir) }()
	old2 := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w
	err := installFullHookScripts(t.TempDir())
	_ = w.Close()
	os.Stdout = old2
	if err != nil {
		t.Fatalf("installFullHookScripts: %v", err)
	}
}

func TestInstallFullHookScripts_DryRunNoRepo(t *testing.T) {
	old := hooksSourceDir
	oldDry := hooksDryRun
	hooksSourceDir = ""
	hooksDryRun = true
	defer func() { hooksSourceDir = old; hooksDryRun = oldDry }()
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	_ = os.Chdir(tmpDir)
	defer func() { _ = os.Chdir(origDir) }()
	old2 := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	err := installFullHookScripts(t.TempDir())
	_ = w.Close()
	os.Stdout = old2
	if err != nil {
		t.Fatalf("dry-run: %v", err)
	}
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	if !strings.Contains(buf.String(), "embedded") {
		t.Error("expected embedded message")
	}
}

func TestInstallFullHookScripts_DryRunWithRepo(t *testing.T) {
	srcDir := t.TempDir()
	os.MkdirAll(filepath.Join(srcDir, "hooks"), 0755)
	os.WriteFile(filepath.Join(srcDir, "hooks", "hooks.json"), []byte("{}"), 0644)
	old := hooksSourceDir
	oldDry := hooksDryRun
	hooksSourceDir = srcDir
	hooksDryRun = true
	defer func() { hooksSourceDir = old; hooksDryRun = oldDry }()
	old2 := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w
	err := installFullHookScripts(t.TempDir())
	_ = w.Close()
	os.Stdout = old2
	if err != nil {
		t.Fatalf("dry-run with repo: %v", err)
	}
}

func TestInstallFullHookScripts_WithRepo(t *testing.T) {
	srcDir := t.TempDir()
	os.MkdirAll(filepath.Join(srcDir, ".git"), 0755)
	hDir := filepath.Join(srcDir, "hooks")
	os.MkdirAll(hDir, 0755)
	os.WriteFile(filepath.Join(hDir, "hooks.json"), []byte("{}"), 0644)
	os.WriteFile(filepath.Join(hDir, "test.sh"), []byte("#!/bin/bash\n"), 0755)
	old := hooksSourceDir
	oldDry := hooksDryRun
	hooksSourceDir = srcDir
	hooksDryRun = false
	defer func() { hooksSourceDir = old; hooksDryRun = oldDry }()
	old2 := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w
	err := installFullHookScripts(t.TempDir())
	_ = w.Close()
	os.Stdout = old2
	if err != nil {
		t.Fatalf("with repo: %v", err)
	}
}

// ---------------------------------------------------------------------------
// loadHooksSettings
// ---------------------------------------------------------------------------

func TestLoadSettings_Valid(t *testing.T) {
	sp := filepath.Join(t.TempDir(), "settings.json")
	os.WriteFile(sp, []byte(`{"hooks": {}}`), 0644)
	r, err := loadHooksSettings(sp)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := r["hooks"]; !ok {
		t.Error("expected hooks")
	}
}

func TestLoadSettings_BadJSON(t *testing.T) {
	sp := filepath.Join(t.TempDir(), "settings.json")
	os.WriteFile(sp, []byte("bad"), 0644)
	_, err := loadHooksSettings(sp)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestLoadSettings_Missing(t *testing.T) {
	r, err := loadHooksSettings("/nonexistent")
	if err != nil {
		t.Fatal(err)
	}
	if len(r) != 0 {
		t.Error("expected empty")
	}
}

// ---------------------------------------------------------------------------
// writeHooksSettings
// ---------------------------------------------------------------------------

func TestWriteSettings_CreatesDir(t *testing.T) {
	sp := filepath.Join(t.TempDir(), ".claude", "settings.json")
	err := writeHooksSettings(sp, map[string]any{"hooks": map[string]any{}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(sp); err != nil {
		t.Error("expected file created")
	}
}

func TestWriteSettings_Valid(t *testing.T) {
	sp := filepath.Join(t.TempDir(), "settings.json")
	err := writeHooksSettings(sp, map[string]any{"hooks": map[string]any{}, "tools": []string{"Read"}})
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(sp)
	var p map[string]any
	if err := json.Unmarshal(data, &p); err != nil {
		t.Fatal(err)
	}
	if _, ok := p["hooks"]; !ok {
		t.Error("expected hooks")
	}
}

// ---------------------------------------------------------------------------
// dryRunPrintSettings
// ---------------------------------------------------------------------------

func TestDryRun_Off(t *testing.T) {
	oldVal := hooksDryRun
	hooksDryRun = false
	defer func() { hooksDryRun = oldVal }()
	done, err := dryRunPrintSettings("/fake", map[string]any{})
	if err != nil || done {
		t.Error("expected (false, nil)")
	}
}

func TestDryRun_On(t *testing.T) {
	oldVal := hooksDryRun
	hooksDryRun = true
	defer func() { hooksDryRun = oldVal }()
	old := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w
	done, err := dryRunPrintSettings("/fake", map[string]any{})
	_ = w.Close()
	os.Stdout = old
	if err != nil || !done {
		t.Error("expected (true, nil)")
	}
}

// ---------------------------------------------------------------------------
// generateHooksForInstall / generateFullHooksConfig / generateHooksConfig
// ---------------------------------------------------------------------------

func TestGenerateForInstall(t *testing.T) {
	h, e, err := generateHooksForInstall(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if h == nil || len(e) == 0 {
		t.Error("expected hooks and events")
	}
}

func TestGenerateHooksConfig_NonNil(t *testing.T) {
	c := generateHooksConfig()
	if c == nil {
		t.Fatal("expected non-nil")
	}
}

func TestGenerateFullConfig(t *testing.T) {
	config, err := generateFullHooksConfig()
	if err != nil {
		t.Fatalf("generateFullHooksConfig: %v", err)
	}
	if config == nil {
		t.Fatal("expected non-nil")
	}
}

// ---------------------------------------------------------------------------
// ReadHooksManifest
// ---------------------------------------------------------------------------

func TestReadManifest_MissingKey(t *testing.T) {
	_, err := ReadHooksManifest([]byte(`{"version": 1}`))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestReadManifest_BadJSON(t *testing.T) {
	_, err := ReadHooksManifest([]byte("bad"))
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---------------------------------------------------------------------------
// commitHooksSettings
// ---------------------------------------------------------------------------

func TestCommitSettings(t *testing.T) {
	sp := filepath.Join(t.TempDir(), "settings.json")
	os.WriteFile(sp, []byte(`{"old": true}`), 0644)
	hooks := generateHooksConfig()
	events := activeEventNamesFromConfig(hooks)
	old := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w
	err := commitHooksSettings(sp, map[string]any{"hooks": map[string]any{}}, hooks, events, len(events))
	_ = w.Close()
	os.Stdout = old
	if err != nil {
		t.Fatalf("commitHooksSettings: %v", err)
	}
	dir := filepath.Dir(sp)
	entries, _ := os.ReadDir(dir)
	foundBackup := false
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "settings.json.backup.") {
			foundBackup = true
		}
	}
	if !foundBackup {
		t.Error("expected backup")
	}
}

// ---------------------------------------------------------------------------
// formatLegacyPreservationReport
// ---------------------------------------------------------------------------

func TestFormatLegacyReport(t *testing.T) {
	report := formatLegacyPreservationReport([]string{"PreToolUse"})
	if !strings.Contains(report, "PreToolUse") {
		t.Error("expected PreToolUse")
	}
}

// ---------------------------------------------------------------------------
// countInstalledEventsForList
// ---------------------------------------------------------------------------

func TestCountInstalledEventsForList_Empty(t *testing.T) {
	c := countInstalledEventsForList(map[string]any{}, AllEventNames())
	if c != 0 {
		t.Errorf("expected 0, got %d", c)
	}
}

func TestCountInstalledEventsForList_NonArray(t *testing.T) {
	c := countInstalledEventsForList(map[string]any{"SessionStart": "not array"}, AllEventNames())
	if c != 0 {
		t.Errorf("expected 0, got %d", c)
	}
}

// ---------------------------------------------------------------------------
// collectLegacyAoManagedEvents
// ---------------------------------------------------------------------------

func TestCollectLegacy_AllActive(t *testing.T) {
	// When an ao-managed event IS in the active list, it's not legacy
	hooksMap := map[string]any{
		"SessionStart": []any{
			map[string]any{"matcher": "", "hooks": []any{
				map[string]any{"type": "command", "command": "bash ~/.agentops/hooks/session-start.sh"},
			}},
		},
	}
	result := collectLegacyAoManagedEvents(hooksMap, []string{"SessionStart"})
	if len(result) != 0 {
		t.Errorf("expected 0 legacy events (SessionStart is active), got %d", len(result))
	}
}

func TestCollectLegacy_NoAo(t *testing.T) {
	hooksMap := map[string]any{
		"PreToolUse": []any{
			map[string]any{"matcher": "", "hooks": []any{
				map[string]any{"type": "command", "command": "echo custom"},
			}},
		},
	}
	result := collectLegacyAoManagedEvents(hooksMap, []string{"PreToolUse"})
	if len(result) != 0 {
		t.Errorf("expected 0, got %d", len(result))
	}
}

// ---------------------------------------------------------------------------
// collectScriptNames
// ---------------------------------------------------------------------------

func TestCollectScriptNames_Present(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.sh"), []byte(""), 0644)
	os.WriteFile(filepath.Join(dir, "b.txt"), []byte(""), 0644)
	names := collectScriptNames(dir)
	if len(names) != 1 {
		t.Errorf("expected 1, got %d", len(names))
	}
}

func TestCollectScriptNames_Missing(t *testing.T) {
	if len(collectScriptNames("/nonexistent")) != 0 {
		t.Error("expected empty")
	}
}

// ---------------------------------------------------------------------------
// readSettingsHooksMap
// ---------------------------------------------------------------------------

func TestReadSettingsHooksMap_Missing(t *testing.T) {
	old := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w
	r, err := readSettingsHooksMap("/nonexistent")
	_ = w.Close()
	os.Stdout = old
	if err != nil || r != nil {
		t.Error("expected (nil, nil)")
	}
}

func TestReadSettingsHooksMap_BadJSON(t *testing.T) {
	sp := filepath.Join(t.TempDir(), "s.json")
	os.WriteFile(sp, []byte("bad"), 0644)
	_, err := readSettingsHooksMap(sp)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestReadSettingsHooksMap_NoKey(t *testing.T) {
	sp := filepath.Join(t.TempDir(), "s.json")
	os.WriteFile(sp, []byte(`{"other": 1}`), 0644)
	old := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w
	r, err := readSettingsHooksMap(sp)
	_ = w.Close()
	os.Stdout = old
	if err != nil || r != nil {
		t.Error("expected (nil, nil)")
	}
}

func TestReadSettingsHooksMap_NotMap(t *testing.T) {
	sp := filepath.Join(t.TempDir(), "s.json")
	os.WriteFile(sp, []byte(`{"hooks": "str"}`), 0644)
	r, err := readSettingsHooksMap(sp)
	if err != nil || r != nil {
		t.Error("expected (nil, nil)")
	}
}

// ---------------------------------------------------------------------------
// loadHooksMap
// ---------------------------------------------------------------------------

func TestLoadHooksMap_BadJSON(t *testing.T) {
	sp := filepath.Join(t.TempDir(), "s.json")
	os.WriteFile(sp, []byte("bad"), 0644)
	_, err := loadHooksMap(sp)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestLoadHooksMap_NoKey(t *testing.T) {
	sp := filepath.Join(t.TempDir(), "s.json")
	os.WriteFile(sp, []byte(`{"other": 1}`), 0644)
	old := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w
	r, err := loadHooksMap(sp)
	_ = w.Close()
	os.Stdout = old
	if err != nil || r != nil {
		t.Error("expected (nil, nil)")
	}
}

func TestLoadHooksMap_NotMap(t *testing.T) {
	sp := filepath.Join(t.TempDir(), "s.json")
	os.WriteFile(sp, []byte(`{"hooks": 42}`), 0644)
	old := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w
	r, err := loadHooksMap(sp)
	_ = w.Close()
	os.Stdout = old
	if err != nil || r != nil {
		t.Error("expected (nil, nil)")
	}
}

func TestLoadHooksMap_Missing(t *testing.T) {
	old := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w
	r, err := loadHooksMap("/nonexistent")
	_ = w.Close()
	os.Stdout = old
	if err != nil || r != nil {
		t.Error("expected (nil, nil)")
	}
}

// ---------------------------------------------------------------------------
// existingAoHooksBlock
// ---------------------------------------------------------------------------

func TestExistingAoBlock_True(t *testing.T) {
	s := map[string]any{
		"hooks": map[string]any{
			"SessionStart": []any{
				map[string]any{"matcher": "", "hooks": []any{
					map[string]any{"type": "command", "command": "bash ~/.agentops/hooks/session-start.sh"},
				}},
			},
		},
	}
	if !existingAoHooksBlock(s) {
		t.Error("expected true")
	}
}

func TestExistingAoBlock_False(t *testing.T) {
	if existingAoHooksBlock(map[string]any{}) {
		t.Error("expected false")
	}
}

// ---------------------------------------------------------------------------
// runHooksInit
// ---------------------------------------------------------------------------

func TestRunHooksInit_Shell(t *testing.T) {
	oldFmt := hooksOutputFormat
	hooksOutputFormat = "shell"
	defer func() { hooksOutputFormat = oldFmt }()
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	err := runHooksInit(nil, nil)
	_ = w.Close()
	os.Stdout = old
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	if !strings.Contains(buf.String(), "SessionStart") {
		t.Error("expected SessionStart")
	}
}

// ---------------------------------------------------------------------------
// runHooksInstall
// ---------------------------------------------------------------------------

func TestRunHooksInstall_Fresh(t *testing.T) {
	oldDry := hooksDryRun
	oldForce := hooksForce
	oldFull := hooksFull
	hooksDryRun = false
	hooksForce = false
	hooksFull = false
	defer func() { hooksDryRun = oldDry; hooksForce = oldForce; hooksFull = oldFull }()
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	old := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w
	err := runHooksInstall(nil, nil)
	_ = w.Close()
	os.Stdout = old
	if err != nil {
		t.Fatalf("runHooksInstall: %v", err)
	}
}

func TestRunHooksInstall_AlreadyInstalled(t *testing.T) {
	oldDry := hooksDryRun
	oldForce := hooksForce
	oldFull := hooksFull
	hooksDryRun = false
	hooksForce = false
	hooksFull = false
	defer func() { hooksDryRun = oldDry; hooksForce = oldForce; hooksFull = oldFull }()
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	claudeDir := filepath.Join(tmpHome, ".claude")
	os.MkdirAll(claudeDir, 0755)
	settings := map[string]any{
		"hooks": map[string]any{
			"SessionStart": []any{
				map[string]any{"matcher": "", "hooks": []any{
					map[string]any{"type": "command", "command": "bash ~/.agentops/hooks/session-start.sh"},
				}},
			},
		},
	}
	data, _ := json.MarshalIndent(settings, "", "  ")
	os.WriteFile(filepath.Join(claudeDir, "settings.json"), data, 0644)
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	err := runHooksInstall(nil, nil)
	_ = w.Close()
	os.Stdout = old
	if err != nil {
		t.Fatalf("already installed: %v", err)
	}
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	if !strings.Contains(buf.String(), "already installed") {
		t.Errorf("expected 'already installed', got: %s", buf.String())
	}
}

func TestRunHooksInstall_Force(t *testing.T) {
	oldDry := hooksDryRun
	oldForce := hooksForce
	oldFull := hooksFull
	hooksDryRun = false
	hooksForce = true
	hooksFull = false
	defer func() { hooksDryRun = oldDry; hooksForce = oldForce; hooksFull = oldFull }()
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	claudeDir := filepath.Join(tmpHome, ".claude")
	os.MkdirAll(claudeDir, 0755)
	settings := map[string]any{
		"hooks": map[string]any{
			"SessionStart": []any{
				map[string]any{"matcher": "", "hooks": []any{
					map[string]any{"type": "command", "command": "bash ~/.agentops/hooks/session-start.sh"},
				}},
			},
		},
	}
	data, _ := json.MarshalIndent(settings, "", "  ")
	os.WriteFile(filepath.Join(claudeDir, "settings.json"), data, 0644)
	old := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w
	err := runHooksInstall(nil, nil)
	_ = w.Close()
	os.Stdout = old
	if err != nil {
		t.Fatalf("force install: %v", err)
	}
}

func TestRunHooksInstall_DryRun(t *testing.T) {
	oldDry := hooksDryRun
	oldForce := hooksForce
	oldFull := hooksFull
	hooksDryRun = true
	hooksForce = false
	hooksFull = false
	defer func() { hooksDryRun = oldDry; hooksForce = oldForce; hooksFull = oldFull }()
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	old := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w
	err := runHooksInstall(nil, nil)
	_ = w.Close()
	os.Stdout = old
	if err != nil {
		t.Fatalf("dry-run: %v", err)
	}
}

func TestRunHooksInstall_FullMode(t *testing.T) {
	oldDry := hooksDryRun
	oldForce := hooksForce
	oldFull := hooksFull
	oldSrc := hooksSourceDir
	hooksDryRun = false
	hooksForce = false
	hooksFull = true
	hooksSourceDir = ""
	defer func() { hooksDryRun = oldDry; hooksForce = oldForce; hooksFull = oldFull; hooksSourceDir = oldSrc }()
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	_ = os.Chdir(tmpDir)
	defer func() { _ = os.Chdir(origDir) }()
	old := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w
	err := runHooksInstall(nil, nil)
	_ = w.Close()
	os.Stdout = old
	if err != nil {
		t.Fatalf("full mode: %v", err)
	}
}

func TestRunHooksInstall_ExistingNonAo(t *testing.T) {
	oldDry := hooksDryRun
	oldForce := hooksForce
	oldFull := hooksFull
	hooksDryRun = false
	hooksForce = false
	hooksFull = false
	defer func() { hooksDryRun = oldDry; hooksForce = oldForce; hooksFull = oldFull }()
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	claudeDir := filepath.Join(tmpHome, ".claude")
	os.MkdirAll(claudeDir, 0755)
	settings := map[string]any{
		"hooks": map[string]any{
			"PreToolUse": []any{
				map[string]any{"matcher": "Write", "hooks": []any{
					map[string]any{"type": "command", "command": "echo custom"},
				}},
			},
		},
	}
	data, _ := json.MarshalIndent(settings, "", "  ")
	os.WriteFile(filepath.Join(claudeDir, "settings.json"), data, 0644)
	old := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w
	err := runHooksInstall(nil, nil)
	_ = w.Close()
	os.Stdout = old
	if err != nil {
		t.Fatalf("existing non-ao: %v", err)
	}
}

// ---------------------------------------------------------------------------
// runHooksShow
// ---------------------------------------------------------------------------

func TestRunHooksShow_Full(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	claudeDir := filepath.Join(tmpHome, ".claude")
	os.MkdirAll(claudeDir, 0755)
	settings := map[string]any{
		"hooks": map[string]any{
			"SessionStart": []any{
				map[string]any{"matcher": "", "hooks": []any{
					map[string]any{"type": "command", "command": "bash ~/.agentops/hooks/session-start.sh"},
				}},
			},
		},
	}
	data, _ := json.MarshalIndent(settings, "", "  ")
	os.WriteFile(filepath.Join(claudeDir, "settings.json"), data, 0644)
	old := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w
	err := runHooksShow(nil, nil)
	_ = w.Close()
	os.Stdout = old
	if err != nil {
		t.Fatalf("runHooksShow: %v", err)
	}
}

func TestRunHooksShow_Missing(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	old := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w
	err := runHooksShow(nil, nil)
	_ = w.Close()
	os.Stdout = old
	if err != nil {
		t.Fatalf("runHooksShow missing: %v", err)
	}
}

// ---------------------------------------------------------------------------
// runForgeTranscriptAccessTest
// ---------------------------------------------------------------------------

func TestForgeTranscriptAccess_WithDir(t *testing.T) {
	origDryRun := hooksDryRun
	hooksDryRun = false
	t.Cleanup(func() { hooksDryRun = origDryRun })

	tmpHome := t.TempDir()
	os.MkdirAll(filepath.Join(tmpHome, ".claude", "projects"), 0755)
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	runForgeTranscriptAccessTest(1, tmpHome)
	_ = w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	if !strings.Contains(buf.String(), "projects directory found") {
		t.Errorf("expected projects found, got: %s", buf.String())
	}
}
