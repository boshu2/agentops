# Ubiquitous language

| Term | Meaning |
|---|---|
| Intent | The caller's requested behavior and constraints. |
| Intent source | The caller-owned bead, issue, or conversation containing behavior, acceptance, and scope. |
| Intent digest | A runtime-derived digest used to detect acceptance drift; not a model-authored artifact. |
| Subject manifest | Deterministic content identity independent of Git. |
| Fresh validator | A declared context identity distinct from the candidate author's identity. |
| Verdict | `PASS`, `FAIL`, or `NOT_PROVEN` over one acceptance digest and one subject digest. |
| RPI report | `NOT_PLANNED`, `NOT_BUILT`, or the semantic verdict, followed by stop. |
| Revision | A change to the caller-owned intent source followed by a new invocation. |
| Strategy | Optional advice such as premortem, postmortem, council, or an idea genie. |
| Adapter | Optional transport or runtime that cannot change core semantics. |

Avoid using admission, claim, lease, queue, close, land, release, or delivery as
AgentOps lifecycle states. Repositories and callers may use those words in their
own systems; AgentOps does not own those transitions.
