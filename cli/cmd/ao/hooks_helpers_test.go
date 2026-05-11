// practices: [design-by-contract, gitops]
package main

import (
	"testing"
)

// Tests for pure helper functions in hooks.go

func TestIsAoManagedHookCommand(t *testing.T) {
	tests := []struct {
		name string
		cmd  string
		want bool
	}{
		{name: "ao command returns true", cmd: "ao context status", want: true},
		{name: "ao with path returns true", cmd: "/usr/local/bin/ao flywheel", want: true},
		{name: "agentops hooks path returns true", cmd: "/home/user/.agentops/hooks/session-start.sh", want: true},
		{name: "unrelated command returns false", cmd: "git commit -m 'test'", want: false},
		{name: "empty string returns false", cmd: "", want: false},
		{name: "script with ao in path but not command", cmd: "/opt/cacao/run.sh", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isAoManagedHookCommand(tt.cmd)
			if got != tt.want {
				t.Errorf("isAoManagedHookCommand(%q) = %v, want %v", tt.cmd, got, tt.want)
			}
		})
	}
}

func TestHookGroupContainsAo(t *testing.T) {
	t.Run("empty hooks map returns false", func(t *testing.T) {
		got := hookGroupContainsAo(map[string]any{}, "UserPromptSubmit")
		if got {
			t.Error("expected false for empty hooks map")
		}
	})

	t.Run("event not present returns false", func(t *testing.T) {
		hooksMap := map[string]any{
			"PreToolUse": []any{},
		}
		got := hookGroupContainsAo(hooksMap, "UserPromptSubmit")
		if got {
			t.Error("expected false for missing event")
		}
	})

	t.Run("new format with ao command returns true", func(t *testing.T) {
		hooksMap := map[string]any{
			"SessionStart": []any{
				map[string]any{
					"hooks": []any{
						map[string]any{
							"command": "ao context status",
						},
					},
				},
			},
		}
		got := hookGroupContainsAo(hooksMap, "SessionStart")
		if !got {
			t.Error("expected true for new format with ao command")
		}
	})

	t.Run("new format with non-ao command returns false", func(t *testing.T) {
		hooksMap := map[string]any{
			"SessionStart": []any{
				map[string]any{
					"hooks": []any{
						map[string]any{
							"command": "echo hello",
						},
					},
				},
			},
		}
		got := hookGroupContainsAo(hooksMap, "SessionStart")
		if got {
			t.Error("expected false for new format with non-ao command")
		}
	})

	t.Run("legacy format with ao command returns true", func(t *testing.T) {
		hooksMap := map[string]any{
			"SessionStart": []any{
				map[string]any{
					"command": []any{"bash", "ao flywheel status"},
				},
			},
		}
		got := hookGroupContainsAo(hooksMap, "SessionStart")
		if !got {
			t.Error("expected true for legacy format with ao command")
		}
	})

	t.Run("non-array event type returns false", func(t *testing.T) {
		hooksMap := map[string]any{
			"SessionStart": "not-an-array",
		}
		got := hookGroupContainsAo(hooksMap, "SessionStart")
		if got {
			t.Error("expected false for non-array event")
		}
	})
}

func TestMergeHookEvents(t *testing.T) {
	t.Run("installs events from newHooks into hooksMap", func(t *testing.T) {
		hooksMap := map[string]any{}
		newHooks := &HooksConfig{}
		newHooks.SetEventGroups("SessionStart", []HookGroup{
			{Hooks: []HookEntry{{Command: "ao context status"}}},
		})
		newHooks.SetEventGroups("PostToolUse", []HookGroup{
			{Hooks: []HookEntry{{Command: "ao post-tool"}}},
		})
		eventsToInstall := []string{"SessionStart", "PostToolUse"}
		count := mergeHookEvents(hooksMap, newHooks, eventsToInstall)
		if count != 2 {
			t.Errorf("expected 2 installed events, got %d", count)
		}
		if _, ok := hooksMap["SessionStart"]; !ok {
			t.Error("expected SessionStart in hooksMap")
		}
		if _, ok := hooksMap["PostToolUse"]; !ok {
			t.Error("expected PostToolUse in hooksMap")
		}
	})

	t.Run("event not in newHooks is not installed", func(t *testing.T) {
		hooksMap := map[string]any{}
		newHooks := &HooksConfig{} // no events set
		eventsToInstall := []string{"SessionStart"}
		count := mergeHookEvents(hooksMap, newHooks, eventsToInstall)
		if count != 0 {
			t.Errorf("expected 0 installed events, got %d", count)
		}
		if _, ok := hooksMap["SessionStart"]; ok {
			t.Error("expected SessionStart not in hooksMap when no groups")
		}
	})

	t.Run("preserves non-ao existing hooks", func(t *testing.T) {
		hooksMap := map[string]any{
			"SessionStart": []any{
				map[string]any{
					"hooks": []any{
						map[string]any{"command": "echo hello"},
					},
				},
			},
		}
		newHooks := &HooksConfig{}
		newHooks.SetEventGroups("SessionStart", []HookGroup{
			{Hooks: []HookEntry{{Command: "ao context status"}}},
		})
		count := mergeHookEvents(hooksMap, newHooks, []string{"SessionStart"})
		if count != 1 {
			t.Errorf("expected 1 installed event, got %d", count)
		}
		// The merged groups should contain both existing and new
		// mergeHookEvents sets hooksMap[event] = []map[string]any
		groups, ok := hooksMap["SessionStart"].([]map[string]any)
		if !ok {
			t.Fatalf("expected []map[string]any for SessionStart, got %T", hooksMap["SessionStart"])
		}
		// Should have at least 2 groups: original + new
		if len(groups) < 2 {
			t.Errorf("expected at least 2 groups after merge, got %d", len(groups))
		}
	})
}
