// practices: [hexagonal-architecture, tdd]
package ports

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestInMemoryContextCompiler_ReturnsConfiguredPacket(t *testing.T) {
	compiler := NewInMemoryContextCompiler(map[string]ContextPacket{
		"discovery": {
			Sections:      []ContextSection{{Name: "intent", Body: "build packet", Source: "test"}},
			Citations:     []string{"docs/cdlc.md"},
			Warnings:      []string{"budget near cap"},
			TokenEstimate: 42,
		},
	})

	packet, err := compiler.Assemble(context.Background(), ContextAssemblyRequest{Phase: "discovery"})
	if err != nil {
		t.Fatal(err)
	}
	if len(packet.Sections) != 1 || packet.Sections[0].Name != "intent" {
		t.Fatalf("Sections = %#v, want configured section", packet.Sections)
	}
	if packet.TokenEstimate != 42 {
		t.Fatalf("TokenEstimate = %d, want 42", packet.TokenEstimate)
	}
	if len(packet.Citations) != 1 || packet.Citations[0] != "docs/cdlc.md" {
		t.Fatalf("Citations = %#v, want docs/cdlc.md", packet.Citations)
	}
}

func TestInMemoryContextCompiler_UnknownPhaseReturnsEmptyNonNilSlices(t *testing.T) {
	compiler := NewInMemoryContextCompiler(nil)
	packet, err := compiler.Assemble(context.Background(), ContextAssemblyRequest{Phase: "validation"})
	if err != nil {
		t.Fatal(err)
	}
	if packet.Sections == nil {
		t.Fatal("Sections is nil, want non-nil empty slice")
	}
	if packet.Citations == nil {
		t.Fatal("Citations is nil, want non-nil empty slice")
	}
	if packet.Warnings == nil {
		t.Fatal("Warnings is nil, want non-nil empty slice")
	}
}

func TestInMemoryContextCompiler_RejectsEmptyPhase(t *testing.T) {
	compiler := NewInMemoryContextCompiler(nil)
	_, err := compiler.Assemble(context.Background(), ContextAssemblyRequest{})
	if err == nil {
		t.Fatal("expected error for empty phase")
	}
	if !strings.Contains(err.Error(), "phase required") {
		t.Fatalf("error = %v, want phase required", err)
	}
}

func TestInMemoryContextCompiler_ReturnsDefensiveCopies(t *testing.T) {
	compiler := NewInMemoryContextCompiler(map[string]ContextPacket{
		"session-start": {
			Sections:  []ContextSection{{Name: "one"}},
			Citations: []string{"a"},
			Warnings:  []string{"w"},
		},
	})

	first, err := compiler.Assemble(context.Background(), ContextAssemblyRequest{Phase: "session-start"})
	if err != nil {
		t.Fatal(err)
	}
	first.Sections[0].Name = "mutated"
	first.Citations[0] = "mutated"
	first.Warnings[0] = "mutated"

	second, err := compiler.Assemble(context.Background(), ContextAssemblyRequest{Phase: "session-start"})
	if err != nil {
		t.Fatal(err)
	}
	if second.Sections[0].Name != "one" || second.Citations[0] != "a" || second.Warnings[0] != "w" {
		t.Fatalf("second packet was mutated: %#v", second)
	}
}

func TestInMemoryContextCompiler_HonorsContextCancellation(t *testing.T) {
	compiler := NewInMemoryContextCompiler(nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := compiler.Assemble(ctx, ContextAssemblyRequest{Phase: "x"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}
