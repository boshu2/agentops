// practices: [cmm-process-maturity, dora-metrics]
package main

import (
	"testing"
)

// Tests for pure helper functions in maturity.go

func TestFloatValueFromData(t *testing.T) {
	t.Run("returns value when key exists as float64", func(t *testing.T) {
		data := map[string]any{"utility": float64(0.75)}
		got := floatValueFromData(data, "utility", 0.5)
		if got != 0.75 {
			t.Errorf("expected 0.75, got %f", got)
		}
	})

	t.Run("returns default when key missing", func(t *testing.T) {
		data := map[string]any{}
		got := floatValueFromData(data, "utility", 0.5)
		if got != 0.5 {
			t.Errorf("expected default 0.5, got %f", got)
		}
	})

	t.Run("returns default when value is wrong type", func(t *testing.T) {
		data := map[string]any{"utility": "not-a-float"}
		got := floatValueFromData(data, "utility", 0.3)
		if got != 0.3 {
			t.Errorf("expected default 0.3, got %f", got)
		}
	})

	t.Run("returns zero value when zero stored", func(t *testing.T) {
		data := map[string]any{"score": float64(0.0)}
		got := floatValueFromData(data, "score", 1.0)
		if got != 0.0 {
			t.Errorf("expected 0.0, got %f", got)
		}
	})
}

func TestNonEmptyStringFromData(t *testing.T) {
	t.Run("returns value when key exists", func(t *testing.T) {
		data := map[string]any{"status": "established"}
		got := nonEmptyStringFromData(data, "status", "unknown")
		if got != "established" {
			t.Errorf("expected %q, got %q", "established", got)
		}
	})

	t.Run("returns default when key missing", func(t *testing.T) {
		data := map[string]any{}
		got := nonEmptyStringFromData(data, "status", "unknown")
		if got != "unknown" {
			t.Errorf("expected %q, got %q", "unknown", got)
		}
	})

	t.Run("returns default when value is empty string", func(t *testing.T) {
		data := map[string]any{"status": ""}
		got := nonEmptyStringFromData(data, "status", "default")
		if got != "default" {
			t.Errorf("expected %q, got %q", "default", got)
		}
	})

	t.Run("returns default when value is wrong type", func(t *testing.T) {
		data := map[string]any{"status": 42}
		got := nonEmptyStringFromData(data, "status", "fallback")
		if got != "fallback" {
			t.Errorf("expected %q, got %q", "fallback", got)
		}
	})
}
