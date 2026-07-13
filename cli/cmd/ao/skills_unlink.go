// practices: [design-by-contract, code-complete]
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

var (
	skillsUnlinkDest string
	skillsUnlinkJSON bool
)

// skillUnlinkResult summarizes a `skills unlink` sweep of one destination: which
// AgentOps-owned live-tier symlinks were removed, and which entries were left
// untouched because they are NOT ours — a foreign symlink pointing outside the
// repo, or a real directory/file (a foreign corpus such as jsm). It is the exact
// inverse of skillLinkResult.
type skillUnlinkResult struct {
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
func unlinkOwnedSkills(srcDir, destDir string, dryRun bool) (skillUnlinkResult, error) {
	res := skillUnlinkResult{Dest: destDir, DryRun: dryRun}

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

var skillsUnlinkCmd = &cobra.Command{
	Use:   "unlink",
	Short: "Remove the repo-skill symlinks that `skills link` minted (Claude, Codex, AGY, Cursor, Pi)",
	Long: `The clean uninstall inverse of ` + "`ao skills link`" + `. Scan each runtime's
live tier and remove EXACTLY the symlinks that link minted — those whose target
resolves into THIS repo's skills/ tree. By DEFAULT it sweeps EVERY agent runtime
you have installed — ~/.claude/skills, ~/.codex/skills, ~/.gemini/skills
(AGY/Gemini), ~/.cursor/skills, and ~/.pi/skills — detected by the runtime's
config dir existing under $HOME; --dest overrides to a single dir. Idempotent and
non-destructive: a foreign symlink pointing outside the repo and a name owned by
a real directory (a foreign corpus such as jsm) are both reported as foreign and
never removed. A stale link to a skill since removed from the repo is still
cleaned up.

This is the documented rollback for the clone-linked "track main" install path:
after you stop following a repo clone, this leaves your runtimes with only the
skills they had before. It removes only symlinks, never your own directories or
another corpus.

Must be run from inside the agentops repo (guarded) — it needs the repo skills/
path to know which links are its own.

  ao skills unlink                        # remove owned links from every installed runtime
  ao skills unlink --dry-run              # show what would be removed without removing
  ao skills unlink --dest ~/.codex/skills # sweep ONE specific dir only`,
	Args: cobra.NoArgs,
	RunE: runSkillsUnlink,
}

func init() {
	skillsCmd.AddCommand(skillsUnlinkCmd)
	skillsUnlinkCmd.Flags().StringVar(&skillsUnlinkDest, "dest", "", "Sweep this single dir instead of the auto-detected runtimes (default: every installed runtime — ~/.claude, ~/.codex, ~/.gemini, ~/.cursor, ~/.pi)")
	skillsUnlinkCmd.Flags().BoolVar(&skillsUnlinkJSON, "json", false, "Emit machine-readable JSON")
}

func runSkillsUnlink(cmd *cobra.Command, args []string) error {
	skillsDir, err := resolveRepoSkillsDir()
	if err != nil {
		cmd.SilenceUsage = true
		return err
	}

	dests, err := resolveTargetDests(skillsUnlinkDest)
	if err != nil {
		cmd.SilenceUsage = true
		return err
	}

	results, anyErr := unlinkAllDests(skillsDir, dests, GetDryRun())

	if skillsUnlinkJSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		if eerr := enc.Encode(results); eerr != nil {
			return eerr
		}
	} else {
		out := cmd.OutOrStdout()
		for _, res := range results {
			renderUnlinkResult(out, res)
		}
	}

	// A per-dest failure is reported per-dest above but must still surface as a
	// non-zero exit — after every runtime was attempted, never before.
	if anyErr {
		cmd.SilenceUsage = true
		return fmt.Errorf("one or more runtime skill dirs could not be swept (see per-runtime errors)")
	}
	return nil
}

// unlinkAllDests unlinks owned skills from every destination, RESILIENTLY: a
// per-dest error is captured on that dest's result and the sweep continues to
// the remaining runtimes rather than aborting (which would leave earlier dests
// mutated and later ones silently skipped). Returns the per-dest results and
// whether any dest errored.
func unlinkAllDests(srcDir string, dests []string, dryRun bool) ([]skillUnlinkResult, bool) {
	results := make([]skillUnlinkResult, 0, len(dests))
	anyErr := false
	for _, dest := range dests {
		res, err := unlinkOwnedSkills(srcDir, dest, dryRun)
		if err != nil {
			res.Err = err.Error()
			anyErr = true
		}
		results = append(results, res)
	}
	return results, anyErr
}

// renderUnlinkResult prints one destination's unlink summary.
func renderUnlinkResult(out io.Writer, res skillUnlinkResult) {
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
