package gates

import (
	"context"
	"os"
	"path/filepath"
	"strings"
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

// setGateSelfBinary overrides the running-gate-binary resolver for the test and
// restores it via t.Cleanup (package-global shared state — .claude/rules/go.md).
func setGateSelfBinary(t *testing.T, fn func() (string, error)) {
	t.Helper()
	prev := gateSelfBinary
	gateSelfBinary = fn
	t.Cleanup(func() { gateSelfBinary = prev })
}

// childAOBin runs the echo-aobin.sh sub-check under root and returns the AO_BIN
// value the spawned process actually observed (the load-bearing behavior: what
// the child resolves, not what the parent intended).
func childAOBin(t *testing.T, root string) (ports.GateVerdict, string) {
	t.Helper()
	writeScript(t, root, "echo-aobin.sh", "#!/usr/bin/env bash\necho \"AO_BIN=[${AO_BIN:-}]\"\nexit 0\n")
	v, err := NewScriptRunner(root).Run(context.Background(), ports.GateRunRequest{Name: "echo-aobin.sh"})
	if err != nil {
		t.Fatalf("Run echo-aobin.sh: %v", err)
	}
	start := strings.Index(v.LogTail, "AO_BIN=[")
	if start < 0 {
		t.Fatalf("sub-check did not report AO_BIN; log: %q", v.LogTail)
	}
	rest := v.LogTail[start+len("AO_BIN=["):]
	end := strings.Index(rest, "]")
	if end < 0 {
		t.Fatalf("malformed AO_BIN report; log: %q", v.LogTail)
	}
	return v, rest[:end]
}

// TestScriptRunner_InjectsGateSelfBinaryAsAOBin proves the age-jmfl fix: when the
// gate runs as an `ao` binary, every spawned sub-check RECEIVES that same binary
// via AO_BIN. Asserted by BEHAVIOR/name (basename "ao", equal to the running
// binary), never by encoding a fixed test-binary path.
func TestScriptRunner_InjectsGateSelfBinaryAsAOBin(t *testing.T) {
	// A STALE ambient AO_BIN (the exact false-fail source: an old cli/bin/ao) must
	// be overridden by the fresh gate binary — assert the child never sees it.
	t.Setenv("AO_BIN", "/stale/cli/bin/ao")
	// Simulate the pre-push reality: the gate is a freshly-built binary named "ao".
	freshAo := filepath.Join(t.TempDir(), "ao")
	setGateSelfBinary(t, func() (string, error) { return freshAo, nil })

	v, got := childAOBin(t, t.TempDir())
	if v.Status != ports.GateStatusPass {
		t.Fatalf("status = %s, want PASS; log: %q", v.Status, v.LogTail)
	}
	if got != freshAo {
		t.Fatalf("child AO_BIN = %q, want the running gate binary %q", got, freshAo)
	}
	if filepath.Base(got) != "ao" {
		t.Fatalf("injected AO_BIN basename = %q, want \"ao\"", filepath.Base(got))
	}
}

// TestScriptRunner_CallerAOBinWins proves req.Env AO_BIN beats self-injection —
// even when the gate runs as an `ao` binary that WOULD otherwise inject.
func TestScriptRunner_CallerAOBinWins(t *testing.T) {
	t.Setenv("AO_BIN", "/stale/ao")
	setGateSelfBinary(t, func() (string, error) { return filepath.Join(t.TempDir(), "ao"), nil })

	callerAo := filepath.Join(t.TempDir(), "caller", "ao")
	root := t.TempDir()
	writeScript(t, root, "echo-aobin.sh", "#!/usr/bin/env bash\necho \"AO_BIN=[${AO_BIN:-}]\"\nexit 0\n")
	v, err := NewScriptRunner(root).Run(context.Background(), ports.GateRunRequest{
		Name: "echo-aobin.sh",
		Env:  map[string]string{"AO_BIN": callerAo},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if want := "AO_BIN=[" + callerAo + "]"; !strings.Contains(v.LogTail, want) {
		t.Fatalf("caller AO_BIN did not win: log %q, want substring %q", v.LogTail, want)
	}
}

// TestScriptRunner_NameGuardSuppressesInjectionUnderGoTest proves the CRITICAL
// guard: with a non-"ao" executable (exactly the `go test` binary case), NOTHING
// is injected — so a stat-based guard's spurious `"$AO_BIN" provenance verify`
// failure can never happen under the test binary. This is why the existing gate
// tests stay green.
func TestScriptRunner_NameGuardSuppressesInjectionUnderGoTest(t *testing.T) {
	// Empty ambient AO_BIN → the child must observe empty (nothing injected).
	t.Setenv("AO_BIN", "")
	// The real os.Executable() under `go test` (basename "gates.test") — no override.
	_, got := childAOBin(t, t.TempDir())
	if got != "" {
		t.Fatalf("AO_BIN injected under a non-ao executable: %q (guard must suppress)", got)
	}
}

// TestBuildCheckEnv composes exactly one AO_BIN per the age-jmfl precedence.
func TestBuildCheckEnv(t *testing.T) {
	countAOBin := func(env []string) (int, string) {
		n, last := 0, ""
		for _, e := range env {
			if strings.HasPrefix(e, "AO_BIN=") {
				n++
				last = strings.TrimPrefix(e, "AO_BIN=")
			}
		}
		return n, last
	}

	t.Run("self-inject-strips-ambient", func(t *testing.T) {
		setGateSelfBinary(t, func() (string, error) { return "/fresh/ao", nil })
		env := buildCheckEnv([]string{"PATH=/usr/bin", "AO_BIN=/stale/cli/bin/ao"}, nil)
		n, v := countAOBin(env)
		if n != 1 || v != "/fresh/ao" {
			t.Fatalf("self-inject: count=%d value=%q, want 1 /fresh/ao", n, v)
		}
	})

	t.Run("caller-wins-over-self-and-ambient", func(t *testing.T) {
		setGateSelfBinary(t, func() (string, error) { return "/fresh/ao", nil })
		env := buildCheckEnv([]string{"AO_BIN=/stale/ao"}, map[string]string{"AO_BIN": "/caller/ao"})
		n, v := countAOBin(env)
		if n != 1 || v != "/caller/ao" {
			t.Fatalf("caller-wins: count=%d value=%q, want 1 /caller/ao", n, v)
		}
	})

	t.Run("name-guard-leaves-ambient-untouched", func(t *testing.T) {
		setGateSelfBinary(t, func() (string, error) { return "/some/gates.test", nil })
		env := buildCheckEnv([]string{"AO_BIN=/ambient/ao"}, nil)
		n, v := countAOBin(env)
		if n != 1 || v != "/ambient/ao" {
			t.Fatalf("name-guard: count=%d value=%q, want the ambient value untouched", n, v)
		}
	})
}

// TestAOBinInjection is the pure name-guard unit: only a basename of "ao" injects.
func TestAOBinInjection(t *testing.T) {
	cases := []struct {
		exe     string
		wantVal string
		wantOK  bool
	}{
		{"/tmp/ao-gate.XXXX/ao", "/tmp/ao-gate.XXXX/ao", true},
		{"/usr/local/bin/ao", "/usr/local/bin/ao", true},
		{"/repo/cli/internal/gates/gates.test", "", false},
		{"/tmp/go-buildXXXX/b001/ao.test", "", false},
	}
	for _, c := range cases {
		gotVal, gotOK := aoBinInjection(c.exe)
		if gotVal != c.wantVal || gotOK != c.wantOK {
			t.Errorf("aoBinInjection(%q) = (%q,%v), want (%q,%v)", c.exe, gotVal, gotOK, c.wantVal, c.wantOK)
		}
	}
}
