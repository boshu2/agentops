package ports

import (
	"context"
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"
)

// FindingObservation is one evidence-backed defect observation produced by a
// validation run. ObjectiveID names the unit of intent being validated. Several
// review attempts inside one objective are retries, not independent recurrence.
type FindingObservation struct {
	ID          string `json:"id"`
	ClassKey    string `json:"class_key"`
	ObjectiveID string `json:"objective_id"`
	EvidenceRef string `json:"evidence_ref"`
	Summary     string `json:"summary,omitempty"`
}

// ProducerEvidenceRef preserves one representative observation for each
// distinct objective supporting a producer-rule candidate.
type ProducerEvidenceRef struct {
	ObservationID string `json:"observation_id"`
	ObjectiveID   string `json:"objective_id"`
	EvidenceRef   string `json:"evidence_ref"`
}

// ProducerRuleCandidate is an advisory proposal to change a producer surface
// (for example Discovery, Plan, or Premortem). It is not a blocker or mechanical
// constraint. RecurrenceCount always counts distinct objectives.
type ProducerRuleCandidate struct {
	ID              string                `json:"id"`
	ClassKey        string                `json:"class_key"`
	Summary         string                `json:"summary,omitempty"`
	RecurrenceCount int                   `json:"recurrence_count"`
	Advisory        bool                  `json:"advisory"`
	Evidence        []ProducerEvidenceRef `json:"evidence"`
}

// FindingRecurrenceReducerPort reconciles validation observations into
// advisory producer-rule candidates. A class becomes recurrent only after it
// appears in at least two distinct objectives.
type FindingRecurrenceReducerPort interface {
	Reduce(context.Context, []FindingObservation) ([]ProducerRuleCandidate, error)
}

// InMemoryFindingRecurrenceReducer is the zero-I/O adapter used by tests and
// callers that already hold observations in memory.
type InMemoryFindingRecurrenceReducer struct{}

func NewInMemoryFindingRecurrenceReducer() *InMemoryFindingRecurrenceReducer {
	return &InMemoryFindingRecurrenceReducer{}
}

func (r *InMemoryFindingRecurrenceReducer) Reduce(ctx context.Context, observations []FindingObservation) ([]ProducerRuleCandidate, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return ReduceFindingRecurrence(observations)
}

// ReduceFindingRecurrence is the deterministic domain reduction shared by port
// adapters. It retains the first observation per (class, objective), so retries
// cannot inflate recurrence or evidence weight.
func ReduceFindingRecurrence(observations []FindingObservation) ([]ProducerRuleCandidate, error) {
	type classGroup struct {
		summary     string
		byObjective map[string]FindingObservation
	}
	groups := make(map[string]*classGroup)
	for i, observation := range observations {
		observation.ID = strings.TrimSpace(observation.ID)
		observation.ClassKey = strings.TrimSpace(observation.ClassKey)
		observation.ObjectiveID = strings.TrimSpace(observation.ObjectiveID)
		observation.EvidenceRef = strings.TrimSpace(observation.EvidenceRef)
		observation.Summary = strings.TrimSpace(observation.Summary)
		if observation.ID == "" || observation.ClassKey == "" || observation.ObjectiveID == "" || observation.EvidenceRef == "" {
			return nil, fmt.Errorf("ports: finding observation %d requires id, class_key, objective_id, and evidence_ref", i)
		}
		group := groups[observation.ClassKey]
		if group == nil {
			group = &classGroup{byObjective: make(map[string]FindingObservation)}
			groups[observation.ClassKey] = group
		}
		if group.summary == "" && observation.Summary != "" {
			group.summary = observation.Summary
		}
		if _, exists := group.byObjective[observation.ObjectiveID]; !exists {
			group.byObjective[observation.ObjectiveID] = observation
		}
	}

	classKeys := make([]string, 0, len(groups))
	for classKey, group := range groups {
		if len(group.byObjective) >= 2 {
			classKeys = append(classKeys, classKey)
		}
	}
	sort.Strings(classKeys)

	candidates := make([]ProducerRuleCandidate, 0, len(classKeys))
	for _, classKey := range classKeys {
		group := groups[classKey]
		objectiveIDs := make([]string, 0, len(group.byObjective))
		for objectiveID := range group.byObjective {
			objectiveIDs = append(objectiveIDs, objectiveID)
		}
		sort.Strings(objectiveIDs)
		evidence := make([]ProducerEvidenceRef, 0, len(objectiveIDs))
		for _, objectiveID := range objectiveIDs {
			observation := group.byObjective[objectiveID]
			evidence = append(evidence, ProducerEvidenceRef{
				ObservationID: observation.ID,
				ObjectiveID:   objectiveID,
				EvidenceRef:   observation.EvidenceRef,
			})
		}
		digest := sha256.Sum256([]byte(classKey))
		candidates = append(candidates, ProducerRuleCandidate{
			ID:              fmt.Sprintf("producer-%x", digest[:6]),
			ClassKey:        classKey,
			Summary:         group.summary,
			RecurrenceCount: len(objectiveIDs),
			Advisory:        true,
			Evidence:        evidence,
		})
	}
	if candidates == nil {
		return []ProducerRuleCandidate{}, nil
	}
	return candidates, nil
}

var _ FindingRecurrenceReducerPort = (*InMemoryFindingRecurrenceReducer)(nil)
