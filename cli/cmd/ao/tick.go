// practices: [design-by-contract, continuous-delivery]
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/boshu2/agentops/cli/internal/liveness"
	"github.com/spf13/cobra"
)

const (
	tickExitUsage      = 1
	tickExitCloseRef   = 3
	tickExitNoCommit   = 4
	tickExitUnverified = 5
	tickExitCouncil    = 6
	tickExitDisagree   = 8
	tickExitCloseFail  = 10
)

type tickExitError struct {
	code int
	msg  string
}

func (e *tickExitError) Error() string { return e.msg }
func (e *tickExitError) ExitCode() int { return e.code }

type tickRuntime struct {
	workDir string
	stdin   io.Reader
	stdout  io.Writer
	stderr  io.Writer
	env     []string
	// requireCrossFamily is the OPTIONAL cross-family strengthener. Default false
	// = the distinct-context floor IS the floor; true = additionally require >=2
	// model families in the PASS quorum (newTickRuntime leaves it false).
	requireCrossFamily bool
}

func newTickRuntime(cmd *cobra.Command) tickRuntime {
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}
	return tickRuntime{
		workDir: cwd,
		stdin:   cmd.InOrStdin(),
		stdout:  cmd.OutOrStdout(),
		stderr:  cmd.ErrOrStderr(),
	}
}

func (rt tickRuntime) run(name string, args ...string) ([]byte, int, error) {
	c := exec.Command(name, args...)
	c.Dir = rt.workDir
	env := append(os.Environ(), rt.env...)
	if name == "br" {
		if _, ok := beadsEnvValue(env); !ok {
			env = append(env, "BEADS_DIR="+resolveBeadsDir(rt.workDir, env).Path)
		}
	}
	if len(env) > 0 {
		c.Env = env
	}
	out, err := c.CombinedOutput()
	if err == nil {
		return out, 0, nil
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return out, exitErr.ExitCode(), err
	}
	return out, 127, err
}

var tickCmd = &cobra.Command{
	Use:   "tick",
	Short: "Typed port of the assured loop tick oracle",
	Long: `Run the typed AgentOps port of the control-plane tick helper.

The shell oracle remains the regression source until conformance proves the Go
surface. This command preserves the same state-store boundary: br is the work
bus, git is the durable ledger, and only explicit scoped paths are staged.`,
}

var tickNextCmd = &cobra.Command{
	Use:   "next",
	Short: "Print the next ready bead id",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		rt := newTickRuntime(cmd)
		id := tickNextReady(rt)
		if id != "" {
			fmt.Fprintln(rt.stdout, id)
		}
		return nil
	},
}

var tickStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Print ready-work status or convergence state",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		return tickStatus(newTickRuntime(cmd))
	},
}

var tickClaimCmd = &cobra.Command{
	Use:   "claim <id>",
	Short: "Claim a bead through br",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return tickPassthrough(newTickRuntime(cmd), "br", "update", args[0], "--claim")
	},
}

var tickReopenCmd = &cobra.Command{
	Use:   "reopen <id>",
	Short: "Reopen a bead after failed validation",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return tickPassthrough(newTickRuntime(cmd), "br", "update", args[0], "--status", "open")
	},
}

var tickCloseCmd = &cobra.Command{
	Use:   "close <id> <commit-message> <evidence-ref> [paths...]",
	Short: "Close a bead and persist the explicit ledger/evidence paths",
	Args:  cobra.MinimumNArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		return tickClose(newTickRuntime(cmd), args[0], args[1], args[2], args[3:])
	},
}

var tickVerdictGateCmd = &cobra.Command{
	Use:   "verdict-gate <file|->",
	Short: "Reject verdicts without commands and independent judge identity",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return tickRunVerdictGate(newTickRuntime(cmd), args[0])
	},
}

var tickCouncilGateCmd = &cobra.Command{
	Use:   "council-gate <verdict1> <verdict2> [...]",
	Short: "Fail-closed two-plus judge verdict aggregation",
	Args:  cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		return tickCouncilGate(newTickRuntime(cmd), args)
	},
}

var tickInstallGuardsCmd = &cobra.Command{
	Use:   "install-guards",
	Short: "Install repo-local git guard hooks",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		return tickInstallGuards(newTickRuntime(cmd))
	},
}

var tickGuardStatusCmd = &cobra.Command{
	Use:   "guard-status",
	Short: "Verify guard hook and validator launcher installation",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		return tickGuardStatus(newTickRuntime(cmd))
	},
}

var tickSmokeCmd = &cobra.Command{
	Use:   "smoke",
	Short: "Run a read-only smoke test of the tick membrane",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		return tickSmoke(newTickRuntime(cmd))
	},
}

var verdictGateCmd = &cobra.Command{
	Use:   "verdict-gate <file|->",
	Short: "Reject verdicts without commands and independent judge identity",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return tickRunVerdictGate(newTickRuntime(cmd), args[0])
	},
}

var councilGateCmd = &cobra.Command{
	Use:   "council-gate <verdict1> <verdict2> [...]",
	Short: "Fail-closed two-plus judge verdict aggregation",
	Args:  cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		return tickCouncilGate(newTickRuntime(cmd), args)
	},
}

var installGuardsCmd = &cobra.Command{
	Use:   "install-guards",
	Short: "Install repo-local git guard hooks",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		return tickInstallGuards(newTickRuntime(cmd))
	},
}

var guardStatusCmd = &cobra.Command{
	Use:   "guard-status",
	Short: "Verify guard hook and validator launcher installation",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		return tickGuardStatus(newTickRuntime(cmd))
	},
}

var chaosTestCmd = &cobra.Command{
	Use:   "chaos-test",
	Short: "Run a read-only smoke test of the tick membrane",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		return tickSmoke(newTickRuntime(cmd))
	},
}

var readyCmd = &cobra.Command{
	Use:   "ready",
	Short: "Print harness-neutral ready bead state as JSON",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		return tickReady(newTickRuntime(cmd))
	},
}

var closeCmd = &cobra.Command{
	Use:   "close <id> <commit-message> <evidence-ref> [paths...]",
	Short: "Close a bead and persist the explicit ledger/evidence paths",
	Args:  cobra.MinimumNArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		return tickClosePort(newTickRuntime(cmd), args[0], args[1], args[2], args[3:])
	},
}

func init() {
	tickCmd.GroupID = "workflow"
	rootCmd.AddCommand(tickCmd)
	rootCmd.AddCommand(
		readyCmd,
		closeCmd,
		verdictGateCmd,
		councilGateCmd,
		installGuardsCmd,
		guardStatusCmd,
		chaosTestCmd,
	)
	tickCmd.AddCommand(
		tickNextCmd,
		tickStatusCmd,
		tickClaimCmd,
		tickReopenCmd,
		tickCloseCmd,
		tickVerdictGateCmd,
		tickCouncilGateCmd,
		tickInstallGuardsCmd,
		tickGuardStatusCmd,
		tickSmokeCmd,
	)
}

func tickPassthrough(rt tickRuntime, name string, args ...string) error {
	out, code, err := rt.run(name, args...)
	if len(out) > 0 {
		_, _ = rt.stdout.Write(out)
	}
	if err != nil {
		return &tickExitError{code: code, msg: strings.TrimSpace(err.Error())}
	}
	return nil
}

type tickBead struct {
	ID       string `json:"id"`
	Title    string `json:"title,omitempty"`
	Status   string `json:"status"`
	Priority int    `json:"priority,omitempty"`
}

type tickReadyState struct {
	StateSource string     `json:"state_source"`
	GitHead     string     `json:"git_head,omitempty"`
	Next        string     `json:"next,omitempty"`
	Ready       []tickBead `json:"ready"`
	Counts      tickCounts `json:"counts"`
}

type tickCounts struct {
	Ready      int `json:"ready"`
	Open       int `json:"open"`
	InProgress int `json:"in_progress"`
	Closed     int `json:"closed"`
}

func tickReady(rt tickRuntime) error {
	readyOut, code, err := rt.run("br", "ready", "--json")
	if err != nil || code != 0 {
		return &tickExitError{code: code, msg: "br ready --json failed"}
	}
	ready := tickParseBeads(readyOut)

	allOut, code, err := rt.run("br", "list", "--all", "--json")
	if err != nil || code != 0 {
		return &tickExitError{code: code, msg: "br list --all --json failed"}
	}
	all := tickParseBeads(allOut)
	state := tickReadyState{
		StateSource: "br",
		GitHead:     tickGitRevParse(rt),
		Ready:       ready,
		Counts:      tickCountBeads(ready, all),
	}
	if len(ready) > 0 {
		state.Next = ready[0].ID
	}
	enc := json.NewEncoder(rt.stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(state)
}

func tickNextReady(rt tickRuntime) string {
	if out, code, err := rt.run("br", "ready", "--json"); err == nil && code == 0 {
		if id := tickFirstReadyFromJSON(out); id != "" {
			return id
		}
	}
	out, _, _ := rt.run("br", "ready")
	return regexp.MustCompile(`cp-[A-Za-z0-9.]+`).FindString(string(out))
}

func tickFirstReadyFromJSON(out []byte) string {
	if list := tickParseBeads(out); len(list) > 0 {
		return list[0].ID
	}
	return ""
}

func tickParseBeads(out []byte) []tickBead {
	var list []tickBead
	if err := json.Unmarshal(out, &list); err == nil {
		return list
	}
	var wrapped struct {
		Issues []tickBead `json:"issues"`
	}
	if err := json.Unmarshal(out, &wrapped); err == nil {
		return wrapped.Issues
	}
	return nil
}

func tickCountBeads(ready, all []tickBead) tickCounts {
	counts := tickCounts{Ready: len(ready)}
	for _, item := range all {
		switch item.Status {
		case "open":
			counts.Open++
		case "in_progress":
			counts.InProgress++
		case "closed":
			counts.Closed++
		}
	}
	return counts
}

func tickStatus(rt tickRuntime) error {
	out, _, _ := rt.run("br", "ready")
	if regexp.MustCompile(`cp-`).Match(out) {
		_, _ = rt.stdout.Write(out)
		return nil
	}
	if tickHasOpenOrInProgress(rt) {
		fmt.Fprintln(rt.stdout, "NO_READY: no schedulable beads; remaining work is blocked or in progress")
	} else {
		fmt.Fprintln(rt.stdout, "CONVERGED: no open or in-progress beads")
	}
	return nil
}

func tickHasOpenOrInProgress(rt tickRuntime) bool {
	out, code, err := rt.run("br", "list", "--all", "--json")
	if err != nil || code != 0 {
		return false
	}
	var list []tickBead
	if err := json.Unmarshal(out, &list); err == nil {
		return tickAnyOpenOrInProgress(list)
	}
	var wrapped struct {
		Issues []tickBead `json:"issues"`
	}
	if err := json.Unmarshal(out, &wrapped); err == nil {
		return tickAnyOpenOrInProgress(wrapped.Issues)
	}
	return false
}

func tickAnyOpenOrInProgress(list []tickBead) bool {
	for _, item := range list {
		if item.Status == "open" || item.Status == "in_progress" {
			return true
		}
	}
	return false
}

func tickClose(rt tickRuntime, id, msg, evidence string, paths []string) error {
	evFirst := tickFirstEvidenceToken(evidence)
	if reason := tickEvidenceRefusal(rt, evidence, evFirst); reason != "" {
		fmt.Fprintf(rt.stderr, "REFUSED close %s: evidence %q %s\n", id, evidence, reason)
		return &tickExitError{code: tickExitCloseRef}
	}

	ledgerDir := tickLedgerDir(rt)
	before := tickGitRevParseInDir(rt, ledgerDir)
	if before == "" {
		before = "none"
	}
	if _, code, err := rt.run("br", "close", id, "--reason", "evidence: "+evidence); err != nil || code != 0 {
		fmt.Fprintf(rt.stderr, "FAILED close %s: br close failed or skipped; no files staged\n", id)
		return &tickExitError{code: tickExitCloseFail}
	}
	if _, code, err := rt.run("br", "sync", "--flush-only"); err != nil || code != 0 {
		_, _, _ = rt.run("br", "sync")
	}
	issuesPath := filepath.Join(ledgerDir, "issues.jsonl")
	metadataPath := filepath.Join(ledgerDir, "metadata.json")
	if !tickLedgerShowsClosed(issuesPath, id) {
		_, _, _ = rt.run("br", "update", id, "--status", "open")
		_, _, _ = rt.run("br", "sync", "--flush-only")
		fmt.Fprintf(rt.stderr, "FAILED close %s: ledger does not show a closed bead after br close; bead reopened\n", id)
		return &tickExitError{code: tickExitCloseFail}
	}

	ledgerStage := []string{"-C", ledgerDir, "add", "--", "issues.jsonl"}
	if tickPathExists(rt.workDir, metadataPath) {
		ledgerStage = append(ledgerStage, "metadata.json")
	}
	if _, code, err := rt.run("git", ledgerStage...); err != nil || code != 0 {
		return &tickExitError{code: code, msg: "ledger git add failed"}
	}
	ledgerCommitOut, code, err := rt.run("git", "-C", ledgerDir, "commit", "-q", "-m", msg)
	ledgerCommitNoop := false
	if err != nil || code != 0 {
		if tickGitCommitNothingToCommit(ledgerCommitOut) {
			ledgerCommitNoop = true
		} else {
			return &tickExitError{code: code, msg: "ledger git commit failed"}
		}
	}
	after := tickGitRevParseInDir(rt, ledgerDir)
	if after == "" {
		after = "none"
	}
	if before == after && !ledgerCommitNoop {
		_, _, _ = rt.run("br", "update", id, "--status", "open")
		fmt.Fprintf(rt.stderr, "FAILED close %s: ledger git commit did not land; bead reopened\n", id)
		return &tickExitError{code: tickExitNoCommit}
	}

	stage := tickPublicStagePaths(rt, ledgerDir, evFirst, paths)
	if len(stage) > 0 {
		args := append([]string{"add", "--"}, stage...)
		if _, code, err := rt.run("git", args...); err != nil || code != 0 {
			return &tickExitError{code: code, msg: "git add failed"}
		}
		out, code, err := rt.run("git", "commit", "-q", "-m", msg)
		if (err != nil || code != 0) && !tickGitCommitNothingToCommit(out) {
			return &tickExitError{code: code, msg: "git commit failed"}
		}
	}
	fmt.Fprintf(rt.stdout, "closed %s @ %s\n", id, tickShortSHA(after))
	return nil
}

func tickClosePort(rt tickRuntime, id, msg, evidence string, paths []string) error {
	if tickLedgerShowsClosed(filepath.Join(tickLedgerDir(rt), "issues.jsonl"), id) {
		after := tickGitRevParse(rt)
		if after == "" {
			after = "none"
		}
		fmt.Fprintf(rt.stdout, "already closed %s @ %s\n", id, tickShortSHA(after))
		return nil
	}
	return tickClose(rt, id, msg, evidence, paths)
}

// tickEvidenceRefusal applies the close-evidence gate and returns a non-empty
// reason if the close must be refused, or "" if it may proceed.
//
// Durable git-blob binding (age-l7yh): a repo-internal evidence path must be a
// committed git blob in HEAD (an ancestor of the close commit tickClose is about
// to create). Evidence that is present in the working tree but not in history
// looks durable yet isn't — refuse it. The gate uses git itself for every
// classification (repo-root relativization, cat-file, ls-tree, diff,
// check-ignore) rather than string heuristics on the cited text, so it cannot be
// bypassed by an absolute path inside the repo, a subdirectory-relative path, a
// crafted substring, a committed directory (tree), a dirty tracked file, a
// committed symlink, or a symlink/component whose target escapes the repo.
//
// Only two classes are allowed:
//   - a committed, clean, regular-file blob in HEAD (the durable ideal);
//   - the gitignored AgentOps runtime corpus (.agents/**), confirmed via
//     git check-ignore — ephemeral by design (admitted state under .ao/ is
//     tracked and so passes the committed-blob check directly).
//
// Everything else is refused, INCLUDING evidence that resolves outside the repo:
// it cannot be a durable in-repo git blob, and exempting outside paths was the
// fail-open that crafted symlinks exploited.
//
// No Agent-Mail lookup is involved, so Agent-Mail availability never blocks a
// close. This strengthens "no verdict = not done" without replacing main's
// ContextID-independence verdict model. Where there is no git history to bind to
// (not a repo, before the first commit) or repo placement is undeterminable
// (a stubbed harness), the binding falls back to plain existence so a legit
// close is not blocked.
func tickEvidenceRefusal(rt tickRuntime, evidence, evFirst string) string {
	_ = evidence // classification is by the cited token + git, not the raw text
	if evFirst == "" {
		return "resolves to no real artifact"
	}
	// Resolve once: tickResolvePath returns evFirst unchanged when it is already
	// absolute, else joins it under rt.workDir. os.Stat on that absolute path is
	// the existence check — an absolute evidence path is never falsely treated as
	// missing by being re-joined under workDir.
	abs := tickResolvePath(rt.workDir, evFirst)
	if _, err := os.Stat(abs); err != nil {
		return "resolves to no real artifact"
	}
	if !tickHasGitHead(rt) {
		return "" // no history to bind to — existence is the only available check
	}
	root, rel, ok := tickRepoRelEvidence(rt, evFirst)
	if !ok {
		return "" // repo placement undeterminable (e.g. a stubbed test harness)
	}
	if rel == ".." || strings.HasPrefix(rel, "../") {
		// Resolves outside the repo (including via a symlink whose target escapes
		// it). Such evidence cannot be a durable in-repo git blob, so refuse it
		// rather than exempt it — exempting outside paths was the fail-open that
		// let crafted symlinks bypass the binding.
		return "resolves outside the repository; durable evidence must be a committed file tracked in this repo"
	}
	if tickEvidenceCommittedRel(rt, root, rel) {
		return "" // durable: a committed regular-file blob, ancestor of the close commit
	}
	if tickEvidenceRuntimeCorpus(rt, root, rel) {
		return "" // gitignored .agents/** runtime corpus — ephemeral by design
	}
	return "is present but not a committed git blob in history (durable-evidence binding); commit the evidence before closing"
}

// tickHasGitHead reports whether rt.workDir is inside a git repo with at least
// one commit (a resolvable HEAD). Used to gate the durable-evidence binding so
// it never fires where there is no history to bind to.
func tickHasGitHead(rt tickRuntime) bool {
	_, code, err := rt.run("git", "rev-parse", "--verify", "-q", "HEAD")
	return err == nil && code == 0
}

// tickRepoRelEvidence maps the cited evidence token to the repo top-level plus a
// clean repo-root-relative slash path. The bool reports only whether repo
// placement could be DETERMINED (a real git top-level was resolved) — not
// whether the path is inside; the returned rel may begin with "../" when the
// path escapes the repo, and the caller refuses that. Relativizing against the
// repo top-level — rather than trusting the caller's spelling — is what makes the
// binding immune to absolute-inside-repo and subdirectory bypasses.
//
// The parent directory is symlink-resolved (so a symlinked prefix like macOS
// /var -> /private/var, or any symlinked component that escapes the repo,
// resolves to its real location and is then correctly judged inside or outside)
// while the leaf name is kept as-is. A leaf or component symlink that escapes the
// repo yields a "../" rel and is refused by the caller — escaping is never
// exempted.
func tickRepoRelEvidence(rt tickRuntime, evFirst string) (root, rel string, ok bool) {
	out, code, err := rt.run("git", "rev-parse", "--show-toplevel")
	if err != nil || code != 0 {
		return "", "", false
	}
	root = strings.TrimSpace(string(out))
	// A real git top-level is an absolute path; anything else (e.g. a stubbed
	// test harness echoing a token) means placement is undeterminable.
	if !filepath.IsAbs(root) {
		return "", "", false
	}
	if resolved, e := filepath.EvalSymlinks(root); e == nil {
		root = resolved
	}
	var lexAbs string
	if filepath.IsAbs(evFirst) {
		// Resolve the parent directory (so a symlinked prefix like /var ->
		// /private/var matches the resolved root) but keep the leaf name as-is, so
		// a leaf symlink is not followed out of the repo.
		clean := filepath.Clean(evFirst)
		parent := filepath.Dir(clean)
		if resolved, e := filepath.EvalSymlinks(parent); e == nil {
			parent = resolved
		}
		lexAbs = filepath.Join(parent, filepath.Base(clean))
	} else {
		base := rt.workDir
		if resolved, e := filepath.EvalSymlinks(base); e == nil {
			base = resolved
		}
		lexAbs = filepath.Clean(filepath.Join(base, evFirst))
	}
	r, err := filepath.Rel(root, lexAbs)
	if err != nil {
		return root, "", false // cannot relativize (e.g. different volume) — undeterminable
	}
	// Determinable: r is the repo-relative path. It may begin with "../" when the
	// path (or a resolved symlink target) escapes the repo; the caller refuses
	// that rather than exempting it.
	return root, filepath.ToSlash(r), true
}

// tickEvidenceCommittedRel reports whether the repo-root-relative path rel is
// durable evidence: a committed FILE in HEAD whose working-tree content matches
// HEAD exactly. Two checks:
//
//   - `git cat-file -t HEAD:<rel>` must report type "blob" — `-e` alone also
//     succeeds for a tree, which would let a close cite a whole committed
//     directory (e.g. docs/) instead of a specific durable artifact. rel is
//     root-relative, which is what the HEAD:<path> form expects.
//   - `git -C root diff --quiet HEAD -- <rel>` must report no difference — a
//     committed path with uncommitted or staged content changes is NOT durably
//     in history, so the cited working-tree artifact must equal the committed
//     blob. diff runs with -C root because its pathspec is cwd-relative whereas
//     rel is root-relative.
//
// A clean committed regular-file blob is durable evidence reachable from (an
// ancestor of) the close commit. A committed SYMLINK is also a blob, but its
// bytes are just the target path — the pointed-at content is not in history — so
// it is rejected via its tree mode (120000).
func tickEvidenceCommittedRel(rt tickRuntime, root, rel string) bool {
	if rel == "" {
		return false
	}
	out, code, err := rt.run("git", "cat-file", "-t", "HEAD:"+rel)
	if err != nil || code != 0 || strings.TrimSpace(string(out)) != "blob" {
		return false
	}
	// Reject a committed symlink: ls-tree reports its mode as 120000. Require a
	// regular file (100644/100755) so the durable bytes ARE the evidence, not a
	// pointer to an untracked, possibly-external, mutable target.
	lt, ltcode, lterr := rt.run("git", "-C", root, "ls-tree", "HEAD", "--", rel)
	if lterr != nil || ltcode != 0 {
		return false
	}
	if mode := strings.TrimSpace(string(lt)); !strings.HasPrefix(mode, "100") {
		return false
	}
	_, dcode, derr := rt.run("git", "-C", root, "diff", "--quiet", "HEAD", "--", rel)
	return derr == nil && dcode == 0
}

// tickEvidenceRuntimeCorpus reports whether the repo-root-relative path rel is
// in the AgentOps runtime corpus (.agents/**) AND is actually gitignored. Both
// conditions are required: scoping to .agents/** keeps other gitignored paths
// (build artifacts, logs, temp files) from serving as durable evidence, and
// confirming via git check-ignore honors the documented exemption being for the
// *gitignored* corpus (a force-added, tracked .agents path is not exempt — it is
// committed and so passes the blob check anyway). check-ignore runs with -C root
// so the root-relative rel resolves correctly regardless of rt.workDir.
func tickEvidenceRuntimeCorpus(rt tickRuntime, root, rel string) bool {
	if rel != ".agents" && !strings.HasPrefix(rel, ".agents/") {
		return false
	}
	_, code, err := rt.run("git", "-C", root, "check-ignore", "-q", "--", rel)
	return err == nil && code == 0
}

func tickFirstEvidenceToken(evidence string) string {
	ev := strings.TrimSpace(evidence)
	if i := strings.IndexByte(ev, ' '); i >= 0 {
		return ev[:i]
	}
	return ev
}

func tickPathExists(root, path string) bool {
	if path == "" {
		return false
	}
	_, err := os.Stat(tickResolvePath(root, path))
	return err == nil
}

func tickResolvePath(root, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(root, path)
}

// tickLedgerDir resolves the tracker ledger directory used for close
// verification and staging. Resolution order matches what spawned `br`
// child processes see: explicit BEADS_DIR (rt.env overrides inherited process
// env), then the canonical git-common-dir `_beads` ledger for linked worktrees.
func tickLedgerDir(rt tickRuntime) string {
	if dir := tickEnvValue(rt, "BEADS_DIR"); dir != "" {
		return tickResolvePath(rt.workDir, dir)
	}
	return resolveBeadsDir(rt.workDir, append(os.Environ(), rt.env...)).Path
}

// tickEnvValue mirrors the environment a tickRuntime child process receives:
// rt.env entries are appended after os.Environ() in rt.run, so the last
// rt.env match wins over the inherited process value.
func tickEnvValue(rt tickRuntime, key string) string {
	prefix := key + "="
	for i := len(rt.env) - 1; i >= 0; i-- {
		if strings.HasPrefix(rt.env[i], prefix) {
			return strings.TrimPrefix(rt.env[i], prefix)
		}
	}
	return os.Getenv(key)
}

// tickStagePath renders a ledger path for git add: repo-relative when the
// path sits under the work dir, absolute otherwise.
func tickStagePath(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return path
	}
	return rel
}

func tickGitRevParse(rt tickRuntime) string {
	out, code, err := rt.run("git", "rev-parse", "HEAD")
	if err != nil || code != 0 {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func tickGitRevParseInDir(rt tickRuntime, dir string) string {
	out, code, err := rt.run("git", "-C", dir, "rev-parse", "HEAD")
	if err != nil || code != 0 {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func tickGitCommitNothingToCommit(out []byte) bool {
	text := strings.ToLower(string(out))
	return strings.Contains(text, "nothing to commit") ||
		strings.Contains(text, "nothing added to commit") ||
		strings.Contains(text, "no changes added to commit")
}

func tickPublicStagePaths(rt tickRuntime, ledgerDir, evFirst string, paths []string) []string {
	stage := []string{}
	if tickPathExists(rt.workDir, evFirst) {
		stage = tickAppendPublicStagePath(stage, rt.workDir, ledgerDir, evFirst)
	}
	for _, path := range paths {
		stage = tickAppendPublicStagePath(stage, rt.workDir, ledgerDir, path)
	}
	return stage
}

func tickAppendPublicStagePath(stage []string, root, ledgerDir, path string) []string {
	if path == "" || tickIsPrivateLedgerPath(root, ledgerDir, path) {
		return stage
	}
	return append(stage, path)
}

func tickIsPrivateLedgerPath(root, ledgerDir, path string) bool {
	resolved := filepath.Clean(tickResolvePath(root, path))
	ledger := filepath.Clean(tickResolvePath(root, ledgerDir))
	if rel, err := filepath.Rel(ledger, resolved); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return true
	}
	clean := filepath.Clean(path)
	return clean == "_beads" || strings.HasPrefix(clean, "_beads"+string(filepath.Separator)) ||
		clean == ".beads" || strings.HasPrefix(clean, ".beads"+string(filepath.Separator))
}

func tickShortSHA(sha string) string {
	if len(sha) <= 7 {
		return sha
	}
	return sha[:7]
}

func tickLedgerShowsClosed(path, id string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer func() { _ = f.Close() }()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var item tickBead
		if json.Unmarshal(scanner.Bytes(), &item) == nil && item.ID == id && item.Status == "closed" {
			return true
		}
	}
	return false
}

func tickReadVerdict(rt tickRuntime, path string) (string, error) {
	var b []byte
	var err error
	if path == "-" {
		b, err = io.ReadAll(rt.stdin)
	} else {
		b, err = os.ReadFile(tickResolvePath(rt.workDir, path))
	}
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func tickRunVerdictGate(rt tickRuntime, path string) error {
	text, err := tickReadVerdict(rt, path)
	if err != nil {
		return err
	}
	if !tickVerdictHasCommandsRun(text) {
		fmt.Fprintln(rt.stderr, "REJECTED: verdict has no non-empty 'COMMANDS RUN' body - unverified; route to tie-break")
		return &tickExitError{code: tickExitUnverified}
	}
	if _, gaps := tickVerdictIdentity(text); len(gaps) > 0 {
		fmt.Fprintf(rt.stderr, "REJECTED: verdict identity unproven: %s\n", strings.Join(gaps, "; "))
		return &tickExitError{code: tickExitUnverified}
	}
	fmt.Fprintln(rt.stdout, "VERIFIED: verdict cites commands and independent judge identity")
	return nil
}

func tickVerdictHasCommandsRun(text string) bool {
	inBlock := false
	commandHeader := regexp.MustCompile(`(?i)commands[ _-]*run`)
	scanner := bufio.NewScanner(strings.NewReader(text))
	for scanner.Scan() {
		line := scanner.Text()
		lower := strings.ToLower(line)
		if commandHeader.MatchString(line) {
			inBlock = true
			if i := strings.Index(line, ":"); i >= 0 && strings.TrimSpace(line[i+1:]) != "" {
				return true
			}
			continue
		}
		if inBlock && regexp.MustCompile(`^\s*reasons`).MatchString(lower) {
			inBlock = false
		}
		if inBlock && strings.TrimSpace(line) != "" {
			return true
		}
	}
	return false
}

func tickCouncilGate(rt tickRuntime, paths []string) error {
	n := len(paths)
	pass, fail, unverified := 0, 0, 0
	families := map[string]bool{}
	judges := map[string]bool{}
	contexts := map[string]bool{}
	for _, path := range paths {
		text, err := tickReadVerdict(rt, path)
		identity, identityGaps := tickVerdictIdentity(text)
		// A verdict with any identity gap — now including a missing context_id or a
		// judge context equal to the author context (self-judge) — is unverified and
		// fails closed.
		if err != nil || !tickVerdictHasCommandsRun(text) || len(identityGaps) > 0 {
			unverified++
			continue
		}
		// The independence axis is the judge CONTEXT, not the judge name: a
		// duplicate context is one judge regardless of how it labels itself.
		// Canonicalize first so a single judge cannot forge "distinct" contexts via
		// whitespace/case/unicode variants.
		canonCtx := liveness.CanonicalizeContextID(identity.ContextID)
		if contexts[canonCtx] {
			fmt.Fprintf(rt.stderr, "FAIL-CLOSED: duplicate judge context %q does not count as an independent judge\n", identity.ContextID)
			return &tickExitError{code: tickExitCouncil}
		}
		contexts[canonCtx] = true
		// Retain the judge-name dedup as a secondary guard.
		if judges[identity.JudgeName] {
			fmt.Fprintf(rt.stderr, "FAIL-CLOSED: duplicate judge %q does not count as an independent judge\n", identity.JudgeName)
			return &tickExitError{code: tickExitCouncil}
		}
		judges[identity.JudgeName] = true
		p, f := tickVerdictTokenCounts(text)
		switch {
		case p == 1 && f == 0:
			pass++
			families[identity.JudgeModelFamily] = true
		case f == 1 && p == 0:
			fail++
		default:
			unverified++
		}
	}
	if unverified > 0 {
		fmt.Fprintf(rt.stderr, "FAIL-CLOSED: %d/%d verdict(s) unverified (no COMMANDS RUN / identity gap)\n", unverified, n)
		return &tickExitError{code: tickExitCouncil}
	}
	if pass == n {
		// Context floor: need >=2 distinct non-author judge contexts.
		if len(contexts) < 2 {
			fmt.Fprintf(rt.stderr, "FAIL-CLOSED: PASS quorum has %d distinct judge context; need at least 2 independent contexts\n", len(contexts))
			return &tickExitError{code: tickExitCouncil}
		}
		// Optional cross-family strengthener: only gated when explicitly required.
		if rt.requireCrossFamily && len(families) < 2 {
			fmt.Fprintf(rt.stderr, "FAIL-CLOSED: --require-cross-family set but PASS quorum spans %d model family; need at least 2 (cross-family)\n", len(families))
			return &tickExitError{code: tickExitCouncil}
		}
		fmt.Fprintf(rt.stdout, "COUNCIL PASS: %d/%d judges unanimous across %d distinct contexts (%d model families)\n", pass, n, len(contexts), len(families))
		return nil
	}
	if fail == n {
		fmt.Fprintf(rt.stderr, "FAIL-CLOSED: %d/%d judges FAIL\n", fail, n)
		return &tickExitError{code: tickExitCouncil}
	}
	fmt.Fprintf(rt.stderr, "DISAGREEMENT: %d PASS / %d FAIL - fail-closed; dispatch tie-break\n", pass, fail)
	return &tickExitError{code: tickExitDisagree}
}

type tickVerdictIdentityInfo struct {
	Author           string
	JudgeName        string
	JudgeProgram     string
	JudgeModelFamily string
	// ContextID is the judge's session-identity axis (the independence axis): two
	// distinct ContextIDs are two independent judges even on the same model.
	ContextID string
	// AuthorContextID is the author's context — a judge whose ContextID equals it
	// is self-judging and fails closed.
	AuthorContextID string
}

func tickVerdictIdentity(text string) (tickVerdictIdentityInfo, []string) {
	info := tickVerdictIdentityInfo{
		Author:           tickNormalizeIdentityValue(tickVerdictMetadataValue(text, "author", "author_id", "author-id", "author id", "author_name", "author-name", "author name")),
		JudgeName:        tickNormalizeIdentityValue(tickVerdictMetadataValue(text, "judge", "judge_name", "judge-name", "judge name", "judge_id", "judge-id", "judge id")),
		JudgeProgram:     tickNormalizeIdentityValue(tickVerdictMetadataValue(text, "judge_program", "judge-program", "judge program", "program", "validator_program", "validator-program", "validator program")),
		JudgeModelFamily: tickNormalizeModelFamily(tickVerdictMetadataValue(text, "judge_model_family", "judge-model-family", "judge model family", "model_family", "model-family", "model family", "family")),
		ContextID:        tickNormalizeIdentityValue(tickVerdictMetadataValue(text, "context_id", "context-id", "context id", "validator_session", "validator-session", "validator session")),
		AuthorContextID:  tickNormalizeIdentityValue(tickVerdictMetadataValue(text, "author_context_id", "author-context-id", "author context id")),
	}
	var gaps []string
	if info.Author == "" {
		gaps = append(gaps, "missing author")
	}
	if info.JudgeName == "" {
		gaps = append(gaps, "missing judge.name")
	}
	if info.JudgeProgram == "" {
		gaps = append(gaps, "missing judge.program")
	}
	if info.JudgeModelFamily == "" {
		gaps = append(gaps, "missing judge.model_family")
	} else if tickUnknownModelFamily(info.JudgeModelFamily) {
		gaps = append(gaps, "judge.model_family is unknown")
	}
	if info.ContextID == "" {
		gaps = append(gaps, "missing judge.context_id")
	}
	if info.Author != "" && info.JudgeName != "" && info.Author == info.JudgeName {
		gaps = append(gaps, "judge.name equals author")
	}
	if info.AuthorContextID != "" && info.ContextID != "" && info.AuthorContextID == info.ContextID {
		gaps = append(gaps, "judge.context_id equals author context")
	}
	if tickVerdictMetadataValue(text, "allow_self", "allow-self", "allow self", "self_waiver", "self-waiver", "self waiver") != "" {
		gaps = append(gaps, "self-judge waiver must be external and principal-logged, not verdict-authored")
	}
	return info, gaps
}

func tickVerdictMetadataValue(text string, keys ...string) string {
	wanted := map[string]bool{}
	for _, key := range keys {
		wanted[tickNormalizeMetadataKey(key)] = true
	}
	scanner := bufio.NewScanner(strings.NewReader(text))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		line = strings.TrimPrefix(line, "-")
		line = strings.TrimPrefix(strings.TrimSpace(line), "*")
		line = strings.TrimSpace(line)
		i := strings.Index(line, ":")
		if i < 0 {
			continue
		}
		key := tickNormalizeMetadataKey(line[:i])
		if !wanted[key] {
			continue
		}
		return strings.Trim(strings.TrimSpace(line[i+1:]), "`\"'")
	}
	return ""
}

func tickNormalizeMetadataKey(key string) string {
	key = strings.ToLower(strings.TrimSpace(key))
	key = strings.ReplaceAll(key, "-", "_")
	key = strings.ReplaceAll(key, " ", "_")
	return key
}

func tickNormalizeIdentityValue(value string) string {
	return strings.TrimSpace(value)
}

func tickNormalizeModelFamily(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func tickUnknownModelFamily(family string) bool {
	switch strings.ToLower(strings.TrimSpace(family)) {
	case "", "unknown", "none", "n/a", "na", "unset":
		return true
	default:
		return false
	}
}

func tickVerdictTokenCounts(text string) (pass, fail int) {
	passRE := regexp.MustCompile(`(?i)^\s*VERDICT:\s*PASS\b`)
	failRE := regexp.MustCompile(`(?i)^\s*VERDICT:\s*FAIL\b`)
	scanner := bufio.NewScanner(strings.NewReader(text))
	for scanner.Scan() {
		line := scanner.Text()
		if passRE.MatchString(line) {
			pass++
		}
		if failRE.MatchString(line) {
			fail++
		}
	}
	return pass, fail
}

func tickInstallGuards(rt tickRuntime) error {
	if out, code, err := rt.run("git", "config", "core.hooksPath", ".githooks"); err != nil || code != 0 {
		_, _ = rt.stderr.Write(out)
		return &tickExitError{code: code, msg: "git config core.hooksPath failed"}
	}
	for _, path := range []string{".githooks/pre-commit", "bin/validator-run", "bin/git-shim/git"} {
		full := filepath.Join(rt.workDir, path)
		if info, err := os.Stat(full); err == nil {
			_ = os.Chmod(full, info.Mode()|0o111)
		}
	}
	out, _, _ := rt.run("git", "config", "--get", "core.hooksPath")
	fmt.Fprintf(rt.stdout, "guards installed: core.hooksPath=%s\n", strings.TrimSpace(string(out)))
	return nil
}

func tickGuardStatus(rt tickRuntime) error {
	out, _, _ := rt.run("git", "config", "--get", "core.hooksPath")
	hooksPath := strings.TrimSpace(string(out))
	if hooksPath == "" {
		hooksPath = "(unset)"
	}
	fmt.Fprintf(rt.stdout, "core.hooksPath: %s\n", hooksPath)
	if hooksPath == ".githooks" && tickExecutable(filepath.Join(rt.workDir, ".githooks/pre-commit")) {
		fmt.Fprintln(rt.stdout, "pre-commit: installed + executable")
	} else {
		fmt.Fprintln(rt.stderr, "pre-commit: NOT active")
		return &tickExitError{code: tickExitUsage}
	}
	if tickExecutable(filepath.Join(rt.workDir, "bin/git-shim/git")) && tickExecutable(filepath.Join(rt.workDir, "bin/validator-run")) {
		fmt.Fprintln(rt.stdout, "validator shim + launcher: present + executable")
		return nil
	}
	fmt.Fprintln(rt.stderr, "validator shim/launcher: missing")
	return &tickExitError{code: tickExitUsage}
}

func tickExecutable(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir() && info.Mode()&0o111 != 0
}

func tickSmoke(rt tickRuntime) error {
	fails := 0
	passLine := func(label string) { fmt.Fprintf(rt.stdout, "PASS  %s\n", label) }
	failLine := func(label string) {
		fmt.Fprintf(rt.stderr, "FAIL  %s\n", label)
		fails++
	}

	if err := tickGuardStatus(tickRuntime{workDir: rt.workDir, stdout: io.Discard, stderr: io.Discard}); err == nil {
		passLine("1 guard-status active")
	} else {
		failLine("1 guard-status NOT active")
	}

	if !tickVerdictHasCommandsRun("COMMANDS RUN:\nREASONS:\nbecause\n") &&
		tickVerdictHasCommandsRun("COMMANDS RUN:\n  ao tick guard-status\nREASONS: ok\n") {
		passLine("2 verdict-gate rejects empty / accepts cited")
	} else {
		failLine("2 verdict-gate command body matrix failed")
	}

	tmp, err := os.MkdirTemp("", "ao-tick-smoke.")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(tmp) }()
	write := func(name, body string) string {
		path := filepath.Join(tmp, name)
		_ = os.WriteFile(path, []byte(body), 0o644)
		return path
	}
	pass1 := write("pass1.md", "author: codex\njudge: athena\njudge_program: claude-code\njudge_model_family: claude\nVERDICT: PASS\nCOMMANDS RUN:\n  ao tick guard-status\n")
	pass2 := write("pass2.md", "author: codex\njudge: windyelm\njudge_program: gemini-cli\njudge_model_family: gemini\nVERDICT: PASS\nCOMMANDS RUN:\n  ao tick verdict-gate -\n")
	fail1 := write("fail1.md", "author: codex\njudge: windyelm\njudge_program: gemini-cli\njudge_model_family: gemini\nVERDICT: FAIL\nCOMMANDS RUN:\n  ao tick guard-status\n")
	unver := write("unver.md", "VERDICT: PASS\nthis verdict cites no commands\n")
	contra := write("contra.md", "VERDICT: FAIL\nVERDICT: PASS\nCOMMANDS RUN:\n  ao tick guard-status\n")
	quietRT := rt
	quietRT.stdout = io.Discard
	quietRT.stderr = io.Discard
	if tickExitCode(tickCouncilGate(quietRT, []string{pass1, pass2})) == 0 &&
		tickExitCode(tickCouncilGate(quietRT, []string{pass1, fail1})) == tickExitDisagree &&
		tickExitCode(tickCouncilGate(quietRT, []string{pass1, unver})) == tickExitCouncil &&
		tickExitCode(tickCouncilGate(quietRT, []string{pass1, contra})) == tickExitCouncil {
		passLine("3 council-gate 2PASS/mixed/unverified/contradictory")
	} else {
		failLine("3 council-gate matrix failed")
	}
	if !tickVerdictHasCommandsRun("VERDICT: FAIL\n") {
		passLine("4 chaos bare-verdict rejected")
	} else {
		failLine("4 chaos bare-verdict accepted")
	}
	if tickSmokeCloseFailure(rt, tmp) {
		passLine("5 close aborts before git commit when br close fails")
	} else {
		failLine("5 close failure path committed")
	}
	if tickNoClaudePrintOnScriptSurfaces(rt) {
		passLine("6 no claude in -p/--print mode on tracked script surfaces")
	} else {
		failLine("6 found claude in -p/--print mode on a tracked script surface")
	}
	if _, code, err := rt.run("br", "ready"); err == nil && code == 0 {
		if _, code, err := rt.run("git", "rev-parse", "HEAD"); err == nil && code == 0 {
			passLine("7 br ready + git rev-parse HEAD resolve")
		} else {
			failLine("7 git rev-parse HEAD failed")
		}
	} else {
		failLine("7 br ready failed")
	}
	if fails == 0 {
		fmt.Fprintln(rt.stdout, "SMOKE PASS")
		return nil
	}
	fmt.Fprintf(rt.stderr, "SMOKE FAIL (%d check(s) failed)\n", fails)
	return &tickExitError{code: tickExitUsage}
}

func tickExitCode(err error) int {
	if err == nil {
		return 0
	}
	if e, ok := err.(*tickExitError); ok {
		return e.ExitCode()
	}
	return 1
}

func tickSmokeCloseFailure(rt tickRuntime, tmp string) bool {
	fakebin := filepath.Join(tmp, "fakebin")
	_ = os.MkdirAll(fakebin, 0o755)
	fakeBR := `#!/usr/bin/env bash
if [ "${1:-}" = "close" ]; then echo "stubbed br close failure" >&2; exit 42; fi
if [ "${1:-}" = "sync" ] || [ "${1:-}" = "update" ]; then exit 0; fi
echo "unexpected br call: $*" >&2; exit 43
`
	fakeGitLog := filepath.Join(tmp, "fake-git.log")
	fakeGit := `#!/usr/bin/env bash
case "${1:-}" in
  rev-parse) echo fakehead ;;
  add) : ;;
  commit) printf 'commit\n' >> "${TICK_SMOKE_FAKE_GIT_LOG:?}" ;;
  *) : ;;
esac
`
	_ = os.WriteFile(filepath.Join(fakebin, "br"), []byte(fakeBR), 0o755)
	_ = os.WriteFile(filepath.Join(fakebin, "git"), []byte(fakeGit), 0o755)
	evidence := filepath.Join(tmp, "close-evidence.md")
	_ = os.WriteFile(evidence, []byte("evidence"), 0o644)
	smokeRT := rt
	smokeRT.stdout = io.Discard
	smokeRT.stderr = io.Discard
	smokeRT.env = []string{
		"PATH=" + fakebin + string(os.PathListSeparator) + os.Getenv("PATH"),
		"TICK_SMOKE_FAKE_GIT_LOG=" + fakeGitLog,
	}
	code := tickExitCode(tickClose(smokeRT, "cp-smoke", "smoke close should not commit", evidence, nil))
	info, err := os.Stat(fakeGitLog)
	return code != 0 && (os.IsNotExist(err) || (err == nil && info.Size() == 0))
}

func tickNoClaudePrintOnScriptSurfaces(rt tickRuntime) bool {
	out, code, err := rt.run("git", "ls-files", "--", "*.sh", ".githooks/*", "bin/*")
	if err != nil || code != 0 {
		return true
	}
	invocation := regexp.MustCompile(`(^|[;&|()]|&&|\|\||\$\()[[:space:]]*claude[[:space:]]+(-p\b|--print\b)`)
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		path := strings.TrimSpace(scanner.Text())
		if path == "" {
			continue
		}
		body, err := os.ReadFile(filepath.Join(rt.workDir, path))
		if err == nil && invocation.Match(body) {
			return false
		}
	}
	return true
}
