package main

import (
	"bytes"
	"strings"
	"testing"
)

// realBRArraySample is a byte-for-byte capture of `br show <id> --format json`
// for a real feature bead (fixture-fidelity rule: round-trip the production
// shape, never a hand-built in-memory constructor).
const realBRArraySample = `[{"id":"age-membrane-memory-arch-tz2s.1.5","title":"E0.TV: br-native per-type acceptance-template verifier","description":"QUORUM (codex): build a br-native verifier.","status":"in_progress","priority":1,"issue_type":"feature","assignee":"bo","created_at":"2026-06-19T18:01:50.001516Z","dependencies":[{"id":"age-membrane-memory-arch-tz2s.1","dependency_type":"parent-child"}],"parent":"age-membrane-memory-arch-tz2s.1"}]`

// realBRErrorSample is a byte-for-byte capture of `br show <missing-id>
// --format json`. NOTE: br emits this error OBJECT with exit code 0, so the
// parser — not the exit code — is the only fail-open guard.
const realBRErrorSample = `{
  "error": {
    "code": "ISSUE_NOT_FOUND",
    "message": "Issue not found: no-such-bead-xyz",
    "hint": "Run 'br list' to see available issues.",
    "retryable": false,
    "context": { "searched_id": "no-such-bead-xyz" }
  }
}`

func TestParseBeadsFromBRJSON_RealArray(t *testing.T) {
	beads, err := parseBeadsFromBRJSON([]byte(realBRArraySample))
	if err != nil {
		t.Fatalf("parse real array: %v", err)
	}
	if len(beads) != 1 {
		t.Fatalf("want 1 bead, got %d", len(beads))
	}
	if beads[0].ID != "age-membrane-memory-arch-tz2s.1.5" {
		t.Errorf("id = %q", beads[0].ID)
	}
	if beads[0].IssueType != "feature" {
		t.Errorf("issue_type = %q, want feature", beads[0].IssueType)
	}
	if !strings.Contains(beads[0].Description, "br-native verifier") {
		t.Errorf("description not captured: %q", beads[0].Description)
	}
}

// The error object must be REJECTED, never coerced to a zero-value bead — that
// coercion is the fail-open the pre-mortem flagged (a missing id otherwise
// becomes an empty-description bead that PASSes for non-feature types).
func TestParseBeadsFromBRJSON_RejectsErrorObject(t *testing.T) {
	_, err := parseBeadsFromBRJSON([]byte(realBRErrorSample))
	if err == nil {
		t.Fatal("expected error for the {error:...} object, got nil (FAIL OPEN)")
	}
	if !strings.Contains(err.Error(), "ISSUE_NOT_FOUND") && !strings.Contains(err.Error(), "not found") {
		t.Errorf("error should surface the br error, got: %v", err)
	}
}

func TestParseBeadsFromBRJSON_EmptyArray(t *testing.T) {
	if _, err := parseBeadsFromBRJSON([]byte(`[]`)); err == nil {
		t.Fatal("empty array should error (no beads)")
	}
}

func TestCheckAcceptanceContract(t *testing.T) {
	featureGood := "Intro prose\n\n## Scenarios\nScenario: ok\n  Given a\n  When b\n  Then c\n\n## Acceptance Criteria\n```yaml\nacceptance_criteria:\n  - check_type: test_pass\n```"
	tests := []struct {
		name        string
		bead        acceptBead
		wantVerdict acceptanceVerdict
		wantMissing string // substring expected in a missing-element message (FAIL only)
	}{
		{"feature pass", acceptBead{IssueType: "feature", Description: featureGood}, acPass, ""},
		{"feature missing gherkin", acceptBead{IssueType: "feature", Description: "just prose, no scenarios"}, acFail, "Gherkin"},
		{"feature gherkin but no tdd", acceptBead{IssueType: "feature", Description: "## Scenarios\nScenario: ok\n  Given a\n  When b\n  Then c\n"}, acFail, "test"},
		{"spike pass", acceptBead{IssueType: "spike", Description: "## Decision Criteria\n- pick X if Y"}, acPass, ""},
		{"spike fail", acceptBead{IssueType: "spike", Description: "no criteria here"}, acFail, "decision"},
		{"design pass", acceptBead{IssueType: "design", Description: "## Specification\nformal spec body"}, acPass, ""},
		{"design fail", acceptBead{IssueType: "design", Description: "vague notes"}, acFail, "spec"},
		{"test pass", acceptBead{IssueType: "test", Description: "## Assertions\n- asserts exit code\n- asserts output"}, acPass, ""},
		{"test fail", acceptBead{IssueType: "test", Description: "no assertions listed"}, acFail, "assertion"},
		{"test: bullet under a DIFFERENT heading does not satisfy Assertions (section-scope guard)", acceptBead{IssueType: "test", Description: "## Assertions\nTBD\n\n## Notes\n- setup note only"}, acFail, "assertion"},
		{"cutover pass", acceptBead{IssueType: "cutover", Description: "## Migration Checklist\n- [ ] step one\n- [x] step two"}, acPass, ""},
		{"cutover fail no checkbox", acceptBead{IssueType: "cutover", Description: "## Migration\njust prose"}, acFail, "checklist"},
		{"task with acceptance passes", acceptBead{IssueType: "task", Description: "do X\n\n## Acceptance Criteria\nacceptance_criteria:\n  - check_command: go test ./..."}, acPass, ""},
		{"task without acceptance fails (not a free pass)", acceptBead{IssueType: "task", Description: "anything"}, acFail, "acceptance"},
		{"task with prose denying acceptance fails (loose-match guard)", acceptBead{IssueType: "task", Description: "No acceptance criteria yet, TBD"}, acFail, "acceptance"},
		{"task: bare 'acceptance_criteria' word (no colon) in prose fails", acceptBead{IssueType: "task", Description: "No acceptance_criteria yet."}, acFail, "acceptance"},
		{"design with incidental 'spec' substring fails (loose-match guard)", acceptBead{IssueType: "design", Description: "specific TBD, nothing formal"}, acFail, "spec"},
		{"design heading 'Specific' does not satisfy 'spec' (exact-word guard)", acceptBead{IssueType: "design", Description: "## Specific TBD\nnothing formal"}, acFail, "spec"},
		{"design heading 'Spec2' does not satisfy 'spec' (alphanumeric-token guard)", acceptBead{IssueType: "design", Description: "## Spec2\nbody"}, acFail, "spec"},
		{"epic with acceptance passes", acceptBead{IssueType: "epic", Description: "## Acceptance Criteria\n- measurable done"}, acPass, ""},
		{"epic without acceptance fails", acceptBead{IssueType: "epic", Description: "anything"}, acFail, "acceptance"},
		{"bug is UNDEFINED", acceptBead{IssueType: "bug", Description: "anything"}, acUndefined, ""},
		{"unknown type is UNDEFINED", acceptBead{IssueType: "wibble", Description: "anything"}, acUndefined, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, missing := checkAcceptanceContract(tt.bead)
			if got != tt.wantVerdict {
				t.Fatalf("verdict = %v, want %v (missing=%v)", got, tt.wantVerdict, missing)
			}
			if tt.wantVerdict == acFail {
				joined := strings.ToLower(strings.Join(missing, " | "))
				if !strings.Contains(joined, strings.ToLower(tt.wantMissing)) {
					t.Errorf("missing messages %v should mention %q", missing, tt.wantMissing)
				}
			}
		})
	}
}

func withStubbedBR(t *testing.T, handler func(args ...string) ([]byte, error)) {
	t.Helper()
	orig := execBR
	t.Cleanup(func() { execBR = orig })
	execBR = handler
}

func TestRunBeadsVerifyAcceptance_AdvisoryExitsZero(t *testing.T) {
	withStubbedBR(t, func(args ...string) ([]byte, error) {
		// A feature bead that FAILs the contract — advisory mode must still exit 0.
		return []byte(`[{"id":"age-x","issue_type":"feature","description":"prose only"}]`), nil
	})
	var out bytes.Buffer
	cmd := newBeadsVerifyAcceptanceCmd()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"age-x"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("advisory mode should not error, got: %v", err)
	}
	if !strings.Contains(out.String(), "FAIL") {
		t.Errorf("output should report the FAIL verdict, got: %q", out.String())
	}
}

func TestRunBeadsVerifyAcceptance_StrictExitsNonZero(t *testing.T) {
	withStubbedBR(t, func(args ...string) ([]byte, error) {
		return []byte(`[{"id":"age-x","issue_type":"feature","description":"prose only"}]`), nil
	})
	var out bytes.Buffer
	cmd := newBeadsVerifyAcceptanceCmd()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--strict", "age-x"})
	err := cmd.Execute()
	var exitErr *beadsExitError
	if err == nil {
		t.Fatal("strict mode with a FAIL should return a non-zero beadsExitError")
	}
	if !asBeadsExit(err, &exitErr) || exitErr.ExitCode() != 1 {
		t.Fatalf("want beadsExitError code 1, got: %v", err)
	}
}

func TestRunBeadsVerifyAcceptance_StrictUndefinedIsNonZero(t *testing.T) {
	withStubbedBR(t, func(args ...string) ([]byte, error) {
		return []byte(`[{"id":"age-b","issue_type":"bug","description":"x"}]`), nil
	})
	var out bytes.Buffer
	cmd := newBeadsVerifyAcceptanceCmd()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--strict", "age-b"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("strict mode must treat UNDEFINED as non-pass (fail-safe), got exit 0")
	}
}

// A partial br response (non-empty array missing a requested id) must NOT slip
// through as exit 0 — it must error, naming the absent id. Guards the multi-id
// fail-open the cross-family refuter flagged.
func TestRunBeadsVerifyAcceptance_PartialResponseErrors(t *testing.T) {
	withStubbedBR(t, func(args ...string) ([]byte, error) {
		// Requested two ids; br returns only one (the hypothetical partial array).
		return []byte(`[{"id":"age-x","issue_type":"task","description":"## Acceptance Criteria\n- done"}]`), nil
	})
	cmd := newBeadsVerifyAcceptanceCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetArgs([]string{"age-x", "age-missing"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("a requested id missing from the response must error, not exit 0 (FAIL OPEN)")
	}
	if !strings.Contains(err.Error(), "age-missing") {
		t.Errorf("error should name the absent id, got: %v", err)
	}
}

func TestRunBeadsVerifyAcceptance_PassExitsZeroStrict(t *testing.T) {
	withStubbedBR(t, func(args ...string) ([]byte, error) {
		// A task that carries a measurable acceptance signal passes even in strict mode.
		return []byte(`[{"id":"age-t","issue_type":"task","description":"do X\n\n## Acceptance Criteria\nacceptance_criteria:\n  - check_command: go test"}]`), nil
	})
	cmd := newBeadsVerifyAcceptanceCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetArgs([]string{"--strict", "age-t"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("task with acceptance should pass in strict mode, got: %v", err)
	}
}
