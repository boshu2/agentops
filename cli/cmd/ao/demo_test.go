// practices: [pragmatic-programmer, agile-manifesto]
package main

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

func captureDemoStdout(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	var buf bytes.Buffer
	done := make(chan struct{})
	go func() { _, _ = io.Copy(&buf, r); close(done) }()
	runErr := fn()
	_ = w.Close()
	os.Stdout = old
	<-done
	_ = r.Close()
	return buf.String(), runErr
}

func TestDemoShowsOnePassBoundary(t *testing.T) {
	out, err := captureDemoStdout(t, quickDemo)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"AGENTOPS ONE-PASS DEMO", "PlanPacket", "CandidatePacket", "subject-manifest.v1", "verdict.v2", "stops"} {
		if !strings.Contains(out, want) {
			t.Fatalf("demo missing %q:\n%s", want, out)
		}
	}
}

func TestDemoConceptsExcludeLifecycleAuthority(t *testing.T) {
	out, err := captureDemoStdout(t, showConcepts)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"fresh independent judgment", "does not own retries", "Git", "delivery"} {
		if !strings.Contains(out, want) {
			t.Fatalf("concepts missing %q:\n%s", want, out)
		}
	}
}
