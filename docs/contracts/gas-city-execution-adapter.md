# Gas City execution adapter

Gas City is an optional execution transport for AgentOps. A caller must select
it explicitly. It may route bounded work into isolated Codex or interactive
Claude sessions, but it
does not change the AgentOps semantic loop:

```text
caller-owned intent
  -> Plan once
  -> Implement once (isolated candidate)
  -> fresh Validate over the exact candidate
  -> verdict.v2: PASS | FAIL | NOT_PROVEN
  -> report and stop
```

Gas City owns session transport, rig isolation, provider launch, and work
routing. AgentOps owns exact intent and subject identity, derived evidence,
freshness requirements, criterion coverage, and the durable verdict. Closing a
Gas City work item is therefore not proof that the AgentOps experiment passed.

## Deployment shape

The deployment is intentionally smaller than a traditional Gas Town:

- one new city with a private `GC_HOME` and `GC_ISOLATED=1`;
- one persisted private loopback supervisor port, distinct from any ambient
  machine-wide supervisor configuration;
- one private `CODEX_HOME` that links, but does not copy, an explicitly selected
  authenticated `auth.json`;
- one deployment-pinned schema-3 toolchain receipt containing exact `gc`, `bd`,
  `ao`, and `agentops-gc-delivery` binaries; delivery also binds the exact Git,
  GitHub CLI, and Bash executables it may invoke;
- one caller-supplied disposable rig, registered suspended;
- exact additional-directory grants for the private GC runtime paths and every
  configured physical rig root; packet dispatch rejects any workspace that is
  not exactly the selected rig root;
- one caller-selected AgentOps pack binding at city and primary-rig scopes;
- the built-in Codex and Claude providers with full SDK option-schema
  replacements exposing Codex's bounded auto-edit default, interactive
  unrestricted sessions where explicitly selected, Claude's safety-classified
  `auto` mode, and explicit role-selectable model choices;
- Codex workspace-write network access enabled for the private loopback Dolt
  endpoint, without broadening its declared writable roots;
- GC-owned interactive tmux sessions for every Claude role, with empty
  `print_args` and deployment rejection of `-p` / `--print`;
- a workspace-wide cap that defaults to one active session and must be raised
  explicitly for bounded factory qualification;
- no `workspace.provider`; the required explicit provider catalog entries still
  materialize SDK-defined generic targets, so deployment patches suspend them
  at city and managed-rig scope;
- scaffold maintenance pools (`bd.dog` and `core.control-dispatcher`) are also
  suspended so they cannot consume the execution-only city's session cap;
- no always-on model session. The thin executor pack has no Mayor or Refiner;
  the optional factory pack exposes on-demand Fable Mayor and gated ambiguity
  sessions around the same packet boundary.

The thin AgentOps pack defines explicit Codex and Claude single-packet
Implementer and Validator roles. Every agent selects its provider and model
explicitly; every packet also names `provider = codex | claude`, and the adapter
verifies Gas City's runtime session, provider, and exact launch model. Codex
uses Terra for implementation and Sol for validation. Claude uses Opus 4.8 for
implementation with a fresh interactive GC session; it is not a 3.3 validation
route. A fresh Validator context must not reuse the Implementer's context
identity.

When selected, `agentops-factory` imports one bounded native chain: Fable
Mayor and fresh Sol plan checks run once, a one-shot feeder admits only a
checked PASS graph, then explicitly routed Terra-high or Opus-medium
implementation and fresh Sol validation terminalize semantics before the
model-free delivery reducer. There is no Mayor rescope retry, Formula drain,
or clean-path Refiner wake; Fable Refiner is nonbinding ambiguity advice only
and Luna-high is support-only. The program records requested and actual
role, model, provider, reasoning, and fallback facts in the 3.3-authoritative
`factory-role-request.v2` and `factory-role-response.v2` contracts. The work
unit is still a bead, never the pack. Candidate rigs expose their admitted
writer and fresh Sol validator; routine delivery exposes no model route. The
factory does not broaden the executor packet's semantic authority. The adapter
rejects an actual runtime that violates the requested fixed role policy or
silently downgrades it; in particular validation is requested and actual
Sol-high/Codex, and every fallback object is exactly `allowed=false`,
`used=false`, and `reason=null`, never Terra-low.

## SDK-owned configuration

| Artifact | Authority and mutation rule |
|---|---|
| AgentOps `deploy/gc/toolchain.lock.json` | Exact accepted GC/official-Beads source pairs and qualification state; same-version unlisted builds are not equivalent |
| AgentOps executor `pack.toml` | Portable role, prompt, and command contract; edited in AgentOps and linted before deployment |
| Live city `pack.toml` | Created by the current `gc init`; its built-in imports and pins remain SDK-owned; bootstrap adds only `[imports.agentops]` |
| Live `city.toml` | Created from `deploy/gc/city.toml`; `gc rig add` owns logical rig entries; bootstrap adds the primary rig import and generic-target patches; the factory adds serialized, fail-closed rig patches for dedicated worktree rigs |
| Live `.gc/site.toml` | Machine-local SDK state containing workspace identity and physical rig paths; never copied into portable config |
| `.gc-home/supervisor.toml` | Bootstrap-owned private loopback address, selected once before start and preserved on managed reruns |
| `.gc-home` and `.gc/codex-home` | Remaining runtime/session state private to this city; only the selected external Codex auth file is linked in |
| `.gc/agentops-bootstrap.json` | Recoverable bootstrap identity, including the selected pair id/status, full source commits, exact GC/Beads paths and binary digests, and auth source; packet commands use its GC path when ambient `GC_BIN` is absent |

For a local pack with uncommitted changes, direct TOML with a plain absolute
`source` is the current Gas City SDK's documented behavior; `gc import add`
would otherwise promote the worktree's `HEAD` and omit the uncommitted subject.
For a released pack, use the normal import command and a durable commit pin.

## Work and evidence flow

1. The caller supplies one resolved intent source. If it has no durable tracker
   artifact, AgentOps snapshots the exact bytes under their digest.
2. The adapter derives a deterministic transport-bead ID from the selected
   rig's bead prefix and exact packet digest, creates or reconciles that bead,
   and records the explicit run envelope and resolved absolute adapter path.
   The bead is transport work, not a second plan or source of acceptance
   criteria. Its identity is persisted before routing; a restart inspects the
   same bead's `gc.routed_to` metadata and never slings an already-routed bead a
   second time. The packet and intent are hashed before dispatch and rechecked
   before return.
3. The implementer consumes the envelope in an isolated rig/workspace and
   performs one bounded RED-to-GREEN experiment.
4. AgentOps derives the candidate manifest, changed paths, subject digest, and
   factual scope/check receipts. The implementer does not transcribe a
   candidate packet by hand.
5. The controller supplies a second explicit packet for a distinct fresh
   Sol-high/Codex context to validate the exact subject, intent digest, scope,
   evidence, and every acceptance criterion. 3.3 has no cross-provider
   validation route or runtime fallback.
6. Validate emits exactly one `verdict.v2` beneath the evidence workspace. The
   GC adapter returns the artifact reference and transport/runtime evidence,
   without copying the semantic result into its own response, then stops.
7. The optional factory writes and closes the immutable semantic terminal before
   PASS-only delivery admission. Its deterministic reducer may later reconcile
   moving `main`, PR, protected CI, and merge without reopening that terminal.

The packet evidence directory also contains digest-bound
`runtime-transport.json` and `runtime-result.json` receipts. They make a
controller restart replayable: a prepared transport resumes from its existing
bead, and a completed runtime result is revalidated against the closed bead,
actual provider session and launch model, artifacts, and current exact subject before it is
returned. These receipts do not own work status, semantic retry, or verdict
authority; the bead and `verdict.v2` retain those meanings.

Successful role responses must reference digest-bound files under the packet's
canonical `.gc/agentops/<packet-id>/` evidence directory. This transport plane
is excluded from the judged subject and is writable by Codex and Claude;
Codex's protected `.agents` tree is intentionally not used. Root and nested
GC, Git, Beads, and provider metadata directories are excluded from subject
manifests, including GC's per-item session scaffolding. A validation response
must reference exactly one valid `verdict.v2`; its intent, subject, author,
validator, and runtime freshness facts must match the packet, manifests, and
the actual provider session and role-pinned launch model reported by Gas City.
The validate envelope also binds
the implementer's runtime-derived scope receipt, which is recomputed from the
two manifests before dispatch. Response schemas are enforced by the adapter
itself because an SDK-exposed command result schema is not an execution-time
validator.

The adapter does not retry a failed verdict, revise intent, merge Git changes,
close the caller's tracker, publish, release, or choose the next experiment.
Those remain caller/repository policy. Gas City health, a zero exit status, or
a closed transport item cannot upgrade `FAIL` or `NOT_PROVEN` to `PASS`.

## Bootstrap and stability gate

`deploy/gc/bootstrap.sh` refuses any nonempty city it did not previously mark as
managed. Before city creation it requires `gc`, `bd`, `ao`, and the delivery
reducer to match one schema-3 receipt rooted in the exact accepted
GC/official-Beads pair; version-only matches and extra receipt runtimes are
rejected. `deploy/gc/materialize-toolchain.sh` builds those four binaries from
the exact sources without modifying installed binaries. Bootstrap clears
inherited Gas City, Dolt, and generic OTel selectors, sets
`GC_HOME=<city>/.gc-home` and `GC_ISOLATED=1`, persists a private loopback
supervisor port for service-manager launches, links and checks Codex auth,
checks first-party Claude authentication without starting a session,
checks that the installed Claude CLI exposes auto-mode policy,
disables implicit Git-to-Dolt remote synchronization during rig registration,
invokes current `gc init` to generate the SDK-owned scaffold and built-in pins,
adds the two local imports and rig-scoped generic-provider suspension with
same-directory atomic replacements, and runs these preflights before an
optional start:

```text
gc lint <pack>
CODEX_HOME=<city>/.gc/codex-home codex login status
claude auth status --json
claude auto-mode defaults
gc config show
gc config explain
gc import status --json
```

GC v1.3.5 native metrics and logs are enabled with an explicit endpoint pair.
Telemetry mode `auto` records a durable degraded state when the pair is absent,
`required` fails before mutation, and `off` is explicit. The generic
`OTEL_EXPORTER_OTLP_ENDPOINT` fallback is always cleared so desktop-wide state
cannot silently override the managed policy.

`deploy/gc/teardown.sh` consumes that same marker and exact GC binary. It binds
supervisor shutdown to the private `GC_HOME`, waits for the supervisor's
destructive stop to finish, idempotently stops managed Dolt once more, and
refuses success while the city's tmux socket or path-scoped processes remain.
It does not delete the city or its bead history.

A 3.3 candidate is release-ready only after the deterministic repository gate,
exact official GC v1.3.5 pack/Formula inspection, materialized-toolchain and two
clean-bootstrap proofs, and a fresh Sol binding verdict all PASS over the frozen
subject. It then gets one bounded width-two live canary in a new city and rig,
with telemetry `required`: one admitted Terra-high and one admitted Opus-medium
implementation, fresh author-distinct Sol-high validation, semantic terminal
closure before delivery, moving-main PR/CI/rebase/merge convergence, cold replay
without duplicate effects, and zero clean-path Refiner or Luna wakes. The old
controller city remains suspended. The first live failure stops qualification;
there is no recursive repair through that city and no second attempt under the
same candidate. Missing provenance, telemetry signals, exact role identity,
terminal/certificate binding, protected delivery evidence, or clean teardown is
`NOT_READY`, not a discretionary waiver.
