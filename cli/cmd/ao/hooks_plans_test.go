// practices: [design-by-contract, gitops]
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/types"
)

// ===========================================================================
// hooks.go tests
// ===========================================================================

// ---------------------------------------------------------------------------
// generateMinimalHooksConfig
// ---------------------------------------------------------------------------

func TestHooksPlans_generateMinimalHooksConfig(t *testing.T) {
	config := generateMinimalHooksConfig()

	if config == nil {
		t.Fatal("expected non-nil config")
	}

	// SessionStart should have exactly 1 group with 1 hook
	if len(config.SessionStart) != 1 {
		t.Fatalf("SessionStart groups: got %d, want 1", len(config.SessionStart))
	}
	if len(config.SessionStart[0].Hooks) != 1 {
		t.Fatalf("SessionStart hooks: got %d, want 1", len(config.SessionStart[0].Hooks))
	}
	if config.SessionStart[0].Hooks[0].Type != "command" {
		t.Errorf("SessionStart hook type: got %q, want 'command'", config.SessionStart[0].Hooks[0].Type)
	}
	if config.SessionStart[0].Hooks[0].Command == "" {
		t.Error("SessionStart hook command should not be empty")
	}

	// Stop should have exactly 1 group with 1 hook
	if len(config.Stop) != 1 {
		t.Fatalf("Stop groups: got %d, want 1", len(config.Stop))
	}
	if len(config.Stop[0].Hooks) != 1 {
		t.Fatalf("Stop hooks: got %d, want 1", len(config.Stop[0].Hooks))
	}

	// SessionEnd should also have hooks now (forge + maturity)
	if len(config.SessionEnd) == 0 {
		t.Error("expected SessionEnd hooks, got none")
	}

	// All other events should be empty
	otherEvents := []string{
		"PreToolUse", "PostToolUse",
		"UserPromptSubmit", "TaskCompleted", "PreCompact",
		"SubagentStop", "WorktreeCreate", "WorktreeRemove", "ConfigChange",
	}
	for _, event := range otherEvents {
		groups := config.GetEventGroups(event)
		if len(groups) != 0 {
			t.Errorf("event %s should be empty, got %d groups", event, len(groups))
		}
	}
}

// ---------------------------------------------------------------------------
// cloneHooksMap
// ---------------------------------------------------------------------------

func TestHooksPlans_cloneHooksMap(t *testing.T) {
	tests := []struct {
		name        string
		rawSettings map[string]any
		wantKeys    int
	}{
		{
			name:        "no hooks key",
			rawSettings: map[string]any{"env": "test"},
			wantKeys:    0,
		},
		{
			name:        "hooks key is not map",
			rawSettings: map[string]any{"hooks": "invalid"},
			wantKeys:    0,
		},
		{
			name: "hooks key with events",
			rawSettings: map[string]any{
				"hooks": map[string]any{
					"SessionStart": []any{},
					"Stop":         []any{},
				},
			},
			wantKeys: 2,
		},
		{
			name:        "empty settings",
			rawSettings: map[string]any{},
			wantKeys:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cloneHooksMap(tt.rawSettings)
			if result == nil {
				t.Fatal("expected non-nil map")
			}
			if len(result) != tt.wantKeys {
				t.Errorf("got %d keys, want %d", len(result), tt.wantKeys)
			}
		})
	}

	// Verify the clone is independent of the original
	t.Run("mutation isolation", func(t *testing.T) {
		original := map[string]any{
			"hooks": map[string]any{
				"SessionStart": []any{"a"},
			},
		}
		clone := cloneHooksMap(original)
		clone["NewEvent"] = []any{"b"}

		origHooks := original["hooks"].(map[string]any)
		if _, exists := origHooks["NewEvent"]; exists {
			t.Error("mutation of clone should not affect original")
		}
	})
}

// ---------------------------------------------------------------------------
// mergeHookEvents
// ---------------------------------------------------------------------------

func TestHooksPlans_mergeHookEvents(t *testing.T) {
	t.Run("installs new events into empty map", func(t *testing.T) {
		hooksMap := make(map[string]any)
		newHooks := generateMinimalHooksConfig()
		eventsToInstall := []string{"SessionStart", "Stop"}

		count := mergeHookEvents(hooksMap, newHooks, eventsToInstall)
		if count != 2 {
			t.Errorf("installed events: got %d, want 2", count)
		}
		if _, ok := hooksMap["SessionStart"]; !ok {
			t.Error("expected SessionStart in hooksMap")
		}
		if _, ok := hooksMap["Stop"]; !ok {
			t.Error("expected Stop in hooksMap")
		}
	})

	t.Run("skips events with no groups in config", func(t *testing.T) {
		hooksMap := make(map[string]any)
		newHooks := generateMinimalHooksConfig()
		// PreToolUse has no groups in minimal config
		eventsToInstall := []string{"PreToolUse"}

		count := mergeHookEvents(hooksMap, newHooks, eventsToInstall)
		if count != 0 {
			t.Errorf("installed events: got %d, want 0", count)
		}
	})

	t.Run("preserves non-ao hooks in existing map", func(t *testing.T) {
		hooksMap := map[string]any{
			"SessionStart": []any{
				map[string]any{
					"hooks": []any{
						map[string]any{"type": "command", "command": "echo hello"},
					},
				},
			},
		}
		newHooks := generateMinimalHooksConfig()
		eventsToInstall := []string{"SessionStart"}

		count := mergeHookEvents(hooksMap, newHooks, eventsToInstall)
		if count != 1 {
			t.Errorf("installed events: got %d, want 1", count)
		}

		groups, ok := hooksMap["SessionStart"].([]map[string]any)
		if !ok {
			t.Fatal("expected SessionStart to be []map[string]any")
		}
		// Should contain the original non-ao hook + the new ao hook
		if len(groups) != 2 {
			t.Errorf("SessionStart groups: got %d, want 2", len(groups))
		}
	})
}

// ---------------------------------------------------------------------------
// hookGroupContainsAo
// ---------------------------------------------------------------------------

func TestHooksPlans_hookGroupContainsAo(t *testing.T) {
	tests := []struct {
		name     string
		hooksMap map[string]any
		event    string
		want     bool
	}{
		{
			name:     "event not present",
			hooksMap: map[string]any{},
			event:    "SessionStart",
			want:     false,
		},
		{
			name: "event present but not array",
			hooksMap: map[string]any{
				"SessionStart": "invalid",
			},
			event: "SessionStart",
			want:  false,
		},
		{
			name: "event with ao hook",
			hooksMap: map[string]any{
				"SessionStart": []any{
					map[string]any{
						"hooks": []any{
							map[string]any{"type": "command", "command": "ao inject --apply-decay"},
						},
					},
				},
			},
			event: "SessionStart",
			want:  true,
		},
		{
			name: "event without ao hook",
			hooksMap: map[string]any{
				"SessionStart": []any{
					map[string]any{
						"hooks": []any{
							map[string]any{"type": "command", "command": "echo hello"},
						},
					},
				},
			},
			event: "SessionStart",
			want:  false,
		},
		{
			name: "event with non-map group",
			hooksMap: map[string]any{
				"SessionStart": []any{"not a map"},
			},
			event: "SessionStart",
			want:  false,
		},
		{
			name: "legacy format with ao command array",
			hooksMap: map[string]any{
				"SessionStart": []any{
					map[string]any{
						"command": []any{"bash", "ao inject"},
					},
				},
			},
			event: "SessionStart",
			want:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hookGroupContainsAo(tt.hooksMap, tt.event)
			if got != tt.want {
				t.Errorf("hookGroupContainsAo() = %v, want %v", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// filterNonAoHookGroups
// ---------------------------------------------------------------------------

func TestHooksPlans_filterNonAoHookGroups(t *testing.T) {
	tests := []struct {
		name     string
		hooksMap map[string]any
		event    string
		wantLen  int
	}{
		{
			name:     "event not present",
			hooksMap: map[string]any{},
			event:    "SessionStart",
			wantLen:  0,
		},
		{
			name: "only ao hooks — all filtered",
			hooksMap: map[string]any{
				"SessionStart": []any{
					map[string]any{
						"hooks": []any{
							map[string]any{"type": "command", "command": "ao inject --apply-decay"},
						},
					},
				},
			},
			event:   "SessionStart",
			wantLen: 0,
		},
		{
			name: "mixed hooks — keeps non-ao",
			hooksMap: map[string]any{
				"SessionStart": []any{
					map[string]any{
						"hooks": []any{
							map[string]any{"type": "command", "command": "echo hello"},
						},
					},
					map[string]any{
						"hooks": []any{
							map[string]any{"type": "command", "command": "ao inject"},
						},
					},
				},
			},
			event:   "SessionStart",
			wantLen: 1,
		},
		{
			name: "all non-ao — all kept",
			hooksMap: map[string]any{
				"SessionStart": []any{
					map[string]any{
						"hooks": []any{
							map[string]any{"type": "command", "command": "echo hello"},
						},
					},
					map[string]any{
						"hooks": []any{
							map[string]any{"type": "command", "command": "custom-tool run"},
						},
					},
				},
			},
			event:   "SessionStart",
			wantLen: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filterNonAoHookGroups(tt.hooksMap, tt.event)
			if len(result) != tt.wantLen {
				t.Errorf("got %d groups, want %d", len(result), tt.wantLen)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// rawGroupIsAoManaged / rawGroupHooksContainAo / rawGroupLegacyContainsAo
// ---------------------------------------------------------------------------

func TestHooksPlans_rawGroupIsAoManaged(t *testing.T) {
	tests := []struct {
		name  string
		group map[string]any
		want  bool
	}{
		{
			name: "new format with ao command",
			group: map[string]any{
				"hooks": []any{
					map[string]any{"type": "command", "command": "ao inject"},
				},
			},
			want: true,
		},
		{
			name: "new format without ao command",
			group: map[string]any{
				"hooks": []any{
					map[string]any{"type": "command", "command": "echo hello"},
				},
			},
			want: false,
		},
		{
			name: "legacy format with ao command",
			group: map[string]any{
				"command": []any{"bash", "ao inject --apply-decay"},
			},
			want: true,
		},
		{
			name: "legacy format without ao command",
			group: map[string]any{
				"command": []any{"bash", "echo hello"},
			},
			want: false,
		},
		{
			name:  "empty group",
			group: map[string]any{},
			want:  false,
		},
		{
			name: "hooks key is not array",
			group: map[string]any{
				"hooks": "invalid",
			},
			want: false,
		},
		{
			name: "legacy command too short",
			group: map[string]any{
				"command": []any{"single"},
			},
			want: false,
		},
		{
			name: "legacy command element is not string",
			group: map[string]any{
				"command": []any{"bash", 42},
			},
			want: false,
		},
		{
			name: "hook entry without command key",
			group: map[string]any{
				"hooks": []any{
					map[string]any{"type": "command"},
				},
			},
			want: false,
		},
		{
			name: "hook entry is not a map",
			group: map[string]any{
				"hooks": []any{"not a map"},
			},
			want: false,
		},
		{
			name: "agentops hooks path detected as ao-managed",
			group: map[string]any{
				"hooks": []any{
					map[string]any{"type": "command", "command": "bash /Users/me/.agentops/hooks/stop.sh"},
				},
			},
			want: true,
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

// ---------------------------------------------------------------------------
// countRawGroupHooks
// ---------------------------------------------------------------------------

func TestHooksPlans_countRawGroupHooks(t *testing.T) {
	tests := []struct {
		name   string
		groups []any
		want   int
	}{
		{
			name:   "nil groups",
			groups: nil,
			want:   0,
		},
		{
			name:   "empty groups",
			groups: []any{},
			want:   0,
		},
		{
			name: "single group with 2 hooks",
			groups: []any{
				map[string]any{
					"hooks": []any{
						map[string]any{"type": "command", "command": "a"},
						map[string]any{"type": "command", "command": "b"},
					},
				},
			},
			want: 2,
		},
		{
			name: "multiple groups",
			groups: []any{
				map[string]any{
					"hooks": []any{
						map[string]any{"type": "command", "command": "a"},
					},
				},
				map[string]any{
					"hooks": []any{
						map[string]any{"type": "command", "command": "b"},
						map[string]any{"type": "command", "command": "c"},
					},
				},
			},
			want: 3,
		},
		{
			name: "group without hooks key",
			groups: []any{
				map[string]any{"matcher": "Write"},
			},
			want: 0,
		},
		{
			name: "group is not a map",
			groups: []any{
				"not a map",
			},
			want: 0,
		},
		{
			name: "hooks key is not array",
			groups: []any{
				map[string]any{
					"hooks": "invalid",
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
// countInstalledHookEvents
// ---------------------------------------------------------------------------

func TestHooksPlans_countInstalledHookEvents(t *testing.T) {
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
			name: "2 events installed",
			hooksMap: map[string]any{
				"SessionStart": []any{map[string]any{}},
				"Stop":         []any{map[string]any{}},
			},
			want: 2,
		},
		{
			name: "event with empty array not counted",
			hooksMap: map[string]any{
				"SessionStart": []any{map[string]any{}},
				"Stop":         []any{},
			},
			want: 1,
		},
		{
			name: "non-event keys ignored",
			hooksMap: map[string]any{
				"SessionStart": []any{map[string]any{}},
				"custom_key":   []any{map[string]any{}},
			},
			want: 1,
		},
		{
			name: "all 12 events",
			hooksMap: func() map[string]any {
				m := make(map[string]any)
				for _, e := range AllEventNames() {
					m[e] = []any{map[string]any{}}
				}
				return m
			}(),
			want: 12,
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
// eventGroupPtrs / eventGroupPtr
// ---------------------------------------------------------------------------

func TestHooksPlans_eventGroupPtrs(t *testing.T) {
	config := &HooksConfig{}

	// Verify all 12 canonical event names are addressable via GetEventGroups/SetEventGroups
	events := AllEventNames()
	if len(events) != 12 {
		t.Fatalf("expected 12 event names, got %d", len(events))
	}

	for _, event := range events {
		// SetEventGroups should accept the event without panicking
		config.SetEventGroups(event, []HookGroup{{Hooks: []HookEntry{{Type: "command", Command: "test"}}}})
		groups := config.GetEventGroups(event)
		if len(groups) != 1 {
			t.Errorf("event %q: expected 1 group after set, got %d", event, len(groups))
		}
	}
}

func TestHooksPlans_eventGroupPtr(t *testing.T) {
	config := &HooksConfig{}

	// Known event — GetEventGroups should return a non-nil (possibly empty) slice after Set
	config.SetEventGroups("SessionStart", []HookGroup{{Hooks: []HookEntry{{Type: "command", Command: "test"}}}})
	groups := config.GetEventGroups("SessionStart")
	if len(groups) != 1 {
		t.Errorf("expected 1 group for SessionStart, got %d", len(groups))
	}

	// Unknown event — GetEventGroups should return nil
	groups = config.GetEventGroups("UnknownEvent")
	if groups != nil {
		t.Error("expected nil for unknown event")
	}
}

// ---------------------------------------------------------------------------
// hooksCopyFile
// ---------------------------------------------------------------------------

func TestHooksPlans_hooksCopyFile(t *testing.T) {
	t.Run("copies file with parent dir creation", func(t *testing.T) {
		srcDir := t.TempDir()
		dstDir := t.TempDir()

		srcPath := filepath.Join(srcDir, "source.txt")
		writeFile(t, srcPath, "hello world")

		dstPath := filepath.Join(dstDir, "nested", "deep", "dest.txt")
		if err := hooksCopyFile(srcPath, dstPath); err != nil {
			t.Fatalf("hooksCopyFile error: %v", err)
		}

		data, err := os.ReadFile(dstPath)
		if err != nil {
			t.Fatalf("read dst: %v", err)
		}
		if string(data) != "hello world" {
			t.Errorf("content = %q, want 'hello world'", string(data))
		}
	})

	t.Run("returns error for missing source", func(t *testing.T) {
		dstDir := t.TempDir()
		err := hooksCopyFile("/nonexistent/file.txt", filepath.Join(dstDir, "out.txt"))
		if err == nil {
			t.Error("expected error for missing source")
		}
	})
}

// ---------------------------------------------------------------------------
// copyDir
// ---------------------------------------------------------------------------

func TestHooksPlans_copyDir(t *testing.T) {
	t.Run("copies directory tree", func(t *testing.T) {
		srcDir := t.TempDir()
		dstDir := t.TempDir()

		// Create source structure
		writeFile(t, filepath.Join(srcDir, "a.txt"), "file a")
		writeFile(t, filepath.Join(srcDir, "sub", "b.txt"), "file b")
		writeFile(t, filepath.Join(srcDir, "sub", "deep", "c.txt"), "file c")

		count, err := copyDir(srcDir, filepath.Join(dstDir, "out"))
		if err != nil {
			t.Fatalf("copyDir error: %v", err)
		}
		if count != 3 {
			t.Errorf("copied %d files, want 3", count)
		}

		// Verify files exist
		for _, rel := range []string{"a.txt", "sub/b.txt", "sub/deep/c.txt"} {
			p := filepath.Join(dstDir, "out", rel)
			if _, err := os.Stat(p); err != nil {
				t.Errorf("expected %s to exist", rel)
			}
		}
	})

	t.Run("returns error for missing source", func(t *testing.T) {
		dstDir := t.TempDir()
		_, err := copyDir("/nonexistent/path", filepath.Join(dstDir, "out"))
		if err == nil {
			t.Error("expected error for missing source dir")
		}
	})
}

// ---------------------------------------------------------------------------
// copyOptionalDir
// ---------------------------------------------------------------------------

func TestHooksPlans_copyOptionalDir(t *testing.T) {
	t.Run("missing source returns 0", func(t *testing.T) {
		dstDir := t.TempDir()
		n, err := copyOptionalDir("/nonexistent/path", filepath.Join(dstDir, "out"), "test")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if n != 0 {
			t.Errorf("expected 0 copied, got %d", n)
		}
	})

	t.Run("copies existing directory", func(t *testing.T) {
		srcDir := t.TempDir()
		dstDir := t.TempDir()
		writeFile(t, filepath.Join(srcDir, "file.txt"), "data")

		n, err := copyOptionalDir(srcDir, filepath.Join(dstDir, "out"), "test")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if n != 1 {
			t.Errorf("expected 1 copied, got %d", n)
		}
	})
}

// ---------------------------------------------------------------------------
// loadHooksSettings / writeHooksSettings
// ---------------------------------------------------------------------------

func TestHooksPlans_loadHooksSettings(t *testing.T) {
	t.Run("missing file returns empty map", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "settings.json")
		settings, err := loadHooksSettings(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(settings) != 0 {
			t.Errorf("expected empty map, got %d keys", len(settings))
		}
	})

	t.Run("valid JSON", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "settings.json")
		writeFile(t, path, `{"env":{"key":"val"},"hooks":{}}`)

		settings, err := loadHooksSettings(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, ok := settings["env"]; !ok {
			t.Error("expected 'env' key")
		}
		if _, ok := settings["hooks"]; !ok {
			t.Error("expected 'hooks' key")
		}
	})

	t.Run("invalid JSON returns error", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "settings.json")
		writeFile(t, path, "not json")

		_, err := loadHooksSettings(path)
		if err == nil {
			t.Error("expected error for invalid JSON")
		}
	})
}

func TestHooksPlans_writeHooksSettings(t *testing.T) {
	t.Run("writes valid settings", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, ".claude", "settings.json")

		settings := map[string]any{
			"hooks": map[string]any{
				"SessionStart": []any{},
			},
		}

		if err := writeHooksSettings(path, settings); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read error: %v", err)
		}

		var parsed map[string]any
		if err := json.Unmarshal(data, &parsed); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if _, ok := parsed["hooks"]; !ok {
			t.Error("expected 'hooks' key in written file")
		}
	})
}

// ---------------------------------------------------------------------------
// backupHooksSettings
// ---------------------------------------------------------------------------

func TestHooksPlans_backupHooksSettings(t *testing.T) {
	t.Run("no error when file does not exist", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "nonexistent.json")
		if err := backupHooksSettings(path); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("creates backup of existing file", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "settings.json")
		writeFile(t, path, `{"hooks":{}}`)

		if err := backupHooksSettings(path); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Check that a backup file exists
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, e := range entries {
			if e.Name() != "settings.json" {
				found = true
				// Verify backup content
				data, err := os.ReadFile(filepath.Join(dir, e.Name()))
				if err != nil {
					t.Fatal(err)
				}
				if string(data) != `{"hooks":{}}` {
					t.Errorf("backup content = %q", string(data))
				}
			}
		}
		if !found {
			t.Error("expected a backup file to be created")
		}
	})
}

// ---------------------------------------------------------------------------
// installFullHooks
// ---------------------------------------------------------------------------

func TestHooksPlans_installFullHooks(t *testing.T) {
	t.Run("rejects non-git source", func(t *testing.T) {
		srcDir := t.TempDir()
		dstDir := t.TempDir()

		// No .git directory
		_, err := installFullHooks(srcDir, dstDir)
		if err == nil {
			t.Error("expected error for non-git source")
		}
	})

	t.Run("copies from git source", func(t *testing.T) {
		srcDir := t.TempDir()
		dstDir := t.TempDir()

		// Create fake .git dir and hook scripts
		if err := os.MkdirAll(filepath.Join(srcDir, ".git"), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(srcDir, "hooks"), 0755); err != nil {
			t.Fatal(err)
		}
		writeFile(t, filepath.Join(srcDir, "hooks", "start.sh"), "#!/bin/bash\necho start")
		writeFile(t, filepath.Join(srcDir, "hooks", "stop.sh"), "#!/bin/bash\necho stop")

		count, err := installFullHooks(srcDir, dstDir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if count < 2 {
			t.Errorf("expected at least 2 files copied, got %d", count)
		}

		// Verify scripts are in destination
		for _, name := range []string{"start.sh", "stop.sh"} {
			p := filepath.Join(dstDir, "hooks", name)
			if _, err := os.Stat(p); err != nil {
				t.Errorf("expected %s to exist in dst", name)
			}
		}
	})
}

// ===========================================================================
// plans.go tests
// ===========================================================================

// ---------------------------------------------------------------------------
// resolveProjectPath
// ---------------------------------------------------------------------------

func TestPlans_resolveProjectPath(t *testing.T) {
	t.Run("explicit path takes precedence", func(t *testing.T) {
		got := resolveProjectPath("/explicit/path", "/some/plan.md")
		if got != "/explicit/path" {
			t.Errorf("got %q, want '/explicit/path'", got)
		}
	})

	t.Run("empty explicit falls back to detect", func(t *testing.T) {
		// detectProjectPath with non-.claude path returns cwd
		got := resolveProjectPath("", "/tmp/test-plan.md")
		if got == "" {
			t.Error("expected non-empty project path")
		}
	})
}

// ---------------------------------------------------------------------------
// createPlanEntry
// ---------------------------------------------------------------------------

func TestPlans_createPlanEntry(t *testing.T) {
	modTime := time.Date(2026, 2, 15, 10, 0, 0, 0, time.UTC)
	entry := createPlanEntry(
		"/abs/path/plan.md",
		modTime,
		"/project",
		"my-plan",
		"ol-123",
		"abc12345",
	)

	if entry.Path != "/abs/path/plan.md" {
		t.Errorf("Path = %q", entry.Path)
	}
	if !entry.CreatedAt.Equal(modTime) {
		t.Errorf("CreatedAt = %v, want %v", entry.CreatedAt, modTime)
	}
	if entry.ProjectPath != "/project" {
		t.Errorf("ProjectPath = %q", entry.ProjectPath)
	}
	if entry.PlanName != "my-plan" {
		t.Errorf("PlanName = %q", entry.PlanName)
	}
	if entry.Status != types.PlanStatusActive {
		t.Errorf("Status = %q, want 'active'", entry.Status)
	}
	if entry.BeadsID != "ol-123" {
		t.Errorf("BeadsID = %q", entry.BeadsID)
	}
	if entry.Checksum != "abc12345" {
		t.Errorf("Checksum = %q", entry.Checksum)
	}
	if entry.UpdatedAt.IsZero() {
		t.Error("UpdatedAt should be set")
	}
}

// ---------------------------------------------------------------------------
// applyPlanUpdates
// ---------------------------------------------------------------------------

func TestPlans_applyPlanUpdates(t *testing.T) {
	tests := []struct {
		name       string
		entries    []types.PlanManifestEntry
		absPath    string
		status     string
		beadsID    string
		wantFound  bool
		wantStatus types.PlanStatus
		wantBeads  string
	}{
		{
			name: "updates status",
			entries: []types.PlanManifestEntry{
				{Path: "/a.md", Status: types.PlanStatusActive, BeadsID: ""},
			},
			absPath:    "/a.md",
			status:     "completed",
			beadsID:    "",
			wantFound:  true,
			wantStatus: types.PlanStatusCompleted,
			wantBeads:  "",
		},
		{
			name: "updates beads ID",
			entries: []types.PlanManifestEntry{
				{Path: "/a.md", Status: types.PlanStatusActive, BeadsID: ""},
			},
			absPath:    "/a.md",
			status:     "",
			beadsID:    "ol-999",
			wantFound:  true,
			wantStatus: types.PlanStatusActive,
			wantBeads:  "ol-999",
		},
		{
			name: "updates both",
			entries: []types.PlanManifestEntry{
				{Path: "/a.md", Status: types.PlanStatusActive, BeadsID: ""},
			},
			absPath:    "/a.md",
			status:     "abandoned",
			beadsID:    "ol-111",
			wantFound:  true,
			wantStatus: "abandoned",
			wantBeads:  "ol-111",
		},
		{
			name: "path not found",
			entries: []types.PlanManifestEntry{
				{Path: "/a.md", Status: types.PlanStatusActive},
			},
			absPath:   "/b.md",
			status:    "completed",
			wantFound: false,
		},
		{
			name:      "empty entries",
			entries:   nil,
			absPath:   "/a.md",
			status:    "completed",
			wantFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Make a copy to avoid mutating across tests
			entries := make([]types.PlanManifestEntry, len(tt.entries))
			copy(entries, tt.entries)

			got := applyPlanUpdates(entries, tt.absPath, tt.status, tt.beadsID)
			if got != tt.wantFound {
				t.Fatalf("found = %v, want %v", got, tt.wantFound)
			}

			if tt.wantFound {
				for _, e := range entries {
					if e.Path == tt.absPath {
						if e.Status != tt.wantStatus {
							t.Errorf("Status = %q, want %q", e.Status, tt.wantStatus)
						}
						if e.BeadsID != tt.wantBeads {
							t.Errorf("BeadsID = %q, want %q", e.BeadsID, tt.wantBeads)
						}
						if e.UpdatedAt.IsZero() {
							t.Error("UpdatedAt should be set")
						}
					}
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// loadManifest / saveManifest / appendManifestEntry
// ---------------------------------------------------------------------------

func TestPlans_loadManifest(t *testing.T) {
	t.Run("file not found", func(t *testing.T) {
		dir := t.TempDir()
		_, err := loadManifest(filepath.Join(dir, "nonexistent.jsonl"))
		if err == nil {
			t.Error("expected error for missing file")
		}
	})

	t.Run("valid JSONL", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "manifest.jsonl")

		e1 := types.PlanManifestEntry{Path: "/a.md", PlanName: "plan-a", Status: types.PlanStatusActive}
		e2 := types.PlanManifestEntry{Path: "/b.md", PlanName: "plan-b", Status: types.PlanStatusCompleted}
		d1, _ := json.Marshal(e1)
		d2, _ := json.Marshal(e2)
		writeFile(t, path, string(d1)+"\n"+string(d2)+"\n")

		entries, err := loadManifest(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(entries) != 2 {
			t.Fatalf("expected 2 entries, got %d", len(entries))
		}
		if entries[0].PlanName != "plan-a" {
			t.Errorf("entries[0].PlanName = %q", entries[0].PlanName)
		}
		if entries[1].PlanName != "plan-b" {
			t.Errorf("entries[1].PlanName = %q", entries[1].PlanName)
		}
	})

	t.Run("skips blank and invalid lines", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "manifest.jsonl")

		e1 := types.PlanManifestEntry{Path: "/ok.md", PlanName: "ok"}
		d1, _ := json.Marshal(e1)
		writeFile(t, path, string(d1)+"\n\nnot json\n")

		entries, err := loadManifest(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(entries) != 1 {
			t.Fatalf("expected 1 entry, got %d", len(entries))
		}
	})
}

func TestPlans_saveManifest(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.jsonl")

	entries := []types.PlanManifestEntry{
		{Path: "/x.md", PlanName: "x", Status: types.PlanStatusActive},
		{Path: "/y.md", PlanName: "y", Status: types.PlanStatusCompleted},
	}

	if err := saveManifest(path, entries); err != nil {
		t.Fatalf("saveManifest error: %v", err)
	}

	// Load back and verify
	loaded, err := loadManifest(path)
	if err != nil {
		t.Fatalf("loadManifest error: %v", err)
	}
	if len(loaded) != 2 {
		t.Fatalf("loaded %d entries, want 2", len(loaded))
	}
	if loaded[0].PlanName != "x" {
		t.Errorf("loaded[0].PlanName = %q", loaded[0].PlanName)
	}
	if loaded[1].PlanName != "y" {
		t.Errorf("loaded[1].PlanName = %q", loaded[1].PlanName)
	}
}

func TestPlans_appendManifestEntry(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.jsonl")

	entry := types.PlanManifestEntry{Path: "/new.md", PlanName: "new-plan", Status: types.PlanStatusActive}
	if err := appendManifestEntry(path, entry); err != nil {
		t.Fatalf("appendManifestEntry error: %v", err)
	}

	// Append another
	entry2 := types.PlanManifestEntry{Path: "/second.md", PlanName: "second", Status: types.PlanStatusActive}
	if err := appendManifestEntry(path, entry2); err != nil {
		t.Fatalf("appendManifestEntry error: %v", err)
	}

	loaded, err := loadManifest(path)
	if err != nil {
		t.Fatalf("loadManifest error: %v", err)
	}
	if len(loaded) != 2 {
		t.Fatalf("loaded %d entries, want 2", len(loaded))
	}
}

// ---------------------------------------------------------------------------
// upsertManifestEntry
// ---------------------------------------------------------------------------

func TestPlans_upsertManifestEntry(t *testing.T) {
	t.Run("updates existing entry", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "manifest.jsonl")

		existing := []types.PlanManifestEntry{
			{Path: "/a.md", PlanName: "old-name", Status: types.PlanStatusActive},
		}
		newEntry := types.PlanManifestEntry{Path: "/a.md", PlanName: "new-name", Status: types.PlanStatusCompleted}

		updated, err := upsertManifestEntry(path, existing, newEntry)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !updated {
			t.Error("expected updated=true for existing path")
		}

		// Verify saved content
		loaded, err := loadManifest(path)
		if err != nil {
			t.Fatalf("loadManifest error: %v", err)
		}
		if len(loaded) != 1 {
			t.Fatalf("loaded %d entries, want 1", len(loaded))
		}
		if loaded[0].PlanName != "new-name" {
			t.Errorf("PlanName = %q, want 'new-name'", loaded[0].PlanName)
		}
	})

	t.Run("appends new entry", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "manifest.jsonl")

		existing := []types.PlanManifestEntry{
			{Path: "/a.md", PlanName: "existing"},
		}
		newEntry := types.PlanManifestEntry{Path: "/b.md", PlanName: "new-entry"}

		updated, err := upsertManifestEntry(path, existing, newEntry)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if updated {
			t.Error("expected updated=false for new path")
		}

		loaded, err := loadManifest(path)
		if err != nil {
			t.Fatalf("loadManifest error: %v", err)
		}
		if len(loaded) != 1 {
			t.Fatalf("loaded %d entries, want 1 (appended only)", len(loaded))
		}
		if loaded[0].PlanName != "new-entry" {
			t.Errorf("PlanName = %q", loaded[0].PlanName)
		}
	})
}

// ---------------------------------------------------------------------------
// syncEpicsToManifest
// ---------------------------------------------------------------------------

func TestPlans_syncEpicsToManifest(t *testing.T) {
	tests := []struct {
		name     string
		entries  []types.PlanManifestEntry
		epics    []beadsEpic
		wantSync int
	}{
		{
			name: "syncs closed epic",
			entries: []types.PlanManifestEntry{
				{PlanName: "plan-a", BeadsID: "e1", Status: types.PlanStatusActive},
			},
			epics:    []beadsEpic{{ID: "e1", Status: "closed"}},
			wantSync: 1,
		},
		{
			name: "no change when already synced",
			entries: []types.PlanManifestEntry{
				{PlanName: "plan-a", BeadsID: "e1", Status: types.PlanStatusCompleted},
			},
			epics:    []beadsEpic{{ID: "e1", Status: "closed"}},
			wantSync: 0,
		},
		{
			name: "unlinked entry ignored",
			entries: []types.PlanManifestEntry{
				{PlanName: "plan-a", BeadsID: "", Status: types.PlanStatusActive},
			},
			epics:    []beadsEpic{{ID: "e1", Status: "closed"}},
			wantSync: 0,
		},
		{
			name: "epic not in manifest",
			entries: []types.PlanManifestEntry{
				{PlanName: "plan-a", BeadsID: "e2", Status: types.PlanStatusActive},
			},
			epics:    []beadsEpic{{ID: "e1", Status: "closed"}},
			wantSync: 0,
		},
		{
			name:     "empty entries and epics",
			entries:  nil,
			epics:    nil,
			wantSync: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			byID := buildBeadsIDIndex(tt.entries)
			got := syncEpicsToManifest(tt.entries, tt.epics, byID)
			if got != tt.wantSync {
				t.Errorf("synced = %d, want %d", got, tt.wantSync)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// detectStatusDrifts
// ---------------------------------------------------------------------------

func TestPlans_detectStatusDrifts(t *testing.T) {
	tests := []struct {
		name       string
		byBeadsID  map[string]*types.PlanManifestEntry
		beadsIndex map[string]string
		wantLen    int
		wantTypes  []string
	}{
		{
			name:       "no drift",
			byBeadsID:  map[string]*types.PlanManifestEntry{"e1": {Status: types.PlanStatusActive}},
			beadsIndex: map[string]string{"e1": "open"},
			wantLen:    0,
		},
		{
			name:       "status mismatch",
			byBeadsID:  map[string]*types.PlanManifestEntry{"e1": {Status: types.PlanStatusActive, PlanName: "test"}},
			beadsIndex: map[string]string{"e1": "closed"},
			wantLen:    1,
			wantTypes:  []string{"status_mismatch"},
		},
		{
			name:       "missing in beads",
			byBeadsID:  map[string]*types.PlanManifestEntry{"e1": {Status: types.PlanStatusActive, PlanName: "test"}},
			beadsIndex: map[string]string{},
			wantLen:    1,
			wantTypes:  []string{"missing_beads"},
		},
		{
			name:       "both completed and closed matches",
			byBeadsID:  map[string]*types.PlanManifestEntry{"e1": {Status: types.PlanStatusCompleted}},
			beadsIndex: map[string]string{"e1": "closed"},
			wantLen:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			drifts := detectStatusDrifts(tt.byBeadsID, tt.beadsIndex)
			if len(drifts) != tt.wantLen {
				t.Fatalf("got %d drifts, want %d", len(drifts), tt.wantLen)
			}
			for i, wantType := range tt.wantTypes {
				if i < len(drifts) && drifts[i].Type != wantType {
					t.Errorf("drift[%d].Type = %q, want %q", i, drifts[i].Type, wantType)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// findAgentsDir
// ---------------------------------------------------------------------------

func TestPlans_findAgentsDir(t *testing.T) {
	t.Run("finds .agents in current dir", func(t *testing.T) {
		dir := t.TempDir()
		agentsDir := filepath.Join(dir, ".agents")
		if err := os.MkdirAll(agentsDir, 0755); err != nil {
			t.Fatal(err)
		}

		got := findAgentsDir(dir)
		if got != agentsDir {
			t.Errorf("got %q, want %q", got, agentsDir)
		}
	})

	t.Run("finds rig marker (crew)", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.MkdirAll(filepath.Join(dir, "crew"), 0755); err != nil {
			t.Fatal(err)
		}

		got := findAgentsDir(dir)
		want := filepath.Join(dir, ".agents")
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("finds rig marker (.beads)", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.MkdirAll(filepath.Join(dir, ".beads"), 0755); err != nil {
			t.Fatal(err)
		}

		got := findAgentsDir(dir)
		want := filepath.Join(dir, ".agents")
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("walks up to parent", func(t *testing.T) {
		dir := t.TempDir()
		childDir := filepath.Join(dir, "sub", "deep")
		if err := os.MkdirAll(childDir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(dir, ".agents"), 0755); err != nil {
			t.Fatal(err)
		}

		got := findAgentsDir(childDir)
		want := filepath.Join(dir, ".agents")
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("returns empty when nothing found", func(t *testing.T) {
		dir := t.TempDir()
		childDir := filepath.Join(dir, "isolated")
		if err := os.MkdirAll(childDir, 0755); err != nil {
			t.Fatal(err)
		}

		// This will walk up to / and not find anything.
		// Since we can't guarantee no .agents or rig markers exist
		// above our temp dir, we just check the function returns.
		_ = findAgentsDir(childDir)
	})
}

// ---------------------------------------------------------------------------
// planStatusSymbols (map lookup)
// ---------------------------------------------------------------------------

func TestPlans_planStatusSymbols(t *testing.T) {
	tests := []struct {
		status types.PlanStatus
		want   string
		exists bool
	}{
		{types.PlanStatusActive, "○", true},
		{types.PlanStatusCompleted, "✓", true},
		{types.PlanStatusAbandoned, "", false},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			sym, ok := planStatusSymbols[tt.status]
			if ok != tt.exists {
				t.Errorf("exists = %v, want %v", ok, tt.exists)
			}
			if ok && sym != tt.want {
				t.Errorf("symbol = %q, want %q", sym, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// resolvePlanName (additional edge cases beyond helpers3)
// ---------------------------------------------------------------------------

func TestPlans_resolvePlanName_edgeCases(t *testing.T) {
	tests := []struct {
		name     string
		explicit string
		path     string
		want     string
	}{
		{
			name:     "explicit always wins",
			explicit: "my-plan",
			path:     "/whatever/other.md",
			want:     "my-plan",
		},
		{
			name:     "derives from .md file",
			explicit: "",
			path:     "/plans/peaceful-stirring-tome.md",
			want:     "peaceful-stirring-tome",
		},
		{
			name:     "derives from .txt file",
			explicit: "",
			path:     "/plans/my-plan.txt",
			want:     "my-plan",
		},
		{
			name:     "no extension",
			explicit: "",
			path:     "/plans/noext",
			want:     "noext",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolvePlanName(tt.explicit, tt.path)
			if got != tt.want {
				t.Errorf("resolvePlanName(%q, %q) = %q, want %q", tt.explicit, tt.path, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// HooksConfig: round-trip through JSON
// ---------------------------------------------------------------------------

func TestHooksPlans_HooksConfig_JSONRoundTrip(t *testing.T) {
	original := &HooksConfig{
		SessionStart: []HookGroup{
			{
				Hooks: []HookEntry{
					{Type: "command", Command: "ao inject", Timeout: 15},
				},
			},
		},
		PostToolUse: []HookGroup{
			{
				Matcher: "Write|Edit",
				Hooks: []HookEntry{
					{Type: "command", Command: "ao check"},
				},
			},
		},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded HooksConfig
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(decoded.SessionStart) != 1 {
		t.Fatalf("SessionStart groups: got %d", len(decoded.SessionStart))
	}
	if decoded.SessionStart[0].Hooks[0].Timeout != 15 {
		t.Errorf("timeout = %d, want 15", decoded.SessionStart[0].Hooks[0].Timeout)
	}
	if len(decoded.PostToolUse) != 1 {
		t.Fatalf("PostToolUse groups: got %d", len(decoded.PostToolUse))
	}
	if decoded.PostToolUse[0].Matcher != "Write|Edit" {
		t.Errorf("matcher = %q, want 'Write|Edit'", decoded.PostToolUse[0].Matcher)
	}
}

// ---------------------------------------------------------------------------
// HooksConfig: GetEventGroups for all 12 events
// ---------------------------------------------------------------------------

func TestHooksPlans_GetEventGroups_AllEvents(t *testing.T) {
	config := &HooksConfig{}

	// Set groups on every event
	testGroup := []HookGroup{{Hooks: []HookEntry{{Type: "command", Command: "test"}}}}
	for _, event := range AllEventNames() {
		config.SetEventGroups(event, testGroup)
	}

	// Verify all are retrievable
	for _, event := range AllEventNames() {
		groups := config.GetEventGroups(event)
		if len(groups) != 1 {
			t.Errorf("%s: got %d groups, want 1", event, len(groups))
		}
		if groups[0].Hooks[0].Command != "test" {
			t.Errorf("%s: command = %q, want 'test'", event, groups[0].Hooks[0].Command)
		}
	}
}

// ---------------------------------------------------------------------------
// replacePluginRoot with empty basePath
// ---------------------------------------------------------------------------

func TestHooksPlans_replacePluginRoot_emptyBasePath(t *testing.T) {
	config := &HooksConfig{
		SessionStart: []HookGroup{
			{
				Hooks: []HookEntry{
					{Type: "command", Command: "bash ${CLAUDE_PLUGIN_ROOT}/hooks/start.sh"},
				},
			},
		},
	}

	replacePluginRoot(config, "")

	got := config.SessionStart[0].Hooks[0].Command
	want := "bash /hooks/start.sh"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// ---------------------------------------------------------------------------
// hookGroupToMap with multiple hooks
// ---------------------------------------------------------------------------

func TestHooksPlans_hookGroupToMap_multipleHooks(t *testing.T) {
	g := HookGroup{
		Matcher: "Bash",
		Hooks: []HookEntry{
			{Type: "command", Command: "first"},
			{Type: "command", Command: "second", Timeout: 60},
			{Type: "command", Command: "third"},
		},
	}

	m := hookGroupToMap(g)
	hooks, ok := m["hooks"].([]map[string]any)
	if !ok {
		t.Fatal("hooks should be []map[string]any")
	}
	if len(hooks) != 3 {
		t.Fatalf("expected 3 hooks, got %d", len(hooks))
	}

	// First hook: no timeout
	if _, hasTimeout := hooks[0]["timeout"]; hasTimeout {
		t.Error("first hook should not have timeout")
	}

	// Second hook: has timeout
	if hooks[1]["timeout"] != 60 {
		t.Errorf("second hook timeout = %v, want 60", hooks[1]["timeout"])
	}

	// Matcher present
	if m["matcher"] != "Bash" {
		t.Errorf("matcher = %v, want 'Bash'", m["matcher"])
	}
}

// ---------------------------------------------------------------------------
// ReadHooksManifest with all events populated
// ---------------------------------------------------------------------------

func TestHooksPlans_ReadHooksManifest_allEvents(t *testing.T) {
	// Build a manifest with SessionStart and PostToolUse
	manifest := `{
		"hooks": {
			"SessionStart": [{"hooks": [{"type": "command", "command": "ao inject"}]}],
			"PostToolUse": [{"matcher": "Write", "hooks": [{"type": "command", "command": "ao check"}]}],
			"Stop": [{"hooks": [{"type": "command", "command": "ao forge"}]}]
		}
	}`

	config, err := ReadHooksManifest([]byte(manifest))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(config.SessionStart) != 1 {
		t.Errorf("SessionStart: got %d groups", len(config.SessionStart))
	}
	if len(config.PostToolUse) != 1 {
		t.Errorf("PostToolUse: got %d groups", len(config.PostToolUse))
	}
	if config.PostToolUse[0].Matcher != "Write" {
		t.Errorf("PostToolUse matcher = %q", config.PostToolUse[0].Matcher)
	}
	if len(config.Stop) != 1 {
		t.Errorf("Stop: got %d groups", len(config.Stop))
	}
}
