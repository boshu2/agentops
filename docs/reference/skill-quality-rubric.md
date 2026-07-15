# Skill Quality Rubric

This advisory rubric identifies concrete hardening opportunities. It does not
change a deep-audit verdict and must not reward unnecessary references,
scripts, assets, self-tests, or subagent packets.

## Scoring

Each category receives 0–3:

| Score | Meaning |
|---:|---|
| 0 | Missing and required for this skill's actual behavior |
| 1 | Present but weak, or warranted but absent |
| 2 | Solid for the skill's scope, including “not needed” |
| 3 | Mechanically strong or unusually complete |

| Category | What good means |
|---|---|
| Trigger quality | Description says what, when, and avoids obvious false positives. |
| Kernel clarity | The bounded procedure and stop condition are easy to find. |
| Progressive disclosure | A concise kernel is self-contained; complex detail is linked. |
| Helper scripts | Repeated deterministic mechanics are scripted; judgment is not. |
| Validation | Evidence commands or artifacts prove executable behavior. |
| Self-test | Trigger or behavior examples exist when complexity warrants them. |
| Assets/templates | Reusable payloads exist only when the workflow actually needs them. |
| Subagents/roles | Delegation packets exist only for intentionally delegated work. |
| Safety boundaries | Mutation, authorization, and non-goals are explicit where relevant. |
| Packaging | The package is small, linked, mode-correct, and projection-safe. |

The maximum remains 30. Rating bands are C (0–10), B (11–20), A (21–26), and
S (27–30). A lower advisory rating is a review signal, not a ship blocker.

## Required repository checks

```bash
bash skills/heal-skill/scripts/heal.sh --check --strict skills/<slug>
bash skills/heal-skill/scripts/audit.sh --strict skills/<slug>
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
2. Compare its declared trigger, boundaries, output, and evidence with actual
   behavior.
3. Run the repository checks.
4. Score optional package features relative to demonstrated need.
5. Recommend the smallest change that removes a real defect.

Never add ceremony solely to raise a numeric score.
