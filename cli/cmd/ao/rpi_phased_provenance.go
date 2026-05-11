// practices: [agile-manifesto, dora-metrics]
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/boshu2/agentops/cli/internal/provenance"
)

// writePromptAuditTrail writes the fully-rendered prompt to the run's audit directory.
// Path: .agents/rpi/runs/<run-id>/phase-<N>-prompt.md
func writePromptAuditTrail(cwd, runID string, phaseNum int, prompt string) error {
	if runID == "" {
		return nil // no-op for empty run ID
	}
	dir := filepath.Join(cwd, ".agents", "rpi", "runs", runID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(dir, fmt.Sprintf("phase-%d-prompt.md", phaseNum))
	return os.WriteFile(path, []byte(prompt), 0o644)
}

// runProvenanceAudit is a thin cobra adapter over provenance.Audit.
// The cobra command wiring layer invokes this to execute a provenance
// audit over the local .agents/ corpus and render a short summary.
// Dream's INGEST stage calls provenance.Audit directly in-process;
// this wrapper is the operator-invocation path that mirrors the same
// logic with human-readable output.
func runProvenanceAudit(cwd string) (*provenance.AuditReport, error) {
	report, err := provenance.Audit(cwd)
	if err != nil {
		return nil, fmt.Errorf("provenance audit: %w", err)
	}
	return report, nil
}
