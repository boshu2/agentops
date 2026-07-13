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

// setPawlGlobalFlags flips the shared rootCmd persistent-flag globals for one test and
// restores them via t.Cleanup (the cmd/ao shuffle-order isolation contract: every
// package-global set-site self-cleans).
func setPawlGlobalFlags(t *testing.T, dry, jsonOut bool) {
	t.Helper()
	prevDry, prevJSON := dryRun, jsonFlag
	dryRun, jsonFlag = dry, jsonOut
	t.Cleanup(func() { dryRun, jsonFlag = prevDry, prevJSON })
}

// writeForgedPawlReviewRepo builds a repo that FORGES the AgentOps markers and plants a
// sentinel-touching scripts/pawl-review.sh. The planted script must NEVER run under
// --dry-run (D1) — and never at all from an installed binary (D4, embed tests).
// insideBinary=true simulates the genuine-checkout dogfood (ao binary inside the repo).
func writeForgedPawlReviewRepo(t *testing.T, insideBinary bool) (repo, sentinel string) {
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

func diffDirState(before, after map[string]int64) []string {
	var d []string
	for k, v := range after {
		if bv, ok := before[k]; !ok {
			d = append(d, "+"+k)
		} else if bv != v {
			d = append(d, "~"+k)
		}
	}
	for k := range before {
		if _, ok := after[k]; !ok {
			d = append(d, "-"+k)
		}
	}
	return d
}

// TestPawlReview_DryRunIsSideEffectFree: under --dry-run the review script must never run.
func TestPawlReview_DryRunIsSideEffectFree(t *testing.T) {
	repo, sentinel := writeForgedPawlReviewRepo(t, true)
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

// TestStripPawlPassthroughFlags_EqualsForm: both bare and --flag=value forms must be
// recognized; an unparseable value fails CLOSED (dry-run wins over a mutation).
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
		{"mixed", []string{"--scope", "head", "--dry-run=true"}, []string{"--scope", "head"}, true, false},
		{"none", []string{"age-x", "--scope", "head"}, []string{"age-x", "--scope", "head"}, false, false},
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

// TestPawlHelpPresentsReviewAsFrontDoor: `ao pawl --help` must present `review` as the
// primary USER path with no warm standing-service verbs.
func TestPawlHelpPresentsReviewAsFrontDoor(t *testing.T) {
	if pawlReviewCmd.GroupID != pawlUserGroupID {
		t.Fatalf("pawl review must sit in the user (front door) group; got GroupID=%q", pawlReviewCmd.GroupID)
	}
	for _, c := range pawlCmd.Commands() {
		if c.Name() == "review" {
			continue
		}
		t.Errorf("unexpected pawl subcommand %q after warm-service removal", c.Name())
	}
	var buf strings.Builder
	pawlCmd.SetOut(&buf)
	t.Cleanup(func() { pawlCmd.SetOut(nil) })
	if err := pawlCmd.Help(); err != nil {
		t.Fatalf("rendering pawl help: %v", err)
	}
	help := buf.String()
	if !strings.Contains(help, pawlUserGroupTitle) {
		t.Fatalf("help must render the user group title %q; got:\n%s", pawlUserGroupTitle, help)
	}
	if !strings.Contains(help, "  review ") {
		t.Fatalf("help must list review; got:\n%s", help)
	}
	for _, warm := range []string{"  up ", "  down ", "  route ", "  health ", "  doctor ", "  smoke ", "  metrics ", "  reap "} {
		if strings.Contains(help, warm) {
			t.Fatalf("help must not list warm verb after removal; found %q in:\n%s", warm, help)
		}
	}
}

// TestPawlReview_DryRunJSONContract: with --json, dry-run review emits exactly one JSON object.
func TestPawlReview_DryRunJSONContract(t *testing.T) {
	writeForgedPawlReviewRepo(t, true)
	setPawlGlobalFlags(t, true, true)
	var out strings.Builder
	pawlReviewCmd.SetOut(&out)
	pawlReviewCmd.SetErr(&out)
	t.Cleanup(func() { pawlReviewCmd.SetOut(nil); pawlReviewCmd.SetErr(nil) })
	if err := runPawlReview(pawlReviewCmd, []string{"age-x", "--scope", "head"}); err != nil {
		t.Fatalf("dry-run review: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(out.String()), &doc); err != nil {
		t.Fatalf("D2: output is not exactly one JSON document: %v\n%s", err, out.String())
	}
	for _, k := range []string{"action", "dry_run", "mutated", "families", "tier", "planned_steps"} {
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
}
