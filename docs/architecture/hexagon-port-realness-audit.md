# Hexagon Port-Realness Audit

> Empirical ground-truth inventory of the AgentOps runtime hexagonal seam, taken
> 2026-05-23 against `main`. Bead: `soc-upwor` (epic `soc-zvhsl` — "3.0: make the
> hexagon real"). This is the disjoint adapter backlog the adapter-building beads
> execute against: `soc-rg5z0` (WorkspacePort), `soc-ebgjk` (TrackerPort),
> `soc-21mrg` (loop-state), `soc-vzyhn` (corpus FS).
>
> Companion to [Ports and Adapters](ports-and-adapters.md) (conceptual overview).
> This audit corrects two stale claims in that doc and in the epic survey: the
> hexagon is **not** "only `storage_fs` is real" — sixteen `production*` adapters
> exist in `cli/cmd/ao/` (compile-cycles 112/113) plus three in `cli/internal/wiki/`.
> The real gaps are narrower and more specific, enumerated below.

## Headline counts

| Metric | Count |
|---|---|
| Declared port interfaces (`cli/internal/ports/*.go`, non-`inmemory`, non-`_test`) | **26** |
| Ports with an in-memory double (`inmemory_*.go`) | 20 |
| Ports with a **real** (non-in-memory) adapter | **18** |
| Ports with **no conforming adapter at all** (in-memory-only or signature-mismatched) | **8** |
| Domains called **directly from `cli/cmd/ao`** with no port (git, bd) | **2** (git: 35 callsites, bd: 10 callsites) |

The epic `soc-zvhsl` survey says "21 ports declared, only `storage_fs` has a real
adapter." Both numbers are stale: 26 interfaces are declared, and 18 have real
adapters. The accurate backlog is the 8 no-adapter ports plus the 2 directly-coupled
domains, scoped below.

## Port realness table

Legend — **declared?**: interface exists in `cli/internal/ports/`. **in-mem?**: an
`inmemory_*.go` double exists. **real?**: a non-in-memory adapter that satisfies the
interface exists. **consumed**: `via-port` = a command/package calls the adapter
through the port's request/result types; `bypassed` = the domain is reached without
the port; `unwired` = real adapter exists but no command constructs it.

| Port | declared? | in-mem? | real? | consumed | real-adapter / evidence (file:line) |
|---|---|---|---|---|---|
| `PacketRepository` | ✅ `storage.go:12` | ❌ | ✅ | via-port | `adapters/storage_fs/packet_repo.go:28` (`var _ ports.PacketRepository`) |
| `CIStatusPort` | ✅ `ci_status.go:67` | ✅ | ✅ | via-port | `cmd/ao/ci_status_adapter.go:128`; wired `cmd/ao/ci.go:115`; runs `gh` at `ci_status_adapter.go:39` |
| `CitationPort` | ✅ `citation.go:78` | ✅ | ✅ | via-port | `cmd/ao/citation_port_adapter.go:82`; wired `cmd/ao/citation_cmd.go:100` |
| `ClaimEvidencePort` | ✅ `claim_evidence.go:57` | ✅ | ✅ | via-port | `cmd/ao/claim_evidence_adapter.go:104` |
| `ClaimEvidenceBinderPort` | ✅ `claim_evidence_binder.go:69` | ✅ | ✅ | via-port | `cmd/ao/claim_evidence_binder_adapter.go:199`; wired `cmd/ao/claim_cmd.go:166` |
| `ConvergenceCheckPort` | ✅ `convergence_check.go:68` | ✅ | ✅ | via-port | `cmd/ao/convergence_check_adapter.go:63`; wired `cmd/ao/loop_converged.go:126` |
| `CorpusReaderPort` | ✅ `corpus_reader.go:48` | ✅ | ✅ | via-port (partial) | `cmd/ao/corpus_reader_adapter.go:145`; wired `cmd/ao/corpus_inject.go:107`; real walk `corpus_reader_adapter.go:64` |
| `CorpusWriterPort` | ✅ `corpus_writer.go:52` | ✅ | ✅ | via-port (partial) | `cmd/ao/corpus_writer_adapter.go:119`; wired `cmd/ao/corpus_capture.go:189` |
| `EventBusPort` | ✅ `event_bus.go:45` | ✅ | ✅ | via-port | `cmd/ao/event_bus_adapter.go:156` |
| `FactoryAdmissionPort` | ✅ `factory_admission.go:75` | ✅ | ✅ | via-port | `cmd/ao/factory_admission_adapter.go:100` |
| `FindingCompilerPort` | ✅ `finding_compiler.go:64` | ✅ | ✅ | via-port | `cmd/ao/finding_compiler_adapter.go:167` |
| `GateRunnerPort` | ✅ `gate_runner.go:69` | ✅ | ✅ | via-port | `cmd/ao/gate_runner_adapter.go:115`; wired `cmd/ao/gate_cmd.go:93`; runs `bash scripts/check-*.sh` `gate_runner_adapter.go:53,61` |
| `HarnessPort` | ✅ `harness.go:49` | ✅ | ✅ | via-port | `cmd/ao/harness_adapter.go:131`; wired `cmd/ao/harness_cmd.go:100` |
| `HypothesisLedgerPort` | ✅ `hypothesis_ledger.go:52` | ✅ | ✅ | via-port | `cmd/ao/hypothesis_ledger_adapter.go:158`; wired `cmd/ao/loop_hypothesis.go:103` |
| `LoopReaderPort` | ✅ `loop_reader.go:57` | ✅ | ✅ | via-port (evolve lane only) | `cmd/ao/loop_reader_adapter.go:158`; wired `cmd/ao/loop.go:142,229` (reads `.agents/evolve/cycle-history.jsonl`) |
| `LoopWriterPort` | ✅ `loop_writer.go:25` | ✅ | ✅ | via-port (evolve lane only) | `cmd/ao/loop_writer_adapter.go:131`; wired `cmd/ao/loop_append.go:141` |
| `OperatorPort` | ✅ `operator.go:35` | ✅ | ✅ | via-port | `cmd/ao/operator_adapter.go:123`; wired `cmd/ao/operator_cmd.go:148` |
| `WikiIndexPort` | ✅ `wiki_index.go:45` | ❌ | ✅ | via-port | `cli/internal/wiki/index.go:80,103` (`Reindex`/`Records`); wired `cmd/ao/wiki.go:213,217` |
| `FrontmatterCodecPort` | ✅ `frontmatter_codec.go:46` | ❌ | ✅ | **unwired** | `cli/internal/wiki/frontmatter.go:223` (`PortCodec` shim, `NewPortCodec` at :227); no command constructs it |
| `FreshnessPolicyPort` | ✅ `freshness_policy.go:66` | ❌ | ⚠️ no conforming | bypassed | `cli/internal/wiki/freshness.go:189` is the production *logic* but `Evaluate` (`freshness.go:228`) uses domain types, **not** the port signature `Evaluate(claimVolatility, claimAuthority string, …)` (`freshness_policy.go:68`) — no shim exists |
| `CloseoutPort` | ✅ `closeout.go:42` | ✅ | ❌ | bypassed | in-memory-only (`inmemory_closeout.go`); **required-7** |
| `ContextCompilerPort` | ✅ `context_compiler.go:54` | ✅ | ❌ | bypassed | in-memory-only (`inmemory_context_compiler.go`); **required-7** |
| `SafetyPolicyPort` | ✅ `safety_policy.go:51` | ✅ | ❌ | bypassed | in-memory-only (`inmemory_safety_policy.go`); **required-7** |
| `WorkspacePort` | ✅ `workspace.go:32` | ✅ | ❌ | bypassed | in-memory-only (`inmemory_workspace.go`), **not constructed anywhere**; **required-7** → `soc-rg5z0` |
| `IssueTracker` | ✅ `tracker.go:7` | ❌ | ❌ | bypassed | no adapter, no in-memory double; `bd` reached directly → `soc-ebgjk` |
| `LLMClient` | ✅ `llm.go:7` | ❌ | ❌ | n/a | no adapter, no in-memory double (forecast port) |

**8 no-adapter ports:** `FreshnessPolicyPort`, `CloseoutPort`, `ContextCompilerPort`,
`SafetyPolicyPort`, `WorkspacePort`, `IssueTracker`, `LLMClient` — plus
`FrontmatterCodecPort` whose real adapter exists but is **unwired** (counts as
"real" in the realness column but is a latent gap).

### Required-port set (`scripts/check-hook-port-replacements.sh`)

The CI gate enforces seven ports as hook-replacement targets:
`Closeout / ContextCompiler / EventBus / GateRunner / Harness / SafetyPolicy / Workspace`
(`scripts/check-hook-port-replacements.sh`, `REQUIRED_PORTS` array). Their backing:

| Required port | Real adapter? | Notes |
|---|---|---|
| `EventBusPort` | ✅ | `cmd/ao/event_bus_adapter.go:156` |
| `GateRunnerPort` | ✅ | `cmd/ao/gate_runner_adapter.go:115` — shells real `check-*.sh` |
| `HarnessPort` | ✅ | `cmd/ao/harness_adapter.go:131` |
| `CloseoutPort` | ❌ in-memory-only | no `production*` / fs adapter |
| `ContextCompilerPort` | ❌ in-memory-only | no `production*` / fs adapter |
| `SafetyPolicyPort` | ❌ in-memory-only | no `production*` / fs adapter |
| `WorkspacePort` | ❌ in-memory-only | not even constructed; `soc-rg5z0` |

So **3 of the required 7 are real; 4 are in-memory-only** (Closeout, ContextCompiler,
SafetyPolicy, Workspace). The gate only checks that the *interface + in-memory double*
exist and are referenced by a non-`remove` hook lease — it does **not** assert a real
adapter, which is exactly why these four passed CI while remaining doubles.

## Direct-coupling hotspots

Domains the CLI reaches without going through a port. These are the bypass surfaces the
adapter beads collapse.

### git — 35 direct `exec.Command("git", …)` callsites in `cmd/ao` (→ `soc-rg5z0` WorkspacePort)

Worktree lifecycle and repo queries shell out to `git` directly. `WorkspacePort`
(`workspace.go:32`, `Setup`/`Cleanup`) is declared to absorb exactly this, but its
only impl is `inmemory_workspace.go` and **nothing constructs it**. Representative
worktree-lifecycle coupling (the load-bearing subset for `soc-rg5z0`):

- `cmd/ao/rpi_parallel.go:281` — `git worktree add -b <branch> <path>`
- `cmd/ao/rpi_parallel.go:200` — `git worktree remove --force <path>`
- `cmd/ao/rpi_cleanup.go:258` — `git worktree list --porcelain`
- `cmd/ao/rpi_cleanup.go:464` — `git worktree remove --force <path>`
- `cmd/ao/rpi_cleanup.go:487` — `git worktree prune`
- `cmd/ao/worktree.go:180` — `git rev-parse --show-toplevel`
- `cmd/ao/worktree.go:381` — `git -C <wt> status --porcelain -uall`
- `cmd/ao/rpi_parallel.go:412` — `git merge <branch> --no-ff`

Plus repo-root/status reads scattered across `beads_resume.go:267`, `curate.go:233`,
`hooks_run.go:156,394`, `init.go:420`, `vibe_check.go:114`, `rpi_status.go:761`,
`beads_audit_cluster.go:120`. There is **no central git wrapper** — each callsite
builds its own `exec.Command`. (Helpers `worktree_config.go`, `internal/rpi/worktree.go`
exist but do not gate the subprocess.)

### bd — 10 direct `exec.Command("bd", …)` callsites in `cmd/ao` (→ `soc-ebgjk` TrackerPort)

`IssueTracker` (`tracker.go:7`, `Mode`/`CreateEpic`/`CreateIssue`) has **no
implementation at all** — no real adapter and no in-memory double. `bd` is invoked
directly:

- `cmd/ao/beads.go:42` — generic `bd <args>` passthrough
- `cmd/ao/session_bootstrap.go:210` — `bd ready --json`
- `cmd/ao/beads_resume.go:66` — `bd show <id> --json`
- `cmd/ao/beads_stale.go:140` — `bd list --status in_progress --json …`
- `cmd/ao/plans.go:710` — `bd list --type epic --json`
- `cmd/ao/quickstart.go:474` — `bd init --prefix <p>`
- `cmd/ao/overnight_packets.go:961,995,1025` — `bd list … --json`
- `cmd/ao/beads_audit_cluster.go` (git, listed above)

`IssueTracker`'s declared surface (epic/issue *creation*) is narrower than the actual
`bd` usage (mostly read/list). `soc-ebgjk` should widen the port to cover the
ready/list/show read paths the CLI actually depends on, not just creation.

### loop-state — split-lane persistence (→ `soc-21mrg`)

The `soc-21mrg` bead and the epic survey cite `.agentops/loop-state.json` read/written
in `rpi_phased_*`. **That literal path does not exist** anywhere in `cli/` — the claim
is imprecise. The real situation is two distinct loop-persistence lanes:

1. **Evolve lane** — `.agents/evolve/cycle-history.jsonl`, already behind
   `LoopReaderPort`/`LoopWriterPort` via `productionLoopReader`/`Writer`
   (`loop_reader_adapter.go:65`, wired `loop.go:142`). **Already a real adapter.**
2. **RPI supervisor lane** — `rpi_loop_supervisor.go` persists its own state with raw
   `json.MarshalIndent` + `os.WriteFile`/`os.ReadFile`, **bypassing the ports**:
   - `rpi_loop_supervisor.go:1007` — `json.MarshalIndent(l.meta, …)` (meta file)
   - `rpi_loop_supervisor.go:1029,1034` — `os.ReadFile` + `json.Unmarshal` of meta
   - `rpi_loop_supervisor.go:1071,1077` — append to `telemetry.jsonl`
   - `rpi_loop_supervisor.go:1131,1139` — `json.Marshal` + atomic `os.WriteFile(tmp)`
   Note `rpi_status.go:677` also reads a fallback flat `phased-state.json`.

   ⚠️ `rpi_loop_supervisor.go` is the **legacy RPI lane** the repo CLAUDE.md flags as
   load-bearing-but-frozen ("do not write new tests or features for the legacy RPI
   lane"). `soc-21mrg` must target the **live** loop-state surface, not retrofit the
   frozen supervisor. Confirm which lane is current (gc bridge vs legacy tmux) before
   building; the atomic-file adapter belongs behind `LoopReaderPort`/`LoopWriterPort`
   covering the live lane, not a new write to the frozen one.

### corpus FS — compile/mine/grow/defrag walk `.agents` directly (→ `soc-vzyhn`)

`CorpusReaderPort`/`CorpusWriterPort` have real adapters
(`corpus_reader_adapter.go:64` does a real `filepath.WalkDir`), wired into the
`corpus inject`/`corpus capture` commands. **But the knowledge-flywheel core
(compile = mine/grow/defrag) does not use them** — it walks the filesystem directly:

- `cmd/ao/compile.go:464,561` — `os.ReadDir(target)` over `.agents/compiled`
- `cli/internal/corpus/fitness.go` — direct corpus walk
- `cli/internal/context/run.go` — direct corpus walk (context assembly)
- `cmd/ao/context_assemble.go:346` — `os.ReadDir(dir)`
- `cmd/ao/maturity.go:472,945` — `os.ReadDir` / `filepath.WalkDir` over `.agents/learnings`
- `cmd/ao/knowledge.go:598,648`, `knowledge_native.go:267,685` — `os.ReadDir`
- `cli/internal/doctor/fix_knowledge.go`, `internal/daemon/{snapshot,wiki_jobs,store}.go`

50 `filepath.Walk`/`WalkDir`/`os.ReadDir` callsites live in `cmd/ao` non-adapter files
alone. `soc-vzyhn` routes the mine/grow/defrag core through `CorpusReaderPort`/
`CorpusWriterPort` so the flywheel becomes testable without on-disk fixtures.

### gh / CI — already behind a port

`gh` is invoked only inside the `CIStatusPort` adapter (`ci_status_adapter.go:39`),
which is the correct pattern. **No CI/gh direct-coupling backlog.** Gate execution is
likewise behind `GateRunnerPort` (`gate_runner_adapter.go:53,61` shells `check-*.sh`).

## Recommended adapter build order

Priority by load-bearing-ness × bypass blast radius, mapped to existing beads.

1. **`WorkspacePort` → `cli/internal/adapters/workspace_git/`** (`soc-rg5z0`, P1).
   Highest leverage: 35 direct `git` callsites, the port is declared and
   **not even constructed**, and worktree lifecycle is load-bearing for the
   multi-agent discipline (`bd worktree create` is mandatory per CLAUDE.md). Collapse
   the `rpi_parallel.go` / `rpi_cleanup.go` / `worktree.go` worktree subset behind
   `Setup`/`Cleanup` first; leave read-only `rev-parse` queries for a follow-up.
   Closes one of the 4 in-memory-only required-7 ports.

2. **`IssueTracker` → bd-backed adapter** (`soc-ebgjk`, P1).
   `bd` has 10 direct callsites and the port has **zero** implementations (not even an
   in-memory double — build that for L1/L2 testability too). Widen the port to the
   read paths (`ready`/`list`/`show`) the CLI actually uses before wiring, then a
   `tasklist` filesystem fallback for the `Mode() == "tasklist"` branch.

3. **`CorpusReader`/`CorpusWriterPort` adoption in the flywheel core** (`soc-vzyhn`, P2, "L").
   The adapters already exist and are real — the work is **migration, not greenfield**:
   route `compile` (`compile.go`), `internal/corpus/fitness.go`, and
   `internal/context/run.go` through the existing ports instead of direct walks. Lower
   risk than 1–2 (no new I/O surface), but largest in scope. Decouples the
   knowledge-flywheel from file layout and makes mine/grow/defrag fixture-free.

4. **Loop-state atomic adapter for the live lane** (`soc-21mrg`, P2).
   The evolve lane is already real; the gap is the RPI supervisor lane — but that lane
   is **frozen legacy code**. Resolve the lane question first (gc bridge is the live
   path per CLAUDE.md). Build the atomic-file adapter behind the existing
   `LoopReaderPort`/`LoopWriterPort` for the live loop-state surface; do not add
   features to `rpi_loop_supervisor.go`.

### Out-of-scope / deferred (no bead yet)

- **`CloseoutPort`, `ContextCompilerPort`, `SafetyPolicyPort`** — the other three
  in-memory-only required-7 ports. Real adapters needed to satisfy the *spirit* (not
  just the letter) of `check-hook-port-replacements.sh`. File follow-up beads under
  `soc-zvhsl`.
- **`FreshnessPolicyPort`** — production logic exists in `cli/internal/wiki/freshness.go`
  but the method signature does not match the port; add a `PortFreshnessPolicy` shim
  (mirror the `PortCodec` pattern at `frontmatter.go:223`).
- **`FrontmatterCodecPort`** — real `PortCodec` adapter exists but is **unwired**;
  a one-line wiring in the frontmatter consumer closes it.
- **`LLMClient`** — forecast port (`llm.go:7`); no adapter and no consumer. Defer.
- **`check-hook-port-replacements.sh` gate gap** — consider extending the gate to
  assert a non-in-memory adapter for required ports so a future port can't regress to
  a double silently.
