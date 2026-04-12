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
