package skilltrial

import (
	"bytes"
	"encoding/json"
	"sort"

	"github.com/boshu2/agentops/cli/internal/evidence"
	"github.com/boshu2/agentops/cli/internal/parser"
)

// Work joins one caller-selected subject to an observed native author. Status
// reports supplied independent judgment, not a new semantic verdict. Endpoint
// reward and execution remain separate; no trial is selected from retries.
type Work struct {
	Status          string                   `json:"status"`
	Execution       string                   `json:"execution"`
	Association     string                   `json:"association"`
	AuthorContextID string                   `json:"author_context_id"`
	TrialDirectory  string                   `json:"trial_directory"`
	ManifestPath    string                   `json:"manifest_path"`
	IntentPath      string                   `json:"intent_path"`
	AuthorEvidence  []Evidence               `json:"author_evidence"`
	Judgments       *evidence.JudgmentResult `json:"judgments"`
	Problems        []string                 `json:"problems"`
}

// InspectWork reuses the provenance owner to inspect original verdicts, exact
// subject/acceptance and native reviewer evidence. An inspection error remains
// visible in the report instead of hiding unsuccessful or incomplete attempts.
func InspectWork(report *Report, options evidence.JudgmentOptions) *Work {
	w := &Work{Status: "not_proven", Execution: "unknown", Association: "missing", AuthorContextID: options.AuthorContextID,
		ManifestPath: options.Manifest, IntentPath: options.Intent, AuthorEvidence: []Evidence{}, Problems: []string{}}
	w.associate(report)
	result, err := evidence.VerifyJudgments(options)
	w.Judgments = result
	if err != nil {
		w.Problems = append(w.Problems, "judgment_verification: "+err.Error())
	}
	if len(w.Problems) == 0 && result != nil {
		if result.Satisfied && w.Execution == "completed" {
			w.Status = "accepted"
		} else if verifiedFailure(result) {
			w.Status = "failed"
		}
	}
	if w.Execution != "completed" {
		w.Problems = append(w.Problems, "author_execution_"+w.Execution)
	}
	sort.Strings(w.Problems)
	return w
}

// Non-PASS is deliberately unsatisfied in VerifyJudgments. A FAIL whose only
// leg problem is verdict_not_pass still supplies an independently bound failure.
// Other missing or invalid legs remain unchanged in the original result.
func verifiedFailure(result *evidence.JudgmentResult) bool {
	for _, leg := range result.Legs {
		if leg.Verdict == "FAIL" && len(leg.Problems) == 1 && leg.Problems[0] == "verdict_not_pass" {
			return true
		}
	}
	return false
}

func (w *Work) associate(report *Report) {
	var author *Session
	for i := range report.Sessions {
		if report.Sessions[i].ID == w.AuthorContextID {
			author = &report.Sessions[i]
			break
		}
	}
	if author == nil || len(author.Copies) == 0 {
		w.Problems = append(w.Problems, "author_session_missing")
		return
	}
	w.Association = "standalone_session"
	if len(author.Diagnostics) > 0 {
		w.Problems = append(w.Problems, "author_session_diagnostics")
	}
	trials, hashes := map[string]bool{}, map[string]bool{}
	for _, copy := range author.Copies {
		w.AuthorEvidence = append(w.AuthorEvidence, copy.Evidence)
		hashes[copy.Evidence.SHA256] = true
		if copy.Trial != "" {
			trials[copy.Trial] = true
		}
		if copy.Accounting == nil || len(copy.Accounting.Diagnostics) > 0 {
			w.Problems = append(w.Problems, "author_accounting_unverified")
		}
	}
	if len(hashes) != 1 || len(trials) > 1 {
		w.Association = "ambiguous"
		w.Problems = append(w.Problems, "author_session_copies_or_trial_assignment_ambiguous")
		return
	}
	w.Execution = authorExecution(author.Copies[0].Accounting)
	for directory := range trials {
		w.Association, w.TrialDirectory = "trial", directory
		for _, job := range report.Jobs {
			for _, trial := range job.Trials {
				if trial.Directory == directory {
					w.bindTrialExecution(trial)
					return
				}
			}
		}
		w.Problems = append(w.Problems, "associated_trial_missing")
	}
}

func authorExecution(accounting *parser.CodexAccounting) string {
	if accounting == nil {
		return "unknown"
	}
	// These terminal markers are retained by accounting as uninterpreted event
	// counts. Their order relative to its latest supported marker is unavailable;
	// do not let an earlier task_complete stand in for proven completion.
	for _, marker := range []string{"event_msg/turn_aborted", "event_msg/error"} {
		if accounting.Uninterpreted[marker] > 0 {
			return "unknown"
		}
	}
	var payload struct {
		Error json.RawMessage `json:"error"`
	}
	if json.Unmarshal(accounting.LatestTurnPayload, &payload) == nil && len(payload.Error) > 0 && !bytes.Equal(bytes.TrimSpace(payload.Error), []byte("null")) {
		return "failed"
	}
	switch accounting.LatestTurnState {
	case "task_complete":
		return "completed"
	case "task_started":
		return "unfinished"
	case "task_aborted":
		return "failed"
	default:
		return "unknown"
	}
}

func (w *Work) bindTrialExecution(trial Trial) {
	switch trial.Outcome {
	case "execution_error", "infrastructure_error":
		w.Execution = "failed"
	case "created", "unfinished":
		w.Execution = "unfinished"
	default:
		if !trial.Completed {
			w.Execution = "unknown"
		}
	}
}
