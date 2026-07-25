package skills

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestJSONUnicodeEscapePreflightRejectsUnpairedSurrogates(t *testing.T) {
	t.Parallel()
	for _, body := range [][]byte{
		[]byte(`{"value":"\uD800"}`),
		[]byte(`{"value":"\uD800x"}`),
		[]byte(`{"value":"\uDC00"}`),
	} {
		if err := validateJSONUnicodeEscapes(body); err == nil {
			t.Fatalf("preflight accepted %s", body)
		}
	}
	if err := validateJSONUnicodeEscapes([]byte(`{"value":"\uD800\uDC00"}`)); err != nil {
		t.Fatalf("preflight rejected paired surrogate: %v", err)
	}
}

func TestCatalogRejectsUnpairedSurrogateBeforeJSONReplacement(t *testing.T) {
	t.Parallel()
	body := mutateV4Fixture(t, func(_ map[string]any, _ map[string]any) {})
	body = strings.Replace(body, `"Create a metadata-complete skill source package."`, `"\uD800"`, 1)
	assertCatalogRejectionContains(t, body, "unpaired high surrogate")
}

func TestCatalogRejectsRawInvalidUTF8BeforeJSONReplacement(t *testing.T) {
	t.Parallel()
	valid := []byte(mutateV4Fixture(t, func(_ map[string]any, _ map[string]any) {}))
	needle := []byte("Create a metadata-complete skill source package.")
	for _, test := range []struct {
		name    string
		invalid []byte
	}{
		{name: "invalid FF", invalid: []byte{0xFF}},
		{name: "raw UTF-8 surrogate ED A0 80", invalid: []byte{0xED, 0xA0, 0x80}},
	} {
		t.Run(test.name, func(t *testing.T) {
			body := bytes.Replace(valid, needle, test.invalid, 1)
			if utf8.Valid(body) {
				t.Fatal("hostile full catalog unexpectedly has valid UTF-8")
			}
			err := loadCatalogBody(t, string(body))
			if err == nil || !strings.Contains(err.Error(), "catalog is not valid UTF-8") {
				t.Fatalf("LoadCatalog error = %v, want raw UTF-8 preflight rejection", err)
			}
		})
	}
}

func TestArtifactReferencesUseSharedLexicalContainment(t *testing.T) {
	t.Parallel()
	for _, reference := range []string{"/absolute", "../escape", `wrong\separator`, "nul\x00path"} {
		t.Run(reference, func(t *testing.T) {
			body := mutateV4Fixture(t, func(_ map[string]any, contract map[string]any) {
				contractArtifacts(contract, "produces")[0]["schema_ref"] = reference
			})
			assertCatalogRejectionContains(t, body, "repository-relative")
		})
	}
}

func TestArtifactReferenceDiagnosticsHaveStableFieldOrder(t *testing.T) {
	body := mutateV4Fixture(t, func(_ map[string]any, contract map[string]any) {
		artifact := contractArtifacts(contract, "produces")[0]
		artifact["schema_ref"] = "/invalid-schema"
		artifact["validator"] = "/invalid-validator"
	})
	for attempt := 0; attempt < 200; attempt++ {
		err := loadCatalogBody(t, body)
		if err == nil {
			t.Fatal("LoadCatalog accepted two invalid artifact references")
		}
		if !strings.Contains(err.Error(), "artifacts.produces[0].schema_ref") {
			t.Fatalf("attempt %d first diagnostic = %q, want schema_ref", attempt, err)
		}
		if strings.Contains(err.Error(), ".validator") {
			t.Fatalf("attempt %d diagnosed validator before schema_ref: %q", attempt, err)
		}
	}
}

func TestProofReferencesRejectNUL(t *testing.T) {
	t.Parallel()
	body := mutateV4Fixture(t, func(_ map[string]any, contract map[string]any) {
		contract["proof"].(map[string]any)["fixture_refs"] = []any{"fixture\x00.json"}
	})
	assertCatalogRejectionContains(t, body, "repository-relative")
}

func TestEmbeddedNormalizationMatchesCanonicalAndEveryScalar(t *testing.T) {
	canonical, err := os.ReadFile(filepath.Join("..", "..", "..", "schemas", "skill-trigger-normalization.v1.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(canonical, triggerNormalizationBytes) {
		t.Fatal("embedded normalization projection differs from canonical source")
	}
	normalization := pinnedTriggerNormalization()
	if normalization.UnicodeVersion != "16.0.0" {
		t.Fatalf("Unicode version = %q, want 16.0.0", normalization.UnicodeVersion)
	}
	if normalizeTriggerText("\u13F0") != normalizeTriggerText("\u13F8") {
		t.Fatal("Cherokee casefold U+13F0/U+13F8 diverged")
	}
	if actual := normalizeTriggerText("\u1C89"); actual != "\u1C8A" {
		t.Fatalf("Unicode 16.0 casefold delta U+1C89 = %q, want U+1C8A", actual)
	}
	whitespace := make(map[rune]bool, len(normalization.Whitespace))
	for _, value := range normalization.Whitespace {
		whitespace[rune(value)] = true
	}
	for value := rune(0); value <= utf8.MaxRune; value++ {
		if value >= 0xD800 && value <= 0xDFFF {
			continue
		}
		expected := string(value)
		if whitespace[value] {
			expected = ""
		} else if folded, exists := normalization.Casefold[fmt.Sprintf("%04X", value)]; exists {
			expected = folded
		}
		if actual := normalizeTriggerText(string(value)); actual != expected {
			t.Fatalf("U+%04X normalized to %q, want %q", value, actual, expected)
		}
	}
}
