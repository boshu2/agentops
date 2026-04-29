package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/openclaw"
	"github.com/boshu2/agentops/cli/internal/quality"
	"github.com/boshu2/agentops/cli/internal/storage"
)

var (
	doctorJSON           bool
	doctorProductRuntime bool
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check AgentOps health",
	Long: `Run health checks on your AgentOps installation.

Validates that all required components are present and configured.
Optional components are reported as warnings but do not cause failure.

Examples:
  ao doctor
  ao doctor --json`,
	RunE: runDoctor,
}

func init() {
	doctorCmd.GroupID = "core"
	doctorCmd.Flags().BoolVar(&doctorJSON, "json", false, "Output results as JSON")
	doctorCmd.Flags().BoolVar(&doctorProductRuntime, "product-runtime", false, "Fail closed on daemon product runtime readiness checks")
	rootCmd.AddCommand(doctorCmd)
}

// Type aliases — canonical types live in internal/quality.
type doctorCheck = quality.Check
type doctorOutput = quality.DoctorOutput

// gatherDoctorChecks runs all doctor checks and returns the results.
func gatherDoctorChecks() []doctorCheck {
	return []doctorCheck{
		{Name: "ao CLI", Status: "pass", Detail: formatVersion(version), Required: true},
		checkCLIDependencies(),
		checkDaemonRuntime(),
		checkGasCityBridge(),
		checkOpenClawConsumer(),
		checkHookCoverage(),
		checkKnowledgeBase(),
		checkKnowledgeFreshness(),
		checkSearchIndex(),
		checkFlywheelHealth(),
		checkSkills(),
		checkCodexSync(),
		checkSkillIntegrity(),
		checkStaleReferences(),
		checkOptionalCLI("codex", "needed for --mixed council"),
	}
}

// Thin wrappers — delegate to quality package, kept for test compatibility.
func doctorStatusIcon(status string) string              { return quality.StatusIcon(status) }
func hasRequiredFailure(checks []doctorCheck) bool       { return quality.HasRequiredFailure(checks) }
func renderDoctorTable(w io.Writer, output doctorOutput) { quality.RenderTable(w, output) }
func newestFileModTime(entries []os.DirEntry) time.Time  { return quality.NewestFileModTime(entries) }
func countEstablished(dir string) int                    { return quality.CountEstablished(dir) }

func runDoctor(cmd *cobra.Command, args []string) error {
	checks := gatherDoctorChecks()
	if doctorProductRuntime {
		checks = gatherDoctorProductRuntimeChecks()
	}
	return quality.RunDoctor(quality.DoctorOptions{
		JSON:   doctorJSON,
		Checks: checks,
		Stdout: cmd.OutOrStdout(),
	})
}

func checkCLIDependencies() doctorCheck {
	return quality.CheckCLIDependencies(exec.LookPath)
}

func checkDaemonRuntime() doctorCheck {
	cwd, err := os.Getwd()
	if err != nil {
		return doctorCheck{Name: "Daemon Runtime", Status: "warn", Detail: "cannot determine working directory", Required: false}
	}
	baseURL, err := resolveDaemonURL(cwd, "")
	if err != nil {
		return doctorCheck{Name: "Daemon Runtime", Status: "warn", Detail: fmt.Sprintf("activation unavailable: %v", err), Required: false}
	}
	return checkDaemonRuntimeURL(baseURL)
}

func checkDaemonRuntimeURL(baseURL string) doctorCheck {
	baseURL = strings.TrimRight(baseURL, "/")
	ready, err := fetchDaemonReady(context.Background(), baseURL)
	if err != nil {
		return doctorCheck{Name: "Daemon Runtime", Status: "warn", Detail: fmt.Sprintf("readiness unavailable at %s: %v", baseURL, err), Required: false}
	}
	detail := fmt.Sprintf(
		"%s; replay=%s projection=%s events=%d last_event=%s",
		baseURL,
		ready.LedgerReplayStatus,
		ready.ProjectionStatus,
		ready.ProjectionLag.EventCount,
		doctorValueOrDash(ready.ProjectionLag.LastEventID),
	)
	if len(ready.DegradedReasons) > 0 {
		detail += "; degraded=" + strings.Join(ready.DegradedReasons, "; ")
	}
	if !ready.Ready {
		return doctorCheck{Name: "Daemon Runtime", Status: "warn", Detail: "not ready at " + detail, Required: false}
	}
	return doctorCheck{Name: "Daemon Runtime", Status: "pass", Detail: "ready at " + detail, Required: false}
}

func checkGasCityBridge() doctorCheck {
	cityPath := ""
	if cwd, err := os.Getwd(); err == nil {
		cityPath = gcBridgeCityPath(cwd)
	}
	return checkGasCityBridgeWith(cityPath, exec.Command, exec.LookPath)
}

func checkGasCityBridgeWith(cityPath string, execCommand gcExecFn, lookPath gcLookFn) doctorCheck {
	diag := gcBridgeDiagnose(cityPath, execCommand, lookPath, true)
	detail := formatGasCityDiagnostic(diag)
	if cityPath != "" {
		detail += "; city=" + cityPath
	}
	if diag.Ready {
		return doctorCheck{Name: "GasCity Bridge", Status: "pass", Detail: detail, Required: false}
	}
	return doctorCheck{Name: "GasCity Bridge", Status: "warn", Detail: detail, Required: false}
}

func gatherDoctorProductRuntimeChecks() []doctorCheck {
	return []doctorCheck{
		checkDaemonRuntime(),
		checkGasCityProductRuntime(),
		checkOpenClawConsumer(),
	}
}

func checkGasCityProductRuntime() doctorCheck {
	cityPath := ""
	if cwd, err := os.Getwd(); err == nil {
		cityPath = gcBridgeCityPath(cwd)
	}
	return checkGasCityProductRuntimeWith(cityPath, exec.Command, exec.LookPath)
}

func checkGasCityProductRuntimeWith(cityPath string, execCommand gcExecFn, lookPath gcLookFn) doctorCheck {
	diag := gcBridgeDiagnose(cityPath, execCommand, lookPath, false)
	detail := formatGasCityDiagnostic(diag)
	if cityPath != "" {
		detail += "; city=" + cityPath
	}
	if diag.APIReachable && diag.ReadinessReady && diag.Ready {
		return doctorCheck{Name: "GasCity Product Runtime", Status: "pass", Detail: detail, Required: true}
	}
	return doctorCheck{Name: "GasCity Product Runtime", Status: "fail", Detail: detail, Required: true}
}

func formatGasCityDiagnostic(diag gcBridgeDiagnostics) string {
	parts := []string{
		fmt.Sprintf("binary=%t", diag.BinaryAvailable),
		fmt.Sprintf("version=%s", doctorValueOrDash(diag.Version)),
		fmt.Sprintf("version_ok=%t", diag.VersionOK),
		fmt.Sprintf("api=%t", diag.APIReachable),
		fmt.Sprintf("ready=%t", diag.ReadinessReady),
	}
	if diag.FallbackEnabled {
		parts = append(parts, "fallback=enabled")
	}
	if diag.Reason != "" {
		parts = append(parts, "reason="+diag.Reason)
	}
	return strings.Join(parts, "; ")
}

func checkOpenClawConsumer() doctorCheck {
	cwd, err := os.Getwd()
	if err != nil {
		return doctorCheck{Name: "OpenClaw Consumer", Status: "warn", Detail: "cannot determine working directory", Required: false}
	}
	baseURL, err := resolveDaemonURL(cwd, "")
	if err != nil {
		return doctorCheck{Name: "OpenClaw Consumer", Status: "warn", Detail: fmt.Sprintf("daemon activation unavailable: %v", err), Required: false}
	}
	return checkOpenClawConsumerURL(baseURL)
}

func checkOpenClawConsumerURL(baseURL string) doctorCheck {
	baseURL = strings.TrimRight(baseURL, "/")
	var health openclaw.HealthResponse
	if err := fetchDaemonJSON(context.Background(), baseURL+"/openclaw/v1/health", &health); err != nil {
		return doctorCheck{Name: "OpenClaw Consumer", Status: "warn", Detail: fmt.Sprintf("health unavailable at %s: %v", baseURL, err), Required: false}
	}
	detail := fmt.Sprintf(
		"%s; status=%s snapshot=%s snapshot_status=%s runs=%d jobs=%d wiki=%d last_event=%s",
		baseURL,
		doctorValueOrDash(health.Status),
		doctorValueOrDash(health.SnapshotID),
		health.SnapshotStatus,
		health.ResourceCounts.Runs,
		health.ResourceCounts.Jobs,
		health.ResourceCounts.Wiki,
		doctorValueOrDash(health.Source.LastEventID),
	)
	if len(health.DegradedReasons) > 0 {
		detail += "; degraded=" + strings.Join(health.DegradedReasons, "; ")
	}
	if health.Status != "ok" || !health.Ready || health.SnapshotStatus != openclaw.SnapshotStatusCurrent {
		return doctorCheck{Name: "OpenClaw Consumer", Status: "warn", Detail: "not ready at " + detail, Required: false}
	}
	return doctorCheck{Name: "OpenClaw Consumer", Status: "pass", Detail: "ready at " + detail, Required: false}
}

func doctorValueOrDash(value string) string {
	if strings.TrimSpace(value) == "" {
		return "-"
	}
	return value
}

// checkHookCoverage checks if Claude hooks are installed with event coverage.
// Stays in cmd/ao because it depends on local AllEventNames / hookCoverageContract / hookGroupContainsAo.
func checkHookCoverage() doctorCheck {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return doctorCheck{Name: "Hook Coverage", Status: "fail", Detail: "cannot determine home directory", Required: true}
	}
	contract := resolveHookCoverageContract()

	// Prefer settings.json (active Claude configuration).
	settingsPath := filepath.Join(homeDir, ".claude", "settings.json")
	if data, err := os.ReadFile(settingsPath); err == nil {
		if hooksMap, ok := extractHooksMap(data); ok {
			return evaluateHookCoverageWithContract(hooksMap, contract)
		}
	}

	// Fallback: standalone hooks.json format.
	hooksPath := filepath.Join(homeDir, ".claude", "hooks.json")
	if data, err := os.ReadFile(hooksPath); err == nil {
		if hooksMap, ok := extractHooksMap(data); ok {
			return evaluateHookCoverageWithContract(hooksMap, contract)
		}
	}

	return doctorCheck{
		Name:     "Hook Coverage",
		Status:   "warn",
		Detail:   "No hooks found \u2014 run 'ao hooks install --force'" + hookCoverageFallbackDetail(contract.FallbackReason),
		Required: false,
	}
}

func evaluateHookCoverage(hooksMap map[string]any) doctorCheck {
	return evaluateHookCoverageWithContract(hooksMap, resolveHookCoverageContract())
}

func hookCoverageFallbackDetail(reason string) string {
	if reason == "" {
		return ""
	}
	return fmt.Sprintf(" (coverage contract fallback: %s)", reason)
}

func evaluateHookCoverageWithContract(hooksMap map[string]any, contract hookCoverageContract) doctorCheck {
	activeEvents := contract.ActiveEvents
	if len(activeEvents) == 0 {
		activeEvents = AllEventNames()
	}
	installedEvents := countInstalledEventsForList(hooksMap, activeEvents)
	fallbackSuffix := hookCoverageFallbackDetail(contract.FallbackReason)

	if installedEvents == 0 {
		return doctorCheck{
			Name:     "Hook Coverage",
			Status:   "warn",
			Detail:   "No hooks found \u2014 run 'ao hooks install --force'" + fallbackSuffix,
			Required: false,
		}
	}

	if !hookGroupContainsAo(hooksMap, "SessionStart") {
		return doctorCheck{
			Name:     "Hook Coverage",
			Status:   "warn",
			Detail:   "Non-ao hooks detected \u2014 run 'ao hooks install --force'" + fallbackSuffix,
			Required: false,
		}
	}

	if installedEvents < len(activeEvents) {
		return doctorCheck{
			Name:     "Hook Coverage",
			Status:   "warn",
			Detail:   fmt.Sprintf("Partial coverage: %d/%d events \u2014 run 'ao hooks install --force'%s", installedEvents, len(activeEvents), fallbackSuffix),
			Required: false,
		}
	}

	return doctorCheck{
		Name:     "Hook Coverage",
		Status:   "pass",
		Detail:   fmt.Sprintf("Full coverage: %d/%d events%s", installedEvents, len(activeEvents), fallbackSuffix),
		Required: false,
	}
}

func extractHooksMap(data []byte) (map[string]any, bool) {
	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		return nil, false
	}

	// settings.json shape
	if hooksRaw, ok := parsed["hooks"]; ok {
		if hooksMap, ok := hooksRaw.(map[string]any); ok {
			return hooksMap, true
		}
	}

	// hooks.json shape with top-level events
	for _, event := range AllEventNames() {
		if _, ok := parsed[event]; ok {
			return parsed, true
		}
	}

	return nil, false
}

func countHooksInMap(raw any) int {
	count := 0
	switch v := raw.(type) {
	case map[string]any:
		for _, val := range v {
			if arr, ok := val.([]any); ok {
				count += len(arr)
			} else {
				count += countHooksInMap(val)
			}
		}
	case []any:
		count += len(v)
	}
	return count
}

func countInstalledEvents(hooksMap map[string]any) int {
	installed := 0
	for _, event := range AllEventNames() {
		if groups, ok := hooksMap[event].([]any); ok && len(groups) > 0 {
			installed++
		}
	}
	return installed
}

func checkKnowledgeBase() doctorCheck {
	cwd, err := os.Getwd()
	if err != nil {
		return doctorCheck{Name: "Knowledge Base", Status: "fail", Detail: "cannot determine working directory", Required: true}
	}
	return quality.CheckKnowledgeBase(filepath.Join(cwd, storage.DefaultBaseDir))
}

func checkKnowledgeFreshness() doctorCheck {
	cwd, err := os.Getwd()
	if err != nil {
		return doctorCheck{Name: "Knowledge Freshness", Status: "warn", Detail: "cannot determine working directory", Required: false}
	}
	return quality.CheckKnowledgeFreshness(filepath.Join(cwd, storage.DefaultBaseDir, "sessions"))
}

// Thin wrappers for pure functions — delegate to quality package.
func formatVersion(v string) string         { return quality.FormatVersion(v) }
func formatDuration(d time.Duration) string { return quality.FormatDuration(d) }
func formatNumber(n int) string             { return quality.FormatNumber(n) }
func countFileLines(path string) int        { return quality.CountFileLines(path) }

func checkSearchIndex() doctorCheck {
	cwd, err := os.Getwd()
	if err != nil {
		return doctorCheck{Name: "Search Index", Status: "warn", Detail: "cannot determine working directory", Required: false}
	}
	return quality.CheckSearchIndex(filepath.Join(cwd, IndexDir, IndexFileName))
}

func checkFlywheelHealth(baseDir ...string) doctorCheck {
	dir := ""
	if len(baseDir) > 0 && baseDir[0] != "" {
		dir = baseDir[0]
	} else {
		var err error
		dir, err = os.Getwd()
		if err != nil {
			return doctorCheck{Name: "Flywheel Health", Status: "warn", Detail: "cannot determine working directory", Required: false}
		}
	}
	return quality.CheckFlywheelHealth(filepath.Join(dir, storage.DefaultBaseDir))
}

// Thin wrappers — delegate Codex/skill checks to internal/quality.
func checkSkills() doctorCheck         { return quality.CheckSkills() }
func checkCodexSync() doctorCheck      { return quality.CheckCodexSync() }
func checkSkillIntegrity() doctorCheck { return quality.CheckSkillIntegrity() }
func checkOptionalCLI(name, reason string) doctorCheck {
	return quality.CheckOptionalCLI(name, reason)
}
func findHealScript() string                 { return quality.FindHealScript() }
func sha256File(path string) (string, error) { return quality.SHA256File(path) }

// fileExists is used by other cmd/ao files (codex.go, codex_runtime.go).
func fileExists(path string) bool { return quality.FileExists(path) }

// Test-compatibility wrappers for skill/codex helpers.
func skillOverlapWarning(base map[string]struct{}, primaryCount int, primary, msgFmt string, others ...map[string]struct{}) *doctorCheck {
	return quality.SkillOverlapWarning(base, primaryCount, primary, msgFmt, others...)
}
func scanSkillDir(dir string) map[string]struct{} { return quality.ScanSkillDir(dir) }
func overlappingSkillNames(base map[string]struct{}, others ...map[string]struct{}) []string {
	return quality.OverlappingSkillNames(base, others...)
}

// Type aliases for stale reference types.
var deprecatedCommands = quality.DeprecatedCommands

type staleReference = quality.StaleReference

func checkStaleReferences() doctorCheck {
	return quality.CheckStaleReferences([]string{
		"hooks/*.sh",
		"skills/*/SKILL.md",
		"skills/*/references/*.md",
		"skills-codex/*/SKILL.md",
		"skills-codex-overrides/*/SKILL.md",
		"docs/*.md",
		"scripts/*.sh",
		"hooks/examples/*.sh",
		"cli/embedded/hooks/*.sh",
		"docs/contracts/*.md",
		"docs/plans/*.md",
	})
}

// Thin wrappers for test compatibility.
func scanFileForDeprecatedCommands(path string) []staleReference {
	return quality.ScanFileForDeprecatedCommands(path)
}
func countUniqueFiles(refs []staleReference) int { return quality.CountUniqueFiles(refs) }

func countHealFindings(output string) int { return quality.CountHealFindings(output) }

func countFiles(dir string) int         { return quality.CountFiles(dir) }
func countLearningFiles(dir string) int { return quality.CountLearningFiles(dir) }
func countCheckStatuses(checks []doctorCheck) (int, int, int) {
	return quality.CountCheckStatuses(checks)
}
func buildDoctorSummary(passes, fails, warns, total int) string {
	return quality.BuildSummary(passes, fails, warns, total)
}
func computeResult(checks []doctorCheck) doctorOutput { return quality.ComputeResult(checks) }
