# Skill behavioral probes (`evals/skill-probes/`)

> **HONESTY (read first).** A probe measures **BEHAVIOR-CHANGE, not
> quality-uplift.** It answers exactly one question: when the declared
> treatment source is included in the treatment prompt but omitted from the
> control, does the agent actually **DO** the scored thing differently — a tool
> call made, an artifact produced, a sequence followed? It **never** scores
> whether the text merely mentions the guidance, and it **never** claims the
> output is better. `BEHAVIORAL` means the treatment increased the scored
> response behavior, `REGRESSIVE` means it reduced it, and `INERT` means the
> two measured rates were equal. Small N (default 2–3) is **DIRECTIONAL, not
> statistical.** Do not overclaim (ADR-0011 discipline).

## Why this exists

Skills are half the product, but tier badges are editorial — the only
enforcement was an enum-membership check. A 2026-06-30 graphify classifier
regression motivated this harness, but its reconstructed fixtures predate the
capture-manifest contract and are now `LEGACY-UNVERIFIED`; they are not current
behavioral evidence. Documentary acceptance still does not imply behavioral
acceptance. This harness measures the behavior separately and fails closed when
capture provenance is absent.

## What a probe is

A directory `evals/skill-probes/<id>/`:

| File | Role |
|------|------|
| `probe.json` | metadata: id, skill, reps, behavior, discriminator, and required `treatment_source` |
| `question.md` | the scenario question — **IDENTICAL for both arms** |
| `treatment-prelude.md` | distilled guidance used only when `treatment_source` is `injected-prelude` |
| `discriminator.sh` | a **deterministic** behavioral check over the harness-owned response envelope: exit `0`=PRESENT, `1`=ABSENT, `2`=infra. **Checks the ACTION, never a mention.** |
| `fixtures-<run>/` | one immutable live capture: structured JSONL `control-<n>.txt` / `treatment-<n>.txt`; the directory name must be new for every capture |
| `fixtures-<run>/capture-contract.json` | pre-dispatch, self-contained copies of the exact probe inputs, canonical skill bytes, per-arm prompt bytes, requested producer config, runtime executable identity, counterbalanced schedule, and scoring contract |
| `fixtures-<run>/fixture-set.json` | v3 binding over that pre-existing capture contract, exact transcript inventory/hashes, native Codex thread ids, treatment source, and every material evaluator/dispatch helper; required for replay |

`treatment_source` has two deliberately different meanings:

- `canonical-skill`: treatment prompt = the exact bound
  `skills/<skill>/SKILL.md` bytes + the question; control = the question. Only
  this mode can count toward product/judgment skill coverage. It measures the
  response-shape effect of including those canonical bytes, not automatic skill
  discovery, task quality, or an executed outcome.
- `injected-prelude`: treatment prompt = the bound `treatment-prelude.md` + the
  question; control = the question. This is replayable evidence about that
  distilled prelude only. It is not full-skill activation and the coverage gate
  refuses to count it as skill coverage.

In both modes the question is identical and the declared treatment source is
the only arm difference.

## Running

```bash
# Live A/B into a new immutable fixture set and a new scorecard path:
run_tag="low-$(date -u +%Y%m%dT%H%M%SZ)"
scorecard_dir="docs/evals/scorecards/$(date -u +%F)"
mkdir -p "$scorecard_dir"
bash scripts/probe-skill.sh \
  --probe anti-ceremony-creation-gate-v2 --live --capture --reps 2 \
  --fixtures "fixtures-$run_tag" --model gpt-5.6-luna --effort low \
  --output "$scorecard_dir/anti-ceremony-$run_tag.json"

# Compatibility-replay the retained v2 classification without dispatching a
# model. It is historical and coverage-ineligible under the v3 contract:
bash scripts/probe-skill.sh \
  --probe anti-ceremony-creation-gate-v2 --replay \
  --fixtures fixtures-low-2026-08-16-v2b
```

Live capture dispatches `codex exec --json`, the sanctioned headless path. The
harness writes a structured `agentops.probe-input.v1` event from the exact
prompt file passed on stdin, followed by the native Codex JSONL events. It
refuses to replace an existing fixture set or scorecard. Replay verifies every
bound byte, prompt event, Codex thread id, and the exact fixture inventory before
scoring, and refuses legacy directories with no `fixture-set.json`. A v3 fixture
remains self-contained for replay after the current probe or skill changes; the
coverage gate separately requires its captured canonical skill and probe inputs
to match the current repository. Historical v1/v2 manifests remain
compatibility-only and cannot count as current tier coverage.

Live dispatch follows the contract-bound alternating schedule: odd reps run
control then treatment, even reps treatment then control. `--reps`, when
provided, must equal `probe.json.reps` (maximum 20). Before discrimination the
harness structurally extracts only the final completed Codex `agent_message`
from the bound JSONL event stream. Human `codex` / `tokens used` delimiter
parsing exists only for compatibility replay of v1/v2 fixtures and cannot shape
a v3 response. A missing or malformed event boundary, an over-time
discriminator, or a discriminator that mutates its read-only scoring snapshot
degrades that rep instead of scoring prompt-echoed text.

Tier coverage additionally requires an explicit model and effort plus a
PATH-resolved native Codex executable identity captured before dispatch;
explicit binary overrides remain replayable but are coverage-ineligible. This
binds local capture provenance, not model quality, external runtime attestation,
or cross-platform reproducibility.

**The seal (`capture-contract.json` v3, `seal` block).** The 2026-08-28
contamination was control-arm reps reading `skills/<skill>/SKILL.md` off the
checkout, so both arms held the same bytes. The prevention is a filesystem
seal around dispatch, and a counted row has to prove it ran under one. Before
`snapshot`, the harness writes its `agentops-skill-probe-seal.v1` record
(`seal.json`) into the capture stage, or hands it to `snapshot --seal-file`;
the snapshot reduces it to the contract `seal` block. The sidecar may stay
beside the contract, where create/verify re-derive it against the bound block
and refuse a swapped record; a stage with no record is bound as mode `none`.
`coverage_eligible` is true only for a native, fully specified producer under a
`seatbelt` seal. Pre-seal v2 contracts (the 2026-08-26 sets) load as
`legacy-unsealed`: replayable, never coverage.

The bound block is `{mode, platform, mechanism, sandbox_exec, wrap, timeout_bin,
timeout_seconds, network, denied_read_roots, denied_read_data_roots, denied_link_roots,
writable_roots, dev_write_paths, allowed_read_paths, launcher_chain,
launcher_invoked,
launcher_sha256, rep_env, env_allowlist, run_root, workspace_root,
dispatch_root, git_common_root, real_tmpdir, real_codex_home, cache_root,
config_sanitized, config_sha256, config_text, auth_copied, profile_sha256,
original_home, repository_root}` (`original_home` = the record's `real_home`,
`repository_root` = the resolved parent of the skills dir). A record carrying
any field outside that set is refused rather than reduced, because an unknown
field is either a shape the verifier cannot check or a place to hide one.

**The block rebuilds its own profile.** `render_seal_profile(block)` in
`scripts/lib/probe-fixture-metadata.py` emits the profile text from the block,
and it is the only writer of one: the harness renders through it at capture
time, and `verify-scorecard` renders again and requires the digest to equal
`profile_sha256`. Before this the roots were assertions ALONGSIDE an opaque
profile, so a record claiming `writable_roots ["/"]`, an `allowed_read_paths`
entry pointing at the main checkout's SKILL.md, or a null `real_tmpdir` was
coverage-eligible while the profile said something else entirely.

**Which fields the verifier PINS.** These are compared against a constant or
recomputed from the filesystem, so the recorded value cannot be whatever the
harness chose to write:

- the profile: the block reconstructs the profile text to `profile_sha256`;
- `platform` is Darwin, `mechanism` is `sandbox-exec`, `sandbox_exec` is
  `/usr/bin/sandbox-exec`, and `wrap` equals `[sandbox_exec, "-p",
  profile_sha256, timeout_bin, "--foreground", timeout_seconds]`;
- `network.mode` is `proxy-allowlist`, `network.hosts` is a subset of the five
  pinned entries, `network.ports` is exactly `[443]`, `network.unix_sockets` is
  empty, and the proxy authority is on `127.0.0.1`;
- `config_text` equals what the config generator emits for the bound effort, and
  `config_sha256` is its digest;
- `dev_write_paths` equals the four-device constant, and every `writable_roots`
  entry sits under `run_root`;
- `env_allowlist` is a subset of the harness constant plus `PROBE_*` seams;
- the launcher chain is bound as STRUCTURE, not as paths (a record the
  verifier checks for consistency anywhere, and against the live filesystem
  only on a host where the chain's head exists; on any other host the
  record is the evidence): each entry records
  what it is, either `{path, kind: symlink, target}` or, for the last,
  `{path, kind: file, sha256}`, and `launcher_invoked` records the path the
  harness resolved. The verifier checks on EVERY host that the chain starts at
  the invoked path, that each entry before the last is a symlink whose target is
  the next entry's path, that the last is the file whose digest is both
  `launcher_sha256` and the producer executable the manifest binds, that no path
  repeats, and that `allowed_read_paths` is exactly the chain paths a denied
  root covers. On a host that HAS the chain it also walks the filesystem and
  refuses any disagreement; on a host that does not, the record is the evidence
  and the walk is skipped. `verify-scorecard` reports which happened as
  `launcher_chain_host_check: performed | skipped-absent`. The chain names paths
  on the machine that captured the set, so checking it by walking the verifying
  host's filesystem made the pin hold on that one Mac and fail on CI;
- `timeout_bin` is present;
- `real_codex_home`, `cache_root`, `real_tmpdir` and `git_common_root` are
  non-null, and EVERY required root (those four, plus `repository_root`, the
  operator home and the four skill roots under it) appears in both
  `denied_read_roots` and `denied_link_roots`, matched literally or by realpath
  since the kernel seals the resolved path. A read deny alone is not a deny: a
  hard link or a clone turns a denied file into a readable one, so the same
  root has to be link-denied and the verifier names any root missing from
  either list;
- the dispatch directory is denied for read-data, link and write; the rep's
  `HOME`/`CODEX_HOME`/`TMPDIR` sit under the writable roots and outside the
  operator home;
- `timeout_seconds` is positive and `wrap` carries the timeout argv, so a
  zero-budget run that omitted the wrapper cannot pass as a bounded one;
- the published `network.log` is present, every line parses with known fields
  and a non-null rep, every attempt is paired with exactly one decision, and
  BOTH the allowed and refused counts recompute to what each rep bound;
- the capture evaluator equals the current one, including the CONNECT proxy: a
  set captured by different bytes is labeled `evaluator-stale` and does not
  count.

**Which fields it only RECORDS.** These are validated for shape and carried into
the profile, but nothing outside the seal corroborates them: `run_root`,
`workspace_root` and `dispatch_root` (real paths at capture time, gone
afterwards), `original_home`, `real_tmpdir`, `real_codex_home` and `cache_root`
as VALUES (they are required to be denied, but that they were the operator's
actual paths is the harness's word), `config_sanitized`, `auth_copied`, and
which specific binary `timeout_bin` names. It also cross-checks a scorecard's
verbatim `seal` copy against the contract when present (null in replay). Any
root or step the seal omits is named on output.

What it does NOT prove: the profile is `(allow default)` outside the operations
it denies, so process capabilities other than the network (signals, IPC, Mach
and XPC messaging, Apple Events, exec of anything already readable) remain open,
and reads outside the denied roots are permitted. The seal is a bound, checkable
statement about the filesystem and the network, not a claim of full isolation.

**The network seal.** The outer profile is `(allow default)` and codex's own
sandbox is bypassed inside it (seatbelt does not nest), so before 2026-09-03
a rep could `curl` the canonical SKILL.md off the forge and the filesystem seal
proved nothing about what it read. A judge demonstrated exactly that. Every
capture now starts a harness-owned CONNECT proxy
(`scripts/lib/probe-connect-proxy.py`, standard library only) on 127.0.0.1 and
an ephemeral port; the profile denies `network*` except outbound to that port;
the rep gets `HTTPS_PROXY`/`HTTP_PROXY`/`ALL_PROXY` pointing at it. The proxy
allows CONNECT only to the bound host allowlist, on port 443 only, and refuses
everything else with 403. It also refuses a destination whose name resolves into
loopback, link-local or private space even when the name is allowed, so a
rebinding answer cannot turn an allowed name into a local service. Every CONNECT
the proxy ACCEPTS is logged to the harness-private dispatch directory: an
`attempt` record before anything is resolved or dialed, and the decision after.
The rep is captured when the connection is accepted, so a connection accepted
between reps carries no rep and is attributed to the run rather than to whichever
rep happened to be current when the decision landed. A refused CONNECT degrades
that rep as `network-egress`; each rep's allowed and refused counts plus the
digest of ITS OWN log lines are bound into its transcript's probe-input event,
the log itself is published with the fixture set as `network.log`, and coverage
requires that file to be present and to reproduce, rep by rep, the digest and
both counts each transcript bound. The log is parsed strictly: an unknown field,
a decision this reader does not know, an attempt with no decision, or a decision
with a null rep refuses the whole set, because a refusal that belongs to no rep
is checked against no transcript. A capture cannot be accepted with a refusal in
it. The file is exempted from the repository `*.log` ignore rule; without that
exemption it was written, never committed, and the digests had nothing to check
against.

The allowlist was pinned from observation: reps run through the proxy in
discovery mode showed codex-cli 0.145 reaching `chatgpt.com`, `ab.chatgpt.com`
and the OpenAI content hosts under `oaiusercontent.com`. `api.openai.com` and
`auth.openai.com` are kept for an API-key producer and were not observed on a
ChatGPT-auth account. `.oaiusercontent.com` is a SUFFIX rule, not a host: an
entry beginning with a dot matches any host under that domain, which those
content hosts need because they carry a rotating region prefix
(`sdmntprsouthcentralus`, `sdmntprcentralus`, `sdmntprwestcentralus` all appeared
in one capture) that no fixed list survives. A lookalike that merely ends with
the same characters, such as `evil-oaiusercontent.com`, does not match. No
unix-socket allowance was needed: the proxy resolves DNS, so the rep never talks
to `mDNSResponder`. The permitted host set, the single port and the empty
unix-socket list are pinned in the verifier, not merely recorded: a capture that
widens any of them records `network.mode: proxy-custom` and can never count.

**v3's seal block gained required keys three times on 2026-09-03.** The first
shape bound only `{mode, denied_read_roots, writable_roots, profile_sha256,
original_home, repository_root}`, which a hand-written record claiming
`seatbelt` on Linux with no mechanism satisfied. The second added the mechanism,
the wrap, the link denies and the rep environment, but still could not
reconstruct its own profile and said nothing about the network. The third could
rebuild its profile and bound the network, but recorded the timeout wrapper
nowhere. Contracts in any of those shapes still load and still replay; none is
ever tier coverage, and they say so with `seal-block-superseded`. The contract
name stays
`agentops-skill-probe-capture.v3` because every v3 set ever committed has been
recaptured under the current seal.

**The rep's producer config is generated, not filtered.** The operator's
`config.toml` carries `[mcp_servers.*]` tables (a rep would otherwise start
those servers and be able to query them, reaching the operator's own vaults and
tools), `[projects.*]` trust entries naming other checkouts, a `notify` hook
naming an operator program, and `web_search` set live. Filtering it left all the
scalars behind, so the file is now written from an allowlist instead: the
reasoning effort and `web_search = "disabled"`, nothing else. Its exact text and
digest are bound in the seal, the same file is copied into every rep, and after
each rep the harness re-parses it: the only permitted growth is codex's own
`[projects."<workspace>"]` trust table, and anything else degrades the rep as
`config-mutated`. `auth.json` is COPIED, not symlinked: the real home is
read-denied, so a symlink into it cannot resolve.

**The rep environment, its descriptors and its process group.** The rep is
launched through a real `env -i` boundary with exactly the variables the seal's
`env_allowlist` names and nothing else. Emptying the environment from inside the
dispatch subshell could not clear bash's readonly exports (`SHELLOPTS`,
`BASHOPTS`, `UID`, `EUID`, `PPID`), which reached the producer undeclared; the
launch is now a plain argv, `env -i <declared assignments> sandbox-exec -p
<profile> <timeout argv> <codex argv>`, and a Darwin test compares the rep's own
`env` output to the recorded allowlist as sets. Every non-stdio descriptor is
closed (the transcript sink was FD 9 and the dispatch handles sat just above
it), and the rep runs in its own process group. After the
rep returns the harness COUNTS the group's live members first, then signals the
group with TERM, escalates to KILL, and refuses to continue until `ps` reports
the group empty. The count comes first because the rep's own codex tree is
already gone when `wait` returns, so anything still in the group outlived the
rep: that count, not the emptiness afterwards, is what degrades the rep as
`rep-survivor`. A group that will not die even after KILL, or a `ps` that cannot
be read, is not a degraded rep at all: it aborts the whole capture, because the
next rep would otherwise start beside something nobody could account for. This
only works because the timeout wrapper runs with `--foreground`; GNU timeout
otherwise calls `setpgid(0,0)` and the reviewer and its children land in
timeout's group rather than the one being reaped, which is how four forked
`sleep` processes once survived a passing test.

A process group is not the whole story: a child that calls `setsid()` leaves the
group entirely and the reap cannot see it. After the group reap the harness runs
`lsof -t +D <run root>` and treats any process still holding a cwd or an open
descriptor under the run directory exactly like a survivor, fatal on the same
terms. What remains open is a child that escapes the session AND holds nothing
under the run root; nothing in this harness detects that.

**One run directory; the workspace is reset, not relocated.** A live capture
creates one `$TMPDIR/probe-run.XXXXXX` (mode 0700) holding `home/` (the scratch
HOME, with `home/.codex` as CODEX_HOME), `ws/` (the rep's cwd), `tmp/` (the
rep's TMPDIR) and `dispatch/` (the harness's own per-rep files: the
materialized prompt, the raw Codex JSONL, stderr). One EXIT trap removes the
whole directory, so a run leaves no probe material behind for a later run to
read. `dispatch/` sits OUTSIDE the checkout because node aborts at startup when
its stdio files sit under a `file-read*` denied tree (it stats them); it is
denied read-data, listing, writes, link and clone, and allowed metadata only.
Before each rep the harness empties `ws/` and `tmp/` and rebuilds `home/.codex`
(codex writes its session rollout there, and a rollout holds the prompt bytes
of the rep that wrote it), then refuses the rep if `ws/` is not empty. The
paths are the same every rep so the profile is constant and the contract binds
one profile digest; the transcript's probe-input event records the `workspace`
and `workspace_reset`, and a sealed capture refuses a transcript that omits
either or that ran outside the seal's writable roots.

**The sibling-read trap.** The prompt reaches the rep on stdin only; nothing a
rep can list from its cwd carries it. This closes the 2026-09-03 leak: with one
shared workspace, an xhigh control rep ran `rg --files`, saw
`treatment-1.prompt`, read it, and held the canonical SKILL.md bytes without
ever naming SKILL.md. Scoring carries a second floor beside the SKILL.md trap:
a successful command whose command string names a `*.prompt` file,
`capture-contract`, `seal.json`, `fixture-set.json`, a rep's raw
`.codex.jsonl`/`.codex.stderr`, the hidden `.capture.`/`.dispatch.` stage, a
`probe-dispatch.`/`probe-ws.`/`probe-seal.` directory, or a run directory's
`dispatch/` or `home/`, or whose captured output lists a sibling rep's file by
name, degrades that rep as `sibling-prompt-read`. A rep naming its OWN `ws/` or
`tmp/` is ordinary work and is not a hit. Replay and `verify-scorecard`
recompute this, so the 2026-09-03 sets reclassify under it without any fixture
edit.

**Effective eligibility is reported per set, for every row.** A scorecard
carries its capture's own `coverage_eligible` claim, and that field is immutable
history:
`docs/evals/scorecards/2026-08-26/premortem-plan-shape-t2-low.json` says `true`
while the gate treats the set as `legacy-unsealed`. `check-skill-probe-coverage`
therefore prints `set <skill>/<probe>: eligible=true|false (<reason>)` for
every ledger row that names a scorecard, including WITHDRAWN, PRELUDE-ONLY and
LEGACY-UNVERIFIED rows, and carries the same rows under `sets` in `--json`. The
gate's value is the effective one; the scorecard field is what the capture
believed. A row whose verdict is not current reads `verdict-<verdict>` rather
than a seal reason: its evidence may verify perfectly and still not be a
measurement.

Replay makes the bound classification replayable; it does not make model
generation deterministic or reproducible.

Verdict: `BEHAVIORAL` iff `treatment_rate > control_rate`; `REGRESSIVE` iff it
is lower; `INERT` iff the rates are equal; `UNMEASURED` iff either arm has no
usable reps.

## The frontier-aces-it caveat

A **frontier** producer often already does the right thing, so a skill's marginal
behavioral effect on it may be nil even for a useful skill. A probe can seek
headroom with a weaker producer or harder task, but a producer-strength claim
requires manifest-backed captures at each compared config. The historical crank
fixtures illustrate a stored null classification only; they are
`LEGACY-UNVERIFIED` and do not establish frontier behavior.

## Two probe tiers

The caveat above proposed two escapes from ceiling saturation: a weaker producer,
or a harder task. Neither worked as run. Five probes dated 2026-08-04/05
saturated at **both** effort levels, and `validate-not-proven-v2` re-saturated
after the scenario was deliberately hardened. The ledger notes conclude: *"the
doctrine is robust in quiz format; next ratchet is task-embedded Tier-2, not
harder quizzes."*

That ratchet is a declared probe tier:

| | Tier 1 — quiz | Tier 2 — seeded task |
|---|---|---|
| Scenario | asks about a situation | hands over work containing a planted defect |
| Grades | which answer was given | whether the agent **acted** on the defect |
| Saturates | fast at frontier | slowly — skimming fails at every altitude |
| `probe.json` | (as before) | adds `probe_tier: 2`, `seeded_defects: N`, `band: [lo, hi]` |

A tier-2 probe plants exactly one forcing defect (floor probe) or N independent
defects (band probe) inside a realistic artifact, and asserts findings land in
`[N-1, N+2]`. The lower bound catches rubber-stamping; the upper bound catches
spray, where an agent lists every conceivable concern and gets credited for the
one that happened to be planted.

Authoring rules, the calibration window, and the seed shapes:
[`skills/skill-eval/SKILL.md`](../../skills/skill-eval/SKILL.md) and
[`skills/skill-eval/references/seeding.md`](../../skills/skill-eval/references/seeding.md).

## The headroom pre-screen (run this before you believe a verdict)

`INERT` is two different results wearing one label. It means the arms matched —
which happens both when the skill changed nothing (a real null, worth a row) and
when the control arm already aced the scenario so nothing could have differed (a
void row: the measurement failed, not the skill). Only the control arm's
absolute rate separates them.

The rule is deterministic, so it is a gate rather than doctrine.
**`skill.probe-headroom`** — [`scripts/check-skill-probe-headroom.sh`](../../scripts/check-skill-probe-headroom.sh),
rule in `cli/internal/probeheadroom`, helper `cli/cmd/probe-headroom`:

```bash
# Classify one probe group (all scorecards for one probe, one per effort level).
cd cli && go build -o bin/probe-headroom ./cmd/probe-headroom && cd ..
cli/bin/probe-headroom docs/evals/scorecards/<date>/<probe>-*.json

# Gate: prove the detector still discriminates, then sweep every group.
bash scripts/check-skill-probe-headroom.sh
```

A scenario is **SATURATED** when the control arm scores ≥ 0.75 at two or more
effort levels with ≥ 2 usable control reps each. Exit `0` SEPARATED · `3`
SATURATED · `4` FLOOR · `5` UNMEASURED. A saturated scenario is **retired, not
re-run** — re-running it produces ledger rows, not knowledge; promote it to a
tier-2 seeded-defect probe or record the honest ceiling finding.

**First reading on the historical corpus: 7 of 11 probe groups are SATURATED.**
That is why those INERT rows could never become coverage, and it is the reason a
new ledger row must cite a passing pre-screen before it counts as evidence.

The gate is advisory (`Blocking:false`): a saturated historical group is a true
finding about the ledger, not a regression introduced by the change under test.

## Spine first, ratchet does the rest

The advisory gate `skill.probe-coverage`
(`scripts/check-skill-probe-coverage.sh`) names every product-/judgment-tier
skill lacking a current, canonical-skill-mode, manifest-backed result. After
the 2026-08-16 provenance migration, the historical rows are excluded and
current coverage is 1/12 (premortem, sealed, 2026-09-03) against a **declared denominator**: the 12 skills
that carry a product/judgment badge. `scripts/.skill-probe-denominator-exclusions`
stays as the mechanism for a category error, with its argument written beside
each entry; it held `goals` (a pure `alias-of fitness`, whose probe would have
measured `fitness` under the wrong name) until ADR-0018 retired that skill,
and it carries no entry today. The gate stays
advisory-first until a deliberately selected spine is recaptured under the
current contract.

## Evidence lands dated

Every counted run has an immutable fixture set, an
`agentops-skill-probe.v3` scorecard under a dated `docs/evals/scorecards/`
directory, a passing `skill.probe-headroom` pre-screen over that scorecard
group, a short dated evidence note when interpretation is needed, and a row
in the **Behavioral Probe Ledger (MEASUREMENT STATUS)** at
`evals/skill-probes/LEDGER.md`. The ledger is hand-maintained and never belongs
inside generated `skills/SKILL-TIERS.md`, where regeneration once wiped it.
