package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// landTestHarness saves + restores every shared seam/global land touches, and
// stands up a minimal genuine checkout (docs/contracts/agents-write-surfaces.md +
// skills/) so resolveAgentsRepoRoot accepts it. It returns the repo root and the
// fresh-binary path. All restoration is via t.Cleanup (the cmd/ao isolation
// contract) so a set seam can never leak into the -shuffle=on neighbor test.
type landTestHarness struct {
	repo     string
	freshBin string
	// captured invocation record
	built        bool
	reexeced     bool
	reexecFresh  string
	warmed       bool
	prepared     bool
	reviewedBase string
	reviewAOBIN  string // AO_BIN as seen by the review seam (proves Step-2 pin)
	reviewedBd   string
	scripts      []string // rel paths passed to landRunScript, in order
	scriptArgs   [][]string
	scriptAOBIN  []string // AO_BIN pinned per script call
}

func newLandHarness(t *testing.T) *landTestHarness {
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
	freshBin := filepath.Join(repo, "cli", "bin", "ao")

	h := &landTestHarness{repo: repo, freshBin: freshBin}

	// t.Setenv auto-saves the pre-test values and restores them on cleanup (and
	// blocks t.Parallel — correct here, since land mutates process env + globals).
	// Start both empty: AO_BIN so Step 2's pin is observable from a clean base, and
	// AO_LAND_REEXECED unset so the parent (not re-exec'd) path runs by default.
	t.Setenv("AO_BIN", "")
	t.Setenv(landReexecEnv, "")
	t.Setenv(landReviewBaseEnv, "")

	// Save every shared seam/global.
	prevDir := testProjectDir
	prevSelf := pawlSelfBinary
	prevBuild := landBuildFreshBinary
	prevPrepare := landPrepareReviewBase
	prevReexec := landReexec
	prevWarm := landEnsureWarmService
	prevReview := landRunReview
	prevScript := landRunScript
	t.Cleanup(func() {
		testProjectDir = prevDir
		pawlSelfBinary = prevSelf
		landBuildFreshBinary = prevBuild
		landPrepareReviewBase = prevPrepare
		landReexec = prevReexec
		landEnsureWarmService = prevWarm
		landRunReview = prevReview
		landRunScript = prevScript
	})

	testProjectDir = repo
	// Default: the running binary IS the fresh binary (no re-exec) — the common
	// post-re-exec state where the real work runs. Individual tests override.
	pawlSelfBinary = func() (string, error) { return freshBin, nil }

	// Default seams: build is a no-op that materializes the dest so aoBinaryInside
	// has a real file; the heavy sub-steps record + succeed.
	landBuildFreshBinary = func(_, dest string) error {
		h.built = true
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}
		return os.WriteFile(dest, []byte("fresh"), 0o755)
	}
	landPrepareReviewBase = func(_ *cobra.Command, _ string) (string, error) {
		h.prepared = true
		h.reviewedBase = "0123456789abcdef0123456789abcdef01234567"
		return h.reviewedBase, nil
	}
	landReexec = func(_ *cobra.Command, freshBin string, _ []string) (int, error) {
		h.reexeced = true
		h.reexecFresh = freshBin
		return 0, nil
	}
	landEnsureWarmService = func(_ *cobra.Command, _, _ string) { h.warmed = true }
	landRunReview = func(_ *cobra.Command, bead string) error {
		h.reviewedBd = bead
		h.reviewAOBIN = os.Getenv("AO_BIN")
		return nil // CONFIRM by default
	}
	landRunScript = func(_ *cobra.Command, _, aoBin, rel string, args ...string) (bool, error) {
		h.scripts = append(h.scripts, rel)
		h.scriptArgs = append(h.scriptArgs, append([]string(nil), args...))
		h.scriptAOBIN = append(h.scriptAOBIN, aoBin)
		return true, nil
	}
	return h
}

// runLandCmd drives runLand with buffered stdio so a test never writes to the
// process streams.
func runLandCmd(t *testing.T, bead string) error {
	t.Helper()
	cmd := &cobra.Command{}
	cmd.SetOut(&strings.Builder{})
	cmd.SetErr(&strings.Builder{})
	return runLand(cmd, []string{bead})
}

// The happy path: fresh build → (already fresh, no re-exec) → AO_BIN pinned →
// warm → review CONFIRM → pawl-land.sh + post-land handoff, in order.
func TestLand_ConfirmHandoffToPawlLand(t *testing.T) {
	h := newLandHarness(t)
	if err := runLandCmd(t, "age-test"); err != nil {
		t.Fatalf("land should succeed on CONFIRM, got %v", err)
	}
	if !h.built {
		t.Error("Step 0: fresh binary was not built")
	}
	if !h.prepared {
		t.Error("Step 0: reviewed origin/main base was not prepared before the review")
	}
	if h.reexeced {
		t.Error("Step 1: must NOT re-exec when already running the fresh binary")
	}
	if !h.warmed {
		t.Error("Step 2: warm-service liveness was not attempted")
	}
	if h.reviewedBd != "age-test" {
		t.Errorf("Step 3: review ran for %q, want age-test", h.reviewedBd)
	}
	// Step 4/5: CONFIRM hands off to pawl-land.sh THEN post-land, in that order.
	wantScripts := []string{"scripts/pawl-land.sh", "scripts/post-land-provenance-emit.sh"}
	if len(h.scripts) != len(wantScripts) {
		t.Fatalf("script handoff = %v, want %v", h.scripts, wantScripts)
	}
	for i, want := range wantScripts {
		if h.scripts[i] != want {
			t.Errorf("script[%d] = %q, want %q", i, h.scripts[i], want)
		}
	}
	wantLandArgs := []string{"age-test", "0", h.reviewedBase}
	if got := h.scriptArgs[0]; strings.Join(got, "\x00") != strings.Join(wantLandArgs, "\x00") {
		t.Errorf("pawl-land args = %q, want %q", got, wantLandArgs)
	}
}

// Step 2 (AO_BIN pin): the review and every land script must run with AO_BIN pinned
// to the fresh in-checkout binary — the VERIFIED GAP the live path leaves open.
func TestLand_PinsAOBINToFreshBinary(t *testing.T) {
	h := newLandHarness(t)
	if err := runLandCmd(t, "age-test"); err != nil {
		t.Fatalf("land: %v", err)
	}
	if h.reviewAOBIN != h.freshBin {
		t.Errorf("review saw AO_BIN=%q, want the fresh binary %q", h.reviewAOBIN, h.freshBin)
	}
	for i, got := range h.scriptAOBIN {
		if got != h.freshBin {
			t.Errorf("script[%d] (%s) pinned AO_BIN=%q, want %q", i, h.scripts[i], got, h.freshBin)
		}
	}
	// And AO_BIN is actually set in the process env for inherited children.
	if os.Getenv("AO_BIN") != h.freshBin {
		t.Errorf("process AO_BIN=%q, want %q", os.Getenv("AO_BIN"), h.freshBin)
	}
}

// Step 1 (re-exec decision): when the running binary is NOT the fresh one, land
// re-execs through the fresh binary and does the real work in the child — it must
// NOT run the review/handoff in the parent process.
func TestLand_ReexecsWhenRunningBinaryIsStale(t *testing.T) {
	h := newLandHarness(t)
	// Simulate the installed/stale ao: the running binary lives OUTSIDE the checkout.
	stale := filepath.Join(t.TempDir(), "ao")
	if err := os.WriteFile(stale, []byte("stale"), 0o755); err != nil {
		t.Fatal(err)
	}
	pawlSelfBinary = func() (string, error) { return stale, nil }

	if err := runLandCmd(t, "age-test"); err != nil {
		t.Fatalf("re-exec path should succeed (child returned 0), got %v", err)
	}
	if !h.reexeced {
		t.Fatal("Step 1: must re-exec when the running binary is not the fresh one")
	}
	if h.reexecFresh != h.freshBin {
		t.Errorf("re-exec targeted %q, want the fresh binary %q", h.reexecFresh, h.freshBin)
	}
	// The parent must delegate to the child — NOT run review/handoff itself.
	if h.reviewedBd != "" || len(h.scripts) != 0 {
		t.Errorf("parent must delegate to the re-exec'd child; ran review=%q scripts=%v", h.reviewedBd, h.scripts)
	}
}

// A non-zero re-exec child code propagates as a landExitError with that exact code.
func TestLand_ReexecNonZeroCodePropagates(t *testing.T) {
	h := newLandHarness(t)
	stale := filepath.Join(t.TempDir(), "ao")
	if err := os.WriteFile(stale, []byte("stale"), 0o755); err != nil {
		t.Fatal(err)
	}
	pawlSelfBinary = func() (string, error) { return stale, nil }
	landReexec = func(_ *cobra.Command, _ string, _ []string) (int, error) {
		h.reexeced = true
		return 3, nil // child REFUTED
	}
	err := runLandCmd(t, "age-test")
	var exitErr *landExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("want *landExitError, got %T: %v", err, err)
	}
	if exitErr.ExitCode() != 3 {
		t.Errorf("ExitCode() = %d, want 3 (propagated child code)", exitErr.ExitCode())
	}
}

// A REFUTED / NO-VERDICT review STOPS the land: no pawl-land, no post-land, and the
// review's exit code is surfaced (never a false land on a non-CONFIRM verdict).
func TestLand_RefutedReviewStopsBeforePawlLand(t *testing.T) {
	h := newLandHarness(t)
	landRunReview = func(_ *cobra.Command, bead string) error {
		h.reviewedBd = bead
		return &pawlReviewExitError{code: 3} // REFUTED
	}
	err := runLandCmd(t, "age-test")
	var exitErr *pawlReviewExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("REFUTED review must surface *pawlReviewExitError, got %T: %v", err, err)
	}
	if exitErr.ExitCode() != 3 {
		t.Errorf("ExitCode() = %d, want 3 (REFUTED)", exitErr.ExitCode())
	}
	if len(h.scripts) != 0 {
		t.Fatalf("REFUTED must NOT hand off to any land script; ran %v", h.scripts)
	}
}

// A pawl-land.sh failure (gate red / push conflict) surfaces as a non-zero
// landExitError and does NOT run post-land provenance.
func TestLand_PawlLandFailurePropagates(t *testing.T) {
	h := newLandHarness(t)
	landRunScript = func(_ *cobra.Command, _, _, rel string, _ ...string) (bool, error) {
		h.scripts = append(h.scripts, rel)
		if rel == "scripts/pawl-land.sh" {
			// A real gate failure returns an *exec.ExitError; simulate a plain error
			// path (the code propagates to exit 1 via the non-ExitError branch).
			return true, errors.New("gate refused the push")
		}
		return true, nil
	}
	err := runLandCmd(t, "age-test")
	if err == nil {
		t.Fatal("a pawl-land failure must surface a non-nil error")
	}
	if !strings.Contains(err.Error(), "pawl-land.sh") {
		t.Errorf("error should name the failing land script, got %v", err)
	}
	for _, s := range h.scripts {
		if s == "scripts/post-land-provenance-emit.sh" {
			t.Error("post-land provenance must NOT run after a pawl-land failure")
		}
	}
}

// land requires a genuine checkout: outside one, resolveAgentsRepoRoot fails and
// land errors clearly rather than silently taking a stranger path.
func TestLand_RequiresGenuineCheckout(t *testing.T) {
	newLandHarness(t)
	// Point the project dir at an empty temp dir with no agents-write-surfaces contract.
	testProjectDir = t.TempDir()
	err := runLandCmd(t, "age-test")
	if err == nil {
		t.Fatal("land outside a genuine checkout must error")
	}
	if !strings.Contains(err.Error(), "AgentOps checkout") {
		t.Errorf("error should explain the checkout requirement, got %v", err)
	}
}

// `ao land --help` must be STATIC: no side effects (no build, no re-exec, no
// review). cobra handles --help before RunE, but guard it as a behavioral probe.
func TestLand_HelpIsStatic(t *testing.T) {
	h := newLandHarness(t)
	// Drive the real registered command with --help through a throwaway root so
	// cobra's help path (not RunE) is exercised.
	root := &cobra.Command{Use: "ao"}
	root.AddGroup(&cobra.Group{ID: "workflow", Title: "Workflow:"})
	probe := *landCmd
	root.AddCommand(&probe)
	root.SetArgs([]string{"land", "--help"})
	root.SetOut(&strings.Builder{})
	root.SetErr(&strings.Builder{})
	if err := root.Execute(); err != nil {
		t.Fatalf("land --help should not error, got %v", err)
	}
	if h.built || h.reexeced || h.reviewedBd != "" {
		t.Errorf("--help must be static — no build/re-exec/review side effects (built=%v reexec=%v reviewed=%q)", h.built, h.reexeced, h.reviewedBd)
	}
}

// The land verb is registered under the workflow group with ExactArgs(1).
func TestLand_CommandWiring(t *testing.T) {
	if landCmd.GroupID != "workflow" {
		t.Errorf("land GroupID = %q, want workflow", landCmd.GroupID)
	}
	if err := landCmd.Args(landCmd, []string{}); err == nil {
		t.Error("land must require exactly one arg (the bead id)")
	}
	if err := landCmd.Args(landCmd, []string{"a", "b"}); err == nil {
		t.Error("land must reject more than one arg")
	}
}

func TestLand_SourceReviewsUpstreamRangeAndHandsBaseToPawlLand(t *testing.T) {
	source, err := os.ReadFile(findRepoFileForTest(t, "cli", "cmd", "ao", "land.go"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	for _, want := range []string{
		`"--scope", "upstream"`,
		"landPrepareReviewBase",
		`"scripts/pawl-land.sh", bead, "0", reviewedBase`,
	} {
		if !strings.Contains(text, want) {
			t.Errorf("ao land source missing truthful range-review contract %q", want)
		}
	}
}

func TestLand_PrepareFailureStopsBeforeBuildReviewOrLand(t *testing.T) {
	h := newLandHarness(t)
	landPrepareReviewBase = func(_ *cobra.Command, _ string) (string, error) {
		h.prepared = true
		return "", errors.New("rebase conflict")
	}
	err := runLandCmd(t, "age-test")
	if err == nil || !strings.Contains(err.Error(), "preparing the reviewed branch base") {
		t.Fatalf("prepare failure = %v, want actionable stop", err)
	}
	if h.built || h.reviewedBd != "" || len(h.scripts) != 0 {
		t.Fatalf("prepare failure must stop before build/review/land: built=%v review=%q scripts=%v", h.built, h.reviewedBd, h.scripts)
	}
}

func TestLand_ReexecChildRequiresAndReusesPreparedBase(t *testing.T) {
	h := newLandHarness(t)
	t.Setenv(landReexecEnv, "1")
	t.Setenv(landReviewBaseEnv, "abcdef0123456789abcdef0123456789abcdef01")
	if err := runLandCmd(t, "age-test"); err != nil {
		t.Fatalf("reexec child: %v", err)
	}
	if h.prepared || h.built {
		t.Fatalf("reexec child must reuse parent preparation/build: prepared=%v built=%v", h.prepared, h.built)
	}
	want := "abcdef0123456789abcdef0123456789abcdef01"
	if got := h.scriptArgs[0][2]; got != want {
		t.Fatalf("pawl-land reviewed base = %q, want inherited %q", got, want)
	}
}
