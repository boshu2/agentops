---
name: research
description: 'Explore and write findings. Triggers: "research", "explore and write findings.", "research skill".'
practices:
- wiki-knowledge-surface
- pragmatic-programmer
- ddd-bounded-context
hexagonal_role: driving-adapter
consumes:
- repo-context
produces:
- .agents/research/*.md
- result.json
context_rel: []
skill_api_version: 1
allowed-tools: Read, Grep, Glob, Bash, Write
metadata:
  tier: execution
  dependencies: [cass, ms, reverse-engineer, codebase-recon, pattern-mining]
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
# Research Skill

Answer a bounded question with current, cited evidence and a durable research
artifact. Execute the investigation; do not return a search diary or an
uncited opinion.

## Critical Constraints

- **Why: avoid aimless exploration.** State the question, decision it informs,
  scope, non-goals, freshness needs, and evidence-for-done before searching.
- **Why: prevent rediscovery.** Search `ao lookup` and existing `.agents/`
  knowledge first, then test retrieved claims against current authoritative sources.
- **Why: keep facts trustworthy.** Every load-bearing claim cites `file:line`, a
  commit, or a direct external source; distinguish observation from inference.
- **Why: control context.** Search in bounded directories, follow discovered
  symbols, and stop after three iterative-retrieval cycles unless new evidence
  materially changes the answer.
- **Why: honor operator control.** Use one inline agent by default. Spawn an
  Explore agent or parallel lanes only when the user or active workflow explicitly
  authorizes multi-agent research and scopes non-overlapping work.
- **Why: avoid stale external claims.** Browse current primary sources for
  changing APIs, standards, products, or upstream behavior and cite them directly.
- **Why: preserve uncertainty.** Record gaps, contradictions, failed searches,
  and confidence; do not turn absence of evidence into evidence of absence.

## Inputs and Modes

`/research <question> [--auto] [--from-pr <url>] [quick|medium|very-thorough]`

- `--auto` skips the Gate-1 approval prompt after quality validation; it does
  not authorize external mutations, extra runtimes, or unbounded delegation.
- `--from-pr` narrows source and history inspection to the PR's changed paths.
- Quick answers may stay in chat when no durable handoff is needed. Medium and
  architecture/cross-cutting work writes `.agents/research/`.

## Workflow

1. **Frame the inquiry.** Write the primary question, subquestions, target
   decision, repositories/systems in scope, non-goals, freshness horizon, and
   completion test. Choose quick, medium, or very-thorough depth.
2. **Retrieve prior knowledge.** Run `ao lookup --query "<topic>" --limit 5`
   when available and search `.agents/{research,learnings,knowledge,patterns,
   retros,plans,brainstorm}/` by content. For each applicable hit, record how it
   changes the inquiry and verify it against current source.
3. **Choose evidence lanes.** Use code-map and
   [codebase-archaeology.md](references/codebase-archaeology.md) for repository
   questions; [structural-graph-navigation.md](references/structural-graph-navigation.md)
   for refreshed graphify structure; scoped git history for decision context;
   [software-research.md](references/software-research.md) or primary web sources
   for upstream facts. Structure maps locate relationships, not in-body logic.
4. **Run iterative retrieval.** Start broad inside the declared scope, score
   evidence relevance 0-1, extract symbols/config keys from items scoring at
   least 0.5, and use them in the next pass. Read authoritative files to verify
   every structural or semantic-search lead. Stop after three cycles or saturation.
5. **Select backend deliberately.** Detect the available backend and record it.
   When parallelism is authorized, give Explore agent lanes distinct questions
   and read-only scopes, then merge their evidence. Otherwise research inline.
   See the backend references for Codex, background-task, Claude-team, and inline
   variants; runtime and host instructions decide which are legal.
6. **Validate quality.** Assess coverage, depth (0-4 per critical area), gaps,
   contradictory evidence, and assumptions. Under `--auto`, any critical depth
   below 2 produces WARN plus `.agents/research/quality-warning.md`; do not hide it.
7. **Synthesize.** Write `.agents/research/YYYY-MM-DD-<topic-slug>.md` using
   [document-template.md](references/document-template.md). Lead with the answer,
   then key files/sources, findings, evidence, unresolved questions, confidence,
   recommendations, and the backend used.
8. **Persist reusable findings selectively.** Only reusable findings that should
   alter future planning enter `.agents/findings/registry.jsonl`. Require
   provenance, `dedup_key`, pattern, detection question, checklist item,
   applicability, confidence, and lifecycle fields; merge by key using the
   contract's temp-file-plus-rename atomic write rule. Then run
   `bash hooks/finding-compiler.sh --quiet` when present.
9. **Gate and report.** Unless `--auto`, ask whether the evidence is sufficient
   to proceed to `/plan`, needs revision, or should be abandoned. Report the
   answer, artifact path, confidence/gaps, approval status, and next route.

## Backend Policy

| Condition | Backend |
|---|---|
| no explicit multi-agent authorization | inline current agent |
| authorized Codex parallel lanes | bounded Codex sub-agents |
| authorized runtime lacks sub-agents | documented background-task fallback |
| no legal spawn backend | inline current agent |

Backend selection changes execution mechanics, never evidence standards. Read
[iterative-retrieval.md](references/iterative-retrieval.md) and only the backend
module selected for the run.

## Output Specification

- **Artifact directory:** `.agents/research/`; optional quality warning at
  `.agents/research/quality-warning.md`; reusable findings use the findings registry.
- **Filename convention:** `YYYY-MM-DD-<topic-slug>.md`; stable slug, no
  overwrite of unrelated research.
- **Serialization/schema format:** Markdown following the document template plus
  `result.json` conforming to `skills/research/schemas/findings.json` when a
  machine handoff is required.
- **Validator command:** run `bash skills/research/scripts/validate.sh`, verify
  cited paths/lines or URLs, and confirm critical depth/gap reporting.
- **Downstream handoff:** consumed by `/plan`, `/product`, `/premortem`, or the
  requesting decision maker; reusable findings feed compiled prevention context.

## Quality Rubric

- **Decision-focused:** directly answers the framed question and names implications.
- **Authoritative:** current primary sources and source code outrank summaries.
- **Traceable:** every material claim has reproducible evidence and provenance.
- **Scoped:** search breadth matches the question without context flooding.
- **Honest:** inferences, contradictions, gaps, freshness, and confidence are explicit.
- **Durable:** a fresh reader can act from the artifact without chat context.

## Examples

**User says:** `/research "authentication request flow"`

Trace one entry point through current code, use scoped history for rationale,
cite every transition, and write a medium-depth artifact.

**User says:** `/research --from-pr <url> "does this change preserve retries?"`

Restrict evidence to changed paths and their callers/tests, verify upstream
context, and state remaining uncertainty before recommending action.

## Troubleshooting

| Problem | Response |
|---|---|
| Topic is too broad | Split it into decision-sized questions |
| Prior research conflicts with source | Prefer current source and record the drift |
| Graph result lacks logic | Open the defining/calling files and verify behavior |
| Critical depth is below 2 | WARN, record the gap, and do not imply completeness |
| No spawn backend is authorized | Research inline; do not treat that as degraded evidence |

## References

- [research.feature](references/research.feature) · [document-template.md](references/document-template.md) · [iterative-retrieval.md](references/iterative-retrieval.md)
- [context-discovery.md](references/context-discovery.md) · [source-discovery-and-pattern-extraction.md](references/source-discovery-and-pattern-extraction.md) · [failure-patterns.md](references/failure-patterns.md)
- [codebase-archaeology.md](references/codebase-archaeology.md) · [data-flow-from-entry-points.md](references/data-flow-from-entry-points.md) · [onboarding-methodology.md](references/onboarding-methodology.md)
- [structural-graph-navigation.md](references/structural-graph-navigation.md) · [software-research.md](references/software-research.md) · [deep-research-mcp.md](references/deep-research-mcp.md)
- [backend-codex-subagents.md](references/backend-codex-subagents.md) · [backend-background-tasks.md](references/backend-background-tasks.md) · [backend-claude-teams.md](references/backend-claude-teams.md) · [backend-inline.md](references/backend-inline.md)
- [ralph-loop-contract.md](references/ralph-loop-contract.md) · [vibe-methodology.md](references/vibe-methodology.md) · [claude-code-latest-features.md](references/claude-code-latest-features.md)
