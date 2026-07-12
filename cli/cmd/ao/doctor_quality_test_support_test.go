package main

import (
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/boshu2/agentops/cli/internal/quality"
)

type doctorCheck = quality.Check
type doctorOutput = quality.DoctorOutput

func doctorStatusIcon(status string) string        { return quality.StatusIcon(status) }
func hasRequiredFailure(checks []doctorCheck) bool { return quality.HasRequiredFailure(checks) }
func countCheckStatuses(checks []doctorCheck) (int, int, int) {
	return quality.CountCheckStatuses(checks)
}
func buildDoctorSummary(passes, fails, warns, total int) string {
	return quality.BuildSummary(passes, fails, warns, total)
}
func computeResult(checks []doctorCheck) doctorOutput         { return quality.ComputeResult(checks) }
func renderDoctorTable(writer io.Writer, output doctorOutput) { quality.RenderTable(writer, output) }
func formatNumber(value int) string                           { return quality.FormatNumber(value) }
func formatDuration(value time.Duration) string               { return quality.FormatDuration(value) }
func countFileLines(path string) int                          { return quality.CountFileLines(path) }
func countHealFindings(output string) int                     { return quality.CountHealFindings(output) }
func newestFileModTime(entries []os.DirEntry) time.Time       { return quality.NewestFileModTime(entries) }
func countFiles(path string) int                              { return quality.CountFiles(path) }
func countLearningFiles(path string) int                      { return quality.CountLearningFiles(path) }
func countEstablished(path string) int                        { return quality.CountEstablished(path) }
func fileExists(path string) bool                             { return quality.FileExists(path) }
func checkFlywheelHealth(root string) doctorCheck {
	return quality.CheckFlywheelHealth(filepath.Join(root, ".agents"))
}
