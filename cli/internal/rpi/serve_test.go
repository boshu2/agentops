package rpi

import "testing"

// TestClassifyServeArg_FlagVsPositional covers the asymmetry between the
// explicit --run-id flag and positional arguments:
//   - Flag path uses ExplicitServeRunIDPattern (accepts bare 8-hex too).
//   - Positional path uses ServeRunIDPattern (rejects bare 8-hex to avoid
//     collision with short git SHAs typed as goal strings).
//
// This regression pinned the bug where legacy run directories like
// `.agents/rpi/runs/0aa420a9/` were unreachable via `ao rpi serve --run-id`.
func TestClassifyServeArg_FlagVsPositional(t *testing.T) {
	tests := []struct {
		name      string
		flagRunID string
		args      []string
		wantGoal  string
		wantRunID string
	}{
		// --- Numeric/bare shapes ---
		{"bare 12-hex via flag", "760fc86f0c0f", nil, "", "760fc86f0c0f"},
		{"bare 12-hex via arg", "", []string{"760fc86f0c0f"}, "", "760fc86f0c0f"},
		{"bare 8-hex via flag accepted (legacy)", "0aa420a9", nil, "", "0aa420a9"},
		{"bare 8-hex via arg treated as goal", "", []string{"4c538e8a"}, "4c538e8a", ""},

		// --- rpi-prefixed shapes (valid on both paths) ---
		{"rpi- 8-hex via flag", "rpi-a1b2c3d4", nil, "", "rpi-a1b2c3d4"},
		{"rpi- 8-hex via arg", "", []string{"rpi-a1b2c3d4"}, "", "rpi-a1b2c3d4"},
		{"rpi- 12-hex via flag", "rpi-760fc86f0c0f", nil, "", "rpi-760fc86f0c0f"},
		{"rpi- 12-hex via arg", "", []string{"rpi-760fc86f0c0f"}, "", "rpi-760fc86f0c0f"},
		{"rpi- 10-hex via flag", "rpi-a1b2c3d4e5", nil, "", "rpi-a1b2c3d4e5"},

		// --- UUID-shape tokens: always rejected as run ID ---
		{"uuid via flag treated as goal", "550e8400-e29b-41d4-a716-446655440000", nil, "550e8400-e29b-41d4-a716-446655440000", ""},
		{"uuid via arg treated as goal", "", []string{"550e8400-e29b-41d4-a716-446655440000"}, "550e8400-e29b-41d4-a716-446655440000", ""},

		// --- Plain goal strings ---
		{"goal via flag", "fix the cache bug", nil, "fix the cache bug", ""},
		{"goal via arg", "", []string{"add auth"}, "add auth", ""},

		// --- Invalid/malformed ---
		{"uppercase hex via flag", "ABCDEF01", nil, "ABCDEF01", ""},
		{"rpi- uppercase via flag", "RPI-abcdef01", nil, "RPI-abcdef01", ""},
		{"rpi- too short via flag", "rpi-abcde", nil, "rpi-abcde", ""},
		{"bare 10-hex via flag", "abcdef0123", nil, "abcdef0123", ""},
		{"extra chars via flag", "rpi-abcdef01x", nil, "rpi-abcdef01x", ""},

		// --- Empty ---
		{"empty", "", nil, "", ""},
		{"empty slice", "", []string{}, "", ""},

		// --- Flag wins over positional ---
		{"flag overrides positional goal", "rpi-deadbeef", []string{"some-goal"}, "", "rpi-deadbeef"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			goal, runID := ClassifyServeArg(tt.flagRunID, tt.args)
			if goal != tt.wantGoal || runID != tt.wantRunID {
				t.Errorf("ClassifyServeArg(%q, %v) = (%q, %q), want (%q, %q)",
					tt.flagRunID, tt.args, goal, runID, tt.wantGoal, tt.wantRunID)
			}
		})
	}
}

func TestValidateExplicitServeRunID_AcceptsBare8Hex(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{"empty allowed", "", "", false},
		{"bare 12-hex", "760fc86f0c0f", "760fc86f0c0f", false},
		{"bare 8-hex (legacy, now accepted)", "3f0d90bd", "3f0d90bd", false},
		{"prefixed 8-hex", "rpi-a1b2c3d4", "rpi-a1b2c3d4", false},
		{"prefixed 12-hex", "rpi-760fc86f0c0f", "rpi-760fc86f0c0f", false},
		{"whitespace trimmed", "  760fc86f0c0f  ", "760fc86f0c0f", false},
		{"uppercase hex rejected", "ABCDEF01", "", true},
		{"uuid rejected", "550e8400-e29b-41d4-a716-446655440000", "", true},
		{"bare 10-hex rejected", "abcdef0123", "", true},
		{"bare 11-hex rejected", "abcdef01234", "", true},
		{"goal string rejected", "add auth", "", true},
		{"prefixed 7-hex rejected", "rpi-abcdef0", "", true},
		{"prefixed 13-hex rejected", "rpi-abcdef0123456", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ValidateExplicitServeRunID(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q, got nil (result=%q)", tt.input, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ValidateExplicitServeRunID(%q): unexpected error: %v", tt.input, err)
			}
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// TestServeRunIDPatterns_Asymmetry documents that the positional pattern
// deliberately rejects bare 8-hex while the explicit-flag pattern accepts it.
func TestServeRunIDPatterns_Asymmetry(t *testing.T) {
	bare8 := "0aa420a9"
	if ServeRunIDPattern.MatchString(bare8) {
		t.Errorf("ServeRunIDPattern should reject bare 8-hex %q (git SHA collision)", bare8)
	}
	if !ExplicitServeRunIDPattern.MatchString(bare8) {
		t.Errorf("ExplicitServeRunIDPattern should accept bare 8-hex %q (legacy run IDs)", bare8)
	}

	// Every string matched by the positional pattern must also be matched by
	// the explicit pattern (explicit is a proper superset).
	for _, ok := range []string{
		"760fc86f0c0f",
		"rpi-a1b2c3d4",
		"rpi-760fc86f0c0f",
		"rpi-a1b2c3d4e5",
	} {
		if !ServeRunIDPattern.MatchString(ok) {
			t.Errorf("ServeRunIDPattern should accept %q", ok)
		}
		if !ExplicitServeRunIDPattern.MatchString(ok) {
			t.Errorf("ExplicitServeRunIDPattern should accept %q", ok)
		}
	}
}
