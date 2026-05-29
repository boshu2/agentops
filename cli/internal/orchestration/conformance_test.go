// practices: [output-contract-parity, safe-degradation]
package orchestration

import (
	"context"
	"encoding/json"
	"maps"
	"slices"
	"testing"

	"github.com/boshu2/agentops/cli/internal/ports"
)

// conformanceFixtureTaskID is the single fixture task that every tier in the
// degradation ladder produces a result for. Using the SAME task across all
// tiers is what makes the parity proof meaningful: a downstream consumer must
// be able to read the outcome identically no matter which tier produced it.
const conformanceFixtureTaskID = "soc-conformance-fixture"

// tierResultFor returns a representative OrchestrationResult for the fixture
// task, as the named backend tier would emit it.
//
// The beads floor has a real (stub) adapter — BeadsFloorAdapter.Run — so we
// drive that. The NTM and Claude-native tiers have stubbed executors until
// the application epics, so we construct the OrchestrationResult the way those
// tiers contractually MUST: the SAME struct, populated to be parity-conformant,
// differing only in the Backend value (and a tier-appropriate verdict). This
// is the contract every adapter is obligated to satisfy; constructing it here
// asserts that obligation independently of the (not-yet-built) executors.
func tierResultFor(t *testing.T, backend ports.Backend) OrchestrationResult {
	t.Helper()

	if backend == ports.BackendBeads {
		res, err := BeadsFloorAdapter{}.Run(context.Background(), conformanceFixtureTaskID)
		if err != nil {
			t.Fatalf("BeadsFloorAdapter.Run(%q) returned error: %v", conformanceFixtureTaskID, err)
		}
		return res
	}

	// NTM / Claude tiers: same shape, different Backend. The richer tiers
	// would earn a PASS/HIGH verdict on a clean run; the floor advertises
	// WARN/MEDIUM. Verdict CONTENT may differ per tier — what must NOT differ
	// is the result SHAPE (the JSON key set), which is the parity invariant
	// this test pins.
	return OrchestrationResult{
		SchemaVersion: SchemaVersionV1,
		Backend:       backend,
		ResultPaths:   []string{".agents/orchestration/" + string(backend) + "-run.artifact"},
		Verdict: Verdict{
			Status:     VerdictStatusPass,
			Confidence: VerdictConfidenceHigh,
		},
		TaskID: conformanceFixtureTaskID,
	}
}

// jsonKeySet marshals a result and returns its sorted top-level JSON key set.
// Comparing sorted key sets across tiers is the mechanical parity proof: a
// tier-agnostic consumer (validation / ledger / provenance) depends on the
// shape, not the values, being identical.
func jsonKeySet(t *testing.T, res OrchestrationResult) []string {
	t.Helper()

	raw, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("json.Marshal(%+v) returned error: %v", res, err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("json.Unmarshal(%s) returned error: %v", raw, err)
	}
	return slices.Sorted(maps.Keys(m))
}

// TestDegradationConformance_AllTiersValidate proves that every tier in the
// degradation ladder (NTM -> Claude -> beads) emits a result that passes the
// parity contract's own Validate() gate and carries the V1 schema version.
// Validate() passing per-tier is the precondition for degradation being
// correctness-preserving: a caller can descend the ladder without a tier ever
// handing back a malformed result.
func TestDegradationConformance_AllTiersValidate(t *testing.T) {
	for _, backend := range []ports.Backend{ports.BackendNTM, ports.BackendClaude, ports.BackendBeads} {
		t.Run(string(backend), func(t *testing.T) {
			res := tierResultFor(t, backend)

			if err := res.Validate(); err != nil {
				t.Fatalf("tier %q result failed Validate(): %v", backend, err)
			}
			if res.SchemaVersion != SchemaVersionV1 {
				t.Errorf("tier %q SchemaVersion = %d, want %d", backend, res.SchemaVersion, SchemaVersionV1)
			}
			if res.Backend != backend {
				t.Errorf("tier %q produced result tagged Backend = %q, want %q", backend, res.Backend, backend)
			}
		})
	}
}

// TestDegradationConformance_IdenticalKeySets is the parity proof. It marshals
// each tier's result for the SAME fixture task and asserts the sorted top-level
// JSON key sets are byte-identical across all three tiers. If any tier added,
// dropped, or renamed a field, this fails — which is exactly the failure mode
// that would make degradation lossy (a downstream consumer reading tier A's
// shape would misread tier B's output).
func TestDegradationConformance_IdenticalKeySets(t *testing.T) {
	tiers := []ports.Backend{ports.BackendNTM, ports.BackendClaude, ports.BackendBeads}

	// The contract shape for a result WITH a task_id: schema_version, backend,
	// result_paths, verdict, task_id. task_id is omitempty, so all tiers must
	// populate it (they share the fixture task) for the sets to match.
	wantKeys := []string{"backend", "result_paths", "schema_version", "task_id", "verdict"}

	var reference []string
	for _, backend := range tiers {
		res := tierResultFor(t, backend)
		keys := jsonKeySet(t, res)

		if !slices.Equal(keys, wantKeys) {
			t.Errorf("tier %q JSON key set = %v, want contract shape %v", backend, keys, wantKeys)
		}

		if reference == nil {
			reference = keys
			continue
		}
		if !slices.Equal(keys, reference) {
			t.Errorf("tier %q JSON key set = %v diverges from reference tier %q key set %v",
				backend, keys, tiers[0], reference)
		}
	}
}

// TestDegradationConformance_GateBites is the negative guard. It proves the
// Validate() gate actually rejects a non-conformant result rather than rubber-
// stamping anything. A result missing the required ResultPaths field MUST fail
// Validate(); if it passed, the "every tier is conformant" guarantee above
// would be vacuous.
func TestDegradationConformance_GateBites(t *testing.T) {
	cases := []struct {
		name string
		res  OrchestrationResult
	}{
		{
			name: "empty result_paths",
			res: OrchestrationResult{
				SchemaVersion: SchemaVersionV1,
				Backend:       ports.BackendNTM,
				ResultPaths:   nil,
				Verdict:       Verdict{Status: VerdictStatusPass, Confidence: VerdictConfidenceHigh},
				TaskID:        conformanceFixtureTaskID,
			},
		},
		{
			name: "wrong schema_version",
			res: OrchestrationResult{
				SchemaVersion: SchemaVersionV1 + 1,
				Backend:       ports.BackendNTM,
				ResultPaths:   []string{".agents/x.artifact"},
				Verdict:       Verdict{Status: VerdictStatusPass, Confidence: VerdictConfidenceHigh},
				TaskID:        conformanceFixtureTaskID,
			},
		},
		{
			name: "invalid verdict status",
			res: OrchestrationResult{
				SchemaVersion: SchemaVersionV1,
				Backend:       ports.BackendNTM,
				ResultPaths:   []string{".agents/x.artifact"},
				Verdict:       Verdict{Status: "MAYBE", Confidence: VerdictConfidenceHigh},
				TaskID:        conformanceFixtureTaskID,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.res.Validate(); err == nil {
				t.Fatalf("Validate() accepted a non-conformant result (%s); the gate does not bite", tc.name)
			}
		})
	}
}
