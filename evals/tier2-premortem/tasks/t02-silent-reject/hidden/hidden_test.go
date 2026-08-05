package t02

import (
	"strings"
	"testing"
)

// Holdout acceptance: the operator can tell from the error alone what was
// wrong AND what they typed. An error like "invalid port" fails this bar;
// the rejected input and a reason must both appear. Injected only at scoring.
func TestValidatePortDiagnosableErrors(t *testing.T) {
	tests := []struct {
		name string
		in   string
	}{
		{"out of range carries input", "65536"},
		{"non-numeric carries input", "http"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ValidatePort(tt.in)
			if err == nil {
				t.Fatalf("ValidatePort(%q) expected error", tt.in)
			}
			if !strings.Contains(err.Error(), tt.in) {
				t.Errorf("error %q does not name the rejected input %q", err.Error(), tt.in)
			}
		})
	}
}
