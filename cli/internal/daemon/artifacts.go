package daemon

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const artifactStoreRel = ".agents/handoffs/sha256"

// ArtifactRef is the ledger-resident identity for an artifact stored by
// content hash. Path is repository-relative so replay can reconstruct the
// compatibility artifact map without consulting local runtime state.
type ArtifactRef struct {
	Path      string `json:"path"`
	SHA256    string `json:"sha256"`
	Size      int64  `json:"size"`
	WrittenAt string `json:"written_at"`
}

type ArtifactStoreOptions struct {
	Now func() time.Time
}

type ContentAddressedArtifactStore struct {
	root string
	now  func() time.Time
}

func NewContentAddressedArtifactStore(root string, opts ArtifactStoreOptions) *ContentAddressedArtifactStore {
	now := opts.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &ContentAddressedArtifactStore{root: root, now: now}
}

func (r ArtifactRef) Validate() error {
	if strings.TrimSpace(r.Path) == "" {
		return fmt.Errorf("artifact path is required")
	}
	if len(r.SHA256) != sha256.Size*2 {
		return fmt.Errorf("artifact sha256 must be %d hex characters", sha256.Size*2)
	}
	if _, err := hex.DecodeString(r.SHA256); err != nil {
		return fmt.Errorf("artifact sha256 must be hex: %w", err)
	}
	if r.Size < 0 {
		return fmt.Errorf("artifact size must be >= 0")
	}
	if _, err := time.Parse(time.RFC3339Nano, r.WrittenAt); err != nil {
		return fmt.Errorf("artifact written_at is invalid: %w", err)
	}
	return nil
}

func (s *ContentAddressedArtifactStore) PutBytes(data []byte) (ArtifactRef, error) {
	sum := sha256.Sum256(data)
	digest := hex.EncodeToString(sum[:])
	relPath := filepath.ToSlash(filepath.Join(artifactStoreRel, digest[:2], digest[2:4], digest))
	absPath := filepath.Join(s.root, filepath.FromSlash(relPath))
	ref := ArtifactRef{
		Path:      relPath,
		SHA256:    digest,
		Size:      int64(len(data)),
		WrittenAt: s.now().UTC().Format(time.RFC3339Nano),
	}
	if info, err := os.Stat(absPath); err == nil {
		if err := verifyArtifactFile(absPath, digest); err != nil {
			return ArtifactRef{}, err
		}
		ref.WrittenAt = info.ModTime().UTC().Format(time.RFC3339Nano)
		return ref, nil
	} else if !os.IsNotExist(err) {
		return ArtifactRef{}, fmt.Errorf("stat artifact %s: %w", relPath, err)
	}
	if err := os.MkdirAll(filepath.Dir(absPath), 0o700); err != nil {
		return ArtifactRef{}, fmt.Errorf("create artifact dir: %w", err)
	}
	tmp, err := writeArtifactTemp(filepath.Dir(absPath), digest, data)
	if err != nil {
		return ArtifactRef{}, err
	}
	if err := os.Rename(tmp, absPath); err != nil {
		_ = os.Remove(tmp)
		if info, statErr := os.Stat(absPath); statErr == nil {
			if verifyErr := verifyArtifactFile(absPath, digest); verifyErr != nil {
				return ArtifactRef{}, verifyErr
			}
			ref.WrittenAt = info.ModTime().UTC().Format(time.RFC3339Nano)
			return ref, nil
		}
		return ArtifactRef{}, fmt.Errorf("commit artifact %s: %w", relPath, err)
	}
	return ref, nil
}

func writeArtifactTemp(dir, digest string, data []byte) (string, error) {
	tmp := filepath.Join(dir, "."+digest+".tmp")
	file, err := os.OpenFile(tmp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if os.IsExist(err) {
		tmp = filepath.Join(dir, fmt.Sprintf(".%s.%d.tmp", digest, os.Getpid()))
		file, err = os.OpenFile(tmp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	}
	if err != nil {
		return "", fmt.Errorf("create artifact temp: %w", err)
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		_ = os.Remove(tmp)
		return "", fmt.Errorf("write artifact temp: %w", err)
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		_ = os.Remove(tmp)
		return "", fmt.Errorf("sync artifact temp: %w", err)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(tmp)
		return "", fmt.Errorf("close artifact temp: %w", err)
	}
	return tmp, nil
}

func verifyArtifactFile(path, wantSHA256 string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open existing artifact: %w", err)
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return fmt.Errorf("hash existing artifact: %w", err)
	}
	if got := hex.EncodeToString(hash.Sum(nil)); got != wantSHA256 {
		return fmt.Errorf("artifact hash path contains different content: path=%s got=%s want=%s", path, got, wantSHA256)
	}
	return nil
}

func cloneArtifactRefs(in map[string]ArtifactRef) map[string]ArtifactRef {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]ArtifactRef, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func validateArtifactRefs(refs map[string]ArtifactRef) error {
	for key, ref := range refs {
		if strings.TrimSpace(key) == "" {
			return fmt.Errorf("artifact ref key is required")
		}
		if err := ref.Validate(); err != nil {
			return fmt.Errorf("artifact ref %q: %w", key, err)
		}
	}
	return nil
}

func artifactRefsFromPayload(payload map[string]any) map[string]ArtifactRef {
	raw, ok := payload["artifact_refs"]
	if !ok {
		return nil
	}
	out := map[string]ArtifactRef{}
	switch values := raw.(type) {
	case map[string]ArtifactRef:
		for key, value := range values {
			out[key] = value
		}
	case map[string]any:
		for key, value := range values {
			ref, ok := artifactRefFromValue(value)
			if ok {
				out[key] = ref
			}
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func artifactRefFromValue(value any) (ArtifactRef, bool) {
	if ref, ok := value.(ArtifactRef); ok {
		return ref, true
	}
	data, err := json.Marshal(value)
	if err != nil {
		return ArtifactRef{}, false
	}
	var ref ArtifactRef
	if err := json.Unmarshal(data, &ref); err != nil {
		return ArtifactRef{}, false
	}
	return ref, true
}
