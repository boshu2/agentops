package liveness

import (
	"errors"
	"testing"
	"time"
)

func fixedClock(at time.Time) Clock { return func() time.Time { return at } }

func TestLeaseTracker_CreateSetsFieldsAndTimes(t *testing.T) {
	base := time.Date(2026, 6, 5, 3, 0, 0, 0, time.UTC)
	tr := NewLeaseTracker(fixedClock(base))

	got, err := tr.Create("ag-1", "feat/ag-1", "ruby", "claude", "build slice", "ev-1", time.Hour)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if got.IssuedAt != base {
		t.Fatalf("IssuedAt = %v, want %v", got.IssuedAt, base)
	}
	if got.ExpiresAt != base.Add(time.Hour) {
		t.Fatalf("ExpiresAt = %v, want %v", got.ExpiresAt, base.Add(time.Hour))
	}
	if !got.RenewedAt.IsZero() {
		t.Fatalf("RenewedAt should be zero before renewal, got %v", got.RenewedAt)
	}
	if got.OwnerAgentID != "ruby" || got.ModelFamily != "claude" || got.Branch != "feat/ag-1" || got.EvidenceRef != "ev-1" {
		t.Fatalf("fields not recorded: %+v", got)
	}
	stored, err := tr.Get("ag-1")
	if err != nil || stored.BeadID != "ag-1" {
		t.Fatalf("Get after Create: %+v err=%v", stored, err)
	}
}

func TestLeaseTracker_CreateRejectsInvalidAndActiveDuplicate(t *testing.T) {
	base := time.Date(2026, 6, 5, 3, 0, 0, 0, time.UTC)
	tr := NewLeaseTracker(fixedClock(base))

	if _, err := tr.Create("", "b", "owner", "f", "r", "e", time.Hour); err == nil {
		t.Fatal("empty beadID accepted")
	}
	if _, err := tr.Create("ag-1", "b", "", "f", "r", "e", time.Hour); err == nil {
		t.Fatal("empty ownerAgentID accepted")
	}
	if _, err := tr.Create("ag-1", "b", "owner", "f", "r", "e", 0); err == nil {
		t.Fatal("non-positive ttl accepted")
	}
	if _, err := tr.Create("ag-1", "b", "owner", "f", "r", "e", time.Hour); err != nil {
		t.Fatalf("first Create: %v", err)
	}
	// Duplicate over an ACTIVE lease is rejected.
	if _, err := tr.Create("ag-1", "b", "owner2", "f", "r", "e", time.Hour); !errors.Is(err, ErrLeaseActive) {
		t.Fatalf("duplicate active create: want ErrLeaseActive, got %v", err)
	}
}

func TestLeaseTracker_CreateReleasesExpiredForTakeover(t *testing.T) {
	base := time.Date(2026, 6, 5, 3, 0, 0, 0, time.UTC)
	clk := base
	tr := NewLeaseTracker(func() time.Time { return clk })

	if _, err := tr.Create("ag-1", "b", "ruby", "claude", "r", "e", time.Hour); err != nil {
		t.Fatalf("first Create: %v", err)
	}
	// Advance past expiry; a new owner may take over the abandoned bead.
	clk = base.Add(2 * time.Hour)
	got, err := tr.Create("ag-1", "b2", "windy", "claude", "takeover", "ev-2", time.Hour)
	if err != nil {
		t.Fatalf("takeover Create over expired lease: %v", err)
	}
	if got.OwnerAgentID != "windy" || got.Branch != "b2" || got.IssuedAt != clk {
		t.Fatalf("takeover did not replace owner: %+v", got)
	}
}

func TestLeaseTracker_RenewRequiresFreshEvidence(t *testing.T) {
	base := time.Date(2026, 6, 5, 3, 0, 0, 0, time.UTC)
	clk := base
	tr := NewLeaseTracker(func() time.Time { return clk })
	if _, err := tr.Create("ag-1", "b", "ruby", "claude", "r", "ev-1", time.Hour); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Renewal without evidence is rejected — alive = external evidence.
	if _, err := tr.Renew("ag-1", "", time.Hour); !errors.Is(err, ErrEvidenceRequired) {
		t.Fatalf("renew without evidence: want ErrEvidenceRequired, got %v", err)
	}
	// Renewal with fresh evidence extends expiry and stamps RenewedAt.
	clk = base.Add(30 * time.Minute)
	got, err := tr.Renew("ag-1", "ev-2", time.Hour)
	if err != nil {
		t.Fatalf("Renew: %v", err)
	}
	if got.RenewedAt != clk {
		t.Fatalf("RenewedAt = %v, want %v", got.RenewedAt, clk)
	}
	if got.ExpiresAt != clk.Add(time.Hour) {
		t.Fatalf("ExpiresAt = %v, want %v", got.ExpiresAt, clk.Add(time.Hour))
	}
	if got.EvidenceRef != "ev-2" {
		t.Fatalf("EvidenceRef = %q, want ev-2", got.EvidenceRef)
	}
	// Renewing a missing lease is an error.
	if _, err := tr.Renew("nope", "ev", time.Hour); !errors.Is(err, ErrLeaseNotFound) {
		t.Fatalf("renew missing: want ErrLeaseNotFound, got %v", err)
	}
}

func TestLeaseTracker_StatusListAndExpired(t *testing.T) {
	base := time.Date(2026, 6, 5, 3, 0, 0, 0, time.UTC)
	clk := base
	tr := NewLeaseTracker(func() time.Time { return clk })
	mustCreate(t, tr, "ag-2", 2*time.Hour)
	mustCreate(t, tr, "ag-1", time.Hour)

	// Before any expiry: none expired, list sorted by BeadID.
	if exp := tr.Expired(); len(exp) != 0 {
		t.Fatalf("want 0 expired, got %d", len(exp))
	}
	all := tr.List()
	if len(all) != 2 || all[0].BeadID != "ag-1" || all[1].BeadID != "ag-2" {
		t.Fatalf("List not sorted/complete: %+v", all)
	}

	// Advance past ag-1's expiry only.
	clk = base.Add(90 * time.Minute)
	exp := tr.Expired()
	if len(exp) != 1 || exp[0].BeadID != "ag-1" {
		t.Fatalf("want [ag-1] expired, got %+v", exp)
	}
	if exp[0].Status(clk) != LeaseExpired {
		t.Fatalf("ag-1 status = %s, want expired", exp[0].Status(clk))
	}
	if ag2, _ := tr.Get("ag-2"); ag2.Status(clk) != LeaseActive {
		t.Fatalf("ag-2 status = %s, want active", ag2.Status(clk))
	}
}

func TestLeaseTracker_ExpireForceExpires(t *testing.T) {
	base := time.Date(2026, 6, 5, 3, 0, 0, 0, time.UTC)
	tr := NewLeaseTracker(fixedClock(base))
	mustCreate(t, tr, "ag-1", time.Hour)

	got, err := tr.Expire("ag-1")
	if err != nil {
		t.Fatalf("Expire: %v", err)
	}
	if got.ExpiresAt != base {
		t.Fatalf("force-expire should set ExpiresAt=now (%v), got %v", base, got.ExpiresAt)
	}
	if got.Status(base) != LeaseExpired {
		t.Fatal("force-expired lease should report expired at now")
	}
	if _, err := tr.Expire("nope"); !errors.Is(err, ErrLeaseNotFound) {
		t.Fatalf("expire missing: want ErrLeaseNotFound, got %v", err)
	}
}

func mustCreate(t *testing.T, tr *LeaseTracker, beadID string, ttl time.Duration) {
	t.Helper()
	if _, err := tr.Create(beadID, "branch", "owner", "claude", "reason", "ev", ttl); err != nil {
		t.Fatalf("Create(%s): %v", beadID, err)
	}
}
