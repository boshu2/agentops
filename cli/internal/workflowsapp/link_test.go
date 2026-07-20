// practices: [design-by-contract, code-complete]
package workflowsapp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// mkWorkflow creates dir/<name> as a regular workflow script file.
func mkWorkflow(t *testing.T, dir, name string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte("// "+name+"\n"), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

func TestLinkWorkflows_LinksAbsentAndResolves(t *testing.T) {
	src := t.TempDir()
	dest := filepath.Join(t.TempDir(), ".claude", "workflows") // absent — created lazily
	mkWorkflow(t, src, "a.js")
	mkWorkflow(t, src, "b.js")

	res, err := LinkWorkflows(src, dest, false)
	if err != nil {
		t.Fatalf("LinkWorkflows: %v", err)
	}
	if len(res.Linked) != 2 || res.Linked[0] != "a.js" || res.Linked[1] != "b.js" {
		t.Fatalf("Linked = %v, want [a.js b.js]", res.Linked)
	}
	for _, name := range []string{"a.js", "b.js"} {
		tgt := filepath.Join(dest, name)
		got, err := os.Readlink(tgt)
		if err != nil {
			t.Fatalf("expected a symlink at %s: %v", tgt, err)
		}
		want := filepath.Join(src, name)
		if got != want {
			t.Fatalf("symlink target = %q, want %q", got, want)
		}
		body, err := os.ReadFile(tgt)
		if err != nil || string(body) != "// "+name+"\n" {
			t.Fatalf("script not reachable through link: body=%q err=%v", body, err)
		}
	}
}

func TestLinkWorkflows_DryRunDoesNotWrite(t *testing.T) {
	src := t.TempDir()
	dest := filepath.Join(t.TempDir(), ".claude", "workflows")
	mkWorkflow(t, src, "crank.js")

	res, err := LinkWorkflows(src, dest, true)
	if err != nil {
		t.Fatalf("dry run: %v", err)
	}
	if len(res.Linked) != 1 || res.Linked[0] != "crank.js" {
		t.Fatalf("dry-run Linked = %v, want [crank.js]", res.Linked)
	}
	if !res.DryRun {
		t.Fatal("res.DryRun = false, want true")
	}
	// Dry-run writes NOTHING — not even the destination directory.
	if _, err := os.Lstat(dest); !os.IsNotExist(err) {
		t.Fatalf("dry-run created the dest dir; Lstat err = %v, want IsNotExist", err)
	}
}

func TestLinkWorkflows_Idempotent(t *testing.T) {
	src := t.TempDir()
	dest := t.TempDir()
	mkWorkflow(t, src, "loop.js")

	if _, err := LinkWorkflows(src, dest, false); err != nil {
		t.Fatalf("first run: %v", err)
	}
	res, err := LinkWorkflows(src, dest, false)
	if err != nil {
		t.Fatalf("second run: %v", err)
	}
	if len(res.Linked) != 0 {
		t.Fatalf("second run Linked = %v, want []", res.Linked)
	}
	if len(res.Present) != 1 || res.Present[0] != "loop.js" {
		t.Fatalf("second run Present = %v, want [loop.js]", res.Present)
	}
}

func TestLinkWorkflows_RealFileIsConflictNotClobbered(t *testing.T) {
	src := t.TempDir()
	dest := t.TempDir()
	mkWorkflow(t, src, "ship.js")
	foreign := filepath.Join(dest, "ship.js")
	if err := os.WriteFile(foreign, []byte("operator-owned"), 0o644); err != nil {
		t.Fatalf("write foreign file: %v", err)
	}

	res, err := LinkWorkflows(src, dest, false)
	if err != nil {
		t.Fatalf("LinkWorkflows: %v", err)
	}
	if len(res.Conflicts) != 1 || res.Conflicts[0] != "ship.js" {
		t.Fatalf("Conflicts = %v, want [ship.js]", res.Conflicts)
	}
	if len(res.Linked) != 0 {
		t.Fatalf("Linked = %v, want [] (must not clobber)", res.Linked)
	}
	info, err := os.Lstat(foreign)
	if err != nil || info.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("real file was replaced; Lstat=%v mode=%v", err, info.Mode())
	}
	if body, err := os.ReadFile(foreign); err != nil || string(body) != "operator-owned" {
		t.Fatalf("real file content clobbered; body=%q err=%v", body, err)
	}
}

func TestLinkWorkflows_ForeignSymlinkIsConflict(t *testing.T) {
	src := t.TempDir()
	dest := t.TempDir()
	mkWorkflow(t, src, "audit.js")
	elsewhere := filepath.Join(t.TempDir(), "audit.js")
	if err := os.Symlink(elsewhere, filepath.Join(dest, "audit.js")); err != nil {
		t.Fatal(err)
	}

	res, err := LinkWorkflows(src, dest, false)
	if err != nil {
		t.Fatalf("LinkWorkflows: %v", err)
	}
	if len(res.Present) != 0 || len(res.Conflicts) != 1 || res.Conflicts[0] != "audit.js" {
		t.Fatalf("foreign symlink classified as %+v, want a conflict", res)
	}
	// The foreign symlink still points where the operator aimed it.
	if got, err := os.Readlink(filepath.Join(dest, "audit.js")); err != nil || got != elsewhere {
		t.Fatalf("foreign symlink was rewritten: got=%q err=%v", got, err)
	}
}

func TestLinkWorkflows_SkipsNonScriptEntries(t *testing.T) {
	src := t.TempDir()
	dest := t.TempDir()
	mkWorkflow(t, src, "real.js")
	if err := os.WriteFile(filepath.Join(src, "README.md"), []byte("docs"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(src, "helpers.js"), 0o755); err != nil {
		t.Fatal(err) // a DIRECTORY named *.js must still be skipped
	}

	res, err := LinkWorkflows(src, dest, false)
	if err != nil {
		t.Fatalf("LinkWorkflows: %v", err)
	}
	if len(res.Linked) != 1 || res.Linked[0] != "real.js" {
		t.Fatalf("Linked = %v, want [real.js] only", res.Linked)
	}
	for _, skipped := range []string{"README.md", "helpers.js"} {
		if _, err := os.Lstat(filepath.Join(dest, skipped)); !os.IsNotExist(err) {
			t.Fatalf("non-script entry %s was linked; err = %v, want IsNotExist", skipped, err)
		}
	}
}

func TestLinkWorkflows_EmptySrcFailsClosed(t *testing.T) {
	dest := t.TempDir()
	res, err := LinkWorkflows("", dest, false)
	if err == nil {
		t.Fatalf("empty srcDir must fail closed with an error, got nil (res=%+v)", res)
	}
	if len(res.Linked) != 0 {
		t.Fatalf("empty srcDir must link nothing, got Linked=%v", res.Linked)
	}
	if _, werr := LinkWorkflows("   ", dest, false); werr == nil {
		t.Fatal("whitespace-only srcDir must fail closed with an error, got nil")
	}
}

// RenderLinkResult is the human-facing summary: dry-run must never claim links
// were made, and the conflict line must state the non-clobber guarantee.
func TestRenderLinkResult_DryRunAndConflictLines(t *testing.T) {
	var buf strings.Builder
	RenderLinkResult(&buf, LinkResult{
		Dest: "/d", DryRun: true,
		Linked: []string{"a.js"}, Conflicts: []string{"c.js"},
	})
	out := buf.String()
	if !strings.Contains(out, "missing (dry-run, not linked): 1") {
		t.Errorf("dry-run rendering must label links as not created, got:\n%s", out)
	}
	if !strings.Contains(out, "? a.js") {
		t.Errorf("dry-run link mark must be '?', got:\n%s", out)
	}
	if !strings.Contains(out, "! c.js (real file or foreign symlink — not clobbered; resolve ownership explicitly)") {
		t.Errorf("conflict line must state the non-clobber guarantee, got:\n%s", out)
	}
	if !strings.Contains(out, "Claude-only") {
		t.Errorf("summary must name the Claude-only adapter boundary, got:\n%s", out)
	}
}
