package orchestration

import (
	"context"
	"errors"
	"testing"

	"github.com/boshu2/agentops/cli/internal/ports"
)

// TestBeadsFloorAdapter_Run_EmitsParityConformantResult asserts the floor
// adapter emits a result that conforms to the output-contract parity shape:
// Backend == beads, SchemaVersion == 1, non-empty ResultPaths, a valid
// verdict, the requested TaskID, and that Validate accepts it.
func TestBeadsFloorAdapter_Run_EmitsParityConformantResult(t *testing.T) {
	var adapter BeadsFloorAdapter
	const taskID = "soc-floor-1"

	result, err := adapter.Run(context.Background(), taskID)
	if err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	if result.Backend != ports.BackendBeads {
		t.Errorf("Backend: want %q, got %q", ports.BackendBeads, result.Backend)
	}
	if result.SchemaVersion != SchemaVersionV1 {
		t.Errorf("SchemaVersion: want %d, got %d", SchemaVersionV1, result.SchemaVersion)
	}
	if len(result.ResultPaths) == 0 {
		t.Error("ResultPaths: want non-empty, got empty")
	}
	for i, p := range result.ResultPaths {
		if p == "" {
			t.Errorf("ResultPaths[%d]: want non-empty path, got empty string", i)
		}
	}
	if result.TaskID != taskID {
		t.Errorf("TaskID: want %q, got %q", taskID, result.TaskID)
	}
	if !validVerdictStatuses[result.Verdict.Status] {
		t.Errorf("Verdict.Status: %q is not a valid status", result.Verdict.Status)
	}
	if !validVerdictConfidences[result.Verdict.Confidence] {
		t.Errorf("Verdict.Confidence: %q is not a valid confidence", result.Verdict.Confidence)
	}
	if err := result.Validate(); err != nil {
		t.Errorf("Validate: want nil, got %v", err)
	}
}

// TestBeadsFloorAdapter_Run_HonorsCancelledContext asserts the floor does
// not fabricate a result when the caller's context is already cancelled.
func TestBeadsFloorAdapter_Run_HonorsCancelledContext(t *testing.T) {
	var adapter BeadsFloorAdapter
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := adapter.Run(ctx, "soc-floor-2")
	if err == nil {
		t.Fatal("Run: want error for cancelled context, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Run: want error wrapping context.Canceled, got %v", err)
	}
}

// TestBeadsFloorResult_Validate_RejectsBadVerdict asserts the parity
// validator rejects results whose verdict fields fall outside the schema
// enums — the negative half of the parity contract.
func TestBeadsFloorResult_Validate_RejectsBadVerdict(t *testing.T) {
	tests := []struct {
		name   string
		result OrchestrationResult
	}{
		{
			name: "bad status",
			result: OrchestrationResult{
				SchemaVersion: SchemaVersionV1,
				Backend:       ports.BackendBeads,
				ResultPaths:   []string{beadsFloorPlaceholderPath},
				Verdict:       Verdict{Status: "MAYBE", Confidence: VerdictConfidenceHigh},
			},
		},
		{
			name: "bad confidence",
			result: OrchestrationResult{
				SchemaVersion: SchemaVersionV1,
				Backend:       ports.BackendBeads,
				ResultPaths:   []string{beadsFloorPlaceholderPath},
				Verdict:       Verdict{Status: VerdictStatusPass, Confidence: "PRETTY_SURE"},
			},
		},
		{
			name: "wrong schema version",
			result: OrchestrationResult{
				SchemaVersion: 99,
				Backend:       ports.BackendBeads,
				ResultPaths:   []string{beadsFloorPlaceholderPath},
				Verdict:       Verdict{Status: VerdictStatusPass, Confidence: VerdictConfidenceHigh},
			},
		},
		{
			name: "empty result paths",
			result: OrchestrationResult{
				SchemaVersion: SchemaVersionV1,
				Backend:       ports.BackendBeads,
				ResultPaths:   nil,
				Verdict:       Verdict{Status: VerdictStatusPass, Confidence: VerdictConfidenceHigh},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.result.Validate(); err == nil {
				t.Errorf("Validate: want error for %s, got nil", tt.name)
			}
		})
	}
}
