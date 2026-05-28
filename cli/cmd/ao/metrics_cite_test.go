// practices: [dora-metrics, sre]
package main

import (
	"os"
	"testing"

	"github.com/spf13/cobra"
)

func TestMetricsCite_DetectSessionID_FromEnv(t *testing.T) {
	const testID = "session-test-abc123"
	t.Setenv("CLAUDE_SESSION_ID", testID)

	got := detectSessionID()
	if got != testID {
		t.Errorf("detectSessionID() = %q, want %q", got, testID)
	}
}

func TestMetricsCite_DetectSessionID_NoEnv(t *testing.T) {
	t.Setenv("CLAUDE_SESSION_ID", "")

	got := detectSessionID()
	// Should return a canonical session ID with "session-" prefix
	if got == "" {
		t.Error("detectSessionID() returned empty string when no env var set")
	}
}

// findCiteSubcmd locates the "cite" subcommand from metricsCmd.
func findCiteSubcmd() *cobra.Command {
	for _, c := range metricsCmd.Commands() {
		if c.Use == "cite <artifact-path>" {
			return c
		}
	}
	return nil
}

func TestMetricsCite_MissingArtifact(t *testing.T) {
	dir := t.TempDir()

	testProjectDir = dir
	defer func() { testProjectDir = "" }()

	cmd := findCiteSubcmd()
	if cmd == nil {
		t.Skip("cite subcommand not found on metricsCmd")
	}

	// Point to a nonexistent artifact
	err := runMetricsCite(cmd, []string{"/nonexistent/artifact.md"})
	if err == nil {
		t.Fatal("expected error for missing artifact")
	}
}

func TestMetricsCite_DryRun(t *testing.T) {
	dir := t.TempDir()

	testProjectDir = dir
	defer func() { testProjectDir = "" }()

	// Create a test artifact file
	artifactPath := dir + "/test-artifact.md"
	if err := os.WriteFile(artifactPath, []byte("# Test"), 0o644); err != nil {
		t.Fatal(err)
	}

	oldDryRun := dryRun
	dryRun = true
	defer func() { dryRun = oldDryRun }()

	cmd := findCiteSubcmd()
	if cmd == nil {
		t.Skip("cite subcommand not found on metricsCmd")
	}

	err := runMetricsCite(cmd, []string{artifactPath})
	if err != nil {
		t.Fatalf("runMetricsCite dry-run: %v", err)
	}

	// Verify no citations file was created
	citationsPath := dir + "/.agents/ao/citations.jsonl"
	if _, err := os.Stat(citationsPath); !os.IsNotExist(err) {
		t.Error("expected no citations file in dry-run mode")
	}
}

func TestMetricsCite_ValidArtifact(t *testing.T) {
	dir := t.TempDir()

	testProjectDir = dir
	defer func() { testProjectDir = "" }()

	// Create directories for citations
	if err := os.MkdirAll(dir+"/.agents/ao", 0o755); err != nil {
		t.Fatal(err)
	}

	// Create a test artifact file
	artifactPath := dir + "/test-artifact.md"
	if err := os.WriteFile(artifactPath, []byte("# Test"), 0o644); err != nil {
		t.Fatal(err)
	}

	oldDryRun := dryRun
	dryRun = false
	defer func() { dryRun = oldDryRun }()

	cmd := findCiteSubcmd()
	if cmd == nil {
		t.Skip("cite subcommand not found on metricsCmd")
	}

	err := runMetricsCite(cmd, []string{artifactPath})
	if err != nil {
		t.Fatalf("runMetricsCite failed: %v", err)
	}
}

// ---------------------------------------------------------------------------
// detectModelVendor
// ---------------------------------------------------------------------------

func TestDetectModelVendor_Codex(t *testing.T) {
	t.Setenv("CODEX_SESSION", "sess-123")
	t.Setenv("CODEX_SANDBOX_TYPE", "")
	t.Setenv("CLAUDE_CODE_SESSION", "")
	t.Setenv("CLAUDE_SESSION_ID", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("ANTHROPIC_API_KEY", "")

	got := detectModelVendor()
	if got != "codex" {
		t.Errorf("detectModelVendor() = %q, want %q", got, "codex")
	}
}

func TestDetectModelVendor_CodexSandbox(t *testing.T) {
	t.Setenv("CODEX_SESSION", "")
	t.Setenv("CODEX_SANDBOX_TYPE", "docker")
	t.Setenv("CLAUDE_CODE_SESSION", "")
	t.Setenv("CLAUDE_SESSION_ID", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("ANTHROPIC_API_KEY", "")

	got := detectModelVendor()
	if got != "codex" {
		t.Errorf("detectModelVendor() = %q, want %q", got, "codex")
	}
}

func TestDetectModelVendor_Claude(t *testing.T) {
	t.Setenv("CODEX_SESSION", "")
	t.Setenv("CODEX_SANDBOX_TYPE", "")
	t.Setenv("CLAUDE_CODE_SESSION", "active")
	t.Setenv("CLAUDE_SESSION_ID", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("ANTHROPIC_API_KEY", "")

	got := detectModelVendor()
	if got != "claude" {
		t.Errorf("detectModelVendor() = %q, want %q", got, "claude")
	}
}

func TestDetectModelVendor_ClaudeSessionID(t *testing.T) {
	t.Setenv("CODEX_SESSION", "")
	t.Setenv("CODEX_SANDBOX_TYPE", "")
	t.Setenv("CLAUDE_CODE_SESSION", "")
	t.Setenv("CLAUDE_SESSION_ID", "sess-claude")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("ANTHROPIC_API_KEY", "")

	got := detectModelVendor()
	if got != "claude" {
		t.Errorf("detectModelVendor() = %q, want %q", got, "claude")
	}
}

func TestDetectModelVendor_OpenAIKeyOnly(t *testing.T) {
	t.Setenv("CODEX_SESSION", "")
	t.Setenv("CODEX_SANDBOX_TYPE", "")
	t.Setenv("CLAUDE_CODE_SESSION", "")
	t.Setenv("CLAUDE_SESSION_ID", "")
	t.Setenv("OPENAI_API_KEY", "sk-test")
	t.Setenv("ANTHROPIC_API_KEY", "")

	got := detectModelVendor()
	if got != "codex" {
		t.Errorf("detectModelVendor() = %q, want %q", got, "codex")
	}
}

func TestDetectModelVendor_AnthropicKeyOnly(t *testing.T) {
	t.Setenv("CODEX_SESSION", "")
	t.Setenv("CODEX_SANDBOX_TYPE", "")
	t.Setenv("CLAUDE_CODE_SESSION", "")
	t.Setenv("CLAUDE_SESSION_ID", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("ANTHROPIC_API_KEY", "sk-ant-test")

	got := detectModelVendor()
	if got != "claude" {
		t.Errorf("detectModelVendor() = %q, want %q", got, "claude")
	}
}

func TestDetectModelVendor_Unknown(t *testing.T) {
	t.Setenv("CODEX_SESSION", "")
	t.Setenv("CODEX_SANDBOX_TYPE", "")
	t.Setenv("CLAUDE_CODE_SESSION", "")
	t.Setenv("CLAUDE_SESSION_ID", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("ANTHROPIC_API_KEY", "")

	got := detectModelVendor()
	if got != "" {
		t.Errorf("detectModelVendor() = %q, want empty", got)
	}
}

func TestDetectModelVendor_BothKeys(t *testing.T) {
	t.Setenv("CODEX_SESSION", "")
	t.Setenv("CODEX_SANDBOX_TYPE", "")
	t.Setenv("CLAUDE_CODE_SESSION", "")
	t.Setenv("CLAUDE_SESSION_ID", "")
	t.Setenv("OPENAI_API_KEY", "sk-test")
	t.Setenv("ANTHROPIC_API_KEY", "sk-ant-test")

	got := detectModelVendor()
	// Both keys set, neither condition matches (one requires the other to be empty)
	if got != "" {
		t.Errorf("detectModelVendor() = %q, want empty (both keys set)", got)
	}
}
