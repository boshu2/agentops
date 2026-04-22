package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDetectRuntimeKind_CodexThread(t *testing.T) {
	t.Setenv("CODEX_THREAD_ID", "abc-123")
	t.Setenv("CLAUDE_SESSION_ID", "")
	t.Setenv("OPENCODE_SESSION_ID", "")
	t.Setenv("CODEX_INTERNAL_ORIGINATOR_OVERRIDE", "")
	t.Setenv("CODEX_CI", "")

	got := detectRuntimeKind(false)
	if got != runtimeKindCodex {
		t.Errorf("detectRuntimeKind(false) = %q, want %q", got, runtimeKindCodex)
	}
}

func TestDetectRuntimeKind_ForceCodex(t *testing.T) {
	t.Setenv("CODEX_THREAD_ID", "")
	t.Setenv("CLAUDE_SESSION_ID", "claude-sess-1")
	t.Setenv("OPENCODE_SESSION_ID", "")
	t.Setenv("CODEX_INTERNAL_ORIGINATOR_OVERRIDE", "")
	t.Setenv("CODEX_CI", "")

	got := detectRuntimeKind(true)
	if got != runtimeKindCodex {
		t.Errorf("detectRuntimeKind(true) = %q, want %q (force should override)", got, runtimeKindCodex)
	}
}

func TestDetectRuntimeKind_Claude(t *testing.T) {
	t.Setenv("CODEX_THREAD_ID", "")
	t.Setenv("CODEX_INTERNAL_ORIGINATOR_OVERRIDE", "")
	t.Setenv("CODEX_CI", "")
	t.Setenv("CLAUDE_SESSION_ID", "sess-claude-99")
	t.Setenv("OPENCODE_SESSION_ID", "")

	got := detectRuntimeKind(false)
	if got != runtimeKindClaude {
		t.Errorf("detectRuntimeKind(false) = %q, want %q", got, runtimeKindClaude)
	}
}

func TestDetectRuntimeKind_OpenCode(t *testing.T) {
	t.Setenv("CODEX_THREAD_ID", "")
	t.Setenv("CODEX_INTERNAL_ORIGINATOR_OVERRIDE", "")
	t.Setenv("CODEX_CI", "")
	t.Setenv("CLAUDE_SESSION_ID", "")
	t.Setenv("OPENCODE_SESSION_ID", "opencode-42")

	got := detectRuntimeKind(false)
	if got != runtimeKindOpenCode {
		t.Errorf("detectRuntimeKind(false) = %q, want %q", got, runtimeKindOpenCode)
	}
}

func TestDetectRuntimeKind_Unknown(t *testing.T) {
	t.Setenv("CODEX_THREAD_ID", "")
	t.Setenv("CODEX_INTERNAL_ORIGINATOR_OVERRIDE", "")
	t.Setenv("CODEX_CI", "")
	t.Setenv("CLAUDE_SESSION_ID", "")
	t.Setenv("OPENCODE_SESSION_ID", "")

	got := detectRuntimeKind(false)
	if got != runtimeKindUnknown {
		t.Errorf("detectRuntimeKind(false) = %q, want %q", got, runtimeKindUnknown)
	}
}

func TestDetectRuntimeKind_CodexCI(t *testing.T) {
	t.Setenv("CODEX_THREAD_ID", "")
	t.Setenv("CODEX_INTERNAL_ORIGINATOR_OVERRIDE", "")
	t.Setenv("CODEX_CI", "true")
	t.Setenv("CLAUDE_SESSION_ID", "")
	t.Setenv("OPENCODE_SESSION_ID", "")

	got := detectRuntimeKind(false)
	if got != runtimeKindCodex {
		t.Errorf("detectRuntimeKind(false) = %q, want %q", got, runtimeKindCodex)
	}
}

func TestDetectRuntimeKind_CodexOriginator(t *testing.T) {
	t.Setenv("CODEX_THREAD_ID", "")
	t.Setenv("CODEX_INTERNAL_ORIGINATOR_OVERRIDE", "codex-internal")
	t.Setenv("CODEX_CI", "")
	t.Setenv("CLAUDE_SESSION_ID", "")
	t.Setenv("OPENCODE_SESSION_ID", "")

	got := detectRuntimeKind(false)
	if got != runtimeKindCodex {
		t.Errorf("detectRuntimeKind(false) = %q, want %q", got, runtimeKindCodex)
	}
}

func TestFindLastTranscriptInDir_ReturnsNewest(t *testing.T) {
	dir := t.TempDir()

	// Create two .jsonl files with different mod times
	older := filepath.Join(dir, "older.jsonl")
	newer := filepath.Join(dir, "newer.jsonl")

	if err := os.WriteFile(older, []byte(`{"test":"old"}`+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	// Set older file's mod time to the past
	pastTime := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(older, pastTime, pastTime); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(newer, []byte(`{"test":"new"}`+"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	got, err := findLastTranscriptInDir(dir, "no transcripts")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != newer {
		t.Errorf("findLastTranscriptInDir() = %q, want %q", got, newer)
	}
}

func TestFindLastTranscriptInDir_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	_, err := findLastTranscriptInDir(dir, "custom empty message")
	if err == nil {
		t.Fatal("expected error for empty dir")
	}
	if err.Error() != "custom empty message" {
		t.Errorf("error = %q, want %q", err.Error(), "custom empty message")
	}
}

func TestFindLastTranscriptInDir_NoJsonl(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("not jsonl"), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := findLastTranscriptInDir(dir, "no jsonl files")
	if err == nil {
		t.Fatal("expected error when no .jsonl files exist")
	}
	if err.Error() != "no jsonl files" {
		t.Errorf("error = %q, want %q", err.Error(), "no jsonl files")
	}
}

func TestFindLastTranscriptInDir_NonexistentDir(t *testing.T) {
	_, err := findLastTranscriptInDir("/tmp/nonexistent-dir-abc123xyz", "fallback msg")
	if err == nil {
		t.Fatal("expected error for nonexistent directory")
	}
}

func TestReadLatestCodexSessionIndexEntry_NoDir(t *testing.T) {
	tmp := t.TempDir()
	_, err := readLatestCodexSessionIndexEntry(tmp)
	if err == nil {
		t.Fatal("expected error when .codex dir does not exist")
	}
}

func TestReadLatestCodexSessionIndexEntry_ValidEntries(t *testing.T) {
	tmp := t.TempDir()
	codexDir := filepath.Join(tmp, ".codex")
	if err := os.MkdirAll(codexDir, 0755); err != nil {
		t.Fatal(err)
	}

	indexPath := filepath.Join(codexDir, "session_index.jsonl")
	lines := `{"id":"sess-old","thread_name":"Old Thread","updated_at":"2026-01-01T10:00:00Z"}
{"id":"sess-new","thread_name":"New Thread","updated_at":"2026-04-01T10:00:00Z"}
{"id":"sess-mid","thread_name":"Mid Thread","updated_at":"2026-02-15T10:00:00Z"}
`
	if err := os.WriteFile(indexPath, []byte(lines), 0644); err != nil {
		t.Fatal(err)
	}

	entry, err := readLatestCodexSessionIndexEntry(tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entry == nil {
		t.Fatal("expected non-nil entry")
	}
	if entry.ID != "sess-new" {
		t.Errorf("entry.ID = %q, want %q (should return most recent)", entry.ID, "sess-new")
	}
	if entry.ThreadName != "New Thread" {
		t.Errorf("entry.ThreadName = %q, want %q", entry.ThreadName, "New Thread")
	}
}

func TestReadLatestCodexSessionIndexEntry_EmptyFile(t *testing.T) {
	tmp := t.TempDir()
	codexDir := filepath.Join(tmp, ".codex")
	if err := os.MkdirAll(codexDir, 0755); err != nil {
		t.Fatal(err)
	}
	indexPath := filepath.Join(codexDir, "session_index.jsonl")
	if err := os.WriteFile(indexPath, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	entry, err := readLatestCodexSessionIndexEntry(tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entry != nil {
		t.Errorf("expected nil entry for empty file, got %+v", entry)
	}
}

func TestReadLatestCodexSessionIndexEntry_MalformedLines(t *testing.T) {
	tmp := t.TempDir()
	codexDir := filepath.Join(tmp, ".codex")
	if err := os.MkdirAll(codexDir, 0755); err != nil {
		t.Fatal(err)
	}
	indexPath := filepath.Join(codexDir, "session_index.jsonl")
	lines := `not valid json
{"id":"valid","thread_name":"Good","updated_at":"2026-03-01T10:00:00Z"}
{bad json too}
`
	if err := os.WriteFile(indexPath, []byte(lines), 0644); err != nil {
		t.Fatal(err)
	}

	entry, err := readLatestCodexSessionIndexEntry(tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entry == nil {
		t.Fatal("expected non-nil entry despite malformed lines")
	}
	if entry.ID != "valid" {
		t.Errorf("entry.ID = %q, want %q", entry.ID, "valid")
	}
}

// ---------------------------------------------------------------------------
// detectLifecycleRuntimeProfileWithOptions
// ---------------------------------------------------------------------------

func TestDetectLifecycleRuntimeProfileWithOptions_Claude_NoManifest(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CODEX_THREAD_ID", "")
	t.Setenv("CODEX_INTERNAL_ORIGINATOR_OVERRIDE", "")
	t.Setenv("CODEX_CI", "")
	t.Setenv("CLAUDE_SESSION_ID", "claude-test-session")
	t.Setenv("OPENCODE_SESSION_ID", "")

	profile := detectLifecycleRuntimeProfileWithOptions(false)
	if profile.Runtime != runtimeKindClaude {
		t.Errorf("Runtime = %q, want %q", profile.Runtime, runtimeKindClaude)
	}
	if profile.SessionID != "claude-test-session" {
		t.Errorf("SessionID = %q, want %q", profile.SessionID, "claude-test-session")
	}
	if !profile.HookCapable {
		t.Error("HookCapable should be true for Claude")
	}
	if profile.HookConfigured {
		t.Error("HookConfigured should be false without manifest")
	}
	if profile.Reason == "" {
		t.Error("Reason should be set when no manifest found")
	}
}

func TestDetectLifecycleRuntimeProfileWithOptions_Claude_WithManifest(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CODEX_THREAD_ID", "")
	t.Setenv("CODEX_INTERNAL_ORIGINATOR_OVERRIDE", "")
	t.Setenv("CODEX_CI", "")
	t.Setenv("CLAUDE_SESSION_ID", "claude-session-2")
	t.Setenv("OPENCODE_SESSION_ID", "")

	// Create hook manifest
	manifestDir := filepath.Join(home, ".agentops")
	if err := os.MkdirAll(manifestDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(manifestDir, "hooks.json"), []byte(`{"hooks":[]}`), 0644); err != nil {
		t.Fatal(err)
	}

	profile := detectLifecycleRuntimeProfileWithOptions(false)
	if profile.Runtime != runtimeKindClaude {
		t.Errorf("Runtime = %q, want %q", profile.Runtime, runtimeKindClaude)
	}
	if profile.Mode != lifecycleModeHookCapable {
		t.Errorf("Mode = %q, want %q", profile.Mode, lifecycleModeHookCapable)
	}
	if !profile.HookConfigured {
		t.Error("HookConfigured should be true with manifest")
	}
	if profile.HookManifestPath == "" {
		t.Error("HookManifestPath should be set")
	}
}

func TestDetectLifecycleRuntimeProfileWithOptions_OpenCode(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CODEX_THREAD_ID", "")
	t.Setenv("CODEX_INTERNAL_ORIGINATOR_OVERRIDE", "")
	t.Setenv("CODEX_CI", "")
	t.Setenv("CLAUDE_SESSION_ID", "")
	t.Setenv("OPENCODE_SESSION_ID", "opencode-session-1")

	profile := detectLifecycleRuntimeProfileWithOptions(false)
	if profile.Runtime != runtimeKindOpenCode {
		t.Errorf("Runtime = %q, want %q", profile.Runtime, runtimeKindOpenCode)
	}
	if !profile.HookCapable {
		t.Error("HookCapable should be true for OpenCode")
	}
	if profile.HookConfigured {
		t.Error("HookConfigured should be false without manifest")
	}
}

func TestDetectLifecycleRuntimeProfileWithOptions_OpenCode_WithManifest(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CODEX_THREAD_ID", "")
	t.Setenv("CODEX_INTERNAL_ORIGINATOR_OVERRIDE", "")
	t.Setenv("CODEX_CI", "")
	t.Setenv("CLAUDE_SESSION_ID", "")
	t.Setenv("OPENCODE_SESSION_ID", "opencode-session-2")

	manifestDir := filepath.Join(home, ".config", "opencode", "agentops", "hooks")
	if err := os.MkdirAll(manifestDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(manifestDir, "hooks.json"), []byte(`{}`), 0644); err != nil {
		t.Fatal(err)
	}

	profile := detectLifecycleRuntimeProfileWithOptions(false)
	if profile.Runtime != runtimeKindOpenCode {
		t.Errorf("Runtime = %q, want %q", profile.Runtime, runtimeKindOpenCode)
	}
	if profile.Mode != lifecycleModeHookCapable {
		t.Errorf("Mode = %q, want %q", profile.Mode, lifecycleModeHookCapable)
	}
	if !profile.HookConfigured {
		t.Error("HookConfigured should be true with manifest")
	}
}

func TestDetectLifecycleRuntimeProfileWithOptions_Unknown_WithManifest(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CODEX_THREAD_ID", "")
	t.Setenv("CODEX_INTERNAL_ORIGINATOR_OVERRIDE", "")
	t.Setenv("CODEX_CI", "")
	t.Setenv("CLAUDE_SESSION_ID", "")
	t.Setenv("OPENCODE_SESSION_ID", "")

	// Create legacy Claude manifest — should be detected in unknown mode
	manifestDir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(manifestDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(manifestDir, "hooks.json"), []byte(`{}`), 0644); err != nil {
		t.Fatal(err)
	}

	profile := detectLifecycleRuntimeProfileWithOptions(false)
	if profile.Runtime != runtimeKindUnknown {
		t.Errorf("Runtime = %q, want %q", profile.Runtime, runtimeKindUnknown)
	}
	if profile.Mode != lifecycleModeHookCapable {
		t.Errorf("Mode = %q, want %q (should detect installed manifest)", profile.Mode, lifecycleModeHookCapable)
	}
	if !profile.HookCapable {
		t.Error("HookCapable should be true when manifest found")
	}
	if !profile.HookConfigured {
		t.Error("HookConfigured should be true when manifest found")
	}
}

func TestDetectLifecycleRuntimeProfileWithOptions_Unknown_NoManifest(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CODEX_THREAD_ID", "")
	t.Setenv("CODEX_INTERNAL_ORIGINATOR_OVERRIDE", "")
	t.Setenv("CODEX_CI", "")
	t.Setenv("CLAUDE_SESSION_ID", "")
	t.Setenv("OPENCODE_SESSION_ID", "")

	profile := detectLifecycleRuntimeProfileWithOptions(false)
	if profile.Runtime != runtimeKindUnknown {
		t.Errorf("Runtime = %q, want %q", profile.Runtime, runtimeKindUnknown)
	}
	if profile.Mode != lifecycleModeManual {
		t.Errorf("Mode = %q, want %q", profile.Mode, lifecycleModeManual)
	}
	if profile.Reason == "" {
		t.Error("expected non-empty Reason")
	}
}

func TestDetectLifecycleRuntimeProfileWithOptions_Codex(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CODEX_THREAD_ID", "codex-thread-123")
	t.Setenv("CODEX_INTERNAL_ORIGINATOR_OVERRIDE", "")
	t.Setenv("CODEX_CI", "")
	t.Setenv("CLAUDE_SESSION_ID", "")
	t.Setenv("OPENCODE_SESSION_ID", "")

	profile := detectLifecycleRuntimeProfileWithOptions(false)
	if profile.Runtime != runtimeKindCodex {
		t.Errorf("Runtime = %q, want %q", profile.Runtime, runtimeKindCodex)
	}
	if profile.Mode != lifecycleModeCodexHookless {
		t.Errorf("Mode = %q, want %q", profile.Mode, lifecycleModeCodexHookless)
	}
	if profile.HookCapable {
		t.Error("HookCapable should be false for Codex")
	}
}

func TestDetectLifecycleRuntimeProfileWithOptions_Codex_WithNativeHooksConfigured(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CODEX_THREAD_ID", "codex-thread-native")
	t.Setenv("CODEX_INTERNAL_ORIGINATOR_OVERRIDE", "")
	t.Setenv("CODEX_CI", "")
	t.Setenv("CLAUDE_SESSION_ID", "")
	t.Setenv("OPENCODE_SESSION_ID", "")

	codexDir := filepath.Join(home, ".codex")
	if err := os.MkdirAll(codexDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(codexDir, "version.json"), []byte(`{"latest_version":"0.122.0"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(codexDir, "config.toml"), []byte("[features]\ncodex_hooks = true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	manifest := `{"hooks":{"SessionStart":[{"hooks":[{"type":"command","command":"bash /tmp/session-start.sh"}]}]}}`
	if err := os.WriteFile(filepath.Join(codexDir, "hooks.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}

	profile := detectLifecycleRuntimeProfileWithOptions(false)
	if profile.Runtime != runtimeKindCodex {
		t.Errorf("Runtime = %q, want %q", profile.Runtime, runtimeKindCodex)
	}
	if profile.Mode != lifecycleModeHookCapable {
		t.Errorf("Mode = %q, want %q", profile.Mode, lifecycleModeHookCapable)
	}
	if !profile.HookCapable {
		t.Error("HookCapable should be true when native Codex hooks are supported")
	}
	if !profile.HookConfigured {
		t.Error("HookConfigured should be true with a valid native Codex hook manifest")
	}
	if got := profile.HookManifestPath; got != filepath.Join(codexDir, "hooks.json") {
		t.Errorf("HookManifestPath = %q, want %q", got, filepath.Join(codexDir, "hooks.json"))
	}
}

func TestDetectLifecycleRuntimeProfileWithOptions_Codex_WithLegacyFlatManifest(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CODEX_THREAD_ID", "codex-thread-legacy")
	t.Setenv("CODEX_INTERNAL_ORIGINATOR_OVERRIDE", "")
	t.Setenv("CODEX_CI", "")
	t.Setenv("CLAUDE_SESSION_ID", "")
	t.Setenv("OPENCODE_SESSION_ID", "")

	codexDir := filepath.Join(home, ".codex")
	if err := os.MkdirAll(codexDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(codexDir, "version.json"), []byte(`{"latest_version":"0.122.0"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(codexDir, "config.toml"), []byte("[features]\ncodex_hooks = true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	legacyManifest := `{"hooks":[{"name":"agentops-session-start","event":"SessionStart","command":"bash /tmp/session-start.sh","timeout":10000}]}`
	if err := os.WriteFile(filepath.Join(codexDir, "hooks.json"), []byte(legacyManifest), 0o644); err != nil {
		t.Fatal(err)
	}

	profile := detectLifecycleRuntimeProfileWithOptions(false)
	if profile.Runtime != runtimeKindCodex {
		t.Errorf("Runtime = %q, want %q", profile.Runtime, runtimeKindCodex)
	}
	if profile.Mode != lifecycleModeCodexHookless {
		t.Errorf("Mode = %q, want %q", profile.Mode, lifecycleModeCodexHookless)
	}
	if !profile.HookCapable {
		t.Error("HookCapable should remain true when the installed Codex version supports native hooks")
	}
	if profile.HookConfigured {
		t.Error("HookConfigured should be false for the legacy flat hooks.json shape")
	}
	if !strings.Contains(profile.Reason, "unsupported hooks shape") {
		t.Errorf("Reason = %q, want unsupported hooks shape guidance", profile.Reason)
	}
}

func TestExtractSessionIDFromCodexArchivedPath(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{
			name: "valid UUID path",
			path: "/home/user/.codex/archived_sessions/a1b2c3d4-e5f6-7890-abcd-ef1234567890.jsonl",
			want: "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
		},
		{
			name: "path with prefix",
			path: "/home/.codex/archived_sessions/session_a1b2c3d4-e5f6-7890-abcd-ef1234567890.jsonl",
			want: "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
		},
		{
			name: "no UUID in path",
			path: "/home/.codex/archived_sessions/random-file.jsonl",
			want: "",
		},
		{
			name: "empty path",
			path: "",
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractSessionIDFromCodexArchivedPath(tt.path)
			if got != tt.want {
				t.Errorf("extractSessionIDFromCodexArchivedPath(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestResolveCodexSessionID_EmptyHome(t *testing.T) {
	t.Setenv("CODEX_THREAD_ID", "")
	tmp := t.TempDir()
	got := resolveCodexSessionID(tmp)
	// No .codex directory, should return empty
	if got != "" {
		t.Errorf("resolveCodexSessionID(%q) = %q, want empty", tmp, got)
	}
}
