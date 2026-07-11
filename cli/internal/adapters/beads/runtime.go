package beads

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/boshu2/agentops/cli/internal/trackerresolve"
)

type Runtime struct{}

func NewRuntime() Runtime { return Runtime{} }

func (Runtime) Now() time.Time { return time.Now().UTC() }

func (Runtime) Actor() string { return os.Getenv("BEADS_ACTOR") }

func (Runtime) ResolveRepoPath(path string) (string, error) {
	if filepath.IsAbs(path) {
		return path, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("resolve cwd: %w", err)
	}
	return filepath.Join(trackerresolve.RepoRoot(cwd), path), nil
}

func (Runtime) AppendEvent(path string, event any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir ledger dir: %w", err)
	}
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open ledger: %w", err)
	}
	defer func() { _ = file.Close() }()
	if err := json.NewEncoder(file).Encode(event); err != nil {
		return fmt.Errorf("encode event: %w", err)
	}
	return nil
}
