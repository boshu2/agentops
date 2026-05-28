// practices: [tdd]
package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/ports"
)

func TestGateRun_StubReturnsVerdict(t *testing.T) {
	stub := func(_ context.Context, _ gateRunOptions) (ports.GateVerdict, error) {
		return ports.GateVerdict{
			Status:  ports.GateStatusPass,
			Reason:  "exit 0",
			LogTail: "all checks passed",
		}, nil
	}
	var buf bytes.Buffer
	err := gateRunRun(context.Background(), gateRunOptions{
		name:   "compile-health",
		writer: &buf,
		runFn:  stub,
	})
	if err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, `"Status":"PASS"`) {
		t.Fatalf("missing Status:PASS in output: %s", out)
	}
	if !strings.Contains(out, `"Reason":"exit 0"`) {
		t.Fatalf("missing Reason: %s", out)
	}
}

func TestGateRun_EmptyNameRejected(t *testing.T) {
	err := gateRunRun(context.Background(), gateRunOptions{
		name: "",
	})
	if err == nil {
		t.Fatal("expected error on empty name, got nil")
	}
	if !strings.Contains(err.Error(), "name required") {
		t.Fatalf("error not informative: %v", err)
	}
}

func TestGateRun_FailureWrapped(t *testing.T) {
	stub := func(_ context.Context, _ gateRunOptions) (ports.GateVerdict, error) {
		return ports.GateVerdict{}, errors.New("subprocess died")
	}
	err := gateRunRun(context.Background(), gateRunOptions{
		name:  "x",
		runFn: stub,
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "gate run:") {
		t.Fatalf("error not wrapped: %v", err)
	}
}

func TestGateRun_UnknownStatusFromMissingGate(t *testing.T) {
	// The port contract says missing script → UNKNOWN; verify the
	// CLI emits it cleanly.
	stub := func(_ context.Context, _ gateRunOptions) (ports.GateVerdict, error) {
		return ports.GateVerdict{
			Status: ports.GateStatusUnknown,
			Reason: `no script for gate "does-not-exist"`,
		}, nil
	}
	var buf bytes.Buffer
	_ = gateRunRun(context.Background(), gateRunOptions{
		name:   "does-not-exist",
		writer: &buf,
		runFn:  stub,
	})
	if !strings.Contains(buf.String(), `"Status":"UNKNOWN"`) {
		t.Fatalf("missing UNKNOWN: %s", buf.String())
	}
}

func TestGateRun_LogTailIncluded(t *testing.T) {
	stub := func(_ context.Context, _ gateRunOptions) (ports.GateVerdict, error) {
		return ports.GateVerdict{
			Status:  ports.GateStatusFail,
			Reason:  "exit 1",
			LogTail: "ERROR: something broke\nat line 42",
		}, nil
	}
	var buf bytes.Buffer
	_ = gateRunRun(context.Background(), gateRunOptions{
		name:   "broken",
		writer: &buf,
		runFn:  stub,
	})
	if !strings.Contains(buf.String(), "ERROR: something broke") {
		t.Fatalf("LogTail not surfaced: %s", buf.String())
	}
}
