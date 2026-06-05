package liveness

import (
	"errors"
	"fmt"
	"sort"
	"time"
)

// Clock returns the current time. Inject a fixed Clock in tests for
// deterministic lease behaviour; production passes time.Now.
type Clock func() time.Time

// LeaseStatus is the computed state of a WorkLease relative to a clock.
type LeaseStatus string

const (
	// LeaseActive means the lease has not yet reached its ExpiresAt.
	LeaseActive LeaseStatus = "active"
	// LeaseExpired means the lease's ExpiresAt is at or before now.
	LeaseExpired LeaseStatus = "expired"
)

// WorkLease is the substrate record of who owns an active work arc and when the
// claim lapses. It makes no admission decision itself; deadman/fallback consume
// expired leases to know which work is abandoned and eligible for quorum
// takeover. A lease stays alive only on externally-visible evidence (renewal
// requires a fresh evidence_ref), never on self-report.
type WorkLease struct {
	BeadID       string    // the work item under lease
	Branch       string    // the worktree branch carrying the work
	OwnerAgentID string    // the agent holding the lease
	ModelFamily  string    // owner's model family (for cross-model accounting)
	LeaseReason  string    // why the lease was taken
	IssuedAt     time.Time // when the lease was first created
	ExpiresAt    time.Time // when the lease lapses absent renewal
	RenewedAt    time.Time // last renewal; zero until first renewal
	EvidenceRef  string    // external evidence justifying the latest lease
}

// Status reports whether the lease is active or expired as of now.
func (l WorkLease) Status(now time.Time) LeaseStatus {
	if !now.Before(l.ExpiresAt) {
		return LeaseExpired
	}
	return LeaseActive
}

// Sentinel errors for lease operations. Callers compare with errors.Is.
var (
	// ErrLeaseNotFound is returned when no lease exists for a BeadID.
	ErrLeaseNotFound = errors.New("liveness: work lease not found")
	// ErrLeaseActive is returned when creating over a still-active lease.
	ErrLeaseActive = errors.New("liveness: an active work lease already exists for bead")
	// ErrEvidenceRequired is returned when a renewal omits fresh evidence —
	// alive means externally-visible evidence, not self-report.
	ErrEvidenceRequired = errors.New("liveness: lease renewal requires a non-empty evidence_ref")
	// ErrLeaseExpired is returned when renewing an already-expired lease.
	// Expiry is a HARD state: an expired lease cannot be resurrected by its old
	// owner; it must go through Create (quorum takeover). This keeps deadman /
	// fallback mechanical instead of scheduler-luck dependent.
	ErrLeaseExpired = errors.New("liveness: cannot renew an expired lease; use Create for takeover")
)

// LeaseTracker is a deterministic in-memory store of work leases keyed by
// BeadID. Every time decision routes through the injected Clock, so the
// behaviour is reproducible and testable.
type LeaseTracker struct {
	now    Clock
	leases map[string]WorkLease
}

// NewLeaseTracker returns a tracker driven by the given Clock. A nil Clock
// defaults to time.Now.
func NewLeaseTracker(now Clock) *LeaseTracker {
	if now == nil {
		now = time.Now
	}
	return &LeaseTracker{now: now, leases: map[string]WorkLease{}}
}

// Create issues a new lease for beadID, expiring ttl from now. It succeeds when
// no lease exists or the existing lease has already expired (quorum takeover
// re-leases abandoned work); it fails on a still-active lease, empty identity
// fields, or a non-positive ttl.
func (t *LeaseTracker) Create(beadID, branch, ownerAgentID, modelFamily, reason, evidenceRef string, ttl time.Duration) (WorkLease, error) {
	if beadID == "" || ownerAgentID == "" {
		return WorkLease{}, errors.New("liveness: Create requires non-empty beadID and ownerAgentID")
	}
	if ttl <= 0 {
		return WorkLease{}, errors.New("liveness: Create requires a positive ttl")
	}
	now := t.now()
	if existing, ok := t.leases[beadID]; ok && existing.Status(now) == LeaseActive {
		return WorkLease{}, fmt.Errorf("%w: %s (owner %s, expires %s)", ErrLeaseActive, beadID, existing.OwnerAgentID, existing.ExpiresAt.Format(time.RFC3339))
	}
	lease := WorkLease{
		BeadID:       beadID,
		Branch:       branch,
		OwnerAgentID: ownerAgentID,
		ModelFamily:  modelFamily,
		LeaseReason:  reason,
		IssuedAt:     now,
		ExpiresAt:    now.Add(ttl),
		EvidenceRef:  evidenceRef,
	}
	t.leases[beadID] = lease
	return lease, nil
}

// Renew extends an active lease by ttl from now and records fresh evidence.
// The evidenceRef MUST be non-empty: a lease is kept alive by externally-visible
// progress, not by self-report. Renewal is for ACTIVE leases only — an already
// expired lease returns ErrLeaseExpired and is left expired (expiry is a hard
// state; takeover goes through Create). Fails if the lease is absent.
func (t *LeaseTracker) Renew(beadID, evidenceRef string, ttl time.Duration) (WorkLease, error) {
	if ttl <= 0 {
		return WorkLease{}, errors.New("liveness: Renew requires a positive ttl")
	}
	if evidenceRef == "" {
		return WorkLease{}, ErrEvidenceRequired
	}
	lease, ok := t.leases[beadID]
	if !ok {
		return WorkLease{}, fmt.Errorf("%w: %s", ErrLeaseNotFound, beadID)
	}
	now := t.now()
	if lease.Status(now) == LeaseExpired {
		return WorkLease{}, fmt.Errorf("%w: %s (expired %s)", ErrLeaseExpired, beadID, lease.ExpiresAt.Format(time.RFC3339))
	}
	lease.RenewedAt = now
	lease.ExpiresAt = now.Add(ttl)
	lease.EvidenceRef = evidenceRef
	t.leases[beadID] = lease
	return lease, nil
}

// Expire force-expires a lease (explicit release or quorum takeover) by setting
// ExpiresAt to now. The record is retained for audit.
func (t *LeaseTracker) Expire(beadID string) (WorkLease, error) {
	lease, ok := t.leases[beadID]
	if !ok {
		return WorkLease{}, fmt.Errorf("%w: %s", ErrLeaseNotFound, beadID)
	}
	lease.ExpiresAt = t.now()
	t.leases[beadID] = lease
	return lease, nil
}

// Get returns the lease for beadID.
func (t *LeaseTracker) Get(beadID string) (WorkLease, error) {
	lease, ok := t.leases[beadID]
	if !ok {
		return WorkLease{}, fmt.Errorf("%w: %s", ErrLeaseNotFound, beadID)
	}
	return lease, nil
}

// List returns all leases sorted by BeadID for deterministic output.
func (t *LeaseTracker) List() []WorkLease {
	out := make([]WorkLease, 0, len(t.leases))
	for _, l := range t.leases {
		out = append(out, l)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].BeadID < out[j].BeadID })
	return out
}

// Expired returns the leases whose ExpiresAt is at or before now, sorted by
// BeadID. deadman/fallback consumes this to mark work for quorum takeover.
func (t *LeaseTracker) Expired() []WorkLease {
	now := t.now()
	var out []WorkLease
	for _, l := range t.List() {
		if l.Status(now) == LeaseExpired {
			out = append(out, l)
		}
	}
	return out
}
