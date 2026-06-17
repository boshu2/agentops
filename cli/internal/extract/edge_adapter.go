package extract

// This file is the adapter from the typed extractor's relation output to the
// provenance graph write model (age-zbo). It maps an extracted relation record
// {from_id, relation, to_id} onto a provenancegraph.Edge and appends it via
// provenancegraph.Store.Append, REUSING the store's built-in idempotency and
// the edge.go Seal/ComputeHashes sealing primitives AS-IS. It never forks the
// hash-chain logic.
//
// Node types: the extractor's relation records carry only ids, so endpoint
// node types are resolved from a caller-supplied id->entity-node_type map
// (built from the extracted entities' node_type field). The extractor's
// 'finding' entity type is NOT a member of the closed provenancegraph.NodeTypes
// set, so it is mapped to 'artifact' at graph-write time (age-48z documented
// this gap; the template's node_type doc records the same decision). An id with
// no known entity type, or an unmappable type, falls back to 'artifact' — a
// valid NodeType — so a relation is never sealed with an invalid endpoint type.

import (
	"fmt"
	"time"

	"github.com/boshu2/agentops/cli/internal/provenancegraph"
)

// defaultNodeType is the safe fallback endpoint node type for a relation whose
// endpoint id has no known (or no mappable) entity node_type. It is a valid
// member of provenancegraph.NodeTypes.
const defaultNodeType = "artifact"

// mapNodeType maps an extractor entity node_type onto a valid
// provenancegraph.NodeTypes value. The extractor's 'finding' concept has no
// node type in the closed set and is mapped to 'artifact' (age-48z). Any type
// already in the closed set passes through unchanged; anything else falls back
// to defaultNodeType so the resulting edge endpoint is always schema-valid.
func mapNodeType(entityType string) string {
	if entityType == "finding" {
		return "artifact"
	}
	if inNodeTypes(entityType) {
		return entityType
	}
	return defaultNodeType
}

// inNodeTypes reports whether v is a member of provenancegraph.NodeTypes.
func inNodeTypes(v string) bool {
	for _, nt := range provenancegraph.NodeTypes {
		if nt == v {
			return true
		}
	}
	return false
}

// recordString returns the string value of key in a relation Record, or "" if
// absent or not a string.
func recordString(r Record, key string) string {
	if v, ok := r[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// AppendRelation maps one extracted relation Record {from_id, relation, to_id}
// onto a provenancegraph.Edge and appends it to store, reusing the store's
// idempotency and the edge sealing primitives AS-IS. nodeTypes maps a node id
// to its extracted entity node_type (from the same extraction's entities); a
// missing entry falls back to defaultNodeType. trustTier is the edge's trust
// tier (e.g. "mined" for LLM-extracted edges).
//
// The relation verb is rejected BEFORE any Append if it is not a member of the
// closed provenancegraph.Relations enum, so a non-PROV-O verb never reaches the
// sealing/write path and no invalid edge is sealed.
func AppendRelation(
	store *provenancegraph.Store,
	rel Record,
	nodeTypes map[string]string,
	trustTier string,
) (provenancegraph.AppendResult, error) {
	fromID := recordString(rel, "from_id")
	relation := recordString(rel, "relation")
	toID := recordString(rel, "to_id")

	if fromID == "" {
		return provenancegraph.AppendResult{}, fmt.Errorf("relation from_id is required")
	}
	if toID == "" {
		return provenancegraph.AppendResult{}, fmt.Errorf("relation to_id is required")
	}

	// Reject a non-PROV-O verb BEFORE Append so no invalid edge is sealed.
	if !relationInEnum(relation) {
		return provenancegraph.AppendResult{}, fmt.Errorf(
			"relation %q is not a valid PROV-O verb (closed set %v)",
			relation, provenancegraph.Relations,
		)
	}

	edge := provenancegraph.Edge{
		FromID:    fromID,
		FromType:  mapNodeType(nodeTypes[fromID]),
		ToID:      toID,
		ToType:    mapNodeType(nodeTypes[toID]),
		Relation:  relation,
		TrustTier: trustTier,
		TS:        time.Now().UTC().Format(time.RFC3339Nano),
	}

	return store.Append(edge)
}

// relationInEnum reports whether v is a member of the closed
// provenancegraph.Relations PROV-O verb set.
func relationInEnum(v string) bool {
	for _, r := range provenancegraph.Relations {
		if r == v {
			return true
		}
	}
	return false
}
