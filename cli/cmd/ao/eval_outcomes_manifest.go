package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/boshu2/agentops/cli/internal/eval"
)

// This file (ag-fy6e) gives `ao eval outcomes ingest` a second, optional output:
// an eval-run.v1 RUN MANIFEST alongside the council verdict record. With
// --manifest-out <dir> set, ingest ALSO writes <dir>/<run-id>/manifest.json
// conforming to schemas/eval-run.v1.schema.json, so the existing verdict
// pipeline (docs/contracts/eval-verdict-pipeline.md) picks it up and feeds the
// Knowledge Flywheel — closing the Outcomes→Flywheel loop end-to-end. The writer
// lives in this NEW file (diversifies off eval_outcomes_ingest.go); ingest.go
// only wires the thin flag.
//
// SCHEMA-vs-bead reconciliation (source-of-truth precedence, CLAUDE.md): the
// bead text said "status=complete", but that value belongs to the *other*
// SCHEMA.md run-manifest (the rc3 struct-verdict form). eval-run.v1's `status`
// enum is pass/fail/error/skipped/inconclusive — it has NO "complete" member, so
// this writer follows the schema and maps the PASS/WARN/FAIL band onto the
// allowed enum (see outcomesBandToStatus). An out-of-process Outcomes grade has
// no candidate git sha / live runtime / sandboxed environment, so git/runtime/
// environment carry honest, schema-valid stubs and a `notes` entry records the
// provenance.

// outcomesManifestDimensions is the set of dimension keys eval-run.v1's
// dimension_scores object permits (additionalProperties:false). A criterion
// score whose key is outside this set cannot be placed in the manifest, so
// outcomesDimensionScores filters to exactly these. Note context_comprehension
// is a valid eval.Dimension but is NOT a dimension_scores property in
// eval-run.v1, so it is intentionally excluded here.
var outcomesManifestDimensions = map[eval.Dimension]bool{
	eval.DimensionCorrectness:          true,
	eval.DimensionProcessAdherence:     true,
	eval.DimensionArtifactQuality:      true,
	eval.DimensionRuntimeCompatibility: true,
	eval.DimensionEfficiency:           true,
	eval.DimensionSafety:               true,
	eval.DimensionLearningClosure:      true,
}

var outcomesRunIDInvalid = regexp.MustCompile(`[^a-zA-Z0-9._:-]`)

// clampOutcomesScore bounds a score into the schema's [0,1] range so a payload
// carrying an out-of-band aggregate (or a malformed criterion score) still
// produces a schema-valid manifest rather than a validation failure downstream.
func clampOutcomesScore(v float64) float64 {
	switch {
	case v < 0:
		return 0
	case v > 1:
		return 1
	default:
		return v
	}
}

// outcomesBandToVerdict maps the council PASS/WARN/FAIL band onto the
// eval-run.v1 verdict enum: PASS→pass, FAIL→fail, anything else (WARN)→advisory.
// improvement/regression are baseline-delta verdicts; an Outcomes grade carries
// no baseline, so this writer never emits them.
func outcomesBandToVerdict(band string) eval.Verdict {
	switch band {
	case "PASS":
		return eval.VerdictPass
	case "FAIL":
		return eval.VerdictFail
	default:
		return eval.VerdictAdvisory
	}
}

// outcomesBandToStatus maps the band onto the eval-run.v1 status enum. The
// schema's status enum lacks "complete", so the run lifecycle status mirrors the
// band: PASS→pass, FAIL→fail, WARN→inconclusive.
func outcomesBandToStatus(band string) eval.Status {
	switch band {
	case "PASS":
		return eval.StatusPass
	case "FAIL":
		return eval.StatusFail
	default:
		return eval.StatusInconclusive
	}
}

// outcomesVisibility maps the score's eval split onto the suite visibility:
// a holdout grade is a private_holdout suite, everything else a public_canary.
func outcomesVisibility(split string) eval.Visibility {
	if split == "holdout" {
		return eval.VisibilityPrivateHoldout
	}
	return eval.VisibilityPublicCanary
}

// outcomesDimensionScores projects an Outcomes score's per-criterion scores onto
// the eval-run.v1 dimension_scores object, keeping only the schema-permitted
// dimension keys (additionalProperties:false). If no criterion maps to a known
// dimension, it synthesizes correctness=aggregate so the object satisfies the
// schema's minProperties:1 — an Outcomes grade always carries an aggregate.
func outcomesDimensionScores(crit map[string]float64, aggregate float64) map[eval.Dimension]float64 {
	out := map[eval.Dimension]float64{}
	for k, v := range crit {
		d := eval.Dimension(k)
		if outcomesManifestDimensions[d] {
			out[d] = clampOutcomesScore(v)
		}
	}
	if len(out) == 0 {
		out[eval.DimensionCorrectness] = clampOutcomesScore(aggregate)
	}
	return out
}

// resolveOutcomesRunID picks the run id in precedence order — explicit
// --run-id, then the score's own run_id, then its source_task_id — sanitizes it
// to the schema pattern (^[a-zA-Z0-9][a-zA-Z0-9._:-]*$), and falls back to a
// constant when nothing usable remains.
func resolveOutcomesRunID(explicit string, s outcomesScore) string {
	for _, cand := range []string{explicit, s.RunID, s.SourceTaskID} {
		if id := sanitizeOutcomesRunID(cand); id != "" {
			return id
		}
	}
	return "outcomes-run"
}

// sanitizeOutcomesRunID coerces an arbitrary string into the eval-run.v1 run_id
// pattern: invalid characters become '-', then leading non-alphanumerics are
// trimmed (the first char must be [a-zA-Z0-9]). Returns "" when nothing valid
// remains, so the caller can fall through to the next candidate.
func sanitizeOutcomesRunID(raw string) string {
	if raw == "" {
		return ""
	}
	cleaned := outcomesRunIDInvalid.ReplaceAllString(raw, "-")
	cleaned = strings.TrimLeft(cleaned, "-._:")
	return cleaned
}

// buildOutcomesRunManifest assembles an eval-run.v1 RunRecord from an Outcomes
// score and its council band. startedAt is injected (clock seam) so callers and
// tests are deterministic. git/runtime/environment are honest stubs because the
// grade was produced out-of-process; a notes entry records that provenance.
func buildOutcomesRunManifest(s outcomesScore, band, runID string, startedAt time.Time) eval.RunRecord {
	completed := startedAt
	dims := outcomesDimensionScores(s.CriterionScores, s.Aggregate)
	caseID := s.SourceTaskID
	if caseID == "" {
		caseID = "aggregate"
	}
	suiteID := s.SuiteRef
	if suiteID == "" {
		suiteID = "outcomes"
	}
	return eval.RunRecord{
		SchemaVersion: 1,
		RunID:         runID,
		Suite: eval.SuiteRef{
			ID:         suiteID,
			Path:       "out-of-process",
			Visibility: outcomesVisibility(s.Split),
			Tier:       eval.TierLive,
		},
		StartedAt:   startedAt,
		CompletedAt: &completed,
		Status:      outcomesBandToStatus(band),
		Verdict:     outcomesBandToVerdict(band),
		Git: eval.GitRecord{
			// No candidate sha is known for an out-of-process grade; a zeroed
			// hex sha satisfies the schema pattern while signalling "unknown".
			CandidateRef: "out-of-process-outcomes-grade",
			CandidateSHA: "0000000",
			Dirty:        false,
		},
		Runtime: eval.RuntimeRecord{
			Name: eval.RuntimeManual,
			Live: false,
		},
		Environment: eval.EnvironmentRecord{
			ScrubbedEnvPrefixes: []string{},
			IsolatedHome:        false,
			IsolatedCodexHome:   false,
			NetworkAccess:       eval.NetworkUnknown,
		},
		CaseResults: []eval.CaseResult{
			{
				ID:              caseID,
				Status:          outcomesBandToStatus(band),
				Score:           clampOutcomesScore(s.Aggregate),
				DimensionScores: dims,
			},
		},
		AggregateScore:  clampOutcomesScore(s.Aggregate),
		DimensionScores: dims,
		Notes: []string{
			fmt.Sprintf("Out-of-process Outcomes grade ingested by `ao eval outcomes ingest` (band=%s); git/runtime/environment are stubs — the grade was produced outside this harness.", band),
		},
	}
}

// writeOutcomesRunManifest writes rec to <dir>/<runID>/manifest.json (creating
// the directory) and returns the manifest path. It is the file-system adapter
// for buildOutcomesRunManifest.
func writeOutcomesRunManifest(dir, runID string, rec eval.RunRecord) (string, error) {
	runDir := filepath.Join(dir, runID)
	if err := os.MkdirAll(runDir, 0o750); err != nil {
		return "", fmt.Errorf("create manifest dir %s: %w", runDir, err)
	}
	blob, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return "", fmt.Errorf("encode eval-run manifest: %w", err)
	}
	path := filepath.Join(runDir, "manifest.json")
	if err := os.WriteFile(path, append(blob, '\n'), 0o600); err != nil {
		return "", fmt.Errorf("write eval-run manifest %s: %w", path, err)
	}
	return path, nil
}
