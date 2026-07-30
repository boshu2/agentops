package checks

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/gates"
)

// Check-liveness closure: every BLOCKING script-backed gate must have a test
// that demonstrates it FAILING on the thing it claims to detect.
//
// WHY: the 2026-07-25 review found two load-bearing rules that existed only as
// text — ADR-0016's "Python never ships in skills" with no gate, and an
// exact-identity digest contract whose test mocked Validate with the code under
// test's own digest function. Both were green while broken. Those were
// instances of one class: a rule nobody executes, and a check nobody proved
// executes. A gate verified only on a green tree is indistinguishable from
// `exit 0` — it would still pass with its detector deleted. This test is the
// standing requirement the review asked for.
//
// SHAPE: a shrink-only ratchet, the same mechanism the repo already uses for
// preamble adoption and shipped Python. The gates lacking a witness at the
// cutoff are pinned in scripts/.gate-negative-witness-grandfather and exempt;
// a NEW blocking gate must ship with a witness; a pinned gate that gains one
// must be pruned. The list cannot grow — the growth guard below rejects any
// entry not present at HEAD, so a change cannot exempt its own new gate.
//
// WHAT COUNTS: any file under tests/ that names the backing script AND asserts
// a non-zero outcome from it. Deliberately generous about form — bats
// (`[ "$status" -eq 1 ]`), shell integration tests, and Python
// (`returncode != 0`) all qualify. The bar is "somebody proved it can fail",
// not a particular framework.
const negativeWitnessGrandfather = "scripts/.gate-negative-witness-grandfather"

// negativeAssertion matches the ways this repo's tests assert a failing run.
// Kept broad on purpose: a false NEGATIVE here (missing a real witness) would
// wrongly pin a well-tested gate, which is merely noisy; the ratchet's teeth
// come from the growth guard, not from this regex being exhaustive.
var negativeAssertion = regexp.MustCompile(
	`\$status" -eq [1-9]|\$status" -ne 0|assert_failure|refute_success|returncode, 1|returncode != 0|-ne 0 \]`,
)

// scrubbedGitEnv is the package-local twin of gitDiscoveryEnv
// (cli/cmd/ao/git_read.go): the process environment minus the four git
// discovery overrides. A hook-launched process inherits GIT_DIR/GIT_WORK_TREE,
// and under a leaked GIT_DIR even a `-C <dir>`-scoped git call resolves against
// the LEAKED repository — which is how a fixture `git init` once rewrote a
// shared .git/config and bricked every worktree
// (age-gate-scripts-worktree-gitdir-p62wo, ek8v).
func scrubbedGitEnv() []string {
	const prefixes = "GIT_DIR=|GIT_WORK_TREE=|GIT_COMMON_DIR=|GIT_INDEX_FILE="
	env := make([]string, 0, len(os.Environ()))
	for _, entry := range os.Environ() {
		drop := false
		for _, p := range strings.Split(prefixes, "|") {
			if strings.HasPrefix(entry, p) {
				drop = true
				break
			}
		}
		if !drop {
			env = append(env, entry)
		}
	}
	return env
}

func loadPinnedGates(t *testing.T, root string) map[string]bool {
	t.Helper()
	pinned := map[string]bool{}
	f, err := os.Open(filepath.Join(root, negativeWitnessGrandfather))
	if err != nil {
		t.Fatalf("read %s: %v", negativeWitnessGrandfather, err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		pinned[line] = true
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan %s: %v", negativeWitnessGrandfather, err)
	}
	return pinned
}

// collectTestBodies reads every git-tracked file under tests/ once.
// Tracked-only is load-bearing: untracked/gitignored files under tests/ (e.g.
// session transcripts in tests/claude-code/logs/*.jsonl) contain gate-script
// names next to assertion-shaped text, which reads as a phantom negative
// witness — making this suite's verdict depend on checkout dirt instead of
// committed tests (2026-07-29: dirty checkout failed while a clean worktree
// passed).
func collectTestBodies(t *testing.T, root string) map[string]string {
	t.Helper()
	cmd := exec.Command("git", "ls-files", "-z", "--", "tests")
	cmd.Dir = root
	cmd.Env = scrubbedGitEnv()
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git ls-files tests/: %v", err)
	}
	bodies := map[string]string{}
	for _, rel := range strings.Split(string(out), "\x00") {
		if rel == "" {
			continue
		}
		body, readErr := os.ReadFile(filepath.Join(root, rel))
		if readErr != nil {
			continue // tracked but absent from the worktree is not this test's business
		}
		bodies[rel] = string(body)
	}
	return bodies
}

// hasNegativeWitness reports whether some test names the backing script and
// asserts a non-zero outcome.
func hasNegativeWitness(bodies map[string]string, backing string) bool {
	base := filepath.Base(backing)
	for _, body := range bodies {
		if strings.Contains(body, base) && negativeAssertion.MatchString(body) {
			return true
		}
	}
	return false
}

func TestBlockingGatesHaveProvenNegativeWitness(t *testing.T) {
	root := repoRootFromTest(t)
	pinned := loadPinnedGates(t, root)
	bodies := collectTestBodies(t, root)

	proven := map[string]bool{}
	for _, c := range gates.Default.All() {
		if c.Backing == "" || !c.Blocking {
			continue
		}
		if hasNegativeWitness(bodies, c.Backing) {
			proven[c.ID] = true
			continue
		}
		if pinned[c.ID] {
			continue // grandfathered: inert, but visibly and countably so
		}
		t.Errorf("blocking gate %q (backing %s) has no test proving it FAILS on what it detects.\n"+
			"    Add a test under tests/ that names %s and asserts a non-zero outcome.\n"+
			"    Pinning it in %s is NOT a repair — the growth guard rejects new entries.",
			c.ID, c.Backing, filepath.Base(c.Backing), negativeWitnessGrandfather)
	}

	// Shrink direction: a pinned gate that now has a witness must be pruned, so
	// the count can only go down.
	for id := range pinned {
		if proven[id] {
			t.Errorf("gate %q now has a negative witness but is still pinned in %s — remove that line (the allowlist only shrinks)",
				id, negativeWitnessGrandfather)
		}
	}

	// Stale direction: a pinned gate that no longer exists in the registry is
	// dead weight hiding the real count.
	live := map[string]bool{}
	for _, c := range gates.Default.All() {
		live[c.ID] = true
	}
	for id := range pinned {
		if !live[id] {
			t.Errorf("gate %q is pinned in %s but is not in the registry — remove that line", id, negativeWitnessGrandfather)
		}
	}

	t.Logf("check-liveness: %d blocking gate(s) still lack a proven negative witness (shrink-only)", len(pinned))
}

// TestCollectTestBodiesIgnoresUntrackedFiles is the regression guard for the
// phantom-witness class: an untracked file under tests/ (a gitignored session
// log, a scratch fixture) that happens to contain a gate-script name plus an
// assertion-shaped string must NOT count as a negative witness. Only committed
// tests are evidence.
func TestCollectTestBodiesIgnoresUntrackedFiles(t *testing.T) {
	repo := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		cmd.Env = scrubbedGitEnv()
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	git("init", "-q")

	logsDir := filepath.Join(repo, "tests", "logs")
	if err := os.MkdirAll(logsDir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", logsDir, err)
	}
	tracked := filepath.Join(repo, "tests", "real_witness.bats")
	if err := os.WriteFile(tracked, []byte("run check-tracked.sh\n[ \"$status\" -eq 1 ]\n"), 0o644); err != nil {
		t.Fatalf("write tracked fixture: %v", err)
	}
	git("add", "tests/real_witness.bats")

	// The decoy mimics a session transcript: names a gate script and carries an
	// assertion-shaped string, but is never added to the index.
	decoy := filepath.Join(logsDir, "session.jsonl")
	if err := os.WriteFile(decoy, []byte(`{"text":"ran check-decoy.sh and saw [ \"$status\" -eq 1 ] in the output"}`+"\n"), 0o644); err != nil {
		t.Fatalf("write decoy: %v", err)
	}

	bodies := collectTestBodies(t, repo)
	if _, ok := bodies["tests/real_witness.bats"]; !ok {
		t.Errorf("tracked test under tests/ missing from collected bodies: %v", bodies)
	}
	if _, ok := bodies["tests/logs/session.jsonl"]; ok {
		t.Errorf("untracked decoy under tests/ was collected — phantom-witness regression")
	}
	if !hasNegativeWitness(bodies, "scripts/check-tracked.sh") {
		t.Errorf("tracked witness for check-tracked.sh not recognized")
	}
	if hasNegativeWitness(bodies, "scripts/check-decoy.sh") {
		t.Errorf("untracked decoy counted as a negative witness for check-decoy.sh")
	}
}

// TestNegativeWitnessAllowlistOnlyShrinks is the growth guard. Without it the
// ratchet has no teeth: a change could add a new blocking gate and exempt it in
// the same diff, which is exactly the "rule nobody executes" pattern this whole
// mechanism exists to stop. HEAD is the authority — a same-diff self-allowlist
// grants no protection.
func TestNegativeWitnessAllowlistOnlyShrinks(t *testing.T) {
	root := repoRootFromTest(t)
	// cmd.Dir pins the repository and cmd.Env strips the git discovery overrides.
	// TestMain's process-level ScrubGitDiscoveryEnv already covers this package,
	// but the per-command scrub is the repo's stated contract and does not rely
	// on a TestMain elsewhere in the package staying correct
	// (age-gate-scripts-worktree-gitdir-p62wo; gitDiscoveryEnv in
	// cli/cmd/ao/git_read.go). A hook-leaked GIT_DIR would otherwise resolve this
	// read against a different repository entirely.
	cmd := exec.Command("git", "show", "HEAD:"+negativeWitnessGrandfather)
	cmd.Dir = root
	cmd.Env = scrubbedGitEnv()
	out, err := cmd.Output()
	if err != nil {
		t.Skip("no HEAD version of the allowlist (initial snapshot)")
	}
	base := map[string]bool{}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		base[line] = true
	}
	for id := range loadPinnedGates(t, root) {
		if !base[id] {
			t.Errorf("%s gained entry %q — the allowlist only SHRINKS. A new blocking gate ships with a negative witness or it does not ship.",
				negativeWitnessGrandfather, id)
		}
	}
}
