// practices: [design-by-contract, code-complete]
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

var (
	skillsLinkDest string
	skillsLinkJSON bool
)

// skillLinkResult summarizes a `skills link` sync: which repo skills received a
// fresh live-tier symlink, which were already present, and which names collide
// with a real directory (a foreign corpus such as jsm) that we refuse to touch.
type skillLinkResult struct {
	Dest      string   `json:"dest"`
	DryRun    bool     `json:"dry_run"`
	Linked    []string `json:"linked"`
	Present   []string `json:"present"`
	Conflicts []string `json:"conflicts"`
}

// linkMissingSkills scans srcDir for skill directories (a subdir holding a
// SKILL.md) and ensures each has a symlink at destDir/<name> pointing at the
// absolute source. It is idempotent and non-destructive: an existing symlink is
// left as Present, and a real directory/file already owning a name (a foreign
// corpus) is reported as a Conflict and never clobbered. When dryRun is true no
// symlink is created, but the would-be links are still reported under Linked.
//
// Repairing an existing wrong or broken symlink is deliberately out of scope
// (any symlink counts as Present); use dotfiles/bin/link-skill --relink for
// that copy-verify-replace path.
func linkMissingSkills(srcDir, destDir string, dryRun bool) (skillLinkResult, error) {
	res := skillLinkResult{Dest: destDir, DryRun: dryRun}

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
			res.Present = append(res.Present, name) // already live-linked
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

var skillsLinkCmd = &cobra.Command{
	Use:   "link",
	Short: "Symlink repo skills that are missing from ~/.claude/skills (live tier)",
	Long: `Scan skills/ and create a live-tier symlink for every skill dir that has
no entry yet in the destination (default ~/.claude/skills). Idempotent and
non-destructive: skills already linked are left alone, and a name owned by a
real directory (a foreign corpus such as jsm) is reported as a conflict and
never clobbered.

This is the focused "a new skill landed but Claude can't see it" fix: merging a
new skill dir to main puts files in the repo but mints no symlink, and
/reload-skills only re-reads links that already exist. Run this and the new
skill is live next session.

Track main (optional): this is how to follow the latest skills from a repo clone
instead of waiting for a plugin release. Clone the repo, run this once, then
'git pull && ao skills link' to keep up — the symlinks point at the repo, so
edits are live with no reinstall. Additive to the plugin install, never a
replacement; must be run from inside the agentops repo (guarded).

Repairing an existing wrong/broken link is out of scope — use
dotfiles/bin/link-skill --relink for that copy-verify-replace path.

  ao skills link                       # link missing into ~/.claude/skills
  ao skills link --dry-run             # show what's missing without linking
  git pull && ao skills link           # track main: pick up newly-landed skills
  ao skills link --dest ~/.codex/skills`,
	Args: cobra.NoArgs,
	RunE: runSkillsLink,
}

func init() {
	skillsCmd.AddCommand(skillsLinkCmd)
	skillsLinkCmd.Flags().StringVar(&skillsLinkDest, "dest", "", "Destination skills dir (default ~/.claude/skills)")
	skillsLinkCmd.Flags().BoolVar(&skillsLinkJSON, "json", false, "Emit machine-readable JSON")
}

// resolveRepoSkillsDir returns the ABSOLUTE agentops repo skills/ directory, or
// an error if the caller is not inside the repo. It relies on the resolver's
// real signal: resolveSkillsRoots returns absolute paths ONLY when it located a
// directory holding BOTH skills/ and skills-codex/ (the agentops structure)
// walking up from cwd; its fallback returns the RELATIVE literal "skills". A
// mere existence check is not enough — running from an unrelated directory that
// happens to contain a stray skills/ subdir would pass os.Stat and scan/link
// that tree. Requiring an absolute, pair-verified path fails closed instead
// (cross-family refuter age-u031, codex-fresh-review, two rounds).
func resolveRepoSkillsDir() (string, error) {
	skillsDir, codexDir := resolveSkillsRoots()
	if !filepath.IsAbs(skillsDir) || !isDir(skillsDir) || !isDir(codexDir) {
		return "", fmt.Errorf("could not locate the agentops repo skills/ tree (resolved %q) — run `ao skills link` from inside the agentops repo", skillsDir)
	}
	// Shape is not identity: a skills/+skills-codex/ pair could exist outside
	// agentops. Require distinctive agentops repo-root markers (siblings of
	// skills/) so the command never scans a look-alike tree into
	// ~/.claude/skills (cross-family refuter age-u031, codex-fresh-review, 3
	// rounds — this is the terminal identity check).
	root := filepath.Dir(skillsDir)
	for _, marker := range []string{"registry.json", "PRODUCT.md"} {
		if fi, err := os.Stat(filepath.Join(root, marker)); err != nil || fi.IsDir() {
			return "", fmt.Errorf("resolved %q is not the agentops repo root (missing %s) — run `ao skills link` from inside the agentops repo", root, marker)
		}
	}
	return skillsDir, nil
}

func runSkillsLink(cmd *cobra.Command, args []string) error {
	skillsDir, err := resolveRepoSkillsDir()
	if err != nil {
		cmd.SilenceUsage = true
		return err
	}

	dest := skillsLinkDest
	if dest == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			cmd.SilenceUsage = true
			return fmt.Errorf("resolve home dir for default --dest: %w", err)
		}
		dest = filepath.Join(home, ".claude", "skills")
	}

	res, err := linkMissingSkills(skillsDir, dest, GetDryRun())
	if err != nil {
		cmd.SilenceUsage = true
		return err
	}

	if skillsLinkJSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(res)
	}

	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Skills link → %s\n", res.Dest)
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
	return nil
}
