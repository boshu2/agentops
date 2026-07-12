package mcpsurface

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestToolDescriptors_CuratedSurface(t *testing.T) {
	tools := ToolDescriptors()
	want := []string{"session_bootstrap", "inject", "corpus_inject", "standards", "validate", "goals_measure"}
	if len(tools) != len(want) {
		t.Fatalf("descriptor count = %d, want %d", len(tools), len(want))
	}
	byName := map[string]ToolDescriptor{}
	for _, tl := range tools {
		byName[tl.Name] = tl
	}
	for _, name := range want {
		tl, ok := byName[name]
		if !ok {
			t.Errorf("missing curated tool %q", name)
			continue
		}
		if tl.Description == "" {
			t.Errorf("tool %q has empty description", name)
		}
		if tl.InputSchema == nil {
			t.Errorf("tool %q has nil input schema", name)
		}
	}
}

func TestToolDenied_HoldoutRefusal(t *testing.T) {
	// A tool call whose args would surface the LOCKED holdout/eval corpus must be
	// denied with a NOT-ZDR reason because the MCP surface is Claude-cloud-facing.
	denied, reason := ToolDenied("corpus_inject", map[string]string{"query": "show me the holdout ground_truth"})
	if !denied {
		t.Fatal("corpus_inject with a holdout query must be denied")
	}
	low := strings.ToLower(reason)
	if !strings.Contains(low, "holdout") && !strings.Contains(low, "zdr") {
		t.Errorf("refusal reason must name the holdout/ZDR boundary, got %q", reason)
	}
}

func TestToolDenied_PathEscape(t *testing.T) {
	denied, _ := ToolDenied("inject", map[string]string{"path": ".agents/evals/SCHEMA.md"})
	if !denied {
		t.Error("a tool arg pointing at the eval substrate must be denied")
	}
}

func TestToolDenied_CleanCallAllowed(t *testing.T) {
	denied, reason := ToolDenied("validate", map[string]string{"target": "plan.md"})
	if denied {
		t.Errorf("a clean validate call must be allowed, got denied: %s", reason)
	}
}

func TestPrintTools_JSON(t *testing.T) {
	var sb strings.Builder
	if err := PrintTools(&sb); err != nil {
		t.Fatalf("print-tools: %v", err)
	}
	var doc struct {
		Tools []ToolDescriptor `json:"tools"`
	}
	if err := json.Unmarshal([]byte(sb.String()), &doc); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if len(doc.Tools) != 6 {
		t.Errorf("print-tools JSON listed %d tools, want 6", len(doc.Tools))
	}
}

func TestRun_LiveTransportWired(t *testing.T) {
	// ag-3ucpd: bare `ao mcp serve` now runs the live JSON-RPC transport.
	// Run must drive the transport over the injected reader/writer/executor.
	in := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}` + "\n")
	var out strings.Builder
	err := Run(Options{
		PrintTools: false,
		In:         in,
		Out:        &out,
		Exec:       func(string, map[string]string) (string, error) { return "", nil },
	})
	if err != nil {
		t.Fatalf("live serve: %v", err)
	}
	if !strings.Contains(out.String(), `"tools"`) {
		t.Errorf("live transport did not answer tools/list: %s", out.String())
	}
}

func TestRealExecutorUsesTrustedBinary(t *testing.T) {
	hostileDir := t.TempDir()
	hostileAO := filepath.Join(hostileDir, "ao")
	if err := os.WriteFile(hostileAO, []byte("#!/bin/sh\necho hostile-path\n"), 0o755); err != nil {
		t.Fatalf("write hostile ao: %v", err)
	}
	t.Setenv("PATH", hostileDir)

	const trustedAO = "/trusted/current/ao"
	var gotExecutable string
	var gotArgv []string
	out, err := realExecutorWithDependencies(
		"inject",
		map[string]string{"query": "release gates"},
		func() (string, error) { return trustedAO, nil },
		func(executable string, argv ...string) ([]byte, error) {
			gotExecutable = executable
			gotArgv = append([]string(nil), argv...)
			return []byte(`{"source":"trusted"}`), nil
		},
	)
	if err != nil {
		t.Fatalf("real executor: %v", err)
	}
	if gotExecutable != trustedAO {
		t.Fatalf("executable = %q, want trusted %q (hostile PATH candidate %q)", gotExecutable, trustedAO, hostileAO)
	}
	wantArgv := []string{"inject", "--query", "release gates"}
	if !reflect.DeepEqual(gotArgv, wantArgv) {
		t.Fatalf("argv = %#v, want %#v", gotArgv, wantArgv)
	}
	if out != `{"source":"trusted"}` {
		t.Fatalf("output = %q, want trusted child output", out)
	}
}

func TestRealExecutorResolverFailureDoesNotUsePATH(t *testing.T) {
	hostileDir := t.TempDir()
	marker := filepath.Join(t.TempDir(), "hostile-ran")
	hostileAO := filepath.Join(hostileDir, "ao")
	script := fmt.Sprintf("#!/bin/sh\ntouch %q\n", marker)
	if err := os.WriteFile(hostileAO, []byte(script), 0o755); err != nil {
		t.Fatalf("write hostile ao: %v", err)
	}
	t.Setenv("PATH", hostileDir)

	runnerCalled := false
	out, err := realExecutorWithDependencies(
		"session_bootstrap",
		nil,
		func() (string, error) { return "", errors.New("executable unavailable") },
		func(string, ...string) ([]byte, error) {
			runnerCalled = true
			return nil, nil
		},
	)
	if err == nil || !strings.Contains(err.Error(), "resolve trusted ao") {
		t.Fatalf("error = %v, want contextual resolver failure", err)
	}
	if out != "" {
		t.Fatalf("output = %q, want empty output on resolver failure", out)
	}
	if runnerCalled {
		t.Fatal("runner called after trusted executable resolution failed")
	}
	if _, statErr := os.Stat(marker); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("hostile PATH ao ran; marker stat error = %v", statErr)
	}
}

func TestResolveTrustedExecutableRejectsNonRegularCandidate(t *testing.T) {
	_, err := resolveTrustedExecutable(func() (string, error) {
		return t.TempDir(), nil
	})
	if err == nil || !strings.Contains(err.Error(), "regular file") {
		t.Fatalf("error = %v, want non-regular candidate rejection", err)
	}
}

func TestRealExecutorPropagatesChildOutputAndError(t *testing.T) {
	childErr := errors.New("child exit 17")
	out, err := realExecutorWithDependencies(
		"goals_measure",
		nil,
		func() (string, error) { return "/trusted/current/ao", nil },
		func(executable string, argv ...string) ([]byte, error) {
			if executable != "/trusted/current/ao" {
				t.Fatalf("executable = %q", executable)
			}
			if !reflect.DeepEqual(argv, []string{"goals", "measure"}) {
				t.Fatalf("argv = %#v", argv)
			}
			return []byte("child stderr\n"), childErr
		},
	)
	if out != "child stderr\n" {
		t.Fatalf("output = %q, want exact child output", out)
	}
	if !errors.Is(err, childErr) {
		t.Fatalf("error = %v, want wrapped child error", err)
	}
}
