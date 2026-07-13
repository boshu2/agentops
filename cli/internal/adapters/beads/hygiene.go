package beads

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	beadsapp "github.com/boshu2/agentops/cli/internal/beads"
)

type HygieneRepository struct {
	tracker *Tracker
	once    sync.Once
	index   map[string]string
}

func NewHygieneRepository(tracker *Tracker) *HygieneRepository {
	return &HygieneRepository{tracker: tracker}
}

func (repository *HygieneRepository) Available() bool {
	return repository != nil && repository.tracker != nil && repository.tracker.Available()
}

func (repository *HygieneRepository) List(status string) ([]beadsapp.BeadRecord, error) {
	raw, err := repository.tracker.Output(context.Background(), "list", "--status", status, "--json")
	if err != nil {
		return nil, fmt.Errorf("bd list --status %s --json: %w", status, err)
	}
	return beadsapp.ParseBDRecordList(raw)
}

func (repository *HygieneRepository) Show(id string) (beadsapp.BeadRecord, error) {
	raw, err := repository.tracker.Output(context.Background(), "show", id, "--json")
	if err != nil {
		return beadsapp.BeadRecord{}, err
	}
	return beadsapp.ParseBDRecord(raw)
}

func (repository *HygieneRepository) Close(id, note string) error {
	if _, err := repository.tracker.Output(context.Background(), "update", id, "--status", "closed", "--append-notes", note); err != nil {
		return fmt.Errorf("auto-close bead %s: %w", id, err)
	}
	return nil
}

func (repository *HygieneRepository) Reparent(id, parent string) error {
	_, err := repository.tracker.Output(context.Background(), "update", id, "--parent", parent)
	return err
}

func (repository *HygieneRepository) Commits() []beadsapp.AuditCommit {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	const recordSeparator, fieldSeparator = "\x1e", "\x1f"
	command := exec.CommandContext(ctx, "git", "log", "--all", "--name-only", "--pretty=format:"+recordSeparator+"%h"+fieldSeparator+"%cI"+fieldSeparator+"%s"+fieldSeparator+"%b"+fieldSeparator) // #nosec G204 -- fixed read-only git invocation.
	output, err := command.Output()
	if err != nil {
		return nil
	}
	var commits []beadsapp.AuditCommit
	for _, rawRecord := range strings.Split(string(output), recordSeparator) {
		rawRecord = strings.TrimLeft(rawRecord, "\n")
		if rawRecord == "" {
			continue
		}
		parts := strings.SplitN(rawRecord, fieldSeparator, 5)
		if len(parts) < 5 {
			continue
		}
		commitAt, _ := time.Parse(time.RFC3339, strings.TrimSpace(parts[1]))
		commit := beadsapp.AuditCommit{
			ShortSHA: strings.TrimSpace(parts[0]), CommitAt: commitAt,
			Subject: strings.TrimSpace(parts[2]), Body: parts[3], Files: make(map[string]struct{}),
		}
		for _, line := range strings.Split(parts[4], "\n") {
			if file := strings.TrimSpace(line); file != "" {
				commit.Files[file] = struct{}{}
			}
		}
		commits = append(commits, commit)
	}
	return commits
}

func (repository *HygieneRepository) PatternExists(pattern string) bool {
	if pattern == "" {
		return false
	}
	repository.once.Do(func() { repository.index = buildHygieneIndex() })
	for _, content := range repository.index {
		if strings.Contains(content, pattern) {
			return true
		}
	}
	return false
}

func buildHygieneIndex() map[string]string {
	index := make(map[string]string)
	for _, root := range []string{"cli", "skills", "skills-codex", "scripts", "docs", "tests"} {
		openRoot, err := os.OpenRoot(root)
		if err != nil {
			continue
		}
		_ = fs.WalkDir(openRoot.FS(), ".", func(walkPath string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return nil
			}
			if entry.IsDir() {
				switch path.Base(walkPath) {
				case ".git", ".beads", ".agents", "node_modules", "vendor", "testdata":
					return fs.SkipDir
				}
				return nil
			}
			switch filepath.Ext(walkPath) {
			case ".go", ".py", ".sh", ".ts", ".js", ".md":
			default:
				return nil
			}
			info, err := entry.Info()
			if err != nil || info.Size() > 1_000_000 {
				return nil
			}
			if content, err := openRoot.ReadFile(walkPath); err == nil {
				index[path.Join(root, walkPath)] = string(content)
			}
			return nil
		})
		_ = openRoot.Close()
	}
	return index
}

var _ beadsapp.HygieneRepository = (*HygieneRepository)(nil)
