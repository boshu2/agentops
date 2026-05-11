// practices: [dora-metrics, lean-startup]
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/goals"
)

func TestDetectGates_CliGoMod(t *testing.T) {
	root := t.TempDir()
	cliDir := filepath.Join(root, "cli")
	if err := os.MkdirAll(cliDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cliDir, "go.mod"), []byte("module test\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	detected := detectGates(root)
	if len(detected) < 2 {
		t.Fatalf("expected >=2 gates for cli/go.mod project, got %d", len(detected))
	}

	var hasGoBuild, hasGoTest bool
	for _, g := range detected {
		if g.ID == "go-build" && strings.Contains(g.Check, "cd cli") {
			hasGoBuild = true
		}
		if g.ID == "go-test" && strings.Contains(g.Check, "cd cli") {
			hasGoTest = true
		}
	}
	if !hasGoBuild {
		t.Error("missing go-build gate with 'cd cli' prefix")
	}
	if !hasGoTest {
		t.Error("missing go-test gate with 'cd cli' prefix")
	}
}

func TestDetectGates_RootGoMod(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module test\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	detected := detectGates(root)
	if len(detected) < 2 {
		t.Fatalf("expected >=2 gates for root go.mod project, got %d", len(detected))
	}

	var hasGoBuild bool
	for _, g := range detected {
		if g.ID == "go-build" && g.Check == "go build ./..." {
			hasGoBuild = true
		}
	}
	if !hasGoBuild {
		t.Error("expected root-level go-build gate (no cd cli prefix)")
	}
}

func TestDetectGates_PackageJSON(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "package.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	detected := detectGates(root)
	var hasNpmTest bool
	for _, g := range detected {
		if g.ID == "npm-test" {
			hasNpmTest = true
		}
	}
	if !hasNpmTest {
		t.Error("expected npm-test gate for package.json project")
	}
}

func TestDetectGates_MultipleProjectFiles(t *testing.T) {
	root := t.TempDir()
	// Create package.json and Cargo.toml (no go.mod, so switch/case falls through)
	if err := os.WriteFile(filepath.Join(root, "package.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "Cargo.toml"), []byte("[package]\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	detected := detectGates(root)
	ids := map[string]bool{}
	for _, g := range detected {
		ids[g.ID] = true
	}
	if !ids["npm-test"] {
		t.Error("expected npm-test gate")
	}
	if !ids["cargo-test"] {
		t.Error("expected cargo-test gate")
	}
}

func TestDetectGates_EmptyDir(t *testing.T) {
	root := t.TempDir()
	detected := detectGates(root)
	if len(detected) != 0 {
		t.Errorf("expected 0 gates for empty dir, got %d", len(detected))
	}
}

func TestDetectGates_CliGoModTakesPriority(t *testing.T) {
	// When both cli/go.mod and root go.mod exist, the switch/case
	// matches cli/go.mod first due to case ordering.
	root := t.TempDir()
	cliDir := filepath.Join(root, "cli")
	if err := os.MkdirAll(cliDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cliDir, "go.mod"), []byte("module test/cli\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module test\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	detected := detectGates(root)
	for _, g := range detected {
		if g.ID == "go-build" && !strings.Contains(g.Check, "cd cli") {
			t.Error("expected cli/go.mod to take priority, but got root-level go-build gate")
		}
	}
}

func TestBuildInteractiveGoalFile_Defaults(t *testing.T) {
	// Empty input — all prompts get empty strings, triggering defaults.
	r := strings.NewReader("\n\n\n\n\n")
	gf, err := buildInteractiveGoalFile(r)
	if err != nil {
		t.Fatal(err)
	}

	if gf.Version != 4 {
		t.Errorf("Version = %d, want 4", gf.Version)
	}
	if len(gf.NorthStars) == 0 {
		t.Error("expected default north stars")
	}
	if gf.NorthStars[0] != "All checks pass on every commit" {
		t.Errorf("NorthStars[0] = %q, want default", gf.NorthStars[0])
	}
	if len(gf.Directives) != 1 {
		t.Fatalf("Directives = %d, want 1", len(gf.Directives))
	}
	if gf.Directives[0].Title != "Establish baseline" {
		t.Errorf("Directive title = %q, want default", gf.Directives[0].Title)
	}
}

func TestBuildInteractiveGoalFile_CustomInput(t *testing.T) {
	input := "Ship reliable software\nZero downtime, Fast deploys\nManual releases, Untested code\nAutomate CI\nEnsure every merge is validated.\n"
	r := strings.NewReader(input)
	gf, err := buildInteractiveGoalFile(r)
	if err != nil {
		t.Fatal(err)
	}

	if gf.Mission != "Ship reliable software" {
		t.Errorf("Mission = %q", gf.Mission)
	}
	if len(gf.NorthStars) != 2 {
		t.Fatalf("NorthStars = %d, want 2", len(gf.NorthStars))
	}
	if gf.NorthStars[0] != "Zero downtime" {
		t.Errorf("NorthStars[0] = %q", gf.NorthStars[0])
	}
	if gf.NorthStars[1] != "Fast deploys" {
		t.Errorf("NorthStars[1] = %q", gf.NorthStars[1])
	}
	if len(gf.AntiStars) != 2 {
		t.Fatalf("AntiStars = %d, want 2", len(gf.AntiStars))
	}
	if gf.AntiStars[0] != "Manual releases" {
		t.Errorf("AntiStars[0] = %q", gf.AntiStars[0])
	}
	if gf.AntiStars[1] != "Untested code" {
		t.Errorf("AntiStars[1] = %q", gf.AntiStars[1])
	}
	if len(gf.Directives) != 1 {
		t.Fatalf("Directives = %d, want 1", len(gf.Directives))
	}
	if gf.Directives[0].Title != "Automate CI" {
		t.Errorf("Directive title = %q", gf.Directives[0].Title)
	}
	if gf.Directives[0].Description != "Ensure every merge is validated." {
		t.Errorf("Directive description = %q", gf.Directives[0].Description)
	}

	// ValidateGoals checks gates, not metadata — no errors expected for an empty-gates file.
	if errs := goals.ValidateGoals(gf); len(errs) != 0 {
		t.Errorf("expected 0 validation errors, got %d: %v", len(errs), errs)
	}
}

// --- Template tests ---

func TestGoalsInit_Template_GoCLI(t *testing.T) {
	tmpl, err := loadTemplate("go-cli")
	if err != nil {
		t.Fatalf("loadTemplate(go-cli): %v", err)
	}

	if tmpl.Name != "go-cli" {
		t.Errorf("Name = %q, want go-cli", tmpl.Name)
	}
	if len(tmpl.Gates) < 3 {
		t.Fatalf("expected >=3 gates in go-cli template, got %d", len(tmpl.Gates))
	}

	converted := templateGatesToGoals(tmpl)
	ids := map[string]bool{}
	for _, g := range converted {
		ids[g.ID] = true
		if g.Type == "" {
			t.Errorf("gate %q has empty Type", g.ID)
		}
	}

	for _, expected := range []string{"go-build", "go-test", "go-vet"} {
		if !ids[expected] {
			t.Errorf("missing expected gate %q in go-cli template", expected)
		}
	}
}

func TestGoalsInit_Template_NonInteractive(t *testing.T) {
	// Simulate --non-interactive + --template=go-cli: build default goal file,
	// then append template gates instead of auto-detected gates.
	gf := buildDefaultGoalFile()

	tmpl, err := loadTemplate("go-cli")
	if err != nil {
		t.Fatalf("loadTemplate: %v", err)
	}
	gf.Goals = append(gf.Goals, templateGatesToGoals(tmpl)...)

	// Verify the goal file has the expected structure.
	if gf.Version != 4 {
		t.Errorf("Version = %d, want 4", gf.Version)
	}
	if len(gf.Goals) != 3 {
		t.Errorf("Goals = %d, want 3 (go-build, go-test, go-vet)", len(gf.Goals))
	}

	// Render to markdown and verify it produces valid output.
	content := goals.RenderGoalsMD(gf)
	if !strings.Contains(content, "go-build") {
		t.Error("rendered markdown missing go-build gate")
	}
	if !strings.Contains(content, "go vet") {
		t.Error("rendered markdown missing go-vet check command")
	}
}

func TestGoalsInit_AutoDetect(t *testing.T) {
	tests := []struct {
		name     string
		files    map[string]string // relative path → content
		wantTmpl string
	}{
		{
			name:     "go.mod → go-cli",
			files:    map[string]string{"go.mod": "module test\n"},
			wantTmpl: "go-cli",
		},
		{
			name:     "cli/go.mod → go-cli",
			files:    map[string]string{"cli/go.mod": "module test/cli\n"},
			wantTmpl: "go-cli",
		},
		{
			name:     "package.json → web-app",
			files:    map[string]string{"package.json": "{}"},
			wantTmpl: "web-app",
		},
		{
			name:     "pyproject.toml → python-lib",
			files:    map[string]string{"pyproject.toml": "[project]\n"},
			wantTmpl: "python-lib",
		},
		{
			name:     "Cargo.toml → rust-cli",
			files:    map[string]string{"Cargo.toml": "[package]\n"},
			wantTmpl: "rust-cli",
		},
		{
			name:     "empty → no template",
			files:    map[string]string{},
			wantTmpl: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			for rel, content := range tt.files {
				full := filepath.Join(root, rel)
				if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
					t.Fatal(err)
				}
			}

			got := autoDetectTemplate(root)
			if got != tt.wantTmpl {
				t.Errorf("autoDetectTemplate() = %q, want %q", got, tt.wantTmpl)
			}
		})
	}
}

func TestGoalsInit_LoadTemplate_AllValid(t *testing.T) {
	// Verify all templates load and parse without error.
	for _, name := range validTemplateNames {
		t.Run(name, func(t *testing.T) {
			tmpl, err := loadTemplate(name)
			if err != nil {
				t.Fatalf("loadTemplate(%q): %v", name, err)
			}
			if tmpl.Name != name {
				t.Errorf("Name = %q, want %q", tmpl.Name, name)
			}
			if len(tmpl.Gates) == 0 {
				t.Error("expected at least one gate")
			}
			if len(tmpl.Directives) == 0 {
				t.Error("expected at least one directive")
			}
		})
	}
}

func TestGoalsInit_LoadTemplate_Invalid(t *testing.T) {
	_, err := loadTemplate("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent template, got nil")
	}
}
