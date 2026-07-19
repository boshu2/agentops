// Package skills owns Cobra presentation for the `ao skills` command family.
// The module builds its command tree and delegates every direct filesystem
// effect (skills-root resolution and the link/unlink sweeps) to
// internal/skillsapp, so this package performs no direct effect.
package skills

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/clicontract"
	"github.com/boshu2/agentops/cli/internal/skills"
	"github.com/boshu2/agentops/cli/internal/skillsapp"
	"github.com/boshu2/agentops/cli/internal/skillshealth"
	"github.com/boshu2/agentops/cli/internal/skillsresolve"
)

// HostOptions carries the ambient CLI seams the skills commands read. The
// global dry-run flag drives link/unlink; it is injected so the module stays
// free of direct host access.
type HostOptions struct {
	DryRun func() bool
}

// Module owns Cobra presentation for the skills command family.
type Module struct {
	host HostOptions

	// check
	checkJSON   bool
	checkStrict bool
	checkOnly   string

	// resolve
	resolveJSON   bool
	resolveStrict bool

	// find
	findJSON  bool
	findLimit int

	// list
	listJSON          bool
	listRole          string
	listProduces      string
	listConsumes      string
	listPractice      string
	listUserInvocable string // tri-state: "", "true", "false"

	// consumers / producers
	consumersJSON bool
	producersJSON bool

	// graph
	graphFormat string

	// link / unlink
	linkDest   string
	linkJSON   bool
	unlinkDest string
	unlinkJSON bool
}

// NewModule constructs the skills command module from its host seams.
func NewModule(host HostOptions) *Module {
	return &Module{host: host}
}

// Contract declares skills's real behavior for the family architecture gate.
// The skills family did not attach a capabilities contract before the
// carve-out, so the composition does not attach this one either; it exists to
// document the family's effect and profile shape.
func (*Module) Contract() clicontract.CommandContract {
	return clicontract.CommandContract{
		ID:       "ao.skills",
		Profiles: clicontract.ProfileDefault | clicontract.ProfileFlywheel | clicontract.ProfileLegacy | clicontract.ProfileCombined,
		Args:     clicontract.ArgsPolicy{Name: "arbitrary", Validate: cobra.ArbitraryArgs},
		Output:   clicontract.OutputText,
		Effects:  clicontract.EffectFilesystem,
		ExitClasses: map[int]clicontract.ExitClass{
			0: clicontract.ExitSuccess,
			1: clicontract.ExitFailure,
		},
	}
}

// Command builds the `ao skills` command tree.
func (m *Module) Command() *cobra.Command {
	root := &cobra.Command{
		Use:     "skills",
		Short:   "Inspect and validate the skills/ tree",
		GroupID: "knowledge",
		Long: `Tooling for the skills/ source-of-truth and its skills-codex/
parity sibling. Subcommands surface health (frontmatter completeness,
broken reference links, codex parity drift) without mutating either
tree.`,
	}
	root.AddCommand(m.checkCommand())
	root.AddCommand(m.resolveCommand())
	root.AddCommand(m.findCommand())
	root.AddCommand(m.listCommand())
	root.AddCommand(m.consumersCommand())
	root.AddCommand(m.producersCommand())
	root.AddCommand(m.graphCommand())
	root.AddCommand(m.linkCommand())
	root.AddCommand(m.unlinkCommand())
	return root
}

func (m *Module) checkCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "check",
		Short: "Audit skills/ frontmatter, references, and codex parity",
		Long: `Walk skills/ and skills-codex/, validating each skill's YAML
frontmatter (name + description present, name matches dir), checking
that every references/*.md is linked from SKILL.md (and vice versa),
and reporting parity drift against skills-codex/.

Exits 0 by default. With --strict, exits 1 if any finding (missing
frontmatter, broken reference, parity drift) is reported, suitable for
CI gating.`,
		RunE: m.runCheck,
	}
	cmd.Flags().BoolVar(&m.checkJSON, "json", false, "Emit machine-readable JSON")
	cmd.Flags().BoolVar(&m.checkStrict, "strict", false, "Exit non-zero on any finding (CI mode)")
	cmd.Flags().StringVar(&m.checkOnly, "skill", "", "Restrict the audit to a single skill name")
	return cmd
}

func (m *Module) runCheck(cmd *cobra.Command, _ []string) error {
	skillsDir, codexDir := skillsapp.ResolveSkillsRoots()
	opts := skillshealth.Options{
		SkillsDir: skillsDir,
		CodexDir:  codexDir,
		OnlySkill: m.checkOnly,
		Strict:    m.checkStrict,
	}
	report, err := skillshealth.Audit(opts)
	if err != nil {
		return err
	}

	out := cmd.OutOrStdout()
	if m.checkJSON {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		if err := enc.Encode(report); err != nil {
			return err
		}
	} else {
		fmt.Fprintf(out, "Skills audit (%s)\n", report.Generated)
		fmt.Fprintf(out, "================\n")
		fmt.Fprintf(out, "Skills audited: %d\n", len(report.Skills))
		fmt.Fprintf(out, "Errors:         %d\n", len(report.Errors))
		fmt.Fprintf(out, "Parity drift:   %d\n\n", len(report.ParityDrift))

		if len(report.Errors) > 0 {
			fmt.Fprintln(out, "Errors:")
			for _, e := range report.Errors {
				fmt.Fprintf(out, "  - %s\n", e)
			}
			fmt.Fprintln(out)
		}
		if len(report.ParityDrift) > 0 {
			fmt.Fprintln(out, "Codex parity drift:")
			for _, e := range report.ParityDrift {
				fmt.Fprintf(out, "  - %s\n", e)
			}
		}
		if len(report.Errors) == 0 && len(report.ParityDrift) == 0 {
			fmt.Fprintln(out, "All skills healthy.")
		}
	}

	if m.checkStrict && (len(report.Errors) > 0 || len(report.ParityDrift) > 0) {
		// Use SilenceUsage to avoid printing usage on this expected non-zero exit.
		cmd.SilenceUsage = true
		return fmt.Errorf("skills check failed: %d errors, %d parity-drift",
			len(report.Errors), len(report.ParityDrift))
	}
	return nil
}

func (m *Module) resolveCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "resolve",
		Short: "MECE audit: flag overlapping skills (merge candidates) and coverage gaps",
		Long: `Walk skills/ and resolve the corpus toward MECE:

  - Mutually Exclusive (ME): cluster skills by name-family stem and
    description-token Jaccard, surfacing overlapping or near-duplicate skills
    for caller review.
  - Collectively Exhaustive (CE): flag thin or description-less SKILL.md files
    as coverage-quality gaps.

Read-only; mutates nothing. Installation and symlink selection remain an
operator-side concern outside this report.

Exits 0 by default. With --strict, exits 1 when any ME overlap is reported,
suitable for a CI dedup gate.`,
		RunE: m.runResolve,
	}
	cmd.Flags().BoolVar(&m.resolveJSON, "json", false, "Emit machine-readable JSON")
	cmd.Flags().BoolVar(&m.resolveStrict, "strict", false, "Exit non-zero when ME overlaps are found (CI dedup gate)")
	return cmd
}

func (m *Module) runResolve(cmd *cobra.Command, _ []string) error {
	skillsDir, _ := skillsapp.ResolveSkillsRoots()
	report, err := skillsresolve.Resolve(skillsresolve.Options{SkillsDir: skillsDir})
	if err != nil {
		return err
	}

	out := cmd.OutOrStdout()
	if m.resolveJSON {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		if err := enc.Encode(report); err != nil {
			return err
		}
	} else {
		fmt.Fprintf(out, "Skills MECE resolve (%s)\n", report.Generated)
		fmt.Fprintf(out, "===================\n")
		fmt.Fprintf(out, "Skills:               %d\n", report.SkillsCount)
		fmt.Fprintf(out, "ME candidate overlaps: %d  (caller review)\n", len(report.Overlaps))
		fmt.Fprintf(out, "CE coverage flags:     %d  (thin/triggerless)\n\n", len(report.CoverageGaps))
		if len(report.Overlaps) > 0 {
			fmt.Fprintln(out, "Overlap candidates:")
			for _, o := range report.Overlaps {
				stem := ""
				if o.SharedStem {
					stem = "  [name-family]"
				}
				fmt.Fprintf(out, "  %.2f  %s <-> %s%s\n", o.Jaccard, o.A, o.B, stem)
			}
			fmt.Fprintln(out)
		}
		if len(report.CoverageGaps) > 0 {
			fmt.Fprintln(out, "Coverage flags:")
			for _, g := range report.CoverageGaps {
				fmt.Fprintf(out, "  - %s (%d bytes, desc=%t)\n", g.Name, g.Size, g.HasDesc)
			}
		}
		if len(report.Overlaps) == 0 && len(report.CoverageGaps) == 0 {
			fmt.Fprintln(out, "Corpus is MECE-clean: no overlaps, no coverage gaps.")
		}
	}

	if m.resolveStrict && len(report.Overlaps) > 0 {
		cmd.SilenceUsage = true
		return fmt.Errorf("skills resolve: %d ME overlap candidate(s) need merge review", len(report.Overlaps))
	}
	return nil
}

func (m *Module) findCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "find <intent>",
		Short: "Rank skills by how well they match a free-text intent",
		Long: `Score every skills/<name>/SKILL.md against a free-text intent and
print the best matches by name, description, and score. Turns the skill
catalog from oral tradition into a queryable surface — give an agent a
discovery API instead of memorizing every skill name.

Scoring is a transparent token-overlap: a query word hitting the skill
name counts most, a declared trigger next, a description word least, with
light plural/stem tolerance. Scores are normalized to [0,1]. The catalog
is read from the tree on every run, so a newly added skill is found with
no index rebuild.

Examples:
  ao skills find "close the loop"
  ao skills find autonomous improvement loop --limit 3
  ao skills find "release validation" --json`,
		Args: cobra.MinimumNArgs(1),
		RunE: m.runFind,
	}
	cmd.Flags().BoolVar(&m.findJSON, "json", false, "Emit machine-readable JSON on stdout")
	cmd.Flags().IntVar(&m.findLimit, "limit", 5, "Maximum number of results to return")
	return cmd
}

func (m *Module) runFind(cmd *cobra.Command, args []string) error {
	if m.findLimit < 1 {
		cmd.SilenceUsage = true
		return fmt.Errorf("--limit must be >= 1 (got %d); rerun with e.g. --limit 5", m.findLimit)
	}

	query := joinArgs(args)
	skillsDir, _ := skillsapp.ResolveSkillsRoots()
	metas, err := skills.Load(skillsDir)
	if err != nil {
		cmd.SilenceUsage = true
		return fmt.Errorf("load skills from %s: %w; run `ao skills check` to inspect the tree", skillsDir, err)
	}

	ranked := skills.Score(query, metas)
	top := capResults(ranked, m.findLimit)

	if m.findJSON {
		return renderFindJSON(cmd, top)
	}
	return renderFindText(cmd, query, top)
}

func (m *Module) listCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "Query the generated skill catalog (skills/catalog.json)",
		Long: `Filter the generated skill catalog by hexagonal role, produced or
consumed port, declared practice, or user-invocability. Reads
skills/catalog.json (emitted by scripts/generate-skill-mesh.py and kept
in sync by CI), so queries are fast and never re-parse SKILL.md frontmatter.

Examples:
  ao skills list --role domain
  ao skills list --produces verdict-ledger --json
  ao skills list --practice tdd --user-invocable true`,
		RunE: m.runList,
	}
	cmd.Flags().BoolVar(&m.listJSON, "json", false, "Emit machine-readable JSON")
	cmd.Flags().StringVar(&m.listRole, "role", "", "Filter by hexagonal_role (domain, driving-adapter, ...)")
	cmd.Flags().StringVar(&m.listProduces, "produces", "", "Filter to skills that produce this port/artifact")
	cmd.Flags().StringVar(&m.listConsumes, "consumes", "", "Filter to skills that consume this port/sibling")
	cmd.Flags().StringVar(&m.listPractice, "practice", "", "Filter to skills that apply this practice")
	cmd.Flags().StringVar(&m.listUserInvocable, "user-invocable", "", "Filter by user-invocability (true|false)")
	return cmd
}

func (m *Module) runList(cmd *cobra.Command, _ []string) error {
	cat, err := m.loadCatalogOrErr(cmd)
	if err != nil {
		return err
	}

	filter := skills.ListFilter{
		Role:     m.listRole,
		Produces: m.listProduces,
		Consumes: m.listConsumes,
		Practice: m.listPractice,
	}
	switch m.listUserInvocable {
	case "":
		// no filter
	case "true":
		v := true
		filter.UserInvocable = &v
	case "false":
		v := false
		filter.UserInvocable = &v
	default:
		cmd.SilenceUsage = true
		return fmt.Errorf("--user-invocable must be true or false (got %q)", m.listUserInvocable)
	}

	matches := skills.List(cat.Skills, filter)

	out := cmd.OutOrStdout()
	if m.listJSON {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(matches)
	}
	for _, e := range matches {
		fmt.Fprintf(out, "%-28s %-16s %s\n", e.Name, e.HexagonalRole, e.Description)
	}
	if len(matches) == 0 {
		fmt.Fprintln(cmd.ErrOrStderr(), "no skills match the given filters")
	}
	return nil
}

func (m *Module) consumersCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "consumers <skill>",
		Short: "List skills that consume the named skill",
		Long: `Print the skills whose consumes[] list includes <skill> — i.e. who
depends on it. Reads skills/catalog.json.

Examples:
  ao skills consumers rpi
  ao skills consumers discovery --json`,
		Args: cobra.ExactArgs(1),
		RunE: m.runConsumers,
	}
	cmd.Flags().BoolVar(&m.consumersJSON, "json", false, "Emit machine-readable JSON")
	return cmd
}

func (m *Module) runConsumers(cmd *cobra.Command, args []string) error {
	cat, err := m.loadCatalogOrErr(cmd)
	if err != nil {
		return err
	}
	got := skills.Consumers(cat.Skills, args[0])
	return renderNameList(cmd, got, m.consumersJSON,
		fmt.Sprintf("no skills consume %q", args[0]))
}

func (m *Module) producersCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "producers <output>",
		Short: "List skills that produce the named port/artifact",
		Long: `Print the skills whose produces[] list includes <output> — i.e. who
writes that port or artifact. Reads skills/catalog.json.

Examples:
  ao skills producers verdict-ledger
  ao skills producers handoff --json`,
		Args: cobra.ExactArgs(1),
		RunE: m.runProducers,
	}
	cmd.Flags().BoolVar(&m.producersJSON, "json", false, "Emit machine-readable JSON")
	return cmd
}

func (m *Module) runProducers(cmd *cobra.Command, args []string) error {
	cat, err := m.loadCatalogOrErr(cmd)
	if err != nil {
		return err
	}
	got := skills.Producers(cat.Skills, args[0])
	return renderNameList(cmd, got, m.producersJSON,
		fmt.Sprintf("no skills produce %q", args[0]))
}

func (m *Module) graphCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "graph",
		Short: "Render the generated skill dependency graph",
		Long: `Render the skill execution/delegation graph (A --> B means A declares
B in metadata.dependencies) from skills/catalog.json. Mermaid is the compact
human view; JSON carries typed dependency/context edges and topology diagnostics
for Graphify and other explorers.

Examples:
  ao skills graph
  ao skills graph --format mermaid
  ao skills graph --format json`,
		RunE: m.runGraph,
	}
	cmd.Flags().StringVar(&m.graphFormat, "format", "mermaid", "Graph output format (mermaid|json)")
	return cmd
}

func (m *Module) runGraph(cmd *cobra.Command, _ []string) error {
	cat, err := m.loadCatalogOrErr(cmd)
	if err != nil {
		return err
	}
	switch m.graphFormat {
	case "mermaid":
		fmt.Fprint(cmd.OutOrStdout(), skills.Mermaid(cat.Skills))
		return nil
	case "json":
		raw, err := skills.GraphJSON(cat.Skills)
		if err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(raw))
		return nil
	default:
		cmd.SilenceUsage = true
		return fmt.Errorf("unsupported --format %q (use mermaid or json)", m.graphFormat)
	}
}

func (m *Module) linkCommand() *cobra.Command {
	cmd := &cobra.Command{
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
		RunE: m.runLink,
	}
	cmd.Flags().StringVar(&m.linkDest, "dest", "", "Link into this single dir instead of the auto-detected roots (default: ~/.agents plus every installed runtime)")
	cmd.Flags().BoolVar(&m.linkJSON, "json", false, "Emit machine-readable JSON")
	return cmd
}

func (m *Module) runLink(cmd *cobra.Command, _ []string) error {
	skillsDir, err := skillsapp.ResolveRepoSkillsDir()
	if err != nil {
		cmd.SilenceUsage = true
		return err
	}

	dests, err := skillsapp.ResolveTargetDests(m.linkDest)
	if err != nil {
		cmd.SilenceUsage = true
		return err
	}

	results, anyErr := skillsapp.LinkAllDests(skillsDir, dests, m.host.DryRun())

	if m.linkJSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		if eerr := enc.Encode(results); eerr != nil {
			return eerr
		}
	} else {
		out := cmd.OutOrStdout()
		for _, res := range results {
			skillsapp.RenderLinkResult(out, res)
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

func (m *Module) unlinkCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unlink",
		Short: "Remove the repo-skill symlinks that `skills link` minted",
		Long: `The clean uninstall inverse of ` + "`ao skills link`" + `. Scan each runtime's
live tier and remove EXACTLY the symlinks that link minted — those whose target
resolves into THIS repo's skills/ tree. By DEFAULT it sweeps EVERY agent runtime
you have installed — ~/.agents/skills, ~/.claude/skills, ~/.codex/skills,
~/.gemini/skills, ~/.cursor/skills, and ~/.pi/skills — detected by the config
root existing under $HOME; --dest overrides to a single dir. Idempotent and
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
		RunE: m.runUnlink,
	}
	cmd.Flags().StringVar(&m.unlinkDest, "dest", "", "Sweep this single dir instead of the auto-detected roots (default: ~/.agents plus every installed runtime)")
	cmd.Flags().BoolVar(&m.unlinkJSON, "json", false, "Emit machine-readable JSON")
	return cmd
}

func (m *Module) runUnlink(cmd *cobra.Command, _ []string) error {
	skillsDir, err := skillsapp.ResolveRepoSkillsDir()
	if err != nil {
		cmd.SilenceUsage = true
		return err
	}

	dests, err := skillsapp.ResolveTargetDests(m.unlinkDest)
	if err != nil {
		cmd.SilenceUsage = true
		return err
	}

	results, anyErr := skillsapp.UnlinkAllDests(skillsDir, dests, m.host.DryRun())

	if m.unlinkJSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		if eerr := enc.Encode(results); eerr != nil {
			return eerr
		}
	} else {
		out := cmd.OutOrStdout()
		for _, res := range results {
			skillsapp.RenderUnlinkResult(out, res)
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

// loadCatalogOrErr loads skills/catalog.json with a remediation hint on failure.
func (m *Module) loadCatalogOrErr(cmd *cobra.Command) (*skills.Catalog, error) {
	skillsDir, _ := skillsapp.ResolveSkillsRoots()
	cat, err := skills.LoadCatalog(skillsDir)
	if err != nil {
		cmd.SilenceUsage = true
		return nil, fmt.Errorf("%w; run `python3 scripts/generate-skill-mesh.py` to (re)build it", err)
	}
	return cat, nil
}

// joinArgs concatenates positional args into a single query string so both
// quoted ("close the loop") and bare (close the loop) forms work.
func joinArgs(args []string) string {
	out := ""
	for i, a := range args {
		if i > 0 {
			out += " "
		}
		out += a
	}
	return out
}

// capResults returns at most limit matches.
func capResults(matches []skills.Match, limit int) []skills.Match {
	if len(matches) > limit {
		return matches[:limit]
	}
	return matches
}

// renderFindJSON writes the matches as a JSON array on stdout (data only;
// diagnostics, if any, go to stderr).
func renderFindJSON(cmd *cobra.Command, top []skills.Match) error {
	if top == nil {
		top = []skills.Match{}
	}
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(top)
}

// renderFindText prints the positive-score matches as a ranked list on stdout.
// When nothing scores above zero it reports gracefully on stderr and exits 0 —
// an unmatched intent is not an error.
func renderFindText(cmd *cobra.Command, query string, top []skills.Match) error {
	shown := 0
	out := cmd.OutOrStdout()
	for _, mm := range top {
		if mm.Score <= 0 {
			continue
		}
		shown++
		fmt.Fprintf(out, "%d. %-28s %.3f  %s\n", shown, mm.Name, mm.Score, mm.Description)
	}
	if shown == 0 {
		fmt.Fprintf(cmd.ErrOrStderr(),
			"no strong matches for %q — try a broader intent or fewer words\n", query)
	}
	return nil
}

// renderNameList prints a sorted slice of skill names as JSON or one-per-line
// text, with a stderr note when the list is empty (an empty result is not an
// error — the query simply matched nothing).
func renderNameList(cmd *cobra.Command, names []string, asJSON bool, emptyNote string) error {
	out := cmd.OutOrStdout()
	if asJSON {
		if names == nil {
			names = []string{}
		}
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(names)
	}
	for _, n := range names {
		fmt.Fprintln(out, n)
	}
	if len(names) == 0 {
		fmt.Fprintln(cmd.ErrOrStderr(), emptyNote)
	}
	return nil
}
