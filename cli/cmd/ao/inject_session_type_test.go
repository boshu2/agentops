package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSessionTypeBoost_ExactMatch(t *testing.T) {
	l := learning{SessionType: "career"}
	got := sessionTypeBoost(l, "career")
	if got != 1.3 {
		t.Errorf("exact match boost = %v, want 1.3", got)
	}
}

func TestSessionTypeBoost_RelatedMatch(t *testing.T) {
	l := learning{SessionType: "career"}
	got := sessionTypeBoost(l, "coaching")
	if got != 1.15 {
		t.Errorf("related match boost = %v, want 1.15", got)
	}
}

func TestSessionTypeBoost_NoMatch(t *testing.T) {
	l := learning{SessionType: "career"}
	got := sessionTypeBoost(l, "implement")
	if got != 1.0 {
		t.Errorf("no match boost = %v, want 1.0", got)
	}
}

func TestSessionTypeBoost_EmptyType(t *testing.T) {
	l := learning{SessionType: "career"}
	got := sessionTypeBoost(l, "")
	if got != 1.0 {
		t.Errorf("empty type boost = %v, want 1.0", got)
	}
}

func TestDetectSessionTypeFromGoal_Career(t *testing.T) {
	got := detectSessionTypeFromGoal("career coaching Shield AI")
	if got != "career" {
		t.Errorf("detectSessionTypeFromGoal(career) = %q, want %q", got, "career")
	}
}

func TestDetectSessionTypeFromGoal_Debug(t *testing.T) {
	got := detectSessionTypeFromGoal("fix broken inject pipeline")
	if got != "debug" {
		t.Errorf("detectSessionTypeFromGoal(debug) = %q, want %q", got, "debug")
	}
}

func TestDetectSessionTypeFromGoal_Default(t *testing.T) {
	got := detectSessionTypeFromGoal("add feature X to the CLI")
	if got != "implement" {
		t.Errorf("detectSessionTypeFromGoal(default) = %q, want %q", got, "implement")
	}
}

func TestIsRelatedSessionType(t *testing.T) {
	tests := []struct {
		a, b string
		want bool
	}{
		{"career", "coaching", true},
		{"debug", "debugging", true},
		{"research", "brainstorm", true},
		{"career", "debug", false},
		{"implement", "career", false},
		{"", "career", false},
	}
	for _, tc := range tests {
		got := isRelatedSessionType(tc.a, tc.b)
		if got != tc.want {
			t.Errorf("isRelatedSessionType(%q, %q) = %v, want %v", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestInjectProfile_Exists(t *testing.T) {
	tmp := t.TempDir()
	agentsDir := filepath.Join(tmp, ".agents")
	if err := os.MkdirAll(agentsDir, 0750); err != nil {
		t.Fatal(err)
	}
	content := "**Name:** Test User\n**Background:** Testing\n"
	if err := os.WriteFile(filepath.Join(agentsDir, "profile.md"), []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	profilePath := filepath.Join(tmp, ".agents", "profile.md")
	data, err := os.ReadFile(profilePath)
	if err != nil {
		t.Fatalf("profile should exist: %v", err)
	}
	output := "## Identity\n\n" + string(data) + "\n\n## Learnings"
	if len(output) == 0 {
		t.Error("expected non-empty output with profile prepended")
	}
	if !strings.Contains(output, "Test User") {
		t.Error("output should contain profile content")
	}
}

func TestInjectProfile_Missing(t *testing.T) {
	tmp := t.TempDir()
	profilePath := filepath.Join(tmp, ".agents", "profile.md")
	_, err := os.ReadFile(profilePath)
	if err == nil {
		t.Error("expected error reading missing profile")
	}
}
