package daemon

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	// ProjectionSnapshotDirName lives under .agents/daemon/.
	ProjectionSnapshotDirName = "projections"
	// ProjectionSnapshotPrefix matches files of the form snapshot-<ts>.json so
	// the timestamp encoded in the filename gives a stable chronological sort.
	ProjectionSnapshotPrefix = "snapshot-"
	ProjectionSnapshotSuffix = ".json"
)

// ProjectionSnapshotDir returns the on-disk location for projection snapshots.
// The dir is created lazily by WriteProjectionSnapshot.
func (s *Store) ProjectionSnapshotDir() string {
	return filepath.Join(s.Dir(), ProjectionSnapshotDirName)
}

// WriteProjectionSnapshot writes the ProjectionSet to a timestamped file under
// the snapshot dir using a temp-file + atomic rename so concurrent readers
// never observe a partial JSON document. Returns the absolute path written.
func (s *Store) WriteProjectionSnapshot(set ProjectionSet) (string, error) {
	if set.SchemaVersion == 0 {
		return "", fmt.Errorf("projection snapshot missing schema_version")
	}
	dir := s.ProjectionSnapshotDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", fmt.Errorf("create projection snapshot dir: %w", err)
	}
	ts := time.Now().UTC().Format("20060102T150405.000000000Z")
	dst := filepath.Join(dir, ProjectionSnapshotPrefix+ts+ProjectionSnapshotSuffix)

	data, err := json.MarshalIndent(set, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal projection snapshot: %w", err)
	}
	data = append(data, '\n')

	tmp := dst + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return "", fmt.Errorf("open projection snapshot tmp: %w", err)
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return "", fmt.Errorf("write projection snapshot tmp: %w", err)
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return "", fmt.Errorf("sync projection snapshot tmp: %w", err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return "", fmt.Errorf("close projection snapshot tmp: %w", err)
	}
	if err := os.Rename(tmp, dst); err != nil {
		_ = os.Remove(tmp)
		return "", fmt.Errorf("rename projection snapshot: %w", err)
	}
	return dst, nil
}

// ListProjectionSnapshots returns every snapshot file under the snapshot dir,
// sorted chronologically (oldest first). Both .json and .json.tmp leftovers
// from a crashed write are excluded; only completed snapshot-<ts>.json files
// are returned.
func (s *Store) ListProjectionSnapshots() ([]string, error) {
	dir := s.ProjectionSnapshotDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read projection snapshot dir: %w", err)
	}
	var out []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasPrefix(name, ProjectionSnapshotPrefix) {
			continue
		}
		if !strings.HasSuffix(name, ProjectionSnapshotSuffix) {
			continue
		}
		out = append(out, filepath.Join(dir, name))
	}
	sort.Strings(out)
	return out, nil
}

// LoadLatestProjectionSnapshot finds the newest snapshot by filename ordering
// and json-decodes it. Returns (zero ProjectionSet, "", nil) when no snapshot
// exists — callers must distinguish empty-set from absent via the path.
func (s *Store) LoadLatestProjectionSnapshot() (ProjectionSet, string, error) {
	paths, err := s.ListProjectionSnapshots()
	if err != nil {
		return ProjectionSet{}, "", err
	}
	if len(paths) == 0 {
		return ProjectionSet{}, "", nil
	}
	latest := paths[len(paths)-1]
	data, err := os.ReadFile(latest)
	if err != nil {
		return ProjectionSet{}, "", fmt.Errorf("read projection snapshot %s: %w", filepath.Base(latest), err)
	}
	var set ProjectionSet
	if err := json.Unmarshal(data, &set); err != nil {
		return ProjectionSet{}, "", fmt.Errorf("decode projection snapshot %s: %w", filepath.Base(latest), err)
	}
	if set.SchemaVersion != ProjectionSchemaVersion {
		return ProjectionSet{}, latest, fmt.Errorf("projection snapshot %s has schema_version %d, want %d", filepath.Base(latest), set.SchemaVersion, ProjectionSchemaVersion)
	}
	return set, latest, nil
}
