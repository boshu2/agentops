// practices: [sre, distributed-tracing]
package main

import (
	"cmp"
	"fmt"
	"os"
	"strings"
	"time"
)

// WriteLiveStatus writes a Markdown status table to path atomically.
// It writes to path+".tmp" first, then renames to path to avoid partial reads.
// currentPhase is the 0-based index into allPhases that is currently running.
// Phases before currentPhase are marked "done", after are "pending".
func WriteLiveStatus(path string, allPhases []PhaseProgress, currentPhase int) error {
	var b strings.Builder

	b.WriteString("# Live Status\n\n")
	b.WriteString("| Phase | Status | Elapsed | Tools | Tokens | Cost | Action | Retries | Last Error | Updated |\n")
	b.WriteString("|-------|--------|---------|-------|--------|------|--------|---------|------------|---------|\n")

	for i, p := range allPhases {
		var status string
		switch {
		case i < currentPhase:
			status = "done"
		case i == currentPhase:
			status = "running"
		default:
			status = "pending"
		}

		elapsed := p.Elapsed.Truncate(time.Second).String()
		cost := fmt.Sprintf("$%.4f", p.CostUSD)
		action := normalizeLiveStatusField(p.CurrentAction)
		errText := normalizeLiveStatusField(p.LastError)
		updated := "-"
		if !p.LastUpdate.IsZero() {
			updated = p.LastUpdate.Format("15:04:05")
		}

		fmt.Fprintf(&b, "| %s | %s | %s | %d | %d | %s | %s | %d | %s | %s |\n",
			p.Name, status, elapsed, p.ToolCount, p.Tokens, cost, action, p.RetryCount, errText, updated)
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(b.String()), 0o600); err != nil {
		return fmt.Errorf("write tmp: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("rename: %w", err)
	}
	return nil
}

func normalizeLiveStatusField(s string) string {
	v := cmp.Or(strings.TrimSpace(strings.ReplaceAll(s, "|", "/")), "-")
	const maxLen = 72
	if len(v) <= maxLen {
		return v
	}
	return v[:maxLen-3] + "..."
}
