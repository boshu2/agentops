package delivery

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"

	"github.com/boshu2/agentops/cli/internal/verdictcheck"
)

func TestDecodeExactSubjectManifestAcceptsExecutorCanonicalIdentity(t *testing.T) {
	identity := map[string]any{
		"schema_version": "subject-manifest.v1",
		"declared_roots": []string{"."},
		"exclusions":     []string{".git"},
		"entries": []any{map[string]any{
			"path": "cmd/run", "kind": "file", "executable": true,
			"digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		}},
	}
	canonical, err := verdictcheck.CanonicalJSON(identity)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(canonical)
	digest := hex.EncodeToString(sum[:])
	raw := []byte("{\n  \"schema_version\": \"subject-manifest.v1\",\n  \"declared_roots\": [\".\"],\n  \"exclusions\": [\".git\"],\n  \"entries\": [{\"path\": \"cmd/run\", \"kind\": \"file\", \"executable\": true, \"digest\": \"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\"}],\n  \"git_metadata\": {\"descriptive\": true},\n  \"canonical_manifest_digest\": \"" + digest + "\"\n}\n")

	manifest, err := DecodeExactSubjectManifest(raw, digest)
	if err != nil {
		t.Fatalf("executor-compatible pretty manifest: %v", err)
	}
	if manifest.CanonicalManifestDigest != digest {
		t.Fatalf("digest = %q, want %q", manifest.CanonicalManifestDigest, digest)
	}
}

func TestSubjectManifestDigestMatchesPythonEscapingRules(t *testing.T) {
	identity := map[string]any{
		"schema_version": "subject-manifest.v1",
		"declared_roots": []string{"."},
		"exclusions":     []string{"docs/<keep>\u2028\u2029"},
		"entries":        []any{},
	}
	canonical, err := verdictcheck.CanonicalJSON(identity)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(canonical)
	digest := hex.EncodeToString(sum[:])
	manifest := map[string]any{}
	for key, value := range identity {
		manifest[key] = value
	}
	manifest["git_metadata"] = map[string]any{"html": "<ok>&"}
	manifest["canonical_manifest_digest"] = digest
	raw, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeExactSubjectManifest(raw, digest); err != nil {
		t.Fatalf("Python canonical escaping-compatible manifest: %v", err)
	}
}
