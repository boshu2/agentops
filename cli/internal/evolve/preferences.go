// Package evolve provides per-repo operator preferences for the autonomous
// /evolve loop. Preferences live in .agents/evolve/preferences.yaml. The
// resolution order is:
//
//  1. defaults (Go constants in Defaults())
//  2. preferences.yaml overrides defaults
//  3. CLI flag overrides preferences.yaml (the caller applies this step)
//
// Invalid keys, types, or out-of-range values produce an error containing the
// preferences.yaml line:column for operator triage. There is no silent
// fallback.
package evolve

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// PreferencesRelPath is the location of the preferences file relative to a
// repo working directory.
const PreferencesRelPath = ".agents/evolve/preferences.yaml"

// Prefs is the in-memory representation of evolve preferences after Load has
// merged defaults with the on-disk file.
type Prefs struct {
	SchemaVersion            int              `yaml:"schema_version" json:"schema_version"`
	ModeDefault              string           `yaml:"mode_default" json:"mode_default"`
	ScopeFilter              ScopeFilterPrefs `yaml:"scope_filter" json:"scope_filter"`
	RecommendedPointerStrict bool             `yaml:"recommended_pointer_strict" json:"recommended_pointer_strict"`
	HaltSignals              []string         `yaml:"halt_signals" json:"halt_signals"`
	GeneratorLayersEnabled   bool             `yaml:"generator_layers_enabled" json:"generator_layers_enabled"`
}

// ScopeFilterPrefs controls when /evolve narrows scope from explore to exploit.
type ScopeFilterPrefs struct {
	ProductiveThreshold int  `yaml:"productive_threshold" json:"productive_threshold"`
	ScoutStreakHalt     bool `yaml:"scout_streak_halt" json:"scout_streak_halt"`
}

// Defaults returns a fresh Prefs populated with the canonical defaults. The
// caller may mutate the returned value safely; HaltSignals is a fresh slice.
func Defaults() *Prefs {
	return &Prefs{
		SchemaVersion: 1,
		ModeDefault:   "burst",
		ScopeFilter: ScopeFilterPrefs{
			ProductiveThreshold: 5,
			ScoutStreakHalt:     true,
		},
		RecommendedPointerStrict: true,
		HaltSignals: []string{
			".agents/evolve/STOP",
			".agents/evolve/KILL",
		},
		GeneratorLayersEnabled: true,
	}
}

// Load reads .agents/evolve/preferences.yaml relative to the current working
// directory. A missing file is not an error; Defaults() is returned with a nil
// error. Malformed YAML or schema violations produce an error whose message
// includes "preferences.yaml:line:column" context.
func Load(ctx context.Context) (*Prefs, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("get working directory: %w", err)
	}
	return LoadFromDir(ctx, cwd)
}

// LoadFromDir is the testable variant of Load. The path .agents/evolve/preferences.yaml
// is resolved relative to dir.
func LoadFromDir(ctx context.Context, dir string) (*Prefs, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, PreferencesRelPath)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Defaults(), nil
		}
		return nil, fmt.Errorf("read preferences file %s: %w", path, err)
	}
	return parsePreferences(path, data)
}

// parsePreferences walks the YAML document via yaml.Node so we can attach
// line:column context to every validation error.
func parsePreferences(path string, data []byte) (*Prefs, error) {
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("%s: parse yaml: %w", path, err)
	}
	prefs := Defaults()
	if root.Kind == 0 {
		// Empty file → defaults stand.
		return prefs, nil
	}
	doc := &root
	if doc.Kind == yaml.DocumentNode {
		if len(doc.Content) == 0 {
			return prefs, nil
		}
		doc = doc.Content[0]
	}
	if doc.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("%s:%d:%d: expected mapping at root, got %s",
			path, doc.Line, doc.Column, yamlKind(doc))
	}
	if err := applyMapping(path, doc, prefs); err != nil {
		return nil, err
	}
	if err := validatePrefs(path, doc, prefs); err != nil {
		return nil, err
	}
	return prefs, nil
}

// applyMapping populates prefs from the top-level YAML mapping.
func applyMapping(path string, node *yaml.Node, prefs *Prefs) error {
	for i := 0; i < len(node.Content); i += 2 {
		k := node.Content[i]
		v := node.Content[i+1]
		switch k.Value {
		case "schema_version":
			n, err := requireInt(path, k.Value, v)
			if err != nil {
				return err
			}
			prefs.SchemaVersion = n
		case "mode_default":
			s, err := requireString(path, k.Value, v)
			if err != nil {
				return err
			}
			prefs.ModeDefault = s
		case "scope_filter":
			if err := applyScopeFilter(path, v, &prefs.ScopeFilter); err != nil {
				return err
			}
		case "recommended_pointer_strict":
			b, err := requireBool(path, k.Value, v)
			if err != nil {
				return err
			}
			prefs.RecommendedPointerStrict = b
		case "halt_signals":
			ss, err := requireStringList(path, k.Value, v)
			if err != nil {
				return err
			}
			prefs.HaltSignals = ss
		case "generator_layers_enabled":
			b, err := requireBool(path, k.Value, v)
			if err != nil {
				return err
			}
			prefs.GeneratorLayersEnabled = b
		default:
			return fmt.Errorf("%s:%d:%d: unknown key %q",
				path, k.Line, k.Column, k.Value)
		}
	}
	return nil
}

// applyScopeFilter populates the nested ScopeFilter struct.
func applyScopeFilter(path string, node *yaml.Node, sf *ScopeFilterPrefs) error {
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("%s:%d:%d: scope_filter: expected mapping, got %s",
			path, node.Line, node.Column, yamlKind(node))
	}
	for i := 0; i < len(node.Content); i += 2 {
		k := node.Content[i]
		v := node.Content[i+1]
		key := "scope_filter." + k.Value
		switch k.Value {
		case "productive_threshold":
			n, err := requireInt(path, key, v)
			if err != nil {
				return err
			}
			sf.ProductiveThreshold = n
		case "scout_streak_halt":
			b, err := requireBool(path, key, v)
			if err != nil {
				return err
			}
			sf.ScoutStreakHalt = b
		default:
			return fmt.Errorf("%s:%d:%d: unknown key %q under scope_filter",
				path, k.Line, k.Column, k.Value)
		}
	}
	return nil
}

// validatePrefs enforces the schema constraints that depend on the populated
// struct (range checks, enum membership).
func validatePrefs(path string, doc *yaml.Node, prefs *Prefs) error {
	if prefs.SchemaVersion != 1 {
		line, col := locateKey(doc, "schema_version")
		return fmt.Errorf("%s:%d:%d: schema_version: expected 1, got %d",
			path, line, col, prefs.SchemaVersion)
	}
	switch prefs.ModeDefault {
	case "burst", "loop":
	default:
		line, col := locateKey(doc, "mode_default")
		return fmt.Errorf("%s:%d:%d: mode_default: expected one of [burst, loop], got %q",
			path, line, col, prefs.ModeDefault)
	}
	if prefs.ScopeFilter.ProductiveThreshold < 1 || prefs.ScopeFilter.ProductiveThreshold > 100 {
		line, col := locateNestedKey(doc, "scope_filter", "productive_threshold")
		return fmt.Errorf("%s:%d:%d: scope_filter.productive_threshold: expected int in [1..100], got %d",
			path, line, col, prefs.ScopeFilter.ProductiveThreshold)
	}
	return nil
}

// requireInt extracts an integer scalar; returns a typed error otherwise.
func requireInt(path, key string, v *yaml.Node) (int, error) {
	if v.Kind != yaml.ScalarNode || (v.Tag != "" && v.Tag != "!!int") {
		return 0, fmt.Errorf("%s:%d:%d: %s: expected int, got %s %q",
			path, v.Line, v.Column, key, yamlScalarType(v), v.Value)
	}
	var n int
	if err := v.Decode(&n); err != nil {
		return 0, fmt.Errorf("%s:%d:%d: %s: expected int, got %q",
			path, v.Line, v.Column, key, v.Value)
	}
	return n, nil
}

// requireString extracts a string scalar.
func requireString(path, key string, v *yaml.Node) (string, error) {
	if v.Kind != yaml.ScalarNode {
		return "", fmt.Errorf("%s:%d:%d: %s: expected string, got %s",
			path, v.Line, v.Column, key, yamlKind(v))
	}
	if v.Tag != "" && v.Tag != "!!str" {
		return "", fmt.Errorf("%s:%d:%d: %s: expected string, got %s %q",
			path, v.Line, v.Column, key, yamlScalarType(v), v.Value)
	}
	return v.Value, nil
}

// requireBool extracts a bool scalar.
func requireBool(path, key string, v *yaml.Node) (bool, error) {
	if v.Kind != yaml.ScalarNode || (v.Tag != "" && v.Tag != "!!bool") {
		return false, fmt.Errorf("%s:%d:%d: %s: expected bool, got %s %q",
			path, v.Line, v.Column, key, yamlScalarType(v), v.Value)
	}
	var b bool
	if err := v.Decode(&b); err != nil {
		return false, fmt.Errorf("%s:%d:%d: %s: expected bool, got %q",
			path, v.Line, v.Column, key, v.Value)
	}
	return b, nil
}

// requireStringList extracts a sequence of strings.
func requireStringList(path, key string, v *yaml.Node) ([]string, error) {
	if v.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("%s:%d:%d: %s: expected list, got %s",
			path, v.Line, v.Column, key, yamlKind(v))
	}
	out := make([]string, 0, len(v.Content))
	for _, item := range v.Content {
		s, err := requireString(path, key+"[]", item)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, nil
}

// locateKey finds the (line, col) of a top-level key, or the document's
// position if the key isn't present.
func locateKey(doc *yaml.Node, key string) (int, int) {
	if doc.Kind != yaml.MappingNode {
		return doc.Line, doc.Column
	}
	for i := 0; i < len(doc.Content); i += 2 {
		if doc.Content[i].Value == key {
			return doc.Content[i].Line, doc.Content[i].Column
		}
	}
	return doc.Line, doc.Column
}

// locateNestedKey finds (line, col) for a child key under a top-level key.
func locateNestedKey(doc *yaml.Node, parent, child string) (int, int) {
	if doc.Kind != yaml.MappingNode {
		return doc.Line, doc.Column
	}
	for i := 0; i < len(doc.Content); i += 2 {
		if doc.Content[i].Value != parent {
			continue
		}
		v := doc.Content[i+1]
		if v.Kind != yaml.MappingNode {
			return v.Line, v.Column
		}
		for j := 0; j < len(v.Content); j += 2 {
			if v.Content[j].Value == child {
				return v.Content[j].Line, v.Content[j].Column
			}
		}
		return v.Line, v.Column
	}
	return doc.Line, doc.Column
}

// yamlKind returns a human-friendly name for a node kind.
func yamlKind(n *yaml.Node) string {
	switch n.Kind {
	case yaml.DocumentNode:
		return "document"
	case yaml.SequenceNode:
		return "list"
	case yaml.MappingNode:
		return "mapping"
	case yaml.ScalarNode:
		return yamlScalarType(n)
	case yaml.AliasNode:
		return "alias"
	default:
		return "unknown"
	}
}

// yamlScalarType returns the friendly name of a scalar node's tag.
func yamlScalarType(n *yaml.Node) string {
	switch n.Tag {
	case "!!int":
		return "int"
	case "!!bool":
		return "bool"
	case "!!str":
		return "string"
	case "!!float":
		return "float"
	case "!!null":
		return "null"
	case "":
		return "scalar"
	default:
		return n.Tag
	}
}
