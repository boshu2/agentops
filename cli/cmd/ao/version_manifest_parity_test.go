// practices: [continuous-delivery, supply-chain-integrity]
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// findReleaseManifestRoot walks up from the package directory looking for the
// AgentOps repo root, identified by the co-presence of the Go module file and
// the Claude plugin manifest. Both are tracked, so this works in a fresh clone.
// Returns "" when the test is not running inside a checkout.
func findReleaseManifestRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		_, modErr := os.Stat(filepath.Join(dir, "cli", "go.mod"))
		_, pluginErr := os.Stat(filepath.Join(dir, ".claude-plugin", "plugin.json"))
		if modErr == nil && pluginErr == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// jsonVersionAt walks a slash-separated path through a decoded JSON document
// and returns the string found there. A numeric element indexes an array
// (e.g. "plugins/0/version").
func jsonVersionAt(doc any, path string) (string, bool) {
	cur := doc
	for _, seg := range strings.Split(path, "/") {
		switch node := cur.(type) {
		case map[string]any:
			next, ok := node[seg]
			if !ok {
				return "", false
			}
			cur = next
		case []any:
			idx, err := strconv.Atoi(seg)
			if err != nil || idx < 0 || idx >= len(node) {
				return "", false
			}
			cur = node[idx]
		default:
			return "", false
		}
	}
	s, ok := cur.(string)
	return s, ok
}

// TestVersion_FallbackMatchesReleaseManifests guards the release-cut invariant
// that every version-bearing release surface agrees with the checked-in
// `version` fallback in main.go.
//
// This closes a real drift class, not a hypothetical one. At the 3.5.0 -> 3.6.0
// cut, `.claude-plugin/plugin.json`, `.claude-plugin/marketplace.json`,
// `.codex-plugin/plugin.json`, and `images/gemini/plugin.json` were bumped
// while `images/claude/verify.sh`'s EXPECTED_VERSION default was missed. The
// guard whose whole job is catching plugin.json drift behind the release then
// rejected the CORRECT version, so a user following `images/claude/README.md`
// on the shipped tag would have hit a hard FAIL. The
// `check_manifest_version_consistency` step in `scripts/ci-local-release.sh`
// compares only the two Claude manifests to each other, so it could not see
// this.
//
// ldflags-injected git-describe builds are exempt: they carry a "-g<hash>"
// commit segment or a "-dirty" suffix and are not the checked-in fallback.
func TestVersion_FallbackMatchesReleaseManifests(t *testing.T) {
	if strings.Contains(version, "-g") || strings.HasSuffix(version, "-dirty") {
		t.Skipf("version %q is an ldflags-injected build-describe string, not the checked-in fallback", version)
	}

	root := findReleaseManifestRoot(t)
	if root == "" {
		t.Skip("not running inside an AgentOps checkout; no release manifests to compare")
	}

	jsonSurfaces := map[string][]string{
		filepath.Join(".claude-plugin", "plugin.json"):      {"version"},
		filepath.Join(".claude-plugin", "marketplace.json"): {"metadata/version", "plugins/0/version"},
		filepath.Join(".codex-plugin", "plugin.json"):       {"version"},
		filepath.Join("images", "gemini", "plugin.json"):    {"version"},
	}
	for rel, paths := range jsonSurfaces {
		raw, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Errorf("release manifest %s is unreadable: %v", rel, err)
			continue
		}
		var doc any
		if err := json.Unmarshal(raw, &doc); err != nil {
			t.Errorf("release manifest %s is not valid JSON: %v", rel, err)
			continue
		}
		for _, path := range paths {
			got, ok := jsonVersionAt(doc, path)
			if !ok {
				t.Errorf("%s: no string value at JSON path %q", rel, path)
				continue
			}
			if got != version {
				t.Errorf("%s: %s = %q, want %q (main.go release fallback)", rel, path, got, version)
			}
		}
	}

	// images/claude/verify.sh hard-codes the expected release as the default of
	// an env-overridable variable, so it needs a textual read, not a JSON one.
	verifyRel := filepath.Join("images", "claude", "verify.sh")
	raw, err := os.ReadFile(filepath.Join(root, verifyRel))
	if err != nil {
		t.Fatalf("read %s: %v", verifyRel, err)
	}
	expectedVersionRe := regexp.MustCompile(`EXPECTED_VERSION="\$\{AGENTOPS_EXPECTED_VERSION:-([^}"]+)\}"`)
	match := expectedVersionRe.FindSubmatch(raw)
	if match == nil {
		t.Fatalf("%s: no EXPECTED_VERSION default found; the version guard was renamed or removed", verifyRel)
	}
	if got := string(match[1]); got != version {
		t.Errorf("%s: EXPECTED_VERSION default = %q, want %q (main.go release fallback)", verifyRel, got, version)
	}
}
