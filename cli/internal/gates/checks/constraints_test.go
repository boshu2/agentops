package checks

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/boshu2/agentops/cli/internal/gates"
	"github.com/boshu2/agentops/cli/internal/ports"
)

// writeConstraintFixture builds an isolated repo root with a constraints index
// (raw JSON, so tests can write malformed records too) plus optional target
// files, and returns a RunContext whose ChangedFiles are the given repo-relative
// paths.
func writeConstraintFixture(t *testing.T, indexJSON string, files map[string]string, changed []string) gates.RunContext {
	t.Helper()
	root := t.TempDir()
	if indexJSON != "" {
		dir := filepath.Join(root, ".agents", "constraints")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir constraints: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, "index.json"), []byte(indexJSON), 0o644); err != nil {
			t.Fatalf("write index: %v", err)
		}
	}
	for rel, content := range files {
		abs := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatalf("mkdir for %s: %v", rel, err)
		}
		if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	return gates.RunContext{RepoRoot: root, ChangedFiles: changed, Mode: gates.Fast}
}

func runConstraintGate(t *testing.T, rc gates.RunContext) ports.GateVerdict {
	t.Helper()
	v, err := runConstraintEnforceGate(context.Background(), rc)
	if err != nil {
		t.Fatalf("runConstraintEnforceGate returned error (should encode outcome in verdict, not err): %v", err)
	}
	return v
}

const forbidConstraint = `{
  "schema_version": 1,
  "constraints": [
    {
      "id": "f-test-001",
      "title": "no panic in cli",
      "status": "active",
      "compiled_at": "2026-06-21T00:00:00Z",
      "applies_to": {"path_globs": ["cli/**"]},
      "detector": {"kind": "regex", "mode": "match", "pattern": "panic\\(", "message": "no panic() in cli"}
    }
  ]
}`

func TestConstraintGate_NoIndex_Passes(t *testing.T) {
	rc := writeConstraintFixture(t, "", nil, []string{"cli/internal/foo.go"})
	v := runConstraintGate(t, rc)
	if v.Status != ports.GateStatusPass {
		t.Fatalf("no index should PASS (nothing to enforce), got %s: %s", v.Status, v.Reason)
	}
}

func TestConstraintGate_EmptyIndex_Passes(t *testing.T) {
	rc := writeConstraintFixture(t, `{"schema_version":1,"constraints":[]}`, nil, []string{"cli/internal/foo.go"})
	v := runConstraintGate(t, rc)
	if v.Status != ports.GateStatusPass {
		t.Fatalf("empty index should PASS, got %s: %s", v.Status, v.Reason)
	}
}

func TestConstraintGate_Violation_Fails(t *testing.T) {
	rc := writeConstraintFixture(t, forbidConstraint,
		map[string]string{"cli/internal/foo.go": "func x() {\n\tpanic(\"boom\")\n}\n"},
		[]string{"cli/internal/foo.go"})
	v := runConstraintGate(t, rc)
	if v.Status != ports.GateStatusFail {
		t.Fatalf("a changed file re-introducing the forbidden pattern must FAIL, got %s: %s", v.Status, v.Reason)
	}
}

func TestConstraintGate_SafeChangeOnMatchingPath_Passes(t *testing.T) {
	rc := writeConstraintFixture(t, forbidConstraint,
		map[string]string{"cli/internal/foo.go": "func x() {\n\treturn\n}\n"},
		[]string{"cli/internal/foo.go"})
	v := runConstraintGate(t, rc)
	if v.Status != ports.GateStatusPass {
		t.Fatalf("a safe change on a matching path must PASS, got %s: %s", v.Status, v.Reason)
	}
}

func TestConstraintGate_NonMatchingPath_Passes(t *testing.T) {
	// The violating content exists but the changed file is outside path_globs.
	rc := writeConstraintFixture(t, forbidConstraint,
		map[string]string{"docs/foo.md": "panic(\"boom\")"},
		[]string{"docs/foo.md"})
	v := runConstraintGate(t, rc)
	if v.Status != ports.GateStatusPass {
		t.Fatalf("a constraint whose path_globs don't match the change must not fire, got %s: %s", v.Status, v.Reason)
	}
}

func TestConstraintGate_MalformedIndex_FailsClosed(t *testing.T) {
	rc := writeConstraintFixture(t, `{"schema_version":1,"constraints":[ this is not json `,
		nil, []string{"cli/internal/foo.go"})
	v := runConstraintGate(t, rc)
	if v.Status != ports.GateStatusFail {
		t.Fatalf("a malformed index must FAIL CLOSED (parse error = gate fail, never skip), got %s: %s", v.Status, v.Reason)
	}
}

func TestConstraintGate_ActiveConstraintEmptyGlobs_FailsClosed(t *testing.T) {
	idx := `{"schema_version":1,"constraints":[{"id":"f-x","title":"t","status":"active","compiled_at":"2026-06-21T00:00:00Z","applies_to":{},"detector":{"kind":"regex","pattern":"x"}}]}`
	rc := writeConstraintFixture(t, idx, map[string]string{"cli/a.go": "x"}, []string{"cli/a.go"})
	v := runConstraintGate(t, rc)
	if v.Status != ports.GateStatusFail {
		t.Fatalf("an active constraint with no path_globs is malformed -> FAIL CLOSED, got %s: %s", v.Status, v.Reason)
	}
}

func TestConstraintGate_ActiveConstraintBadRegex_FailsClosed(t *testing.T) {
	idx := `{"schema_version":1,"constraints":[{"id":"f-x","title":"t","status":"active","compiled_at":"2026-06-21T00:00:00Z","applies_to":{"path_globs":["cli/**"]},"detector":{"kind":"regex","pattern":"("}}]}`
	rc := writeConstraintFixture(t, idx, map[string]string{"cli/a.go": "anything"}, []string{"cli/a.go"})
	v := runConstraintGate(t, rc)
	if v.Status != ports.GateStatusFail {
		t.Fatalf("an active constraint with an uncompilable regex is malformed -> FAIL CLOSED, got %s: %s", v.Status, v.Reason)
	}
}

func TestConstraintGate_UnsupportedKind_FailsClosed(t *testing.T) {
	idx := `{"schema_version":1,"constraints":[{"id":"f-x","title":"t","status":"active","compiled_at":"2026-06-21T00:00:00Z","applies_to":{"path_globs":["cli/**"]},"detector":{"kind":"command","command":"grep -q x"}}]}`
	rc := writeConstraintFixture(t, idx, map[string]string{"cli/a.go": "x"}, []string{"cli/a.go"})
	v := runConstraintGate(t, rc)
	if v.Status != ports.GateStatusFail {
		t.Fatalf("an active constraint with an unsupported detector kind must FAIL CLOSED (never silently skip an active constraint), got %s: %s", v.Status, v.Reason)
	}
}

func TestConstraintGate_ActiveDetectorMissingKind_FailsClosed(t *testing.T) {
	idx := `{"schema_version":1,"constraints":[{"id":"f","title":"t","status":"active","compiled_at":"x","applies_to":{"path_globs":["cli/**"]},"detector":{"mode":"match","pattern":"panic"}}]}`
	rc := writeConstraintFixture(t, idx, map[string]string{"cli/a.go": "safe"}, []string{"cli/a.go"})
	v := runConstraintGate(t, rc)
	if v.Status != ports.GateStatusFail {
		t.Fatalf("an active detector with a blank kind is ambiguous and must FAIL CLOSED, got %s: %s", v.Status, v.Reason)
	}
}

func TestConstraintGate_DraftConstraint_NotEnforced(t *testing.T) {
	idx := `{"schema_version":1,"constraints":[{"id":"f-x","title":"t","status":"draft","compiled_at":"2026-06-21T00:00:00Z","applies_to":{"path_globs":["cli/**"]},"detector":{"kind":"regex","mode":"match","pattern":"panic\\("}}]}`
	rc := writeConstraintFixture(t, idx, map[string]string{"cli/a.go": "panic(\"x\")"}, []string{"cli/a.go"})
	v := runConstraintGate(t, rc)
	if v.Status != ports.GateStatusPass {
		t.Fatalf("a draft (non-active) constraint must not be enforced, got %s: %s", v.Status, v.Reason)
	}
}

func TestConstraintGate_ExcludeSuppressesViolation(t *testing.T) {
	idx := `{"schema_version":1,"constraints":[{"id":"f-x","title":"t","status":"active","compiled_at":"2026-06-21T00:00:00Z","applies_to":{"path_globs":["cli/**"]},"detector":{"kind":"regex","mode":"match","pattern":"panic\\(","exclude":"//nolint"}}]}`
	rc := writeConstraintFixture(t, idx,
		map[string]string{"cli/a.go": "\tpanic(\"x\") //nolint\n"},
		[]string{"cli/a.go"})
	v := runConstraintGate(t, rc)
	if v.Status != ports.GateStatusPass {
		t.Fatalf("a match on an excluded line must not be a violation, got %s: %s", v.Status, v.Reason)
	}
}

func TestConstraintGate_AbsentMode_RequiredPatternMissing_Fails(t *testing.T) {
	idx := `{"schema_version":1,"constraints":[{"id":"f-x","title":"license header","status":"active","compiled_at":"2026-06-21T00:00:00Z","applies_to":{"path_globs":["cli/**"]},"detector":{"kind":"regex","mode":"absent","pattern":"SPDX-License-Identifier","message":"missing license header"}}]}`
	rc := writeConstraintFixture(t, idx, map[string]string{"cli/a.go": "package main\n"}, []string{"cli/a.go"})
	v := runConstraintGate(t, rc)
	if v.Status != ports.GateStatusFail {
		t.Fatalf("absent-mode: required pattern missing must FAIL, got %s: %s", v.Status, v.Reason)
	}
}

func TestConstraintGate_AbsentMode_RequiredPatternPresent_Passes(t *testing.T) {
	idx := `{"schema_version":1,"constraints":[{"id":"f-x","title":"license header","status":"active","compiled_at":"2026-06-21T00:00:00Z","applies_to":{"path_globs":["cli/**"]},"detector":{"kind":"regex","mode":"absent","pattern":"SPDX-License-Identifier"}}]}`
	rc := writeConstraintFixture(t, idx, map[string]string{"cli/a.go": "// SPDX-License-Identifier: MIT\npackage main\n"}, []string{"cli/a.go"})
	v := runConstraintGate(t, rc)
	if v.Status != ports.GateStatusPass {
		t.Fatalf("absent-mode: required pattern present must PASS, got %s: %s", v.Status, v.Reason)
	}
}

// --- Fail-open holes caught by cross-family review (2026-06-21) ---

// #1 Full mode: the orchestrator passes changed=nil in full mode, so the check
// must enumerate the repo itself or it silently enforces zero files in CI.
func TestConstraintGate_FullMode_ScansRepo_Fails(t *testing.T) {
	rc := writeConstraintFixture(t, forbidConstraint,
		map[string]string{"cli/internal/foo.go": "func x() {\n\tpanic(\"boom\")\n}\n"},
		nil) // full mode: ChangedFiles is nil
	rc.ChangedFiles = nil
	rc.Mode = gates.Full
	v := runConstraintGate(t, rc)
	if v.Status != ports.GateStatusFail {
		t.Fatalf("full mode must scan the repo (changed=nil) and FAIL a violation, got %s: %s", v.Status, v.Reason)
	}
}

func TestConstraintGate_FullMode_NoViolation_Passes(t *testing.T) {
	rc := writeConstraintFixture(t, forbidConstraint,
		map[string]string{"cli/internal/foo.go": "func x() {\n\treturn\n}\n"}, nil)
	rc.ChangedFiles = nil
	rc.Mode = gates.Full
	v := runConstraintGate(t, rc)
	if v.Status != ports.GateStatusPass {
		t.Fatalf("full mode with no violation must PASS, got %s: %s", v.Status, v.Reason)
	}
}

// #2 A JSON-valid index that drops constraints (typo'd top-level key) must not
// silently pass — strict decoding fails closed.
func TestConstraintGate_TypoTopLevelKey_FailsClosed(t *testing.T) {
	// "constraint" (singular) is an unknown field; the real active constraint is
	// silently dropped under permissive decoding.
	idx := `{"schema_version":1,"constraint":[{"id":"f","title":"t","status":"active","compiled_at":"2026-06-21T00:00:00Z","applies_to":{"path_globs":["cli/**"]},"detector":{"kind":"regex","pattern":"panic"}}]}`
	rc := writeConstraintFixture(t, idx, map[string]string{"cli/a.go": "panic()"}, []string{"cli/a.go"})
	v := runConstraintGate(t, rc)
	if v.Status != ports.GateStatusFail {
		t.Fatalf("a JSON-valid index with an unknown top-level key must FAIL CLOSED, got %s: %s", v.Status, v.Reason)
	}
}

func TestConstraintGate_WrongSchemaVersion_FailsClosed(t *testing.T) {
	for _, idx := range []string{
		`{"schema_version":2,"constraints":[]}`,
		`{}`,
	} {
		rc := writeConstraintFixture(t, idx, nil, []string{"cli/a.go"})
		v := runConstraintGate(t, rc)
		if v.Status != ports.GateStatusFail {
			t.Fatalf("unknown/absent schema_version (%s) must FAIL CLOSED, got %s: %s", idx, v.Status, v.Reason)
		}
	}
}

// #3 A matching changed file that is present-but-unreadable (here: a directory)
// must fail closed — only a genuine deletion (IsNotExist) is a safe skip.
func TestConstraintGate_UnreadableMatchingFile_FailsClosed(t *testing.T) {
	rc := writeConstraintFixture(t, forbidConstraint, nil, []string{"cli/adir"})
	// Make the changed path a directory so os.ReadFile fails non-IsNotExist.
	if err := os.MkdirAll(filepath.Join(rc.RepoRoot, "cli", "adir"), 0o755); err != nil {
		t.Fatal(err)
	}
	v := runConstraintGate(t, rc)
	if v.Status != ports.GateStatusFail {
		t.Fatalf("a present-but-unreadable matching file must FAIL CLOSED, got %s: %s", v.Status, v.Reason)
	}
}

func TestConstraintGate_DeletedFile_SkippedSafely(t *testing.T) {
	// File is in the change set but absent on disk (a deletion): safe to skip.
	rc := writeConstraintFixture(t, forbidConstraint, nil, []string{"cli/gone.go"})
	v := runConstraintGate(t, rc)
	if v.Status != ports.GateStatusPass {
		t.Fatalf("a deleted (IsNotExist) changed file must be skipped safely -> PASS, got %s: %s", v.Status, v.Reason)
	}
}

// #4 A glob shape the gate's matcher cannot faithfully evaluate must fail closed,
// never silently route an active constraint to zero files.
func TestConstraintGate_UnsupportedGlob_FailsClosed(t *testing.T) {
	idx := `{"schema_version":1,"constraints":[{"id":"f","title":"t","status":"active","compiled_at":"2026-06-21T00:00:00Z","applies_to":{"path_globs":["cli/**/*.go"]},"detector":{"kind":"regex","pattern":"panic"}}]}`
	rc := writeConstraintFixture(t, idx, map[string]string{"cli/internal/a.go": "panic()"}, []string{"cli/internal/a.go"})
	v := runConstraintGate(t, rc)
	if v.Status != ports.GateStatusFail {
		t.Fatalf("an unsupported glob shape (cli/**/*.go) on an active constraint must FAIL CLOSED, got %s: %s", v.Status, v.Reason)
	}
}

func TestConstraintGate_BlankGlob_FailsClosed(t *testing.T) {
	idx := `{"schema_version":1,"constraints":[{"id":"f","title":"t","status":"active","compiled_at":"2026-06-21T00:00:00Z","applies_to":{"path_globs":[""]},"detector":{"kind":"regex","mode":"match","pattern":"panic"}}]}`
	rc := writeConstraintFixture(t, idx, map[string]string{"cli/a.go": "panic()"}, []string{"cli/a.go"})
	v := runConstraintGate(t, rc)
	if v.Status != ports.GateStatusFail {
		t.Fatalf("a blank path glob on an active constraint matches nothing -> would pass vacuously; must FAIL CLOSED, got %s: %s", v.Status, v.Reason)
	}
}

// #2 (round 2): a structurally-incomplete index must not silently yield zero
// enforced constraints. Close the whole class, not the three instances.
func TestConstraintGate_StructurallyIncompleteIndex_FailsClosed(t *testing.T) {
	cases := map[string]string{
		"missing constraints array": `{"schema_version":1}`,
		"constraints null":          `{"schema_version":1,"constraints":null}`,
		"entry missing status":      `{"schema_version":1,"constraints":[{"id":"f","title":"t","compiled_at":"x","applies_to":{"path_globs":["cli/**"]},"detector":{"kind":"regex","pattern":"panic"}}]}`,
		"entry unknown status":       `{"schema_version":1,"constraints":[{"id":"f","title":"t","status":"bogus","compiled_at":"x","applies_to":{"path_globs":["cli/**"]},"detector":{"kind":"regex","pattern":"panic"}}]}`,
		"entry missing id":           `{"schema_version":1,"constraints":[{"title":"t","status":"active","compiled_at":"x","applies_to":{"path_globs":["cli/**"]},"detector":{"kind":"regex","pattern":"panic"}}]}`,
		"trailing brace":             `{"schema_version":1,"constraints":[]}}`,
		"duplicate constraints key":  `{"schema_version":1,"constraints":[{"id":"f","title":"t","status":"active","compiled_at":"x","applies_to":{"path_globs":["cli/**"]},"detector":{"kind":"regex","pattern":"panic"}}],"constraints":[]}`,
		"duplicate nested status key": `{"schema_version":1,"constraints":[{"id":"f","title":"t","status":"active","status":"draft","compiled_at":"x","applies_to":{"path_globs":["cli/**"]},"detector":{"kind":"regex","pattern":"panic"}}]}`,
	}
	for name, idx := range cases {
		t.Run(name, func(t *testing.T) {
			rc := writeConstraintFixture(t, idx, map[string]string{"cli/a.go": "panic()"}, []string{"cli/a.go"})
			v := runConstraintGate(t, rc)
			if v.Status != ports.GateStatusFail {
				t.Fatalf("%s must FAIL CLOSED, got %s: %s", name, v.Status, v.Reason)
			}
		})
	}
}

// The check must be registered into the default registry so `ao gate check`
// actually enforces it (the loop only closes if the gate runs it).
func TestConstraintGate_RegisteredInDefault(t *testing.T) {
	found := false
	for _, c := range gates.Default.All() {
		if c.ID == "constraints.enforce" {
			found = true
			if !c.Blocking {
				t.Fatalf("constraints.enforce must be Blocking (fail-closed enforcement)")
			}
			if !c.Tiers.Has(gates.Fast) || !c.Tiers.Has(gates.Full) {
				t.Fatalf("constraints.enforce must run in both Fast and Full tiers")
			}
		}
	}
	if !found {
		t.Fatalf("constraints.enforce not registered in gates.Default")
	}
}
