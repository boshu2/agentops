// practices: [pragmatic-programmer, agile-manifesto]
package main

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

// withoutDemoDelay zeroes demoStepDelay for the duration of a test so the
// 500 ms × 6-step pacing sleep in quickDemo does not inflate test runtime.
// The 500 ms is purely for human pacing in a live terminal; nothing in
// quickDemo depends on it for correctness.
func withoutDemoDelay(t *testing.T) {
	t.Helper()
	prev := demoStepDelay
	demoStepDelay = 0
	t.Cleanup(func() { demoStepDelay = prev })
}

func captureDemoStdout(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stdout: %v", err)
	}
	os.Stdout = w

	var buf bytes.Buffer
	copyDone := make(chan struct{})
	go func() {
		_, _ = io.Copy(&buf, r)
		close(copyDone)
	}()

	runErr := fn()
	_ = w.Close()
	os.Stdout = old
	<-copyDone
	_ = r.Close()
	return buf.String(), runErr
}

func TestDemo_CommandExists(t *testing.T) {
	if demoCmd == nil {
		t.Fatal("demoCmd should not be nil")
	}
	if demoCmd.Use != "demo" {
		t.Errorf("demoCmd.Use = %q, want %q", demoCmd.Use, "demo")
	}
	if demoCmd.GroupID != "start" {
		t.Errorf("demoCmd.GroupID = %q, want %q", demoCmd.GroupID, "start")
	}
}

func TestDemo_HasFlags(t *testing.T) {
	if demoCmd.Flags().Lookup("quick") == nil {
		t.Error("demo command should have --quick flag")
	}
	if demoCmd.Flags().Lookup("concepts") == nil {
		t.Error("demo command should have --concepts flag")
	}
}

func TestDemo_ShowConcepts(t *testing.T) {
	out, err := captureDemoStdout(t, showConcepts)
	if err != nil {
		t.Fatalf("showConcepts returned error: %v", err)
	}
	for _, want := range []string{
		"AGENTOPS 3.0 PRODUCT MODEL",
		"ENGINEERING OS FOR AGENT TEAMS",
		"DOMAIN AND PRACTICE PACKETS",
		"COUNCIL VERDICTS",
		"From agent opinions to engineering verdicts",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("showConcepts output missing %q:\n%s", want, out)
		}
	}
}

func TestDemo_QuickDemo(t *testing.T) {
	withoutDemoDelay(t)
	out, err := captureDemoStdout(t, quickDemo)
	if err != nil {
		t.Fatalf("quickDemo returned error: %v", err)
	}
	for _, want := range []string{
		"AGENTOPS QUICK DEMO",
		"engineering operating system for agent teams",
		"domain/practice packet",
		"ao context assemble",
		"/council --mixed",
		".agents/council/<run-id>/verdict.md",
		"ao quick-start",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("quickDemo output missing %q:\n%s", want, out)
		}
	}
	for _, stale := range []string{"ol quick-start", "unreliable agents", "superpowers"} {
		if strings.Contains(out, stale) {
			t.Fatalf("quickDemo output contains stale phrase %q:\n%s", stale, out)
		}
	}
}

func TestDemo_RunDemoDispatch_Concepts(t *testing.T) {
	origConcepts := demoConcepts
	origQuick := demoQuick
	defer func() {
		demoConcepts = origConcepts
		demoQuick = origQuick
	}()

	demoConcepts = true
	demoQuick = false
	err := runDemo(demoCmd, nil)
	if err != nil {
		t.Fatalf("runDemo with concepts: %v", err)
	}
}

func TestDemo_RunDemoDispatch_Quick(t *testing.T) {
	withoutDemoDelay(t)
	origConcepts := demoConcepts
	origQuick := demoQuick
	defer func() {
		demoConcepts = origConcepts
		demoQuick = origQuick
	}()

	demoConcepts = false
	demoQuick = true
	err := runDemo(demoCmd, nil)
	if err != nil {
		t.Fatalf("runDemo with quick: %v", err)
	}
}

func TestDemo_RegisteredOnRoot(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Use == "demo" {
			found = true
			break
		}
	}
	if !found {
		t.Error("demoCmd should be registered on rootCmd")
	}
}
