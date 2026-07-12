---
name: scaffold
description: Stamp project/component/CI scaffolds — but
---
# Scaffold Skill

> **Quick Ref:** Domain-slice manifests (the repo binding) + generic project/component/CI scaffolds. `$scaffold domain <name>` for a scoped operating-loop slice; `$scaffold <language> <name>`, `$scaffold component <type> <name>`, `$scaffold ci <platform>` for the generic modes.

Stamp real project, component, or CI boilerplate plus its executable verification surface.

**YOU MUST EXECUTE THIS WORKFLOW. Do not just describe it.** Generate real files, run real commands, verify real output.

## Critical Constraints

- Snapshot `git status`, resolve the target root, and declare the exact write scope before generation. **Why:** scaffold must not absorb unrelated user changes or write outside the requested boundary.
- Require explicit authorization before `--force`, overwriting, deleting, or replacing any existing path; stop on overlap with pre-existing edits. **Why:** generated convenience never outranks user-owned work.
- Use the current agent and local shell; do not start alternate runtimes or orchestration substrates unless the user explicitly requested them. **Why:** scaffolding is a bounded write operation, not automatic permission to fan out.
- Run the target's build, behavioral test, and lint contract; commit only generated paths after they pass. Never push. **Why:** a tree is not a usable scaffold until its executable acceptance surface is green and reviewable.
- `WARN|FAIL|REFUTED -> AUTO-REDO`: consult the pawl, repair generated output, and rerun the failed validator on the same requested scaffold. **Why:** validator findings are loop evidence, not an andon by themselves.
- `BREAKER -> HOLD -> ONE-HELPER`; `HELPER-UNSTUCK -> AUTO-REDO`. Hold writes and use one bounded local-shell helper to inspect path conflicts, permissions, or missing toolchains. **Why:** one recovery pass can restore progress without masking a true boundary stop.
- `HELPER-ESCALATE -> HUMAN`; `REFUSAL-LANE|EXPLICIT-JUDGMENT|EXHAUSTED-BUDGET -> HUMAN`. **Why:** only an unresolved overwrite decision, unavailable authority, explicit judgment, or exhausted recovery earns the human andon.

## Modes

| Mode | Invocation | Output | Where |
|------|-----------|--------|-------|
| **Domain-Slice** | `$scaffold domain <name>` | Domain-slice manifest for a scoped operating-loop run | **this file** (repo binding) |
| **Project** | `$scaffold <language> <name>` | Full project directory with build, test, lint | [references/generic-templates.md](references/generic-templates.md) |
| **Component** | `$scaffold component <type> <name>` | New module/package added to existing project | [references/generic-templates.md](references/generic-templates.md) |
| **CI** | `$scaffold ci <platform>` | CI/CD pipeline configuration | [references/generic-templates.md](references/generic-templates.md) |

Parse the invocation: `domain` first-positional → Domain-Slice; `component` → Component; `ci` → CI; otherwise Project. If ambiguous, ask ONE clarifying question, then proceed.

## Generic scaffolding (project / component / CI)

**A frontier model needs no template for standard project trees, best-practice config, or GitHub-Actions / GitLab-CI YAML — ask it directly.** State the language, type, and name; it produces an idiomatic go/python/node/rust/react tree with real (not placeholder) files, a passing test, and CI, then verifies build/test/lint and makes the `bootstrap(<name>): …` commit. The four-step spine is **gather → generate → verify → commit** (do not push).

The canonical tree shapes, `.editorconfig`/pre-commit/CI YAML skeletons, verification-command table, per-mode component layouts, and the error-recovery + output-summary blocks the skill historically stamped are preserved verbatim in **[references/generic-templates.md](references/generic-templates.md)** — consult it only when you want those exact shapes. For installer scripts, agent-facing tool servers, MCP surfaces, or Rust CLI storage scaffolds, apply [references/agent-facing-tool-scaffolds.md](references/agent-facing-tool-scaffolds.md) before writing files.

## Domain-Slice Mode

When invoked as `$scaffold domain <name>`, scaffold a **domain-slice manifest** — the bounded-context declaration used to scope an operating-loop run.

> There is **no `scaffold` subcommand on the `ao` CLI**. Domain-slice scaffolding is this skill's responsibility; the old phased-engine flags are superseded by ADR-0009.

### Workflow

1. **Generate the manifest.** Run the write-and-exit flag — it creates the template and returns without starting an RPI run:

   Run `$scaffold domain <name>`.

   This writes `docs/domains/<name>/manifest.yaml` from a template that already validates against `schemas/domain-slice-manifest.v1.schema.json`. An existing manifest is **not** overwritten unless `--force` is passed.

2. **Fill in the placeholders.** Edit the generated manifest:
   - `bounded_context` — one sentence: what this slice owns and explicitly does NOT own.
   - `directive_ids` — stable GOALS.md directive IDs (pattern `d-<slug>`) this slice owns.
   - `scenario_ids` — promoted spec scenario IDs from `spec/scenarios/` (may stay `[]` initially).
   - `context_roots` — repo-relative implementation surface (at least one entry).
   - `allowed_read_globs` / `denied_read_globs` — the read fence (gitignore syntax; deny wins).
   - `validation_commands` — ordered build/test/lint steps.

3. **Verify it loads.** The scaffolded manifest already passes the F3.1 schema/loader. After editing, confirm it still validates:

   Dry-run the operating-loop plan against `docs/domains/<name>/manifest.yaml` before execution.

   A dry run loads the manifest, prints the scoped phase prompts, and exits — proving the slice attaches.

4. **Run scoped RPI.** Once the manifest is real:

   Run the operating loop with `docs/domains/<name>/manifest.yaml` as the explicit scope contract.

   Phase prompts carry the slice's boundaries; each run also writes a domain-scope audit artifact reporting any out-of-domain references visible in evidence.

### Next commands the scaffold names

After writing the manifest, lint executable-spec links with `ao goals scenarios --lint`, preview the scoped operating-loop plan, then execute with the manifest as the scope contract. Run them in that order.

Error-recovery and output-summary conventions (shared with the generic modes) live in [references/generic-templates.md](references/generic-templates.md).

## Output Specification

**Artifact directory:** generated files stay under the declared target root; write the durable handoff to `.agents/evidence/scaffold/<run-id>/` at the invocation root.
**Filename convention:** required `receipt.json`; Domain-Slice mode additionally produces `docs/domains/<name>/manifest.yaml`; other filenames follow the selected scaffold mode.
**Serialization/schema format:** `receipt.json` is JSON with `schema_version: 1`, `mode` (`domain|project|component|ci`), nonempty `target_root`, string arrays `files_created`/`files_modified`, a `validation` array of `{kind,command,exit_code}` covering `build`, `test`, and `lint`, optional string/null `commit`, verdict `PASS|WARN|FAIL`, and nonempty `next_action`.
**Validator command:** with `OUT=.agents/evidence/scaffold/<run-id>`, run `jq -e '. as $r | .schema_version==1 and (["domain","project","component","ci"]|index($r.mode))!=null and ($r.target_root|type=="string" and length>0) and ($r.files_created|type=="array" and all(.[]; type=="string")) and ($r.files_modified|type=="array" and all(.[]; type=="string")) and (($r.files_created|length)+($r.files_modified|length)>0) and ($r.validation|type=="array" and length>0 and all(.[]; (.kind as $kind | (["build","test","lint"]|index($kind))!=null) and (.command|type=="string" and length>0) and (.exit_code|type=="number"))) and ((["build","test","lint"]-[$r.validation[].kind])|length==0) and (($r.commit==null) or ($r.commit|type=="string")) and (["PASS","WARN","FAIL"]|index($r.verdict))!=null and ($r.verdict != "PASS" or all($r.validation[]; .exit_code == 0)) and ($r.next_action|type=="string" and length>0)' "$OUT/receipt.json"`.
**Downstream handoff:** pass the receipt path, declared target root, generated file list, validation evidence, commit (if any), verdict, and next action to the consuming operating-loop skill; a FAIL re-enters scaffold through the pawl.

## Quality Checklist

- [ ] The generated paths equal the declared write scope and preserve pre-existing changes.
- [ ] Files contain real behavior and at least one behavioral test—no placeholder-only green.
- [ ] Build, tests, and lint are recorded with actual exit codes in `receipt.json`.
- [ ] Domain manifests validate against `schemas/domain-slice-manifest.v1.schema.json` and retain their read fence.
- [ ] Negative verdicts consult the pawl before any human andon; commits exclude unrelated paths and are never pushed.

## References

- [references/agent-facing-tool-scaffolds.md](references/agent-facing-tool-scaffolds.md)
- [references/recommended-reading.md](references/recommended-reading.md) — forward-looking index of external skills (e.g., `mcp-server-design`) worth absorbing into scaffold when their trigger conditions arrive. Consult before designing a new scaffold mode that targets agent-facing tool surfaces.
- [references/scaffold.feature](references/scaffold.feature) — Executable spec: project/component/CI scaffolding entry points + domain-slice manifest routing (soc-qk4b)
