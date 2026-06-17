// Package extract implements native template-driven typed structured
// extraction for the AgentOps corpus path. This file owns the
// extraction-template format: a typed Template struct, a YAML loader, and a
// two-layer validator (JSON Schema for structural shape + Go checks for the
// identifier-expression well-formedness the schema cannot express).
//
// Design: .agents/plans/2026-06-17-native-structured-extraction.md (the
// Hyper-Extract WHAT/HOW/identifiers steal). Canonical schema:
// schemas/extraction-template.v1.schema.json (the embedded copy below is kept
// byte-identical by TestTemplate_SchemaParity).
package extract

import (
	"bytes"
	_ "embed"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"gopkg.in/yaml.v3"
)

// schemaJSON is the embedded extraction-template JSON Schema, kept
// byte-identical with schemas/extraction-template.v1.schema.json.
//
//go:embed extraction-template.v1.schema.json
var schemaJSON []byte

// provenanceTemplateYAML is the embedded canonical AgentOps provenance
// extraction template, kept byte-identical with
// templates/agentops_provenance.yaml. Embedding lets callers (e.g. the forge
// typed opt-in path) load the template without depending on a repo checkout
// being present at the process cwd.
//
//go:embed templates/agentops_provenance.yaml
var provenanceTemplateYAML []byte

// LoadProvenanceTemplate parses and validates the embedded canonical AgentOps
// provenance extraction template. It is the runtime-safe equivalent of
// Load("templates/agentops_provenance.yaml") and is used by the forge typed
// opt-in extraction path.
func LoadProvenanceTemplate() (*Template, error) {
	tmpl, err := Parse(provenanceTemplateYAML)
	if err != nil {
		return nil, fmt.Errorf("embedded agentops_provenance template: %w", err)
	}
	return tmpl, nil
}

// schemaPath is the logical name used when compiling the embedded schema.
const schemaPath = "schemas/extraction-template.v1.schema.json"

// allowedTypes enumerates the supported template output structures. It mirrors
// the schema `type` enum. hypergraph/spatial/spatio-temporal are deliberately
// excluded (non-goal §7 of the design).
var allowedTypes = map[string]bool{
	"graph":          true,
	"temporal_graph": true,
	"model":          true,
	"set":            true,
}

// canonicalRelationID is the canonical relation identifier expression:
// '{from}|{relation}|{to}'. A template's relation_id must equal this form.
const canonicalRelationID = "{from}|{relation}|{to}"

// placeholderRe matches a single {token} placeholder in an identifier
// expression.
var placeholderRe = regexp.MustCompile(`\{[a-z_]+\}`)

// Field is one typed field in an entity or relation output schema. It is the
// WHAT: it names a field, its type, a human description, and whether it is
// required.
type Field struct {
	Name        string `yaml:"name" json:"name"`
	Type        string `yaml:"type" json:"type"`
	Description string `yaml:"description" json:"description"`
	Required    bool   `yaml:"required" json:"required"`
}

// Output is the WHAT block: the typed field schemas for extracted entities and
// relations.
type Output struct {
	Entities  []Field `yaml:"entities" json:"entities"`
	Relations []Field `yaml:"relations" json:"relations"`
}

// Identifiers holds the dedup-key expressions. RelationID uses the canonical
// '{from}|{relation}|{to}' form.
type Identifiers struct {
	EntityID   string `yaml:"entity_id" json:"entity_id"`
	RelationID string `yaml:"relation_id" json:"relation_id"`
}

// Template is a typed extraction template: WHAT (Output), HOW (Guideline), and
// dedup keys (Identifiers).
type Template struct {
	Language    string      `yaml:"language" json:"language"`
	Name        string      `yaml:"name" json:"name"`
	Type        string      `yaml:"type" json:"type"`
	Tags        []string    `yaml:"tags" json:"tags"`
	Description string      `yaml:"description" json:"description"`
	Output      Output      `yaml:"output" json:"output"`
	Guideline   string      `yaml:"guideline" json:"guideline"`
	Identifiers Identifiers `yaml:"identifiers" json:"identifiers"`
}

// compiledSchema lazily compiles the embedded JSON Schema. It is computed once
// and reused.
func compiledSchema() (*jsonschema.Schema, error) {
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(schemaJSON))
	if err != nil {
		return nil, fmt.Errorf("parse embedded schema %s: %w", schemaPath, err)
	}
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource(schemaPath, doc); err != nil {
		return nil, fmt.Errorf("add schema resource %s: %w", schemaPath, err)
	}
	schema, err := compiler.Compile(schemaPath)
	if err != nil {
		return nil, fmt.Errorf("compile schema %s: %w", schemaPath, err)
	}
	return schema, nil
}

// Load reads a YAML extraction-template from path, parses it into a typed
// Template, and validates it. A non-nil error means the file is missing,
// unparseable, or invalid.
func Load(path string) (*Template, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read template %s: %w", path, err)
	}
	tmpl, err := Parse(data)
	if err != nil {
		return nil, fmt.Errorf("template %s: %w", path, err)
	}
	return tmpl, nil
}

// Parse parses raw YAML bytes into a typed Template and validates it. It is the
// in-memory equivalent of Load.
func Parse(data []byte) (*Template, error) {
	// Pre-check the type enum from a shallow decode first: jsonschema/v6 reports
	// the allowed set but not the offending value, and the contract requires the
	// error to NAME the disallowed type. This Go check names it.
	var head struct {
		Type string `yaml:"type"`
	}
	if err := yaml.Unmarshal(data, &head); err == nil && head.Type != "" && !allowedTypes[head.Type] {
		return nil, fmt.Errorf("invalid template type %q: must be one of graph, temporal_graph, model, set", head.Type)
	}
	// Validate the raw document against the JSON Schema (structural shape,
	// required keys, enums, additionalProperties:false) before binding to the
	// typed struct, mirroring cli/cmd/ao/codex_schema.go.
	if err := validateSchema(data); err != nil {
		return nil, err
	}
	var tmpl Template
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&tmpl); err != nil {
		return nil, fmt.Errorf("parse template yaml: %w", err)
	}
	if err := tmpl.Validate(); err != nil {
		return nil, err
	}
	return &tmpl, nil
}

// validateSchema validates raw YAML bytes against the JSON Schema. YAML is
// decoded to a generic value, then validated, so the schema's required/enum/
// additionalProperties rules apply to the on-disk shape.
func validateSchema(data []byte) error {
	schema, err := compiledSchema()
	if err != nil {
		return err
	}
	var raw any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("parse template yaml: %w", err)
	}
	// yaml.v3 decodes mappings into map[string]interface{} when keys are
	// strings, which jsonschema/v6 accepts. Normalize any map[interface{}]...
	// defensively in case of non-string keys.
	norm, err := normalizeYAML(raw)
	if err != nil {
		return err
	}
	if err := schema.Validate(norm); err != nil {
		return fmt.Errorf("template violates %s: %w", schemaPath, err)
	}
	return nil
}

// normalizeYAML converts any map[interface{}]interface{} produced by YAML
// decoding into map[string]interface{} so jsonschema/v6 can validate it.
func normalizeYAML(v any) (any, error) {
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			nv, err := normalizeYAML(val)
			if err != nil {
				return nil, err
			}
			out[k] = nv
		}
		return out, nil
	case map[any]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			ks, ok := k.(string)
			if !ok {
				return nil, fmt.Errorf("non-string map key %v in template", k)
			}
			nv, err := normalizeYAML(val)
			if err != nil {
				return nil, err
			}
			out[ks] = nv
		}
		return out, nil
	case []any:
		out := make([]any, len(t))
		for i, val := range t {
			nv, err := normalizeYAML(val)
			if err != nil {
				return nil, err
			}
			out[i] = nv
		}
		return out, nil
	default:
		return v, nil
	}
}

// Validate runs the Go-level invariants the JSON Schema cannot express: the
// type enum (defense-in-depth), the presence of the guideline (HOW), and the
// well-formedness of the identifier expressions. It is exported so callers that
// build a Template in memory can validate without re-serializing.
func (t *Template) Validate() error {
	if !allowedTypes[t.Type] {
		return fmt.Errorf("invalid template type %q: must be one of graph, temporal_graph, model, set", t.Type)
	}
	if strings.TrimSpace(t.Guideline) == "" {
		return fmt.Errorf("template missing required guideline (the HOW)")
	}
	if strings.TrimSpace(t.Identifiers.RelationID) != "" {
		if err := validateRelationIdentifier(t.Identifiers.RelationID); err != nil {
			return err
		}
	}
	return nil
}

// validateRelationIdentifier enforces that a relation identifier expression is
// of the canonical form '{from}|{relation}|{to}': exactly three placeholders,
// pipe-separated, in order, with no extra literal text.
func validateRelationIdentifier(expr string) error {
	if expr == canonicalRelationID {
		return nil
	}
	parts := strings.Split(expr, "|")
	if len(parts) != 3 {
		return fmt.Errorf("malformed relation identifier %q: must be of the form %q (three pipe-separated placeholders)", expr, canonicalRelationID)
	}
	want := []string{"{from}", "{relation}", "{to}"}
	for i, p := range parts {
		p = strings.TrimSpace(p)
		if !placeholderRe.MatchString(p) {
			return fmt.Errorf("malformed relation identifier %q: segment %q is not a {placeholder}", expr, p)
		}
		if p != want[i] {
			return fmt.Errorf("malformed relation identifier %q: expected %q at position %d, got %q", expr, want[i], i+1, p)
		}
	}
	return nil
}
