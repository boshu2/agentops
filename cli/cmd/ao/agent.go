// practices: [hexagonal-architecture, design-by-contract]
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// agentCmd is the `ao agent` noun: produce runtime-specific Agent definitions
// that make an out-of-session loop (Managed Agent / Codex-NTM swarm)
// AgentOps-native. Distinct from `ao agents` (which manages AGENTS.md surfaces).
var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Produce AgentOps-native Agent definitions for out-of-session loops",
	Long: `Emit a runtime-specific Agent definition (Managed Agents payload or
NTM bundle) that carries the AgentOps skill set plus the ao tool surface, so an
out-of-session agent runs under the same guardrails as interactive work.

Managed Agents are NOT ZDR: the bundle builder refuses to inline any holdout /
eval content (the eval substrate is LOCKED).`,
}

var (
	agentBundleRuntime string
	agentBundleSkills  string
	agentBundleSandbox string
	agentBundleOut     string
	agentBundleJSON    bool
)

var agentBundleCmd = &cobra.Command{
	Use:   "bundle",
	Short: "Emit a runtime-specific Agent definition (managed | codex-ntm)",
	Long: `Stitch the selected AgentOps skills + the ao tool surface into an
Agent definition for the chosen runtime.

  ao agent bundle --runtime managed              # Managed Agents JSON payload
  ao agent bundle --runtime codex-ntm --json     # NTM-consumable bundle
  ao agent bundle --runtime managed --sandbox self-hosted

Default skills: session-bootstrap, standards, validation, provenance.
Refuses (non-zero) if any selected skill would inline holdout/eval content.`,
	RunE: runAgentBundle,
}

func init() {
	rootCmd.AddCommand(agentCmd)
	agentCmd.AddCommand(agentBundleCmd)
	agentBundleCmd.Flags().StringVar(&agentBundleRuntime, "runtime", "", "Target runtime: managed | codex-ntm (required)")
	agentBundleCmd.Flags().StringVar(&agentBundleSkills, "skills", "", "Comma-separated skill names (default: session-bootstrap,standards,validation,provenance)")
	agentBundleCmd.Flags().StringVar(&agentBundleSandbox, "sandbox", "", "Sandbox placement: self-hosted | cloud")
	agentBundleCmd.Flags().StringVar(&agentBundleOut, "out", "", "Write the bundle to this path instead of stdout")
	agentBundleCmd.Flags().BoolVar(&agentBundleJSON, "json", false, "Emit machine-readable JSON (always JSON for now; reserved for parity)")
}

func runAgentBundle(cmd *cobra.Command, _ []string) error {
	if agentBundleRuntime == "" {
		return fmt.Errorf("--runtime is required (managed | codex-ntm)")
	}
	var skills []string
	if s := strings.TrimSpace(agentBundleSkills); s != "" {
		for _, part := range strings.Split(s, ",") {
			if p := strings.TrimSpace(part); p != "" {
				skills = append(skills, p)
			}
		}
	}
	bundle, err := buildAgentBundle(bundleOptions{
		Runtime: agentBundleRuntime,
		Skills:  skills,
		Sandbox: agentBundleSandbox,
	})
	if err != nil {
		return err
	}
	raw, err := json.MarshalIndent(bundle, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling agent bundle: %w", err)
	}
	if agentBundleOut != "" {
		if err := os.WriteFile(agentBundleOut, append(raw, '\n'), 0o644); err != nil {
			return fmt.Errorf("writing bundle to %s: %w", agentBundleOut, err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "wrote %s agent bundle to %s\n", bundle.Runtime, agentBundleOut)
		return nil
	}
	fmt.Fprintln(cmd.OutOrStdout(), string(raw))
	return nil
}
