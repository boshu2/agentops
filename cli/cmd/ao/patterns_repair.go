// Package main: ao patterns repair-filenames is a one-shot migration that
// repairs the doubled/tripled filename prefixes left behind by the bug fixed
// in soc-sx99.7. The promotion path historically re-prepended <rig>-<id>
// segments on every close-loop pass so existing on-disk pattern files like
//
//	2026-04-29-pend-2026-04-19-6a97752-19-2026-04-19-6a97752-19-2026-04-19-6a97752-19-19b1f808.md
//
// accumulated 2-3 copies of the same identity segment. The repair walks
// .agents/patterns/*.md, detects consecutively-repeated hyphenated runs,
// collapses them to one canonical occurrence, and renames the file in place.
//
// Default mode is dry-run: proposed renames are printed and the command exits
// 0 without touching disk. --apply performs os.Rename for each proposal and
// rewrites the frontmatter `name:` / `id:` field if it references the old
// filename. The operation is idempotent: a second run on the same directory
// surfaces zero proposals.
// practices: [wiki-knowledge-surface, design-patterns]
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

var (
	patternsRepairApply bool
	patternsRepairDir   string
	patternsRepairQuiet bool
)

var patternsCmd = &cobra.Command{
	Use:   "patterns",
	Short: "Inspect and repair .agents/patterns/ artifacts",
	Long:  `Maintenance commands for the .agents/patterns/ directory.`,
}

var patternsRepairCmd = &cobra.Command{
	Use:   "repair-filenames",
	Short: "Repair doubled/tripled prefixes in .agents/patterns/ filenames",
	Long: `Walk .agents/patterns/*.md, detect filenames whose hyphenated segments
were duplicated by the legacy promotion bug (soc-sx99.7), and rename to the
canonical single-prefix form.

By default this is a dry-run: proposed renames are printed and nothing is
written. Pass --apply to perform os.Rename for each proposed rename and
rewrite the frontmatter id/name field if it references the old basename.

Running --apply twice in a row is a no-op on the second pass.

Examples:
  ao patterns repair-filenames                 # dry-run (default)
  ao patterns repair-filenames --apply         # perform renames
  ao patterns repair-filenames --dir /tmp/p    # use a custom patterns dir`,
	RunE: runPatternsRepair,
}

func init() {
	patternsCmd.GroupID = "knowledge"
	rootCmd.AddCommand(patternsCmd)

	patternsCmd.AddCommand(patternsRepairCmd)
	patternsRepairCmd.Flags().BoolVar(&patternsRepairApply, "apply", false,
		"Perform renames (default: dry-run, no disk writes)")
	patternsRepairCmd.Flags().StringVar(&patternsRepairDir, "dir", "",
		"Patterns directory to repair (default: <cwd>/.agents/patterns)")
	patternsRepairCmd.Flags().BoolVar(&patternsRepairQuiet, "quiet", false,
		"Suppress per-rename output")
}

// patternRenameProposal describes a single file rename the repair command
// would perform. The Reason field is included in dry-run output so operators
// can see which detector fired (doubled prefix, doubled core, etc.).
type patternRenameProposal struct {
	OldPath string
	NewPath string
	Reason  string
}

func runPatternsRepair(cmd *cobra.Command, _ []string) error {
	dir, err := resolvePatternsDir(patternsRepairDir)
	if err != nil {
		return fmt.Errorf("resolve patterns dir: %w", err)
	}

	proposals, err := planPatternRenames(dir)
	if err != nil {
		return fmt.Errorf("scan patterns dir %s: %w", dir, err)
	}

	out := cmd.OutOrStdout()
	if !patternsRepairApply {
		printPatternRepairDryRun(out, dir, proposals)
		return nil
	}

	applied, errs := applyPatternRenames(proposals)
	printPatternRepairApply(out, dir, applied, errs)
	if len(errs) > 0 {
		return fmt.Errorf("%d rename(s) failed", len(errs))
	}
	return nil
}

// resolvePatternsDir returns the directory the repair should operate on.
// Empty input falls back to <cwd>/.agents/patterns.
func resolvePatternsDir(override string) (string, error) {
	if override != "" {
		return override, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return filepath.Join(cwd, ".agents", "patterns"), nil
}

// planPatternRenames walks dir and returns one proposal per malformed file.
// Returns nil (not an error) when dir does not exist — the migration is a
// best-effort cleanup, and a missing patterns dir simply means there is
// nothing to repair.
func planPatternRenames(dir string) ([]patternRenameProposal, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	used := map[string]bool{}
	for _, e := range entries {
		used[e.Name()] = true
	}

	var proposals []patternRenameProposal
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		canonical, reason := canonicalPatternFilename(e.Name())
		if canonical == "" || canonical == e.Name() {
			continue
		}
		// On collision with another existing or already-proposed file, append
		// a short hash so we don't clobber distinct content.
		final := uniquePatternFilename(canonical, used)
		used[final] = true
		proposals = append(proposals, patternRenameProposal{
			OldPath: filepath.Join(dir, e.Name()),
			NewPath: filepath.Join(dir, final),
			Reason:  reason,
		})
	}
	sort.Slice(proposals, func(i, j int) bool {
		return proposals[i].OldPath < proposals[j].OldPath
	})
	return proposals, nil
}

// uniquePatternFilename returns name unmodified if unused; otherwise appends
// a short numeric suffix until an unused name is found. This avoids clobbering
// when two malformed files would collapse to the same canonical name.
func uniquePatternFilename(name string, used map[string]bool) string {
	if !used[name] {
		return name
	}
	ext := filepath.Ext(name)
	stem := strings.TrimSuffix(name, ext)
	for i := 2; i < 1000; i++ {
		cand := fmt.Sprintf("%s-%d%s", stem, i, ext)
		if !used[cand] {
			return cand
		}
	}
	return name
}

// canonicalPatternFilename returns the canonical filename and the reason a
// rename was proposed. Returns "" when the input is already canonical.
//
// The detector collapses any hyphen-separated segment-run that appears 2+
// times consecutively to a single occurrence. For example:
//
//	pend-X-X-X  -> pend-X
//	a-b-a-b-c   -> a-b-c
//	A-A-A-suffix-> A-suffix
//
// We split on "-" once, search for the longest non-empty contiguous repeat,
// collapse it, and re-run until the name is stable. This handles arbitrarily
// nested doublings without recursing into rig-vs-id ambiguity.
func canonicalPatternFilename(name string) (string, string) {
	ext := filepath.Ext(name)
	stem := strings.TrimSuffix(name, ext)
	collapsed, reason := collapseRepeatedSegments(stem)
	if collapsed == stem {
		return "", ""
	}
	return collapsed + ext, reason
}

// collapseRepeatedSegments reduces consecutively-repeated hyphen-segment runs
// to a single occurrence. It returns the collapsed string and a human-readable
// reason naming the longest collapsed run.
func collapseRepeatedSegments(stem string) (string, string) {
	parts := strings.Split(stem, "-")
	reason := ""
	for {
		idx, runLen := findLongestConsecutiveRepeat(parts)
		if idx < 0 || runLen <= 0 {
			break
		}
		// Keep one copy of the repeated run; drop the rest.
		repeats := contiguousRepeatCount(parts, idx, runLen)
		drop := runLen * (repeats - 1)
		repeated := strings.Join(parts[idx:idx+runLen], "-")
		if reason == "" {
			reason = fmt.Sprintf("collapsed %dx run %q", repeats, repeated)
		}
		parts = append(parts[:idx+runLen], parts[idx+runLen+drop:]...)
	}
	return strings.Join(parts, "-"), reason
}

// findLongestConsecutiveRepeat scans parts for the longest non-empty run of
// segments that appears at least twice consecutively. Returns the start index
// and the run length. (-1, 0) means no consecutive repeat was found.
//
// Preference goes to the longest run; ties break on earliest start. We cap
// the run length at len(parts)/2 because a run longer than half the slice
// cannot repeat consecutively.
func findLongestConsecutiveRepeat(parts []string) (int, int) {
	best := -1
	bestLen := 0
	maxRun := len(parts) / 2
	for runLen := maxRun; runLen >= 1; runLen-- {
		for i := 0; i+2*runLen <= len(parts); i++ {
			if segmentsEqual(parts[i:i+runLen], parts[i+runLen:i+2*runLen]) {
				if runLen > bestLen {
					best = i
					bestLen = runLen
				}
				break
			}
		}
		if bestLen > 0 {
			break
		}
	}
	return best, bestLen
}

// contiguousRepeatCount counts how many times the run [start, start+runLen)
// repeats contiguously starting at start. Always at least 2 when called from
// findLongestConsecutiveRepeat.
func contiguousRepeatCount(parts []string, start, runLen int) int {
	count := 1
	for j := start + runLen; j+runLen <= len(parts); j += runLen {
		if !segmentsEqual(parts[start:start+runLen], parts[j:j+runLen]) {
			break
		}
		count++
	}
	return count
}

// segmentsEqual reports whether a and b are non-empty and equal segment-wise.
func segmentsEqual(a, b []string) bool {
	if len(a) == 0 || len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// applyPatternRenames performs os.Rename for each proposal and rewrites the
// frontmatter id/name field if it references the old basename. Returns the
// list of successful proposals plus any errors. A failure on one proposal
// does not abort the rest — the migration is best-effort.
func applyPatternRenames(proposals []patternRenameProposal) ([]patternRenameProposal, []error) {
	var applied []patternRenameProposal
	var errs []error
	for _, p := range proposals {
		if err := os.Rename(p.OldPath, p.NewPath); err != nil {
			errs = append(errs, fmt.Errorf("rename %s -> %s: %w", p.OldPath, p.NewPath, err))
			continue
		}
		if err := rewritePatternFrontmatter(p.NewPath, oldStem(p.OldPath), oldStem(p.NewPath)); err != nil {
			errs = append(errs, fmt.Errorf("rewrite frontmatter %s: %w", p.NewPath, err))
			// Keep the rename: filename canonicalization is the primary goal.
		}
		applied = append(applied, p)
	}
	return applied, errs
}

// oldStem returns the base filename minus its extension.
func oldStem(p string) string {
	b := filepath.Base(p)
	return strings.TrimSuffix(b, filepath.Ext(b))
}

// frontmatterFieldRe matches the id: / name: line in YAML frontmatter so we
// can rewrite it after a rename. We match value-side substrings only — the
// rewrite is conservative and only touches lines where the value references
// the old basename verbatim.
var frontmatterFieldRe = regexp.MustCompile(`^(\s*(?:id|name)\s*:\s*)(.*)$`)

// rewritePatternFrontmatter rewrites id: / name: frontmatter lines whose
// value references the old stem so it now references the new stem. Bodies
// outside the frontmatter block are left untouched.
func rewritePatternFrontmatter(path, oldName, newName string) error {
	if oldName == newName {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	updated, changed := substituteFrontmatterName(string(data), oldName, newName)
	if !changed {
		return nil
	}
	return os.WriteFile(path, []byte(updated), 0o644)
}

// substituteFrontmatterName rewrites the id: / name: fields inside the first
// YAML frontmatter block. Two rewrite strategies are tried per line, in order:
//
//  1. Verbatim oldName -> newName replacement (when the field value embeds
//     the renamed file's full stem, e.g. `name: my-stem.md`).
//  2. Apply the same consecutive-segment collapse used on the filename to
//     the bare value (handles `id: pend-X-X-X` whose duplication is independent
//     of the on-disk filename's date prefix).
//
// Returns the new content and whether any change was made.
func substituteFrontmatterName(content, oldName, newName string) (string, bool) {
	if !strings.HasPrefix(strings.TrimLeft(content, " \t\n"), "---") {
		return content, false
	}
	scanner := bufio.NewScanner(strings.NewReader(content))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	var sb strings.Builder
	inFM := false
	seenStart := false
	changed := false
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		switch {
		case !seenStart && trimmed == "---":
			inFM = true
			seenStart = true
		case inFM && trimmed == "---":
			inFM = false
		case inFM:
			if m := frontmatterFieldRe.FindStringSubmatch(line); m != nil {
				newVal, didChange := canonicalizeFrontmatterValue(m[2], oldName, newName)
				if didChange {
					line = m[1] + newVal
					changed = true
				}
			}
		}
		sb.WriteString(line)
		sb.WriteByte('\n')
	}
	if err := scanner.Err(); err != nil {
		return content, false
	}
	out := sb.String()
	// Preserve final-newline shape: if the original ended without a newline,
	// drop the one bufio added.
	if !strings.HasSuffix(content, "\n") && strings.HasSuffix(out, "\n") {
		out = strings.TrimSuffix(out, "\n")
	}
	return out, changed
}

// canonicalizeFrontmatterValue applies the two-strategy rewrite described on
// substituteFrontmatterName. Returns the new value and a changed flag.
func canonicalizeFrontmatterValue(val, oldName, newName string) (string, bool) {
	if oldName != "" && newName != "" && strings.Contains(val, oldName) {
		return strings.Replace(val, oldName, newName, 1), true
	}
	collapsed, _ := collapseRepeatedSegments(strings.TrimSpace(val))
	if collapsed != "" && collapsed != strings.TrimSpace(val) {
		return collapsed, true
	}
	return val, false
}

func printPatternRepairDryRun(w io.Writer, dir string, proposals []patternRenameProposal) {
	if patternsRepairQuiet {
		fmt.Fprintf(w, "%d\n", len(proposals))
		return
	}
	fmt.Fprintf(w, "ao patterns repair-filenames (dry-run)\n")
	fmt.Fprintf(w, "  dir:       %s\n", dir)
	fmt.Fprintf(w, "  proposals: %d\n", len(proposals))
	if len(proposals) == 0 {
		fmt.Fprintln(w, "  (nothing to repair)")
		return
	}
	for _, p := range proposals {
		fmt.Fprintf(w, "  rename %s -> %s  (%s)\n",
			filepath.Base(p.OldPath), filepath.Base(p.NewPath), p.Reason)
	}
	fmt.Fprintln(w, "Re-run with --apply to perform these renames.")
}

func printPatternRepairApply(w io.Writer, dir string, applied []patternRenameProposal, errs []error) {
	if patternsRepairQuiet {
		fmt.Fprintf(w, "%d\n", len(applied))
		return
	}
	fmt.Fprintf(w, "ao patterns repair-filenames (apply)\n")
	fmt.Fprintf(w, "  dir:     %s\n", dir)
	fmt.Fprintf(w, "  renamed: %d\n", len(applied))
	for _, p := range applied {
		fmt.Fprintf(w, "  renamed %s -> %s\n",
			filepath.Base(p.OldPath), filepath.Base(p.NewPath))
	}
	if len(errs) > 0 {
		fmt.Fprintf(w, "  errors:  %d\n", len(errs))
		for _, err := range errs {
			fmt.Fprintf(w, "  ! %v\n", err)
		}
	}
}
