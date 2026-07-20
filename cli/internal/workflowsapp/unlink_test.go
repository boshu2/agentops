// practices: [design-by-contract, code-complete]
package workflowsapp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUnlinkWorkflows_RemovesOwnedLeavesForeign(t *testing.T) {
	src := t.TempDir()
	dest := t.TempDir()
	mkWorkflow(t, src, "a.js")
	if _, err := LinkWorkflows(src, dest, false); err != nil {
		t.Fatalf("link setup: %v", err)
	}
	// A foreign symlink pointing outside the checkout, and a real file.
	elsewhere := filepath.Join(t.TempDir(), "other.js")
	if err := os.Symlink(elsewhere, filepath.Join(dest, "foreign.js")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dest, "mine.js"), []byte("operator"), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := UnlinkWorkflows(src, dest, false)
	if err != nil {
		t.Fatalf("UnlinkWorkflows: %v", err)
	}
	if len(res.Removed) != 1 || res.Removed[0] != "a.js" {
		t.Fatalf("Removed = %v, want [a.js]", res.Removed)
	}
	if len(res.Foreign) != 2 || res.Foreign[0] != "foreign.js" || res.Foreign[1] != "mine.js" {
		t.Fatalf("Foreign = %v, want [foreign.js mine.js]", res.Foreign)
	}
	if _, err := os.Lstat(filepath.Join(dest, "a.js")); !os.IsNotExist(err) {
		t.Fatalf("owned link not removed; err = %v, want IsNotExist", err)
	}
	if got, err := os.Readlink(filepath.Join(dest, "foreign.js")); err != nil || got != elsewhere {
		t.Fatalf("foreign symlink touched: got=%q err=%v", got, err)
	}
	if body, err := os.ReadFile(filepath.Join(dest, "mine.js")); err != nil || string(body) != "operator" {
		t.Fatalf("real file touched: body=%q err=%v", body, err)
	}
}

func TestUnlinkWorkflows_DryRunPreviewsWithoutRemoving(t *testing.T) {
	src := t.TempDir()
	dest := t.TempDir()
	mkWorkflow(t, src, "b.js")
	if _, err := LinkWorkflows(src, dest, false); err != nil {
		t.Fatalf("link setup: %v", err)
	}

	res, err := UnlinkWorkflows(src, dest, true)
	if err != nil {
		t.Fatalf("dry run: %v", err)
	}
	if !res.DryRun {
		t.Fatal("res.DryRun = false, want true")
	}
	if len(res.Removed) != 1 || res.Removed[0] != "b.js" {
		t.Fatalf("dry-run Removed = %v, want [b.js]", res.Removed)
	}
	if _, err := os.Lstat(filepath.Join(dest, "b.js")); err != nil {
		t.Fatalf("dry-run actually removed the link: %v", err)
	}
}

func TestUnlinkWorkflows_MissingDestIsCleanNoop(t *testing.T) {
	src := t.TempDir()
	mkWorkflow(t, src, "c.js")
	dest := filepath.Join(t.TempDir(), "never-created")

	res, err := UnlinkWorkflows(src, dest, false)
	if err != nil {
		t.Fatalf("missing dest must be a clean no-op, got: %v", err)
	}
	if len(res.Removed) != 0 || len(res.Foreign) != 0 {
		t.Fatalf("missing dest must report nothing, got %+v", res)
	}
}

// A stale link whose target was removed from the checkout is still OURS —
// unlink must clean it up, not leave it as foreign.
func TestUnlinkWorkflows_StaleOwnedLinkStillRemoved(t *testing.T) {
	src := t.TempDir()
	dest := t.TempDir()
	mkWorkflow(t, src, "gone.js")
	if _, err := LinkWorkflows(src, dest, false); err != nil {
		t.Fatalf("link setup: %v", err)
	}
	if err := os.Remove(filepath.Join(src, "gone.js")); err != nil {
		t.Fatal(err)
	}

	res, err := UnlinkWorkflows(src, dest, false)
	if err != nil {
		t.Fatalf("UnlinkWorkflows: %v", err)
	}
	if len(res.Removed) != 1 || res.Removed[0] != "gone.js" {
		t.Fatalf("Removed = %v, want [gone.js] (stale link is still ours)", res.Removed)
	}
	if _, err := os.Lstat(filepath.Join(dest, "gone.js")); !os.IsNotExist(err) {
		t.Fatalf("stale owned link not removed; err = %v", err)
	}
}

func TestUnlinkWorkflows_EmptySrcFailsClosed(t *testing.T) {
	dest := t.TempDir()
	if err := os.Symlink(filepath.Join(t.TempDir(), "x.js"), filepath.Join(dest, "x.js")); err != nil {
		t.Fatal(err)
	}
	res, err := UnlinkWorkflows("", dest, false)
	if err == nil {
		t.Fatalf("empty srcDir must fail closed with an error, got nil (res=%+v)", res)
	}
	if len(res.Removed) != 0 {
		t.Fatalf("empty srcDir must remove nothing, got Removed=%v", res.Removed)
	}
	if _, err := os.Lstat(filepath.Join(dest, "x.js")); err != nil {
		t.Fatalf("fail-closed path still removed a link: %v", err)
	}
}

func TestRenderUnlinkResult_DryRunAndForeignLines(t *testing.T) {
	var buf strings.Builder
	RenderUnlinkResult(&buf, UnlinkResult{
		Dest: "/d", DryRun: true,
		Removed: []string{"a.js"}, Foreign: []string{"f.js"},
	})
	out := buf.String()
	if !strings.Contains(out, "would remove (dry-run): 1") {
		t.Errorf("dry-run rendering must label removals as previews, got:\n%s", out)
	}
	if !strings.Contains(out, "? a.js") {
		t.Errorf("dry-run removal mark must be '?', got:\n%s", out)
	}
	if !strings.Contains(out, ". f.js (not owned by this checkout — kept)") {
		t.Errorf("foreign line must state the kept guarantee, got:\n%s", out)
	}
}
