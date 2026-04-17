package main

import (
	"testing"
)

// withoutDemoDelay zeroes demoStepDelay for the duration of a test so the
// 500 ms × 5-step pacing sleep in quickDemo does not inflate test runtime.
// The 500 ms is purely for human pacing in a live terminal; nothing in
// quickDemo depends on it for correctness.
func withoutDemoDelay(t *testing.T) {
	t.Helper()
	prev := demoStepDelay
	demoStepDelay = 0
	t.Cleanup(func() { demoStepDelay = prev })
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
	err := showConcepts()
	if err != nil {
		t.Fatalf("showConcepts returned error: %v", err)
	}
}

func TestDemo_QuickDemo(t *testing.T) {
	withoutDemoDelay(t)
	err := quickDemo()
	if err != nil {
		t.Fatalf("quickDemo returned error: %v", err)
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
