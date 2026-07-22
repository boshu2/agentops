package delivery

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"
)

// DeliveryStatus is a caller-time-attested, read-only inventory. It is an
// observation boundary, never a scheduler or a delivery lifecycle authority.
type DeliveryStatus struct {
	SchemaVersion string        `json:"schema_version"`
	ObservedAt    string        `json:"observed_at"`
	WIP           []DeliveryWIP `json:"wip"`
}

// DeliveryWIP is the sole currently actionable (or blocked) epoch observed
// for one handoff. A pending successor leaves its published predecessor as the
// WIP item until that successor has become a real published epoch.
type DeliveryWIP struct {
	HandoffID      string `json:"handoff_id"`
	DeliveryBeadID string `json:"delivery_bead_id"`
	BeadStatus     string `json:"bead_status"`
	State          string `json:"state"`
	Publication    string `json:"publication"`
	Route          string `json:"route"`
	ReadyAt        string `json:"ready_at"`
	AgeSeconds     int64  `json:"age_seconds"`
}

// DeliveryStatus scans every nonclosed delivery bead. The fixed scan cap has
// an explicit equality fence: a potentially truncated response is an error,
// never a partial status report.
func (p *NativeProviders) DeliveryStatus(ctx context.Context, observedAt time.Time) (DeliveryStatus, error) {
	const scanLimit = 128
	records, err := p.bd(ctx, "list", "--all", "--metadata-field", "gc.kind=delivery", "--exclude-type=epic", "--json", "--sort", "created", "--limit", "128")
	if err != nil {
		return DeliveryStatus{}, err
	}
	if len(records) == scanLimit {
		return DeliveryStatus{}, errors.New("delivery status scan is truncated")
	}
	return deliveryStatusFromRecords(records, observedAt)
}

func deliveryStatusFromRecords(records []bdRecord, observedAt time.Time) (DeliveryStatus, error) {
	if observedAt.IsZero() {
		return DeliveryStatus{}, errors.New("delivery status requires a caller-attested observation time")
	}
	byHandoff := map[string][]readyDeliveryCandidate{}
	for _, record := range records {
		if record.Status == "closed" {
			continue
		}
		if !openDeliveryBeadStatus(record.Status) {
			return DeliveryStatus{}, fmt.Errorf("delivery status scan returned non-open bead %s with status %q", record.ID, record.Status)
		}
		candidate, err := readyDeliveryCandidateFromRecord(record)
		if err != nil {
			return DeliveryStatus{}, err
		}
		byHandoff[candidate.bead.Record.HandoffID] = append(byHandoff[candidate.bead.Record.HandoffID], candidate)
	}
	items := make([]DeliveryWIP, 0, len(byHandoff))
	for handoffID, chain := range byHandoff {
		selected, readyAt, err := selectStatusDelivery(chain)
		if err != nil {
			return DeliveryStatus{}, err
		}
		parsedReadyAt, err := time.Parse(time.RFC3339, readyAt)
		if err != nil || observedAt.Before(parsedReadyAt) {
			return DeliveryStatus{}, errors.New("delivery status observation predates ready_at")
		}
		items = append(items, DeliveryWIP{HandoffID: handoffID, DeliveryBeadID: selected.record.ID, BeadStatus: selected.record.Status, State: string(selected.bead.Record.State), Publication: selected.bead.Record.Publication, Route: selected.bead.Route, ReadyAt: readyAt, AgeSeconds: int64(observedAt.Sub(parsedReadyAt).Seconds())})
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].ReadyAt != items[j].ReadyAt {
			return items[i].ReadyAt < items[j].ReadyAt
		}
		return items[i].HandoffID < items[j].HandoffID
	})
	return DeliveryStatus{SchemaVersion: "gc.delivery-status.v1", ObservedAt: observedAt.UTC().Format(time.RFC3339), WIP: items}, nil
}

func openDeliveryBeadStatus(status string) bool {
	switch status {
	case "open", "in_progress", "blocked", "deferred":
		return true
	default:
		return false
	}
}

func selectStatusDelivery(chain []readyDeliveryCandidate) (readyDeliveryCandidate, string, error) {
	sort.Slice(chain, func(i, j int) bool { return chain[i].bead.Record.Epoch.Number < chain[j].bead.Record.Epoch.Number })
	if err := validReadyDeliveryChain(chain); err != nil {
		return readyDeliveryCandidate{}, "", err
	}
	minimum := ""
	for _, candidate := range chain {
		readyAt := candidate.bead.Record.ReadyAt
		if _, err := time.Parse(time.RFC3339, readyAt); err != nil {
			return readyDeliveryCandidate{}, "", errors.New("delivery status record has invalid schema ready_at")
		}
		if minimum == "" || readyAt < minimum {
			minimum = readyAt
		}
	}
	leaf := chain[len(chain)-1]
	if leaf.bead.Record.EpochSuccessorID != "" && !recoverableAbsentSuccessor(leaf.bead) {
		return readyDeliveryCandidate{}, "", errors.New("delivery status handoff leaf references an absent successor")
	}
	if leaf.bead.Record.Publication == "pending" && len(chain) > 1 {
		leaf = chain[len(chain)-2]
	}
	return leaf, minimum, nil
}
