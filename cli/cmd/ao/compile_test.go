package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestResolveCompileMode(t *testing.T) {
	resetCommandState(t)

	mode, err := resolveCompileMode()
	if err != nil {
		t.Fatalf("resolveCompileMode default: %v", err)
	}
	if mode != "full" {
		t.Fatalf("default mode = %q, want full", mode)
	}

	compileOnly = true
	mode, err = resolveCompileMode()
	if err != nil {
		t.Fatalf("resolveCompileMode compile-only: %v", err)
	}
	if mode != "compile-only" {
		t.Fatalf("compile-only mode = %q", mode)
	}

	compileLintOnly = true
	if _, err := resolveCompileMode(); err == nil {
		t.Fatal("expected error for multiple mode flags")
	}
}

func TestRunCompileFullOrchestratesPhases(t *testing.T) {
	resetCommandState(t)
	tmp := t.TempDir()
	testProjectDir = tmp
	t.Cleanup(func() { testProjectDir = "" })

	var calls []string
	var gotScript compileScriptOptions
	stubCompileRunners(t,
		func(cwd, since string, quiet bool) error {
			calls = append(calls, "mine:"+cwd+":"+since)
			if quiet {
				t.Fatal("quiet should default false")
			}
			return nil
		},
		func(ctx context.Context, cwd string, opts compileScriptOptions, stdout, stderr io.Writer) error {
			calls = append(calls, "script:"+cwd)
			gotScript = opts
			return nil
		},
		func(cwd string, dryRun bool) error {
			calls = append(calls, "defrag:"+cwd)
			if dryRun {
				t.Fatal("dryRun should default false")
			}
			return nil
		},
	)

	output = "json"
	compileRuntime = "ollama"
	var out, errOut bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)

	if err := runCompile(cmd, nil); err != nil {
		t.Fatalf("runCompile: %v", err)
	}

	wantCalls := []string{"mine:" + tmp + ":26h", "script:" + tmp, "defrag:" + tmp}
	if !reflect.DeepEqual(calls, wantCalls) {
		t.Fatalf("calls = %#v, want %#v", calls, wantCalls)
	}
	if gotScript.Sources != ".agents" || gotScript.Output != ".agents/compiled" {
		t.Fatalf("script paths = %+v", gotScript)
	}
	if gotScript.Runtime != "ollama" || !gotScript.Incremental || gotScript.Force || gotScript.LintOnly {
		t.Fatalf("script opts = %+v", gotScript)
	}

	var report compileReport
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatalf("compile JSON invalid: %v\n%s", err, out.String())
	}
	if report.Mode != "full" || len(report.Phases) != 3 {
		t.Fatalf("report = %+v", report)
	}
	if !strings.Contains(errOut.String(), "Compile mine") {
		t.Fatalf("expected progress on stderr for JSON output, got %q", errOut.String())
	}
}

func TestRunCompileLintOnlySkipsMineAndDefrag(t *testing.T) {
	resetCommandState(t)
	tmp := t.TempDir()
	testProjectDir = tmp
	t.Cleanup(func() { testProjectDir = "" })

	var calls []string
	var gotScript compileScriptOptions
	stubCompileRunners(t,
		func(cwd, since string, quiet bool) error {
			calls = append(calls, "mine")
			return nil
		},
		func(ctx context.Context, cwd string, opts compileScriptOptions, stdout, stderr io.Writer) error {
			calls = append(calls, "script")
			gotScript = opts
			return nil
		},
		func(cwd string, dryRun bool) error {
			calls = append(calls, "defrag")
			return nil
		},
	)

	compileLintOnly = true
	compileOutputDir = "wiki"
	cmd := &cobra.Command{}
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	if err := runCompile(cmd, nil); err != nil {
		t.Fatalf("runCompile lint-only: %v", err)
	}
	if !reflect.DeepEqual(calls, []string{"script"}) {
		t.Fatalf("calls = %#v, want script only", calls)
	}
	if !gotScript.LintOnly || gotScript.Output != "wiki" {
		t.Fatalf("script opts = %+v", gotScript)
	}
}

func TestMaterializeCompileScriptNormalizesCRLF(t *testing.T) {
	resetCommandState(t)
	tmp := t.TempDir()
	script := filepath.Join(tmp, "skills", "compile", "scripts", "compile.sh")
	if err := os.MkdirAll(filepath.Dir(script), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(script, []byte("#!/usr/bin/env bash\r\necho hi\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	path, cleanup, err := materializeCompileScript(tmp)
	if err != nil {
		t.Fatalf("materializeCompileScript: %v", err)
	}
	defer cleanup()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "\r") {
		t.Fatalf("materialized script still contains CR: %q", string(data))
	}
}

// TestMaterializeCompileScriptFallsBackToEmbedded verifies the regression
// reported on 2026-04-15: when `ao compile --full` runs outside a source
// checkout (no local skills/compile/scripts/compile.sh), the embedded copy
// must resolve instead of returning "file does not exist".
func TestMaterializeCompileScriptFallsBackToEmbedded(t *testing.T) {
	resetCommandState(t)
	tmp := t.TempDir()
	// Intentionally do NOT create skills/compile/scripts/compile.sh in tmp.
	path, cleanup, err := materializeCompileScript(tmp)
	if err != nil {
		t.Fatalf("materializeCompileScript outside repo: %v", err)
	}
	defer cleanup()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read materialized: %v", err)
	}
	if !strings.HasPrefix(string(data), "#!/usr/bin/env bash") {
		t.Fatalf("materialized script missing shebang; embedded copy may be absent.\nfirst 80 bytes: %q", string(data[:min(80, len(data))]))
	}
	if !strings.Contains(string(data), "AGENTOPS_COMPILE_RUNTIME") {
		t.Fatalf("materialized script missing runtime marker; embedded content appears wrong")
	}
}

func TestResolveCompileRuntime(t *testing.T) {
	t.Setenv("AGENTOPS_COMPILE_RUNTIME", "")
	origLookPath := lookPathFn
	t.Cleanup(func() { lookPathFn = origLookPath })

	// flag wins
	lookPathFn = func(string) (string, error) { return "", os.ErrNotExist }
	if got := resolveCompileRuntime("ollama"); got != "ollama" {
		t.Fatalf("flag override: got %q want ollama", got)
	}

	// env wins over auto-detect
	t.Setenv("AGENTOPS_COMPILE_RUNTIME", "openai")
	if got := resolveCompileRuntime(""); got != "openai" {
		t.Fatalf("env var: got %q want openai", got)
	}

	// auto-detect claude-cli when claude binary is present and nothing set
	t.Setenv("AGENTOPS_COMPILE_RUNTIME", "")
	lookPathFn = func(name string) (string, error) {
		if name == "claude" {
			return "/usr/local/bin/claude", nil
		}
		return "", os.ErrNotExist
	}
	if got := resolveCompileRuntime(""); got != "claude-cli" {
		t.Fatalf("auto-detect: got %q want claude-cli", got)
	}

	// no config and no claude → empty (preflight will fail with actionable error)
	lookPathFn = func(string) (string, error) { return "", os.ErrNotExist }
	if got := resolveCompileRuntime(""); got != "" {
		t.Fatalf("no config: got %q want empty", got)
	}
}

func TestPreflightCompileRuntimeErrors(t *testing.T) {
	origLookPath := lookPathFn
	t.Cleanup(func() { lookPathFn = origLookPath })

	cases := []struct {
		name    string
		runtime string
		env     map[string]string
		lookup  func(string) (string, error)
		wantErr string // substring
	}{
		{
			name:    "empty runtime names actionable env vars",
			runtime: "",
			wantErr: "AGENTOPS_COMPILE_RUNTIME=claude-cli",
		},
		{
			name:    "claude-cli missing binary",
			runtime: "claude-cli",
			lookup:  func(string) (string, error) { return "", os.ErrNotExist },
			wantErr: "'claude' binary is not on PATH",
		},
		{
			name:    "claude runtime missing API key",
			runtime: "claude",
			env:     map[string]string{"ANTHROPIC_API_KEY": ""},
			wantErr: "ANTHROPIC_API_KEY is not set",
		},
		{
			name:    "openai runtime missing API key",
			runtime: "openai",
			env:     map[string]string{"OPENAI_API_KEY": ""},
			wantErr: "OPENAI_API_KEY is not set",
		},
		{
			name:    "unknown runtime",
			runtime: "gemini",
			wantErr: "unknown runtime",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for k, v := range tc.env {
				t.Setenv(k, v)
			}
			if tc.lookup != nil {
				lookPathFn = tc.lookup
			} else {
				lookPathFn = func(string) (string, error) { return "", os.ErrNotExist }
			}
			err := preflightCompileRuntime(tc.runtime)
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error %q does not contain %q", err.Error(), tc.wantErr)
			}
		})
	}
}

func TestPreflightCompileRuntimeSuccess(t *testing.T) {
	origLookPath := lookPathFn
	t.Cleanup(func() { lookPathFn = origLookPath })

	lookPathFn = func(name string) (string, error) {
		if name == "claude" {
			return "/usr/local/bin/claude", nil
		}
		return "", os.ErrNotExist
	}
	t.Setenv("ANTHROPIC_API_KEY", "sk-fake")
	t.Setenv("OPENAI_API_KEY", "sk-fake")

	for _, rt := range []string{"claude-cli", "claude", "openai", "ollama"} {
		if err := preflightCompileRuntime(rt); err != nil {
			t.Fatalf("%s: unexpected error %v", rt, err)
		}
	}
}

// TestCompileScriptOptionsPassesBatchFlags verifies --batch-size and
// --max-batches propagate through to the shell invocation. This keeps the
// 2000+ file corpus regression from recurring.
func TestCompileScriptOptionsPassesBatchFlags(t *testing.T) {
	resetCommandState(t)
	tmp := t.TempDir()
	testProjectDir = tmp
	t.Cleanup(func() { testProjectDir = "" })

	// Seed a no-op compile script locally so materialize succeeds.
	script := filepath.Join(tmp, "skills", "compile", "scripts", "compile.sh")
	if err := os.MkdirAll(filepath.Dir(script), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(script, []byte("#!/usr/bin/env bash\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	compileFull = true
	compileBatchSize = 50
	compileMaxBatches = 3
	compileRuntime = "claude-cli"
	origLookPath := lookPathFn
	lookPathFn = func(name string) (string, error) {
		if name == "claude" {
			return "/usr/local/bin/claude", nil
		}
		return "", os.ErrNotExist
	}
	t.Cleanup(func() { lookPathFn = origLookPath })

	var got compileScriptOptions
	stubCompileRunners(t,
		func(string, string, bool) error { return nil },
		func(_ context.Context, _ string, opts compileScriptOptions, _, _ io.Writer) error {
			got = opts
			return nil
		},
		func(string, bool) error { return nil },
	)

	cmd := &cobra.Command{}
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetContext(context.Background())
	if err := runCompile(cmd, nil); err != nil {
		t.Fatalf("runCompile: %v", err)
	}
	if got.BatchSize != 50 {
		t.Fatalf("BatchSize = %d, want 50", got.BatchSize)
	}
	if got.MaxBatches != 3 {
		t.Fatalf("MaxBatches = %d, want 3", got.MaxBatches)
	}
}

// ensure unused imports stay referenced if tests shrink
var _ = json.NewEncoder

func stubCompileRunners(t *testing.T, mineFn func(string, string, bool) error, scriptFn func(context.Context, string, compileScriptOptions, io.Writer, io.Writer) error, defragFn func(string, bool) error) {
	t.Helper()
	origMine := runCompileMineFn
	origScript := runCompileScriptFn
	origDefrag := runCompileDefragFn
	runCompileMineFn = mineFn
	runCompileScriptFn = func(ctx context.Context, cwd string, opts compileScriptOptions, stdout, stderr io.Writer) error {
		return scriptFn(ctx, cwd, opts, stdout, stderr)
	}
	runCompileDefragFn = defragFn
	t.Cleanup(func() {
		runCompileMineFn = origMine
		runCompileScriptFn = origScript
		runCompileDefragFn = origDefrag
	})
}
