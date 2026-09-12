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
| `agent-mail` | `supplier-to` | `agent-native` |
| `agent-native` | `customer-of` | `agent-mail` |
| `agent-native` | `customer-of` | `codex-exec` |
| `agent-native` | `customer-of` | `ntm` |
| `agy-native` | `separate-ways` | `codex-exec` |
| `codex-exec` | `supplier-to` | `validate` |
| `craft-goal` | `supplier-to` | `plan` |
| `idea-genie` | `customer-of` | `research` |
| `idea-genie` | `supplier-to` | `plan` |
| `implement` | `customer-of` | `plan` |
| `ntm` | `supplier-to` | `agent-native` |
| `premortem` | `supplier-to` | `plan` |
| `reality-check` | `supplier-to` | `plan` |
| `rpi` | `customer-of` | `implement` |
| `rpi` | `customer-of` | `plan` |
| `rpi` | `customer-of` | `validate` |
| `security` | `supplier-to` | `validate` |
| `skill-eval` | `supplier-to` | `skill-builder` |
| `using-flywheel` | `partnership` | `using-gc` |
| `using-gc` | `partnership` | `agent-native` |
| `validate` | `customer-of` | `implement` |
| `validate` | `customer-of` | `plan` |

## Data flow

| Skill | Direction | Artifact |
|---|---|---|
| `agent-mail` | consumes | `coordination-request` |
| `agent-mail` | produces | `agent-identity` |
| `agent-mail` | produces | `file-reservation` |
| `agent-mail` | produces | `acknowledged-handoff` |
| `agent-native` | consumes | `explicit-role-packets` |
| `agent-native` | produces | `runtime-evidence` |
| `agent-native` | produces | `worker-handoff` |
| `agent-native` | produces | `per-packet-results` |
| `agy-native` | consumes | `explicit-packet` |
| `agy-native` | produces | `agy-run-evidence` |
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
| `idea-genie` | consumes | `repo-context` |
| `idea-genie` | consumes | `task-question` |
| `idea-genie` | consumes | `idea-portfolio.v1` |
| `idea-genie` | produces | `idea-portfolio.v1` |
| `idea-genie` | produces | `idea-challenge.v1` |
| `implement` | produces | `subject-manifest.v1` |
| `memory` | produces | `applicable-context` |
| `memory` | produces | `reviewed-topic-pages` |
| `memory` | produces | `ranked-toil-evidence` |
| `ntm` | consumes | `pane-command-request` |
| `ntm` | produces | `ntm-robot-state` |
| `ntm` | produces | `agent-worker-transcript` |
| `postmortem` | produces | `postmortem-report.md` |
| `premortem` | produces | `premortem-plan-review.v1` |
| `reality-check` | consumes | `caller-question` |
| `reality-check` | consumes | `native-source-evidence` |
| `reality-check` | produces | `reality-check-report.v1` |
| `reality-check` | produces | `goal-measurement-report` |
| `reality-check` | produces | `native-status-snapshot` |
| `refactor` | consumes | `repo-context` |
| `refactor` | produces | `code-changes` |
| `research` | consumes | `research-question` |
| `research` | produces | `research-report` |
| `research` | produces | `codebase-recon.v1` |
| `research` | produces | `pattern-mining.v1` |
| `reverse-engineer` | produces | `.agents/scratch/reverse-engineer/*/` |
| `rpi` | consumes | `plan` |
| `rpi` | consumes | `implement` |
| `rpi` | consumes | `validate` |
| `rpi` | produces | `rpi-report.v1` |
| `security` | consumes | `repo-context` |
| `security` | produces | `security-gate-summary.json` |
| `security` | produces | `suite-summary.json` |
| `security` | produces | `redteam-results.json` |
| `skill-builder` | produces | `skill-source-package` |
| `skill-builder` | produces | `skill-hygiene-report` |
| `skill-builder` | produces | `converted-skill` |
| `skill-builder` | produces | `operationalization-proposal` |
| `skill-eval` | consumes | `skill-source-package` |
| `skill-eval` | produces | `probe-package` |
| `skill-eval` | produces | `probe-result.v1` |
| `test` | consumes | `standards` |
| `test` | consumes | `repo-context` |
| `test` | produces | `test-evidence` |
| `using-flywheel` | consumes | `explicit-packets` |
| `using-flywheel` | produces | `flywheel-runtime-evidence` |
| `using-gc` | consumes | `explicit-packets` |
| `using-gc` | produces | `gas-city-runtime-evidence` |
| `validate` | consumes | `subject-manifest.v1` |
| `validate` | produces | `subject-manifest.v1` |
| `validate` | produces | `validation-result` |
| `validate` | produces | `verdict.v2` |
