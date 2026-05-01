# Shared Task Notes - Epic soc-o6eb

> Cross-wave context for workers. Read before starting. Report discoveries in task output.
> Maintained by the crank orchestrator - workers do not write to this file directly.

## Initial Context

- W0 and W1 are closed. W1 artifact:
  `.agents/triage/2026-05-01-w1-issue-graph-normalization.md`.
- Keep artifact paths replayable; closure evidence must point to files that are
  committed in this repo, not ephemeral scratch paths.
- Avoid `ao metrics cite` and dry-run mutation paths unless followed by
  `git status --short`; W0 found citation/dry-run surfaces can dirty tracked
  finding metadata.
- Do not mass-reparent implementation beads under `soc-o6eb`. W2/W3/W4 close by
  recording execution decisions and tracker notes, not by absorbing all 142 open
  issues into this portfolio epic.

## Wave 1 (2026-05-01T19:46:01Z)

- W2: The plan paths referenced for `soc-b8jo`, `soc-eh1z`, and `soc-8412`
  are absent in this checkout. Routing used live bd issue bodies plus committed
  docs, scripts, and tests instead.
- W2: Raw ready order would pick `soc-6wuw` and `soc-lmoq` before
  `soc-b8jo.1`, but W1 explicitly selected scheduler-first routing.
- W2: `soc-eh1z` has one open child, `soc-0pzj`; classify the PR #204
  eval-advisory failure before routing `soc-v7s8`.
- W3: `psite-agu.9` is closed, but it delegates `psite-355` supersedence to
  `psite-agu.5`; keep `psite-355` live until Morai daemon cutover is proven.
- W3: Do not merge or close `soc-ygo.38` and `soc-q4c.6` without concrete Tier
  2/Tier 3 starvation evidence.
- W4: `.agents/crank/results/` had to be created for worker result artifacts.
- W4: `soc-23m2` still has feature/epic graph-shape drift; do not normalize it
  through mass reparenting during portfolio routing.
- W4: `soc-957` has partial implementation evidence but remains blocked by
  `soc-ylb`; do not route it as ready execution.
- W4: Treat `soc-5tky` as security-sensitive despite P2 priority because it
  phases out empty-password remote Dolt root access.
