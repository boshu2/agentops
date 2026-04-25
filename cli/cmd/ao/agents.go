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

var agentsCmd = &cobra.Command{
	Use:   "agents",
	Short: "Inspect and manage .agents/ contracts and surfaces",
	Long: `Tooling for the .agents/ knowledge surface that backs the
AgentOps flywheel. Subcommands surface the catalogued write surfaces,
the active skill-owned subdirs, and (in future cycles) lint and
migration helpers.`,
}

var (
	agentsInspectJSON     bool
	agentsInspectContract string
)

var agentsInspectCmd = &cobra.Command{
	Use:   "inspect",
	Short: "Show catalogued .agents/ write surfaces and active skill-owned subdirs",
	Long: `Read the .agents/ write-surface contract and emit a structured
summary: the catalogued allowlist plus the set of active skills whose
.agents/<skill-name>/ subdirs are auto-allowed by
scripts/check-agents-write-surfaces.sh.`,
	RunE: runAgentsInspect,
}

func init() {
	rootCmd.AddCommand(agentsCmd)
	agentsCmd.AddCommand(agentsInspectCmd)
	agentsInspectCmd.Flags().BoolVar(&agentsInspectJSON, "json", false, "Emit machine-readable JSON")
	agentsInspectCmd.Flags().StringVar(&agentsInspectContract, "contract",
		"docs/contracts/agents-write-surfaces.md",
		"Path to the .agents/ write-surfaces contract doc")
}

// AgentsInventory is the shape returned by `ao agents inspect --json`.
type AgentsInventory struct {
	Contract  string   `json:"contract"`
	Allowlist []string `json:"allowlist"`
	Skills    []string `json:"skills"`
}

func runAgentsInspect(cmd *cobra.Command, args []string) error {
	contract := agentsInspectContract
	data, err := os.ReadFile(contract)
	if err != nil {
		return fmt.Errorf("reading contract %s: %w", contract, err)
	}

	inv := AgentsInventory{
		Contract:  contract,
		Allowlist: parseAgentsAllowlist(string(data)),
		Skills:    discoverActiveSkills("skills"),
	}

	if agentsInspectJSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(inv)
	}

	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Contract: %s\n", inv.Contract)
	fmt.Fprintf(out, "Catalogued surfaces: %d\n", len(inv.Allowlist))
	fmt.Fprintf(out, "Skill-owned subdirs: %d\n", len(inv.Skills))
	fmt.Fprintln(out)

	fmt.Fprintln(out, "Catalogued surfaces (allowlist):")
	for _, e := range inv.Allowlist {
		fmt.Fprintf(out, "  .agents/%s/\n", e)
	}
	if len(inv.Allowlist) == 0 {
		fmt.Fprintln(out, "  (none)")
	}
	fmt.Fprintln(out)

	fmt.Fprintln(out, "Skill-owned subdirs (auto-allowed):")
	for _, e := range inv.Skills {
		fmt.Fprintf(out, "  .agents/%s/\n", e)
	}
	if len(inv.Skills) == 0 {
		fmt.Fprintln(out, "  (none)")
	}
	return nil
}

// parseAgentsAllowlist extracts the allowlist between the BEGIN/END
// markers in the contract doc. Inline `# comment` and blank lines are
// stripped. The result is sorted and de-duplicated.
func parseAgentsAllowlist(content string) []string {
	const beginMarker = "<!-- BEGIN agents-write-surfaces-allowlist -->"
	const endMarker = "<!-- END agents-write-surfaces-allowlist -->"

	seen := make(map[string]bool)
	out := []string{}
	inside := false
	for _, line := range strings.Split(content, "\n") {
		if strings.Contains(line, beginMarker) {
			inside = true
			continue
		}
		if strings.Contains(line, endMarker) {
			inside = false
			continue
		}
		if !inside {
			continue
		}
		if i := strings.Index(line, "#"); i >= 0 {
			line = line[:i]
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if seen[line] {
			continue
		}
		seen[line] = true
		out = append(out, line)
	}
	sort.Strings(out)
	return out
}

// discoverActiveSkills returns the names of skills/<name>/ entries that
// have a SKILL.md file. Result is sorted.
func discoverActiveSkills(skillsDir string) []string {
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		return []string{}
	}
	out := []string{}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if _, err := os.Stat(filepath.Join(skillsDir, e.Name(), "SKILL.md")); err != nil {
			continue
		}
		out = append(out, e.Name())
	}
	sort.Strings(out)
	return out
}
