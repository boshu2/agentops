package main

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var hooksRunCmd = &cobra.Command{
	Use:   "run <hook-name>",
	Short: "Run a managed hook backend",
	Long: `Run a managed AgentOps hook backend.

This command is intended for hook wrapper scripts. It reads the runtime hook JSON
payload from stdin, writes hook JSON output when the hook has context to inject,
and otherwise exits silently.`,
	Args: cobra.ExactArgs(1),
	RunE: runManagedHook,
}

type runtimeHookInput struct {
	ToolName  string `json:"tool_name"`
	ToolInput struct {
		Command  string `json:"command"`
		FilePath string `json:"file_path"`
	} `json:"tool_input"`
	ToolResponse struct {
		ExitCode any `json:"exit_code"`
	} `json:"tool_response"`
	Prompt string `json:"prompt"`
}

func runManagedHook(cmd *cobra.Command, args []string) error {
	payload, err := io.ReadAll(os.Stdin)
	if err != nil {
		return nil
	}

	switch strings.TrimSpace(args[0]) {
	case "commit-review-gate", "commit-review":
		return runCommitReviewHook(payload)
	case "ratchet-advance":
		return runRatchetAdvanceHook(payload)
	case "quality-signals":
		return runQualitySignalsHook(payload)
	default:
		return fmt.Errorf("unknown managed hook %q", args[0])
	}
}

func decodeRuntimeHookInput(payload []byte) runtimeHookInput {
	var input runtimeHookInput
	if len(strings.TrimSpace(string(payload))) == 0 {
		return input
	}
	_ = json.Unmarshal(payload, &input)
	return input
}

func emitHookAdditionalContext(eventName, contextText string) error {
	if contextText == "" {
		return nil
	}
	output := map[string]any{
		"hookSpecificOutput": map[string]string{
			"hookEventName":     eventName,
			"additionalContext": contextText,
		},
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetEscapeHTML(false)
	return encoder.Encode(output)
}

func envInt(name string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 {
		return fallback
	}
	return value
}

func runCommitReviewHook(payload []byte) error {
	if os.Getenv("AGENTOPS_HOOKS_DISABLED") == "1" || os.Getenv("AGENTOPS_COMMIT_REVIEW_DISABLED") == "1" {
		return nil
	}

	input := decodeRuntimeHookInput(payload)
	toolName := hookFirstNonEmpty(os.Getenv("CLAUDE_TOOL_NAME"), input.ToolName)
	commandText := hookFirstNonEmpty(os.Getenv("CLAUDE_TOOL_INPUT_COMMAND"), input.ToolInput.Command)

	if toolName != "Bash" || !strings.Contains(commandText, "git commit") {
		return nil
	}
	if strings.Contains(commandText, "--amend") && strings.Contains(commandText, "--no-edit") {
		return nil
	}

	diffStat, err := gitOutput("diff", "--cached", "--stat")
	if err != nil || strings.TrimSpace(diffStat) == "" {
		return nil
	}
	fullDiff, err := gitOutput("diff", "--cached")
	if err != nil || strings.TrimSpace(fullDiff) == "" {
		return nil
	}

	fileCount := countDiffFiles(fullDiff)
	if fileCount == 0 {
		return nil
	}

	lineLimit := envInt("AGENTOPS_COMMIT_REVIEW_DIFF_LINES", 80)
	if os.Getenv("AGENTOPS_COMMIT_REVIEW_FULL_DIFF") == "1" && lineLimit < 200 {
		lineLimit = 200
	}
	diffLines := hookCountLines(fullDiff)
	diffPreview := redactSensitiveDiff(firstLines(fullDiff, lineLimit))

	truncated := ""
	if diffLines > lineLimit {
		truncated = fmt.Sprintf(" (showing first %d of %d lines; run 'git diff --cached' for full diff)", lineLimit, diffLines)
	}

	reviewMsg := fmt.Sprintf(`SELF-REVIEW before committing (%d files changed):
Check for: wrong variable references, changed defaults, removed error handling, silent data loss, YAML syntax errors.

Staged changes:
%s
%s

%s`, fileCount, diffStat, truncated, diffPreview)

	return emitHookAdditionalContext("PreToolUse", reviewMsg)
}

func gitOutput(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	out, err := cmd.Output()
	return string(out), err
}

func countDiffFiles(diff string) int {
	count := 0
	scanner := bufio.NewScanner(strings.NewReader(diff))
	for scanner.Scan() {
		if strings.HasPrefix(scanner.Text(), "diff --git ") {
			count++
		}
	}
	return count
}

func hookCountLines(s string) int {
	if s == "" {
		return 0
	}
	lines := strings.Count(s, "\n")
	if !strings.HasSuffix(s, "\n") {
		lines++
	}
	return lines
}

func firstLines(s string, limit int) string {
	if limit <= 0 {
		return ""
	}
	scanner := bufio.NewScanner(strings.NewReader(s))
	var b strings.Builder
	lines := 0
	for scanner.Scan() {
		if lines >= limit {
			break
		}
		if lines > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(scanner.Text())
		lines++
	}
	return b.String()
}

var (
	diffSecretAssignmentRE = regexp.MustCompile("(?i)((?:[A-Za-z0-9_-]*(?:api[_-]?key|token|password|passwd|secret)[A-Za-z0-9_-]*)[[:space:]]*[:=][[:space:]]*)[^[:space:]\"'`]+")
	diffAuthorizationRE    = regexp.MustCompile("(?i)((?:Authorization)[[:space:]]*:[[:space:]]*(?:Bearer|Basic)[[:space:]]+)[^[:space:]\"'`]+")
)

func redactSensitiveDiff(diff string) string {
	diff = diffSecretAssignmentRE.ReplaceAllString(diff, "${1}[REDACTED]")
	return diffAuthorizationRE.ReplaceAllString(diff, "${1}[REDACTED]")
}

func runRatchetAdvanceHook(payload []byte) error {
	if os.Getenv("AGENTOPS_HOOKS_DISABLED") == "1" || os.Getenv("AGENTOPS_AUTOCHAIN") == "0" {
		return nil
	}

	input := decodeRuntimeHookInput(payload)
	commandText := input.ToolInput.Command
	if !strings.Contains(commandText, "ao ratchet record") {
		return nil
	}
	if !hookExitCodeAllowsAdvance(input.ToolResponse.ExitCode) {
		return nil
	}

	step := extractRatchetStep(commandText)
	if step == "" {
		return nil
	}
	if !ratchetBeadBelongsToActiveEpic(commandText) {
		return nil
	}

	next := ratchetNextSkill(step)
	if next == "" {
		return nil
	}

	artifact := extractOutputArtifact(commandText)
	root := repoRoot()
	if ratchetNextStepAlreadyDone(filepath.Join(root, ".agents", "ao", "chain.jsonl"), step) {
		return nil
	}

	flagDir := filepath.Join(root, ".agents", "ao")
	_ = os.MkdirAll(flagDir, 0750)
	_ = os.WriteFile(filepath.Join(flagDir, ".ratchet-advance-fired"), []byte(time.Now().UTC().Format(time.RFC3339)+" "+step+"\n"), 0600)

	var msg string
	switch {
	case next == "Cycle complete":
		msg = fmt.Sprintf("RPI auto-advance: %s completed. Cycle complete; all RPI steps done.", step)
	case artifact != "":
		msg = fmt.Sprintf("RPI auto-advance: %s completed. Suggested next skill: %s %s", step, next, artifact)
	default:
		msg = fmt.Sprintf("RPI auto-advance: %s completed. Suggested next skill: %s", step, next)
	}
	return emitHookAdditionalContext("PostToolUse", msg)
}

func hookExitCodeAllowsAdvance(value any) bool {
	switch v := value.(type) {
	case nil:
		return true
	case float64:
		return v == 0
	case string:
		return v == "" || v == "0"
	default:
		return false
	}
}

var ratchetRecordRE = regexp.MustCompile(`(?:^|[;&|[:space:]])ao[[:space:]]+ratchet[[:space:]]+record[[:space:]]+([a-z_-]+)`)

func extractRatchetStep(commandText string) string {
	matches := ratchetRecordRE.FindStringSubmatch(commandText)
	if len(matches) < 2 {
		return ""
	}
	return matches[1]
}

func ratchetBeadBelongsToActiveEpic(commandText string) bool {
	activeEpic := strings.TrimSpace(os.Getenv("AGENTOPS_ACTIVE_EPIC"))
	if activeEpic == "" {
		return true
	}
	beadID := extractRatchetBeadID(commandText)
	if beadID == "" {
		return true
	}
	bdPath, err := exec.LookPath("bd")
	if err != nil {
		return true
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, bdPath, "parent", beadID).Output()
	if err != nil {
		return true
	}
	return strings.Contains(string(out), activeEpic)
}

var beadIDRE = regexp.MustCompile(`\b[a-z]{2}-[a-z0-9]+\b`)

func extractRatchetBeadID(commandText string) string {
	idx := strings.Index(commandText, "ao ratchet record")
	if idx < 0 {
		return ""
	}
	after := commandText[idx+len("ao ratchet record"):]
	fields := strings.Fields(after)
	if len(fields) < 2 {
		return ""
	}
	for _, field := range fields[1:] {
		if beadIDRE.MatchString(field) {
			return beadIDRE.FindString(field)
		}
	}
	return ""
}

func ratchetNextSkill(step string) string {
	if os.Getenv("AGENTOPS_RATCHET_ADVANCE_DYNAMIC_NEXT") == "1" {
		if next := ratchetNextFromCLI(); next != "" {
			return next
		}
	}
	switch step {
	case "research":
		return "plan"
	case "plan":
		return "pre-mortem"
	case "pre-mortem":
		return "implement or crank"
	case "implement", "crank":
		return "vibe"
	case "vibe":
		return "post-mortem"
	case "post-mortem":
		return "Cycle complete"
	default:
		return ""
	}
}

func ratchetNextFromCLI() string {
	aoPath, err := exec.LookPath("ao")
	if err != nil {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(envInt("AGENTOPS_RATCHET_ADVANCE_TIMEOUT", 2))*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, aoPath, "ratchet", "next", "--json")
	cmd.Stderr = io.Discard
	out, err := cmd.Output()
	if err != nil || len(strings.TrimSpace(string(out))) == 0 {
		return ""
	}
	var result struct {
		Skill    string `json:"skill"`
		Complete bool   `json:"complete"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		return ""
	}
	if result.Complete {
		return "Cycle complete"
	}
	return strings.TrimPrefix(result.Skill, "/")
}

func extractOutputArtifact(commandText string) string {
	fields := strings.Fields(commandText)
	for i, field := range fields {
		if field != "--output" || i+1 >= len(fields) {
			continue
		}
		artifact := fields[i+1]
		if strings.HasPrefix(artifact, "/") || strings.HasPrefix(artifact, "..") || strings.Contains(artifact, "/../") {
			return ""
		}
		return artifact
	}
	return ""
}

func repoRoot() string {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err == nil {
		root := strings.TrimSpace(string(out))
		if root != "" {
			return root
		}
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return cwd
}

func ratchetNextStepAlreadyDone(chainPath, step string) bool {
	nextStep := ratchetNextStepName(step)
	if nextStep == "" {
		return false
	}
	data, err := os.ReadFile(chainPath)
	if err != nil {
		return false
	}
	entry := lastChainEntryForStep(string(data), nextStep)
	return chainEntryDone(entry)
}

func ratchetNextStepName(step string) string {
	switch step {
	case "research":
		return "plan"
	case "plan":
		return "pre-mortem"
	case "pre-mortem":
		return "implement"
	case "implement", "crank":
		return "vibe"
	case "vibe":
		return "post-mortem"
	default:
		return ""
	}
}

func lastChainEntryForStep(chain, step string) string {
	var last string
	scanner := bufio.NewScanner(strings.NewReader(chain))
	stepRE := regexp.MustCompile(`"(step|gate)"[[:space:]]*:[[:space:]]*"` + regexp.QuoteMeta(step) + `"`)
	for scanner.Scan() {
		line := scanner.Text()
		if stepRE.MatchString(line) {
			last = line
		}
	}
	return last
}

func chainEntryDone(entry string) bool {
	if entry == "" {
		return false
	}
	if regexp.MustCompile(`"status"[[:space:]]*:[[:space:]]*"(locked|skipped)"`).MatchString(entry) {
		return true
	}
	return regexp.MustCompile(`"locked"[[:space:]]*:[[:space:]]*true`).MatchString(entry)
}

func runQualitySignalsHook(payload []byte) error {
	if os.Getenv("AGENTOPS_HOOKS_DISABLED") == "1" || os.Getenv("AGENTOPS_QUALITY_SIGNALS_DISABLED") == "1" {
		return nil
	}
	input := decodeRuntimeHookInput(payload)
	prompt := input.Prompt
	if strings.TrimSpace(prompt) == "" || prompt == "null" {
		return nil
	}

	root := repoRoot()
	stateDir := filepath.Join(root, ".agents", "ao")
	signalDir := filepath.Join(root, ".agents", "signals")
	if err := os.MkdirAll(stateDir, 0750); err != nil {
		return nil
	}
	if err := os.MkdirAll(signalDir, 0750); err != nil {
		return nil
	}

	fingerprint := hashText(prompt)
	lastPromptFile := filepath.Join(stateDir, ".last-prompt")
	if previous, err := os.ReadFile(lastPromptFile); err == nil && strings.TrimSpace(string(previous)) == fingerprint {
		appendQualitySignal(filepath.Join(signalDir, "session-quality.jsonl"), "repeated_prompt", "User submitted identical prompt twice in a row")
	}
	_ = os.WriteFile(lastPromptFile, []byte(fingerprint), 0600)

	promptLower := strings.ToLower(prompt)
	correctionRE := regexp.MustCompile(`^[[:space:]]*(no|wrong|not what|stop|undo|revert|that's not|incorrect)\b`)
	if correctionRE.MatchString(promptLower) {
		appendQualitySignal(filepath.Join(signalDir, "session-quality.jsonl"), "correction", "Prompt starts with correction pattern")
	}
	return nil
}

func hashText(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

func appendQualitySignal(path, signalType, detail string) {
	sessionID := hookFirstNonEmpty(os.Getenv("CODEX_SESSION_ID"), os.Getenv("CODEX_THREAD_ID"), os.Getenv("CLAUDE_SESSION_ID"), "unknown")
	entry := qualitySignalInfo{
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
		SignalType: signalType,
		Detail:     detail,
		SessionID:  sessionID,
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.Write(append(data, '\n'))
}

func hookFirstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
