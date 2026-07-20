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
	Skipped      []SkippedCheck
	Coverage     *WorkflowCoverage
	// ForeignRepo is true when the run root is not the agentops repository.
	// Human suppresses per-check agentops-internal detail there (the
	// not-applicable rows and the routing-skip wall name backing scripts that
	// do not exist in the user's repo); JSON always carries every row.
	ForeignRepo bool
}

// ExitCode is 1 if any blocking check failed the run, else 0. A blocking check
// fails on FAIL, UNKNOWN, an empty/unrecognized status, or an evaluation error
// (fail-closed — see isBlockingFail). WARN/SKIP and any non-blocking result are
// advisory. A blocking check that could not be evaluated at all (res.Err) fails
// the run even if it also returned a non-FAIL verdict.
func (r *Report) ExitCode() int {
	for _, res := range r.Results {
		if res.Check.Blocking && res.Err != nil {
			return 1
		}
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

// NotApplicableCount tallies checks that SKIPped as not-applicable outside the
// agentops repository (see NotApplicableReason), so Human can end the report
// with one honest aggregate line instead of leaving N per-check SKIP rows
// unexplained.
func (r *Report) NotApplicableCount() int {
	n := 0
	for _, res := range r.Results {
		if res.Verdict.Status == ports.GateStatusSkip && res.Verdict.Reason == NotApplicableReason {
			n++
		}
	}
	return n
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
	Run          jsonRun           `json:"run"`
	Gates        []jsonGate        `json:"gates"`
	SkippedGates []jsonSkippedGate `json:"skipped_gates,omitempty"`
	Coverage     *WorkflowCoverage `json:"coverage,omitempty"`
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
	Name            string `json:"name"`
	Tier            string `json:"tier"`
	Status          string `json:"status"`
	Blocking        bool   `json:"blocking"`
	Reason          string `json:"reason"`
	SelectedReason  string `json:"selected_reason,omitempty"`
	WorkflowBacking string `json:"workflow_backing"`
	ArtifactPath    string `json:"artifact_path"`
	RepairHint      string `json:"repair_hint"`
	LogTail         string `json:"log_tail,omitempty"`
	DurationMs      int64  `json:"duration_ms"`
}

type jsonSkippedGate struct {
	Name            string `json:"name"`
	Tier            string `json:"tier"`
	Blocking        bool   `json:"blocking"`
	SkipReason      string `json:"skip_reason"`
	WorkflowBacking string `json:"workflow_backing"`
	ArtifactPath    string `json:"artifact_path"`
	RepairHint      string `json:"repair_hint"`
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
	if r.Coverage != nil {
		jr.Coverage = r.Coverage
	}
	for _, res := range r.Results {
		reason := res.Verdict.Reason
		if res.Err != nil {
			reason = "evaluation error: " + res.Err.Error()
		}
		jr.Gates = append(jr.Gates, jsonGate{
			Name:            res.Check.ID,
			Tier:            tierString(res.Check.Tiers),
			Status:          string(res.Verdict.Status),
			Blocking:        res.Check.Blocking,
			Reason:          reason,
			SelectedReason:  res.SelectedReason,
			WorkflowBacking: res.Check.WorkflowBacking(),
			ArtifactPath:    res.Check.ArtifactPath(),
			RepairHint:      res.Check.EffectiveRepairHint(),
			LogTail:         jsonLogTail(res, reason),
			DurationMs:      res.Duration.Milliseconds(),
		})
	}
	if len(r.Skipped) > 0 {
		jr.SkippedGates = make([]jsonSkippedGate, 0, len(r.Skipped))
		for _, skip := range r.Skipped {
			jr.SkippedGates = append(jr.SkippedGates, jsonSkippedGate{
				Name:            skip.Check.ID,
				Tier:            tierString(skip.Check.Tiers),
				Blocking:        skip.Check.Blocking,
				SkipReason:      skip.Reason,
				WorkflowBacking: skip.Check.WorkflowBacking(),
				ArtifactPath:    skip.Check.ArtifactPath(),
				RepairHint:      skip.Check.EffectiveRepairHint(),
			})
		}
	}
	out, err := json.MarshalIndent(jr, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("gates: marshal report: %w", err)
	}
	return out, nil
}

// maxLogTailLines bounds the JSON log_tail to a readable window — the last few
// lines of a failing gate's output, where the actionable summary lives.
const maxLogTailLines = 15

// jsonLogTail computes the log_tail emitted for a gate. It returns the last
// maxLogTailLines lines of the captured output, and — the load-bearing part —
// falls back to the reason when a FAILing (or eval-errored) gate captured no
// output at all. Native-Go checks and silently-failing backing scripts
// otherwise emit an empty log_tail, so a --json FAIL entry carried no detail and
// diagnosing which gates failed meant re-running with tee-log archaeology
// (age-wy2t). A failing gate now always carries actionable detail.
func jsonLogTail(res CheckResult, reason string) string {
	tail := res.Verdict.LogTail
	failing := res.Verdict.Status == ports.GateStatusFail || res.Err != nil
	if strings.TrimSpace(tail) == "" && failing {
		tail = reason
	}
	return lastLines(tail, maxLogTailLines)
}

// lastLines returns at most the final n lines of s (trailing blank lines
// trimmed first), preserving order. Empty in, empty out.
func lastLines(s string, n int) string {
	s = strings.TrimRight(s, "\n")
	if s == "" {
		return ""
	}
	lines := strings.Split(s, "\n")
	if len(lines) <= n {
		return s
	}
	return strings.Join(lines[len(lines)-n:], "\n")
}

// Human writes a concise human-readable report to w.
func (r *Report) Human(w io.Writer) {
	s := r.Summary()
	for _, res := range r.Results {
		if r.ForeignRepo && res.Verdict.Status == ports.GateStatusSkip && res.Verdict.Reason == NotApplicableReason {
			continue // aggregated into the not-applicable line below
		}
		mark := string(res.Verdict.Status)
		fmt.Fprintf(w, "%-5s %s%s\n", mark, res.Check.ID, humanCheckDetails(res.Check, res.SelectedReason))
	}
	// In a foreign repo the per-gate routing-skip detail names agentops-internal
	// backing scripts that do not exist in the user's repository — noise for the
	// novice the summary lines below already cover. JSON keeps every row.
	if len(r.Skipped) > 0 && !r.ForeignRepo {
		fmt.Fprintln(w, "\nskipped gates:")
		for _, skip := range r.Skipped {
			fmt.Fprintf(w, "SKIP  %s | %s | backing: %s | artifact: %s | repair: %s\n",
				skip.Check.ID,
				skip.Reason,
				skip.Check.WorkflowBacking(),
				skip.Check.ArtifactPath(),
				skip.Check.EffectiveRepairHint(),
			)
		}
	}
	// The summary counts UNKNOWN in its own bucket: a run with blocking
	// UNKNOWNs exits 1, so the one-line summary must never read all-clear
	// ("0 fail, 0 skip") while UNKNOWN rows exist above it.
	fmt.Fprintf(w, "\n%s/%s: %d checks — %d pass, %d warn, %d fail, %d unknown, %d skip (%dms)\n",
		modeString(r.Mode), r.Scope, s.Total, s.Passed, s.Warned, s.Failed, s.Unknown, s.Skipped, r.Elapsed.Milliseconds())
	if n := r.NotApplicableCount(); n > 0 {
		fmt.Fprintf(w, "%d agentops-repo checks not applicable outside the agentops repository\n", n)
	}
	if len(r.Skipped) > 0 {
		fmt.Fprintf(w, "not run: %d gates (routing/tier/fail-fast)\n", len(r.Skipped))
	}
	if r.Coverage != nil {
		fmt.Fprintf(w, "workflow coverage: %d workflow scripts, %d registry scripts, %d missing (%d blocking, %d advisory, %d deferred), %d registry-only\n",
			r.Coverage.WorkflowScriptCount,
			r.Coverage.RegistryScriptCount,
			r.Coverage.MissingScriptCount,
			r.Coverage.MissingBlockingCount,
			r.Coverage.MissingAdvisoryCount,
			r.Coverage.MissingDeferredCount,
			r.Coverage.RegistryOnlyScriptCount,
		)
	}
}

func humanCheckDetails(c Check, selectedReason string) string {
	parts := []string{}
	if selectedReason != "" {
		parts = append(parts, selectedReason)
	}
	parts = append(parts,
		"backing: "+c.WorkflowBacking(),
		"artifact: "+c.ArtifactPath(),
		"repair: "+c.EffectiveRepairHint(),
	)
	return " | " + strings.Join(parts, " | ")
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
	if r.Coverage != nil {
		r.Coverage.GitHubAnnotations(w)
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
