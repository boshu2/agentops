# Skill Quality Rubric

A good skill is conformant, safe for its declared effects, and demonstrably
improves the work it claims to help with. Package polish is useful evidence,
but it cannot compensate for a failed safety gate or missing behavioral proof.

This repository therefore reports three independent results. Do not collapse
them into one average:

1. **Conformance gate:** the package satisfies the open Agent Skills contract,
   the selected host profile, and repository structural checks.
2. **Safety gate:** every bundled file and declared effect has been reviewed;
   privileges, writes, network access, credentials, destructive actions,
   external content, approval points, bounds, and cleanup match the stated job.
3. **Effectiveness evidence:** representative tasks show correct activation and
   better observable outcomes than the no-skill or previous-version baseline.

A failed gate is `FAIL`. A gate or required evidence layer that was not actually
checked is `NOT_PROVEN`, never an inferred pass. An overall quality `PASS`
requires both gates to pass and effectiveness level E2 or E3 below.

### Gate decision rules

Portable conformance is evaluated against the Agent Skills specification before
any host profile. `PASS` requires only the six specified top-level frontmatter
keys (`name`, `description`, `license`, `compatibility`, `metadata`, and
experimental `allowed-tools`), all field constraints, string-to-string
`metadata`, a space-separated `allowed-tools` string when present, valid
relative resource links, and a loadable body. A package that
passes its repository schema but uses additional host fields is
`FAIL — HOST_EXTENDED` for the portable gate, not portable `PASS`. Report the
repository/host profile separately so an extension cannot hide a portable
failure.

The static safety screen uses these severity rules:

- `FAIL`: a concrete high-impact path permits destructive storage, external
  transmission, arbitrary process/code execution, or credential/secret
  persistence without the required approval, containment, bounds, and cleanup;
  or declared effects materially contradict implemented behavior.
- `WARN`: a concrete lower-impact or local-only confidentiality, retention,
  bound, cleanup, or contract gap exists, but the inspected package does not
  expose a direct uncontrolled high-impact path.
- `NOT_PROVEN`: consequential enforcement lives outside the package and its
  runtime controls were not attested.
- `PASS (static)`: every tracked package file was inspected and no concrete gap
  was found. It is a negative static screen, never runtime-safety proof.

## Static package-readiness score

The deterministic scorer remains a cheap triage signal. It measures visible
package properties only; it evaluates neither the safety gate nor skill
effectiveness. Each mechanically inspectable category receives 0–3:

| Score | Meaning |
|---:|---|
| 0 | Missing and required for this skill's declared behavior |
| 1 | Weak or no visible evidence; necessity is not inferred |
| 2 | Solid visible evidence for the declared scope |
| 3 | Mechanically strong or unusually complete |

| Category | What the static scorer looks for |
|---|---|
| Trigger quality | Description presence plus literal `Triggers:`/`Use when` and false-positive-boundary phrases. |
| Kernel clarity | `SKILL.md` line count and Markdown heading count only. |
| Progressive disclosure | Literal `references/` file/link counts, or a concise kernel with no references. |
| Helper scripts | Literal `scripts/` contents and recognizable validation/helper names; absence is uncertainty, not proof that none are needed. |
| Validation | Validation/check/test keywords, recognizable helper names, and `SELF-TEST.md` presence. |
| Self-test | `SELF-TEST.md` or `.feature` presence, plus trigger/non-trigger/failure words in `SELF-TEST.md`. |
| Assets/templates | Literal `assets/` contents only; this legacy category does not discover templates stored elsewhere. |
| Subagents/roles | Literal `subagents/` contents only; absence does not establish that delegation is unnecessary. |
| Safety boundaries | Boundary words are visible. This is a signal only, not the safety gate. |
| Packaging | File count, symlink absence, and executable bits on literal `scripts/` files. |

The three optional-component categories score an absent directory as 1
(`not evidenced/unknown`), never 2 (`solid`) and never 0 (`known required and
missing`). Static inspection cannot decide whether absence is a sound
simplification; reviewers must not add files merely to raise this score. The
maximum remains 30 for wire compatibility. Static readiness bands are C
(0–10), B (11–20), A (21–26), and S (27–30). A lower band is a review signal,
not a ship blocker; a high band is not an effectiveness or safety claim.

These category names are legacy labels for the exact mechanical proxies above.
They do not establish semantic trigger quality, a bounded procedure, conditional
loading, executable validation, or host/projection conformance.

## Effectiveness evidence levels

| Level | Required evidence |
|---|---|
| E0 | No skill-specific behavioral scenario or receipt was located. |
| E1 | A scenario, feature, or self-test exists, but there is no current baseline comparison proving outcome delta. |
| E2 | On the exact skill version, representative direct, indirect, incomplete-input, should-not-trigger, and edge cases pass against an explicit no-skill or prior-version baseline. Activation and output are graded separately. |
| E3 | E2 is repeatable as a regression suite across every intended model, host surface, and realistic installed-skill catalog; traces cover relevant tool calls, guardrails, and handoffs. |

Test observable behavior, not prose presence. Record the target model and host,
skill version or content digest, dataset, grader, baseline, treatment, failures,
and date. Structural validation can establish package conformance; it cannot
establish E2.

For a corpus audit, E1 discovery includes package-local behavior-shaped
scenarios, `SELF-TEST.md`, feature files, and tests, plus repository-global tests
or evals that explicitly bind the skill slug and exercise its workflow or
bundled/runtime implementation. A labeled example or recovery playbook counts
as a scenario when it gives an explicit input or signal and an observable
outcome, regardless of filename. Illustrative or procedural prose without that
shape, and static `grep`/`artifact_contains` assertions over skill prose, do not
count. A probe whose `skill` field or measured ledger row names the slug counts
as E1 even when it is stale or directional; that limitation must be disclosed.
E0 is assigned only after this exact search scope finds none.

## Portable baseline and host profiles

The portable baseline is the [Agent Skills specification](https://agentskills.io/specification):
valid name/directory identity, a useful what-and-when description, relative
resource links, and progressive disclosure. Host rules are additional profiles,
not universal requirements. For example, Codex may impose a tighter metadata
budget or packaging surface than another compatible host.

For progressive disclosure, use the documented working limits: keep `SKILL.md`
under 500 lines and roughly 5,000 tokens, keep references one level deep, make
load conditions explicit, and add a table of contents to long reference files.
Split a skill when triggers, inputs, or success conditions materially differ.

## Current primary sources

- [OpenAI: Build skills for ChatGPT and Codex](https://learn.chatgpt.com/docs/build-skills)
  covers focused jobs, front-loaded descriptions, explicit inputs/outputs,
  progressive loading, and trigger testing.
- [OpenAI: Build skills for plugins](https://developers.openai.com/plugins/build/skills)
  specifies direct, indirect, incomplete-input, should-not-trigger, and edge
  cases and separates activation failures from output failures.
- [OpenAI API skills](https://developers.openai.com/api/docs/guides/tools-skills)
  treats skill bundles as privileged, initially untrusted code and instructions
  and requires bounded, approved high-impact actions.
- [OpenAI agent evaluations](https://developers.openai.com/api/docs/guides/agent-evals)
  describes repeatable datasets, graders, and trace inspection.
- [Anthropic skill authoring best practices](https://platform.claude.com/docs/en/agents-and-tools/agent-skills/best-practices)
  recommends a no-skill baseline, representative scenarios, observable results,
  concise kernels, and testing on every intended model.
- [Anthropic enterprise skills](https://platform.claude.com/docs/en/agents-and-tools/agent-skills/enterprise)
  covers full-bundle safety review and catalog-scale recall/coexistence tests.
- [GitHub Copilot agent skills](https://docs.github.com/en/enterprise-cloud@latest/copilot/how-tos/copilot-on-github/customize-copilot/customize-cloud-agent/add-skills)
  emphasizes provenance, pinning, preview, and dry-run validation.
- [Microsoft Agent Framework skills](https://learn.microsoft.com/en-us/agent-framework/agents/skills)
  covers sandboxing, resource limits, input allowlists, audit logs, and when a
  deterministic high-side-effect workflow is a better fit than a skill.

## Required repository checks

```bash
bash skills/skill-builder/scripts/heal.sh --check --strict skills/<slug>
bash skills/skill-builder/scripts/audit.sh --strict skills/<slug>
bash scripts/validate-skill-frontmatter.sh --strict
bash tests/docs/validate-skill-count.sh
python3 scripts/generate-skill-mesh.py --check
```

When behavior or metadata changes:

```bash
bash scripts/refresh-codex-artifacts.sh --scope worktree
bash scripts/validate-codex-generated-artifacts.sh --scope worktree
```

Marketplace export checks apply only when preparing that package. A
self-contained repo skill is not defective merely because it lacks marketplace
assets or a dedicated delegation tree.

## Audit method

1. Read the complete `SKILL.md` and every linked resource.
2. Apply the conformance and safety gates without compensation.
3. Run the portable validator and the repository/host checks separately, then
   record the static readiness score as triage.
4. Search package-local and explicitly skill-bound repository-global behavioral
   evidence, then compare declared activation and output behavior with a
   baseline and grade
   the effectiveness evidence E0–E3.
5. Test coexistence in the intended installed catalog and every target model or
   host before claiming E3.
6. Recommend the smallest change that removes an observed defect.

Never add ceremony solely to raise a numeric score.
