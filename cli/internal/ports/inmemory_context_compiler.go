// practices: [hexagonal-architecture, ddd-bounded-context]
package ports

import (
	"context"
	"errors"
)

// InMemoryContextCompiler is a ContextCompilerPort backed by a fixed
// phase -> packet map. It is intended for tests and dry-runs of
// phase-scoped context assembly without touching the real corpus.
type InMemoryContextCompiler struct {
	packets map[string]ContextPacket
}

// NewInMemoryContextCompiler returns an adapter with the supplied
// packets keyed by phase.
func NewInMemoryContextCompiler(packets map[string]ContextPacket) *InMemoryContextCompiler {
	if packets == nil {
		packets = map[string]ContextPacket{}
	}
	return &InMemoryContextCompiler{packets: cloneContextPackets(packets)}
}

// Assemble returns the configured packet for req.Phase, or an empty
// non-nil packet when the phase is unknown.
func (c *InMemoryContextCompiler) Assemble(ctx context.Context, req ContextAssemblyRequest) (ContextPacket, error) {
	if err := ctx.Err(); err != nil {
		return ContextPacket{}, err
	}
	if req.Phase == "" {
		return ContextPacket{}, errors.New("ports: ContextCompilerPort.Assemble phase required")
	}
	packet, ok := c.packets[req.Phase]
	if !ok {
		return ContextPacket{Sections: []ContextSection{}, Citations: []string{}, Warnings: []string{}}, nil
	}
	return cloneContextPacket(packet), nil
}

func cloneContextPackets(in map[string]ContextPacket) map[string]ContextPacket {
	out := make(map[string]ContextPacket, len(in))
	for key, packet := range in {
		out[key] = cloneContextPacket(packet)
	}
	return out
}

func cloneContextPacket(packet ContextPacket) ContextPacket {
	return ContextPacket{
		Sections:      append([]ContextSection(nil), packet.Sections...),
		TokenEstimate: packet.TokenEstimate,
		Citations:     append([]string(nil), packet.Citations...),
		Warnings:      append([]string(nil), packet.Warnings...),
	}
}

var _ ContextCompilerPort = (*InMemoryContextCompiler)(nil)
