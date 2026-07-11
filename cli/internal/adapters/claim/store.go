package claim

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/boshu2/agentops/cli/internal/ports"
)

type EvidenceStore struct {
	mu        sync.Mutex
	directory func() (string, error)
}

func NewEvidenceStore(directory func() (string, error)) *EvidenceStore {
	return &EvidenceStore{directory: directory}
}

func (store *EvidenceStore) Bind(ctx context.Context, binding ports.EvidenceBinding) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if binding.Claim == "" {
		return errors.New("productionClaimEvidenceBinder: Claim required")
	}
	if binding.Path == "" {
		return errors.New("productionClaimEvidenceBinder: Path required")
	}
	if err := ports.ValidateEvidenceBindingReviewers(binding); err != nil {
		return fmt.Errorf("productionClaimEvidenceBinder: %w", err)
	}
	path, err := store.path()
	if err != nil {
		return err
	}

	store.mu.Lock()
	defer store.mu.Unlock()
	existing, err := store.scanLatest(path, binding.Claim, binding.Path)
	if err != nil {
		return err
	}
	if existing != nil {
		if levelRank(binding.Level) < levelRank(existing.Level) {
			return fmt.Errorf("productionClaimEvidenceBinder: refusing downgrade %s → %s for claim %q path %q", existing.Level, binding.Level, binding.Claim, binding.Path)
		}
		if binding.Level == existing.Level && sameAnchors(binding.Anchors, existing.Anchors) {
			return nil
		}
	}
	payload, err := json.Marshal(bindingRecord{
		Claim: string(binding.Claim), Path: binding.Path, Level: string(binding.Level),
		Anchors: binding.Anchors, AuthorID: binding.AuthorID, JudgeID: binding.JudgeID,
	})
	if err != nil {
		return fmt.Errorf("productionClaimEvidenceBinder marshal: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("productionClaimEvidenceBinder open %q: %w", path, err)
	}
	defer func() { _ = file.Close() }()
	if _, err := file.Write(append(payload, '\n')); err != nil {
		return fmt.Errorf("productionClaimEvidenceBinder write: %w", err)
	}
	return nil
}

func (store *EvidenceStore) List(ctx context.Context) ([]ports.EvidenceBinding, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	path, err := store.path()
	if err != nil {
		return nil, err
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	all, err := store.scan(path)
	if err != nil {
		return nil, err
	}
	out := make([]ports.EvidenceBinding, 0, len(all))
	for index := len(all) - 1; index >= 0; index-- {
		out = append(out, all[index])
	}
	return out, nil
}

func (store *EvidenceStore) path() (string, error) {
	if store == nil || store.directory == nil {
		return "", errors.New("productionClaimEvidenceBinder: file path required")
	}
	directory, err := store.directory()
	if err != nil {
		return "", err
	}
	if directory == "" {
		return "", errors.New("productionClaimEvidenceBinder: file path required")
	}
	return filepath.Join(directory, ".agents", "findings", "evidence-bindings.jsonl"), nil
}

func (store *EvidenceStore) scanLatest(path string, claim ports.ClaimID, evidencePath string) (*ports.EvidenceBinding, error) {
	all, err := store.scan(path)
	if err != nil {
		return nil, err
	}
	var found *ports.EvidenceBinding
	for index := range all {
		if all[index].Claim == claim && all[index].Path == evidencePath {
			copyOf := all[index]
			found = &copyOf
		}
	}
	return found, nil
}

func (store *EvidenceStore) scan(path string) ([]ports.EvidenceBinding, error) {
	out := make([]ports.EvidenceBinding, 0)
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return out, nil
		}
		return nil, fmt.Errorf("productionClaimEvidenceBinder open %q: %w", path, err)
	}
	defer func() { _ = file.Close() }()
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		var record bindingRecord
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			continue
		}
		out = append(out, ports.EvidenceBinding{
			Claim: ports.ClaimID(record.Claim), Path: record.Path,
			Level: ports.EvidenceLevel(record.Level), Anchors: record.Anchors,
			AuthorID: record.AuthorID, JudgeID: record.JudgeID,
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("productionClaimEvidenceBinder scan: %w", err)
	}
	return out, nil
}

type bindingRecord struct {
	Claim    string   `json:"claim"`
	Path     string   `json:"path"`
	Level    string   `json:"level,omitempty"`
	Anchors  []string `json:"anchors,omitempty"`
	AuthorID string   `json:"author_id,omitempty"`
	JudgeID  string   `json:"judge_id,omitempty"`
}

func levelRank(level ports.EvidenceLevel) int {
	switch level {
	case ports.EvidenceLevelPG1:
		return 1
	case ports.EvidenceLevelPG2:
		return 2
	case ports.EvidenceLevelPG3:
		return 3
	case ports.EvidenceLevelPG4:
		return 4
	default:
		return 0
	}
}

func sameAnchors(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

var _ ports.ClaimEvidenceBinderPort = (*EvidenceStore)(nil)
