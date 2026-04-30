package openclaw

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

const ConsumerSnapshotSchemaVersion = 1

var (
	ErrUnsupportedSnapshotVersion = errors.New("unsupported OpenClaw snapshot schema_version")
	ErrInvalidSnapshotSchema      = errors.New("invalid OpenClaw snapshot schema")
)

type SnapshotStatus string

const (
	SnapshotStatusCurrent  SnapshotStatus = "current"
	SnapshotStatusStale    SnapshotStatus = "stale"
	SnapshotStatusDegraded SnapshotStatus = "degraded"
)

type ResourceKind string

const (
	ResourceKindRun  ResourceKind = "run"
	ResourceKindJob  ResourceKind = "job"
	ResourceKindWiki ResourceKind = "wiki"
)

// ConsumerSnapshot is the versioned, read-only projection consumed by
// OpenClaw-style clients. It deliberately uses public string shapes instead of
// daemon package types so clients do not depend on AgentOps internals.
type ConsumerSnapshot struct {
	SchemaVersion int               `json:"schema_version"`
	SnapshotID    string            `json:"snapshot_id"`
	GeneratedAt   string            `json:"generated_at"`
	Source        SnapshotSource    `json:"source"`
	Status        SnapshotStatus    `json:"status"`
	Resources     SnapshotResources `json:"resources"`
}

type SnapshotSource struct {
	Ledger      string `json:"ledger"`
	LastEventID string `json:"last_event_id,omitempty"`
}

type SnapshotResources struct {
	Runs []ResourceSummary `json:"runs"`
	Jobs []ResourceSummary `json:"jobs"`
	Wiki []ResourceSummary `json:"wiki"`
}

type ResourceSummary struct {
	ResourceID        string                 `json:"resource_id"`
	ResourceKind      ResourceKind           `json:"resource_kind"`
	JobID             string                 `json:"job_id,omitempty"`
	JobType           string                 `json:"job_type,omitempty"`
	RunID             string                 `json:"run_id,omitempty"`
	RequestID         string                 `json:"request_id,omitempty"`
	RequestIDs        []string               `json:"request_ids,omitempty"`
	Status            string                 `json:"status"`
	ResultStatus      string                 `json:"result_status,omitempty"`
	Failure           *FailureSummary        `json:"failure,omitempty"`
	Artifacts         map[string]string      `json:"artifacts,omitempty"`
	ArtifactRefs      map[string]ArtifactRef `json:"artifact_refs,omitempty"`
	ProjectionTargets []string               `json:"projection_targets,omitempty"`
	CreatedAt         string                 `json:"created_at,omitempty"`
	UpdatedAt         string                 `json:"updated_at,omitempty"`
	LastEventID       string                 `json:"last_event_id,omitempty"`
	Provenance        []ProvenanceLink       `json:"provenance,omitempty"`
}

type FailureSummary struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Retryable bool   `json:"retryable,omitempty"`
}

type ArtifactRef struct {
	Path      string `json:"path"`
	SHA256    string `json:"sha256"`
	Size      int64  `json:"size"`
	WrittenAt string `json:"written_at"`
}

type ProvenanceLink struct {
	Rel      string `json:"rel"`
	Kind     string `json:"kind"`
	URI      string `json:"uri,omitempty"`
	Path     string `json:"path,omitempty"`
	JobID    string `json:"job_id,omitempty"`
	RunID    string `json:"run_id,omitempty"`
	EventID  string `json:"event_id,omitempty"`
	Artifact string `json:"artifact,omitempty"`
}

func ParseConsumerSnapshot(raw []byte) (ConsumerSnapshot, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return ConsumerSnapshot{}, fmt.Errorf("%w: empty snapshot", ErrInvalidSnapshotSchema)
	}
	var snapshot ConsumerSnapshot
	dec := json.NewDecoder(bytes.NewReader(raw))
	if err := dec.Decode(&snapshot); err != nil {
		return ConsumerSnapshot{}, fmt.Errorf("%w: %v", ErrInvalidSnapshotSchema, err)
	}
	var extra struct{}
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		return ConsumerSnapshot{}, fmt.Errorf("%w: trailing JSON tokens", ErrInvalidSnapshotSchema)
	}
	if err := ValidateConsumerSnapshot(snapshot); err != nil {
		return ConsumerSnapshot{}, err
	}
	return snapshot, nil
}

func ValidateConsumerSnapshot(snapshot ConsumerSnapshot) error {
	if snapshot.SchemaVersion != ConsumerSnapshotSchemaVersion {
		return fmt.Errorf("%w: got %d want %d", ErrUnsupportedSnapshotVersion, snapshot.SchemaVersion, ConsumerSnapshotSchemaVersion)
	}
	if strings.TrimSpace(snapshot.SnapshotID) == "" {
		return fmt.Errorf("%w: snapshot_id is required", ErrInvalidSnapshotSchema)
	}
	if strings.TrimSpace(snapshot.GeneratedAt) == "" {
		return fmt.Errorf("%w: generated_at is required", ErrInvalidSnapshotSchema)
	}
	if _, err := time.Parse(time.RFC3339Nano, snapshot.GeneratedAt); err != nil {
		return fmt.Errorf("%w: generated_at: %v", ErrInvalidSnapshotSchema, err)
	}
	if strings.TrimSpace(snapshot.Source.Ledger) == "" {
		return fmt.Errorf("%w: source.ledger is required", ErrInvalidSnapshotSchema)
	}
	switch snapshot.Status {
	case SnapshotStatusCurrent, SnapshotStatusStale, SnapshotStatusDegraded:
	default:
		return fmt.Errorf("%w: unsupported status %q", ErrInvalidSnapshotSchema, snapshot.Status)
	}
	if err := validateResources(snapshot.Resources.Runs, ResourceKindRun); err != nil {
		return err
	}
	if err := validateResources(snapshot.Resources.Jobs, ResourceKindJob); err != nil {
		return err
	}
	if err := validateResources(snapshot.Resources.Wiki, ResourceKindWiki); err != nil {
		return err
	}
	return nil
}

func validateResources(resources []ResourceSummary, expected ResourceKind) error {
	for i, resource := range resources {
		if strings.TrimSpace(resource.ResourceID) == "" {
			return fmt.Errorf("%w: resources.%s[%d].resource_id is required", ErrInvalidSnapshotSchema, expected, i)
		}
		if resource.ResourceKind != expected {
			return fmt.Errorf("%w: resources.%s[%d].resource_kind = %q", ErrInvalidSnapshotSchema, expected, i, resource.ResourceKind)
		}
		if strings.TrimSpace(resource.Status) == "" {
			return fmt.Errorf("%w: resources.%s[%d].status is required", ErrInvalidSnapshotSchema, expected, i)
		}
		for j, link := range resource.Provenance {
			if strings.TrimSpace(link.Rel) == "" || strings.TrimSpace(link.Kind) == "" {
				return fmt.Errorf("%w: resources.%s[%d].provenance[%d] requires rel and kind", ErrInvalidSnapshotSchema, expected, i, j)
			}
			if strings.TrimSpace(link.URI) == "" && strings.TrimSpace(link.Path) == "" && strings.TrimSpace(link.JobID) == "" && strings.TrimSpace(link.RunID) == "" && strings.TrimSpace(link.EventID) == "" {
				return fmt.Errorf("%w: resources.%s[%d].provenance[%d] needs a target", ErrInvalidSnapshotSchema, expected, i, j)
			}
		}
	}
	return nil
}
