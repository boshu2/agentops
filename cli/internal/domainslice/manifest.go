// Package domainslice implements the domain-slice manifest model and loader.
//
// A domain-slice manifest (docs/domains/<name>/manifest.yaml) declares a bounded
// DDD domain slice: its owned directives, promoted spec scenarios, implementation
// surface (context_roots), read fence (allowed/denied globs), ordered validation
// commands, and responsible owner.
//
// This package is explicitly DISTINCT from the phaseManifest in
// cli/cmd/ao/rpi_phased_manifest.go.  phaseManifest is a per-phase context-budget
// struct (token limits, handoff field selection); domainSliceManifest is a
// domain-scope declaration.  The two compose during "ao rpi phased --domain":
// phaseManifest controls context depth; domainSliceManifest controls context breadth.
//
// See docs/adr/ADR-0004-domain-slice-manifest-contract.md for design decisions.
package domainslice

import (
	"bytes"
	"fmt"
	"os"
	"regexp"

	"gopkg.in/yaml.v3"
)

// directiveIDPattern is the canonical stable-ID pattern from GOALS.md.
// See schemas/domain-slice-manifest.v1.schema.json and SHARED_TASK_NOTES.md.
var directiveIDPattern = regexp.MustCompile(`^d-[a-z0-9][a-z0-9-]*$`)

// domainNamePattern mirrors the schema constraint for the domain field.
var domainNamePattern = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// semverPattern mirrors the schema constraint for the version field.
var semverPattern = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`)

// ValidationCommand is a single ordered validation step declared in a manifest.
// Mirrors the items schema of validation_commands (additionalProperties:false).
type ValidationCommand struct {
	Label          string `yaml:"label"`
	Command        string `yaml:"command"`
	WorkingDir     string `yaml:"working_dir,omitempty"`
	TimeoutSeconds int    `yaml:"timeout_seconds,omitempty"`
}

// domainSliceManifest declares the bounded DDD domain slice that scopes an
// "ao rpi phased --domain" run: owned directives, scenarios, context roots,
// read fence, and validation commands.
//
// This is DISTINCT from phaseManifest (rpi_phased_manifest.go), which is a
// per-phase context-budget declaration (token limits, handoff field selection)
// unrelated to DDD domain slicing.
//
// All eleven schema fields are required; unknown fields are rejected.
type domainSliceManifest struct {
	SchemaVersion      int                 `yaml:"schema_version"`
	Domain             string              `yaml:"domain"`
	Version            string              `yaml:"version"`
	BoundedContext     string              `yaml:"bounded_context"`
	DirectiveIDs       []string            `yaml:"directive_ids"`
	ScenarioIDs        []string            `yaml:"scenario_ids"`
	ContextRoots       []string            `yaml:"context_roots"`
	AllowedReadGlobs   []string            `yaml:"allowed_read_globs"`
	DeniedReadGlobs    []string            `yaml:"denied_read_globs"`
	ValidationCommands []ValidationCommand `yaml:"validation_commands"`
	Owner              string              `yaml:"owner"`
}

// LoadError is returned for any load or validation failure. It carries the file
// path and (when applicable) the offending field name so callers can produce
// actionable error messages.
type LoadError struct {
	Path  string
	Field string
	Err   error
}

func (e *LoadError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("domain-slice manifest: %s: field %q: %v", e.Path, e.Field, e.Err)
	}
	return fmt.Sprintf("domain-slice manifest: %s: %v", e.Path, e.Err)
}

func (e *LoadError) Unwrap() error { return e.Err }

func loadErr(path, field string, err error) error {
	return &LoadError{Path: path, Field: field, Err: err}
}

// Load reads the manifest.yaml at path, decodes it with strict unknown-field
// rejection, and validates all required fields and pattern constraints.
// Errors name the offending field so callers can present actionable messages.
func Load(path string) (*domainSliceManifest, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, loadErr(path, "", fmt.Errorf("read: %w", err))
	}
	return parse(path, raw)
}

// parse decodes and validates raw YAML content as a domainSliceManifest.
// Separated from Load so tests can exercise parse without touching the filesystem.
func parse(path string, raw []byte) (*domainSliceManifest, error) {
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true) // unknown fields → error (mirrors additionalProperties:false)

	var m domainSliceManifest
	if err := dec.Decode(&m); err != nil {
		return nil, loadErr(path, "", fmt.Errorf("decode: %w", err))
	}
	if err := validate(path, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

// validate enforces the required-field and pattern constraints that the JSON
// Schema declares but that yaml.v3 alone cannot enforce.
// Each check is named so the LoadError carries the offending field name.
func validate(path string, m *domainSliceManifest) error {
	if err := validateScalarFields(path, m); err != nil {
		return err
	}
	if err := validateDirectiveIDs(path, m.DirectiveIDs); err != nil {
		return err
	}
	if err := validateContextRoots(path, m.ContextRoots); err != nil {
		return err
	}
	if err := validateValidationCommands(path, m.ValidationCommands); err != nil {
		return err
	}
	return nil
}

// validateScalarFields checks the simple required-and-pattern constraints on
// the top-level scalar fields.  Extracted to keep validate() under complexity 18.
func validateScalarFields(path string, m *domainSliceManifest) error {
	if m.SchemaVersion != 1 {
		return loadErr(path, "schema_version", fmt.Errorf("must be 1, got %d", m.SchemaVersion))
	}
	if m.Domain == "" {
		return loadErr(path, "domain", fmt.Errorf("required field is empty"))
	}
	if !domainNamePattern.MatchString(m.Domain) {
		return loadErr(path, "domain", fmt.Errorf("must match ^[a-z][a-z0-9-]*$, got %q", m.Domain))
	}
	if m.Version == "" {
		return loadErr(path, "version", fmt.Errorf("required field is empty"))
	}
	if !semverPattern.MatchString(m.Version) {
		return loadErr(path, "version", fmt.Errorf("must be semver (X.Y.Z), got %q", m.Version))
	}
	if m.BoundedContext == "" {
		return loadErr(path, "bounded_context", fmt.Errorf("required field is empty"))
	}
	if m.Owner == "" {
		return loadErr(path, "owner", fmt.Errorf("required field is empty"))
	}
	return nil
}

// validateDirectiveIDs checks every directive_ids entry matches the stable ID pattern.
func validateDirectiveIDs(path string, ids []string) error {
	for i, id := range ids {
		if !directiveIDPattern.MatchString(id) {
			return loadErr(path, fmt.Sprintf("directive_ids[%d]", i),
				fmt.Errorf("must match ^d-[a-z0-9][a-z0-9-]*$, got %q", id))
		}
	}
	return nil
}

// validateContextRoots ensures context_roots has at least one entry (minItems:1).
func validateContextRoots(path string, roots []string) error {
	if len(roots) == 0 {
		return loadErr(path, "context_roots", fmt.Errorf("required: at least one context root"))
	}
	return nil
}

// validateValidationCommands checks each validation_commands entry has the
// required sub-fields (label and command).
func validateValidationCommands(path string, cmds []ValidationCommand) error {
	for i, cmd := range cmds {
		if cmd.Label == "" {
			return loadErr(path, fmt.Sprintf("validation_commands[%d].label", i),
				fmt.Errorf("required field is empty"))
		}
		if cmd.Command == "" {
			return loadErr(path, fmt.Sprintf("validation_commands[%d].command", i),
				fmt.Errorf("required field is empty"))
		}
	}
	return nil
}
