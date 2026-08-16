package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// handoffArtifact is caller-authored session evidence. It deliberately carries
// no tracker, reservation, phase, retry, ownership, or next-work state.
type handoffArtifact struct {
	SchemaVersion int           `json:"schema_version"`
	ID            string        `json:"id"`
	CreatedAt     string        `json:"created_at"`
	Goal          string        `json:"goal,omitempty"`
	Summary       string        `json:"summary,omitempty"`
	Continuation  string        `json:"continuation,omitempty"`
	State         *handoffState `json:"state,omitempty"`
}

// handoffState contains optional read-only Git observations. Git availability
// never controls whether a handoff can be written or consumed.
type handoffState struct {
	GitBranch     string   `json:"git_branch,omitempty"`
	GitDirty      bool     `json:"git_dirty"`
	ModifiedFiles []string `json:"modified_files,omitempty"`
	RecentCommits []string `json:"recent_commits,omitempty"`
}

var (
	handoffGoal         string
	handoffContinuation string
	handoffCollect      bool
	handoffDryRun       bool
)

var handoffCmd = &cobra.Command{
	Use:   "handoff [summary]",
	Short: "Write caller-authored session evidence",
	Long: `Write a small handoff artifact without selecting work, claiming it,
deciding what happens next, or restarting a runtime. --collect adds only
best-effort, read-only Git observations.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runHandoff,
}

func init() {
	// handoffCmd is attached to the session parent by newSessionCommand in
	// session_composition.go; the session parent now lives in the session module.
	handoffCmd.Flags().StringVar(&handoffGoal, "goal", "", "Caller-supplied goal")
	handoffCmd.Flags().StringVar(&handoffContinuation, "continuation", "", "Caller-supplied continuation note")
	handoffCmd.Flags().BoolVar(&handoffCollect, "collect", false, "Collect best-effort read-only Git observations")
	handoffCmd.Flags().BoolVar(&handoffDryRun, "dry-run", false, "Print the artifact without writing it")
}

func runHandoff(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get cwd: %w", err)
	}
	now := time.Now().UTC()
	// The id carries sub-second precision so two handoffs written in the same
	// second cannot collide on the same .agents/ao/handoff/<id>.json path. The
	// handoff.v1.schema.json id pattern accepts the optional fractional part.
	artifact := handoffArtifact{
		SchemaVersion: 1,
		ID:            "handoff-" + now.Format("20060102T150405.000000000Z"),
		CreatedAt:     now.Format(time.RFC3339Nano),
		Goal:          handoffGoal,
		Continuation:  handoffContinuation,
	}
	if len(args) == 1 {
		artifact.Summary = args[0]
	}
	if handoffCollect {
		artifact.State = collectHandoffState(cwd)
	}

	data, err := json.MarshalIndent(artifact, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal handoff: %w", err)
	}
	data = append(data, '\n')
	if handoffDryRun {
		_, err = cmd.OutOrStdout().Write(data)
		return err
	}

	path, err := writeHandoffArtifact(cwd, &artifact, data)
	if err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Handoff written: %s\n", path)
	return nil
}

func collectHandoffState(cwd string) *handoffState {
	state := &handoffState{}
	if branch, err := getCurrentBranch(cwd); err == nil {
		state.GitBranch = branch
	}
	state.ModifiedFiles = gitChangedFiles(cwd, 20)
	state.GitDirty = len(state.ModifiedFiles) > 0
	command := exec.Command("git", "log", "--oneline", "-5", "--no-decorate")
	command.Dir = cwd
	command.Env = gitDiscoveryEnv()
	if output, err := command.Output(); err == nil {
		for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
			if line = strings.TrimSpace(line); line != "" {
				state.RecentCommits = append(state.RecentCommits, line)
			}
		}
	}
	return state
}

func writeHandoffArtifact(cwd string, artifact *handoffArtifact, data []byte) (string, error) {
	dir := filepath.Join(cwd, ".agents", "ao", "handoff")
	if artifact == nil || artifact.ID == "" || artifact.ID == "." || artifact.ID == ".." || strings.ContainsAny(artifact.ID, `/\\`) {
		return "", fmt.Errorf("publish handoff: invalid artifact id")
	}
	root, err := openHandoffWriteRoot(cwd, true)
	if err != nil {
		return "", err
	}
	defer func() { _ = root.Close() }()

	targetName := artifact.ID + ".json"
	target := filepath.Join(dir, targetName)
	tmpName, tmp, err := createHandoffTemp(root)
	if err != nil {
		return "", fmt.Errorf("create handoff temporary file: %w", err)
	}
	defer func() { _ = root.Remove(tmpName) }()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return "", fmt.Errorf("write handoff: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return "", fmt.Errorf("flush handoff: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return "", fmt.Errorf("close handoff: %w", err)
	}
	if err := verifyHandoffWriteRoot(cwd, root); err != nil {
		return "", err
	}
	// A hard-link publish is an atomic no-clobber operation: unlike Rename it
	// never replaces evidence already stored under the same id. Both names are
	// resolved by the descriptor-anchored Root, so a parent-directory swap
	// cannot redirect the write through a symlink.
	if err := root.Link(tmpName, targetName); err != nil {
		return "", fmt.Errorf("publish handoff: %w", err)
	}
	if err := verifyHandoffWriteRoot(cwd, root); err != nil {
		if removeErr := root.Remove(targetName); removeErr != nil {
			return "", fmt.Errorf("%w; cleanup published handoff: %w", err, removeErr)
		}
		return "", err
	}
	if err := root.Remove(tmpName); err != nil {
		return "", fmt.Errorf("remove handoff temporary file after publish: %w", err)
	}
	return target, nil
}

// openHandoffWriteRoot resolves .agents/ao/handoff one component at a time.
// Existing components must be real directories, and every opened descriptor
// must still identify the component that was inspected. Missing components
// are created only when create is true.
func openHandoffWriteRoot(cwd string, create bool) (*os.Root, error) {
	root, err := os.OpenRoot(cwd)
	if err != nil {
		return nil, fmt.Errorf("open workspace root: %w", err)
	}
	current := root
	for _, component := range []string{".agents", "ao", "handoff"} {
		next, openErr := openRealHandoffDir(current, component, create)
		if openErr != nil {
			_ = current.Close()
			return nil, fmt.Errorf("open handoff directory component %s: %w", component, openErr)
		}
		_ = current.Close()
		current = next
	}
	return current, nil
}

func openRealHandoffDir(parent *os.Root, component string, create bool) (*os.Root, error) {
	for {
		before, err := parent.Lstat(component)
		if err != nil {
			if create && os.IsNotExist(err) {
				if mkdirErr := parent.Mkdir(component, 0o755); mkdirErr != nil && !os.IsExist(mkdirErr) {
					return nil, mkdirErr
				}
				continue
			}
			return nil, err
		}
		if before.Mode()&os.ModeSymlink != 0 || !before.IsDir() {
			return nil, fmt.Errorf("not a real directory (refused_unsafe)")
		}
		next, err := parent.OpenRoot(component)
		if err != nil {
			return nil, err
		}
		opened, statErr := next.Stat(".")
		after, afterErr := parent.Lstat(component)
		if statErr != nil || afterErr != nil || after.Mode()&os.ModeSymlink != 0 || !after.IsDir() || !os.SameFile(before, opened) || !os.SameFile(after, opened) {
			_ = next.Close()
			return nil, fmt.Errorf("changed identity while opening (refused_unsafe)")
		}
		return next, nil
	}
}

func verifyHandoffWriteRoot(cwd string, opened *os.Root) error {
	current, err := openHandoffWriteRoot(cwd, false)
	if err != nil {
		return fmt.Errorf("verify handoff directory: %w", err)
	}
	defer func() { _ = current.Close() }()
	want, err := opened.Stat(".")
	if err != nil {
		return fmt.Errorf("stat opened handoff directory: %w", err)
	}
	got, err := current.Stat(".")
	if err != nil {
		return fmt.Errorf("stat current handoff directory: %w", err)
	}
	if !os.SameFile(want, got) {
		return fmt.Errorf("verify handoff directory: path changed identity (refused_unsafe)")
	}
	return nil
}

func createHandoffTemp(root *os.Root) (string, *os.File, error) {
	for range 100 {
		var random [16]byte
		if _, err := rand.Read(random[:]); err != nil {
			return "", nil, err
		}
		name := ".handoff-" + hex.EncodeToString(random[:]) + ".tmp"
		file, err := root.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o600)
		if err == nil {
			return name, file, nil
		}
		if !os.IsExist(err) {
			return "", nil, err
		}
	}
	return "", nil, fmt.Errorf("exhausted unique temporary names")
}
