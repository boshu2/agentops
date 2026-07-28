# Fresh Sol advisory review — adapters-13 audit v14

## Disposition

**REQUEST_CHANGES — one report-integrity correction only.**

The v14 footer correctly repairs v13's RC-3: its provenance chain is now literal,
complete, and strictly newest-first:

`v14 → v13 → v12 → v11 → v10 → v9 → v8 → v7 → v6 → v5`.

Every substantive technical criterion I checked reproduces. The sole required
change is a new self-description defect introduced by v14:

### RC-4 — v14 changed more than “one paragraph”

At lines 63–66, v14 says it “rewrote one paragraph.” At line 1175 it likewise
says “it rewrites this paragraph.” The exact v13→v14 diff does not support that
scope statement:

- 10 zero-context diff hunks;
- 42 added lines and 16 deleted lines;
- changes to the title;
- a new current-version §0.0 and a new v13 history §0.1;
- renumbering and reframing of the v12 history section;
- expansion of the direct-input table and its provenance paragraph;
- replacement of the footer; and
- replacement of the closing disclaimer.

This does **not** invalidate the report's narrower and important assertion that
the technical content is unchanged. Sections 2–11 are byte-identical between
v13 and v14: both extract to 658 lines / 78,395 bytes with SHA-256
`5b4433279e7a21ea1a6ed389b701e322f3fd4d7e1eb0f6aea2f8aeb4e24d47ac`.
The defect is only that v14's account of its own editorial scope is too narrow.

Required correction: replace the two “one/this paragraph” statements with an
exact account, for example: “v14 rewrote the footer and advanced the
current-version metadata and history framing; §§2–11 and all technical
judgments remain byte-preserved from v13.” No technical finding, severity,
count, inventory row, owner classification, evidence item, or limitation needs
to change.

Subject to that wording correction, this review has no other requested change.

## Immutable subjects and review basis

I opened and read the v14 subject in full:

- path:
  `/tmp/agentops-opus5-verified-skill-audit-adapters-13-v14.md`
- SHA-256:
  `79bbd141a27429e5d12434a8eacbd32a53476260f7273bb395a744c1265a0885`
- extent: 1,183 lines / 142,998 bytes
- mode: `0444`

I also read the two direct lineage inputs in full and recomputed their
identities:

- v13 audit:
  `1d1709a3b5be5557e51776e94a0c11612028cf406cc16b74d8116ce8b335d3f5`,
  1,146 lines / 138,200 bytes, mode `0400`;
- v13 Sol review:
  `86e8b566998972f9e9c0277036ab9463e119d341cc9ba469b8216727f2290571`,
  171 lines / 13,619 bytes, mode `0444`.

The pinned landing remained clean throughout:

- worktree: `/Users/bo/dev/agentops-worktrees/skill-overhaul`
- HEAD: `0088c6e3824da201eabb1e751ac8e976599e0b5c`
- tree: `c0c43eefb8042af5a6a7877c0f7f0de80149ffc6`
- status: 0 porcelain rows

This is advisory review evidence. It is not a `verdict.v2`, runtime verdict,
implementation authorization, merge decision, or release decision.

## RC-3 and generic integrity sweep

RC-3 is repaired exactly:

1. The footer opens with the current relation, v14 superseding v13 under the
   v13 Sol review.
2. It then proceeds v13→v12 and v12→v11.
3. It explicitly records that v12 was author-lane initiated because no Sol
   review of v11 exists.
4. It continues in order through v11→v10, v10→v9, v9→v8, v8→v7, v7→v6,
   and v6→v5.
5. It closes with the same explicit complete sequence.

The v13 and v13-review digests, extents, and modes in the footer and input table
match the files on disk. RC-1 and RC-2 remain repaired. The title, current
section, history labels, input rows, footer identity, and disclaimer consistently
name v14 except for the narrow editorial-scope defect above.

The Markdown structure scan found 28 table separators and zero
header/separator/first-row adjacency failures. The 13 per-skill headings are
present exactly once. No duplicate finding ID, orphan ledger row, stale current
output path, false PASS claim, or minted runtime verdict was found.

## Ledger, inventory, and projections

I independently parsed the finding tables:

| Severity | Rows | Unique IDs |
|---|---:|---:|
| P0 | 5 | 5 |
| P1 | 34 | 34 |
| P2 | 39 | 39 |
| **Total** | **78** | **78** |

The 20 absorbed aliases are enumerated and uncounted. The six refuted claim
rows are separate from the counted ledger. `S3` names exactly four skills.
`AN-4`, `AN-5`, and `RCH-5` are present in P2.

I rebuilt the owned-file inventory from the pinned Git tree:

| Skill | Paths | Lines | Bytes |
|---|---:|---:|---:|
| account-rotation | 1 | 50 | 1,976 |
| agent-mail | 4 | 829 | 18,167 |
| agent-native | 3 | 373 | 14,576 |
| agy-native | 1 | 50 | 2,042 |
| cass | 24 | 4,500 | 152,821 |
| cc-hooks | 15 | 3,057 | 95,805 |
| codex-exec | 1 | 80 | 2,815 |
| converter | 4 | 989 | 35,127 |
| dcg | 9 | 1,707 | 47,443 |
| ms | 4 | 506 | 22,961 |
| ntm | 1 | 106 | 4,050 |
| rch | 8 | 1,516 | 59,116 |
| swarm | 3 | 252 | 9,181 |
| **Total** | **78** | **14,015** | **466,080** |

All 13 Codex override catalog entries are `parity_only`. Every per-skill
`.agentops-generated.json` names `codex-sync`, and its source/generated hashes
match the global manifest. The scoped generator check passed, and the parity
audit returned `[]` for all 13 skills.

## Thirteen one-by-one technical results

1. **account-rotation** — Supported as T7b support,
   `cross_cutting`, standalone. Target-runtime identity is the correct boundary
   for stale credentials; the report's authority/effects and thin-proof
   findings match the skill and plan.
2. **agent-mail** — Supported as T7a runtime,
   `runtime_transport`. The reachable reset, hook-admin, force-release, and
   retired-CLI surfaces support `AM-1` and the narrower protocol/effect findings.
3. **agent-native** — Supported as T7a runtime,
   `runtime_transport`. A contained `validate_cross` witness accepted equal
   context IDs, degraded requested AGY validation to Codex, omitted intent and
   subject-manifest digests, emitted a freshness attestation, and exited 0.
4. **agy-native** — Supported as T7a runtime,
   `runtime_transport`. Discover-before-act and the live adapter behavior match
   the report; the shared timeout fallback remains a genuine untyped/unbounded
   risk.
5. **cass** — Supported as T4 evidence with `plan_input` and
   `post_verdict` seams. The stale-spec, mutation/effects, version-drift, and
   post-verdict-gap claims are source-backed. The 123 CASS executions are
   correctly attributed to v6, not v14 or this review.
6. **cc-hooks** — Supported as T7b support, `cross_cutting`.
   The default-on/documented-default contradiction and legitimate `_beads`
   enforcement path are present as reported; installation/uninstall and effect
   boundaries remain under-specified.
7. **codex-exec** — Supported as T7a runtime,
   `runtime_transport`. The adapter is one-shot; higher-level retry belongs to
   the eval owner. With both `timeout` and `gtimeout` hidden, the timeout helper
   returned an empty wrapper and silently permits unbounded execution.
8. **converter** — Supported as T5 implementation,
   `implement_method`, not as the shipped Codex projection owner. The line
   deletion and source=output deletion witnesses reproduce. On `/bin/bash`
   3.2.57, `local -n` fails with rc 2 before output creation, supporting the
   corrected portability/availability scope for `CV-8`.
9. **dcg** — Supported as T7b support, `cross_cutting`.
   Installed 0.5.6 reproduced the exact matrix: `/tmp`, `/private/tmp`, and
   literal `$TMPDIR` forms allowed; `$LAB`, `$output_dir`, and `./build`
   blocked. The upstream-identity and version-drift qualifications are valid.
10. **ms** — Supported as T7b support, `cross_cutting`,
    standalone. The effect declaration conflicts with the skill and
    `ms-reindex.sh`; substring process discovery/TERM/KILL and raw query
    interpolation are present.
11. **ntm** — Supported as T7a runtime,
    `runtime_transport`. Capability discovery works read-only; the report
    correctly limits the skill to transport/operational evidence rather than
    lifecycle or verdict authority.
12. **rch** — Supported as T7a runtime with
    `implement_method` and `cross_cutting` seams. Six sibling references are
    dangling; unconditional remote/privileged instructions support `RCH-A`.
    At rch 1.0.26, `rch check` returned rc 2 with the daemon down while
    `rch doctor --json` returned rc 0, `success:true`, `failed:0`, and a daemon
    warning, reproducing `N11`.
13. **swarm** — Supported as T7a runtime,
    `runtime_transport`. The six existing tests pass but do not cover host path
    identity. Contained case-alias and symlink-alias witnesses proved the paths
    were host-equal while `_overlap` returned false and both packets dispatched;
    `write_scope.exclude` is also ignored by `_includes()`.

The canonical placement matrix agrees with the overhaul plan and
`orchestration-ports.md`. The six live Go/shell owner line counts reproduce:
520, 85, 815, 198, 242, and 301. The selected AGY count is 12 raw paths and
11 semantic paths, with the transcript occurrence correctly excluded.

## Deterministic checks reproduced

- `reviewer-adapters.bats`: 16/16 passed.
- `codex-exec-lib.bats`: 9/9 passed.
- swarm unit tests: 6/6 passed.
- ADR-0016 ratchet: PASS, 24 grandfathered paths.
- cc-hooks lint: PASS, 4 policies.
- policy-dispatch bats: 28/28 passed.
- installed-skill-edit-guard bats: 10/10 passed.
- MS validator: PASS.
- MS MCP bats: 6/6 passed.
- converter validator: 2 passed / 0 failed.
- DCG validator: PASS at installed 0.5.6.
- scoped `codex-sync --check`: PASS.
- scoped Codex parity audit: `[]` for all 13.

Read-only live surfaces also reproduced the report's relevant observations:
`ntm --robot-capabilities` returned JSON; `ms config` returned successfully;
the DCG policy matrix matched; and the RCH daemon-down status inversion matched.

## Checked

- v14, v13, and the v13 Sol review read in full and identity-checked.
- Exact v13→v14 diff, current/history labels, footer order, input identities,
  output path, disclaimer, and self-attribution language.
- All 13 canonical `SKILL.md` files read in full.
- Finding-bearing source ranges and live owners for all 13 skills.
- Overhaul target matrix, T7a/T7b acceptance, ADR-0016 plus amendment, and
  orchestration-port contract.
- Ledger arithmetic/uniqueness, absorbed aliases, refuted rows, per-skill
  headings, Markdown table structure, and inventory arithmetic.
- Projection catalog, per-skill manifests, generator output, and parity.
- The deterministic suites and contained witnesses listed above.
- Subject and landing identities rechecked after the evidence runs.

## Not checked and residual risk

- No destructive, privileged, remote-write, credential-switch, real Agent Mail
  mutation, real MS feedback/outcome, real CASS mutation, or real multi-host
  dispatch was performed.
- I did not execute all commands described across the 78 owned files, nor repeat
  the inherited 123-command CASS population. I checked its source attribution
  and retained arithmetic against the v6 artifact.
- Upstream behavior beyond the installed versions and inspected primary/local
  command surfaces was not generalized.
- The report describes serious source risks that remain present at the pinned
  tree. This review validates the audit evidence; it does not repair those
  source defects.

The only review blocker is RC-4's inaccurate editorial-scope sentence. RC-3 is
fixed, and the full technical audit is otherwise supported.
