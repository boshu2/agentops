---
name: domain
description: 'Load the AgentOps language and bounded-context contracts when a term needs precise meaning. Triggers: "define this domain term", "check the bounded context".'
practices:
- ddd-bounded-context
- pragmatic-programmer
hexagonal_role: domain
consumes: []
produces:
- stdout
context_rel: []
skill_api_version: 1
context:
  window: isolated
  intent:
    mode: none
  intel_scope: none
metadata:
  capabilities: [domain]
  effects: []
  canonical_status: canonical
  disposition: keep_specialist
  tier: knowledge
  dependencies: []
output_contract: concise domain-language reference
---
# Domain — ubiquitous language

Use this read-only library when an AgentOps term or bounded-context boundary
needs precise meaning.

## Procedure

1. Read `docs/contracts/ubiquitous-language.md` for the term.
2. Read `docs/contracts/bounded-contexts.yaml` only when ownership or a port
   boundary matters.
3. Return the exact definition and source path.
4. Stop.

Do not invent synonyms that imply lifecycle authority. In particular, Plan,
Candidate, manifest, verdict, revision, strategy, and adapter are semantic
terms; queue, claim, lease, close, land, release, and delivery belong to caller
systems rather than AgentOps core state.

Vocabulary changes are normal source edits to the two contracts above. This
skill does not promote terms, mutate a knowledge index, or create continuation.
