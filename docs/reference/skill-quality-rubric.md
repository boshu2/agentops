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

## Static package-readiness score

The existing deterministic scorer remains a cheap triage signal. It measures
visible package properties only; it evaluates neither the safety gate nor skill
effectiveness. Each category receives 0–3:

| Score | Meaning |
|---:|---|
| 0 | Missing and required for this skill's declared behavior |
| 1 | Present but weak, or warranted but absent |
| 2 | Solid for the declared scope, including justified “not needed” |
| 3 | Mechanically strong or unusually complete |

| Category | What the static scorer looks for |
|---|---|
| Trigger quality | Description says what and when and includes an obvious false-positive boundary. |
| Kernel clarity | The bounded procedure and stop condition are easy to find. |
| Progressive disclosure | A concise kernel is self-contained; complex detail is directly linked and loaded only when needed. |
| Helper scripts | Repeated deterministic mechanics are scripted; judgment is not. |
| Validation | Commands or artifacts exist that can check executable behavior. |
| Self-test | Trigger or behavior examples exist when complexity warrants them. |
| Assets/templates | Reusable payloads exist only when the workflow actually needs them. |
| Subagents/roles | AgentOps-specific delegation packets exist only for intentionally delegated work. This is not a portable Agent Skills quality criterion. |
| Safety boundaries | Boundary words are visible. This is a signal only, not the safety gate. |
| Packaging | The package is small, linked, host-profile-correct, and projection-safe. |

The maximum is 30. Static readiness bands are C (0–10), B (11–20), A (21–26),
and S (27–30). A lower band is a review signal, not a ship blocker; a high band
is not an effectiveness or safety claim.

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
3. Run the repository checks and record the static readiness score as triage.
4. Compare declared activation and output behavior with a baseline and grade
   the effectiveness evidence E0–E3.
5. Test coexistence in the intended installed catalog and every target model or
   host before claiming E3.
6. Recommend the smallest change that removes an observed defect.

Never add ceremony solely to raise a numeric score.
