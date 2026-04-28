package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/boshu2/agentops/cli/embedded"
)

func TestSeed_CreatesAgentsDir(t *testing.T) {
	tmp := t.TempDir()

	// Reset flags
	dryRun = false
	seedForce = false
	seedTemplate = "generic"
	output = "table"

	if err := runSeed(seedCmd, []string{tmp}); err != nil {
		t.Fatalf("runSeed: %v", err)
	}

	// Verify all agentsDirs exist
	for _, dir := range agentsDirs {
		target := filepath.Join(tmp, dir)
		if _, err := os.Stat(target); os.IsNotExist(err) {
			t.Errorf("expected dir %s to exist", dir)
		}
	}
}

func TestSeed_CreatesGitignore(t *testing.T) {
	tmp := t.TempDir()

	// Create a git repo so git protection runs
	if err := os.MkdirAll(filepath.Join(tmp, ".git"), 0755); err != nil {
		t.Fatal(err)
	}

	dryRun = false
	seedForce = false
	seedTemplate = "generic"
	output = "table"

	if err := runSeed(seedCmd, []string{tmp}); err != nil {
		t.Fatalf("runSeed: %v", err)
	}

	// Verify .agents/.gitignore was created
	gitignorePath := filepath.Join(tmp, ".agents", ".gitignore")
	if _, err := os.Stat(gitignorePath); os.IsNotExist(err) {
		t.Error("expected .agents/.gitignore to be created by seed")
	}
}

func TestSeed_CreatesStorageDirs(t *testing.T) {
	tmp := t.TempDir()

	dryRun = false
	seedForce = false
	seedTemplate = "generic"
	output = "table"

	if err := runSeed(seedCmd, []string{tmp}); err != nil {
		t.Fatalf("runSeed: %v", err)
	}

	// Verify storage subdirs created by initStorage
	for _, subdir := range []string{"sessions", "index", "provenance"} {
		target := filepath.Join(tmp, ".agents", "ao", subdir)
		if _, err := os.Stat(target); os.IsNotExist(err) {
			t.Errorf("expected .agents/ao/%s to be created by seed", subdir)
		}
	}
}

func TestSeed_CreatesGoals(t *testing.T) {
	tmp := t.TempDir()

	dryRun = false
	seedForce = false
	seedTemplate = "generic"
	output = "table"

	if err := runSeed(seedCmd, []string{tmp}); err != nil {
		t.Fatalf("runSeed: %v", err)
	}

	goalsPath := filepath.Join(tmp, "GOALS.md")
	data, err := os.ReadFile(goalsPath)
	if err != nil {
		t.Fatalf("expected GOALS.md to be created: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "# Goals") {
		t.Error("expected GOALS.md to contain '# Goals' header")
	}
	if !strings.Contains(content, "Fitness goals for") {
		t.Error("expected GOALS.md to contain mission statement")
	}
	if !strings.Contains(content, "## North Stars") {
		t.Error("expected GOALS.md to contain North Stars section")
	}
	if !strings.Contains(content, "## Anti Stars") {
		t.Error("expected GOALS.md to contain Anti Stars section")
	}
	if !strings.Contains(content, "## Directives") {
		t.Error("expected GOALS.md to contain Directives section")
	}
}

func TestSeed_AutoDetect_Go(t *testing.T) {
	tmp := t.TempDir()

	// Create go.mod to trigger Go detection
	goMod := "module example.com/myproject\n\ngo 1.22\n"
	if err := os.WriteFile(filepath.Join(tmp, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatal(err)
	}

	dryRun = false
	seedForce = false
	seedTemplate = "" // auto-detect
	output = "table"

	if err := runSeed(seedCmd, []string{tmp}); err != nil {
		t.Fatalf("runSeed: %v", err)
	}

	goalsPath := filepath.Join(tmp, "GOALS.md")
	data, err := os.ReadFile(goalsPath)
	if err != nil {
		t.Fatalf("expected GOALS.md to be created: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "Go CLI") {
		t.Error("expected GOALS.md to contain 'Go CLI' for go-cli template")
	}
	if !strings.Contains(content, "go vet") || !strings.Contains(content, "golangci-lint") {
		t.Error("expected GOALS.md north stars to mention go vet and golangci-lint")
	}

	// Verify detected gates include go-build
	if !strings.Contains(content, "go-build") {
		t.Error("expected GOALS.md to contain auto-detected go-build gate")
	}
}

func TestSeed_AutoDetect_Node(t *testing.T) {
	tmp := t.TempDir()

	// Create package.json to trigger web-app detection
	pkgJSON := `{"name": "my-app", "version": "1.0.0"}`
	if err := os.WriteFile(filepath.Join(tmp, "package.json"), []byte(pkgJSON), 0644); err != nil {
		t.Fatal(err)
	}

	dryRun = false
	seedForce = false
	seedTemplate = "" // auto-detect
	output = "table"

	if err := runSeed(seedCmd, []string{tmp}); err != nil {
		t.Fatalf("runSeed: %v", err)
	}

	goalsPath := filepath.Join(tmp, "GOALS.md")
	data, err := os.ReadFile(goalsPath)
	if err != nil {
		t.Fatalf("expected GOALS.md to be created: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "web application") {
		t.Error("expected GOALS.md to contain 'web application' for web-app template")
	}
	if !strings.Contains(content, "npm-test") {
		t.Error("expected GOALS.md to contain auto-detected npm-test gate")
	}
}

func TestSeed_DryRun(t *testing.T) {
	tmp := t.TempDir()

	dryRun = true
	seedForce = false
	seedTemplate = "generic"
	output = "table"
	defer func() { dryRun = false }()

	if err := runSeed(seedCmd, []string{tmp}); err != nil {
		t.Fatalf("runSeed dry-run: %v", err)
	}

	// No .agents/ directories should be created
	for _, dir := range agentsDirs {
		if _, err := os.Stat(filepath.Join(tmp, dir)); err == nil {
			t.Errorf("expected dir %s NOT to exist in dry-run", dir)
		}
	}

	// No GOALS.md should be created
	if _, err := os.Stat(filepath.Join(tmp, "GOALS.md")); err == nil {
		t.Error("expected GOALS.md NOT to exist in dry-run")
	}

	// No CLAUDE.md should be created
	if _, err := os.Stat(filepath.Join(tmp, "CLAUDE.md")); err == nil {
		t.Error("expected CLAUDE.md NOT to exist in dry-run")
	}

	// No learnings should be created
	learningsDir := filepath.Join(tmp, ".agents", "learnings")
	if _, err := os.Stat(learningsDir); err == nil {
		t.Error("expected .agents/learnings/ NOT to exist in dry-run")
	}
}

func TestSeed_DryRun_JSON(t *testing.T) {
	tmp := t.TempDir()

	dryRun = true
	seedForce = false
	seedTemplate = "generic"
	output = "json"
	defer func() {
		dryRun = false
		output = "table"
	}()

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := runSeed(seedCmd, []string{tmp})

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("runSeed dry-run JSON: %v", err)
	}

	buf := make([]byte, 65536)
	n, _ := r.Read(buf)
	r.Close()

	// dry-run prints [dry-run] messages before JSON; find the actual JSON object.
	// Init functions may print braces (e.g. {sessions,...}), so search backwards
	// from the end for the last top-level '{' which is the JSON envelope.
	raw := string(buf[:n])
	jsonStart := strings.LastIndex(raw, "{\n")
	if jsonStart < 0 {
		t.Fatalf("no JSON object in output:\n%s", raw)
	}

	var result seedResult
	if err := json.Unmarshal([]byte(raw[jsonStart:]), &result); err != nil {
		t.Fatalf("parse JSON output: %v\nraw: %s", err, raw[jsonStart:])
	}

	if !result.DryRun {
		t.Error("expected DryRun=true in JSON output")
	}
	if result.Template != "generic" {
		t.Errorf("expected template 'generic', got %s", result.Template)
	}

	// No files should be created in dry-run
	if _, err := os.Stat(filepath.Join(tmp, "GOALS.md")); err == nil {
		t.Error("expected GOALS.md NOT to exist in dry-run")
	}
	if _, err := os.Stat(filepath.Join(tmp, "CLAUDE.md")); err == nil {
		t.Error("expected CLAUDE.md NOT to exist in dry-run")
	}
}

func TestSeed_BootstrapLearning(t *testing.T) {
	tmp := t.TempDir()

	dryRun = false
	seedForce = false
	seedTemplate = "go-cli"
	output = "table"

	if err := runSeed(seedCmd, []string{tmp}); err != nil {
		t.Fatalf("runSeed: %v", err)
	}

	// Find the bootstrap learning file
	learningsDir := filepath.Join(tmp, ".agents", "learnings")
	entries, err := os.ReadDir(learningsDir)
	if err != nil {
		t.Fatalf("read learnings dir: %v", err)
	}

	var found bool
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), "-seed-bootstrap.md") {
			found = true
			data, err := os.ReadFile(filepath.Join(learningsDir, e.Name()))
			if err != nil {
				t.Fatalf("read bootstrap learning: %v", err)
			}
			content := string(data)
			if !strings.Contains(content, "**Type:** decision") {
				t.Error("expected bootstrap learning to have type 'decision'")
			}
			if !strings.Contains(content, "template go-cli") {
				t.Error("expected bootstrap learning to mention template name")
			}
			if !strings.Contains(content, "ao seed") {
				t.Error("expected bootstrap learning to mention 'ao seed' as source")
			}
			break
		}
	}
	if !found {
		t.Error("expected bootstrap learning file to be created in .agents/learnings/")
	}
}

func TestSeed_IdempotentForce(t *testing.T) {
	tmp := t.TempDir()

	dryRun = false
	seedForce = false
	seedTemplate = "generic"
	output = "table"

	// First seed
	if err := runSeed(seedCmd, []string{tmp}); err != nil {
		t.Fatalf("first runSeed: %v", err)
	}

	// Read original GOALS.md
	goalsPath := filepath.Join(tmp, "GOALS.md")
	origGoals, err := os.ReadFile(goalsPath)
	if err != nil {
		t.Fatalf("read GOALS.md: %v", err)
	}

	// Modify GOALS.md to detect overwrite
	modifiedContent := string(origGoals) + "\n# Modified by test\n"
	if err := os.WriteFile(goalsPath, []byte(modifiedContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Second seed without --force: should skip existing files
	if err := runSeed(seedCmd, []string{tmp}); err != nil {
		t.Fatalf("second runSeed (no force): %v", err)
	}

	// GOALS.md should still have our modification (was skipped)
	data, _ := os.ReadFile(goalsPath)
	if !strings.Contains(string(data), "# Modified by test") {
		t.Error("expected GOALS.md to be preserved without --force")
	}

	// Third seed with --force: should overwrite
	seedForce = true
	if err := runSeed(seedCmd, []string{tmp}); err != nil {
		t.Fatalf("third runSeed (force): %v", err)
	}

	// GOALS.md should be regenerated (no modification marker)
	data, _ = os.ReadFile(goalsPath)
	if strings.Contains(string(data), "# Modified by test") {
		t.Error("expected GOALS.md to be overwritten with --force")
	}
}

func TestSeed_ClaudeMDCreated(t *testing.T) {
	tmp := t.TempDir()

	dryRun = false
	seedForce = false
	seedTemplate = "generic"
	output = "table"

	if err := runSeed(seedCmd, []string{tmp}); err != nil {
		t.Fatalf("runSeed: %v", err)
	}

	claudePath := filepath.Join(tmp, "CLAUDE.md")
	data, err := os.ReadFile(claudePath)
	if err != nil {
		t.Fatalf("expected CLAUDE.md to be created: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "ao flywheel status") {
		t.Error("expected CLAUDE.md to contain 'ao flywheel status' instruction")
	}
	if !strings.Contains(content, "MEMORY.md") {
		t.Error("expected CLAUDE.md to contain MEMORY.md reference")
	}
	if !strings.Contains(content, claudeMDSeedMarker) {
		t.Error("expected CLAUDE.md to contain seed section marker")
	}
}

func TestSeed_ClaudeMDAppend(t *testing.T) {
	tmp := t.TempDir()

	// Pre-create CLAUDE.md with existing content
	existing := "# My Project\n\nSome existing instructions.\n"
	if err := os.WriteFile(filepath.Join(tmp, "CLAUDE.md"), []byte(existing), 0644); err != nil {
		t.Fatal(err)
	}

	dryRun = false
	seedForce = false
	seedTemplate = "generic"
	output = "table"

	if err := runSeed(seedCmd, []string{tmp}); err != nil {
		t.Fatalf("runSeed: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(tmp, "CLAUDE.md"))
	content := string(data)

	// Should preserve existing content
	if !strings.Contains(content, "Some existing instructions.") {
		t.Error("expected existing CLAUDE.md content to be preserved")
	}
	// Should append seed section
	if !strings.Contains(content, claudeMDSeedMarker) {
		t.Error("expected seed section to be appended to CLAUDE.md")
	}
}

func TestSeed_JSONOutput(t *testing.T) {
	tmp := t.TempDir()

	dryRun = false
	seedForce = false
	seedTemplate = "generic"
	output = "json"
	defer func() { output = "table" }()

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := runSeed(seedCmd, []string{tmp})

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("runSeed: %v", err)
	}

	buf := make([]byte, 65536)
	n, _ := r.Read(buf)
	r.Close()

	var result seedResult
	if err := json.Unmarshal(buf[:n], &result); err != nil {
		t.Fatalf("parse JSON output: %v\nraw: %s", err, string(buf[:n]))
	}

	if result.Path != tmp {
		t.Errorf("expected path %s, got %s", tmp, result.Path)
	}
	if result.Template != "generic" {
		t.Errorf("expected template 'generic', got %s", result.Template)
	}
	if len(result.Created) == 0 {
		t.Error("expected at least one created entry")
	}
}

func TestSeed_InvalidTemplate(t *testing.T) {
	tmp := t.TempDir()

	dryRun = false
	seedForce = false
	seedTemplate = "invalid-template"
	output = "table"

	err := runSeed(seedCmd, []string{tmp})
	if err == nil {
		t.Fatal("expected error for invalid template")
	}
	if !strings.Contains(err.Error(), "unknown template") {
		t.Errorf("expected 'unknown template' error, got: %v", err)
	}
}

func TestSeed_ValidTemplatesHaveEmbeddedFiles(t *testing.T) {
	if err := validateTemplateMapEntries(validTemplates, embedded.TemplatesFS); err != nil {
		t.Fatalf("validTemplates drifted from embedded templates: %v", err)
	}
}

func TestSeed_ValidTemplatesDetectMissingEmbeddedFile(t *testing.T) {
	err := validateTemplateMapEntries(
		map[string]bool{"missing-template": true},
		fstest.MapFS{},
	)
	if err == nil {
		t.Fatal("expected missing template error, got nil")
	}
	if !strings.Contains(err.Error(), "missing-template") {
		t.Fatalf("expected missing template name in error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "templates/missing-template.yaml") {
		t.Fatalf("expected missing template path in error, got: %v", err)
	}
}

func TestSeed_NonexistentPath(t *testing.T) {
	dryRun = false
	seedForce = false
	seedTemplate = "generic"
	output = "table"

	err := runSeed(seedCmd, []string{"/nonexistent/path/that/does/not/exist"})
	if err == nil {
		t.Fatal("expected error for nonexistent path")
	}
}

func TestSeed_AutoDetect_Python(t *testing.T) {
	tmp := t.TempDir()

	pyproject := `[project]
name = "my-lib"
version = "1.0.0"
`
	if err := os.WriteFile(filepath.Join(tmp, "pyproject.toml"), []byte(pyproject), 0644); err != nil {
		t.Fatal(err)
	}

	dryRun = false
	seedForce = false
	seedTemplate = "" // auto-detect
	output = "table"

	if err := runSeed(seedCmd, []string{tmp}); err != nil {
		t.Fatalf("runSeed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(tmp, "GOALS.md"))
	if err != nil {
		t.Fatalf("read GOALS.md: %v", err)
	}
	if !strings.Contains(string(data), "Python library") {
		t.Error("expected GOALS.md to contain 'Python library' for python-lib template")
	}
}

func TestSeed_AutoDetect_Rust(t *testing.T) {
	tmp := t.TempDir()

	cargoToml := `[package]
name = "my-cli"
version = "0.1.0"
`
	if err := os.WriteFile(filepath.Join(tmp, "Cargo.toml"), []byte(cargoToml), 0644); err != nil {
		t.Fatal(err)
	}

	dryRun = false
	seedForce = false
	seedTemplate = "" // auto-detect
	output = "table"

	if err := runSeed(seedCmd, []string{tmp}); err != nil {
		t.Fatalf("runSeed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(tmp, "GOALS.md"))
	if err != nil {
		t.Fatalf("read GOALS.md: %v", err)
	}
	if !strings.Contains(string(data), "Rust CLI") {
		t.Error("expected GOALS.md to contain 'Rust CLI' for rust-cli template")
	}
}

func TestSeed_DefaultPath(t *testing.T) {
	tmp := t.TempDir()

	t.Chdir(tmp)

	dryRun = false
	seedForce = false
	seedTemplate = "generic"
	output = "table"

	// Call with no args (defaults to ".")
	if err := runSeed(seedCmd, nil); err != nil {
		t.Fatalf("runSeed with no args: %v", err)
	}

	// Verify GOALS.md was created in the temp dir
	if _, err := os.Stat(filepath.Join(tmp, "GOALS.md")); os.IsNotExist(err) {
		t.Error("expected GOALS.md to be created in current directory")
	}
}

func TestDetectTemplate(t *testing.T) {
	tests := []struct {
		name     string
		files    map[string]string
		expected string
	}{
		{
			name:     "go.mod",
			files:    map[string]string{"go.mod": "module example.com\n"},
			expected: "go-cli",
		},
		{
			name:     "cli/go.mod",
			files:    map[string]string{"cli/go.mod": "module example.com/cli\n"},
			expected: "go-cli",
		},
		{
			name:     "package.json",
			files:    map[string]string{"package.json": "{}"},
			expected: "web-app",
		},
		{
			name:     "pyproject.toml",
			files:    map[string]string{"pyproject.toml": "[project]"},
			expected: "python-lib",
		},
		{
			name:     "Cargo.toml",
			files:    map[string]string{"Cargo.toml": "[package]"},
			expected: "rust-cli",
		},
		{
			name:     "empty dir",
			files:    map[string]string{},
			expected: "generic",
		},
		{
			name:     "go.mod takes precedence over package.json",
			files:    map[string]string{"go.mod": "module m\n", "package.json": "{}"},
			expected: "go-cli",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmp := t.TempDir()
			for path, content := range tt.files {
				fullPath := filepath.Join(tmp, path)
				if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
					t.Fatal(err)
				}
			}

			got := detectTemplate(tmp)
			if got != tt.expected {
				t.Errorf("detectTemplate() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestBuildSeedGoalFile_AllTemplates(t *testing.T) {
	// Verify templateConfigs covers every validTemplate
	for name := range validTemplates {
		if _, ok := templateConfigs[name]; !ok {
			t.Errorf("validTemplates contains %q but templateConfigs does not", name)
		}
	}

	// Verify each template produces a well-formed GoalFile
	for name := range templateConfigs {
		t.Run(name, func(t *testing.T) {
			gf := buildSeedGoalFile("/tmp/myproject", name)
			if gf.Version != 4 {
				t.Errorf("expected version 4, got %d", gf.Version)
			}
			if gf.Format != "md" {
				t.Errorf("expected format 'md', got %q", gf.Format)
			}
			if !strings.Contains(gf.Mission, "Fitness goals for myproject") {
				t.Errorf("expected mission to contain dir name, got %q", gf.Mission)
			}
			if len(gf.NorthStars) == 0 {
				t.Error("expected at least one NorthStar")
			}
			if len(gf.AntiStars) == 0 {
				t.Error("expected at least one AntiStar")
			}
			if len(gf.Directives) == 0 {
				t.Error("expected at least one Directive")
			}
		})
	}
}

func TestBuildSeedGoalFile_UnknownFallsBackToGeneric(t *testing.T) {
	gf := buildSeedGoalFile("/tmp/proj", "nonexistent-template")
	generic := buildSeedGoalFile("/tmp/proj", "generic")

	if gf.Mission != generic.Mission {
		t.Errorf("unknown template mission = %q, want generic %q", gf.Mission, generic.Mission)
	}
	if len(gf.NorthStars) != len(generic.NorthStars) {
		t.Errorf("unknown template NorthStars count = %d, want %d", len(gf.NorthStars), len(generic.NorthStars))
	}
}

func TestValidTemplatesMatchEmbeddedTemplates(t *testing.T) {
	templatesDir := filepath.Join("..", "..", "embedded", "templates")
	entries, err := os.ReadDir(templatesDir)
	if err != nil {
		t.Fatalf("read templates dir %s: %v", templatesDir, err)
	}

	available := map[string]bool{}
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasSuffix(name, ".yaml") {
			available[strings.TrimSuffix(name, ".yaml")] = true
		}
	}

	for template := range validTemplates {
		if !available[template] {
			t.Errorf("validTemplates map contains %q but templates/%q.yaml is missing", template, template)
		}
	}
}
