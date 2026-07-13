// practices: [design-by-contract, code-complete]
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/boshu2/agentops/cli/embedded"
	"github.com/spf13/cobra"
)

var pawlCmd = &cobra.Command{
	Use:   "pawl",
	Short: "Cross-family membrane review and verdict tooling (the in-repo acceptance pawl)",
	Long: `The pawl is AgentOps's acceptance gate: a change reaches "done" only with an
INDEPENDENT cross-family verdict (never the author, never the same model).

'ao pawl review' is the FRONT DOOR — it works in any git repo with just this binary
plus one reviewer CLI (codex or agy) on PATH: no NTM, no tmux, no config. On
CONFIRMED it writes the commit-bound verdict the push-to-main gate enforces.`,
	Args: cobra.NoArgs,
}

// pawl help groups: `review` is the user front door (no NTM/tmux required).
const (
	pawlUserGroupID    = "pawl-user"
	pawlUserGroupTitle = "Use the membrane (the front door — needs no NTM, no setup):"
)

// pawlReviewExitError carries scripts/pawl-review.sh's exit code so it propagates
// VERBATIM through ao (the exit code IS the verdict, like ao plan-pawl / ao validate):
// 0 CONFIRMED+written · 3 REFUTED · 4 --converge advisory-only (no lineage) · 2 usage · 1 hard error.
type pawlReviewExitError struct{ code int }

func (e *pawlReviewExitError) Error() string { return "" }

// ExitCode returns the process exit code this verdict maps to.
func (e *pawlReviewExitError) ExitCode() int { return e.code }

const defaultPawlReviewScript = "scripts/pawl-review.sh"

var pawlReviewCmd = &cobra.Command{
	Use:   "review <bead-id> [--scope head|staged|upstream] [--base <sha>] [--converge] [--strict] [--author-family <fam>] [--context <s>] [--smoke <cmd>]",
	Short: "Run the cross-family (codex) membrane review; on CONFIRMED write the commit-bound verdict",
	Long: `Wrap scripts/pawl-review.sh and surface it on the ao CLI. Dispatches the codex
refuter against the commit and, on CONFIRMED, writes + verifies the commit-bound pawl
verdict the pre-push gate requires (REFUTED prints the defects + exits 3; --converge is
advisory-only without adversarial lineage and exits 4). LAW 0: the refuter is codex (a
cross-family reviewer), never a same-model self-review. All arguments after 'review' are
forwarded verbatim to the script.

--scope upstream reviews the complete configured-upstream merge-base through the
review target and binds the verdict to that target. With --base, that exact
ancestor commit replaces configured-upstream discovery. 'ao land' always pins
the origin/main commit it fetched before review, so the independent packet covers
the same branch delta that the guarded push would introduce.

--strict (age-rk3r.13): the OPT-IN two-family cold quorum for the highest-irreversibility
doors — TWO DISTINCT strict-eligible cold families must BOTH CONFIRMED, and strict REFUSES
to degrade to one (an outage HOLDs, exit 5, never a single-family pass). It DOUBLES review
cost (opt-in only) and is the portable cold two-family quorum. Today no
second strict-eligible cold family exists yet (agy A7-benched; no cold claude adapter — LAW
0 forbids the Claude headless print path), so --strict prints an honest UNAVAILABLE and
exits 5 rather than faking a pass; the machinery is built and flipping one eligibility list
turns real strict on. See 'ao verify --help' for the full posture.`,
	// Forward all flags verbatim to the script (it owns the flag contract).
	DisableFlagParsing: true,
	RunE:               runPawlReview,
}

func init() {
	rootCmd.AddCommand(pawlCmd)
	pawlCmd.AddGroup(&cobra.Group{ID: pawlUserGroupID, Title: pawlUserGroupTitle})
	pawlReviewCmd.GroupID = pawlUserGroupID
	pawlCmd.AddCommand(pawlReviewCmd)
}


// pawlDryRunDoc is the single JSON document a dry-run pawl command emits (D2):
// exactly one parseable object, never interleaved human log lines.
type pawlDryRunDoc struct {
	Action       string   `json:"action"`
	DryRun       bool     `json:"dry_run"`
	Mutated      bool     `json:"mutated"`
	Session      string   `json:"session"`
	Families     string   `json:"families"`
	Tier         string   `json:"tier"`
	PlannedSteps []string `json:"planned_steps"`
}

// pawlDryRunPlan builds the planned-action report for `review` under --dry-run.
// Pure reads only — no script execution, no bundle extraction, no state writes.
func pawlDryRunPlan(sub string, args []string) pawlDryRunDoc {
	doc := pawlDryRunDoc{Action: "pawl " + sub, DryRun: true, Mutated: false}
	if sub == "review" {
		doc.Families, doc.Tier = "cod", "fresh"
		doc.PlannedSteps = []string{
			"resolve the trusted pawl-review script (live checkout or embedded bundle)",
			"dispatch the cross-family refuter against the commit",
			"on CONFIRMED write + verify the commit-bound verdict",
		}
	}
	return doc
}

// stripPawlPassthroughFlags scans the raw leaf args for the GLOBAL --dry-run/--json
// tokens and removes them. Root cause of D1: pawl leaves set DisableFlagParsing, so
// cobra never parses inherited persistent flags placed before OR after the subcommand.
// The leaf must extract them itself and OR with the globals. Accepts both the bare form
// (`--dry-run`) and cobra's `--flag=value` form. An unparseable value fails CLOSED.
func stripPawlPassthroughFlags(args []string) (rest []string, sawDry, sawJSON bool) {
	boolVal := func(v string) bool {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return true // unparseable ⇒ fail closed (dry-run wins over mutating)
		}
		return b
	}
	for _, a := range args {
		name, val, hasVal := strings.Cut(a, "=")
		switch name {
		case "--dry-run":
			if !hasVal {
				sawDry = true
			} else if boolVal(val) {
				sawDry = true
			}
		case "--json":
			if !hasVal {
				sawJSON = true
			} else if boolVal(val) {
				sawJSON = true
			}
		default:
			rest = append(rest, a)
		}
	}
	return rest, sawDry, sawJSON
}

// emitPawlDryRunPlan reports the plan: with --json exactly ONE JSON object (D2), else
// human "DRY-RUN … would:" lines. This is the entire dry-run execution — nothing runs.
func emitPawlDryRunPlan(cmd *cobra.Command, sub string, args []string, jsonOut bool) error {
	doc := pawlDryRunPlan(sub, args)
	if jsonOut {
		b, err := json.Marshal(doc)
		if err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(b))
		return nil
	}
	fmt.Fprintf(cmd.OutOrStdout(), "DRY-RUN %s: no mutation performed (session=%s families=%s tier=%s)\n",
		doc.Action, doc.Session, doc.Families, doc.Tier)
	for _, s := range doc.PlannedSteps {
		fmt.Fprintf(cmd.OutOrStdout(), "  would: %s\n", s)
	}
	return nil
}

func runPawlReview(cmd *cobra.Command, args []string) error {
	// -h/--help is for THIS command, not the script.
	for _, a := range args {
		if a == "-h" || a == "--help" {
			return cmd.Help()
		}
	}
	// D1: a review writes verdicts/evidence — under global --dry-run plan and report
	// without executing the script at all. DisableFlagParsing means --dry-run/--json may
	// arrive as raw args; on a REAL run the original args are forwarded verbatim (the
	// script owns its flag contract).
	if rest, sawDry, sawJSON := stripPawlPassthroughFlags(args); GetDryRun() || sawDry {
		return emitPawlDryRunPlan(cmd, "review", rest, jsonFlag || sawJSON)
	}
	// EDGE 1 — GENUINE in-AgentOps dogfood: run the LIVE scripts so a script edit is
	// immediately exercised. SECURITY: "has the AgentOps marker files" is NOT enough —
	// a repo under review can FORGE docs/contracts/agents-write-surfaces.md + skills/ +
	// scripts/pawl-review.sh, which would make an installed `ao pawl review` execute the
	// repo's PLANTED script (RCE). The trust test is therefore stronger: take the live
	// path only when the running ao BINARY physically lives inside the resolved checkout
	// (i.e. we are running THIS repo's own build). An installed ao on a foreign/forged
	// repo fails this test → the embedded + untrusted-guard path. (If a user runs a repo's
	// OWN cli/bin/ao they have already chosen to execute that repo's code — not our boundary.)
	if repoRoot, err := resolveAgentsRepoRoot(); err == nil && aoBinaryInside(repoRoot) {
		script := filepath.Join(repoRoot, defaultPawlReviewScript)
		if _, statErr := os.Stat(script); statErr != nil {
			return fmt.Errorf("pawl-review script not found at %s: %w", script, statErr)
		}
		return runForwardedPawlScript(cmd, script, repoRoot, "", args, nil)
	}
	// Stranger path: not a genuine AgentOps checkout (installed ao, or forged markers).
	// Run the EMBEDDED scripts against the user's OWN git repo so the cross-family catch
	// works zero-config — decoupled from resolveAgentsRepoRoot, never executing a script
	// from the untrusted repo under review.
	return runPawlReviewEmbedded(cmd, args)
}

// aoBinaryInside reports whether the running ao binary physically lives inside repoRoot —
// the trust test for "this is genuinely our own checkout" (forge-proof, unlike marker
// files). Symlinks are resolved on both sides so a symlinked install/checkout compares
// correctly. On any resolution error it returns false (fail-safe → embedded path).
func aoBinaryInside(repoRoot string) bool {
	self, err := pawlSelfBinary()
	if err != nil || self == "" {
		return false
	}
	return pathInside(realpathOrSelf(self), realpathOrSelf(repoRoot))
}

// realpathOrSelf and pathInside now live in path_containment.go (same package) so the tar
// extractor in corpus_snapshot.go and this file share ONE canonical containment implementation.

// trustedPATH returns the process PATH with every entry dropped that the untrusted repo
// could control: empty, ".", any relative entry (resolves against cwd = the repo), AND any
// absolute entry that lives INSIDE excludeRoot (e.g. a user who put $PWD/bin on PATH while
// reviewing). excludeRoot "" only strips the relative entries. This is what stops a bare
// git/bash/codex/jq/timeout resolving to a repo-planted binary.
func trustedPATH(excludeRoot string) string {
	sep := string(os.PathListSeparator)
	var rootReal string
	if excludeRoot != "" {
		rootReal = realpathOrSelf(excludeRoot)
	}
	var kept []string
	for _, p := range strings.Split(os.Getenv("PATH"), sep) {
		if p == "" || p == "." || !filepath.IsAbs(p) {
			continue
		}
		if rootReal != "" && pathInside(realpathOrSelf(p), rootReal) {
			continue
		}
		kept = append(kept, p)
	}
	return strings.Join(kept, sep)
}

// trustedLookPath finds an executable named name on the trusted PATH (excludeRoot's dirs
// removed), returning an absolute path. It never consults `.`/relative or repo-internal
// dirs, so the resolved binary is never one the untrusted repo controls.
func trustedLookPath(name, excludeRoot string) (string, error) {
	sep := string(os.PathListSeparator)
	for _, dir := range strings.Split(trustedPATH(excludeRoot), sep) {
		cand := filepath.Join(dir, name)
		if info, err := os.Stat(cand); err == nil && !info.IsDir() && info.Mode().Perm()&0o111 != 0 {
			return cand, nil
		}
	}
	return "", fmt.Errorf("%s not found on a trusted PATH (repo-internal/relative entries excluded)", name)
}

// runPawlReviewEmbedded runs the embedded pawl scripts against the user's own git
// repository. It extracts the scripts/ + schemas/ sibling bundle from the binary to a
// temp dir and points the scripts at the user's repo via the existing env seams
// (AGENTOPS_REPO_ROOT for git ops + verdict/yield dir).
//
// cwd is the user's repo so the read-only codex refuter can READ the changed files there
// (large diffs elide added lines and require reading the files); the bare-binary RCE class
// is closed instead by a SANITIZED PATH (pawlReviewColdEnv) that drops every `.`/relative
// entry, so a bare codex/git/jq/timeout can never resolve a planted `./binary` from the
// repo — plus the per-binary AO_BIN guards for the subshells that cd into the repo.
func runPawlReviewEmbedded(cmd *cobra.Command, args []string) error {
	startDir, err := resolveProjectDir()
	if err != nil {
		return err
	}
	userRoot, err := gitToplevel(startDir)
	if err != nil {
		return fmt.Errorf("ao pawl review must run inside a git repository (it reviews your latest commit): %w", err)
	}
	cacheDir, cleanup, err := extractPawlBundle()
	if err != nil {
		return fmt.Errorf("preparing embedded pawl scripts: %w", err)
	}
	defer cleanup()
	script := filepath.Join(cacheDir, "scripts", filepath.Base(defaultPawlReviewScript))
	return runForwardedPawlScript(cmd, script, userRoot, userRoot, args, pawlReviewColdEnv(userRoot))
}

// pawlReviewColdEnv is the env overlay for the stranger (embedded) path. It re-roots the
// scripts onto the user's repo (git ops, verdict + yield ledger writes). EDGE 2: the
// schema is resolved script-relative from the extracted bundle, so no override is needed
// for it — only the user-repo seams.
//
// SECURITY: the repo under review is UNTRUSTED while it is cwd, so:
//   - PATH is SANITIZED (every empty/"."/relative entry dropped) so a bare codex/git/jq
//     the scripts invoke can never resolve a planted `./binary` from the repo;
//   - PAWL_UNTRUSTED_REPO=1 stops the scripts executing anything from $REPO_ROOT/cli/*;
//   - AO_BIN pins the membrane catch/recall/emits to THIS trusted invoking binary.
func pawlReviewColdEnv(userRoot string) []string {
	env := []string{
		"AGENTOPS_REPO_ROOT=" + userRoot,
		"PAWL_NO_SERVICE=1",
		"PAWL_UNTRUSTED_REPO=1",
		"PATH=" + trustedPATH(userRoot),
		// Neutralize shell-startup injection: non-interactive bash sources $BASH_ENV (and
		// $ENV in POSIX mode) BEFORE the script, so a repo-controlled value would run code
		// despite the PATH guard. Empty disables it.
		"BASH_ENV=",
		"ENV=",
		// Belt-and-suspenders for git diff-helper code-exec: the script also passes
		// --no-ext-diff; clearing GIT_EXTERNAL_DIFF closes the env route too.
		"GIT_EXTERNAL_DIFF=",
	}
	if self, err := pawlSelfBinary(); err == nil && self != "" {
		env = append(env, "AO_BIN="+self)
	}
	return env
}

// pawlSelfBinary resolves the trusted invoking ao binary (production: os.Executable()).
// It is a seam so tests can point AO_BIN at a benign fake ao — the test binary itself
// would re-run the whole suite when the script invokes it with `membrane recall …`.
var pawlSelfBinary = os.Executable

// runForwardedPawlScript runs a pawl script under bash, forwarding stdio + args and
// propagating the script's exit code verbatim (the exit code IS the verdict). extraEnv,
// when non-nil, is appended to the process environment (the stranger-path seams).
func runForwardedPawlScript(cmd *cobra.Command, script, dir, untrustedRoot string, args, extraEnv []string) error {
	// Resolve bash to an ABSOLUTE path on a TRUSTED PATH (no `.`/relative entries, no dir
	// inside untrustedRoot) so the Go-side launch can never pick a planted bash the repo
	// controls. untrustedRoot is "" on the in-checkout path (PATH is trusted there).
	bashBin, err := trustedLookPath("bash", untrustedRoot)
	if err != nil {
		return fmt.Errorf("locating bash: %w", err)
	}
	c := exec.Command(bashBin, append([]string{script}, args...)...) // #nosec G204 -- args are operator-supplied pawl flags forwarded to a fixed in-repo/embedded script.
	c.Dir = dir
	c.Stdin = cmd.InOrStdin()
	c.Stdout = cmd.OutOrStdout()
	c.Stderr = cmd.ErrOrStderr()
	if extraEnv != nil {
		c.Env = append(os.Environ(), extraEnv...)
	}
	runErr := c.Run()
	if runErr == nil {
		return nil
	}
	// The script's exit code is the verdict — propagate it verbatim, with no extra
	// cobra usage/error noise (the script already printed the verdict + defects).
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	var exitErr *exec.ExitError
	if errors.As(runErr, &exitErr) {
		return &pawlReviewExitError{code: exitErr.ExitCode()}
	}
	return runErr
}

// gitToplevel returns the working-tree root containing dir — the review target on the
// stranger path. It walks up for a `.git` entry (a directory for a normal clone, a file for
// a worktree/submodule) in PURE Go, executing NO binary, so a git the repo planted can
// never run during root discovery (the chicken-and-egg: you can't trust a PATH-resolved git
// to find the very repo whose trust you're establishing). The result is symlink-resolved so
// it compares correctly against PATH entries in trustedPATH.
func gitToplevel(dir string) (string, error) {
	cur := realpathOrSelf(dir)
	for {
		if _, err := os.Stat(filepath.Join(cur, ".git")); err == nil {
			return cur, nil
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return "", fmt.Errorf("not inside a git repository (no .git found from %s)", dir)
		}
		cur = parent
	}
}

// extractPawlBundle materializes the embedded pawl bundle (scripts/ + schemas/) to a
// fresh temp dir, preserving the sibling layout pawl-verdict.sh depends on (it reads
// its schema as $SCRIPT_DIR/../schemas/pawl-verdict.v1.schema.json). Returns the dir
// and a cleanup func. Shell scripts are written executable + CRLF-normalized.
func extractPawlBundle() (string, func(), error) {
	dir, err := os.MkdirTemp("", "ao-pawl-*")
	if err != nil {
		return "", func() {}, fmt.Errorf("create pawl cache dir: %w", err)
	}
	cleanup := func() { _ = os.RemoveAll(dir) }
	walkErr := fs.WalkDir(embedded.PawlFS, "pawl", func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, relErr := filepath.Rel("pawl", p)
		if relErr != nil {
			return relErr
		}
		if rel == "." {
			return nil
		}
		dest := filepath.Join(dir, rel)
		if d.IsDir() {
			return os.MkdirAll(dest, 0o755)
		}
		data, readErr := fs.ReadFile(embedded.PawlFS, p)
		if readErr != nil {
			return readErr
		}
		mode := os.FileMode(0o600)
		if strings.HasSuffix(rel, ".sh") {
			data = normalizeShellScript(data)
			mode = 0o700
		}
		if mkErr := os.MkdirAll(filepath.Dir(dest), 0o755); mkErr != nil {
			return mkErr
		}
		return os.WriteFile(dest, data, mode)
	})
	if walkErr != nil {
		cleanup()
		return "", func() {}, fmt.Errorf("extract embedded pawl bundle: %w", walkErr)
	}
	return dir, cleanup, nil
}

// defaultPawlVerdictScript is the verdict writer/checker (sibling of pawl-review.sh),
// the script that owns the `rebind-verified` verb `ao verify --rebind` forwards to.
const defaultPawlVerdictScript = "scripts/pawl-verdict.sh"

// resolvePawlVerdictScript resolves scripts/pawl-verdict.sh under the SAME trust split
// runPawlReview uses: the LIVE repo script when the running ao binary physically lives
// inside a genuine AgentOps checkout (dogfood — a script edit is exercised immediately),
// else the EMBEDDED bundle extracted to a temp dir against the user's own repo (zero
// config, never executing a script from the untrusted repo under review). Returns the
// script path, the working dir (the repo being operated on), the extra env overlay
// (nil in-checkout; the sanitized cold seams on the stranger path), and a cleanup func.
func resolvePawlVerdictScript() (script, dir string, extraEnv []string, cleanup func(), err error) {
	cleanup = func() {}
	if repoRoot, rerr := resolveAgentsRepoRoot(); rerr == nil && aoBinaryInside(repoRoot) {
		script = filepath.Join(repoRoot, defaultPawlVerdictScript)
		if _, statErr := os.Stat(script); statErr != nil {
			return "", "", nil, cleanup, fmt.Errorf("pawl-verdict script not found at %s: %w", script, statErr)
		}
		return script, repoRoot, nil, cleanup, nil
	}
	// Stranger path: extract the embedded bundle and run against the user's own repo.
	startDir, derr := resolveProjectDir()
	if derr != nil {
		return "", "", nil, cleanup, derr
	}
	userRoot, terr := gitToplevel(startDir)
	if terr != nil {
		return "", "", nil, cleanup, fmt.Errorf("ao verify --rebind must run inside a git repository: %w", terr)
	}
	cacheDir, bcleanup, xerr := extractPawlBundle()
	if xerr != nil {
		return "", "", nil, cleanup, fmt.Errorf("preparing embedded pawl scripts: %w", xerr)
	}
	script = filepath.Join(cacheDir, "scripts", filepath.Base(defaultPawlVerdictScript))
	return script, userRoot, pawlReviewColdEnv(userRoot), bcleanup, nil
}
