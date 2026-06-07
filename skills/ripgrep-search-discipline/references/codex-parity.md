# Codex / cross-vendor parity — rg-optimized

This skill is a CLI-reference contract, not an interactive procedure, so the same
`SKILL.md` content is portable across harnesses. Install path differs by vendor;
the *content* does not get converted.

## Claude / Gemini / Antigravity

Install the directory as-is (`skills/rg-optimized/`). `SKILL.md` is the single
source; the `description` block (with its `Triggers:` clause) is what the model
uses to select the skill. No conversion needed.

## Codex (dual-file form)

Codex expects a slim `SKILL.md` plus a `prompt.md`. For a pure reference contract
like this one, the slim file is the same frontmatter + the Flag Reference and
Strategy Decision Tree tables; the `prompt.md` carries the Critical Constraints
and Robot Mode sections verbatim. No behavioral content changes — only the file
split. Generate with the factory's parity stage; do not hand-fork the rules, or
the two copies will drift.

## What stays identical across vendors

- The flag semantics (`-t`, `-g`, `-A/-B/-C`, `-U`, `-o`, `-F`, `-w`, `--json`).
- The exit-code contract (`0` match / `1` no-match / `2` error).
- The "narrow before you read bodies" strategy reflex.

These are properties of the `ripgrep` binary, not of any harness, so parity is
about install mechanics, not content translation.
