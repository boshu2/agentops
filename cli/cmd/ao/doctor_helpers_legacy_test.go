package main

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/boshu2/agentops/cli/internal/quality"
	"github.com/boshu2/agentops/cli/internal/storage"
)

type doctorCheck = quality.Check
type doctorOutput = quality.DoctorOutput
type staleReference = quality.StaleReference

var deprecatedCommands = quality.DeprecatedCommands

func doctorStatusIcon(status string) string                   { return quality.StatusIcon(status) }
func hasRequiredFailure(checks []doctorCheck) bool            { return quality.HasRequiredFailure(checks) }
func renderDoctorTable(writer io.Writer, output doctorOutput) { quality.RenderTable(writer, output) }
func newestFileModTime(entries []os.DirEntry) time.Time       { return quality.NewestFileModTime(entries) }
func countEstablished(directory string) int                   { return quality.CountEstablished(directory) }
func formatVersion(value string) string                       { return quality.FormatVersion(value) }
func formatDuration(value time.Duration) string               { return quality.FormatDuration(value) }
func formatNumber(value int) string                           { return quality.FormatNumber(value) }
func countFileLines(path string) int                          { return quality.CountFileLines(path) }
func checkCLIDependencies() doctorCheck                       { return quality.CheckCLIDependencies(exec.LookPath) }
func checkKnowledgeBase() doctorCheck {
	cwd, _ := os.Getwd()
	return quality.CheckKnowledgeBase(filepath.Join(cwd, storage.DefaultBaseDir))
}
func checkKnowledgeFreshness() doctorCheck {
	cwd, _ := os.Getwd()
	return quality.CheckKnowledgeFreshness(filepath.Join(cwd, storage.DefaultBaseDir, "sessions"))
}
func checkSearchIndex() doctorCheck {
	cwd, _ := os.Getwd()
	return quality.CheckSearchIndex(filepath.Join(cwd, IndexDir, IndexFileName))
}
func checkFlywheelHealth(base ...string) doctorCheck {
	cwd, _ := os.Getwd()
	if len(base) > 0 && base[0] != "" {
		cwd = base[0]
	}
	return quality.CheckFlywheelHealth(filepath.Join(cwd, storage.DefaultBaseDir))
}
func checkSkills() doctorCheck                         { return quality.CheckSkills() }
func checkCodexSync() doctorCheck                      { return quality.CheckCodexSync() }
func checkSkillIntegrity() doctorCheck                 { return quality.CheckSkillIntegrity() }
func checkOptionalCLI(name, reason string) doctorCheck { return quality.CheckOptionalCLI(name, reason) }
func findHealScript() string                           { return quality.FindHealScript() }
func sha256File(path string) (string, error)           { return quality.SHA256File(path) }
func fileExists(path string) bool                      { return quality.FileExists(path) }
func skillOverlapWarning(base map[string]struct{}, count int, primary, format string, others ...map[string]struct{}) *doctorCheck {
	return quality.SkillOverlapWarning(base, count, primary, format, others...)
}
func scanSkillDir(path string) map[string]struct{} { return quality.ScanSkillDir(path) }
func overlappingSkillNames(base map[string]struct{}, others ...map[string]struct{}) []string {
	return quality.OverlappingSkillNames(base, others...)
}
func checkStaleReferences() doctorCheck {
	return quality.CheckStaleReferences([]string{"skills/*/SKILL.md", "skills/*/references/*.md", "skills-codex/*/SKILL.md", "skills-codex-overrides/*/SKILL.md", "docs/*.md", "scripts/*.sh", "docs/contracts/*.md", "docs/plans/*.md"})
}
func scanFileForDeprecatedCommands(path string) []staleReference {
	return quality.ScanFileForDeprecatedCommands(path)
}
func countUniqueFiles(refs []staleReference) int { return quality.CountUniqueFiles(refs) }
func countHealFindings(output string) int        { return quality.CountHealFindings(output) }
func countFiles(path string) int                 { return quality.CountFiles(path) }
func countLearningFiles(path string) int         { return quality.CountLearningFiles(path) }
func countCheckStatuses(checks []doctorCheck) (int, int, int) {
	return quality.CountCheckStatuses(checks)
}
func buildDoctorSummary(passes, fails, warns, total int) string {
	return quality.BuildSummary(passes, fails, warns, total)
}
func computeResult(checks []doctorCheck) doctorOutput { return quality.ComputeResult(checks) }
