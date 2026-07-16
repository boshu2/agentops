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
| `agent-native` | `customer-of` | `ntm` |
| `agy-native` | `separate-ways` | `codex-exec` |
| `automation-shape-routing` | `supplier-to` | `agent-native` |
| `automation-shape-routing` | `supplier-to` | `operationalize` |
| `automation-shape-routing` | `supplier-to` | `skill-builder` |
| `automation-shape-routing` | `supplier-to` | `using-gc` |
| `automation-shape-routing` | `supplier-to` | `workflow-builder` |
| `codebase-recon` | `customer-of` | `doc` |
| `codebase-recon` | `customer-of` | `research` |
| `codebase-recon` | `customer-of` | `validate` |
| `codex-exec` | `supplier-to` | `validate` |
| `dueling-idea-genies` | `customer-of` | `idea-genie` |
| `dueling-idea-genies` | `supplier-to` | `plan` |
| `goals` | `shared-kernel` | `standards` |
| `idea-genie` | `customer-of` | `research` |
| `idea-genie` | `supplier-to` | `plan` |
| `implement` | `customer-of` | `plan` |
| `learn` | `customer-of` | `validate` |
| `ntm` | `supplier-to` | `agent-native` |
| `operationalize` | `supplier-to` | `skill-builder` |
| `operationalize` | `supplier-to` | `workflow-builder` |
| `pattern-mining` | `customer-of` | `research` |
| `pattern-mining` | `customer-of` | `validate` |
| `pattern-mining` | `supplier-to` | `operationalize` |
| `premortem` | `supplier-to` | `plan` |
| `product` | `supplier-to` | `plan` |
| `reality-check` | `supplier-to` | `plan` |
| `rpi` | `customer-of` | `implement` |
| `rpi` | `customer-of` | `plan` |
| `rpi` | `customer-of` | `validate` |
| `scope` | `supplier-to` | `plan` |
| `security` | `supplier-to` | `validate` |
| `toil-mining` | `supplier-to` | `automation-shape-routing` |
| `using-gc` | `partnership` | `agent-native` |
| `validate` | `customer-of` | `implement` |
| `validate` | `customer-of` | `plan` |
| `workflow-builder` | `customer-of` | `automation-shape-routing` |
| `workflow-builder` | `shared-kernel` | `operationalize` |

## Data flow

| Skill | Direction | Artifact |
|---|---|---|
| `agent-mail` | consumes | `task-intent` |
| `agent-mail` | produces | `agent-identity` |
| `agent-mail` | produces | `file-reservation` |
| `agent-mail` | produces | `acknowledged-handoff` |
| `agent-native` | consumes | `explicit-role-packets` |
| `agent-native` | produces | `runtime-evidence` |
| `agent-native` | produces | `worker-handoff` |
| `agy-native` | consumes | `explicit-packet` |
| `agy-native` | produces | `agy-run-evidence` |
| `automation-shape-routing` | consumes | `task-intent` |
| `automation-shape-routing` | produces | `automation-shape-verdict` |
| `bootstrap` | consumes | `goals` |
| `bootstrap` | consumes | `product` |
| `bootstrap` | consumes | `doc` |
| `bootstrap` | consumes | `shared` |
| `codebase-recon` | consumes | `repo-context` |
| `codebase-recon` | consumes | `existing-docs` |
| `codebase-recon` | produces | `codebase-recon.v1` |
| `codebase-recon` | produces | `evidence-bounded-recon-report` |
| `codex-exec` | produces | `codex-run-output` |
| `converter` | produces | `converted-skill` |
| `council` | consumes | `explicit-question` |
| `council` | consumes | `evidence` |
| `council` | produces | `council-report.v1` |
| `doc` | consumes | `repo-context` |
| `doc` | produces | `documentation` |
| `domain` | produces | `stdout` |
| `dueling-idea-genies` | consumes | `idea-portfolio.v1` |
| `dueling-idea-genies` | consumes | `task-question` |
| `dueling-idea-genies` | produces | `idea-challenge.v1` |
| `goals` | produces | `result.json` |
| `handoff` | produces | `.agents/handoff/*.md` |
| `idea-genie` | consumes | `repo-context` |
| `idea-genie` | consumes | `task-question` |
| `idea-genie` | produces | `idea-portfolio.v1` |
| `implement` | produces | `subject-manifest.v1` |
| `learn` | consumes | `verdict.v2` |
| `learn` | produces | `learning-observations` |
| `ntm` | consumes | `task-intent` |
| `ntm` | produces | `ntm-robot-state` |
| `ntm` | produces | `agent-worker-transcript` |
| `operationalize` | consumes | `evidence-backed-expertise` |
| `operationalize` | produces | `operationalization-proposal.v1` |
| `pattern-mining` | consumes | `repo-context` |
| `pattern-mining` | consumes | `task-question` |
| `pattern-mining` | produces | `pattern-mining.v1` |
| `postmortem` | consumes | `verdict.v2` |
| `postmortem` | produces | `postmortem-report.md` |
| `premortem` | produces | `premortem-plan-review.v1` |
| `product` | produces | `PRODUCT.md` |
| `reality-check` | consumes | `claim` |
| `reality-check` | consumes | `repository-evidence` |
| `reality-check` | produces | `reality-check-report.v1` |
| `refactor` | consumes | `repo-context` |
| `refactor` | produces | `code-changes` |
| `research` | consumes | `research-question` |
| `research` | produces | `research-report` |
| `reverse-engineer` | produces | `.agents/research/*.md` |
| `rpi` | consumes | `plan` |
| `rpi` | consumes | `implement` |
| `rpi` | consumes | `validate` |
| `rpi` | produces | `rpi-report.v1` |
| `scaffold` | produces | `project-scaffold` |
| `scope` | consumes | `proposed-write-scope` |
| `scope` | produces | `scope-review` |
| `security` | consumes | `repo-context` |
| `security` | produces | `security-report.json` |
| `shared` | produces | `reference-documents` |
| `skill-builder` | produces | `skill-source-package` |
| `skill-builder` | produces | `skill-hygiene-report` |
| `standards` | produces | `stdout` |
| `status` | produces | `stdout` |
| `swarm` | consumes | `explicit-disjoint-packets` |
| `swarm` | produces | `per-packet-results` |
| `test` | consumes | `standards` |
| `test` | consumes | `repo-context` |
| `test` | produces | `result.json` |
| `toil-mining` | produces | `result.json` |
| `using-gc` | consumes | `explicit-packets` |
| `using-gc` | produces | `gas-city-runtime-evidence` |
| `validate` | consumes | `subject-manifest.v1` |
| `validate` | produces | `subject-manifest.v1` |
| `validate` | produces | `verdict.v2` |
| `workflow-builder` | produces | `workflow-script` |
