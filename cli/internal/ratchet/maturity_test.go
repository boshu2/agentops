package ratchet

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/domain"
)

// writeLearning is a test helper that writes a JSONL learning file.
func writeLearning(t *testing.T, dir, name string, data map[string]any) string {
	t.Helper()
	b, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal learning data: %v", err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, b, 0644); err != nil {
		t.Fatalf("write learning file: %v", err)
	}
	return path
}

func TestCheckMaturityTransition_ProvisionalToCandidate(t *testing.T) {
	tests := []struct {
		name         string
		data         map[string]any
		wantTransit  bool
		wantNew      domain.Maturity
		wantReasonSS string // substring expected in reason
	}{
		{
			name: "promotes when utility and reward_count meet thresholds",
			data: map[string]any{
				"id":           "learn-001",
				"maturity":     "provisional",
				"utility":      0.8,
				"reward_count": 4.0,
			},
			wantTransit: true,
			wantNew:     domain.MaturityCandidate,
		},
		{
			name: "promotes at exact thresholds",
			data: map[string]any{
				"id":           "learn-002",
				"maturity":     "provisional",
				"utility":      0.7,
				"reward_count": 3.0,
			},
			wantTransit: true,
			wantNew:     domain.MaturityCandidate,
		},
		{
			name: "no promotion when utility too low",
			data: map[string]any{
				"id":           "learn-003",
				"maturity":     "provisional",
				"utility":      0.5,
				"reward_count": 5.0,
			},
			wantTransit:  false,
			wantNew:      domain.MaturityProvisional,
			wantReasonSS: "not enough positive feedback",
		},
		{
			name: "no promotion when reward_count too low",
			data: map[string]any{
				"id":           "learn-004",
				"maturity":     "provisional",
				"utility":      0.9,
				"reward_count": 2.0,
			},
			wantTransit:  false,
			wantNew:      domain.MaturityProvisional,
			wantReasonSS: "not enough positive feedback",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := writeLearning(t, dir, "test.jsonl", tt.data)

			result, err := CheckMaturityTransition(path)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.Transitioned != tt.wantTransit {
				t.Errorf("Transitioned = %v, want %v", result.Transitioned, tt.wantTransit)
			}
			if result.NewMaturity != tt.wantNew {
				t.Errorf("NewMaturity = %q, want %q", result.NewMaturity, tt.wantNew)
			}
			if tt.wantReasonSS != "" && result.Reason == "" {
				t.Errorf("expected reason containing %q, got empty", tt.wantReasonSS)
			}
		})
	}
}

func TestCheckMaturityTransition_CandidateToEstablished(t *testing.T) {
	tests := []struct {
		name        string
		data        map[string]any
		wantTransit bool
		wantNew     domain.Maturity
	}{
		{
			name: "promotes when all conditions met",
			data: map[string]any{
				"maturity":      "candidate",
				"utility":       0.8,
				"reward_count":  6.0,
				"helpful_count": 5.0,
				"harmful_count": 1.0,
			},
			wantTransit: true,
			wantNew:     domain.MaturityEstablished,
		},
		{
			name: "no promotion when helpful not greater than harmful",
			data: map[string]any{
				"maturity":      "candidate",
				"utility":       0.8,
				"reward_count":  6.0,
				"helpful_count": 2.0,
				"harmful_count": 2.0,
			},
			wantTransit: false,
			wantNew:     domain.MaturityCandidate,
		},
		{
			name: "no promotion when reward_count less than 5",
			data: map[string]any{
				"maturity":      "candidate",
				"utility":       0.8,
				"reward_count":  4.0,
				"helpful_count": 3.0,
				"harmful_count": 1.0,
			},
			wantTransit: false,
			wantNew:     domain.MaturityCandidate,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := writeLearning(t, dir, "test.jsonl", tt.data)

			result, err := CheckMaturityTransition(path)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.Transitioned != tt.wantTransit {
				t.Errorf("Transitioned = %v, want %v", result.Transitioned, tt.wantTransit)
			}
			if result.NewMaturity != tt.wantNew {
				t.Errorf("NewMaturity = %q, want %q", result.NewMaturity, tt.wantNew)
			}
		})
	}
}

func TestCheckMaturityTransition_CandidateToProvisionalDemotion(t *testing.T) {
	dir := t.TempDir()
	path := writeLearning(t, dir, "test.jsonl", map[string]any{
		"maturity": "candidate",
		"utility":  0.2,
	})

	result, err := CheckMaturityTransition(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Transitioned {
		t.Error("expected transition (demotion)")
	}
	if result.NewMaturity != domain.MaturityProvisional {
		t.Errorf("NewMaturity = %q, want %q", result.NewMaturity, domain.MaturityProvisional)
	}
	if result.OldMaturity != domain.MaturityCandidate {
		t.Errorf("OldMaturity = %q, want %q", result.OldMaturity, domain.MaturityCandidate)
	}
}

func TestCheckMaturityTransition_EstablishedToCandidateDemotion(t *testing.T) {
	tests := []struct {
		name        string
		utility     float64
		wantTransit bool
		wantNew     domain.Maturity
	}{
		{"demotes when utility below 0.5", 0.4, true, domain.MaturityCandidate},
		{"stays established at exactly 0.5", 0.5, false, domain.MaturityEstablished},
		{"stays established when utility high", 0.8, false, domain.MaturityEstablished},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := writeLearning(t, dir, "test.jsonl", map[string]any{
				"maturity": "established",
				"utility":  tt.utility,
			})

			result, err := CheckMaturityTransition(path)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.Transitioned != tt.wantTransit {
				t.Errorf("Transitioned = %v, want %v", result.Transitioned, tt.wantTransit)
			}
			if result.NewMaturity != tt.wantNew {
				t.Errorf("NewMaturity = %q, want %q", result.NewMaturity, tt.wantNew)
			}
		})
	}
}

func TestCheckMaturityTransition_AntiPatternPriority(t *testing.T) {
	// Anti-pattern transition takes priority over all other transitions.
	tests := []struct {
		name        string
		data        map[string]any
		wantTransit bool
		wantNew     domain.Maturity
	}{
		{
			name: "provisional becomes anti-pattern",
			data: map[string]any{
				"maturity":      "provisional",
				"utility":       0.1,
				"harmful_count": 6.0,
			},
			wantTransit: true,
			wantNew:     domain.MaturityAntiPattern,
		},
		{
			name: "candidate becomes anti-pattern",
			data: map[string]any{
				"maturity":      "candidate",
				"utility":       0.2,
				"harmful_count": 5.0,
			},
			wantTransit: true,
			wantNew:     domain.MaturityAntiPattern,
		},
		{
			name: "established becomes anti-pattern",
			data: map[string]any{
				"maturity":      "established",
				"utility":       0.15,
				"harmful_count": 7.0,
			},
			wantTransit: true,
			wantNew:     domain.MaturityAntiPattern,
		},
		{
			name: "already anti-pattern stays anti-pattern (no transition)",
			data: map[string]any{
				"maturity":      "anti-pattern",
				"utility":       0.1,
				"harmful_count": 10.0,
			},
			wantTransit: false,
			wantNew:     domain.MaturityAntiPattern,
		},
		{
			name: "not anti-pattern when harmful_count below threshold",
			data: map[string]any{
				"maturity":      "provisional",
				"utility":       0.1,
				"harmful_count": 2.0,
			},
			wantTransit: false,
			wantNew:     domain.MaturityProvisional,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := writeLearning(t, dir, "test.jsonl", tt.data)

			result, err := CheckMaturityTransition(path)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.Transitioned != tt.wantTransit {
				t.Errorf("Transitioned = %v, want %v", result.Transitioned, tt.wantTransit)
			}
			if result.NewMaturity != tt.wantNew {
				t.Errorf("NewMaturity = %q, want %q", result.NewMaturity, tt.wantNew)
			}
		})
	}
}

func TestCheckMaturityTransition_AntiPatternRehabilitation(t *testing.T) {
	tests := []struct {
		name        string
		data        map[string]any
		wantTransit bool
		wantNew     domain.Maturity
	}{
		{
			name: "rehabilitates when utility high and helpful dominant",
			data: map[string]any{
				"maturity":      "anti-pattern",
				"utility":       0.7,
				"helpful_count": 11.0,
				"harmful_count": 5.0,
			},
			wantTransit: true,
			wantNew:     domain.MaturityProvisional,
		},
		{
			name: "no rehab when helpful not greater than 2x harmful",
			data: map[string]any{
				"maturity":      "anti-pattern",
				"utility":       0.7,
				"helpful_count": 10.0,
				"harmful_count": 5.0,
			},
			wantTransit: false,
			wantNew:     domain.MaturityAntiPattern,
		},
		{
			name: "no rehab when utility below 0.6",
			data: map[string]any{
				"maturity":      "anti-pattern",
				"utility":       0.5,
				"helpful_count": 20.0,
				"harmful_count": 5.0,
			},
			wantTransit: false,
			wantNew:     domain.MaturityAntiPattern,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := writeLearning(t, dir, "test.jsonl", tt.data)

			result, err := CheckMaturityTransition(path)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.Transitioned != tt.wantTransit {
				t.Errorf("Transitioned = %v, want %v", result.Transitioned, tt.wantTransit)
			}
			if result.NewMaturity != tt.wantNew {
				t.Errorf("NewMaturity = %q, want %q", result.NewMaturity, tt.wantNew)
			}
		})
	}
}

func TestCheckMaturityTransition_DefaultValues(t *testing.T) {
	// When fields are missing, defaults should be applied.
	dir := t.TempDir()
	path := writeLearning(t, dir, "test.jsonl", map[string]any{
		"id": "bare-learning",
	})

	result, err := CheckMaturityTransition(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.LearningID != "bare-learning" {
		t.Errorf("LearningID = %q, want %q", result.LearningID, "bare-learning")
	}
	if result.OldMaturity != domain.MaturityProvisional {
		t.Errorf("OldMaturity = %q, want %q (default)", result.OldMaturity, domain.MaturityProvisional)
	}
	if result.Utility != domain.InitialUtility {
		t.Errorf("Utility = %f, want %f (InitialUtility)", result.Utility, domain.InitialUtility)
	}
	if result.Confidence != 0.5 {
		t.Errorf("Confidence = %f, want 0.5 (default)", result.Confidence)
	}
	if result.HelpfulCount != 0 {
		t.Errorf("HelpfulCount = %d, want 0", result.HelpfulCount)
	}
	if result.HarmfulCount != 0 {
		t.Errorf("HarmfulCount = %d, want 0", result.HarmfulCount)
	}
	if result.RewardCount != 0 {
		t.Errorf("RewardCount = %d, want 0", result.RewardCount)
	}
}

func TestCheckMaturityTransition_IDFromFilename(t *testing.T) {
	// When no "id" field, LearningID should be derived from filename.
	dir := t.TempDir()
	path := writeLearning(t, dir, "my-learning.jsonl", map[string]any{
		"maturity": "provisional",
	})

	result, err := CheckMaturityTransition(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.LearningID != "my-learning.jsonl" {
		t.Errorf("LearningID = %q, want %q", result.LearningID, "my-learning.jsonl")
	}
}

func TestCheckMaturityTransition_Errors(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(t *testing.T) string
		wantErr string
	}{
		{
			name: "nonexistent file",
			setup: func(t *testing.T) string {
				t.Helper()
				return filepath.Join(t.TempDir(), "nonexistent.jsonl")
			},
			wantErr: "read learning",
		},
		{
			name: "empty file",
			setup: func(t *testing.T) string {
				t.Helper()
				dir := t.TempDir()
				path := filepath.Join(dir, "empty.jsonl")
				if err := os.WriteFile(path, []byte(""), 0644); err != nil {
					t.Fatalf("write file: %v", err)
				}
				return path
			},
			wantErr: "empty learning file",
		},
		{
			name: "invalid JSON",
			setup: func(t *testing.T) string {
				t.Helper()
				dir := t.TempDir()
				path := filepath.Join(dir, "bad.jsonl")
				if err := os.WriteFile(path, []byte("not json"), 0644); err != nil {
					t.Fatalf("write file: %v", err)
				}
				return path
			},
			wantErr: "parse learning",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := tt.setup(t)
			_, err := CheckMaturityTransition(path)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if got := err.Error(); !contains(got, tt.wantErr) {
				t.Errorf("error = %q, want substring %q", got, tt.wantErr)
			}
		})
	}
}

func TestCheckMaturityTransition_SentinelErrors(t *testing.T) {
	// Verify sentinel errors work with errors.Is.
	dir := t.TempDir()
	emptyPath := filepath.Join(dir, "empty.jsonl")
	if err := os.WriteFile(emptyPath, []byte(""), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	_, err := CheckMaturityTransition(emptyPath)
	if err == nil {
		t.Fatal("expected error for empty file")
	}
	if !errors.Is(err, ErrEmptyLearningFile) {
		t.Errorf("expected ErrEmptyLearningFile, got %v", err)
	}
}

func TestApplyMaturityTransition_WritesFile(t *testing.T) {
	dir := t.TempDir()
	path := writeLearning(t, dir, "test.jsonl", map[string]any{
		"id":           "apply-test",
		"maturity":     "provisional",
		"utility":      0.8,
		"reward_count": 4.0,
	})

	result, err := ApplyMaturityTransition(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Transitioned {
		t.Fatal("expected transition")
	}
	if result.NewMaturity != domain.MaturityCandidate {
		t.Errorf("NewMaturity = %q, want %q", result.NewMaturity, domain.MaturityCandidate)
	}

	// Verify the file was updated.
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read updated file: %v", err)
	}
	var updated map[string]any
	if err := json.Unmarshal(content, &updated); err != nil {
		t.Fatalf("parse updated file: %v", err)
	}
	if m, ok := updated["maturity"].(string); !ok || m != "candidate" {
		t.Errorf("file maturity = %q, want %q", m, "candidate")
	}
	if _, ok := updated["maturity_changed_at"].(string); !ok {
		t.Error("expected maturity_changed_at timestamp in updated file")
	}
	if _, ok := updated["maturity_reason"].(string); !ok {
		t.Error("expected maturity_reason in updated file")
	}
}

func TestApplyMaturityTransition_NoTransitionDoesNotWriteFile(t *testing.T) {
	dir := t.TempDir()
	data := map[string]any{
		"id":       "no-change",
		"maturity": "provisional",
		"utility":  0.3,
	}
	path := writeLearning(t, dir, "test.jsonl", data)

	// Get original content for comparison.
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read original: %v", err)
	}

	result, err := ApplyMaturityTransition(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Transitioned {
		t.Fatal("expected no transition")
	}

	// File should be unchanged.
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read after: %v", err)
	}
	if string(original) != string(after) {
		t.Error("file was modified despite no transition")
	}
}

func TestApplyMaturityTransition_ErrorOnMissingFile(t *testing.T) {
	_, err := ApplyMaturityTransition(filepath.Join(t.TempDir(), "missing.jsonl"))
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestScanForMaturityTransitions(t *testing.T) {
	dir := t.TempDir()

	// File that would transition (provisional -> candidate).
	writeLearning(t, dir, "promote.jsonl", map[string]any{
		"id":           "promote",
		"maturity":     "provisional",
		"utility":      0.8,
		"reward_count": 4.0,
	})

	// File that would NOT transition (stays provisional).
	writeLearning(t, dir, "stay.jsonl", map[string]any{
		"id":       "stay",
		"maturity": "provisional",
		"utility":  0.4,
	})

	// File that would transition (established -> candidate demotion).
	writeLearning(t, dir, "demote.jsonl", map[string]any{
		"id":       "demote",
		"maturity": "established",
		"utility":  0.3,
	})

	// Invalid file (should be skipped).
	if err := os.WriteFile(filepath.Join(dir, "bad.jsonl"), []byte("not json"), 0644); err != nil {
		t.Fatalf("write bad file: %v", err)
	}

	results, err := ScanForMaturityTransitions(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}

	// Verify all returned results are transitions.
	for _, r := range results {
		if !r.Transitioned {
			t.Errorf("result for %q should have Transitioned=true", r.LearningID)
		}
	}
}

func TestScanForMaturityTransitions_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	results, err := ScanForMaturityTransitions(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("got %d results, want 0", len(results))
	}
}

func TestGetAntiPatterns(t *testing.T) {
	dir := t.TempDir()

	writeLearning(t, dir, "anti1.jsonl", map[string]any{
		"maturity": "anti-pattern",
	})
	writeLearning(t, dir, "anti2.jsonl", map[string]any{
		"maturity": "anti-pattern",
	})
	writeLearning(t, dir, "prov.jsonl", map[string]any{
		"maturity": "provisional",
	})
	writeLearning(t, dir, "est.jsonl", map[string]any{
		"maturity": "established",
	})

	// Invalid file should be skipped.
	if err := os.WriteFile(filepath.Join(dir, "bad.jsonl"), []byte("{bad"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	results, err := GetAntiPatterns(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("got %d anti-patterns, want 2", len(results))
	}
}

func TestGetAntiPatterns_EmptyDir(t *testing.T) {
	results, err := GetAntiPatterns(t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("got %d results, want 0", len(results))
	}
}

func TestGetEstablishedLearnings(t *testing.T) {
	dir := t.TempDir()

	writeLearning(t, dir, "est1.jsonl", map[string]any{
		"maturity": "established",
	})
	writeLearning(t, dir, "est2.jsonl", map[string]any{
		"maturity": "established",
	})
	writeLearning(t, dir, "cand.jsonl", map[string]any{
		"maturity": "candidate",
	})
	writeLearning(t, dir, "prov.jsonl", map[string]any{
		"maturity": "provisional",
	})

	results, err := GetEstablishedLearnings(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("got %d established learnings, want 2", len(results))
	}
}

func TestGetEstablishedLearnings_EmptyDir(t *testing.T) {
	results, err := GetEstablishedLearnings(t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("got %d results, want 0", len(results))
	}
}

func TestGetMaturityDistribution(t *testing.T) {
	dir := t.TempDir()

	writeLearning(t, dir, "p1.jsonl", map[string]any{"maturity": "provisional"})
	writeLearning(t, dir, "p2.jsonl", map[string]any{"maturity": "provisional"})
	writeLearning(t, dir, "c1.jsonl", map[string]any{"maturity": "candidate"})
	writeLearning(t, dir, "e1.jsonl", map[string]any{"maturity": "established"})
	writeLearning(t, dir, "e2.jsonl", map[string]any{"maturity": "established"})
	writeLearning(t, dir, "e3.jsonl", map[string]any{"maturity": "established"})
	writeLearning(t, dir, "a1.jsonl", map[string]any{"maturity": "anti-pattern"})
	writeLearning(t, dir, "u1.jsonl", map[string]any{"maturity": "bogus-value"})

	// Invalid JSON counts as unknown.
	if err := os.WriteFile(filepath.Join(dir, "bad.jsonl"), []byte("not json"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	dist, err := GetMaturityDistribution(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if dist.Provisional != 2 {
		t.Errorf("Provisional = %d, want 2", dist.Provisional)
	}
	if dist.Candidate != 1 {
		t.Errorf("Candidate = %d, want 1", dist.Candidate)
	}
	if dist.Established != 3 {
		t.Errorf("Established = %d, want 3", dist.Established)
	}
	if dist.AntiPattern != 1 {
		t.Errorf("AntiPattern = %d, want 1", dist.AntiPattern)
	}
	if dist.Unknown != 2 {
		t.Errorf("Unknown = %d, want 2 (bogus + bad json)", dist.Unknown)
	}
	if dist.Total != 9 {
		t.Errorf("Total = %d, want 9", dist.Total)
	}
}

func TestGetMaturityDistribution_DefaultsToProvisional(t *testing.T) {
	// A learning with no maturity field should count as provisional.
	dir := t.TempDir()
	writeLearning(t, dir, "nomat.jsonl", map[string]any{
		"id": "no-maturity",
	})

	dist, err := GetMaturityDistribution(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dist.Provisional != 1 {
		t.Errorf("Provisional = %d, want 1 (default for missing maturity)", dist.Provisional)
	}
	if dist.Total != 1 {
		t.Errorf("Total = %d, want 1", dist.Total)
	}
}

func TestGetMaturityDistribution_EmptyDir(t *testing.T) {
	dist, err := GetMaturityDistribution(t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dist.Total != 0 {
		t.Errorf("Total = %d, want 0", dist.Total)
	}
}

func TestStringFromData_RequireNonEmptyFallback(t *testing.T) {
	data := map[string]any{
		"field": "",
	}
	// requireNonEmpty=true with empty value should return default
	got := stringFromData(data, "field", "default-val", true)
	if got != "default-val" {
		t.Errorf("expected default-val for empty string with requireNonEmpty, got %q", got)
	}

	// requireNonEmpty=false with empty value should return ""
	got = stringFromData(data, "field", "default-val", false)
	if got != "" {
		t.Errorf("expected empty string with requireNonEmpty=false, got %q", got)
	}

	// Non-string value should return default
	data2 := map[string]any{
		"field": 42,
	}
	got = stringFromData(data2, "field", "default-val", false)
	if got != "default-val" {
		t.Errorf("expected default-val for non-string value, got %q", got)
	}
}

func TestApplyMaturityTransition_ReadOnlyFile(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root bypasses filesystem permissions")
	}
	dir := t.TempDir()
	path := writeLearning(t, dir, "readonly.jsonl", map[string]any{
		"id":           "readonly-test",
		"maturity":     "provisional",
		"utility":      0.8,
		"reward_count": 4.0,
	})
	// Make file read-only so write-back fails
	if err := os.Chmod(path, 0400); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0644) })

	_, err := ApplyMaturityTransition(path)
	if err == nil {
		t.Error("expected error when writing to read-only learning file")
	}
}

func TestGetEstablishedLearnings_UnreadableFile(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root bypasses filesystem permissions")
	}
	dir := t.TempDir()
	// Write a valid established file
	writeLearning(t, dir, "est.jsonl", map[string]any{
		"maturity": "established",
	})
	// Write an unreadable file
	unreadable := filepath.Join(dir, "unreadable.jsonl")
	if err := os.WriteFile(unreadable, []byte(`{"maturity":"established"}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(unreadable, 0000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(unreadable, 0644) })

	results, err := GetEstablishedLearnings(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should only find the readable one
	if len(results) != 1 {
		t.Errorf("expected 1 established learning (skipping unreadable), got %d", len(results))
	}
}

func TestGetAntiPatterns_UnreadableFile(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root bypasses filesystem permissions")
	}
	dir := t.TempDir()
	writeLearning(t, dir, "anti.jsonl", map[string]any{
		"maturity": "anti-pattern",
	})
	unreadable := filepath.Join(dir, "unreadable.jsonl")
	if err := os.WriteFile(unreadable, []byte(`{"maturity":"anti-pattern"}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(unreadable, 0000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(unreadable, 0644) })

	results, err := GetAntiPatterns(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 anti-pattern (skipping unreadable), got %d", len(results))
	}
}

func TestGetMaturityDistribution_UnreadableFile(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root bypasses filesystem permissions")
	}
	dir := t.TempDir()
	writeLearning(t, dir, "prov.jsonl", map[string]any{
		"maturity": "provisional",
	})
	unreadable := filepath.Join(dir, "unreadable.jsonl")
	if err := os.WriteFile(unreadable, []byte(`{"maturity":"provisional"}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(unreadable, 0000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(unreadable, 0644) })

	dist, err := GetMaturityDistribution(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Readable file counted as provisional, unreadable counted as unknown
	if dist.Provisional != 1 {
		t.Errorf("Provisional = %d, want 1", dist.Provisional)
	}
	if dist.Unknown != 1 {
		t.Errorf("Unknown = %d, want 1 (unreadable file)", dist.Unknown)
	}
	if dist.Total != 2 {
		t.Errorf("Total = %d, want 2 (1 readable + 1 unreadable)", dist.Total)
	}
}

func TestGetMaturityDistribution_EmptyMaturityField(t *testing.T) {
	dir := t.TempDir()
	writeLearning(t, dir, "empty-mat.jsonl", map[string]any{
		"id":       "test",
		"maturity": "",
	})

	dist, err := GetMaturityDistribution(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Empty maturity should default to provisional
	if dist.Provisional != 1 {
		t.Errorf("Provisional = %d, want 1 (default for empty maturity)", dist.Provisional)
	}
}

func TestApplyMaturityTransition_PromoteProvisionalToCandidate(t *testing.T) {
	tmpDir := t.TempDir()
	learningPath := filepath.Join(tmpDir, "learn-test.jsonl")

	// Create learning that meets promotion criteria:
	// utility >= 0.55, reward_count >= 3
	learning := `{"id":"L1","maturity":"provisional","utility":0.8,"reward_count":5,"confidence":0.9}`
	if err := os.WriteFile(learningPath, []byte(learning+"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := ApplyMaturityTransition(learningPath)
	if err != nil {
		t.Fatalf("ApplyMaturityTransition: %v", err)
	}
	if !result.Transitioned {
		t.Error("expected transition from provisional to candidate")
	}
	if result.NewMaturity != domain.MaturityCandidate {
		t.Errorf("NewMaturity = %s, want candidate", result.NewMaturity)
	}

	// Verify file was updated
	content, err := os.ReadFile(learningPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), `"candidate"`) {
		t.Errorf("expected file to contain 'candidate' maturity, got: %s", content)
	}
}

func TestApplyMaturityTransition_WriteError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root bypasses filesystem permissions")
	}
	tmpDir := t.TempDir()
	learningPath := filepath.Join(tmpDir, "learn-test.jsonl")

	// Create learning that meets promotion criteria
	learning := `{"id":"L1","maturity":"provisional","utility":0.8,"reward_count":5,"confidence":0.9}`
	if err := os.WriteFile(learningPath, []byte(learning+"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Make file read-only so WriteFile fails
	if err := os.Chmod(learningPath, 0400); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(learningPath, 0600) })

	_, err := ApplyMaturityTransition(learningPath)
	if err == nil {
		t.Error("expected error when writing to read-only file")
	}
	if !strings.Contains(err.Error(), "write updated learning") {
		t.Errorf("expected 'write updated learning' error, got: %v", err)
	}
}

func TestApplyMaturityTransition_NoTransition(t *testing.T) {
	tmpDir := t.TempDir()
	learningPath := filepath.Join(tmpDir, "learn-test.jsonl")

	// Create learning that does NOT meet transition criteria
	learning := `{"id":"L1","maturity":"provisional","utility":0.3,"reward_count":0,"confidence":0.5}`
	if err := os.WriteFile(learningPath, []byte(learning+"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := ApplyMaturityTransition(learningPath)
	if err != nil {
		t.Fatalf("ApplyMaturityTransition: %v", err)
	}
	if result.Transitioned {
		t.Error("expected no transition for low-utility learning")
	}
}

func TestScanForMaturityTransitions_NonExistentDir(t *testing.T) {
	// Non-existent dir should return empty results (glob returns nil for no matches)
	results, err := ScanForMaturityTransitions("/nonexistent/dir")
	if err != nil {
		t.Fatalf("ScanForMaturityTransitions: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results for nonexistent dir, got %d", len(results))
	}
}

func TestGetAntiPatterns_NonExistentDir(t *testing.T) {
	results, err := GetAntiPatterns("/nonexistent/dir")
	if err != nil {
		t.Fatalf("GetAntiPatterns: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results for nonexistent dir, got %d", len(results))
	}
}

func TestGetEstablishedLearnings_NonExistentDir(t *testing.T) {
	results, err := GetEstablishedLearnings("/nonexistent/dir")
	if err != nil {
		t.Fatalf("GetEstablishedLearnings: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results for nonexistent dir, got %d", len(results))
	}
}

func TestGetEstablishedLearnings_MalformedJSON(t *testing.T) {
	tmpDir := t.TempDir()
	// Write a malformed JSONL file
	if err := os.WriteFile(filepath.Join(tmpDir, "bad.jsonl"), []byte("{bad json\n"), 0644); err != nil {
		t.Fatal(err)
	}
	// Write a valid established learning
	if err := os.WriteFile(filepath.Join(tmpDir, "good.jsonl"), []byte(`{"id":"L1","maturity":"established"}`+"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	results, err := GetEstablishedLearnings(tmpDir)
	if err != nil {
		t.Fatalf("GetEstablishedLearnings: %v", err)
	}
	// Should get the good one, skip the bad one
	if len(results) != 1 {
		t.Errorf("expected 1 established learning (skip malformed), got %d", len(results))
	}
}

// --- updateJSONLFirstLine error paths ---

func TestUpdateJSONLFirstLine_ReadError(t *testing.T) {
	// Exercise line 244-246: os.ReadFile fails on nonexistent file.
	err := updateJSONLFirstLine("/nonexistent/learning.jsonl", map[string]any{"key": "val"})
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
	if !strings.Contains(err.Error(), "read learning for update") {
		t.Errorf("expected 'read learning for update' error, got: %v", err)
	}
}

func TestUpdateJSONLFirstLine_EmptyFile(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "empty.jsonl")
	if err := os.WriteFile(path, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	err := updateJSONLFirstLine(path, map[string]any{"key": "val"})
	if err == nil {
		t.Fatal("expected error for empty file")
	}
	if !errors.Is(err, ErrEmptyFile) {
		t.Errorf("expected ErrEmptyFile, got %v", err)
	}
}

func TestUpdateJSONLFirstLine_BadJSON(t *testing.T) {
	// Exercise line 254-256: bad JSON on first line.
	tmp := t.TempDir()
	path := filepath.Join(tmp, "bad.jsonl")
	if err := os.WriteFile(path, []byte("{bad json\n"), 0644); err != nil {
		t.Fatal(err)
	}

	err := updateJSONLFirstLine(path, map[string]any{"key": "val"})
	if err == nil {
		t.Error("expected error for bad JSON")
	}
	if !strings.Contains(err.Error(), "parse learning for update") {
		t.Errorf("expected 'parse learning for update' error, got: %v", err)
	}
}

func TestUpdateJSONLFirstLine_WriteError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root bypasses filesystem permissions")
	}
	// Exercise the write error path (line 268-270) by making the file read-only.
	tmp := t.TempDir()
	path := filepath.Join(tmp, "readonly.jsonl")
	if err := os.WriteFile(path, []byte(`{"id":"L1"}`+"\n"), 0444); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0644) })

	err := updateJSONLFirstLine(path, map[string]any{"key": "val"})
	if err == nil {
		t.Error("expected error writing to read-only file")
	}
	if !strings.Contains(err.Error(), "write updated learning") {
		t.Errorf("expected 'write updated learning' error, got: %v", err)
	}
}

func TestUpdateJSONLFirstLine_Success(t *testing.T) {
	// Exercise the success path: read, merge, write back.
	tmp := t.TempDir()
	path := filepath.Join(tmp, "learning.jsonl")
	if err := os.WriteFile(path, []byte(`{"id":"L1","maturity":"provisional"}`+"\nsecond line\n"), 0644); err != nil {
		t.Fatal(err)
	}

	err := updateJSONLFirstLine(path, map[string]any{"maturity": "candidate"})
	if err != nil {
		t.Fatalf("updateJSONLFirstLine: %v", err)
	}

	// Verify the file was updated
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), `"candidate"`) {
		t.Error("expected 'candidate' in updated file")
	}
	if !strings.Contains(string(content), "second line") {
		t.Error("expected second line preserved")
	}
}

// --- readFirstLineMaturity edge cases ---

func TestReadFirstLineMaturity_EmptyFile(t *testing.T) {
	// Exercise line 327-329: scanner.Scan() returns false on empty file.
	tmp := t.TempDir()
	path := filepath.Join(tmp, "empty.jsonl")
	if err := os.WriteFile(path, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	result := readFirstLineMaturity(path)
	if result != "" {
		t.Errorf("expected empty string for empty file, got %q", result)
	}
}

func TestReadFirstLineMaturity_BadJSON(t *testing.T) {
	// Exercise line 331-333: bad JSON returns "".
	tmp := t.TempDir()
	path := filepath.Join(tmp, "bad.jsonl")
	if err := os.WriteFile(path, []byte("{bad\n"), 0644); err != nil {
		t.Fatal(err)
	}

	result := readFirstLineMaturity(path)
	if result != "" {
		t.Errorf("expected empty string for bad JSON, got %q", result)
	}
}

func TestReadFirstLineMaturity_NoMaturityField(t *testing.T) {
	// Exercise line 334: data["maturity"] not a string.
	tmp := t.TempDir()
	path := filepath.Join(tmp, "no-maturity.jsonl")
	if err := os.WriteFile(path, []byte(`{"id":"L1"}`+"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	result := readFirstLineMaturity(path)
	if result != "" {
		t.Errorf("expected empty string for missing maturity field, got %q", result)
	}
}

func TestReadFirstLineMaturity_NonexistentFile(t *testing.T) {
	// Exercise line 319-321: os.Open fails.
	result := readFirstLineMaturity("/nonexistent/path.jsonl")
	if result != "" {
		t.Errorf("expected empty string for nonexistent file, got %q", result)
	}
}

// --- classifyLearningFile edge cases ---

func TestClassifyLearningFile_BadJSON(t *testing.T) {
	// Exercise line 386-389: bad JSON in scanner -> Unknown++.
	tmp := t.TempDir()
	path := filepath.Join(tmp, "bad.jsonl")
	if err := os.WriteFile(path, []byte("{bad json\n"), 0644); err != nil {
		t.Fatal(err)
	}

	dist := &MaturityDistribution{}
	classifyLearningFile(path, dist)
	if dist.Unknown != 1 {
		t.Errorf("expected Unknown=1, got %d", dist.Unknown)
	}
	if dist.Total != 1 {
		t.Errorf("expected Total=1, got %d", dist.Total)
	}
}

func TestClassifyLearningFile_EmptyFile(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "empty.jsonl")
	if err := os.WriteFile(path, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	dist := &MaturityDistribution{}
	classifyLearningFile(path, dist)
	// Empty file: readLearningData returns error, counts as unknown.
	if dist.Unknown != 1 {
		t.Errorf("expected Unknown=1 for empty file, got %d", dist.Unknown)
	}
	if dist.Total != 1 {
		t.Errorf("expected Total=1 for empty file, got %d", dist.Total)
	}
}

func TestClassifyLearningFile_NonexistentFile(t *testing.T) {
	// readLearningData returns error for nonexistent file, counts as unknown.
	dist := &MaturityDistribution{}
	classifyLearningFile("/nonexistent/file.jsonl", dist)
	if dist.Unknown != 1 {
		t.Errorf("expected Unknown=1 for nonexistent file, got %d", dist.Unknown)
	}
	if dist.Total != 1 {
		t.Errorf("expected Total=1 for nonexistent file, got %d", dist.Total)
	}
}

// contains is a test helper for substring matching.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstr(s, substr))
}

func containsSubstr(s, sub string) bool {
	for i := range len(s) - len(sub) + 1 {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// writeMDLearning is a test helper that writes a .md learning file with YAML frontmatter.
func writeMDLearning(t *testing.T, dir, name string, frontMatter map[string]string, body string) string {
	t.Helper()
	var sb strings.Builder
	sb.WriteString("---\n")
	for k, v := range frontMatter {
		fmt.Fprintf(&sb, "%s: %s\n", k, v)
	}
	sb.WriteString("---\n")
	sb.WriteString(body)
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(sb.String()), 0644); err != nil {
		t.Fatalf("write md learning: %v", err)
	}
	return path
}

func TestReadLearningData_YAMLFrontMatter(t *testing.T) {
	dir := t.TempDir()
	path := writeMDLearning(t, dir, "test.md", map[string]string{
		"id":           "learn-yaml-001",
		"utility":      "0.85",
		"maturity":     "provisional",
		"reward_count": "4",
	}, "# Some Learning\n\nBody text here.\n")

	data, err := readLearningData(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	id, ok := data["id"].(string)
	if !ok || id != "learn-yaml-001" {
		t.Errorf("id = %v, want %q", data["id"], "learn-yaml-001")
	}
	maturity, ok := data["maturity"].(string)
	if !ok || maturity != "provisional" {
		t.Errorf("maturity = %v, want %q", data["maturity"], "provisional")
	}
	// YAML unmarshals numbers — utility as float64, reward_count as int
	utility := floatFromData(data, "utility", 0)
	if utility != 0.85 {
		t.Errorf("utility = %v, want 0.85", utility)
	}
	rewardCount := intFromData(data, "reward_count")
	if rewardCount != 4 {
		t.Errorf("reward_count = %v, want 4", rewardCount)
	}
}

func TestCheckMaturityTransition_MDFile(t *testing.T) {
	dir := t.TempDir()
	path := writeMDLearning(t, dir, "promote.md", map[string]string{
		"id":           "md-promote",
		"maturity":     "provisional",
		"utility":      "0.8",
		"reward_count": "4",
	}, "# Learning\n")

	result, err := CheckMaturityTransition(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Transitioned {
		t.Error("expected transition from provisional to candidate")
	}
	if result.NewMaturity != domain.MaturityCandidate {
		t.Errorf("NewMaturity = %q, want %q", result.NewMaturity, domain.MaturityCandidate)
	}
	if result.OldMaturity != domain.MaturityProvisional {
		t.Errorf("OldMaturity = %q, want %q", result.OldMaturity, domain.MaturityProvisional)
	}
}

func TestScanForMaturityTransitions_IncludesMD(t *testing.T) {
	dir := t.TempDir()

	// JSONL file that would transition (provisional -> candidate)
	writeLearning(t, dir, "promote.jsonl", map[string]any{
		"id":           "jsonl-promote",
		"maturity":     "provisional",
		"utility":      0.8,
		"reward_count": 4.0,
	})

	// JSONL file that would NOT transition
	writeLearning(t, dir, "stay.jsonl", map[string]any{
		"id":       "jsonl-stay",
		"maturity": "provisional",
		"utility":  0.4,
	})

	// MD file that would transition (provisional -> candidate)
	writeMDLearning(t, dir, "promote.md", map[string]string{
		"id":           "md-promote",
		"maturity":     "provisional",
		"utility":      "0.9",
		"reward_count": "5",
	}, "# Learning\n")

	// MD file that would NOT transition
	writeMDLearning(t, dir, "stay.md", map[string]string{
		"id":       "md-stay",
		"maturity": "provisional",
		"utility":  "0.3",
	}, "# Learning\n")

	results, err := ScanForMaturityTransitions(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("got %d results, want 2 (1 jsonl + 1 md)", len(results))
	}

	// Verify both transitioned results are present
	ids := make(map[string]bool)
	for _, r := range results {
		if !r.Transitioned {
			t.Errorf("result for %q should have Transitioned=true", r.LearningID)
		}
		ids[r.LearningID] = true
	}
	if !ids["jsonl-promote"] {
		t.Error("expected jsonl-promote in results")
	}
	if !ids["md-promote"] {
		t.Error("expected md-promote in results")
	}
}

func TestApplyMaturityTransition_MDFile(t *testing.T) {
	dir := t.TempDir()
	path := writeMDLearning(t, dir, "promote.md", map[string]string{
		"id":           "md-apply",
		"maturity":     "provisional",
		"utility":      "0.8",
		"reward_count": "4",
	}, "# Learning\n\nBody text.\n")

	result, err := ApplyMaturityTransition(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Transitioned {
		t.Fatal("expected transition")
	}
	if result.NewMaturity != domain.MaturityCandidate {
		t.Errorf("NewMaturity = %q, want %q", result.NewMaturity, domain.MaturityCandidate)
	}

	// Verify the file was updated with new frontmatter
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read updated file: %v", err)
	}
	contentStr := string(content)
	if !strings.Contains(contentStr, "maturity: candidate") {
		t.Errorf("expected 'maturity: candidate' in updated file, got:\n%s", contentStr)
	}
	if !strings.Contains(contentStr, "maturity_changed_at:") {
		t.Error("expected maturity_changed_at in updated file")
	}
	if !strings.Contains(contentStr, "maturity_reason:") {
		t.Error("expected maturity_reason in updated file")
	}
	// Verify body is preserved
	if !strings.Contains(contentStr, "Body text.") {
		t.Error("expected body text preserved after front matter update")
	}
}

func TestGetMaturityDistribution_MDFiles(t *testing.T) {
	dir := t.TempDir()

	// MD files with different maturity values
	writeMDLearning(t, dir, "prov.md", map[string]string{
		"maturity": "provisional",
	}, "")
	writeMDLearning(t, dir, "cand.md", map[string]string{
		"maturity": "candidate",
	}, "")
	writeMDLearning(t, dir, "est.md", map[string]string{
		"maturity": "established",
	}, "")
	writeMDLearning(t, dir, "anti.md", map[string]string{
		"maturity": "anti-pattern",
	}, "")

	dist, err := GetMaturityDistribution(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if dist.Provisional != 1 {
		t.Errorf("Provisional = %d, want 1", dist.Provisional)
	}
	if dist.Candidate != 1 {
		t.Errorf("Candidate = %d, want 1", dist.Candidate)
	}
	if dist.Established != 1 {
		t.Errorf("Established = %d, want 1", dist.Established)
	}
	if dist.AntiPattern != 1 {
		t.Errorf("AntiPattern = %d, want 1", dist.AntiPattern)
	}
	if dist.Total != 4 {
		t.Errorf("Total = %d, want 4", dist.Total)
	}
}

func TestGlobLearningFiles_FindsBothFormats(t *testing.T) {
	dir := t.TempDir()

	// Create .jsonl files
	writeLearning(t, dir, "a.jsonl", map[string]any{"id": "a"})
	writeLearning(t, dir, "b.jsonl", map[string]any{"id": "b"})

	// Create .md files
	writeMDLearning(t, dir, "c.md", map[string]string{"id": "c"}, "# C\n")
	writeMDLearning(t, dir, "d.md", map[string]string{"id": "d"}, "# D\n")

	// Create a .txt file (should NOT be returned)
	if err := os.WriteFile(filepath.Join(dir, "ignored.txt"), []byte("not a learning"), 0644); err != nil {
		t.Fatal(err)
	}

	files, err := GlobLearningFiles(dir)
	if err != nil {
		t.Fatalf("GlobLearningFiles: %v", err)
	}
	if len(files) != 4 {
		t.Errorf("got %d files, want 4 (2 jsonl + 2 md)", len(files))
	}
}

func TestGlobLearningFiles_EmptyDir(t *testing.T) {
	files, err := GlobLearningFiles(t.TempDir())
	if err != nil {
		t.Fatalf("GlobLearningFiles: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("got %d files, want 0", len(files))
	}
}

func TestGlobLearningFiles_NonExistentDir(t *testing.T) {
	// filepath.Glob returns nil (not error) for non-existent dirs
	files, err := GlobLearningFiles("/nonexistent/dir")
	if err != nil {
		t.Fatalf("GlobLearningFiles: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("got %d files, want 0", len(files))
	}
}

func TestFilterLearningsByMaturity_IncludesMD(t *testing.T) {
	dir := t.TempDir()

	// JSONL files
	writeLearning(t, dir, "est1.jsonl", map[string]any{
		"maturity": "established",
	})
	writeLearning(t, dir, "prov1.jsonl", map[string]any{
		"maturity": "provisional",
	})

	// MD files
	writeMDLearning(t, dir, "est2.md", map[string]string{
		"maturity": "established",
	}, "")
	writeMDLearning(t, dir, "cand1.md", map[string]string{
		"maturity": "candidate",
	}, "")

	results, err := filterLearningsByMaturity(dir, domain.MaturityEstablished)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("got %d results, want 2 (1 jsonl + 1 md)", len(results))
	}

	// Verify both established files are found
	hasJSONL := false
	hasMD := false
	for _, r := range results {
		if strings.HasSuffix(r, ".jsonl") {
			hasJSONL = true
		}
		if strings.HasSuffix(r, ".md") {
			hasMD = true
		}
	}
	if !hasJSONL {
		t.Error("expected .jsonl file in established results")
	}
	if !hasMD {
		t.Error("expected .md file in established results")
	}
}

// --- GlobLearningFiles error paths ---

func TestGlobLearningFiles_BadPatternJSONL(t *testing.T) {
	// A directory name containing '[' causes filepath.Glob to return ErrBadPattern
	// because the resulting pattern like "[bad/*.jsonl" is invalid.
	badDir := filepath.Join(t.TempDir(), "[bad")
	if err := os.MkdirAll(badDir, 0755); err != nil {
		t.Fatal(err)
	}

	files, err := GlobLearningFiles(badDir)
	if err == nil {
		t.Fatal("expected error from GlobLearningFiles with bad pattern dir")
	}
	if !strings.Contains(err.Error(), "glob jsonl") {
		t.Errorf("expected 'glob jsonl' in error, got: %v", err)
	}
	if files != nil {
		t.Errorf("expected nil files on error, got %v", files)
	}
}

// --- ScanForMaturityTransitions error path ---

func TestScanForMaturityTransitions_GlobError(t *testing.T) {
	badDir := filepath.Join(t.TempDir(), "[invalid")
	if err := os.MkdirAll(badDir, 0755); err != nil {
		t.Fatal(err)
	}

	results, err := ScanForMaturityTransitions(badDir)
	if err == nil {
		t.Fatal("expected error from ScanForMaturityTransitions with bad dir")
	}
	if !strings.Contains(err.Error(), "glob learnings") {
		t.Errorf("expected 'glob learnings' in error, got: %v", err)
	}
	if results != nil {
		t.Errorf("expected nil results on error, got %v", results)
	}
}

// --- filterLearningsByMaturity error path ---

func TestFilterLearningsByMaturity_GlobError(t *testing.T) {
	badDir := filepath.Join(t.TempDir(), "[invalid")
	if err := os.MkdirAll(badDir, 0755); err != nil {
		t.Fatal(err)
	}

	results, err := filterLearningsByMaturity(badDir, domain.MaturityProvisional)
	if err == nil {
		t.Fatal("expected error from filterLearningsByMaturity with bad dir")
	}
	if !strings.Contains(err.Error(), "glob learnings") {
		t.Errorf("expected 'glob learnings' in error, got: %v", err)
	}
	if results != nil {
		t.Errorf("expected nil results on error, got %v", results)
	}
}

// --- GetMaturityDistribution error path ---

func TestGetMaturityDistribution_GlobError(t *testing.T) {
	badDir := filepath.Join(t.TempDir(), "[invalid")
	if err := os.MkdirAll(badDir, 0755); err != nil {
		t.Fatal(err)
	}

	dist, err := GetMaturityDistribution(badDir)
	if err == nil {
		t.Fatal("expected error from GetMaturityDistribution with bad dir")
	}
	if !strings.Contains(err.Error(), "glob learnings") {
		t.Errorf("expected 'glob learnings' in error, got: %v", err)
	}
	if dist != nil {
		t.Errorf("expected nil distribution on error, got %v", dist)
	}
}

// --- mergeJSONData marshal error ---

func TestMergeJSONData_MarshalError(t *testing.T) {
	// json.Marshal fails on values like math.Inf or math.NaN
	firstLine := `{"id":"test","maturity":"provisional"}`
	updates := map[string]any{
		"utility": math.Inf(1), // +Inf is not valid JSON
	}

	_, err := mergeJSONData(firstLine, updates)
	if err == nil {
		t.Fatal("expected error from mergeJSONData when updates contain non-marshalable value")
	}
	if !strings.Contains(err.Error(), "marshal updated learning") {
		t.Errorf("expected 'marshal updated learning' in error, got: %v", err)
	}
}

// --- mergeJSONData unmarshal error ---

func TestMergeJSONData_UnmarshalError(t *testing.T) {
	_, err := mergeJSONData("not-json", map[string]any{"key": "val"})
	if err == nil {
		t.Fatal("expected error from mergeJSONData with invalid JSON input")
	}
	if !strings.Contains(err.Error(), "parse learning for update") {
		t.Errorf("expected 'parse learning for update' in error, got: %v", err)
	}
}
