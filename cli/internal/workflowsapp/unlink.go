// practices: [design-by-contract, code-complete]
package workflowsapp

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// UnlinkResult summarizes a `workflows unlink` sweep of the single Claude
// target: which checkout-owned symlinks were removed, and which entries were
// left untouched because they are NOT ours — a foreign symlink pointing
// outside this checkout's workflows/ tree, or a real file. It is the exact
// inverse of LinkResult.
type UnlinkResult struct {
	Source  string   `json:"source"`
	Dest    string   `json:"dest"`
	DryRun  bool     `json:"dry_run"`
	Removed []string `json:"removed"`
	Foreign []string `json:"foreign"`
}

// UnlinkWorkflows removes EXACTLY the symlinks under destDir that
// `workflows link` minted: symlinks whose target resolves INTO the absolute
// checkout workflows/ tree (srcDir). It is the exact inverse of LinkWorkflows
// — idempotent and non-destructive to everything else. A foreign symlink
// pointing outside the checkout and a real file/directory are both reported
// as Foreign and never removed. A stale link to a workflow since removed from
// the checkout is still ours to clean up (the target need not exist). When
// dryRun is true nothing is removed but the would-be removals are still
// reported under Removed.
func UnlinkWorkflows(srcDir, destDir string, dryRun bool) (UnlinkResult, error) {
	res := UnlinkResult{Dest: destDir, DryRun: dryRun}

	// Fail-closed on an unresolved source, exactly as LinkWorkflows does: an
	// empty srcDir would let filepath.Abs("") resolve to the CURRENT directory,
	// and any symlink pointing there would be wrongly claimed as ours and
	// removed. Refuse rather than guess (mirror of the age-u031 guard).
	if strings.TrimSpace(srcDir) == "" {
		return res, fmt.Errorf("workflows source dir is empty — cannot resolve the checkout workflows/ tree to identify owned links (run from inside the agentops repo)")
	}
	absSrc, err := filepath.Abs(srcDir)
	if err != nil {
		return res, fmt.Errorf("resolve workflows dir %s: %w", srcDir, err)
	}
	res.Source = absSrc

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
			res.Foreign = append(res.Foreign, name) // real file/dir — operator-owned
			continue
		}
		if owned, _ := symlinkResolvesInto(tgt, destDir, absSrc); !owned {
			res.Foreign = append(res.Foreign, name) // symlink pointing outside this checkout
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

// symlinkResolvesInto reports whether the symlink at linkPath points at a
// target inside absRoot (the checkout workflows/ tree). A relative link
// target is resolved against destDir first. The target need NOT exist — a
// stale link to a workflow since removed from the checkout is still ours.
// Returns the resolved absolute target for reporting.
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
	// relative path does not escape upward (never ".." or "../…").
	inRoot := rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
	return inRoot, dst
}

// RenderUnlinkResult prints the human unlink summary for the single Claude target.
func RenderUnlinkResult(out io.Writer, res UnlinkResult) {
	fmt.Fprintf(out, "Workflows unlink (Claude-only) → %s\n", res.Dest)
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
		fmt.Fprintf(out, "  . %s (not owned by this checkout — kept)\n", n)
	}
	if len(res.Removed) == 0 {
		fmt.Fprintln(out, "  no checkout-owned links found (nothing to remove).")
	}
}
