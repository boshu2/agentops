package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

const (
	defaultOmnigentBundle      = "/Users/bo/dev/omnigent-agents/agents/olympus-trinity"
	defaultOmnigentTimeout     = 1800
	defaultOmnigentReceiptPath = ".agents/ao/omnigent/<run-id>-receipt.json"
	defaultOmnigentLogDir      = "~/.omnigent/logs"
)

var (
	omnigentDispatchBundle         string
	omnigentDispatchTask           string
	omnigentDispatchTimeoutSeconds int
	omnigentDispatchReceiptPath    string
	omnigentDispatchPacketPath     string
)

type omnigentTaskPacket struct {
	SchemaVersion  int    `json:"schema_version,omitempty"`
	Bundle         string `json:"bundle,omitempty"`
	Task           string `json:"task,omitempty"`
	TimeoutSeconds int    `json:"timeout_seconds,omitempty"`
	ReceiptPath    string `json:"receipt_path,omitempty"`
	Output         struct {
		ReceiptPath string `json:"receipt_path,omitempty"`
	} `json:"output,omitempty"`
}

type omnigentDispatchOptions struct {
	Bundle         string
	Task           string
	TimeoutSeconds int
	ReceiptPath    string
	PacketPath     string
}

type omnigentRunReceipt struct {
	SchemaVersion  int                    `json:"schema_version"`
	ReceiptID      string                 `json:"receipt_id"`
	RunID          string                 `json:"run_id"`
	Bundle         string                 `json:"bundle"`
	Task           string                 `json:"task"`
	StartedAt      string                 `json:"started_at"`
	EndedAt        string                 `json:"ended_at"`
	CWD            string                 `json:"cwd"`
	Argv           []string               `json:"argv"`
	TimeoutSeconds int                    `json:"timeout_seconds"`
	ExitCode       int                    `json:"exit_code"`
	TimedOut       bool                   `json:"timed_out"`
	StdoutPath     string                 `json:"stdout_path"`
	ReceiptPath    string                 `json:"receipt_path"`
	LogDir         string                 `json:"log_dir"`
	Verdict        omnigentReceiptVerdict `json:"verdict"`
	ChangedFiles   []string               `json:"changed_files"`
	FailureReason  string                 `json:"failure_reason,omitempty"`
}

type omnigentReceiptVerdict struct {
	Status           string `json:"status"`
	Branch           string `json:"branch,omitempty"`
	Reason           string `json:"reason,omitempty"`
	Judge            string `json:"judge"`
	JudgeModelFamily string `json:"judge_model_family"`
	Summary          string `json:"summary"`
}

var omnigentCmd = &cobra.Command{
	Use:   "omnigent",
	Short: "Omnigent orchestration commands",
	Args:  cobra.NoArgs,
	Long: `Omnigent orchestration commands for running first-class AgentOps
orchestration targets through managed Omnigent bundles.`,
}

var omnigentDispatchCmd = &cobra.Command{
	Use:     "dispatch",
	Aliases: []string{"run"},
	Short:   "Run an Omnigent bundle task and write a receipt",
	Args:    cobra.NoArgs,
	RunE:    runOmnigentDispatch,
}

func init() {
	omnigentCmd.GroupID = "workflow"
	rootCmd.AddCommand(omnigentCmd)
	omnigentCmd.AddCommand(omnigentDispatchCmd)

	omnigentDispatchCmd.Flags().StringVar(&omnigentDispatchBundle, "bundle", defaultOmnigentBundle, "Omnigent bundle directory")
	omnigentDispatchCmd.Flags().StringVar(&omnigentDispatchTask, "task", "", "Task prompt to run (required unless --packet supplies task)")
	omnigentDispatchCmd.Flags().IntVar(&omnigentDispatchTimeoutSeconds, "timeout-seconds", defaultOmnigentTimeout, "Timeout budget for the Omnigent run")
	omnigentDispatchCmd.Flags().StringVar(&omnigentDispatchReceiptPath, "receipt", defaultOmnigentReceiptPath, "Receipt output path")
	omnigentDispatchCmd.Flags().StringVar(&omnigentDispatchPacketPath, "packet", "", "Optional Omnigent task packet JSON file")
}

func runOmnigentDispatch(cmd *cobra.Command, _ []string) error {
	opts, err := omnigentDispatchOptionsFromFlags(cmd)
	if err != nil {
		return err
	}
	receipt, err := performOmnigentDispatch(opts)
	if receipt.ReceiptID != "" {
		printOmnigentDispatchReceipt(cmd, receipt)
	}
	return err
}

func omnigentDispatchOptionsFromFlags(cmd *cobra.Command) (omnigentDispatchOptions, error) {
	opts := omnigentDispatchOptions{
		Bundle:         defaultOmnigentBundle,
		TimeoutSeconds: defaultOmnigentTimeout,
		ReceiptPath:    defaultOmnigentReceiptPath,
		PacketPath:     strings.TrimSpace(omnigentDispatchPacketPath),
	}
	if opts.PacketPath != "" {
		packet, err := readOmnigentTaskPacket(opts.PacketPath)
		if err != nil {
			return omnigentDispatchOptions{}, err
		}
		opts = opts.withPacket(packet)
	}
	if cmd.Flags().Changed("bundle") {
		opts.Bundle = omnigentDispatchBundle
	}
	if cmd.Flags().Changed("task") {
		opts.Task = omnigentDispatchTask
	}
	if cmd.Flags().Changed("timeout-seconds") {
		opts.TimeoutSeconds = omnigentDispatchTimeoutSeconds
	}
	if cmd.Flags().Changed("receipt") {
		opts.ReceiptPath = omnigentDispatchReceiptPath
	}
	return opts, validateOmnigentDispatchOptions(opts)
}

func (opts omnigentDispatchOptions) withPacket(packet omnigentTaskPacket) omnigentDispatchOptions {
	if strings.TrimSpace(packet.Bundle) != "" {
		opts.Bundle = packet.Bundle
	}
	if strings.TrimSpace(packet.Task) != "" {
		opts.Task = packet.Task
	}
	if packet.TimeoutSeconds > 0 {
		opts.TimeoutSeconds = packet.TimeoutSeconds
	}
	if strings.TrimSpace(packet.ReceiptPath) != "" {
		opts.ReceiptPath = packet.ReceiptPath
	}
	if strings.TrimSpace(packet.Output.ReceiptPath) != "" {
		opts.ReceiptPath = packet.Output.ReceiptPath
	}
	return opts
}

func readOmnigentTaskPacket(path string) (omnigentTaskPacket, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return omnigentTaskPacket{}, fmt.Errorf("read omnigent task packet: %w", err)
	}
	var packet omnigentTaskPacket
	if err := json.Unmarshal(data, &packet); err != nil {
		return omnigentTaskPacket{}, fmt.Errorf("parse omnigent task packet: %w", err)
	}
	if packet.SchemaVersion != 0 && packet.SchemaVersion != 1 {
		return omnigentTaskPacket{}, fmt.Errorf("omnigent task packet schema_version = %d, want 1", packet.SchemaVersion)
	}
	return packet, nil
}

func validateOmnigentDispatchOptions(opts omnigentDispatchOptions) error {
	if strings.TrimSpace(opts.Bundle) == "" {
		return fmt.Errorf("omnigent dispatch requires --bundle")
	}
	if strings.TrimSpace(opts.Task) == "" {
		return fmt.Errorf("omnigent dispatch requires --task")
	}
	if opts.TimeoutSeconds <= 0 {
		return fmt.Errorf("omnigent dispatch --timeout-seconds must be > 0")
	}
	if strings.TrimSpace(opts.ReceiptPath) == "" {
		return fmt.Errorf("omnigent dispatch requires --receipt")
	}
	return nil
}

func performOmnigentDispatch(opts omnigentDispatchOptions) (omnigentRunReceipt, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return omnigentRunReceipt{}, fmt.Errorf("get current directory: %w", err)
	}
	cwd, err = filepath.Abs(cwd)
	if err != nil {
		return omnigentRunReceipt{}, fmt.Errorf("resolve current directory: %w", err)
	}

	started := time.Now().UTC()
	runID := newOmnigentRunID(started)
	receiptPath := omnigentReceiptPathForRun(opts.ReceiptPath, runID)
	stdoutPath := filepath.Join(".agents", "ao", "omnigent", runID+"-stdout.txt")
	stdoutAbs, err := resolveOmnigentOutputPath(cwd, stdoutPath)
	if err != nil {
		return omnigentRunReceipt{}, err
	}
	receiptAbs, err := resolveOmnigentOutputPath(cwd, receiptPath)
	if err != nil {
		return omnigentRunReceipt{}, err
	}
	if err := os.MkdirAll(filepath.Dir(stdoutAbs), 0o750); err != nil {
		return omnigentRunReceipt{}, fmt.Errorf("create omnigent stdout dir: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(receiptAbs), 0o750); err != nil {
		return omnigentRunReceipt{}, fmt.Errorf("create omnigent receipt dir: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(opts.TimeoutSeconds)*time.Second)
	defer cancel()

	// NOTE: no --log — it is REPL-only and conflicts with headless -p. The agent's
	// full final report (incl. the "VERDICT:" sentinel) goes to stdout, which we capture.
	argv := []string{"omnigent", "run", strings.TrimSpace(opts.Bundle), "-p", strings.TrimSpace(opts.Task)}
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir = cwd
	devNull, err := os.Open(os.DevNull)
	if err != nil {
		return omnigentRunReceipt{}, fmt.Errorf("open %s: %w", os.DevNull, err)
	}
	defer devNull.Close()
	cmd.Stdin = devNull
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()
	ended := time.Now().UTC()
	timedOut := ctx.Err() == context.DeadlineExceeded
	exitCode := omnigentDispatchExitCode(runErr, timedOut)
	if err := writeOmnigentCombinedOutput(stdoutAbs, stdout.Bytes(), stderr.Bytes()); err != nil {
		return omnigentRunReceipt{}, err
	}

	verdict := parseOmnigentVerdict(stdout.String())
	failureReason := omnigentDispatchFailureReason(runErr, timedOut, stderr.String())
	if failureReason == "" && verdict.Status != "WORTHY" {
		failureReason = verdict.Summary
	}

	receipt := omnigentRunReceipt{
		SchemaVersion:  1,
		ReceiptID:      "omnigent-receipt-" + started.Format("20060102T150405Z") + "-" + runID,
		RunID:          runID,
		Bundle:         strings.TrimSpace(opts.Bundle),
		Task:           strings.TrimSpace(opts.Task),
		StartedAt:      started.Format(time.RFC3339),
		EndedAt:        ended.Format(time.RFC3339),
		CWD:            cwd,
		Argv:           argv,
		TimeoutSeconds: opts.TimeoutSeconds,
		ExitCode:       exitCode,
		TimedOut:       timedOut,
		StdoutPath:     stdoutPath,
		ReceiptPath:    receiptPath,
		LogDir:         defaultOmnigentLogDir,
		Verdict:        verdict,
		ChangedFiles:   collectOmnigentDispatchChangedFiles(cwd),
		FailureReason:  failureReason,
	}
	if err := writeOmnigentRunReceipt(receiptAbs, receipt); err != nil {
		return omnigentRunReceipt{}, err
	}

	if timedOut {
		return receipt, fmt.Errorf("omnigent dispatch timed out after %ds", opts.TimeoutSeconds)
	}
	switch receipt.Verdict.Status {
	case "WORTHY":
		return receipt, nil
	case "UNWORTHY", "BLOCKED", "UNKNOWN":
		return receipt, fmt.Errorf("omnigent verdict %s: %s", receipt.Verdict.Status, receipt.Verdict.Summary)
	default:
		return receipt, fmt.Errorf("omnigent verdict %s: %s", receipt.Verdict.Status, receipt.Verdict.Summary)
	}
}

func parseOmnigentVerdict(stdout string) omnigentReceiptVerdict {
	verdict := omnigentReceiptVerdict{
		Status:           "UNKNOWN",
		Judge:            "themis",
		JudgeModelFamily: "codex/openai",
		Summary:          omnigentVerdictSummary(stdout),
	}
	re := regexp.MustCompile(`(?mi)^\s*VERDICT\s*[:=]\s*(WORTHY|UNWORTHY|BLOCKED)\b(.*)$`)
	if match := re.FindStringSubmatch(stdout); len(match) == 3 {
		verdict.Status = strings.ToUpper(match[1])
		rest := strings.TrimSpace(match[2])
		verdict.Branch = regexpFirstSubmatch(rest, `\bbranch=(\S+)`)
		verdict.Reason = regexpFirstSubmatch(rest, `\breason=(.*)$`)
		verdict.Summary = strings.TrimSpace(match[0])
		if verdict.Status == "BLOCKED" && verdict.Reason != "" {
			verdict.Summary = verdict.Reason
		}
		return verdict
	}

	// FALLBACK — fail-CLOSED. Only the canonical VERDICT: sentinel above can mark a
	// PASS. A bare "WORTHY" in prose is NOT trusted: a model can write "not yet
	// worthy of merge" / "worthy of more review", and promoting that to a pass is a
	// fail-OPEN hole. We DO still honor a clear UNWORTHY in prose, because failing
	// closed is always safe. No sentinel and no clear UNWORTHY -> stays UNKNOWN
	// (non-zero exit), forcing a real sentinel rather than guessing a pass.
	tail := stdout
	if len(tail) > 2048 {
		tail = tail[len(tail)-2048:]
	}
	if regexp.MustCompile(`(?i)\bUNWORTHY\b`).MatchString(tail) {
		verdict.Status = "UNWORTHY"
		verdict.Summary = omnigentVerdictSummary(tail)
	}
	return verdict
}

func regexpFirstSubmatch(text, pattern string) string {
	match := regexp.MustCompile(pattern).FindStringSubmatch(text)
	if len(match) != 2 {
		return ""
	}
	return strings.TrimSpace(match[1])
}

func omnigentVerdictSummary(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return "No Omnigent verdict found."
	}
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			return line
		}
	}
	return "No Omnigent verdict found."
}

func writeOmnigentCombinedOutput(path string, stdout, stderr []byte) error {
	var combined bytes.Buffer
	combined.Write(stdout)
	if len(stderr) > 0 {
		if combined.Len() > 0 && !bytes.HasSuffix(combined.Bytes(), []byte("\n")) {
			combined.WriteByte('\n')
		}
		combined.WriteString("[stderr]\n")
		combined.Write(stderr)
	}
	if err := atomicWriteFile(path, combined.Bytes(), 0o600); err != nil {
		return fmt.Errorf("write omnigent stdout: %w", err)
	}
	return nil
}

func writeOmnigentRunReceipt(path string, receipt omnigentRunReceipt) error {
	data, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal omnigent run receipt: %w", err)
	}
	if err := atomicWriteFile(path, append(data, '\n'), 0o600); err != nil {
		return fmt.Errorf("write omnigent run receipt: %w", err)
	}
	return nil
}

func printOmnigentDispatchReceipt(cmd *cobra.Command, receipt omnigentRunReceipt) {
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Verdict: %s\n", receipt.Verdict.Status)
	if receipt.Verdict.Branch != "" {
		fmt.Fprintf(out, "Branch:  %s\n", receipt.Verdict.Branch)
	}
	fmt.Fprintf(out, "Receipt: %s\n", receipt.ReceiptPath)
}

func newOmnigentRunID(started time.Time) string {
	return fmt.Sprintf("omnigent-%s-%d", started.Format("20060102T150405Z"), os.Getpid())
}

func omnigentReceiptPathForRun(path, runID string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		path = defaultOmnigentReceiptPath
	}
	return strings.ReplaceAll(path, "<run-id>", runID)
}

func resolveOmnigentOutputPath(cwd, path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("empty omnigent output path")
	}
	if filepath.IsAbs(path) {
		return filepath.Clean(path), nil
	}
	return filepath.Clean(filepath.Join(cwd, path)), nil
}

func omnigentDispatchExitCode(err error, timedOut bool) int {
	if timedOut {
		return -1
	}
	if err == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	return -1
}

func omnigentDispatchFailureReason(err error, timedOut bool, stderr string) string {
	if timedOut {
		return "timeout"
	}
	if err == nil {
		return ""
	}
	stderr = strings.TrimSpace(stderr)
	if stderr != "" {
		return stderr
	}
	return err.Error()
}

func collectOmnigentDispatchChangedFiles(cwd string) []string {
	files := []string{}
	cmd := exec.Command("git", "-C", cwd, "status", "--short", "--untracked-files=all")
	out, err := cmd.Output()
	if err != nil {
		return files
	}
	for _, line := range strings.Split(string(out), "\n") {
		if len(line) < 4 {
			continue
		}
		path := strings.TrimSpace(line[3:])
		if strings.Contains(path, " -> ") {
			parts := strings.Split(path, " -> ")
			path = strings.TrimSpace(parts[len(parts)-1])
		}
		if path != "" {
			files = append(files, path)
		}
	}
	return files
}
