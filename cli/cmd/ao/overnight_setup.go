package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/boshu2/agentops/cli/internal/config"
)

var (
	overnightSetupApply       bool
	overnightSetupScheduler   string
	overnightSetupAt          string
	overnightSetupRunners     []string
	overnightSetupKeepAwake   bool
	overnightSetupNoKeepAwake bool
)

var (
	dreamOS            = runtime.GOOS
	dreamBatteryStatus = detectBatteryPresence
)

type dreamRuntimeStatus struct {
	Name      string `json:"name" yaml:"name"`
	Available bool   `json:"available" yaml:"available"`
	Supported bool   `json:"supported" yaml:"supported"`
	Command   string `json:"command,omitempty" yaml:"command,omitempty"`
	Note      string `json:"note,omitempty" yaml:"note,omitempty"`
}

type dreamSchedulerStatus struct {
	Mode        string `json:"mode" yaml:"mode"`
	Available   bool   `json:"available" yaml:"available"`
	Recommended bool   `json:"recommended" yaml:"recommended"`
	Note        string `json:"note,omitempty" yaml:"note,omitempty"`
}

type dreamHostProfile struct {
	OS                 string                 `json:"os" yaml:"os"`
	DeviceClass        string                 `json:"device_class" yaml:"device_class"`
	HasBattery         bool                   `json:"has_battery" yaml:"has_battery"`
	KeepAwakeSupported bool                   `json:"keep_awake_supported" yaml:"keep_awake_supported"`
	RecommendedMode    string                 `json:"recommended_mode" yaml:"recommended_mode"`
	Schedulers         []dreamSchedulerStatus `json:"schedulers" yaml:"schedulers"`
}

type dreamSetupSummary struct {
	SchemaVersion  int                     `json:"schema_version" yaml:"schema_version"`
	Mode           string                  `json:"mode" yaml:"mode"`
	Status         string                  `json:"status" yaml:"status"`
	Apply          bool                    `json:"apply" yaml:"apply"`
	RepoRoot       string                  `json:"repo_root" yaml:"repo_root"`
	ConfigPath     string                  `json:"config_path" yaml:"config_path"`
	Host           dreamHostProfile        `json:"host" yaml:"host"`
	Runtimes       []dreamRuntimeStatus    `json:"runtimes" yaml:"runtimes"`
	LocalCurator   dreamLocalCuratorStatus `json:"local_curator" yaml:"local_curator"`
	DreamConfig    config.DreamConfig      `json:"dream" yaml:"dream"`
	GeneratedFiles map[string]string       `json:"generated_files,omitempty" yaml:"generated_files,omitempty"`
	Warnings       []string                `json:"warnings,omitempty" yaml:"warnings,omitempty"`
	Recommended    []string                `json:"recommended,omitempty" yaml:"recommended,omitempty"`
	NextAction     string                  `json:"next_action" yaml:"next_action"`
}

var overnightSetupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Inspect the host and bootstrap Dream configuration",
	Long: `Inspect the current machine, detect available Dream runtimes and scheduler
surfaces, and build an honest Dream configuration preview.

This command is intentionally conservative:
  - it does not pretend sleeping laptops have guaranteed scheduler semantics
  - it prefers manual bedtime runs when host behavior is ambiguous
  - it can persist Dream config and generate host-specific scheduler snippets
    when you opt in with --apply`,
	Args: cobra.NoArgs,
	RunE: runOvernightSetup,
}

func init() {
	overnightCmd.AddCommand(overnightSetupCmd)
	overnightSetupCmd.Flags().BoolVar(&overnightSetupApply, "apply", false, "Persist the detected Dream config and generate scheduler assistance artifacts")
	overnightSetupCmd.Flags().StringVar(&overnightSetupScheduler, "scheduler", "auto", "Scheduler mode to persist (auto, manual, launchd, cron, systemd, task-scheduler)")
	overnightSetupCmd.Flags().StringVar(&overnightSetupAt, "at", "", "Preferred local Dream run time in HH:MM (used for scheduler assistance)")
	overnightSetupCmd.Flags().StringSliceVar(&overnightSetupRunners, "runner", nil, "Dream runner to persist (repeatable: --runner codex --runner claude)")
	overnightSetupCmd.Flags().BoolVar(&overnightSetupKeepAwake, "keep-awake", false, "Persist keep-awake on for Dream runs")
	overnightSetupCmd.Flags().BoolVar(&overnightSetupNoKeepAwake, "no-keep-awake", false, "Persist keep-awake off for Dream runs")
}

func runOvernightSetup(cmd *cobra.Command, args []string) error {
	cwd, err := resolveProjectDir()
	if err != nil {
		return err
	}
	cfg, err := config.Load(nil)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	host := detectDreamHostProfile()
	runtimes := detectDreamRuntimes()
	localCuratorCfg := resolveDreamLocalCuratorConfig(cfg.Dream.LocalCurator, 400*time.Millisecond)
	localCurator := buildDreamLocalCuratorStatus(localCuratorCfg, 400*time.Millisecond)
	selectedRunners, warnings := selectDreamRunners(cfg.Dream, runtimes)
	keepAwake, keepAwakeWarnings := resolveDreamSetupKeepAwake(cfg.Dream, host)
	warnings = append(warnings, keepAwakeWarnings...)

	schedulerMode, schedulerWarnings, err := resolveDreamSchedulerMode(cfg.Dream, host)
	if err != nil {
		return err
	}
	warnings = append(warnings, schedulerWarnings...)

	scheduleAt := strings.TrimSpace(cfg.Dream.ScheduleAt)
	if cmd.Flags().Changed("at") {
		scheduleAt = strings.TrimSpace(overnightSetupAt)
	}
	if scheduleAt != "" && !isValidDailyTime(scheduleAt) {
		return fmt.Errorf("invalid --at value %q: expected HH:MM in 24-hour local time", scheduleAt)
	}

	consensusPolicy := strings.TrimSpace(cfg.Dream.ConsensusPolicy)
	if consensusPolicy == "" {
		consensusPolicy = "majority"
	}

	creativeLane := false
	if cfg.Dream.CreativeLane != nil {
		creativeLane = *cfg.Dream.CreativeLane
	}

	dreamCfg := config.DreamConfig{
		ReportDir:       strings.TrimSpace(cfg.Dream.ReportDir),
		RunTimeout:      strings.TrimSpace(cfg.Dream.RunTimeout),
		KeepAwake:       dreamBoolPtr(keepAwake),
		Runners:         selectedRunners,
		SchedulerMode:   schedulerMode,
		ScheduleAt:      scheduleAt,
		ConsensusPolicy: consensusPolicy,
		CreativeLane:    dreamBoolPtr(creativeLane),
	}
	if localCurator.Available || isDreamLocalCuratorConfigured(cfg.Dream.LocalCurator) {
		dreamCfg.LocalCurator = localCuratorCfg
	}
	if dreamCfg.ReportDir == "" {
		dreamCfg.ReportDir = ".agents/overnight/latest"
	}
	if dreamCfg.RunTimeout == "" {
		dreamCfg.RunTimeout = "8h"
	}

	summary := dreamSetupSummary{
		SchemaVersion: 1,
		Mode:          "dream.setup",
		Status:        "dry-run",
		Apply:         overnightSetupApply && !GetDryRun(),
		RepoRoot:      cwd,
		ConfigPath:    projectConfigPath(),
		Host:          host,
		Runtimes:      runtimes,
		LocalCurator:  localCurator,
		DreamConfig:   dreamCfg,
		Warnings:      warnings,
	}

	if summary.Apply {
		if err := config.Save(&config.Config{Dream: dreamCfg}); err != nil {
			return fmt.Errorf("save dream config: %w", err)
		}
		summary.Status = "configured"
		generated, genWarnings, err := maybeWriteDreamSchedulerArtifacts(cwd, host, dreamCfg)
		if err != nil {
			return err
		}
		summary.GeneratedFiles = generated
		summary.Warnings = append(summary.Warnings, genWarnings...)
	}

	summary.Recommended = recommendedDreamSetupCommands(summary)
	summary.NextAction = deriveDreamSetupNextAction(summary)
	return outputDreamSetupSummary(summary)
}

func detectDreamHostProfile() dreamHostProfile {
	host := dreamHostProfile{
		OS:                 dreamOS,
		DeviceClass:        "desktop-or-unknown",
		KeepAwakeSupported: dreamOS == "darwin" && dreamLookPath("caffeinate") == nil,
		RecommendedMode:    "manual",
	}
	host.HasBattery = dreamBatteryStatus()
	if host.HasBattery {
		host.DeviceClass = "laptop"
	}

	switch dreamOS {
	case "darwin":
		host.Schedulers = append(host.Schedulers, dreamSchedulerStatus{
			Mode:      "launchd",
			Available: true,
			Note:      "Native macOS scheduler. Best-effort only on laptops that may sleep or close the lid.",
		})
		if host.HasBattery {
			host.Schedulers = append(host.Schedulers, dreamSchedulerStatus{
				Mode:        "manual",
				Available:   true,
				Recommended: true,
				Note:        "Recommended on laptops when you want an honest bedtime run with keep-awake assistance.",
			})
		} else {
			host.RecommendedMode = "launchd"
			host.Schedulers[0].Recommended = true
		}
	case "linux":
		systemdAvailable := dreamLookPath("systemctl") == nil
		cronAvailable := dreamLookPath("crontab") == nil
		host.Schedulers = append(host.Schedulers, dreamSchedulerStatus{
			Mode:      "systemd",
			Available: systemdAvailable,
			Note:      "Preferred on Linux when user timers are available.",
		})
		host.Schedulers = append(host.Schedulers, dreamSchedulerStatus{
			Mode:      "cron",
			Available: cronAvailable,
			Note:      "Portable fallback; also subject to host sleep semantics.",
		})
		host.Schedulers = append(host.Schedulers, dreamSchedulerStatus{
			Mode:        "manual",
			Available:   true,
			Recommended: !systemdAvailable || host.HasBattery,
			Note:        "Honest fallback when power or host wake semantics are uncertain.",
		})
		if !host.HasBattery {
			switch {
			case systemdAvailable:
				host.RecommendedMode = "systemd"
				host.Schedulers[0].Recommended = true
				host.Schedulers[2].Recommended = false
			case cronAvailable:
				host.RecommendedMode = "cron"
				host.Schedulers[1].Recommended = true
				host.Schedulers[2].Recommended = false
			}
		}
	case "windows":
		host.RecommendedMode = "task-scheduler"
		host.Schedulers = append(host.Schedulers, dreamSchedulerStatus{
			Mode:        "task-scheduler",
			Available:   true,
			Recommended: true,
			Note:        "Generates a reviewed Windows Task Scheduler script; AgentOps does not register it without operator action.",
		})
		host.Schedulers = append(host.Schedulers, dreamSchedulerStatus{
			Mode:      "manual",
			Available: true,
			Note:      "Honest fallback when you want to arm the bedtime run yourself.",
		})
	default:
		host.Schedulers = append(host.Schedulers, dreamSchedulerStatus{
			Mode:        "manual",
			Available:   true,
			Recommended: true,
			Note:        "No first-class scheduler assistance is implemented for this OS yet.",
		})
	}

	return host
}

func detectDreamRuntimes() []dreamRuntimeStatus {
	type candidate struct {
		name      string
		command   string
		supported bool
		note      string
	}
	candidates := []candidate{
		{name: "codex", command: "codex", supported: true},
		{name: "claude", command: "claude", supported: true},
		{name: "gemini", command: "gemini", supported: false, note: "Detected for future Dream integrations; overnight execution is not wired yet."},
		{name: "openclaw", command: "openclaw", supported: false, note: "OpenClaw/Morai bridge must be discoverable before Dream Council can execute it."},
		{name: "oc-ask", command: "oc-ask", supported: false, note: "OpenClaw/Morai bridge must be discoverable before Dream Council can execute it."},
		{name: "opencode", command: "opencode", supported: false, note: "Detected locally, but Dream Council does not execute it yet."},
	}
	out := make([]dreamRuntimeStatus, 0, len(candidates))
	for _, cand := range candidates {
		_, err := exec.LookPath(cand.command)
		status := dreamRuntimeStatus{
			Name:      cand.name,
			Available: err == nil,
			Supported: cand.supported,
			Command:   cand.command,
			Note:      cand.note,
		}
		if err != nil {
			status.Command = ""
		}
		out = append(out, status)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func selectDreamRunners(existing config.DreamConfig, runtimes []dreamRuntimeStatus) ([]string, []string) {
	var warnings []string
	selected := normalizeDreamRunnerList(existing.Runners)
	if len(overnightSetupRunners) > 0 {
		selected = normalizeDreamRunnerList(overnightSetupRunners)
	}
	if len(selected) == 0 {
		for _, rt := range runtimes {
			if rt.Available && rt.Supported {
				selected = append(selected, rt.Name)
			}
		}
	}

	available := map[string]dreamRuntimeStatus{}
	for _, rt := range runtimes {
		available[rt.Name] = rt
	}
	filtered := make([]string, 0, len(selected))
	for _, name := range selected {
		rt, ok := available[name]
		if !ok {
			warnings = append(warnings, fmt.Sprintf("unknown Dream runner %q was ignored", name))
			continue
		}
		if !rt.Available {
			warnings = append(warnings, fmt.Sprintf("Dream runner %q is not installed on this machine", name))
			continue
		}
		if !rt.Supported {
			warnings = append(warnings, fmt.Sprintf("Dream runner %q is detected but not yet executable by Dream Council", name))
			continue
		}
		filtered = append(filtered, name)
	}
	return filtered, warnings
}

func resolveDreamSetupKeepAwake(existing config.DreamConfig, host dreamHostProfile) (bool, []string) {
	warnings := []string{}
	keepAwake := host.KeepAwakeSupported
	if existing.KeepAwake != nil {
		keepAwake = *existing.KeepAwake
	}
	if overnightSetupKeepAwake {
		keepAwake = true
	}
	if overnightSetupNoKeepAwake {
		keepAwake = false
	}
	if keepAwake && !host.KeepAwakeSupported {
		keepAwake = false
		warnings = append(warnings, "keep-awake was disabled because automatic sleep management is only supported on macOS with caffeinate")
	}
	return keepAwake, warnings
}

func resolveDreamSchedulerMode(existing config.DreamConfig, host dreamHostProfile) (string, []string, error) {
	mode := resolveDreamSchedulerModeBase(existing, host)
	mode = resolveDreamSchedulerModeFlagOverride(mode, host)

	if !isDreamSchedulerModeValid(mode) {
		return "", nil, fmt.Errorf("invalid scheduler mode %q: expected auto, manual, launchd, cron, systemd, or task-scheduler", mode)
	}
	warnings, err := resolveDreamSchedulerModeWarnings(mode, host)
	if err != nil {
		return "", nil, err
	}
	return mode, warnings, nil
}

func resolveDreamSchedulerModeBase(existing config.DreamConfig, host dreamHostProfile) string {
	mode := strings.TrimSpace(existing.SchedulerMode)
	if mode == "" {
		mode = host.RecommendedMode
	}
	if mode == "" {
		mode = "manual"
	}
	return mode
}

func resolveDreamSchedulerModeFlagOverride(mode string, host dreamHostProfile) string {
	if cmdMode := strings.TrimSpace(overnightSetupScheduler); cmdMode != "" && cmdMode != "auto" {
		return cmdMode
	}
	if overnightSetupScheduler == "auto" {
		if host.RecommendedMode != "" {
			return host.RecommendedMode
		}
		return "manual"
	}
	return mode
}

func isDreamSchedulerModeValid(mode string) bool {
	switch mode {
	case "manual", "launchd", "cron", "systemd", "task-scheduler":
		return true
	default:
		return false
	}
}

func resolveDreamSchedulerModeWarnings(mode string, host dreamHostProfile) ([]string, error) {
	switch mode {
	case "launchd":
		return dreamLaunchdSchedulerWarnings(host)
	case "systemd":
		return dreamSystemdSchedulerWarnings(host)
	case "cron":
		return dreamCronSchedulerWarnings(host)
	case "task-scheduler":
		return dreamTaskSchedulerWarnings()
	default:
		return nil, nil
	}
}

func dreamLaunchdSchedulerWarnings(host dreamHostProfile) ([]string, error) {
	if dreamOS != "darwin" {
		return nil, fmt.Errorf("launchd scheduling is only valid on macOS")
	}
	warnings := []string{}
	if host.HasBattery {
		warnings = append(warnings, "launchd is available, but laptop sleep or lid-close behavior can still suppress overnight runs")
	}
	return warnings, nil
}

func dreamSystemdSchedulerWarnings(host dreamHostProfile) ([]string, error) {
	if dreamOS != "linux" {
		return nil, fmt.Errorf("systemd scheduling is only valid on Linux")
	}
	if dreamLookPath("systemctl") != nil {
		return nil, fmt.Errorf("systemd scheduling requested, but systemctl is unavailable")
	}
	warnings := []string{}
	if host.HasBattery {
		warnings = append(warnings, "systemd timers are best-effort on battery-powered laptops that may sleep")
	}
	return warnings, nil
}

func dreamCronSchedulerWarnings(host dreamHostProfile) ([]string, error) {
	if dreamLookPath("crontab") != nil {
		return nil, fmt.Errorf("cron scheduling requested, but crontab is unavailable")
	}
	warnings := []string{}
	if host.HasBattery {
		warnings = append(warnings, "cron is best-effort on laptops that sleep")
	}
	return warnings, nil
}

func dreamTaskSchedulerWarnings() ([]string, error) {
	if dreamOS != "windows" {
		return nil, fmt.Errorf("task-scheduler scheduling is only valid on Windows")
	}
	return []string{"Windows Task Scheduler assistance will be generated for review; AgentOps will not register it automatically"}, nil
}

func maybeWriteDreamSchedulerArtifacts(cwd string, host dreamHostProfile, cfg config.DreamConfig) (map[string]string, []string, error) {
	if cfg.SchedulerMode == "" || cfg.SchedulerMode == "manual" {
		return nil, nil, nil
	}
	if strings.TrimSpace(cfg.ScheduleAt) == "" {
		return nil, []string{"scheduler mode was saved, but no schedule time was provided, so no host artifact was generated"}, nil
	}
	baseDir := filepath.Join(cwd, ".agentops", "generated", "dream")
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return nil, nil, fmt.Errorf("create dream generated dir: %w", err)
	}
	generated := map[string]string{}

	switch cfg.SchedulerMode {
	case "cron":
		path := filepath.Join(baseDir, "cron.txt")
		if err := os.WriteFile(path, []byte(renderDreamCronLine(cwd, cfg.ScheduleAt)), 0o644); err != nil {
			return nil, nil, fmt.Errorf("write cron artifact: %w", err)
		}
		generated["cron"] = path
	case "launchd":
		path := filepath.Join(baseDir, "com.agentops.dream.plist")
		if err := os.WriteFile(path, []byte(renderDreamLaunchdPlist(cwd, cfg.ScheduleAt)), 0o644); err != nil {
			return nil, nil, fmt.Errorf("write launchd artifact: %w", err)
		}
		generated["launchd"] = path
	case "systemd":
		servicePath := filepath.Join(baseDir, "ao-dream.service")
		timerPath := filepath.Join(baseDir, "ao-dream.timer")
		if err := os.WriteFile(servicePath, []byte(renderDreamSystemdService(cwd)), 0o644); err != nil {
			return nil, nil, fmt.Errorf("write systemd service: %w", err)
		}
		if err := os.WriteFile(timerPath, []byte(renderDreamSystemdTimer(cfg.ScheduleAt)), 0o644); err != nil {
			return nil, nil, fmt.Errorf("write systemd timer: %w", err)
		}
		generated["systemd_service"] = servicePath
		generated["systemd_timer"] = timerPath
	case "task-scheduler":
		path := filepath.Join(baseDir, "register-dream-task.ps1")
		if err := os.WriteFile(path, []byte(renderDreamWindowsTaskSchedulerScript(cwd, cfg.ScheduleAt)), 0o644); err != nil {
			return nil, nil, fmt.Errorf("write Windows Task Scheduler artifact: %w", err)
		}
		generated["task_scheduler"] = path
	}

	var warnings []string
	if host.HasBattery && cfg.SchedulerMode != "manual" {
		warnings = append(warnings, fmt.Sprintf("%s assistance was generated, but the host still has laptop sleep risk", cfg.SchedulerMode))
	}
	return generated, warnings, nil
}

func renderDreamCronLine(cwd, at string) string {
	hour, minute := splitDailyTime(at)
	return fmt.Sprintf("%s %s * * * cd %s && ao overnight start >> %s 2>&1\n",
		minute, hour, shellQuote(cwd), shellQuote(filepath.Join(cwd, ".agents", "overnight", "cron.log")))
}

func renderDreamLaunchdPlist(cwd, at string) string {
	hour, minute := splitDailyTime(at)
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>com.agentops.dream</string>
  <key>ProgramArguments</key>
  <array>
    <string>/bin/sh</string>
    <string>-lc</string>
    <string>cd %s && ao overnight start</string>
  </array>
  <key>StartCalendarInterval</key>
  <dict>
    <key>Hour</key>
    <integer>%s</integer>
    <key>Minute</key>
    <integer>%s</integer>
  </dict>
  <key>StandardOutPath</key>
  <string>%s</string>
  <key>StandardErrorPath</key>
  <string>%s</string>
</dict>
</plist>
`, xmlEscape(cwd), hour, minute, xmlEscape(filepath.Join(cwd, ".agents", "overnight", "launchd.log")), xmlEscape(filepath.Join(cwd, ".agents", "overnight", "launchd.err.log")))
}

func renderDreamSystemdService(cwd string) string {
	return fmt.Sprintf(`[Unit]
Description=AgentOps Dream overnight run

[Service]
Type=oneshot
WorkingDirectory=%s
ExecStart=/bin/sh -lc 'cd %s && ao overnight start'
`, shellQuote(cwd), shellQuote(cwd))
}

func renderDreamSystemdTimer(at string) string {
	hour, minute := splitDailyTime(at)
	return fmt.Sprintf(`[Unit]
Description=AgentOps Dream overnight timer

[Timer]
OnCalendar=*-*-* %s:%s:00
Persistent=true

[Install]
WantedBy=timers.target
`, hour, minute)
}

func renderDreamWindowsTaskSchedulerScript(cwd, at string) string {
	hour, minute := splitDailyTime(at)
	taskTime := fmt.Sprintf("%s:%s", hour, minute)
	logPath := filepath.Join(cwd, ".agents", "overnight", "task-scheduler.log")
	return fmt.Sprintf(`# Review before running. This script registers the local AgentOps Dream task.
$TaskName = "AgentOps Dream"
$WorkingDirectory = %q
$LogPath = %q
$Command = "Set-Location -LiteralPath '$WorkingDirectory'; ao overnight start *>> '$LogPath'"
$Action = New-ScheduledTaskAction -Execute "powershell.exe" -Argument "-NoProfile -ExecutionPolicy Bypass -Command $Command"
$Trigger = New-ScheduledTaskTrigger -Daily -At %q
$Settings = New-ScheduledTaskSettingsSet -StartWhenAvailable -AllowStartIfOnBatteries:$false -DisallowStartIfOnBatteries:$true
Register-ScheduledTask -TaskName $TaskName -Action $Action -Trigger $Trigger -Settings $Settings -Description "AgentOps Dream overnight knowledge compounding run"
`, cwd, logPath, taskTime)
}

func recommendedDreamSetupCommands(summary dreamSetupSummary) []string {
	cmds := []string{
		`ao overnight start`,
		`ao config --show --json`,
	}
	for _, path := range []string{summary.GeneratedFiles["launchd"], summary.GeneratedFiles["cron"], summary.GeneratedFiles["systemd_timer"], summary.GeneratedFiles["task_scheduler"]} {
		if path != "" {
			cmds = append(cmds, fmt.Sprintf(`cat %q`, path))
		}
	}
	return cmds
}

func deriveDreamSetupNextAction(summary dreamSetupSummary) string {
	if summary.DreamConfig.SchedulerMode != "" && summary.DreamConfig.SchedulerMode != "manual" && len(summary.GeneratedFiles) > 0 {
		return fmt.Sprintf("Review the generated %s assistance artifact, install it if the host semantics are acceptable, then run `ao overnight start` once manually to confirm the report shape.", summary.DreamConfig.SchedulerMode)
	}
	return "Run `ao overnight start` manually once, confirm the morning report is useful, and only then decide whether to automate it."
}

func outputDreamSetupSummary(summary dreamSetupSummary) error {
	switch GetOutput() {
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(summary)
	case "yaml":
		enc := yaml.NewEncoder(os.Stdout)
		if err := enc.Encode(summary); err != nil {
			_ = enc.Close()
			return err
		}
		return enc.Close()
	default:
		var b strings.Builder
		b.WriteString("# Dream Setup\n\n")
		fmt.Fprintf(&b, "- Status: `%s`\n", summary.Status)
		fmt.Fprintf(&b, "- Repo: `%s`\n", summary.RepoRoot)
		fmt.Fprintf(&b, "- Config: `%s`\n", summary.ConfigPath)
		fmt.Fprintf(&b, "- Host: `%s` / `%s`\n", summary.Host.OS, summary.Host.DeviceClass)
		fmt.Fprintf(&b, "- Recommended mode: `%s`\n", summary.Host.RecommendedMode)
		b.WriteString("\n## Local Curator\n\n")
		fmt.Fprintf(&b, "- engine: `%s`\n", summary.LocalCurator.Engine)
		fmt.Fprintf(&b, "- enabled: `%t`\n", summary.LocalCurator.Enabled)
		fmt.Fprintf(&b, "- available: `%t`\n", summary.LocalCurator.Available)
		if summary.LocalCurator.Model != "" {
			fmt.Fprintf(&b, "- model: `%s`\n", summary.LocalCurator.Model)
		}
		if summary.LocalCurator.WorkerDir != "" {
			fmt.Fprintf(&b, "- worker_dir: `%s`\n", summary.LocalCurator.WorkerDir)
		}
		b.WriteString("\n## Runtimes\n\n")
		for _, rt := range summary.Runtimes {
			fmt.Fprintf(&b, "- `%s`: available=%t supported=%t", rt.Name, rt.Available, rt.Supported)
			if rt.Note != "" {
				fmt.Fprintf(&b, " (%s)", rt.Note)
			}
			b.WriteString("\n")
		}
		b.WriteString("\n## Dream Config Preview\n\n")
		fmt.Fprintf(&b, "- runners: `%v`\n", summary.DreamConfig.Runners)
		fmt.Fprintf(&b, "- scheduler_mode: `%s`\n", summary.DreamConfig.SchedulerMode)
		if summary.DreamConfig.ScheduleAt != "" {
			fmt.Fprintf(&b, "- schedule_at: `%s`\n", summary.DreamConfig.ScheduleAt)
		}
		if summary.DreamConfig.KeepAwake != nil {
			fmt.Fprintf(&b, "- keep_awake: `%t`\n", *summary.DreamConfig.KeepAwake)
		}
		fmt.Fprintf(&b, "- consensus_policy: `%s`\n", summary.DreamConfig.ConsensusPolicy)
		if len(summary.GeneratedFiles) > 0 {
			b.WriteString("\n## Generated Files\n\n")
			keys := make([]string, 0, len(summary.GeneratedFiles))
			for key := range summary.GeneratedFiles {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range keys {
				fmt.Fprintf(&b, "- `%s`: `%s`\n", key, summary.GeneratedFiles[key])
			}
		}
		if len(summary.Warnings) > 0 {
			b.WriteString("\n## Warnings\n\n")
			for _, warning := range summary.Warnings {
				fmt.Fprintf(&b, "- %s\n", warning)
			}
		}
		b.WriteString("\n## Next Action\n\n")
		b.WriteString(summary.NextAction + "\n")
		fmt.Print(b.String())
		return nil
	}
}

var dreamLookPath = func(file string) error {
	_, err := exec.LookPath(file)
	return err
}

func detectBatteryPresence() bool {
	switch dreamOS {
	case "darwin":
		if dreamLookPath("pmset") != nil {
			return false
		}
		out, err := exec.Command("pmset", "-g", "batt").Output()
		if err != nil {
			return false
		}
		text := string(out)
		return strings.Contains(text, "InternalBattery") || strings.Contains(text, "Battery Power")
	case "linux":
		matches, _ := filepath.Glob("/sys/class/power_supply/BAT*")
		return len(matches) > 0
	default:
		return false
	}
}

func normalizeDreamRunnerList(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, raw := range values {
		for _, part := range strings.Split(raw, ",") {
			name := strings.ToLower(strings.TrimSpace(part))
			if name == "" || seen[name] {
				continue
			}
			seen[name] = true
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

func isValidDailyTime(value string) bool {
	parts := strings.Split(value, ":")
	if len(parts) != 2 {
		return false
	}
	hour, minute := splitDailyTime(value)
	if hour == "" || minute == "" {
		return false
	}
	return true
}

func splitDailyTime(value string) (string, string) {
	parts := strings.Split(strings.TrimSpace(value), ":")
	if len(parts) != 2 {
		return "", ""
	}
	hour := strings.TrimSpace(parts[0])
	minute := strings.TrimSpace(parts[1])
	if len(hour) != 2 || len(minute) != 2 {
		return "", ""
	}
	for _, r := range hour + minute {
		if r < '0' || r > '9' {
			return "", ""
		}
	}
	if hour > "23" || minute > "59" {
		return "", ""
	}
	return hour, minute
}

func xmlEscape(value string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
		"'", "&apos;",
	)
	return replacer.Replace(value)
}

func projectConfigPath() string {
	if cfg := strings.TrimSpace(os.Getenv("AGENTOPS_CONFIG")); cfg != "" {
		return cfg
	}
	cwd, err := os.Getwd()
	if err != nil {
		return ".agentops/config.yaml"
	}
	return filepath.Join(cwd, ".agentops", "config.yaml")
}

func dreamBoolPtr(v bool) *bool {
	return &v
}
