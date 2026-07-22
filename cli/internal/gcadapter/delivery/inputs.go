package delivery

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/boshu2/agentops/cli/internal/verdictcheck"
)

// SubjectManifest is deliberately the same subject-manifest.v1 wire contract
// and content identity used by validator/skills/validate/scripts/validate.py.
// git_metadata is descriptive and excluded from canonical content identity.
type SubjectManifest struct {
	SchemaVersion           string          `json:"schema_version"`
	DeclaredRoots           []string        `json:"declared_roots"`
	Exclusions              []string        `json:"exclusions"`
	Entries                 []ManifestEntry `json:"entries"`
	BaseManifestDigest      string          `json:"base_manifest_digest,omitempty"`
	GitMetadata             json.RawMessage `json:"git_metadata,omitempty"`
	CanonicalManifestDigest string          `json:"canonical_manifest_digest"`
}

type ManifestEntry struct {
	Path       string `json:"path"`
	Kind       string `json:"kind"`
	Executable bool   `json:"executable"`
	Digest     string `json:"digest,omitempty"`
}

type ExecutableBinding struct {
	Path   string `json:"path"`
	Digest string `json:"digest"`
}

// NativeContext binds one fixed native delivery execution namespace.  The
// executable bindings are content identities, not caller-selectable commands.
type NativeContext struct {
	SchemaVersion       string                       `json:"schema_version"`
	RigID               string                       `json:"rig_id"`
	Repository          string                       `json:"repository"`
	RepositoryDir       string                       `json:"repository_dir"`
	WorktreeRoot        string                       `json:"worktree_root"`
	BeadsDir            string                       `json:"beads_dir"`
	Remote              string                       `json:"remote"`
	BaseRef             string                       `json:"base_ref"`
	SuccessorCapability string                       `json:"successor_capability_digest"`
	ToolchainLock       string                       `json:"toolchain_lock_digest"`
	ToolchainReceipt    string                       `json:"toolchain_receipt_path"`
	ToolchainReceiptSum string                       `json:"toolchain_receipt_digest"`
	BeadsRepresentation string                       `json:"beads_representation"`
	Executables         map[string]ExecutableBinding `json:"executables"`
	CheckOnlyGateArgv   [][]string                   `json:"check_only_gate_argv"`
}

func DecodeExactSubjectManifest(raw []byte, expectedDigest string) (SubjectManifest, error) {
	var manifest SubjectManifest
	if err := decodeStrict(raw, &manifest); err != nil {
		return SubjectManifest{}, err
	}
	if manifest.SchemaVersion != "subject-manifest.v1" || !isHex(manifest.CanonicalManifestDigest, 64) || expectedDigest != manifest.CanonicalManifestDigest {
		return SubjectManifest{}, errors.New("subject manifest identity is invalid")
	}
	var identity map[string]any
	if err := json.Unmarshal(raw, &identity); err != nil {
		return SubjectManifest{}, err
	}
	delete(identity, "canonical_manifest_digest")
	delete(identity, "git_metadata")
	canonical, err := verdictcheck.CanonicalJSON(identity)
	if err != nil {
		return SubjectManifest{}, err
	}
	sum := sha256.Sum256(canonical)
	if manifest.CanonicalManifestDigest != hex.EncodeToString(sum[:]) {
		return SubjectManifest{}, errors.New("subject manifest canonical digest is invalid")
	}
	if err := validateManifestShape(manifest); err != nil {
		return SubjectManifest{}, err
	}
	return manifest, nil
}

func validateManifestShape(manifest SubjectManifest) error {
	if len(manifest.DeclaredRoots) == 0 || !uniqueManifestPaths(manifest.DeclaredRoots, true) || !uniqueManifestPaths(manifest.Exclusions, true) {
		return errors.New("subject manifest declared roots or exclusions are invalid")
	}
	seen := map[string]bool{}
	previous := ""
	for _, entry := range manifest.Entries {
		if !safeRelativePath(entry.Path, false) || seen[entry.Path] || (previous != "" && entry.Path <= previous) || (entry.Kind != "file" && entry.Kind != "symlink" && entry.Kind != "deletion") {
			return errors.New("subject manifest entry is invalid")
		}
		if entry.Kind == "deletion" {
			if entry.Digest != "" {
				return errors.New("subject manifest deletion has a digest")
			}
		} else if !isHex(entry.Digest, 64) {
			return errors.New("subject manifest entry digest is invalid")
		}
		seen[entry.Path] = true
		previous = entry.Path
	}
	return nil
}

func uniqueManifestPaths(paths []string, allowDot bool) bool {
	if !sort.StringsAreSorted(paths) {
		return false
	}
	seen := map[string]bool{}
	for _, path := range paths {
		if !safeRelativePath(path, allowDot) || seen[path] {
			return false
		}
		seen[path] = true
	}
	return true
}

func DecodeExactNativeContext(raw []byte, digest string) (NativeContext, error) {
	var native NativeContext
	if err := decodeExactJSON(raw, digest, &native); err != nil {
		return NativeContext{}, err
	}
	if native.SchemaVersion != "gc-delivery-native-context.v1" || native.RigID == "" || native.Repository == "" || native.Remote == "" || native.BaseRef == "" || native.BeadsRepresentation != "B-successor-delivery-bead" || !isHex(native.SuccessorCapability, 64) || !isHex(native.ToolchainLock, 64) || !filepath.IsAbs(native.ToolchainReceipt) || !isHex(native.ToolchainReceiptSum, 64) {
		return NativeContext{}, errors.New("native context identity is invalid")
	}
	for _, path := range []string{native.RepositoryDir, native.WorktreeRoot, native.BeadsDir} {
		if !filepath.IsAbs(path) {
			return NativeContext{}, errors.New("native context paths must be absolute")
		}
	}
	for _, name := range []string{"gc", "bd", "git", "gh", "agentops-gc-delivery"} {
		binding, ok := native.Executables[name]
		if !ok || !filepath.IsAbs(binding.Path) || !isHex(binding.Digest, 64) {
			return NativeContext{}, errors.New("native executable binding is invalid")
		}
	}
	for _, gate := range native.CheckOnlyGateArgv {
		if len(gate) == 0 || !filepath.IsAbs(gate[0]) {
			return NativeContext{}, errors.New("native context gate must begin with an absolute executable")
		}
	}
	return native, nil
}

func ReadExactSubjectManifest(path, digest string) (SubjectManifest, []byte, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return SubjectManifest{}, nil, err
	}
	value, err := DecodeExactSubjectManifest(raw, digest)
	return value, raw, err
}

func ReadExactNativeContext(path, digest string) (NativeContext, []byte, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return NativeContext{}, nil, err
	}
	value, err := DecodeExactNativeContext(raw, digest)
	return value, raw, err
}

func decodeExactJSON(raw []byte, digest string, target any) error {
	sum := sha256.Sum256(raw)
	if digest != hex.EncodeToString(sum[:]) {
		return errors.New("exact input digest does not match bytes")
	}
	if err := decodeStrict(raw, target); err != nil {
		return err
	}
	canonical, err := json.Marshal(target)
	if err != nil {
		return err
	}
	if !bytes.Equal(bytes.TrimSpace(raw), canonical) {
		return errors.New("exact input is not canonical JSON")
	}
	return nil
}

func safeRelativePath(path string, allowDot bool) bool {
	if path == "" || strings.ContainsAny(path, "\\\\\x00") || filepath.IsAbs(path) {
		return false
	}
	if path == "." {
		return allowDot
	}
	for _, part := range strings.Split(path, "/") {
		if part == "" || part == "." || part == ".." {
			return false
		}
	}
	return true
}
