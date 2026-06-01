package domain

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestTranscriptMessageJSONRoundTrip(t *testing.T) {
	original := TranscriptMessage{
		Type:         "assistant",
		Timestamp:    time.Date(2026, 1, 24, 10, 30, 0, 0, time.UTC),
		Role:         "assistant",
		Content:      "Let me help you with that.",
		SessionID:    "session-123",
		MessageIndex: 5,
		Tools: []ToolCall{
			{
				Name:   "Read",
				Input:  map[string]any{"file_path": "/tmp/test.go"},
				Output: "file contents",
			},
		},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var decoded TranscriptMessage
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if decoded.Type != original.Type {
		t.Errorf("Type mismatch: got %q, want %q", decoded.Type, original.Type)
	}
	if decoded.Role != original.Role {
		t.Errorf("Role mismatch: got %q, want %q", decoded.Role, original.Role)
	}
	if decoded.Content != original.Content {
		t.Errorf("Content mismatch: got %q, want %q", decoded.Content, original.Content)
	}
	if decoded.SessionID != original.SessionID {
		t.Errorf("SessionID mismatch: got %q, want %q", decoded.SessionID, original.SessionID)
	}
	if decoded.MessageIndex != original.MessageIndex {
		t.Errorf("MessageIndex mismatch: got %d, want %d", decoded.MessageIndex, original.MessageIndex)
	}
	if len(decoded.Tools) != 1 {
		t.Fatalf("Tools length mismatch: got %d, want 1", len(decoded.Tools))
	}
	if decoded.Tools[0].Name != "Read" {
		t.Errorf("Tool name mismatch: got %q, want %q", decoded.Tools[0].Name, "Read")
	}
}

func TestCandidateJSONRoundTrip(t *testing.T) {
	original := Candidate{
		ID:      "ol-cand-abc123",
		Type:    KnowledgeTypeDecision,
		Content: "Use context.WithCancel for graceful shutdown",
		Context: "When implementing Go services that need cleanup",
		Source: Source{
			TranscriptPath: "/home/user/.claude/sessions/abc.jsonl",
			MessageIndex:   42,
			Timestamp:      time.Date(2026, 1, 24, 10, 30, 0, 0, time.UTC),
			SessionID:      "session-123",
		},
		RawScore:      0.87,
		Tier:          TierGold,
		ProvenanceIDs: []string{"prov-1", "prov-2"},
		ExtractedAt:   time.Date(2026, 1, 24, 10, 35, 0, 0, time.UTC),
		Metadata: map[string]any{
			"extractor": "transcript-forge",
		},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var decoded Candidate
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if decoded.ID != original.ID {
		t.Errorf("ID mismatch: got %q, want %q", decoded.ID, original.ID)
	}
	if decoded.Type != original.Type {
		t.Errorf("Type mismatch: got %q, want %q", decoded.Type, original.Type)
	}
	if decoded.Content != original.Content {
		t.Errorf("Content mismatch: got %q, want %q", decoded.Content, original.Content)
	}
	if decoded.RawScore != original.RawScore {
		t.Errorf("RawScore mismatch: got %f, want %f", decoded.RawScore, original.RawScore)
	}
	if decoded.Tier != original.Tier {
		t.Errorf("Tier mismatch: got %q, want %q", decoded.Tier, original.Tier)
	}
	if decoded.Source.TranscriptPath != original.Source.TranscriptPath {
		t.Errorf("Source.TranscriptPath mismatch: got %q, want %q",
			decoded.Source.TranscriptPath, original.Source.TranscriptPath)
	}
	if len(decoded.ProvenanceIDs) != len(original.ProvenanceIDs) {
		t.Errorf("ProvenanceIDs length mismatch: got %d, want %d",
			len(decoded.ProvenanceIDs), len(original.ProvenanceIDs))
	}
}

func TestScoringJSONRoundTrip(t *testing.T) {
	original := Scoring{
		RawScore:       0.82,
		TierAssignment: TierSilver,
		Rubric: RubricScores{
			Specificity:   0.85,
			Actionability: 0.80,
			Novelty:       0.75,
			Context:       0.90,
			Confidence:    0.70,
		},
		GateRequired: false,
		ScoredAt:     time.Date(2026, 1, 24, 10, 40, 0, 0, time.UTC),
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var decoded Scoring
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if decoded.RawScore != original.RawScore {
		t.Errorf("RawScore mismatch: got %f, want %f", decoded.RawScore, original.RawScore)
	}
	if decoded.TierAssignment != original.TierAssignment {
		t.Errorf("TierAssignment mismatch: got %q, want %q", decoded.TierAssignment, original.TierAssignment)
	}
	if decoded.Rubric.Specificity != original.Rubric.Specificity {
		t.Errorf("Rubric.Specificity mismatch: got %f, want %f",
			decoded.Rubric.Specificity, original.Rubric.Specificity)
	}
	if decoded.GateRequired != original.GateRequired {
		t.Errorf("GateRequired mismatch: got %v, want %v", decoded.GateRequired, original.GateRequired)
	}
}

func TestPoolEntryJSONRoundTrip(t *testing.T) {
	reviewTime := time.Date(2026, 1, 24, 11, 0, 0, 0, time.UTC)
	original := PoolEntry{
		Candidate: Candidate{
			ID:      "ol-cand-xyz789",
			Type:    KnowledgeTypeSolution,
			Content: "Fix rate limiting with backoff",
		},
		ScoringResult: Scoring{
			RawScore:       0.65,
			TierAssignment: TierBronze,
			GateRequired:   true,
		},
		HumanReview: &HumanReview{
			Reviewed:   true,
			Approved:   true,
			Reviewer:   "boden",
			Notes:      "Good insight",
			ReviewedAt: reviewTime,
		},
		Status:    PoolStatusArchived,
		AddedAt:   time.Date(2026, 1, 24, 10, 45, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 1, 24, 11, 0, 0, 0, time.UTC),
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var decoded PoolEntry
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if decoded.Status != original.Status {
		t.Errorf("Status mismatch: got %q, want %q", decoded.Status, original.Status)
	}
	if decoded.HumanReview == nil {
		t.Fatal("HumanReview is nil")
	}
	if decoded.HumanReview.Approved != original.HumanReview.Approved {
		t.Errorf("HumanReview.Approved mismatch: got %v, want %v",
			decoded.HumanReview.Approved, original.HumanReview.Approved)
	}
	if decoded.HumanReview.Reviewer != original.HumanReview.Reviewer {
		t.Errorf("HumanReview.Reviewer mismatch: got %q, want %q",
			decoded.HumanReview.Reviewer, original.HumanReview.Reviewer)
	}
}

func TestKnowledgeTypeValues(t *testing.T) {
	types := []KnowledgeType{
		KnowledgeTypeDecision,
		KnowledgeTypeSolution,
		KnowledgeTypeLearning,
		KnowledgeTypeFailure,
		KnowledgeTypeReference,
	}

	expected := []string{"decision", "solution", "learning", "failure", "reference"}

	for i, kt := range types {
		if string(kt) != expected[i] {
			t.Errorf("KnowledgeType value mismatch: got %q, want %q", kt, expected[i])
		}
	}
}

func TestTierValues(t *testing.T) {
	tiers := []Tier{TierGold, TierSilver, TierBronze, TierDiscard}
	expected := []string{"gold", "silver", "bronze", "discard"}

	for i, tier := range tiers {
		if string(tier) != expected[i] {
			t.Errorf("Tier value mismatch: got %q, want %q", tier, expected[i])
		}
	}
}

func TestPoolStatusValues(t *testing.T) {
	statuses := []PoolStatus{
		PoolStatusPending,
		PoolStatusStaged,
		PoolStatusArchived,
		PoolStatusRejected,
	}
	expected := []string{"pending", "staged", "archived", "rejected"}

	for i, status := range statuses {
		if string(status) != expected[i] {
			t.Errorf("PoolStatus value mismatch: got %q, want %q", status, expected[i])
		}
	}
}

// --- Supersession tests (ol-a46.1.4) ---

func TestSupersede(t *testing.T) {
	older := &Candidate{
		ID:                "L1",
		Type:              KnowledgeTypeLearning,
		Content:           "Original learning",
		IsCurrent:         true,
		SupersessionDepth: 0,
	}

	newer := &Candidate{
		ID:      "L2",
		Type:    KnowledgeTypeLearning,
		Content: "Updated learning",
	}

	err := Supersede(older, newer)
	if err != nil {
		t.Fatalf("Supersede failed: %v", err)
	}

	// Check older candidate
	if older.SupersededBy != "L2" {
		t.Errorf("older.SupersededBy: got %q, want %q", older.SupersededBy, "L2")
	}
	if older.IsCurrent {
		t.Error("older.IsCurrent should be false")
	}

	// Check newer candidate
	if newer.Supersedes != "L1" {
		t.Errorf("newer.Supersedes: got %q, want %q", newer.Supersedes, "L1")
	}
	if !newer.IsCurrent {
		t.Error("newer.IsCurrent should be true")
	}
	if newer.SupersessionDepth != 1 {
		t.Errorf("newer.SupersessionDepth: got %d, want 1", newer.SupersessionDepth)
	}
}

func TestSupersede_MaxDepth(t *testing.T) {
	// Create a chain at max depth
	older := &Candidate{
		ID:                "L3",
		Type:              KnowledgeTypeLearning,
		Content:           "Learning at max depth",
		IsCurrent:         true,
		SupersessionDepth: MaxSupersessionDepth, // Already at max
	}

	newer := &Candidate{
		ID:      "L4",
		Type:    KnowledgeTypeLearning,
		Content: "Would exceed max depth",
	}

	err := Supersede(older, newer)
	if err == nil {
		t.Fatal("Expected error for exceeding max depth, got nil")
	}

	var supersessionErr *SupersessionError
	if !errors.As(err, &supersessionErr) {
		t.Fatalf("Expected SupersessionError, got %T", err)
	}

	if supersessionErr.Depth != MaxSupersessionDepth+1 {
		t.Errorf("Error depth: got %d, want %d", supersessionErr.Depth, MaxSupersessionDepth+1)
	}
}

func TestValidateSupersessionDepth(t *testing.T) {
	tests := []struct {
		name    string
		depth   int
		wantErr bool
	}{
		{"depth 0", 0, false},
		{"depth 1", 1, false},
		{"depth 2", 2, false},
		{"depth 3 (max)", 3, false},
		{"depth 4 (exceeds)", 4, true},
		{"depth 10 (way over)", 10, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Candidate{
				ID:                "test",
				SupersessionDepth: tt.depth,
			}

			err := ValidateSupersessionDepth(c)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateSupersessionDepth() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCandidate_IsSuperseded(t *testing.T) {
	notSuperseded := &Candidate{ID: "L1"}
	if notSuperseded.IsSuperseded() {
		t.Error("Expected IsSuperseded() = false for empty SupersededBy")
	}

	superseded := &Candidate{ID: "L1", SupersededBy: "L2"}
	if !superseded.IsSuperseded() {
		t.Error("Expected IsSuperseded() = true when SupersededBy is set")
	}
}

func TestCandidateSupersessionJSONRoundTrip(t *testing.T) {
	original := Candidate{
		ID:                "ol-cand-learning-001",
		Type:              KnowledgeTypeLearning,
		Content:           "Updated learning about context usage",
		IsCurrent:         true,
		Supersedes:        "ol-cand-learning-000",
		SupersededBy:      "",
		SupersessionDepth: 1,
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var decoded Candidate
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if decoded.IsCurrent != original.IsCurrent {
		t.Errorf("IsCurrent mismatch: got %v, want %v", decoded.IsCurrent, original.IsCurrent)
	}
	if decoded.Supersedes != original.Supersedes {
		t.Errorf("Supersedes mismatch: got %q, want %q", decoded.Supersedes, original.Supersedes)
	}
	if decoded.SupersededBy != original.SupersededBy {
		t.Errorf("SupersededBy mismatch: got %q, want %q", decoded.SupersededBy, original.SupersededBy)
	}
	if decoded.SupersessionDepth != original.SupersessionDepth {
		t.Errorf("SupersessionDepth mismatch: got %d, want %d", decoded.SupersessionDepth, original.SupersessionDepth)
	}
}

// --- ValidUntil parsing tests (ag-p43.18) ---

func TestCandidateIsExpired_DateFormats(t *testing.T) {
	tests := []struct {
		name        string
		validUntil  string
		wantExpired bool
	}{
		{
			name:        "empty validUntil",
			validUntil:  "",
			wantExpired: false,
		},
		{
			name:        "date-only format future",
			validUntil:  "2099-12-31",
			wantExpired: false,
		},
		{
			name:        "date-only format past",
			validUntil:  "2020-01-01",
			wantExpired: true,
		},
		{
			name:        "RFC3339 format future",
			validUntil:  "2099-12-31T23:59:59Z",
			wantExpired: false,
		},
		{
			name:        "RFC3339 format past",
			validUntil:  "2020-01-01T00:00:00Z",
			wantExpired: true,
		},
		{
			name:        "RFC3339 with timezone",
			validUntil:  "2099-06-15T12:00:00-07:00",
			wantExpired: false,
		},
		{
			name:        "invalid format",
			validUntil:  "not-a-date",
			wantExpired: false, // Invalid dates return false (not expired)
		},
		{
			name:        "partial date format",
			validUntil:  "2026-01",
			wantExpired: false, // Invalid format returns false
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Candidate{
				ID:         "test",
				ValidUntil: tt.validUntil,
			}
			if got := c.IsExpired(); got != tt.wantExpired {
				t.Errorf("IsExpired() = %v, want %v for validUntil=%q",
					got, tt.wantExpired, tt.validUntil)
			}
		})
	}
}

func TestUpdateExpiryStatus(t *testing.T) {
	tests := []struct {
		name           string
		validUntil     string
		initialStatus  ExpiryStatus
		expectedStatus ExpiryStatus
	}{
		{
			name:           "expired date sets expired",
			validUntil:     "2020-01-01",
			initialStatus:  ExpiryStatusActive,
			expectedStatus: ExpiryStatusExpired,
		},
		{
			name:           "future date stays active",
			validUntil:     "2099-12-31",
			initialStatus:  ExpiryStatusActive,
			expectedStatus: ExpiryStatusActive,
		},
		{
			name:           "empty validUntil stays active",
			validUntil:     "",
			initialStatus:  ExpiryStatusActive,
			expectedStatus: ExpiryStatusActive,
		},
		{
			name:           "archived status not overwritten by expired",
			validUntil:     "2020-01-01",
			initialStatus:  ExpiryStatusArchived,
			expectedStatus: ExpiryStatusArchived,
		},
		{
			name:           "archived status not overwritten by active",
			validUntil:     "2099-12-31",
			initialStatus:  ExpiryStatusArchived,
			expectedStatus: ExpiryStatusArchived,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Candidate{
				ID:           "test",
				ValidUntil:   tt.validUntil,
				ExpiryStatus: tt.initialStatus,
			}
			c.UpdateExpiryStatus()
			if c.ExpiryStatus != tt.expectedStatus {
				t.Errorf("ExpiryStatus = %q, want %q", c.ExpiryStatus, tt.expectedStatus)
			}
		})
	}
}

func TestSupersedeChain(t *testing.T) {
	// Build a chain L1 -> L2 -> L3 and verify depth tracking
	l1 := &Candidate{ID: "L1", IsCurrent: true, SupersessionDepth: 0}
	l2 := &Candidate{ID: "L2"}
	l3 := &Candidate{ID: "L3"}

	if err := Supersede(l1, l2); err != nil {
		t.Fatalf("Supersede L1->L2: %v", err)
	}
	if err := Supersede(l2, l3); err != nil {
		t.Fatalf("Supersede L2->L3: %v", err)
	}

	if l3.SupersessionDepth != 2 {
		t.Errorf("L3 depth: got %d, want 2", l3.SupersessionDepth)
	}
	if l1.IsCurrent || l2.IsCurrent {
		t.Error("L1 and L2 should not be current")
	}
	if !l3.IsCurrent {
		t.Error("L3 should be current")
	}
}

func TestSupersessionError_Error(t *testing.T) {
	e := &SupersessionError{
		Message: "test error message",
		ChainID: "L1",
		Depth:   4,
	}
	if got := e.Error(); got != "test error message" {
		t.Errorf("Error() = %q, want %q", got, "test error message")
	}
}

func TestGetKnowledgeTier(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		want     KnowledgeTier
	}{
		{"strict", "STRICT", KnowledgeTierStrict},
		{"standard", "STANDARD", KnowledgeTierStandard},
		{"minimal", "MINIMAL", KnowledgeTierMinimal},
		{"empty defaults to standard", "", KnowledgeTierStandard},
		{"invalid defaults to standard", "INVALID", KnowledgeTierStandard},
		{"lowercase defaults to standard", "strict", KnowledgeTierStandard},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(KnowledgeTierEnvVar, tt.envValue)
			if got := GetKnowledgeTier(); got != tt.want {
				t.Errorf("GetKnowledgeTier() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestKnowledgeTier_MCPRequired(t *testing.T) {
	tests := []struct {
		tier KnowledgeTier
		want bool
	}{
		{KnowledgeTierStrict, true},
		{KnowledgeTierStandard, false},
		{KnowledgeTierMinimal, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.tier), func(t *testing.T) {
			if got := tt.tier.MCPRequired(); got != tt.want {
				t.Errorf("MCPRequired() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestKnowledgeTier_MCPEnabled(t *testing.T) {
	tests := []struct {
		tier KnowledgeTier
		want bool
	}{
		{KnowledgeTierStrict, true},
		{KnowledgeTierStandard, true},
		{KnowledgeTierMinimal, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.tier), func(t *testing.T) {
			if got := tt.tier.MCPEnabled(); got != tt.want {
				t.Errorf("MCPEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMCPError_Error(t *testing.T) {
	e := &MCPError{
		Tier:    KnowledgeTierStrict,
		Message: "MCP unavailable",
	}
	if got := e.Error(); got != "MCP unavailable" {
		t.Errorf("Error() = %q, want %q", got, "MCP unavailable")
	}
}

func TestHandleMCPFailure(t *testing.T) {
	tests := []struct {
		name      string
		tier      KnowledgeTier
		operation string
		wantErr   bool
	}{
		{"strict returns error", KnowledgeTierStrict, "inject", true},
		{"standard returns nil", KnowledgeTierStandard, "inject", false},
		{"minimal returns nil", KnowledgeTierMinimal, "inject", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := HandleMCPFailure(tt.tier, tt.operation, fmt.Errorf("connection refused"))
			if (err != nil) != tt.wantErr {
				t.Errorf("HandleMCPFailure() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				var mcpErr *MCPError
				if !errors.As(err, &mcpErr) {
					t.Fatalf("Expected *MCPError, got %T", err)
				}
				if mcpErr.Tier != KnowledgeTierStrict {
					t.Errorf("MCPError.Tier = %q, want %q", mcpErr.Tier, KnowledgeTierStrict)
				}
			}
		})
	}
}

func TestGetTierBehaviors(t *testing.T) {
	behaviors := GetTierBehaviors()
	if len(behaviors) != 3 {
		t.Fatalf("GetTierBehaviors() returned %d items, want 3", len(behaviors))
	}

	// Verify each tier is represented
	tiers := map[KnowledgeTier]bool{}
	for _, b := range behaviors {
		tiers[b.Tier] = true
		if b.Description == "" {
			t.Errorf("Tier %q has empty description", b.Tier)
		}
		// Verify MCPRequired/MCPEnabled match the method results
		if b.MCPRequired != b.Tier.MCPRequired() {
			t.Errorf("Tier %q: behavior MCPRequired=%v, method=%v", b.Tier, b.MCPRequired, b.Tier.MCPRequired())
		}
		if b.MCPEnabled != b.Tier.MCPEnabled() {
			t.Errorf("Tier %q: behavior MCPEnabled=%v, method=%v", b.Tier, b.MCPEnabled, b.Tier.MCPEnabled())
		}
	}
	for _, tier := range []KnowledgeTier{KnowledgeTierStrict, KnowledgeTierStandard, KnowledgeTierMinimal} {
		if !tiers[tier] {
			t.Errorf("Missing tier %q in behaviors", tier)
		}
	}
}

func TestEscapeVelocityStatus(t *testing.T) {
	t.Run("nil metrics", func(t *testing.T) {
		var m *FlywheelMetrics
		if got := m.EscapeVelocityStatus(); got != "UNKNOWN" {
			t.Fatalf("EscapeVelocityStatus() = %q, want UNKNOWN", got)
		}
	})

	tests := []struct {
		name                string
		aboveEscapeVelocity bool
		velocity            float64
		want                string
	}{
		{"compounding", true, 0.1, "COMPOUNDING"},
		{"near escape", false, -0.03, "NEAR ESCAPE"},
		{"near escape at boundary", false, -0.05, "DECAYING"},
		{"decaying", false, -0.2, "DECAYING"},
		{"zero velocity not above", false, 0.0, "NEAR ESCAPE"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &FlywheelMetrics{
				AboveEscapeVelocity: tt.aboveEscapeVelocity,
				Velocity:            tt.velocity,
			}
			if got := m.EscapeVelocityStatus(); got != tt.want {
				t.Errorf("EscapeVelocityStatus() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestHealthStatus(t *testing.T) {
	tests := []struct {
		name string
		m    *FlywheelMetrics
		want string
	}{
		{
			name: "nil metrics",
			m:    nil,
			want: "UNKNOWN",
		},
		{
			name: "golden signal verdict wins",
			m: &FlywheelMetrics{
				AboveEscapeVelocity: true,
				Velocity:            0.4,
				GoldenSignals: &GoldenSignals{
					OverallVerdict: "accumulating",
				},
			},
			want: "ACCUMULATING",
		},
		{
			name: "near escape falls back to accumulating",
			m: &FlywheelMetrics{
				Velocity: -0.01,
			},
			want: "ACCUMULATING",
		},
		{
			name: "escape velocity compounding fallback",
			m: &FlywheelMetrics{
				AboveEscapeVelocity: true,
				Velocity:            0.1,
			},
			want: "COMPOUNDING",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.m.HealthStatus(); got != tt.want {
				t.Errorf("HealthStatus() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestHealthCompounding(t *testing.T) {
	tests := []struct {
		name string
		m    *FlywheelMetrics
		want bool
	}{
		{name: "nil metrics", m: nil, want: false},
		{
			name: "golden signal compounding",
			m: &FlywheelMetrics{
				AboveEscapeVelocity: false,
				GoldenSignals: &GoldenSignals{
					OverallVerdict: "compounding",
				},
			},
			want: true,
		},
		{
			name: "golden signal non-compounding wins",
			m: &FlywheelMetrics{
				AboveEscapeVelocity: true,
				GoldenSignals: &GoldenSignals{
					OverallVerdict: "decaying",
				},
			},
			want: false,
		},
		{
			name: "falls back to escape velocity",
			m: &FlywheelMetrics{
				AboveEscapeVelocity: true,
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.m.HealthCompounding(); got != tt.want {
				t.Fatalf("HealthCompounding() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExpiryStatusValues(t *testing.T) {
	statuses := []ExpiryStatus{ExpiryStatusActive, ExpiryStatusExpired, ExpiryStatusArchived}
	expected := []string{"active", "expired", "archived"}

	for i, s := range statuses {
		if string(s) != expected[i] {
			t.Errorf("ExpiryStatus value mismatch: got %q, want %q", s, expected[i])
		}
	}
}

func TestMaturityValues(t *testing.T) {
	maturities := []Maturity{MaturityProvisional, MaturityCandidate, MaturityEstablished, MaturityAntiPattern}
	expected := []string{"provisional", "candidate", "established", "anti-pattern"}

	for i, m := range maturities {
		if string(m) != expected[i] {
			t.Errorf("Maturity value mismatch: got %q, want %q", m, expected[i])
		}
	}
}

func TestPlanStatusValues(t *testing.T) {
	statuses := []PlanStatus{PlanStatusActive, PlanStatusCompleted, PlanStatusAbandoned, PlanStatusSuperseded}
	expected := []string{"active", "completed", "abandoned", "superseded"}

	for i, s := range statuses {
		if string(s) != expected[i] {
			t.Errorf("PlanStatus value mismatch: got %q, want %q", s, expected[i])
		}
	}
}

func TestKnowledgeTierValues(t *testing.T) {
	tiers := []KnowledgeTier{KnowledgeTierStrict, KnowledgeTierStandard, KnowledgeTierMinimal}
	expected := []string{"STRICT", "STANDARD", "MINIMAL"}

	for i, tier := range tiers {
		if string(tier) != expected[i] {
			t.Errorf("KnowledgeTier value mismatch: got %q, want %q", tier, expected[i])
		}
	}
}

func TestCitationEventJSONRoundTrip(t *testing.T) {
	original := CitationEvent{
		ArtifactPath:    "/home/user/.agents/learnings/test.md",
		SessionID:       "session-456",
		CitedAt:         time.Date(2026, 2, 1, 14, 0, 0, 0, time.UTC),
		CitationType:    "retrieved",
		MetricNamespace: "shadow",
		MatchConfidence: 0.75,
		MatchProvenance: "substring",
		SectionHeading:  "Acceptance Criteria",
		SectionLocator:  "L10-L18",
		Query:           "context cancellation",
		FeedbackGiven:   true,
		FeedbackReward:  0.8,
		UtilityBefore:   0.5,
		UtilityAfter:    0.53,
		FeedbackAt:      time.Date(2026, 2, 1, 14, 5, 0, 0, time.UTC),
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var decoded CitationEvent
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if decoded.ArtifactPath != original.ArtifactPath {
		t.Errorf("ArtifactPath mismatch: got %q, want %q", decoded.ArtifactPath, original.ArtifactPath)
	}
	if decoded.CitationType != original.CitationType {
		t.Errorf("CitationType mismatch: got %q, want %q", decoded.CitationType, original.CitationType)
	}
	if decoded.MetricNamespace != original.MetricNamespace {
		t.Errorf("MetricNamespace mismatch: got %q, want %q", decoded.MetricNamespace, original.MetricNamespace)
	}
	if decoded.MatchConfidence != original.MatchConfidence {
		t.Errorf("MatchConfidence mismatch: got %f, want %f", decoded.MatchConfidence, original.MatchConfidence)
	}
	if decoded.MatchProvenance != original.MatchProvenance {
		t.Errorf("MatchProvenance mismatch: got %q, want %q", decoded.MatchProvenance, original.MatchProvenance)
	}
	if decoded.SectionHeading != original.SectionHeading {
		t.Errorf("SectionHeading mismatch: got %q, want %q", decoded.SectionHeading, original.SectionHeading)
	}
	if decoded.SectionLocator != original.SectionLocator {
		t.Errorf("SectionLocator mismatch: got %q, want %q", decoded.SectionLocator, original.SectionLocator)
	}
	if decoded.FeedbackGiven != original.FeedbackGiven {
		t.Errorf("FeedbackGiven mismatch: got %v, want %v", decoded.FeedbackGiven, original.FeedbackGiven)
	}
	if decoded.FeedbackReward != original.FeedbackReward {
		t.Errorf("FeedbackReward mismatch: got %f, want %f", decoded.FeedbackReward, original.FeedbackReward)
	}
	if decoded.UtilityAfter != original.UtilityAfter {
		t.Errorf("UtilityAfter mismatch: got %f, want %f", decoded.UtilityAfter, original.UtilityAfter)
	}
}

func TestPlanManifestEntryJSONRoundTrip(t *testing.T) {
	original := PlanManifestEntry{
		Path:         ".agents/plans/peaceful-stirring-tome.md",
		CreatedAt:    time.Date(2026, 2, 1, 10, 0, 0, 0, time.UTC),
		ProjectPath:  "/home/user/project",
		PlanName:     "peaceful-stirring-tome",
		Status:       PlanStatusActive,
		BeadsID:      "ag-abc",
		UpdatedAt:    time.Date(2026, 2, 1, 11, 0, 0, 0, time.UTC),
		Checksum:     "sha256:abcdef",
		SupersededBy: "",
		Metadata:     map[string]any{"source": "plan-skill"},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var decoded PlanManifestEntry
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if decoded.Path != original.Path {
		t.Errorf("Path mismatch: got %q, want %q", decoded.Path, original.Path)
	}
	if decoded.PlanName != original.PlanName {
		t.Errorf("PlanName mismatch: got %q, want %q", decoded.PlanName, original.PlanName)
	}
	if decoded.Status != original.Status {
		t.Errorf("Status mismatch: got %q, want %q", decoded.Status, original.Status)
	}
	if decoded.BeadsID != original.BeadsID {
		t.Errorf("BeadsID mismatch: got %q, want %q", decoded.BeadsID, original.BeadsID)
	}
	if decoded.Checksum != original.Checksum {
		t.Errorf("Checksum mismatch: got %q, want %q", decoded.Checksum, original.Checksum)
	}
}

func TestFlywheelMetricsJSONRoundTrip(t *testing.T) {
	original := FlywheelMetrics{
		Timestamp:            time.Date(2026, 2, 1, 12, 0, 0, 0, time.UTC),
		PeriodStart:          time.Date(2026, 1, 25, 0, 0, 0, 0, time.UTC),
		PeriodEnd:            time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC),
		Delta:                0.17,
		Sigma:                0.6,
		Rho:                  0.4,
		SigmaRho:             0.24,
		Velocity:             0.07,
		AboveEscapeVelocity:  true,
		TotalArtifacts:       50,
		CitationsThisPeriod:  12,
		UniqueCitedArtifacts: 8,
		NewArtifacts:         5,
		StaleArtifacts:       3,
		TierCounts:           map[string]int{"gold": 10, "silver": 20, "bronze": 15},
		MeanUtility:          0.65,
		HighUtilityCount:     15,
		LowUtilityCount:      5,
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var decoded FlywheelMetrics
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if decoded.Delta != original.Delta {
		t.Errorf("Delta mismatch: got %f, want %f", decoded.Delta, original.Delta)
	}
	if decoded.AboveEscapeVelocity != original.AboveEscapeVelocity {
		t.Errorf("AboveEscapeVelocity mismatch: got %v, want %v", decoded.AboveEscapeVelocity, original.AboveEscapeVelocity)
	}
	if decoded.TotalArtifacts != original.TotalArtifacts {
		t.Errorf("TotalArtifacts mismatch: got %d, want %d", decoded.TotalArtifacts, original.TotalArtifacts)
	}
	if decoded.TierCounts["gold"] != 10 {
		t.Errorf("TierCounts[gold] mismatch: got %d, want 10", decoded.TierCounts["gold"])
	}
	if decoded.MeanUtility != original.MeanUtility {
		t.Errorf("MeanUtility mismatch: got %f, want %f", decoded.MeanUtility, original.MeanUtility)
	}
}

func TestConstants(t *testing.T) {
	// Verify key constants have expected values
	if MaxSupersessionDepth != 3 {
		t.Errorf("MaxSupersessionDepth = %d, want 3", MaxSupersessionDepth)
	}
	if DefaultDelta != 0.17 {
		t.Errorf("DefaultDelta = %f, want 0.17", DefaultDelta)
	}
	if DefaultAlpha != 0.1 {
		t.Errorf("DefaultAlpha = %f, want 0.1", DefaultAlpha)
	}
	if DefaultLambda != 0.5 {
		t.Errorf("DefaultLambda = %f, want 0.5", DefaultLambda)
	}
	if InitialUtility != 0.5 {
		t.Errorf("InitialUtility = %f, want 0.5", InitialUtility)
	}
	if KnowledgeTierEnvVar != "KNOWLEDGE_TIER" {
		t.Errorf("KnowledgeTierEnvVar = %q, want %q", KnowledgeTierEnvVar, "KNOWLEDGE_TIER")
	}
	if MaturityPromotionThreshold != 0.55 {
		t.Errorf("MaturityPromotionThreshold = %f, want 0.55", MaturityPromotionThreshold)
	}
	if MaturityDemotionThreshold != 0.3 {
		t.Errorf("MaturityDemotionThreshold = %f, want 0.3", MaturityDemotionThreshold)
	}
	if MaturityAntiPatternThreshold != 0.2 {
		t.Errorf("MaturityAntiPatternThreshold = %f, want 0.2", MaturityAntiPatternThreshold)
	}
	if MinFeedbackForPromotion != 3 {
		t.Errorf("MinFeedbackForPromotion = %d, want 3", MinFeedbackForPromotion)
	}
	if MinFeedbackForAntiPattern != 3 {
		t.Errorf("MinFeedbackForAntiPattern = %d, want 3", MinFeedbackForAntiPattern)
	}
	if ConfidenceDecayRate != 0.1 {
		t.Errorf("ConfidenceDecayRate = %f, want 0.1", ConfidenceDecayRate)
	}
}

func TestParseValidUntil(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "date-only format",
			input:   "2026-06-30",
			wantErr: false,
		},
		{
			name:    "RFC3339 format",
			input:   "2026-06-30T15:30:00Z",
			wantErr: false,
		},
		{
			name:    "RFC3339 with timezone",
			input:   "2026-06-30T15:30:00-05:00",
			wantErr: false,
		},
		{
			name:    "invalid format",
			input:   "06/30/2026",
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
		{
			name:    "partial ISO format",
			input:   "2026-06",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseValidUntil(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseValidUntil(%q) error = %v, wantErr %v",
					tt.input, err, tt.wantErr)
			}
		})
	}
}
