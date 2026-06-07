package gates

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/boshu2/agentops/cli/internal/ports"
)

// Report is the outcome of an orchestrator run. It serves all three consumers:
// a human renderer (cockpit), JSON (refinery), and an exit code (hook/CI).
type Report struct {
	Mode         Tier
	Scope        Scope
	ChangedCount int
	StartedAt    time.Time
	Elapsed      time.Duration
	Results      []CheckResult
}

// ExitCode is 1 if any blocking check FAILed, else 0. WARN/SKIP and
// non-blocking FAIL are advisory.
func (r *Report) ExitCode() int {
	for _, res := range r.Results {
		if isBlockingFail(res.Check, res.Verdict) {
			return 1
		}
	}
	return 0
}

// Summary tallies verdicts by status.
func (r *Report) Summary() Summary {
	var s Summary
	s.Total = len(r.Results)
	for _, res := range r.Results {
		switch res.Verdict.Status {
		case ports.GateStatusPass:
			s.Passed++
		case ports.GateStatusWarn:
			s.Warned++
		case ports.GateStatusFail:
			s.Failed++
		case ports.GateStatusSkip:
			s.Skipped++
		default:
			s.Unknown++
		}
	}
	return s
}

// Summary is the per-status tally of a run.
type Summary struct {
	Total   int `json:"total"`
	Passed  int `json:"passed"`
	Warned  int `json:"warned"`
	Failed  int `json:"failed"`
	Skipped int `json:"skipped"`
	Unknown int `json:"unknown"`
}

// ---- JSON wire format (the contract the refinery + CI consume) ----

type jsonReport struct {
	Run   jsonRun    `json:"run"`
	Gates []jsonGate `json:"gates"`
}

type jsonRun struct {
	Mode              string  `json:"mode"`
	Scope             string  `json:"scope"`
	StartedAt         string  `json:"started_at"`
	ElapsedMs         int64   `json:"elapsed_ms"`
	ChangedFilesCount int     `json:"changed_files_count"`
	Summary           Summary `json:"summary"`
}

type jsonGate struct {
	Name       string `json:"name"`
	Tier       string `json:"tier"`
	Status     string `json:"status"`
	Blocking   bool   `json:"blocking"`
	Reason     string `json:"reason"`
	LogTail    string `json:"log_tail,omitempty"`
	DurationMs int64  `json:"duration_ms"`
}

// JSON renders the report as the wire contract.
func (r *Report) JSON() ([]byte, error) {
	jr := jsonReport{
		Run: jsonRun{
			Mode:              modeString(r.Mode),
			Scope:             string(r.Scope),
			StartedAt:         r.StartedAt.UTC().Format(time.RFC3339),
			ElapsedMs:         r.Elapsed.Milliseconds(),
			ChangedFilesCount: r.ChangedCount,
			Summary:           r.Summary(),
		},
		Gates: make([]jsonGate, 0, len(r.Results)),
	}
	for _, res := range r.Results {
		reason := res.Verdict.Reason
		if res.Err != nil {
			reason = "evaluation error: " + res.Err.Error()
		}
		jr.Gates = append(jr.Gates, jsonGate{
			Name:       res.Check.ID,
			Tier:       tierString(res.Check.Tiers),
			Status:     string(res.Verdict.Status),
			Blocking:   res.Check.Blocking,
			Reason:     reason,
			LogTail:    res.Verdict.LogTail,
			DurationMs: res.Duration.Milliseconds(),
		})
	}
	out, err := json.MarshalIndent(jr, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("gates: marshal report: %w", err)
	}
	return out, nil
}

// Human writes a concise human-readable report to w.
func (r *Report) Human(w io.Writer) {
	s := r.Summary()
	for _, res := range r.Results {
		mark := string(res.Verdict.Status)
		fmt.Fprintf(w, "%-5s %s\n", mark, res.Check.ID)
	}
	fmt.Fprintf(w, "\n%s/%s: %d checks — %d pass, %d warn, %d fail, %d skip (%dms)\n",
		modeString(r.Mode), r.Scope, s.Total, s.Passed, s.Warned, s.Failed, s.Skipped, r.Elapsed.Milliseconds())
}

// GitHubAnnotations writes GitHub Actions log annotations for failing or
// advisory checks. Human/JSON output remains the primary report; annotations
// preserve per-check CI ergonomics when validate.yml delegates to ao gate check.
func (r *Report) GitHubAnnotations(w io.Writer) {
	for _, res := range r.Results {
		level := ""
		switch res.Verdict.Status {
		case ports.GateStatusFail:
			if res.Check.Blocking {
				level = "error"
			} else {
				level = "warning"
			}
		case ports.GateStatusWarn:
			level = "warning"
		default:
			continue
		}
		msg := res.Verdict.Reason
		if res.Err != nil {
			msg = "evaluation error: " + res.Err.Error()
		}
		if res.Verdict.LogTail != "" {
			msg += "\n" + res.Verdict.LogTail
		}
		fmt.Fprintf(w, "::%s title=%s::%s\n",
			level,
			escapeGitHubAnnotation(res.Check.ID),
			escapeGitHubAnnotation(msg),
		)
	}
}

func escapeGitHubAnnotation(s string) string {
	replacer := strings.NewReplacer(
		"%", "%25",
		"\r", "%0D",
		"\n", "%0A",
	)
	return replacer.Replace(s)
}

func modeString(t Tier) string {
	switch t {
	case Fast:
		return "fast"
	case Full:
		return "full"
	default:
		return tierString(t)
	}
}

// tierString renders a check's tier membership, e.g. "fast", "full", "fast,full".
func tierString(t Tier) string {
	switch {
	case t.Has(Fast) && t.Has(Full):
		return "fast,full"
	case t.Has(Fast):
		return "fast"
	case t.Has(Full):
		return "full"
	default:
		return ""
	}
}
