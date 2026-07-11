package close

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	closeapp "github.com/boshu2/agentops/cli/internal/close"
	"github.com/boshu2/agentops/cli/internal/trackerresolve"
)

type Tracker struct{}

type trackerRecord struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

func (Tracker) Resolve(_ context.Context, snapshot closeapp.Snapshot) (closeapp.Resolution, error) {
	resolved, err := trackerresolve.ResolveWithLookPath(snapshot.WorkDir, snapshot.Env, func(name string) (string, error) {
		return lookPath(snapshot.Env, name)
	})
	if err != nil {
		return closeapp.Resolution{}, err
	}
	return closeapp.Resolution{
		Backend: resolved.Tracker, Binary: resolved.Binary, LedgerDir: resolved.LedgerDir,
		RepoRoot: resolved.RepoRoot, WorkDir: resolved.WorkDir, ChildEnv: append([]string(nil), resolved.ChildEnv...),
	}, nil
}

func (Tracker) Status(ctx context.Context, resolution closeapp.Resolution, id string) (bool, error) {
	queries := [][]string{{"show", id, "--json"}, {"list", "--all", "--json"}}
	querySucceeded := false
	for _, args := range queries {
		out, code, err := runTracker(ctx, resolution, args...)
		if err != nil || code != 0 {
			continue
		}
		querySucceeded = true
		for _, item := range parseRecords(out) {
			if item.ID == id {
				return item.Status == "closed", nil
			}
		}
	}
	if querySucceeded {
		return false, fmt.Errorf("issue %s not found", id)
	}
	return false, fmt.Errorf("tracker status query failed")
}

func (Tracker) Close(ctx context.Context, resolution closeapp.Resolution, id, reason string) error {
	out, code, err := runTracker(ctx, resolution, "close", id, "--reason", reason)
	if err != nil || code != 0 {
		return effectError("tracker close", code, out, err)
	}
	return nil
}

func (Tracker) Sync(ctx context.Context, resolution closeapp.Resolution) error {
	out, code, err := runTracker(ctx, resolution, "sync", "--flush-only")
	if err == nil && code == 0 {
		return nil
	}
	out, code, err = runTracker(ctx, resolution, "sync")
	if err != nil || code != 0 {
		return effectError("tracker sync", code, out, err)
	}
	return nil
}

func runTracker(ctx context.Context, resolution closeapp.Resolution, args ...string) ([]byte, int, error) {
	command := exec.CommandContext(ctx, resolution.Binary, args...) // #nosec G204 -- tracker resolution constrains the binary to br|bd.
	command.Dir = resolution.WorkDir
	command.Env = append([]string(nil), resolution.ChildEnv...)
	return combined(command)
}

func parseRecords(out []byte) []trackerRecord {
	var records []trackerRecord
	if json.Unmarshal(out, &records) == nil {
		return records
	}
	var wrapped struct {
		Issues []trackerRecord `json:"issues"`
	}
	if json.Unmarshal(out, &wrapped) == nil {
		return wrapped.Issues
	}
	return nil
}

func lookPath(env []string, name string) (string, error) {
	if strings.ContainsRune(name, filepath.Separator) {
		return name, nil
	}
	pathValue := envLast(env, "PATH")
	for _, directory := range filepath.SplitList(pathValue) {
		candidate := filepath.Join(directory, name)
		info, err := os.Stat(candidate)
		if err == nil && !info.IsDir() && info.Mode()&0o111 != 0 {
			return candidate, nil
		}
	}
	return "", exec.ErrNotFound
}

func envLast(env []string, key string) string {
	prefix := key + "="
	for index := len(env) - 1; index >= 0; index-- {
		if strings.HasPrefix(env[index], prefix) {
			return strings.TrimPrefix(env[index], prefix)
		}
	}
	return ""
}

func combined(command *exec.Cmd) ([]byte, int, error) {
	out, err := command.CombinedOutput()
	if err == nil {
		return out, 0, nil
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return out, exitErr.ExitCode(), err
	}
	return out, 127, err
}

func effectError(operation string, code int, out []byte, cause error) error {
	detail := strings.TrimSpace(string(out))
	if detail == "" && cause != nil {
		detail = cause.Error()
	}
	if detail == "" {
		detail = "unknown failure"
	}
	return fmt.Errorf("%s (child exit %d): %s", operation, code, detail)
}
