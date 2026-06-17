package extract

// This file compiles a Template's Output block (entities[]/relations[] Field
// schemas) into a JSON Schema string suitable for codex's structured-output
// mode (`codex exec --output-schema`). codex's strict mode requires
// "additionalProperties": false on EVERY object node AND every property listed
// in "required" (a missing additionalProperties yields a 400
// invalid_json_schema — the footgun documented in cli/cmd/ao/eval_scenario_ab.go).
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
// where each item object lists the template's typed fields, marks
// `required:true` fields in its "required" array, and itself sets
// additionalProperties:false.

import (
	"encoding/json"
	"fmt"
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
// It always sets additionalProperties:false and only lists Required fields in
// "required" (we do NOT over-require — optional fields stay out of the array).
func objectSchema(fields []Field) map[string]any {
	props := make(map[string]any, len(fields))
	required := make([]string, 0, len(fields))
	for _, f := range fields {
		prop := map[string]any{"type": jsonType(f.Type)}
		if f.Description != "" {
			prop["description"] = f.Description
		}
		// Arrays need an items schema to be valid JSON Schema; an open string
		// item keeps it permissive without violating additionalProperties.
		if prop["type"] == "array" {
			prop["items"] = map[string]any{"type": "string"}
		}
		props[f.Name] = prop
		if f.Required {
			required = append(required, f.Name)
		}
	}
	obj := map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"properties":           props,
	}
	// Only emit "required" when non-empty: an empty required array is legal but
	// noisy, and not over-requiring is part of the contract.
	if len(required) > 0 {
		obj["required"] = required
	}
	return obj
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
