---
name: standards
description: 'Load only the standards relevant to a caller-supplied change, then report concrete findings. Triggers: "check standards", "which standards apply".'
practices:
- pragmatic-programmer
- clean-code
hexagonal_role: supporting
consumes: []
produces:
- stdout
context_rel: []
skill_api_version: 1
metadata:
  capabilities: [standards]
  effects: []
  canonical_status: canonical
  disposition: keep_specialist
  tier: knowledge
  dependencies: []
output_contract: cited standards and factual findings
---
# Standards — focused engineering guidance

Load the smallest set of standards justified by the caller's files, language,
and risks. Do not preload the entire reference corpus.

## Constraints

- One invocation reviews only caller-supplied repository-relative paths and
  stops after one findings packet because this skill supplies judgment context,
  not mutation or lifecycle authority.
- Load `common-standards.md`, the exact language owner, and only checklists
  justified by declared risk cues. Every selected reference needs a concrete
  reason and must resolve inside this package because broad citation cannot
  show which rule actually governed a finding.
- Findings require an input path, positive line, selected reference, section,
  severity, and concrete message. Empty findings are valid after nonempty
  checked scope because a clean review still needs an auditable inspected
  surface.
- `COMPLETE` requires empty `not_checked`; otherwise use `INCOMPLETE` and name
  each unchecked surface.
- Captured JSON must pass `scripts/check-findings.sh` before it is reported as
  conforming to this output contract.

## Procedure

1. Record the supplied paths, language, change type, and risk cues.
2. Load `common-standards.md` plus only the matching language or checklist
   references.
3. Compare the supplied artifact to those sources.
4. Return cited findings with path and line when possible, plus checked and
   not-checked scope.
5. Stop.

This skill provides context and findings. It does not edit, validate, retry,
approve, commit, release, deliver, or decide continuation.

## Load-bearing conventions for produced code

When the caller is about to WRITE code (not only review it), surface the
matching language rules INLINE in the working context — a behind-the-link
reference does not change behavior; an inline imperative does. The Go core:

- Wrap every propagated error with context: `fmt.Errorf("doing X: %w", err)`
  — never return a bare inner error.
- Multi-case functions get TABLE-DRIVEN tests (`[]struct` cases + `t.Run` per
  case), asserting exact expected values including the error cases.

For other languages, pull the matching reference below and inline its top
rules the same way.

Historical note: a 2026-08-04 two-run directional probe favored inline rules,
but its ledger entry is legacy-unverified. It motivates the placement above;
it is not current runtime or effectiveness proof.

## Mutation-safety standards

When the supplied change rewrites existing files in bulk — formatters,
codemods, migration scripts, generators pointed at hand-written sources —
check it against three standards and report each as a finding when absent:

- **Single audited mutation chokepoint.** All rewrites flow through one named
  command or script whose inputs, outputs, and dry-run mode can be inspected.
  Edits scattered across ad-hoc one-liners and manual touch-ups are the
  **diffuse mutation** failure mode: no single point can be audited, re-run,
  or blamed. Finding: name every mutation path outside the chokepoint.
- **Hash-witnessed backups before rewrite.** Before the chokepoint runs, the
  originals are preserved with content hashes recorded (a committed baseline
  counts), so "the rewrite changed only what it claims" is checkable
  byte-for-byte, not asserted. Finding: a bulk rewrite with no verifiable
  before-state.
- **Self-administered ambition gate.** The change states what it deliberately
  does not touch, and the diff respects it. A formatter run that also renames,
  a codemod that also refactors, is the **scope-creep rewrite** failure mode.
  Finding: any file class in the diff outside the change's own stated scope.

Stop condition for this check: all three standards have an explicit pass or
finding; a bulk-rewrite review that reports style nits but skips these is
incomplete.

## Quality, output, and done

Return JSON with `decision`, `change` (`paths`, `language`, `change_type`, and
`risk_cues`), `selected_references`, one `reference_reasons` entry per selected
reference, `checked`, `not_checked`, and `findings`. Use the risk-cue vocabulary
enforced by `scripts/check-findings.sh`.

Named failure mode — **citation laundering**: a plausible rule is attributed to
a reference that was never loaded, leaving no auditable source. Anti-pattern:
emit generic prose or cite the entire corpus. Corrective: select the minimum
justified files and bind every finding to one selected file, section, input
path, and line.

Done means the executable checker accepts the packet and its decision matches
the checked boundary. This proves output integrity and package-local reference
resolution; it does not prove that the reviewed artifact is correct or that an
external tool/runtime follows the cited standard.

- Every selected reference resolves within this package and has one stated
  selection reason.
- Every finding binds one supplied path and positive line to one selected
  reference section.
- The decision is `COMPLETE` only with nonempty checked scope and empty
  `not_checked`; incomplete reviews preserve the missing surface.

## References

- [Common standards](references/common-standards.md)
- [Go](references/go.md)
- [Python](references/python.md)
- [Rust](references/rust.md)
- [TypeScript](references/typescript.md)
- [JavaScript](references/javascript.md)
- [Shell](references/shell.md)
- [JSON](references/json.md)
- [YAML](references/yaml.md)
- [Markdown](references/markdown.md)
- [SQL safety](references/sql-safety-checklist.md)
- [Race conditions](references/race-condition-checklist.md)
- [LLM trust boundaries](references/llm-trust-boundary-checklist.md)
- [Skill structure](references/skill-structure.md)
- [Test strategy](references/test-pyramid.md)
