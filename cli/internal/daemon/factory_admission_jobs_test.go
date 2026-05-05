package daemon

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestFactoryAdmissionJobSpecRoundTrip(t *testing.T) {
	spec := NewFactoryAdmissionJobSpec("factory-run-1", validFactoryWorkOrder())
	job, err := spec.ToJobSpec("job-factory-admission")
	if err != nil {
		t.Fatalf("ToJobSpec: %v", err)
	}
	if job.Type != JobTypeFactoryAdmission {
		t.Fatalf("job type = %q, want %q", job.Type, JobTypeFactoryAdmission)
	}
	got, err := FactoryAdmissionJobSpecFromPayload(job.Payload)
	if err != nil {
		t.Fatalf("FactoryAdmissionJobSpecFromPayload: %v", err)
	}
	if got.RunID != spec.RunID || got.WorkOrder.WorkOrderID != spec.WorkOrder.WorkOrderID {
		t.Fatalf("roundtrip mismatch: got %#v want %#v", got, spec)
	}
}

func TestFactoryLocalPilotJobSpecRoundTrip(t *testing.T) {
	spec := NewFactoryLocalPilotJobSpec("factory-pilot-1", validFactoryWorkOrder())
	spec.Mode = FactoryAdmissionModeRPIHandoff
	spec.Handoff = FactoryHandoff{
		Kind:                FactoryHandoffRPI,
		ExecutionPacketPath: ".agents/rpi/execution-packet.json",
		EpicID:              "soc-ff7b.7",
	}
	job, err := spec.ToJobSpec("job-factory-pilot")
	if err != nil {
		t.Fatalf("ToJobSpec: %v", err)
	}
	got, err := FactoryLocalPilotJobSpecFromPayload(job.Payload)
	if err != nil {
		t.Fatalf("FactoryLocalPilotJobSpecFromPayload: %v", err)
	}
	if got.Handoff.Kind != FactoryHandoffRPI || got.Handoff.ExecutionPacketPath == "" {
		t.Fatalf("handoff did not roundtrip: %#v", got.Handoff)
	}
}

func TestFactoryWorkOrderValidationRejectsUnsafeInputs(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*FactoryWorkOrder)
		want   string
	}{
		{
			name: "expired before generated",
			mutate: func(work *FactoryWorkOrder) {
				work.ExpiresAt = "2026-05-04T23:00:00Z"
			},
			want: "expires_at",
		},
		{
			name: "missing target",
			mutate: func(work *FactoryWorkOrder) {
				work.Target = FactoryTarget{}
			},
			want: "target",
		},
		{
			name: "absolute allowed file",
			mutate: func(work *FactoryWorkOrder) {
				work.AllowedFiles = []string{"/tmp/outside"}
			},
			want: "must be relative",
		},
		{
			name: "parent escape allowed file",
			mutate: func(work *FactoryWorkOrder) {
				work.AllowedFiles = []string{"../outside"}
			},
			want: "parent-directory",
		},
		{
			name: "interior parent segment allowed file",
			mutate: func(work *FactoryWorkOrder) {
				work.AllowedFiles = []string{"cli/../outside"}
			},
			want: "parent-directory",
		},
		{
			name: "unsafe landing policy",
			mutate: func(work *FactoryWorkOrder) {
				work.LandingPolicy = "auto_merge"
			},
			want: "landing policy",
		},
		{
			name: "missing blocker matrix",
			mutate: func(work *FactoryWorkOrder) {
				work.OpenPRBlockers = nil
			},
			want: "open_pr_blockers",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			work := validFactoryWorkOrder()
			tc.mutate(&work)
			err := work.Validate()
			if err == nil {
				t.Fatal("Validate accepted invalid work order")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want substring %q", err, tc.want)
			}
		})
	}
}

func TestFactoryAdmissionDecisionRequiresBlockedReasons(t *testing.T) {
	decision := FactoryAdmissionDecision{
		SchemaVersion: FactoryAdmissionJobSpecSchemaVersion,
		WorkOrderID:   "factory-work-1",
		RunID:         "factory-run-1",
		EvaluatedAt:   "2026-05-04T23:35:00Z",
		Allowed:       false,
		LandingPolicy: FactoryLandingPolicyManualPR,
		DigestPolicy:  FactoryDigestPolicyRequired,
		Evidence: FactoryDecisionEvidence{
			BaseSHA:            "abcdef1",
			OpenPRBlockerCount: 0,
			MainCIStatus:       FactoryCIStatusGreen,
		},
	}
	if err := decision.Validate(); err == nil {
		t.Fatal("Validate accepted blocked decision without reasons")
	}
	decision.Reasons = []string{"open_pr_overlap"}
	if err := decision.Validate(); err != nil {
		t.Fatalf("Validate with reason: %v", err)
	}
}

func TestRecurrence_MaterializesFactoryLocalPilotPayload(t *testing.T) {
	payloadBytes, err := json.Marshal(map[string]any{
		"work_order": validFactoryWorkOrder(),
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	tmpl := RecurringJobTemplate{
		Name:    "factory-pilot",
		Cron:    "0 3 * * *",
		JobType: JobTypeFactoryLocalPilot,
		Payload: payloadBytes,
	}
	payload, err := materializeRecurringJobPayload(tmpl, "sub-123", fixedTickAt)
	if err != nil {
		t.Fatalf("materialize payload: %v", err)
	}
	spec, err := FactoryLocalPilotJobSpecFromPayload(payload)
	if err != nil {
		t.Fatalf("FactoryLocalPilotJobSpecFromPayload: %v", err)
	}
	if spec.RunID == "" || spec.Mode != FactoryAdmissionModeAdmissionOnly {
		t.Fatalf("materialized factory local pilot spec = %#v", spec)
	}
}

func validFactoryWorkOrder() FactoryWorkOrder {
	return FactoryWorkOrder{
		SchemaVersion: FactoryAdmissionJobSpecSchemaVersion,
		WorkOrderID:   "factory-work-1",
		GeneratedAt:   "2026-05-04T23:30:00Z",
		ExpiresAt:     "2026-05-05T00:30:00Z",
		BaseSHA:       "abcdef123456",
		Target: FactoryTarget{
			Type:    FactoryTargetBead,
			ID:      "soc-ff7b.7.1",
			Summary: "Define factory admission contract",
		},
		AllowedFiles: []string{
			"docs/contracts/factory-admission.md",
			"schemas/factory-work-order.v1.schema.json",
		},
		ValidationCommands: []string{
			"python3 tests/scripts/test-factory-admission-contracts.py",
			"cd cli && go test ./internal/daemon -run FactoryAdmission",
		},
		LandingPolicy:         FactoryLandingPolicyManualPR,
		DigestPolicy:          FactoryDigestPolicyRequired,
		OpenPRBlockers:        []FactoryOpenPRBlocker{},
		UnknownEvidencePolicy: FactoryUnknownEvidenceBlock,
		MainCIBaseline: FactoryMainCIBaseline{
			Status:     FactoryCIStatusGreen,
			RunID:      "123",
			CheckedAt:  "2026-05-04T23:29:00Z",
			FailedJobs: []string{},
		},
	}
}
