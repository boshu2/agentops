// practices: [sre, resilience-patterns]
package main

import (
	"testing"
)

// Tests for doctor.go helper functions

func TestFormatVersion(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"dev", "vdev"},
		{"v2.18.2", "v2.18.2"},
		{"v2.18.2-20-g3fef2f4-dirty", "v2.18.2-20-g3fef2f4-dirty"},
		{"2.18.2", "v2.18.2"},
		{"", "v"},
	}
	for _, tt := range tests {
		got := formatVersion(tt.input)
		if got != tt.want {
			t.Errorf("formatVersion(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
