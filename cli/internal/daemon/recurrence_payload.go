package daemon

import (
	"fmt"
	"strings"
	"time"

	"github.com/boshu2/agentops/cli/internal/agentworker"
)

var recurringValidationTick = time.Unix(0, 0).UTC()

// ValidateRecurringJobTemplatePayload applies the same defaults the recurrence
// supervisor applies at fire time, then validates the materialized job payload.
func ValidateRecurringJobTemplatePayload(t RecurringJobTemplate) error {
	_, err := materializeRecurringJobPayload(t, "validation-submission", recurringValidationTick)
	return err
}

func materializeRecurringJobPayload(t RecurringJobTemplate, subID string, tickAt time.Time) (map[string]any, error) {
	if err := ValidateJobType(t.JobType); err != nil {
		return nil, err
	}
	payload, err := decodePayload(t.Payload)
	if err != nil {
		return nil, fmt.Errorf("decode payload: %w", err)
	}
	applyRecurringJobDefaults(t, payload, subID)
	payload["schedule_name"] = t.Name
	payload["submission_id"] = subID
	payload["tick_at"] = tickAt.UTC().Format(time.RFC3339Nano)
	if t.Timeout > 0 {
		payload["timeout"] = t.Timeout.String()
	}
	if err := validateRecurringMaterializedPayload(t.JobType, payload); err != nil {
		return nil, fmt.Errorf("%s payload: %w", t.JobType, err)
	}
	return payload, nil
}

func applyRecurringJobDefaults(t RecurringJobTemplate, payload map[string]any, subID string) {
	setDefault(payload, "job_type", string(t.JobType))
	switch t.JobType {
	case JobTypeRPIRun:
		setDefault(payload, "schema_version", RPIJobSpecSchemaVersion)
		setDefault(payload, "run_id", recurringSyntheticID("rpi", t.Name, subID))
		setDefault(payload, "goal", recurringDefaultGoal(t.Name))
		setDefault(payload, "start_phase", 1)
		setDefault(payload, "max_phase", 3)
		setDefault(payload, "test_first", true)
		setDefault(payload, "backend", string(RPIBackendGasCityAPI))
	case JobTypeRPIPhase:
		setDefault(payload, "schema_version", RPIJobSpecSchemaVersion)
		setDefault(payload, "run_id", recurringSyntheticID("rpi", t.Name, subID))
		setDefault(payload, "goal", recurringDefaultGoal(t.Name))
		setDefault(payload, "phase", 1)
		setDefault(payload, "phase_name", RPIPhaseName(payloadIntDefault(payload, "phase", 1)))
		setDefault(payload, "backend", string(RPIBackendGasCityAPI))
	case JobTypeDreamRun:
		setDefault(payload, "schema_version", DreamJobSpecSchemaVersion)
		setDefault(payload, "dream_run_id", recurringSyntheticID("dream", t.Name, subID))
		setDefault(payload, "mode", string(DreamModeDaemon))
	case JobTypeDreamStage:
		setDefault(payload, "schema_version", DreamJobSpecSchemaVersion)
		setDefault(payload, "dream_run_id", recurringSyntheticID("dream", t.Name, subID))
		setDefault(payload, "stage", string(DreamStageIngest))
		setDefault(payload, "mode", string(DreamModeDaemon))
	case JobTypeWikiForge:
		setDefault(payload, "schema_version", WikiJobSpecSchemaVersion)
		setDefault(payload, "output_dir", ".agents/wiki/forge/"+sanitizeIDPart(t.Name))
		setDefault(payload, "worker_kind", string(defaultWikiForgeWorkerKind))
		setDefault(payload, "provider", string(agentworker.ProviderGasCity))
		setDefault(payload, "max_attempts", 2)
	case JobTypeFactoryAdmission:
		setDefault(payload, "schema_version", FactoryAdmissionJobSpecSchemaVersion)
		setDefault(payload, "run_id", recurringSyntheticID("factory", t.Name, subID))
		setDefault(payload, "mode", string(FactoryAdmissionModeAdmissionOnly))
	case JobTypeFactoryLocalPilot:
		setDefault(payload, "schema_version", FactoryAdmissionJobSpecSchemaVersion)
		setDefault(payload, "run_id", recurringSyntheticID("factory", t.Name, subID))
		setDefault(payload, "mode", string(FactoryAdmissionModeAdmissionOnly))
	case JobTypePlansProjection:
		setDefault(payload, "schema_version", PlansProjectionJobSpecSchemaVersion)
		setDefault(payload, "refresh_trigger", string(PlansProjectionTriggerInterval))
	}
}

func validateRecurringMaterializedPayload(jobType JobType, payload map[string]any) error {
	switch jobType {
	case JobTypeRPIRun:
		_, err := RPIRunJobSpecFromPayload(payload)
		return err
	case JobTypeRPIPhase:
		_, err := RPIPhaseJobSpecFromPayload(payload)
		return err
	case JobTypeDreamRun:
		_, err := DreamRunJobSpecFromPayload(payload)
		return err
	case JobTypeDreamStage:
		_, err := DreamStageJobSpecFromPayload(payload)
		return err
	case JobTypeWikiForge:
		_, err := WikiForgeJobSpecFromPayload(payload)
		return err
	case JobTypeFactoryAdmission:
		_, err := FactoryAdmissionJobSpecFromPayload(payload)
		return err
	case JobTypeFactoryLocalPilot:
		_, err := FactoryLocalPilotJobSpecFromPayload(payload)
		return err
	case JobTypePlansProjection:
		_, err := PlansProjectionJobSpecFromPayload(payload)
		return err
	default:
		return nil
	}
}

func setDefault(payload map[string]any, key string, value any) {
	if existing, ok := payload[key]; ok {
		if text, isString := existing.(string); !isString || strings.TrimSpace(text) != "" {
			return
		}
	}
	payload[key] = value
}

func payloadIntDefault(payload map[string]any, key string, fallback int) int {
	if value, ok := intPayload(payload, key); ok {
		return value
	}
	return fallback
}

func recurringSyntheticID(prefix, scheduleName, subID string) string {
	name := sanitizeIDPart(scheduleName)
	if name == "" {
		name = "schedule"
	}
	return fmt.Sprintf("%s_%s_%s", prefix, name, sanitizeIDPart(subID))
}

func recurringDefaultGoal(scheduleName string) string {
	name := strings.TrimSpace(scheduleName)
	if name == "" {
		return "Scheduled daemon job"
	}
	return "Scheduled daemon job: " + name
}
