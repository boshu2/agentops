package daemon

import (
	"errors"
	"fmt"
)

// ErrInvalidJobPayload marks a submitted job payload that does not match the
// structural shape of its JobType. It is wrapped by validateJobPayload so the
// HTTP submit path can classify the failure as a 4xx (client error) rather than
// a 5xx, and so callers can branch with errors.Is.
var ErrInvalidJobPayload = errors.New("daemon queue: invalid job payload")

// validateJobPayload performs a structural shape check on a submitted job's
// payload BEFORE it is appended to the ledger. It is the submit-time companion
// to the execute-time *JobSpecFromPayload validators: the executors already run
// the full parse (mapToStruct + semantic Validate) at claim time, but by then a
// malformed payload has already been durably written to the ledger, polluting
// it. On replay the projection builder silently degrades unparseable events, so
// the contamination is otherwise invisible until execution.
//
// This guard runs only the STRUCTURAL parse (mapToStruct), not the full
// semantic Validate(). That is deliberate: minimal-but-valid direct submissions
// historically omit fields the schema later defaults (no schema_version, no
// run_id, empty payload), and rejecting those would change long-standing valid
// behavior. The structural parse rejects exactly the contaminating case — a
// payload whose fields carry the wrong JSON type for the JobType's schema
// (e.g. a string where an int is expected, an object where a string is
// expected) — which is precisely what makes an event unparseable on replay.
//
// JobTypes without a typed payload schema (e.g. eval/llmwiki/openclaw lanes)
// pass through unchanged: there is nothing to structurally reject.
func validateJobPayload(jobType JobType, payload map[string]any) error {
	if err := structurallyParseJobPayload(jobType, payload); err != nil {
		return fmt.Errorf("%w: %s payload: %v", ErrInvalidJobPayload, jobType, err)
	}
	return nil
}

// structurallyParseJobPayload dispatches on jobType to the matching spec
// struct's structural unmarshal. It mirrors the JobType→spec mapping used by
// validateRecurringMaterializedPayload and the executors, but stops at the
// mapToStruct boundary so it never rejects a structurally-sound payload that
// merely lacks defaulted fields.
func structurallyParseJobPayload(jobType JobType, payload map[string]any) error {
	if len(payload) == 0 {
		return nil
	}
	switch jobType {
	case JobTypeRPIRun:
		var spec RPIRunJobSpec
		return mapToStruct(payload, &spec)
	case JobTypeRPIPhase:
		var spec RPIPhaseJobSpec
		return mapToStruct(payload, &spec)
	case JobTypeDreamRun:
		var spec DreamRunJobSpec
		return mapToStruct(payload, &spec)
	case JobTypeDreamStage:
		var spec DreamStageJobSpec
		return mapToStruct(payload, &spec)
	case JobTypeWikiForge:
		var spec WikiForgeJobSpec
		return mapToStruct(payload, &spec)
	case JobTypeFactoryAdmission:
		var spec FactoryAdmissionJobSpec
		return mapToStruct(payload, &spec)
	case JobTypeFactoryLocalPilot:
		var spec FactoryLocalPilotJobSpec
		return mapToStruct(payload, &spec)
	case JobTypePlansProjection:
		var spec PlansProjectionJobSpec
		return mapToStruct(payload, &spec)
	case JobTypeSkillInvoke:
		// SkillInvoke parses args out-of-band (string or list), so structural
		// validation must mirror that or it would reject valid string-args.
		raw := make(map[string]any, len(payload))
		for k, v := range payload {
			raw[k] = v
		}
		if _, err := parseSkillInvokeArgs(raw["args"]); err != nil {
			return err
		}
		delete(raw, "args")
		var spec SkillInvokeJobSpec
		return mapToStruct(raw, &spec)
	default:
		// JobTypes with no typed payload schema: nothing to structurally reject.
		return nil
	}
}
