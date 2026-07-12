package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestPawlReviewUseDocumentsSmokeFlag guards that `ao pawl review`'s Use line documents
// the --smoke reviewer-optional lane (parsed verbatim by scripts/pawl-review.sh). The crank
// land-protocol reference (age-e508.3) cites `ao pawl review <bead> --scope head --smoke
// "<check>"` as the false-REFUTE recovery command; the skill cli-snippets + body-refs gates
// resolve that flag against this command's help text, so --smoke must stay discoverable here
// or those gates false-fail the referenced command.
func TestPawlReviewUseDocumentsSmokeFlag(t *testing.T) {
	if !strings.Contains(pawlReviewCmd.Use, "--smoke") {
		t.Fatalf("pawl review Use line must document --smoke (the reviewer-optional live-smoke lane); got %q", pawlReviewCmd.Use)
	}
}

func TestPawlReviewUseDocumentsUpstreamScope(t *testing.T) {
	if !strings.Contains(pawlReviewCmd.Use, "head|staged|upstream") {
		t.Fatalf("pawl review Use line must document the full configured-upstream range; got %q", pawlReviewCmd.Use)
	}
	if !strings.Contains(pawlReviewCmd.Use, "--base <sha>") {
		t.Fatalf("pawl review Use line must document exact range-base pinning; got %q", pawlReviewCmd.Use)
	}
}

func TestValidPawlRouteID(t *testing.T) {
	for _, tc := range []struct {
		id   string
		want bool
	}{
		{id: "age-ghk3i.12", want: true},
		{id: "A_route-9", want: true},
		{id: "_leading-separator", want: false},
		{id: "contains/slash", want: false},
		{id: strings.Repeat("a", 65), want: false},
	} {
		if got := validPawlRouteID(tc.id); got != tc.want {
			t.Errorf("validPawlRouteID(%q) = %v, want %v", tc.id, got, tc.want)
		}
	}
}

// writePawlTestRepo builds a minimal repo root resolveAgentsRepoRoot() accepts
// (docs/contracts/agents-write-surfaces.md + skills/) plus a stub scripts/pawl-review.sh
// that exits with the given code, and points testProjectDir at it. Restores all shared
// global state via t.Cleanup (the cmd/ao test-isolation contract).
func writePawlTestRepo(t *testing.T, exitCode int) {
	t.Helper()
	repo := t.TempDir()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(os.MkdirAll(filepath.Join(repo, "docs", "contracts"), 0o755))
	must(os.WriteFile(filepath.Join(repo, "docs", "contracts", "agents-write-surfaces.md"), []byte("ok"), 0o644))
	must(os.MkdirAll(filepath.Join(repo, "skills"), 0o755))
	must(os.MkdirAll(filepath.Join(repo, "scripts"), 0o755))
	stub := "#!/usr/bin/env bash\nexit " + strconv.Itoa(exitCode) + "\n"
	must(os.WriteFile(filepath.Join(repo, "scripts", "pawl-review.sh"), []byte(stub), 0o755))
	// Simulate the GENUINE-checkout dogfood: the live-script path is taken only when the
	// running ao binary physically lives inside the resolved checkout (forge-proof trust),
	// so place a dummy binary inside repo/cli/bin and point pawlSelfBinary at it.
	must(os.MkdirAll(filepath.Join(repo, "cli", "bin"), 0o755))
	selfAo := filepath.Join(repo, "cli", "bin", "ao")
	must(os.WriteFile(selfAo, []byte("dummy"), 0o755))

	prevDir := testProjectDir
	testProjectDir = repo
	prevSelf := pawlSelfBinary
	pawlSelfBinary = func() (string, error) { return selfAo, nil }
	// runPawlReview mutates these shared cobra-command flags on error — restore them.
	prevSU, prevSE := pawlReviewCmd.SilenceUsage, pawlReviewCmd.SilenceErrors
	t.Cleanup(func() {
		testProjectDir = prevDir
		pawlSelfBinary = prevSelf
		pawlReviewCmd.SilenceUsage = prevSU
		pawlReviewCmd.SilenceErrors = prevSE
	})
}

// The exit code IS the verdict in `ao pawl review`; it must propagate the wrapped
// script's code verbatim (0 CONFIRMED · 3 REFUTED · 4 converge-advisory · 2 usage).
func TestRunPawlReview_PropagatesScriptExitCode(t *testing.T) {
	cases := []struct {
		name     string
		code     int
		wantNil  bool
		wantCode int
	}{
		{"CONFIRMED exit 0 -> nil", 0, true, 0},
		{"REFUTED exit 3 -> exitErr 3", 3, false, 3},
		{"converge advisory exit 4 -> exitErr 4", 4, false, 4},
		{"usage exit 2 -> exitErr 2", 2, false, 2},
		{"hard error exit 1 -> exitErr 1", 1, false, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			writePawlTestRepo(t, tc.code)
			err := runPawlReview(pawlReviewCmd, []string{"age-test", "--scope", "head"})
			if tc.wantNil {
				if err != nil {
					t.Fatalf("exit %d: want nil error, got %v", tc.code, err)
				}
				return
			}
			var exitErr *pawlReviewExitError
			if !errors.As(err, &exitErr) {
				t.Fatalf("exit %d: want *pawlReviewExitError, got %T: %v", tc.code, err, err)
			}
			if exitErr.ExitCode() != tc.wantCode {
				t.Fatalf("ExitCode() = %d, want %d", exitErr.ExitCode(), tc.wantCode)
			}
		})
	}
}

// --help is for ao's command, not the wrapped script — it must NOT forward (which would
// run the exit-3 stub) and must return nil.
func TestRunPawlReview_HelpDoesNotForwardToScript(t *testing.T) {
	writePawlTestRepo(t, 3)
	if err := runPawlReview(pawlReviewCmd, []string{"--help"}); err != nil {
		t.Fatalf("--help should return nil (print help, not forward), got %v", err)
	}
}

// A missing script is a hard error, not a silent success.
func TestRunPawlReview_MissingScriptErrors(t *testing.T) {
	writePawlTestRepo(t, 0)
	repo := testProjectDir
	if err := os.Remove(filepath.Join(repo, "scripts", "pawl-review.sh")); err != nil {
		t.Fatal(err)
	}
	err := runPawlReview(pawlReviewCmd, []string{"age-test"})
	if err == nil {
		t.Fatal("missing pawl-review.sh should error, got nil")
	}
	var exitErr *pawlReviewExitError
	if errors.As(err, &exitErr) {
		t.Fatalf("missing script should be a plain error, not a propagated exit code, got %v", err)
	}
}

// writePawlServiceTestRepo is writePawlTestRepo's sibling for the standing-service script:
// a stub scripts/pawl.sh that exits with the given code. pawlServiceCmd builds a FRESH
// cobra command per call (no package-global mutation), so only testProjectDir needs restore.
func writePawlServiceTestRepo(t *testing.T, exitCode int) {
	t.Helper()
	repo := t.TempDir()
	mk := func(rel string, b []byte, m os.FileMode) {
		t.Helper()
		p := filepath.Join(repo, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, b, m); err != nil {
			t.Fatal(err)
		}
	}
	mk(filepath.Join("docs", "contracts", "agents-write-surfaces.md"), []byte("ok"), 0o644)
	if err := os.MkdirAll(filepath.Join(repo, "skills"), 0o755); err != nil {
		t.Fatal(err)
	}
	mk(filepath.Join("scripts", "pawl.sh"), []byte("#!/usr/bin/env bash\nexit "+strconv.Itoa(exitCode)+"\n"), 0o755)
	// Simulate the GENUINE-checkout dogfood (age-l3xj D4): service verbs take the live
	// script only when the running ao binary physically lives inside the checkout.
	selfAo := filepath.Join(repo, "cli", "bin", "ao")
	mk(filepath.Join("cli", "bin", "ao"), []byte("dummy"), 0o755)
	prevDir := testProjectDir
	prevSelf := pawlSelfBinary
	testProjectDir = repo
	pawlSelfBinary = func() (string, error) { return selfAo, nil }
	t.Cleanup(func() {
		testProjectDir = prevDir
		pawlSelfBinary = prevSelf
	})
}

// ml8: `ao pawl up/down/health/doctor/smoke/route/metrics` forward to scripts/pawl.sh and propagate
// its exit code verbatim (so e.g. a REFUTED route exits non-zero through ao).
func TestPawlServiceCmd_DelegatesAndPropagatesExitCode(t *testing.T) {
	for _, code := range []int{0, 1, 2} {
		t.Run("exit-"+strconv.Itoa(code), func(t *testing.T) {
			writePawlServiceTestRepo(t, code)
			cmd := pawlServiceCmd("metrics", "metrics", "SLOs")
			err := cmd.RunE(cmd, nil)
			if code == 0 {
				if err != nil {
					t.Fatalf("exit 0: want nil, got %v", err)
				}
				return
			}
			var exitErr *pawlReviewExitError
			if !errors.As(err, &exitErr) {
				t.Fatalf("exit %d: want *pawlReviewExitError, got %T: %v", code, err, err)
			}
			if exitErr.ExitCode() != code {
				t.Fatalf("ExitCode() = %d, want %d", exitErr.ExitCode(), code)
			}
		})
	}
}

// The `ao pawl` surface exposes the full standing-service contract, not just `review`.
func TestPawlCmd_HasServiceSubcommands(t *testing.T) {
	want := map[string]bool{"up": false, "down": false, "reap": false, "health": false, "doctor": false, "smoke": false, "route": false, "metrics": false, "review": false}
	for _, c := range pawlCmd.Commands() {
		if _, ok := want[c.Name()]; ok {
			want[c.Name()] = true
		}
	}
	for name, found := range want {
		if !found {
			t.Errorf("ao pawl is missing subcommand %q", name)
		}
	}
}

func TestPawlCmd_DoctorSmokeRegisteredOnRoot(t *testing.T) {
	for _, path := range [][]string{
		{"pawl", "doctor"},
		{"pawl", "smoke"},
	} {
		cmd, _, err := rootCmd.Find(path)
		if err != nil {
			t.Fatalf("rootCmd.Find(%v): %v", path, err)
		}
		if cmd == rootCmd {
			t.Fatalf("rootCmd.Find(%v) returned rootCmd, not a leaf command", path)
		}
	}
}

// setPawlGlobalFlags flips the shared rootCmd persistent-flag globals for one test and
// restores them via t.Cleanup (the cmd/ao shuffle-order isolation contract: every
// package-global set-site self-cleans).
func setPawlGlobalFlags(t *testing.T, dry, jsonOut bool) {
	t.Helper()
	prevDry, prevJSON := dryRun, jsonFlag
	dryRun, jsonFlag = dry, jsonOut
	t.Cleanup(func() { dryRun, jsonFlag = prevDry, prevJSON })
}

// writeForgedPawlServiceRepo builds a repo that FORGES the AgentOps markers and plants a
// sentinel-touching scripts/pawl.sh (+ pawl-review.sh). The planted scripts must NEVER
// run under --dry-run (D1) — and never at all from an installed binary (D4, embed tests).
// insideBinary=true simulates the genuine-checkout dogfood (ao binary inside the repo).
func writeForgedPawlServiceRepo(t *testing.T, insideBinary bool) (repo, sentinel string) {
	t.Helper()
	repo = t.TempDir()
	sentinel = filepath.Join(repo, "PWNED")
	mk := func(rel string, b []byte, m os.FileMode) {
		t.Helper()
		p := filepath.Join(repo, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, b, m); err != nil {
			t.Fatal(err)
		}
	}
	mk(filepath.Join("docs", "contracts", "agents-write-surfaces.md"), []byte("ok"), 0o644)
	if err := os.MkdirAll(filepath.Join(repo, "skills"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	hostile := []byte("#!/usr/bin/env bash\ntouch " + sentinel + "\nexit 0\n")
	mk(filepath.Join("scripts", "pawl.sh"), hostile, 0o755)
	mk(filepath.Join("scripts", "pawl-review.sh"), hostile, 0o755)

	prevDir := testProjectDir
	testProjectDir = repo
	prevSelf := pawlSelfBinary
	if insideBinary {
		selfAo := filepath.Join(repo, "cli", "bin", "ao")
		mk(filepath.Join("cli", "bin", "ao"), []byte("dummy"), 0o755)
		pawlSelfBinary = func() (string, error) { return selfAo, nil }
	} else {
		outside := filepath.Join(t.TempDir(), "ao")
		if err := os.WriteFile(outside, []byte("#!/usr/bin/env bash\nexit 0\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		pawlSelfBinary = func() (string, error) { return outside, nil }
	}
	prevSU, prevSE := pawlReviewCmd.SilenceUsage, pawlReviewCmd.SilenceErrors
	t.Cleanup(func() {
		testProjectDir = prevDir
		pawlSelfBinary = prevSelf
		pawlReviewCmd.SilenceUsage = prevSU
		pawlReviewCmd.SilenceErrors = prevSE
	})
	return repo, sentinel
}

// snapshotDirState fingerprints a tree (relative path + size) so dry-run tests can prove
// byte-for-byte "nothing was created, removed, or rewritten" (the D1 acceptance:
// dry-run is side-effect free).
func snapshotDirState(t *testing.T, root string) map[string]int64 {
	t.Helper()
	out := map[string]int64{}
	err := filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, rerr := filepath.Rel(root, p)
		if rerr != nil {
			return rerr
		}
		if info.IsDir() {
			out[rel+"/"] = 0
			return nil
		}
		out[rel] = info.Size()
		return nil
	})
	if err != nil {
		t.Fatalf("snapshot %s: %v", root, err)
	}
	return out
}

func diffDirState(a, b map[string]int64) []string {
	var d []string
	for k, v := range b {
		if av, ok := a[k]; !ok {
			d = append(d, "created: "+k)
		} else if av != v {
			d = append(d, "rewritten: "+k)
		}
	}
	for k := range a {
		if _, ok := b[k]; !ok {
			d = append(d, "removed: "+k)
		}
	}
	return d
}

// TestPawlService_DryRunIsSideEffectFree is the D1 regression (observed in production:
// `ao --dry-run --json pawl up` CREATED agentops--pawl-service). Every MUTATING pawl
// command under global --dry-run must not execute the service script at all — even on
// the trusted in-checkout path — and must leave the filesystem untouched.
func TestPawlService_DryRunIsSideEffectFree(t *testing.T) {
	cases := []struct {
		sub  string
		args []string
	}{
		{"up", nil},
		{"up", []string{"--models", "cc,cod"}},
		{"down", nil},
		{"reap", nil},
		{"route", []string{"age-x", "packet.md"}},
	}
	for _, tc := range cases {
		t.Run(tc.sub+"/"+strings.Join(tc.args, "_"), func(t *testing.T) {
			repo, sentinel := writeForgedPawlServiceRepo(t, true)
			setPawlGlobalFlags(t, true, true)
			pre := snapshotDirState(t, repo)

			cmd := pawlServiceCmd(tc.sub, tc.sub, "test")
			var out strings.Builder
			cmd.SetOut(&out)
			cmd.SetErr(&out)
			if err := cmd.RunE(cmd, tc.args); err != nil {
				t.Fatalf("dry-run %s: want nil error, got %v", tc.sub, err)
			}
			if _, err := os.Stat(sentinel); err == nil {
				t.Fatalf("SECURITY/D1: dry-run %s EXECUTED scripts/pawl.sh (sentinel touched)", tc.sub)
			}
			if d := diffDirState(pre, snapshotDirState(t, repo)); len(d) != 0 {
				t.Fatalf("dry-run %s mutated the filesystem: %v", tc.sub, d)
			}
			if out.Len() == 0 {
				t.Fatalf("dry-run %s: must report the planned action, got empty output", tc.sub)
			}
		})
	}
}

// TestPawlReview_DryRunIsSideEffectFree extends D1 to `ao pawl review` (a verdict write
// is a mutation): under --dry-run the review script must never run.
func TestPawlReview_DryRunIsSideEffectFree(t *testing.T) {
	repo, sentinel := writeForgedPawlServiceRepo(t, true)
	setPawlGlobalFlags(t, true, false)
	pre := snapshotDirState(t, repo)

	var out strings.Builder
	pawlReviewCmd.SetOut(&out)
	pawlReviewCmd.SetErr(&out)
	t.Cleanup(func() { pawlReviewCmd.SetOut(nil); pawlReviewCmd.SetErr(nil) })
	if err := runPawlReview(pawlReviewCmd, []string{"age-x", "--scope", "head"}); err != nil {
		t.Fatalf("dry-run review: want nil error, got %v", err)
	}
	if _, err := os.Stat(sentinel); err == nil {
		t.Fatal("SECURITY/D1: dry-run review EXECUTED scripts/pawl-review.sh")
	}
	if d := diffDirState(pre, snapshotDirState(t, repo)); len(d) != 0 {
		t.Fatalf("dry-run review mutated the filesystem: %v", d)
	}
	if !strings.Contains(out.String(), "DRY-RUN") {
		t.Fatalf("dry-run review must report the planned action; got: %q", out.String())
	}
}

// TestPawlService_DryRunJSONContract is the D2 regression: with global --json, a dry-run
// mutating command emits EXACTLY ONE parseable JSON object carrying at least action,
// dry_run, mutated, session, families, tier, planned_steps — no human log lines.
func TestPawlService_DryRunJSONContract(t *testing.T) {
	cases := []struct {
		sub          string
		args         []string
		wantFamilies string
		wantTier     string
	}{
		{"up", []string{"--models", "cc,cod"}, "cc cod", "multi"},
		{"up", []string{"--dual"}, "cc cod", "multi"},
		{"up", []string{"--tri"}, "cc cod agy", "multi"},
		{"up", []string{"--models", "claude"}, "cc", "fresh"},
		{"up", nil, "adaptive", "adaptive"},
		{"down", nil, "", ""},
		{"reap", nil, "", ""},
		{"route", []string{"age-x", "p.md"}, "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.sub+"/"+strings.Join(tc.args, "_"), func(t *testing.T) {
			writeForgedPawlServiceRepo(t, true)
			setPawlGlobalFlags(t, true, true)

			cmd := pawlServiceCmd(tc.sub, tc.sub, "test")
			var out strings.Builder
			cmd.SetOut(&out)
			cmd.SetErr(&out)
			if err := cmd.RunE(cmd, tc.args); err != nil {
				t.Fatalf("dry-run %s: %v", tc.sub, err)
			}
			var doc map[string]any
			if err := json.Unmarshal([]byte(out.String()), &doc); err != nil {
				t.Fatalf("D2: output is not exactly one JSON document: %v\n%s", err, out.String())
			}
			for _, k := range []string{"action", "dry_run", "mutated", "session", "families", "tier", "planned_steps"} {
				if _, ok := doc[k]; !ok {
					t.Fatalf("dry-run JSON missing %q: %s", k, out.String())
				}
			}
			if doc["dry_run"] != true {
				t.Fatalf("dry_run must be true: %s", out.String())
			}
			if doc["mutated"] != false {
				t.Fatalf("mutated must be false: %s", out.String())
			}
			if doc["action"] != "pawl "+tc.sub {
				t.Fatalf("action = %v, want %q", doc["action"], "pawl "+tc.sub)
			}
			if tc.wantFamilies != "" && doc["families"] != tc.wantFamilies {
				t.Fatalf("families = %v, want %q", doc["families"], tc.wantFamilies)
			}
			if tc.wantTier != "" && doc["tier"] != tc.wantTier {
				t.Fatalf("tier = %v, want %q", doc["tier"], tc.wantTier)
			}
			steps, ok := doc["planned_steps"].([]any)
			if !ok || len(steps) == 0 {
				t.Fatalf("planned_steps must be a non-empty array: %s", out.String())
			}
		})
	}
}

// TestPawlPlannedSession is the refuter round-8 regression: the dry-run planner must derive
// the session EXACTLY as scripts/pawl.sh does (${PROJECT}--${LABEL}), not hardcode
// "agentops--pawl-service" — else the plan misreports the target from any other repo.
func TestPawlPlannedSession(t *testing.T) {
	t.Run("PAWL_SESSION wins", func(t *testing.T) {
		t.Setenv("PAWL_SESSION", "my--custom")
		if got := pawlPlannedSession(); got != "my--custom" {
			t.Fatalf("got %q, want my--custom", got)
		}
	})
	t.Run("PAWL_PROJECT + default label", func(t *testing.T) {
		t.Setenv("PAWL_SESSION", "")
		t.Setenv("PAWL_PROJECT", "personal-site")
		t.Setenv("PAWL_LABEL", "")
		if got := pawlPlannedSession(); got != "personal-site--pawl-service" {
			t.Fatalf("got %q, want personal-site--pawl-service", got)
		}
	})
	t.Run("derives basename of the resolved repo", func(t *testing.T) {
		t.Setenv("PAWL_SESSION", "")
		t.Setenv("PAWL_PROJECT", "")
		t.Setenv("PAWL_LABEL", "")
		repo := t.TempDir()
		if err := os.MkdirAll(filepath.Join(repo, "sub", ".git"), 0o755); err != nil {
			// place .git at repo root instead
			if err2 := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err2 != nil {
				t.Fatal(err2)
			}
		}
		if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
			t.Fatal(err)
		}
		prev := testProjectDir
		testProjectDir = repo
		t.Cleanup(func() { testProjectDir = prev })
		want := filepath.Base(repo) + "--pawl-service"
		if got := pawlPlannedSession(); got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	})
	t.Run("plan reports the derived session, not a hardcode", func(t *testing.T) {
		t.Setenv("PAWL_SESSION", "")
		t.Setenv("PAWL_PROJECT", "widgets")
		t.Setenv("PAWL_LABEL", "")
		doc := pawlDryRunPlan("up", nil)
		if doc.Session != "widgets--pawl-service" {
			t.Fatalf("plan session = %q, want widgets--pawl-service", doc.Session)
		}
	})
}

// TestPawlService_DryRunFlagsArriveAsRawArgs is the PRODUCTION-FAITHFUL D1 regression:
// pawl leaves set DisableFlagParsing, so cobra never parses the inherited persistent
// flags — `ao --dry-run --json pawl up` reaches RunE with ["--dry-run","--json"] still
// in args and the dryRun/jsonFlag GLOBALS FALSE. The first fix keyed only off the
// globals and executed the script for real (caught live: it spawned + killed a session
// named agentops--pawl-service). The leaf must extract the flags from raw args itself.
func TestPawlService_DryRunFlagsArriveAsRawArgs(t *testing.T) {
	cases := []struct {
		sub  string
		args []string // exactly what cobra passes through, flags included
	}{
		{"up", []string{"--dry-run", "--json"}},
		{"up", []string{"--json", "--dry-run", "--models", "cc,cod"}},
		{"up", []string{"--models", "cc,cod", "--dry-run"}}, // flag AFTER the subcommand args
		{"down", []string{"--dry-run", "--json"}},
		{"reap", []string{"--dry-run", "--json"}},
		{"route", []string{"--dry-run", "--json", "age-x", "packet.md"}},
	}
	for _, tc := range cases {
		t.Run(tc.sub+"/"+strings.Join(tc.args, "_"), func(t *testing.T) {
			repo, sentinel := writeForgedPawlServiceRepo(t, true)
			// Globals deliberately FALSE — production shape for DisableFlagParsing leaves.
			setPawlGlobalFlags(t, false, false)
			pre := snapshotDirState(t, repo)

			cmd := pawlServiceCmd(tc.sub, tc.sub, "test")
			var out strings.Builder
			cmd.SetOut(&out)
			cmd.SetErr(&out)
			if err := cmd.RunE(cmd, tc.args); err != nil {
				t.Fatalf("raw-arg dry-run %s: want nil error, got %v", tc.sub, err)
			}
			if _, err := os.Stat(sentinel); err == nil {
				t.Fatalf("SECURITY/D1: raw-arg --dry-run %s EXECUTED scripts/pawl.sh", tc.sub)
			}
			if d := diffDirState(pre, snapshotDirState(t, repo)); len(d) != 0 {
				t.Fatalf("raw-arg dry-run %s mutated the filesystem: %v", tc.sub, d)
			}
			wantJSON := false
			for _, a := range tc.args {
				if a == "--json" {
					wantJSON = true
				}
			}
			if wantJSON {
				var doc map[string]any
				if err := json.Unmarshal([]byte(out.String()), &doc); err != nil {
					t.Fatalf("raw-arg --json dry-run must emit exactly one JSON object: %v\n%s", err, out.String())
				}
				if doc["dry_run"] != true || doc["mutated"] != false {
					t.Fatalf("raw-arg dry-run doc wrong: %s", out.String())
				}
			} else if !strings.Contains(out.String(), "DRY-RUN") {
				t.Fatalf("raw-arg dry-run without --json must report the plan in human form, got: %q", out.String())
			}
		})
	}
	// review: same raw-arg shape via runPawlReview
	t.Run("review", func(t *testing.T) {
		repo, sentinel := writeForgedPawlServiceRepo(t, true)
		setPawlGlobalFlags(t, false, false)
		pre := snapshotDirState(t, repo)
		var out strings.Builder
		pawlReviewCmd.SetOut(&out)
		pawlReviewCmd.SetErr(&out)
		t.Cleanup(func() { pawlReviewCmd.SetOut(nil); pawlReviewCmd.SetErr(nil) })
		if err := runPawlReview(pawlReviewCmd, []string{"--dry-run", "--json", "age-x", "--scope", "head"}); err != nil {
			t.Fatalf("raw-arg dry-run review: %v", err)
		}
		if _, err := os.Stat(sentinel); err == nil {
			t.Fatal("SECURITY/D1: raw-arg --dry-run review EXECUTED scripts/pawl-review.sh")
		}
		if d := diffDirState(pre, snapshotDirState(t, repo)); len(d) != 0 {
			t.Fatalf("raw-arg dry-run review mutated the filesystem: %v", d)
		}
		var doc map[string]any
		if err := json.Unmarshal([]byte(out.String()), &doc); err != nil {
			t.Fatalf("review raw-arg --json dry-run must emit one JSON object: %v\n%s", err, out.String())
		}
	})
}

// TestStripPawlPassthroughFlags_EqualsForm is the refuter regression on the first cut:
// matching only the BARE `--dry-run` token let cobra's standard `--dry-run=true` form fall
// through — `ao --dry-run=true --json pawl up` spawned the service for real, and `pawl route`
// consumed the flag as its bead id. Both forms must be recognized, and an unparseable value
// fails CLOSED (dry-run wins over a mutation).
func TestStripPawlPassthroughFlags_EqualsForm(t *testing.T) {
	cases := []struct {
		name     string
		args     []string
		wantRest []string
		wantDry  bool
		wantJSON bool
	}{
		{"bare", []string{"--dry-run", "--json", "age-x"}, []string{"age-x"}, true, true},
		{"equals true", []string{"--dry-run=true", "--json=true", "age-x"}, []string{"age-x"}, true, true},
		{"equals false", []string{"--dry-run=false", "--json=false", "age-x"}, []string{"age-x"}, false, false},
		{"equals 1/0", []string{"--dry-run=1", "--json=0"}, nil, true, false},
		{"unparseable fails closed", []string{"--dry-run=maybe"}, nil, true, false},
		{"mixed", []string{"--models", "cc,cod", "--dry-run=true"}, []string{"--models", "cc,cod"}, true, false},
		{"none", []string{"age-x", "packet.md"}, []string{"age-x", "packet.md"}, false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rest, dry, jsonOut := stripPawlPassthroughFlags(tc.args)
			if dry != tc.wantDry || jsonOut != tc.wantJSON {
				t.Fatalf("stripPawlPassthroughFlags(%v) = dry %v json %v, want dry %v json %v", tc.args, dry, jsonOut, tc.wantDry, tc.wantJSON)
			}
			if strings.Join(rest, " ") != strings.Join(tc.wantRest, " ") {
				t.Fatalf("rest = %v, want %v", rest, tc.wantRest)
			}
		})
	}
}

// TestPawlDryRunValidate is the refuter round-24 regression: a dry-run must report the EXACT
// planned action, so invalid args (missing route bead/packet, or an unknown --models family)
// fail like the real command instead of reporting a bogus successful plan.
func TestPawlDryRunValidate(t *testing.T) {
	cases := []struct {
		name    string
		sub     string
		args    []string
		wantErr bool
	}{
		{"route ok", "route", []string{"age-x", "packet.md"}, false},
		{"route missing packet", "route", []string{"age-x"}, true},
		{"route no args", "route", nil, true},
		{"route bad id", "route", []string{"../evil", "packet.md"}, true},
		{"route leading-dash bead (positional, not skipped)", "route", []string{"--bogus", "age-x", "packet.md"}, true},
		{"up ok models", "up", []string{"--models", "cc,cod"}, false},
		{"up dual", "up", []string{"--dual"}, false},
		{"up unknown model", "up", []string{"--models", "nope"}, true},
		{"up unknown among valid", "up", []string{"--models", "cc,nope"}, true},
		{"up adaptive", "up", nil, false},
		{"down", "down", nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := pawlDryRunValidate(tc.sub, tc.args)
			if (err != nil) != tc.wantErr {
				t.Fatalf("pawlDryRunValidate(%q,%v) err=%v, wantErr=%v", tc.sub, tc.args, err, tc.wantErr)
			}
		})
	}
}

// The end-to-end half: a dry-run with invalid args returns a non-zero error through the command.
func TestPawlService_DryRunInvalidArgsError(t *testing.T) {
	writeForgedPawlServiceRepo(t, true)
	setPawlGlobalFlags(t, true, true)
	cmd := pawlServiceCmd("route", "route", "test")
	var out strings.Builder
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.RunE(cmd, nil); err == nil { // route with no bead/packet
		t.Fatal("dry-run route with no args must error (match the real command), got nil")
	}
}

// TestPawlService_EqualsFormDryRunIsSideEffectFree: the end-to-end half of the same catch —
// `--dry-run=true` must plan, not execute, for every mutating verb.
func TestPawlService_EqualsFormDryRunIsSideEffectFree(t *testing.T) {
	for _, tc := range []struct {
		sub  string
		args []string // valid args so the dry-run VALIDATES (route needs bead+packet)
	}{
		{"up", nil}, {"down", nil}, {"reap", nil}, {"route", []string{"age-x", "packet.md"}},
	} {
		sub := tc.sub
		t.Run(sub, func(t *testing.T) {
			repo, sentinel := writeForgedPawlServiceRepo(t, true)
			setPawlGlobalFlags(t, false, false)
			pre := snapshotDirState(t, repo)

			cmd := pawlServiceCmd(sub, sub, "test")
			var out strings.Builder
			cmd.SetOut(&out)
			cmd.SetErr(&out)
			if err := cmd.RunE(cmd, append([]string{"--dry-run=true", "--json=true"}, tc.args...)); err != nil {
				t.Fatalf("--dry-run=true %s: %v", sub, err)
			}
			if _, err := os.Stat(sentinel); err == nil {
				t.Fatalf("SECURITY: --dry-run=true %s EXECUTED scripts/pawl.sh", sub)
			}
			if d := diffDirState(pre, snapshotDirState(t, repo)); len(d) != 0 {
				t.Fatalf("--dry-run=true %s mutated the filesystem: %v", sub, d)
			}
			var doc map[string]any
			if err := json.Unmarshal([]byte(out.String()), &doc); err != nil {
				t.Fatalf("--json=true must emit one JSON object: %v\n%s", err, out.String())
			}
		})
	}
}

// TestPawlService_ReadOnlyCmdsInspectUnderDryRun: health/doctor/smoke/metrics MAY run
// under --dry-run (they inspect real state) but must be marked non-mutating via
// PAWL_DRY_RUN=1 so even prompt-clearing key sends are suppressed script-side.
func TestPawlService_ReadOnlyCmdsInspectUnderDryRun(t *testing.T) {
	for _, sub := range []string{"health", "doctor", "smoke", "metrics"} {
		t.Run(sub, func(t *testing.T) {
			repo := t.TempDir()
			mk := func(rel string, b []byte, m os.FileMode) {
				t.Helper()
				p := filepath.Join(repo, rel)
				if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(p, b, m); err != nil {
					t.Fatal(err)
				}
			}
			mk(filepath.Join("docs", "contracts", "agents-write-surfaces.md"), []byte("ok"), 0o644)
			if err := os.MkdirAll(filepath.Join(repo, "skills"), 0o755); err != nil {
				t.Fatal(err)
			}
			// The stub asserts the dry-run seam: exit 0 iff PAWL_DRY_RUN=1 reached the script.
			mk(filepath.Join("scripts", "pawl.sh"), []byte("#!/usr/bin/env bash\n[ \"${PAWL_DRY_RUN:-}\" = \"1\" ] || exit 9\nexit 0\n"), 0o755)
			selfAo := filepath.Join(repo, "cli", "bin", "ao")
			mk(filepath.Join("cli", "bin", "ao"), []byte("dummy"), 0o755)
			prevDir, prevSelf := testProjectDir, pawlSelfBinary
			testProjectDir = repo
			pawlSelfBinary = func() (string, error) { return selfAo, nil }
			t.Cleanup(func() { testProjectDir = prevDir; pawlSelfBinary = prevSelf })
			setPawlGlobalFlags(t, true, false)

			cmd := pawlServiceCmd(sub, sub, "test")
			var out strings.Builder
			cmd.SetOut(&out)
			cmd.SetErr(&out)
			if err := cmd.RunE(cmd, nil); err != nil {
				t.Fatalf("read-only %s under dry-run must still delegate WITH PAWL_DRY_RUN=1 (stub exit 9 = seam missing): %v", sub, err)
			}
		})
	}
}

// TestPawlHelpPresentsReviewAsFrontDoor (age-hk5zg.2 / S2 of the pawl-user-front-door
// packet): `ao pawl --help` must present `review` as the primary USER path and group the
// warm standing-service verbs (up/down/reap/health/doctor/smoke/route/metrics) as
// operator-only with a note that they require NTM — so a user is never led to believe
// they must run `ao pawl up` to use the membrane. Locks group membership, render order
// (user group before operator group), and the NTM note in the operator title.
func TestPawlHelpPresentsReviewAsFrontDoor(t *testing.T) {
	if pawlReviewCmd.GroupID != pawlUserGroupID {
		t.Fatalf("pawl review must sit in the user (front door) group; got GroupID=%q", pawlReviewCmd.GroupID)
	}
	warm := map[string]bool{"up": true, "down": true, "reap": true, "health": true,
		"doctor": true, "smoke": true, "route": true, "metrics": true}
	seen := 0
	for _, c := range pawlCmd.Commands() {
		if !warm[c.Name()] {
			continue
		}
		seen++
		if c.GroupID != pawlOperatorGroupID {
			t.Errorf("warm verb %q must sit in the operator group; got GroupID=%q", c.Name(), c.GroupID)
		}
	}
	if seen != len(warm) {
		t.Fatalf("expected all %d warm verbs registered on pawl; saw %d", len(warm), seen)
	}

	var buf strings.Builder
	pawlCmd.SetOut(&buf)
	t.Cleanup(func() { pawlCmd.SetOut(nil) })
	if err := pawlCmd.Help(); err != nil {
		t.Fatalf("rendering pawl help: %v", err)
	}
	help := buf.String()
	userIdx := strings.Index(help, pawlUserGroupTitle)
	opIdx := strings.Index(help, pawlOperatorGroupTitle)
	if userIdx < 0 {
		t.Fatalf("help must render the user group title %q; got:\n%s", pawlUserGroupTitle, help)
	}
	if opIdx < 0 {
		t.Fatalf("help must render the operator group title %q; got:\n%s", pawlOperatorGroupTitle, help)
	}
	if userIdx > opIdx {
		t.Fatalf("the user (front door) group must render BEFORE the operator group (review-first); user@%d operator@%d", userIdx, opIdx)
	}
	if !strings.Contains(pawlOperatorGroupTitle, "NTM") {
		t.Fatalf("the operator group title must name the NTM requirement; got %q", pawlOperatorGroupTitle)
	}
	reviewIdx := strings.Index(help, "\n  review ")
	if reviewIdx < 0 || reviewIdx > opIdx {
		t.Fatalf("review must be listed before the operator group; review@%d operator@%d\n%s", reviewIdx, opIdx, help)
	}
}

// TestPawlDryRunPlan_SpawnStepIsSeamNeutral (age-pawl-intent-zhndq.16): the `up` planned step must
// NOT hardcode "atm spawn" — the swarm binary is resolved ntm-first by the shell seam
// (PAWL_SWARM_BIN -> ntm -> atm) and `ao pawl doctor` reports which won, so a hardcoded "atm"
// contradicted the tool's own output. The step must be seam-neutral and point at doctor.
func TestPawlDryRunPlan_SpawnStepIsSeamNeutral(t *testing.T) {
	doc := pawlDryRunPlan("up", nil)
	var spawnStep string
	for _, s := range doc.PlannedSteps {
		if strings.Contains(s, "spawn session") {
			spawnStep = s
		}
	}
	if spawnStep == "" {
		t.Fatalf("no spawn step in planned steps: %v", doc.PlannedSteps)
	}
	if strings.Contains(spawnStep, "atm spawn") {
		t.Fatalf("spawn step hardcodes the atm binary (contradicts the ntm-first seam + doctor): %q", spawnStep)
	}
	if !strings.Contains(spawnStep, "swarm spawn") {
		t.Fatalf("spawn step should name the neutral swarm seam; got %q", spawnStep)
	}
}
