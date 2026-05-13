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

func TestClaimBind_EmptyClaimRejected(t *testing.T) {
	err := claimBindRun(context.Background(), claimOptions{path: "p", level: "PG2"})
	if err == nil {
		t.Fatal("expected error on empty claim")
	}
}

func TestClaimBind_EmptyPathRejected(t *testing.T) {
	err := claimBindRun(context.Background(), claimOptions{claim: "X", level: "PG2"})
	if err == nil {
		t.Fatal("expected error on empty path")
	}
}

func TestClaimBind_InvalidLevelRejected(t *testing.T) {
	err := claimBindRun(context.Background(), claimOptions{claim: "X", path: "p", level: "PG99"})
	if err == nil {
		t.Fatal("expected error on bogus level")
	}
	if !strings.Contains(err.Error(), "invalid --level") {
		t.Fatalf("error not informative: %v", err)
	}
}

func TestClaimBind_StubCalledWithBinding(t *testing.T) {
	called := false
	var gotOpts claimOptions
	stub := func(_ context.Context, opts claimOptions) error {
		called = true
		gotOpts = opts
		return nil
	}
	var buf bytes.Buffer
	err := claimBindRun(context.Background(), claimOptions{
		claim:   "AOP-CLAIM-X",
		path:    ".agents/findings/x.md",
		level:   "PG3",
		anchors: []string{"L10", "L20"},
		writer:  &buf,
		bindFn:  stub,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("bindFn not invoked")
	}
	if gotOpts.claim != "AOP-CLAIM-X" || gotOpts.level != "PG3" {
		t.Fatalf("opts mismatch: %+v", gotOpts)
	}
	if !strings.Contains(buf.String(), `level=PG3`) {
		t.Fatalf("confirmation missing level: %q", buf.String())
	}
}

func TestClaimBind_DefaultLevelIsPG1(t *testing.T) {
	// Note: level default is set by cobra; in the test path we pass it explicitly.
	// This checks the validator accepts PG1 default value.
	stub := func(_ context.Context, _ claimOptions) error { return nil }
	err := claimBindRun(context.Background(), claimOptions{
		claim:  "X",
		path:   "p",
		level:  "PG1",
		bindFn: stub,
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestClaimBind_StubErrorWrapped(t *testing.T) {
	stub := func(_ context.Context, _ claimOptions) error {
		return errors.New("downgrade rejected")
	}
	err := claimBindRun(context.Background(), claimOptions{
		claim:  "X",
		path:   "p",
		level:  "PG1",
		bindFn: stub,
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "claim bind:") {
		t.Fatalf("error not wrapped: %v", err)
	}
}

func TestClaimList_StubReturnsBindings(t *testing.T) {
	stub := func(_ context.Context, _ claimOptions) ([]ports.EvidenceBinding, error) {
		return []ports.EvidenceBinding{
			{Claim: "AOP-A", Path: "a.md", Level: ports.EvidenceLevelPG2},
			{Claim: "AOP-B", Path: "b.md", Level: ports.EvidenceLevelPG4},
		}, nil
	}
	var buf bytes.Buffer
	err := claimListRun(context.Background(), claimOptions{
		writer: &buf,
		listFn: stub,
	})
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("len = %d, want 2", len(lines))
	}
	if !strings.Contains(lines[0], `"Level":"PG2"`) || !strings.Contains(lines[1], `"Level":"PG4"`) {
		t.Fatalf("levels missing: %s", buf.String())
	}
}

func TestClaimList_EmptyBindings(t *testing.T) {
	stub := func(_ context.Context, _ claimOptions) ([]ports.EvidenceBinding, error) {
		return []ports.EvidenceBinding{}, nil
	}
	var buf bytes.Buffer
	err := claimListRun(context.Background(), claimOptions{
		writer: &buf,
		listFn: stub,
	})
	if err != nil {
		t.Fatal(err)
	}
	if buf.Len() != 0 {
		t.Fatalf("empty should be 0 bytes, got %q", buf.String())
	}
}
