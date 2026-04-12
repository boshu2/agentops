package main

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/boshu2/agentops/cli/internal/config"
)

const (
	defaultCuratorEngine    = "ollama"
	defaultCuratorOllamaURL = "http://127.0.0.1:11435"
	defaultCuratorModel     = "gemma4:e4b"
	defaultCuratorHourlyCap = 20
)

var defaultCuratorJobKinds = []string{"ingest-claude-session", "lint-wiki", "dream-seed"}

var (
	curatorStatusJSON        bool
	curatorEnqueueKind       string
	curatorEnqueueSource     string
	curatorEnqueueChunkStart int
	curatorEnqueueChunkEnd   int
	curatorCompactDryRun     bool
	curatorCompactApply      bool
	curatorEventSource       string
	curatorEventSeverity     string
	curatorEventAction       string
	curatorEventTarget       string
	curatorEventBudget       int
	curatorEventNote         string
)

type dreamLocalCuratorStatus struct {
	Available       bool                           `json:"available" yaml:"available"`
	Supported       bool                           `json:"supported" yaml:"supported"`
	Enabled         bool                           `json:"enabled" yaml:"enabled"`
	Engine          string                         `json:"engine" yaml:"engine"`
	OllamaURL       string                         `json:"ollama_url,omitempty" yaml:"ollama_url,omitempty"`
	Model           string                         `json:"model,omitempty" yaml:"model,omitempty"`
	ModelInstalled  bool                           `json:"model_installed" yaml:"model_installed"`
	WorkerDir       string                         `json:"worker_dir,omitempty" yaml:"worker_dir,omitempty"`
	VaultDir        string                         `json:"vault_dir,omitempty" yaml:"vault_dir,omitempty"`
	HourlyCap       int                            `json:"hourly_cap,omitempty" yaml:"hourly_cap,omitempty"`
	AllowedJobKinds []string                       `json:"allowed_job_kinds,omitempty" yaml:"allowed_job_kinds,omitempty"`
	Ollama          curatorOllamaProbe             `json:"ollama" yaml:"ollama"`
	Worker          curatorWorkerProbe             `json:"worker" yaml:"worker"`
	Status          map[string]any                 `json:"status_json,omitempty" yaml:"status_json,omitempty"`
	Diagnostics     []string                       `json:"diagnostics,omitempty" yaml:"diagnostics,omitempty"`
	Runners         map[string]curatorRunnerStatus `json:"runners,omitempty" yaml:"runners,omitempty"`
}

type curatorOllamaProbe struct {
	Reachable bool     `json:"reachable" yaml:"reachable"`
	Version   string   `json:"version,omitempty" yaml:"version,omitempty"`
	Models    []string `json:"models,omitempty" yaml:"models,omitempty"`
	Error     string   `json:"error,omitempty" yaml:"error,omitempty"`
}

type curatorWorkerProbe struct {
	QueueDepth      int    `json:"queue_depth" yaml:"queue_depth"`
	ProcessingDepth int    `json:"processing_depth" yaml:"processing_depth"`
	PendingLogDepth int    `json:"pending_log_depth" yaml:"pending_log_depth"`
	StatusPath      string `json:"status_path,omitempty" yaml:"status_path,omitempty"`
	StalePID        bool   `json:"stale_pid,omitempty" yaml:"stale_pid,omitempty"`
	Error           string `json:"error,omitempty" yaml:"error,omitempty"`
}

type curatorRunnerStatus struct {
	Available bool   `json:"available" yaml:"available"`
	Command   string `json:"command,omitempty" yaml:"command,omitempty"`
	Note      string `json:"note,omitempty" yaml:"note,omitempty"`
}

type curatorJob struct {
	ID         string            `json:"id"`
	Kind       string            `json:"kind"`
	CreatedAt  string            `json:"created_at"`
	MaxRetries int               `json:"max_retries"`
	Source     *curatorJobSource `json:"source,omitempty"`
}

type curatorJobSource struct {
	Path       string `json:"path"`
	ChunkStart int    `json:"chunk_start"`
	ChunkEnd   int    `json:"chunk_end"`
}

type curatorEvent struct {
	SchemaVersion    int    `json:"schema_version"`
	ID               string `json:"id"`
	CreatedAt        string `json:"created_at"`
	Source           string `json:"source"`
	Severity         string `json:"severity"`
	DesiredAction    string `json:"desired_action"`
	EscalationTarget string `json:"escalation_target"`
	Budget           int    `json:"budget"`
	Note             string `json:"note,omitempty"`
	Status           string `json:"status"`
}

var overnightCuratorCmd = &cobra.Command{
	Use:   "curator",
	Short: "Operate the local Tier 1 Dream curator",
	Long: `Inspect and operate a local Tier 1 Dream curator such as Ollama/Gemma.

The curator adapter is intentionally bounded: it can report health, enqueue
allowlisted knowledge jobs, compact pending audit log entries, and write
needs-review escalation events for Tier 2 runners. It does not create an
unbounded model-to-model invocation loop.`,
}

var overnightCuratorStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Report local curator queue, worker, and model health",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		status, err := getDreamLocalCuratorStatus(2 * time.Second)
		if err != nil {
			return err
		}
		return outputCuratorStatus(status, curatorStatusJSON || GetOutput() == "json")
	},
}

var overnightCuratorDiagnoseCmd = &cobra.Command{
	Use:   "diagnose",
	Short: "Explain local curator setup problems",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		status, err := getDreamLocalCuratorStatus(2 * time.Second)
		if err != nil {
			return err
		}
		if curatorStatusJSON || GetOutput() == "json" {
			return outputCuratorStatus(status, true)
		}
		if len(status.Diagnostics) == 0 {
			fmt.Println("Local curator looks ready.")
			return nil
		}
		for _, diagnostic := range status.Diagnostics {
			fmt.Printf("- %s\n", diagnostic)
		}
		return nil
	},
}

var overnightCuratorEnqueueCmd = &cobra.Command{
	Use:   "enqueue",
	Short: "Enqueue an allowlisted local curator job",
	Args:  cobra.NoArgs,
	RunE:  runCuratorEnqueue,
}

var overnightCuratorCompactCmd = &cobra.Command{
	Use:   "compact",
	Short: "Run the local curator pending-log compactor",
	Args:  cobra.NoArgs,
	RunE:  runCuratorCompact,
}

var overnightCuratorEventCmd = &cobra.Command{
	Use:   "event",
	Short: "Write a bounded needs-review escalation event",
	Args:  cobra.NoArgs,
	RunE:  runCuratorEvent,
}

func init() {
	overnightCmd.AddCommand(overnightCuratorCmd)
	overnightCuratorCmd.AddCommand(overnightCuratorStatusCmd, overnightCuratorDiagnoseCmd, overnightCuratorEnqueueCmd, overnightCuratorCompactCmd, overnightCuratorEventCmd)

	overnightCuratorStatusCmd.Flags().BoolVar(&curatorStatusJSON, "json", false, "Render curator status as JSON")
	overnightCuratorDiagnoseCmd.Flags().BoolVar(&curatorStatusJSON, "json", false, "Render curator diagnosis as JSON")

	overnightCuratorEnqueueCmd.Flags().StringVar(&curatorEnqueueKind, "kind", "", "Job kind to enqueue: ingest-claude-session, lint-wiki, or dream-seed")
	overnightCuratorEnqueueCmd.Flags().StringVar(&curatorEnqueueSource, "source", "", "Source path for ingest-claude-session jobs")
	overnightCuratorEnqueueCmd.Flags().IntVar(&curatorEnqueueChunkStart, "chunk-start", 0, "Start chunk index for ingest-claude-session jobs")
	overnightCuratorEnqueueCmd.Flags().IntVar(&curatorEnqueueChunkEnd, "chunk-end", 0, "End chunk index for ingest-claude-session jobs")

	overnightCuratorCompactCmd.Flags().BoolVar(&curatorCompactDryRun, "dry-run", false, "Preview pending-log compaction without writing")
	overnightCuratorCompactCmd.Flags().BoolVar(&curatorCompactApply, "apply", false, "Apply pending-log compaction")

	overnightCuratorEventCmd.Flags().StringVar(&curatorEventSource, "source", "", "Event source, for example gemma or local-soc")
	overnightCuratorEventCmd.Flags().StringVar(&curatorEventSeverity, "severity", "", "Event severity: info, warn, high, or critical")
	overnightCuratorEventCmd.Flags().StringVar(&curatorEventAction, "desired-action", "", "Requested bounded action for Tier 2 review")
	overnightCuratorEventCmd.Flags().StringVar(&curatorEventTarget, "target", "dream-council", "Escalation target")
	overnightCuratorEventCmd.Flags().IntVar(&curatorEventBudget, "budget", 1, "Maximum downstream event budget")
	overnightCuratorEventCmd.Flags().StringVar(&curatorEventNote, "note", "", "Optional operator note")
}

func getDreamLocalCuratorStatus(timeout time.Duration) (dreamLocalCuratorStatus, error) {
	cfg, err := config.Load(nil)
	if err != nil {
		return dreamLocalCuratorStatus{}, fmt.Errorf("load config: %w", err)
	}
	curator := resolveDreamLocalCuratorConfig(cfg.Dream.LocalCurator, timeout)
	return buildDreamLocalCuratorStatus(curator, timeout), nil
}

func resolveDreamLocalCuratorConfig(cfg config.DreamLocalCuratorConfig, timeout time.Duration) config.DreamLocalCuratorConfig {
	resolved := cfg
	if resolved.Engine == "" {
		resolved.Engine = defaultCuratorEngine
	}
	if resolved.WorkerDir == "" && curatorDirExists(`D:\dream`) {
		resolved.WorkerDir = `D:\dream`
	}
	if resolved.VaultDir == "" && curatorDirExists(`D:\vault`) {
		resolved.VaultDir = `D:\vault`
	}
	if resolved.OllamaURL == "" {
		fallbackEndpoint := ""
		for _, endpoint := range []string{defaultCuratorOllamaURL, "http://127.0.0.1:11434"} {
			probe := probeOllama(endpoint, timeout)
			if !probe.Reachable {
				continue
			}
			if fallbackEndpoint == "" {
				fallbackEndpoint = endpoint
			}
			if containsString(probe.Models, defaultCuratorModel) {
				resolved.OllamaURL = endpoint
				break
			}
		}
		if resolved.OllamaURL == "" {
			resolved.OllamaURL = fallbackEndpoint
		}
		if resolved.OllamaURL == "" {
			resolved.OllamaURL = defaultCuratorOllamaURL
		}
	}
	if resolved.Model == "" {
		probe := probeOllama(resolved.OllamaURL, timeout)
		for _, candidate := range []string{defaultCuratorModel, "gemma4:latest", "gemma4:26b"} {
			if containsString(probe.Models, candidate) {
				resolved.Model = candidate
				break
			}
		}
		if resolved.Model == "" {
			resolved.Model = defaultCuratorModel
		}
	}
	if resolved.HourlyCap == 0 {
		resolved.HourlyCap = defaultCuratorHourlyCap
	}
	if len(resolved.AllowedJobKinds) == 0 {
		resolved.AllowedJobKinds = append([]string{}, defaultCuratorJobKinds...)
	}
	if resolved.Enabled == nil {
		enabled := resolved.WorkerDir != "" || containsString(probeOllama(resolved.OllamaURL, timeout).Models, resolved.Model)
		resolved.Enabled = &enabled
	}
	return resolved
}

func buildDreamLocalCuratorStatus(curator config.DreamLocalCuratorConfig, timeout time.Duration) dreamLocalCuratorStatus {
	status := dreamLocalCuratorStatus{
		Enabled:         curator.Enabled != nil && *curator.Enabled,
		Engine:          curator.Engine,
		OllamaURL:       curator.OllamaURL,
		Model:           curator.Model,
		WorkerDir:       curator.WorkerDir,
		VaultDir:        curator.VaultDir,
		HourlyCap:       curator.HourlyCap,
		AllowedJobKinds: append([]string{}, curator.AllowedJobKinds...),
		Runners:         detectCuratorRunnerStatuses(),
	}
	status.Ollama = probeOllama(curator.OllamaURL, timeout)
	status.ModelInstalled = containsString(status.Ollama.Models, curator.Model)
	status.Worker = probeCuratorWorker(curator.WorkerDir)
	status.Status = readCuratorStatusJSON(status.Worker.StatusPath)
	status.Diagnostics = diagnoseLocalCurator(status)
	status.Available = status.Ollama.Reachable && status.ModelInstalled && status.Worker.Error == ""
	status.Supported = status.Engine == defaultCuratorEngine
	return status
}

func isDreamLocalCuratorConfigured(curator config.DreamLocalCuratorConfig) bool {
	return curator.Enabled != nil ||
		curator.Engine != "" ||
		curator.OllamaURL != "" ||
		curator.Model != "" ||
		curator.WorkerDir != "" ||
		curator.VaultDir != "" ||
		curator.HourlyCap != 0 ||
		len(curator.AllowedJobKinds) > 0
}

func probeOllama(endpoint string, timeout time.Duration) curatorOllamaProbe {
	probe := curatorOllamaProbe{}
	if strings.TrimSpace(endpoint) == "" {
		probe.Error = "ollama URL is empty"
		return probe
	}
	client := &http.Client{Timeout: timeout}
	versionURL := strings.TrimRight(endpoint, "/") + "/api/version"
	var versionPayload struct {
		Version string `json:"version"`
	}
	if err := getJSON(client, versionURL, &versionPayload); err != nil {
		probe.Error = err.Error()
		return probe
	}
	probe.Reachable = true
	probe.Version = versionPayload.Version

	var tagsPayload struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	tagsURL := strings.TrimRight(endpoint, "/") + "/api/tags"
	if err := getJSON(client, tagsURL, &tagsPayload); err != nil {
		probe.Error = err.Error()
		return probe
	}
	for _, model := range tagsPayload.Models {
		if model.Name != "" {
			probe.Models = append(probe.Models, model.Name)
		}
	}
	sort.Strings(probe.Models)
	return probe
}

func getJSON(client *http.Client, url string, dst any) error {
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("%s returned %s: %s", url, resp.Status, strings.TrimSpace(string(body)))
	}
	return json.NewDecoder(resp.Body).Decode(dst)
}

func probeCuratorWorker(workerDir string) curatorWorkerProbe {
	probe := curatorWorkerProbe{}
	if strings.TrimSpace(workerDir) == "" {
		probe.Error = "worker_dir is not configured"
		return probe
	}
	if !curatorDirExists(workerDir) {
		probe.Error = fmt.Sprintf("worker_dir %q does not exist", workerDir)
		return probe
	}
	probe.QueueDepth = countJSONFiles(filepath.Join(workerDir, "queue"))
	probe.ProcessingDepth = countJSONFiles(filepath.Join(workerDir, "processing"))
	probe.PendingLogDepth = countTextFiles(filepath.Join(workerDir, "logs", "pending-log-entries"))
	probe.StatusPath = filepath.Join(workerDir, "logs", "status.json")
	status := readCuratorStatusJSON(probe.StatusPath)
	if pid, ok := status["pid"].(float64); ok && pid > 0 {
		probe.StalePID = !curatorProcessExists(int(pid))
	}
	return probe
}

func readCuratorStatusJSON(path string) map[string]any {
	if path == "" {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	out := map[string]any{}
	if err := json.Unmarshal(data, &out); err != nil {
		return nil
	}
	return out
}

func diagnoseLocalCurator(status dreamLocalCuratorStatus) []string {
	var diagnostics []string
	if status.Engine != defaultCuratorEngine {
		diagnostics = append(diagnostics, fmt.Sprintf("unsupported local curator engine %q; V1 supports ollama", status.Engine))
	}
	if status.Worker.Error != "" {
		diagnostics = append(diagnostics, status.Worker.Error)
	}
	if strings.TrimSpace(status.VaultDir) == "" {
		diagnostics = append(diagnostics, "vault_dir is not configured")
	} else if !curatorDirExists(status.VaultDir) {
		diagnostics = append(diagnostics, fmt.Sprintf("vault_dir %q does not exist", status.VaultDir))
	}
	if !status.Ollama.Reachable {
		diagnostics = append(diagnostics, fmt.Sprintf("Ollama is not reachable at %s: %s", status.OllamaURL, status.Ollama.Error))
	}
	if status.Ollama.Reachable && !status.ModelInstalled {
		diagnostics = append(diagnostics, fmt.Sprintf("model %q is not installed at %s", status.Model, status.OllamaURL))
	}
	if status.Worker.StalePID {
		diagnostics = append(diagnostics, "worker status.json pid does not appear to be running")
	}
	for _, name := range []string{"openclaw", "oc-ask"} {
		if runner, ok := status.Runners[name]; ok && runner.Available {
			continue
		}
		diagnostics = append(diagnostics, fmt.Sprintf("%s bridge is not discoverable; leaving it unsupported for trigger mesh execution", name))
	}
	if runner, ok := status.Runners["codex"]; ok && !runner.Available {
		diagnostics = append(diagnostics, "codex runner is not discoverable on PATH for Tier 2 review")
	}
	if runner, ok := status.Runners["claude"]; ok && !runner.Available {
		diagnostics = append(diagnostics, "claude runner is not discoverable on PATH for Tier 2 review")
	}
	return diagnostics
}

func detectCuratorRunnerStatuses() map[string]curatorRunnerStatus {
	out := map[string]curatorRunnerStatus{}
	for _, name := range []string{"codex", "claude", "openclaw", "oc-ask"} {
		path, err := exec.LookPath(name)
		status := curatorRunnerStatus{Available: err == nil}
		if err == nil {
			status.Command = path
		}
		switch name {
		case "openclaw", "oc-ask":
			status.Note = "bridge must be discoverable before OpenClaw/Morai can be marked supported"
		case "codex", "claude":
			status.Note = "Tier 2 Dream Council runner"
		}
		out[name] = status
	}
	return out
}

func runCuratorEnqueue(cmd *cobra.Command, args []string) error {
	status, err := getDreamLocalCuratorStatus(2 * time.Second)
	if err != nil {
		return err
	}
	kind := strings.TrimSpace(curatorEnqueueKind)
	if !containsString(status.AllowedJobKinds, kind) {
		return fmt.Errorf("unsupported curator job kind %q: allowed kinds are %s", kind, strings.Join(status.AllowedJobKinds, ", "))
	}
	job := curatorJob{
		ID:         buildCuratorID(kind),
		Kind:       kind,
		CreatedAt:  time.Now().UTC().Format(time.RFC3339),
		MaxRetries: 1,
	}
	if kind == "ingest-claude-session" {
		if strings.TrimSpace(curatorEnqueueSource) == "" {
			return errors.New("--source is required for ingest-claude-session")
		}
		if curatorEnqueueChunkEnd <= curatorEnqueueChunkStart {
			return errors.New("--chunk-end must be greater than --chunk-start for ingest-claude-session")
		}
		job.Source = &curatorJobSource{
			Path:       curatorEnqueueSource,
			ChunkStart: curatorEnqueueChunkStart,
			ChunkEnd:   curatorEnqueueChunkEnd,
		}
	}
	if GetDryRun() {
		return outputCuratorObject(map[string]any{"dry_run": true, "job": job}, curatorStatusJSON || GetOutput() == "json")
	}
	queueDir := filepath.Join(status.WorkerDir, "queue")
	if err := os.MkdirAll(queueDir, 0o755); err != nil {
		return fmt.Errorf("create curator queue dir: %w", err)
	}
	path := filepath.Join(queueDir, job.ID+".json")
	if err := writeJSONAtomic(path, job); err != nil {
		return err
	}
	return outputCuratorObject(map[string]any{"enqueued": path, "job": job}, curatorStatusJSON || GetOutput() == "json")
}

func runCuratorCompact(cmd *cobra.Command, args []string) error {
	if curatorCompactDryRun && curatorCompactApply {
		return errors.New("--dry-run and --apply are mutually exclusive")
	}
	status, err := getDreamLocalCuratorStatus(2 * time.Second)
	if err != nil {
		return err
	}
	script := filepath.Join(status.WorkerDir, "compact-log.js")
	if _, err := os.Stat(script); err != nil {
		return fmt.Errorf("compact-log.js is not available at %s: %w", script, err)
	}
	args = []string{script}
	if curatorCompactDryRun || !curatorCompactApply {
		args = append(args, "--dry-run")
	}
	run := exec.Command("node", args...)
	var stdout, stderr bytes.Buffer
	run.Stdout = &stdout
	run.Stderr = &stderr
	if err := run.Run(); err != nil {
		return fmt.Errorf("run compact-log.js: %w\n%s", err, strings.TrimSpace(stderr.String()))
	}
	fmt.Print(stdout.String())
	if stderr.Len() > 0 {
		fmt.Fprint(os.Stderr, stderr.String())
	}
	return nil
}

func runCuratorEvent(cmd *cobra.Command, args []string) error {
	status, err := getDreamLocalCuratorStatus(2 * time.Second)
	if err != nil {
		return err
	}
	source := strings.TrimSpace(curatorEventSource)
	severity := strings.TrimSpace(curatorEventSeverity)
	action := strings.TrimSpace(curatorEventAction)
	if source == "" || severity == "" || action == "" {
		return errors.New("--source, --severity, and --desired-action are required")
	}
	if !containsString([]string{"info", "warn", "high", "critical"}, severity) {
		return fmt.Errorf("unsupported severity %q: expected info, warn, high, or critical", severity)
	}
	if curatorEventBudget < 0 {
		return errors.New("--budget cannot be negative")
	}
	event := curatorEvent{
		SchemaVersion:    1,
		ID:               buildCuratorID("event"),
		CreatedAt:        time.Now().UTC().Format(time.RFC3339),
		Source:           source,
		Severity:         severity,
		DesiredAction:    action,
		EscalationTarget: strings.TrimSpace(curatorEventTarget),
		Budget:           curatorEventBudget,
		Note:             strings.TrimSpace(curatorEventNote),
		Status:           "needs-review",
	}
	if event.EscalationTarget == "" {
		event.EscalationTarget = "dream-council"
	}
	if GetDryRun() {
		return outputCuratorObject(map[string]any{"dry_run": true, "event": event}, curatorStatusJSON || GetOutput() == "json")
	}
	eventsDir := filepath.Join(status.WorkerDir, "events")
	if err := os.MkdirAll(eventsDir, 0o755); err != nil {
		return fmt.Errorf("create curator events dir: %w", err)
	}
	path := filepath.Join(eventsDir, "pending.jsonl")
	line, err := json.Marshal(event)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open curator event ledger: %w", err)
	}
	defer f.Close()
	if _, err := f.Write(append(line, '\n')); err != nil {
		return fmt.Errorf("write curator event ledger: %w", err)
	}
	return outputCuratorObject(map[string]any{"event_path": path, "event": event}, curatorStatusJSON || GetOutput() == "json")
}

func outputCuratorStatus(status dreamLocalCuratorStatus, asJSON bool) error {
	return outputCuratorObject(status, asJSON)
}

func outputCuratorObject(value any, asJSON bool) error {
	if asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(value)
	}
	if GetOutput() == "yaml" {
		enc := yaml.NewEncoder(os.Stdout)
		if err := enc.Encode(value); err != nil {
			_ = enc.Close()
			return err
		}
		return enc.Close()
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

func writeJSONAtomic(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename temp file: %w", err)
	}
	return nil
}

func buildCuratorID(kind string) string {
	var b [4]byte
	_, _ = rand.Read(b[:])
	suffix := hex.EncodeToString(b[:])
	return fmt.Sprintf("%s-%s-%s", time.Now().UTC().Format("20060102T150405Z"), strings.ReplaceAll(kind, ":", "-"), suffix)
}

func curatorDirExists(path string) bool {
	if path == "" {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func countJSONFiles(dir string) int {
	return countFilesByExt(dir, ".json")
}

func countTextFiles(dir string) int {
	return countFilesByExt(dir, ".txt")
}

func countFilesByExt(dir, ext string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	count := 0
	for _, entry := range entries {
		if !entry.IsDir() && strings.EqualFold(filepath.Ext(entry.Name()), ext) {
			count++
		}
	}
	return count
}

func containsString(items []string, needle string) bool {
	for _, item := range items {
		if item == needle {
			return true
		}
	}
	return false
}

func curatorProcessExists(pid int) bool {
	if pid <= 0 {
		return false
	}
	if runtime.GOOS == "windows" {
		out, err := exec.Command("tasklist", "/FI", fmt.Sprintf("PID eq %d", pid), "/FO", "CSV", "/NH").Output()
		return err == nil && strings.Contains(string(out), fmt.Sprintf(`"%d"`, pid))
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return process.Signal(syscall.Signal(0)) == nil
}
