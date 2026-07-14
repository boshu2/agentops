package trackerresolve

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveSelectedTrackerDoesNotFallback(t *testing.T) {
	look := func(name string) (string, error) {
		if name == BD {
			return "/fake/bd", nil
		}
		return "", errors.New("missing")
	}
	got, err := ResolveWithLookPath(t.TempDir(), []string{"AGENTOPS_TRACKER=br"}, look)
	if err != nil {
		t.Fatal(err)
	}
	if got.Tracker != BR || got.Binary != BR {
		t.Fatalf("selected BR silently fell back: %+v", got)
	}
}

func TestResolveMalformedExplicitConfigFailsClosed(t *testing.T) {
	cwd := t.TempDir()
	home := t.TempDir()
	configDir := filepath.Join(cwd, ".agentops")
	if err := os.Mkdir(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(configDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("tracker: [\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := ResolveWithLookPath(cwd, []string{"HOME=" + home}, fakeLookPath(true, true))
	if err == nil {
		t.Fatalf("ResolveWithLookPath() error = nil, want malformed explicit config %q to fail closed", configPath)
	}
}

func TestResolveAbsentValidUnreadableMalformedInvalidConfigPrecedence(t *testing.T) {
	tests := []struct {
		name           string
		config         string
		configIsDir    bool
		configDangling bool
		wantTracker    string
		wantSource     string
		wantErr        []string
	}{
		{
			name:        "absent falls back to binary discovery",
			wantTracker: BR,
			wantSource:  SourceBinary,
		},
		{
			name:        "missing tracker key falls back to binary discovery",
			config:      "output: json\n",
			wantTracker: BR,
			wantSource:  SourceBinary,
		},
		{
			name:        "valid selects configured backend",
			config:      "tracker: bd\n",
			wantTracker: BD,
			wantSource:  SourceConfig,
		},
		{
			name:    "empty tracker value fails closed",
			config:  "tracker: \"\"\n",
			wantErr: []string{"tracker key in", "unknown tracker"},
		},
		{
			name:    "whitespace tracker value fails closed",
			config:  "tracker: \"   \"\n",
			wantErr: []string{"tracker key in", "unknown tracker"},
		},
		{
			name:    "null tracker value fails closed",
			config:  "tracker: null\n",
			wantErr: []string{"tracker key in", "unknown tracker"},
		},
		{
			name:        "unreadable fails closed",
			configIsDir: true,
			wantErr:     []string{"read tracker configuration"},
		},
		{
			name:           "dangling symlink fails closed",
			configDangling: true,
			wantErr:        []string{"read tracker configuration"},
		},
		{
			name:    "malformed fails closed",
			config:  "tracker: [\n",
			wantErr: []string{"parse tracker configuration"},
		},
		{
			name:    "invalid backend fails closed",
			config:  "tracker: fossil\n",
			wantErr: []string{"tracker key in", `unknown tracker "fossil"`},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cwd := t.TempDir()
			home := t.TempDir()
			configPath := filepath.Join(cwd, ".agentops", "config.yaml")
			if test.config != "" || test.configIsDir || test.configDangling {
				if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
					t.Fatal(err)
				}
				switch {
				case test.configIsDir:
					if err := os.Mkdir(configPath, 0o755); err != nil {
						t.Fatal(err)
					}
				case test.configDangling:
					if err := os.Symlink(filepath.Join(cwd, "missing-config.yaml"), configPath); err != nil {
						t.Fatal(err)
					}
				default:
					if err := os.WriteFile(configPath, []byte(test.config), 0o644); err != nil {
						t.Fatal(err)
					}
				}
			}

			got, err := ResolveWithLookPath(cwd, []string{"HOME=" + home}, fakeLookPath(true, true))
			if len(test.wantErr) > 0 {
				if err == nil {
					t.Fatalf("ResolveWithLookPath() error = nil, want error containing %q", test.wantErr)
				}
				for _, part := range append([]string{configPath}, test.wantErr...) {
					if !strings.Contains(err.Error(), part) {
						t.Errorf("ResolveWithLookPath() error = %q, want substring %q", err, part)
					}
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got.Tracker != test.wantTracker || got.Source != test.wantSource {
				t.Fatalf("ResolveWithLookPath() = %+v, want tracker=%q source=%q", got, test.wantTracker, test.wantSource)
			}
		})
	}
}

func TestResolveBeadsDirLinkedWorktreeUsesGitCommonDir(t *testing.T) {
	root := initTrackerRepo(t)
	lane := filepath.Join(t.TempDir(), "lane")
	runGit(t, root, "worktree", "add", "-b", "test-lane", lane)
	ledger := filepath.Join(root, "_beads")
	if err := os.Mkdir(ledger, 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := ResolveWithLookPath(lane, []string{"HOME=" + t.TempDir()}, fakeLookPath(true, true))
	if err != nil {
		t.Fatal(err)
	}
	if got.Tracker != BR || got.LedgerDir != ledger || got.RepoRoot != root {
		t.Fatalf("linked-worktree resolution = %+v, want br ledger %q rooted at %q", got, ledger, root)
	}
	if got.LedgerSource != LedgerSourceGitCommon || got.GitCommonDir != filepath.Join(root, ".git") {
		t.Fatalf("linked-worktree source = %+v", got)
	}
	if got.WorkDir != lane || envLast(got.ChildEnv, "BEADS_DIR") != ledger {
		t.Fatalf("br child context = workdir %q env %q", got.WorkDir, envLast(got.ChildEnv, "BEADS_DIR"))
	}
}

func TestResolveTrackerBDStripsBeadsDirFromChildEnvironment(t *testing.T) {
	root := initTrackerRepo(t)
	ledger := filepath.Join(root, ".beads")
	if err := os.Mkdir(ledger, 0o755); err != nil {
		t.Fatal(err)
	}
	env := []string{"HOME=" + t.TempDir(), "AGENTOPS_TRACKER=bd", "BEADS_DIR=/wrong", "KEEP=present"}
	got, err := ResolveWithLookPath(root, env, fakeLookPath(true, true))
	if err != nil {
		t.Fatal(err)
	}
	if got.Tracker != BD || got.LedgerDir != ledger || got.WorkDir != root {
		t.Fatalf("bd resolution = %+v", got)
	}
	if _, ok := envValue(got.ChildEnv, "BEADS_DIR"); ok {
		t.Fatalf("bd child environment leaked BEADS_DIR: %v", got.ChildEnv)
	}
	if envLast(got.ChildEnv, "KEEP") != "present" {
		t.Fatalf("bd child environment dropped unrelated values: %v", got.ChildEnv)
	}
}

func TestResolveTrackerIgnoresForeignBeadsDirDuringBackendSelection(t *testing.T) {
	root := initTrackerRepo(t)
	bdLedger := filepath.Join(root, ".beads")
	if err := os.Mkdir(bdLedger, 0o755); err != nil {
		t.Fatal(err)
	}
	foreignBRLedger := filepath.Join(t.TempDir(), "_beads")
	if err := os.Mkdir(foreignBRLedger, 0o755); err != nil {
		t.Fatal(err)
	}

	env := []string{"HOME=" + t.TempDir(), "BEADS_DIR=" + foreignBRLedger}
	got, err := ResolveWithLookPath(root, env, fakeLookPath(true, true))
	if err != nil {
		t.Fatal(err)
	}
	if got.Tracker != BD || got.LedgerDir != bdLedger || got.Source != SourceLedger {
		t.Fatalf("foreign BEADS_DIR hijacked backend selection: got %+v, want bd ledger %q", got, bdLedger)
	}
	if _, ok := envValue(got.ChildEnv, "BEADS_DIR"); ok {
		t.Fatalf("selected bd child environment leaked foreign BEADS_DIR: %v", got.ChildEnv)
	}
}

func TestResolveBeadsDirExplicitRelativeOverride(t *testing.T) {
	cwd := t.TempDir()
	got := ResolveLedger(cwd, []string{"BEADS_DIR=private/ledger"}, BR)
	want := filepath.Join(cwd, "private", "ledger")
	if got.Path != want || got.Source != LedgerSourceEnv {
		t.Fatalf("ResolveLedger() = %+v, want path=%q source=%q", got, want, LedgerSourceEnv)
	}
}

func initTrackerRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	runGit(t, root, "init")
	runGit(t, root, "config", "user.email", "test@example.com")
	runGit(t, root, "config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("fixture\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", "README.md")
	runGit(t, root, "commit", "-m", "fixture")
	if real, err := filepath.EvalSymlinks(root); err == nil {
		root = real
	}
	return root
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, output)
	}
}

func fakeLookPath(br, bd bool) LookPath {
	return func(name string) (string, error) {
		if name == BR && br || name == BD && bd {
			return "/fake/bin/" + name, nil
		}
		return "", exec.ErrNotFound
	}
}

func envLast(env []string, key string) string {
	value, _ := envValue(env, key)
	return value
}
