// Package ports declares the secondary (driven) ports of the AgentOps domain.
// Concrete adapters live in cli/internal/adapters/*.
package ports

import (
	"context"

	"github.com/boshu2/agentops/cli/internal/domain/packet"
)

// PacketRepository is the driven port for ExecutionPacket persistence.
type PacketRepository interface {
	Save(ctx context.Context, runID string, p packet.ExecutionPacket) error
	Load(ctx context.Context, runID string) (packet.ExecutionPacket, error)
	LoadLatest(ctx context.Context) (packet.ExecutionPacket, error)
}
