package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestContextAssembleOutputFileFlag(t *testing.T) {
	// Find the assemble subcommand under context
	cmd, _, _ := rootCmd.Find([]string{"context", "assemble"})
	if cmd == nil {
		t.Fatal("context assemble command not found")
	}
	if phaseFlag := cmd.Flags().Lookup("phase"); phaseFlag == nil {
		t.Fatal("expected --phase flag on context assemble, not found")
	}
	f := cmd.Flags().Lookup("output-file")
	if f == nil {
		t.Fatal("expected --output-file flag on context assemble, not found")
	}
	// Check local flags only — root's persistent --output/-o (output format) is inherited and fine
	if old := cmd.LocalFlags().Lookup("output"); old != nil {
		t.Error("--output local flag should be renamed to --output-file on context assemble")
	}
}

func TestContextAssemble_FiveSections(t *testing.T) {
	tmp := t.TempDir()

	// Create GOALS.md
	goalsContent := `# GOALS

## Mission
Test project goals

## Fitness Gates

### test-gate
- **Description:** Ensure tests pass
- **Check:** echo pass
- **Weight:** 5
- **Type:** quality
`
	if err := os.WriteFile(filepath.Join(tmp, "GOALS.md"), []byte(goalsContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Create cycle history
	histDir := filepath.Join(tmp, ".agents", "evolve")
	if err := os.MkdirAll(histDir, 0755); err != nil {
		t.Fatal(err)
	}
	histLine := `{"timestamp":"2026-02-20T10:00:00Z","cycle":1,"status":"pass","summary":"test cycle"}`
	if err := os.WriteFile(filepath.Join(histDir, "cycle-history.jsonl"), []byte(histLine+"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a learning
	learnDir := filepath.Join(tmp, ".agents", "learnings")
	if err := os.MkdirAll(learnDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(learnDir, "test-lesson.md"), []byte("# Test Lesson\nAlways verify."), 0644); err != nil {
		t.Fatal(err)
	}

	// Assemble.
	sections := assembleSections(tmp, "Build the auth system", defaultAssembleMaxChars)

	if len(sections) != 5 {
		t.Fatalf("expected 5 sections, got %d", len(sections))
	}

	expectedNames := []string{"GOALS", sectionHistory, sectionIntel, sectionTask, "PROTOCOL"}
	for i, name := range expectedNames {
		if sections[i].Name != name {
			t.Errorf("section %d: expected name %q, got %q", i, name, sections[i].Name)
		}
		if sections[i].CharCount == 0 {
			t.Errorf("section %q has 0 chars", name)
		}
		if sections[i].Content == "" {
			t.Errorf("section %q has empty content", name)
		}
	}

	// Verify GOALS section references goals.
	if !strings.Contains(sections[0].Content, "GOALS") {
		t.Error("GOALS section missing GOALS header")
	}

	// Verify HISTORY section has the entry.
	if !strings.Contains(sections[1].Content, "HISTORY") {
		t.Error("HISTORY section missing HISTORY header")
	}

	// Verify INTEL section has the learning.
	if !strings.Contains(sections[2].Content, "INTEL") {
		t.Error("INTEL section missing INTEL header")
	}

	// Verify TASK section contains the task description.
	if !strings.Contains(sections[3].Content, "Build the auth system") {
		t.Error("TASK section missing task description")
	}

	// Verify PROTOCOL section has execution contract.
	if !strings.Contains(sections[4].Content, "PROTOCOL") {
		t.Error("PROTOCOL section missing PROTOCOL header")
	}
	if !strings.Contains(sections[4].Content, "Execution Contract") {
		t.Error("PROTOCOL section missing Execution Contract content")
	}
}

func TestContextAssemble_EmptyRepo(t *testing.T) {
	tmp := t.TempDir()

	// No GOALS, no history, no learnings — should gracefully handle all.
	sections := assembleSections(tmp, "Some task", defaultAssembleMaxChars)

	if len(sections) != 5 {
		t.Fatalf("expected 5 sections even in empty repo, got %d", len(sections))
	}

	// GOALS should have graceful fallback.
	if !strings.Contains(sections[0].Content, "No GOALS file found") {
		t.Error("GOALS section should indicate no GOALS file found")
	}

	// HISTORY should have graceful fallback.
	if !strings.Contains(sections[1].Content, "No cycle history found") {
		t.Error("HISTORY section should indicate no cycle history")
	}

	// INTEL should have graceful fallback.
	if !strings.Contains(sections[2].Content, "No learnings or patterns found") {
		t.Error("INTEL section should indicate no learnings or patterns")
	}

	// TASK should still have the task.
	if !strings.Contains(sections[3].Content, "Some task") {
		t.Error("TASK section missing task description")
	}

	// PROTOCOL should still have the template.
	if !strings.Contains(sections[4].Content, "Execution Contract") {
		t.Error("PROTOCOL section missing static template")
	}
}

func TestContextAssemble_CharBudget(t *testing.T) {
	tmp := t.TempDir()

	// Create oversized learnings to test budget enforcement.
	learnDir := filepath.Join(tmp, ".agents", "learnings")
	if err := os.MkdirAll(learnDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Write 20 large learning files (each ~2KB).
	for i := 0; i < 20; i++ {
		content := "# Large Learning " + strings.Repeat("x", 2000)
		name := filepath.Join(learnDir, "learning-"+strings.Repeat("a", 3)+string(rune('a'+i))+".md")
		if err := os.WriteFile(name, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// Use a small max-chars budget.
	smallBudget := 5000
	sections := assembleSections(tmp, "Test budget", smallBudget)

	totalChars := 0
	for _, s := range sections {
		totalChars += s.CharCount
	}

	// Total should not exceed the budget by much (some overhead from headers).
	// The budget is distributed proportionally, so we allow 20% slack for section headers.
	maxAllowed := int(float64(smallBudget) * 1.3)
	if totalChars > maxAllowed {
		t.Errorf("total chars %d exceeds budget %d (max allowed %d)", totalChars, smallBudget, maxAllowed)
	}

	// Verify each section is individually bounded.
	scale := float64(smallBudget) / float64(defaultAssembleMaxChars)
	intelBudget := int(float64(budgetIntel) * scale)

	// Intel section should be within its scaled budget + truncation marker.
	if sections[2].CharCount > intelBudget+100 {
		t.Errorf("INTEL section %d chars exceeds scaled budget %d", sections[2].CharCount, intelBudget)
	}
}

func TestContextAssemble_Redaction_EnvVar(t *testing.T) {
	tmp := t.TempDir()

	// Create a learning with a secret env var.
	learnDir := filepath.Join(tmp, ".agents", "learnings")
	if err := os.MkdirAll(learnDir, 0755); err != nil {
		t.Fatal(err)
	}
	secretContent := "# Config\nSet API_KEY=sk-abc123xyz to authenticate.\nNormal line here."
	if err := os.WriteFile(filepath.Join(learnDir, "config-secret.md"), []byte(secretContent), 0644); err != nil {
		t.Fatal(err)
	}

	sections := assembleSections(tmp, "Test redaction", defaultAssembleMaxChars)

	// Find INTEL section.
	intelContent := sections[2].Content

	// The API_KEY=sk-abc123xyz should be redacted.
	if strings.Contains(intelContent, "sk-abc123xyz") {
		t.Error("env var secret was not redacted from INTEL section")
	}
	if !strings.Contains(intelContent, "[REDACTED: env-var]") {
		t.Error("expected [REDACTED: env-var] marker in INTEL section")
	}

	// Verify redaction count.
	if sections[2].Redactions == 0 {
		t.Error("expected non-zero redaction count for INTEL section with secret")
	}

	// Check redaction log was written.
	logPath := filepath.Join(tmp, ".agents", "ao", "redaction.log")
	if _, err := os.Stat(logPath); err != nil {
		t.Error("expected redaction.log to be written")
	}
}

func TestContextAssemble_Redaction_JWT(t *testing.T) {
	tmp := t.TempDir()

	// Create a learning with a JWT token.
	learnDir := filepath.Join(tmp, ".agents", "learnings")
	if err := os.MkdirAll(learnDir, 0755); err != nil {
		t.Fatal(err)
	}

	jwtToken := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0"
	jwtContent := "# Auth Token\nUse this token: " + jwtToken + "\nDone."
	if err := os.WriteFile(filepath.Join(learnDir, "auth-token.md"), []byte(jwtContent), 0644); err != nil {
		t.Fatal(err)
	}

	sections := assembleSections(tmp, "Test JWT redaction", defaultAssembleMaxChars)

	intelContent := sections[2].Content

	// JWT should be redacted.
	if strings.Contains(intelContent, "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9") {
		t.Error("JWT token was not redacted from INTEL section")
	}
	if !strings.Contains(intelContent, "[REDACTED: jwt-token]") {
		t.Error("expected [REDACTED: jwt-token] marker in INTEL section")
	}

	if sections[2].Redactions == 0 {
		t.Error("expected non-zero redaction count for INTEL section with JWT")
	}
}

func TestContextAssemble_JSONOutput(t *testing.T) {
	tmp := t.TempDir()

	sections := assembleSections(tmp, "JSON test task", defaultAssembleMaxChars)

	totalChars := 0
	totalRedacted := 0
	for _, s := range sections {
		totalChars += s.CharCount
		totalRedacted += s.Redactions
	}

	outPath := filepath.Join(tmp, "briefing.md")

	out := assembleJSONOutput{
		OutputPath:    outPath,
		TotalChars:    totalChars,
		Sections:      sections,
		TotalRedacted: totalRedacted,
		Timestamp:     "2026-02-24T00:00:00Z",
	}

	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		t.Fatal(err)
	}

	// Verify it's valid JSON.
	var parsed assembleJSONOutput
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("JSON output is not valid: %v", err)
	}

	// Verify metadata fields.
	if parsed.OutputPath != outPath {
		t.Errorf("output_path = %q, want %q", parsed.OutputPath, outPath)
	}
	if parsed.TotalChars == 0 {
		t.Error("total_chars should not be 0")
	}
	if len(parsed.Sections) != 5 {
		t.Errorf("expected 5 sections in JSON, got %d", len(parsed.Sections))
	}
	if parsed.Timestamp == "" {
		t.Error("timestamp should not be empty")
	}

	// Verify section names in JSON.
	expectedNames := []string{"GOALS", sectionHistory, sectionIntel, sectionTask, "PROTOCOL"}
	for i, name := range expectedNames {
		if parsed.Sections[i].Name != name {
			t.Errorf("JSON section %d: expected name %q, got %q", i, name, parsed.Sections[i].Name)
		}
	}
}

func TestContextAssemble_ReadIntelDirReadsJSONFiles(t *testing.T) {
	tmp := t.TempDir()
	learnDir := filepath.Join(tmp, ".agents", "learnings")
	if err := os.MkdirAll(learnDir, 0o755); err != nil {
		t.Fatal(err)
	}
	patternDir := filepath.Join(tmp, ".agents", "patterns")
	if err := os.MkdirAll(patternDir, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(learnDir, "from-md.md"), []byte("# Markdown learning"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(learnDir, "from-json.json"), []byte(`{"content":"json learning"}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(patternDir, "pattern.json"), []byte(`{"pattern":"go"}`), 0644); err != nil {
		t.Fatal(err)
	}

	sections := assembleSections(tmp, "Include JSON artifacts", defaultAssembleMaxChars)
	intelContent := sections[2].Content

	if !strings.Contains(intelContent, "from-md") || !strings.Contains(intelContent, "from-json") {
		t.Error("INTEL section should include both markdown and json learnings")
	}
	if !strings.Contains(intelContent, "pattern") {
		t.Error("INTEL section should include pattern JSON artifacts")
	}
}

func TestContextAssemble_CommandWritesBriefingAndManifest(t *testing.T) {
	tmp := t.TempDir()

	goalsPath := filepath.Join(tmp, "GOALS.md")
	if err := os.WriteFile(goalsPath, []byte("# GOALS\n\n## Mission\nSmoke"), 0o644); err != nil {
		t.Fatal(err)
	}
	evolveDir := filepath.Join(tmp, ".agents", "evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(evolveDir, "cycle-history.jsonl"), []byte(`{"cycle":1,"status":"pass"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	learnDir := filepath.Join(tmp, ".agents", "learnings")
	if err := os.MkdirAll(learnDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(learnDir, "learn.md"), []byte("# Learn"), 0o644); err != nil {
		t.Fatal(err)
	}

	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = os.Chdir(oldWD)
	}()

	outPath := filepath.Join(tmp, ".agents", "rpi", "smoke-briefing.md")
	oldTask := assembleTask
	oldPhase := assemblePhase
	oldMax := assembleMaxChars
	oldOutput := assembleOutput
	oldMode := output

	assembleTask = "Smoke task"
	assemblePhase = "planning"
	assembleMaxChars = 12000
	assembleOutput = outPath
	output = "table"
	defer func() {
		assembleTask = oldTask
		assemblePhase = oldPhase
		assembleMaxChars = oldMax
		assembleOutput = oldOutput
		output = oldMode
	}()

	cmd := &cobra.Command{}
	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := runContextAssemble(cmd, nil); err != nil {
		t.Fatalf("runContextAssemble failed: %v", err)
	}

	briefingPath := outPath
	data, err := os.ReadFile(briefingPath)
	if err != nil {
		t.Fatalf("expected briefing file %s to exist: %v", briefingPath, err)
	}
	if !strings.Contains(string(data), "# Context Briefing") {
		t.Fatal("expected briefing markdown header")
	}
	if !strings.Contains(out.String(), "Briefing written to") {
		t.Error("expected command to print writing confirmation")
	}
	if !strings.Contains(string(data), "Smoke task") {
		t.Error("expected task content in generated briefing")
	}

	injectDir := filepath.Join(tmp, ".agents", "ao", "injections")
	manifestEntries, err := os.ReadDir(injectDir)
	if err != nil {
		t.Fatalf("expected injections directory to exist: %v", err)
	}
	if len(manifestEntries) == 0 {
		t.Fatal("expected at least one provenance manifest file")
	}

	manifestPath := filepath.Join(injectDir, manifestEntries[0].Name())
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read provenance manifest: %v", err)
	}
	var manifest provenanceManifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		t.Fatalf("parse provenance manifest: %v", err)
	}
	if manifest.Phase != "planning" {
		t.Fatalf("manifest phase = %q, want planning", manifest.Phase)
	}
}

func TestContextAssemble_IntelUsesRankedSignals(t *testing.T) {
	tmp := t.TempDir()
	for _, rel := range []string{
		filepath.Join(".agents", "findings"),
		filepath.Join(".agents", "planning-rules"),
		filepath.Join(".agents", "pre-mortem-checks"),
		filepath.Join(".agents", "rpi"),
	} {
		if err := os.MkdirAll(filepath.Join(tmp, rel), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	finding := `---
id: "f-startup-001"
title: "Rank startup context before recency"
status: "active"
severity: "high"
applicable_when: ["task","startup"]
scope_tags: ["startup","context"]
---
# Finding

Use ranked startup context instead of arbitrary recent documents.
`
	if err := os.WriteFile(filepath.Join(tmp, ".agents", "findings", "f-startup-001.md"), []byte(finding), 0o644); err != nil {
		t.Fatal(err)
	}

	rule := `---
id: "f-startup-001"
---
# Planning Rule

- Ask: Did startup context prefer ranked signals before recent artifacts?
`
	if err := os.WriteFile(filepath.Join(tmp, ".agents", "planning-rules", "f-startup-001.md"), []byte(rule), 0o644); err != nil {
		t.Fatal(err)
	}

	check := `---
id: "f-startup-001"
---
# Pre-Mortem Check

- Ask: Did startup context avoid arbitrary recency?
`
	if err := os.WriteFile(filepath.Join(tmp, ".agents", "pre-mortem-checks", "f-startup-001.md"), []byte(check), 0o644); err != nil {
		t.Fatal(err)
	}

	queue := `{"source_epic":"ag-amg","timestamp":"2026-04-01T22:00:00Z","items":[{"title":"Wire startup context to ranked packet","type":"feature","severity":"high","source":"research","description":"Use packet-ranked startup payloads","target_repo":"` + detectRepoName(tmp) + `","consumed":false,"claim_status":"available"}],"consumed":false,"claim_status":"available","claimed_by":null,"claimed_at":null,"consumed_by":null,"consumed_at":null}
`
	if err := os.WriteFile(filepath.Join(tmp, ".agents", "rpi", "next-work.jsonl"), []byte(queue), 0o644); err != nil {
		t.Fatal(err)
	}

	sections := assembleSectionsForPhase(tmp, "rank startup context before falling back to recency", "startup", defaultAssembleMaxChars)
	intelContent := sections[2].Content

	if !strings.Contains(intelContent, "### Planning Rules") {
		t.Fatalf("expected planning rules in ranked INTEL, got:\n%s", intelContent)
	}
	if !strings.Contains(intelContent, "### Relevant Next Work") {
		t.Fatalf("expected ranked next work in INTEL, got:\n%s", intelContent)
	}
	if !strings.Contains(intelContent, "Wire startup context to ranked packet") {
		t.Fatalf("expected ranked queue item in INTEL, got:\n%s", intelContent)
	}
}

func TestTruncateToCharBudget_RuneSafe(t *testing.T) {
	// Multi-byte characters: each emoji is multiple bytes but 1 rune.
	input := "Hello 🌍🌎🌏 World and more text to exceed budget"
	// Budget of 10 runes — verify no replacement characters (rune-safe).
	result := truncateToCharBudget(input, 10)
	for _, r := range result {
		if r == 0xFFFD { // Unicode replacement character = bad truncation
			t.Error("truncation produced replacement character, not rune-safe")
		}
	}
	// Result should be shorter than the original input.
	if len([]rune(result)) >= len([]rune(input)) {
		t.Error("truncated result should be shorter than input")
	}
}

func TestTruncateToCharBudget_ZeroBudget(t *testing.T) {
	if got := truncateToCharBudget("anything", 0); got != "" {
		t.Errorf("zero budget should return empty, got %q", got)
	}
}

func TestTruncateToCharBudget_UnderBudget(t *testing.T) {
	input := "short"
	if got := truncateToCharBudget(input, 100); got != input {
		t.Errorf("under-budget should return input unchanged, got %q", got)
	}
}

func TestShannonEntropy(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantMin float64
		wantMax float64
	}{
		{"empty", "", 0, 0},
		{"single char repeated", "aaaaaaa", 0, 0.01},
		{"low entropy", "aabb", 0.9, 1.1},
		{"high entropy hex", "a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6", 3.5, 5.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shannonEntropy(tt.input)
			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("shannonEntropy(%q) = %f, want [%f, %f]", tt.input, got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestRedactHighEntropy(t *testing.T) {
	// A 32-char high-entropy string (>4.5 bits/char) should be redacted.
	// Use a base64-like string with high character diversity.
	secret := "Kx9mPqR3vZ7wJ5nLtY2fBgC8hDsE4aUo"
	entropy := shannonEntropy(secret)
	if entropy <= 4.5 {
		t.Fatalf("test setup: secret entropy %.2f must be >4.5", entropy)
	}

	input := "prefix " + secret + " suffix"
	result, count := redactHighEntropy(input)
	if strings.Contains(result, secret) {
		t.Error("high-entropy string should be redacted")
	}
	if !strings.Contains(result, "[REDACTED: high-entropy]") {
		t.Error("expected [REDACTED: high-entropy] marker")
	}
	if count == 0 {
		t.Error("expected non-zero redaction count")
	}

	// A 32-char low-entropy string should NOT be redacted.
	lowEntropy := strings.Repeat("a", 32)
	result2, count2 := redactHighEntropy("before " + lowEntropy + " after")
	if !strings.Contains(result2, lowEntropy) {
		t.Error("low-entropy string should not be redacted")
	}
	if count2 != 0 {
		t.Errorf("expected 0 redactions for low-entropy, got %d", count2)
	}
}

func TestFormatHistoryEntry(t *testing.T) {
	entry := map[string]interface{}{
		"timestamp":     "2026-02-20T10:00:00Z",
		"cycle":         float64(3),
		"target":        "goals-proof",
		"result":        "improved",
		"sha":           "abc1234",
		"canonical_sha": "abc1234",
		"log_sha":       "def5678",
		"goals_passing": float64(17),
		"goals_total":   float64(18),
		"goal_ids":      []interface{}{"goals-proof", "cmd-ao-coverage-floor"},
		"summary":       "test cycle summary",
	}
	result := formatHistoryEntry(entry, 1)
	if !strings.Contains(result, "### Entry 1") {
		t.Error("expected entry header")
	}
	if !strings.Contains(result, "**timestamp**") {
		t.Error("expected timestamp field")
	}
	if !strings.Contains(result, "**target**: goals-proof") {
		t.Error("expected target field")
	}
	if !strings.Contains(result, "**result**: improved") {
		t.Error("expected result field")
	}
	if !strings.Contains(result, "**canonical_sha**: abc1234") {
		t.Error("expected canonical_sha field")
	}
	if !strings.Contains(result, "**log_sha**: def5678") {
		t.Error("expected log_sha field")
	}
	if !strings.Contains(result, "**goal_ids**: goals-proof, cmd-ao-coverage-floor") {
		t.Error("expected formatted goal_ids field")
	}
}

func TestFormatHistoryEntry_LegacyFallbacks(t *testing.T) {
	entry := map[string]interface{}{
		"timestamp": "2026-02-20T10:00:00Z",
		"cycle":     float64(4),
		"goal_id":   "legacy-goal",
		"status":    "pass",
	}

	result := formatHistoryEntry(entry, 2)
	if !strings.Contains(result, "**target**: legacy-goal") {
		t.Error("expected goal_id to render as target")
	}
	if !strings.Contains(result, "**result**: pass") {
		t.Error("expected status to render as result")
	}
}

func TestContextAssemble_CommandOutputsJSON(t *testing.T) {
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, ".agents", "learnings"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, ".agents", "learnings", "learn.md"), []byte("# Learn"), 0o644); err != nil {
		t.Fatal(err)
	}

	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = os.Chdir(oldWD)
	}()

	outPath := filepath.Join(tmp, ".agents", "rpi", "smoke-briefing.json")
	oldTask := assembleTask
	oldMax := assembleMaxChars
	oldOutput := assembleOutput
	oldMode := output

	assembleTask = "Smoke JSON task"
	assembleMaxChars = 12000
	assembleOutput = outPath
	output = "json"
	defer func() {
		assembleTask = oldTask
		assembleMaxChars = oldMax
		assembleOutput = oldOutput
		output = oldMode
	}()

	cmd := &cobra.Command{}
	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := runContextAssemble(cmd, nil); err != nil {
		t.Fatalf("runContextAssemble failed: %v", err)
	}

	var parsed assembleJSONOutput
	if err := json.Unmarshal(out.Bytes(), &parsed); err != nil {
		t.Fatalf("expected valid JSON output: %v", err)
	}
	if parsed.OutputPath != outPath {
		t.Fatalf("expected output_path=%q, got=%q", outPath, parsed.OutputPath)
	}
	if parsed.TotalChars <= 0 {
		t.Error("expected total_chars > 0")
	}
	if len(parsed.Sections) != 5 {
		t.Fatalf("expected 5 sections, got %d", len(parsed.Sections))
	}
}
