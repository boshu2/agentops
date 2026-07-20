package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestOutputYAMLTruthfulness is the acceptance probe for age-6j9ee.2: `-o yaml`
// is advertised uniformly (capabilities output_formats [table, json, yaml], the
// --output flag help, robot-docs), so it must be REAL everywhere `-o json` is
// real. This probe enumerates every read-side command that emits structured
// output and asserts, for each, that `-o yaml`:
//
//   - parses as YAML (a human table does not), AND
//   - is semantically equal to the `-o json` output (same decoded tree), AND
//   - is not byte-identical to the `-o table` human view (the silent-fallback
//     signature this bead exists to kill).
//
// RED (pre-fix, HEAD 11e25f93f): status, version, doctor, goals, provenance,
// session, and skills had no yaml branch, so `-o yaml <cmd>` silently returned
// the human table — yaml.Unmarshal of that table does not reproduce the json
// tree, so every such row FAILS. GREEN (this commit): each command routes
// `-o yaml` through clicontract.WriteYAML (or JSONToYAML), so every row passes.
//
// Determinism guard: a few commands embed now-derived leaf values (e.g. an
// average-age metric). For those the probe compares tree SHAPE (keys + value
// types, recursively) instead of exact leaf values, which still catches a table
// fallback and a missing-key divergence (the flywheel-status yaml that used to
// omit "metrics") while staying immune to sub-second float drift. For every
// deterministic command it compares full values.
func TestOutputYAMLTruthfulness(t *testing.T) {
	// Structured read-side commands. Each is invocable with no required file
	// argument and resolves its repository content by walking up from the test
	// working directory, so it emits a single structured document in this repo.
	cases := []struct {
		name string
		args []string
	}{
		{"capabilities", []string{"capabilities"}},
		{"version", []string{"version"}},
		{"status", []string{"status"}},
		{"doctor", []string{"doctor"}},
		{"goals history", []string{"goals", "history"}},
		{"provenance list", []string{"provenance", "list"}},
		{"provenance position", []string{"provenance", "position"}},
		{"provenance verify", []string{"provenance", "verify"}},
		{"provenance export", []string{"provenance", "export"}},
		{"session bootstrap", []string{"session", "bootstrap"}},
		{"session rehydrate", []string{"session", "rehydrate"}},
		{"flywheel status", []string{"flywheel", "status"}},
		{"flywheel compare", []string{"flywheel", "compare"}},
		{"skills check", []string{"skills", "check"}},
		{"skills list", []string{"skills", "list"}},
		{"skills resolve", []string{"skills", "resolve"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			jsonOut, _ := executeCommand(withOutput("json", tc.args)...)
			var jsonTree any
			if err := json.Unmarshal([]byte(jsonOut), &jsonTree); err != nil {
				// -o json did not emit a single JSON document in this environment
				// (e.g. the command is not applicable to this repository state).
				// There is nothing to hold -o yaml against, so skip — but never
				// treat this as a pass for the yaml contract.
				t.Skipf("%s: -o json did not emit a single JSON document here (%v); nothing to compare", tc.name, err)
				return
			}

			yamlOut, _ := executeCommand(withOutput("yaml", tc.args)...)
			var yamlTree any
			if err := yaml.Unmarshal([]byte(yamlOut), &yamlTree); err != nil {
				t.Fatalf("%s: -o yaml did not parse as YAML — silent table fallback?\nerr=%v\noutput:\n%s", tc.name, err, yamlOut)
			}

			// A YAML document that decodes to a bare string is the table text
			// masquerading as YAML; the structured commands here decode to a map
			// or a sequence.
			switch yamlTree.(type) {
			case map[string]any, []any:
				// structured — good
			default:
				t.Fatalf("%s: -o yaml decoded to %T, not a mapping/sequence — silent table fallback?\noutput:\n%s", tc.name, yamlTree, yamlOut)
			}

			// Second json run to detect now-derived (non-deterministic) leaves.
			jsonOut2, _ := executeCommand(withOutput("json", tc.args)...)
			var jsonTree2 any
			_ = json.Unmarshal([]byte(jsonOut2), &jsonTree2)

			if canonical(t, jsonTree) == canonical(t, jsonTree2) {
				// Deterministic: yaml must equal json exactly (same tree).
				if got, want := canonical(t, yamlTree), canonical(t, jsonTree); got != want {
					t.Fatalf("%s: -o yaml tree != -o json tree\nyaml: %s\njson: %s", tc.name, got, want)
				}
			} else {
				// Non-deterministic leaves: compare tree SHAPE (keys + types).
				if got, want := canonical(t, shapeOf(yamlTree)), canonical(t, shapeOf(jsonTree)); got != want {
					t.Fatalf("%s: -o yaml shape != -o json shape (missing key or table fallback)\nyaml: %s\njson: %s", tc.name, got, want)
				}
			}

			// Belt-and-suspenders: -o yaml must not be the human table verbatim.
			tableOut, _ := executeCommand(withOutput("table", tc.args)...)
			if strings.TrimSpace(tableOut) != "" && strings.TrimSpace(yamlOut) == strings.TrimSpace(tableOut) {
				t.Fatalf("%s: -o yaml is byte-identical to -o table (silent fallback)", tc.name)
			}
		})
	}
}

// withOutput prepends the global -o <mode> flag to a command's args.
func withOutput(mode string, args []string) []string {
	return append([]string{"-o", mode}, args...)
}

// canonical marshals a decoded tree to canonical JSON (sorted map keys, number
// types normalized), so a YAML-decoded tree and a JSON-decoded tree of the same
// data compare equal regardless of decoder-specific int/float representation.
func canonical(t *testing.T, v any) string {
	t.Helper()
	data, err := json.Marshal(normalizeForJSON(v))
	if err != nil {
		t.Fatalf("canonicalize: %v", err)
	}
	return string(data)
}

// normalizeForJSON rewrites yaml.v3's map[any]any (possible for non-string keys)
// into map[string]any so encoding/json can marshal it. The structured surfaces
// here use string keys, but this keeps the probe robust.
func normalizeForJSON(v any) any {
	switch val := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(val))
		for k, item := range val {
			out[k] = normalizeForJSON(item)
		}
		return out
	case map[any]any:
		out := make(map[string]any, len(val))
		for k, item := range val {
			out[fmt.Sprint(k)] = normalizeForJSON(item)
		}
		return out
	case []any:
		out := make([]any, len(val))
		for i, item := range val {
			out[i] = normalizeForJSON(item)
		}
		return out
	default:
		return val
	}
}

// shapeOf replaces every scalar leaf with its type name, preserving map keys and
// sequence structure. Two trees with the same keys and value types but different
// leaf values (e.g. a now-derived metric) share a shape.
func shapeOf(v any) any {
	switch val := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(val))
		for k, item := range val {
			out[k] = shapeOf(item)
		}
		return out
	case map[any]any:
		out := make(map[string]any, len(val))
		for k, item := range val {
			out[fmt.Sprint(k)] = shapeOf(item)
		}
		return out
	case []any:
		out := make([]any, len(val))
		for i, item := range val {
			out[i] = shapeOf(item)
		}
		return out
	case nil:
		return "null"
	case bool:
		return "bool"
	case string:
		return "string"
	default:
		// json.Number, float64, int, int64, uint64 — all numeric leaves.
		return "number"
	}
}

// TestOutputYAMLProbeStalenessGuard fails when a new top-level command appears
// that the yaml-truthfulness probe has neither exercised nor explicitly excused.
// It is a coarse staleness net: it forces whoever adds a command to decide
// whether it emits structured output (add it to TestOutputYAMLTruthfulness) or
// not (add it to the excused set below with a reason). Subcommand-level coverage
// (e.g. provenance list vs position) is the probe's own concern.
func TestOutputYAMLProbeStalenessGuard(t *testing.T) {
	// Top-level commands the yaml probe exercises (via at least one subcommand
	// for the families).
	probed := map[string]bool{
		"capabilities": true,
		"version":      true,
		"status":       true,
		"doctor":       true,
		"goals":        true,
		"provenance":   true,
		"session":      true,
		"flywheel":     true,
		"skills":       true,
		"eval":         true, // yaml wired at every eval json site; probed by eval package tests (needs suite fixtures)
	}
	// Commands with no structured output, so -o yaml/-o json are not applicable
	// (they emit human text or are pure side-effect verbs). Each ignores -o
	// consistently — no silent yaml→table divergence to assert.
	excused := map[string]string{
		"config":      "structured show/models covered by config package tests; top-level config prints help",
		"demo":        "human-only demonstrative output",
		"gate":        "human/exit-code gate; no structured document contract",
		"init":        "side-effect scaffolder, human output",
		"quick-start": "human onboarding text",
		"redact":      "stream transform, not a structured document",
		"robot-docs":  "markdown handbook, not json/yaml",
		"completion":  "shell completion script (cobra builtin)",
		"help":        "cobra builtin help",
	}

	var unclassified []string
	for _, cmd := range rootCmd.Commands() {
		name := cmd.Name()
		if probed[name] || excused[name] != "" {
			continue
		}
		unclassified = append(unclassified, name)
	}
	sort.Strings(unclassified)
	if len(unclassified) > 0 {
		t.Fatalf("top-level command(s) %v are neither probed by TestOutputYAMLTruthfulness "+
			"nor excused in TestOutputYAMLProbeStalenessGuard — classify each: if it emits "+
			"structured output, add it to the probe; if not, excuse it with a reason.", unclassified)
	}
}
