// Package scenarioresults — F2.T1 gap-fill: loader boundary tests not covered
// by scenarioresults_test.go (schema version mismatch, invalid JSON, score out
// of range, threshold out of range, missing judged_at).
package scenarioresults

import (
	"os"
	"path/filepath"
	"testing"
)

// writeArtifactBytes stages raw JSON bytes at the canonical artifact path.
func writeArtifactBytes(t *testing.T, root string, data []byte) {
	t.Helper()
	dst := filepath.Join(root, filepath.FromSlash(ArtifactRelPath))
	if err := os.MkdirAll(filepath.Dir(dst), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(dst, data, 0o600); err != nil {
		t.Fatalf("write artifact: %v", err)
	}
}

func TestLoad_SchemaVersionMismatch_IsMalformed(t *testing.T) {
	// A document with a schema_version that is not "scenario-results.v1" must
	// yield StatusMalformed, not StatusOK. This is distinct from the
	// malformed-directive-mismatch fixture which has a bad directive_id value.
	const wrongVersion = `{
  "schema_version": "scenario-results.v99",
  "run_id": "run-x",
  "iteration": 0,
  "generated_at": "2026-05-17T10:00:00Z",
  "results": []
}`
	root := t.TempDir()
	writeArtifactBytes(t, root, []byte(wrongVersion))

	// Strict: must return error + StatusMalformed.
	res, err := Load(root, true)
	if err == nil {
		t.Fatalf("strict Load on wrong schema_version: want error, got nil")
	}
	if res.Status != StatusMalformed {
		t.Fatalf("strict Status = %q, want %q", res.Status, StatusMalformed)
	}

	// Non-strict: StatusMalformed + warning, no error.
	res2, err2 := Load(root, false)
	if err2 != nil {
		t.Fatalf("non-strict Load on wrong schema_version: unexpected error: %v", err2)
	}
	if res2.Status != StatusMalformed {
		t.Fatalf("non-strict Status = %q, want %q", res2.Status, StatusMalformed)
	}
	if res2.Warning == "" {
		t.Fatalf("non-strict Warning = empty, want malformed warning")
	}
	if !res2.IsSkip() {
		t.Fatalf("IsSkip() = false, want true for schema-version mismatch")
	}
}

func TestLoad_InvalidJSON_IsMalformed(t *testing.T) {
	// A file that is not valid JSON must yield StatusMalformed (parse error path),
	// not a crash or StatusUnknown (which is reserved for absent artifacts).
	root := t.TempDir()
	writeArtifactBytes(t, root, []byte(`{ not valid json !! `))

	// Strict: error + StatusMalformed.
	res, err := Load(root, true)
	if err == nil {
		t.Fatalf("strict Load on invalid JSON: want error, got nil")
	}
	if res.Status != StatusMalformed {
		t.Fatalf("strict Status = %q, want %q", res.Status, StatusMalformed)
	}

	// Non-strict: StatusMalformed + warning, no error, IsSkip.
	res2, err2 := Load(root, false)
	if err2 != nil {
		t.Fatalf("non-strict Load on invalid JSON: unexpected error: %v", err2)
	}
	if res2.Status != StatusMalformed {
		t.Fatalf("non-strict Status = %q, want %q", res2.Status, StatusMalformed)
	}
	if !res2.IsSkip() {
		t.Fatalf("IsSkip() = false, want true for invalid JSON")
	}
}

func TestLoad_ScoreOutOfRange_IsMalformed(t *testing.T) {
	// A result whose score > 1.0 must fail validateResult and yield StatusMalformed
	// in strict mode, ensuring the gate never silently accepts corrupt scores.
	const badScore = `{
  "schema_version": "scenario-results.v1",
  "run_id": "run-badscore",
  "iteration": 0,
  "generated_at": "2026-05-17T10:00:00Z",
  "results": [
    {
      "scenario_id": "s-2026-05-17-001",
      "directive_id": "d-scenario-gate",
      "score": 1.5,
      "threshold": 0.8,
      "verdict": "pass",
      "judged_at": "2026-05-17T09:00:00Z",
      "evidence": []
    }
  ]
}`
	root := t.TempDir()
	writeArtifactBytes(t, root, []byte(badScore))

	res, err := Load(root, true)
	if err == nil {
		t.Fatalf("strict Load on out-of-range score: want error, got nil")
	}
	if res.Status != StatusMalformed {
		t.Fatalf("Status = %q, want %q", res.Status, StatusMalformed)
	}
}

func TestLoad_ThresholdOutOfRange_IsMalformed(t *testing.T) {
	// A result with threshold < 0 must fail validateResult; same gate as score.
	const badThreshold = `{
  "schema_version": "scenario-results.v1",
  "run_id": "run-badthresh",
  "iteration": 0,
  "generated_at": "2026-05-17T10:00:00Z",
  "results": [
    {
      "scenario_id": "s-2026-05-17-001",
      "directive_id": "d-scenario-gate",
      "score": 0.9,
      "threshold": -0.1,
      "verdict": "pass",
      "judged_at": "2026-05-17T09:00:00Z",
      "evidence": []
    }
  ]
}`
	root := t.TempDir()
	writeArtifactBytes(t, root, []byte(badThreshold))

	res, err := Load(root, true)
	if err == nil {
		t.Fatalf("strict Load on negative threshold: want error, got nil")
	}
	if res.Status != StatusMalformed {
		t.Fatalf("Status = %q, want %q", res.Status, StatusMalformed)
	}
}

func TestLoad_MissingJudgedAt_IsMalformed(t *testing.T) {
	// A result with an empty judged_at must fail validateResult (missing
	// judged_at), preventing a clean load that would corrupt deduplication order.
	const missingJudgedAt = `{
  "schema_version": "scenario-results.v1",
  "run_id": "run-nojudge",
  "iteration": 0,
  "generated_at": "2026-05-17T10:00:00Z",
  "results": [
    {
      "scenario_id": "s-2026-05-17-001",
      "directive_id": "d-scenario-gate",
      "score": 0.9,
      "threshold": 0.8,
      "verdict": "pass",
      "judged_at": "",
      "evidence": []
    }
  ]
}`
	root := t.TempDir()
	writeArtifactBytes(t, root, []byte(missingJudgedAt))

	res, err := Load(root, true)
	if err == nil {
		t.Fatalf("strict Load on missing judged_at: want error, got nil")
	}
	if res.Status != StatusMalformed {
		t.Fatalf("Status = %q, want %q", res.Status, StatusMalformed)
	}
}

func TestLoad_EmptyResultsSlice_IsOK(t *testing.T) {
	// An artifact with a valid schema but zero results must load cleanly
	// (StatusOK, empty Results), not be rejected as malformed. Zero-scenario
	// artifacts are valid — the gate just yields unknown for every directive.
	const empty = `{
  "schema_version": "scenario-results.v1",
  "run_id": "run-empty",
  "iteration": 0,
  "generated_at": "2026-05-17T10:00:00Z",
  "results": []
}`
	root := t.TempDir()
	writeArtifactBytes(t, root, []byte(empty))

	res, err := Load(root, true)
	if err != nil {
		t.Fatalf("Load on empty results: unexpected error: %v", err)
	}
	if res.Status != StatusOK {
		t.Fatalf("Status = %q, want %q", res.Status, StatusOK)
	}
	if res.Artifact == nil {
		t.Fatalf("Artifact = nil, want non-nil")
	}
	if len(res.Artifact.Results) != 0 {
		t.Fatalf("len(Results) = %d, want 0", len(res.Artifact.Results))
	}
	if res.IsSkip() {
		t.Fatalf("IsSkip() = true, want false for StatusOK empty artifact")
	}
}
