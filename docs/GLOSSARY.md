# Glossary

| Term | Definition |
|---|---|
| Acceptance digest | Canonical identity of intent and acceptance examples. |
| CandidatePacket | One implemented subject, its Plan digest, author identity, changed paths, and evidence. |
| Fresh validator | A declared context identity distinct from the candidate author. |
| PlanPacket | One active behavior, examples, non-goals, evidence, and write scope. |
| RPI | The one-pass wrapper around Plan, Implement, and Validate. |
| Subject manifest | Deterministic content identity over paths, kinds, executable bits, and content or symlink digests. |
| Validate | Independent semantic judgment over exact acceptance and subject identities. |
| Verdict | `PASS`, `FAIL`, or `NOT_PROVEN` in `verdict.v2`. |
| RevisionPacket | Caller-created evidence linking a prior verdict to a changed subject under unchanged acceptance. |
| Strategy | Optional advice such as premortem, postmortem, council, or an idea genie. |
| Adapter | Optional runtime or transport that cannot change core semantics. |

`NOT_PLANNED` and `NOT_BUILT` are RPI report statuses, not verdicts.
