package paths

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// withEnv sets env vars for the test and restores them on cleanup. It also
// unsets the full AO_* + CLAUDE_PLUGIN_DATA family so leaked values from the
// surrounding shell never reach into the resolver under test.
func withEnv(t *testing.T, env map[string]string) {
	t.Helper()
	keys := []string{
		"AO_HOME", "CLAUDE_PLUGIN_DATA",
		"AO_AGENTS_DIR", "AO_KNOWLEDGE_ROOT", "AO_HOOKS_DIR", "AO_SCOPE_LOCK",
		"AO_RPI_DIR", "AO_FINDINGS_DIR", "AO_PLANS_DIR", "AO_COUNCIL_DIR",
		"AO_LEARNINGS_DIR", "AO_PATTERNS_DIR", "AO_DECISIONS_DIR",
		"AO_PATHS_DEBUG",
	}
	prev := map[string]string{}
	prevSet := map[string]bool{}
	for _, k := range keys {
		v, ok := os.LookupEnv(k)
		prev[k] = v
		prevSet[k] = ok
		if err := os.Unsetenv(k); err != nil {
			t.Fatalf("unsetenv %s: %v", k, err)
		}
	}
	for k, v := range env {
		if err := os.Setenv(k, v); err != nil {
			t.Fatalf("setenv %s=%s: %v", k, v, err)
		}
	}
	t.Cleanup(func() {
		for _, k := range keys {
			if prevSet[k] {
				_ = os.Setenv(k, prev[k])
			} else {
				_ = os.Unsetenv(k)
			}
		}
	})
}

// chdir changes the working directory for the test and restores it.
func chdir(t *testing.T, dir string) {
	t.Helper()
	prev, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir %s: %v", dir, err)
	}
	t.Cleanup(func() { _ = os.Chdir(prev) })
}

func TestResolve_EnvPrecedence(t *testing.T) {
	// Cannot run table cases in parallel: env mutation is process-global.
	tmp := t.TempDir()
	// macOS resolves /var → /private/var via os.Getwd; align expected paths
	// with what Resolve() will observe after chdir.
	if resolved, err := filepath.EvalSymlinks(tmp); err == nil {
		tmp = resolved
	}
	pluginRoot := filepath.Join(tmp, "plugin-root")
	homeOverride := filepath.Join(tmp, "explicit-home")
	cwdRoot := filepath.Join(tmp, "cwd-root")
	for _, d := range []string{pluginRoot, homeOverride, cwdRoot} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", d, err)
		}
	}

	cases := []struct {
		name      string
		env       map[string]string
		chdirTo   string
		wantHome  string
		wantAgent string // AgentsDir override target ("" means equal to home)
	}{
		{
			name:     "no env set, defaults to cwd/.agents",
			env:      nil,
			chdirTo:  cwdRoot,
			wantHome: filepath.Join(cwdRoot, ".agents"),
		},
		{
			name:     "AO_HOME set explicitly",
			env:      map[string]string{"AO_HOME": homeOverride},
			chdirTo:  cwdRoot,
			wantHome: homeOverride,
		},
		{
			name:     "CLAUDE_PLUGIN_DATA set",
			env:      map[string]string{"CLAUDE_PLUGIN_DATA": pluginRoot},
			chdirTo:  cwdRoot,
			wantHome: filepath.Join(pluginRoot, ".agents"),
		},
		{
			name: "both AO_HOME and CLAUDE_PLUGIN_DATA set — AO_HOME wins",
			env: map[string]string{
				"AO_HOME":            homeOverride,
				"CLAUDE_PLUGIN_DATA": pluginRoot,
			},
			chdirTo:  cwdRoot,
			wantHome: homeOverride,
		},
		{
			name: "AO_AGENTS_DIR override decouples from home",
			env: map[string]string{
				"AO_HOME":       homeOverride,
				"AO_AGENTS_DIR": filepath.Join(tmp, "agents-elsewhere"),
			},
			chdirTo:   cwdRoot,
			wantHome:  homeOverride,
			wantAgent: filepath.Join(tmp, "agents-elsewhere"),
		},
		{
			name:     "AO_HOME with relative path is preserved verbatim (no normalization)",
			env:      map[string]string{"AO_HOME": "rel/agents"},
			chdirTo:  cwdRoot,
			wantHome: "rel/agents",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			withEnv(t, tc.env)
			chdir(t, tc.chdirTo)
			got := Resolve()
			if got == nil {
				t.Fatalf("Resolve() returned nil")
			}
			if got.Home != tc.wantHome {
				t.Errorf("Home = %q, want %q", got.Home, tc.wantHome)
			}
			wantAgent := tc.wantAgent
			if wantAgent == "" {
				wantAgent = tc.wantHome
			}
			if got.AgentsDir != wantAgent {
				t.Errorf("AgentsDir = %q, want %q", got.AgentsDir, wantAgent)
			}
			// Default sub-roots must hang off AgentsDir.
			wantWiki := filepath.Join(wantAgent, "wiki")
			if got.KnowledgeRoot != wantWiki {
				t.Errorf("KnowledgeRoot = %q, want %q", got.KnowledgeRoot, wantWiki)
			}
			wantLock := filepath.Join(wantAgent, "scope.lock")
			if got.ScopeLock != wantLock {
				t.Errorf("ScopeLock = %q, want %q", got.ScopeLock, wantLock)
			}
		})
	}
}

func TestResolve_Idempotent(t *testing.T) {
	tmp := t.TempDir()
	withEnv(t, map[string]string{"AO_HOME": tmp})
	chdir(t, tmp)
	a := Resolve()
	b := Resolve()
	if *a != *b {
		t.Errorf("Resolve() not idempotent: a=%+v b=%+v", a, b)
	}
}

func TestResolveFromRoot_UsesGitRootAndEnvOverrides(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git unavailable")
	}
	tmp := t.TempDir()
	repo := filepath.Join(tmp, "repo")
	subdir := filepath.Join(repo, "nested")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatalf("mkdir subdir: %v", err)
	}
	cmd := exec.Command("git", "init")
	cmd.Dir = repo
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}

	withEnv(t, nil)
	got := ResolveFromRoot(subdir)
	if got.AgentsDir != filepath.Join(repo, ".agents") {
		t.Fatalf("AgentsDir = %q, want repo root fallback", got.AgentsDir)
	}

	agentsOverride := filepath.Join(tmp, "agents-override")
	withEnv(t, map[string]string{"AO_AGENTS_DIR": agentsOverride})
	got = ResolveFromRoot(subdir)
	if got.AgentsDir != agentsOverride {
		t.Fatalf("AgentsDir = %q, want AO_AGENTS_DIR override %q", got.AgentsDir, agentsOverride)
	}
}

func TestValidate_CreatesMissingDirs(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "fresh", ".agents")
	withEnv(t, map[string]string{"AO_HOME": home})
	chdir(t, tmp)
	p := Resolve()
	if err := p.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	for _, dir := range []string{p.AgentsDir, p.KnowledgeRoot, p.HooksDir, p.FindingsDir, p.PlansDir} {
		if info, err := os.Stat(dir); err != nil || !info.IsDir() {
			t.Errorf("Validate did not create %s (err=%v)", dir, err)
		}
	}
}

func TestValidate_RejectsFileInPlaceOfDir(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, ".agents")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatalf("mkdir home: %v", err)
	}
	// Make a file where the wiki dir should go.
	wikiPath := filepath.Join(home, "wiki")
	if err := os.WriteFile(wikiPath, []byte("not a dir"), 0o644); err != nil {
		t.Fatalf("write wiki file: %v", err)
	}
	withEnv(t, map[string]string{"AO_HOME": home})
	chdir(t, tmp)
	p := Resolve()
	if err := p.Validate(); err == nil {
		t.Error("Validate should fail when KnowledgeRoot path is a file")
	}
}

// TestShellGoAgreement is the cross-language L2 scenario: under controlled
// env, lib/ao-paths.sh and Resolve() must produce identical AO_* values.
func TestShellGoAgreement(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell resolver unavailable on Windows")
	}
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not on PATH")
	}

	// Locate lib/ao-paths.sh by walking up from the test's working dir.
	scriptPath := findScript(t)
	if scriptPath == "" {
		t.Skip("lib/ao-paths.sh not found relative to test cwd")
	}

	cases := []struct {
		name string
		env  map[string]string
	}{
		{
			name: "explicit AO_HOME",
			env:  map[string]string{"AO_HOME": filepath.Join(t.TempDir(), "agents")},
		},
		{
			name: "CLAUDE_PLUGIN_DATA",
			env:  map[string]string{"CLAUDE_PLUGIN_DATA": filepath.Join(t.TempDir(), "plugin")},
		},
		{
			name: "AO_AGENTS_DIR override",
			env: map[string]string{
				"AO_HOME":       filepath.Join(t.TempDir(), "home"),
				"AO_AGENTS_DIR": filepath.Join(t.TempDir(), "agents"),
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			withEnv(t, tc.env)
			// Use the script's parent dir as cwd so default repo-root resolution
			// stays consistent between shell and Go.
			chdir(t, filepath.Dir(filepath.Dir(scriptPath)))

			// Run the shell resolver.
			cmd := exec.Command("bash", "-c", `eval "$(`+scriptPath+`)" && env | grep '^AO_' | sort`)
			cmd.Env = os.Environ()
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("shell resolver failed: %v\nout: %s", err, out)
			}
			shellEnv := parseEnvLines(string(out))

			// Run the Go resolver under the same env.
			goPaths := Resolve()
			goEnv := goPathsToEnv(goPaths)

			// Compare every AO_* key — shell may export AO_PATHS_DEBUG (excluded).
			for _, key := range []string{
				"AO_HOME", "AO_AGENTS_DIR", "AO_KNOWLEDGE_ROOT",
				"AO_HOOKS_DIR", "AO_SCOPE_LOCK", "AO_RPI_DIR",
				"AO_FINDINGS_DIR", "AO_PLANS_DIR", "AO_COUNCIL_DIR",
				"AO_LEARNINGS_DIR", "AO_PATTERNS_DIR", "AO_DECISIONS_DIR",
			} {
				if shellEnv[key] != goEnv[key] {
					t.Errorf("%s: shell=%q go=%q", key, shellEnv[key], goEnv[key])
				}
			}
		})
	}
}

// findScript walks up from cwd to find lib/ao-paths.sh.
func findScript(t *testing.T) string {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	dir := cwd
	for i := 0; i < 8; i++ {
		candidate := filepath.Join(dir, "lib", "ao-paths.sh")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

// parseEnvLines parses `KEY=value` lines into a map.
func parseEnvLines(s string) map[string]string {
	m := map[string]string{}
	scanner := bufio.NewScanner(strings.NewReader(s))
	for scanner.Scan() {
		line := scanner.Text()
		i := strings.Index(line, "=")
		if i <= 0 {
			continue
		}
		m[line[:i]] = line[i+1:]
	}
	return m
}

func goPathsToEnv(p *Paths) map[string]string {
	return map[string]string{
		"AO_HOME":           p.Home,
		"AO_AGENTS_DIR":     p.AgentsDir,
		"AO_KNOWLEDGE_ROOT": p.KnowledgeRoot,
		"AO_HOOKS_DIR":      p.HooksDir,
		"AO_SCOPE_LOCK":     p.ScopeLock,
		"AO_RPI_DIR":        p.RPIDir,
		"AO_FINDINGS_DIR":   p.FindingsDir,
		"AO_PLANS_DIR":      p.PlansDir,
		"AO_COUNCIL_DIR":    p.CouncilDir,
		"AO_LEARNINGS_DIR":  p.LearningsDir,
		"AO_PATTERNS_DIR":   p.PatternsDir,
		"AO_DECISIONS_DIR":  p.DecisionsDir,
	}
}
