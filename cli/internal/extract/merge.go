package extract

// This file implements deterministic identifier-keyed dedup and incremental
// field-merge of extracted Records (age-f66, the owned ontomem steal). It is the
// NET-NEW entity/relation merge engine: there is no pre-existing merge engine in
// the corpus path (the string dedup_key marker at rpi/types.go is just a marker).
//
// v1 scope is EXACT-KEY only. The merge key is derived from the template's
// identifier expressions: for entities, the value of the field named by
// Identifiers.EntityID; for relations, the Identifiers.RelationID expression
// '{from}|{relation}|{to}' with each {token} placeholder substituted by the
// value of the record field named by that token. On a key collision the merge is
// field-level: keep an existing non-empty scalar, fill an empty scalar from the
// incoming record, and UNION list-valued fields. Output is deterministic and
// byte-stable across runs (merged records are emitted in sorted-key order and
// list unions preserve first-seen order without duplicates).
//
// Explicitly DEFERRED / out of scope for v1: fuzzy/semantic identifier
// resolution and any LLM-based field synthesis. Merge makes NO model call and
// takes no client/Generator — it is pure, in-memory, and deterministic.
//
// Design: .agents/plans/2026-06-17-native-structured-extraction.md.

import (
	"fmt"
	"sort"
	"strings"
)

// mergeKeyPlaceholderRe matches a single {token} placeholder in an identifier
// expression. It mirrors template.go's placeholderRe; kept local so merge.go has
// no hidden coupling to the template validator's package-level regexp.
var mergeKeyPlaceholderRe = placeholderRe

// recordKey derives the deterministic merge key for a record given an identifier
// expression. If the expression contains {token} placeholders it is a relation
// identifier (e.g. '{from}|{relation}|{to}'): every placeholder is replaced by
// the string value of the like-named record field, with the surrounding literal
// text (the pipes) preserved. If the expression contains no placeholders it is
// treated as a bare entity field NAME (e.g. "id" or "name") and the key is the
// string value of that field. An empty/absent field value contributes an empty
// segment — records with the same resolved key string collide.
func recordKey(r Record, idExpr string) string {
	if mergeKeyPlaceholderRe.MatchString(idExpr) {
		return mergeKeyPlaceholderRe.ReplaceAllStringFunc(idExpr, func(tok string) string {
			field := strings.TrimSuffix(strings.TrimPrefix(tok, "{"), "}")
			return recordString(r, field)
		})
	}
	return recordString(r, idExpr)
}

// mergeRecords field-merges incoming into existing IN PLACE on existing,
// returning whether existing was changed. The merge rule per field:
//   - list-valued fields ([]any): UNION (existing values, then incoming values
//     not already present), preserving first-seen order.
//   - scalar/other fields: keep an existing non-empty value; fill from incoming
//     only when the existing value is empty/absent.
//
// "Empty" for a scalar means absent, a nil value, or an empty string. Non-string
// scalars (numbers, bools) are treated as non-empty when present.
func mergeRecords(existing, incoming Record) bool {
	changed := false
	for k, inVal := range incoming {
		exVal, hasEx := existing[k]
		if inList, ok := asList(inVal); ok {
			exList, _ := asList(exVal)
			merged, listChanged := unionLists(exList, inList)
			if listChanged || !hasEx {
				existing[k] = merged
				changed = true
			}
			continue
		}
		if scalarEmpty(exVal, hasEx) && !scalarEmpty(inVal, true) {
			existing[k] = inVal
			changed = true
		}
	}
	return changed
}

// asList reports whether v is a list value and returns it as []any. Records are
// decoded from JSON, so list fields are []any.
func asList(v any) ([]any, bool) {
	l, ok := v.([]any)
	return l, ok
}

// unionLists returns the union of existing and incoming, preserving existing
// order then appending incoming values not already present. It reports whether
// any incoming value was added (i.e. the union differs from existing). Equality
// uses the values' fmt %v rendering, which is stable for the JSON scalar/list
// element types records carry.
func unionLists(existing, incoming []any) ([]any, bool) {
	seen := make(map[string]bool, len(existing)+len(incoming))
	out := make([]any, 0, len(existing)+len(incoming))
	for _, v := range existing {
		key := elemKey(v)
		if !seen[key] {
			seen[key] = true
			out = append(out, v)
		}
	}
	added := false
	for _, v := range incoming {
		key := elemKey(v)
		if !seen[key] {
			seen[key] = true
			out = append(out, v)
			added = true
		}
	}
	return out, added
}

// elemKey renders a list element to a stable string for de-dup comparison.
func elemKey(v any) string {
	return fmt.Sprintf("%T:%v", v, v)
}

// scalarEmpty reports whether a scalar field value should be treated as empty
// (and therefore fillable from an incoming record). present is whether the key
// existed at all.
func scalarEmpty(v any, present bool) bool {
	if !present || v == nil {
		return true
	}
	if s, ok := v.(string); ok {
		return strings.TrimSpace(s) == ""
	}
	return false
}

// dedupMerge collapses records by their identifier key (idExpr), field-merging
// colliding records, and returns the merged records in deterministic
// sorted-key order. The input slice is not mutated; merged records are fresh
// copies. Records whose resolved key is empty are kept as-is and keyed by their
// empty-key string (so multiple keyless records of the SAME empty key still
// collide deterministically, matching the exact-key rule).
func dedupMerge(records []Record, idExpr string) []Record {
	byKey := make(map[string]Record, len(records))
	order := make([]string, 0, len(records))
	for _, rec := range records {
		key := recordKey(rec, idExpr)
		if existing, ok := byKey[key]; ok {
			mergeRecords(existing, rec)
			continue
		}
		byKey[key] = cloneRecord(rec)
		order = append(order, key)
	}
	sort.Strings(order)
	out := make([]Record, 0, len(order))
	for _, key := range order {
		out = append(out, byKey[key])
	}
	return out
}

// cloneRecord returns a shallow copy of a Record. List-valued fields are copied
// to fresh slices so a later in-place union on the merged record never mutates a
// caller's input record.
func cloneRecord(r Record) Record {
	out := make(Record, len(r))
	for k, v := range r {
		if l, ok := v.([]any); ok {
			cp := make([]any, len(l))
			copy(cp, l)
			out[k] = cp
			continue
		}
		out[k] = v
	}
	return out
}

// Merge deduplicates and incrementally field-merges a base Result with the
// records produced by a later extraction pass, keyed by the template's
// identifier expressions. It is deterministic and makes NO LLM call.
//
// Both base and next may be nil. Entities are keyed by tmpl.Identifiers.EntityID;
// relations by tmpl.Identifiers.RelationID. The result's Entities and Relations
// are emitted in deterministic sorted-key order. SurvivingChunks from both
// inputs are carried through (base then next), since the merge does not re-run
// extraction.
//
// Merge is incremental and idempotent: merging the same next pass into an
// already-merged base a second time produces a byte-identical Result (no field
// changes, no new keys), because the field-merge rule (keep-existing scalars,
// set-union lists) is itself idempotent and the key derivation is pure.
func Merge(base, next *Result, tmpl *Template) (*Result, error) {
	if tmpl == nil {
		return nil, fmt.Errorf("merge: nil template")
	}
	entityID := strings.TrimSpace(tmpl.Identifiers.EntityID)
	relationID := strings.TrimSpace(tmpl.Identifiers.RelationID)

	var entities, relations []Record
	var survivors []int
	if base != nil {
		entities = append(entities, base.Entities...)
		relations = append(relations, base.Relations...)
		survivors = append(survivors, base.SurvivingChunks...)
	}
	if next != nil {
		entities = append(entities, next.Entities...)
		relations = append(relations, next.Relations...)
		survivors = append(survivors, next.SurvivingChunks...)
	}

	out := &Result{
		Entities:        dedupMerge(entities, entityID),
		Relations:       dedupMerge(relations, relationID),
		SurvivingChunks: survivors,
	}
	return out, nil
}
