package daemon

import (
	"fmt"
	"strings"
	"time"
)

type RequestID string

type LedgerEventInput struct {
	EventID           string
	RequestID         RequestID
	JobID             string
	EventType         EventType
	OccurredAt        time.Time
	Actor             string
	JobType           JobType
	ProjectionTargets []ProjectionName
	Payload           map[string]any
}

func NewLedgerEvent(input LedgerEventInput) (LedgerEvent, error) {
	payload := clonePayload(input.Payload)
	if input.JobType != "" {
		if err := ValidateJobType(input.JobType); err != nil {
			return LedgerEvent{}, err
		}
		payload["job_type"] = string(input.JobType)
	}
	if len(input.ProjectionTargets) > 0 {
		payload["projection_targets"] = projectionTargetStrings(input.ProjectionTargets)
	}
	occurredAt := ""
	if !input.OccurredAt.IsZero() {
		occurredAt = input.OccurredAt.UTC().Format(time.RFC3339Nano)
	}
	return NormalizeLedgerEvent(LedgerEvent{
		SchemaVersion: LedgerSchemaVersion,
		EventID:       input.EventID,
		RequestID:     string(input.RequestID),
		JobID:         input.JobID,
		EventType:     input.EventType,
		OccurredAt:    occurredAt,
		Actor:         input.Actor,
		Payload:       payload,
	})
}

func NormalizeLedgerEvent(event LedgerEvent) (LedgerEvent, error) {
	if event.SchemaVersion == 0 {
		event.SchemaVersion = LedgerSchemaVersion
	}
	event.EventID = strings.TrimSpace(event.EventID)
	event.RequestID = strings.TrimSpace(event.RequestID)
	event.JobID = strings.TrimSpace(event.JobID)
	event.Actor = strings.TrimSpace(event.Actor)
	if strings.TrimSpace(event.OccurredAt) == "" {
		event.OccurredAt = time.Now().UTC().Format(time.RFC3339Nano)
	}
	if event.Payload == nil {
		event.Payload = map[string]any{}
	}
	if err := ValidateLedgerEvent(event); err != nil {
		return LedgerEvent{}, err
	}
	return event, nil
}

func ValidateRequestID(value string) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fmt.Errorf("request_id is required")
	}
	if strings.ContainsAny(trimmed, " \t\r\n") {
		return fmt.Errorf("request_id %q must not contain whitespace", value)
	}
	return nil
}

func clonePayload(payload map[string]any) map[string]any {
	if payload == nil {
		return map[string]any{}
	}
	clone := make(map[string]any, len(payload))
	for k, v := range payload {
		clone[k] = v
	}
	return clone
}
