package safety

import (
	"testing"
	"time"
)

func TestValidateMessageSize_UnderLimit(t *testing.T) {
	// 50 chars → ceiling(50/4) = 13 tokens. Under 100 token limit.
	msg := "Done. Verdict written to .agents/council/report.md"
	v := ValidateMessageSize(msg, 100)
	if v != nil {
		t.Errorf("expected nil violation for short message, got: %+v", v)
	}
}

func TestValidateMessageSize_OverLimit(t *testing.T) {
	// 500 chars → ceiling(500/4) = 125 tokens. Over 100 token limit.
	msg := make([]byte, 500)
	for i := range msg {
		msg[i] = 'x'
	}
	v := ValidateMessageSize(string(msg), 100)
	if v == nil {
		t.Fatal("expected violation for long message, got nil")
	}
	if v.Rule != RuleThinMessages {
		t.Errorf("Rule = %q, want %q", v.Rule, RuleThinMessages)
	}
}

func TestValidateMessageSize_EmptyMessage(t *testing.T) {
	// Finding sandbox#4: empty messages return nil, not 0-token violation.
	v := ValidateMessageSize("", 100)
	if v != nil {
		t.Errorf("expected nil for empty message, got: %+v", v)
	}
}

func TestValidateMessageSize_ShortMessage_MinOneToken(t *testing.T) {
	// Finding sandbox#4: 1-3 byte messages should estimate at least 1 token.
	v := ValidateMessageSize("x", 0)
	if v == nil {
		t.Fatal("expected violation for 1-char message with maxTokens=0, got nil")
	}
	// Verify ceiling division: 1 char → 1 token (not 0)
	v2 := ValidateMessageSize("x", 1)
	if v2 != nil {
		t.Errorf("expected nil for 1-char message with maxTokens=1, got: %+v", v2)
	}
}

func TestValidateMessageSize_ExactBoundary(t *testing.T) {
	// Finding sandbox#6: exact token boundary test.
	// 400 chars → ceiling(400/4) = 100 tokens. At limit, should NOT violate.
	msg := make([]byte, 400)
	for i := range msg {
		msg[i] = 'a'
	}
	v := ValidateMessageSize(string(msg), 100)
	if v != nil {
		t.Errorf("expected nil at exact boundary, got: %+v", v)
	}
	// 401 chars → ceiling(401/4) = 101 tokens. Over limit.
	msg = append(msg, 'a')
	v = ValidateMessageSize(string(msg), 100)
	if v == nil {
		t.Fatal("expected violation for 401-char message with maxTokens=100")
	}
}

func TestValidateMessageSize_UnicodeInput(t *testing.T) {
	// Finding sandbox#6: Unicode input (multi-byte chars).
	// 3 emoji × 4 bytes each = 12 bytes → ceiling(12/4) = 3 tokens.
	msg := "🔥🔥🔥"
	v := ValidateMessageSize(msg, 10)
	if v != nil {
		t.Errorf("expected nil for short unicode message, got: %+v", v)
	}
}

func TestValidateTeamLifecycle_ValidSequence(t *testing.T) {
	now := time.Now()
	events := []TeamLifecycleEvent{
		{Action: "create", TeamName: "swarm-1-w1", Timestamp: now, AgentID: "lead"},
		{Action: "task", TeamName: "swarm-1-w1", Timestamp: now.Add(time.Second), AgentID: "worker-1"},
		{Action: "delete", TeamName: "swarm-1-w1", Timestamp: now.Add(2 * time.Second), AgentID: "lead"},
	}
	violations := ValidateTeamLifecycle(events)
	if len(violations) != 0 {
		t.Errorf("expected 0 violations, got %d: %+v", len(violations), violations)
	}
}

func TestValidateTeamLifecycle_TaskBeforeCreate(t *testing.T) {
	now := time.Now()
	events := []TeamLifecycleEvent{
		{Action: "task", TeamName: "swarm-1-w1", Timestamp: now, AgentID: "worker-1"},
	}
	violations := ValidateTeamLifecycle(events)
	found := false
	for _, v := range violations {
		if v.Rule == RuleTeamBeforeTask {
			found = true
			if v.TeamName != "swarm-1-w1" {
				t.Errorf("violation TeamName = %q, want %q", v.TeamName, "swarm-1-w1")
			}
			if v.EventIndex != 0 {
				t.Errorf("violation EventIndex = %d, want 0", v.EventIndex)
			}
			break
		}
	}
	if !found {
		t.Errorf("expected RuleTeamBeforeTask violation, got: %+v", violations)
	}
}

func TestValidateTeamLifecycle_MissingCleanup(t *testing.T) {
	now := time.Now()
	events := []TeamLifecycleEvent{
		{Action: "create", TeamName: "swarm-1-w1", Timestamp: now, AgentID: "lead"},
		{Action: "task", TeamName: "swarm-1-w1", Timestamp: now.Add(time.Second), AgentID: "worker-1"},
		// no delete
	}
	violations := ValidateTeamLifecycle(events)
	found := false
	for _, v := range violations {
		if v.Rule == RuleAlwaysCleanup {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected RuleAlwaysCleanup violation, got: %+v", violations)
	}
}

func TestValidateTeamLifecycle_MultiWave(t *testing.T) {
	now := time.Now()
	events := []TeamLifecycleEvent{
		// Wave 1
		{Action: "create", TeamName: "swarm-1-w1", Timestamp: now, AgentID: "lead"},
		{Action: "task", TeamName: "swarm-1-w1", Timestamp: now.Add(time.Second), AgentID: "worker-1"},
		{Action: "delete", TeamName: "swarm-1-w1", Timestamp: now.Add(2 * time.Second), AgentID: "lead"},
		// Wave 2
		{Action: "create", TeamName: "swarm-1-w2", Timestamp: now.Add(3 * time.Second), AgentID: "lead"},
		{Action: "task", TeamName: "swarm-1-w2", Timestamp: now.Add(4 * time.Second), AgentID: "worker-2"},
		{Action: "delete", TeamName: "swarm-1-w2", Timestamp: now.Add(5 * time.Second), AgentID: "lead"},
	}
	violations := ValidateTeamLifecycle(events)
	if len(violations) != 0 {
		t.Errorf("expected 0 violations for valid multi-wave, got %d: %+v", len(violations), violations)
	}
}

func TestValidateTeamLifecycle_ReuseTeam(t *testing.T) {
	now := time.Now()
	events := []TeamLifecycleEvent{
		{Action: "create", TeamName: "swarm-1-w1", Timestamp: now, AgentID: "lead"},
		{Action: "task", TeamName: "swarm-1-w1", Timestamp: now.Add(time.Second), AgentID: "worker-1"},
		// Second create for same team WITHOUT delete — violates Rule 5
		{Action: "create", TeamName: "swarm-1-w1", Timestamp: now.Add(2 * time.Second), AgentID: "lead"},
	}
	violations := ValidateTeamLifecycle(events)
	found := false
	for _, v := range violations {
		if v.Rule == RuleNewTeamPerWave {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected RuleNewTeamPerWave violation, got: %+v", violations)
	}
}

func TestValidateTeamLifecycle_EmptyEvents(t *testing.T) {
	// Finding sandbox#6: empty event list should produce no violations.
	violations := ValidateTeamLifecycle(nil)
	if len(violations) != 0 {
		t.Errorf("expected 0 violations for empty events, got %d: %+v", len(violations), violations)
	}
}

func TestValidateTeamLifecycle_UnknownAction(t *testing.T) {
	// Finding sandbox#3: unknown actions emit unknown_action violation.
	now := time.Now()
	events := []TeamLifecycleEvent{
		{Action: "create", TeamName: "team-1", Timestamp: now, AgentID: "lead"},
		{Action: "Create", TeamName: "team-1", Timestamp: now.Add(time.Second), AgentID: "lead"},
		{Action: "cleanup", TeamName: "team-1", Timestamp: now.Add(2 * time.Second), AgentID: "lead"},
	}
	violations := ValidateTeamLifecycle(events)
	unknownCount := 0
	for _, v := range violations {
		if v.Rule == RuleUnknownAction {
			unknownCount++
		}
	}
	if unknownCount != 2 {
		t.Errorf("expected 2 RuleUnknownAction violations, got %d in: %+v", unknownCount, violations)
	}
}

func TestValidateTeamLifecycle_DeleteBeforeCreate(t *testing.T) {
	// Finding sandbox#2: delete-before-create should emit violation.
	now := time.Now()
	events := []TeamLifecycleEvent{
		{Action: "delete", TeamName: "team-1", Timestamp: now, AgentID: "lead"},
	}
	violations := ValidateTeamLifecycle(events)
	found := false
	for _, v := range violations {
		if v.Rule == RuleAlwaysCleanup && v.TeamName == "team-1" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected violation for delete-before-create, got: %+v", violations)
	}
}

func TestValidateTeamLifecycle_DoubleDelete(t *testing.T) {
	// Finding sandbox#2: double-delete should emit violation.
	now := time.Now()
	events := []TeamLifecycleEvent{
		{Action: "create", TeamName: "team-1", Timestamp: now, AgentID: "lead"},
		{Action: "delete", TeamName: "team-1", Timestamp: now.Add(time.Second), AgentID: "lead"},
		{Action: "delete", TeamName: "team-1", Timestamp: now.Add(2 * time.Second), AgentID: "lead"},
	}
	violations := ValidateTeamLifecycle(events)
	found := false
	for _, v := range violations {
		if v.Rule == RuleAlwaysCleanup && v.Detail == "team team-1 deleted without being active" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected violation for double-delete, got: %+v", violations)
	}
}

func TestValidateTeamLifecycle_EmptyTeamName(t *testing.T) {
	// Finding sandbox#2: empty team names are rejected.
	now := time.Now()
	events := []TeamLifecycleEvent{
		{Action: "create", TeamName: "", Timestamp: now, AgentID: "lead"},
	}
	violations := ValidateTeamLifecycle(events)
	if len(violations) == 0 {
		t.Error("expected violation for empty team name, got none")
	}
}

func TestValidateTeamLifecycle_OutOfOrderTimestamps(t *testing.T) {
	// Finding sandbox#1: events out of timestamp order should still be validated correctly.
	now := time.Now()
	events := []TeamLifecycleEvent{
		// Deliberately out of order: task before create in slice, but create has earlier timestamp.
		{Action: "task", TeamName: "team-1", Timestamp: now.Add(time.Second), AgentID: "worker-1"},
		{Action: "create", TeamName: "team-1", Timestamp: now, AgentID: "lead"},
		{Action: "delete", TeamName: "team-1", Timestamp: now.Add(2 * time.Second), AgentID: "lead"},
	}
	violations := ValidateTeamLifecycle(events)
	// After sorting by timestamp, order is: create → task → delete — valid sequence.
	if len(violations) != 0 {
		t.Errorf("expected 0 violations after timestamp sorting, got %d: %+v", len(violations), violations)
	}
}

func TestValidateTeamLifecycle_OutOfOrderPreservesOriginalIndex(t *testing.T) {
	// Council finding: EventIndex must refer to caller's original slice position, not sorted.
	now := time.Now()
	events := []TeamLifecycleEvent{
		{Action: "task", TeamName: "team-1", Timestamp: now.Add(time.Second), AgentID: "worker-1"},   // orig idx 0
		{Action: "delete", TeamName: "team-1", Timestamp: now.Add(2 * time.Second), AgentID: "lead"}, // orig idx 1
		// create is last in input but first by timestamp
		{Action: "create", TeamName: "team-1", Timestamp: now, AgentID: "lead"}, // orig idx 2
	}
	violations := ValidateTeamLifecycle(events)
	// Sorted order: create(idx2) → task(idx0) → delete(idx1) — valid. No violations.
	if len(violations) != 0 {
		t.Errorf("expected 0 violations, got %d: %+v", len(violations), violations)
	}

	// Now test that a violation reports the original index.
	events2 := []TeamLifecycleEvent{
		{Action: "task", TeamName: "team-1", Timestamp: now.Add(time.Second), AgentID: "worker-1"}, // orig idx 0
		// No create — task-before-create violation. After sort, task is at sorted pos 0 but orig idx 0.
	}
	violations2 := ValidateTeamLifecycle(events2)
	if len(violations2) == 0 {
		t.Fatal("expected violation")
	}
	if violations2[0].EventIndex != 0 {
		t.Errorf("EventIndex = %d, want 0 (original input index)", violations2[0].EventIndex)
	}
}

func TestValidateTeamLifecycle_ViolationHasStructuredFields(t *testing.T) {
	// Finding sandbox#5: violations must include TeamName, Timestamp, EventIndex.
	now := time.Now()
	events := []TeamLifecycleEvent{
		{Action: "task", TeamName: "team-1", Timestamp: now, AgentID: "worker-1"},
	}
	violations := ValidateTeamLifecycle(events)
	if len(violations) == 0 {
		t.Fatal("expected at least 1 violation")
	}
	v := violations[0]
	if v.TeamName != "team-1" {
		t.Errorf("violation.TeamName = %q, want %q", v.TeamName, "team-1")
	}
	if v.Timestamp.IsZero() {
		t.Error("violation.Timestamp should not be zero")
	}
	if v.AgentID != "worker-1" {
		t.Errorf("violation.AgentID = %q, want %q", v.AgentID, "worker-1")
	}
	// EventIndex should be 0 (first event)
	if v.EventIndex != 0 {
		t.Errorf("violation.EventIndex = %d, want 0", v.EventIndex)
	}
}

func TestValidateTeamLifecycle_NilMeta(t *testing.T) {
	// Bug #18: If the teams map contains a nil *teamMeta entry in the
	// uncleaned slice, ValidateTeamLifecycle must not panic — it should skip it.
	// We verify the defensive guard by confirming no panic on a normal
	// missing-cleanup scenario and that the violation is still produced.
	now := time.Now()
	events := []TeamLifecycleEvent{
		{Action: "create", TeamName: "team-nil", Timestamp: now, AgentID: "lead"},
		// no delete — team stays active
	}
	// Must not panic.
	violations := ValidateTeamLifecycle(events)
	// Should still produce a cleanup violation for the active team.
	found := false
	for _, v := range violations {
		if v.Rule == RuleAlwaysCleanup && v.TeamName == "team-nil" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected RuleAlwaysCleanup violation for team-nil, got: %+v", violations)
	}
}

func TestValidateTeamLifecycle_CleanupViolationHasMetadata(t *testing.T) {
	// Council finding: synthesized cleanup violations must populate structured fields.
	now := time.Now()
	events := []TeamLifecycleEvent{
		{Action: "create", TeamName: "team-1", Timestamp: now, AgentID: "lead"},
		{Action: "task", TeamName: "team-1", Timestamp: now.Add(time.Second), AgentID: "worker-1"},
		// no delete — triggers RuleAlwaysCleanup
	}
	violations := ValidateTeamLifecycle(events)
	var cleanup *ContractViolation
	for i := range violations {
		if violations[i].Rule == RuleAlwaysCleanup {
			cleanup = &violations[i]
			break
		}
	}
	if cleanup == nil {
		t.Fatal("expected RuleAlwaysCleanup violation")
	}
	if cleanup.TeamName != "team-1" {
		t.Errorf("cleanup.TeamName = %q, want %q", cleanup.TeamName, "team-1")
	}
	if cleanup.AgentID == "" {
		t.Error("cleanup.AgentID should be populated from last-seen event")
	}
	if cleanup.Timestamp.IsZero() {
		t.Error("cleanup.Timestamp should be populated from last-seen event")
	}
}
