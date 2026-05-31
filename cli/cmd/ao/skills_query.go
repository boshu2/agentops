// practices: [design-by-contract, code-complete]
package main

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/skills"
)

var (
	skillsListJSON          bool
	skillsListRole          string
	skillsListProduces      string
	skillsListConsumes      string
	skillsListPractice      string
	skillsListUserInvocable string // tri-state: "", "true", "false"

	skillsConsumersJSON bool
	skillsProducersJSON bool

	skillsGraphFormat string
)

var skillsListCmd = &cobra.Command{
	Use:   "list",
	Short: "Query the generated skill catalog (skills/catalog.json)",
	Long: `Filter the generated skill catalog by hexagonal role, produced or
consumed port, declared practice, or user-invocability. Reads
skills/catalog.json (emitted by scripts/generate-skill-catalog.sh and kept
in sync by CI), so queries are fast and never re-parse SKILL.md frontmatter.

Examples:
  ao skills list --role domain
  ao skills list --produces verdict-ledger --json
  ao skills list --practice tdd --user-invocable true`,
	RunE: runSkillsList,
}

var skillsConsumersCmd = &cobra.Command{
	Use:   "consumers <skill>",
	Short: "List skills that consume the named skill",
	Long: `Print the skills whose consumes[] list includes <skill> — i.e. who
depends on it. Reads skills/catalog.json.

Examples:
  ao skills consumers rpi
  ao skills consumers discovery --json`,
	Args: cobra.ExactArgs(1),
	RunE: runSkillsConsumers,
}

var skillsProducersCmd = &cobra.Command{
	Use:   "producers <output>",
	Short: "List skills that produce the named port/artifact",
	Long: `Print the skills whose produces[] list includes <output> — i.e. who
writes that port or artifact. Reads skills/catalog.json.

Examples:
  ao skills producers verdict-ledger
  ao skills producers handoff --json`,
	Args: cobra.ExactArgs(1),
	RunE: runSkillsProducers,
}

var skillsGraphCmd = &cobra.Command{
	Use:   "graph",
	Short: "Render the skill consumes-graph",
	Long: `Render the skill dependency graph (A --> B means A consumes B) from
skills/catalog.json. Only the Mermaid flowchart format is supported.

Examples:
  ao skills graph
  ao skills graph --format mermaid`,
	RunE: runSkillsGraph,
}

func init() {
	skillsCmd.AddCommand(skillsListCmd)
	skillsCmd.AddCommand(skillsConsumersCmd)
	skillsCmd.AddCommand(skillsProducersCmd)
	skillsCmd.AddCommand(skillsGraphCmd)

	skillsListCmd.Flags().BoolVar(&skillsListJSON, "json", false, "Emit machine-readable JSON")
	skillsListCmd.Flags().StringVar(&skillsListRole, "role", "", "Filter by hexagonal_role (domain, driving-adapter, ...)")
	skillsListCmd.Flags().StringVar(&skillsListProduces, "produces", "", "Filter to skills that produce this port/artifact")
	skillsListCmd.Flags().StringVar(&skillsListConsumes, "consumes", "", "Filter to skills that consume this port/sibling")
	skillsListCmd.Flags().StringVar(&skillsListPractice, "practice", "", "Filter to skills that apply this practice")
	skillsListCmd.Flags().StringVar(&skillsListUserInvocable, "user-invocable", "", "Filter by user-invocability (true|false)")

	skillsConsumersCmd.Flags().BoolVar(&skillsConsumersJSON, "json", false, "Emit machine-readable JSON")
	skillsProducersCmd.Flags().BoolVar(&skillsProducersJSON, "json", false, "Emit machine-readable JSON")

	skillsGraphCmd.Flags().StringVar(&skillsGraphFormat, "format", "mermaid", "Graph output format (mermaid)")
}

// loadCatalogOrErr loads skills/catalog.json with a remediation hint on failure.
func loadCatalogOrErr(cmd *cobra.Command) (*skills.Catalog, error) {
	skillsDir, _ := resolveSkillsRoots()
	cat, err := skills.LoadCatalog(skillsDir)
	if err != nil {
		cmd.SilenceUsage = true
		return nil, fmt.Errorf("%w; run `scripts/generate-skill-catalog.sh` to (re)build it", err)
	}
	return cat, nil
}

func runSkillsList(cmd *cobra.Command, _ []string) error {
	cat, err := loadCatalogOrErr(cmd)
	if err != nil {
		return err
	}

	filter := skills.ListFilter{
		Role:     skillsListRole,
		Produces: skillsListProduces,
		Consumes: skillsListConsumes,
		Practice: skillsListPractice,
	}
	switch skillsListUserInvocable {
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
		return fmt.Errorf("--user-invocable must be true or false (got %q)", skillsListUserInvocable)
	}

	matches := skills.List(cat.Skills, filter)

	out := cmd.OutOrStdout()
	if skillsListJSON {
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

func runSkillsConsumers(cmd *cobra.Command, args []string) error {
	cat, err := loadCatalogOrErr(cmd)
	if err != nil {
		return err
	}
	got := skills.Consumers(cat.Skills, args[0])
	return renderNameList(cmd, got, skillsConsumersJSON,
		fmt.Sprintf("no skills consume %q", args[0]))
}

func runSkillsProducers(cmd *cobra.Command, args []string) error {
	cat, err := loadCatalogOrErr(cmd)
	if err != nil {
		return err
	}
	got := skills.Producers(cat.Skills, args[0])
	return renderNameList(cmd, got, skillsProducersJSON,
		fmt.Sprintf("no skills produce %q", args[0]))
}

func runSkillsGraph(cmd *cobra.Command, _ []string) error {
	if skillsGraphFormat != "mermaid" {
		cmd.SilenceUsage = true
		return fmt.Errorf("unsupported --format %q (only 'mermaid' is supported)", skillsGraphFormat)
	}
	cat, err := loadCatalogOrErr(cmd)
	if err != nil {
		return err
	}
	fmt.Fprint(cmd.OutOrStdout(), skills.Mermaid(cat.Skills))
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
