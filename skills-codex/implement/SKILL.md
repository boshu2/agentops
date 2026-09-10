---
name: implement
description: 'Implement authorized work and repair understood defects; return derived subject identity and check facts. Triggers: "implement", "implement this bead", "run the experiment". Full plan-to-validation requests route to rpi.'
---
# Implement

Implement the accepted outcome. Repair ordinary known defects directly. Use the existing
intent; no Plan, Recall or Learn worksheet is owed for a clear edit. Implement
owns source changes and factual checks; the runtime derives identity and receipts.

## Workflow

1. Read intent, acceptance, scope and the RPI [boundaries](../rpi/references/boundaries.md)
   before the first write; reuse contracts already loaded in this context.
   When the caller selected episode tracking, obtain permitted work/source
   references before execution and return observed runtime/context identity at
   startup through the native recording channel. Unknowns and recording failures
   stay explicit; do not invent parentage or a second tracker. The optional
   [session association reference](../cass/references/SESSION_FORMATS.md#work-to-session-associations)
   supplies mechanics for that selected workflow.
2. Find nearby validation scripts and tests that consume the edited paths or
   contract wording. Run the smallest applicable existing check before editing
   and after the change. Behavioral changes preserve RED for
   the expected missing behavior; a pure refactor, relocation or documentation
   change may have an honest green baseline. Avoid building elaborate fixtures
   when an existing test or small discriminating probe answers the question.
3. Make the smallest in-scope change. Fix known failures directly, then rerun
   the affected check. A disproved assumption may change the approach within
   accepted outcome and scope; use Plan only for consequential uncertainty.
4. Use targeted tests and applicable repository lint/static checks before
   broad integration. Read the repository's actual check recipe, including
   instrumentation and environment, rather than reconstructing it from memory.
   Run required full checks at integration, not after each small edit. Reuse
   exact-input receipts only while source, tool and relevant environment match.
5. Refactor while acceptance remains green. Inspect changed tests, fixtures,
   goldens, tolerances, suppressions and specification text against original
   intent. Mocks, placeholders or weakened oracles cannot substitute for the
   requested behavior.
6. Have the runtime derive actual changed paths and `subject-manifest.v1`. When changed paths
   affect bound acceptance evidence, run `ao provenance evidence-orphans --root
   <repo-root>` with one `--changed <path>` per derived path, and retain the
   actual output for the validator. Repeat after repairs that change that
   subject; do not hand-invent an orphan list.
7. Return the manifest digest, author context ID, acceptance-check facts and
   evidence references through the caller's response/runtime channel, then stop.
   Include command, result and decision-relevant failures. Keep full output
   accessible instead of pasting successful logs or repeated receipt inventories
   into every handoff. Missing or truncated evidence must remain visible.

## Scope and finish

Report an uncovered live consumer as `file:line` for a caller scope amendment;
continue independent authorized work. Generated companions already included as
scope require no new approval. Acceptance changes always require caller authority.

Specialists advise only. Known defects stay implementation work; a genuine
causal stall follows RPI's at-most-one bounded helper rule. Respect remaining
caller/native bounds and reserve finishing capacity. No retry resets them.

Return facts, not semantic PASS. An implement-only handoff does not authorize
Git, tracker or delivery transitions; existing caller authority remains usable.
A full outcome request uses RPI through fresh final judgment. Success is working
behavior with usable evidence, not volume of logs or process artifacts.
