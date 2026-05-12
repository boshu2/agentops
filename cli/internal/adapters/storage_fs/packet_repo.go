// Package storage_fs is a filesystem adapter for the PacketRepository port.
// Storage layout: <root>/.agents/rpi/execution-packet.json (latest)
//
//	<root>/.agents/rpi/runs/<runID>/execution-packet.json (archive)
package storage_fs

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/boshu2/agentops/cli/internal/domain/packet"
	"github.com/boshu2/agentops/cli/internal/ports"
)

type Repo struct{ Root string }

// Compile-time interface check.
var _ ports.PacketRepository = (*Repo)(nil)

func (r *Repo) Save(_ context.Context, runID string, p packet.ExecutionPacket) error {
	if err := p.Validate(); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(r.Root, ".agents/rpi/runs", runID), 0o755); err != nil {
		return err
	}
	if err := writeJSON(filepath.Join(r.Root, ".agents/rpi/execution-packet.json"), p); err != nil {
		return err
	}
	return writeJSON(filepath.Join(r.Root, ".agents/rpi/runs", runID, "execution-packet.json"), p)
}

func (r *Repo) Load(_ context.Context, runID string) (packet.ExecutionPacket, error) {
	return readJSON(filepath.Join(r.Root, ".agents/rpi/runs", runID, "execution-packet.json"))
}

func (r *Repo) LoadLatest(_ context.Context) (packet.ExecutionPacket, error) {
	p, err := readJSON(filepath.Join(r.Root, ".agents/rpi/execution-packet.json"))
	if errors.Is(err, os.ErrNotExist) {
		return packet.ExecutionPacket{}, err
	}
	return p, err
}

func writeJSON(path string, p packet.ExecutionPacket) error {
	b, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

func readJSON(path string) (packet.ExecutionPacket, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return packet.ExecutionPacket{}, err
	}
	var p packet.ExecutionPacket
	return p, json.Unmarshal(b, &p)
}
