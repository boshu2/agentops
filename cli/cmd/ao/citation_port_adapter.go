// practices: [hexagonal-architecture, ddd-bounded-context]
package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/boshu2/agentops/cli/internal/ports"
)

// productionCitationAdapter satisfies ports.CitationPort by delegating
// to the existing verifyFileCitation / verifyFunctionCitation /
// verifySymbolCitation helpers in beads.go. This is the first BC1.1
// (soc-pm5t) wire-up: the adapter lets callers depend on the port
// type, while the production-path implementation continues to live in
// beads.go.
//
// The cycle 75 tests in beads_test.go already cover the underlying
// helpers at 100% (TestVerifyFunctionCitation_Fresh/Stale +
// TestVerifySymbolCitation_Fresh/Stale + TestVerifyFileCitation_*).
// This adapter is a thin shim; its own test surface verifies the
// kind→helper dispatch and the verdict translation.
type productionCitationAdapter struct{}

// newProductionCitationAdapter returns the stateless adapter. Holds
// no fields; concurrent callers are safe.
func newProductionCitationAdapter() *productionCitationAdapter {
	return &productionCitationAdapter{}
}

// Verify routes req to the appropriate verify* helper by Kind and
// translates the helper's mutation-of-Citation output into a
// ports.CitationVerdict. Unknown Kind returns UNKNOWN with a Reason
// that names the offending kind.
func (a *productionCitationAdapter) Verify(ctx context.Context, req ports.CitationRequest) (ports.CitationVerdict, error) {
	if err := ctx.Err(); err != nil {
		return ports.CitationVerdict{}, err
	}
	if strings.TrimSpace(req.Raw) == "" {
		return ports.CitationVerdict{
			Status: ports.CitationStatusUnknown,
			Reason: "empty Raw",
		}, nil
	}
	c := Citation{Kind: string(req.Kind), Raw: req.Raw}
	switch req.Kind {
	case ports.CitationKindFile:
		verifyFileCitation(&c, req.Cwd)
	case ports.CitationKindFunction:
		verifyFunctionCitation(&c, req.Cwd)
	case ports.CitationKindSymbol:
		verifySymbolCitation(&c, req.Cwd)
	default:
		return ports.CitationVerdict{
			Status: ports.CitationStatusUnknown,
			Reason: fmt.Sprintf("unknown citation kind %q (expected file|function|symbol)", req.Kind),
		}, nil
	}
	return ports.CitationVerdict{
		Status:   translateCitationStatus(c.Status),
		Reason:   c.Reason,
		Resolved: c.Resolved,
	}, nil
}

// translateCitationStatus maps the existing CitationStatus values to
// the port's CitationStatusResult. The string values are identical,
// but the types are distinct — this conversion is the boundary.
func translateCitationStatus(s CitationStatus) ports.CitationStatusResult {
	switch s {
	case CitationFresh:
		return ports.CitationStatusFresh
	case CitationStale:
		return ports.CitationStatusStale
	default:
		return ports.CitationStatusUnknown
	}
}

// Compile-time assertion: productionCitationAdapter satisfies the port.
var _ ports.CitationPort = (*productionCitationAdapter)(nil)
