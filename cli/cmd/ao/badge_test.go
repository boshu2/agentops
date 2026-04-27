package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/types"
)

func TestBadge_GetEscapeStatus_Boundaries(t *testing.T) {
	tests := []struct {
		name       string
		sigmaRho   float64
		delta      float64
		wantStatus string
	}{
		{"exact delta boundary", 0.17, 17.0, "APPROACHING"}, // not >, falls to 0.17 > 0.136 = true
		{"just above delta", 0.18, 17.0, "ESCAPE VELOCITY"},
		{"above 80% of delta", 0.14, 17.0, "APPROACHING"}, // 0.14 > 0.136
		{"at 80% of delta", 0.136, 17.0, "BUILDING"},      // 0.136 ~= 0.136, not strictly >, falls through
		{"below 80% above 50%", 0.10, 17.0, "BUILDING"},   // 0.10 > 0.085
		{"at 50% of delta", 0.085, 17.0, "STARTING"},      // 0.085 ~= 0.085, not strictly >
		{"just below 50%", 0.084, 17.0, "STARTING"},
		{"zero sigmaRho", 0.0, 17.0, "STARTING"},
		{"zero delta", 0.0, 0.0, "STARTING"},
		{"both high", 5.0, 17.0, "ESCAPE VELOCITY"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, _ := getEscapeStatus(tt.sigmaRho, tt.delta)
			if status != tt.wantStatus {
				t.Errorf("getEscapeStatus(%f, %f) status = %q, want %q", tt.sigmaRho, tt.delta, status, tt.wantStatus)
			}
		})
	}
}

func TestBadge_MakeProgressBar_WidthVariants(t *testing.T) {
	tests := []struct {
		name  string
		value float64
		width int
		want  string
	}{
		{"zero width=1", 0.0, 1, "░"},
		{"full width=1", 1.0, 1, "█"},
		{"0.3 width=10", 0.3, 10, "███░░░░░░░"},
		{"0.7 width=10", 0.7, 10, "███████░░░"},
		{"0.99 width=10", 0.99, 10, "█████████░"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := makeProgressBar(tt.value, tt.width)
			if got != tt.want {
				t.Errorf("makeProgressBar(%f, %d) = %q, want %q", tt.value, tt.width, got, tt.want)
			}
		})
	}
}

func TestBadge_PrintBadge_NilMetrics(t *testing.T) {
	// Should not panic with nil metrics
	var out bytes.Buffer
	printBadge(&out, 0, nil)
	stdout := out.String()

	if !strings.Contains(stdout, "AGENTOPS KNOWLEDGE") {
		t.Errorf("expected badge header, got: %q", stdout)
	}
	// nil metrics should default to DefaultDelta
	if !strings.Contains(stdout, "STARTING") {
		t.Errorf("expected STARTING status for nil metrics, got: %q", stdout)
	}
}

func TestBadge_PrintBadge_WithMetrics(t *testing.T) {
	m := &FlywheelMetrics{
		Sigma:               0.8,
		Rho:                 1.5,
		Delta:               types.DefaultDelta * 100,
		SigmaRho:            1.2,
		CitationsThisPeriod: 10,
		TierCounts:          map[string]int{"learning": 5, "pattern": 3},
	}

	var out bytes.Buffer
	printBadge(&out, 42, m)
	stdout := out.String()

	if !strings.Contains(stdout, "42") {
		t.Errorf("expected session count 42, got: %q", stdout)
	}
	if !strings.Contains(stdout, "ESCAPE VELOCITY") {
		t.Errorf("expected ESCAPE VELOCITY status for high sigma*rho, got: %q", stdout)
	}
}

func TestBadge_PrintBadge_BoxDrawingChars(t *testing.T) {
	m := &FlywheelMetrics{
		Delta:      types.DefaultDelta * 100,
		TierCounts: map[string]int{},
	}

	var out bytes.Buffer
	printBadge(&out, 0, m)
	stdout := out.String()

	// Verify box drawing characters are present
	for _, char := range []string{"╔", "╗", "╚", "╝", "║", "═", "╠", "╣"} {
		if !strings.Contains(stdout, char) {
			t.Errorf("expected box character %q in badge output", char)
		}
	}
}

func TestBadge_PrintBadge_ComparisonOperator(t *testing.T) {
	t.Run("above delta shows >", func(t *testing.T) {
		m := &FlywheelMetrics{
			SigmaRho:   0.3,
			Delta:      17.0,
			TierCounts: map[string]int{},
		}
		var out bytes.Buffer
		printBadge(&out, 0, m)
		stdout := out.String()

		if !strings.Contains(stdout, ">") {
			t.Errorf("expected '>' operator for sigmaRho > delta, got: %q", stdout)
		}
	})
}

func TestBadge_RunBadgeWithOptions_JSONUsesWriter(t *testing.T) {
	var out bytes.Buffer
	dir := t.TempDir()

	err := runBadgeWithOptions(badgeOptions{
		Cwd:    dir,
		Output: "json",
		Writer: &out,
	})
	if err != nil {
		t.Fatalf("runBadgeWithOptions returned error: %v", err)
	}

	var got badgeResult
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("json output did not decode as badgeResult: %v\noutput: %s", err, out.String())
	}
	if got.SessionsMined != 0 {
		t.Fatalf("SessionsMined = %d, want 0", got.SessionsMined)
	}
	if got.Status == "" {
		t.Fatal("expected status in JSON output")
	}
}

func TestBadge_RunBadge_EmptyDir(t *testing.T) {
	dir := t.TempDir()

	testProjectDir = dir
	defer func() { testProjectDir = "" }()

	// Should not error even on empty directory
	if err := runBadge(&cobra.Command{}, nil); err != nil {
		t.Fatalf("runBadge failed on empty dir: %v", err)
	}
}
