// practices: [wiki-knowledge-surface, lean-startup]
package main

import "testing"

func TestDetectSessionTypeFromContent_Implementation(t *testing.T) {
	got := detectSessionTypeFromContent("added new feature", []string{"go test passed"}, []string{"use cobra for CLI"})
	if got != "implement" {
		t.Errorf("detectSessionTypeFromContent(implement) = %q, want %q", got, "implement")
	}
}

func TestDetectSessionTypeFromContent_Career(t *testing.T) {
	got := detectSessionTypeFromContent("career coaching session", []string{"interview prep for Shield AI"}, nil)
	if got != "career" {
		t.Errorf("detectSessionTypeFromContent(career) = %q, want %q", got, "career")
	}
}

func TestDetectSessionTypeFromContent_Debug(t *testing.T) {
	got := detectSessionTypeFromContent("debugging session", []string{"stack trace analysis"}, nil)
	if got != "debug" {
		t.Errorf("detectSessionTypeFromContent(debug) = %q, want %q", got, "debug")
	}
}

func TestDetectSessionTypeFromContent_General(t *testing.T) {
	got := detectSessionTypeFromContent("random session", []string{"did some stuff"}, nil)
	if got != "general" {
		t.Errorf("detectSessionTypeFromContent(general) = %q, want %q", got, "general")
	}
}
