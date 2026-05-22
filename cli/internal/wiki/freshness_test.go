package wiki

import (
	"testing"
	"time"
)

// fixedClock returns a Now func pinned to t, for deterministic Evaluate tests.
func fixedClock(t time.Time) func() time.Time {
	return func() time.Time { return t }
}

// TestFreshnessPolicy exercises the spike freshness model across the W3
// acceptance criterion and the surrounding state space. The reference clock is
// pinned so every case is deterministic.
func TestFreshnessPolicy(t *testing.T) {
	now := time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC)
	policy := FreshnessPolicy{Now: fixedClock(now)}

	tests := []struct {
		name         string
		volatility   VolatilityClass
		authority    AuthorityClass
		signal       ChangeSignal
		verifiedAt   time.Time
		wantStates   []FreshnessState // result must be one of these
		wantReviewAt time.Time        // exact expected NextReviewAt
		wantAction   bool             // expect a non-empty RefreshAction
	}{
		{
			// W3 acceptance criterion: release-bound claim + newer release
			// signal => stale, next_review_at set (to now).
			name:       "release-bound with newer release signal is stale",
			volatility: VolatilityReleaseBound,
			authority:  AuthorityAgents,
			signal: ChangeSignal{
				Kind:       ChangeSignalRelease,
				ObservedAt: now.Add(-1 * time.Hour),
				Detail:     "ao 2.5.0",
			},
			verifiedAt:   now.Add(-48 * time.Hour),
			wantStates:   []FreshnessState{FreshnessStale},
			wantReviewAt: now,
			wantAction:   true,
		},
		{
			// No signal, freshly verified => fresh, far-future review.
			name:         "no signal recently verified is fresh",
			volatility:   VolatilityReleaseBound,
			authority:    AuthorityAgents,
			signal:       ChangeSignal{},
			verifiedAt:   now.Add(-1 * time.Hour),
			wantStates:   []FreshnessState{FreshnessFresh},
			wantReviewAt: now.Add(-1 * time.Hour).Add(30 * 24 * time.Hour),
			wantAction:   false,
		},
		{
			// Invariant claim, ancient verification, no signal => still fresh:
			// a one-year review interval has not elapsed.
			name:         "invariant claim old but within interval is fresh",
			volatility:   VolatilityInvariant,
			authority:    AuthorityCode,
			signal:       ChangeSignal{},
			verifiedAt:   now.Add(-90 * 24 * time.Hour),
			wantStates:   []FreshnessState{FreshnessFresh},
			wantReviewAt: now.Add(-90 * 24 * time.Hour).Add(time.Duration(float64(365*24*time.Hour) * 1.5)),
			wantAction:   false,
		},
		{
			// Fast claim, no signal, past its 3-day interval => stale by clock.
			name:         "fast claim past review interval is stale",
			volatility:   VolatilityFast,
			authority:    AuthorityAgents,
			signal:       ChangeSignal{},
			verifiedAt:   now.Add(-10 * 24 * time.Hour),
			wantStates:   []FreshnessState{FreshnessStale},
			wantReviewAt: now,
			wantAction:   true,
		},
		{
			// Ephemeral claim with a newer file-hash signal => stale (ephemeral
			// is version/time-pinned, so any relevant signal contradicts it).
			name:       "ephemeral claim with newer signal is stale",
			volatility: VolatilityEphemeral,
			authority:  AuthorityAgents,
			signal: ChangeSignal{
				Kind:       ChangeSignalFileHash,
				ObservedAt: now.Add(-30 * time.Minute),
			},
			verifiedAt:   now.Add(-2 * time.Hour),
			wantStates:   []FreshnessState{FreshnessStale},
			wantReviewAt: now,
			wantAction:   true,
		},
		{
			// Fast claim with a newer file-hash signal => aging (fast is not
			// version-pinned, so a signal is a re-verify nudge, not a
			// contradiction).
			name:       "fast claim with newer file-hash signal is aging",
			volatility: VolatilityFast,
			authority:  AuthorityCode,
			signal: ChangeSignal{
				Kind:       ChangeSignalFileHash,
				ObservedAt: now.Add(-15 * time.Minute),
			},
			verifiedAt:   now.Add(-1 * time.Hour),
			wantStates:   []FreshnessState{FreshnessAging},
			wantReviewAt: now,
			wantAction:   true,
		},
		{
			// A schema-version signal contradicts even a non-pinned claim.
			name:       "fast claim with schema-version signal is stale",
			volatility: VolatilityFast,
			authority:  AuthoritySchema,
			signal: ChangeSignal{
				Kind:       ChangeSignalSchemaVersion,
				ObservedAt: now.Add(-5 * time.Minute),
				Detail:     "schema v3",
			},
			verifiedAt:   now.Add(-1 * time.Hour),
			wantStates:   []FreshnessState{FreshnessStale},
			wantReviewAt: now,
			wantAction:   true,
		},
		{
			// A stale signal (observed BEFORE the claim was verified) is
			// irrelevant — the claim ages only by the clock and stays fresh.
			name:       "signal older than verification is irrelevant",
			volatility: VolatilityReleaseBound,
			authority:  AuthorityAgents,
			signal: ChangeSignal{
				Kind:       ChangeSignalRelease,
				ObservedAt: now.Add(-100 * 24 * time.Hour),
			},
			verifiedAt:   now.Add(-1 * time.Hour),
			wantStates:   []FreshnessState{FreshnessFresh},
			wantReviewAt: now.Add(-1 * time.Hour).Add(30 * 24 * time.Hour),
			wantAction:   false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			claim := Claim{
				ID:              "c1",
				Text:            "a statement",
				SourceRefs:      []string{"ref"},
				VolatilityClass: tc.volatility,
				AuthorityClass:  tc.authority,
			}
			got := policy.Evaluate(claim, tc.signal, tc.verifiedAt)

			if !got.State.Valid() {
				t.Fatalf("Evaluate returned invalid state %q", got.State)
			}
			matched := false
			for _, want := range tc.wantStates {
				if got.State == want {
					matched = true
					break
				}
			}
			if !matched {
				t.Fatalf("State = %q, want one of %v", got.State, tc.wantStates)
			}

			if got.NextReviewAt.IsZero() {
				t.Fatal("NextReviewAt is the zero time; Evaluate must always set it")
			}
			if !got.NextReviewAt.Equal(tc.wantReviewAt) {
				t.Fatalf("NextReviewAt = %v, want %v", got.NextReviewAt, tc.wantReviewAt)
			}

			if tc.wantAction && got.RefreshAction == "" {
				t.Error("RefreshAction is empty, want a non-empty suggestion")
			}
			if !tc.wantAction && got.RefreshAction != "" {
				t.Errorf("RefreshAction = %q, want empty for a fresh claim", got.RefreshAction)
			}
			if got.Reason == "" {
				t.Error("Reason is empty, want a machine-stable explanation")
			}
		})
	}
}

// TestFreshnessPolicy_AcceptanceCriterion isolates the exact W3 acceptance
// criterion (soc-wiki.5 ac-wiki.5.1) as a standalone assertion.
func TestFreshnessPolicy_AcceptanceCriterion(t *testing.T) {
	now := time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC)
	policy := FreshnessPolicy{Now: fixedClock(now)}

	claim := Claim{
		ID:              "ac-claim",
		Text:            "ao 2.4 exposes the wiki command group",
		SourceRefs:      []string{"README.md"},
		VolatilityClass: VolatilityReleaseBound,
		AuthorityClass:  AuthorityAgents,
	}
	signal := ChangeSignal{
		Kind:       ChangeSignalRelease,
		ObservedAt: now.Add(-2 * time.Hour),
		Detail:     "ao 2.5.0 released",
	}

	got := policy.Evaluate(claim, signal, now.Add(-30*24*time.Hour))

	if got.State != FreshnessAging && got.State != FreshnessStale {
		t.Fatalf("State = %q, want aging or stale", got.State)
	}
	if got.NextReviewAt.IsZero() {
		t.Fatal("NextReviewAt must be set when a newer release signal is observed")
	}
}

// TestFreshnessPolicy_ZeroValueClock verifies the zero-value FreshnessPolicy is
// usable: it falls back to time.Now and still produces a valid verdict.
func TestFreshnessPolicy_ZeroValueClock(t *testing.T) {
	var policy FreshnessPolicy // Now is nil — must fall back to time.Now

	claim := Claim{
		ID:              "z1",
		Text:            "a statement",
		SourceRefs:      []string{"ref"},
		VolatilityClass: VolatilityReleaseBound,
		AuthorityClass:  AuthorityAgents,
	}
	got := policy.Evaluate(claim, ChangeSignal{}, time.Now().Add(-1*time.Hour))

	if got.State != FreshnessFresh {
		t.Fatalf("State = %q, want fresh for a recently verified claim", got.State)
	}
	if got.NextReviewAt.IsZero() {
		t.Fatal("NextReviewAt must be set")
	}
}

// TestFreshnessState_Valid pins the closed set of freshness states.
func TestFreshnessState_Valid(t *testing.T) {
	valid := []FreshnessState{FreshnessFresh, FreshnessAging, FreshnessStale}
	for _, s := range valid {
		if !s.Valid() {
			t.Errorf("FreshnessState %q should be valid", s)
		}
	}
	for _, s := range []FreshnessState{"", "expired", "unknown", "FRESH"} {
		if s.Valid() {
			t.Errorf("FreshnessState %q should be invalid", s)
		}
	}
}
