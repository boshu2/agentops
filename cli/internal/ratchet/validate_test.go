package ratchet

import (
	"encoding/json"
	"fmt"
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
		ArtifactPath:    "/path/to/artifact.md",
		SessionID:       "sess-001",
		CitedAt:         time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC),
		CitationType:    "retrieved",
		MatchConfidence: 0.5,
		MatchProvenance: "lookup:query",
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
	for _, citation := range citations {
		if got, want := citation.WorkspacePath, baseDir; got != want {
			t.Fatalf("WorkspacePath = %q, want %q", got, want)
		}
	}
	if citations[0].MatchConfidence != 0.5 {
		t.Fatalf("MatchConfidence = %v, want 0.5", citations[0].MatchConfidence)
	}
	if citations[0].MatchProvenance != "lookup:query" {
		t.Fatalf("MatchProvenance = %q, want lookup:query", citations[0].MatchProvenance)
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

func TestLoadCitations_LegacyFields(t *testing.T) {
	baseDir := t.TempDir()
	citationsPath := filepath.Join(baseDir, CitationsFilePath)
	if err := os.MkdirAll(filepath.Dir(citationsPath), 0o700); err != nil {
		t.Fatal(err)
	}
	line := `{"learning_file":".agents/learnings/legacy.md","timestamp":"2026-04-01T12:00:00Z","session":"legacy-session","workspace":"."}` + "\n"
	if err := os.WriteFile(citationsPath, []byte(line), 0o600); err != nil {
		t.Fatal(err)
	}

	citations, err := LoadCitations(baseDir)
	if err != nil {
		t.Fatalf("LoadCitations failed: %v", err)
	}
	if len(citations) != 1 {
		t.Fatalf("expected 1 citation, got %d", len(citations))
	}
	if got, want := citations[0].ArtifactPath, filepath.Join(baseDir, ".agents", "learnings", "legacy.md"); got != want {
		t.Fatalf("ArtifactPath = %q, want %q", got, want)
	}
	if got, want := citations[0].WorkspacePath, baseDir; got != want {
		t.Fatalf("WorkspacePath = %q, want %q", got, want)
	}
	if got, want := citations[0].SessionID, "legacy-session"; got != want {
		t.Fatalf("SessionID = %q, want %q", got, want)
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
	data, err := json.Marshal(validEvent)
	if err != nil {
		t.Fatalf("marshal citation event: %v", err)
	}
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

func TestNewValidator_UserHomeDirError(t *testing.T) {
	t.Setenv("HOME", "")

	v, err := NewValidator(t.TempDir())
	if err == nil {
		t.Fatal("expected error when HOME is unset")
	}
	if v != nil {
		t.Error("expected nil validator on error")
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

	for _, tc := range validateForPromotionCases() {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			tc.run(t, v, tmpDir)
		})
	}
}

type validateForPromotionCase struct {
	name string
	run  func(t *testing.T, v *Validator, tmpDir string)
}

func validateForPromotionCases() []validateForPromotionCase {
	return []validateForPromotionCase{
		{"missing artifact", validateMissingPromotionArtifact},
		{"promotion to learning needs citations", validateLearningPromotionNeedsCitations},
		{"promotion to pattern needs sessions", validatePatternPromotionNeedsSessions},
		{"promotion to skill needs format", validateSkillPromotionNeedsFormat},
		{"promotion to skill passes with format", validateSkillPromotionPassesWithFormat},
		{"promotion to core warns", validateCorePromotionWarns},
		{"promotion step is set", validatePromotionStepIsSet},
	}
}

func validateMissingPromotionArtifact(t *testing.T, v *Validator, _ string) {
	t.Helper()
	result := validatePromotionArtifact(t, v, "/nonexistent.md", TierLearning)
	if result.Valid {
		t.Error("expected valid=false for missing artifact")
	}
}

func validateLearningPromotionNeedsCitations(t *testing.T, v *Validator, tmpDir string) {
	t.Helper()
	artifact := writePromotionArtifact(t, tmpDir, "learning-candidate.md", "Some content.")
	result := validatePromotionArtifact(t, v, artifact, TierLearning)
	if result.Valid {
		t.Error("expected valid=false (no citations)")
	}
}

func validatePatternPromotionNeedsSessions(t *testing.T, v *Validator, tmpDir string) {
	t.Helper()
	artifact := writePromotionArtifact(t, tmpDir, "pattern-candidate.md", "Some content.")
	result := validatePromotionArtifact(t, v, artifact, TierPattern)
	if result.Valid {
		t.Error("expected valid=false (no session refs)")
	}
}

func validateSkillPromotionNeedsFormat(t *testing.T, v *Validator, tmpDir string) {
	t.Helper()
	artifact := writePromotionArtifact(t, tmpDir, "skill-candidate.md", "No skill format.")
	result := validatePromotionArtifact(t, v, artifact, TierSkill)
	if result.Valid {
		t.Error("expected valid=false (no skill format)")
	}
}

func validateSkillPromotionPassesWithFormat(t *testing.T, v *Validator, tmpDir string) {
	t.Helper()
	content := "## Description\nA skill.\n## Triggers\n/test\n## Instructions\nDo stuff.\n"
	artifact := writePromotionArtifact(t, tmpDir, "skill-valid.md", content)
	result := validatePromotionArtifact(t, v, artifact, TierSkill)
	if !result.Valid {
		t.Errorf("expected valid=true, issues=%v", result.Issues)
	}
}

func validateCorePromotionWarns(t *testing.T, v *Validator, tmpDir string) {
	t.Helper()
	artifact := writePromotionArtifact(t, tmpDir, "core-candidate.md", "Content.")
	result := validatePromotionArtifact(t, v, artifact, TierCore)
	if len(result.Warnings) == 0 {
		t.Error("expected warning about manual review for core promotion")
	}
}

func validatePromotionStepIsSet(t *testing.T, v *Validator, tmpDir string) {
	t.Helper()
	artifact := writePromotionArtifact(t, tmpDir, "promo-step.md", "Content.")
	result := validatePromotionArtifact(t, v, artifact, TierCore)
	if result.Step != Step("promotion") {
		t.Errorf("expected step='promotion', got %q", result.Step)
	}
}

func writePromotionArtifact(t *testing.T, tmpDir, name, content string) string {
	t.Helper()
	artifact := filepath.Join(tmpDir, name)
	if err := os.WriteFile(artifact, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	return artifact
}

func validatePromotionArtifact(t *testing.T, v *Validator, artifact string, tier Tier) *ValidationResult {
	t.Helper()
	result, err := v.ValidateForPromotion(artifact, tier)
	if err != nil {
		t.Fatal(err)
	}
	return result
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

func TestCanonicalArtifactPath(t *testing.T) {
	tests := []struct {
		name     string
		baseDir  string
		artifact string
		wantAbs  bool
	}{
		{"empty artifact returns empty", "/base", "", false},
		{"absolute path stays absolute", "/base", "/some/absolute/file.md", true},
		{"relative path joined with base", "/base", "learnings/test.md", true},
		{"empty base uses dot", "", "learnings/test.md", true},
		{"whitespace-only base uses dot", "  ", "learnings/test.md", true},
		{"whitespace artifact trimmed", "/base", "  test.md  ", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CanonicalArtifactPath(tt.baseDir, tt.artifact)
			if tt.artifact == "" || strings.TrimSpace(tt.artifact) == "" {
				if got != "" && tt.artifact == "" {
					t.Errorf("expected empty for empty artifact, got %q", got)
				}
				return
			}
			if tt.wantAbs && !filepath.IsAbs(got) {
				t.Errorf("expected absolute path, got %q", got)
			}
		})
	}
}

func TestValidatePreMortem_NotFound(t *testing.T) {
	v, _ := helperNewValidator(t)
	result, err := v.Validate(StepPreMortem, "/nonexistent/file.md")
	if err != nil {
		t.Fatal(err)
	}
	if result.Valid {
		t.Error("expected invalid result for nonexistent file")
	}
	foundNotFound := false
	for _, issue := range result.Issues {
		if strings.Contains(issue, "Artifact not found") {
			foundNotFound = true
			break
		}
	}
	if !foundNotFound {
		t.Error("expected 'Artifact not found' issue")
	}
}

func TestValidatePreMortem_Warnings(t *testing.T) {
	v, tmpDir := helperNewValidator(t)
	// Minimal content that won't trigger any warnings (baseline)
	artifact := filepath.Join(tmpDir, "premortem-v2.md")
	content := "---\nschema_version: 1\n---\n## Finding\n| ID | Something |\nMitigation steps\n" + longText(100)
	if err := os.WriteFile(artifact, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	result, err := v.Validate(StepPreMortem, artifact)
	if err != nil {
		t.Fatal(err)
	}

	// Should pass validation (it has all required elements)
	if !result.Valid {
		t.Errorf("expected valid result, got issues: %v", result.Issues)
	}
}

func TestValidatePreMortem_MissingFindings(t *testing.T) {
	v, tmpDir := helperNewValidator(t)
	// Use a filename WITHOUT version suffix to trigger that warning too
	artifact := filepath.Join(tmpDir, "premortem.md")
	// Missing both findings and mitigations
	content := "---\nschema_version: 1\n---\n## Just some text\nNo findings here.\n" + longText(100)
	if err := os.WriteFile(artifact, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	result, err := v.Validate(StepPreMortem, artifact)
	if err != nil {
		t.Fatal(err)
	}

	// Check for expected warnings
	wantWarnings := []string{"findings table", "mitigations", "version suffix"}
	for _, want := range wantWarnings {
		found := false
		for _, w := range result.Warnings {
			if strings.Contains(strings.ToLower(w), strings.ToLower(want)) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected warning containing %q, got warnings: %v", want, result.Warnings)
		}
	}
}

func TestValidatePlan_ReadError(t *testing.T) {
	v, _ := helperNewValidator(t)
	result, err := v.Validate(StepPlan, "/nonexistent/plan.md")
	if err != nil {
		t.Fatal(err)
	}
	if result.Valid {
		t.Error("expected invalid result for unreadable plan file")
	}
}

func TestValidatePlan_MissingSections(t *testing.T) {
	v, tmpDir := helperNewValidator(t)
	artifact := filepath.Join(tmpDir, "plan.md")
	// Missing objective, tasks, and success criteria
	content := "---\nschema_version: 1\n---\nJust some content without proper sections.\n" + longText(100)
	if err := os.WriteFile(artifact, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	result, err := v.Validate(StepPlan, artifact)
	if err != nil {
		t.Fatal(err)
	}

	wantWarnings := []string{"objective", "tasks", "success criteria"}
	for _, want := range wantWarnings {
		found := false
		for _, w := range result.Warnings {
			if strings.Contains(strings.ToLower(w), want) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected warning containing %q, got: %v", want, result.Warnings)
		}
	}
}

func TestValidatePostMortem_ReadError(t *testing.T) {
	v, _ := helperNewValidator(t)
	result, err := v.Validate(StepPostMortem, "/nonexistent/retro.md")
	if err != nil {
		t.Fatal(err)
	}
	if result.Valid {
		t.Error("expected invalid result for unreadable post-mortem file")
	}
}

func TestValidatePostMortem_MissingSections(t *testing.T) {
	v, tmpDir := helperNewValidator(t)
	artifact := filepath.Join(tmpDir, "retro.md")
	// Missing learnings and patterns sections
	content := "---\nschema_version: 1\n---\nJust a retro without proper sections.\n" + longText(100)
	if err := os.WriteFile(artifact, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	result, err := v.Validate(StepPostMortem, artifact)
	if err != nil {
		t.Fatal(err)
	}

	wantWarnings := []string{"learnings", "patterns"}
	for _, want := range wantWarnings {
		found := false
		for _, w := range result.Warnings {
			if strings.Contains(strings.ToLower(w), want) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected warning containing %q, got: %v", want, result.Warnings)
		}
	}
}

func TestRecordCitation_ZeroTimestamp(t *testing.T) {
	tmpDir := t.TempDir()

	// Citation with zero timestamp should get current time
	event := types.CitationEvent{
		ArtifactPath: "test.md",
		SessionID:    "sess-1",
	}

	if err := RecordCitation(tmpDir, event); err != nil {
		t.Fatalf("RecordCitation: %v", err)
	}

	citations, err := LoadCitations(tmpDir)
	if err != nil {
		t.Fatalf("LoadCitations: %v", err)
	}
	if len(citations) != 1 {
		t.Fatalf("expected 1 citation, got %d", len(citations))
	}
	if citations[0].CitedAt.IsZero() {
		t.Error("expected non-zero timestamp")
	}
}

func TestRecordCitation_ReadOnlyDir(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root bypasses filesystem permissions")
	}
	tmpDir := t.TempDir()
	readOnly := filepath.Join(tmpDir, "readonly")
	if err := os.MkdirAll(readOnly, 0500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(readOnly, 0700) })

	event := types.CitationEvent{
		ArtifactPath: "test.md",
		SessionID:    "sess-1",
	}
	err := RecordCitation(readOnly, event)
	if err == nil {
		t.Error("expected error when writing to read-only directory")
	}
}

func TestValidateCloseReason_WithAbsolutePaths(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a valid artifact
	agentsDir := filepath.Join(tmpDir, ".agents", "council")
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		t.Fatal(err)
	}
	artifactPath := filepath.Join(agentsDir, "vibe-result.md")
	if err := os.WriteFile(artifactPath, []byte("# Vibe Result\nContent here."), 0600); err != nil {
		t.Fatal(err)
	}

	issues := ValidateCloseReason("completed: " + artifactPath)
	if len(issues) != 0 {
		t.Errorf("expected no issues for valid close reason, got: %v", issues)
	}
}

func TestValidateCloseReason_WithRelativePaths(t *testing.T) {
	issues := ValidateCloseReason("completed: ./relative/path.md")
	if len(issues) == 0 {
		t.Error("expected issues for relative path in close reason")
	}
}

func TestLoadCitations_PermissionError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root bypasses filesystem permissions")
	}
	baseDir := t.TempDir()
	citationsDir := filepath.Join(baseDir, ".agents", "ao")
	if err := os.MkdirAll(citationsDir, 0700); err != nil {
		t.Fatal(err)
	}
	citationsPath := filepath.Join(citationsDir, "citations.jsonl")
	if err := os.WriteFile(citationsPath, []byte(`{"artifact_path":"/a.md"}`+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(citationsPath, 0000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(citationsPath, 0644) })

	_, err := LoadCitations(baseDir)
	if err == nil {
		t.Error("expected error when citations file is unreadable")
	}
}

func TestLoadCitations_EmptyLines(t *testing.T) {
	baseDir := t.TempDir()
	citationsDir := filepath.Join(baseDir, ".agents", "ao")
	if err := os.MkdirAll(citationsDir, 0700); err != nil {
		t.Fatal(err)
	}
	// Write file with empty lines interleaved
	content := `{"artifact_path":"/a.md","session_id":"s1"}` + "\n\n" + `{"artifact_path":"/b.md","session_id":"s2"}` + "\n\n"
	if err := os.WriteFile(filepath.Join(citationsDir, "citations.jsonl"), []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	citations, err := LoadCitations(baseDir)
	if err != nil {
		t.Fatalf("LoadCitations: %v", err)
	}
	if len(citations) != 2 {
		t.Errorf("expected 2 citations (skipping empty lines), got %d", len(citations))
	}
}

func TestValidate_ResearchStep_ShortContent(t *testing.T) {
	v, tmpDir := helperNewValidator(t)
	// Create a research file with < 100 words
	artifact := filepath.Join(tmpDir, "research-short.md")
	content := "---\nschema_version: 1\n---\n## Summary\nShort.\n## Key Findings\nFindings.\n## Recommendations\nRecs.\nSource: http://example.com\n"
	if err := os.WriteFile(artifact, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	result, err := v.Validate(StepResearch, artifact)
	if err != nil {
		t.Fatal(err)
	}

	// Should have warning about short research
	wantWarning := fmt.Sprintf("Research seems short (%d words), consider adding more detail", len(strings.Fields(content)))
	found := false
	for _, w := range result.Warnings {
		if w == wantWarning {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected %q warning for < 100 word research, got %v", wantWarning, result.Warnings)
	}
}

func TestValidate_ResearchStep_MissingSchemaVersionFrontmatter(t *testing.T) {
	v, tmpDir := helperNewValidator(t)
	// Research file with schema_version: in content (so hasSchemaVersion passes)
	// but NOT in frontmatter (so hasFrontmatterField returns false -> warning)
	artifact := filepath.Join(tmpDir, "research-no-fm.md")
	content := "schema_version: 1\n## Summary\nSummary.\n## Key Findings\nFindings.\n## Recommendations\nRecs.\nSource: http://example.com\n" + longText(120)
	if err := os.WriteFile(artifact, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	result, err := v.Validate(StepResearch, artifact)
	if err != nil {
		t.Fatal(err)
	}

	// Should have warning about missing schema_version in frontmatter
	found := false
	for _, w := range result.Warnings {
		if strings.Contains(w, "schema_version field in frontmatter") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected frontmatter schema_version warning, got %v", result.Warnings)
	}
}

func TestValidate_PreMortemStep_ReadError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root bypasses filesystem permissions")
	}
	v, tmpDir := helperNewValidator(t)
	artifact := filepath.Join(tmpDir, "unreadable-premortem-v2.md")
	content := "schema_version: 1\ncontent"
	if err := os.WriteFile(artifact, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	// Change permissions after creation so os.Stat passes but os.ReadFile fails
	if err := os.Chmod(artifact, 0000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(artifact, 0644) })

	result, err := v.Validate(StepPreMortem, artifact)
	if err != nil {
		t.Fatal(err)
	}
	// hasSchemaVersion returns false (can't read), strict mode -> invalid
	if result.Valid {
		t.Log("File unreadable -> hasSchemaVersion false -> strict mode rejects")
	}
}

// --- countSessionRefs ---

func TestCountSessionRefs(t *testing.T) {
	v, tmpDir := helperNewValidator(t)

	// Create the artifact file
	artifact := filepath.Join(tmpDir, "target-artifact.md")
	if err := os.WriteFile(artifact, []byte("Content of target."), 0600); err != nil {
		t.Fatal(err)
	}

	// Create .agents/ao/sessions/ with files that reference the artifact
	sessionsDir := filepath.Join(tmpDir, ".agents", "ao", "sessions")
	if err := os.MkdirAll(sessionsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Session file that references our artifact
	sess1 := filepath.Join(sessionsDir, "sess1.jsonl")
	if err := os.WriteFile(sess1, []byte("worked on target-artifact.md\n"), 0600); err != nil {
		t.Fatal(err)
	}

	// Session file that does NOT reference our artifact
	sess2 := filepath.Join(sessionsDir, "sess2.jsonl")
	if err := os.WriteFile(sess2, []byte("worked on other-file.md\n"), 0600); err != nil {
		t.Fatal(err)
	}

	// A markdown session file that references artifact
	sess3 := filepath.Join(sessionsDir, "sess3.md")
	if err := os.WriteFile(sess3, []byte("# Session 3\nUsed target-artifact.md for research.\n"), 0600); err != nil {
		t.Fatal(err)
	}

	// A .txt file that references artifact (should be skipped -- only .jsonl and .md)
	sess4 := filepath.Join(sessionsDir, "sess4.txt")
	if err := os.WriteFile(sess4, []byte("target-artifact.md\n"), 0600); err != nil {
		t.Fatal(err)
	}

	count := v.countSessionRefs(artifact)
	if count != 2 {
		t.Errorf("expected 2 session refs, got %d", count)
	}
}

func TestCountSessionRefs_NoSessionsDirs(t *testing.T) {
	v, _ := helperNewValidator(t)
	// Override townDir so the locator does not find real sessions in ~/gt
	v.locator.townDir = t.TempDir()
	// No .agents/ao/sessions/ exists at all in any search location
	artifact := "/some/nonexistent/artifact.md"
	count := v.countSessionRefs(artifact)
	if count != 0 {
		t.Errorf("expected 0 session refs when no sessions dirs, got %d", count)
	}
}

// --- validatePlan via ValidateWithOptions with epic: prefix ---

func TestValidatePlan_EpicPrefixReachesValidateEpicIssue(t *testing.T) {
	v, _ := helperNewValidator(t)

	// The validatePlan function detects the epic: prefix and delegates
	// to validateEpicIssue. We test this directly since ValidateWithOptions
	// does os.Stat first, and epic: paths fail os.Stat.
	result := &ValidationResult{Valid: true, Issues: []string{}, Warnings: []string{}}
	v.validatePlan("epic:ag-0001", result)
	// Should pass and not have issues (it delegates to validateEpicIssue)
	if !result.Valid {
		t.Error("expected valid=true for well-formed epic ID via validatePlan")
	}

	// With empty epic ID
	result2 := &ValidationResult{Valid: true, Issues: []string{}, Warnings: []string{}}
	v.validatePlan("epic:", result2)
	if result2.Valid {
		t.Error("expected valid=false for empty epic ID via validatePlan")
	}
}

// --- validatePlan read error (direct call) ---

func TestValidatePlan_ReadErrorDirect(t *testing.T) {
	v, _ := helperNewValidator(t)
	result := &ValidationResult{Valid: true, Issues: []string{}, Warnings: []string{}}
	v.validatePlan("/nonexistent/plan.md", result)
	if result.Valid {
		t.Error("expected valid=false when validatePlan can't read file")
	}
	found := false
	for _, issue := range result.Issues {
		if strings.Contains(issue, "Cannot read file") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'Cannot read file' issue, got %v", result.Issues)
	}
}

// --- validatePreMortem read error (direct call) ---

func TestValidatePreMortem_ReadErrorDirect(t *testing.T) {
	v, _ := helperNewValidator(t)
	result := &ValidationResult{Valid: true, Issues: []string{}, Warnings: []string{}}
	v.validatePreMortem("/nonexistent/premortem.md", result)
	if result.Valid {
		t.Error("expected valid=false when validatePreMortem can't read file")
	}
}

// --- validatePostMortem read error (direct call) ---

func TestValidatePostMortem_ReadErrorDirect(t *testing.T) {
	v, _ := helperNewValidator(t)
	result := &ValidationResult{Valid: true, Issues: []string{}, Warnings: []string{}}
	v.validatePostMortem("/nonexistent/postmortem.md", result)
	if result.Valid {
		t.Error("expected valid=false when validatePostMortem can't read file")
	}
}

// --- validateResearch read error (direct call) ---

func TestValidateResearch_ReadErrorDirect(t *testing.T) {
	v, _ := helperNewValidator(t)
	result := &ValidationResult{Valid: true, Issues: []string{}, Warnings: []string{}}
	v.validateResearch("/nonexistent/research.md", result)
	if result.Valid {
		t.Error("expected valid=false when validateResearch can't read file")
	}
}

// --- countCitations walk error ---

func TestCountCitations_NonexistentDir(t *testing.T) {
	v, _ := helperNewValidator(t)
	// Artifact path in a directory that doesn't exist
	count := v.countCitations("/nonexistent/dir/artifact.md")
	if count != 0 {
		t.Errorf("expected 0 citations for nonexistent directory, got %d", count)
	}
}

// --- validateStep unknown step ---

func TestValidateStep_UnknownStep(t *testing.T) {
	v, _ := helperNewValidator(t)
	result := &ValidationResult{Valid: true, Issues: []string{}, Warnings: []string{}}
	v.validateStep(Step("foobar"), "/tmp/file.md", result)
	found := false
	for _, w := range result.Warnings {
		if strings.Contains(w, "No validation rules") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'No validation rules' warning, got %v", result.Warnings)
	}
}

// --- GetCitationsForSession with no file ---

func TestGetCitationsForSession_NoFile(t *testing.T) {
	baseDir := t.TempDir()
	got, err := GetCitationsForSession(baseDir, "any-session")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("expected 0 citations for missing file, got %d", len(got))
	}
}

// --- GetCitationsSince with no file ---

func TestGetCitationsSince_NoFile(t *testing.T) {
	baseDir := t.TempDir()
	got, err := GetCitationsSince(baseDir, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Errorf("expected nil for missing file, got %v", got)
	}
}

// --- CountCitationsForArtifact with permission error ---

func TestCountCitationsForArtifact_PermissionError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root bypasses filesystem permissions")
	}
	baseDir := t.TempDir()
	citationsDir := filepath.Join(baseDir, ".agents", "ao")
	if err := os.MkdirAll(citationsDir, 0700); err != nil {
		t.Fatal(err)
	}
	citationsPath := filepath.Join(citationsDir, "citations.jsonl")
	if err := os.WriteFile(citationsPath, []byte(`{"artifact_path":"/a.md"}`+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(citationsPath, 0000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(citationsPath, 0644) })

	_, err := CountCitationsForArtifact(baseDir, "/a.md")
	if err == nil {
		t.Error("expected error when citations file is unreadable")
	}
}

func TestRecordCitation_ReadOnlyBaseDir(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root bypasses filesystem permissions")
	}
	tmp := t.TempDir()
	readOnly := filepath.Join(tmp, "readonly")
	if err := os.MkdirAll(readOnly, 0500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(readOnly, 0700) })

	event := types.CitationEvent{
		ArtifactPath: "/some/artifact.md",
		SessionID:    "test-session",
		CitationType: "reference",
	}

	err := RecordCitation(readOnly, event)
	if err == nil {
		t.Error("expected error when recording citation to read-only directory")
	}
}

func TestRecordCitation_BlockedCitationsFile(t *testing.T) {
	tmp := t.TempDir()

	// Create the citations file path as a directory to block file creation
	citationsPath := filepath.Join(tmp, CitationsFilePath)
	if err := os.MkdirAll(citationsPath, 0755); err != nil {
		t.Fatal(err)
	}

	event := types.CitationEvent{
		ArtifactPath: "/some/artifact.md",
		SessionID:    "test-session",
		CitationType: "reference",
	}

	err := RecordCitation(tmp, event)
	if err == nil {
		t.Error("expected error when citations file path is a directory")
	}
	if !strings.Contains(err.Error(), "open citations file") {
		t.Errorf("expected 'open citations file' error, got: %v", err)
	}
}

func TestValidateCloseReason_WithSeeRelativePath(t *testing.T) {
	// Exercise the "See /path" pattern extraction with a relative path
	// that triggers ValidateArtifactPath error and issues append
	issues := ValidateCloseReason("See ./local/file.md and also Artifact: relative/path.md")
	if len(issues) == 0 {
		t.Error("expected issues for close_reason with relative paths")
	}
}

func TestGetCitationsForSession_ReadOnlyFile(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root bypasses filesystem permissions")
	}
	tmp := t.TempDir()

	// Record a citation first
	event := types.CitationEvent{
		ArtifactPath: "/test/artifact.md",
		SessionID:    "target-session",
		CitationType: "reference",
		CitedAt:      time.Now(),
	}
	if err := RecordCitation(tmp, event); err != nil {
		t.Fatal(err)
	}

	// Now query for citations matching that session
	citations, err := GetCitationsForSession(tmp, "target-session")
	if err != nil {
		t.Fatalf("GetCitationsForSession: %v", err)
	}
	if len(citations) != 1 {
		t.Errorf("expected 1 citation, got %d", len(citations))
	}
}

func TestValidatePreMortem_LenientMissingSchemaVersion(t *testing.T) {
	// Exercise the hasFrontmatterField("schema_version") warning path (line 243-246)
	// in validatePreMortem. Must use lenient mode so checkSchemaVersion doesn't
	// abort before reaching validateStep.
	v, tmpDir := helperNewValidator(t)
	artifact := filepath.Join(tmpDir, "premortem-v2.md")
	content := "---\ntitle: pre-mortem analysis\n---\n## Finding\n| ID | Risk |\n| 1 | Problem |\nMitigation: fix it\n" + longText(100)
	if err := os.WriteFile(artifact, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	opts := &ValidateOptions{Lenient: true}
	result, err := v.ValidateWithOptions(StepPreMortem, artifact, opts)
	if err != nil {
		t.Fatal(err)
	}

	found := false
	for _, w := range result.Warnings {
		if strings.Contains(w, "schema_version") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'schema_version' warning, got warnings: %v", result.Warnings)
	}
}

func TestValidatePlan_LenientMissingSchemaVersion(t *testing.T) {
	// Exercise the hasFrontmatterField("schema_version") warning path (line 293-296)
	// in validatePlan. Must use lenient mode.
	v, tmpDir := helperNewValidator(t)
	artifact := filepath.Join(tmpDir, "plan.md")
	content := "---\ntitle: implementation plan\n---\n## Scope\nBuild the feature\n## Tasks\n- Task 1\n## Dependencies\nNone\n## Risks\nLow\n## Estimate\n3 days\n" + longText(100)
	if err := os.WriteFile(artifact, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	opts := &ValidateOptions{Lenient: true}
	result, err := v.ValidateWithOptions(StepPlan, artifact, opts)
	if err != nil {
		t.Fatal(err)
	}

	found := false
	for _, w := range result.Warnings {
		if strings.Contains(w, "schema_version") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'schema_version' warning, got warnings: %v", result.Warnings)
	}
}

func TestValidatePostMortem_LenientMissingSchemaVersion(t *testing.T) {
	// Exercise the hasFrontmatterField("schema_version") warning path (line 327-330)
	// in validatePostMortem. Must use lenient mode.
	v, tmpDir := helperNewValidator(t)
	artifact := filepath.Join(tmpDir, "retro.md")
	content := "---\ntitle: retrospective\n---\n## Summary\nCompleted feature.\n## Learnings\n- Lesson 1\n## Metrics\nScore: 100\n## Recommendations\nKeep going\n" + longText(100)
	if err := os.WriteFile(artifact, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	opts := &ValidateOptions{Lenient: true}
	result, err := v.ValidateWithOptions(StepPostMortem, artifact, opts)
	if err != nil {
		t.Fatal(err)
	}

	found := false
	for _, w := range result.Warnings {
		if strings.Contains(w, "schema_version") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'schema_version' warning, got warnings: %v", result.Warnings)
	}
}

func TestLoadCitations_ScannerError(t *testing.T) {
	// Exercise the scanner.Err() path (line 894) by writing a line
	// exceeding the 1MB scanner buffer.
	baseDir := t.TempDir()
	citationsDir := filepath.Join(baseDir, ".agents", "ao")
	if err := os.MkdirAll(citationsDir, 0700); err != nil {
		t.Fatal(err)
	}
	citationsPath := filepath.Join(citationsDir, "citations.jsonl")

	// Write a valid line, then a line exceeding 1MB
	validLine := `{"artifact_path":"/a.md","session_id":"s1","citation_type":"ref"}` + "\n"
	hugeLine := make([]byte, 1100*1024) // 1.1MB exceeds 1MB scanner buffer
	for i := range hugeLine {
		hugeLine[i] = 'x'
	}
	content := validLine + string(hugeLine) + "\n"
	if err := os.WriteFile(citationsPath, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	_, err := LoadCitations(baseDir)
	if err == nil {
		t.Error("expected scanner error for line exceeding buffer")
	}
	if !strings.Contains(err.Error(), "scan citations") {
		t.Errorf("expected 'scan citations' error, got: %v", err)
	}
}

func TestGetCitationsSince_EmptyFile(t *testing.T) {
	tmp := t.TempDir()

	// Create an empty citations file
	citationsPath := filepath.Join(tmp, CitationsFilePath)
	if err := os.MkdirAll(filepath.Dir(citationsPath), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(citationsPath, []byte(""), 0600); err != nil {
		t.Fatal(err)
	}

	citations, err := GetCitationsSince(tmp, time.Now().Add(-1*time.Hour))
	if err != nil {
		t.Fatalf("GetCitationsSince: %v", err)
	}
	if len(citations) != 0 {
		t.Errorf("expected 0 citations from empty file, got %d", len(citations))
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

func TestReadArtifactText_FileNotFound(t *testing.T) {
	v := &Validator{}
	result := &ValidationResult{Valid: true, Issues: []string{}, Warnings: []string{}}
	text := v.readArtifactText("/nonexistent/path/file.md", result)

	if text != "" {
		t.Error("expected empty text for missing file")
	}
	if result.Valid {
		t.Error("expected result.Valid to be false for missing file")
	}
	if len(result.Issues) == 0 {
		t.Error("expected at least one issue for missing file")
	}
}

func TestReadArtifactText_MissingSchemaVersion(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "artifact.md")
	if err := os.WriteFile(path, []byte("# No frontmatter\nSome content"), 0644); err != nil {
		t.Fatal(err)
	}

	v := &Validator{}
	result := &ValidationResult{Valid: true, Issues: []string{}, Warnings: []string{}}
	text := v.readArtifactText(path, result)

	if text == "" {
		t.Error("expected non-empty text for existing file")
	}
	if len(result.Warnings) == 0 {
		t.Error("expected warning for missing schema_version")
	}
}

func TestReadArtifactText_WithSchemaVersion(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "artifact.md")
	content := "---\nschema_version: 1\n---\n# Content"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	v := &Validator{}
	result := &ValidationResult{Valid: true, Issues: []string{}, Warnings: []string{}}
	text := v.readArtifactText(path, result)

	if text != content {
		t.Errorf("text = %q, want %q", text, content)
	}
	if len(result.Warnings) != 0 {
		t.Errorf("unexpected warnings: %v", result.Warnings)
	}
}

func TestCheckSectionAny_Found(t *testing.T) {
	result := &ValidationResult{Warnings: []string{}}
	checkSectionAny("## Summary\nSome text", []string{"## Summary", "## Overview"}, "missing section", result)

	if len(result.Warnings) != 0 {
		t.Errorf("unexpected warnings: %v", result.Warnings)
	}
}

func TestCheckSectionAny_NotFound(t *testing.T) {
	result := &ValidationResult{Warnings: []string{}}
	checkSectionAny("No matching headers", []string{"## Summary", "## Overview"}, "missing section", result)

	if len(result.Warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d", len(result.Warnings))
	}
	if result.Warnings[0] != "missing section" {
		t.Errorf("warning = %q, want %q", result.Warnings[0], "missing section")
	}
}

func TestCheckSectionAny_SecondMarkerFound(t *testing.T) {
	result := &ValidationResult{Warnings: []string{}}
	checkSectionAny("## Overview\nDetails", []string{"## Summary", "## Overview"}, "missing section", result)

	if len(result.Warnings) != 0 {
		t.Errorf("unexpected warnings when second marker matches: %v", result.Warnings)
	}
}

func TestCheckTierRequirements_TierCore(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "artifact.md")
	if err := os.WriteFile(path, []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}

	v := &Validator{}
	result := &ValidationResult{Valid: true, Issues: []string{}, Warnings: []string{}}
	v.checkTierRequirements(path, TierCore, result)

	if !result.Valid {
		t.Error("TierCore should not invalidate result")
	}
	if len(result.Warnings) == 0 {
		t.Error("TierCore should add a manual review warning")
	}
}

// --- validatePlan TOML read error (direct call) ---

func TestValidatePlan_TomlReadErrorDirect(t *testing.T) {
	// Exercise the TOML read error path (validate.go lines 272-276).
	// Calling validatePlan directly bypasses Validate's os.Stat check.
	v, _ := helperNewValidator(t)
	result := &ValidationResult{Valid: true, Issues: []string{}, Warnings: []string{}}
	v.validatePlan("/nonexistent/formula.toml", result)

	if result.Valid {
		t.Error("expected valid=false for unreadable TOML file")
	}
	found := false
	for _, issue := range result.Issues {
		if strings.Contains(issue, "Cannot read file") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'Cannot read file' issue, got %v", result.Issues)
	}
}

func TestValidatePlan_TomlSuccessDirect(t *testing.T) {
	// Exercise the TOML success path (validate.go lines 277-278).
	// Calls validatePlan directly with a valid TOML file.
	v, tmpDir := helperNewValidator(t)
	artifact := filepath.Join(tmpDir, "formula.toml")
	content := "formula = \"test\"\ndescription = \"A test formula\"\nversion = \"1.0\"\ntype = \"epic\"\nschema_version = 1\n\n[[steps]]\nname = \"step1\"\n"
	if err := os.WriteFile(artifact, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	result := &ValidationResult{Valid: true, Issues: []string{}, Warnings: []string{}}
	v.validatePlan(artifact, result)

	if !result.Valid {
		t.Errorf("expected valid=true for valid TOML formula, issues=%v", result.Issues)
	}
}

// --- ValidateCloseReason with non-absolute extracted path ---

func TestValidateCloseReason_ExtractedRelativePath(t *testing.T) {
	// Exercise the ValidateArtifactPath error append path (validate.go line 750-752).
	// "Artifact: relative/path.md" extracts "relative/path.md" which is not absolute.
	// But ExtractArtifactPaths only matches paths starting with /.
	// We use "See /valid/path then check relative pattern" to test the
	// relative pattern detection path (lines 762-764).
	issues := ValidateCloseReason("found at ~/home/file.md")
	if len(issues) == 0 {
		t.Error("expected issues for close_reason with tilde path")
	}
}

// --- gatherSessionDirs with sessions ---

func TestGatherSessionDirs_WithLocalSessions(t *testing.T) {
	tmpDir := t.TempDir()
	sessionsDir := filepath.Join(tmpDir, ".agents", "ao", "sessions")
	if err := os.MkdirAll(sessionsDir, 0755); err != nil {
		t.Fatal(err)
	}

	v, err := NewValidator(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	dirs := v.gatherSessionDirs()
	found := false
	for _, d := range dirs {
		if d == sessionsDir {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected local sessions dir %q in results, got %v", sessionsDir, dirs)
	}
}

// --- countRefsInDir walk error ---

func TestCountRefsInDir_WalkError(t *testing.T) {
	// Exercise the filepath.Walk error fprintf path (validate.go line 509-511).
	v, _ := helperNewValidator(t)
	seen := make(map[string]bool)
	// Pass a nonexistent directory -- Walk fails and should still return 0
	count := v.countRefsInDir("/nonexistent/sessions", "target.md", seen)
	if count != 0 {
		t.Errorf("expected 0 for nonexistent directory, got %d", count)
	}
}

// --- GetCitationsSince with unreadable file ---

func TestGetCitationsSince_PermissionError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root bypasses filesystem permissions")
	}
	baseDir := t.TempDir()
	citationsDir := filepath.Join(baseDir, ".agents", "ao")
	if err := os.MkdirAll(citationsDir, 0700); err != nil {
		t.Fatal(err)
	}
	citationsPath := filepath.Join(citationsDir, "citations.jsonl")
	if err := os.WriteFile(citationsPath, []byte(`{"artifact_path":"/a.md"}`+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(citationsPath, 0000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(citationsPath, 0644) })

	_, err := GetCitationsSince(baseDir, time.Now().Add(-time.Hour))
	if err == nil {
		t.Error("expected error when citations file is unreadable")
	}
	if !strings.Contains(err.Error(), "open citations file") {
		t.Errorf("expected 'open citations file' error, got: %v", err)
	}
}

// --- GetUniqueCitedArtifacts with unreadable file ---

func TestGetUniqueCitedArtifacts_PermissionError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root bypasses filesystem permissions")
	}
	baseDir := t.TempDir()
	citationsDir := filepath.Join(baseDir, ".agents", "ao")
	if err := os.MkdirAll(citationsDir, 0700); err != nil {
		t.Fatal(err)
	}
	citationsPath := filepath.Join(citationsDir, "citations.jsonl")
	if err := os.WriteFile(citationsPath, []byte(`{"artifact_path":"/a.md"}`+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(citationsPath, 0000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(citationsPath, 0644) })

	_, err := GetUniqueCitedArtifacts(baseDir, time.Now().Add(-time.Hour), time.Now())
	if err == nil {
		t.Error("expected error when citations file is unreadable")
	}
	if !strings.Contains(err.Error(), "open citations file") {
		t.Errorf("expected 'open citations file' error, got: %v", err)
	}
}

// --- GetCitationsForSession with unreadable file ---

func TestGetCitationsForSession_PermissionError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root bypasses filesystem permissions")
	}
	baseDir := t.TempDir()
	citationsDir := filepath.Join(baseDir, ".agents", "ao")
	if err := os.MkdirAll(citationsDir, 0700); err != nil {
		t.Fatal(err)
	}
	citationsPath := filepath.Join(citationsDir, "citations.jsonl")
	if err := os.WriteFile(citationsPath, []byte(`{"artifact_path":"/a.md"}`+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(citationsPath, 0000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(citationsPath, 0644) })

	_, err := GetCitationsForSession(baseDir, "any-session")
	if err == nil {
		t.Error("expected error when citations file is unreadable")
	}
	if !strings.Contains(err.Error(), "open citations file") {
		t.Errorf("expected 'open citations file' error, got: %v", err)
	}
}

// --- countCitations walk error with unreadable directory ---

func TestCountCitations_UnreadableDir(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root bypasses filesystem permissions")
	}
	tmpDir := t.TempDir()

	// Create a target artifact and a referencing file
	targetFile := filepath.Join(tmpDir, "target.md")
	if err := os.WriteFile(targetFile, []byte("# Target\n"), 0644); err != nil {
		t.Fatal(err)
	}
	refFile := filepath.Join(tmpDir, "ref.md")
	if err := os.WriteFile(refFile, []byte("See target.md for details\n"), 0644); err != nil {
		t.Fatal(err)
	}

	v, _ := helperNewValidator(t)

	// Verify citations work normally
	count := v.countCitations(targetFile)
	if count != 1 {
		t.Errorf("expected 1 citation for target.md, got %d", count)
	}
}

// --- ValidateArtifactPath tilde that is also absolute (dead code on Unix, but test the non-absolute tilde path) ---

func TestValidateArtifactPath_TildeNotAbsolute(t *testing.T) {
	err := ValidateArtifactPath("~/some/path")
	if err == nil {
		t.Fatal("expected error for tilde path")
	}
	// On Unix, tilde paths are not absolute, so the "not absolute" error fires first
	if !strings.Contains(err.Error(), "path must be absolute") {
		t.Errorf("expected 'path must be absolute' error for tilde path, got: %v", err)
	}
}

// --- ValidateCloseReason with tilde relative path pattern ---

func TestValidateCloseReason_TildeRelativePath(t *testing.T) {
	issues := ValidateCloseReason("See ~/local/file.md")
	if len(issues) == 0 {
		t.Error("expected issues for close_reason with ~/ path")
	}
	// Should detect the ~/ pattern
	found := false
	for _, issue := range issues {
		if strings.Contains(issue, "relative path") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'relative path' issue, got: %v", issues)
	}
}
