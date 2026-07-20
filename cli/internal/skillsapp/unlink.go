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

// UnlinkResult summarizes a `skills unlink` sweep of one destination: which
// AgentOps-owned live-tier symlinks were removed, and which entries were left
// untouched because they are NOT ours — a foreign symlink pointing outside the
// repo, or a real directory/file (a foreign corpus such as jsm). It is the exact
// inverse of LinkResult.
type UnlinkResult struct {
	Dest    string   `json:"dest"`
	DryRun  bool     `json:"dry_run"`
	Removed []string `json:"removed"`
	Foreign []string `json:"foreign"`
	// Err is this destination's error, if any. A per-dest error does NOT abort
	// the fan-out — every other installed runtime is still swept and reported.
	Err string `json:"error,omitempty"`
}

// unlinkOwnedSkills removes EXACTLY the live-tier symlinks under destDir that
// `skills link` minted: symlinks whose target resolves INTO the absolute repo
// skills/ tree (srcDir). It is the exact inverse of linkMissingSkills — idempotent
// and non-destructive to everything else. A foreign symlink pointing outside the
// repo and a real directory/file (a foreign corpus such as jsm) are both reported
// as Foreign and never removed. A stale link to a skill since removed from the
// repo is still ours to clean up (the target need not exist). When dryRun is true
// nothing is removed but the would-be removals are still reported under Removed.
func unlinkOwnedSkills(srcDir, destDir string, dryRun bool) (UnlinkResult, error) {
	res := UnlinkResult{Dest: destDir, DryRun: dryRun}

	// Fail-closed on an unresolved source, exactly as linkMissingSkills does: an
	// empty srcDir would let filepath.Abs("") resolve to the CURRENT directory,
	// and any symlink pointing there would be wrongly claimed as ours and
	// removed. Refuse rather than guess (mirror of the age-u031 guard on link).
	if strings.TrimSpace(srcDir) == "" {
		return res, fmt.Errorf("skills source dir is empty — cannot resolve the repo skills/ tree to identify owned links (run from inside the agentops repo)")
	}
	absSrc, err := filepath.Abs(srcDir)
	if err != nil {
		return res, fmt.Errorf("resolve skills dir %s: %w", srcDir, err)
	}

	entries, err := os.ReadDir(destDir)
	if err != nil {
		if os.IsNotExist(err) {
			return res, nil // nothing installed here — a clean no-op, idempotent
		}
		return res, fmt.Errorf("read dest dir %s: %w", destDir, err)
	}

	for _, e := range entries {
		name := e.Name()
		tgt := filepath.Join(destDir, name)
		info, lerr := os.Lstat(tgt)
		if lerr != nil {
			return res, fmt.Errorf("lstat %s: %w", tgt, lerr)
		}
		if info.Mode()&os.ModeSymlink == 0 {
			res.Foreign = append(res.Foreign, name) // real dir/file — foreign corpus
			continue
		}
		if owned, _ := symlinkResolvesInto(tgt, destDir, absSrc); !owned {
			res.Foreign = append(res.Foreign, name) // symlink pointing outside the repo
			continue
		}
		res.Removed = append(res.Removed, name)
		if !dryRun {
			if rmErr := os.Remove(tgt); rmErr != nil {
				return res, fmt.Errorf("remove link %s: %w", tgt, rmErr)
			}
		}
	}

	sort.Strings(res.Removed)
	sort.Strings(res.Foreign)
	return res, nil
}

// symlinkResolvesInto reports whether the symlink at linkPath points at a target
// inside absRoot (the repo skills/ tree). A relative link target is resolved
// against destDir first. The target need NOT exist — a stale link to a skill
// since removed from the repo is still ours. Returns the resolved absolute target
// for reporting.
func symlinkResolvesInto(linkPath, destDir, absRoot string) (bool, string) {
	dst, err := os.Readlink(linkPath)
	if err != nil {
		return false, ""
	}
	if !filepath.IsAbs(dst) {
		dst = filepath.Join(destDir, dst)
	}
	dst = filepath.Clean(dst)
	rel, err := filepath.Rel(absRoot, dst)
	if err != nil {
		return false, dst
	}
	// Owned iff the target is absRoot itself or a descendant of it — i.e. the
	// relative path does not escape upward (never "" ".." or "../…").
	inRoot := rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
	return inRoot, dst
}

// UnlinkAllDests unlinks owned skills from every destination, RESILIENTLY: a
// per-dest error is captured on that dest's result and the sweep continues to
// the remaining runtimes rather than aborting (which would leave earlier dests
// mutated and later ones silently skipped). Returns the per-dest results and
// whether any dest errored.
func UnlinkAllDests(srcDir string, dests []string, dryRun bool) ([]UnlinkResult, bool) {
	results := make([]UnlinkResult, 0, len(dests))
	anyErr := false
	for _, dest := range dests {
		res, err := unlinkOwnedSkills(srcDir, dest, dryRun) // nosemgrep -- res is a value struct (never nil); setting res.Err on error cannot nil-deref.
		if err != nil {
			res.Err = err.Error()
			anyErr = true
		}
		results = append(results, res)
	}
	return results, anyErr
}

// RenderUnlinkResult prints one destination's unlink summary.
func RenderUnlinkResult(out io.Writer, res UnlinkResult) {
	fmt.Fprintf(out, "Skills unlink → %s\n", res.Dest)
	if res.Err != "" {
		fmt.Fprintf(out, "  ERROR: %s (other runtimes still attempted)\n", res.Err)
		return
	}
	if res.DryRun {
		fmt.Fprintf(out, "  would remove (dry-run): %d\n", len(res.Removed))
	} else {
		fmt.Fprintf(out, "  removed:        %d\n", len(res.Removed))
	}
	fmt.Fprintf(out, "  foreign (kept): %d\n", len(res.Foreign))
	for _, n := range res.Removed {
		mark := "-"
		if res.DryRun {
			mark = "?"
		}
		fmt.Fprintf(out, "  %s %s\n", mark, n)
	}
	for _, n := range res.Foreign {
		fmt.Fprintf(out, "  . %s (not AgentOps-owned — kept)\n", n)
	}
	if len(res.Removed) == 0 {
		fmt.Fprintln(out, "  no AgentOps-owned links found (nothing to remove).")
	}
}
