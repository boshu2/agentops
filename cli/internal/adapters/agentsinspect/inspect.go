// practices: [wiki-knowledge-surface, design-by-contract]
package agentsinspect

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/boshu2/agentops/cli/internal/adapters/agentsurface"
)

// Options configures the .agents surface inspection adapter.
type Options struct {
	Contract  string
	SkillsDir string
	JSON      bool
	Stdout    io.Writer
}

// Run reads the .agents write-surface contract and emits either the JSON
// inventory or the human-readable inspection report.
func Run(opts Options) error {
	if opts.Stdout == nil {
		opts.Stdout = io.Discard
	}

	data, err := os.ReadFile(opts.Contract)
	if err != nil {
		return fmt.Errorf("reading contract %s: %w", opts.Contract, err)
	}

	inv := agentsurface.Inventory{
		Contract:  opts.Contract,
		Allowlist: agentsurface.ParseAllowlist(string(data)),
		Skills:    agentsurface.DiscoverActiveSkills(opts.SkillsDir),
	}

	if opts.JSON {
		enc := json.NewEncoder(opts.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(inv)
	}

	PrintText(opts.Stdout, inv)
	return nil
}

// PrintText renders the human inspection report.
func PrintText(out io.Writer, inv agentsurface.Inventory) {
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
}
