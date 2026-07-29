package evalsubstrate

import "testing"

func TestValidateID_RejectsTraversalAndSeparators(t *testing.T) {
	rejected := []string{
		"",              // empty
		".",             // self-reference
		"..",            // parent traversal
		"../../escape",  // classic traversal
		"a/b",           // forward separator
		`a\b`,           // backslash separator (Windows)
		"/etc/passwd",   // absolute
		"tasks/../evil", // embedded traversal
		"bad\x00id",     // NUL control char
		"tab\tid",       // control char
	}
	for _, id := range rejected {
		if err := ValidateID(id); err == nil {
			t.Errorf("ValidateID(%q) = nil, want rejection", id)
		}
	}
}

func TestValidateID_AcceptsLegitimateIDs(t *testing.T) {
	accepted := []string{
		"task-1",
		"finance-categorize-txn",
		"ms:stable",              // model-spec IDs use colons
		"ms:test-2026-05-01",     // dated model-spec id
		"2026-05-01-qwen-vs-c47", // dated suite id
		"..leading-dots",         // dots that are not the ".." component
		"trailing..",
	}
	for _, id := range accepted {
		if err := ValidateID(id); err != nil {
			t.Errorf("ValidateID(%q) = %v, want nil", id, err)
		}
	}
}
