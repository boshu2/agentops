---
name: research
description: 'Answer a bounded question with current cited evidence. Triggers: "research", "investigate", "find evidence".'
practices:
- pragmatic-programmer
- ddd-bounded-context
hexagonal_role: driving-adapter
consumes:
- research-question
produces:
- research-report
context_rel: []
skill_api_version: 1
allowed-tools: Read, Grep, Glob, Bash, Write
metadata:
  capabilities: [research]
  effects: []
  canonical_status: canonical
  disposition: keep_specialist
  tier: execution
  dependencies: []
context:
  window: fork
  intent:
    mode: questions
  sections:
    exclude:
    - HISTORY
    - TASK
  intel_scope: topic
output_contract: skills/research/schemas/findings.json
---
# Research

Answer one bounded question with current evidence. Research informs a caller;
it does not select work, approve a plan, mutate lifecycle state, or decide what
happens next.

## Contract

1. State the question, decision it informs, scope, non-goals, and evidence
   required for a useful answer.
2. Search the smallest relevant local sources. For changing external facts,
   use current primary sources.
3. Verify structural or semantic-search leads against authoritative content.
4. Separate observation, inference, contradiction, and unknown.
5. Lead with the answer and cite every load-bearing claim.
6. Report unchecked scope and stop.

Use the current agent inline by default. Parallel readers or alternate runtimes
are optional execution choices only when the caller authorizes them. Prior
research, CASS, MS, codebase recon, and pattern mining are advisory sources,
not required phases.

## Output

For a quick question, return the cited answer directly. When the caller asks
for a durable artifact, write one report containing:

- question and scope;
- answer;
- evidence references;
- contradictions and unknowns;
- checked and unchecked areas.

Do not emit approval, confidence gates, retry instructions, owner, next action,
or delivery state.
