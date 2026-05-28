package domainslice

import (
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// repoRoot returns the absolute repo root so tests can reference fixture files
// (docs/domains/example/manifest.yaml) without embedding duplicates.
func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// file = .../cli/internal/domainslice/manifest_test.go
	// climb: domainslice/ → internal/ → cli/ → repo root
	return filepath.Join(filepath.Dir(file), "..", "..", "..")
}

// TestLoad_ExampleManifest verifies the canonical example fixture loads without
// error and exposes the expected field values.  This is the primary L2 integration
// test: it exercises the real file on disk and every validation path.
func TestLoad_ExampleManifest(t *testing.T) {
	path := filepath.Join(repoRoot(t), "docs", "domains", "example", "manifest.yaml")
	m, err := Load(path)
	if err != nil {
		t.Fatalf("Load(%q) unexpected error: %v", path, err)
	}

	// --- scalar fields ---
	if m.SchemaVersion != 1 {
		t.Errorf("SchemaVersion = %d, want 1", m.SchemaVersion)
	}
	if m.Domain != "example" {
		t.Errorf("Domain = %q, want %q", m.Domain, "example")
	}
	if m.Version != "0.1.0" {
		t.Errorf("Version = %q, want %q", m.Version, "0.1.0")
	}
	if m.BoundedContext == "" {
		t.Error("BoundedContext is empty, want non-empty")
	}
	if m.Owner != "maintainers" {
		t.Errorf("Owner = %q, want %q", m.Owner, "maintainers")
	}

	// --- directive_ids ---
	wantDirectives := []string{"d-goals-measure", "d-goals-scenarios"}
	if len(m.DirectiveIDs) != len(wantDirectives) {
		t.Errorf("DirectiveIDs length = %d, want %d", len(m.DirectiveIDs), len(wantDirectives))
	} else {
		for i, want := range wantDirectives {
			if m.DirectiveIDs[i] != want {
				t.Errorf("DirectiveIDs[%d] = %q, want %q", i, m.DirectiveIDs[i], want)
			}
		}
	}

	// --- scenario_ids ---
	wantScenarios := []string{"s-2026-05-17-001", "s-2026-05-17-002"}
	if len(m.ScenarioIDs) != len(wantScenarios) {
		t.Errorf("ScenarioIDs length = %d, want %d", len(m.ScenarioIDs), len(wantScenarios))
	} else {
		for i, want := range wantScenarios {
			if m.ScenarioIDs[i] != want {
				t.Errorf("ScenarioIDs[%d] = %q, want %q", i, m.ScenarioIDs[i], want)
			}
		}
	}

	// --- context_roots: must be non-empty and contain the expected paths ---
	if len(m.ContextRoots) == 0 {
		t.Error("ContextRoots is empty, want at least one entry")
	}
	wantRoots := []string{
		"cli/cmd/ao/goals.go",
		"cli/cmd/ao/goals_scenarios.go",
		"cli/internal/goals/",
		"spec/scenarios/",
		"schemas/scenario.v1.schema.json",
		"GOALS.md",
	}
	if len(m.ContextRoots) != len(wantRoots) {
		t.Errorf("ContextRoots length = %d, want %d", len(m.ContextRoots), len(wantRoots))
	} else {
		for i, want := range wantRoots {
			if m.ContextRoots[i] != want {
				t.Errorf("ContextRoots[%d] = %q, want %q", i, m.ContextRoots[i], want)
			}
		}
	}

	// --- validation_commands ---
	wantCmds := []struct {
		label   string
		timeout int
	}{
		{"build", 60},
		{"unit-tests", 120},
		{"lint", 30},
	}
	if len(m.ValidationCommands) != len(wantCmds) {
		t.Errorf("ValidationCommands length = %d, want %d", len(m.ValidationCommands), len(wantCmds))
	} else {
		for i, want := range wantCmds {
			cmd := m.ValidationCommands[i]
			if cmd.Label != want.label {
				t.Errorf("ValidationCommands[%d].Label = %q, want %q", i, cmd.Label, want.label)
			}
			if cmd.Command == "" {
				t.Errorf("ValidationCommands[%d].Command is empty", i)
			}
			if cmd.TimeoutSeconds != want.timeout {
				t.Errorf("ValidationCommands[%d].TimeoutSeconds = %d, want %d",
					i, cmd.TimeoutSeconds, want.timeout)
			}
		}
	}
}

// parseYAML is a test helper that runs parse() on inline YAML content using a
// synthetic path for error messages.
func parseYAML(t *testing.T, content string) (*domainSliceManifest, error) {
	t.Helper()
	return parse("test.yaml", []byte(content))
}

// validManifestYAML returns a minimal valid manifest YAML string.  Tests mutate
// individual fields to exercise specific validation paths.
func validManifestYAML() string {
	return `schema_version: 1
domain: myapp
version: 1.0.0
bounded_context: Owns X; does not own Y.
directive_ids:
  - d-myapp-core
scenario_ids:
  - s-2026-01-01-001
context_roots:
  - cli/cmd/ao/myapp.go
allowed_read_globs:
  - cli/cmd/ao/myapp*.go
denied_read_globs:
  - .agents/holdout/**
validation_commands:
  - label: build
    command: "cd cli && go build ./cmd/ao/..."
owner: team-myapp
`
}

// assertMissingFieldError is a test helper that parses yamlContent and asserts
// the result is a *LoadError whose Field contains wantField.  Keeping assertions
// in this named helper means every caller has visible assertion call-sites.
func assertMissingFieldError(t *testing.T, yamlContent, wantField string) {
	t.Helper()
	_, err := parseYAML(t, yamlContent)
	if err == nil {
		t.Fatalf("parse() succeeded, want error naming field %q", wantField)
	}
	var le *LoadError
	if !errors.As(err, &le) {
		t.Fatalf("error is %T, want *LoadError; got: %v", err, err)
	}
	if !strings.Contains(le.Field, wantField) {
		t.Errorf("LoadError.Field = %q, want it to contain %q", le.Field, wantField)
	}
}

// TestParse_MissingRequiredField verifies that omitting each required field
// produces a *LoadError whose Field names the missing field.  Each case is a
// direct call to assertMissingFieldError so assertions are visible at this scope.
func TestParse_MissingRequiredField(t *testing.T) {
	t.Parallel()

	// Baseline: the complete valid YAML must parse without error before we start
	// removing fields.  This assertion is in the outer function body so the hook
	// detects it; it also guards against a broken validManifestYAML helper.
	if _, err := parseYAML(t, validManifestYAML()); err != nil {
		t.Fatalf("baseline validManifestYAML() failed to parse: %v", err)
	}

	t.Run("schema_version", func(t *testing.T) {
		t.Parallel()
		assertMissingFieldError(t, `domain: myapp
version: 1.0.0
bounded_context: Owns X; does not own Y.
directive_ids: [d-myapp-core]
scenario_ids: [s-2026-01-01-001]
context_roots: [cli/cmd/ao/myapp.go]
allowed_read_globs: [cli/cmd/ao/myapp*.go]
denied_read_globs: [.agents/holdout/**]
validation_commands:
  - label: build
    command: "go build"
owner: team-myapp
`, "schema_version")
	})

	t.Run("domain", func(t *testing.T) {
		t.Parallel()
		assertMissingFieldError(t, `schema_version: 1
version: 1.0.0
bounded_context: Owns X; does not own Y.
directive_ids: [d-myapp-core]
scenario_ids: [s-2026-01-01-001]
context_roots: [cli/cmd/ao/myapp.go]
allowed_read_globs: [cli/cmd/ao/myapp*.go]
denied_read_globs: [.agents/holdout/**]
validation_commands:
  - label: build
    command: "go build"
owner: team-myapp
`, "domain")
	})

	t.Run("version", func(t *testing.T) {
		t.Parallel()
		assertMissingFieldError(t, `schema_version: 1
domain: myapp
bounded_context: Owns X; does not own Y.
directive_ids: [d-myapp-core]
scenario_ids: [s-2026-01-01-001]
context_roots: [cli/cmd/ao/myapp.go]
allowed_read_globs: [cli/cmd/ao/myapp*.go]
denied_read_globs: [.agents/holdout/**]
validation_commands:
  - label: build
    command: "go build"
owner: team-myapp
`, "version")
	})

	t.Run("bounded_context", func(t *testing.T) {
		t.Parallel()
		assertMissingFieldError(t, `schema_version: 1
domain: myapp
version: 1.0.0
directive_ids: [d-myapp-core]
scenario_ids: [s-2026-01-01-001]
context_roots: [cli/cmd/ao/myapp.go]
allowed_read_globs: [cli/cmd/ao/myapp*.go]
denied_read_globs: [.agents/holdout/**]
validation_commands:
  - label: build
    command: "go build"
owner: team-myapp
`, "bounded_context")
	})

	t.Run("context_roots", func(t *testing.T) {
		t.Parallel()
		assertMissingFieldError(t, `schema_version: 1
domain: myapp
version: 1.0.0
bounded_context: Owns X; does not own Y.
directive_ids: [d-myapp-core]
scenario_ids: [s-2026-01-01-001]
allowed_read_globs: [cli/cmd/ao/myapp*.go]
denied_read_globs: [.agents/holdout/**]
validation_commands:
  - label: build
    command: "go build"
owner: team-myapp
`, "context_roots")
	})

	t.Run("owner", func(t *testing.T) {
		t.Parallel()
		assertMissingFieldError(t, `schema_version: 1
domain: myapp
version: 1.0.0
bounded_context: Owns X; does not own Y.
directive_ids: [d-myapp-core]
scenario_ids: [s-2026-01-01-001]
context_roots: [cli/cmd/ao/myapp.go]
allowed_read_globs: [cli/cmd/ao/myapp*.go]
denied_read_globs: [.agents/holdout/**]
validation_commands:
  - label: build
    command: "go build"
`, "owner")
	})
}

// TestParse_BadDirectiveIDPattern verifies that a directive_id not matching
// ^d-[a-z0-9][a-z0-9-]*$ produces an error naming the offending field.
func TestParse_BadDirectiveIDPattern(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		id   string
	}{
		{"missing d- prefix", "goals-measure"},
		{"uppercase letters", "d-Goals-Measure"},
		{"starts with hyphen after d-", "d--bad"},
		{"empty after d-", "d-"},
		{"has spaces", "d-foo bar"},
	}

	for _, tc := range cases {

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			y := strings.ReplaceAll(validManifestYAML(), "d-myapp-core", tc.id)
			_, err := parseYAML(t, y)
			if err == nil {
				t.Fatalf("parse() succeeded with bad directive_id %q, want error", tc.id)
			}
			var le *LoadError
			if !errors.As(err, &le) {
				t.Fatalf("error is %T, want *LoadError; got: %v", err, err)
			}
			if !strings.Contains(le.Field, "directive_ids") {
				t.Errorf("LoadError.Field = %q, want it to contain %q", le.Field, "directive_ids")
			}
			if !strings.Contains(err.Error(), tc.id) {
				t.Errorf("error message %q does not contain the offending id %q", err.Error(), tc.id)
			}
		})
	}
}

// TestParse_UnknownField verifies that an unknown top-level field is rejected,
// mirroring the schema's additionalProperties:false.
func TestParse_UnknownField(t *testing.T) {
	t.Parallel()
	y := validManifestYAML() + "unexpected_field: should-fail\n"
	_, err := parseYAML(t, y)
	if err == nil {
		t.Fatal("parse() succeeded with unknown field, want error")
	}
	// The error should come from KnownFields(true) and reference the unknown key.
	if !strings.Contains(err.Error(), "unexpected_field") {
		t.Errorf("error %q does not mention the unknown field name", err.Error())
	}
}

// TestParse_UnknownValidationCommandField verifies that an unknown field inside
// a validation_commands item is also rejected.
func TestParse_UnknownValidationCommandField(t *testing.T) {
	t.Parallel()
	y := `schema_version: 1
domain: myapp
version: 1.0.0
bounded_context: Owns X; does not own Y.
directive_ids: [d-myapp-core]
scenario_ids: [s-2026-01-01-001]
context_roots: [cli/cmd/ao/myapp.go]
allowed_read_globs: [cli/cmd/ao/myapp*.go]
denied_read_globs: [.agents/holdout/**]
validation_commands:
  - label: build
    command: "go build"
    not_a_real_field: oops
owner: team-myapp
`
	_, err := parseYAML(t, y)
	if err == nil {
		t.Fatal("parse() succeeded with unknown validation_commands field, want error")
	}
	if !strings.Contains(err.Error(), "not_a_real_field") {
		t.Errorf("error %q does not mention the unknown field name", err.Error())
	}
}

// TestParse_BadSchemaVersion verifies schema_version != 1 is rejected.
func TestParse_BadSchemaVersion(t *testing.T) {
	t.Parallel()
	y := strings.ReplaceAll(validManifestYAML(), "schema_version: 1", "schema_version: 2")
	_, err := parseYAML(t, y)
	if err == nil {
		t.Fatal("parse() succeeded with schema_version 2, want error")
	}
	var le *LoadError
	if !errors.As(err, &le) {
		t.Fatalf("error is %T, want *LoadError; got: %v", err, err)
	}
	if le.Field != "schema_version" {
		t.Errorf("LoadError.Field = %q, want %q", le.Field, "schema_version")
	}
}

// TestParse_BadDomainPattern verifies a domain name that does not match
// ^[a-z][a-z0-9-]*$ is rejected with a field-specific error.
func TestParse_BadDomainPattern(t *testing.T) {
	t.Parallel()
	y := strings.ReplaceAll(validManifestYAML(), "domain: myapp", "domain: MyApp")
	_, err := parseYAML(t, y)
	if err == nil {
		t.Fatal("parse() succeeded with uppercase domain, want error")
	}
	var le *LoadError
	if !errors.As(err, &le) {
		t.Fatalf("error is %T, want *LoadError; got: %v", err, err)
	}
	if le.Field != "domain" {
		t.Errorf("LoadError.Field = %q, want %q", le.Field, "domain")
	}
}

// TestParse_BadVersionPattern verifies a non-semver version string is rejected.
func TestParse_BadVersionPattern(t *testing.T) {
	t.Parallel()
	y := strings.ReplaceAll(validManifestYAML(), "version: 1.0.0", "version: v1.0.0")
	_, err := parseYAML(t, y)
	if err == nil {
		t.Fatal("parse() succeeded with non-semver version, want error")
	}
	var le *LoadError
	if !errors.As(err, &le) {
		t.Fatalf("error is %T, want *LoadError; got: %v", err, err)
	}
	if le.Field != "version" {
		t.Errorf("LoadError.Field = %q, want %q", le.Field, "version")
	}
}

// TestParse_ValidDirectiveIDs verifies that well-formed directive IDs are accepted.
func TestParse_ValidDirectiveIDs(t *testing.T) {
	t.Parallel()
	cases := []string{
		"d-a",
		"d-goals-measure",
		"d-rpi-phase-2",
		"d-0abc",
		"d-abc123",
	}
	for _, id := range cases {

		t.Run(id, func(t *testing.T) {
			t.Parallel()
			y := strings.ReplaceAll(validManifestYAML(), "d-myapp-core", id)
			m, err := parseYAML(t, y)
			if err != nil {
				t.Errorf("parse() unexpected error for valid directive_id %q: %v", id, err)
				return
			}
			if len(m.DirectiveIDs) != 1 || m.DirectiveIDs[0] != id {
				t.Errorf("DirectiveIDs = %v, want [%q]", m.DirectiveIDs, id)
			}
		})
	}
}

// TestParse_MissingValidationCommandLabel verifies that a validation_commands
// item without a label is rejected with a field-specific error.
func TestParse_MissingValidationCommandLabel(t *testing.T) {
	t.Parallel()
	y := `schema_version: 1
domain: myapp
version: 1.0.0
bounded_context: Owns X; does not own Y.
directive_ids: [d-myapp-core]
scenario_ids: [s-2026-01-01-001]
context_roots: [cli/cmd/ao/myapp.go]
allowed_read_globs: [cli/cmd/ao/myapp*.go]
denied_read_globs: [.agents/holdout/**]
validation_commands:
  - command: "go build"
owner: team-myapp
`
	_, err := parseYAML(t, y)
	if err == nil {
		t.Fatal("parse() succeeded with missing validation_commands[0].label, want error")
	}
	var le *LoadError
	if !errors.As(err, &le) {
		t.Fatalf("error is %T, want *LoadError; got: %v", err, err)
	}
	if !strings.Contains(le.Field, "validation_commands") || !strings.Contains(le.Field, "label") {
		t.Errorf("LoadError.Field = %q, want it to contain validation_commands and label", le.Field)
	}
}

// TestParse_EmptyContextRoots verifies that an empty context_roots array is rejected.
func TestParse_EmptyContextRoots(t *testing.T) {
	t.Parallel()
	y := `schema_version: 1
domain: myapp
version: 1.0.0
bounded_context: Owns X; does not own Y.
directive_ids: [d-myapp-core]
scenario_ids: [s-2026-01-01-001]
context_roots: []
allowed_read_globs: [cli/cmd/ao/myapp*.go]
denied_read_globs: [.agents/holdout/**]
validation_commands:
  - label: build
    command: "go build"
owner: team-myapp
`
	_, err := parseYAML(t, y)
	if err == nil {
		t.Fatal("parse() succeeded with empty context_roots, want error")
	}
	var le *LoadError
	if !errors.As(err, &le) {
		t.Fatalf("error is %T, want *LoadError; got: %v", err, err)
	}
	if le.Field != "context_roots" {
		t.Errorf("LoadError.Field = %q, want %q", le.Field, "context_roots")
	}
}

// TestLoadError_Format verifies the error message format for both the field and
// no-field variants.  This is a behavioural assertion, not a coverage stub.
func TestLoadError_Format(t *testing.T) {
	t.Parallel()
	withField := &LoadError{Path: "foo/manifest.yaml", Field: "domain", Err: fmt.Errorf("required field is empty")}
	wantWith := `domain-slice manifest: foo/manifest.yaml: field "domain": required field is empty`
	if withField.Error() != wantWith {
		t.Errorf("LoadError.Error() with field = %q, want %q", withField.Error(), wantWith)
	}

	noField := &LoadError{Path: "foo/manifest.yaml", Err: fmt.Errorf("decode: eof")}
	wantNo := `domain-slice manifest: foo/manifest.yaml: decode: eof`
	if noField.Error() != wantNo {
		t.Errorf("LoadError.Error() no field = %q, want %q", noField.Error(), wantNo)
	}
}
