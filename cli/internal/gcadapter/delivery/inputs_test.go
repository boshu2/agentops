package delivery

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/verdictcheck"
)

func canonicalWire(t testing.TB, value any) []byte {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var wire any
	if err := json.Unmarshal(encoded, &wire); err != nil {
		t.Fatal(err)
	}
	canonical, err := verdictcheck.CanonicalJSON(wire)
	if err != nil {
		t.Fatal(err)
	}
	return canonical
}

func TestDecodeExactNativeContextUsesWireCanonicalKeyOrder(t *testing.T) {
	native := NativeContext{
		SchemaVersion: "gc-delivery-native-context.v1", RigID: "agentops", Repository: "boshu2/agentops",
		RepositoryDir: "/repo", WorktreeRoot: "/worktrees", BeadsDir: "/repo/.beads", Remote: "origin", BaseRef: "main",
		SuccessorCapability: strings.Repeat("a", 64), ToolchainLock: strings.Repeat("b", 64),
		ToolchainReceipt: "/toolchain.json", ToolchainReceiptSum: strings.Repeat("c", 64), BeadsRepresentation: "B-successor-delivery-bead",
		Executables: map[string]ExecutableBinding{
			"gc": {Path: "/gc", Digest: strings.Repeat("1", 64)}, "bd": {Path: "/bd", Digest: strings.Repeat("2", 64)},
			"git": {Path: "/git", Digest: strings.Repeat("3", 64)}, "gh": {Path: "/gh", Digest: strings.Repeat("4", 64)},
			"bash": {Path: "/bash", Digest: strings.Repeat("5", 64)}, "agentops-gc-delivery": {Path: "/delivery", Digest: strings.Repeat("6", 64)},
		},
		CheckOnlyGateArgv: [][]string{{"/bash", "check"}},
	}
	canonical := canonicalWire(t, native)
	sum := sha256.Sum256(canonical)
	if _, err := DecodeExactNativeContext(canonical, fmt.Sprintf("%x", sum)); err != nil {
		t.Fatalf("sorted canonical wire: %v", err)
	}

	structOrder, err := json.Marshal(native)
	if err != nil {
		t.Fatal(err)
	}
	structSum := sha256.Sum256(structOrder)
	if _, err := DecodeExactNativeContext(structOrder, fmt.Sprintf("%x", structSum)); err == nil || !strings.Contains(err.Error(), "canonical JSON") {
		t.Fatalf("Go declaration-order wire accepted: %v", err)
	}
}

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
