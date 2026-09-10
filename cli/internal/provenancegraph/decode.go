package provenancegraph

import (
	"encoding/json"
	"fmt"

	"github.com/boshu2/agentops/cli/internal/verdictcheck"
)

// decodeEdge checks the exact wire fields before Go's struct decoder can hide
// duplicates, unknown keys, missing fields or nulls behind zero values. Legacy
// optional observations remain accepted without changing their payload hashes.
func decodeEdge(payload []byte) (Edge, error) {
	raw, err := verdictcheck.DecodeObject(payload)
	if err != nil {
		return Edge{}, err
	}
	if err := verdictcheck.ExactFields(raw, []string{
		"schema_version", "from_id", "from_type", "to_id", "to_type", "relation",
		"trust_tier", "ts", "prev_hash", "payload_hash", "hash",
	}, []string{
		"evidence_ref", "bead_id", "merge_sha", "reviewer_family", "degraded",
		"rounds", "duration_s", "tokens_est", "evidence_path",
	}); err != nil {
		return Edge{}, err
	}
	for key, value := range raw {
		if value == nil {
			return Edge{}, fmt.Errorf("field %q must not be null", key)
		}
	}
	var edge Edge
	if err := json.Unmarshal(payload, &edge); err != nil {
		return Edge{}, err
	}
	return edge, nil
}
