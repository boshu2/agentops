# AOP-CLAIM-COMP-TONS-OF-SKILLS-CORPUS-COMPOUNDING — evidence (v2.39.0)

**Claim location:** `docs/comparisons/vs-tons-of-skills.md` line 54 — "Persistent corpus" section. Cross-referenced 5 times across the competitive surface.

**Claim summary:** AgentOps adds a compounding corpus on top of skill inventory rather than a larger inventory of skills. Skills end with the session; AgentOps ships a bookkeeping schema that turns each session's output into durable corpus that the next session reads.

## Repo surfaces that demonstrate it

### The corpus itself

- **`.agents/`** as a tracked vault — markdown surfaces under `learnings/`, `patterns/`, `findings/`, `discovery/`, `council/`, `post-mortems/`, `ratchet/`, `evolve/`, `crank/`, `harvest/`. Each session writes to one or more of these surfaces via skill protocols; none of them disappear at session boundary.
- **`.agents/ao/citations.jsonl`** — durable record of which corpus entries were retrieved + applied per session. Citations feed back into the decay-ranked retrieval at the next session start.
- **`.agents/evolve/cycle-history.jsonl`** — durable cycle ledger across days. Each entry is one /evolve cycle's outcome; the file is committed and grows monotonically.

### The bookkeeping schema

- **`docs/contracts/factory-claim-ledger.md`** + **`docs/contracts/factory-yield-ledger.md`** + **`docs/contracts/finding-registry.md`** + **`docs/contracts/factory-admission.md`** — the schema by which corpus content is structured for retrieval and verification. Each contract has a schema, a check script, and a ledger of admitted entries.
- **`skills/*/SKILL.md` `practices:` declarations** — 756/756 primitives declare their reusable patterns; the practice citations link skill-level reuse back to the same corpus surfaces.

### The retrieval mechanism

- **`cli/cmd/ao/inject*.go`** — `ao inject` with decay-ranked retrieval, finding scoring, learning surfacing, and quality gating. Companion: `cli/internal/context/` (versioned context assembly).
- **`cli/cmd/ao/mine.go`** + **`cli/cmd/ao/defrag.go`** + **`cli/cmd/ao/compile.go`** — the Mine → Defrag → Compile loop that promotes corpus content from raw to indexed to wiki-quality.
- **`cli/cmd/ao/forge*.go`** — extraction of learnings from transcripts back into the corpus.

### The compounding proof

- **`docs/releases/flywheel-compounding-snapshot.json`** (tracked) — durable snapshot showing `escape_velocity_compounding=true`, σρ × utility > δ × decay, recorded against a known git SHA.
- **`scripts/check-flywheel-compounding-snapshot.sh`** — CI gate that fails if the snapshot is > 14 days stale or the value flipped to non-compounding.

## Verification surfaces

| Gate | What it asserts |
|---|---|
| `scripts/check-flywheel-compounding.sh` | Live escape-velocity computation: capture × utility > natural decay. Fails when the corpus stops compounding. |
| `scripts/check-flywheel-compounding-snapshot.sh` | Tracked snapshot is < 14 days old and shows `compounding=true`. CI-blocking (G1, weight 5). |
| `scripts/check-flywheel-proof.sh` | Cross-session evidence: learnings captured in one session are applied in another (citations against learnings file paths). |
| `scripts/check-retrieval-quality-ratchet.sh` | `ao inject` retrieval can name the corpus sources behind each context window. Drift-blocking. |
| `scripts/check-competitive-freshness.sh` | Competitive docs (the surface this claim lives on) refreshed against current state, not stale. The ledger row's declared `closure_gate`. |
| `scripts/check-factory-claim-ledger.sh` | The claim's marker remains in `docs/comparisons/vs-tons-of-skills.md` and the ledger row stays in sync. |

## Why this is enough

The original ledger row records `evidence_status: "partial"` with `missing_proof: "Needs generated corpus-growth and competitive-freshness scorecards before quantitative inventory-vs-corpus claims."` The corpus-growth side now has tracked evidence (`docs/releases/flywheel-compounding-snapshot.json` with σρδ values + the live `check-flywheel-compounding.sh` lane). The competitive-freshness side has its own gate (`check-competitive-freshness.sh`). Both are CI-wired today.

The claim is structural (the corpus exists and compounds), not quantitative (AgentOps has more skills than X marketplace). The anti-claim below makes the structural framing explicit.

## Cross-claim composition

This claim composes with `AOP-CLAIM-TRUST-FACTORY-FIVE-STEP-PRIMITIVE` (the corpus is the "evidence" step of the trust factory) and `AOP-CLAIM-README-AUTONOMOUS-FLYWHEEL` (the snapshot artifact is shared). The three together describe the same product surface from three operator vantage points: positioning vs marketplaces, primitive shape, and live-state verification.

## Anti-claim

Not claiming AgentOps has more skills than Tons-of-Skills or any other marketplace. Not claiming the corpus is automatically high-quality without operator participation (mining + curation are real steps). Not claiming the corpus is portable to non-git substrates (the contract is "markdown wiki in a tracked repo"; substituting a SaaS knowledge store breaks the portability claim). Not claiming inventory-vs-corpus is binary — both are legitimate; AgentOps is just an inventory-orthogonal product.

## Companion beads

- Claim ledger row owner: soc-kizn.5.
- A1 audit (closed 2026-05-11): soc-waod.
- All-claims-evidence-map entry: soc-wock.
- Evidence-file write (this cycle): cycle 47 of the 2026-05-10 evolution session.
