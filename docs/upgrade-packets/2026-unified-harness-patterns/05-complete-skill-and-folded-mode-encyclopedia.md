# Volume 5: Complete Encyclopedia of All 60+ Skills, Folded Modes & Companion Scripts

> **Scope:** Exhaustive technical reference for all **45 core platform skills** (including their **90+ internal folded `modes/*/mode.md`** and **50+ companion validator scripts**), **8 polyglot robotics skills**, and **14 package-bundled runtime/SDK skills**. All identifiers are sanitized to portable open-source names.

---

## 1. Hexagonal Skill-Tool Registry & Placement Architecture

### 1.1 Resolution Order (`Loop -> Bounded Context -> Port -> Tool`)
To prevent a 60+ skill catalog from degenerating into an unsearchable flat list, the registry enforces a strict 4-layer hexagonal hierarchy (`skill-tool-registry`):
1. **Application Core (Operating Loops):** High-level lifecycle state machines (`CDLC`, `RPI`, `Orchestrate`, `Incident-Triage`, `Knowledge-Ratchet`).
2. **Bounded Contexts (Domain Skills):** Grouped strictly by the **noun the skill mutates or judges** (not the nouns it merely reads).
3. **Ports (Named Capabilities):** Abstract contracts (`validate-diff`, `query-graph`, `diagnose-workload`, `stage-wave`).
4. **Driven Adapters (CLI & Host Tools):** Concrete `ao` / CLI verbs and host-broker extension tools.
- **MECE / DRY Invariant:** Every in-domain skill appears **exactly once**—either as Loop Machinery (§1) or as a Bounded-Context Domain Capability (§2). `registry-check` and `skill-cli-audit` enforce this mechanically (exit `4` on duplicate or orphan entries).

### 1.2 The 5-Question Placement Gate (`skill-maintenance`)
Whenever a new rule, workflow, or tool is proposed, five ordered questions decide its home:
1. **Does it require host credentials, network access, or subagent brokering?** $\to$ **Host Extension Tool (Locus 3)**.
2. **Must it run automatically on every subagent spawn, tool result, or commit gate across all repos?** $\to$ **Software Factory Hook / Gate (`oracle-gate`, `sqz`)**.
3. **Is it a deterministic CPU/AST/graph/YAML computation over local files?** $\to$ **Compiled CLI Compute Kernel (Locus 2)**.
4. **Is it a repository-local code invariant anchored to specific `file:symbol` definitions?** $\to$ **`.agents/corpus/{domains,topics,decisions}/`**.
5. **Is it a multi-step human/agent judgment procedure, coaching stance, or domain playbook?** $\to$ **Folded-Mode Skill (`SKILL.md` + `modes/*/mode.md`, Locus 1)**.

---

## 2. Core Platform Skills (45 Root Skills + 90+ Folded Modes)

### 2.1 Session, Onboarding & Human-Collaboration Skills (5 Skills)

#### 1. `i-have-adhd` (Tier: `session` · Role: `supporting`)
- **Folded Modes:** None (always-loaded session shaper).
- **Artifact Edge:** `consumes: [artifact-caps-and-examples, session-turn-state, operator-prompt]` $\to$ `produces: [action-first-response]`.
- **Algorithm & Hard Caps:**
  1. Line 1 must be the direct answer, command, `file:line` edit, or verdict (zero preamble/closer fluff).
  2. Number multi-step work (`<= 5` items per visible group; rank highest-leverage first).
  3. Open multi-turn work with a 1-line state restatement (`Step 2/4 done...`).
  4. Use concrete time units (`~3 mins`), never vague prose.
  5. Enforce brevity caps: bullets `<= 120` chars (1 line); Gherkin `3–4` steps (`<= 90` chars/step); issue need `1–2` sentences (`<= 40` words); PR background `1` sentence (`<= 120` chars).
  6. End open turns with **one `< 2 min` next action** (or separate minimal `host timeout=N` blocks).
  7. `"wait what"` recovery: re-pitch in `3–5` plain-English sentences grounding domain terms before use.

#### 2. `paired-onboarding-101` (Tier: `session` · Role: `driving-adapter`)
- **Folded Modes:**
  - `onboarding-brief`: Fast 5-minute pair orientation and safety boundary check.
  - `onboarding-flow`: Full checkout-neutral install $\to$ identity verification $\to$ first bounded task $\to$ evidence receipt $\to$ update/rollback loop.
- **Artifact Edge:** `consumes: [promotion-path, safety-canon, assurance-refs]` $\to$ `produces: [paired-session-contract, install-identity-receipt, handoff]`.

#### 3. `pair-mob-workflow` (Tier: `session` · Role: `domain`)
- **Folded Modes:**
  - `handoff`: Generates the **4-Line Cross-Pair Behavior Boundary** (`[Handoff | Decision | Help / Andon | FYI]` $\to$ `Behavior / Why + proof boundary / Next revisable behavior / Ask`, `<= 80` words, zero machine-specific paths).
  - `pairing-brief`: Pre-session alignment on driver/navigator rotation and single-slice goal.
  - `review-session-packet`: Synchronous pair review guide over a candidate diff.
  - `team-context-packet`: Converts solo spike exploration into `local proof -> lean issue comment -> pairing agreement -> branch/PR` so teammates never reverse-engineer a surprise diff.
- **Artifact Edge:** `consumes: [proposal, diff, evidence, proof-gap-guidance]` $\to$ `produces: [team-comment, pairing-brief, publication-gate]`.

#### 4. `daily-queue-debrief` (Tier: `session` · Role: `supporting`)
- **Folded Modes:** None.
- **Artifact Edge:** `consumes: [mr-list, diff, ci-status, review-threads]` $\to$ `produces: [team-debrief, mr-status-summary]`.
- **Algorithm:** Fetches bounded active PRs, classifies each into `Merged`, `Ready for Review and Merge`, or `In Progress`, and formats 1 line per PR leading with the observable behavior changed + 1-phrase *why* (omitting empty sections).

#### 5. `tmux-governed-session` (Tier: `session` · Role: `driven-adapter`)
- **Folded Modes:** None.
- **Artifact Edge:** `consumes: [terminal-mux-api, governed-runtime, skill-catalog]` $\to$ `produces: [isolated-pane-session, governance-load-proof, fault-layer-diagnosis]`.
- **Algorithm:** Creates an isolated user pane, targets the agent process via deterministic pane ID, verifies governance-extension and skill-catalog initialization from live output, and isolates faults across 5 layers (mux, harness, governance extension, model gateway, installer).

---

### 2.2 Orchestration, Lifecycle & Work-Graph Skills (5 Skills)

#### 6. `pi-orchestrate` (Tier: `orchestration` · Role: `driving-adapter`)
- **Folded Modes:**
  - `intake-triage`: Converts raw bug reports, alerts, or incident threads into bounded, agent-ready execution briefs.
  - `work-graph`: Decomposes multi-part initiatives into dependency-linked DAGs with provably disjoint `write_scope[]` globs.
- **Artifact Edge:** `consumes: [accepted-intent, subagent-broker-tools, native-work-state]` $\to$ `produces: [bounded-worker-results, integrated-answer]`.
- **Algorithm:**
  1. **Drain Before Starting:** Cap external review WIP at `1–2` open PRs per pair; free capacity drains existing review/check/repair queues before starting new slices.
  2. **Parallel Reconnaissance:** Fan out `>= 3` distinct `explore` scopes in one parallel call for decomposable research.
  3. **Zero-Token Read $\to$ Write Budget:** Workers read `<= 3` files before making their first edit + test run.
  4. **Strict 1->5 Phase Transitions:** (1) Continue in-place $\to$ (2) Clear/reset $\to$ (3) 4-line portable handoff $\to$ (4) Delegate to subagent $\to$ (5) `/compact` before `150k` tokens (preventing `170k–200k` gateway drops).

#### 7. `cdlc` (Complete Development Lifecycle · Tier: `orchestration` · Role: `driving-adapter`)
- **Folded Modes:**
  - `intent-discovery`: Socratic discovery of system boundaries, stakeholders, and failure modes when intent is unsettled.
  - `intent-shaping`: Converts discovered intent into a Walking Skeleton issue and first vertical slice packet.
- **Artifact Edge:** `consumes: [lifecycle-templates, assurance-inputs, issue-state]` $\to$ `produces: [work-packet, bdd-slice, wave-plan]`.
- **Algorithm:** Enters at the current work state rather than replaying waterfall phases; binds every implementation slice to an assurance lane and folds post-merge learnings back into the tracking issue.

#### 8. `beads` (Tier: `orchestration` · Role: `driven-adapter`)
- **Folded Modes:** None.
- **Artifact Edge:** `consumes: [work-intent, bead-cli-contract]` $\to$ `produces: [bead-slice, ready-frontier, dependency-graph, provenance-log]`.
- **Algorithm:** Enforces Walking Skeleton integer slicing (`1..N`), separates *Tidy First* structural beads from behavioral beads, requires `write_scope[]` + `check_commands[]` + Given/When/Then acceptance with negative invariant on every bead, tracks lease heartbeats, and requires `close_with_evidence` bound to `HEAD`.

#### 9. `explicit-rpi` (Research-Plan-Implement-Validate · Tier: `orchestration` · Role: `driving-adapter`)
- **Folded Modes:**
  - `repo-retrieve`: Bounded source retrieval into an immutable context packet.
  - `plan`: Produces an execution plan bound to exact target paths and checks.
  - `implement`: Executes the plan in an isolated checkout.
  - `validate`: Produces `subject-manifest.v1` and fresh author-distinct `verdict.v2` (`PASS | FAIL | NOT_PROVEN`) using atomic `fsync`ed file journals (`validate.py:412-443`).
- **Companion Scripts:** `scripts/run_once.py`, `scripts/validate.py`, `scripts/validate.sh`, `scripts/test_validate.py`.

#### 10. `sealed-council` (Tier: `orchestration` · Role: `domain`)
- **Folded Modes:**
  - `council-profile-builder`: Composes orthogonal reviewer lenses (e.g., DDD/Hexagonal, SRE/Resilience, Security/Boundary, Minimalism/YAGNI) tailored to the artifact under review.
- **Artifact Edge:** `consumes: [candidate-artifact, shared-evidence, rubric]` $\to$ `produces: [sealed-council-report, dissent-ledger, evidence-gaps]`.

---

### 2.3 Judgment, Assurance & Anti-Gaming Skills (8 Skills)

#### 11. `just-say-no-to-process-porn-and-ceremony` (Tier: `judgment` · Role: `domain`)
- **Folded Modes:** None (uses 3 reference manuals + 3 audit worksheets).
- **References & Assets:** `references/ENFORCEMENT.md`, `references/HONESTY-PROTOCOL.md`, `references/REWARD-HACKING-CATALOG.md`, `assets/process-porn-worksheet.md`, `assets/real-work-audit-worksheet.md`, `assets/honesty-inventory.md`.
- **Algorithm:** Executes the Product-vs-Process test (*Does production or gate code branch on this artifact?*), enforces the 4-Part Creation Gate (`Consumer / Gate / Observed Defect / Retirement Trigger`), enforces the Honest-Credit Floor (`RH-1..17`, `PL-1..5`), and triggers an immediate **Machinery Freeze** whenever governance/meta commits outpace feature delivery.

#### 12. `software-review` (Tier: `judgment` · Role: `domain`)
- **Folded Modes (6 Modes):**
  - `advisory`: Ranked `file:line` defect and design-improvement findings across the 4 Adversarial Lenses (`implementation-risk`, `docs-consistency`, `security`, `test-signal`).
  - `acceptance`: Fresh author-distinct `PASS | FAIL | NOT_PROVEN` judgment where `not_checked` must be empty for `PASS`.
  - `reality-check`: Audits a `"done / ready to ship"` claim against actual repository bytes, git status, and check receipts.
  - `security`: Trust-boundary, CWE-22 path containment, secret exposure, subprocess execution, and dependency audit.
  - `council`: Sealed multi-perspective synthesis preserving minority dissent.
  - `interface-ergonomics`: Audits CLI/API/tool discoverability (`--help`, `capabilities --json`), `stdout`/`stderr` separation, exit codes, and error recovery from an agent caller's perspective.

#### 13. `promotion-gate` (Tier: `judgment` · Role: `domain`)
- **Folded Modes:**
  - `promotion-graduation`: Evaluates whether locally validated work has the evidence, pair sign-off, and explicit per-destination human authorization required to publish or promote across branches/environments.

#### 14. `assurance-case` (Tier: `judgment` · Role: `domain`)
- **Folded Modes (8 Modes):**
  - `evidence-manifest-audit`: Verifies that cited evidence files, SHAs, and logs exist and match their claims.
  - `packet-audit`: Deterministic structural audit of issue/MR assurance packets (`scripts/audit-packet.py`).
  - `premortem`: Assumes the change caused a production incident 30 days post-merge and works backward to identify missing bulkheads or rollback traps.
  - `proof-gap-finder`: Separates *validated* claims from *asserted* claims (`VA-1..4` and `SA-1..3`).
  - `proof-gate`: Evaluates whether the selected validation lane covers the blast radius.
  - `regulated-review`: Checks compliance, air-gap, classification, and authority constraints.
  - `simulation-ladder`: Selects the fastest faithful validation rung (static $\to$ dry-run $\to$ container/VM lab $\to$ live canary).
  - `team-assurance`: Summarizes Claim $\to$ Evidence $\to$ Residual Risk for human pair sign-off.
- **Companion Script:** `modes/packet-audit/scripts/audit-packet.py`.

#### 15. `agent-governance` (Tier: `judgment` · Role: `domain`)
- **Folded Modes:**
  - `agent-identity-design`: Designs least-privilege service accounts, scoped tokens, RBAC roles, and human-escalation gates for automated agents.

#### 16. `mr-advisory-review` (Tier: `judgment` · Role: `domain`)
- **Folded Modes (6 Modes):**
  - `mr-inline-review`: Crafts line-tied diff discussions using Giraffe NVC (`Observation -> Impact/Need -> Request/Question`, `<= 60` words).
  - `mr-review-brief`: 15-second reviewer briefing (behavior, 6-part SHA identity, kernel verdict, and focus areas).
  - `mr-scope`: Audits whether an MR mixes multiple behaviors or crosses layer boundaries (`scripts/check-mr-scope.py`).
  - `pre-submit-review`: Runs the full local pre-submit gate suite (`scripts/pre-submit.sh`) before requesting human review.
  - `mr-scope-split`: Partitions an oversized multi-concern branch into a dependency-ordered sequence of single-behavior MRs.
  - `weekly-merge-review`: Retrospective audit of merged MRs for drift, rework, or escaped defects.
- **Companion Scripts:** `modes/mr-scope/scripts/check-mr-scope.py`, `modes/pre-submit-review/scripts/pre-submit.sh`.

#### 17. `layered-platform-change` (Tier: `judgment` · Role: `domain`)
- **Folded Modes (5 Modes):**
  - `disconnected-delivery`: Verifies air-gap artifact bundle closure and offline transport parity.
  - `ansible-ci-slice`: Maps changed roles/playbooks to exact CI stages and tag selectors.
  - `local-ansible-preflight`: Runs syntax, task-list, and real-inventory dry-run checks locally before pushing.
  - `molecule-functional-test-ci`: Designs and validates container/VM functional role tests.
  - `platform-boundary`: Enforces the 4-layer platform ownership boundary (`scripts/advise-layers.sh`).
- **Companion Scripts:** `scripts/advise-layers.sh`, `check-airgap.py`, `check-slice.py`, `check-functional-test.py`.

#### 18. `team-canon-calibration` (Tier: `judgment` · Role: `supporting`)
- **Folded Modes:**
  - `team-canon-framing`: Distinguishes battle-tested team survivor patterns and recurring bug classes from unratified personal preferences.

---

### 2.4 Core Software Engineering Skills (6 Skills)

#### 19. `software-design` (Tier: `product` · Role: `domain`)
- **Folded Modes (5 Modes):**
  - `deep-modules`: Evaluates interfaces using Ousterhout's Deep Modules, Design It Twice, the 4-Category Seam Taxonomy, Zero-Token Control/Data Plane split, Tricorder semantic-vs-attribution scope, and the `SYS-01..05` cost model.
  - `domain-modeling`: Establishes Bounded Contexts, Ubiquitous Language glossaries, `_Avoid_` synonyms, and 3-part-filtered ADRs.
  - `interview`: Socratic 1-question-at-a-time requirement grilling for human-held preferences.
  - `options`: Generates 2–4 genuinely distinct architectural options (including `"do nothing"`) ranked by reversibility and system fit.
  - `premortem`: Failure-mode analysis separating goal defects from approach defects.

#### 20. `software-delivery` (Tier: `execution` · Role: `domain`)
- **Folded Modes (5 Modes):**
  - `tdd`: Port-boundary `Red -> Black -> Refactor` execution preserving test oracles.
  - `testing`: Designs discriminating positive + boundary/negative tests at public Port seams without brittle internal change-detector mocks.
  - `refactor`: *Tidy First?* + Feathers Characterization Pinning + Isomorphism Proof + Opportunity Matrix ($\text{Score} \ge 2.0$).
  - `prototype`: Builds isolated throwaway spikes to answer a specific interface/behavioral question without polluting production paths.
  - `merge-conflicts`: Resolves rebase/merge conflicts by recovering commit intent on both branches and running covering checks.
- **Reference Manual:** `references/polyglot-idioms-and-reuse-first.md` (3 Universal Laws, 8-Language Idiomatic Matrix, Gauntlet Rust Patterns `200..240`).

#### 21. `software-investigation` (Tier: `execution` · Role: `domain`)
- **Folded Modes (3 Modes):**
  - `diagnosis`: Competing-hypothesis root-cause investigation (`Observation / Inference / Contradiction / Unknown`) with discriminating probes.
  - `pattern-research`: Bounded codebase reconnaissance verifying dynamic dispatch, reflection, registries, and re-exports before claiming symbol absence.
  - `reverse-engineering`: Reconstructs architecture, state machines, and wire contracts from unfamiliar or legacy repositories.

#### 22. `minimalism` (Tier: `judgment` · Role: `supporting`)
- **Folded Modes:** None.
- **Algorithm:** Applies the **8-Rung Simplification Ladder** (Do nothing $\to$ Change input/reuse owner $\to$ Stdlib $\to$ Platform primitive $\to$ Installed dependency $\to$ Short local expression $\to$ Minimal new code $\to$ Narrowest interface) and proves reachability/caller closure before deleting dead code.

#### 23. `idea-wizard` (Tier: `product` · Role: `supporting`)
- **Folded Modes:** None.
- **Algorithm:** Surveys repository friction, corpus staleness, and telemetry to propose `3–5` evidence-backed high-leverage improvements (always including `"no work"`), creating tracker slices only after human selection.

#### 24. `interactive-wizard` (Tier: `execution` · Role: `driving-adapter`)
- **Folded Modes:** None.
- **Algorithm:** Generates checkpointed, idempotent shell/UI runbooks for human-only operations (credential rotation, cutovers, break-glass recovery) with preflight verification, point-of-effect confirmation prompts, and exact rollback commands.

---

### 2.5 Knowledge, Documentation & Control-Plane Skills (6 Skills)

#### 25. `learning` (Tier: `knowledge` · Role: `domain`)
- **Folded Modes (3 Modes):**
  - `lesson`: Distills an observed bug or counterexample into a code-anchored corpus rule (`corpus_ratify`) or deterministic lint check.
  - `postmortem`: Blameless causal analysis separating proximate trigger, contributing conditions, proof gaps, and mechanized prevention.
  - `session-coach`: Runs `coach` JSONL analytics to identify tool-loop friction, context-bloat outliers (`ACI-01`), and 3-gram crystallization candidates.

#### 26. `documentation` (Tier: `knowledge` · Role: `domain`)
- **Folded Modes (3 Modes):**
  - `agent-instructions`: Authors and audits `AGENTS.md`, `CLAUDE.md`, and `SKILL.md` files using context pointers and progressive disclosure.
  - `handoff`: Produces portable 4-line cross-pair handoffs (`<= 80` words, zero machine-local paths).
  - `specification`: Writes source-verified outcome/interface/acceptance specs and executes `noslop` plain-prose checks.

#### 27. `skill-maintenance` (Tier: `meta` · Role: `domain`)
- **Folded Modes (7 Modes):**
  - `external-skill-admission`: Audits third-party skills for security, license, token bloat, and profile fit before admission.
  - `skill-auditor`: Checks frontmatter contracts, trigger quality, and reference freshness.
  - `skill-code-sweep`: Incremental repository-to-skill knowledge refresh engine (`scripts/knowledge_refresh.py`, `clean_parallel.py`, `reconcile_legacy_claims.py`).
  - `governed-skill-authoring`: Packaging and `uv` dependency warming for runtime-bundled skills.
  - `headless-validation`: Runs automated headless agent evaluation suites (`--dangerously-skip-permissions` only in isolated QA sandboxes; verifies `ctx.abort()`-before-`throw` egress ordering).
  - `skill-authoring`: Scaffolds compact root `SKILL.md` + folded `modes/*/mode.md`.
  - `skill-evaluation`: Runs positive trigger, negative trigger, and behavioral compliance evals.

#### 28. `skill-tool-registry` (Tier: `meta` · Role: `supporting`)
- **Folded Modes:** None.
- **Algorithm:** Maintains the canonical `Loop -> Bounded Context -> Port -> Tool` registry and verifies the MECE/DRY single-home invariant (`pal registry-check`).

#### 29. `pi-extension-manager` (Tier: `meta` · Role: `driven-adapter`)
- **Folded Modes:**
  - `extension-conformance`: Audits TypeScript host extensions for dual mode-gating (`Regulated` vs `Commercial`), shell-free `execFile`, output redaction, and TypeBox schema completeness.

#### 30. `pi-runtime-operations` (Tier: `meta` · Role: `driven-adapter`)
- **Folded Modes (3 Modes):**
  - `agent-operations`: Manages interactive, headless, and background agent sessions (`scripts/pi-agent-session.sh`).
  - `audit-forensics`: Diagnoses audit logs, `loop_guard_armed / nudge / blocked / hard_stop` events, and egress blocks.
  - `workspace-classification`: Audits strictest-wins repository/remote tier classification and COM lockdown mounts.

---

### 2.6 Infrastructure, GitOps, Observability & Operations Skills (15 Skills)

| Skill (Sanitized) | Folded Modes (`modes/*/mode.md`) | Companion Scripts & Core Engineering Pattern |
| --- | --- | --- |
| **31. `cluster-diagnose`** | **16 Folded Modes:** `artifact-remote-recovery`, `chronic-vs-acute-diagnosis`, `smart-cache-cleanup`, `registry-canary`, `host-write-escalation`, `readonly-host-inspection`, `cluster-capacity`, `postgres-operator-resolution`, `deployment-proof`, `host-exec-policy-debugger`, `flux-reconcile-debugger`, `object-store-cleanup`, `grafana-dashboards`, `mr-deploy-watch`, `storage-class-migration`, `worker-node-maintenance` | **Scripts:** `artifactory-smart-cache-cleanup.py` (AQL transitive closure + pull-replication disable), `migrate-pvc-storage.py` + `storage_migrator/` (atomic `fsync`ed journal PVC migration), `check-grafana-dashboards.py`, `flux_reconcile_probe.sh`, `probe-exec-policy-denials.sh`, `serial-worker-maintenance.sh`, `watch_mr_deploy.sh`. **Pattern:** Multi-plane workload-identity correlation with explicit confidence (`proven / likely / possible / unknown`). |
| **32. `code-lint-and-trace`** | **8 Folded Modes:** `source-trace`, `ansible-responsibility-map`, `ansible-style`, `ci-style`, `container-image-style`, `inventory-vars-style`, `python-style`, `architecture-audit` | **Scripts:** `repo-lint.py` (`--fix` `SuggestedFix` applier), `check-ansible-responsibility-map.py`, `check-ansible-style.py`, `check-ci-style.py`, `check-inventory-style.py` (`136x` incremental equivalence-class parity join), `check-python-style.py` (`S-PY1..S-PY6` AST + `.sh` heredoc scanner). |
| **33. `mr-worktree-lifecycle`** | **4 Folded Modes:** `gitlab-proposal`, `branch-prune-planner`, `mr-diff-debugger`, `team-mr-flow` | **Script:** `scripts/check-worktree-hygiene.sh`. **Pattern:** Canonical worktree slot state machine (`issue-<IID>/mrs/mr-<MR-IID>/`), pre/post topology gates, and stale empty-diff recovery. |
| **34. `lean-gherkin-issue`** | **2 Folded Modes:** `bdd-gherkin-coach`, `issue-from-context` | **Scripts:** `scripts/check-packet-shape.py`, `modes/bdd-gherkin-coach/scripts/check-gherkin.py`. **Pattern:** 15-second Two-Layer Progressive Disclosure (`Need` <= 40 words + 1 Gherkin scenario + 1-MR-at-a-time scoreboard + `<details>` fold-back drawer). |
| **35. `incident-learning`** | **5 Folded Modes:** `decision-record`, `learn`, `postmortem`, `postmortem-mob-packet`, `postmortem-skill-feedback` | **Script:** `modes/decision-record/scripts/check-adr.py`. **Pattern:** Out-of-critical-path causal analysis routing recurring defects into mechanized CLI/linter rules. |
| **36. `source-graph-query`** | None | **Pattern:** Authenticated source-only graph queries (`pal graph`) separating static topology reachability from live runtime proof. |
| **37. `umbrella-helm-values`** | None | **Pattern:** Traces umbrella chart values, shared common-values overlays, and Flux `HelmRelease` ownership separately from air-gap transport. |
| **38. `airgap-image-closure`** | None | **Pattern:** Computes **selected render-derived image closure** (images actually referenced by rendered manifests for enabled packages) vs. upstream cache closure. |
| **39. `k8s-airgap-bundle-deploy`** | **1 Folded Mode:** `deployment-flow-map` | **Script:** `scripts/check-bundle-config.py`. **Pattern:** Preserves proof boundaries across 6 deployment stages (collection $\to$ portable artifact $\to$ zone delivery $\to$ GitOps reconciliation $\to$ pod readiness $\to$ storage durability). |
| **40. `sops-value-blind-secrets`** | **1 Folded Mode:** `secret-materialization` | **Script:** `scripts/check-sops-style.py`. **Pattern:** Value-blind `.sops.yaml` recipient-tier symmetry analysis (`pal sops-tier`) without decrypting secret values; separates Git secret materialization from live DB/IdP credential rotation. |
| **41. `ci-rehearsal-safety`** | **2 Folded Modes:** `release-tag-flow`, `no-publication-or-batch-rehearsal` | **Pattern:** Traces tag/mirror child/trigger pipelines and designs safe rehearsals that exclude artifact publication and batch job spawning at **rule-evaluation time** rather than relying on manual job cancellation. |
| **42. `stateful-data-migration`** | **3 Folded Modes:** `pg-migrate`, `redis-migrate`, `s3-migrate` | **Pattern:** Offline archive-verified backup/restore across PostgreSQL, Redis, and S3/MinIO with explicit partial-failure classification. |
| **43. `disk-hygiene`** | None | **Pattern:** Measure-first disk pressure forensics; classifies active/irreplaceable state (worktrees, databases, local archives) vs. disposable caches before proposing minimum-recovery-cost cleanup. |
| **44. `k8s-node-roll`** | None | **Pattern:** Resumable pool-based cordon/drain/reboot/readiness campaigns using K8s node annotations as portable campaign leases and checking PDBs/emptyDirs/unmanaged pods first. |
| **45. `libvirt-k8s-vm-lab`** | None | **Pattern:** 4-layer local Linux/KVM Kubernetes rehearsal (`L1 OS -> L2 K8s -> L3 Registry/GitOps -> L4 Platform Slices`) with SHA-256 snapshot input hashes and dry-run defaults. |

---

## 3. Polyglot Robotics & Package-Bundled Skills (22 Skills)

### 3.1 Repository-Adaptive Polyglot Robotics Skills (8 Skills)

| Skill (Sanitized) | Target Surfaces | Encoded Invariants & Verification Protocol |
| --- | --- | --- |
| **46. `cpp-realtime-concurrency`** | C++17 Middleware Blocks, Callbacks, Timers | Enforces constructor member initialization (`BA104`), callback/timer mutex lock symmetry and non-recursive re-entry prevention (`BA103`), copy-under-lock + publish-outside-lock, upstream `FAILURE` propagation, headless GDB batch backtraces for hangs, and seeded RNG simulation isomorphism. |
| **47. `polyglot-architecture-map`** | 6 Execution Spines (C++, IDL, CUE, Ontology, Analytics, Packaging/CI) | Routes changes to the single owning spine; enforces the 3 Universal Engineering Laws (Anti-NIH, Zero Transliteration, Isomorphic Refactoring $\ge 2.0$) and produces the 6-plane Cross-Layer Contract Dossier (`mr_validate`). |
| **48. `batch-ci-diagnostics`** | K8s Batch Pods, batch job queueing, Prometheus, Loki, Tempo | Queries observability planes over zero-port-forward K8s `services/proxy` (`robotics_k8s_diagnose`); classifies failures into 4 accountable owners (`Dev Code`, `Dev Config`, `Simulation Framework`, `IT Platform`); enforces 3-pass convergence (`mr_validate -> ubs -> cross-file seam audit`). |
| **49. `dind-delivery-packaging`** | Multi-Stage Dockerfiles, Rootless DinD, Helm/Release Bundles | Blocks reverse imports from modern delivery packages into deprecated legacy trees (`BA303`), verifies multi-stage `Dockerfile` runtime import closure (`BA305`), enforces unique multi-agent `agent_id`s, and requires local package smoke testing since release CI runs on tags only. |
| **50. `polyglot-setup`** | Workstation Toolchain, Credentials, Analyzers | 6-checkpoint idempotent installer with `--doctor` and `--undo`: configures HTTPS Git credential helpers, installs skills + generic workbench, verifies `ast-grep`/`cppcheck`/`cue`, caps Conan/CMake jobs at `-j2` (preventing workstation OOM freezes), and runs read-only `S01..S12` OIDC/CA kubeconfig doctor checks. |
| **51. `ontology-migration-bridge`** | Legacy vs. Modern Python Domain Ontologies & Message Logs | Enforces `BA302` (forbids mixing legacy `LegacyTable` types with modern `ModernTable + ScenarioAnalyzer` objects in the same execution path) and requires raw message-log timeline triage (`robot-log-analysis`) before debugging high-level evaluators. |
| **52. `cue-platform-overlay-matrix`** | CUE Schemas, 5 Platform Overlays, K8s Batch Specs | Requires overridable base CUE fields to use default unification (`*"NONE" \| string`, `BA202`), enforces 5-platform overlay parity (`BA203`), C++ $\leftrightarrow$ CUE port parity (`BA204`), and the 41-char K8s Indexed Job name budget (`BA401`). |
| **53. `vertical-action-slice`** | Cross-Layer Feature Slices (IDL $\to$ C++ $\to$ CUE $\to$ Python $\to$ Tests) | Enforces strict bottom-up contract order: (1) validate `.idl` IDL schema first, (2) implement pure callback-injected C++ handler core, (3) wrap in thin non-publishing constructor mixin, (4) wire CUE `params` with byte-identical port names, (5) run `robotics_preflight(mode="impact")` + `robotics_ctest_slice`. |

### 3.2 Package-Bundled Runtime, SDK & Specialized Skills (14 Skills)

| Skill (Sanitized) | Owning Package | Reusable Engineering Pattern |
| --- | --- | --- |
| **54. `conan-cmake-integration`** | SDK Workbench | Manages `conan.lock`, `CMakeDeps`/`CMakeToolchain`, `VirtualRunEnv` activation, and runtime plugin library paths. |
| **55. `cue-language-guide`** | SDK Workbench | Reference grammar and unification laws (`&`, `\|`, `*default`, closed structs, comprehension guards) for CUE schemas. |
| **56. `middleware-block-guide`** | SDK Workbench | Lifecycle contracts (`init`, `start`, `step`, `stop`), communicator pub/sub wiring, and thread-pool assignment for event-driven blocks. |
| **57. `simulation-scenario-guide`** | SDK Workbench | Scenario composition, scenario evaluation metrics, and workspace configuration for deterministic simulation runs. |
| **58. `behavior-tree-guide`** | SDK Workbench | Task arbitration, heuristic assignment modifiers, and behavior-handler unit testing patterns. |
| **59. `sdk-userguide`** | SDK Workbench | Progressive-disclosure index over versioned SDK documentation without loading full manuals into context. |
| **60. `cpp-realtime-safety-critical-generic`** | Robotics Workbench | Generic, organization-neutral counterpart of `BA101..BA108` C++17 concurrency, initialization, and lifecycle checks. |
| **61. `cue-overlay-schema-generic`** | Robotics Workbench | Generic counterpart of `BA201..BA204` and `BA401` CUE default unification, port parity, and DNS-1123 job naming checks. |
| **62. `extension-security-model`** | Governance Core | Security checklist for authoring host extensions: mode-gating, `bwrap` delegation, fixed-argv `execFile`, secret redaction, and avoiding raw `spawn("pi")` breakouts. |
| **63. `skill-dependency-authoring`** | Governance Core | Patterns for bundling helper scripts and `uv`-managed Python virtual environments inside skills with lazy `warm_skill_dependencies` caching. |
| **64. `tls-ca-troubleshooting`** | Governance Core | Diagnoses and repairs Node.js / Python / Git enterprise CA trust chains (`NODE_EXTRA_CA_CERTS`, `REQUESTS_CA_BUNDLE`). |
| **65. `extension-url-migration`** | Governance Core | Rewrites legacy package remote URLs and updates cached Git remotes in place without losing local state. |
| **66. `robot-log-timeline-analysis`** | Governance Core | Pure-Python reader for robotics message-log archives without the robotics framework installed (topic inventory, message frequency, byte size, trajectory & frame extraction). |
| **67. `cold-read-pedagogy`** | Onboarding Lab | Withholds AI code explanation until the learner completes a **Variable State Card**, traces boundary inputs (`0.0 / None / [] / NaN`), and writes a falsifiable probe; gates outward notes with `sanitize_and_export_check.py`. |
