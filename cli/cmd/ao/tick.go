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
	if len(rt.env) > 0 {
		c.Env = append(os.Environ(), rt.env...)
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
	Short: "Reject verdicts without a non-empty COMMANDS RUN body",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rt := newTickRuntime(cmd)
		text, err := tickReadVerdict(rt, args[0])
		if err != nil {
			return err
		}
		if tickVerdictHasCommandsRun(text) {
			fmt.Fprintln(rt.stdout, "VERIFIED: verdict cites the commands it ran")
			return nil
		}
		fmt.Fprintln(rt.stderr, "REJECTED: verdict has no non-empty 'COMMANDS RUN' body - unverified; route to tie-break")
		return &tickExitError{code: tickExitUnverified}
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

func init() {
	tickCmd.GroupID = "workflow"
	rootCmd.AddCommand(tickCmd)
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
	ID     string `json:"id"`
	Status string `json:"status"`
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
	var list []tickBead
	if err := json.Unmarshal(out, &list); err == nil && len(list) > 0 {
		return list[0].ID
	}
	var wrapped struct {
		Issues []tickBead `json:"issues"`
	}
	if err := json.Unmarshal(out, &wrapped); err == nil && len(wrapped.Issues) > 0 {
		return wrapped.Issues[0].ID
	}
	return ""
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
	if evFirst == "" || (!tickPathExists(rt.workDir, evFirst) && !strings.Contains(evidence, "evidence/")) {
		fmt.Fprintf(rt.stderr, "REFUSED close %s: evidence ref %q resolves to no real artifact\n", id, evidence)
		return &tickExitError{code: tickExitCloseRef}
	}

	before := tickGitRevParse(rt)
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
	if !tickLedgerShowsClosed(filepath.Join(rt.workDir, ".beads", "issues.jsonl"), id) {
		_, _, _ = rt.run("br", "update", id, "--status", "open")
		_, _, _ = rt.run("br", "sync", "--flush-only")
		fmt.Fprintf(rt.stderr, "FAILED close %s: ledger does not show a closed bead after br close; bead reopened\n", id)
		return &tickExitError{code: tickExitCloseFail}
	}

	stage := []string{"add", "--", ".beads/issues.jsonl"}
	if tickPathExists(rt.workDir, ".beads/metadata.json") {
		stage = append(stage, ".beads/metadata.json")
	}
	if tickPathExists(rt.workDir, evFirst) {
		stage = append(stage, evFirst)
	}
	stage = append(stage, paths...)
	if _, code, err := rt.run("git", stage...); err != nil || code != 0 {
		return &tickExitError{code: code, msg: "git add failed"}
	}
	if _, code, err := rt.run("git", "commit", "-q", "-m", msg); err != nil || code != 0 {
		return &tickExitError{code: code, msg: "git commit failed"}
	}
	after := tickGitRevParse(rt)
	if after == "" {
		after = "none"
	}
	if before == after {
		_, _, _ = rt.run("br", "update", id, "--status", "open")
		fmt.Fprintf(rt.stderr, "FAILED close %s: git commit did not land - ledger would lie; bead reopened\n", id)
		return &tickExitError{code: tickExitNoCommit}
	}
	fmt.Fprintf(rt.stdout, "closed %s @ %s\n", id, tickShortSHA(after))
	return nil
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

func tickGitRevParse(rt tickRuntime) string {
	out, code, err := rt.run("git", "rev-parse", "HEAD")
	if err != nil || code != 0 {
		return ""
	}
	return strings.TrimSpace(string(out))
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
	defer f.Close()
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
	for _, path := range paths {
		text, err := tickReadVerdict(rt, path)
		if err != nil || !tickVerdictHasCommandsRun(text) {
			unverified++
			continue
		}
		p, f := tickVerdictTokenCounts(text)
		switch {
		case p == 1 && f == 0:
			pass++
		case f == 1 && p == 0:
			fail++
		default:
			unverified++
		}
	}
	if unverified > 0 {
		fmt.Fprintf(rt.stderr, "FAIL-CLOSED: %d/%d verdict(s) unverified (no COMMANDS RUN)\n", unverified, n)
		return &tickExitError{code: tickExitCouncil}
	}
	if pass == n {
		fmt.Fprintf(rt.stdout, "COUNCIL PASS: %d/%d judges unanimous\n", pass, n)
		return nil
	}
	if fail == n {
		fmt.Fprintf(rt.stderr, "FAIL-CLOSED: %d/%d judges FAIL\n", fail, n)
		return &tickExitError{code: tickExitCouncil}
	}
	fmt.Fprintf(rt.stderr, "DISAGREEMENT: %d PASS / %d FAIL - fail-closed; dispatch tie-break\n", pass, fail)
	return &tickExitError{code: tickExitDisagree}
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
	defer os.RemoveAll(tmp)
	write := func(name, body string) string {
		path := filepath.Join(tmp, name)
		_ = os.WriteFile(path, []byte(body), 0o644)
		return path
	}
	pass1 := write("pass1.md", "VERDICT: PASS\nCOMMANDS RUN:\n  ao tick guard-status\n")
	pass2 := write("pass2.md", "VERDICT: PASS\nCOMMANDS RUN:\n  ao tick verdict-gate -\n")
	fail1 := write("fail1.md", "VERDICT: FAIL\nCOMMANDS RUN:\n  ao tick guard-status\n")
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
