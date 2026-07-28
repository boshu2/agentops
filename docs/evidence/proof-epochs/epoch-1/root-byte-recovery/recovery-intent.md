---
record_type: root-byte-recovery-intent
proof_epoch: 1
status: frozen
packet_revision: 2
scope: single-file-source-restoration
author_context_id: claude-session-01PEdDmbiBJu4nrV1HywZTQc
base_commit: 0ba51a5f6cda954b976875435ca11ea10fd7d4f8
base_tree: 9919c7e7a2707477f237718efdc4dec6f4a6630b
branch: codex/root-byte-recovery-20260728
subject_ref: skills/rpi/scripts/run_once.py
binding_role: rpi-dispatcher
expected_sha256: 57fb4e491216adc75e15fabc3b117d498b1ff4601edb5f2773e619ce82f253be
observed_sha256: 52e4be4c51cd047d225f2bc83fdbfd18133c22c1e008d5076c32c4b608fd279b
expected_mode: "0755"
observed_mode: "0755"
target_blob: 3334b269b8f19419a7d037e382214682f2dfde44
current_blob: 4ae64e9a5a7df42fdf2f6a89f861ad9500643c92
source_commits:
  - 090ac7713c2d51a65022b6fad46f052902db9625
  - 5b6bf22dad6a855fa385d946fecc0eb58605228d
delta_numstat: "3 0"
observation_root_id: skills-source
observation_includes: ["skills"]
observation_cache_policy: cache-free-base
active_pointer_sha256: 25bc0adcf9ab9d64a088a0580b8693a3721f8a363d758d3bcb748f511892a1f3
active_descriptor_sha256: f6358e3858d4e6f67844966334547d6df88b58c5a2e9f7f5889ac2d1fadd2340
transition_recorder_sha256: 49f22c1af70f8a0de09c44bd132e74d22f053d0b9a5f353f98bdc8082c0c5e58
kernel_sha256: f7787f4505c6f49c77890411a49387a02beec7a267595e158af6e4184ca6ef70
cross_score_ref: /tmp/agentops-sol-fable-plan-crossscore-20260727.md
cross_score_sha256: 9853e2e33cb9dff0f5499d4fdededc66f30d92f1ed04e114e899520707d97c33
---

# Root-byte recovery intent — restore the bound `rpi-dispatcher` bytes

## Claim

This record claims **intent only**. It is not a PASS, not a verdict, and not an
authorization to land. No `verdict.v3` is authored here, and no validation field
is invented or backfilled.

## Packet correction — why this is revision 2

A first packet was built and discarded before reporting. Its before-manifest was
frozen while Python bytecode caches from the author's own pre-change
verification were present on disk, so the derived effect receipt reported an
unrelated `DELETED` entry for
`skills/validate/scripts/__pycache__/kernel_v3.cpython-314.pyc` and a nonempty
`undeclared_paths`. Before 307 entries, final 306. That is the cache
fixed-point defect: the observation policy made the packet's own tooling part of
the observed subject, and it would have put a cache deletion outside the
one-file recovery scope.

It is corrected here rather than disclosed, because disclosure would have left a
false effect receipt in the packet. `__pycache__` cannot be excluded through the
canonical mechanism — `build_manifest_v2` rejects any exclusion outside the
closed `RUNTIME_EXCLUSIONS` frozenset (`.git`, `.agents/ao/intents`,
`.agents/ao/verdicts`, `.agents/ao/reports`) — so the exclusion route does not
exist and the packet was **restarted from a cache-free exact base** instead.

The correction rule, binding for every artifact in this packet:

- the tree is returned to base `0ba51a5f6` with **zero** `__pycache__`/`*.pyc`
  under any observation root before the before-manifest is frozen;
- every mint, freeze, manifest, check and derivation runs with
  `PYTHONDONTWRITEBYTECODE=1`, so no cache is created at any point;
- both manifests are re-derived under that policy and every dependent digest and
  receipt is recomputed, never edited;
- records live under `docs/evidence/**`, outside the `skills` observation root,
  so freezing records cannot perturb the observed subject.

Acceptance **A8** below makes the resulting property checkable rather than
asserted.

## The defect

At base `0ba51a5f6`, the deterministic walk over the active epoch-1 descriptor's
25 components plus its separately checked transition recorder reports:

```json
{"binding_count": 26, "match_count": 25, "mismatch_count": 1,
 "mismatches": [{"role": "rpi-dispatcher",
   "ref": "skills/rpi/scripts/run_once.py",
   "expected_digest": "57fb4e491216adc75e15fabc3b117d498b1ff4601edb5f2773e619ce82f253be",
   "observed_digest": "52e4be4c51cd047d225f2bc83fdbfd18133c22c1e008d5076c32c4b608fd279b",
   "expected_mode": "0755", "observed_mode": "0755"}]}
```

`load_active_proof` consequently raises
`TerminalValidation: active proof component bytes or mode changed:
skills/rpi/scripts/run_once.py`, so the active root cannot judge anything.

Mode is **not** the defect: expected and observed are both `0755`. Only the
bytes diverge.

## The exact delta

`git diff 090ac7713 HEAD -- skills/rpi/scripts/run_once.py` is exactly three
added `#` comment lines at current lines 124-126; `--numstat` is `3 0`. The
added lines are commentary only and change no executable statement:

```text
+    # The identity is byte-addressed. Plan minted the exact snapshot, RPI
+    # independently consumed and rehashed it above, and Validate must bind its
+    # verdict to the same digest. RPI never invents a canonical-value digest.
```

Both `090ac7713` and `5b6bf22dad6a855fa385d946fecc0eb58605228d` resolve the path
to Git blob `3334b269b8f19419a7d037e382214682f2dfde44`, whose exact bytes hash to
the bound `57fb4e49…`. HEAD's blob is `4ae64e9a5a7df42fdf2f6a89f861ad9500643c92`.

## What this recovery does

Restore `skills/rpi/scripts/run_once.py` to blob
`3334b269b8f19419a7d037e382214682f2dfde44` byte-for-byte, preserving executable
mode `0755`. This is a **subject-side conformance repair**: the active descriptor
already authorizes the target bytes, so restoring them is neither a descriptor
refreeze nor a proof activation, and there is no fixed point.

## Source write scope

Exactly one path:

- `skills/rpi/scripts/run_once.py`

Records and evidence are written under
`docs/evidence/proof-epochs/epoch-1/root-byte-recovery/**`, which is outside the
`skills` observation root and is therefore not observed subject content.

## Non-goals

- No descriptor change. `subject-refreeze-candidate-descriptor.json` stays byte-identical.
- No pointer change. `docs/contracts/proof-contracts/active.json` stays byte-identical.
- No epoch transition, no activation, no new proof contract.
- No `verdict.v3` authored, and no validation field invented or backfilled.
- No CI binding gate added here.
- No merge, no push, no integration.
- No regeneration of the `skills-codex/**` projection; projection drift is a
  separately classified landing obligation and is out of this write scope.
- No other file touched, including the transition recorder and the kernel.

## Acceptance

- **A1** — the deterministic walk reports `binding_count: 26`,
  `match_count: 26`, `mismatch_count: 0`: full binding **and mode** parity.
- **A2** — `load_active_proof` returns the full epoch-1 identity: epoch `1`,
  contract digest `f6358e3858d4e6f67844966334547d6df88b58c5a2e9f7f5889ac2d1fadd2340`,
  activation transition digest
  `2f9e95bcb7841030a96b97e5bf1b9d4936ca3247c6f340e70e77abf11f045e8a`.
- **A3** — the restored file's blob is exactly
  `3334b269b8f19419a7d037e382214682f2dfde44`, its sha256 is `57fb4e49…`, and its
  mode is `0755`.
- **A4** — the source commit changes exactly one path, and its diff against
  `090ac7713` for that path is empty.
- **A5** — the active pointer and the epoch-1 descriptor are byte-unchanged from
  base, proven by digest equality.
- **A6** — RPI focused tests pass over the restored subject.
- **A7** — the worktree is clean at closeout.
- **A8** — the derived effect receipt's `changes` is exactly one `MODIFIED`
  entry for `skills/rpi/scripts/run_once.py` carrying
  `before_digest 52e4be4c…` and `after_digest 57fb4e49…`, and
  `undeclared_paths` is empty. Before and final manifests carry the same entry
  count and differ semantically only at that path. No `__pycache__` or `*.pyc`
  entry appears in either manifest.

## Later obligation, deliberately not discharged here

A fresh, author-distinct context must mint the full epoch-1 `verdict.v3` packet
over this exact recovery subject **before** it lands. The v3 reader enforces
distinct author and validator IDs. This record supplies the author-side packet
inputs — minted intent identity, scope index, before/final manifests, check
receipts, and a derived effect receipt — so that validator can judge exact
content without any field being backfilled. The author context ID for that
distinctness check is `claude-session-01PEdDmbiBJu4nrV1HywZTQc`.
