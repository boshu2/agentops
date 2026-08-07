// Package quality provides doctor health checks, metrics collection, and badge generation.
package quality

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
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
