# Codex parity — reliable-service-from-spec

This skill's `SKILL.md` is portable as-is to Claude, Gemini, and Antigravity
(install the directory; no conversion). Codex uses the dual-file form: a slim
`SKILL.md` plus a `prompt.md` that carries the full procedure. The method below is
identical across runtimes — only the invocation surface differs.

## Tool mapping (Claude → Codex)

| Claude tool | Codex equivalent |
|---|---|
| Read | shell read (`cat`/`sed`) or `rg` |
| Write | shell redirection or `apply_patch` |
| Edit | `apply_patch` |
| Bash | `shell_command` |
| Glob / Grep | `rg --files` / `rg` |

## Running the method under Codex

1. Read the spec and build the clause inventory (every testable assertion → an id).
2. Write `acceptance-examples.feature` via `apply_patch`; run the suite with
   `shell_command` and confirm it is RED before implementing.
3. Implement one clause per change; after each, run the suite + lint + type-check
   with `shell_command`.
4. Add reliability primitives (error handling, retries on idempotent ops only,
   idempotency keys, observability) — each with a scenario that fails without it.
5. Run `bash scripts/validate.sh` via `shell_command`; then run the acceptance
   suite against the running service.
6. Emit `conformance-report.md` (clause → scenario → PASS/FAIL, verdict).

## Parity guarantee

The Output Specification, Quality Rubric, and verification loop are runtime-neutral.
A Codex run and a Claude run on the same spec must produce the same artifacts
(`acceptance-examples.feature`, `conformance-report.md`) and the same ship/no-ship
verdict for a given service state.
