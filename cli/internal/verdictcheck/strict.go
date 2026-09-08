package verdictcheck

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"unicode/utf8"
)

// DecodeObject is the shared strict JSON entry point for evidence mechanics.
// It rejects duplicate keys at every depth, trailing data and non-object roots.
func DecodeObject(payload []byte) (map[string]any, error) {
	if err := validUnicodeEscapes(payload); err != nil {
		return nil, err
	}
	if !utf8.Valid(payload) {
		return nil, fmt.Errorf("JSON is not UTF-8")
	}
	if dup, err := duplicateKey(payload); err != nil {
		return nil, err
	} else if dup != "" {
		return nil, fmt.Errorf("duplicate JSON key %q", dup)
	}
	var value map[string]any
	dec := json.NewDecoder(bytes.NewReader(payload))
	dec.UseNumber()
	if err := dec.Decode(&value); err != nil {
		return nil, err
	}
	if err := requireJSONEOF(dec); err != nil {
		return nil, err
	}
	if value == nil {
		return nil, fmt.Errorf("expected JSON object")
	}
	return value, nil
}

// ExactFields rejects unknown fields and missing required fields.
func ExactFields(value map[string]any, required, optional []string) error {
	allowed := map[string]bool{}
	for _, key := range required {
		allowed[key] = true
		if _, ok := value[key]; !ok {
			return fmt.Errorf("missing required field %q", key)
		}
	}
	for _, key := range optional {
		allowed[key] = true
	}
	for key := range value {
		if !allowed[key] {
			return fmt.Errorf("unknown field %q", key)
		}
	}
	return nil
}

func strictVerdictFields(raw map[string]any) error {

	// DisallowUnknownFields alone is insufficient: encoding/json matches struct
	// field names without regard to case. Check the original maps before decoding.
	if err := ExactFields(raw, []string{
		"schema_version", "acceptance_digest", "subject_manifest_digest",
		"author_context_id", "validator_context_id", "freshness_attestation",
		"verdict", "criteria", "findings", "evidence_refs", "checked",
		"not_checked", "validated_at", "artifact_digest",
	}, nil); err != nil {
		return err
	}
	if value := raw["freshness_attestation"]; value != nil {
		freshness, ok := value.(map[string]any)
		if !ok {
			return fmt.Errorf("freshness_attestation must be null or an object")
		}
		if err := ExactFields(freshness, []string{"source", "attester_identity"}, nil); err != nil {
			return err
		}
	}
	for _, key := range []string{"criteria", "findings", "evidence_refs", "checked", "not_checked"} {
		if _, ok := raw[key].([]any); !ok {
			return fmt.Errorf("%s must be an array", key)
		}
	}
	for _, entry := range raw["criteria"].([]any) {
		obj, ok := entry.(map[string]any)
		if !ok {
			return fmt.Errorf("criterion must be an object")
		}
		if err := ExactFields(obj, []string{"id", "result", "evidence_refs"}, []string{"reason"}); err != nil {
			return err
		}
		if reason, exists := obj["reason"]; exists {
			if _, ok := reason.(string); !ok {
				return fmt.Errorf("criterion reason must be a string")
			}
		}
	}
	for _, entry := range raw["findings"].([]any) {
		obj, ok := entry.(map[string]any)
		if !ok {
			return fmt.Errorf("finding must be an object")
		}
		if err := ExactFields(obj, []string{"id", "summary", "evidence_refs"}, []string{"class"}); err != nil {
			return err
		}
		if class, exists := obj["class"]; exists {
			if _, ok := class.(string); !ok {
				return fmt.Errorf("finding class must be a string")
			}
		}
	}
	return nil
}

// encoding/json replaces unpaired UTF-16 surrogates with U+FFFD. That would
// silently change exact input identity; the Python canonical UTF-8 writer also
// refuses these strings. Consume escaped backslashes so literal text survives.
func validUnicodeEscapes(payload []byte) error {
	inString := false
	for i := 0; i < len(payload); i++ {
		if payload[i] == '"' {
			inString = !inString
			continue
		}
		if !inString || payload[i] != 92 {
			continue
		}
		i++
		if i >= len(payload) || payload[i] != 'u' {
			continue
		}
		if i+4 >= len(payload) {
			return fmt.Errorf("invalid Unicode escape")
		}
		code, err := strconv.ParseUint(string(payload[i+1:i+5]), 16, 16)
		if err != nil {
			return err
		}
		i += 4
		if code >= 0xdc00 && code <= 0xdfff {
			return fmt.Errorf("unpaired Unicode surrogate")
		}
		if code < 0xd800 || code > 0xdbff {
			continue
		}
		if i+6 >= len(payload) || payload[i+1] != 92 || payload[i+2] != 'u' {
			return fmt.Errorf("unpaired Unicode surrogate")
		}
		low, err := strconv.ParseUint(string(payload[i+3:i+7]), 16, 16)
		if err != nil || low < 0xdc00 || low > 0xdfff {
			return fmt.Errorf("unpaired Unicode surrogate")
		}
		i += 6
	}
	return nil
}
