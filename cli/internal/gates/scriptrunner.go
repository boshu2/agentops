package gates

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/boshu2/agentops/cli/internal/ports"
)

// ScriptRunner runs a script-backed check and maps its exit code to a
// GateVerdict. Basename backings resolve under scripts/ (for example
// "check-cathedral-cut-conformance.py"). Path backings resolve from the repo root (for
// example "skills/skill-builder/scripts/heal.sh").
//
// It satisfies ports.GateRunnerPort, so the deterministic runner can dispatch
// ANY scripts/* gate — shell and Python — not only the check-* names the legacy
// productionGateRunner (ao gate run) resolves. Exit mapping: 0=PASS, 2=WARN,
// 75=SKIP, missing script=UNKNOWN, else=FAIL. Whether WARN is allowed is a
// property of the registered check, enforced by the orchestrator.
type ScriptRunner struct {
	repoRoot string
}

// NewScriptRunner returns a ScriptRunner rooted at repoRoot.
func NewScriptRunner(repoRoot string) *ScriptRunner { return &ScriptRunner{repoRoot: repoRoot} }

// gateSelfBinary resolves the ao binary currently executing the gate
// (production: os.Executable()). It is indirected as a package var so a test
// can simulate running as a differently-named binary — the AO_BIN self-injection
// below is guarded on the executable BASENAME, and under `go test`
// os.Executable() is the package test binary, not `ao`.
var gateSelfBinary = os.Executable

// aoBinInjection returns the AO_BIN value the gate should propagate to a spawned
// sub-check given the running executable path exe, and whether to inject at all.
//
// A caller may run a freshly built `ao gate check` while shell sub-checks would
// otherwise resolve a stale repository binary. Propagating the running binary
// keeps one deterministic check invocation internally consistent.
//
// The guard is on the basename SUFFIX, NOT a stat/regular-file test and NOT an
// exact basename=="ao" match: under `go test`,
// os.Executable() is the package test binary (`<pkg>.test`, a real regular file),
// so a stat guard would inject the test binary and make a sub-script run
// `"$AO_BIN" provenance verify` against a non-ao executable and fail spuriously.
// The OLD exact `== "ao"` guard over-narrowed: a renamed / wrapped / symlinked
// production binary (basename not literally "ao") then LOST self-injection and
// dropped back to a possibly-stale `$ROOT/cli/bin/ao` — the exact age-jmfl
// false-fail. Injecting the running gate binary is always MORE correct than
// letting a sub-script resolve an arbitrary one, so inject for ANY basename
// EXCEPT the go-test binary (`*.test`), which is the sole case that must be
// suppressed (production binaries never end in ".test").
func aoBinInjection(exe string) (string, bool) {
	if strings.HasSuffix(filepath.Base(exe), ".test") {
		return "", false
	}
	return exe, true
}

// buildCheckEnv composes the environment for a spawned sub-check. Precedence for
// AO_BIN (exactly one value reaches the child): a caller-set AO_BIN in reqEnv
// wins; else the running gate binary self-injects when it is named "ao"
// (age-jmfl); else AO_BIN is left untouched. base is the command's already-
// configured environment (typically cmd.Environ()).
func buildCheckEnv(base []string, reqEnv map[string]string) []string {
	aoBin, haveAOBin := "", false
	if v, ok := reqEnv["AO_BIN"]; ok {
		aoBin, haveAOBin = v, true // caller wins
	} else if exe, err := gateSelfBinary(); err == nil {
		aoBin, haveAOBin = aoBinInjection(exe)
	}

	env := base
	if haveAOBin {
		// Drop any ambient AO_BIN so the intended value is the single one the
		// child resolves (getenv-first-match is libc-dependent otherwise).
		env = stripEnvKey(env, "AO_BIN")
	}
	for k, v := range reqEnv {
		if k == "AO_BIN" {
			continue // re-appended below as the single authoritative value
		}
		env = append(env, k+"="+v)
	}
	if haveAOBin {
		env = append(env, "AO_BIN="+aoBin)
	}
	return env
}

// stripEnvKey returns env with every "key=..." entry removed. It allocates a new
// slice and does not mutate the input.
func stripEnvKey(env []string, key string) []string {
	prefix := key + "="
	out := make([]string, 0, len(env))
	for _, e := range env {
		if strings.HasPrefix(e, prefix) {
			continue
		}
		out = append(out, e)
	}
	return out
}

// Run executes the requested backing and returns its verdict.
func (s *ScriptRunner) Run(ctx context.Context, req ports.GateRunRequest) (ports.GateVerdict, error) {
	if err := ctx.Err(); err != nil {
		return ports.GateVerdict{}, err
	}
	name := string(req.Name)
	if name == "" {
		return ports.GateVerdict{Status: ports.GateStatusUnknown, Reason: "empty gate name"}, nil
	}
	script := filepath.Join(s.repoRoot, "scripts", name)
	if strings.Contains(name, "/") {
		script = filepath.Join(s.repoRoot, name)
	}
	if fi, err := os.Stat(script); err != nil || fi.IsDir() {
		return ports.GateVerdict{Status: ports.GateStatusUnknown, Reason: fmt.Sprintf("no script %s", script)}, nil
	}

	interpreter := backingInterpreter(script)
	if interpreter == "bash" {
		interpreter = resolveBash()
	}
	interpreterArgs := append([]string{script}, req.Args...)
	cmd := exec.CommandContext(ctx, interpreter, interpreterArgs...)
	cmd.Dir = s.repoRoot
	cmd.Env = buildCheckEnv(cmd.Environ(), req.Env)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	runErr := cmd.Run()

	code := 0
	if exitErr, ok := runErr.(*exec.ExitError); ok {
		code = exitErr.ExitCode()
	} else if runErr != nil {
		reason := fmt.Sprintf("subprocess error: %v", runErr)
		logTail := tailBytes(out.Bytes(), 4096)
		if strings.TrimSpace(logTail) == "" {
			logTail = reason
		}
		return ports.GateVerdict{
			Status:  ports.GateStatusUnknown,
			Reason:  reason,
			LogTail: logTail,
		}, nil
	}
	v := exitCodeToVerdict(code)
	v.LogTail = tailBytes(out.Bytes(), 4096)
	return v, nil
}

// resolveBash returns a bash interpreter, preferring a Homebrew bash (>= 4) on
// macOS so check scripts that use bash-4 features (declare -A, mapfile) don't
// silently misbehave under the system /bin/bash 3.2 (ag-qidx TX2). On Linux
// (bushido) these paths are absent and it falls back to PATH "bash" (5.x).
func resolveBash() string {
	for _, p := range []string{"/opt/homebrew/bin/bash", "/usr/local/bin/bash"} {
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
			return p
		}
	}
	return "bash"
}

// exitCodeToVerdict maps a script exit code to a GateVerdict (the canonical
// gate convention: 0 pass, 2 advisory warn, 75 structural skip, else fail).
func exitCodeToVerdict(code int) ports.GateVerdict {
	switch code {
	case 0:
		return ports.GateVerdict{Status: ports.GateStatusPass, Reason: "exit 0"}
	case 2:
		return ports.GateVerdict{Status: ports.GateStatusWarn, Reason: "exit 2 (advisory)"}
	case 75:
		return ports.GateVerdict{Status: ports.GateStatusSkip, Reason: "exit 75 (structural skip)"}
	}
	return ports.GateVerdict{Status: ports.GateStatusFail, Reason: fmt.Sprintf("exit %d", code)}
}

// tailBytes returns at most n trailing bytes of b, trimmed.
func tailBytes(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return strings.TrimSpace(string(b[len(b)-n:]))
}

var _ ports.GateRunnerPort = (*ScriptRunner)(nil)
