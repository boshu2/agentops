# Research Skill Self-Test

## Trigger Cases

- User says: `/research "authentication system"` (or any `/research <topic>`).
  - Expected: load `research`, create `.agents/research/`, search prior art first, then dispatch an explore agent.

- User says: "investigate how the cache layer works and write up the findings."
  - Expected: load `research` and produce a cited `.agents/research/YYYY-MM-DD-<slug>.md` artifact.

- User says: `/research "payment processing flow" --auto`.
  - Expected: load `research` and run the full workflow without the Gate-1 human approval step.

## Non-Trigger Cases

- User asks to implement or change code directly with no investigation request.
  - Expected: do not load `research`; route to `/implement` or `/plan`.

- User asks for session/handoff history ("what did we decide last session?").
  - Expected: use `/recover`, not `research` — `research` reads git commit history, not session history.

## Behavior Checks

These map to the four scenarios in [references/research.feature](references/research.feature):

- Prior art is searched before fresh exploration: `ao inject`/`lookup` plus the `.agents/` knowledge dirs run first, and applicable learnings are cited in the output (not just loaded passively).
- An explore agent is actually dispatched (not merely described) using the detected backend, and it uses iterative retrieval — score results, extract new terms from high-relevance hits, refine over up to 3 cycles.
- Findings are written to `.agents/research/YYYY-MM-DD-<slug>.md`, and every claim carries a `file:line` citation.
- Interactive runs request human approval (Gate 1) before reporting completion; `--auto` proceeds without the gate.

## Validation Commands

Run from the repo root:

```bash
bash skills/heal-skill/scripts/heal.sh --strict skills/research
bash scripts/validate-skill-frontmatter.sh --strict
```

## Failure Cases

- Explore agent only described, never dispatched: re-run and dispatch the agent (or perform the exploration inline if no spawn backend is available) — see the Key Rules in `SKILL.md`.
- Findings written without `file:line` citations: fail the artifact and re-cite every claim before reporting.
- Missing reference file linked from `SKILL.md`: fail heal validation and restore the file or remove the link.
