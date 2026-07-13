// `ao session bootstrap` — the universal init prompt (soc-vuu6.25).
//
// Every agent spawned into an AgentOps repo runs this command first. The
// output is the same shape regardless of model — that's the fungibility
// guarantee: no two agents in a swarm start with different orientation.
//
// Substeps (all fail-open):
//   1. Confirm AGENTS.md and its canonical on-demand workflow, CI, Codex, and
//      runtime destinations exist. Read by the agent itself when triggered.
//   2. Run `ao onboard --auto` if it exists (soc-vuu6.9 — currently a P3
//      stub). Falls back to phase="skipped:not-implemented" if absent.
//   3. Resolve the live br ledger directory and call `br ready --json` with
//      BEADS_DIR wired. Falls back to ready_beads_count=null if br is missing
//      or errors.
//   4. mcp-agent-mail register/check — fully optional. Phase reports
//      `present` / `absent`; never fails the bootstrap.
//
// The output validates against schemas/session-bootstrap.v1.schema.json.
// Default human output is a 1-line summary plus a path-to-orientation hint;
// `--json` emits the structured shape for headless callers (SessionStart
// hooks, evolve runs, swarm spawn scripts).
//
// practices: [agile-manifesto, dora-metrics, fungibility-charter]

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/boshu2/agentops/cli/internal/promptsafe"
	"github.com/boshu2/agentops/cli/internal/search"
	"github.com/boshu2/agentops/cli/internal/types"

	"github.com/spf13/cobra"
)

var (
	sessionBootstrapJSON   bool
	sessionBootstrapNoMail bool
	sessionBootstrapRobot  bool
)

var sessionBootstrapCmd = &cobra.Command{
	Use:   "bootstrap",
	Short: "Universal init prompt — every agent runs this first",
	Long: `Universal init prompt for any agent spawned into an AgentOps repo.

Reports orientation status (AGENTS.md and canonical route presence), invokes
optional follow-on commands (ao onboard, mcp-agent-mail), counts ready
work, and emits a machine-readable summary so swarm coordination layers
can rely on identical agent starting frames regardless of model.

The bootstrap is intentionally fail-open: missing subcommands or
optional integrations are reported but never abort the run.

Flags:
  --json       Emit the full status object as JSON (default: 1-line summary).
  --no-mail    Skip the mcp-agent-mail probe even if the MCP server is reachable.
  --robot      Same as --json but tighter exit-code contract for hooks.

Examples:
  ao session bootstrap            # human summary
  ao session bootstrap --json     # full JSON status
  ao session bootstrap --robot    # for SessionStart hooks`,
	Args: cobra.NoArgs,
	RunE: runSessionBootstrap,
}

func init() {
	sessionCmd.AddCommand(sessionBootstrapCmd)
	sessionBootstrapCmd.Flags().BoolVar(&sessionBootstrapJSON, "json", false,
		"Emit machine-readable status as JSON")
	sessionBootstrapCmd.Flags().BoolVar(&sessionBootstrapNoMail, "no-mail", false,
		"Skip the mcp-agent-mail probe")
	sessionBootstrapCmd.Flags().BoolVar(&sessionBootstrapRobot, "robot", false,
		"Robot mode: JSON output with tight exit-code contract for SessionStart hooks")
}

// SessionBootstrapStatus is the canonical bootstrap report shape. Field
// names and types must match schemas/session-bootstrap.v1.schema.json.
type SessionBootstrapStatus struct {
	AgentsMDRead                bool                         `json:"agents_md_read"`
	AgentsSiblingsRead          []string                     `json:"agents_siblings_read"`
	CanonicalRoutesPresent      []string                     `json:"canonical_routes_present"`
	OnboardPhase                string                       `json:"onboard_phase"`
	BeadsDir                    string                       `json:"beads_dir"`
	BeadsDirSource              string                       `json:"beads_dir_source"`
	ReadyBeadsCount             *int                         `json:"ready_beads_count"`
	MailUnreadCount             *int                         `json:"mail_unread_count"`
	Runtime                     string                       `json:"runtime"`
	StartedAt                   string                       `json:"started_at"`
	BootstrapVersion            string                       `json:"bootstrap_version"`
	BootstrapMemory             []SessionBootstrapMemoryItem `json:"bootstrap_memory"`
	BootstrapMemoryBudgetTokens int                          `json:"bootstrap_memory_budget_tokens"`
	BootstrapMemoryUsedTokens   int                          `json:"bootstrap_memory_used_tokens"`
	BootstrapMemoryOmittedCount int                          `json:"bootstrap_memory_omitted_count"`
	BootstrapMemoryOverBudget   bool                         `json:"bootstrap_memory_over_budget"`
	GateHook                    string                       `json:"gate_hook"`
}

// SessionBootstrapMemoryItem is a compact canon-gated T2 memory entry surfaced
// at bootstrap.
type SessionBootstrapMemoryItem struct {
	ID       string  `json:"id"`
	Title    string  `json:"title"`
	Content  string  `json:"content"`
	Source   string  `json:"source"`
	Maturity string  `json:"maturity"`
	Reach    string  `json:"reach"`
	Score    float64 `json:"score"`
	Tokens   int     `json:"tokens"`
}

// canonicalAgentRoutes are the on-demand owners reached from the root contract.
var canonicalAgentRoutes = []string{
	"docs/agent-workflow-reference.md",
	"docs/CI-CD.md",
	"docs/contracts/codex-skill-api.md",
	"docs/contracts/repo-execution-profile.md",
}

const sessionBootstrapMemoryTokenBudget = 1200

func runSessionBootstrap(cmd *cobra.Command, _ []string) error {
	robot := sessionBootstrapRobot || sessionBootstrapJSON
	cwd, err := os.Getwd()
	if err != nil {
		cwd = os.Getenv("PWD")
	}
	status := computeBootstrapStatus(cmd.Context(), cwd, sessionBootstrapNoMail)
	if err := writeBootstrapMemoryWarning(cmd.ErrOrStderr(), status); err != nil {
		return err
	}

	if robot {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(status)
	}

	return printBootstrapSummary(cmd, status)
}

// computeBootstrapStatus performs the four substeps and returns a populated
// status. Errors are absorbed into nil/skipped-marker fields — the function
// never errors out.
func computeBootstrapStatus(ctx context.Context, cwd string, noMail bool) SessionBootstrapStatus {
	if ctx == nil {
		ctx = context.Background()
	}
	if cwd == "" {
		if pwd, err := os.Getwd(); err == nil {
			cwd = pwd
		}
	}

	status := SessionBootstrapStatus{
		AgentsSiblingsRead:          []string{}, // v1 compatibility: topology retired, always empty.
		CanonicalRoutesPresent:      []string{},
		Runtime:                     detectRuntime(),
		StartedAt:                   time.Now().UTC().Format(time.RFC3339),
		BootstrapVersion:            "v1",
		OnboardPhase:                sessionBootstrapOnboard(ctx, cwd),
		BootstrapMemory:             []SessionBootstrapMemoryItem{},
		BootstrapMemoryBudgetTokens: sessionBootstrapMemoryTokenBudget,
	}
	beadsDir := resolveBeadsDir(cwd, os.Environ())
	status.BeadsDir = beadsDir.Path
	status.BeadsDirSource = beadsDir.Source

	status.AgentsMDRead = fileExists(filepath.Join(cwd, "AGENTS.md"))
	for _, route := range canonicalAgentRoutes {
		if fileExists(filepath.Join(cwd, route)) {
			status.CanonicalRoutesPresent = append(status.CanonicalRoutesPresent, route)
		}
	}

	if n, ok := sessionBootstrapMakeReady(ctx, cwd); ok {
		status.ReadyBeadsCount = &n
	}

	if !noMail {
		if n, ok := sessionBootstrapMailUnread(ctx); ok {
			status.MailUnreadCount = &n
		}
	}

	memory, used, omitted := sessionBootstrapCanonMemory(cwd, sessionBootstrapMemoryTokenBudget, nowFunc())
	status.BootstrapMemory = memory
	status.BootstrapMemoryUsedTokens = used
	status.BootstrapMemoryOmittedCount = omitted
	status.BootstrapMemoryOverBudget = omitted > 0

	status.GateHook = sessionBootstrapGateHookStatus(cwd)

	return status
}

// sessionBootstrapGateHookStatus DETECTS whether the pre-push gate (the
// push-as-CI release authority, wired into the untracked .git/hooks/ by
// scripts/install-pre-push-gate.sh) is armed, so a fresh clone can't be
// silently gate-less. It is DETECT-ONLY and never executes anything: auto-
// running a working-tree script from bootstrap was refuted by cross-family
// review as an arbitrary-code-execution vector — any tree-based "is this the
// agentops repo" identity (go.mod, markers) is spoofable by a hostile checkout
// the agent might `cd` into. Arming is done by the explicit setup/install step;
// bootstrap's job is to make absence LOUD (the warn in printBootstrapSummary).
// Returns "active" (gate wired) | "inactive" (agentops repo, gate missing) |
// "unavailable" (not the repo, or git absent). (age-push-equals-ci-0ua.3)
func sessionBootstrapGateHookStatus(cwd string) string {
	if cwd == "" {
		return "unavailable"
	}
	root := strings.TrimSpace(sessionBootstrapGitOut(cwd, "rev-parse", "--show-toplevel"))
	if root == "" {
		return "unavailable"
	}
	// Only ASSESS in the agentops repo (no spurious "inactive" warnings
	// elsewhere). Spoofing this is harmless — nothing is ever executed.
	if !sessionBootstrapIsAgentopsRepo(root) {
		return "unavailable"
	}
	hookPath := strings.TrimSpace(sessionBootstrapGitOut(cwd, "rev-parse", "--git-path", "hooks/pre-push"))
	if hookPath != "" && !filepath.IsAbs(hookPath) {
		hookPath = filepath.Join(root, hookPath)
	}
	// The installer writes this specific marker only when it wires the gate.
	if hookPath != "" && fileContainsSubstring(hookPath, "AGENTOPS PRE-PUSH GATE") {
		return "active"
	}
	return "inactive"
}

// sessionBootstrapIsAgentopsRepo confirms root is THE agentops checkout, used
// only to scope the gate-hook ASSESSMENT (never to gate execution — nothing is
// executed). Spoofable by a hostile tree, but harmless here by construction.
func sessionBootstrapIsAgentopsRepo(root string) bool {
	data, ok := readRegularFileCapped(filepath.Join(root, "cli", "go.mod"), 1<<16)
	return ok && strings.Contains(string(data), "module github.com/boshu2/agentops/cli")
}

// sessionBootstrapGitOut runs `git -C dir <args...>` and returns stdout (empty on error).
func sessionBootstrapGitOut(dir string, args ...string) string {
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).Output()
	if err != nil {
		return ""
	}
	return string(out)
}

// readRegularFileCapped reads up to maxBytes of a REGULAR file. Non-regular
// paths (FIFO/device/socket/symlink/dir) are skipped WITHOUT opening, so a
// hostile repo cannot hang bootstrap by planting a FIFO at a path we read
// (cross-family DoS refute); the byte cap bounds a giant-file read.
func readRegularFileCapped(path string, maxBytes int64) ([]byte, bool) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return nil, false
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, false
	}
	// Read-only handle: a Close error cannot lose data, so it is safe to drop.
	defer func() { _ = f.Close() }()
	data, err := io.ReadAll(io.LimitReader(f, maxBytes))
	if err != nil {
		return nil, false
	}
	return data, true
}

// fileContainsSubstring reports whether the regular file at path contains substr.
func fileContainsSubstring(path, substr string) bool {
	data, ok := readRegularFileCapped(path, 1<<20)
	return ok && strings.Contains(string(data), substr)
}

type sessionBootstrapMemoryCandidate struct {
	item  SessionBootstrapMemoryItem
	score float64
}

func sessionBootstrapCanonMemory(cwd string, tokenBudget int, now time.Time) ([]SessionBootstrapMemoryItem, int, int) {
	if tokenBudget <= 0 || cwd == "" {
		return []SessionBootstrapMemoryItem{}, 0, 0
	}
	canonPath := filepath.Join(cwd, canonLearningsDir)
	if !dirExists(canonPath) {
		return []SessionBootstrapMemoryItem{}, 0, 0
	}

	candidates := make([]sessionBootstrapMemoryCandidate, 0)
	for _, file := range globLearningFiles(canonPath) {
		l, err := parseLearningFile(file)
		if err != nil || l.Superseded {
			continue
		}
		meta := sessionBootstrapLearningMetadata(file)
		if !sessionBootstrapMemoryEligible(l, meta) {
			continue
		}
		applyFreshnessScore(&l, file, now)
		if l.Utility <= 0 {
			l.Utility = types.InitialUtility
		}
		// Title + body are attacker-influenceable corpus content interpolated
		// into agent-readable bootstrap output (human summary AND --json); strip
		// harness delimiter tags at this single choke point so both paths are
		// covered. (age-gascity-port-slate-irye.1)
		title := promptsafe.SanitizeLeaf(l.Title)
		content := promptsafe.SanitizeLeaf(sessionBootstrapMemoryContent(l))
		tokens := max(1, estimateTokens(strings.Join([]string{title, content}, "\n")))
		score := l.Utility * meta.confidence * maxFloat(l.FreshnessScore, 0.1)
		item := SessionBootstrapMemoryItem{
			ID:       l.ID,
			Title:    title,
			Content:  content,
			Source:   l.Source,
			Maturity: l.Maturity,
			Reach:    search.ComputeReach(l.Reach, l.Maturity, true),
			Score:    score,
			Tokens:   tokens,
		}
		candidates = append(candidates, sessionBootstrapMemoryCandidate{item: item, score: score})
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].score == candidates[j].score {
			return candidates[i].item.ID < candidates[j].item.ID
		}
		return candidates[i].score > candidates[j].score
	})

	selected := make([]SessionBootstrapMemoryItem, 0, len(candidates))
	used := 0
	omitted := 0
	for _, candidate := range candidates {
		if used+candidate.item.Tokens > tokenBudget {
			omitted++
			continue
		}
		selected = append(selected, candidate.item)
		used += candidate.item.Tokens
	}
	return selected, used, omitted
}

type sessionBootstrapMemoryMetadata struct {
	confidence float64
	severity   string
}

func sessionBootstrapLearningMetadata(file string) sessionBootstrapMemoryMetadata {
	meta := sessionBootstrapMemoryMetadata{confidence: 0.5}
	data, err := os.ReadFile(file)
	if err != nil {
		return meta
	}
	content := string(data)
	if strings.HasSuffix(file, ".jsonl") {
		first := strings.SplitN(content, "\n", 2)[0]
		var payload map[string]any
		if err := json.Unmarshal([]byte(first), &payload); err == nil {
			if confidence, ok := payload["confidence"].(float64); ok && confidence > 0 {
				meta.confidence = confidence
			}
			if severity, ok := payload["severity"].(string); ok {
				meta.severity = strings.ToLower(strings.TrimSpace(severity))
			}
		}
		return meta
	}
	fields := parseFrontmatterFromContent(content, "confidence", "severity")
	if confidence, err := strconv.ParseFloat(strings.TrimSpace(fields["confidence"]), 64); err == nil && confidence > 0 {
		meta.confidence = confidence
	}
	meta.severity = strings.ToLower(strings.TrimSpace(fields["severity"]))
	return meta
}

func sessionBootstrapMemoryEligible(l learning, meta sessionBootstrapMemoryMetadata) bool {
	switch types.Maturity(strings.ToLower(strings.TrimSpace(l.Maturity))) {
	case types.MaturityEstablished:
		return true
	case types.MaturityAntiPattern:
		return meta.severity == "high" || meta.severity == "critical"
	default:
		return false
	}
}

func sessionBootstrapMemoryContent(l learning) string {
	content := strings.TrimSpace(l.BodyText)
	if content == "" {
		content = strings.TrimSpace(l.Summary)
	}
	return content
}

func writeBootstrapMemoryWarning(w io.Writer, s SessionBootstrapStatus) error {
	if !s.BootstrapMemoryOverBudget {
		return nil
	}
	_, err := fmt.Fprintf(w, "warn: bootstrap memory canon set exceeded %d tokens; injected %d tokens and omitted %d lower-ranked item(s)\n",
		s.BootstrapMemoryBudgetTokens, s.BootstrapMemoryUsedTokens, s.BootstrapMemoryOmittedCount)
	return err
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func detectRuntime() string {
	if v := os.Getenv("AGENTOPS_RPI_RUNTIME"); v != "" {
		return v
	}
	if os.Getenv("CLAUDE_CODE_SESSION_ID") != "" || os.Getenv("CLAUDECODE") != "" {
		return "claude-code"
	}
	if os.Getenv("CODEX_CLI") != "" {
		return "codex"
	}
	return runtime.GOOS
}

// sessionBootstrapOnboard returns the onboard phase. Falls back to
// "skipped:not-implemented" when `ao onboard` is not built (soc-vuu6.9 stub).
func sessionBootstrapOnboard(ctx context.Context, cwd string) string {
	// Check if `ao onboard` exists as a subcommand of this binary. We can detect
	// it by introspecting the root command tree. If absent, return the
	// skipped-with-reason marker so consumers can distinguish "ran and yielded
	// no work" from "command does not exist yet."
	for _, c := range rootCmd.Commands() {
		if c.Name() == "onboard" {
			return runOnboardSubprocess(ctx, cwd)
		}
	}
	return "skipped:not-implemented"
}

// runOnboardSubprocess executes `ao onboard --auto` via a child process when
// available. Returns the phase reported by onboard, or a skipped marker on
// error. Kept tiny so a swallowed error never crashes bootstrap.
func runOnboardSubprocess(ctx context.Context, cwd string) string {
	cmd := exec.CommandContext(ctx, "ao", "onboard", "--auto", "--json")
	cmd.Dir = cwd
	out, err := cmd.Output()
	if err != nil {
		return "skipped:error"
	}
	var payload struct {
		Phase string `json:"phase"`
	}
	if err := json.Unmarshal(out, &payload); err != nil || payload.Phase == "" {
		return "ran:no-phase"
	}
	return payload.Phase
}

// sessionBootstrapReadyBeads queries ready beads via `br ready --json` and
// returns the count of returned items. Returns (0, false) when br is missing
// or errors.
func sessionBootstrapReadyBeads(_ context.Context, cwd string) (int, bool) {
	readyOut := runBeadsTracker(cwd, 1500*time.Millisecond, "ready", "--json")
	if readyOut == "" {
		return 0, false
	}
	return parseReadyCount(readyOut), true
}

// sessionBootstrapMailUnread is a soft probe for mcp-agent-mail. The current
// implementation returns (0, false) — wiring to the MCP transport lives in a
// follow-up bead so this primitive can ship now. The schema field stays
// nullable so callers handle both states.
func sessionBootstrapMailUnread(_ context.Context) (int, bool) {
	if os.Getenv("MCP_AGENT_MAIL_DISABLED") == "1" {
		return 0, false
	}
	return 0, false
}

// printBootstrapSummary writes the 1-line human form. Used when neither
// --json nor --robot is set.
func printBootstrapSummary(cmd *cobra.Command, s SessionBootstrapStatus) error {
	mdMark := "missing"
	if s.AgentsMDRead {
		mdMark = "ok"
	}
	mail := "n/a"
	if s.MailUnreadCount != nil {
		mail = fmt.Sprintf("%d", *s.MailUnreadCount)
	}
	ready := "n/a"
	if s.ReadyBeadsCount != nil {
		ready = fmt.Sprintf("%d", *s.ReadyBeadsCount)
	}

	parts := []string{
		fmt.Sprintf("agents_md=%s", mdMark),
		fmt.Sprintf("routes=%d/%d", len(s.CanonicalRoutesPresent), len(canonicalAgentRoutes)),
		fmt.Sprintf("onboard=%s", s.OnboardPhase),
		fmt.Sprintf("beads=%s", s.BeadsDirSource),
		fmt.Sprintf("ready=%s", ready),
		fmt.Sprintf("mail=%s", mail),
		fmt.Sprintf("gate=%s", s.GateHook),
	}
	if _, err := fmt.Fprintf(cmd.OutOrStdout(), "session bootstrap: %s\n", strings.Join(parts, " ")); err != nil {
		return err
	}
	if s.GateHook == "inactive" {
		if _, err := fmt.Fprintln(cmd.ErrOrStderr(),
			"warn: pre-push gate not installed — run `bash scripts/install-pre-push-gate.sh` to arm the push-as-CI release authority (otherwise a push bypasses the gate)"); err != nil {
			return err
		}
	}
	if s.BeadsDir != "" {
		if _, err := fmt.Fprintf(cmd.OutOrStdout(), "tracker: BEADS_DIR=%s br ready --json\n", s.BeadsDir); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintln(cmd.OutOrStdout(),
		`skills: ms search "<task>" (or mcp__ms__search) to find a skill before hand-grepping the catalog`); err != nil {
		return err
	}
	if len(s.BootstrapMemory) > 0 {
		if _, err := fmt.Fprintf(cmd.OutOrStdout(), "bootstrap memory: %d item(s), %d/%d tokens\n",
			len(s.BootstrapMemory), s.BootstrapMemoryUsedTokens, s.BootstrapMemoryBudgetTokens); err != nil {
			return err
		}
		for _, item := range s.BootstrapMemory {
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "- %s [%s/%s] score=%.3f source=%s\n",
				item.Title, item.Maturity, item.Reach, item.Score, item.Source); err != nil {
				return err
			}
			if content := formatBootstrapMemoryContent(item.Content); content != "" {
				if _, err := fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", content); err != nil {
					return err
				}
			}
		}
	}
	if !s.AgentsMDRead {
		_, err := fmt.Fprintln(cmd.ErrOrStderr(),
			"warn: AGENTS.md missing — start with `cat AGENTS.md` if it exists, or `BEADS_DIR=\"$(ao beads dir)\" br ready --json` for repo orientation")
		return err
	}
	return nil
}

func formatBootstrapMemoryContent(content string) string {
	content = strings.TrimSpace(content)
	if content == "" {
		return ""
	}
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimSpace(line)
	}
	return strings.Join(lines, "\n  ")
}

// sessionBootstrapMakeReady is a test seam for the ready-beads path. Kept here
// so the test file can override without touching exec.Command.
var sessionBootstrapMakeReady = sessionBootstrapReadyBeads

// errNotImplemented is reserved for future use when one of the bootstrap steps
// hard-fails. Currently every step is fail-open and returns a marker instead.
var errNotImplemented = errors.New("bootstrap substep not yet implemented")

var _ = errNotImplemented // suppress unused-var until follow-up bead wires it in
