package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// makeGitRepoForTracker inits a throwaway git repo and returns its real
// (symlink-resolved) root so git-common-dir comparisons match on macOS, where
// t.TempDir lives under a /var -> /private/var symlink.
func makeGitRepoForTracker(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git unavailable: %v", err)
	}
	root := t.TempDir()
	runGit(t, root, "init")
	if real, err := filepath.EvalSymlinks(root); err == nil {
		root = real
	}
	return root
}

// setTrackerLookPath swaps the binary resolver for one test and restores it.
func setTrackerLookPath(t *testing.T, br, bd bool) {
	t.Helper()
	orig := trackerLookPath
	t.Cleanup(func() { trackerLookPath = orig })
	trackerLookPath = func(name string) (string, error) {
		switch name {
		case trackerBR:
			if br {
				return "/fake/bin/br", nil
			}
		case trackerBD:
			if bd {
				return "/fake/bin/bd", nil
			}
		}
		return "", exec.ErrNotFound
	}
}

func TestResolveTracker(t *testing.T) {
	cases := []struct {
		name        string
		makeBeads   bool   // create _beads (br ledger)
		makeBd      bool   // create .beads (bd ledger)
		configYAML  string // written to <root>/.agentops/config.yaml when non-empty
		envTracker  string // AGENTOPS_TRACKER value ("" = unset)
		lookBR      bool   // trackerLookPath finds br
		lookBD      bool   // trackerLookPath finds bd
		wantTracker string
		wantSource  string
		wantLedger  string // "_beads" | ".beads" (joined to root)
		wantBinary  string // asserted only when set
		wantErr     bool
	}{
		{
			// Backward-compat anchor: a _beads repo resolves to br + _beads.
			name: "br ledger present", makeBeads: true, lookBR: true, lookBD: true,
			wantTracker: trackerBR, wantSource: trackerSourceLedger, wantLedger: "_beads",
		},
		{
			name: "bd ledger only", makeBd: true, lookBR: true, lookBD: true,
			wantTracker: trackerBD, wantSource: trackerSourceLedger, wantLedger: ".beads",
		},
		{
			// Deterministic tie-break: both dirs present -> br (this repo's case).
			name: "both present -> br", makeBeads: true, makeBd: true, lookBR: true, lookBD: true,
			wantTracker: trackerBR, wantSource: trackerSourceLedger, wantLedger: "_beads",
		},
		{
			name: "neither ledger, only bd binary", lookBD: true,
			wantTracker: trackerBD, wantSource: trackerSourceBinary, wantLedger: ".beads", wantBinary: "/fake/bin/bd",
		},
		{
			name: "neither ledger, only br binary", lookBR: true,
			wantTracker: trackerBR, wantSource: trackerSourceBinary, wantLedger: "_beads", wantBinary: "/fake/bin/br",
		},
		{
			// Explicit override beats a present br ledger.
			name: "env override wins over ledger", makeBeads: true, envTracker: "bd", lookBR: true, lookBD: true,
			wantTracker: trackerBD, wantSource: trackerSourceEnv, wantLedger: ".beads",
		},
		{
			// Config key beats a present br ledger (and proves config is read).
			name: "config wins over ledger", makeBeads: true, configYAML: "tracker: bd\n", lookBR: true, lookBD: true,
			wantTracker: trackerBD, wantSource: trackerSourceConfig, wantLedger: ".beads",
		},
		{
			// Env beats config when both are set.
			name: "env beats config", configYAML: "tracker: br\n", envTracker: "bd", lookBR: true, lookBD: true,
			wantTracker: trackerBD, wantSource: trackerSourceEnv, wantLedger: ".beads",
		},
		{
			name: "no ledger and no binary -> error", wantErr: true,
		},
		{
			name: "invalid env value -> error", envTracker: "mercurial", lookBR: true, lookBD: true, wantErr: true,
		},
		{
			name: "invalid config value -> error", configYAML: "tracker: svn\n", lookBR: true, lookBD: true, wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := makeGitRepoForTracker(t)
			if tc.makeBeads {
				if err := os.Mkdir(filepath.Join(root, "_beads"), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			if tc.makeBd {
				if err := os.Mkdir(filepath.Join(root, ".beads"), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			if tc.configYAML != "" {
				if err := os.Mkdir(filepath.Join(root, ".agentops"), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(root, ".agentops", "config.yaml"), []byte(tc.configYAML), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			setTrackerLookPath(t, tc.lookBR, tc.lookBD)

			// HOME points at an empty temp dir so no real ~/.agentops/config.yaml
			// leaks into the resolution.
			env := []string{"HOME=" + t.TempDir()}
			if tc.envTracker != "" {
				env = append(env, "AGENTOPS_TRACKER="+tc.envTracker)
			}

			res, err := resolveTracker(root, env)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("resolveTracker(%s) = %+v, want error", tc.name, res)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolveTracker(%s) unexpected error: %v", tc.name, err)
			}
			if res.Tracker != tc.wantTracker {
				t.Errorf("Tracker = %q, want %q", res.Tracker, tc.wantTracker)
			}
			if res.Source != tc.wantSource {
				t.Errorf("Source = %q, want %q", res.Source, tc.wantSource)
			}
			wantDir := filepath.Join(root, tc.wantLedger)
			if res.LedgerDir != wantDir {
				t.Errorf("LedgerDir = %q, want %q", res.LedgerDir, wantDir)
			}
			if tc.wantBinary != "" && res.Binary != tc.wantBinary {
				t.Errorf("Binary = %q, want %q", res.Binary, tc.wantBinary)
			}
		})
	}
}

// TestResolveTrackerErrorNamesBothInstallPaths pins the unresolvable-case error
// to the actionable install text (precedence d).
func TestResolveTrackerErrorNamesBothInstallPaths(t *testing.T) {
	root := makeGitRepoForTracker(t)
	setTrackerLookPath(t, false, false)
	_, err := resolveTracker(root, []string{"HOME=" + t.TempDir()})
	if err == nil {
		t.Fatal("resolveTracker with no ledger/binary = nil error, want actionable error")
	}
	msg := err.Error()
	for _, want := range []string{"brew install boshu2/agentops/br", "brew install beads", "AGENTOPS_TRACKER"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error missing %q:\n%s", want, msg)
		}
	}
}

// setBeadsTrackerJSON flips the --json cobra global for one test and restores
// it (shared rootCmd state; see .claude/rules/go.md test-isolation rule).
func setBeadsTrackerJSON(t *testing.T, v bool) {
	t.Helper()
	orig := beadsTrackerJSON
	t.Cleanup(func() { beadsTrackerJSON = orig })
	beadsTrackerJSON = v
}

func TestRunBeadsTrackerCmdJSON(t *testing.T) {
	root := makeGitRepoForTracker(t)
	if err := os.Mkdir(filepath.Join(root, "_beads"), 0o755); err != nil {
		t.Fatal(err)
	}
	setTrackerLookPath(t, true, true)
	setBeadsTrackerJSON(t, true)
	t.Setenv("AGENTOPS_TRACKER", "")
	t.Setenv("BEADS_DIR", "")
	t.Setenv("HOME", t.TempDir())
	t.Setenv("P3_TRACKER_SECRET", "must-not-serialize")
	t.Chdir(root)

	var out strings.Builder
	cmd := beadsTrackerCmd
	cmd.SetOut(&out)
	t.Cleanup(func() { cmd.SetOut(nil) })
	if err := runBeadsTrackerCmd(cmd, nil); err != nil {
		t.Fatalf("runBeadsTrackerCmd: %v", err)
	}
	var res trackerResolution
	if err := json.Unmarshal([]byte(out.String()), &res); err != nil {
		t.Fatalf("json output: %v: %s", err, out.String())
	}
	if res.Tracker != trackerBR {
		t.Errorf("tracker = %q, want br", res.Tracker)
	}
	if res.Source != trackerSourceLedger {
		t.Errorf("source = %q, want ledger", res.Source)
	}
	if res.LedgerDir != filepath.Join(root, "_beads") {
		t.Errorf("ledger_dir = %q, want %q", res.LedgerDir, filepath.Join(root, "_beads"))
	}
	if res.Binary != "/fake/bin/br" {
		t.Errorf("binary = %q, want /fake/bin/br", res.Binary)
	}
	var payload map[string]json.RawMessage
	if err := json.Unmarshal([]byte(out.String()), &payload); err != nil {
		t.Fatalf("json object: %v: %s", err, out.String())
	}
	if len(payload) != 4 {
		t.Fatalf("JSON keys = %v, want exactly tracker, binary, ledger_dir, source", payload)
	}
	for _, key := range []string{"tracker", "binary", "ledger_dir", "source"} {
		if _, ok := payload[key]; !ok {
			t.Errorf("JSON missing public key %q: %v", key, payload)
		}
	}
	if strings.Contains(out.String(), "P3_TRACKER_SECRET") || strings.Contains(out.String(), "must-not-serialize") {
		t.Fatalf("JSON leaked ambient child environment: %s", out.String())
	}
}

// TestRunBeadsDirBdRepoReturnsBeadsDir proves `ao beads dir` returns the bd
// .beads directory in a bd-only repo (the tracker-agnostic behavior).
func TestRunBeadsDirBdRepoReturnsBeadsDir(t *testing.T) {
	root := makeGitRepoForTracker(t)
	beadsDir := filepath.Join(root, ".beads")
	if err := os.Mkdir(beadsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	setTrackerLookPath(t, true, true)
	t.Setenv("AGENTOPS_TRACKER", "")
	t.Setenv("BEADS_DIR", "")
	t.Setenv("HOME", t.TempDir())
	t.Chdir(root)

	origJSON := beadsDirJSON
	t.Cleanup(func() { beadsDirJSON = origJSON })
	beadsDirJSON = false

	var out strings.Builder
	cmd := beadsDirCmd
	cmd.SetOut(&out)
	t.Cleanup(func() { cmd.SetOut(nil) })
	if err := runBeadsDir(cmd, nil); err != nil {
		t.Fatalf("runBeadsDir: %v", err)
	}
	if got := strings.TrimSpace(out.String()); got != beadsDir {
		t.Fatalf("beads dir in bd repo = %q, want %q", got, beadsDir)
	}
}

// TestRunBeadsDirBothPresentStaysBr is the backward-compat guard: in a repo
// with both _beads and .beads (this repo's shape), `ao beads dir` must still
// return the br _beads dir with the historical git-common-dir source.
func TestRunBeadsDirBothPresentStaysBr(t *testing.T) {
	root := makeGitRepoForTracker(t)
	beadsDir := filepath.Join(root, "_beads")
	if err := os.Mkdir(beadsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, ".beads"), 0o755); err != nil {
		t.Fatal(err)
	}
	setTrackerLookPath(t, true, true)
	t.Setenv("AGENTOPS_TRACKER", "")
	t.Setenv("BEADS_DIR", "")
	t.Setenv("HOME", t.TempDir())
	t.Chdir(root)

	origJSON := beadsDirJSON
	t.Cleanup(func() { beadsDirJSON = origJSON })
	beadsDirJSON = true

	var out strings.Builder
	cmd := beadsDirCmd
	cmd.SetOut(&out)
	t.Cleanup(func() { cmd.SetOut(nil) })
	if err := runBeadsDir(cmd, nil); err != nil {
		t.Fatalf("runBeadsDir: %v", err)
	}
	var payload map[string]string
	if err := json.Unmarshal([]byte(out.String()), &payload); err != nil {
		t.Fatalf("json output: %v: %s", err, out.String())
	}
	if payload["beads_dir"] != beadsDir {
		t.Errorf("beads_dir = %q, want br dir %q", payload["beads_dir"], beadsDir)
	}
	if payload["source"] != beadsDirSourceGitCommon {
		t.Errorf("source = %q, want %q (br path unchanged)", payload["source"], beadsDirSourceGitCommon)
	}
}
