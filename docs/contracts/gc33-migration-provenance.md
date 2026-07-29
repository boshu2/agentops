# GC 3.3 migration provenance

AgentOps 3.3 targets the official Gas City v1.4.0 commit
`a7297c511d637a3609947386f3389d76ddb2f23b` and official Beads v1.1.0 commit
`8e4e59d39f3459a43cf21a3236a13eca4dd874f7`.

The optional factory also composes the public `gascity` workflow pack v0.1.6 at
`3b3b89f2011e06d84459aa7bea1552382f13930a`, the pack family used by the
public Maintainer City factory. `deploy/gc/pack-registry.lock.json` records the
accepted registry source, content hash, roles subpack, and public factory URL.
Runtime `packs.lock` files remain Gas City's resolved import authority.
The workflow stays composed into `agentops-factory`; the sibling roles bind at
`defaults.rig.imports.gc`, matching the official formulas' `gc.*` targets.

The earlier custom AgentOps factory feeder, packet RPC, schema family, and Go
delivery reducer are intentionally absent from the active 3.3 runtime. Their
merged Git history is retained as archaeology. No Gas City or Beads fork is
part of the released pack.
