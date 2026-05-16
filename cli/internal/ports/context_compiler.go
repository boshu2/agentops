// practices: [hexagonal-architecture, ddd-bounded-context]
package ports

import "context"

// ContextSection is one compiled context unit. Name is stable and
// caller-visible; Body is the compact payload delivered to the phase
// or operator surface; Source is the artifact path, command, or
// contract that produced it.
type ContextSection struct {
	Name   string
	Body   string
	Source string
}

// ContextAssemblyRequest asks the Context Compiler bubble to build a
// bounded packet for one phase or runtime surface. Phase is required
// because token budgets, freshness rules, and allowed sources differ
// between discovery, implementation, validation, closeout, and
// session-start.
type ContextAssemblyRequest struct {
	Phase     string
	Objective string
	MaxTokens int
	Surfaces  []string
}

// ContextPacket is the assembled output. TokenEstimate is advisory;
// adapters that cannot estimate tokens MAY return 0. Citations point
// to durable artifacts that explain why a section was included.
type ContextPacket struct {
	Sections      []ContextSection
	TokenEstimate int
	Citations     []string
	Warnings      []string
}

// ContextCompilerPort is the hookless-first Context Compiler surface.
// Callers such as RPI discovery, explicit startup, standards lookup,
// context budget checks, and future `ao context assemble` flows depend
// on this port so context delivery is an explicit phase operation
// instead of an ambient SessionStart/UserPromptSubmit hook.
//
// Contract:
//
//   - Assemble rejects an empty Phase.
//   - Assemble returns a non-nil Sections slice on success.
//   - Returned slices MUST be safe for the caller to mutate.
//   - Context cancellation MUST be honored on a best-effort basis.
//
// See docs/contracts/hook-lease-inventory.md for the hooks this port
// replaces (`session-start.sh`, `context-guard.sh`,
// `standards-injector.sh`, and context telemetry hooks).
type ContextCompilerPort interface {
	Assemble(ctx context.Context, req ContextAssemblyRequest) (ContextPacket, error)
}
