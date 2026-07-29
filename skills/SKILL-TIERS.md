<!-- generated from skills/*/SKILL.md metadata -->

# Skill tiers

## cross-vendor

`agy-native`, `converter`

## execution

`account-rotation`, `agent-mail`, `cass`, `cc-hooks`, `codebase-recon`, `dcg`, `idea-genie`, `implement`, `learn`, `ms`, `ntm`, `pattern-mining`, `plan`, `rch`, `refactor`, `research`, `reverse-engineer`, `sbh`, `scaffold`, `swarm`, `test`, `using-gc`

## judgment

`council`, `craft-goal`, `postmortem`, `premortem`, `reality-check`, `validate`

## knowledge

`domain`, `standards`

## library

`shared`

## meta

`agent-native`, `automation-shape-routing`, `operationalize`, `rpi`, `scope`, `skill-builder`, `toil-mining`, `workflow-builder`

## orchestration

`codex-exec`

## product

`doc`, `fitness`, `goals`, `product`, `security`

## session

`bootstrap`, `handoff`, `status`

## Inventory

| Skill | Tier | Disposition | Hard dependencies | Capabilities | Effects |
|---|---|---|---|---|---|
| `account-rotation` | execution | `keep_specialist` | - | `account_rotation` | `rotate_agent_account` |
| `agent-mail` | execution | `keep_optional_adapter` | - | `agent_mail` | `write_agent_mail_records`, `install_precommit_guard`, `authorized_destructive_reset` |
| `agent-native` | meta | `keep_optional_adapter` | - | `role_dispatch`, `observe_workers`, `handoff` | `manage_runtime_sessions` |
| `agy-native` | cross-vendor | `keep_optional_adapter` | - | `dispatch_explicit_packet`, `provide_fresh_context` | `start_agy_session` |
| `automation-shape-routing` | meta | `keep_optional_adapter` | - | `automation_shape_routing` | - |
| `bootstrap` | session | `keep_specialist` | - | `bootstrap` | `write_project_docs` |
| `cass` | execution | `keep_specialist` | - | `cass` | `rebuild_local_index`, `sync_remote_sources`, `download_semantic_model` |
| `cc-hooks` | execution | `keep_specialist` | - | `cc_hooks` | `write_hook_config`, `append_guardrail_telemetry`, `write_session_sentinel` |
| `codebase-recon` | execution | `keep_specialist` | - | `codebase_recon` | `write_recon_pack` |
| `codex-exec` | orchestration | `keep_optional_adapter` | - | `codex_exec` | `run_codex_process`, `sandbox_tiered_workspace_and_network_effects` |
| `converter` | cross-vendor | `keep_specialist` | - | `converter` | `write_converted_skill_projection` |
| `council` | judgment | `keep_strategy` | - | `collect_independent_judgments`, `synthesize_disagreement` | `write_advisory_council_report` |
| `craft-goal` | judgment | `keep_specialist` | - | `goal_prompt_design`, `goal_prompt_lint` | - |
| `dcg` | execution | `keep_specialist` | - | `dcg` | `write_dcg_config` |
| `doc` | product | `keep_specialist` | - | `doc` | `write_documentation` |
| `domain` | knowledge | `keep_specialist` | - | `domain` | - |
| `fitness` | product | `keep_specialist` | - | `fitness` | `write_goal_snapshot`, `write_rendered_spec` |
| `goals` | product | `keep_off_path` | - | - | - |
| `handoff` | session | `keep_specialist` | - | `handoff` | `write_handoff_artifact`, `read_git_state`, `read_clock` |
| `idea-genie` | execution | `keep_strategy` | - | `generate_evidenced_options`, `dueling_idea_genies` | `write_idea_portfolio` |
| `implement` | execution | `keep` | - | `execute_one_experiment`, `collect_factual_evidence` | `modify_declared_subject`, `derive_subject_manifest` |
| `learn` | execution | `keep_off_path` | - | `analyze_verdict_collections` | `write_advisory_observations` |
| `ms` | execution | `keep_specialist` | - | `ms` | `spawn_search_server`, `write_feedback_outcomes`, `rebuild_search_index` |
| `ntm` | execution | `keep_optional_adapter` | - | `ntm` | `manage_ntm_panes`, `dispatch_pane_commands` |
| `operationalize` | meta | `keep_specialist` | - | `distill_expertise`, `propose_artifact_shape` | `write_advisory_proposal` |
| `pattern-mining` | execution | `keep_specialist` | - | `pattern_mining` | `write_pattern_evidence` |
| `plan` | execution | `keep` | - | `shape_intent`, `define_acceptance`, `bound_write_scope` | `update_intent_source` |
| `postmortem` | judgment | `keep_strategy` | - | `postmortem` | `write_postmortem_report` |
| `premortem` | judgment | `keep_strategy` | - | `challenge_plan` | `write_advisory_plan_review` |
| `product` | product | `keep_specialist` | - | `shape_product_boundary` | `write_product_document` |
| `rch` | execution | `keep_specialist` | - | `rch` | `remote_compilation_offload`, `authorized_remote_daemon_worker_mutation` |
| `reality-check` | judgment | `keep_strategy` | - | `compare_claim_to_evidence` | `write_advisory_gap_report` |
| `refactor` | execution | `keep_specialist` | - | `refactor` | `modify_source_files` |
| `research` | execution | `keep_specialist` | - | `research` | `write_research_report` |
| `reverse-engineer` | execution | `keep_specialist` | - | `reverse_engineer` | `clone_upstream_repo`, `authorized_binary_execution`, `write_teardown_artifacts` |
| `rpi` | meta | `keep` | `plan`, `implement`, `validate` | `orchestrate_once`, `report` | `dispatch_core_phases` |
| `sbh` | execution | `keep_specialist` | - | `sbh` | `delete_reclaimable_files`, `release_disk_ballast`, `modify_host_storage_config` |
| `scaffold` | execution | `keep_specialist` | - | `scaffold` | `write_project_files` |
| `scope` | meta | `keep_specialist` | - | `scope_review` | - |
| `security` | product | `keep_specialist` | - | `security` | `write_scan_artifacts` |
| `shared` | library | `keep_specialist` | - | `provide_reference_context` | - |
| `skill-builder` | meta | `keep_specialist` | - | `skill_builder`, `heal_skill` | `write_skill_source`, `write_build_report`, `regenerate_skill_projections`, `repair_skill_projections` |
| `standards` | knowledge | `keep_specialist` | - | `standards` | - |
| `status` | session | `keep_specialist` | - | `status` | `read_filesystem`, `read_clock` |
| `swarm` | execution | `keep_optional_adapter` | - | `dispatch_once` | `invoke_selected_executor` |
| `test` | execution | `keep_specialist` | - | `test` | `write_test_files`, `write_test_evidence`, `modify_source_files` |
| `toil-mining` | meta | `keep_specialist` | - | `toil_mining` | `write_toil_candidates` |
| `using-gc` | execution | `keep_optional_adapter` | - | `dispatch_explicit_packet`, `observe_gc_runtime`, `drive_mayor_shepherd` | `operate_gas_city`, `configure_codex_trust` |
| `validate` | judgment | `keep` | - | `compute_subject_identity`, `judge_acceptance`, `persist_verdict` | `write_verdict_artifact` |
| `workflow-builder` | meta | `keep_specialist` | - | `workflow_builder` | `write_workflow_script` |
