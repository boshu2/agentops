// practices: [wiki-knowledge-surface, design-by-contract]
package agentslint

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRun_MissingScript(t *testing.T) {
	err := Run(Options{Script: filepath.Join(t.TempDir(), "missing.sh")})
	if err == nil {
		t.Fatal("expected error when script is missing")
	}
	if !strings.Contains(err.Error(), "lint script not found") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRun_PassthroughExitCodes(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		wantCode int
	}{
		{
			name:     "exit zero passes",
			body:     "#!/usr/bin/env bash\nexit 0\n",
			wantCode: 0,
		},
		{
			name:     "exit one surfaces as Error 1",
			body:     "#!/usr/bin/env bash\necho violation >&2\nexit 1\n",
			wantCode: 1,
		},
		{
			name:     "exit two surfaces as Error 2",
			body:     "#!/usr/bin/env bash\necho misuse >&2\nexit 2\n",
			wantCode: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scriptPath := filepath.Join(t.TempDir(), "fake-lint.sh")
			if err := os.WriteFile(scriptPath, []byte(tt.body), 0o755); err != nil {
				t.Fatal(err)
			}

			var stdout, stderr bytes.Buffer
			err := Run(Options{
				Script: scriptPath,
				Stdout: &stdout,
				Stderr: &stderr,
			})
			if tt.wantCode == 0 {
				if err != nil {
					t.Fatalf("expected nil, got %v", err)
				}
				return
			}
			var lintErr *Error
			if !errors.As(err, &lintErr) {
				t.Fatalf("expected *Error, got %T: %v", err, err)
			}
			if lintErr.ExitCode != tt.wantCode {
				t.Errorf("ExitCode = %d, want %d", lintErr.ExitCode, tt.wantCode)
			}
			if lintErr.Script != scriptPath {
				t.Errorf("Script = %q, want %q", lintErr.Script, scriptPath)
			}
		})
	}
}

func TestRun_ForwardsJSONFlag(t *testing.T) {
	scriptPath := filepath.Join(t.TempDir(), "echo-json.sh")
	body := "#!/usr/bin/env bash\nif [ \"$1\" = \"--json\" ]; then echo '{\"got\":\"json\"}'; else echo 'no flag'; fi\n"
	if err := os.WriteFile(scriptPath, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	if err := Run(Options{Script: scriptPath, JSON: true, Stdout: &stdout}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	var got map[string]string
	if err := json.Unmarshal(bytes.TrimSpace(stdout.Bytes()), &got); err != nil {
		t.Fatalf("stdout not JSON: %v\nGot: %s", err, stdout.String())
	}
	if got["got"] != "json" {
		t.Errorf("script did not receive --json flag (stdout: %s)", stdout.String())
	}
}

func TestError_Message(t *testing.T) {
	err := &Error{ExitCode: 7, Script: "/path/to/lint.sh"}
	got := err.Error()
	if !strings.Contains(got, "/path/to/lint.sh") {
		t.Errorf("message missing script path: %q", got)
	}
	if !strings.Contains(got, "7") {
		t.Errorf("message missing exit code: %q", got)
	}
}
