package extract

// This file compiles a Template's Output block (entities[]/relations[] Field
// schemas) into a JSON Schema string suitable for codex's structured-output
// mode (`codex exec --output-schema`). codex's strict mode (OpenAI strict
// structured outputs) requires, on EVERY object node:
//
//   - "additionalProperties": false, AND
//   - "required" listing EVERY key in "properties" (not just the
//     template's required:true fields).
//
// Violating either yields a 400 invalid_json_schema, e.g.
// "'required' is required to be supplied and to be an array including every
// key in properties". A template-OPTIONAL field cannot simply be omitted from
// "required" — under strict mode it must instead be made NULLABLE (its "type"
// becomes a ["<jsonType>","null"] union) so the model may emit it as null.
// This is the strict-mode contract enforced by ValidateCodexStrictSchema (the
// single source of truth for "codex-valid") and matched by the hand-written
// judgeOutputSchema in cli/cmd/ao/eval_scenario_ab.go.
//
// The emitted shape is:
//
//	{
//	  "type": "object",
//	  "additionalProperties": false,
//	  "properties": {
//	    "entities":  {"type":"array","items": <entity object>},
//	    "relations": {"type":"array","items": <relation object>}
//	  },
//	  "required": ["entities","relations"]
//	}
//
// where each item object lists the template's typed fields, lists EVERY field
// in its "required" array (template-optional fields made nullable), and itself
// sets additionalProperties:false.

import (
	"encoding/json"
	"fmt"
	"sort"
)

// jsonType maps a template Field.Type onto a JSON Schema primitive type. Unknown
// types fall back to "string" (the safest permissive shape) so an exotic
// template field never produces an uncompilable schema.
func jsonType(fieldType string) string {
	switch fieldType {
	case "string", "text":
		return "string"
	case "int", "integer", "number", "float":
		return "number"
	case "bool", "boolean":
		return "boolean"
	case "array", "list":
		return "array"
	case "object", "map":
		return "object"
	default:
		return "string"
	}
}

// objectSchema builds the JSON Schema object node for one list of typed fields.
// Per the codex strict-mode contract (see file header), it sets
// additionalProperties:false AND lists EVERY field in "required". A
// template-optional field (Required==false) cannot be dropped from "required"
// under strict mode; instead its "type" is widened to a ["<jsonType>","null"]
// union so the model may emit it as null. Required fields keep a single-string
// type. The output of this function always satisfies ValidateCodexStrictSchema.
func objectSchema(fields []Field) map[string]any {
	props := make(map[string]any, len(fields))
	required := make([]string, 0, len(fields))
	for _, f := range fields {
		jt := jsonType(f.Type)
		prop := map[string]any{}
		if f.Required {
			prop["type"] = jt
		} else {
			// Optional fields must be nullable under strict mode so they can be
			// emitted as null while still appearing in "required".
			prop["type"] = []any{jt, "null"}
		}
		if f.Description != "" {
			prop["description"] = f.Description
		}
		// Arrays need an items schema to be valid JSON Schema; an open string
		// item keeps it permissive without violating additionalProperties.
		if jt == "array" {
			prop["items"] = map[string]any{"type": "string"}
		}
		props[f.Name] = prop
		// Strict mode: required must include EVERY property key, optional or not.
		required = append(required, f.Name)
	}
	// Sort for deterministic output (map iteration order is otherwise random).
	sort.Strings(required)
	obj := map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"properties":           props,
	}
	if len(required) > 0 {
		obj["required"] = required
	}
	return obj
}

// ValidateCodexStrictSchema is the single source of truth for "codex-valid":
// it recursively walks a marshaled JSON Schema and returns an error unless
// EVERY object node sets "additionalProperties": false AND its "required"
// array contains exactly every key in its "properties". This is the contract
// OpenAI strict structured outputs (codex --output-schema) enforces; a schema
// that passes this validator will not be rejected with 400 invalid_json_schema
// for the required/additionalProperties reasons. Both CompileSchema output and
// the hand-written judgeOutputSchema are pinned to this contract by tests.
func ValidateCodexStrictSchema(schema []byte) error {
	var doc any
	if err := json.Unmarshal(schema, &doc); err != nil {
		return fmt.Errorf("validate codex strict schema: unmarshal: %w", err)
	}
	return validateStrictNode(doc, "$")
}

func validateStrictNode(node any, path string) error {
	switch n := node.(type) {
	case map[string]any:
		propsRaw, hasProps := n["properties"]
		isObject := n["type"] == "object"
		if isObject || hasProps {
			// additionalProperties must be present and false.
			ap, ok := n["additionalProperties"]
			if !ok {
				return fmt.Errorf("object node at %s is missing additionalProperties (codex 400)", path)
			}
			if ap != false {
				return fmt.Errorf("object node at %s has additionalProperties=%v, want false", path, ap)
			}
			// required must list exactly every key in properties.
			props, _ := propsRaw.(map[string]any)
			reqRaw, hasReq := n["required"]
			reqSet := map[string]bool{}
			if hasReq {
				reqArr, ok := reqRaw.([]any)
				if !ok {
					return fmt.Errorf("object node at %s has non-array required", path)
				}
				for _, r := range reqArr {
					s, ok := r.(string)
					if !ok {
						return fmt.Errorf("object node at %s has non-string entry in required", path)
					}
					reqSet[s] = true
				}
			}
			if len(reqSet) != len(props) {
				return fmt.Errorf("object node at %s: required has %d keys, properties has %d (every property must be required under strict mode)", path, len(reqSet), len(props))
			}
			for k := range props {
				if !reqSet[k] {
					return fmt.Errorf("object node at %s: property %q is missing from required (codex 400 invalid_json_schema)", path, k)
				}
			}
		}
		for k, v := range n {
			if err := validateStrictNode(v, path+"/"+k); err != nil {
				return err
			}
		}
	case []any:
		for i, v := range n {
			if err := validateStrictNode(v, fmt.Sprintf("%s[%d]", path, i)); err != nil {
				return err
			}
		}
	}
	return nil
}

// CompileSchema compiles a Template's Output into a JSON Schema document for
// codex --output-schema. The top-level object carries an "entities" array and a
// "relations" array, both required, with additionalProperties:false set on the
// top-level object and on every item object.
func CompileSchema(tmpl *Template) ([]byte, error) {
	if tmpl == nil {
		return nil, fmt.Errorf("compile schema: nil template")
	}
	root := map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"properties": map[string]any{
			"entities": map[string]any{
				"type":  "array",
				"items": objectSchema(tmpl.Output.Entities),
			},
			"relations": map[string]any{
				"type":  "array",
				"items": objectSchema(tmpl.Output.Relations),
			},
		},
		"required": []string{"entities", "relations"},
	}
	out, err := json.Marshal(root)
	if err != nil {
		return nil, fmt.Errorf("marshal compiled schema: %w", err)
	}
	return out, nil
}

// CompileSchemaString is the string convenience wrapper over CompileSchema.
func CompileSchemaString(tmpl *Template) (string, error) {
	b, err := CompileSchema(tmpl)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
