package beads

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"
)

type TransferInfo struct {
	PriorRevision string `json:"prior_revision"`
	NewRevision   string `json:"new_revision"`
	NotesAppended bool   `json:"notes_appended"`
}

type TransferredEvent struct {
	StaleEvent
	NewClaimant StaleAgent   `json:"new_claimant"`
	Transfer    TransferInfo `json:"transfer"`
}

func ParseShownBead(raw []byte, beadID string) (StaleBeadRecord, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) > 0 && trimmed[0] == '[' {
		var records []StaleBeadRecord
		if err := json.Unmarshal(raw, &records); err != nil {
			return StaleBeadRecord{}, fmt.Errorf("parse br show array: %w", err)
		}
		if len(records) == 0 {
			return StaleBeadRecord{}, fmt.Errorf("br show %s returned empty array", beadID)
		}
		return records[0], nil
	}
	var record StaleBeadRecord
	if err := json.Unmarshal(raw, &record); err != nil {
		return StaleBeadRecord{}, fmt.Errorf("parse br show object: %w", err)
	}
	return record, nil
}

func BuildTransferredEvent(beadID, agent string, prior, posterior StaleBeadRecord, now time.Time) TransferredEvent {
	priorAgent := prior.Assignee
	if priorAgent == "" {
		priorAgent = "unknown"
	}
	return TransferredEvent{
		StaleEvent: StaleEvent{
			SchemaVersion:    1,
			EventType:        "claim_transferred",
			BeadID:           beadID,
			DetectedAt:       now.UTC().Format(time.RFC3339),
			OriginalClaimant: StaleAgent{ID: priorAgent},
			Evidence: StaleEvidence{
				LastTouchTS:       prior.UpdatedAt,
				LastEvidenceEvent: "br update --claim",
			},
		},
		NewClaimant: StaleAgent{ID: agent},
		Transfer: TransferInfo{
			PriorRevision: Fingerprint(prior),
			NewRevision:   Fingerprint(posterior),
			NotesAppended: false,
		},
	}
}

func Fingerprint(record StaleBeadRecord) string {
	if record.Assignee == "" && record.UpdatedAt == "" {
		return "unset"
	}
	assignee := record.Assignee
	if assignee == "" {
		assignee = "_"
	}
	updatedAt := record.UpdatedAt
	if updatedAt == "" {
		updatedAt = "_"
	}
	return assignee + "@" + updatedAt
}
