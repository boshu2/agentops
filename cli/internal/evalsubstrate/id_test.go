package evalsubstrate

import (
	"strings"
	"testing"
)

func TestValidateID_RejectsUnsafeComponents(t *testing.T) {
	c1 := "a" + string(rune(0x0080)) + "b"                   // C1 control; a C0-only check misses it
	del := "a" + string(rune(0x007f)) + "b"                  // DEL
	nfd := "caf" + string(rune('e')) + string(rune(0x0301))  // 'e' + combining acute (not NFC)
	rejected := map[string]string{
		"empty":               "",
		"self-reference":      ".",
		"parent traversal":    "..",
		"classic traversal":   "../../escape",
		"forward separator":   "a/b",
		"backslash separator": `a\b`,
		"absolute":            "/etc/passwd",
		"embedded traversal":  "tasks/../evil",
		"NUL control":         "bad\x00id",
		"tab control":         "tab\tid",
		"C1 control":          c1,
		"DEL control":         del,
		"trailing space":      ".. ", // Win32 strips the trailing space -> ".." traversal
		"leading space":       " ok",
		"trailing dot":        "foo.",
		"leading dot":         ".foo",
		"whitespace only":     "   ",
		"reserved colon":      "ms:stable", // ':' is a Windows ADS separator; must be encoded first
		"drive colon":         "a:b",
		"overlong":            strings.Repeat("a", 129), // exceeds the 128-byte cap
		"non-NFC (NFD)":       nfd,
	}
	for name, id := range rejected {
		if err := ValidateID(id); err == nil {
			t.Errorf("%s: ValidateID(%q) = nil, want rejection", name, id)
		}
	}
}

func TestValidateID_AcceptsCanonicalIDs(t *testing.T) {
	nfc := "caf" + string(rune(0x00e9)) // precomposed e-acute (NFC)
	accepted := []string{
		"task-1",
		"finance-categorize-txn",
		"2026-05-01-qwen-vs-c47", // dated suite id
		"qwen3.6-vs-claude4.7",   // interior dots are fine
		nfc,                      // precomposed (NFC) e-acute
		"ms%3Astable",            // model-spec id in its encoded, path-safe form
		strings.Repeat("a", maxIDBytes), // exactly at the cap
	}
	for _, id := range accepted {
		if err := ValidateID(id); err != nil {
			t.Errorf("ValidateID(%q) = %v, want nil", id, err)
		}
	}
}
