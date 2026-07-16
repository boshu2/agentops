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

var skillsLinkCmd = &cobra.Command{
	Use:   "link",
	Short: "Symlink repo skills into portable and installed runtime skill roots",
	Long: `Scan skills/ and create a live-tier symlink for every skill dir that has
no entry yet. By DEFAULT it links into EVERY agent runtime you have installed —
~/.agents/skills, ~/.claude/skills, ~/.codex/skills, ~/.gemini/skills,
~/.cursor/skills, and ~/.pi/skills — detected by the config root existing under $HOME;
--dest overrides to a single dir. Idempotent and non-destructive: skills already
linked to this repository are left alone. A wrong or broken symlink, or a name
owned by a real directory, is reported as a conflict and never clobbered.

This is the focused "a new skill landed but the agent can't see it" fix: merging
a new skill dir to main puts files in the repo but mints no symlink. Run this and
the new skill is live next session in every detected runtime.

Track main (optional): this is how to follow the latest skills from a repo clone
instead of waiting for a plugin release. Clone the repo, run this once, then
'git pull && ao skills link' to keep up — the symlinks point at the repo, so
edits are live with no reinstall or plugin cache. Run from inside the AgentOps
repository so the source identity check can fail closed.

The command audits existing links but does not replace conflicts automatically;
inspect and resolve the named operator-owned path explicitly.

  ao skills link                        # link missing into every installed runtime
  ao skills link --dry-run              # show what's missing without linking
  git pull && ao skills link            # track main: pick up newly-landed skills
  ao skills link --dest ~/.codex/skills # link into ONE specific dir only`,
	Args: cobra.NoArgs,
	RunE: runSkillsLink,
}

func init() {
	skillsCmd.AddCommand(skillsLinkCmd)
	skillsLinkCmd.Flags().StringVar(&skillsLinkDest, "dest", "", "Link into this single dir instead of the auto-detected roots (default: ~/.agents plus every installed runtime)")
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

// runtimeConfigDirs are the per-agent config dirs whose skills/ subdir is that
// runtime's live tier. AgentOps skills are identical across runtimes, so a
// default `ao skills link` links into EVERY runtime the user actually has
// installed — Claude, Codex (~/.codex/skills), AGY/Gemini (~/.gemini/skills),
// Cursor, and Pi (~/.pi/skills) — not just Claude. Detection is by the config
// dir existing under $HOME. Order is display order.
var runtimeConfigDirs = []string{".claude", ".codex", ".gemini", ".cursor", ".pi"}

// resolveTargetDests returns the skills dirs to link into. An explicit --dest
// wins as the single target. Otherwise it returns <home>/<rt>/skills for every
// runtime config dir that EXISTS under $HOME. The portable ~/.agents/skills root
// is always included, even in a fresh home with no runtime configuration yet.
func resolveTargetDests(explicitDest string) ([]string, error) {
	if strings.TrimSpace(explicitDest) != "" {
		return []string{explicitDest}, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("resolve home dir for default --dest: %w", err)
	}
	dests := []string{filepath.Join(home, ".agents", "skills")}
	for _, rt := range runtimeConfigDirs {
		if isDir(filepath.Join(home, rt)) {
			dests = append(dests, filepath.Join(home, rt, "skills"))
		}
	}
	return dests, nil
}

func runSkillsLink(cmd *cobra.Command, args []string) error {
	skillsDir, err := resolveRepoSkillsDir()
	if err != nil {
		cmd.SilenceUsage = true
		return err
	}

	dests, err := resolveTargetDests(skillsLinkDest)
	if err != nil {
		cmd.SilenceUsage = true
		return err
	}

	results, anyErr := linkAllDests(skillsDir, dests, GetDryRun())

	if skillsLinkJSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		if eerr := enc.Encode(results); eerr != nil {
			return eerr
		}
	} else {
		out := cmd.OutOrStdout()
		for _, res := range results {
			renderLinkResult(out, res)
		}
	}

	// A per-dest failure is reported per-dest above but must still surface as a
	// non-zero exit — after every runtime was attempted, never before.
	if anyErr {
		cmd.SilenceUsage = true
		return fmt.Errorf("one or more runtime skill dirs could not be linked (see per-runtime errors)")
	}
	return nil
}

// linkAllDests links the repo skills into every destination, RESILIENTLY: a
// per-dest error is captured on that dest's result and the fan-out continues to
// the remaining runtimes rather than aborting (which would leave earlier dests
// mutated and later ones silently skipped). Returns the per-dest results and
// whether any dest errored.
func linkAllDests(srcDir string, dests []string, dryRun bool) ([]skillLinkResult, bool) {
	results := make([]skillLinkResult, 0, len(dests))
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

// renderLinkResult prints one destination's link summary.
func renderLinkResult(out io.Writer, res skillLinkResult) {
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
