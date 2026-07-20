// practices: [pragmatic-programmer, agile-manifesto]
package demo

import (
	"bytes"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/clicontract"
)

func TestModule_Contract(t *testing.T) {
	contract := NewModule().Contract()
	if contract.ID != "ao.demo" {
		t.Fatalf("contract ID = %q, want ao.demo", contract.ID)
	}
	if contract.Output != clicontract.OutputText {
		t.Fatalf("output = %v, want OutputText", contract.Output)
	}
	if contract.Effects != clicontract.EffectPure {
		t.Fatalf("effects = %v, want EffectPure", contract.Effects)
	}
}

func TestModule_CommandAttributes(t *testing.T) {
	command := NewModule().Command()
	if command.Use != "demo" {
		t.Fatalf("Use = %q, want demo", command.Use)
	}
	if command.GroupID != "start" {
		t.Fatalf("GroupID = %q, want start", command.GroupID)
	}
}

func TestDemoShowsOnePassBoundary(t *testing.T) {
	var out bytes.Buffer
	if err := quickDemo(&out); err != nil {
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
	if err := showConcepts(&out); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"fresh independent judgment", "does not own retries", "Git", "delivery"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("concepts missing %q:\n%s", want, out.String())
		}
	}
}

func TestPublishedDemoQuick(t *testing.T) {
	command := NewModule().Command()
	var out bytes.Buffer
	command.SetOut(&out)
	command.SetArgs([]string{"--quick"})
	if err := command.Execute(); err != nil {
		t.Fatalf("ao demo --quick failed: %v\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), "AGENTOPS ONE-PASS DEMO") {
		t.Fatalf("demo output missing one-pass example:\n%s", out.String())
	}
}

func TestDemoConceptsFlagRendersBoundary(t *testing.T) {
	command := NewModule().Command()
	var out bytes.Buffer
	command.SetOut(&out)
	command.SetArgs([]string{"--concepts"})
	if err := command.Execute(); err != nil {
		t.Fatalf("ao demo --concepts failed: %v\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), "AGENTOPS PRODUCT BOUNDARY") {
		t.Fatalf("demo --concepts missing product boundary:\n%s", out.String())
	}
}
