package t01

import "testing"

// Holdout acceptance: "ratios humans actually type into the config file"
// includes surrounding whitespace and a spaced slash. Never shipped to the
// agent workspace; injected only at scoring time.
func TestParseRatioHumanInput(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want float64
	}{
		{"leading-trailing space", " 3/4 ", 0.75},
		{"spaced slash", "3 / 4", 0.75},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseRatio(tt.in)
			if err != nil {
				t.Fatalf("ParseRatio(%q) unexpected error: %v", tt.in, err)
			}
			if got != tt.want {
				t.Errorf("ParseRatio(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}
