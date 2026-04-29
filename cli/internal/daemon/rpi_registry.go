package daemon

import (
	"fmt"

	cliRPI "github.com/boshu2/agentops/cli/internal/rpi"
)

type DaemonRPIRegistryProjection struct {
	States      []cliRPI.RunRegistryState `json:"states"`
	LastEventID string                    `json:"last_event_id,omitempty"`
}

func (s *Store) RebuildRPIRegistryProjection() (DaemonRPIRegistryProjection, error) {
	replay, err := s.ReplayLedger()
	if err != nil {
		return DaemonRPIRegistryProjection{}, err
	}
	return RebuildRPIRegistryProjection(replay.Events)
}

func RebuildRPIRegistryProjection(events []LedgerEvent) (DaemonRPIRegistryProjection, error) {
	projection := DaemonRPIRegistryProjection{}
	statesByRun := map[string]*cliRPI.RunRegistryState{}
	jobToRun := map[string]string{}
	var order []string

	for _, event := range events {
		if err := ValidateLedgerEvent(event); err != nil {
			return DaemonRPIRegistryProjection{}, err
		}
		projection.LastEventID = event.EventID
		if event.EventType == EventProjectionMarkedStale || event.EventType == EventProjectionRebuilt {
			continue
		}

		runID := jobToRun[event.JobID]
		if event.EventType == EventJobAccepted {
			state, err := rpiRegistryStateFromAcceptedEvent(event)
			if err != nil {
				return DaemonRPIRegistryProjection{}, err
			}
			if state.RunID != "" {
				existing := statesByRun[state.RunID]
				if existing == nil {
					statesByRun[state.RunID] = &state
					order = append(order, state.RunID)
				} else {
					mergeRPIRegistryState(existing, state)
				}
				jobToRun[event.JobID] = state.RunID
				runID = state.RunID
			}
		}
		if runID == "" {
			continue
		}
		state := statesByRun[runID]
		if state == nil {
			continue
		}
		state.LastEventID = event.EventID
		state.DaemonRequestID = event.RequestID
		applyRPIRegistryLifecycleEvent(state, event)
	}

	for _, runID := range order {
		state := *statesByRun[runID]
		if state.Verdicts == nil {
			state.Verdicts = map[string]string{}
		}
		if state.Attempts == nil {
			state.Attempts = map[string]int{}
		}
		projection.States = append(projection.States, state)
	}
	return projection, nil
}

func WriteRPIRegistryProjection(root string, projection DaemonRPIRegistryProjection, writer cliRPI.RunRegistryWriter) error {
	if writer == nil {
		writer = cliRPI.FileRunRegistryWriter{}
	}
	for _, state := range projection.States {
		if err := writer.WriteRunRegistryState(root, state); err != nil {
			return err
		}
	}
	return nil
}

func rpiRegistryStateFromAcceptedEvent(event LedgerEvent) (cliRPI.RunRegistryState, error) {
	payload := nestedPayload(event.Payload, "job_payload")
	if payload == nil {
		payload = event.Payload
	}
	if jobType, ok, err := jobTypeFromPayload(payload); err != nil {
		return cliRPI.RunRegistryState{}, err
	} else if ok {
		switch jobType {
		case JobTypeRPIRun:
			spec, err := RPIRunJobSpecFromPayload(payload)
			if err != nil {
				return cliRPI.RunRegistryState{}, err
			}
			return registryStateFromRPIRunSpec(event, spec), nil
		case JobTypeRPIPhase:
			spec, err := RPIPhaseJobSpecFromPayload(payload)
			if err != nil {
				return cliRPI.RunRegistryState{}, err
			}
			return registryStateFromRPIPhaseSpec(event, spec), nil
		}
	}
	return cliRPI.RunRegistryState{}, nil
}

func registryStateFromRPIRunSpec(event LedgerEvent, spec RPIRunJobSpec) cliRPI.RunRegistryState {
	phaseName := RPIPhaseName(spec.StartPhase)
	return cliRPI.RunRegistryState{
		SchemaVersion:   cliRPI.RunRegistrySchemaVersion,
		Goal:            spec.Goal,
		EpicID:          spec.EpicID,
		Phase:           spec.StartPhase,
		StartPhase:      spec.StartPhase,
		Cycle:           1,
		TestFirst:       spec.TestFirst,
		Complexity:      spec.Complexity,
		Verdicts:        map[string]string{},
		Attempts:        map[string]int{phaseName: 0},
		StartedAt:       event.OccurredAt,
		RunID:           spec.RunID,
		Backend:         string(spec.Backend),
		DaemonJobID:     event.JobID,
		DaemonRequestID: event.RequestID,
		LastEventID:     event.EventID,
	}
}

func registryStateFromRPIPhaseSpec(event LedgerEvent, spec RPIPhaseJobSpec) cliRPI.RunRegistryState {
	return cliRPI.RunRegistryState{
		SchemaVersion:   cliRPI.RunRegistrySchemaVersion,
		Goal:            spec.Goal,
		EpicID:          spec.EpicID,
		Phase:           spec.Phase,
		StartPhase:      spec.Phase,
		Cycle:           1,
		TestFirst:       true,
		Verdicts:        map[string]string{},
		Attempts:        map[string]int{spec.PhaseName: spec.Attempt},
		StartedAt:       event.OccurredAt,
		RunID:           spec.RunID,
		Backend:         string(spec.Backend),
		DaemonJobID:     event.JobID,
		DaemonRequestID: event.RequestID,
		LastEventID:     event.EventID,
	}
}

func mergeRPIRegistryState(existing *cliRPI.RunRegistryState, next cliRPI.RunRegistryState) {
	if next.Phase > existing.Phase {
		existing.Phase = next.Phase
	}
	if existing.Goal == "" {
		existing.Goal = next.Goal
	}
	if existing.EpicID == "" {
		existing.EpicID = next.EpicID
	}
	if next.Backend != "" {
		existing.Backend = next.Backend
	}
	if next.DaemonJobID != "" {
		existing.DaemonJobID = next.DaemonJobID
	}
	if existing.Attempts == nil {
		existing.Attempts = map[string]int{}
	}
	for key, value := range next.Attempts {
		existing.Attempts[key] = value
	}
}

func applyRPIRegistryLifecycleEvent(state *cliRPI.RunRegistryState, event LedgerEvent) {
	switch event.EventType {
	case EventJobClaimed, EventJobHeartbeat:
		state.TerminalStatus = ""
		state.TerminalReason = ""
	case EventJobCompleted:
		state.TerminalStatus = "completed"
		state.TerminatedAt = event.OccurredAt
	case EventJobFailed:
		state.TerminalStatus = "failed"
		state.TerminatedAt = event.OccurredAt
		failure := failureFromPayload(event.Payload)
		if failure.Message != "" {
			state.TerminalReason = failure.Message
		} else {
			state.TerminalReason = string(failure.Code)
		}
	case EventJobCancelled:
		state.TerminalStatus = "cancelled"
		state.TerminatedAt = event.OccurredAt
	}
}

func nestedPayload(payload map[string]any, key string) map[string]any {
	raw, ok := payload[key]
	if !ok {
		return nil
	}
	switch value := raw.(type) {
	case map[string]any:
		return value
	case map[string]string:
		out := make(map[string]any, len(value))
		for key, val := range value {
			out[key] = val
		}
		return out
	default:
		return nil
	}
}

func ValidateRPIRegistryProjection(projection DaemonRPIRegistryProjection) error {
	for _, state := range projection.States {
		if err := state.Validate(); err != nil {
			return fmt.Errorf("run %q: %w", state.RunID, err)
		}
	}
	return nil
}
