package gates

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/boshu2/agentops/cli/internal/ports"
)

func TestResolveBash(t *testing.T) {
	b := resolveBash()
	if b == "" {
		t.Fatal("resolveBash returned empty")
	}
	if b != "bash" {
		if _, err := os.Stat(b); err != nil {
			t.Errorf("resolveBash returned %q which is not a file: %v", b, err)
		}
	}
}

func TestExitCodeToVerdict(t *testing.T) {
	cases := map[int]ports.GateStatus{
		0:   ports.GateStatusPass,
		2:   ports.GateStatusWarn,
		75:  ports.GateStatusSkip,
		1:   ports.GateStatusFail,
		70:  ports.GateStatusFail, // bash4-guard's EX_SOFTWARE must read as FAIL, never PASS
		127: ports.GateStatusFail,
	}
	for code, want := range cases {
		if got := exitCodeToVerdict(code).Status; got != want {
			t.Errorf("exit %d -> %s, want %s", code, got, want)
		}
	}
}

func writeScript(t *testing.T, root, name, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, "scripts"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "scripts", name), []byte(body), 0o755); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

func TestScriptRunner_MissingScriptIsUnknown(t *testing.T) {
	r := NewScriptRunner(t.TempDir())
	v, err := r.Run(context.Background(), ports.GateRunRequest{Name: "does-not-exist.sh"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if v.Status != ports.GateStatusUnknown {
		t.Errorf("missing script -> %s, want UNKNOWN", v.Status)
	}
}

func TestScriptRunner_PassAndFail(t *testing.T) {
	root := t.TempDir()
	writeScript(t, root, "ok.sh", "#!/usr/bin/env bash\nexit 0\n")
	writeScript(t, root, "bad.sh", "#!/usr/bin/env bash\nexit 1\n")
	r := NewScriptRunner(root)

	v, err := r.Run(context.Background(), ports.GateRunRequest{Name: "ok.sh"})
	if err != nil {
		t.Fatalf("Run ok: %v", err)
	}
	if v.Status != ports.GateStatusPass {
		t.Errorf("ok.sh -> %s, want PASS", v.Status)
	}

	v, err = r.Run(context.Background(), ports.GateRunRequest{Name: "bad.sh"})
	if err != nil {
		t.Fatalf("Run bad: %v", err)
	}
	if v.Status != ports.GateStatusFail {
		t.Errorf("bad.sh -> %s, want FAIL", v.Status)
	}
}
