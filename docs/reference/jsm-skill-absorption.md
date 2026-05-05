# JSM Skill Absorption Matrix

This matrix records the Bushido standalone skills managed by JSM on 2026-05-05 and their AgentOps disposition. The absorption rule is pattern-only: no external source text is copied into AgentOps, and absorbed references carry source footers.

## Disposition Summary

| Slice | Count | Landing |
|---|---:|---|
| Testing and quality proof | 4 | `skills/test/` |
| Codebase research, audit, documentation | 8 | `skills/research/`, `skills/review/`, `skills/doc/` |
| Bug, debugger, perf, refactor, system tuning | 7 | `skills/bug-hunt/`, `skills/perf/`, `skills/refactor/`, `skills/system-tuning/` |
| Planning, beads, and project design | 5 | `skills/plan/`, `skills/beads/`, `skills/design/` |
| Agent coordination and command guardrails | 8 | `skills/codex-team/`, `skills/scope/` |
| Release, dependency, scaffold, remote validation | 13 | `skills/release/`, `skills/deps/`, `skills/scaffold/`, `skills/validation/` |
| Total | 45 | Shared skills plus `skills-codex/` mirrors |

## Matrix

| JSM skill | Slice | AgentOps disposition |
|---|---|---|
| `agent-fungibility-philosophy` | Coordination | Absorbed into `codex-team/references/fungible-agent-coordination.md`. |
| `agent-mail` | Coordination | Absorbed as coordination-file and file-reservation rules in `codex-team`. |
| `bd-to-br-migration` | Tracker migration | Absorbed into `beads/references/tracker-migration-and-triage.md`. |
| `beads-br` | Tracker migration | Absorbed as compatibility/triage guidance; bd remains canonical. |
| `beads-bv` | Tracker migration | Absorbed as graph-aware triage guidance; no new runtime dependency. |
| `beads-workflow` | Planning | Absorbed into `plan/references/plan-to-beads-workflow.md`. |
| `cc-hooks` | Guardrails | Absorbed into `scope/references/command-approval-and-hook-guardrails.md`. |
| `codebase-archaeology` | Research | Absorbed into existing research archaeology refs and `source-discovery-and-pattern-extraction.md`. |
| `codebase-audit` | Review | Absorbed into `review/references/audit-and-mock-sweeps.md`. |
| `codebase-pattern-extraction` | Research | Absorbed into `research/references/source-discovery-and-pattern-extraction.md`. |
| `codebase-report` | Docs/research | Absorbed into `doc/references/prose-and-report-workmanship.md` and research reporting guidance. |
| `dcg` | Guardrails | Already absorbed by destructive command guard patterns; extended with command approval guidance. |
| `de-slopify` | Docs/refactor | Absorbed into doc workmanship and refactor simplification guidance. |
| `deadlock-finder-and-fixer` | Bug hunt | Absorbed into `bug-hunt/references/deadlock-and-hang-triage.md`. |
| `dsr` | Release | Absorbed as publisher-boundary guidance in release preflight; no product-specific command copied. |
| `extreme-software-optimization` | Performance | Absorbed into `perf/references/optimization-proof-loop.md`. |
| `gdb-for-debugging` | Bug hunt | Absorbed into `bug-hunt/references/debugger-attach-triage.md`. |
| `gh-actions` | Release/CI | Already absorbed into release GitHub Actions CI and release automation refs. |
| `gh-cli` | Release/GitHub | Absorbed as inspection/publisher-boundary guidance; GitHub plugin remains primary. |
| `installer-workmanship` | Scaffold | Absorbed into `scaffold/references/agent-facing-tool-scaffolds.md`. |
| `library-updater` | Dependencies | Absorbed into `deps/references/library-update-ratchet.md`. |
| `mcp-server-design` | Scaffold | Absorbed into agent-facing tool scaffold rules. |
| `mock-code-finder` | Review/test | Absorbed into audit and mock sweep guidance. |
| `multi-pass-bug-hunting` | Bug hunt | Absorbed into multi-pass, audit-fix-rescan, and convergence refs. |
| `ntm` | Coordination | Absorbed into fungible coordination; no tmux manager dependency added. |
| `profiling-software-performance` | Performance | Absorbed into `perf/references/profiling-playbook.md`. |
| `rch` | Validation | Absorbed into `validation/references/remote-and-multi-repo-validation.md`. |
| `reality-check-for-project` | Design | Absorbed into `design/references/project-reality-check.md`. |
| `release-preparations` | Release | Absorbed into release cadence and preflight publisher guidance. |
| `repeatedly-apply-skill` | Coordination | Absorbed as repeated-pass stop conditions in fungible coordination. |
| `research-software` | Research | Absorbed into source discovery and software research output rules. |
| `ru-multi-repo-workflow` | Release/validation | Absorbed into release preflight and remote/multi-repo validation. |
| `rust-cli-with-sqlite` | Scaffold | Absorbed as local-state CLI scaffold guidance. |
| `rust-crates-publishing` | Release | Absorbed as registry publisher guidance. |
| `simplify-and-refactor-code-isomorphically` | Refactor | Absorbed into behavior-preserving simplification guidance. |
| `slb` | Guardrails | Absorbed into command approval and hook guardrails. |
| `ssh` | Validation | Absorbed as remote validation rules; no generic SSH skill added. |
| `system-performance-remediation` | System tuning | Already absorbed into `system-tuning`; perf host-pressure guidance mirrors the diagnostic split. |
| `testing-conformance-harnesses` | Testing | Absorbed into `test/references/conformance-harnesses.md`. |
| `testing-fuzzing` | Testing | Absorbed into `test/references/fuzzing.md`. |
| `testing-golden-artifacts` | Testing | Absorbed into `test/references/golden-artifacts.md`. |
| `testing-real-service-e2e-no-mocks` | Testing | Absorbed into `test/references/real-service-e2e.md`. |
| `ubs` | Review | Absorbed as external-review reconciliation in audit and mock sweeps. |
| `vercel` | Release | Absorbed as hosted deploy/publisher guidance; no Vercel-specific dependency added. |
| `vibing-with-ntm` | Coordination | Absorbed into fungible coordination and repeated-pass orchestration. |

## Guardrails

- Keep JSM-managed runtime installs user-local.
- Do not add standalone skills unless AgentOps has a durable product surface for them.
- Attribute every pattern-only reference with a footer.
- Prefer shared skill references plus `skills-codex/` mirrors over hidden `.agents/` notes.
