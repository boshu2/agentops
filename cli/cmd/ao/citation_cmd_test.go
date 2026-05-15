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

func TestCitationVerify_EmptyKindRejected(t *testing.T) {
	err := citationVerifyRun(context.Background(), citationVerifyOptions{
		raw: "foo",
	})
	if err == nil {
		t.Fatal("expected error on empty kind")
	}
	if !strings.Contains(err.Error(), "--kind required") {
		t.Fatalf("error not informative: %v", err)
	}
}

func TestCitationVerify_EmptyRawRejected(t *testing.T) {
	err := citationVerifyRun(context.Background(), citationVerifyOptions{
		kind: "file",
	})
	if err == nil {
		t.Fatal("expected error on empty raw")
	}
}

func TestCitationVerify_StubReturnsFresh(t *testing.T) {
	stub := func(_ context.Context, _ citationVerifyOptions) (ports.CitationVerdict, error) {
		return ports.CitationVerdict{
			Status:   ports.CitationStatusFresh,
			Reason:   "file resolves at HEAD",
			Resolved: "cli/cmd/ao/beads.go",
		}, nil
	}
	var buf bytes.Buffer
	err := citationVerifyRun(context.Background(), citationVerifyOptions{
		kind:     "file",
		raw:      "cli/cmd/ao/beads.go",
		writer:   &buf,
		verifyFn: stub,
	})
	if err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, `"Status":"FRESH"`) {
		t.Fatalf("missing Status:FRESH: %s", out)
	}
	if !strings.Contains(out, `"Resolved":"cli/cmd/ao/beads.go"`) {
		t.Fatalf("missing Resolved: %s", out)
	}
}

func TestCitationVerify_StubReturnsStale(t *testing.T) {
	stub := func(_ context.Context, _ citationVerifyOptions) (ports.CitationVerdict, error) {
		return ports.CitationVerdict{
			Status: ports.CitationStatusStale,
			Reason: "file moved or deleted",
		}, nil
	}
	var buf bytes.Buffer
	_ = citationVerifyRun(context.Background(), citationVerifyOptions{
		kind:     "file",
		raw:      "old/path.go",
		writer:   &buf,
		verifyFn: stub,
	})
	if !strings.Contains(buf.String(), `"Status":"STALE"`) {
		t.Fatalf("missing STALE: %s", buf.String())
	}
}

func TestCitationVerify_StubReturnsUnknown(t *testing.T) {
	stub := func(_ context.Context, _ citationVerifyOptions) (ports.CitationVerdict, error) {
		return ports.CitationVerdict{
			Status: ports.CitationStatusUnknown,
			Reason: "unknown citation kind",
		}, nil
	}
	var buf bytes.Buffer
	_ = citationVerifyRun(context.Background(), citationVerifyOptions{
		kind:     "weird",
		raw:      "x",
		writer:   &buf,
		verifyFn: stub,
	})
	if !strings.Contains(buf.String(), `"Status":"UNKNOWN"`) {
		t.Fatalf("missing UNKNOWN: %s", buf.String())
	}
}

func TestCitationVerify_StubErrorWrapped(t *testing.T) {
	stub := func(_ context.Context, _ citationVerifyOptions) (ports.CitationVerdict, error) {
		return ports.CitationVerdict{}, errors.New("git unavailable")
	}
	err := citationVerifyRun(context.Background(), citationVerifyOptions{
		kind:     "file",
		raw:      "x",
		verifyFn: stub,
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "citation verify:") {
		t.Fatalf("error not wrapped: %v", err)
	}
}
