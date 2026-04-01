# Session Intelligence Trust Model

> **Status:** Draft
> **Consumers:** `ao context assemble`, `ao context explain`, `ao codex start`, future runtime relevance scoring

This contract defines which artifact classes are allowed to influence runtime behavior for Session Intelligence, and which classes stay discovery-only, experimental, or archive-only.

The goal is simple: do not let high-volume `.agents` capture turn into low-trust startup sludge.

## Trust Tiers

| Class | Tier | Default startup | Planning | Pre-mortem | Post-mortem | Notes |
|------|------|-----------------|----------|------------|-------------|-------|
| discovery-notes | discovery-only | no | no | no | yes | Brainstorms, raw discovery notes, and rough research are useful provenance, not default runtime context. |
| pending-knowledge | discovery-only | no | no | no | yes | Pending extraction artifacts are not stable enough for automatic injection. |
| raw-transcripts | archive-only | no | no | no | yes | Raw transcripts remain lookup-only because they are noisy and too large. |
| learning | runtime-eligible | yes | yes | no | yes | Ranked learnings may enter runtime payloads once they pass quality gates. |
| pattern | runtime-eligible | yes | yes | yes | yes | Patterns are compact enough to reuse when query-matched. |
| finding | canonical | yes | yes | yes | yes | Promoted findings are the highest-trust reusable runtime signal. |
| planning-rule | canonical | yes | yes | yes | yes | Compiled planning rules are canonical prevention memory. |
| known-risk | canonical | yes | yes | yes | yes | Compiled pre-mortem checks are canonical risk memory. |
| next-work | runtime-eligible | yes | yes | yes | yes | Ranked next-work gives the next session continuity. |
| recent-session | runtime-eligible | yes | no | no | yes | Session summaries help startup recovery when query-matched. |
| research | runtime-eligible | yes | yes | no | yes | Research can help, but it does not outrank findings or compiled rules. |
| topic-packets | experimental | no | no | no | yes | Remain behind packet-health review before default runtime injection. |
| source-manifests | experimental | no | no | no | yes | Good provenance surfaces, poor default startup payloads. |
| promoted-packets | experimental | no | no | no | yes | Higher promise, but still behind health gates until rollout metrics stabilize. |

## Default Suppression Rules

The following classes remain suppressed from default startup context:

- discovery-only artifacts
- archive-only artifacts
- experimental packet families

Suppression does not mean deletion. These classes remain valid lookup, provenance, and validation surfaces. The contract only restricts what enters automatic runtime payloads.

## Runtime Ranking Implications

Session Intelligence should prefer this order when building startup or planning context:

1. canonical artifacts
2. runtime-eligible artifacts
3. experimental families only after explicit health review
4. discovery-only or archive-only artifacts only by explicit lookup or post-mortem use

## Explainability Requirement

Any runtime surface that injects context must be able to explain:

- which artifacts were selected
- why they were selected
- which classes were suppressed
- why packet families were considered missing, thin, or experimental

`ao context explain` is the first CLI surface that exposes this contract directly.
