# idea-wizard — THE EXACT PROMPT blocks

Copy-paste prompt blocks for each phase, with iteration round-counts. Run them in order;
each block is self-contained. Replace `<PROJECT>` and `<AXIS>` once at the top.

## Round counts (at a glance)

| Phase | Prompt block | Rounds |
|---|---|---|
| 2. Generate | GENERATE | 1 (re-run once if < 25 ideas) |
| 3. Think | THINK | 1 |
| 4. Winnow | WINNOW | 1 |
| 5. Expand | EXPAND | 1 |
| 6. Overlap | OVERLAP | 1 (after running `br list`) |
| 7. Operationalize | OPERATIONALIZE | 1 per NEW survivor |
| 8. Refine | REFINE | 2–4 (repeat until a pass changes nothing) |

## GENERATE — THE EXACT PROMPT

```text
Project: <PROJECT>. Improvement axis: <AXIS>.
Generate exactly 30 candidate improvements to this project along that axis.
Rules:
- Number them 1–30.
- Do NOT evaluate, rank, or comment on any of them yet.
- Cover the system end to end (entry points, core, edges) and include 3–4
  deliberately ambitious candidates and a few from adjacent axes.
- No two candidates may be rewordings of the same idea.
Output: the numbered list only.
```

If the result has fewer than ~25 distinct ideas, re-run with: "These overlap too much —
regenerate, forcing each to touch a different part of the system."

## THINK — THE EXACT PROMPT

```text
For each of the 30 candidates above, add exactly three short lines:
- Upside: <what improves and roughly how much>
- Cost:   <effort / blast radius / who's affected>
- Risk:   <what could go wrong or is uncertain>
Keep each line to one clause. Do not pick winners yet.
```

## WINNOW — THE EXACT PROMPT

```text
Using the upside/cost/risk notes, winnow the 30 down to the best 5.
- First state your cut criteria in one sentence (e.g. weight upside ÷ cost,
  discard unbounded-risk items, prefer reversible changes).
- Then list the 5 survivors.
- Then list every cut idea with a one-clause reason it was cut (the graveyard).
```

## EXPAND — THE EXACT PROMPT

```text
Expand each of the 5 survivors into ~3 concrete, file-able sub-ideas (~15 total).
Each sub-idea must be small enough to become one tracked issue and to verify.
Output as: <survivor> → [sub-idea a, sub-idea b, sub-idea c].
```

## OVERLAP — THE EXACT PROMPT

First gather the existing tracked work, then run the block:

```bash
br list
br ready
```

```text
Here is the existing tracked work: <paste br list / br ready output>.
For each of the ~15 sub-ideas, classify it as NEW, MERGE, or DROP:
- NEW:   nothing covers it.
- MERGE: an existing issue (give its id) covers part of it; say what context to add.
- DROP:  already fully covered (give the id).
Output a table: sub-idea | decision | existing id (if any) | note.
```

## OPERATIONALIZE — THE EXACT PROMPT (run once per NEW sub-idea)

```text
Write a self-documenting tracked issue for this sub-idea: <sub-idea>.
The body must let a cold reader act without this conversation. Use this template:

  Problem:   <what's wrong / missing today>
  Why:       <why it matters along the axis>
  Done when: <the concrete acceptance check>

Then give me the exact br commands to:
1. create the improvement issue with that body,
2. create a child "Test: verify <sub-idea>" issue naming HOW we prove it landed,
3. wire the test to depend on the improvement,
4. wire any prerequisite-improvement dependencies.
```

The issue-body template the prompt produces:

```text
Problem:   <what's wrong or missing today>
Why:       <why it matters along the chosen axis>
Done when: <the concrete, checkable acceptance condition>
```

The commands it produces follow this shape:

```bash
br create "Improve <X>: <one-line outcome>" -d "$(printf 'Problem: ...\nWhy: ...\nDone when: ...')"
br create "Test: verify <X> landed" -d "How we prove it: <measurement / assertion / manual check>"
br dep add <test-id> <improvement-id>          # test depends on the improvement
br dep add <dependent-id> <prerequisite-id>    # chain prerequisite improvements
```

## REFINE — THE EXACT PROMPT (repeat 2–4 rounds)

```text
Here is the current filed backlog: <paste br list>.
Do one refinement pass, in plan space only (no code):
- Split any issue too big to land in one move.
- Merge near-duplicates.
- Fix the dependency order so `br ready` surfaces the right first move.
- Confirm each issue is still self-documenting.
Tell me exactly what changed this pass. If nothing changed, say STABLE.
```

Repeat REFINE until it returns STABLE. That is the exit condition — only then leave plan
space and begin implementation.
