// Package beads owns tracker-independent application behavior for the
// top-level beads command family.
package beads

import (
	"sort"
	"time"
)

// StaleBeadRecord is the subset of tracker list output needed for staleness.
type StaleBeadRecord struct {
	ID        string `json:"id"`
	Status    string `json:"status"`
	Assignee  string `json:"assignee"`
	UpdatedAt string `json:"updated_at"`
}

// StaleEvent mirrors stale-claim-event.v1 for stale_detected events.
type StaleEvent struct {
	SchemaVersion    int           `json:"schema_version"`
	EventType        string        `json:"event_type"`
	BeadID           string        `json:"bead_id"`
	DetectedAt       string        `json:"detected_at"`
	OriginalClaimant StaleAgent    `json:"original_claimant"`
	Evidence         StaleEvidence `json:"evidence"`
}

type StaleAgent struct {
	ID string `json:"id"`
}

type StaleEvidence struct {
	LastTouchTS       string  `json:"last_touch_ts,omitempty"`
	ClaimAgeHours     float64 `json:"claim_age_hours,omitempty"`
	ThresholdHours    float64 `json:"threshold_hours,omitempty"`
	LastEvidenceEvent string  `json:"last_evidence_event,omitempty"`
}

// ComputeStaleEvents derives a stable oldest-first event set without reading
// clocks, files, environment, or tracker state.
func ComputeStaleEvents(records []StaleBeadRecord, now time.Time, thresholdHours float64) []StaleEvent {
	var events []StaleEvent
	for _, record := range records {
		if record.Status != "in_progress" || record.UpdatedAt == "" {
			continue
		}
		updated, err := time.Parse(time.RFC3339, record.UpdatedAt)
		if err != nil {
			continue
		}
		ageHours := now.Sub(updated).Hours()
		if ageHours < thresholdHours {
			continue
		}
		claimant := record.Assignee
		if claimant == "" {
			claimant = "unknown"
		}
		events = append(events, StaleEvent{
			SchemaVersion:    1,
			EventType:        "stale_detected",
			BeadID:           record.ID,
			DetectedAt:       now.UTC().Format(time.RFC3339),
			OriginalClaimant: StaleAgent{ID: claimant},
			Evidence: StaleEvidence{
				LastTouchTS:    record.UpdatedAt,
				ClaimAgeHours:  round(ageHours, 1),
				ThresholdHours: thresholdHours,
			},
		})
	}
	sort.SliceStable(events, func(left, right int) bool {
		return events[left].Evidence.ClaimAgeHours > events[right].Evidence.ClaimAgeHours
	})
	return events
}

func round(value float64, decimals int) float64 {
	factor := 1.0
	for range decimals {
		factor *= 10
	}
	return float64(int(value*factor+0.5)) / factor
}
