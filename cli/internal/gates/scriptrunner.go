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

// ScriptRunner runs a shell-backed check and maps its exit code to a
// GateVerdict. Basename backings resolve under scripts/ (for example
// "check-registry-drift.sh"). Path backings resolve from the repo root (for
// example "skills/heal-skill/scripts/heal.sh").
//
// It satisfies ports.GateRunnerPort, so the orchestrator can shell to ANY
// scripts/* gate — both check-*.sh and validate-*.sh — not only the check-*
// names the legacy productionGateRunner (ao gate run) resolves. Exit mapping:
// 0=PASS, 2=WARN, 75=SKIP, missing script=UNKNOWN, else=FAIL.
type ScriptRunner struct {
	repoRoot string
}

// NewScriptRunner returns a ScriptRunner rooted at repoRoot.
func NewScriptRunner(repoRoot string) *ScriptRunner { return &ScriptRunner{repoRoot: repoRoot} }

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

	bashArgs := append([]string{script}, req.Args...)
	cmd := exec.CommandContext(ctx, resolveBash(), bashArgs...)
	cmd.Dir = s.repoRoot
	if len(req.Env) > 0 {
		env := cmd.Environ()
		for k, v := range req.Env {
			env = append(env, k+"="+v)
		}
		cmd.Env = env
	}
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	runErr := cmd.Run()

	code := 0
	if exitErr, ok := runErr.(*exec.ExitError); ok {
		code = exitErr.ExitCode()
	} else if runErr != nil {
		return ports.GateVerdict{
			Status:  ports.GateStatusUnknown,
			Reason:  fmt.Sprintf("subprocess error: %v", runErr),
			LogTail: tailBytes(out.Bytes(), 4096),
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
