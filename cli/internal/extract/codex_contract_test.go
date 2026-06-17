package extract

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestCodexContract_LiveSchemaAccepted is the GROUND-TRUTH windshield for the
// codex strict-mode schema contract (age-nzx). It compiles the real
// agentops_provenance template to a schema file and invokes a real
// `codex exec --output-schema` turn, asserting codex does NOT reject the schema
// with a 400 invalid_json_schema. This is the only test that proves the
// CompileSchema output is actually codex-valid against live codex (the
// ValidateCodexStrictSchema unit tests prove it against our model of the
// contract; this proves the model is right).
//
// Opt-in: it bills the codex sub, so it skips unless AGENTOPS_CODEX_CONTRACT_TEST=1
// AND `codex` is on PATH. Law 0: codex exec only — never claude -p.
func TestCodexContract_LiveSchemaAccepted(t *testing.T) {
	if os.Getenv("AGENTOPS_CODEX_CONTRACT_TEST") != "1" {
		t.Skip("set AGENTOPS_CODEX_CONTRACT_TEST=1 to run the live codex contract smoke")
	}
	if _, err := exec.LookPath("codex"); err != nil {
		t.Skip("codex not on PATH; skipping live contract smoke")
	}

	tmpl, err := LoadProvenanceTemplate()
	if err != nil {
		t.Fatalf("LoadProvenanceTemplate: %v", err)
	}
	schema, err := CompileSchema(tmpl)
	if err != nil {
		t.Fatalf("CompileSchema: %v", err)
	}
	// Belt-and-suspenders: our own validator must agree before we spend a turn.
	if err := ValidateCodexStrictSchema(schema); err != nil {
		t.Fatalf("compiled schema failed local strict validator (would 400 live): %v", err)
	}

	dir := t.TempDir()
	schemaPath := filepath.Join(dir, "schema.json")
	if err := os.WriteFile(schemaPath, schema, 0o600); err != nil {
		t.Fatalf("write schema: %v", err)
	}
	msgPath := filepath.Join(dir, "last-message.txt")

	args := []string{
		"exec",
		"--dangerously-bypass-approvals-and-sandbox",
		"--skip-git-repo-check",
		"--output-schema", schemaPath,
		"--output-last-message", msgPath,
		`Return exactly this JSON and nothing else: {"entities":[],"relations":[]}`,
	}
	cmd := exec.Command("codex", args...)
	cmd.Stdin = strings.NewReader("")

	done := make(chan struct{})
	var combined []byte
	var runErr error
	go func() {
		combined, runErr = cmd.CombinedOutput()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Minute):
		_ = cmd.Process.Kill()
		t.Fatalf("codex exec timed out after 3m (combined so far:\n%s)", combined)
	}

	out := string(combined)
	lower := strings.ToLower(out)
	// The whole point: the fixed schema must NOT be rejected by codex.
	if strings.Contains(lower, "invalid_json_schema") || strings.Contains(lower, "status 400") || strings.Contains(out, "400") && strings.Contains(lower, "schema") {
		// Surface the offending line for the report.
		for _, line := range strings.Split(out, "\n") {
			ll := strings.ToLower(line)
			if strings.Contains(ll, "invalid_json_schema") || strings.Contains(ll, "400") {
				t.Fatalf("codex REJECTED the compiled schema: %s\n(full output:\n%s)", strings.TrimSpace(line), out)
			}
		}
		t.Fatalf("codex rejected the compiled schema (400/invalid_json_schema):\n%s", out)
	}
	if runErr != nil {
		// A non-schema error (auth, network) is not a contract failure but we
		// must not silently pass — surface it.
		t.Fatalf("codex exec errored (not a 400 schema rejection — check auth/network): %v\n%s", runErr, out)
	}
	t.Logf("codex accepted the compiled agentops_provenance schema (no 400). last-message file: %s", msgPath)
}
