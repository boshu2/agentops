package main

import "strings"

type mixedModeProvenance struct {
	Requested      bool
	Effective      bool
	PlannerVendor  string
	ReviewerVendor string
	DegradedReason string
}

func mixedModeProvenanceFromOpts(opts phasedEngineOptions) mixedModeProvenance {
	if !opts.Mixed {
		return mixedModeProvenance{}
	}

	plannerVendor := runtimeVendorName(opts.RuntimeCommand)
	reviewerVendor := "codex"
	prov := mixedModeProvenance{
		Requested:      true,
		PlannerVendor:  plannerVendor,
		ReviewerVendor: reviewerVendor,
	}

	switch {
	case plannerVendor == "":
		prov.DegradedReason = "mixed mode requested but the phase runtime vendor is unknown"
	case plannerVendor == reviewerVendor:
		prov.DegradedReason = "mixed mode requested with codex as the phase runtime; no distinct reviewer vendor was selected"
	default:
		prov.Effective = true
	}

	return prov
}

func runtimeVendorName(command string) string {
	name := strings.ToLower(strings.TrimSpace(runtimeBinaryName(effectiveRuntimeCommand(command))))
	switch {
	case strings.Contains(name, "claude"):
		return "claude"
	case strings.Contains(name, "codex"):
		return "codex"
	case strings.Contains(name, "opencode"):
		return "opencode"
	default:
		return ""
	}
}
