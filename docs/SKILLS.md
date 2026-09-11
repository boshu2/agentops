<!-- generated from skills/*/SKILL.md metadata -->

# Skill Router

34 live skills. Choose guidance for a concrete task need; no skill is mandatory.
A clear task can proceed in the native agent. Read a skill only when its description fits.
Names and descriptions below come from each source SKILL.md; explicit invocation remains available.

## Intent, implementation and final judgment

| Skill | Use it for |
|---|---|
| [implement](../skills/implement/SKILL.md) | Implement accepted behavior, repair understood defects or execute a selected wave with per-lane evidence. Use when: authorized coding work is ready; shape only consequential missing intent and return check facts. |
| [plan](../skills/plan/SKILL.md) | Describe intended behavior, review write scope and assess whether decisions are reversible. Use when: acceptance or approach is unclear before coding; reuse existing intent and stop when actionable. |
| [validate](../skills/validate/SKILL.md) | Freshly judge a finished change against original acceptance before merge. Use when: independent proof is needed; author tests cannot issue PASS. Triggers: "check this change". |

## Engineering specialists

| Skill | Use it for |
|---|---|
| [doc](../skills/doc/SKILL.md) | Write source-grounded documentation, READMEs, repo instructions or continuity handoffs. Use when: these documents are the requested output; do not create reports as a routine completion ritual. |
| [domain](../skills/domain/SKILL.md) | Clarify domain terms, bounded contexts and repository conventions for a change. Use when: naming, rule ownership or applicable Go and other language standards are unclear; avoid a full architecture survey. |
| [refactor](../skills/refactor/SKILL.md) | Simplify structure, interfaces or responsibilities while preserving observable behavior. Use when: the caller requests a focused refactor; feature changes and architecture churn need their own intent. |
| [research](../skills/research/SKILL.md) | Answer a bounded question by tracing code, investigating evidence or testing a recurring pattern. Use when: consequential uncertainty needs sources. Not for external feature teardowns; use reverse-engineer. |
| [reverse-engineer](../skills/reverse-engineer/SKILL.md) | Tear down an authorized competitor or upstream repository, binary or product into a feature inventory and adoption choices. Use when: comparing an external system; local code questions belong to Research. |
| [security](../skills/security/SKILL.md) | Review code or run authorized security scans for vulnerabilities, secrets, dependencies and prompt boundaries. Use when: concrete security exposure needs assessment; report gaps without silently changing policy. |
| [skill-builder](../skills/skill-builder/SKILL.md) | Create, adapt, consolidate or repair skill packages and generated projections. Use when: authoring guidance, descriptions or package structure; Skill Eval measures behavior rather than structural conformance. |
| [skill-eval](../skills/skill-eval/SKILL.md) | Measure whether a skill helps a named task or needs revision, removal or more evidence. Use when: a bounded routing or controlled coding evaluation is requested; static conformance alone cannot show benefit. |
| [test](../skills/test/SKILL.md) | Write or strengthen behavioral tests, practice TDD or investigate important coverage gaps. Use when: test design or missing proof needs work; running an existing suite needs no skill. |

## Memory on demand

| Skill | Use it for |
|---|---|
| [memory](../skills/memory/SKILL.md) | Recall reviewed lessons or deliberately mine and curate experience. Use when: prior evidence can change an action, or learning is requested; no mandatory recall or lesson. |

## Deliberate planning and review strategies

| Skill | Use it for |
|---|---|
| [council](../skills/council/SKILL.md) | Compare independent perspectives on a consequential or contested decision. Use when: the caller selects multiple judges; evidence resolves disagreement, not majority voting or model prestige. |
| [craft-goal](../skills/craft-goal/SKILL.md) | Draft or lint a bounded persistent goal above a bead graph of RPI experiments. Use when: this goal workflow is explicitly selected; shaping a single change belongs to Plan. |
| [idea-genie](../skills/idea-genie/SKILL.md) | Generate evidenced options or challenge a proposed idea. Use when: deciding what to build or comparing alternatives; exploration does not authorize implementation or approve the result. |
| [postmortem](../skills/postmortem/SKILL.md) | Test a retrospective causal question against outcome evidence. Use when: the caller explicitly requests a postmortem; a finished task does not automatically require a report or a new lesson. |
| [premortem](../skills/premortem/SKILL.md) | Challenge a rollout plan with one fresh judge before implementation; identify what could make it fail. Not for finished-code judgment. Triggers: "one judge", "challenge this plan". |
| [reality-check](../skills/reality-check/SKILL.md) | Check whether a claimed shipped feature, repository state or goal status holds up in observable evidence. Use when: comparing a claim with what exists; a gap report is not a final candidate verdict. |
| [rpi](../skills/rpi/SKILL.md) | Apply the optional outcome-to-judgment operating charter. Use when: the caller explicitly selects RPI; ordinary coding, delegated work and native goals do not require this workflow. |

## Explicit tool and runtime adapters

| Skill | Use it for |
|---|---|
| [account-rotation](../skills/account-rotation/SKILL.md) | Switch a coding-agent account and verify its observed identity. Use when: the caller requests an account change on this host; does not allocate quota or switch accounts automatically. |
| [agent-mail](../skills/agent-mail/SKILL.md) | Coordinate explicitly selected writers with Agent Mail messages and advisory file reservations. Use when: work already needs this adapter; messages and reservations do not own tracker status. |
| [agent-native](../skills/agent-native/SKILL.md) | Dispatch independent tasks to parallel workers or operate selected persistent roles. Use when: delegation is authorized and scopes are disjoint; runtime completion does not validate output. |
| [agy-native](../skills/agy-native/SKILL.md) | Run a supplied task in AGY Antigravity and collect its result. Use when: the caller selects AGY; never a fallback for native coding. |
| [cass](../skills/cass/SKILL.md) | Search agent session logs and inspect cited episodes with CASS. Use when: past prompts, decisions or failures may answer a question; repeated text is a candidate observation, not a proven lesson. |
| [cc-hooks](../skills/cc-hooks/SKILL.md) | Configure Claude Code hooks and narrow enforcement guards. Use when: the caller requests hook installation, repair or policy changes; a hook is not required to use other skills. |
| [codex-exec](../skills/codex-exec/SKILL.md) | Run one supplied prompt through headless Codex and capture its result. Use when: a noninteractive Codex process is requested; this adapter does not choose work, retry or judge correctness. |
| [dcg](../skills/dcg/SKILL.md) | Diagnose a Destructive Command Guard block or configure its guardrails. Use when: DCG rejected an operation or the caller requests policy work; do not bypass a block by disguising the command. |
| [ms](../skills/ms/SKILL.md) | Find and load skill guidance with the configured meta_skill search engine. Use when: searching a larger skill corpus for a task; use CASS for past sessions and Skill Builder for authoring packages. |
| [ntm](../skills/ntm/SKILL.md) | Operate caller-selected NTM agent panes and inspect their native state. Use when: persistent tmux roles are explicitly requested; pane liveness and prompt delivery are not validation. |
| [rch](../skills/rch/SKILL.md) | Offload one requested build through RCH or diagnose its remote compiler path. Use when: remote compilation is explicitly selected; report errors without creating a retry controller. |
| [sbh](../skills/sbh/SKILL.md) | Inspect storage pressure with SBH and perform a specifically authorized recovery action. Use when: diagnosing disk pressure or selecting SBH recovery; inspection alone does not authorize deletion. |
| [using-flywheel](../skills/using-flywheel/SKILL.md) | Operate the Agentic Coding Flywheel through its native workflow. Use when: the caller explicitly selects this factory; convergence and closed work do not prove semantic acceptance. |
| [using-gc](../skills/using-gc/SKILL.md) | Operate Gas City through its Mayor, registry packs and native run state. Use when: the caller explicitly selects Gas City; factory completion does not replace independent judgment. |

## Complete inventory

| Skill | Tier | Disposition | Hard dependencies | Capabilities | Effects |
|---|---|---|---|---|---|
| `account-rotation` | execution | `keep_optional_adapter` | - | `account_rotation` | `rotate_agent_account` |
| `agent-mail` | execution | `keep_optional_adapter` | - | `agent_mail` | `write_agent_mail_records`, `install_precommit_guard`, `authorized_destructive_reset` |
| `agent-native` | meta | `keep_optional_adapter` | - | `role_dispatch`, `observe_workers`, `handoff`, `dispatch_once` | `manage_runtime_sessions`, `invoke_selected_executor` |
| `agy-native` | cross-vendor | `keep_optional_adapter` | - | `dispatch_explicit_packet`, `provide_fresh_context` | `start_agy_session` |
| `cass` | execution | `keep_optional_adapter` | - | `cass` | `rebuild_local_index`, `sync_remote_sources`, `download_semantic_model` |
| `cc-hooks` | execution | `keep_optional_adapter` | - | `cc_hooks` | `write_hook_config`, `append_guardrail_telemetry`, `write_session_sentinel` |
| `codex-exec` | orchestration | `keep_optional_adapter` | - | `codex_exec` | `run_codex_process`, `sandbox_tiered_workspace_and_network_effects` |
| `council` | judgment | `keep_strategy` | - | `collect_independent_judgments`, `synthesize_disagreement` | `write_advisory_council_report` |
| `craft-goal` | judgment | `keep_strategy` | - | `goal_prompt_design`, `goal_prompt_lint` | - |
| `dcg` | execution | `keep_optional_adapter` | - | `dcg` | `write_dcg_config` |
| `doc` | product | `keep_specialist` | - | `doc`, `initialize_missing_docs`, `write_session_handoff` | `write_documentation`, `write_requested_handoff`, `create_requested_evidence_directory` |
| `domain` | knowledge | `keep_specialist` | - | `domain`, `clarify_domain_language`, `reconcile_domain_names` | `update_existing_domain_contracts` |
| `idea-genie` | execution | `keep_strategy` | - | `generate_evidenced_options`, `dueling_idea_genies` | `write_idea_portfolio` |
| `implement` | execution | `keep` | - | `execute_one_experiment`, `collect_factual_evidence` | `modify_declared_subject`, `derive_subject_manifest` |
| `memory` | execution | `keep_off_path` | - | `recall_applicable_context`, `mine_supported_observations`, `curate_topic_pages`, `toil_mining` | `write_protected_drafts`, `update_authorized_topic_pages`, `write_requested_toil_report` |
| `ms` | execution | `keep_optional_adapter` | - | `ms` | `spawn_search_server`, `write_feedback_outcomes`, `rebuild_search_index` |
| `ntm` | execution | `keep_optional_adapter` | - | `ntm` | `manage_ntm_panes`, `dispatch_pane_commands` |
| `plan` | execution | `keep` | - | `shape_intent`, `define_acceptance`, `bound_write_scope` | `update_intent_source` |
| `postmortem` | judgment | `keep_strategy` | - | `postmortem` | `write_postmortem_report` |
| `premortem` | judgment | `keep_strategy` | - | `challenge_plan` | `write_advisory_plan_review` |
| `rch` | execution | `keep_optional_adapter` | - | `rch` | `remote_compilation_offload`, `authorized_remote_daemon_worker_mutation` |
| `reality-check` | judgment | `keep_strategy` | - | `compare_claim_to_evidence`, `measure_declared_goals`, `report_native_status` | `write_advisory_gap_report`, `write_goal_snapshot`, `write_requested_rendered_spec` |
| `refactor` | execution | `keep_specialist` | - | `refactor` | `modify_source_files` |
| `research` | execution | `keep_specialist` | - | `research`, `codebase_recon`, `pattern_mining` | `write_research_report`, `write_recon_pack`, `write_pattern_evidence` |
| `reverse-engineer` | execution | `keep_specialist` | - | `reverse_engineer` | `clone_upstream_repo`, `authorized_binary_execution`, `write_teardown_artifacts` |
| `rpi` | meta | `keep_strategy` | `plan`, `implement`, `validate` | `own_authorized_outcome`, `report` | `dispatch_core_phases` |
| `sbh` | execution | `keep_optional_adapter` | - | `sbh` | `delete_reclaimable_files`, `release_disk_ballast`, `modify_host_storage_config` |
| `security` | product | `keep_specialist` | - | `security` | `write_scan_artifacts` |
| `skill-builder` | meta | `keep_specialist` | - | `skill_builder`, `heal_skill`, `export_skill`, `distill_expertise` | `write_skill_source`, `write_build_report`, `regenerate_skill_projections`, `repair_skill_projections`, `write_converted_skill_projection`, `write_advisory_proposal` |
| `skill-eval` | meta | `keep_specialist` | - | `author_seeded_probe`, `run_probe_tier`, `evaluate_skill_decision` | `write_probe_package`, `dispatch_probe_producer` |
| `test` | execution | `keep_specialist` | - | `test` | `write_test_files`, `write_test_evidence`, `modify_source_files` |
| `using-flywheel` | execution | `keep_optional_adapter` | - | `route_to_native_flywheel_workflow`, `expose_agentops_skills`, `observe_flywheel_runtime` | - |
| `using-gc` | execution | `keep_optional_adapter` | - | `dispatch_explicit_packet`, `observe_gc_runtime`, `inspect_pack_registries`, `drive_mayor_door` | `operate_gas_city`, `configure_codex_trust` |
| `validate` | judgment | `keep` | - | `compute_subject_identity`, `judge_acceptance`, `return_validation_result`, `persist_verdict` | `write_verdict_artifact` |
