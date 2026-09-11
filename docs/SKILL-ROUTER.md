<!-- generated from skills/*/SKILL.md metadata -->

# Skill Router

34 live skills. Choose guidance for a concrete task need; no skill is mandatory.
A clear task can proceed in the native agent. Read a skill only when its description fits.
Names and descriptions below come from each source SKILL.md; explicit invocation remains available.

## Intent, implementation and final judgment

| Skill | Use it for |
|---|---|
| [implement](../skills/implement/SKILL.md) | Implement accepted behavior, repair defects or execute a selected wave with per-lane evidence. Use when: coding is authorized and ready; return facts, not a binding verdict. |
| [plan](../skills/plan/SKILL.md) | Define intended behavior, review write scope and assess reversible decisions. Use when: acceptance or approach is unclear before coding; stop once actionable. |
| [validate](../skills/validate/SKILL.md) | Freshly judge a finished change against original acceptance before merge. Use when: independent proof is needed; author tests cannot issue PASS. Triggers: "check this change". |

## Engineering specialists

| Skill | Use it for |
|---|---|
| [doc](../skills/doc/SKILL.md) | Write grounded docs, READMEs, repo instructions or continuity handoffs. Use when: these documents are requested; no reports as a routine completion ritual. |
| [domain](../skills/domain/SKILL.md) | Clarify domain terms, bounded contexts and repository conventions. Use when: naming, rule ownership or Go and other language standards are unclear; avoid a broad survey. |
| [refactor](../skills/refactor/SKILL.md) | Simplify structure, interfaces or responsibilities while preserving behavior. Use when: a focused refactor is requested; feature changes need their own intent. |
| [research](../skills/research/SKILL.md) | Trace code or test a recurring pattern to answer one cited question. Use when: uncertainty needs evidence. Not for external feature teardowns; use reverse-engineer. |
| [reverse-engineer](../skills/reverse-engineer/SKILL.md) | Tear down an authorized competitor repo, binary or product into a feature inventory and adoption choices. Use when: comparing an external system; local questions go to Research. |
| [security](../skills/security/SKILL.md) | Review code or scan for security vulnerabilities, secrets, dependencies and prompt risks. Use when: concrete exposure needs assessment; never silently change policy. |
| [skill-builder](../skills/skill-builder/SKILL.md) | Create, adapt, consolidate or repair skill packages and projections. Use when: authoring guidance, descriptions or structure; Skill Eval measures behavioral benefit. |
| [skill-eval](../skills/skill-eval/SKILL.md) | Measure whether a skill helps a named task or needs revision or removal. Use when: a bounded routing or coding evaluation is requested; conformance alone cannot show benefit. |
| [test](../skills/test/SKILL.md) | Write behavioral tests, practice TDD or inspect important coverage gaps. Use when: test design or missing proof needs work; running an existing suite needs no skill. |

## Memory on demand

| Skill | Use it for |
|---|---|
| [memory](../skills/memory/SKILL.md) | Recall reviewed lessons or deliberately mine and curate experience. Use when: prior evidence can change an action, or learning is requested; no mandatory recall or lesson. |

## Deliberate planning and review strategies

| Skill | Use it for |
|---|---|
| [council](../skills/council/SKILL.md) | Compare independent views on a consequential or contested decision. Use when: the caller selects multiple judges; evidence resolves disagreement, not voting. |
| [craft-goal](../skills/craft-goal/SKILL.md) | Draft or lint a bounded persistent goal above a bead graph of RPI experiments. Use when: this goal workflow is explicitly selected; shaping a single change belongs to Plan. |
| [idea-genie](../skills/idea-genie/SKILL.md) | Generate evidenced options or challenge an idea. Use when: deciding what to build or comparing alternatives; exploration does not authorize implementation. |
| [postmortem](../skills/postmortem/SKILL.md) | Analyze outcomes or an interim cutoff. Use when: a postmortem is explicitly requested; consumes available judgment, never gates code acceptance or requires a lesson. |
| [premortem](../skills/premortem/SKILL.md) | Challenge a rollout plan with one fresh judge before implementation; identify what could make it fail. Not for finished-code judgment. Triggers: "one judge", "challenge this plan". |
| [reality-check](../skills/reality-check/SKILL.md) | Check whether a claimed shipped feature, repo state or goal status holds up in evidence. Use when: comparing a claim with what exists; a gap report is not a verdict. |
| [rpi](../skills/rpi/SKILL.md) | Apply the outcome-to-judgment charter. Use when: the caller explicitly selects RPI; ordinary coding, delegation and native goals do not require this workflow. |

## Explicit tool and runtime adapters

| Skill | Use it for |
|---|---|
| [account-rotation](../skills/account-rotation/SKILL.md) | Switch coding-agent accounts and verify runtime identity. Use when: the caller requests an account change; never rotate automatically to evade a quota. |
| [agent-mail](../skills/agent-mail/SKILL.md) | Coordinate selected writers with Agent Mail messages and advisory file reservations. Use when: this adapter is requested; mail does not own tracker status. |
| [agent-native](../skills/agent-native/SKILL.md) | Dispatch independent tasks to parallel workers or selected persistent roles. Use when: delegation is authorized with disjoint scopes; execution does not validate output. |
| [agy-native](../skills/agy-native/SKILL.md) | Run a supplied task in AGY Antigravity and collect its result. Use when: the caller selects AGY; never a fallback for native coding. |
| [cass](../skills/cass/SKILL.md) | Search agent session logs and cited episodes with CASS. Use when: past prompts, decisions or failures may answer a question; repeated text is not a proven lesson. |
| [cc-hooks](../skills/cc-hooks/SKILL.md) | Configure Claude Code hooks and narrow enforcement guards. Use when: the caller requests hook installation, repair or policy changes; a hook is not required to use other skills. |
| [codex-exec](../skills/codex-exec/SKILL.md) | Run one prompt through headless Codex and capture its result. Use when: requesting a single noninteractive Codex process. Not for worker batches or retries. |
| [dcg](../skills/dcg/SKILL.md) | Diagnose a Destructive Command Guard block or configure its rules. Use when: DCG rejected an operation or policy work is requested; never disguise commands to bypass it. |
| [ms](../skills/ms/SKILL.md) | Find and load guidance with the meta_skill search engine. Use when: searching a skill corpus; CASS owns past sessions and Skill Builder owns package authoring. |
| [ntm](../skills/ntm/SKILL.md) | Operate selected NTM agent panes and inspect native state. Use when: persistent tmux roles are requested; pane liveness and prompt delivery are not validation. |
| [rch](../skills/rch/SKILL.md) | Offload one build through RCH or diagnose its remote compiler. Use when: remote compilation is selected; report errors without creating a retry controller. |
| [sbh](../skills/sbh/SKILL.md) | Inspect disk pressure with SBH and perform an authorized recovery action. Use when: storage diagnosis or SBH recovery is requested; inspection does not authorize deletion. |
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
