// practices: [wiki-knowledge-surface, design-patterns]
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// TestPatternsRepair_DryRunListsProposals seeds a temp .agents/patterns/ with
// the on-disk shape that motivated soc-sx99.7 (doubled and tripled hyphenated
// runs) and verifies the dry-run lists proposals without touching disk.
func TestPatternsRepair_DryRunListsProposals(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "2026-04-22-pend-X-X-X.md"),
		"---\nid: pend-X-X-X\ntype: decision\n---\n\nbody\n")
	mustWriteFile(t, filepath.Join(dir, "2026-04-22-pend-Y-Y.md"),
		"---\nid: pend-Y-Y\ntype: decision\n---\n\nbody\n")
	mustWriteFile(t, filepath.Join(dir, "2026-04-22-already-canonical.md"),
		"---\nid: already-canonical\n---\n\nbody\n")

	proposals, err := planPatternRenames(dir)
	if err != nil {
		t.Fatalf("planPatternRenames: %v", err)
	}
	if len(proposals) != 2 {
		t.Fatalf("proposals=%d, want 2 (canonical file should be untouched)", len(proposals))
	}

	var newNames []string
	for _, p := range proposals {
		newNames = append(newNames, filepath.Base(p.NewPath))
	}
	sort.Strings(newNames)
	want := []string{"2026-04-22-pend-X.md", "2026-04-22-pend-Y.md"}
	for i, n := range newNames {
		if n != want[i] {
			t.Errorf("newNames[%d]=%q, want %q", i, n, want[i])
		}
	}

	// Disk must be untouched after a plan-only call.
	for _, name := range []string{
		"2026-04-22-pend-X-X-X.md",
		"2026-04-22-pend-Y-Y.md",
		"2026-04-22-already-canonical.md",
	} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("dry-run modified disk: %s missing: %v", name, err)
		}
	}
}

// TestPatternsRepair_ApplyRenamesAndIsIdempotent verifies that --apply renames
// each malformed file to its canonical name, rewrites frontmatter id, and a
// second --apply pass is a no-op (zero proposals).
func TestPatternsRepair_ApplyRenamesAndIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	oldName := "2026-04-29-pend-Z-Z-Z.md"
	mustWriteFile(t, filepath.Join(dir, oldName),
		"---\nid: pend-Z-Z-Z\ntype: decision\n---\n\nbody\n")

	proposals, err := planPatternRenames(dir)
	if err != nil {
		t.Fatalf("planPatternRenames: %v", err)
	}
	if len(proposals) != 1 {
		t.Fatalf("first-pass proposals=%d, want 1", len(proposals))
	}
	applied, errs := applyPatternRenames(proposals)
	if len(errs) != 0 {
		t.Fatalf("applyPatternRenames errors: %v", errs)
	}
	if len(applied) != 1 {
		t.Fatalf("applied=%d, want 1", len(applied))
	}

	wantNewName := "2026-04-29-pend-Z.md"
	if base := filepath.Base(applied[0].NewPath); base != wantNewName {
		t.Errorf("renamed to %q, want %q", base, wantNewName)
	}

	// Old file must be gone, new file must exist with rewritten frontmatter.
	if _, err := os.Stat(filepath.Join(dir, oldName)); !os.IsNotExist(err) {
		t.Errorf("old file %q still exists after rename: %v", oldName, err)
	}
	data, err := os.ReadFile(filepath.Join(dir, wantNewName))
	if err != nil {
		t.Fatalf("read renamed file: %v", err)
	}
	got := string(data)
	if strings.Contains(got, "pend-Z-Z-Z") {
		t.Errorf("frontmatter still references old id: %s", got)
	}
	if !strings.Contains(got, "id: pend-Z") {
		t.Errorf("frontmatter id not rewritten to new stem: %s", got)
	}

	// Second pass must be a no-op (idempotent).
	proposals2, err := planPatternRenames(dir)
	if err != nil {
		t.Fatalf("second-pass planPatternRenames: %v", err)
	}
	if len(proposals2) != 0 {
		t.Fatalf("second-pass proposals=%d, want 0 (idempotent)", len(proposals2))
	}
}

// TestPatternsRepair_DryRunPrintsSampleOutput exercises the operator-facing
// stdout writer so we know it lists each proposed rename and the rationale.
func TestPatternsRepair_DryRunPrintsSampleOutput(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "2026-04-29-pend-A-A-A.md"),
		"---\nid: pend-A-A-A\n---\n\nbody\n")

	proposals, err := planPatternRenames(dir)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	var buf bytes.Buffer
	printPatternRepairDryRun(&buf, dir, proposals)
	out := buf.String()
	if !strings.Contains(out, "(dry-run)") {
		t.Errorf("dry-run banner missing: %s", out)
	}
	if !strings.Contains(out, "proposals: 1") {
		t.Errorf("proposal count missing: %s", out)
	}
	if !strings.Contains(out, "2026-04-29-pend-A-A-A.md -> 2026-04-29-pend-A.md") {
		t.Errorf("rename line missing: %s", out)
	}
	if !strings.Contains(out, "collapsed") {
		t.Errorf("reason missing: %s", out)
	}
}

// TestPatternsRepair_HandlesCollisionsByAppendingSuffix verifies that two
// malformed files which canonicalize to the same name don't clobber each
// other — the second gets a numeric suffix appended.
func TestPatternsRepair_HandlesCollisionsByAppendingSuffix(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "2026-04-29-pend-B-B-B.md"), "first\n")
	mustWriteFile(t, filepath.Join(dir, "2026-04-29-pend-B-B.md"), "second\n")

	proposals, err := planPatternRenames(dir)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if len(proposals) != 2 {
		t.Fatalf("proposals=%d, want 2", len(proposals))
	}

	var newNames []string
	for _, p := range proposals {
		newNames = append(newNames, filepath.Base(p.NewPath))
	}
	sort.Strings(newNames)

	// Both proposals canonicalize to "2026-04-29-pend-B.md"; one gets the
	// suffix to avoid clobbering. The exact ordering depends on filesystem
	// readdir order, but both names must be distinct.
	if newNames[0] == newNames[1] {
		t.Fatalf("collision not resolved: %v", newNames)
	}
	for _, n := range newNames {
		if !strings.HasPrefix(n, "2026-04-29-pend-B") {
			t.Errorf("unexpected canonical name %q", n)
		}
	}
}

// TestPatternsRepair_MissingDirIsNoop verifies the migration tolerates a
// missing patterns dir (fresh checkout, never harvested) and returns nil.
func TestPatternsRepair_MissingDirIsNoop(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "does-not-exist")
	proposals, err := planPatternRenames(dir)
	if err != nil {
		t.Fatalf("missing dir should not error: %v", err)
	}
	if len(proposals) != 0 {
		t.Fatalf("missing dir should yield 0 proposals, got %d", len(proposals))
	}
}

// TestCanonicalPatternFilename_Cases pins the segment-collapse behavior on a
// table of inputs so future edits don't silently regress the bug fix.
func TestCanonicalPatternFilename_Cases(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		// Already canonical — no proposal.
		{"2026-04-29-foo.md", ""},
		{"2026-04-29-pend-foo.md", ""},

		// Single segment doubled at the tail.
		{"2026-04-29-pend-X-X.md", "2026-04-29-pend-X.md"},

		// Single segment tripled.
		{"2026-04-29-pend-X-X-X.md", "2026-04-29-pend-X.md"},

		// Multi-segment run repeated.
		{"2026-04-29-pend-X-Y-X-Y.md", "2026-04-29-pend-X-Y.md"},

		// On-disk shape from .agents/patterns/.
		{
			"2026-04-29-pend-2026-04-19-6a97752-19-2026-04-19-6a97752-19-2026-04-19-6a97752-19-19b1f808.md",
			"2026-04-29-pend-2026-04-19-6a97752-19-19b1f808.md",
		},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			got, _ := canonicalPatternFilename(tc.in)
			if got != tc.want {
				t.Errorf("canonicalPatternFilename(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
