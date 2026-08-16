package sessionapp

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPruneAgentsDryRunReportsWithoutDeleting(t *testing.T) {
	repo := t.TempDir()
	now := time.Date(2026, 8, 16, 18, 0, 0, 0, time.UTC)
	writePruneCandidates(t, filepath.Join(repo, ".agents", "ao", "handoff"), "handoff", 12, now.Add(-time.Hour))

	var output bytes.Buffer
	result, err := PruneAgents(PruneAgentsOptions{
		RepoRoot: repo,
		Stdout:   &output,
		Now:      func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("PruneAgents dry run: %v", err)
	}
	if result.Files != 2 {
		t.Fatalf("dry-run candidate count = %d, want 2", result.Files)
	}
	if got := countDirectFiles(t, filepath.Join(repo, ".agents", "ao", "handoff")); got != 12 {
		t.Fatalf("dry run left %d handoffs, want 12", got)
	}
	if !strings.Contains(output.String(), "Files that would be deleted: 2") {
		t.Fatalf("dry-run summary missing candidate count:\n%s", output.String())
	}
}

func TestPruneAgentsExecutePreservesLegacyAndDistinctMTOHandoff(t *testing.T) {
	repo := t.TempDir()
	now := time.Date(2026, 8, 16, 18, 0, 0, 0, time.UTC)
	canonical := filepath.Join(repo, ".agents", "ao", "handoff")
	legacy := filepath.Join(repo, ".agents", "handoff")
	mto := filepath.Join(repo, ".agents", "mto-handoff")
	writePruneCandidates(t, canonical, "canonical", 12, now.Add(-time.Hour))
	writePruneCandidates(t, legacy, "legacy", 12, now.Add(-2*time.Hour))
	writeFileForPrune(t, filepath.Join(mto, "recurrence.json"), []byte("distinct recurrence bytes\n"), now.Add(-3*time.Hour))
	legacyBefore := snapshotPruneTree(t, legacy)
	mtoBefore := snapshotPruneTree(t, mto)

	var output bytes.Buffer
	result, err := PruneAgents(PruneAgentsOptions{
		RepoRoot: repo,
		Execute:  true,
		Stdout:   &output,
		Now:      func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("PruneAgents execute: %v", err)
	}
	if result.Files != 2 {
		t.Fatalf("execute deletion count = %d, want 2", result.Files)
	}
	if got := countDirectFiles(t, canonical); got != 10 {
		t.Fatalf("canonical handoff count = %d, want 10", got)
	}
	assertPruneSnapshot(t, legacy, legacyBefore)
	assertPruneSnapshot(t, mto, mtoBefore)
	if !strings.Contains(output.String(), "Files deleted: 2") {
		t.Fatalf("execute summary missing deletion count:\n%s", output.String())
	}
}

func TestPruneAgentsIntermediateAOSwapFailsClosedWithDescriptorRootedDelete(t *testing.T) {
	repo := t.TempDir()
	outside := t.TempDir()
	now := time.Date(2026, 8, 16, 18, 0, 0, 0, time.UTC)
	canonical := filepath.Join(repo, ".agents", "ao", "handoff")
	externalHandoff := filepath.Join(outside, "ao", "handoff")
	writePruneCandidates(t, canonical, "canonical", 12, now.Add(-time.Hour))
	// Mirror the selected basenames outside so a path-based delete would remove
	// a real external artifact rather than harmlessly targeting a missing name.
	writePruneCandidates(t, externalHandoff, "canonical", 12, now.Add(-time.Hour))
	writeFileForPrune(t, filepath.Join(externalHandoff, "sentinel"), []byte("outside sentinel bytes\n"), now)
	externalBefore := snapshotPruneTree(t, outside)

	hookCalls := 0
	pruneAgentsBeforeDeleteTestHook = func(relativePath string) {
		if hookCalls != 0 || !strings.HasPrefix(relativePath, filepath.Join(".agents", "ao", "handoff")+string(filepath.Separator)) {
			return
		}
		hookCalls++
		if err := os.Rename(filepath.Join(repo, ".agents", "ao"), filepath.Join(repo, ".agents", "ao-original")); err != nil {
			t.Fatalf("rename canonical ao directory in race hook: %v", err)
		}
		if err := os.Symlink(filepath.Join(outside, "ao"), filepath.Join(repo, ".agents", "ao")); err != nil {
			t.Fatalf("plant external ao symlink in race hook: %v", err)
		}
	}
	t.Cleanup(func() { pruneAgentsBeforeDeleteTestHook = nil })

	_, err := PruneAgents(PruneAgentsOptions{
		RepoRoot: repo,
		Execute:  true,
		Stdout:   &bytes.Buffer{},
		Now:      func() time.Time { return now },
	})
	if err == nil {
		t.Fatal("PruneAgents succeeded after .agents/ao was swapped to an external symlink")
	}
	if hookCalls != 1 {
		t.Fatalf("race hook calls = %d, want 1", hookCalls)
	}
	if !strings.Contains(err.Error(), "refusing to prune") {
		t.Fatalf("race failure did not explain fail-closed refusal: %v", err)
	}
	assertPruneSnapshot(t, outside, externalBefore)
	// The delete was bound to the already-open original handoff directory. It
	// may remove the selected original artifact, but never the symlink target.
	if got := countDirectFiles(t, filepath.Join(repo, ".agents", "ao-original", "handoff")); got != 11 {
		t.Fatalf("descriptor-rooted original handoff count = %d, want 11", got)
	}
}

func TestPruneAgentsTopLevelAgentsSwapCannotRedirectOtherPolicies(t *testing.T) {
	repo := t.TempDir()
	outside := t.TempDir()
	now := time.Date(2026, 8, 16, 18, 0, 0, 0, time.UTC)
	writePruneCandidates(t, filepath.Join(repo, ".agents", "council"), "council", 31, now.Add(-time.Hour))
	// Mirror the selected basenames outside so the negative catches redirection,
	// not merely the command's non-zero status.
	writePruneCandidates(t, filepath.Join(outside, "council"), "council", 31, now.Add(-time.Hour))
	writeFileForPrune(t, filepath.Join(outside, "sentinel"), []byte("outside root sentinel\n"), now)
	externalBefore := snapshotPruneTree(t, outside)

	hookCalls := 0
	pruneAgentsBeforeDeleteTestHook = func(relativePath string) {
		if hookCalls != 0 || !strings.HasPrefix(relativePath, filepath.Join(".agents", "council")+string(filepath.Separator)) {
			return
		}
		hookCalls++
		if err := os.Rename(filepath.Join(repo, ".agents"), filepath.Join(repo, ".agents-original")); err != nil {
			t.Fatalf("rename .agents in race hook: %v", err)
		}
		if err := os.Symlink(outside, filepath.Join(repo, ".agents")); err != nil {
			t.Fatalf("plant external .agents symlink in race hook: %v", err)
		}
	}
	t.Cleanup(func() { pruneAgentsBeforeDeleteTestHook = nil })

	_, err := PruneAgents(PruneAgentsOptions{
		RepoRoot: repo,
		Execute:  true,
		Stdout:   &bytes.Buffer{},
		Now:      func() time.Time { return now },
	})
	if err == nil {
		t.Fatal("PruneAgents succeeded after .agents was swapped to an external symlink")
	}
	if hookCalls != 1 {
		t.Fatalf("race hook calls = %d, want 1", hookCalls)
	}
	assertPruneSnapshot(t, outside, externalBefore)
	if got := countDirectFiles(t, filepath.Join(repo, ".agents-original", "council")); got != 30 {
		t.Fatalf("descriptor-rooted original council count = %d, want 30", got)
	}
}

func writePruneCandidates(t *testing.T, dir, prefix string, count int, firstModTime time.Time) {
	t.Helper()
	for i := 0; i < count; i++ {
		name := fmt.Sprintf("%s-%02d.json", prefix, i)
		writeFileForPrune(t, filepath.Join(dir, name), []byte(fmt.Sprintf("%s bytes %02d\n", prefix, i)), firstModTime.Add(time.Duration(i)*time.Minute))
	}
}

func writeFileForPrune(t *testing.T, path string, data []byte, modTime time.Time) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, modTime, modTime); err != nil {
		t.Fatal(err)
	}
}

func countDirectFiles(t *testing.T, dir string) int {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, entry := range entries {
		if entry.Type().IsRegular() {
			count++
		}
	}
	return count
}

func snapshotPruneTree(t *testing.T, root string) map[string][]byte {
	t.Helper()
	snapshot := map[string][]byte{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.Type().IsRegular() {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		snapshot[relative] = data
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func assertPruneSnapshot(t *testing.T, root string, want map[string][]byte) {
	t.Helper()
	got := snapshotPruneTree(t, root)
	if len(got) != len(want) {
		t.Fatalf("snapshot file count = %d, want %d", len(got), len(want))
	}
	for name, wantData := range want {
		gotData, ok := got[name]
		if !ok {
			t.Fatalf("snapshot lost %s", name)
		}
		if !bytes.Equal(gotData, wantData) {
			t.Fatalf("snapshot bytes changed for %s: got %q want %q", name, gotData, wantData)
		}
	}
}
