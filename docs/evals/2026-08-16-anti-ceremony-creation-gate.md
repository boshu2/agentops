# Anti-ceremony creation-gate probe — 2026-08-16

> **Historical v2 compatibility result: `INERT` (directional N=2).** Both
> control and treatment made the discriminator-approved decision in both
> repetitions. The retained fixture predates the v3 response-only,
> counterbalanced, self-contained capture contract, so it is not current
> coverage evidence and does not establish quality or outcomes.

## Question and discriminator

The identical question in both arms asks for two create-or-drop decisions:

- drop a permanent release-readiness dashboard with no named consumer, blocked
  decision, observed defect, or deletion condition;
- create a short-lived provenance snapshot consumed by a release owner to
  prevent recurrence of a named evidence-loss incident.

The deterministic discriminator reports `PRESENT` only for `A: DROP` and
`B: CREATE`. It extracts decisions only from the final Codex response segment,
not the echoed question, so a partial answer cannot borrow a missing decision
from prompt text. It does not score explanation quality or an executed release
outcome.

## Canonical-skill v2 run

The treatment prompt included the exact bound canonical
`skills/operationalize/SKILL.md`; control received only the identical question.
This tests the response-shape effect of including those skill bytes. It does
not test automatic routing or discovery.

| Field | Value |
|---|---|
| Producer observed in all transcript headers | Codex `gpt-5.6-luna`, low effort |
| Requested producer | `gpt-5.6-luna`, low effort |
| Treatment source | `canonical-skill` |
| Control | 2 present / 2 usable (1.0) |
| Treatment | 2 present / 2 usable (1.0) |
| Verdict | `INERT` |
| Evaluator identity | matched in the retained v2 scorecards; differs under the current v3 compatibility replay |
| Fixture binding | `sha256:80c4d0e983b8f750cea0114ad29f9c7036e35ab5e75206a9cf346d21da01642f` |

Evidence:

- live scorecard: `docs/evals/scorecards/2026-08-16/anti-ceremony-low-v2b.json`
- replay scorecard: `docs/evals/scorecards/2026-08-16/anti-ceremony-low-v2b-replay.json`
- fixture manifest and transcripts:
  `evals/skill-probes/anti-ceremony-creation-gate-v2/fixtures-low-2026-08-16-v2b/`
- probe inputs and discriminator:
  `evals/skill-probes/anti-ceremony-creation-gate-v2/`

The retained v2 replay scorecard was written before the v3 evaluator changes;
at that time it verified the canonical skill digest, four evaluation inputs,
all four transcripts, observed/requested producer fields, and the capture
evaluator's harness, preamble, dispatch helper, and metadata helper. It
reproduced the 2/2-versus-2/2 classification with
`evaluator_matches_capture: true`.

Replaying the same bound fixture with the current v3 harness still reproduces
the 2/2-versus-2/2 `INERT` classification, but now reports
`evaluator_matches_capture: false`. That is expected evaluator evolution, not a
new live result; no replacement scorecard was created from this compatibility
replay.

## Earlier and failed attempts

The first bound run used `anti-ceremony-creation-gate` and injected only its
distilled treatment prelude. Its v1 fixture binding is
`sha256:e5d15b63f7264c21f4db7e61b11a5e4a097a04a76dcee9f6c57b274ec0af6c74`
and its stored classification is also 2/2 versus 2/2 (`INERT`). The honest
replay scorecard is
`docs/evals/scorecards/2026-08-16/anti-ceremony-low-v1-replay.json`; it labels
the treatment `injected-prelude` and discloses evaluator drift. The immutable
earlier `anti-ceremony-low.json` scorecard is deprecated because its
loaded-skill wording exceeds what that v1 fixture binds. This prelude-only run
does not count as canonical skill evidence.

The first v2 live attempt is retained as
`docs/evals/scorecards/2026-08-16/anti-ceremony-low-v2.json`. It is
`UNMEASURED`: control produced 2/2 usable responses, both treatment dispatches
failed, and no fixture set was published. The failure exposed a shared runner
bug in which a prompt beginning with YAML's `---` was parsed as CLI options.
The runner now inserts an end-of-options marker, with a regression test; the
successful v2 run used a new immutable fixture and scorecard name.

## Interpretation boundary

The retained historical claim is only: in these four v2 canonical-skill-bound
responses at the recorded configuration, including the operationalize skill
did not change the old scored dual decision. No v3 capture was run: this is a
meta-tier probe that gates no named feature or product/judgment coverage
denominator, so manufacturing replacement evidence would be ceremony rather
than capability work. Claims about weaker producers, harder artifact decisions,
automatic skill activation, production outcomes, or broader skill value remain
unmeasured.
