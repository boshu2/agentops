// practices: [pragmatic-programmer, agile-manifesto]
package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestDemoShowsOnePassBoundary(t *testing.T) {
	var out bytes.Buffer
	err := quickDemo(&out)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"AGENTOPS ONE-PASS DEMO", "existing intent source", "runtime derives", "subject-manifest.v1", "verdict.v2", "stops"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("demo missing %q:\n%s", want, out.String())
		}
	}
	for _, forbidden := range []string{"PlanPacket", "CandidatePacket", "RevisionPacket"} {
		if strings.Contains(out.String(), forbidden) {
			t.Fatalf("demo advertises removed %s contract:\n%s", forbidden, out.String())
		}
	}
}

func TestDemoConceptsExcludeLifecycleAuthority(t *testing.T) {
	var out bytes.Buffer
	err := showConcepts(&out)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"fresh independent judgment", "does not own retries", "Git", "delivery"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("concepts missing %q:\n%s", want, out.String())
		}
	}
}

func TestPublishedDemoQuick(t *testing.T) {
	removed := pruneToDefaultSpine(rootCmd)
	t.Cleanup(func() { restorePrunedCommands(rootCmd, removed) })

	out, err := executeCommand("demo", "--quick")
	if err != nil {
		t.Fatalf("published ao demo --quick failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "AGENTOPS ONE-PASS DEMO") {
		t.Fatalf("published demo output missing one-pass example:\n%s", out)
	}
}
