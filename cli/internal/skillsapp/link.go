// practices: [design-by-contract, code-complete]
package skillsapp

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// LinkResult summarizes a `skills link` sync: which repo skills received a
// fresh live-tier symlink, which were already present, and which names collide
// with a real directory (a foreign corpus such as jsm) that we refuse to touch.
type LinkResult struct {
	Dest      string   `json:"dest"`
	DryRun    bool     `json:"dry_run"`
	Linked    []string `json:"linked"`
	Present   []string `json:"present"`
	Conflicts []string `json:"conflicts"`
	// Err is this destination's error, if any. A per-dest error does NOT abort
	// the fan-out — every other installed runtime is still linked and reported.
	Err string `json:"error,omitempty"`
}

// linkMissingSkills scans srcDir for skill directories (a subdir holding a
// SKILL.md) and ensures each has a symlink at destDir/<name> pointing at the
// absolute source. It is idempotent and non-destructive: an existing symlink is
// left as Present, and a real directory/file already owning a name (a foreign
// corpus) is reported as a Conflict and never clobbered. When dryRun is true no
// symlink is created, but the would-be links are still reported under Linked.
//
// Repairing an existing wrong or broken symlink is deliberately out of scope;
// it is reported as a conflict so an operator can resolve ownership explicitly.
func linkMissingSkills(srcDir, destDir string, dryRun bool) (LinkResult, error) {
	res := LinkResult{Dest: destDir, DryRun: dryRun}

	// Fail-closed on an unresolved source: an empty srcDir would let
	// filepath.Abs("") resolve to the CURRENT directory and silently scan/link
	// whatever happens to sit there. Refuse rather than guess (cross-family
	// refuter age-u031, codex-fresh-review).
	if strings.TrimSpace(srcDir) == "" {
		return res, fmt.Errorf("skills source dir is empty — cannot resolve the repo skills/ tree (run from inside the agentops repo)")
	}
	absSrc, err := filepath.Abs(srcDir)
	if err != nil {
		return res, fmt.Errorf("resolve skills dir %s: %w", srcDir, err)
	}
	entries, err := os.ReadDir(absSrc)
	if err != nil {
		return res, fmt.Errorf("read skills dir %s: %w", absSrc, err)
	}

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		src := filepath.Join(absSrc, name)
		if _, statErr := os.Stat(filepath.Join(src, "SKILL.md")); statErr != nil {
			continue // not a skill dir — no SKILL.md
		}

		tgt := filepath.Join(destDir, name)
		info, lerr := os.Lstat(tgt)
		switch {
		case lerr == nil && info.Mode()&os.ModeSymlink != 0:
			dst, readErr := os.Readlink(tgt)
			if readErr != nil {
				return res, fmt.Errorf("read link %s: %w", tgt, readErr)
			}
			if !filepath.IsAbs(dst) {
				dst = filepath.Join(destDir, dst)
			}
			if filepath.Clean(dst) == filepath.Clean(src) {
				res.Present = append(res.Present, name)
			} else {
				// A wrong or broken symlink is not healthy merely because it is a
				// symlink. Report it without replacing an operator-owned path.
				res.Conflicts = append(res.Conflicts, name)
			}
		case lerr == nil:
			res.Conflicts = append(res.Conflicts, name) // real dir/file — foreign corpus
		case os.IsNotExist(lerr):
			res.Linked = append(res.Linked, name) // the missing link
			if !dryRun {
				if err := os.MkdirAll(destDir, 0o755); err != nil {
					return res, fmt.Errorf("create dest dir %s: %w", destDir, err)
				}
				if err := os.Symlink(src, tgt); err != nil {
					return res, fmt.Errorf("link %s -> %s: %w", tgt, src, err)
				}
			}
		default:
			return res, fmt.Errorf("stat %s: %w", tgt, lerr)
		}
	}

	sort.Strings(res.Linked)
	sort.Strings(res.Present)
	sort.Strings(res.Conflicts)
	return res, nil
}

// LinkAllDests links the repo skills into every destination, RESILIENTLY: a
// per-dest error is captured on that dest's result and the fan-out continues to
// the remaining runtimes rather than aborting (which would leave earlier dests
// mutated and later ones silently skipped). Returns the per-dest results and
// whether any dest errored.
func LinkAllDests(srcDir string, dests []string, dryRun bool) ([]LinkResult, bool) {
	results := make([]LinkResult, 0, len(dests))
	anyErr := false
	for _, dest := range dests {
		res, err := linkMissingSkills(srcDir, dest, dryRun) // nosemgrep -- res is a value struct (never nil); setting res.Err on error cannot nil-deref.
		if err != nil {
			res.Err = err.Error()
			anyErr = true
		}
		results = append(results, res)
	}
	return results, anyErr
}

// RenderLinkResult prints one destination's link summary.
func RenderLinkResult(out io.Writer, res LinkResult) {
	fmt.Fprintf(out, "Skills link → %s\n", res.Dest)
	if res.Err != "" {
		fmt.Fprintf(out, "  ERROR: %s (other runtimes still attempted)\n", res.Err)
		return
	}
	if res.DryRun {
		fmt.Fprintf(out, "  missing (dry-run, not linked): %d\n", len(res.Linked))
	} else {
		fmt.Fprintf(out, "  linked:    %d\n", len(res.Linked))
	}
	fmt.Fprintf(out, "  present:   %d\n", len(res.Present))
	fmt.Fprintf(out, "  conflicts: %d\n", len(res.Conflicts))
	for _, n := range res.Linked {
		mark := "+"
		if res.DryRun {
			mark = "?"
		}
		fmt.Fprintf(out, "  %s %s\n", mark, n)
	}
	for _, n := range res.Conflicts {
		fmt.Fprintf(out, "  ! %s (real dir — foreign corpus, not clobbered)\n", n)
	}
	if len(res.Linked) == 0 && len(res.Conflicts) == 0 {
		fmt.Fprintln(out, "  all repo skills already live-linked.")
	}
}
