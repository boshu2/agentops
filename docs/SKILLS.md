# Skills Reference

Narrative reference for checked-in AgentOps skills. The current inventory is
generated from `skills/**/SKILL.md` into `registry.json` and the generated
domain maps; do not hard-code skill counts here.

**Skills are the product front door.** AgentOps is the operating loop from
intent → Gherkin → ATDD → implement → membrane → ratchet. Each skill is one
move (or a wrapper) in that loop. The membrane (move 6) validates against the
slice's acceptance behavior — without Gherkin/ATDD it has nothing honest to
accept.

| Map | Purpose |
|-----|---------|
| [Intent → Validated Code](architecture/intent-to-validated-code.md) | Full flow, artifacts, done signals |
| [Skills Matrix](skills-matrix.md) | Every skill placed on moves 1–7 |
| [Operating Loop](architecture/operating-loop.md) | Discipline (waves, windshield, ratchet) |
| [First-value path](first-value-path.md) | First session via `/plan` → `/implement` → `/validate` → `/learn` |

**Behavioral Contracts:** Most skills include `scripts/validate.sh` behavioral checks to verify key features remain documented. Run `skills/<name>/scripts/validate.sh` when present, or the GOALS.yaml `behavioral-skill-contracts` goal to validate the full covered set.

## First-use summary

The canonical selection tree is the [Skill Router](SKILL-ROUTER.md). Prefer the
[Skills Matrix](skills-matrix.md) when you need the full catalog on the loop.
The compact summary below is illustrative, not a second routing authority. Load
only contracts triggered by the task; `ao session bootstrap` and `ao lookup`
belong to optional archive profiles, not the default runtime.
To search skills by intent, use `ms search "<task>"`
(or `mcp__ms__search`) — ([`skills/ms/SKILL.md`](../skills/ms/SKILL.md)).

```text
What are you trying to do?
│
├─ "Run the full loop / first time"
│   ├─ See the whole product ─────► docs: Intent → Validated Code + Skills Matrix
│   ├─ One behavior end-to-end ───► /plan → /implement → /validate → /learn
│   ├─ One tick wrapped ──────────► /rpi "goal"
│   └─ Repo setup ────────────────► /bootstrap · ao quick-start · ao doctor
│
├─ "Prove it's done / validate" (the Membrane — needs acceptance behavior)
│   ├─ Code ready to ship? ───────► /validate
│   ├─ Deeper code audit? ────────► /validate --mode=post-impl
│   ├─ Plan ready to build? ──────► /premortem
│   ├─ Independent judges ────────► /council validate recent
│   ├─ Adversarially probe it ────► /validate --debate
│   ├─ Need fresh pawl evidence? ──► /pawl-review → ao pawl
│   ├─ Drive fixes to agreement ──► /converge
│   ├─ Mid-epic drift check ──────► /reality-check
│   ├─ Security + release gate ───► /security
│   └─ Work ready to close? ──────► /validate, then /learn
│
├─ "Track it / bookkeep it" (the Bookkeeper)
│   ├─ Break it into issues ──────► /plan
│   ├─ Manage/close issues ───────► /beads-br
│   ├─ Turn a goal into a loop-ready packet ─► /goal-design
│   ├─ Shape a fuzzy idea ────────► /discovery --ideate
│   ├─ Build a single issue ──────► /implement
│   ├─ Where was I? ──────────────► /status
│   └─ Save for next session ─────► /handoff
│
├─ "Build a feature"
│   ├─ Small (1-2 files) ─────────► /implement
│   ├─ Medium (3-6 issues) ───────► /plan → /crank
│   └─ Large (7+ issues) ─────────► /rpi (full pipeline)
│
├─ "Now build it"
│   ├─ Small/single issue ─────────► /implement
│   ├─ Multi-issue epic ───────────► /crank <epic-id>
│   └─ Full flow in one command ───► /rpi "goal"
│
├─ "Fix a bug"
│   ├─ Already scoped? ────────────► /implement <issue-id>
│   └─ Need to investigate? ───────► /validate --mode=pr
│
├─ "Explore or research"
│   ├─ Understand this codebase ──► /research
│   ├─ Compare approaches ────────► /council research <topic>
│   └─ Generate ideas ────────────► /discovery --ideate
│
├─ "Learn from past work"
│   ├─ Turn the corpus into operator surfaces ─► /operationalize
│   ├─ What do we know about X? ──► host-native session search; optional archive profile for ao lookup/search
│   ├─ Record verdict-bound observations ► /learn
│   └─ Test a retrospective causal question ► /postmortem
│
├─ "Parallelize work"
│   ├─ Multiple independent tasks ► /swarm
│   └─ Full epic with waves ──────► /crank <epic-id>
│
├─ "City-shaped multi-quest work" (gas city — operator choice, coexists with NTM; never auto-routed)
│   ├─ Stand up / drive / admin / unstick a city ─► /using-gc
│   └─ The close door + pawl-verdict.v1 internals ► gc-membrane (reference)
│
├─ "Ship a release"
│   └─ Changelog + tag ──────────► /release <version>
│
├─ "Session management"
│   ├─ Compile mature knowledge ──► /operationalize
│   ├─ Where was I? ──────────────► /status
│   ├─ Save for next session ─────► /handoff
│   └─ Recover after compaction ──► /status --recover
│
└─ "First time here" ────────────► /plan → /implement → /validate → /learn
                                   (maps: Intent → Validated Code · Skills Matrix)
```

<!-- BEGIN:spine -->
## The Membrane — validation spine (no verdict = not done)

Move 6 of the operating loop: prove acceptance against the slice's behavior
contract (Gherkin → ATDD). Every change reaches *done* only with an independent
verdict — fresh-context, cross-family, or deterministic check on the completion
claim. **No verdict = not done.** Without scenarios/acceptance tests, HOLD —
the membrane is not a vibe review.

Reach here **after** `/plan` (and usually `/implement`) has frozen the behavior
to prove. Full flow: [Intent → Validated Code](architecture/intent-to-validated-code.md).

### /validate

Final acceptance verdict. Pass that immutable verdict to `/learn` before the
orchestrator chooses the next transition.

```bash
/validate
/validate ag-1234
```

**Use when:** The work is ready for an independent acceptance verdict.

**Absorbed (retired 2026-07-07, folded here):** `/review` → `/validate --mode=pr` (diff/PR review); `/red-team` → `/validate --debate` (adversarial); `/eval-outcomes` → `/validate --mode=pre-impl --target=scenario` (CLI: `ao eval scenario`).

### /validate --mode=post-impl (absorbs /vibe)

Comprehensive code validation across 8 aspects with finding classification (CRITICAL vs INFORMATIONAL), suppression framework for known false positives, and domain-specific checklists (SQL safety, LLM trust boundary, race conditions) auto-loaded from `/standards`. Correlates findings against pre-mortem predictions.

```bash
/validate --mode=post-impl services/auth/
```

**Checks:** Security, Quality, Architecture, Complexity, Testing, Accessibility, Performance, Documentation

### /council

Multi-model validation — the core primitive used by validate, pre-mortem, and post-mortem. Auto-extracts significant findings from WARN/FAIL verdicts into the knowledge flywheel.

```bash
/council validate recent
/council --deep recent
```

### /premortem

Simulate failures before implementing. Includes error/rescue mapping (tabular risk/mitigation), scope mode selection (Expand/Hold/Reduce with auto-detection), temporal interrogation (hour 1/2/4/6+ timeline), and prediction tracking with unique IDs (`pm-YYYYMMDD-NNN`) correlated through validate and post-mortem.

```bash
/premortem "add caching layer"
```

**Output:** Failure modes, error/rescue maps, predictions with IDs, mitigation strategies, spec improvements

### /converge

Drive a fix → re-run-judge-panel loop to terminal agreement or a 3-consecutive-fail BLOCK via the Go `ao converge` command. Thin memo over the CLI — the loop and gates live in Go.

```bash
ao converge --scope head
```

### /reality-check

Mid-epic drift audit: code is ground truth; README/PRODUCT/plan are the measuring stick. Use when a wave boundary lands and bead counts look healthy but value feels absent.

```bash
/reality-check
```

### /security

Run repository security scans for vulnerabilities, dependency risk, secrets, and release gates — plus the composable binary/prompt-surface suite (offline redteam, policy gating).

```bash
/security audit
```

### /pawl-review

Run one immutable, fresh, read-only reviewer lane and hand its contained evidence
to `ao pawl`. The skill does not decide or write the panel verdict.

```bash
/pawl-review
```

---

## The Bookkeeper — tracking + session spine

Where work lives between your head and *done*: the linked-intent packet, its
issues and dependency waves, and the session continuity that keeps the next
turn from starting from scratch.

### /beads-br

Git-native issue tracking operations.

```bash
BEADS_DIR="$(ao beads dir)" br ready      # Unblocked issues
BEADS_DIR="$(ao beads dir)" br show <id>  # Issue details
BEADS_DIR="$(ao beads dir)" br close <id> # Close issue
```

### /status

Single-screen dashboard of project state.

```bash
/status
```

**Absorbed (retired 2026-07-07, folded here):** `/recover` → `/status --recover` (post-compaction recovery; deep playbook at `skills/status/references/recovery-playbook.md`).

### /handoff

Session handoff — preserve context for continuation.

```bash
/handoff
```

### /discovery --ideate

Structured idea exploration. Four phases: assess clarity, understand idea, explore approaches, capture design.

```bash
/discovery --ideate "add user authentication"
```

**Output:** `.agents/discovery/YYYY-MM-DD-<slug>.md`

### /goal-design

Create a schema-backed `.agents/goal-design/<slug>/` packet with checked
`intent.md` and `driver.md` artifacts before discovery or planning.

```bash
/goal-design "harden packet identity checks"
```

**Output:** `.agents/goal-design/<slug>/intent.md` and `driver.md`

### /plan

Decompose goals into trackable beads issues with dependencies.

```bash
/plan "Add user authentication with OAuth2"
```

**Output:** Beads issues with parent/child relationships

### /implement

Execute a single beads issue with full lifecycle.

```bash
/implement ap-1234
```

**Phases:** Context → Tests → Code → Validation → Commit
<!-- END:spine -->

---

## Flow Skills

The research → build → ship path. These move work through the system; the
Membrane proves each step and the Bookkeeper tracks it.

### /research

Deep codebase exploration using Explore agents.

```bash
/research authentication flows in services/auth
```

**Output:** `.agents/research/<topic>.md`

### /rpi

Full RPI lifecycle orchestrator. Discovery → Crank → Validate → Learn in one command.

```bash
/rpi "Add user authentication"
/rpi --fast-path "fix typo in README"
/rpi --from=implementation ag-1234
```

**Phases:** Discovery (`/discovery`) → Crank (`/crank`) → Validate (`/validate`) → Learn (`/learn`)

### /crank

Executes one bounded, evidence-producing wave. Crank does not close the epic,
mutate caller-owned tracker state, or deliver Git refs; the caller consumes the
wave receipt and chooses the next transition.

```bash
/crank <epic-id>
```

**Execution model:** Wave-based orchestration via `/swarm` with runtime-native workers.

### /swarm

Parallel agent spawning for concurrent task execution.

```bash
/swarm <epic-id>
```

### Runtime-native multi-agent lanes

Spawn parallel execution agents through the current runtime/substrate. Use
`/swarm` for the skill-level entry point; use Codex subagents or NTM when
the active runtime owns that transport.

```bash
/swarm <epic-id>
```

### /learn

Consume an immutable Validate verdict, copy only structured observations, and
emit the fourth RPI receipt plus `plan_impact`. Learn never changes proof, the
plan, delivery state, or constraint activation.

**Output:** `learn-receipt.json` and `.agents/rpi/phase-4-summary.md`.

### /postmortem

Optional retrospective causal analysis after Validate and Learn. Pin an explicit
causal question, test competing explanations and counterfactuals, preserve
unknowns, and return evidence to the caller. It is not a completion gate,
general learning umbrella, plan owner, or constraint activator.

```bash
/postmortem "why did the rollout exceed the error budget?"
```

**Output:** `.agents/council/YYYY-MM-DD-postmortem-<topic>.md`.

---

## Utility Skills

### Knowledge queries (no slash command)

There is no default-profile knowledge-query command. Use host-native search over
the workspace-local `.agents/` tree, then `/operationalize`
to promote evidence into a future-consumed artifact. The optional archive build
retains historical lookup commands for operators who deliberately select it:

```bash
# Archive build only; absent from the default CLI.
ao lookup --query "patterns for rate limiting"
ao search --all "patterns for rate limiting"
```

**Searches:** `.agents/learnings/`, `.agents/patterns/`, `.agents/research/`, `.agents/compiled/`

### Skill search (ms — no slash command)

Find a skill by intent across both corpora before hand-grepping the catalog.
Consume via MCP (`mcp__ms__search` / `mcp__ms__load full:true`); writes and admin
go through the `ms` CLI. See [`skills/ms/SKILL.md`](../skills/ms/SKILL.md).

```bash
ms search "switch accounts on a rate limit" -O json
ms load account-rotation --full -O json | jq -r '.data.content'
```

### /refactor (absorbs /complexity)

Code complexity analysis using radon (Python) or gocyclo (Go).

```bash
/refactor services/
```

**Threshold:** CC > 10 triggers refactoring issue

### /doc

Generate and validate repo documentation. `--mode` selects the artifact family:
default (code/API docs, code-maps), `--mode=readme` (gold-standard README via
interview + council validation), or `--mode=oss` (open-source doc pack:
CONTRIBUTING, CHANGELOG, AGENTS).

```bash
/doc services/auth/          # code/API docs (default)
/doc --mode=readme           # gold-standard README
/doc --mode=oss              # scaffold/audit OSS doc pack
```

### /release

Pre-flight checks, changelog generation, version bumps, and tagging.

```bash
/release
```

### /status (absorbs /quickstart)

Interactive onboarding — mini RPI cycle for new users.

```bash
/status
```

### Out-of-session compounding

Retirement pointer. The in-tree out-of-session compounding engine was removed
(soc-2rtm0); scheduled, between-session knowledge compounding now runs via an
adopted substrate, and AgentOps ships no out-of-session runner of its own.
In-session knowledge work stays skill-driven through `/learn`,
`/pattern-mining`, and `/operationalize`. Archive-only compile and lookup
commands are not dependencies of the default loop.

**Output:** none — this skill no longer drives an in-repo command.

### Knowledge operationalization

Operationalize a mature `.agents` corpus into reusable belief, playbook, briefing, and gap surfaces.

```bash
ao knowledge activate --goal "productize knowledge activation"
ao knowledge gaps
```

### /evolve

Autonomous fitness-scored improvement loop. Measures GOALS.yaml, fixes the worst gap, compounds via knowledge flywheel.

```bash
/evolve                      # Run until stopped or the full producer ladder is exhausted
/evolve --max-cycles=5       # Cap at 5 cycles
/evolve --dry-run            # Measure only, don't execute
```

### /product

Interactive PRODUCT.md generation. Interviews about mission, personas, value props, and competitive landscape.

```bash
/product
```

**Output:** `PRODUCT.md` in repo root

### /heal-skill

Detect and auto-fix skill hygiene issues (missing frontmatter, unlinked references, dead references).

```bash
/heal-skill --check                     # Report issues
/heal-skill --fix                       # Auto-fix what's safe
/heal-skill --check skills/council      # Check specific skill
```

**Checks:** MISSING_NAME, MISSING_DESC, NAME_MISMATCH, UNLINKED_REF, EMPTY_DIR, DEAD_REF

### /converter

Convert skills to other platforms (Codex, Cursor).

```bash
/converter skills/council codex          # Single skill to Codex format
/converter --all cursor                  # All skills to Cursor .mdc format
```

**Targets:** codex (SKILL.md + prompt.md), cursor (.mdc + optional mcp.json), test (raw bundle)

### /pr-prep

Prepare structured PR bodies with validation evidence. Includes commit split advisor (Phase 4.5) suggesting bisectable commit ordering.

```bash
/pr-prep
```

---

## Additional Skills

Single-purpose skills not listed above. See each skill's `SKILL.md` for triggers,
phases, and flags.

| Skill | Purpose |
|-------|---------|
| `/bootstrap` | Idempotent product-layer setup (`GOALS.md`, `PRODUCT.md`, `README.md`, `.agents/`); leaves runtime hooks unmanaged |
| `/security` (absorbs deps) | Dependency audit, updates, vulnerability scanning, license compliance |
| `/product` | Maintain `PRODUCT.md` so validation and planning share the same product contract |
| `/discovery` | Full discovery-phase orchestrator (ideation + search + research + plan + premortem) |
| `/goal-design` | Create checked goal-design packets before discovery or planning |
| `/goals` | Maintain `GOALS.yaml`/`GOALS.md` fitness specs; measure drift; add/prune directives |
| `/push` | Run the repository-selected delivery adapter (direct push, PR, or user-owned CI); commit only when requested |
| `/refactor` | Safe, verified refactoring with regression tests at each step |
| `/scaffold` | Project scaffolding, component generation, boilerplate |
| `/test` | Test generation, coverage analysis, TDD workflow |

---

## Internal Skills

These are loaded by other skills or lifecycle hooks; they are not primary
user-facing entry points:

| Skill | Purpose |
|-------|---------|
| `standards` | Language-specific coding standards (auto-loaded by /validate, /implement) |
| `shared` | Shared reference documents for multi-agent backends |
| `beads-br` | Issue tracking reference (local-first beads_rust tracker) |

---

## Subagents

Judgment skills may delegate independent reviews when their own contract calls
for them. The live role, evidence, and authority boundaries belong in each
`skills/<slug>/SKILL.md`; this router does not maintain a duplicate role count.

---

## Default-profile ao CLI integration

The default CLI is the verification and bookkeeping spine. Archive-profile
commands mentioned in historical skill references are not implied here.

| Skill | ao CLI Command |
|-------|----------------|
| `/plan`, `/premortem` | `ao plan-pawl` when a binding plan verdict is required |
| `/validate`, `/pawl-review` | `ao gate`, `ao verify`, `ao pawl`, `ao verdict-gate` |
| `/beads-br`, `/implement` | `ao beads`, `ao claim`, `ao ready`, `ao done` |
| `/push`, `/release` | `ao gate check --fast --scope head`, then `ao land <bead>` for bead-backed work |
| `/learn` | `ao provenance` for durable evidence; constraint candidates remain advisory pending replay and shadow evidence |
