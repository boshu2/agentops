export const meta = {
  name: 'ship-beads',
  description: 'Drive/ship a list of beads to confirmed-merged with worktree-isolated implementer subagents + orchestrator-only reconcile (sequential for coupled surfaces, parallel for disjoint)',
  whenToUse: 'When you have a list of bead ids to drive/ship to merged PRs in one autonomous session. Pass args = {beads:[...ids], mode:"parallel"|"sequential"}. mode is a hint; the PLAN-GRAPH phase demotes a "parallel" request to a sequential sub-chain whenever two beads share a write/derived surface. Codifies the 2026-05-31 ship-beads post-mortem (§6): worktree-isolated subagents, orchestrator-only merge, mechanical reconciliation of the registry/codex/security/flake taxes, and a confirmed-MERGED-before-br-close gate. (Additive rename of the legacy bead-crank workflow — "crank" was jargon; this drives beads to merged.)',
  phases: [
    { title: 'Prelude', detail: 'git reset --hard origin/main (NEVER pull --rebase); validate every bead exists + unclaimed; run the acceptance ADMISSION gate per bead (fail-closed); classify requested mode' },
    { title: 'Plan-Graph', detail: 'build the bead DAG; surface-collision detector demotes parallel→sequential for coupled surfaces; emit the wave plan' },
    { title: 'Fan-Ship', detail: 'parallel: spawn a wave of implementer agents (cap 4-6) in isolated worktrees; sequential: one agent, confirm-merge, branch next off the freshly-merged SHA' },
    { title: 'Reconcile', detail: 'per-PR state machine: poll → flake-rerun-once → fix-forward derived surfaces → squash-merge --admin → confirm MERGED → br close' },
    { title: 'Epilogue', detail: 'close a parent epic ONLY when every child is confirmed-merged; prune merged worktrees; git -C _beads push + git push; write br comment learnings' },
  ],
}

// ===========================================================================
// ARGS — {beads:[ids], mode:'parallel'|'sequential', epic?:id}
// `mode` is the REQUESTED mode; PLAN-GRAPH may demote parallel→sequential when a
// surface-collision is detected. `epic` (optional) is the parent epic closed in
// EPILOGUE only after every child is confirmed-merged.
// ===========================================================================
const beadIds = (args && Array.isArray(args.beads) ? args.beads
  : args && typeof args.beads === 'string' ? args.beads.split(',').map((s) => s.trim()).filter(Boolean)
  : []).filter(Boolean)
const requestedMode = args && args.mode === 'parallel' ? 'parallel' : 'sequential'
const epicId = (args && (args.epic || args.epicId)) || null

// Parallel fan-out is capped 4-6 to bound CI contention + worktree sprawl
// (post-mortem §6 ordering rule). Pick the cap from the requested wave size,
// clamped to [4,6]; a wave never exceeds 6 even if more disjoint beads exist.
const PARALLEL_CAP = Math.min(6, Math.max(4, beadIds.length || 4))
// Known-flake rerun budget: exactly ONE rerun so a real break can't be masked.
const FLAKE_RERUN_BUDGET = 1

const DOCTRINE = `You are an agent inside the AgentOps ship-beads orchestration workflow. Governing repo doctrine: every change to main is a PR; every PR cites a bead; the PR is the atomic-revert unit. Worktree-isolated subagents implement; the orchestrator alone merges. Anchor domain terms to skills/domain/references/. Read docs/architecture/operating-loop.md only if you need exact gate definitions.`

// ---------------------------------------------------------------------------
// SCHEMAS — structured JSON is what makes the workflow resumable + deterministic.
// The post-mortem's #1 reliability lesson: batch `bd --json` status was flaky
// (null/0), so the workflow carries its OWN structured state, never trusting a
// single bd batch query for control flow.
// ---------------------------------------------------------------------------

// PLAN-GRAPH output: the wave plan + per-bead surface fingerprint.
const PLAN_GRAPH_SCHEMA = {
  type: 'object',
  required: ['mode', 'waves', 'beads'],
  properties: {
    mode: { type: 'string', enum: ['parallel', 'sequential'], description: 'EFFECTIVE mode after surface-collision demotion' },
    demoted: { type: 'boolean', description: 'true if a requested parallel run was demoted to sequential' },
    demotionReason: { type: 'string', description: 'which shared surface forced sequential, if demoted' },
    beads: {
      type: 'array',
      minItems: 1,
      items: {
        type: 'object',
        required: ['beadId', 'touchedSurfaces'],
        properties: {
          beadId: { type: 'string' },
          touchedSurfaces: {
            type: 'array',
            description: 'likely-touched surfaces from bead metadata + scenario files: e.g. cli-command-surface, registry, codex, context-map, a SKILL path, a cli/internal pkg, shared fixtures',
            items: { type: 'string' },
          },
          dependsOn: { type: 'array', items: { type: 'string' }, description: 'bead ids this bead must merge after (DAG edges)' },
        },
      },
    },
    waves: {
      type: 'array',
      minItems: 1,
      description: 'Execution waves in order. A wave with >1 bead is a PARALLEL fan-out (provably disjoint surfaces). A single-bead wave is a sequential link.',
      items: {
        type: 'object',
        required: ['beadIds', 'parallelizable', 'disjointCheck'],
        properties: {
          beadIds: { type: 'array', items: { type: 'string' } },
          parallelizable: { type: 'boolean', description: 'true only if every disjointCheck row passes AND beadIds.length<=cap' },
          disjointCheck: {
            type: 'object',
            description: 'surface-collision rows; each true=disjoint=safe to parallelize',
            required: ['distinctWriteScopes', 'noSharedDerivedSurface', 'noSharedCliSurface', 'noSharedSkill', 'noSharedFixtures'],
            properties: {
              distinctWriteScopes: { type: 'boolean' },
              noSharedDerivedSurface: { type: 'boolean', description: 'no two beads both regenerate registry.json / cli-command-surface / context-map / codex hashes' },
              noSharedCliSurface: { type: 'boolean' },
              noSharedSkill: { type: 'boolean', description: 'no two beads edit the same skills/**/SKILL.md' },
              noSharedFixtures: { type: 'boolean' },
            },
          },
        },
      },
    },
  },
}

// Implementer-agent return blob (post-mortem §6 — machine-checkable, never free-text).
const AGENT_RETURN_SCHEMA = {
  type: 'object',
  required: ['bead_id', 'status', 'pr_number', 'branch', 'base_sha', 'touched_surfaces', 'regen_ran', 'regen_check_clean', 'covered_by_tagged'],
  properties: {
    bead_id: { type: 'string' },
    status: { type: 'string', enum: ['pr-open', 'blocked', 'no-op'] },
    pr_number: { type: 'integer', description: '0 when status is blocked or no-op' },
    branch: { type: 'string' },
    base_sha: { type: 'string', description: 'origin/main SHA branched from — load-bearing for sequential ordering (next link branches off the merge this produced)' },
    touched_surfaces: { type: 'array', items: { type: 'string' }, description: 'e.g. skills, cli, codex, registry, context-map' },
    regen_ran: { type: 'boolean', description: 'did the agent run scripts/regen-all.sh (required if it touched any SKILL.md)' },
    regen_check_clean: { type: 'boolean', description: 'did scripts/regen-all.sh --check pass with no drift after commit' },
    test_fns: { type: 'array', items: { type: 'string' }, description: 'the L2 first-failing test fn names cited in the PR body' },
    covered_by_tagged: { type: 'boolean', description: '@covered-by tags written back to the bead body after green' },
    security_suppressions: { type: 'array', items: { type: 'string' }, description: 'e.g. G401, G505, nosemgrep if SHA-1/MD5 git-object code was touched' },
    blocked_reason: { type: ['string', 'null'] },
    noop_evidence: { type: ['string', 'null'], description: 'required when status=no-op: verified non-reproduction or allowlisted-exception citation' },
  },
}

// RECONCILE return: the authoritative per-PR ledger line (post-mortem §6 state
// machine). `confirmed_merged` is the ONLY source of truth for bead-close — never
// a bd batch query, never a parsed log line.
const RECONCILE_SCHEMA = {
  type: 'object',
  required: ['bead_id', 'pr_number', 'terminal_state', 'confirmed_merged', 'bead_closed'],
  properties: {
    bead_id: { type: 'string' },
    pr_number: { type: 'integer' },
    transitions: { type: 'array', items: { type: 'string' }, description: 'ordered state-machine transitions taken: polling, flake-rerun, fix-forward, merge, confirm-merged, bead-closed' },
    flake_rerun_used: { type: 'boolean', description: 'true if the one tar-cache exit-2 rerun was spent' },
    fix_forward_applied: { type: ['string', 'null'], description: 'what was fixed-forward (e.g. "regen-all registry drift") or null' },
    soft_failed_checks: { type: 'array', items: { type: 'string' }, description: 'checks treated as non-blocking, e.g. claude-review (usage-limit)' },
    terminal_state: { type: 'string', enum: ['merged', 'blocked', 'abandoned'] },
    confirmed_merged: { type: 'boolean', description: 'gh pr view --json state -q .state === MERGED' },
    bead_closed: { type: 'boolean' },
    note: { type: 'string' },
  },
}

const EPILOGUE_SCHEMA = {
  type: 'object',
  required: ['epicClosed', 'childrenAllMerged', 'worktreesPruned', 'pushed'],
  properties: {
    epicClosed: { type: 'boolean' },
    childrenAllMerged: { type: 'boolean', description: 'every child PR independently re-confirmed MERGED via gh pr view --json state' },
    unmergedChildren: { type: 'array', items: { type: 'string' }, description: 'child bead/PR ids that blocked the epic close' },
    worktreesPruned: { type: 'integer', description: 'count of merged-PR worktrees reaped (git worktree remove --force + prune)' },
    pushed: { type: 'boolean', description: 'git -C _beads push + git push succeeded' },
    learnings: { type: 'array', items: { type: 'string' }, description: 'br comments add lines written for recurring reconciliation taxes (≥2× → propose folding into the implementer prompt)' },
  },
}

// ---------------------------------------------------------------------------
// IMPLEMENTER-AGENT PROMPT (sections A-I, post-mortem §6).
// Built per-bead so the sequential base_sha can be threaded in.
// ---------------------------------------------------------------------------
const implementerPrompt = (beadId, baseSha) => {
  const baseClause = baseSha
    ? `branch off the freshly-MERGED SHA ${baseSha} (this is a sequential link; do NOT branch off a pre-fan snapshot of origin/main)`
    : `branch off origin/main`
  return `${DOCTRINE}

You are a worktree-isolated implementer. Implement EXACTLY ONE bead and open a PR. DO NOT MERGE — the orchestrator merges.

(A) ROLE+ISOLATION — You own an isolated worktree for bead ${beadId} ONLY. ${baseClause}. NEVER edit the shared checkout; never touch another bead's files.

(B) CLAIM — \`BEADS_DIR=$PWD/_beads br update ${beadId} --claim\` (skip if already in_progress and owned by this run).

(C) SCOPE — Read the bead's acceptance: a .feature file (canonical when present) or an embedded \`## Scenarios\` block. The bead's \`## Scenarios\` (or bare-GWT) acceptance is admission-guaranteed present (Prelude gate); read it as the contract. If the bead is a verified non-reproduction or an allowlisted-exception, return status="no-op" with noop_evidence and DO NOT open a PR.

(D) TDD-FIRST — Write the L2 failing test BEFORE the implementation; confirm it fails for the right reason (missing behavior, not syntax). Then make the smallest change to flip it green; refactor under green. Cite the test fn name(s) in the PR body and in test_fns. AFTER the L2 test passes red→green: close the coverage temporal gap — add \`@covered-by:<test-path>[::<TestName>]\` on its own line directly above EACH scenario in the bead body, then write the tagged body back with \`BEADS_DIR=$PWD/_beads br update ${beadId} --body\` so the bead later passes /validate full coverage. Set covered_by_tagged=true only after the write-back succeeds.

(E) DERIVED-SURFACE CHECKLIST (load-bearing) — If you touched ANY skills/**/SKILL.md you changed derived surfaces. Run \`scripts/regen-all.sh\`, then \`scripts/regen-all.sh --check\`, and commit ALL regenerated files. The MOST-MISSED surface is registry.json via scripts/generate-registry.sh — SKILL metadata/SKU edits mutate it even when no \`ao\` subcommand changed and it trips contracts-sync. If the codex regen sweeps twin manifest entries you did NOT touch, scope it: pass \`--skills foo,bar\` (or REGEN_SKILLS=foo,bar) so the diff stays scope-clean. Set regen_ran/regen_check_clean honestly.

(F) GATES — \`cd cli && go build ./... && go vet ./... && go test ./...\`; run the relevant bats for any scripts/ you touched. Follow .claude/rules/go.md and .claude/rules/python.md.

(G) SECURITY-LINT PRE-EMPT — For SHA-1/MD5 in git-object code: annotate with \`// #nosec G401 G505\` (standalone gosec IGNORES \`//nolint:gosec\`) AND a BARE \`// nosemgrep\` (a qualified \`nosemgrep: <rule-id>\` does NOT suppress) on BOTH the import line and the call site. Canonical example: cli/internal/drrebuild/drrebuild.go. Record the suppressions you used in security_suppressions.

(H) PR — Open with the required trailers: \`Closes-scenario: ${beadId}#<slug>\`, \`Bounded-context: BC<N>-<name>\`, and \`Evidence: <single changed-file path that appears in THIS run's CI logs>\`. DO NOT merge. DO NOT use --admin.

(I) RETURN — Emit the structured return blob (the AGENT_RETURN_SCHEMA). Record base_sha = the exact SHA you branched from (load-bearing for sequential ordering). If blocked, status="blocked" with blocked_reason and pr_number=0.`
}

// ---------------------------------------------------------------------------
// RECONCILE PROMPT — the orchestrator-side per-PR state machine (post-mortem §6).
// Promotes the throwaway /tmp `recon-one` into the workflow's own logic.
// State machine: polling → flake-rerun → fix-forward → merge → confirm-merged → bead-closed
// ---------------------------------------------------------------------------
const reconcilePrompt = (ret) => `${DOCTRINE}

You are the RECONCILER for ONE pull request. Drive it through the state machine to confirmed-merged-then-bead-closed, handling the recurring reconciliation tax mechanically. Log every transition for idempotent resume.

PR context (from the implementer's structured return):
${JSON.stringify(ret, null, 2)}

GUARD — if regen_check_clean is NOT true, do NOT merge: fix-forward FIRST (step 4) then re-poll.

STATE MACHINE:
1. POLLING — \`gh pr checks ${ret.pr_number}\` until every check is terminal (not pending/queued). Do not act on a non-terminal run.
2. PARTITION — split fails into substantive vs soft. SOFT (non-blocking): \`claude-review\` failing ONLY on a Claude usage-limit message (the LIMIT_RE soft-fail, #635). A generic "rate limit" is NOT soft. Record soft-failed checks.
3. KNOWN-FLAKE RERUN — \`correctness (ubuntu-latest)\` tar-cache-restore exit 2 is a known flake. \`gh run rerun\` it EXACTLY ONCE (budget=${FLAKE_RERUN_BUDGET}), re-poll, then believe the result. One retry only, so it can't mask a real break.
4. FIX-FORWARD — derived-surface drift (registry.json / cli-command-surface / context-map / codex via contracts-sync) is fixed-forward IN THE WORKTREE: \`scripts/regen-all.sh\` (scope with --skills if codex sweeps twins), commit, push; re-poll. NEVER bounce back to the agent; NEVER revert green work. A pre-existing-looking red (contracts-sync/security-canary) is reproduced on a CLEAN origin/main worktree FIRST — if it reproduces there it is a latent-trunk break: file a NEW bead + fix it separately, do not block this feature PR.
5. MERGE — only when all substantive checks are green: \`gh pr merge ${ret.pr_number} --squash --admin\`.
6. CONFIRM-MERGED — \`gh pr view ${ret.pr_number} --json state -q .state\` MUST equal "MERGED". This ledger line is AUTHORITATIVE — never trust a batch tracker query.
7. BEAD-CLOSED — ONLY after confirmed MERGED: \`BEADS_DIR=$PWD/_beads br close ${ret.bead_id} --reason "merged in #${ret.pr_number}"\`.

Return the reconcile ledger blob. terminal_state="blocked" if a substantive red is real and unresolved; "abandoned" only if the PR was intentionally closed without merge.`

// ===========================================================================
// PHASE 1 — PRELUDE
// git reset --hard origin/main (NEVER pull --rebase — silent no-op on diverged
// trees, post-mortem failure #6). Validate beads + classify mode.
// ===========================================================================
phase('Prelude')
if (beadIds.length === 0) {
  log('PRELUDE: no beads passed. Pass args = {beads:[...ids], mode}. Aborting.')
  return { stoppedAt: 'prelude', reason: 'no beads', beads: [] }
}
const prelude = await agent(
  `${DOCTRINE}

PRELUDE for a ship-beads run over: ${beadIds.join(', ')} (requested mode: ${requestedMode}${epicId ? `, parent epic: ${epicId}` : ''}).

1. STALE-CHECKOUT GUARD — on the host orchestration checkout run \`git fetch origin && git reset --hard origin/main\`. \`git pull --rebase\` is BANNED in this workflow (it silently no-ops on a diverged local main and corrupts surveys).
2. VALIDATE BEADS — for EACH bead run \`BEADS_DIR=$PWD/_beads br show <id>\` (do NOT rely on a single batch \`--json\` query for control flow). Confirm each exists and is either unclaimed or already owned by this run. Flag any that are missing, closed, or claimed by someone else.
2.5. ADMISSION GATE — for EACH validated bead run \`BEADS_DIR=$PWD/_beads br show <id> | bash scripts/check-bead-scenario-coverage.sh --admission -\`. Exit 0 → ADMISSIBLE (add to admissibleBeads). Exit 1 → anomaly {beadId, issue: "inadmissible: no runnable acceptance"} — the bead does NOT enter admissibleBeads. Exit 2 → anomaly {beadId, issue: "tracker failure"} — an infra/misuse failure; the bead is NOT classified inadmissible, and it does NOT enter admissibleBeads.
3. Report the validated bead list, the admissible bead list, and any anomalies. Do NOT start implementation.`,
  { label: 'prelude:validate', phase: 'Prelude', schema: {
    type: 'object',
    required: ['resetClean', 'validBeads', 'admissibleBeads', 'anomalies'],
    properties: {
      resetClean: { type: 'boolean', description: 'git reset --hard origin/main succeeded' },
      validBeads: { type: 'array', items: { type: 'string' } },
      admissibleBeads: { type: 'array', items: { type: 'string' }, description: 'validated beads that ALSO passed the --admission acceptance gate (exit 0)' },
      anomalies: { type: 'array', items: { type: 'object', required: ['beadId', 'issue'], properties: { beadId: { type: 'string' }, issue: { type: 'string' } } } },
    },
  } }
)
log(`Prelude: ${prelude.validBeads.length}/${beadIds.length} beads valid${prelude.anomalies.length ? ` · ${prelude.anomalies.length} anomalies` : ''}`)
// FAIL-CLOSED admission filter (ag-iruq3.3): an empty admissible set STOPS the
// run. Never fall back to "admit everything" — the old `validBeads || beadIds`
// fallback was fail-open and would ship beads with no runnable acceptance.
if (!prelude.admissibleBeads || prelude.admissibleBeads.length === 0) {
  log('PRELUDE: zero admissible beads — refusing to ship (fail-closed).')
  return { stoppedAt: 'prelude', reason: 'no admissible beads', anomalies: prelude.anomalies }
}
const workBeads = prelude.admissibleBeads

// ===========================================================================
// PHASE 2 — PLAN-GRAPH
// Build the bead DAG. The surface-collision detector gates mode: parallel is
// legal ONLY for provably-disjoint surfaces; any shared write/derived/skill/
// fixture surface demotes a wave to a sequential sub-chain.
// ===========================================================================
phase('Plan-Graph')
const planGraph = await agent(
  `${DOCTRINE}

PLAN-GRAPH for beads: ${workBeads.join(', ')}. Requested mode: ${requestedMode}.

For each bead, read its metadata + scenario/.feature files and estimate its likely-touched surfaces (write scopes, derived surfaces, the specific SKILL paths, cli/internal packages, shared fixtures). Build the dependency DAG (dependsOn edges).

ORDERING RULE (post-mortem §6, load-bearing): mode=parallel is legal ONLY when surfaces are PROVABLY disjoint. The surface-collision detector — if two beads in a candidate wave BOTH touch \`cli-command-surface\` / a shared derived surface (registry.json / context-map / codex hashes) / the SAME skills/**/SKILL.md / shared fixtures — DEMOTE that wave to a sequential sub-chain (set parallelizable=false, mode=sequential, demoted=true, demotionReason=<the shared surface>). Coupled chains (schema→write-model→export→gate→tests) MUST run sequential. Parallel waves are reserved for disjoint surfaces and capped at ${PARALLEL_CAP}.

Emit the wave plan: ordered waves; each multi-bead wave must pass ALL disjointCheck rows and have beadIds.length<=${PARALLEL_CAP}; everything else goes into single-bead sequential waves in dependency order.`,
  { label: 'plan:graph', phase: 'Plan-Graph', schema: PLAN_GRAPH_SCHEMA }
)
const effectiveMode = planGraph.mode
log(`Plan-Graph: ${planGraph.waves.length} wave(s), effective mode=${effectiveMode}${planGraph.demoted ? ` (DEMOTED parallel→sequential: ${planGraph.demotionReason})` : ''}`)

// ===========================================================================
// PHASE 3+4 — FAN/SHIP + RECONCILE, interleaved per wave.
// Parallel wave: spawn the implementer fan-out (barrier on all returns), then
// reconcile each returned PR. Sequential wave (single bead): spawn one, reconcile
// to confirmed-merge, and thread the resulting merge SHA into the NEXT link's base.
// Branching the next link off the freshly-MERGED SHA (not a pre-fan snapshot) is
// what prevents fixture collisions + false "export missing" surveys (§6).
// ===========================================================================
const reconcileLedger = []
let lastMergedSha = null // threaded base for the next sequential link

// One implementer agent. parallel() maps a thrown thunk to null, so a blocked-
// before-return slice must surface as a structured blocked blob, never be dropped.
const blockedReturn = (beadId, err) => ({
  bead_id: beadId, status: 'blocked', pr_number: 0, branch: '', base_sha: '',
  touched_surfaces: [], regen_ran: false, regen_check_clean: false, test_fns: [], covered_by_tagged: false,
  security_suppressions: [], blocked_reason: `agent errored before returning: ${String((err && err.message) || err)}`, noop_evidence: null,
})
const runImplementer = (beadId, baseSha) => () =>
  agent(implementerPrompt(beadId, baseSha), { label: `ship:${beadId}`, phase: 'Fan-Ship', schema: AGENT_RETURN_SCHEMA })
    .then((r) => (r && r.bead_id ? r : { ...blockedReturn(beadId, 'null/invalid return'), bead_id: beadId }))
    .catch((err) => blockedReturn(beadId, err))

// Reconcile one returned PR. no-op closes the bead with no PR (must carry
// noop_evidence); blocked returns are recorded without a merge attempt.
const reconcileReturn = async (ret) => {
  if (ret.status === 'no-op') {
    if (!ret.noop_evidence) {
      log(`Reconcile ${ret.bead_id}: status=no-op but NO noop_evidence — refusing to close, recording blocked.`)
      return { bead_id: ret.bead_id, pr_number: 0, terminal_state: 'blocked', confirmed_merged: false, bead_closed: false, note: 'no-op without evidence', transitions: [], flake_rerun_used: false, fix_forward_applied: null, soft_failed_checks: [] }
    }
    const closed = await agent(
      `${DOCTRINE}\n\nBead ${ret.bead_id} is a verified no-op (evidence: ${ret.noop_evidence}). Re-confirm the evidence holds, then \`BEADS_DIR=$PWD/_beads br close ${ret.bead_id} --reason "no-op: ${ret.noop_evidence}"\`. No PR is involved. Return the reconcile ledger blob with terminal_state="abandoned" (closed without merge), confirmed_merged=false, bead_closed per the result.`,
      { label: `reconcile:${ret.bead_id}`, phase: 'Reconcile', schema: RECONCILE_SCHEMA })
    return closed || { bead_id: ret.bead_id, pr_number: 0, terminal_state: 'abandoned', confirmed_merged: false, bead_closed: false, note: 'no-op close returned null', transitions: ['bead-closed'], flake_rerun_used: false, fix_forward_applied: null, soft_failed_checks: [] }
  }
  if (ret.status === 'blocked' || !ret.pr_number) {
    log(`Reconcile ${ret.bead_id}: BLOCKED (${ret.blocked_reason || 'no PR opened'}) — no merge attempt.`)
    return { bead_id: ret.bead_id, pr_number: ret.pr_number || 0, terminal_state: 'blocked', confirmed_merged: false, bead_closed: false, note: ret.blocked_reason || 'no PR', transitions: [], flake_rerun_used: false, fix_forward_applied: null, soft_failed_checks: [] }
  }
  phase('Reconcile')
  const led = await agent(reconcilePrompt(ret), { label: `reconcile:${ret.bead_id}#${ret.pr_number}`, phase: 'Reconcile', schema: RECONCILE_SCHEMA })
  return led || { bead_id: ret.bead_id, pr_number: ret.pr_number, terminal_state: 'blocked', confirmed_merged: false, bead_closed: false, note: 'reconcile returned null', transitions: ['polling'], flake_rerun_used: false, fix_forward_applied: null, soft_failed_checks: [] }
}

phase('Fan-Ship')
for (const wave of planGraph.waves) {
  const ids = wave.beadIds.filter(Boolean)
  if (wave.parallelizable && ids.length > 1) {
    // PARALLEL fan-out — disjoint surfaces. Each branches off origin/main; barrier
    // on all returns, then reconcile each (merge order within a disjoint wave is
    // free, so reconcile sequentially to keep the merge SHA / CI floor predictable).
    log(`Fan-Ship wave [${ids.join(',')}]: PARALLEL fan-out (cap ${PARALLEL_CAP}).`)
    const returns = (await parallel(ids.slice(0, PARALLEL_CAP).map((id) => runImplementer(id, null)))).filter(Boolean)
    for (const ret of returns) {
      const led = await reconcileReturn(ret)
      reconcileLedger.push(led)
      if (led.confirmed_merged) lastMergedSha = null // disjoint: next wave re-baselines on origin/main
    }
  } else {
    // SEQUENTIAL link(s) — coupled surfaces. Each link branches off the SHA the
    // PRIOR link's merge produced (lastMergedSha), never a pre-fan snapshot.
    for (const id of ids) {
      log(`Fan-Ship link ${id}: SEQUENTIAL${lastMergedSha ? ` (base ${lastMergedSha})` : ' (base origin/main)'}.`)
      const ret = await runImplementer(id, lastMergedSha)()
      const led = await reconcileReturn(ret)
      reconcileLedger.push(led)
      // Thread the freshly-merged SHA forward ONLY on confirmed merge; a blocked
      // link does not advance the base (the next coupled link would collide).
      if (led.confirmed_merged) {
        const tip = await agent(
          `${DOCTRINE}\n\nPR #${led.pr_number} for bead ${id} is confirmed MERGED. Read the new origin/main tip after a \`git fetch origin\`: \`git rev-parse origin/main\`. Return just that SHA so the next sequential link branches off it.`,
          { label: `base-sha:${id}`, phase: 'Fan-Ship', schema: { type: 'object', required: ['sha'], properties: { sha: { type: 'string' } } } })
        lastMergedSha = (tip && tip.sha) || lastMergedSha
      } else {
        log(`Sequential link ${id} did not confirm-merge (${led.terminal_state}) — base NOT advanced; downstream coupled links may be blocked.`)
      }
    }
  }
}
const merged = reconcileLedger.filter((l) => l.confirmed_merged).length
const blocked = reconcileLedger.filter((l) => l.terminal_state === 'blocked').length
log(`Fan-Ship+Reconcile done · ${merged}/${reconcileLedger.length} confirmed-merged${blocked ? ` · ${blocked} blocked` : ''}`)

// ===========================================================================
// PHASE 5 — EPILOGUE
// Close the parent epic ONLY when every child is confirmed-merged (re-query
// gh pr state per child — the §6 epic-close guard; failure #3). Prune merged
// worktrees, push, and write br comment learnings for recurring taxes.
// ===========================================================================
phase('Epilogue')
const epilogue = await agent(
  `${DOCTRINE}

EPILOGUE for the ship-beads run.${epicId ? ` Parent epic: ${epicId}.` : ' No parent epic to close.'}

Reconcile ledger (authoritative — this carries the workflow's own structured state, NOT a batch tracker query):
${JSON.stringify(reconcileLedger, null, 2)}

1. EPIC-CLOSE GUARD (only if a parent epic was given) — list the epic's children. For EACH child, re-query \`gh pr view <N> --json state -q .state\` independently (do NOT trust a batch \`--json\` tracker query). Close the epic ONLY if EVERY child is confirmed MERGED; one non-merged child ABORTS the close (record it in unmergedChildren). Never close a parent before every child PR is confirmed merged.
2. WORKTREE REAPING — for every CONFIRMED-MERGED PR's worktree: \`git worktree remove <path> --force\`; then \`git worktree prune\`. Leave unmerged-PR worktrees intact. Target: zero orphaned worktrees from this run.
3. PUSH — \`git -C _beads push\` (the private br ledger repo) then \`git push\`; confirm \`git status\` shows up-to-date with origin.
4. LEARNING CAPTURE — write \`BEADS_DIR=$PWD/_beads br comments add <id> "<lesson>"\` on the relevant bead (or the epic) for each reconciliation lesson; if a tax recurred >=2x in the ledger (registry drift, codex sweep, tar-cache flake, security suppression), propose folding it into the implementer prompt template (sections E/G) — the same shaping that reduced the tax in the source session.

Return the epilogue blob.`,
  { label: 'epilogue', phase: 'Epilogue', schema: EPILOGUE_SCHEMA }
)
log(`Epilogue: epic ${epilogue.epicClosed ? 'CLOSED' : (epicId ? `held open (${(epilogue.unmergedChildren || []).length} unmerged children)` : 'n/a')} · ${epilogue.worktreesPruned} worktree(s) pruned · pushed=${epilogue.pushed}`)

return {
  stoppedAt: null,
  requestedMode,
  effectiveMode,
  beads: workBeads,
  planGraph,
  reconcile: reconcileLedger,
  merged,
  blocked,
  epilogue,
}
