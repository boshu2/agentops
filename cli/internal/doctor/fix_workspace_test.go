package doctor

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// writeWorkspaceFile creates a file with the given content and mtime,
// creating parent directories as needed.
func writeWorkspaceFile(t *testing.T, path, content string, mtime time.Time) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	if err := os.Chtimes(path, mtime, mtime); err != nil {
		t.Fatalf("chtimes %s: %v", path, err)
	}
}

func TestWorkspaceAgentsDir(t *testing.T) {
	env := &DetectEnv{RepoRoot: "/repo/root", CWD: "/somewhere/else"}
	got := workspaceAgentsDir(env)
	want := filepath.Join("/repo/root", ".agents")
	if got != want {
		t.Errorf("workspaceAgentsDir = %q, want %q", got, want)
	}
}

func TestWorkspaceDirInventory(t *testing.T) {
	base := t.TempDir()
	outside := t.TempDir()

	t1 := time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 7, 5, 12, 30, 0, 0, time.UTC)
	t3 := time.Date(2026, 7, 3, 8, 0, 0, 0, time.UTC)

	// alpha: two regular files (one nested), plus a symlink to a decoy file
	// that must contribute nothing.
	writeWorkspaceFile(t, filepath.Join(base, "alpha", "a.txt"), "aaaaa", t1) // 5 bytes
	writeWorkspaceFile(t, filepath.Join(base, "alpha", "sub", "b.txt"), "bbb", t2)
	// Decoy: much larger and much newer than anything in the fixture. If any
	// symlink were followed, FileCount/ByteSize/NewestMTime would all drift.
	decoy := filepath.Join(outside, "decoy.bin")
	writeWorkspaceFile(t, decoy, "0123456789012345678901234567890123456789", time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC))
	if err := os.Symlink(decoy, filepath.Join(base, "alpha", "link-file")); err != nil {
		t.Fatalf("symlink file: %v", err)
	}

	// beta: empty directory.
	if err := os.MkdirAll(filepath.Join(base, "beta"), 0o755); err != nil {
		t.Fatal(err)
	}

	// gamma: one file.
	writeWorkspaceFile(t, filepath.Join(base, "gamma", "g.bin"), "ggggggg", t3) // 7 bytes

	// Top-level symlink to a directory: must not be inventoried.
	if err := os.Symlink(filepath.Join(base, "alpha"), filepath.Join(base, "link-dir")); err != nil {
		t.Fatalf("symlink dir: %v", err)
	}
	// Top-level regular file: not a directory, skipped.
	writeWorkspaceFile(t, filepath.Join(base, "plain.txt"), "x", t1)

	got, err := workspaceDirInventory(base)
	if err != nil {
		t.Fatalf("workspaceDirInventory: %v", err)
	}
	want := []workspaceDirInfo{
		{Name: "alpha", FileCount: 2, ByteSize: 8, NewestMTime: t2},
		{Name: "beta", FileCount: 0, ByteSize: 0, NewestMTime: time.Time{}},
		{Name: "gamma", FileCount: 1, ByteSize: 7, NewestMTime: t3},
	}
	if len(got) != len(want) {
		t.Fatalf("inventory length = %d, want %d (%+v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i].Name != want[i].Name {
			t.Errorf("entry %d Name = %q, want %q", i, got[i].Name, want[i].Name)
		}
		if got[i].FileCount != want[i].FileCount {
			t.Errorf("%s FileCount = %d, want %d", want[i].Name, got[i].FileCount, want[i].FileCount)
		}
		if got[i].ByteSize != want[i].ByteSize {
			t.Errorf("%s ByteSize = %d, want %d", want[i].Name, got[i].ByteSize, want[i].ByteSize)
		}
		if !got[i].NewestMTime.Equal(want[i].NewestMTime) {
			t.Errorf("%s NewestMTime = %v, want %v", want[i].Name, got[i].NewestMTime, want[i].NewestMTime)
		}
	}
}

func TestWorkspaceDirInventory_PermissionErrorSkipped(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root; permission denial is not enforceable")
	}
	base := t.TempDir()
	t1 := time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC)
	writeWorkspaceFile(t, filepath.Join(base, "alpha", "a.txt"), "aa", t1) // 2 bytes
	writeWorkspaceFile(t, filepath.Join(base, "alpha", "locked", "hidden.txt"), "hhhh", t1.Add(time.Hour))
	locked := filepath.Join(base, "alpha", "locked")
	if err := os.Chmod(locked, 0o000); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })

	got, err := workspaceDirInventory(base)
	if err != nil {
		t.Fatalf("workspaceDirInventory: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("inventory length = %d, want 1 (%+v)", len(got), got)
	}
	// The unreadable subtree is skipped: only a.txt is counted.
	if got[0].FileCount != 1 {
		t.Errorf("FileCount = %d, want 1", got[0].FileCount)
	}
	if got[0].ByteSize != 2 {
		t.Errorf("ByteSize = %d, want 2", got[0].ByteSize)
	}
	if !got[0].NewestMTime.Equal(t1) {
		t.Errorf("NewestMTime = %v, want %v", got[0].NewestMTime, t1)
	}
}

func TestWorkspaceDirInventory_MissingBase(t *testing.T) {
	_, err := workspaceDirInventory(filepath.Join(t.TempDir(), "does-not-exist"))
	if !os.IsNotExist(err) {
		t.Errorf("err = %v, want os.IsNotExist", err)
	}
}

func TestWorkspaceCanonicalAliases(t *testing.T) {
	tests := []struct {
		alias, canonical string
	}{
		{"post-mortem", "postmortem"},
		{"post-mortems", "postmortem"},
		{"pre-mortem", "pre-mortem-checks"},
		{"pre-mortems", "pre-mortem-checks"},
		{"handoffs", "handoff"},
		{"mto-handoff", "handoff"},
		{"retros", "retro"},
		{"proof", "proofs"},
		{"test", "tests"},
	}
	if len(workspaceCanonicalAliases) != len(tests) {
		t.Errorf("registry has %d entries, want %d", len(workspaceCanonicalAliases), len(tests))
	}
	for _, tt := range tests {
		got, ok := workspaceCanonicalAliases[tt.alias]
		if !ok {
			t.Errorf("alias %q missing from registry", tt.alias)
			continue
		}
		if got != tt.canonical {
			t.Errorf("alias %q -> %q, want %q", tt.alias, got, tt.canonical)
		}
	}
}

func TestWorkspaceStaleNameMatching(t *testing.T) {
	tests := []struct {
		name          string
		explicitStale bool
		retryChain    bool
	}{
		// Real names that MUST match.
		{"land-queue-age-h433.19-native.stale-20260711T221704Z", true, false},
		{"land-queue-age-h433.19.stale-20260711T221032Z", true, false},
		{"land-queue-age-h433.22-native-retry2", false, true},
		{"land-queue-age-h433.27-native-retry2", false, true},
		// A staled retry-chain dir is both.
		{"land-queue-age-h433.22-native-retry2.stale-20260711T221704Z", true, true},
		// Names that must NOT match.
		{"land-queue", false, false},
		{"land-queue-runtime", false, false},
		{"archive", false, false},
		{"knowledge", false, false},
		{"retro", false, false},
	}
	for _, tt := range tests {
		explicitStale, retryChain := workspaceStaleNameKind(tt.name)
		if explicitStale != tt.explicitStale {
			t.Errorf("workspaceStaleNameKind(%q) explicitStale = %v, want %v", tt.name, explicitStale, tt.explicitStale)
		}
		if retryChain != tt.retryChain {
			t.Errorf("workspaceStaleNameKind(%q) retryChain = %v, want %v", tt.name, retryChain, tt.retryChain)
		}
		wantStale := tt.explicitStale || tt.retryChain
		if got := isWorkspaceStaleDirName(tt.name); got != wantStale {
			t.Errorf("isWorkspaceStaleDirName(%q) = %v, want %v", tt.name, got, wantStale)
		}
	}
}

func TestWorkspaceQuarantineDest(t *testing.T) {
	ctx := &MutateContext{RunDir: filepath.Join("/repo", ".doctor", "runs", "r1")}
	got := workspaceQuarantineDest(ctx, "land-queue-age-h433.22-native-retry2")
	want := filepath.Join("/repo", ".doctor", "runs", "r1", "quarantine", "workspace", "land-queue-age-h433.22-native-retry2")
	if got != want {
		t.Errorf("workspaceQuarantineDest = %q, want %q", got, want)
	}
}

func TestWorkspaceGCTTL(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  time.Duration
	}{
		{"default when unset", "", 14 * 24 * time.Hour},
		{"override 7 days", "7", 7 * 24 * time.Hour},
		{"override 1 day", "1", 24 * time.Hour},
		{"zero falls back", "0", 14 * 24 * time.Hour},
		{"negative falls back", "-3", 14 * 24 * time.Hour},
		{"non-numeric falls back", "abc", 14 * 24 * time.Hour},
		{"float falls back", "2.5", 14 * 24 * time.Hour},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(workspaceTTLEnvVar, tt.value)
			if got := workspaceGCTTL(); got != tt.want {
				t.Errorf("workspaceGCTTL() with %q = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}
