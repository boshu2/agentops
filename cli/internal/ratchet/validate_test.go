package ratchet

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/types"
)

func TestValidateArtifactPath(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{"empty is valid", "", false},
		{"absolute unix path", "/home/user/workspace/research.md", false},
		{"absolute root path", "/tmp/file.txt", false},
		{"relative dot path", "./research.md", true},
		{"relative parent path", "../research.md", true},
		{"tilde path", "~/gt/research.md", true},
		{"no leading slash", "gt/research.md", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateArtifactPath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateArtifactPath(%q) error = %v, wantErr %v", tt.path, err, tt.wantErr)
			}
		})
	}
}

func TestExtractArtifactPaths(t *testing.T) {
	tests := []struct {
		name        string
		closeReason string
		wantPaths   []string
	}{
		{
			name:        "empty",
			closeReason: "",
			wantPaths:   nil,
		},
		{
			name:        "artifact with absolute path",
			closeReason: "Complete. Artifact: /home/user/workspace/research.md",
			wantPaths:   []string{"/home/user/workspace/research.md"},
		},
		{
			name:        "see with absolute path",
			closeReason: "Fixed. See /path/to/file.go:123",
			wantPaths:   []string{"/path/to/file.go:123"},
		},
		{
			name:        "multiple paths",
			closeReason: "Done. Artifact: /path/one.md See /path/two.go",
			wantPaths:   []string{"/path/one.md", "/path/two.go"},
		},
		{
			name:        "case insensitive",
			closeReason: "ARTIFACT: /upper/case.md artifact: /lower/case.md",
			wantPaths:   []string{"/upper/case.md", "/lower/case.md"},
		},
		{
			name:        "no path keywords",
			closeReason: "Done, tests passing",
			wantPaths:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractArtifactPaths(tt.closeReason)
			if len(got) != len(tt.wantPaths) {
				t.Errorf("ExtractArtifactPaths() = %v, want %v", got, tt.wantPaths)
				return
			}
			for i, path := range got {
				if path != tt.wantPaths[i] {
					t.Errorf("ExtractArtifactPaths()[%d] = %q, want %q", i, path, tt.wantPaths[i])
				}
			}
		})
	}
}

func TestValidateCloseReason(t *testing.T) {
	tests := []struct {
		name        string
		closeReason string
		wantIssues  int
	}{
		{
			name:        "valid absolute path",
			closeReason: "Complete. Artifact: /home/user/workspace/research.md",
			wantIssues:  0,
		},
		{
			name:        "no paths referenced",
			closeReason: "Done, tests passing",
			wantIssues:  0,
		},
		{
			name:        "relative path with dot",
			closeReason: "Artifact: ./research.md",
			wantIssues:  1,
		},
		{
			name:        "tilde path",
			closeReason: "Artifact: ~/gt/file.md",
			wantIssues:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issues := ValidateCloseReason(tt.closeReason)
			if len(issues) != tt.wantIssues {
				t.Errorf("ValidateCloseReason() found %d issues, want %d: %v", len(issues), tt.wantIssues, issues)
			}
		})
	}
}

func TestValidateCloseReason_ParentPath(t *testing.T) {
	issues := ValidateCloseReason("See ../parent/file.md")
	if len(issues) == 0 {
		t.Error("expected issue for ../ relative path, got none")
	}
}

// --- formatDays ---

func TestFormatDays(t *testing.T) {
	tests := []struct {
		days int
		want string
	}{
		{0, "today"},
		{1, "1 day"},
		{5, "5 days"},
		{30, "30 days"},
		{90, "90 days"},
	}

	for _, tt := range tests {
		got := formatDays(tt.days)
		if got != tt.want {
			t.Errorf("formatDays(%d) = %q, want %q", tt.days, got, tt.want)
		}
	}
}

// --- getLenientExpiry ---

func TestGetLenientExpiry(t *testing.T) {
	t.Run("not lenient returns nil", func(t *testing.T) {
		opts := &ValidateOptions{Lenient: false}
		got := getLenientExpiry(opts)
		if got != nil {
			t.Errorf("expected nil for non-lenient, got %v", got)
		}
	})

	t.Run("lenient with explicit date", func(t *testing.T) {
		expiry := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
		opts := &ValidateOptions{Lenient: true, LenientExpiryDate: &expiry}
		got := getLenientExpiry(opts)
		if got == nil {
			t.Fatal("expected non-nil expiry")
		}
		if !got.Equal(expiry) {
			t.Errorf("expected %v, got %v", expiry, *got)
		}
	})

	t.Run("lenient with default date", func(t *testing.T) {
		opts := &ValidateOptions{Lenient: true}
		before := time.Now().AddDate(0, 0, lenientExpiryDefaultDays)
		got := getLenientExpiry(opts)
		after := time.Now().AddDate(0, 0, lenientExpiryDefaultDays)
		if got == nil {
			t.Fatal("expected non-nil expiry")
		}
		if got.Before(before) || got.After(after) {
			t.Errorf("default expiry %v not in expected range [%v, %v]", *got, before, after)
		}
	})
}

// --- checkLenientExpiry ---

func TestCheckLenientExpiry(t *testing.T) {
	t.Run("nil expiry continues", func(t *testing.T) {
		result := &ValidationResult{Valid: true, Issues: []string{}, Warnings: []string{}}
		cont := checkLenientExpiry(nil, result)
		if !cont {
			t.Error("expected continue=true for nil expiry")
		}
		if !result.Valid {
			t.Error("expected valid=true")
		}
	})

	t.Run("expired fails validation", func(t *testing.T) {
		past := time.Now().AddDate(0, 0, -1)
		result := &ValidationResult{Valid: true, Issues: []string{}, Warnings: []string{}}
		cont := checkLenientExpiry(&past, result)
		if cont {
			t.Error("expected continue=false for expired")
		}
		if result.Valid {
			t.Error("expected valid=false for expired")
		}
		if result.Lenient {
			t.Error("expected lenient=false after expiry")
		}
		if len(result.Issues) == 0 {
			t.Error("expected at least one issue for expired")
		}
	})

	t.Run("expiring soon warns", func(t *testing.T) {
		soon := time.Now().AddDate(0, 0, 15)
		result := &ValidationResult{Valid: true, Issues: []string{}, Warnings: []string{}}
		cont := checkLenientExpiry(&soon, result)
		if !cont {
			t.Error("expected continue=true for expiring-soon")
		}
		if !result.LenientExpiringSoon {
			t.Error("expected LenientExpiringSoon=true")
		}
		if len(result.Warnings) == 0 {
			t.Error("expected at least one warning for expiring-soon")
		}
	})

	t.Run("far future no warning", func(t *testing.T) {
		future := time.Now().AddDate(0, 0, 60)
		result := &ValidationResult{Valid: true, Issues: []string{}, Warnings: []string{}}
		cont := checkLenientExpiry(&future, result)
		if !cont {
			t.Error("expected continue=true")
		}
		if result.LenientExpiringSoon {
			t.Error("expected LenientExpiringSoon=false for far future")
		}
		if len(result.Warnings) != 0 {
			t.Errorf("expected no warnings, got %v", result.Warnings)
		}
	})
}

// --- TierToReward / RewardToTier ---

func TestTierToReward(t *testing.T) {
	tests := []struct {
		tier Tier
		want float64
	}{
		{TierCore, 1.0},
		{TierSkill, 0.9},
		{TierPattern, 0.75},
		{TierLearning, 0.5},
		{TierObservation, 0.25},
		{Tier(99), 0.0},
	}

	for _, tt := range tests {
		got := TierToReward(tt.tier)
		if got != tt.want {
			t.Errorf("TierToReward(%v) = %f, want %f", tt.tier, got, tt.want)
		}
	}
}

func TestRewardToTier(t *testing.T) {
	tests := []struct {
		reward float64
		want   Tier
	}{
		{1.0, TierCore},
		{0.95, TierCore},
		{0.9, TierSkill},
		{0.8, TierSkill},
		{0.75, TierPattern},
		{0.6, TierPattern},
		{0.5, TierLearning},
		{0.35, TierLearning},
		{0.2, TierObservation},
		{0.0, TierObservation},
		{-1.0, TierObservation},
	}

	for _, tt := range tests {
		got := RewardToTier(tt.reward)
		if got != tt.want {
			t.Errorf("RewardToTier(%f) = %v, want %v", tt.reward, got, tt.want)
		}
	}
}

// --- TierFromValidation ---

func TestTierFromValidation(t *testing.T) {
	t.Run("explicit tier takes precedence", func(t *testing.T) {
		tier := TierSkill
		result := &ValidationResult{Valid: true, Tier: &tier, Issues: []string{"issue"}, Warnings: []string{"warn"}}
		got := TierFromValidation(result)
		if got != TierSkill {
			t.Errorf("expected TierSkill, got %v", got)
		}
	})

	t.Run("invalid result is observation", func(t *testing.T) {
		result := &ValidationResult{Valid: false, Issues: []string{}, Warnings: []string{}}
		got := TierFromValidation(result)
		if got != TierObservation {
			t.Errorf("expected TierObservation, got %v", got)
		}
	})

	t.Run("no issues or warnings is pattern", func(t *testing.T) {
		result := &ValidationResult{Valid: true, Issues: []string{}, Warnings: []string{}}
		got := TierFromValidation(result)
		if got != TierPattern {
			t.Errorf("expected TierPattern, got %v", got)
		}
	})

	t.Run("few warnings is learning", func(t *testing.T) {
		result := &ValidationResult{Valid: true, Issues: []string{}, Warnings: []string{"w1", "w2"}}
		got := TierFromValidation(result)
		if got != TierLearning {
			t.Errorf("expected TierLearning, got %v", got)
		}
	})

	t.Run("many warnings is observation", func(t *testing.T) {
		result := &ValidationResult{Valid: true, Issues: []string{}, Warnings: []string{"w1", "w2", "w3"}}
		got := TierFromValidation(result)
		if got != TierObservation {
			t.Errorf("expected TierObservation, got %v", got)
		}
	})
}

// --- RecordCitation / LoadCitations / CountCitationsForArtifact ---

func TestRecordAndLoadCitations(t *testing.T) {
	baseDir := t.TempDir()

	event := types.CitationEvent{
		ArtifactPath: "/path/to/artifact.md",
		SessionID:    "sess-001",
		CitedAt:      time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC),
		CitationType: "retrieved",
	}

	// Record a citation
	if err := RecordCitation(baseDir, event); err != nil {
		t.Fatalf("RecordCitation failed: %v", err)
	}

	// Record another citation for the same artifact
	event2 := types.CitationEvent{
		ArtifactPath: "/path/to/artifact.md",
		SessionID:    "sess-002",
		CitedAt:      time.Date(2026, 1, 16, 10, 0, 0, 0, time.UTC),
		CitationType: "applied",
	}
	if err := RecordCitation(baseDir, event2); err != nil {
		t.Fatalf("RecordCitation (2nd) failed: %v", err)
	}

	// Record a different artifact
	event3 := types.CitationEvent{
		ArtifactPath: "/path/to/other.md",
		SessionID:    "sess-001",
		CitedAt:      time.Date(2026, 1, 17, 10, 0, 0, 0, time.UTC),
	}
	if err := RecordCitation(baseDir, event3); err != nil {
		t.Fatalf("RecordCitation (3rd) failed: %v", err)
	}

	// Load all citations
	citations, err := LoadCitations(baseDir)
	if err != nil {
		t.Fatalf("LoadCitations failed: %v", err)
	}
	if len(citations) != 3 {
		t.Errorf("expected 3 citations, got %d", len(citations))
	}

	// Count for specific artifact
	count, err := CountCitationsForArtifact(baseDir, "/path/to/artifact.md")
	if err != nil {
		t.Fatalf("CountCitationsForArtifact failed: %v", err)
	}
	if count != 2 {
		t.Errorf("expected count=2, got %d", count)
	}

	// Count for nonexistent artifact
	count, err = CountCitationsForArtifact(baseDir, "/nonexistent.md")
	if err != nil {
		t.Fatalf("CountCitationsForArtifact failed: %v", err)
	}
	if count != 0 {
		t.Errorf("expected count=0, got %d", count)
	}
}

func TestLoadCitations_NoFile(t *testing.T) {
	baseDir := t.TempDir()
	citations, err := LoadCitations(baseDir)
	if err != nil {
		t.Fatalf("expected no error for missing file, got %v", err)
	}
	if citations != nil {
		t.Errorf("expected nil citations for missing file, got %v", citations)
	}
}

func TestLoadCitations_MalformedLines(t *testing.T) {
	baseDir := t.TempDir()
	citationsDir := filepath.Join(baseDir, ".agents", "ao")
	if err := os.MkdirAll(citationsDir, 0700); err != nil {
		t.Fatal(err)
	}

	// Write a file with a valid line and a malformed line
	validEvent := types.CitationEvent{
		ArtifactPath: "/path/valid.md",
		SessionID:    "sess-001",
		CitedAt:      time.Now(),
	}
	data, _ := json.Marshal(validEvent)
	content := string(data) + "\n{invalid json}\n"
	if err := os.WriteFile(filepath.Join(citationsDir, "citations.jsonl"), []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	citations, err := LoadCitations(baseDir)
	if err != nil {
		t.Fatalf("expected no error (skip malformed), got %v", err)
	}
	if len(citations) != 1 {
		t.Errorf("expected 1 valid citation (skipping malformed), got %d", len(citations))
	}
}

func TestRecordCitation_DefaultTimestamp(t *testing.T) {
	baseDir := t.TempDir()

	// Zero timestamp should be filled in
	event := types.CitationEvent{
		ArtifactPath: "/path/auto-ts.md",
		SessionID:    "sess-auto",
	}
	before := time.Now()
	if err := RecordCitation(baseDir, event); err != nil {
		t.Fatalf("RecordCitation failed: %v", err)
	}
	after := time.Now()

	citations, err := LoadCitations(baseDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(citations) != 1 {
		t.Fatalf("expected 1 citation, got %d", len(citations))
	}
	if citations[0].CitedAt.Before(before) || citations[0].CitedAt.After(after) {
		t.Errorf("auto-timestamp %v not in expected range", citations[0].CitedAt)
	}
}

func TestCitationPathCanonicalization_MixedRelativeAndAbsolute(t *testing.T) {
	baseDir := t.TempDir()
	rel := ".agents/learnings/L1.md"
	abs := filepath.Join(baseDir, ".agents", "learnings", "L1.md")

	events := []types.CitationEvent{
		{ArtifactPath: rel, SessionID: "s1", CitedAt: time.Now()},
		{ArtifactPath: abs, SessionID: "s2", CitedAt: time.Now().Add(1 * time.Minute)},
	}
	for _, e := range events {
		if err := RecordCitation(baseDir, e); err != nil {
			t.Fatalf("RecordCitation failed: %v", err)
		}
	}

	count, err := CountCitationsForArtifact(baseDir, abs)
	if err != nil {
		t.Fatalf("CountCitationsForArtifact failed: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected canonical count=2, got %d", count)
	}

	since := time.Now().Add(-1 * time.Hour)
	until := time.Now().Add(1 * time.Hour)
	unique, err := GetUniqueCitedArtifacts(baseDir, since, until)
	if err != nil {
		t.Fatalf("GetUniqueCitedArtifacts failed: %v", err)
	}
	if len(unique) != 1 {
		t.Fatalf("expected 1 unique canonical artifact, got %d (%v)", len(unique), unique)
	}
}

// --- GetCitationsSince ---

func TestGetCitationsSince(t *testing.T) {
	baseDir := t.TempDir()

	t1 := time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 1, 20, 0, 0, 0, 0, time.UTC)
	t3 := time.Date(2026, 1, 30, 0, 0, 0, 0, time.UTC)

	for _, ts := range []time.Time{t1, t2, t3} {
		if err := RecordCitation(baseDir, types.CitationEvent{
			ArtifactPath: "/path/file.md",
			SessionID:    "s1",
			CitedAt:      ts,
		}); err != nil {
			t.Fatal(err)
		}
	}

	since := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
	got, err := GetCitationsSince(baseDir, since)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Errorf("expected 2 citations since %v, got %d", since, len(got))
	}
}

// --- GetUniqueCitedArtifacts ---

func TestGetUniqueCitedArtifacts(t *testing.T) {
	baseDir := t.TempDir()

	t1 := time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 1, 20, 0, 0, 0, 0, time.UTC)
	t3 := time.Date(2026, 1, 25, 0, 0, 0, 0, time.UTC)

	events := []types.CitationEvent{
		{ArtifactPath: "/a.md", SessionID: "s1", CitedAt: t1},
		{ArtifactPath: "/b.md", SessionID: "s1", CitedAt: t2},
		{ArtifactPath: "/a.md", SessionID: "s2", CitedAt: t3}, // duplicate artifact
	}
	for _, e := range events {
		if err := RecordCitation(baseDir, e); err != nil {
			t.Fatal(err)
		}
	}

	since := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
	until := time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC)
	got, err := GetUniqueCitedArtifacts(baseDir, since, until)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Errorf("expected 2 unique artifacts, got %d: %v", len(got), got)
	}
}

func TestGetUniqueCitedArtifacts_NoFile(t *testing.T) {
	baseDir := t.TempDir()
	got, err := GetUniqueCitedArtifacts(baseDir,
		time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Errorf("expected nil for missing file, got %v", got)
	}
}

// --- GetCitationsForSession ---

func TestGetCitationsForSession(t *testing.T) {
	baseDir := t.TempDir()

	events := []types.CitationEvent{
		{ArtifactPath: "/a.md", SessionID: "sess-001", CitedAt: time.Now()},
		{ArtifactPath: "/b.md", SessionID: "sess-002", CitedAt: time.Now()},
		{ArtifactPath: "/c.md", SessionID: "sess-001", CitedAt: time.Now()},
	}
	for _, e := range events {
		if err := RecordCitation(baseDir, e); err != nil {
			t.Fatal(err)
		}
	}

	got, err := GetCitationsForSession(baseDir, "sess-001")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Errorf("expected 2 citations for sess-001, got %d", len(got))
	}

	got, err = GetCitationsForSession(baseDir, "nonexistent")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("expected 0 citations for nonexistent session, got %d", len(got))
	}
}

// --- Validator (Validate, ValidateWithOptions, assessTier, GetMetrics) ---

// helperNewValidator creates a Validator using t.TempDir.
func helperNewValidator(t *testing.T) (*Validator, string) {
	t.Helper()
	tmpDir := t.TempDir()
	v, err := NewValidator(tmpDir)
	if err != nil {
		t.Fatalf("NewValidator failed: %v", err)
	}
	return v, tmpDir
}

func TestNewValidator(t *testing.T) {
	v, _ := helperNewValidator(t)
	if v == nil {
		t.Fatal("expected non-nil validator")
	}
	if v.metrics == nil {
		t.Fatal("expected non-nil metrics")
	}
}

func TestValidate_ArtifactNotFound(t *testing.T) {
	v, _ := helperNewValidator(t)
	result, err := v.Validate(StepResearch, "/nonexistent/path.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Valid {
		t.Error("expected valid=false for missing artifact")
	}
	if len(result.Issues) == 0 {
		t.Error("expected at least one issue")
	}
}

func TestValidateWithOptions_NilOpts(t *testing.T) {
	v, tmpDir := helperNewValidator(t)
	// Create a minimal artifact with schema_version
	artifact := filepath.Join(tmpDir, "research.md")
	content := "---\nschema_version: 1\n---\n## Summary\ntest\n## Key Findings\nfoo\n## Recommendations\nbar\nSource: http://example.com\n" + longText(120)
	if err := os.WriteFile(artifact, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	// Should not panic with nil opts
	result, err := v.ValidateWithOptions(StepResearch, artifact, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Valid {
		t.Errorf("expected valid=true, issues=%v", result.Issues)
	}
}

func TestValidate_ResearchStep(t *testing.T) {
	v, tmpDir := helperNewValidator(t)

	t.Run("good research", func(t *testing.T) {
		artifact := filepath.Join(tmpDir, "research-good.md")
		content := "---\nschema_version: 1\n---\n## Summary\nSome summary.\n## Key Findings\nFindings here.\n## Recommendations\nDo this.\nSource: http://example.com\n" + longText(120)
		if err := os.WriteFile(artifact, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
		result, err := v.Validate(StepResearch, artifact)
		if err != nil {
			t.Fatal(err)
		}
		if !result.Valid {
			t.Errorf("expected valid, issues=%v", result.Issues)
		}
	})

	t.Run("missing sections warns", func(t *testing.T) {
		artifact := filepath.Join(tmpDir, "research-missing.md")
		content := "---\nschema_version: 1\n---\nJust some text without sections.\nSource: http://example.com\n" + longText(120)
		if err := os.WriteFile(artifact, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
		result, err := v.Validate(StepResearch, artifact)
		if err != nil {
			t.Fatal(err)
		}
		// Should have warnings for missing sections
		if len(result.Warnings) == 0 {
			t.Error("expected warnings for missing sections")
		}
	})

	t.Run("no sources warns", func(t *testing.T) {
		artifact := filepath.Join(tmpDir, "research-nosrc.md")
		content := "---\nschema_version: 1\n---\n## Summary\nSome summary.\n## Key Findings\nFindings.\n## Recommendations\nDo this.\n" + longText(120)
		if err := os.WriteFile(artifact, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
		result, err := v.Validate(StepResearch, artifact)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, w := range result.Warnings {
			if w == "No sources or references found" {
				found = true
			}
		}
		if !found {
			t.Errorf("expected 'No sources or references found' warning, got %v", result.Warnings)
		}
	})
}

func TestValidate_PlanStep(t *testing.T) {
	v, tmpDir := helperNewValidator(t)

	t.Run("good plan", func(t *testing.T) {
		artifact := filepath.Join(tmpDir, "plan-good.md")
		content := "---\nschema_version: 1\n---\n## Objective\nBuild feature.\n## Tasks\n- task 1\n## Success Criteria\n- criteria 1\n"
		if err := os.WriteFile(artifact, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
		result, err := v.Validate(StepPlan, artifact)
		if err != nil {
			t.Fatal(err)
		}
		if !result.Valid {
			t.Errorf("expected valid, issues=%v", result.Issues)
		}
	})

	t.Run("missing sections warns", func(t *testing.T) {
		artifact := filepath.Join(tmpDir, "plan-missing.md")
		content := "---\nschema_version: 1\n---\nJust a description with no sections.\n"
		if err := os.WriteFile(artifact, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
		result, err := v.Validate(StepPlan, artifact)
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Warnings) < 3 {
			t.Errorf("expected at least 3 warnings for missing plan sections, got %d: %v", len(result.Warnings), result.Warnings)
		}
	})

	t.Run("epic prefix fails stat", func(t *testing.T) {
		// epic: prefix paths fail os.Stat in ValidateWithOptions before
		// reaching validatePlan, so they come back as artifact-not-found.
		result, err := v.Validate(StepPlan, "epic:ag-0001")
		if err != nil {
			t.Fatal(err)
		}
		if result.Valid {
			t.Error("expected valid=false since epic: path is not a real file")
		}
	})

	t.Run("epic empty ID fails stat", func(t *testing.T) {
		// Like any epic: path, fails at os.Stat before reaching validatePlan
		result, err := v.Validate(StepPlan, "epic:")
		if err != nil {
			t.Fatal(err)
		}
		if result.Valid {
			t.Error("expected valid=false for empty epic ID")
		}
	})

	t.Run("toml formula strict missing schema_version", func(t *testing.T) {
		// TOML uses "schema_version = 1" but hasSchemaVersion looks for
		// "schema_version:" (YAML) or "\"schema_version\"" (JSON).
		// So strict mode rejects TOML files without those patterns.
		artifact := filepath.Join(tmpDir, "plan.toml")
		content := "formula = \"test\"\ndescription = \"A test\"\nversion = \"1.0\"\ntype = \"epic\"\nschema_version = 1\n\n[[steps]]\nname = \"step1\"\n"
		if err := os.WriteFile(artifact, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
		result, err := v.Validate(StepPlan, artifact)
		if err != nil {
			t.Fatal(err)
		}
		if result.Valid {
			t.Error("expected valid=false since TOML schema_version format not detected by strict mode")
		}
	})

	t.Run("toml formula lenient", func(t *testing.T) {
		artifact := filepath.Join(tmpDir, "plan-lenient.toml")
		content := "formula = \"test\"\ndescription = \"A test\"\nversion = \"1.0\"\ntype = \"epic\"\nschema_version = 1\n\n[[steps]]\nname = \"step1\"\n"
		if err := os.WriteFile(artifact, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
		result, err := v.ValidateWithOptions(StepPlan, artifact, &ValidateOptions{Lenient: true})
		if err != nil {
			t.Fatal(err)
		}
		if !result.Valid {
			t.Errorf("expected valid=true in lenient mode, issues=%v", result.Issues)
		}
	})
}

func TestValidate_PreMortemStep(t *testing.T) {
	v, tmpDir := helperNewValidator(t)

	t.Run("good pre-mortem", func(t *testing.T) {
		artifact := filepath.Join(tmpDir, "premortem-v2.md")
		content := "---\nschema_version: 1\n---\n| ID | Finding | Mitigation |\n|---|---|---|\n| 1 | Bad thing | Fix it |\n"
		if err := os.WriteFile(artifact, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
		result, err := v.Validate(StepPreMortem, artifact)
		if err != nil {
			t.Fatal(err)
		}
		if !result.Valid {
			t.Errorf("expected valid, issues=%v", result.Issues)
		}
	})

	t.Run("missing version suffix warns", func(t *testing.T) {
		artifact := filepath.Join(tmpDir, "premortem.md")
		content := "---\nschema_version: 1\n---\nFinding: something\nMitigation: fix\n"
		if err := os.WriteFile(artifact, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
		result, err := v.Validate(StepPreMortem, artifact)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, w := range result.Warnings {
			if w == "Filename should include version suffix (e.g., -v2.md)" {
				found = true
			}
		}
		if !found {
			t.Errorf("expected version suffix warning, got %v", result.Warnings)
		}
	})
}

func TestValidate_PostMortemStep(t *testing.T) {
	v, tmpDir := helperNewValidator(t)

	t.Run("good post-mortem", func(t *testing.T) {
		artifact := filepath.Join(tmpDir, "postmortem.md")
		content := "---\nschema_version: 1\n---\n## Learnings\nLearned stuff.\n## Patterns\nUseful patterns.\n## Next Steps\nDo more.\n"
		if err := os.WriteFile(artifact, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
		result, err := v.Validate(StepPostMortem, artifact)
		if err != nil {
			t.Fatal(err)
		}
		if !result.Valid {
			t.Errorf("expected valid, issues=%v", result.Issues)
		}
	})

	t.Run("missing sections warns", func(t *testing.T) {
		artifact := filepath.Join(tmpDir, "postmortem-bare.md")
		content := "---\nschema_version: 1\n---\nJust some text.\n"
		if err := os.WriteFile(artifact, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
		result, err := v.Validate(StepPostMortem, artifact)
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Warnings) < 3 {
			t.Errorf("expected at least 3 warnings, got %d: %v", len(result.Warnings), result.Warnings)
		}
	})
}

func TestValidate_UnknownStep(t *testing.T) {
	v, tmpDir := helperNewValidator(t)
	artifact := filepath.Join(tmpDir, "unknown.md")
	content := "---\nschema_version: 1\n---\nSome content.\n"
	if err := os.WriteFile(artifact, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	result, err := v.Validate(Step("unknown-step"), artifact)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, w := range result.Warnings {
		if w == "No validation rules for step: unknown-step" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected unknown step warning, got %v", result.Warnings)
	}
}

// --- Schema version checks ---

func TestValidate_StrictMissingSchemaVersion(t *testing.T) {
	v, tmpDir := helperNewValidator(t)
	artifact := filepath.Join(tmpDir, "no-schema.md")
	content := "---\ntitle: test\n---\nContent without schema_version.\n"
	if err := os.WriteFile(artifact, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	result, err := v.Validate(StepResearch, artifact)
	if err != nil {
		t.Fatal(err)
	}
	if result.Valid {
		t.Error("expected valid=false for missing schema_version in strict mode")
	}
}

func TestValidate_LenientMissingSchemaVersion(t *testing.T) {
	v, tmpDir := helperNewValidator(t)
	artifact := filepath.Join(tmpDir, "no-schema-lenient.md")
	content := "---\ntitle: test\n---\n## Summary\nSummary here.\n## Key Findings\nFindings.\n## Recommendations\nRecs.\nSource: http://example.com\n" + longText(120)
	if err := os.WriteFile(artifact, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	result, err := v.ValidateWithOptions(StepResearch, artifact, &ValidateOptions{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Valid {
		t.Errorf("expected valid=true in lenient mode, issues=%v", result.Issues)
	}
	if !result.Lenient {
		t.Error("expected Lenient=true in result")
	}
}

func TestValidate_LenientExpiredFails(t *testing.T) {
	v, tmpDir := helperNewValidator(t)
	artifact := filepath.Join(tmpDir, "expired.md")
	content := "---\ntitle: test\n---\nContent.\n"
	if err := os.WriteFile(artifact, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	past := time.Now().AddDate(0, 0, -5)
	result, err := v.ValidateWithOptions(StepResearch, artifact, &ValidateOptions{
		Lenient:           true,
		LenientExpiryDate: &past,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Valid {
		t.Error("expected valid=false for expired lenient")
	}
}

// --- assessTier ---

func TestAssessTier(t *testing.T) {
	v, _ := helperNewValidator(t)

	tests := []struct {
		name     string
		valid    bool
		issues   int
		warnings int
		want     Tier
	}{
		{"invalid", false, 1, 0, TierObservation},
		{"no issues or warnings", true, 0, 0, TierPattern},
		{"few warnings", true, 0, 2, TierLearning},
		{"many warnings", true, 0, 3, TierObservation},
		{"has issues", true, 1, 0, TierObservation},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &ValidationResult{
				Valid:    tt.valid,
				Issues:   make([]string, tt.issues),
				Warnings: make([]string, tt.warnings),
			}
			got := v.assessTier(result)
			if got != tt.want {
				t.Errorf("assessTier() = %v, want %v", got, tt.want)
			}
		})
	}
}

// --- GetMetrics ---

func TestGetMetrics(t *testing.T) {
	v, tmpDir := helperNewValidator(t)

	// Initial metrics should be zero
	m := v.GetMetrics()
	if m.LenientCount != 0 || m.StrictCount != 0 {
		t.Errorf("expected zero metrics, got lenient=%d strict=%d", m.LenientCount, m.StrictCount)
	}

	// Create artifacts and validate
	strictArtifact := filepath.Join(tmpDir, "strict.md")
	if err := os.WriteFile(strictArtifact, []byte("schema_version: 1\ncontent"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := v.Validate(StepResearch, strictArtifact); err != nil {
		t.Fatal(err)
	}

	lenientArtifact := filepath.Join(tmpDir, "lenient.md")
	if err := os.WriteFile(lenientArtifact, []byte("no schema version"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := v.ValidateWithOptions(StepResearch, lenientArtifact, &ValidateOptions{Lenient: true}); err != nil {
		t.Fatal(err)
	}

	m = v.GetMetrics()
	if m.StrictCount != 1 {
		t.Errorf("expected strict=1, got %d", m.StrictCount)
	}
	if m.LenientCount != 1 {
		t.Errorf("expected lenient=1, got %d", m.LenientCount)
	}

	// Verify copy semantics - mutating returned metrics shouldn't affect internal state
	m.StrictCount = 999
	m2 := v.GetMetrics()
	if m2.StrictCount == 999 {
		t.Error("GetMetrics should return a copy, not a reference")
	}
}

// --- ValidateForPromotion ---

func TestValidateForPromotion(t *testing.T) {
	v, tmpDir := helperNewValidator(t)

	t.Run("missing artifact", func(t *testing.T) {
		result, err := v.ValidateForPromotion("/nonexistent.md", TierLearning)
		if err != nil {
			t.Fatal(err)
		}
		if result.Valid {
			t.Error("expected valid=false for missing artifact")
		}
	})

	t.Run("promotion to learning needs citations", func(t *testing.T) {
		artifact := filepath.Join(tmpDir, "learning-candidate.md")
		if err := os.WriteFile(artifact, []byte("Some content."), 0600); err != nil {
			t.Fatal(err)
		}
		result, err := v.ValidateForPromotion(artifact, TierLearning)
		if err != nil {
			t.Fatal(err)
		}
		if result.Valid {
			t.Error("expected valid=false (no citations)")
		}
	})

	t.Run("promotion to pattern needs sessions", func(t *testing.T) {
		artifact := filepath.Join(tmpDir, "pattern-candidate.md")
		if err := os.WriteFile(artifact, []byte("Some content."), 0600); err != nil {
			t.Fatal(err)
		}
		result, err := v.ValidateForPromotion(artifact, TierPattern)
		if err != nil {
			t.Fatal(err)
		}
		if result.Valid {
			t.Error("expected valid=false (no session refs)")
		}
	})

	t.Run("promotion to skill needs format", func(t *testing.T) {
		artifact := filepath.Join(tmpDir, "skill-candidate.md")
		if err := os.WriteFile(artifact, []byte("No skill format."), 0600); err != nil {
			t.Fatal(err)
		}
		result, err := v.ValidateForPromotion(artifact, TierSkill)
		if err != nil {
			t.Fatal(err)
		}
		if result.Valid {
			t.Error("expected valid=false (no skill format)")
		}
	})

	t.Run("promotion to skill passes with format", func(t *testing.T) {
		artifact := filepath.Join(tmpDir, "skill-valid.md")
		content := "## Description\nA skill.\n## Triggers\n/test\n## Instructions\nDo stuff.\n"
		if err := os.WriteFile(artifact, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
		result, err := v.ValidateForPromotion(artifact, TierSkill)
		if err != nil {
			t.Fatal(err)
		}
		if !result.Valid {
			t.Errorf("expected valid=true, issues=%v", result.Issues)
		}
	})

	t.Run("promotion to core warns", func(t *testing.T) {
		artifact := filepath.Join(tmpDir, "core-candidate.md")
		if err := os.WriteFile(artifact, []byte("Content."), 0600); err != nil {
			t.Fatal(err)
		}
		result, err := v.ValidateForPromotion(artifact, TierCore)
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Warnings) == 0 {
			t.Error("expected warning about manual review for core promotion")
		}
	})

	t.Run("promotion step is set", func(t *testing.T) {
		artifact := filepath.Join(tmpDir, "promo-step.md")
		if err := os.WriteFile(artifact, []byte("Content."), 0600); err != nil {
			t.Fatal(err)
		}
		result, err := v.ValidateForPromotion(artifact, TierCore)
		if err != nil {
			t.Fatal(err)
		}
		if result.Step != Step("promotion") {
			t.Errorf("expected step='promotion', got %q", result.Step)
		}
	})
}

// --- hasSchemaVersion ---

func TestHasSchemaVersion(t *testing.T) {
	v, tmpDir := helperNewValidator(t)

	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{"yaml format", "schema_version: 1\n", true},
		{"json format", `{"schema_version": 1}`, true},
		{"hyphenated yaml", "schema-version: 1\n", true},
		{"hyphenated json", `{"schema-version": 1}`, true},
		{"missing", "no version here\n", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(tmpDir, strings.ReplaceAll(tt.name, " ", "_")+".md")
			if err := os.WriteFile(path, []byte(tt.content), 0600); err != nil {
				t.Fatal(err)
			}
			got := v.hasSchemaVersion(path)
			if got != tt.want {
				t.Errorf("hasSchemaVersion() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHasSchemaVersion_NonexistentFile(t *testing.T) {
	v, _ := helperNewValidator(t)
	if v.hasSchemaVersion("/nonexistent/file.md") {
		t.Error("expected false for nonexistent file")
	}
}

// --- hasFrontmatterField ---

func TestHasFrontmatterField(t *testing.T) {
	v, _ := helperNewValidator(t)

	tests := []struct {
		name  string
		text  string
		field string
		want  bool
	}{
		{
			"field present",
			"---\nschema_version: 1\ntitle: test\n---\nBody.",
			"schema_version",
			true,
		},
		{
			"field absent",
			"---\ntitle: test\n---\nBody.",
			"schema_version",
			false,
		},
		{
			"no frontmatter",
			"Just plain text.",
			"schema_version",
			false,
		},
		{
			"field outside frontmatter",
			"---\ntitle: test\n---\nschema_version: 1",
			"schema_version",
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := v.hasFrontmatterField(tt.text, tt.field)
			if got != tt.want {
				t.Errorf("hasFrontmatterField() = %v, want %v", got, tt.want)
			}
		})
	}
}

// --- hasSkillFormat ---

func TestHasSkillFormat(t *testing.T) {
	v, tmpDir := helperNewValidator(t)

	t.Run("valid skill format", func(t *testing.T) {
		path := filepath.Join(tmpDir, "valid-skill.md")
		content := "## Description\nA skill.\n## Triggers\n/test\n## Instructions\nDo stuff.\n"
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
		if !v.hasSkillFormat(path) {
			t.Error("expected true for valid skill format")
		}
	})

	t.Run("missing section", func(t *testing.T) {
		path := filepath.Join(tmpDir, "invalid-skill.md")
		content := "## Description\nA skill.\n## Triggers\n/test\n"
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
		if v.hasSkillFormat(path) {
			t.Error("expected false for missing Instructions section")
		}
	})

	t.Run("nonexistent file", func(t *testing.T) {
		if v.hasSkillFormat("/nonexistent") {
			t.Error("expected false for nonexistent file")
		}
	})
}

// --- validateFormulaToml ---

func TestValidateFormulaToml(t *testing.T) {
	v, _ := helperNewValidator(t)

	t.Run("formula field at start matches", func(t *testing.T) {
		// Note: validateFormulaToml uses ^ in regex, which in Go only matches
		// start of string (not start of line). Only the first field can match
		// via ^. This test verifies the formula field is detected at position 0.
		text := "formula = \"test\"\ndescription = \"A test\"\nversion = \"1.0\"\ntype = \"epic\"\nschema_version = 1\n\n[[steps]]\nname = \"step1\"\n"
		result := &ValidationResult{Valid: true, Issues: []string{}, Warnings: []string{}}
		v.validateFormulaToml(text, result)
		// formula at start matches, but description/version/type on later lines
		// don't match ^field because ^ is start-of-string in Go regex.
		// So we expect warnings for description, version, type but not formula.
		formulaWarningFound := false
		for _, w := range result.Warnings {
			if w == "Missing required TOML field: formula" {
				formulaWarningFound = true
			}
		}
		if formulaWarningFound {
			t.Error("formula field at start of text should have been detected")
		}
	})

	t.Run("missing fields", func(t *testing.T) {
		text := "title = \"test\"\n"
		result := &ValidationResult{Valid: true, Issues: []string{}, Warnings: []string{}}
		v.validateFormulaToml(text, result)
		// Should warn about missing formula, description, version, type, schema_version, and [[steps]]
		if len(result.Warnings) < 4 {
			t.Errorf("expected at least 4 warnings, got %d: %v", len(result.Warnings), result.Warnings)
		}
	})
}

// --- fileContains ---

func TestFileContains(t *testing.T) {
	v, tmpDir := helperNewValidator(t)

	path := filepath.Join(tmpDir, "contains-test.md")
	if err := os.WriteFile(path, []byte("line one\nline two\nline three\n"), 0600); err != nil {
		t.Fatal(err)
	}

	if !v.fileContains(path, "line two") {
		t.Error("expected true for existing content")
	}
	if v.fileContains(path, "nonexistent") {
		t.Error("expected false for nonexistent content")
	}
	if v.fileContains("/nonexistent/file", "anything") {
		t.Error("expected false for nonexistent file")
	}
}

// --- validateEpicIssue ---

func TestValidateEpicIssue(t *testing.T) {
	v, _ := helperNewValidator(t)

	t.Run("empty epic ID", func(t *testing.T) {
		result := &ValidationResult{Valid: true, Issues: []string{}, Warnings: []string{}}
		v.validateEpicIssue("", result)
		if result.Valid {
			t.Error("expected valid=false for empty epic ID")
		}
	})

	t.Run("no prefix warns", func(t *testing.T) {
		result := &ValidationResult{Valid: true, Issues: []string{}, Warnings: []string{}}
		v.validateEpicIssue("0001", result)
		if len(result.Warnings) == 0 {
			t.Error("expected warning for epic without prefix")
		}
	})

	t.Run("valid epic ID", func(t *testing.T) {
		result := &ValidationResult{Valid: true, Issues: []string{}, Warnings: []string{}}
		v.validateEpicIssue("ag-0001", result)
		if !result.Valid {
			t.Error("expected valid=true for well-formed epic")
		}
		if len(result.Warnings) != 0 {
			t.Errorf("expected no warnings, got %v", result.Warnings)
		}
	})
}

// --- countCitations ---

func TestCountCitations(t *testing.T) {
	v, tmpDir := helperNewValidator(t)

	// Create the artifact and some files that reference it
	artifact := filepath.Join(tmpDir, "target.md")
	if err := os.WriteFile(artifact, []byte("Content of target."), 0600); err != nil {
		t.Fatal(err)
	}

	// Create files that reference target.md
	ref1 := filepath.Join(tmpDir, "ref1.md")
	if err := os.WriteFile(ref1, []byte("See target.md for details."), 0600); err != nil {
		t.Fatal(err)
	}
	ref2 := filepath.Join(tmpDir, "ref2.md")
	if err := os.WriteFile(ref2, []byte("Also references target.md here."), 0600); err != nil {
		t.Fatal(err)
	}
	nonref := filepath.Join(tmpDir, "nonref.md")
	if err := os.WriteFile(nonref, []byte("No references here."), 0600); err != nil {
		t.Fatal(err)
	}

	count := v.countCitations(artifact)
	if count != 2 {
		t.Errorf("expected 2 citations, got %d", count)
	}
}

// --- Validate result has tier assigned ---

func TestValidate_TierAssigned(t *testing.T) {
	v, tmpDir := helperNewValidator(t)
	artifact := filepath.Join(tmpDir, "tier-test.md")
	content := "---\nschema_version: 1\n---\n## Summary\nOk.\n## Key Findings\nFindings.\n## Recommendations\nRecs.\nSource: http://example.com\n" + longText(120)
	if err := os.WriteFile(artifact, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	result, err := v.Validate(StepResearch, artifact)
	if err != nil {
		t.Fatal(err)
	}
	if result.Tier == nil {
		t.Error("expected Tier to be assigned after validation")
	}
}

// longText generates filler text with the given number of words.
func longText(wordCount int) string {
	words := make([]string, wordCount)
	for i := range words {
		words[i] = "word"
	}
	return "\n" + strings.Join(words, " ") + "\n"
}
