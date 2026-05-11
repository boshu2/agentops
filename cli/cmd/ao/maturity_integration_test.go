// practices: [cmm-process-maturity, dora-metrics]
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMaturity_Integration_ScanWithLearnings(t *testing.T) {
	dir := chdirTemp(t)
	setupAgentsDir(t, dir)

	// Create learnings with frontmatter at various maturity levels
	writeFile(t, filepath.Join(dir, ".agents", "learnings", "L001-provisional.md"), `---
utility: 0.5000
maturity: provisional
confidence: 0.0000
reward_count: 0
helpful_count: 0
harmful_count: 0
---
# Provisional Learning
This is a provisional learning for testing.
`)

	writeFile(t, filepath.Join(dir, ".agents", "learnings", "L002-candidate.md"), `---
utility: 0.6000
maturity: candidate
confidence: 0.5000
reward_count: 4
helpful_count: 4
harmful_count: 0
---
# Candidate Learning
This is a candidate learning for testing.
`)

	writeFile(t, filepath.Join(dir, ".agents", "learnings", "L003-established.md"), `---
utility: 0.8000
maturity: established
confidence: 0.8000
reward_count: 10
helpful_count: 9
harmful_count: 1
---
# Established Learning
This is an established learning for testing.
`)

	// Run maturity --scan
	out, err := captureStdout(t, func() error {
		// Reset flags
		maturityScan = true
		maturityApply = false
		maturityCurate = false
		maturityExpire = false
		maturityEvict = false
		maturityMigrateMd = false
		maturityRecalibrate = false
		defer func() { maturityScan = false }()
		rootCmd.SetArgs([]string{"maturity", "--scan"})
		return rootCmd.Execute()
	})
	if err != nil {
		t.Fatalf("maturity --scan failed: %v\noutput: %s", err, out)
	}

	// Scan output must contain maturity distribution summary
	if !strings.Contains(out, "Maturity Distribution") {
		t.Errorf("expected 'Maturity Distribution' header in scan output, got: %s", out)
	}
	if !strings.Contains(out, "Provisional") {
		t.Errorf("expected 'Provisional' count in scan output, got: %s", out)
	}
	if !strings.Contains(out, "Candidate") {
		t.Errorf("expected 'Candidate' count in scan output, got: %s", out)
	}
	if !strings.Contains(out, "Established") {
		t.Errorf("expected 'Established' count in scan output, got: %s", out)
	}
}

func TestMaturity_Integration_EmptyStore(t *testing.T) {
	dir := chdirTemp(t)

	// No .agents directory at all
	_ = dir

	out, err := captureStdout(t, func() error {
		maturityScan = true
		defer func() { maturityScan = false }()
		rootCmd.SetArgs([]string{"maturity", "--scan"})
		return rootCmd.Execute()
	})
	if err != nil {
		t.Fatalf("maturity --scan with empty store should not error: %v", err)
	}
	if !strings.Contains(out, "No learnings") {
		t.Errorf("expected 'No learnings' message for empty store, got: %s", out)
	}
}

func TestMaturity_Integration_SingleLearningCheck(t *testing.T) {
	dir := chdirTemp(t)
	setupAgentsDir(t, dir)

	writeFile(t, filepath.Join(dir, ".agents", "learnings", "L001-test.md"), `---
utility: 0.5000
maturity: provisional
confidence: 0.0000
reward_count: 0
helpful_count: 0
harmful_count: 0
---
# Test Learning
A test learning.
`)

	out, err := captureStdout(t, func() error {
		maturityScan = false
		maturityApply = false
		rootCmd.SetArgs([]string{"maturity", "L001-test"})
		return rootCmd.Execute()
	})
	if err != nil {
		t.Fatalf("maturity L001-test failed: %v\noutput: %s", err, out)
	}

	// Should output maturity information for the single learning
	if out == "" {
		t.Error("expected non-empty output for single learning maturity check")
	}
}

func TestMaturity_Integration_NoArgsNoFlags(t *testing.T) {
	dir := chdirTemp(t)
	setupAgentsDir(t, dir)

	// Create a learning so directories exist
	writeFile(t, filepath.Join(dir, ".agents", "learnings", "L001.md"), `---
utility: 0.5000
maturity: provisional
---
# Learning
`)

	// No args and no --scan should produce an error
	_, err := captureStdout(t, func() error {
		maturityScan = false
		maturityApply = false
		maturityCurate = false
		maturityExpire = false
		maturityEvict = false
		maturityMigrateMd = false
		maturityRecalibrate = false
		rootCmd.SetArgs([]string{"maturity"})
		return rootCmd.Execute()
	})
	if err == nil {
		t.Fatal("expected error when no args and no --scan, got nil")
	}
	if !strings.Contains(err.Error(), "must provide learning-id or use --scan") {
		t.Errorf("expected 'must provide learning-id or use --scan' error, got: %v", err)
	}
}

func TestMaturity_Integration_ScanPendingTransitions(t *testing.T) {
	resetCommandState(t)
	t.Setenv("AGENTOPS_RPI_RUNTIME", "")
	dir := chdirTemp(t)
	setupAgentsDir(t, dir)

	// Create a learning that meets candidate transition criteria:
	// utility >= 0.55 AND reward_count >= 3
	writeFile(t, filepath.Join(dir, ".agents", "learnings", "L001-ready.md"), `---
utility: 0.5500
maturity: provisional
confidence: 0.0000
reward_count: 3
helpful_count: 3
harmful_count: 0
---
# Ready for Transition
This learning meets candidate promotion criteria.
`)

	out, err := captureStdout(t, func() error {
		maturityScan = true
		maturityApply = false
		defer func() { maturityScan = false }()
		rootCmd.SetArgs([]string{"maturity", "--scan"})
		return rootCmd.Execute()
	})
	if err != nil {
		t.Fatalf("maturity --scan failed: %v\noutput: %s", err, out)
	}

	// Scan output includes pending transitions section
	if !strings.Contains(out, "Pending Transitions") {
		t.Errorf("expected 'Pending Transitions' section in output, got: %s", out)
	}
	// The learning should be identified for transition
	if !strings.Contains(out, "L001-ready") {
		t.Errorf("expected learning ID L001-ready in scan output, got: %s", out)
	}
	// Should show the current maturity level
	if !strings.Contains(out, "provisional") {
		t.Errorf("expected 'provisional' in scan output, got: %s", out)
	}
	// Should show the target maturity level
	if !strings.Contains(out, "candidate") {
		t.Errorf("expected 'candidate' target in scan output, got: %s", out)
	}
}

func TestMaturity_Integration_MigrateMd(t *testing.T) {
	dir := chdirTemp(t)
	setupAgentsDir(t, dir)

	// Create a markdown learning WITHOUT frontmatter utility field
	writeFile(t, filepath.Join(dir, ".agents", "learnings", "L001-bare.md"), `# Bare Learning
This learning has no frontmatter.
`)

	out, err := captureStdout(t, func() error {
		maturityMigrateMd = true
		maturityScan = false
		defer func() { maturityMigrateMd = false }()
		rootCmd.SetArgs([]string{"maturity", "--migrate-md"})
		return rootCmd.Execute()
	})
	if err != nil {
		t.Fatalf("maturity --migrate-md failed: %v\noutput: %s", err, out)
	}
	if !strings.Contains(out, "Migrated") {
		t.Errorf("expected 'Migrated' in output, got: %s", out)
	}

	// Verify the file now has frontmatter
	content, readErr := os.ReadFile(filepath.Join(dir, ".agents", "learnings", "L001-bare.md"))
	if readErr != nil {
		t.Fatalf("read migrated file: %v", readErr)
	}
	if !strings.Contains(string(content), "utility:") {
		t.Errorf("expected frontmatter with utility field after migration, got:\n%s", content)
	}
}
