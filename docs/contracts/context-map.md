<!-- generated from skills/*/SKILL.md metadata -->

# AgentOps context map

## Hard dependencies

| Source | Target |
|---|---|
| `rpi` | `implement` |
| `rpi` | `plan` |
| `rpi` | `validate` |

## Optional context relationships

| Source | Kind | Target |
|---|---|---|
| `agy-native` | `separate-ways` | `codex-exec` |
| `architecture` | `partnership` | `domain` |
| `architecture` | `supplier-to` | `plan` |
| `architecture` | `supplier-to` | `refactor` |
| `claude-exec` | `separate-ways` | `codex-exec` |
| `claude-exec` | `supplier-to` | `validate` |
| `codex-exec` | `supplier-to` | `validate` |
| `craft-goal` | `supplier-to` | `plan` |
| `ergonomics` | `partnership` | `doc` |
| `ergonomics` | `supplier-to` | `implement` |
| `idea-genie` | `customer-of` | `research` |
| `idea-genie` | `supplier-to` | `plan` |
| `implement` | `customer-of` | `plan` |
| `interview` | `supplier-to` | `craft-goal` |
| `navigate` | `customer-of` | `craft-goal` |
| `navigate` | `supplier-to` | `orchestrate` |
| `orchestrate` | `customer-of` | `agy-native` |
| `orchestrate` | `customer-of` | `claude-exec` |
| `orchestrate` | `customer-of` | `codex-exec` |
| `premortem` | `supplier-to` | `plan` |
| `research` | `supplier-to` | `plan` |
| `rpi` | `customer-of` | `implement` |
| `rpi` | `customer-of` | `plan` |
| `rpi` | `customer-of` | `validate` |
| `security` | `supplier-to` | `validate` |
| `skill-eval` | `supplier-to` | `skill-builder` |
| `triage` | `supplier-to` | `plan` |
| `validate` | `customer-of` | `implement` |
| `validate` | `customer-of` | `plan` |

## Data flow

| Skill | Direction | Artifact |
|---|---|---|
| `agy-native` | consumes | `explicit-packet` |
| `agy-native` | produces | `agy-run-evidence` |
| `architecture` | consumes | `repo-context` |
| `architecture` | produces | `architecture-proposal` |
| `claude-exec` | consumes | `claude-command-packet` |
| `claude-exec` | produces | `claude-run-output` |
| `codex-exec` | consumes | `codex-command-packet` |
| `codex-exec` | produces | `codex-run-output` |
| `council` | consumes | `explicit-question` |
| `council` | consumes | `evidence` |
| `council` | produces | `council-report.v1` |
| `craft-goal` | consumes | `caller-outcome` |
| `craft-goal` | consumes | `goal-acceptance` |
| `craft-goal` | produces | `outer-goal-prompt` |
| `craft-goal` | produces | `goal-safety-report` |
| `doc` | consumes | `repo-context` |
| `doc` | produces | `documentation` |
| `doc` | produces | `session-handoff` |
| `domain` | produces | `domain-language-guidance` |
| `ergonomics` | consumes | `repo-context` |
| `ergonomics` | consumes | `interface-design` |
| `ergonomics` | produces | `ergonomics-findings` |
| `idea-genie` | consumes | `repo-context` |
| `idea-genie` | consumes | `task-question` |
| `idea-genie` | consumes | `idea-portfolio.v1` |
| `idea-genie` | produces | `idea-portfolio.v1` |
| `idea-genie` | produces | `idea-challenge.v1` |
| `implement` | produces | `subject-manifest.v1` |
| `interview` | consumes | `repo-context` |
| `interview` | consumes | `native-work-state` |
| `interview` | produces | `caller-outcome` |
| `interview` | produces | `goal-acceptance` |
| `memory` | produces | `applicable-context` |
| `memory` | produces | `reviewed-topic-pages` |
| `memory` | produces | `ranked-toil-evidence` |
| `navigate` | consumes | `outer-goal-prompt` |
| `navigate` | consumes | `goal-acceptance` |
| `navigate` | consumes | `native-work-state` |
| `navigate` | consumes | `validation-result` |
| `navigate` | produces | `native-handoffs` |
| `orchestrate` | consumes | `accepted-intent` |
| `orchestrate` | consumes | `native-work-state` |
| `orchestrate` | consumes | `candidate-evidence` |
| `orchestrate` | consumes | `explicit-role-packets` |
| `orchestrate` | produces | `native-handoffs` |
| `orchestrate` | produces | `reconciled-feedback` |
| `orchestrate` | produces | `runtime-evidence` |
| `orchestrate` | produces | `per-packet-results` |
| `postmortem` | produces | `postmortem-analysis` |
| `premortem` | produces | `premortem-plan-review.v1` |
| `refactor` | consumes | `repo-context` |
| `refactor` | produces | `code-changes` |
| `research` | consumes | `research-question` |
| `research` | consumes | `native-source-evidence` |
| `research` | produces | `research-report` |
| `research` | produces | `codebase-recon.v1` |
| `research` | produces | `pattern-mining.v1` |
| `research` | produces | `reality-check-report.v1` |
| `research` | produces | `goal-measurement-report` |
| `research` | produces | `native-status-snapshot` |
| `rpi` | consumes | `plan` |
| `rpi` | consumes | `implement` |
| `rpi` | consumes | `validate` |
| `security` | consumes | `repo-context` |
| `security` | produces | `security-gate-summary.json` |
| `security` | produces | `suite-summary.json` |
| `security` | produces | `redteam-results.json` |
| `skill-builder` | produces | `skill-source-package` |
| `skill-builder` | produces | `skill-hygiene-report` |
| `skill-builder` | produces | `converted-skill` |
| `skill-builder` | produces | `operationalization-proposal` |
| `skill-eval` | consumes | `skill-source-package` |
| `skill-eval` | produces | `skill-maintenance-recommendation` |
| `skill-eval` | produces | `probe-package` |
| `test` | consumes | `standards` |
| `test` | consumes | `repo-context` |
| `test` | produces | `test-evidence` |
| `triage` | consumes | `incoming-report` |
| `triage` | consumes | `native-source-evidence` |
| `triage` | produces | `triage-outcome` |
| `triage` | produces | `agent-ready-brief` |
| `validate` | consumes | `subject-manifest.v1` |
| `validate` | produces | `subject-manifest.v1` |
| `validate` | produces | `validation-result` |
| `validate` | produces | `verdict.v2` |
