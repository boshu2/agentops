// practices: [wiki-knowledge-surface, design-by-contract]
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRegistryList_AllTypes(t *testing.T) {
	dir := t.TempDir()
	writeTestRegistry(t, dir)
	testProjectDir = dir
	defer func() { testProjectDir = "" }()

	out := captureRegistryOutput(t, "", false)
	if !strings.Contains(out, "skill") {
		t.Error("expected skill entries in output")
	}
	if !strings.Contains(out, "hook") {
		t.Error("expected hook entries in output")
	}
	if !strings.Contains(out, "job") {
		t.Error("expected job entries in output")
	}
	if !strings.Contains(out, "cadence") {
		t.Error("expected cadence entries in output")
	}
}

func TestRegistryList_FilterByType(t *testing.T) {
	dir := t.TempDir()
	writeTestRegistry(t, dir)
	testProjectDir = dir
	defer func() { testProjectDir = "" }()

	tests := []struct {
		filter   string
		contains string
		excludes string
	}{
		{"skills", "skill", "hook"},
		{"hooks", "pre-push", "skill"},
		{"jobs", "dream.run", "hook"},
		{"cadence", "nightly-dream", "skill"},
	}

	for _, tc := range tests {
		t.Run(tc.filter, func(t *testing.T) {
			out := captureRegistryOutput(t, tc.filter, false)
			if !strings.Contains(out, tc.contains) {
				t.Errorf("filter %q: expected %q in output, got:\n%s", tc.filter, tc.contains, out)
			}
			if strings.Contains(out, tc.excludes) {
				t.Errorf("filter %q: should not contain %q in output", tc.filter, tc.excludes)
			}
		})
	}
}

func TestRegistryList_InvalidType(t *testing.T) {
	dir := t.TempDir()
	writeTestRegistry(t, dir)
	testProjectDir = dir
	defer func() { testProjectDir = "" }()

	registryTypeFilter = "invalid"
	defer func() { registryTypeFilter = "" }()

	err := runRegistryListCommand(registryListCmd, nil)
	if err == nil {
		t.Fatal("expected error for invalid type filter")
	}
	if !strings.Contains(err.Error(), "invalid type") {
		t.Errorf("expected 'invalid type' error, got: %v", err)
	}
}

func TestRegistryList_JSON(t *testing.T) {
	dir := t.TempDir()
	writeTestRegistry(t, dir)
	testProjectDir = dir
	defer func() { testProjectDir = "" }()

	out := captureRegistryOutput(t, "skills", true)
	var skills []registrySkill
	if err := json.Unmarshal([]byte(out), &skills); err != nil {
		t.Fatalf("JSON output should be valid: %v\noutput: %s", err, out)
	}
	if len(skills) != 2 {
		t.Errorf("expected 2 skills, got %d", len(skills))
	}
}

func TestRegistryList_MissingFile(t *testing.T) {
	dir := t.TempDir()
	testProjectDir = dir
	defer func() { testProjectDir = "" }()

	registryTypeFilter = ""
	err := runRegistryListCommand(registryListCmd, nil)
	if err == nil {
		t.Fatal("expected error when registry.json missing")
	}
	if !strings.Contains(err.Error(), "registry.json not found") {
		t.Errorf("expected helpful error message, got: %v", err)
	}
}

func captureRegistryOutput(t *testing.T, typeFilter string, useJSON bool) string {
	t.Helper()
	registryTypeFilter = typeFilter
	origJSON := jsonFlag
	jsonFlag = useJSON
	defer func() {
		registryTypeFilter = ""
		jsonFlag = origJSON
	}()

	old := registryListCmd.OutOrStdout()
	buf := &strings.Builder{}
	registryListCmd.SetOut(buf)
	defer registryListCmd.SetOut(old)

	if err := runRegistryListCommand(registryListCmd, nil); err != nil {
		t.Fatalf("runRegistryListCommand: %v", err)
	}
	return buf.String()
}

func writeTestRegistry(t *testing.T, dir string) {
	t.Helper()
	reg := registryFile{
		SchemaVersion: 1,
		GeneratedAt:   "2026-05-04T00:00:00Z",
		Surfaces: registrySurfaces{
			Skills: []registrySkill{
				{Name: "evolve", Tier: "orchestration", Path: "skills/evolve/"},
				{Name: "forge", Tier: "background", Path: "skills/forge/"},
			},
			Hooks: []registryHook{
				{Name: "pre-push", Lifecycle: "PrePush", Path: "hooks/pre-push.sh"},
			},
			Stores: []registryStore{
				{Name: "learnings", Purpose: "Extracted session learnings"},
			},
			JobTypes: []registryJobType{
				{JobType: "dream.run", Domain: "dream", Action: "run"},
			},
			Evals: []registryEval{
				{Suite: "agentops-core", EvalCount: 42, Path: "evals/"},
			},
			CLI: []registryCLI{
				{Name: "inject", Path: "cli/cmd/ao/inject.go"},
			},
		},
		Cadence: []registryCadence{
			{Name: "nightly-dream", Cadence: "nightly", Cron: "0 3 * * *", JobType: "dream.run", Description: "Full consolidation"},
		},
	}
	data, err := json.Marshal(reg)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "registry.json"), data, 0644); err != nil {
		t.Fatal(err)
	}
}
