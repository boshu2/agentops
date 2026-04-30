package openclaw

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	quest "github.com/boshu2/agentops/cli/internal/types/quest"
)

const SnapshotDirRel = ".agents/daemon/projections/openclaw"

type ProjectionInput struct {
	GeneratedAt time.Time
	Source      SnapshotSource
	Status      SnapshotStatus
	Runs        []ResourceSummary
	Jobs        []ResourceSummary
	Wiki        []ResourceSummary
}

type SnapshotStore struct {
	root string
}

func NewSnapshotStore(root string) *SnapshotStore {
	return &SnapshotStore{root: root}
}

func (s *SnapshotStore) Dir() string {
	return filepath.Join(s.root, SnapshotDirRel)
}

func (s *SnapshotStore) LatestPath() string {
	return filepath.Join(s.Dir(), "latest.json")
}

func (s *SnapshotStore) VersionPath(snapshotID string) (string, error) {
	filename, err := snapshotFilename(snapshotID)
	if err != nil {
		return "", err
	}
	return filepath.Join(s.Dir(), filename), nil
}

func BuildConsumerSnapshot(input ProjectionInput) (ConsumerSnapshot, error) {
	generatedAt := input.GeneratedAt
	if generatedAt.IsZero() {
		generatedAt = time.Now().UTC()
	}
	status := input.Status
	if status == "" {
		status = SnapshotStatusCurrent
	}
	snapshotID := "snap_empty"
	if strings.TrimSpace(input.Source.LastEventID) != "" {
		snapshotID = "snap_" + strings.TrimSpace(input.Source.LastEventID)
	}
	snapshot := ConsumerSnapshot{
		SchemaVersion: ConsumerSnapshotSchemaVersion,
		SnapshotID:    snapshotID,
		GeneratedAt:   generatedAt.UTC().Format(time.RFC3339Nano),
		Source:        input.Source,
		Status:        status,
		Resources: SnapshotResources{
			Runs: normalizeResourceKind(input.Runs, ResourceKindRun),
			Jobs: normalizeResourceKind(input.Jobs, ResourceKindJob),
			Wiki: normalizeResourceKind(input.Wiki, ResourceKindWiki),
		},
	}
	if err := ValidateConsumerSnapshot(snapshot); err != nil {
		return ConsumerSnapshot{}, err
	}
	return snapshot, nil
}

func (s *SnapshotStore) Rebuild(input ProjectionInput) (ConsumerSnapshot, error) {
	return BuildConsumerSnapshot(input)
}

func (s *SnapshotStore) WriteRebuilt(input ProjectionInput) (ConsumerSnapshot, error) {
	snapshot, err := s.Rebuild(input)
	if err != nil {
		return ConsumerSnapshot{}, err
	}
	if err := s.Write(snapshot); err != nil {
		return ConsumerSnapshot{}, err
	}
	return snapshot, nil
}

func (s *SnapshotStore) Write(snapshot ConsumerSnapshot) error {
	snapshot = normalizeSnapshot(snapshot)
	if err := ValidateConsumerSnapshot(snapshot); err != nil {
		return err
	}
	versionPath, err := s.VersionPath(snapshot.SnapshotID)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal OpenClaw snapshot: %w", err)
	}
	data = append(data, '\n')
	if err := os.MkdirAll(s.Dir(), 0700); err != nil {
		return fmt.Errorf("create OpenClaw snapshot dir: %w", err)
	}
	if err := quest.AtomicWriteFileWithPerm(versionPath, data, 0o600); err != nil {
		return err
	}
	if err := quest.AtomicWriteFileWithPerm(s.LatestPath(), data, 0o600); err != nil {
		return err
	}
	return nil
}

func (s *SnapshotStore) ReadLatest() (ConsumerSnapshot, error) {
	return s.ReadPath(s.LatestPath())
}

func (s *SnapshotStore) Read(snapshotID string) (ConsumerSnapshot, error) {
	path, err := s.VersionPath(snapshotID)
	if err != nil {
		return ConsumerSnapshot{}, err
	}
	return s.ReadPath(path)
}

func (s *SnapshotStore) ReadPath(path string) (ConsumerSnapshot, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return ConsumerSnapshot{}, fmt.Errorf("read OpenClaw snapshot: %w", err)
	}
	return ParseConsumerSnapshot(raw)
}

func normalizeSnapshot(snapshot ConsumerSnapshot) ConsumerSnapshot {
	snapshot.Resources.Runs = normalizeResourceKind(snapshot.Resources.Runs, ResourceKindRun)
	snapshot.Resources.Jobs = normalizeResourceKind(snapshot.Resources.Jobs, ResourceKindJob)
	snapshot.Resources.Wiki = normalizeResourceKind(snapshot.Resources.Wiki, ResourceKindWiki)
	return snapshot
}

func normalizeResourceKind(resources []ResourceSummary, kind ResourceKind) []ResourceSummary {
	out := make([]ResourceSummary, 0, len(resources))
	for _, resource := range resources {
		if resource.ResourceKind == "" {
			resource.ResourceKind = kind
		}
		if resource.ResourceID == "" {
			resource.ResourceID = firstNonEmpty(resource.RunID, resource.JobID)
			if resource.ResourceID == "" && resource.JobID != "" {
				resource.ResourceID = string(kind) + "-" + resource.JobID
			}
		}
		out = append(out, resource)
	}
	if out == nil {
		return []ResourceSummary{}
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func snapshotFilename(snapshotID string) (string, error) {
	snapshotID = strings.TrimSpace(snapshotID)
	if snapshotID == "" {
		return "", fmt.Errorf("%w: snapshot_id is required", ErrInvalidSnapshotSchema)
	}
	if snapshotID == "." || snapshotID == ".." || strings.ContainsAny(snapshotID, `/\`) {
		return "", fmt.Errorf("%w: unsafe snapshot_id %q", ErrInvalidSnapshotSchema, snapshotID)
	}
	return snapshotID + ".json", nil
}
