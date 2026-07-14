// practices: [design-by-contract, continuous-delivery]
package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/boshu2/agentops/cli/internal/trackerexec"
	"github.com/boshu2/agentops/cli/internal/trackerresolve"
	verdictparse "github.com/boshu2/agentops/cli/internal/verdict"
	"github.com/spf13/cobra"
)

const (
	tickExitUsage      = 1
	tickExitUnverified = 5
	tickExitCouncil    = 6
	tickExitDisagree   = 8
)

type tickExitError struct {
	code int
	msg  string
}

func (e *tickExitError) Error() string { return e.msg }
func (e *tickExitError) ExitCode() int { return e.code }

type tickRuntime struct {
	workDir string
	ctx     context.Context
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
		ctx:     cmd.Context(),
		stdin:   cmd.InOrStdin(),
		stdout:  cmd.OutOrStdout(),
		stderr:  cmd.ErrOrStderr(),
	}
}

func (rt tickRuntime) run(name string, args ...string) ([]byte, int, error) {
	c := exec.Command(name, args...)
	c.Dir = rt.workDir
	env := append(os.Environ(), rt.env...)
	// resolveTracker returns an absolute binary when BR is installed. Match the
	// executable basename as well as the bare command so resolved BR calls keep
	// the same canonical-ledger environment as direct `br` calls, especially in
	// linked worktrees where $PWD/_beads is intentionally absent.
	if name == trackerBR || filepath.Base(name) == trackerBR {
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

func (rt tickRuntime) runTracker(args ...string) ([]byte, int, error) {
	resolution, err := resolveTracker(rt.workDir, append(os.Environ(), rt.env...))
	if err != nil {
		return nil, 127, err
	}
	return rt.runResolvedTracker(resolution, args...)
}

func (rt tickRuntime) runResolvedTracker(resolution trackerresolve.Resolution, args ...string) ([]byte, int, error) {
	command := (trackerexec.Factory{}).Command(rt.ctx, resolution, args, trackerexec.Streams{Stdin: rt.stdin})
	out, err := command.CombinedOutput()
	if err == nil {
		return out, 0, nil
	}
	if exitErr, ok := err.(*trackerexec.ExitError); ok {
		return out, exitErr.ExitCode(), err
	}
	return out, 127, err
}

var verdictGateCmd = &cobra.Command{
	Use:   "verdict-gate <file|->",
	Short: "Reject verdicts without commands and independent judge identity",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return tickRunVerdictGate(newTickRuntime(cmd), args[0])
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
	Use:    "chaos-test",
	Short:  "Run a read-only smoke test of the tick membrane",
	Args:   cobra.NoArgs,
	Hidden: true, // canonical spelling is `ao eval chaos`; kept for back-compat
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

func init() {
	// The `tick` command + its subcommands are archived behind //go:build legacy
	// (tick_cmd_legacy.go). These sibling top-level commands are spine — they use
	// the untagged tick engine below and ship in the default binary.
	rootCmd.AddCommand(
		readyCmd,
		verdictGateCmd,
		installGuardsCmd,
		guardStatusCmd,
		chaosTestCmd,
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

func tickTrackerPassthrough(rt tickRuntime, args ...string) error {
	out, code, err := rt.runTracker(args...)
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
	resolution, err := resolveTracker(rt.workDir, append(os.Environ(), rt.env...))
	if err != nil {
		return err
	}
	readyOut, code, err := rt.runResolvedTracker(resolution, "ready", "--json")
	if err != nil || code != 0 {
		return &tickExitError{code: code, msg: resolution.Tracker + " ready --json failed"}
	}
	ready := tickParseBeads(readyOut)

	allOut, code, err := rt.runResolvedTracker(resolution, "list", "--all", "--json")
	if err != nil || code != 0 {
		return &tickExitError{code: code, msg: resolution.Tracker + " list --all --json failed"}
	}
	all := tickParseBeads(allOut)
	state := tickReadyState{
		StateSource: resolution.Tracker,
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
	if out, code, err := rt.runTracker("ready", "--json"); err == nil && code == 0 {
		if id := tickFirstReadyFromJSON(out); id != "" {
			return id
		}
	}
	out, _, _ := rt.runTracker("ready")
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
	var single tickBead
	if err := json.Unmarshal(out, &single); err == nil && single.ID != "" {
		return []tickBead{single}
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
	out, _, _ := rt.runTracker("ready")
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
	out, code, err := rt.runTracker("list", "--all", "--json")
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
	if !verdictparse.HasCommandsRun(text) {
		fmt.Fprintln(rt.stderr, "REJECTED: verdict has no non-empty 'COMMANDS RUN' body - unverified; route to tie-break")
		return &tickExitError{code: tickExitUnverified}
	}
	if _, gaps := verdictparse.Identity(text); len(gaps) > 0 {
		fmt.Fprintf(rt.stderr, "REJECTED: verdict identity unproven: %s\n", strings.Join(gaps, "; "))
		return &tickExitError{code: tickExitUnverified}
	}
	fmt.Fprintln(rt.stdout, "VERIFIED: verdict cites commands and independent judge identity")
	return nil
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

	hooksOut, hooksCode, hooksErr := rt.run("git", "config", "--get", "core.hooksPath")
	hooksPath := strings.TrimSpace(string(hooksOut))
	switch {
	case hooksPath != ".githooks" && (hooksErr == nil || hooksCode == 1):
		passLine("1 hookless default")
	case hooksErr == nil && hooksCode == 0 && tickExitCode(tickGuardStatus(tickRuntime{workDir: rt.workDir, stdout: io.Discard, stderr: io.Discard})) == 0:
		passLine("1 guard-status active")
	default:
		failLine("1 guard posture invalid")
	}

	if !verdictparse.HasCommandsRun("COMMANDS RUN:\nREASONS:\nbecause\n") &&
		verdictparse.HasCommandsRun("COMMANDS RUN:\n  ao tick guard-status\nREASONS: ok\n") {
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
	pass1 := write("pass1.md", "author: codex\njudge: athena\njudge_program: claude-code\njudge_model_family: claude\ncontext_id: chaos-athena\nVERDICT: PASS\nCOMMANDS RUN:\n  ao tick guard-status\n")
	pass2 := write("pass2.md", "author: codex\njudge: windyelm\njudge_program: gemini-cli\njudge_model_family: gemini\ncontext_id: chaos-windyelm\nVERDICT: PASS\nCOMMANDS RUN:\n  ao tick verdict-gate -\n")
	fail1 := write("fail1.md", "author: codex\njudge: windyelm\njudge_program: gemini-cli\njudge_model_family: gemini\ncontext_id: chaos-windyelm-fail\nVERDICT: FAIL\nCOMMANDS RUN:\n  ao tick guard-status\n")
	unver := write("unver.md", "VERDICT: PASS\nthis verdict cites no commands\n")
	contra := write("contra.md", "VERDICT: FAIL\nVERDICT: PASS\nCOMMANDS RUN:\n  ao tick guard-status\n")
	quietRT := rt
	quietRT.stdout = io.Discard
	quietRT.stderr = io.Discard
	if tickExitCode(runCouncilGate(quietRT, []string{pass1, pass2})) == 0 &&
		tickExitCode(runCouncilGate(quietRT, []string{pass1, fail1})) == tickExitDisagree &&
		tickExitCode(runCouncilGate(quietRT, []string{pass1, unver})) == tickExitCouncil &&
		tickExitCode(runCouncilGate(quietRT, []string{pass1, contra})) == tickExitCouncil {
		passLine("3 council-gate 2PASS/mixed/unverified/contradictory")
	} else {
		failLine("3 council-gate matrix failed")
	}
	if !verdictparse.HasCommandsRun("VERDICT: FAIL\n") {
		passLine("4 chaos bare-verdict rejected")
	} else {
		failLine("4 chaos bare-verdict accepted")
	}
	if runCloseSmokeFailure(rt, tmp) {
		passLine("5 close aborts before git commit when br close fails")
	} else {
		failLine("5 close failure path committed")
	}
	if tickNoClaudePrintOnScriptSurfaces(rt) {
		passLine("6 no claude in -p/--print mode on tracked script surfaces")
	} else {
		failLine("6 found claude in -p/--print mode on a tracked script surface")
	}
	if _, code, err := rt.runTracker("ready"); err == nil && code == 0 {
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
	if e, ok := err.(interface{ ExitCode() int }); ok {
		return e.ExitCode()
	}
	return 1
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
