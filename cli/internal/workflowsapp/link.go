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

// LinkResult summarizes a `workflows link` sync into the single Claude target:
// which workflow scripts received a fresh symlink, which were already present,
// and which names collide with a real file or a foreign symlink that we refuse
// to touch. It mirrors skillsapp.LinkResult minus the multi-runtime fan-out —
// workflows are a Claude-only adapter, so there is exactly one destination.
type LinkResult struct {
	Source    string   `json:"source"`
	Dest      string   `json:"dest"`
	DryRun    bool     `json:"dry_run"`
	Linked    []string `json:"linked"`
	Present   []string `json:"present"`
	Conflicts []string `json:"conflicts"`
}

// LinkWorkflows scans srcDir for workflow scripts (regular *.js files) and
// ensures each has a symlink at destDir/<name> pointing at the absolute
// source. It is idempotent and non-destructive: an existing correct symlink is
// left as Present, and a name already owned by a REAL file or by a symlink
// pointing elsewhere is reported as a Conflict and never clobbered —
// resolving ownership of an operator-owned path is operator judgment, not
// ours. When dryRun is true nothing is written (destDir is not even created),
// but the would-be links are still reported under Linked.
func LinkWorkflows(srcDir, destDir string, dryRun bool) (LinkResult, error) {
	res := LinkResult{Dest: destDir, DryRun: dryRun}

	// Fail-closed on an unresolved source: an empty srcDir would let
	// filepath.Abs("") resolve to the CURRENT directory and silently scan/link
	// whatever happens to sit there. Refuse rather than guess (mirror of the
	// age-u031 guard on skills link).
	if strings.TrimSpace(srcDir) == "" {
		return res, fmt.Errorf("workflows source dir is empty — cannot resolve the checkout workflows/ tree (run from inside the agentops repo)")
	}
	absSrc, err := filepath.Abs(srcDir)
	if err != nil {
		return res, fmt.Errorf("resolve workflows dir %s: %w", srcDir, err)
	}
	res.Source = absSrc
	entries, err := os.ReadDir(absSrc)
	if err != nil {
		return res, fmt.Errorf("read workflows dir %s: %w", absSrc, err)
	}

	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".js") {
			continue // only workflow scripts are linked; README and subdirs stay home
		}
		src := filepath.Join(absSrc, name)

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
				// A symlink pointing elsewhere is not healthy merely because it
				// is a symlink. Report it without replacing an operator-owned path.
				res.Conflicts = append(res.Conflicts, name)
			}
		case lerr == nil:
			res.Conflicts = append(res.Conflicts, name) // real file — operator-owned
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

// RenderLinkResult prints the human link summary for the single Claude target.
func RenderLinkResult(out io.Writer, res LinkResult) {
	fmt.Fprintf(out, "Workflows link (Claude-only) → %s\n", res.Dest)
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
		fmt.Fprintf(out, "  ! %s (real file or foreign symlink — not clobbered; resolve ownership explicitly)\n", n)
	}
	if len(res.Linked) == 0 && len(res.Conflicts) == 0 {
		fmt.Fprintln(out, "  all workflow scripts already linked.")
	}
}
