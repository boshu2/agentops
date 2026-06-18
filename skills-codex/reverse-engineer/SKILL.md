---
name: reverse-engineer
description: 'Reverse-engineer an external system you own or are authorized to analyze — repo, binary, or product — into a mechanically-verifiable feature inventory + spec set, then a steal-map (have/gap/steal/park/reject) onto our own surfaces. Use when evaluating a competitor, upstream, fork, or reference tool for what to adopt. Triggers: "reverse-engineer X", "tear down Y", "what should we steal from Z", "evaluate competitor/upstream", "should we fork/adopt/build-native".'
---

# reverse-engineer (Codex)

Codex-native entry point for the `reverse-engineer` operator skill.

The AgentOps source skill `../../skills/reverse-engineer/SKILL.md` is the source
of truth for domain behavior, commands, examples, references, and output
expectations. Read it first, then use `prompt.md` for the Codex runtime profile.

## Codex Runtime Contract

- Use Codex plus the local shell. Do not invoke Claude Code as an executor.
- Hard guardrails are mandatory: operate only on code/binaries you own or are
  explicitly authorized to analyze; never extract or reproduce proprietary
  source or system prompts (index only, redact); redact secrets and run the
  secret-scan gate over outputs. Separate docs-say vs code-proves vs hosted.
- Phase 1, mechanical teardown — run
  `python3 skills/reverse-engineer/scripts/reverse_engineer.py <product> --mode=repo --upstream-repo=<url> --upstream-ref=<ref> --output-dir=.agents/research/<product>/`
  (binary mode requires `--authorized`). It writes the feature inventory,
  `feature-registry.yaml`, and the spec set under the output dir.
- Phase 2, the steal-map — map each found capability onto our surfaces and emit
  `.agents/research/<product>/steal-map.md` with a verdict per row
  (have / gap / steal / park / reject); cite the teardown evidence and the
  matching surface for every row.
- Discipline: validated cross-family, not self-report; park substrate we
  delegate; steal the pattern not the platform; probe the real state before
  calling something a gap.
- One-way-door adoptions: hand the steal-map to a fanout duel or judge panel —
  produce the map, let the duel pick the route. Do not decide the fork here.
- Self-test: `bash skills/reverse-engineer/scripts/self_test.sh` (registry
  validator exits 0; security mode passes the secret scan).
- Return concrete evidence: commands run, files written, exit codes, the steal-map.
