# Ubiquitous language

| Term | Meaning |
|---|---|
| Intent | The caller's requested behavior and constraints. |
| PlanPacket | One active behavior, acceptance examples, non-goals, evidence, and write scope. |
| CandidatePacket | The Plan digest, author identity, exact subject identity, changed paths, and factual evidence. |
| Subject manifest | Deterministic content identity independent of Git. |
| Fresh validator | A declared context identity distinct from the candidate author's identity. |
| Verdict | `PASS`, `FAIL`, or `NOT_PROVEN` over one acceptance digest and one subject digest. |
| RPI report | `NOT_PLANNED`, `NOT_BUILT`, or the semantic verdict, followed by stop. |
| Revision | A caller-created packet for a new invocation with unchanged acceptance. |
| Strategy | Optional advice such as premortem, postmortem, council, or an idea genie. |
| Adapter | Optional transport or runtime that cannot change core semantics. |

Avoid using admission, claim, lease, queue, close, land, release, or delivery as
AgentOps lifecycle states. Repositories and callers may use those words in their
own systems; AgentOps does not own those transitions.
