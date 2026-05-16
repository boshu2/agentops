# AgentOps 3.0 Domain/Practice Packet

This packet is the tracked example for the 3.0 council-first launch demo.

## Decision Or Task

Validate the first public AgentOps 3.0 demo path:

> Should AgentOps 3.0 lead with a council-first engineering judgment demo, where
> Claude and Codex judge the same product/design/engineering decision against
> the same domain and practice packet?

## Target User

Agent-heavy maintainers who already use coding agents on real repositories and
feel recurring context debt: repeated investigations, scattered warnings,
weak review evidence, and cold starts between sessions.

## Domain Sources

| Source | Why It Matters |
|---|---|
| `PRODUCT.md` | Canonical product identity, target personas, 10-star experience, and claim posture. |
| `GOALS.md` | Repo north stars, anti-stars, and gates that define what must not regress. |
| `PRACTICE-REGISTRY.md` | Practice lineage and canonical `practices: [slug]` registry for the packet's engineering citations. |
| `README.md` | Current public front door; contains stale language the 3.0 path must replace. |
| `docs/index.md` | Current documentation landing; should converge with README after proof exists. |
| `soc-m6v5.9.7` | Education-led GTM and first-value proof epic. |
| `soc-m6v5.9.7.6` | Domain/practice packet productization task. |
| `soc-m6v5.9.7.7` | Activation profile decision task. |
| `soc-m6v5.9.7.8` | `ao demo` rebuild task. |

## Practice Sources

| Source | Practice Encoded |
|---|---|
| `PRACTICE-REGISTRY.md` | Reverse-traced engineering lineage and stable practice slugs for DevOps/SRE/XP/TDD/BDD/DDD/cloud-native/distributed-systems citations. |
| `skills/council/SKILL.md` | Independent multi-judge consensus over one shared evidence packet. |
| `docs/context-packet.md` | Phase-scoped context budget and GOALS/HISTORY/INTEL/TASK/PROTOCOL briefing model. |
| `docs/standards/markdown-style-guide.md` | Public docs and launch copy must pass markdown hygiene. |
| `scripts/check-factory-claim-ledger.sh --strict --no-fixtures` | Public claims need strict evidence posture. |
| `docs/templates/schedule.yaml.example` | Optional second-stage schedule lane uses real-bodied Dream and wiki forge jobs. |

## Intent Lineage

| Anchor | Purpose |
|---|---|
| This packet | Source intent for the council-first launch demo decision. |
| `.agents/rpi/briefing-current.md` | Phase-scoped context assembled from the packet and repo state. |
| `.agents/council/<run-id>/verdict.md` | Judge output that records whether the shared intent passed, warned, or blocked. |
| `.agents/rpi/execution-packet.json` | Handoff artifact if the verdict becomes implementation work. |
| `bd create "Apply council verdict to launch demo" ...` | Tracked follow-up work created from the verdict. |

## Evidence Rules

- The demo must show the packet before the verdict.
- The council must judge the same decision against the same packet across
  Claude and Codex when mixed mode is available.
- The consolidated verdict should land in `.agents/council/` for local work.
- Any public claim about PMF, productivity, or user outcomes needs exported
  evidence under `docs/releases/` or `evals/workbench/results/`.
- The daemon/schedule lane is proof of deeper automation, not a prerequisite
  for first value.

## Runtime Path

Inspect current packet context:

```bash
ao context packet --goal "AgentOps 3.0 council-first launch demo"
```

Build a planning briefing:

```bash
ao context assemble \
  --phase planning \
  --task "Evaluate the AgentOps 3.0 launch demo against the domain/practice packet" \
  --output-file .agents/rpi/briefing-current.md
```

Run council:

```text
/council --mixed validate "Given docs/examples/agentops-3-domain-practice-packet.md, should the 3.0 launch demo lead with council-first engineering judgment?"
```

Optional second-stage automation:

```bash
ao init --with-schedule
ao daemon run --schedule-file .agents/schedule.yaml
```

## Expected Outputs

| Artifact | Purpose |
|---|---|
| `.agents/rpi/briefing-current.md` | Phase briefing assembled from repo context. |
| `.agents/council/<run-id>/verdict.md` | Consolidated council verdict. |
| `.agents/rpi/execution-packet.json` | Discovery/implementation handoff when the verdict becomes work. |
| `docs/releases/<release>/pmf-scenario.md` | Exported evidence for public claims, when available. |

## Non-Goals

- Do not lead with daemon/factory setup as the first proof.
- Do not frame the product as generic multi-agent orchestration.
- Do not use stale reliability-framed agent copy as public launch language.
- Do not claim PMF or productivity improvements from local `.agents` notes
  alone.

## Pass Criteria

The demo path passes when a maintainer can see:

1. The domain and practice context the agents share.
2. Claude and Codex judging one decision against that shared context.
3. A consolidated verdict artifact.
4. A validation or follow-up path tied to bd issues.
5. An optional schedule path that compounds later, after trust exists.
