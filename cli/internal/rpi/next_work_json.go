package rpi

import (
	"bytes"
	"encoding/json"
	"reflect"
	"sort"
	"strings"
)

// next_work_json.go — lossless JSON round-trip for the next-work.jsonl contract.
//
// The queue is a shared-mutable file: producers append, consumers rewrite lines
// to claim/consume individual items (RewriteNextWorkFile), and operators or
// external-lane tools sometimes hand-edit it. Because the rewrite path decodes a
// line into NextWorkEntry, mutates it, and re-encodes, any object key the typed
// struct does not model would be silently dropped on the next rewrite — the
// reverted-flag / dropped-note hazard (an operator's batch-level consumed_note
// and forward-compat fields like a stray "notes" key both die that way).
//
// The schema promises additionalProperties:true and "readers MUST tolerate
// unknown fields for forward compatibility"; the writer must honor the same
// promise. These marshalers capture unknown object keys into an Extra map on
// decode and re-emit them on encode. When Extra is empty the output is
// byte-identical to default struct marshaling, so existing rows are untouched.

// knownNextWorkItemKeys / knownNextWorkEntryKeys are the JSON object keys the
// typed structs own. Derived by reflection over the struct tags so adding a
// typed field can never desync the set from the struct (a stale hand-maintained
// list would misclassify a new field as unknown and emit it twice).
var (
	knownNextWorkItemKeys  = jsonObjectKeys(reflect.TypeOf(NextWorkItem{}))
	knownNextWorkEntryKeys = jsonObjectKeys(reflect.TypeOf(NextWorkEntry{}))
)

// jsonObjectKeys returns the serialized JSON key of every struct field, skipping
// fields tagged json:"-" (Extra, QueueIndex).
func jsonObjectKeys(t reflect.Type) map[string]struct{} {
	keys := make(map[string]struct{}, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		name := strings.SplitN(t.Field(i).Tag.Get("json"), ",", 2)[0]
		if name == "" || name == "-" {
			continue
		}
		keys[name] = struct{}{}
	}
	return keys
}

// MarshalJSON emits the declared item fields in struct order, then splices in any
// unknown keys captured in Extra. Byte-identical to default marshaling when Extra
// is empty.
func (i NextWorkItem) MarshalJSON() ([]byte, error) {
	type itemAlias NextWorkItem // shed methods to avoid recursion
	base, err := json.Marshal(itemAlias(i))
	if err != nil {
		return nil, err
	}
	return appendUnknownJSONFields(base, i.Extra)
}

// UnmarshalJSON decodes the declared item fields and preserves every other object
// key in Extra so a later rewrite round-trips them losslessly.
func (i *NextWorkItem) UnmarshalJSON(data []byte) error {
	type itemAlias NextWorkItem
	var alias itemAlias
	if err := json.Unmarshal(data, &alias); err != nil {
		return err
	}
	*i = NextWorkItem(alias)
	i.Extra = extraObjectFields(data, knownNextWorkItemKeys)
	return nil
}

// MarshalJSON emits the declared entry fields in struct order (items marshal via
// NextWorkItem.MarshalJSON, so their Extra is preserved too), then splices in any
// unknown batch-level keys captured in Extra.
func (e NextWorkEntry) MarshalJSON() ([]byte, error) {
	type entryAlias NextWorkEntry
	base, err := json.Marshal(entryAlias(e))
	if err != nil {
		return nil, err
	}
	return appendUnknownJSONFields(base, e.Extra)
}

// UnmarshalJSON decodes the declared entry fields and preserves every other
// batch-level object key in Extra.
func (e *NextWorkEntry) UnmarshalJSON(data []byte) error {
	type entryAlias NextWorkEntry
	var alias entryAlias
	if err := json.Unmarshal(data, &alias); err != nil {
		return err
	}
	*e = NextWorkEntry(alias)
	e.Extra = extraObjectFields(data, knownNextWorkEntryKeys)
	return nil
}

// extraObjectFields returns the object keys in data that are not in known, or nil
// when data is not a JSON object or carries no unknown keys.
func extraObjectFields(data []byte, known map[string]struct{}) map[string]json.RawMessage {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil
	}
	var extra map[string]json.RawMessage
	for k, v := range raw {
		if _, ok := known[k]; ok {
			continue
		}
		if extra == nil {
			extra = make(map[string]json.RawMessage, 1)
		}
		extra[k] = v
	}
	return extra
}

// appendUnknownJSONFields splices extra key/value pairs into a marshaled JSON
// object immediately before its closing brace, preserving the declared field
// order. Keys are emitted sorted so repeated rewrites are byte-stable. A nil or
// empty extra returns base unchanged; a base that is not a JSON object is
// returned unchanged (fail-open: never corrupt a value we cannot splice).
func appendUnknownJSONFields(base []byte, extra map[string]json.RawMessage) ([]byte, error) {
	if len(extra) == 0 {
		return base, nil
	}
	trimmed := bytes.TrimRight(base, " \t\r\n")
	if len(trimmed) == 0 || trimmed[len(trimmed)-1] != '}' {
		return base, nil
	}

	keys := make([]string, 0, len(extra))
	for k := range extra {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var buf bytes.Buffer
	buf.Write(trimmed[:len(trimmed)-1]) // object body without the closing brace
	needComma := len(bytes.TrimSpace(trimmed[1:len(trimmed)-1])) > 0
	for _, k := range keys {
		if needComma {
			buf.WriteByte(',')
		}
		keyJSON, err := json.Marshal(k)
		if err != nil {
			return nil, err
		}
		buf.Write(keyJSON)
		buf.WriteByte(':')
		buf.Write(extra[k])
		needComma = true
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}
