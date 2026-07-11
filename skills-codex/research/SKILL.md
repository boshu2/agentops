---
name: research
description: 'Investigate current facts and prior art. Triggers: "$research", "research this", "investigate the codebase".'
---
# $research — Evidence-Bound Investigation

In Codex hookless mode, run `ao codex ensure-start` before research; the CLI records startup once per thread and skips duplicates automatically.

Answer a bounded question with current evidence and a durable artifact. Execute
the investigation; do not return an uncited search diary.

## Critical Constraints

- **Why: avoid aimless search.** Frame the decision, scope, non-goals, freshness,
  and evidence-for-done first.
- **Why: prevent rediscovery.** Run `ao lookup` and search existing `.agents/`
  knowledge, then verify retrieved claims against current source.
- **Why: keep facts trustworthy.** Cite `file:line`, commits, or direct primary
  sources; label inference separately from observation.
- **Why: control context.** Scope searches, follow discovered symbols, and stop
  after three retrieval cycles unless new evidence changes the answer.
- **Why: honor operator control.** Stay inline unless the user or active workflow
  explicitly authorizes parallel research with non-overlapping lanes.
- **Why: preserve uncertainty.** Report gaps, contradictions, failed searches,
  freshness, and confidence instead of inventing completeness.

## Inputs

`$research <question> [--auto] [--from-pr <url>] [quick|medium|very-thorough]`

`--auto` skips the post-research approval prompt only; it does not authorize
external mutations or sub-agent fan-out. `--from-pr` narrows inspection to the
PR's changed paths and their dependencies.

## Workflow

1. **Frame.** State the primary question, subquestions, target decision, scope,
   non-goals, freshness horizon, and completion test.
2. **Retrieve.** Use `ao lookup --query "<topic>" --limit 5` when available and
   search `.agents/{research,learnings,knowledge,patterns,retros,plans,
   brainstorm}/` by content. Record which prior items apply and revalidate them.
3. **Choose lanes.** Use code maps and archaeology for repository questions,
   refreshed graphify for structure, scoped git history for rationale, and
   primary external sources for changing upstream facts.
4. **Iterate.** Score discoveries 0-1, extract symbols/config keys from relevant
   hits, refine scoped searches, and read authoritative files. Stop after three
   cycles or saturation.
5. **Execute with the legal backend.** Prefer `spawn_agent` / `send_input` / `wait` for parallel exploration.
   Apply that preference only when parallelism is explicitly authorized; give
   each Explore agent a distinct read-only question. Otherwise work inline.
6. **Validate quality.** Assess coverage, depth 0-4, gaps, contradictions, and
   assumptions. In `--auto`, critical depth below 2 emits WARN and writes
   `.agents/research/quality-warning.md`.
7. **Synthesize.** Write `.agents/research/YYYY-MM-DD-<topic-slug>.md` with the
   answer, key files/sources, cited findings, unresolved questions, confidence,
   recommendations, and backend.
8. **Persist selectively.** Reusable findings go to
   `.agents/findings/registry.jsonl` with provenance, `dedup_key`, controlled
   applicability, confidence, lifecycle, and the temp-file-plus-rename atomic
   write rule. Refresh with `bash hooks/finding-compiler.sh --quiet` when present.
9. **Gate and report.** Unless `--auto`, ask whether to proceed to `$plan`,
   revise, or abandon. Report answer, artifact, gaps/confidence, and approval.

## Codex Execution Profile

- Write findings to `.agents/research/` with file-level references and concrete evidence.
- Keep backend fallback logic explicit: codex sub-agents, then background-task-fallback, then inline.
- Keep each authorized sub-agent lane bounded, read-only, and independently useful.
- Merge evidence by claim and source, not by concatenating agent transcripts.

## Guardrails

- Do not spawn agents merely because the runtime exposes the capability.
- Do not use unscoped repository grep/glob for broad topics.
- Do not treat graph structure, semantic search, or prior research as proof until
  verified in current authoritative source.
- Do not browse secondary summaries when current primary documentation exists.
- Do not promote transient observations into the findings registry.

## Output Specification

- **Artifact directory:** `.agents/research/`; optional quality warning and
  reusable findings use their canonical `.agents/` paths.
- **Filename convention:** `YYYY-MM-DD-<topic-slug>.md`.
- **Serialization/schema format:** document-template Markdown plus `result.json`
  conforming to `skills-codex/research/schemas/findings.json` when required.
- **Validator command:** run `bash skills-codex/research/scripts/validate.sh`,
  verify every citation, and confirm coverage/depth/gap reporting.
- **Downstream handoff:** consumed by `$plan`, `$product`, `$pre-mortem`, or the
  requesting decision; reusable findings feed compiled prevention context.

## Quality Rubric

- Directly answers a decision-sized question.
- Uses current authoritative evidence with reproducible citations.
- Bounds search breadth and records the selected backend.
- Separates observed fact, inference, contradiction, and unknown.
- Reports coverage, depth, gaps, confidence, and freshness honestly.
- Leaves a durable artifact usable without chat-only context.

## Examples

**User says:** `$research "authentication request flow"`

Trace one path through current code, cite transitions, and write the artifact.

**User says:** `$research --from-pr <url> "does this preserve retries?"`

Inspect the changed paths plus callers/tests and state residual uncertainty.

## Troubleshooting

| Problem | Response |
|---|---|
| Question is too broad | Split it into decision-sized subquestions |
| Prior artifact conflicts | Prefer current source and record drift |
| Graph result lacks logic | Open defining and calling files |
| No fan-out is authorized | Work inline without lowering evidence standards |

## References

- [document-template.md](references/document-template.md) · [iterative-retrieval.md](references/iterative-retrieval.md) · [context-discovery.md](references/context-discovery.md)
- [source-discovery-and-pattern-extraction.md](references/source-discovery-and-pattern-extraction.md) · [failure-patterns.md](references/failure-patterns.md)
- [codebase-archaeology.md](references/codebase-archaeology.md) · [data-flow-from-entry-points.md](references/data-flow-from-entry-points.md) · [onboarding-methodology.md](references/onboarding-methodology.md)
- [structural-graph-navigation.md](references/structural-graph-navigation.md) · [software-research.md](references/software-research.md) · [deep-research-mcp.md](references/deep-research-mcp.md)
- [backend-codex-subagents.md](references/backend-codex-subagents.md) · [backend-background-tasks.md](references/backend-background-tasks.md) · [backend-inline.md](references/backend-inline.md)
- [ralph-loop-contract.md](references/ralph-loop-contract.md) · [vibe-methodology.md](references/vibe-methodology.md)
