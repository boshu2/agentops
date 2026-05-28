// practices: [pragmatic-programmer, design-patterns]
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/search"
	"github.com/boshu2/agentops/cli/internal/types"
)

// ===========================================================================
// doctor.go helpers
// ===========================================================================

func TestHelper_doctorStatusIcon(t *testing.T) {
	tests := []struct {
		status string
		want   string
	}{
		{"pass", "\u2713"},
		{"warn", "!"},
		{"fail", "\u2717"},
		{"unknown", "?"},
		{"", "?"},
	}
	for _, tc := range tests {
		t.Run(tc.status, func(t *testing.T) {
			got := doctorStatusIcon(tc.status)
			if got != tc.want {
				t.Errorf("doctorStatusIcon(%q) = %q, want %q", tc.status, got, tc.want)
			}
		})
	}
}

func TestHelper_countCheckStatuses(t *testing.T) {
	tests := []struct {
		name                             string
		checks                           []doctorCheck
		wantPasses, wantFails, wantWarns int
	}{
		{
			name:       "empty",
			checks:     nil,
			wantPasses: 0, wantFails: 0, wantWarns: 0,
		},
		{
			name: "all pass",
			checks: []doctorCheck{
				{Status: "pass"},
				{Status: "pass"},
			},
			wantPasses: 2, wantFails: 0, wantWarns: 0,
		},
		{
			name: "mixed",
			checks: []doctorCheck{
				{Status: "pass"},
				{Status: "fail"},
				{Status: "warn"},
				{Status: "warn"},
				{Status: "pass"},
			},
			wantPasses: 2, wantFails: 1, wantWarns: 2,
		},
		{
			name: "all fail",
			checks: []doctorCheck{
				{Status: "fail"},
				{Status: "fail"},
			},
			wantPasses: 0, wantFails: 2, wantWarns: 0,
		},
		{
			name: "unknown status ignored",
			checks: []doctorCheck{
				{Status: "pass"},
				{Status: "something_else"},
			},
			wantPasses: 1, wantFails: 0, wantWarns: 0,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p, f, w := countCheckStatuses(tc.checks)
			if p != tc.wantPasses || f != tc.wantFails || w != tc.wantWarns {
				t.Errorf("countCheckStatuses() = (%d, %d, %d), want (%d, %d, %d)",
					p, f, w, tc.wantPasses, tc.wantFails, tc.wantWarns)
			}
		})
	}
}

func TestHelper_buildDoctorSummary(t *testing.T) {
	tests := []struct {
		name                        string
		passes, fails, warns, total int
		want                        string
	}{
		{
			name:   "all pass no warnings",
			passes: 5, fails: 0, warns: 0, total: 5,
			want: "5/5 checks passed",
		},
		{
			name:   "all pass one warning",
			passes: 4, fails: 0, warns: 1, total: 5,
			want: "4/5 checks passed, 1 warning",
		},
		{
			name:   "all pass multiple warnings",
			passes: 3, fails: 0, warns: 2, total: 5,
			want: "3/5 checks passed, 2 warnings",
		},
		{
			name:   "failures only",
			passes: 3, fails: 2, warns: 0, total: 5,
			want: "3/5 checks passed, 2 failed",
		},
		{
			name:   "failures and warnings",
			passes: 2, fails: 1, warns: 2, total: 5,
			want: "2/5 checks passed, 2 warnings, 1 failed",
		},
		{
			name:   "single failure single warning",
			passes: 3, fails: 1, warns: 1, total: 5,
			want: "3/5 checks passed, 1 warning, 1 failed",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := buildDoctorSummary(tc.passes, tc.fails, tc.warns, tc.total)
			if got != tc.want {
				t.Errorf("buildDoctorSummary(%d,%d,%d,%d) = %q, want %q",
					tc.passes, tc.fails, tc.warns, tc.total, got, tc.want)
			}
		})
	}
}

func TestHelper_hasRequiredFailure(t *testing.T) {
	tests := []struct {
		name   string
		checks []doctorCheck
		want   bool
	}{
		{
			name:   "empty",
			checks: nil,
			want:   false,
		},
		{
			name: "no required failures",
			checks: []doctorCheck{
				{Status: "pass", Required: true},
				{Status: "fail", Required: false},
				{Status: "warn", Required: true},
			},
			want: false,
		},
		{
			name: "has required failure",
			checks: []doctorCheck{
				{Status: "pass", Required: true},
				{Status: "fail", Required: true},
			},
			want: true,
		},
		{
			name: "all passing required",
			checks: []doctorCheck{
				{Status: "pass", Required: true},
				{Status: "pass", Required: true},
			},
			want: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := hasRequiredFailure(tc.checks)
			if got != tc.want {
				t.Errorf("hasRequiredFailure() = %v, want %v", got, tc.want)
			}
		})
	}
}

// ===========================================================================
// feedback.go helpers
// ===========================================================================

func TestHelper_applyJSONLRewardFields(t *testing.T) {
	// Save and restore package-level flags used by counterDirectionFromFeedback
	origHelpful := feedbackHelpful
	origHarmful := feedbackHarmful
	defer func() {
		feedbackHelpful = origHelpful
		feedbackHarmful = origHarmful
	}()

	t.Run("basic fields set", func(t *testing.T) {
		feedbackHelpful = true
		feedbackHarmful = false

		data := map[string]any{}
		applyJSONLRewardFields(data, 0.5, 0.6, 1.0)

		if data["utility"] != 0.6 {
			t.Errorf("utility = %v, want 0.6", data["utility"])
		}
		if data["last_reward"] != 1.0 {
			t.Errorf("last_reward = %v, want 1.0", data["last_reward"])
		}
		if data["reward_count"] != 1 {
			t.Errorf("reward_count = %v, want 1", data["reward_count"])
		}
		if _, ok := data["last_reward_at"]; !ok {
			t.Error("last_reward_at not set")
		}
		if _, ok := data["confidence"]; !ok {
			t.Error("confidence not set")
		}
		if data["helpful_count"] != 1 {
			t.Errorf("helpful_count = %v, want 1", data["helpful_count"])
		}
	})

	t.Run("increments existing reward_count", func(t *testing.T) {
		feedbackHelpful = false
		feedbackHarmful = true

		data := map[string]any{
			"reward_count": float64(3),
		}
		applyJSONLRewardFields(data, 0.5, 0.4, 0.0)

		if data["reward_count"] != 4 {
			t.Errorf("reward_count = %v, want 4", data["reward_count"])
		}
		if data["harmful_count"] != 1 {
			t.Errorf("harmful_count = %v, want 1", data["harmful_count"])
		}
	})

	t.Run("increments existing harmful_count", func(t *testing.T) {
		feedbackHelpful = false
		feedbackHarmful = true

		data := map[string]any{
			"harmful_count": float64(2),
		}
		applyJSONLRewardFields(data, 0.5, 0.4, 0.0)

		if data["harmful_count"] != 3 {
			t.Errorf("harmful_count = %v, want 3", data["harmful_count"])
		}
	})

	t.Run("neutral reward no counter increment", func(t *testing.T) {
		feedbackHelpful = false
		feedbackHarmful = false

		data := map[string]any{}
		applyJSONLRewardFields(data, 0.5, 0.55, 0.5) // reward=0.5 is neither helpful nor harmful by thresholds

		if _, ok := data["helpful_count"]; ok {
			t.Error("helpful_count should not be set for neutral reward")
		}
		if _, ok := data["harmful_count"]; ok {
			t.Error("harmful_count should not be set for neutral reward")
		}
	})
}

func TestHelper_parseFrontMatterUtility(t *testing.T) {
	tests := []struct {
		name     string
		lines    []string
		wantIdx  int
		wantUtil float64
		wantErr  bool
	}{
		{
			name:     "valid front matter with utility",
			lines:    []string{"---", "title: test", "utility: 0.75", "---", "body"},
			wantIdx:  3,
			wantUtil: 0.75,
		},
		{
			name:     "valid front matter without utility uses default",
			lines:    []string{"---", "title: test", "---", "body"},
			wantIdx:  2,
			wantUtil: types.InitialUtility,
		},
		{
			name:    "no closing delimiter",
			lines:   []string{"---", "title: test", "utility: 0.8"},
			wantErr: true,
		},
		{
			name:     "utility at zero still uses default parsing",
			lines:    []string{"---", "utility: 0.0", "---"},
			wantIdx:  2,
			wantUtil: 0.0,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			endIdx, utility, err := parseFrontMatterUtility(tc.lines)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if endIdx != tc.wantIdx {
				t.Errorf("endIdx = %d, want %d", endIdx, tc.wantIdx)
			}
			if utility != tc.wantUtil {
				t.Errorf("utility = %f, want %f", utility, tc.wantUtil)
			}
		})
	}
}

func TestHelper_rebuildWithFrontMatter(t *testing.T) {
	tests := []struct {
		name       string
		fm         []string
		body       []string
		wantOutput string
	}{
		{
			name:       "basic rebuild",
			fm:         []string{"title: test", "utility: 0.75"},
			body:       []string{"# Hello", "", "World"},
			wantOutput: "---\ntitle: test\nutility: 0.75\n---\n# Hello\n\nWorld",
		},
		{
			name:       "empty body",
			fm:         []string{"utility: 0.5"},
			body:       []string{},
			wantOutput: "---\nutility: 0.5\n---\n",
		},
		{
			name:       "empty front matter",
			fm:         []string{},
			body:       []string{"content"},
			wantOutput: "---\n---\ncontent",
		},
		{
			name:       "single body line",
			fm:         []string{"k: v"},
			body:       []string{"one line"},
			wantOutput: "---\nk: v\n---\none line",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := rebuildWithFrontMatter(tc.fm, tc.body)
			want := tc.wantOutput
			if got != want {
				t.Errorf("rebuildWithFrontMatter() =\n%q\nwant:\n%q", got, want)
			}
		})
	}
}

// ===========================================================================
// extract.go helpers
// ===========================================================================

func TestHelper_filterUnprocessed(t *testing.T) {
	pending := []PendingExtraction{
		{SessionID: "s1", Summary: "first"},
		{SessionID: "s2", Summary: "second"},
		{SessionID: "s3", Summary: "third"},
	}

	tests := []struct {
		name      string
		processed []string
		wantIDs   []string
	}{
		{
			name:      "all processed",
			processed: []string{"s1", "s2", "s3"},
			wantIDs:   nil,
		},
		{
			name:      "none processed",
			processed: []string{},
			wantIDs:   []string{"s1", "s2", "s3"},
		},
		{
			name:      "partial processed",
			processed: []string{"s1", "s3"},
			wantIDs:   []string{"s2"},
		},
		{
			name:      "nil processed",
			processed: nil,
			wantIDs:   []string{"s1", "s2", "s3"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			remaining := filterUnprocessed(pending, tc.processed)
			var gotIDs []string
			for _, r := range remaining {
				gotIDs = append(gotIDs, r.SessionID)
			}
			if len(tc.wantIDs) == 0 && len(gotIDs) == 0 {
				return // both empty/nil is fine
			}
			if !reflect.DeepEqual(gotIDs, tc.wantIDs) {
				t.Errorf("filterUnprocessed() IDs = %v, want %v", gotIDs, tc.wantIDs)
			}
		})
	}
}

// ===========================================================================
// search.go helpers
// ===========================================================================

func TestHelper_truncateContext(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "short string unchanged",
			input: "hello",
			want:  "hello",
		},
		{
			name:  "exactly at limit",
			input: strings.Repeat("x", search.ContextLineMaxLength),
			want:  strings.Repeat("x", search.ContextLineMaxLength),
		},
		{
			name:  "over limit truncated",
			input: strings.Repeat("x", search.ContextLineMaxLength+10),
			want:  strings.Repeat("x", search.ContextLineMaxLength) + "...",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := truncateContext(tc.input)
			if got != tc.want {
				t.Errorf("truncateContext() = %q (len=%d), want %q (len=%d)",
					got, len(got), tc.want, len(tc.want))
			}
		})
	}
}

func TestHelper_maturityToWeight(t *testing.T) {
	tests := []struct {
		name string
		data map[string]any
		want float64
	}{
		{
			name: "established",
			data: map[string]any{"maturity": "established"},
			want: 1.5,
		},
		{
			name: "candidate",
			data: map[string]any{"maturity": "candidate"},
			want: 1.2,
		},
		{
			name: "provisional",
			data: map[string]any{"maturity": "provisional"},
			want: 1.0,
		},
		{
			name: "anti-pattern",
			data: map[string]any{"maturity": "anti-pattern"},
			want: 0.3,
		},
		{
			name: "unknown maturity defaults to 1.0",
			data: map[string]any{"maturity": "unknown_level"},
			want: 1.0,
		},
		{
			name: "no maturity key defaults to 1.0",
			data: map[string]any{},
			want: 1.0,
		},
		{
			name: "maturity is not a string",
			data: map[string]any{"maturity": 42},
			want: 1.0,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := maturityToWeight(tc.data)
			if got != tc.want {
				t.Errorf("maturityToWeight() = %f, want %f", got, tc.want)
			}
		})
	}
}

// ===========================================================================
// rpi_phased_processing.go helpers
// ===========================================================================

func TestHelper_classifyGateFailureClass(t *testing.T) {
	tests := []struct {
		name     string
		phaseNum int
		gateErr  *gateFailError
		want     types.MemRLFailureClass
	}{
		{
			name:     "nil gate error",
			phaseNum: 1,
			gateErr:  nil,
			want:     "",
		},
		{
			name:     "phase 1 FAIL",
			phaseNum: 1,
			gateErr:  &gateFailError{Phase: 1, Verdict: "FAIL"},
			want:     types.MemRLFailureClassPreMortemFail,
		},
		{
			name:     "phase 2 BLOCKED",
			phaseNum: 2,
			gateErr:  &gateFailError{Phase: 2, Verdict: "BLOCKED"},
			want:     types.MemRLFailureClassCrankBlocked,
		},
		{
			name:     "phase 2 PARTIAL",
			phaseNum: 2,
			gateErr:  &gateFailError{Phase: 2, Verdict: "PARTIAL"},
			want:     types.MemRLFailureClassCrankPartial,
		},
		{
			name:     "phase 3 FAIL",
			phaseNum: 3,
			gateErr:  &gateFailError{Phase: 3, Verdict: "FAIL"},
			want:     types.MemRLFailureClassVibeFail,
		},
		{
			// Note: failReason constants are lowercase; classifyGateFailureClass uppercases
			// the verdict, so the switch in classifyByVerdict never matches these constants.
			// They fall through to default: strings.ToLower(verdict).
			name:     "timeout verdict falls to default lowercase",
			phaseNum: 1,
			gateErr:  &gateFailError{Phase: 1, Verdict: string(failReasonTimeout)},
			want:     types.MemRLFailureClass("timeout"),
		},
		{
			name:     "stall verdict falls to default lowercase",
			phaseNum: 1,
			gateErr:  &gateFailError{Phase: 1, Verdict: string(failReasonStall)},
			want:     types.MemRLFailureClass("stall"),
		},
		{
			name:     "exit_error verdict falls to default lowercase",
			phaseNum: 1,
			gateErr:  &gateFailError{Phase: 1, Verdict: string(failReasonExit)},
			want:     types.MemRLFailureClass("exit_error"),
		},
		{
			name:     "unknown verdict lowercased",
			phaseNum: 4,
			gateErr:  &gateFailError{Phase: 4, Verdict: "CUSTOM_ERROR"},
			want:     types.MemRLFailureClass("custom_error"),
		},
		{
			name:     "phase 1 non-FAIL passes to classifyByVerdict",
			phaseNum: 1,
			gateErr:  &gateFailError{Phase: 1, Verdict: "WARN"},
			want:     types.MemRLFailureClass("warn"),
		},
		{
			name:     "verdict with whitespace is trimmed",
			phaseNum: 3,
			gateErr:  &gateFailError{Phase: 3, Verdict: "  FAIL  "},
			want:     types.MemRLFailureClassVibeFail,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := classifyGateFailureClass(tc.phaseNum, tc.gateErr)
			if got != tc.want {
				t.Errorf("classifyGateFailureClass(%d, %+v) = %q, want %q",
					tc.phaseNum, tc.gateErr, got, tc.want)
			}
		})
	}
}

func TestHelper_ledgerActionFromDetails(t *testing.T) {
	tests := []struct {
		name    string
		details string
		want    string
	}{
		{"empty", "", "event"},
		{"whitespace only", "   ", "event"},
		{"started prefix", "started phase 1", "started"},
		{"completed prefix", "completed phase 2", "completed"},
		{"failed prefix", "failed: some error", "failed"},
		{"fatal prefix", "fatal: crash", "fatal"},
		{"retry prefix", "retry attempt 2/3", "retry"},
		{"dry-run prefix", "dry-run would do X", "dry-run"},
		{"handoff prefix", "handoff to next session", "handoff"},
		{"epic summary", "epic=ag-123 summary", "summary"},
		{"unknown single word", "customaction more words", "customaction"},
		{"colon stripped from first word", "some: detail", "some"},
		{"case insensitive", "STARTED phase 1", "started"},
		{"mixed case", "Started phase 1", "started"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ledgerActionFromDetails(tc.details)
			if got != tc.want {
				t.Errorf("ledgerActionFromDetails(%q) = %q, want %q", tc.details, got, tc.want)
			}
		})
	}
}

// ===========================================================================
// metrics.go helpers
// ===========================================================================

func TestHelper_countStaleInDir(t *testing.T) {
	t.Run("nonexistent dir returns 0", func(t *testing.T) {
		got := countStaleInDir("/tmp", "/nonexistent/path/xyz", time.Now(), nil)
		if got != 0 {
			t.Errorf("expected 0 for nonexistent dir, got %d", got)
		}
	})

	t.Run("counts stale files", func(t *testing.T) {
		baseDir := t.TempDir()
		dir := filepath.Join(baseDir, "learnings")
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}

		// Create a .md file and backdate it
		oldFile := filepath.Join(dir, "old-learning.md")
		if err := os.WriteFile(oldFile, []byte("# old"), 0644); err != nil {
			t.Fatal(err)
		}
		oldTime := time.Now().Add(-200 * 24 * time.Hour)
		if err := os.Chtimes(oldFile, oldTime, oldTime); err != nil {
			t.Fatal(err)
		}

		// Create a recent file
		newFile := filepath.Join(dir, "new-learning.md")
		if err := os.WriteFile(newFile, []byte("# new"), 0644); err != nil {
			t.Fatal(err)
		}

		// Create a non-knowledge file (should be skipped)
		txtFile := filepath.Join(dir, "readme.txt")
		if err := os.WriteFile(txtFile, []byte("readme"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(txtFile, oldTime, oldTime); err != nil {
			t.Fatal(err)
		}

		staleThreshold := time.Now().Add(-90 * 24 * time.Hour)
		lastCited := map[string]time.Time{} // no citations

		got := countStaleInDir(baseDir, dir, staleThreshold, lastCited)
		if got != 1 {
			t.Errorf("countStaleInDir() = %d, want 1 (only old-learning.md)", got)
		}
	})

	t.Run("cited file not stale", func(t *testing.T) {
		baseDir := t.TempDir()
		dir := filepath.Join(baseDir, "learnings")
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}

		oldFile := filepath.Join(dir, "cited-learning.md")
		if err := os.WriteFile(oldFile, []byte("# cited"), 0644); err != nil {
			t.Fatal(err)
		}
		oldTime := time.Now().Add(-200 * 24 * time.Hour)
		if err := os.Chtimes(oldFile, oldTime, oldTime); err != nil {
			t.Fatal(err)
		}

		staleThreshold := time.Now().Add(-90 * 24 * time.Hour)
		// File was cited recently
		lastCited := map[string]time.Time{
			oldFile: time.Now().Add(-10 * 24 * time.Hour),
		}

		got := countStaleInDir(baseDir, dir, staleThreshold, lastCited)
		if got != 0 {
			t.Errorf("countStaleInDir() = %d, want 0 (file was recently cited)", got)
		}
	})
}

// ===========================================================================
// rpi_ledger.go helpers
// ===========================================================================

func TestHelper_validateLedgerRequiredFields(t *testing.T) {
	validRecord := RPILedgerRecord{
		EventID:     "evt-abc",
		RunID:       "run-123",
		Phase:       "discovery",
		Action:      "started",
		TS:          time.Now().UTC().Format(time.RFC3339Nano),
		PayloadHash: "deadbeef",
		Hash:        "cafebabe",
	}

	t.Run("valid record passes", func(t *testing.T) {
		err := validateLedgerRequiredFields(validRecord)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	requiredFields := []string{"EventID", "RunID", "Phase", "Action", "TS", "PayloadHash", "Hash"}
	fieldMap := map[string]func(r RPILedgerRecord) RPILedgerRecord{
		"EventID":     func(r RPILedgerRecord) RPILedgerRecord { r.EventID = ""; return r },
		"RunID":       func(r RPILedgerRecord) RPILedgerRecord { r.RunID = ""; return r },
		"Phase":       func(r RPILedgerRecord) RPILedgerRecord { r.Phase = ""; return r },
		"Action":      func(r RPILedgerRecord) RPILedgerRecord { r.Action = ""; return r },
		"TS":          func(r RPILedgerRecord) RPILedgerRecord { r.TS = ""; return r },
		"PayloadHash": func(r RPILedgerRecord) RPILedgerRecord { r.PayloadHash = ""; return r },
		"Hash":        func(r RPILedgerRecord) RPILedgerRecord { r.Hash = ""; return r },
	}

	for _, field := range requiredFields {
		t.Run("missing "+field, func(t *testing.T) {
			record := fieldMap[field](validRecord)
			err := validateLedgerRequiredFields(record)
			if err == nil {
				t.Errorf("expected error for missing %s, got nil", field)
			}
		})
	}

	t.Run("whitespace-only field fails", func(t *testing.T) {
		record := validRecord
		record.RunID = "   "
		err := validateLedgerRequiredFields(record)
		if err == nil {
			t.Error("expected error for whitespace-only RunID")
		}
	})
}

func TestHelper_validateLedgerTimestamp(t *testing.T) {
	tests := []struct {
		name    string
		ts      string
		wantErr bool
	}{
		{
			name:    "valid UTC RFC3339Nano",
			ts:      time.Now().UTC().Format(time.RFC3339Nano),
			wantErr: false,
		},
		{
			name:    "not RFC3339",
			ts:      "2024-01-15 10:30:00",
			wantErr: true,
		},
		{
			name:    "empty string",
			ts:      "",
			wantErr: true,
		},
		{
			name:    "non-UTC timezone",
			ts:      "2024-01-15T10:30:00-05:00",
			wantErr: true, // must be UTC
		},
		{
			name:    "valid UTC with nanoseconds",
			ts:      "2024-01-15T10:30:00.123456789Z",
			wantErr: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateLedgerTimestamp(tc.ts)
			if (err != nil) != tc.wantErr {
				t.Errorf("validateLedgerTimestamp(%q) error = %v, wantErr %v", tc.ts, err, tc.wantErr)
			}
		})
	}
}

func TestHelper_roundTripJSON(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:  "simple object",
			input: `{"b":2,"a":1}`,
			want:  `{"a":1,"b":2}`, // json.Marshal sorts keys
		},
		{
			name:  "array",
			input: `[1,2,3]`,
			want:  `[1,2,3]`,
		},
		{
			name:  "string value",
			input: `"hello"`,
			want:  `"hello"`,
		},
		{
			name:  "null",
			input: `null`,
			want:  `null`,
		},
		{
			name:    "invalid JSON",
			input:   `{broken`,
			wantErr: true,
		},
		{
			name:  "nested object",
			input: `{"outer":{"inner":"value"}}`,
			want:  `{"outer":{"inner":"value"}}`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := roundTripJSON([]byte(tc.input))
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if string(got) != tc.want {
				t.Errorf("roundTripJSON(%q) = %q, want %q", tc.input, string(got), tc.want)
			}
		})
	}
}

// ===========================================================================
// batch_forge.go helpers
// ===========================================================================

// mockFileInfo implements os.FileInfo for testing isBatchTranscriptCandidate.
type mockFileInfo struct {
	name  string
	size  int64
	isDir bool
}

func (m mockFileInfo) Name() string       { return m.name }
func (m mockFileInfo) Size() int64        { return m.size }
func (m mockFileInfo) Mode() os.FileMode  { return 0644 }
func (m mockFileInfo) ModTime() time.Time { return time.Now() }
func (m mockFileInfo) IsDir() bool        { return m.isDir }
func (m mockFileInfo) Sys() any           { return nil }

func TestHelper_isBatchTranscriptCandidate(t *testing.T) {
	tests := []struct {
		name string
		info os.FileInfo
		path string
		want bool
	}{
		{
			name: "valid jsonl file",
			info: mockFileInfo{name: "transcript.jsonl", size: 500, isDir: false},
			path: "/path/to/transcript.jsonl",
			want: true,
		},
		{
			name: "directory",
			info: mockFileInfo{name: "somedir", size: 500, isDir: true},
			path: "/path/to/somedir.jsonl",
			want: false,
		},
		{
			name: "non-jsonl extension",
			info: mockFileInfo{name: "file.md", size: 500, isDir: false},
			path: "/path/to/file.md",
			want: false,
		},
		{
			name: "too small",
			info: mockFileInfo{name: "tiny.jsonl", size: 50, isDir: false},
			path: "/path/to/tiny.jsonl",
			want: false,
		},
		{
			name: "exactly 100 bytes (boundary)",
			info: mockFileInfo{name: "edge.jsonl", size: 100, isDir: false},
			path: "/path/to/edge.jsonl",
			want: false, // must be > 100
		},
		{
			name: "101 bytes passes",
			info: mockFileInfo{name: "ok.jsonl", size: 101, isDir: false},
			path: "/path/to/ok.jsonl",
			want: true,
		},
		{
			name: "json extension rejected",
			info: mockFileInfo{name: "file.json", size: 500, isDir: false},
			path: "/path/to/file.json",
			want: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := isBatchTranscriptCandidate(tc.info, tc.path)
			if got != tc.want {
				t.Errorf("isBatchTranscriptCandidate(%+v, %q) = %v, want %v",
					tc.info, tc.path, got, tc.want)
			}
		})
	}
}

// ===========================================================================
// context.go helpers
// ===========================================================================

func TestHelper_mergeAssignmentFields(t *testing.T) {
	t.Run("fills empty fields from persisted", func(t *testing.T) {
		current := &contextAssignment{}
		persisted := &contextAssignment{
			AgentName:   "worker-1",
			AgentRole:   "worker",
			TeamName:    "team-alpha",
			IssueID:     "ag-abc",
			TmuxPaneID:  "%5",
			TmuxTarget:  "session:0",
			TmuxSession: "session",
		}
		status := &contextSessionStatus{}
		mergeAssignmentFields(current, persisted, status)

		if status.AgentName != "worker-1" {
			t.Errorf("AgentName = %q, want %q", status.AgentName, "worker-1")
		}
		if status.AgentRole != "worker" {
			t.Errorf("AgentRole = %q, want %q", status.AgentRole, "worker")
		}
		if status.TeamName != "team-alpha" {
			t.Errorf("TeamName = %q, want %q", status.TeamName, "team-alpha")
		}
		if status.IssueID != "ag-abc" {
			t.Errorf("IssueID = %q, want %q", status.IssueID, "ag-abc")
		}
		if status.TmuxPaneID != "%5" {
			t.Errorf("TmuxPaneID = %q, want %q", status.TmuxPaneID, "%5")
		}
		if status.TmuxTarget != "session:0" {
			t.Errorf("TmuxTarget = %q, want %q", status.TmuxTarget, "session:0")
		}
		if status.TmuxSession != "session" {
			t.Errorf("TmuxSession = %q, want %q", status.TmuxSession, "session")
		}
	})

	t.Run("does not overwrite existing fields", func(t *testing.T) {
		current := &contextAssignment{
			AgentName:   "existing-agent",
			AgentRole:   "team-lead",
			TeamName:    "my-team",
			IssueID:     "ag-xyz",
			TmuxPaneID:  "%1",
			TmuxTarget:  "myses:0",
			TmuxSession: "myses",
		}
		persisted := &contextAssignment{
			AgentName:   "old-agent",
			AgentRole:   "worker",
			TeamName:    "old-team",
			IssueID:     "ag-old",
			TmuxPaneID:  "%9",
			TmuxTarget:  "old:0",
			TmuxSession: "old",
		}
		status := &contextSessionStatus{
			AgentName:   "original",
			AgentRole:   "original-role",
			TeamName:    "original-team",
			IssueID:     "original-issue",
			TmuxPaneID:  "original-pane",
			TmuxTarget:  "original-target",
			TmuxSession: "original-session",
		}
		mergeAssignmentFields(current, persisted, status)

		// All fields should remain unchanged since current is non-empty
		if status.AgentName != "original" {
			t.Errorf("AgentName was overwritten: %q", status.AgentName)
		}
		if status.AgentRole != "original-role" {
			t.Errorf("AgentRole was overwritten: %q", status.AgentRole)
		}
		if status.TeamName != "original-team" {
			t.Errorf("TeamName was overwritten: %q", status.TeamName)
		}
		if status.IssueID != "original-issue" {
			t.Errorf("IssueID was overwritten: %q", status.IssueID)
		}
	})

	t.Run("partial merge", func(t *testing.T) {
		current := &contextAssignment{
			AgentName: "set-agent",
			// AgentRole is empty
			TeamName: "set-team",
			// IssueID is empty
		}
		persisted := &contextAssignment{
			AgentName: "persisted-agent",
			AgentRole: "persisted-role",
			TeamName:  "persisted-team",
			IssueID:   "ag-persisted",
		}
		status := &contextSessionStatus{}
		mergeAssignmentFields(current, persisted, status)

		// AgentName set in current, so status should NOT be updated
		if status.AgentName != "" {
			t.Errorf("AgentName should be empty (current had value): got %q", status.AgentName)
		}
		// AgentRole empty in current, so should be filled from persisted
		if status.AgentRole != "persisted-role" {
			t.Errorf("AgentRole = %q, want %q", status.AgentRole, "persisted-role")
		}
		// TeamName set in current, so status should NOT be updated
		if status.TeamName != "" {
			t.Errorf("TeamName should be empty (current had value): got %q", status.TeamName)
		}
		// IssueID empty in current, so should be filled from persisted
		if status.IssueID != "ag-persisted" {
			t.Errorf("IssueID = %q, want %q", status.IssueID, "ag-persisted")
		}
	})
}

// ===========================================================================
// Additional edge-case coverage
// ===========================================================================

func TestHelper_roundTripJSON_empty(t *testing.T) {
	got, err := roundTripJSON([]byte(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(got) != `{}` {
		t.Errorf("roundTripJSON({}) = %q, want {}", string(got))
	}
}

func TestHelper_classifyGateFailureClass_lowercaseVerdict(t *testing.T) {
	// Verify that lowercase "fail" still matches (verdict is uppercased internally)
	got := classifyGateFailureClass(1, &gateFailError{Phase: 1, Verdict: "fail"})
	if got != types.MemRLFailureClassPreMortemFail {
		t.Errorf("classifyGateFailureClass(1, 'fail') = %q, want %q", got, types.MemRLFailureClassPreMortemFail)
	}
}

func TestHelper_filterUnprocessed_emptyPending(t *testing.T) {
	remaining := filterUnprocessed(nil, []string{"s1"})
	if len(remaining) != 0 {
		t.Errorf("expected empty, got %d", len(remaining))
	}
}

func TestHelper_validateLedgerTimestamp_RFC3339_basic(t *testing.T) {
	// UTC RFC3339 without nanoseconds should also pass, since
	// time.RFC3339Nano can parse it and the roundtrip check should hold.
	ts := "2024-06-15T10:30:00Z"
	err := validateLedgerTimestamp(ts)
	if err != nil {
		t.Errorf("validateLedgerTimestamp(%q) unexpected error: %v", ts, err)
	}
}

func TestHelper_countStaleInDir_emptyDir(t *testing.T) {
	dir := t.TempDir()
	got := countStaleInDir(dir, dir, time.Now(), map[string]time.Time{})
	if got != 0 {
		t.Errorf("countStaleInDir on empty dir = %d, want 0", got)
	}
}

func TestHelper_isBatchTranscriptCandidate_zeroSize(t *testing.T) {
	info := mockFileInfo{name: "empty.jsonl", size: 0, isDir: false}
	got := isBatchTranscriptCandidate(info, "/path/empty.jsonl")
	if got {
		t.Error("zero-size file should not be a candidate")
	}
}

func TestHelper_applyJSONLRewardFields_confidenceFormula(t *testing.T) {
	// Verify confidence formula: 1 - 1/(1 + rewardCount/5)
	origHelpful := feedbackHelpful
	origHarmful := feedbackHarmful
	defer func() {
		feedbackHelpful = origHelpful
		feedbackHarmful = origHarmful
	}()
	feedbackHelpful = false
	feedbackHarmful = false

	data := map[string]any{
		"reward_count": float64(9), // will become 10 after increment
	}
	applyJSONLRewardFields(data, 0.5, 0.6, 0.5)

	// Expected: 1 - 1/(1 + 10/5) = 1 - 1/3 = 0.6667
	conf, ok := data["confidence"].(float64)
	if !ok {
		t.Fatal("confidence not set")
	}
	expected := 1.0 - (1.0 / (1.0 + 10.0/5.0))
	if diff := conf - expected; diff > 0.0001 || diff < -0.0001 {
		t.Errorf("confidence = %f, want ~%f", conf, expected)
	}
}

func TestHelper_buildDoctorSummary_zeroTotal(t *testing.T) {
	got := buildDoctorSummary(0, 0, 0, 0)
	if got != "0/0 checks passed" {
		t.Errorf("buildDoctorSummary(0,0,0,0) = %q, want %q", got, "0/0 checks passed")
	}
}

func TestHelper_roundTripJSON_preservesBoolAndNumber(t *testing.T) {
	input := `{"flag":true,"count":42,"name":"test"}`
	got, err := roundTripJSON([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(got, &parsed); err != nil {
		t.Fatalf("result is not valid JSON: %v", err)
	}
	if parsed["flag"] != true {
		t.Errorf("flag = %v, want true", parsed["flag"])
	}
	if parsed["count"] != float64(42) {
		t.Errorf("count = %v, want 42", parsed["count"])
	}
	if parsed["name"] != "test" {
		t.Errorf("name = %v, want test", parsed["name"])
	}
}
