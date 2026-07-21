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
- one deployment-pinned `GC_BIN` shared by pack commands and managed sessions;
- one caller-supplied disposable rig, registered suspended;
- exact additional-directory grants for the private GC runtime paths and every
  configured physical rig root; packet dispatch rejects any workspace that is
  not exactly the selected rig root;
- one caller-selected AgentOps pack binding at city and primary-rig scopes;
- the built-in Codex and Claude providers with full SDK option-schema
  replacements exposing Codex's bounded auto-edit default, Refiner-only
  unrestricted delivery choices, Claude's safety-classified `auto` mode, and
  explicit role-selectable model choices;
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
- no `[[named_session]]` or always-on session. The thin executor pack has no
  Mayor or Refiner; the optional factory pack adds on-demand semantic roles
  around the same packet boundary.

The thin AgentOps pack defines explicit Codex and Claude single-packet
Implementer and Validator roles. Every agent selects its provider and model
explicitly; every packet also names `provider = codex | claude`, and the adapter
verifies Gas City's runtime session, provider, and exact launch model. Codex
uses Terra for implementation and Sol for validation. Claude uses Opus 4.8 for
implementation with a fresh interactive GC session; it is not a 3.3 validation
route. A fresh Validator context must not reuse the Implementer's context
identity.

When selected, `agentops-factory` imports the one-loop target: Fable Mayor,
Sol plan and fresh validation, Terra-high/Opus-medium implementation, and a
model-free serialized Refinery. Fable Refiner is zero-or-one read-only ambiguity
advice; Luna-high is support-only. The program records requested and actual
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
managed. Before city creation it requires `gc` and `bd` to be colocated and to
match one exact entry in `deploy/gc/toolchain.lock.json`; version-only matches
are rejected. `deploy/gc/materialize-toolchain.sh` builds the default qualified
pair from its two exact source commits and emits a local digest receipt without
modifying installed binaries. Bootstrap then clears inherited Gas City and
Dolt selectors, sets
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

`deploy/gc/teardown.sh` consumes that same marker and exact GC binary. It binds
supervisor shutdown to the private `GC_HOME`, waits for the supervisor's
destructive stop to finish, idempotently stops managed Dolt once more, and
refuses success while the city's tmux socket or path-scoped processes remain.
It does not delete the city or its bead history.

A deployment is stable enough for AgentOps use only after a real negative test
rejects an invalid envelope and three consecutive independent experiments
produce correct durable outcomes without operator nudges, retries, manual
session restarts, or transport repair. Restart the city between the first and
second successful experiments, require native-store/doctor checks to remain
clean, verify distinct implementer and validator context identities, exercise
both provider families (including cross-provider validation), and confirm that
no AGY process/config and no historical city state was used or modified.
