// Package ports holds the typed interfaces (hexagonal-architecture
// "ports") that the AgentOps bounded contexts depend on. Concrete
// implementations live alongside the existing packages that already
// own the behavior; the port types here exist so that:
//
//  1. callers can be tested against in-memory adapters instead of real
//     filesystem / network / subprocess collaborators
//  2. drift between the conceptual BC contract and the implementation
//     is visible as a compile-time mismatch on the port type rather
//     than a runtime surprise
//  3. the 5 BCs (Corpus / Validation / Loop / Factory / Runtime, per
//     docs/contracts/ubiquitous-language.md) each get a small, named
//     set of interfaces other BCs can hold without importing the
//     owning BC's full surface
//
// This package is being filled out incrementally per the BC epics:
//
//   - soc-2c1p (BC1 Corpus): CorpusReaderPort, CorpusWriterPort,
//     FindingCompilerPort, CitationPort, ContextCompilerPort
//   - soc-wxh5 (BC2 Validation): GateRunnerPort, CIStatusPort,
//     ClaimEvidenceBinderPort, SafetyPolicyPort
//   - soc-y5vh (BC3 Loop): LoopReaderPort, LoopWriterPort,
//     CloseoutPort, HypothesisLedgerPort, ConvergenceCheckPort
//   - soc-2klg (BC4 Factory): OperatorPort, EventBusPort,
//     FactoryAdmissionPort, ClaimEvidencePort
//   - soc-zd7c (BC5 Runtime): HarnessPort, WorkspacePort
//
// New ports are added in bounded slices, with at least one in-memory
// adapter and a Go test that fires on regression. See
// docs/plans/2026-05-12-rescope-evolve-and-architecture.md for the
// rescoping rationale and ordering.
package ports
