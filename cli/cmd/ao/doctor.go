// practices: [sre, resilience-patterns]
package main

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/doctor"
	"github.com/boshu2/agentops/cli/internal/quality"
	"github.com/boshu2/agentops/cli/internal/storage"
)

var (
	doctorJSON bool
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
	// Attach the diagnose-and-repair engine surface: additive flags
	// (--fix, --dry-run, --only, ...) plus subcommands (fix, undo, explain,
	// capabilities, health, robot-docs, gc, ls, diff). The legacy 16-check
	// behavior is preserved when no engine flag/subcommand is used.
	registerDoctorSurface()
	rootCmd.AddCommand(doctorCmd)
}

// Type aliases — canonical types live in internal/quality.
type doctorCheck = quality.Check
type doctorOutput = quality.DoctorOutput

// gatherDoctorChecks runs all doctor checks and returns the results.
func gatherDoctorChecks() []doctorCheck {
	checks := []doctorCheck{
		{Name: "ao CLI", Status: "pass", Detail: formatVersion(version), Required: true},
		checkCLIDependencies(),
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
	// Wedge/verification preflight (reviewer reachability, cross-family
	// capability, binary freshness, ledger health, LAW-0 guard) — additive.
	return append(checks, wedgeDoctorChecks()...)
}

// Thin wrappers — delegate to quality package, kept for test compatibility.
func doctorStatusIcon(status string) string              { return quality.StatusIcon(status) }
func hasRequiredFailure(checks []doctorCheck) bool       { return quality.HasRequiredFailure(checks) }
func renderDoctorTable(w io.Writer, output doctorOutput) { quality.RenderTable(w, output) }
func newestFileModTime(entries []os.DirEntry) time.Time  { return quality.NewestFileModTime(entries) }
func countEstablished(dir string) int                    { return quality.CountEstablished(dir) }

func runDoctor(cmd *cobra.Command, args []string) error {
	// Engine-flag invocations (--fix, --explain, --robot-triage) route entirely
	// through the diagnose-and-repair engine.
	if doctorFix || doctorExplainFlag != "" || doctorRobotTriage {
		return runDoctorEngineDefault(cmd)
	}

	// `ao doctor --json` is the engine's machine surface: a single diagnose
	// Report (schema_version, exit_code, findings).
	if doctorWantsJSON() {
		return runDoctorEngineDefault(cmd)
	}

	// Human-readable form: the legacy check table, with engine findings
	// appended additively so existing output never regresses.
	checks := gatherDoctorChecks()
	if err := quality.RunDoctor(quality.DoctorOptions{
		JSON:   doctorJSON,
		Checks: checks,
		Stdout: cmd.OutOrStdout(),
	}); err != nil {
		return err
	}
	// Additive: also run the registered failure-mode detectors and surface
	// their findings without disturbing the legacy `checks` output above. With
	// the FOUNDATION wave's empty registry this is a no-op; later waves light up.
	return appendEngineFindings(cmd)
}

// appendEngineFindings runs the doctor engine's detectors and appends their
// findings to the human-readable `ao doctor` output. It never alters the legacy
// `checks` output or the bare command's exit semantics — a doctorExitError is
// only returned when engine findings exist (mapped to exit 1).
func appendEngineFindings(cmd *cobra.Command) error {
	// With no detectors registered there is nothing to add and no reason to
	// create a run directory; skip entirely so the legacy command stays
	// side-effect-free.
	if len(doctor.Detectors()) == 0 {
		return nil
	}
	// JSON callers receive the engine Report from the dedicated --json path in
	// runDoctor; never emit a second JSON document here.
	if doctorWantsJSON() {
		return nil
	}
	opts, err := doctorEngineOptions()
	if err != nil {
		return nil // never let the engine break the legacy command
	}
	rep, derr := doctor.Diagnose(opts)
	if derr != nil || rep == nil || len(rep.Findings) == 0 {
		return nil
	}
	renderEngineFindings(cmd, rep)
	return exitErr(rep.ExitCode, "doctor findings present")
}

func checkCLIDependencies() doctorCheck {
	return quality.CheckCLIDependencies(exec.LookPath)
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

// fileExists is used by other cmd/ao files.
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
		"skills/*/SKILL.md",
		"skills/*/references/*.md",
		"skills-codex/*/SKILL.md",
		"skills-codex-overrides/*/SKILL.md",
		"docs/*.md",
		"scripts/*.sh",
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
