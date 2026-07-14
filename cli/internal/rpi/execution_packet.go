package rpi

import "github.com/boshu2/agentops/cli/internal/domain/packet"

// ExecutionPacketFile is the canonical filename for execution packets.
const ExecutionPacketFile = "execution-packet.json"

// ExecutionPacket and its nested types are domain-canonical in
// internal/domain/packet. RPI keeps these aliases so existing discovery and
// schema tests use the same concrete contract as storage validation.
type ExecutionPacket = packet.ExecutionPacket
type ExecutionPacketProgram = packet.ExecutionPacketProgram
type ExecutionPacketModelFamily = packet.ExecutionPacketModelFamily
type ExecutionPacketVerdict = packet.ExecutionPacketVerdict
type ExecutionPacketRouting = packet.ExecutionPacketRouting
type ExecutionPacketSpec = packet.ExecutionPacketSpec
type Criterion = packet.Criterion
type ValidationLane = packet.ValidationLane
type ExecutionPacketDensity = packet.ExecutionPacketDensity
type ExecutionPacketBoundary = packet.ExecutionPacketBoundary
type ExecutionPacketArtifacts = packet.ExecutionPacketArtifacts
type ExecutionPacketTestLevels = packet.ExecutionPacketTestLevels
type ExecutionPacketOrchestrationDecision = packet.ExecutionPacketOrchestrationDecision
type DiscoveryArtifactScope = packet.DiscoveryArtifactScope
type TestLevel = packet.TestLevel
type Complexity = packet.Complexity

const (
	ExecutionPacketModelClaude = packet.ExecutionPacketModelClaude
	ExecutionPacketModelCodex  = packet.ExecutionPacketModelCodex
	ExecutionPacketModelGemini = packet.ExecutionPacketModelGemini

	ExecutionPacketVerdictPass = packet.ExecutionPacketVerdictPass
	ExecutionPacketVerdictFail = packet.ExecutionPacketVerdictFail

	L0 = packet.L0
	L1 = packet.L1
	L2 = packet.L2
	L3 = packet.L3

	ComplexityFast     = packet.ComplexityFast
	ComplexityStandard = packet.ComplexityStandard
	ComplexityFull     = packet.ComplexityFull
)
