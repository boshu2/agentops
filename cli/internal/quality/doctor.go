// Package quality provides doctor health checks, metrics collection, and badge generation.
package quality

import (
	"bufio"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Check status values. "info" is a non-warning, non-failing informational line
// (e.g. "you haven't run a session yet", "optional dependency absent") — it is
// rendered but never counted as a warning and never degrades the result.
const (
	StatusPass = "pass"
	StatusWarn = "warn"
	StatusFail = "fail"
	StatusInfo = "info"
)

// Check audiences. Every doctor check declares who it is for:
//   - AudienceInstalledUser: relevant to anyone who installed AgentOps.
//   - AudienceRepoDev: only meaningful inside an agentops repo clone (skill
//     hygiene, codex-sync internals, plugin-manifest internals, stale in-repo
//     references, binary freshness). Collapsed to a single info line outside a
//     clone so a pristine install never sees repo-internal warnings.
const (
	AudienceInstalledUser = "installed-user"
	AudienceRepoDev       = "repo-dev"
)

// Check represents a single doctor health check result.
type Check struct {
	Name     string `json:"name"`
	Status   string `json:"status"` // "pass", "warn", "fail", "info"
	Detail   string `json:"detail"`
	Required bool   `json:"required"`
	// Audience is who the check is for: "installed-user" or "repo-dev".
	Audience string `json:"audience,omitempty"`
	// Fix, when set, is a remediation runnable from the reader's own context
	// (a URL or an ao/br/brew/npm/curl command) — never a repo-relative script
	// path. Populated for installed-user checks that have a next action.
	Fix string `json:"fix,omitempty"`
}

// DoctorOutput holds the full doctor report.
type DoctorOutput struct {
	Checks  []Check `json:"checks"`
	Result  string  `json:"result"` // "HEALTHY", "DEGRADED", "UNHEALTHY"
	Summary string  `json:"summary"`
}

// DoctorOptions configures the doctor command.
type DoctorOptions struct {
	JSON   bool
	Checks []Check
	Stdout io.Writer
}

// RunDoctor computes results from checks and renders output.
func RunDoctor(opts DoctorOptions) error {
	if opts.Stdout == nil {
		opts.Stdout = os.Stdout
	}
	output := ComputeResult(opts.Checks)
	if opts.JSON {
		data, err := json.MarshalIndent(output, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal doctor output: %w", err)
		}
		fmt.Fprintln(opts.Stdout, string(data))
		return nil
	}
	RenderTable(opts.Stdout, output)
	if HasRequiredFailure(output.Checks) {
		return fmt.Errorf("doctor failed: one or more required checks did not pass")
	}
	return nil
}

// StatusIcon returns the display icon for a check status.
func StatusIcon(status string) string {
	switch status {
	case "pass":
		return "\u2713"
	case "warn":
		return "!"
	case "fail":
		return "\u2717"
	case "info":
		return "\u00b7"
	}
	return "?"
}

// RenderTable writes the formatted doctor output table.
func RenderTable(w io.Writer, output DoctorOutput) {
	fmt.Fprintln(w, "ao doctor")
	fmt.Fprintln(w, "\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500")
	maxName := 0
	for _, c := range output.Checks {
		if len(c.Name) > maxName {
			maxName = len(c.Name)
		}
	}
	for _, c := range output.Checks {
		padding := strings.Repeat(" ", maxName-len(c.Name))
		fmt.Fprintf(w, "%s %s%s  %s\n", StatusIcon(c.Status), c.Name, padding, c.Detail)
	}
	fmt.Fprintln(w)
	fmt.Fprintf(w, "%s\n", output.Summary)
}

// HasRequiredFailure returns true if any required check has failed.
func HasRequiredFailure(checks []Check) bool {
	for _, c := range checks {
		if c.Required && c.Status == "fail" {
			return true
		}
	}
	return false
}

// ComputeResult determines the overall doctor result from checks.
func ComputeResult(checks []Check) DoctorOutput {
	passes, fails, warns := CountCheckStatuses(checks)
	// "info" lines are informational only: they are not counted toward the
	// pass/total tally and never degrade the result.
	total := passes + fails + warns
	result := "HEALTHY"
	if fails > 0 {
		result = "UNHEALTHY"
	} else if warns > 0 {
		result = "DEGRADED"
	}
	return DoctorOutput{Checks: checks, Result: result, Summary: BuildSummary(passes, fails, warns, total)}
}

// CountCheckStatuses tallies pass, fail, and warn counts from checks.
func CountCheckStatuses(checks []Check) (passes, fails, warns int) {
	for _, c := range checks {
		switch c.Status {
		case "pass":
			passes++
		case "fail":
			fails++
		case "warn":
			warns++
		}
	}
	return
}

// BuildSummary constructs a human-readable summary from check tallies.
func BuildSummary(passes, fails, warns, total int) string {
	switch {
	case fails == 0 && warns == 0:
		return fmt.Sprintf("%d/%d checks passed", passes, total)
	case fails == 0:
		s := fmt.Sprintf("%d/%d checks passed, %d warning", passes, total, warns)
		if warns > 1 {
			s += "s"
		}
		return s
	default:
		parts := []string{fmt.Sprintf("%d/%d checks passed", passes, total)}
		if warns > 0 {
			w := fmt.Sprintf("%d warning", warns)
			if warns > 1 {
				w += "s"
			}
			parts = append(parts, w)
		}
		if fails > 0 {
			parts = append(parts, fmt.Sprintf("%d failed", fails))
		}
		return strings.Join(parts, ", ")
	}
}

// FormatVersion ensures the version string has exactly one "v" prefix.
func FormatVersion(v string) string {
	if strings.HasPrefix(v, "v") {
		return v
	}
	return "v" + v
}

// FormatDuration produces a human-readable duration string.
func FormatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}

// pluralize renders "<n> <word>" with a trailing "s" unless n == 1, so the
// doctor never prints "1 learnings".
func pluralize(n int, word string) string {
	if n == 1 {
		return fmt.Sprintf("1 %s", word)
	}
	return fmt.Sprintf("%d %ss", n, word)
}

// FormatNumber adds comma separators to an integer.
func FormatNumber(n int) string {
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}
	var result []byte
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			result = append(result, ',')
		}
		result = append(result, byte(c))
	}
	return string(result)
}

// CountFileLines counts non-empty lines in a file.
func CountFileLines(path string) int {
	f, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer func() { _ = f.Close() }()
	count := 0
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 256*1024), 1024*1024)
	for scanner.Scan() {
		if len(strings.TrimSpace(scanner.Text())) > 0 {
			count++
		}
	}
	return count
}

// CountFiles counts regular (non-directory) files in a directory.
func CountFiles(dir string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	count := 0
	for _, e := range entries {
		if !e.IsDir() {
			count++
		}
	}
	return count
}

// NewestFileModTime returns the most recent modification time among regular files.
func NewestFileModTime(entries []os.DirEntry) time.Time {
	var newest time.Time
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().After(newest) {
			newest = info.ModTime()
		}
	}
	return newest
}

// CountLearningFiles counts .md and .jsonl files in a directory.
func CountLearningFiles(dir string) int {
	mdFiles, _ := filepath.Glob(filepath.Join(dir, "*.md"))
	jsonlFiles, _ := filepath.Glob(filepath.Join(dir, "*.jsonl"))
	return len(mdFiles) + len(jsonlFiles)
}

// CountEstablished counts files whose name contains "established" or "promoted".
func CountEstablished(dir string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	count := 0
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		lower := strings.ToLower(e.Name())
		if strings.Contains(lower, "established") || strings.Contains(lower, "promoted") {
			count++
		}
	}
	return count
}

// SHA256File computes the SHA-256 hash of a file.
func SHA256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// CheckKnowledgeBase checks that the knowledge base directory exists.
func CheckKnowledgeBase(baseDir string) Check {
	if _, err := os.Stat(baseDir); os.IsNotExist(err) {
		return Check{Name: "Knowledge Base", Status: "fail", Detail: ".agents/ao not initialized", Required: true, Audience: AudienceInstalledUser, Fix: "ao init"}
	}
	return Check{Name: "Knowledge Base", Status: "pass", Detail: ".agents/ao initialized", Required: true, Audience: AudienceInstalledUser}
}

// CheckKnowledgeFreshness checks the most recent file in the sessions directory.
func CheckKnowledgeFreshness(sessionsDir string) Check {
	noSessions := Check{Name: "Knowledge Freshness", Status: "info", Detail: "No sessions yet \u2014 run 'ao forge transcript' after your next session", Required: false, Audience: AudienceInstalledUser, Fix: "ao forge transcript"}
	entries, err := os.ReadDir(sessionsDir)
	if err != nil || len(entries) == 0 {
		return noSessions
	}
	newest := NewestFileModTime(entries)
	if newest.IsZero() {
		return noSessions
	}
	age := time.Since(newest)
	if age > 14*24*time.Hour {
		return Check{Name: "Knowledge Freshness", Status: "warn", Detail: fmt.Sprintf("Last session: %s ago \u2014 knowledge may be stale", FormatDuration(age)), Required: false, Audience: AudienceInstalledUser, Fix: "ao forge transcript"}
	}
	return Check{Name: "Knowledge Freshness", Status: "pass", Detail: fmt.Sprintf("Last session: %s ago", FormatDuration(age)), Required: false, Audience: AudienceInstalledUser}
}

// CheckSearchIndex checks if the search index exists and counts terms.
func CheckSearchIndex(indexPath string) Check {
	info, err := os.Stat(indexPath)
	if err != nil {
		return Check{Name: "Search Index", Status: "info", Detail: "No search index yet \u2014 run 'ao store rebuild' for faster searches", Required: false, Audience: AudienceInstalledUser, Fix: "ao store rebuild"}
	}
	if info.Size() == 0 {
		return Check{Name: "Search Index", Status: "info", Detail: "Search index is empty \u2014 run 'ao store rebuild'", Required: false, Audience: AudienceInstalledUser, Fix: "ao store rebuild"}
	}
	return Check{Name: "Search Index", Status: "pass", Detail: fmt.Sprintf("Index exists (%s terms)", FormatNumber(CountFileLines(indexPath))), Required: false, Audience: AudienceInstalledUser}
}

// CheckFlywheelHealth checks if the flywheel has learnings.
func CheckFlywheelHealth(baseDir string) Check {
	learningsDir := filepath.Join(baseDir, "learnings")
	total := CountLearningFiles(learningsDir)
	if total == 0 {
		altDir := filepath.Join(filepath.Dir(baseDir), "learnings")
		total = CountLearningFiles(altDir)
	}
	if total == 0 {
		return Check{Name: "Flywheel Health", Status: "info", Detail: "No learnings yet \u2014 the flywheel starts after your first sessions", Required: false, Audience: AudienceInstalledUser}
	}
	established := CountEstablished(filepath.Join(baseDir, "learnings"))
	if established == 0 {
		established = CountEstablished(filepath.Join(filepath.Dir(baseDir), "learnings"))
	}
	detail := fmt.Sprintf("%s in flywheel", pluralize(total, "learning"))
	if established > 0 {
		detail = fmt.Sprintf("%s (%d established)", pluralize(total, "learning"), established)
	}
	return Check{Name: "Flywheel Health", Status: "pass", Detail: detail, Required: false, Audience: AudienceInstalledUser}
}

// CheckCLIDependencies verifies gt and bd are available in PATH.
func CheckCLIDependencies(lookPath func(string) (string, error)) Check {
	gtOk := lookPath != nil
	bdOk := lookPath != nil
	if gtOk {
		if _, err := lookPath("gt"); err != nil {
			gtOk = false
		}
	}
	if bdOk {
		if _, err := lookPath("bd"); err != nil {
			bdOk = false
		}
	}
	if gtOk && bdOk {
		return Check{Name: "CLI Dependencies", Status: "pass", Detail: "gt and bd available", Required: false, Audience: AudienceInstalledUser}
	}
	// gt and bd are OPTIONAL: gt enables Gas City substrate ops, bd enables the
	// beads/dolt substrate store. Their absence is informational, never a
	// warning, and never "required".
	var missing, enables, brewPkgs []string
	if !gtOk {
		missing = append(missing, "gt")
		enables = append(enables, "gt enables Gas City substrate ops")
		brewPkgs = append(brewPkgs, "gastown")
	}
	if !bdOk {
		missing = append(missing, "bd")
		enables = append(enables, "bd enables the beads substrate store")
		brewPkgs = append(brewPkgs, "beads")
	}
	detail := fmt.Sprintf("%s not found (optional \u2014 %s)", strings.Join(missing, ", "), strings.Join(enables, "; "))
	fix := ""
	if runtime.GOOS != "windows" {
		fix = "brew install " + strings.Join(brewPkgs, " ")
	} else {
		detail += " \u2014 install from their Windows releases or use WSL/Homebrew"
	}
	return Check{Name: "CLI Dependencies", Status: "info", Detail: detail, Required: false, Audience: AudienceInstalledUser, Fix: fix}
}
