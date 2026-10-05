<!-- generated from skills/*/SKILL.md metadata -->

# Skill Router

Choose guidance for a concrete task need; no skill is mandatory.
A clear task can proceed in the native agent. Read a skill only when its description fits.
Names and descriptions below come from each source SKILL.md; explicit invocation remains available.

## Primary entrypoints

These are independent choices, not a required sequence.
Advisory review does not replace Validate's fresh acceptance judgment.

| Skill | Use it for |
|---|---|
| [plan](https://github.com/boshu2/agentops/blob/main/skills/plan/SKILL.md) | Shape a request into one end-to-end slice with observable behavior; review write scope and reversible decisions. Use when: planning, breaking down or scoping a change. |
| [implement](https://github.com/boshu2/agentops/blob/main/skills/implement/SKILL.md) | Change or repair code, config or services without weakening tests; report what ran and what did not. Use when: implementing a change or fixing a defect. |
| [review](https://github.com/boshu2/agentops/blob/main/skills/review/SKILL.md) | Give advisory feedback on a plan, design or code change. Use when: asked for an opinion or a look-over, even informally. Not for acceptance; use Validate. |
| [validate](https://github.com/boshu2/agentops/blob/main/skills/validate/SKILL.md) | Freshly judge whether a finished change and its claims meet original acceptance: PASS, FAIL or NOT_PROVEN. Use when: asked for a go/no-go, sign-off or independent verdict. |
| [orchestrate](https://github.com/boshu2/agentops/blob/main/skills/orchestrate/SKILL.md) | Coordinate several workers: what idle agents do next, which finished work gets checked first, how to recover a dead one. Use when: managing multiple agents. |
| [memory](https://github.com/boshu2/agentops/blob/main/skills/memory/SKILL.md) | Write, find or curate lessons and agent rules with stated evidence and limits. Use when: asked to remember something or write a rule into agent instructions. |

## Engineering specialists

| Skill | Use it for |
|---|---|
| [doc](https://github.com/boshu2/agentops/blob/main/skills/doc/SKILL.md) | Write or update READMEs, docs, repo instructions and handoff notes, checked against source. Use when: documenting something, writing a README or leaving a session handoff. |
| [domain](https://github.com/boshu2/agentops/blob/main/skills/domain/SKILL.md) | Settle what domain terms mean per context, and which repository conventions or language standards (Go, Python) apply. Use when: names disagree or a rename is proposed. |
| [refactor](https://github.com/boshu2/agentops/blob/main/skills/refactor/SKILL.md) | Restructure or clean up code with no behavior change, proved by before-and-after checks. Use when: asked to clean up, extract, dedupe or simplify, even one function. |
| [research](https://github.com/boshu2/agentops/blob/main/skills/research/SKILL.md) | Answer one cited question: how code works, or whether a repeated pattern deserves a rule. Use when: asked how, why, or whether to enforce a pattern. |
| [reverse-engineer](https://github.com/boshu2/agentops/blob/main/skills/reverse-engineer/SKILL.md) | Tear down a competitor's repo or product into a feature inventory and adoption choices. Use when: comparing us to another tool or asking what to steal. |
| [security](https://github.com/boshu2/agentops/blob/main/skills/security/SKILL.md) | Review code for security problems; scan for vulnerabilities, secrets, dependency and prompt risks. Use when: asked whether code is safe to ship, even one small handler. |
| [skill-builder](https://github.com/boshu2/agentops/blob/main/skills/skill-builder/SKILL.md) | Create, repair, audit or consolidate agent skills (SKILL.md packages). Use when: writing or fixing a skill, its description or structure. Not for one-off lessons; use Memory. |
| [skill-eval](https://github.com/boshu2/agentops/blob/main/skills/skill-eval/SKILL.md) | Measure whether a skill helps by comparing runs with and without it. Use when: reading skill A/B results or deciding to keep, revise or remove one. |
| [test](https://github.com/boshu2/agentops/blob/main/skills/test/SKILL.md) | Write or assess tests that prove behavior and would fail without the fix. Use when: writing tests, TDD, or asked whether a green test is enough. |

## Deliberate planning and review strategies

| Skill | Use it for |
|---|---|
| [council](https://github.com/boshu2/agentops/blob/main/skills/council/SKILL.md) | Compare independent opinions from several models or contexts without inflating agreement. Use when: wanting a second opinion or debate, or summarizing several reviewers' results. |
| [craft-goal](https://github.com/boshu2/agentops/blob/main/skills/craft-goal/SKILL.md) | Draft or lint a bounded long-running goal prompt with a finish line and hard limits. Use when: selected by name; one change goes to Plan. |
| [idea-genie](https://github.com/boshu2/agentops/blob/main/skills/idea-genie/SKILL.md) | Brainstorm evidence-backed options for what to build, or stress-test an idea. Use when: deciding what to build next, comparing options or testing an idea. |
| [interview](https://github.com/boshu2/agentops/blob/main/skills/interview/SKILL.md) | Interview you one question at a time, each with a recommendation, to settle a big outcome before agents work alone. Use when: selected by name. |
| [navigate](https://github.com/boshu2/agentops/blob/main/skills/navigate/SKILL.md) | Pick the next work in an epic or bead graph; closed is not proven. Use when: asked what is next or whether an epic is done. |
| [postmortem](https://github.com/boshu2/agentops/blob/main/skills/postmortem/SKILL.md) | Explain why a change, incident or session went as it did, separating proven causes from coincidence. Use when: a postmortem or retro is selected by name. |
| [premortem](https://github.com/boshu2/agentops/blob/main/skills/premortem/SKILL.md) | Find how a rollout plan could fail before committing to it. Use when: asked what could go wrong or to poke holes in a plan. |
| [reality-check](https://github.com/boshu2/agentops/blob/main/skills/reality-check/SKILL.md) | Audit claims that work is done or shipped against the diff or repo. Use when: asked whether something really got done, even if it looks obvious. |
| [rpi](https://github.com/boshu2/agentops/blob/main/skills/rpi/SKILL.md) | Drive one accepted change through implementation and checks to done, with one fresh review only where a mistake is costly. Use when: selected by name. |

## Explicit tool and runtime adapters

| Skill | Use it for |
|---|---|
| [agent-native](https://github.com/boshu2/agentops/blob/main/skills/agent-native/SKILL.md) | Dispatch independent tasks to parallel workers or subagents without write collisions. Use when: running or planning agents in parallel, even two; check scopes before any launch. |
| [agy-native](https://github.com/boshu2/agentops/blob/main/skills/agy-native/SKILL.md) | Run a supplied task in headless AGY (Antigravity, Gemini) and collect its result. Use when: AGY, Antigravity or Gemini is requested by name; never a fallback. |
| [claude-exec](https://github.com/boshu2/agentops/blob/main/skills/claude-exec/SKILL.md) | Run one prompt through headless Claude with scoped permissions and a time bound. Use when: scripting or automating a `claude -p` call, even a simple one. |
| [codex-exec](https://github.com/boshu2/agentops/blob/main/skills/codex-exec/SKILL.md) | Run one prompt through headless Codex and capture the result. Use when: wanting a one-shot `codex exec` run or CI step. Not for batches or retries. |
| [using-gc](https://github.com/boshu2/agentops/blob/main/skills/using-gc/SKILL.md) | Operate Gas City through its own doors: Mayor, doctor and native run state. Use when: Gas City is selected or a gc run looks stuck. |

## Complete inventory

| Skill | Tier | Disposition | Hard dependencies | Capabilities | Effects |
|---|---|---|---|---|---|
| `agent-native` | meta | `keep_optional_adapter` | - | `role_dispatch`, `observe_workers`, `handoff`, `dispatch_once` | `manage_runtime_sessions`, `invoke_selected_executor` |
| `agy-native` | cross-vendor | `keep_optional_adapter` | - | `dispatch_explicit_packet`, `provide_fresh_context` | `start_agy_session` |
| `claude-exec` | orchestration | `keep_optional_adapter` | - | `claude_exec` | `run_claude_process`, `permission_tiered_workspace_effects` |
| `codex-exec` | orchestration | `keep_optional_adapter` | - | `codex_exec` | `run_codex_process`, `sandbox_tiered_workspace_and_network_effects` |
| `council` | judgment | `keep_strategy` | - | `collect_independent_judgments`, `synthesize_disagreement`, `bounded_deliberation`, `duel_scored_ideas`, `answer_interview_panel` | `write_advisory_council_report` |
| `craft-goal` | judgment | `keep_strategy` | - | `goal_prompt_design`, `goal_prompt_lint` | - |
| `doc` | product | `keep_specialist` | - | `doc`, `initialize_missing_docs`, `write_session_handoff` | `write_documentation`, `write_requested_handoff`, `create_requested_evidence_directory` |
| `domain` | knowledge | `keep_specialist` | - | `domain`, `clarify_domain_language`, `reconcile_domain_names` | `update_existing_domain_contracts` |
| `idea-genie` | execution | `keep_strategy` | - | `generate_evidenced_options`, `dueling_idea_genies` | `write_idea_portfolio` |
| `implement` | execution | `keep` | - | `execute_one_experiment`, `collect_factual_evidence` | `modify_declared_subject`, `derive_subject_manifest` |
| `interview` | execution | `keep_strategy` | - | `interview_caller`, `settle_caller_choices`, `write_acceptance_examples`, `settle_domain_terms` | `update_intent_source` |
| `memory` | execution | `keep_off_path` | - | `recall_applicable_context`, `mine_supported_observations`, `curate_topic_pages`, `toil_mining` | `write_protected_drafts`, `update_authorized_topic_pages`, `write_requested_toil_report` |
| `navigate` | execution | `keep_strategy` | - | `observe_work_graph`, `select_next_wave`, `ratchet_work_graph`, `report_graph_hygiene` | `update_native_graph` |
| `orchestrate` | execution | `keep` | - | `coordinate_native_work`, `recover_assignments`, `reconcile_feedback` | `dispatch_authorized_workers`, `update_native_handoffs` |
| `plan` | execution | `keep` | - | `shape_intent`, `define_acceptance`, `bound_write_scope`, `resume_discovery` | `update_intent_source` |
| `postmortem` | judgment | `keep_strategy` | - | `postmortem` | `write_postmortem_report` |
| `premortem` | judgment | `keep_strategy` | - | `challenge_plan` | `write_advisory_plan_review` |
| `reality-check` | judgment | `keep_strategy` | - | `compare_claim_to_evidence`, `measure_declared_goals`, `report_native_status` | `write_advisory_gap_report`, `write_goal_snapshot`, `write_requested_rendered_spec` |
| `refactor` | execution | `keep_specialist` | - | `refactor` | `modify_source_files` |
| `research` | execution | `keep_specialist` | - | `research`, `codebase_recon`, `pattern_mining` | `write_research_report`, `write_recon_pack`, `write_pattern_evidence` |
| `reverse-engineer` | execution | `keep_specialist` | - | `reverse_engineer` | `clone_upstream_repo`, `authorized_binary_execution`, `write_teardown_artifacts` |
| `review` | judgment | `keep` | - | `review_advisory`, `identify_supported_findings`, `report_review_gaps` | - |
| `rpi` | meta | `keep_strategy` | `plan`, `implement`, `validate` | `own_authorized_outcome`, `report` | `dispatch_core_phases` |
| `security` | product | `keep_specialist` | - | `security` | `write_scan_artifacts` |
| `skill-builder` | meta | `keep_specialist` | - | `skill_builder`, `heal_skill`, `export_skill`, `distill_expertise` | `write_skill_source`, `write_build_report`, `regenerate_skill_projections`, `repair_skill_projections`, `write_converted_skill_projection`, `write_advisory_proposal` |
| `skill-eval` | meta | `keep_specialist` | - | `author_seeded_probe`, `run_probe_tier`, `evaluate_skill_decision` | `write_probe_package`, `dispatch_probe_producer` |
| `test` | execution | `keep_specialist` | - | `test` | `write_test_files`, `write_test_evidence`, `modify_source_files` |
| `using-gc` | execution | `keep_optional_adapter` | - | `dispatch_explicit_packet`, `observe_gc_runtime`, `inspect_pack_registries`, `drive_mayor_door` | `operate_gas_city`, `configure_codex_trust` |
| `validate` | judgment | `keep` | - | `compute_subject_identity`, `judge_acceptance`, `return_validation_result`, `persist_verdict` | `write_verdict_artifact` |
