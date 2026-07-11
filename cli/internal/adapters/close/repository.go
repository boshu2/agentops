package close

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	closeapp "github.com/boshu2/agentops/cli/internal/close"
)

type Repository struct{}

func (Repository) Preflight(ctx context.Context, snapshot closeapp.Snapshot, resolution closeapp.Resolution, evidence string, paths []string) error {
	first := firstEvidenceToken(evidence)
	if reason := EvidenceRefusal(ctx, snapshot, first); reason != "" {
		return fmt.Errorf("evidence %q %s", evidence, reason)
	}
	for _, path := range paths {
		if pathExists(snapshot.WorkDir, path) || gitTracksPath(ctx, snapshot, path) {
			continue
		}
		return fmt.Errorf("path %q resolves to no real or tracked artifact", path)
	}
	return nil
}

func (Repository) LedgerStatus(_ context.Context, resolution closeapp.Resolution, id string) (bool, error) {
	file, err := os.Open(filepath.Join(resolution.LedgerDir, "issues.jsonl"))
	if err != nil {
		return false, err
	}
	defer func() { _ = file.Close() }()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var item trackerRecord
		if json.Unmarshal(scanner.Bytes(), &item) == nil && item.ID == id {
			return item.Status == "closed", nil
		}
	}
	if err := scanner.Err(); err != nil {
		return false, err
	}
	return false, fmt.Errorf("issue %s not found", id)
}

func (Repository) CommitLedger(ctx context.Context, snapshot closeapp.Snapshot, resolution closeapp.Resolution, message string) (string, error) {
	before := gitHead(ctx, snapshot, resolution.LedgerDir)
	stage := []string{"-C", resolution.LedgerDir, "add", "--", "issues.jsonl"}
	if pathExists(snapshot.WorkDir, filepath.Join(resolution.LedgerDir, "metadata.json")) {
		stage = append(stage, "metadata.json")
	}
	if out, code, err := runGit(ctx, snapshot, stage...); err != nil || code != 0 {
		return "", effectError("ledger git add", code, out, err)
	}
	out, code, err := runGit(ctx, snapshot, "-C", resolution.LedgerDir, "commit", "-q", "-m", message)
	noop := false
	if err != nil || code != 0 {
		if gitCommitNothingToCommit(out) {
			noop = true
		} else {
			return "", effectError("ledger git commit", code, out, err)
		}
	}
	after := gitHead(ctx, snapshot, resolution.LedgerDir)
	if before == after && !noop {
		return "", fmt.Errorf("ledger git commit did not land")
	}
	return noneIfEmpty(after), nil
}

func (Repository) CommitPublic(ctx context.Context, snapshot closeapp.Snapshot, resolution closeapp.Resolution, message string, paths []string) (string, error) {
	stage := PublicStagePaths(snapshot.WorkDir, resolution.LedgerDir, paths)
	if len(stage) > 0 {
		args := append([]string{"add", "--"}, stage...)
		if out, code, err := runGit(ctx, snapshot, args...); err != nil || code != 0 {
			return "", effectError("public git add", code, out, err)
		}
		out, code, err := runGit(ctx, snapshot, "commit", "-q", "-m", message)
		if (err != nil || code != 0) && !gitCommitNothingToCommit(out) {
			return "", effectError("public git commit", code, out, err)
		}
	}
	return noneIfEmpty(gitHead(ctx, snapshot, snapshot.WorkDir)), nil
}

func PublicStagePaths(root, ledgerDir string, paths []string) []string {
	stage := make([]string, 0, len(paths))
	for _, path := range paths {
		if path == "" || isPrivateLedgerPath(root, ledgerDir, path) {
			continue
		}
		stage = append(stage, path)
	}
	return stage
}

func EvidenceRefusal(ctx context.Context, snapshot closeapp.Snapshot, first string) string {
	if first == "" {
		return "resolves to no real artifact"
	}
	abs := resolvePath(snapshot.WorkDir, first)
	if _, err := os.Stat(abs); err != nil {
		return "resolves to no real artifact"
	}
	if !hasGitHead(ctx, snapshot) {
		return ""
	}
	root, rel, ok := repoRelativeEvidence(ctx, snapshot, first)
	if !ok {
		return ""
	}
	if rel == ".." || strings.HasPrefix(rel, "../") {
		return "resolves outside the repository; durable evidence must be a committed file tracked in this repo"
	}
	if evidenceCommitted(ctx, snapshot, root, rel) || evidenceRuntimeCorpus(ctx, snapshot, root, rel) {
		return ""
	}
	return "is present but not a committed git blob in history (durable-evidence binding); commit the evidence before closing"
}

func firstEvidenceToken(evidence string) string {
	evidence = strings.TrimSpace(evidence)
	if index := strings.IndexByte(evidence, ' '); index >= 0 {
		return evidence[:index]
	}
	return evidence
}

func pathExists(root, path string) bool {
	if path == "" {
		return false
	}
	_, err := os.Stat(resolvePath(root, path))
	return err == nil
}

func resolvePath(root, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(root, path)
}

func gitTracksPath(ctx context.Context, snapshot closeapp.Snapshot, path string) bool {
	if path == "" {
		return false
	}
	_, code, err := runGit(ctx, snapshot, "ls-files", "--error-unmatch", "--", path)
	return err == nil && code == 0
}

func hasGitHead(ctx context.Context, snapshot closeapp.Snapshot) bool {
	_, code, err := runGit(ctx, snapshot, "rev-parse", "--verify", "-q", "HEAD")
	return err == nil && code == 0
}

func repoRelativeEvidence(ctx context.Context, snapshot closeapp.Snapshot, first string) (string, string, bool) {
	out, code, err := runGit(ctx, snapshot, "rev-parse", "--show-toplevel")
	if err != nil || code != 0 {
		return "", "", false
	}
	root := strings.TrimSpace(string(out))
	if !filepath.IsAbs(root) {
		return "", "", false
	}
	if resolved, resolveErr := filepath.EvalSymlinks(root); resolveErr == nil {
		root = resolved
	}
	var absolute string
	if filepath.IsAbs(first) {
		clean := filepath.Clean(first)
		parent := filepath.Dir(clean)
		if resolved, resolveErr := filepath.EvalSymlinks(parent); resolveErr == nil {
			parent = resolved
		}
		absolute = filepath.Join(parent, filepath.Base(clean))
	} else {
		base := snapshot.WorkDir
		if resolved, resolveErr := filepath.EvalSymlinks(base); resolveErr == nil {
			base = resolved
		}
		absolute = filepath.Clean(filepath.Join(base, first))
	}
	relative, err := filepath.Rel(root, absolute)
	if err != nil {
		return root, "", false
	}
	return root, filepath.ToSlash(relative), true
}

func evidenceCommitted(ctx context.Context, snapshot closeapp.Snapshot, root, relative string) bool {
	if relative == "" {
		return false
	}
	out, code, err := runGit(ctx, snapshot, "cat-file", "-t", "HEAD:"+relative)
	if err != nil || code != 0 || strings.TrimSpace(string(out)) != "blob" {
		return false
	}
	tree, treeCode, treeErr := runGit(ctx, snapshot, "-C", root, "ls-tree", "HEAD", "--", relative)
	if treeErr != nil || treeCode != 0 || !strings.HasPrefix(strings.TrimSpace(string(tree)), "100") {
		return false
	}
	_, diffCode, diffErr := runGit(ctx, snapshot, "-C", root, "diff", "--quiet", "HEAD", "--", relative)
	return diffErr == nil && diffCode == 0
}

func evidenceRuntimeCorpus(ctx context.Context, snapshot closeapp.Snapshot, root, relative string) bool {
	if relative != ".agents" && !strings.HasPrefix(relative, ".agents/") {
		return false
	}
	_, code, err := runGit(ctx, snapshot, "-C", root, "check-ignore", "-q", "--", relative)
	return err == nil && code == 0
}

func isPrivateLedgerPath(root, ledgerDir, path string) bool {
	resolved := filepath.Clean(resolvePath(root, path))
	ledger := filepath.Clean(resolvePath(root, ledgerDir))
	if relative, err := filepath.Rel(ledger, resolved); err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return true
	}
	clean := filepath.Clean(path)
	return clean == "_beads" || strings.HasPrefix(clean, "_beads"+string(filepath.Separator)) ||
		clean == ".beads" || strings.HasPrefix(clean, ".beads"+string(filepath.Separator))
}

func gitHead(ctx context.Context, snapshot closeapp.Snapshot, directory string) string {
	out, code, err := runGit(ctx, snapshot, "-C", directory, "rev-parse", "HEAD")
	if err != nil || code != 0 {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func gitCommitNothingToCommit(out []byte) bool {
	text := strings.ToLower(string(out))
	return strings.Contains(text, "nothing to commit") || strings.Contains(text, "nothing added to commit") || strings.Contains(text, "no changes added to commit")
}

func runGit(ctx context.Context, snapshot closeapp.Snapshot, args ...string) ([]byte, int, error) {
	binary, err := lookPath(snapshot.Env, "git")
	if err != nil {
		return nil, 127, err
	}
	command := exec.CommandContext(ctx, binary, args...) // #nosec G204 -- binary is resolved from the caller's explicit PATH.
	command.Dir = snapshot.WorkDir
	command.Env = append([]string(nil), snapshot.Env...)
	return combined(command)
}

func noneIfEmpty(value string) string {
	if value == "" {
		return "none"
	}
	return value
}
