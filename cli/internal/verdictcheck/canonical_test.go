package verdictcheck

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestCanonicalJSONLineSeparators pins the byte-level behavior of the canonical
// writer for U+2028/U+2029. Real separators are emitted raw (matching Python's
// ensure_ascii=False); the LITERAL 6-char text backslash-u-2028 (an ASCII
// backslash followed by the characters u,2,0,2,8) must survive unchanged.
//
// All source strings use explicit Go escapes: " "/" " are the real
// runes, "\\u2028" is the literal backslash-u-2028 text.
func TestCanonicalJSONLineSeparators(t *testing.T) {
	tests := []struct {
		name  string
		value map[string]any
		want  string
	}{
		{
			name:  "raw U+2028 emitted raw",
			value: map[string]any{"k": "a b"},
			want:  "{\"k\":\"a b\"}",
		},
		{
			name:  "raw U+2029 emitted raw",
			value: map[string]any{"k": "a b"},
			want:  "{\"k\":\"a b\"}",
		},
		{
			name:  "both raw separators emitted raw",
			value: map[string]any{"k": "a b c"},
			want:  "{\"k\":\"a b c\"}",
		},
		{
			name:  "literal backslash-u2028 preserved (even backslash run)",
			value: map[string]any{"k": "a\\u2028b"}, // literal: a \ u 2 0 2 8 b
			want:  "{\"k\":\"a\\\\u2028b\"}",         // JSON-escaped backslash: a \\ u2028 b
		},
		{
			name:  "escaped-backslash then real separator both preserved",
			value: map[string]any{"k": "a\\ b"}, // literal backslash, then a REAL U+2028
			want:  "{\"k\":\"a\\\\ b\"}",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CanonicalJSON(tt.value)
			if err != nil {
				t.Fatalf("CanonicalJSON: %v", err)
			}
			if string(got) != tt.want {
				t.Fatalf("canonical bytes mismatch\n got: %q\nwant: %q", string(got), tt.want)
			}
		})
	}
}

// TestLineSeparatorDigestParity is the direct cross-language digest-byte
// assertion required by acceptance (a): a value whose string fields carry raw
// U+2028 AND U+2029 (plus a literal backslash-u2028 text field) must digest
// identically in Go and in the real Python writer (validate.py digest), not
// merely "both validators agree". The Python side runs for real via python3.
func TestLineSeparatorDigestParity(t *testing.T) {
	py, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 unavailable; digest-parity cross-check skipped")
	}
	validatePy := filepath.Join("..", "..", "..", "skills", "validate", "scripts", "validate.py")
	if _, err := os.Stat(validatePy); err != nil {
		t.Fatalf("validate.py not found at %s: %v", validatePy, err)
	}

	// A top-level object (validate.py digest requires a JSON object) mixing raw
	// separators and literal backslash-u2028 text.
	value := map[string]any{
		"reason":  "line sep here",
		"checked": []string{"cli **"},
		"literal": "keep-\\u2028-literal",
	}

	// Go digest over the canonical form.
	canonical, err := CanonicalJSON(value)
	if err != nil {
		t.Fatalf("CanonicalJSON: %v", err)
	}
	sum := sha256.Sum256(canonical)
	goDigest := hex.EncodeToString(sum[:])

	// Hand the SAME value to Python via a temp file. json.Marshal escapes the
	// raw separators to   /  ; Python's json.load decodes them back to
	// raw runes, so both engines digest the identical logical value.
	marshaled, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	tmp := filepath.Join(t.TempDir(), "value.json")
	if err := os.WriteFile(tmp, marshaled, 0o600); err != nil {
		t.Fatalf("write temp: %v", err)
	}
	out, err := exec.Command(py, validatePy, "digest", tmp).CombinedOutput()
	if err != nil {
		t.Fatalf("python3 validate.py digest failed: %v\n%s", err, out)
	}
	pyDigest := strings.TrimSpace(string(out))

	if goDigest != pyDigest {
		t.Fatalf("cross-language digest fork:\n  Go:     %s\n  Python: %s\n  Go canonical bytes: %q", goDigest, pyDigest, canonical)
	}
}
