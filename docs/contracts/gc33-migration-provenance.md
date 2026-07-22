# GC 3.3 migration provenance

AgentOps 3.3 targets the official Gas City v1.3.5 commit
`8ffc009ded781a2ada2077f3a29bd712b2def0bf` and official Beads v1.1.0 commit
`8e4e59d39f3459a43cf21a3236a13eca4dd874f7`.

The earlier custom AgentOps factory feeder, packet RPC, schema family, and Go
delivery reducer are intentionally absent from the active 3.3 runtime. Their
merged Git history is retained as archaeology. No Gas City or Beads fork is
part of the released pack.
