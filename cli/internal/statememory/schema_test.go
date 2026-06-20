package statememory

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

func TestStateSchemasRejectBadFixtures(t *testing.T) {
	root := repoRoot(t)

	findingSchema := compileSchemaForTest(t, filepath.Join(root, "schemas", "state-finding.v1.schema.json"))
	admissionSchema := compileSchemaForTest(t, filepath.Join(root, "schemas", "state-admission.v1.schema.json"))

	fixtures := filepath.Join(root, "schemas", "fixtures", "state-memory")
	validateFixture(t, findingSchema, filepath.Join(fixtures, "valid-finding.json"), true)
	validateFixture(t, admissionSchema, filepath.Join(fixtures, "valid-admission.json"), true)

	badFixtures := map[string]*jsonschema.Schema{
		"bad-finding-missing-review.json": findingSchema,
		"bad-finding-extra-field.json":    findingSchema,
		"bad-admission-path-escape.json":  admissionSchema,
	}
	for name, schema := range badFixtures {
		validateFixture(t, schema, filepath.Join(fixtures, name), false)
	}
}

func TestValidateStateFileSelectsSchemaByKind(t *testing.T) {
	root := repoRoot(t)
	valid := filepath.Join(root, "schemas", "fixtures", "state-memory", "valid-finding.json")
	if err := ValidateStateFile(root, valid); err != nil {
		t.Fatalf("ValidateStateFile(valid finding): %v", err)
	}

	bad := filepath.Join(root, "schemas", "fixtures", "state-memory", "bad-finding-extra-field.json")
	if err := ValidateStateFile(root, bad); err == nil {
		t.Fatal("ValidateStateFile accepted fixture with additional property")
	}
}

func TestValidateStateFileRejectsUnknownKind(t *testing.T) {
	root := repoRoot(t)
	tmp := t.TempDir()
	path := filepath.Join(tmp, "unknown.json")
	if err := os.WriteFile(path, []byte(`{"schema_version":1,"kind":"unknown"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	err := ValidateStateFile(root, path)
	if err == nil {
		t.Fatal("ValidateStateFile accepted unknown kind")
	}
	if !strings.Contains(err.Error(), "unknown state kind") {
		t.Fatalf("error = %v, want unknown state kind", err)
	}
}

func compileSchemaForTest(t *testing.T, path string) *jsonschema.Schema {
	t.Helper()
	schema, err := CompileSchemaFile(path)
	if err != nil {
		t.Fatalf("compile schema %s: %v", path, err)
	}
	return schema
}

func validateFixture(t *testing.T, schema *jsonschema.Schema, path string, wantValid bool) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	err = ValidateJSON(schema, data)
	if wantValid && err != nil {
		t.Fatalf("%s should validate: %v", filepath.Base(path), err)
	}
	if !wantValid && err == nil {
		t.Fatalf("%s should be rejected by schema", filepath.Base(path))
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "AGENTS.md")); err == nil {
			if _, err := os.Stat(filepath.Join(dir, "schemas")); err == nil {
				return dir
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("repo root not found")
		}
		dir = parent
	}
}
