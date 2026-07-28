# Sol Fresh Independent Validation — Workflow/Support 12-Skill Audit v8

## Binding disposition

**PASS**

The exact v8 audit is internally consistent with the bound repository and
corrects all three blockers from the v7 Sol review:

1. Witness E now classifies all 27 rows as exactly **11 present / 14 absent /
   1 failed / 1 not applicable**.
2. Row 26 now states and distinguishes all three destination/parent outcomes.
3. Handoff's `CreateTemp` plus deferred conditional `Remove` lifecycle is now
   present in the effects vocabulary-gap account, under the existing H6
   empty-legacy-rail finding rather than a new finding.

All preserved claims requested for this review also remain supported: H1–H6,
the exact 70-path inventory and 70 whole-file hashes, the six C1 locations,
C2/C3 membership, the two effects rails, the seven split repair bundles, all
twelve intent/RPI/live-behavior judgments, and the arithmetic
**13 P0 / 25 P1 / 34 P2 = 72**.

This PASS judges the audit document, not the health of the twelve skills. The
audit's 72 repository findings remain findings, including 13 P0s. This is the
requested sealed Markdown review, not a canonical `verdict.v2`, and it grants
no implementation, Git, publication, release, or delivery authority.

## Bound identities

### Review context

- Validator: fresh independent Sol context
  `/root/sol_review_workflow12_audit_v8`
- Subject author and validator are distinct contexts.
- Freshness source: caller dispatch plus direct repository and artifact reads.
- No repository or subject mutation was authorized or performed.

### Exact audit inputs

| Artifact | Expected | Recomputed | Size | Result |
|---|---|---|---:|---|
| `/tmp/agentops-opus5-verified-skill-audit-workflow-12-v8.md` | `b9d6b3d913990509319d86260ce7dd918490d4ecf94ad9b38012208414da357a` | `b9d6b3d913990509319d86260ce7dd918490d4ecf94ad9b38012208414da357a` | 795 lines / 97,069 bytes | **PASS** |
| `/tmp/agentops-opus5-verified-skill-audit-workflow-12-v7-review-sol.md` | `9e6771e7aecd27d63a3941722d4368a86583dc526dd509194b532ba4fb4a07c9` | `9e6771e7aecd27d63a3941722d4368a86583dc526dd509194b532ba4fb4a07c9` | 438 lines / 21,232 bytes | **PASS** |

Both files were read in full and remained byte-identical through close.

### Exact repository subject

| Fact | Expected | Recomputed at open and close |
|---|---|---|
| Repository | `/Users/bo/dev/agentops-worktrees/skill-overhaul` | match |
| Branch | `codex/skill-overhaul-20260724` | match |
| HEAD | `0088c6e3824da201eabb1e751ac8e976599e0b5c` | match |
| Tree | `c0c43eefb8042af5a6a7877c0f7f0de80149ffc6` | match |
| Porcelain status | clean | zero lines |

No repository file, subject byte, generated projection, commit, branch, tag, or
external state was changed.

## Criterion matrix

| Criterion | Result | Independent evidence |
|---|---|---|
| Stable subject, predecessor, HEAD, tree, branch, and status | **PASS** | SHA-256, line/byte counts, Git identities, and porcelain state recomputed at open and close |
| V7 W1: Witness E across all 27 rows | **PASS** | Fresh isolated replay plus source-order walk; exact `11 + 14 + 1 + 1 = 27` |
| V7 W2: row 26 three-way condition | **PASS** | Source plus fresh probes of all three branches |
| V7 W3: handoff cleanup effects gap | **PASS** | `handoff.go:125-152`; v8 §§7.2, 7.3, and 9.3 all carry the lifecycle |
| Prior v6 H1–H6 corrections | **PASS** | All six rechecked; three risk-critical seams replayed |
| Exact 27 producer-row structure | **PASS** | Parsed row sequence is exactly `1,2,…,27`, with no gap or duplicate |
| Exact 70 tracked owned paths and hashes | **PASS** | Current tree vs v2 appendix: 70 rows, zero missing, zero extra, zero digest mismatch |
| Twelve intent and RPI-fit judgments | **PASS** | All twelve canonical `SKILL.md` files read in full against the operating-loop contract and live owners |
| C1/C2/C3 taxonomy | **PASS** | ADR-0016 plus all six selected-skill locations and authority-bearing sources |
| Effects rails and gap inventory | **PASS** | Frontmatter v2, contract v3, selected frontmatter, live writers, and cleanup source |
| Seven repair-unit splits | **PASS** | Each split maps to independently removable text, schema, or proof obligations |
| `13/25/34=72` | **PASS** | Parsed all twelve ledger rows and independently summed each severity column |
| Full provenance | **PASS** | Every available origin/v2–v8 audit and Sol-review artifact rehashed |
| Checked/not-checked disclosure | **PASS** | v8 accurately distinguishes current source derivation, prior witnesses, and unexecuted residual branches |

No binding finding remains against v8.

## Witness E — fresh 27-row validation

Fresh probe:

```text
/tmp/sol-v8-witnessE.YGnGJa
```

Invocation was isolated from the judged worktree, used repository mode with no
upstream repository, no security audit, and a cwd without
`.agents/learnings/`. `PYTHONDONTWRITEBYTECODE=1` prevented repository cache
creation.

Observed:

- feature-registry validation completed before the crash;
- process exit was `1`;
- traceback ended at `reverse_engineer.py:1855`;
- the exception was `FileNotFoundError` for
  `.agents/learnings/2026-07-28-witness-e-reverse-engineer.md`;
- both row-25 council reports existed;
- no learning file or learning parent existed;
- no `spec-cli-surface.md` existed;
- `spec-code-map.md` contained the row-6 CLI-omitted append;
- no binary-enrichment marker existed in `feature-registry.yaml`.

The complete state is therefore:

| State | Rows | Count |
|---|---|---:|
| **Present** | 1–8, 11, 13, 25 | **11** |
| **Absent** | 9, 10, 12, 14–24 | **14** |
| **Failed** | 26 | **1** |
| **Not applicable** | 27, model-produced Phase 2 | **1** |

The output tree directly contained the row-1/2/3/4/5/7/8/11/13 artifacts.
Row 6 was established by the append marker, and row 25 by the two files under
`.agents/council/`. The absent rows follow their ordinary mode/flag/CLI/package
guards, not the late crash.

Arithmetic is exact:

```text
11 present + 14 absent + 1 failed + 1 not applicable = 27
```

The source order independently agrees. Rows 1–24 precede the security block
ending at `:1842`; row 25 renders at `:1850-1851`; row 26 attempts its write at
`:1855-1871`; `return 0` is at `:1873`. V7's “not reached” label is absent
except where v8 quotes and explicitly withdraws it.

### Other witness tables

The v8 A–D tables each cover every row:

| Witness | Present | Absent | N/A | Total |
|---|---:|---:|---:|---:|
| A — repo + clone + security/no SBOM | 15 | 11 | 1 | 27 |
| B — binary `/usr/bin/true` | 14 | 12 | 1 | 27 |
| C — binary Docker | 15 | 11 | 1 | 27 |
| D — both + clone + `/usr/bin/true` | 18 | 8 | 1 | 27 |

Their row membership agrees with the live mode and flag guards and with the
digest-bound prior Sol observations. In particular, D keeps row 20 and drops
row 6, while B/C differ only by row 19's in-place content rewrite.

## Row 26 — all three branches

Live source:

```text
:1845  council_dir = REPO_ROOT / ".agents" / "council"
:1846  _ensure_dirs([council_dir])
:1853  learning_path = REPO_ROOT / ".agents" / "learnings" / ...
:1854  if not learning_path.exists():
:1855      learning_path.write_text(...)
:1873  return 0
```

`learnings` occurs exactly once in the entire script, at `:1853`.
`.agents/learnings/` is absent from the bound tree.

Fresh branch probe:

```text
/tmp/sol-v8-row26-success.SuEaUG
```

| Initial state | Observed result |
|---|---|
| destination absent; parent absent | Witness E: `FileNotFoundError`, rc=1, no learning file |
| destination absent; parent present | rc=0; exactly one learning file written |
| destination already present | rc=0; still one file; SHA-256 unchanged before/after |

The existing-file digest remained
`373e476c8fed297e837e9c3a17acf2536dcfe1eada31e3c5e0531f8a07b9b8d3`
across the third branch. V8's internal outcome table is exact.

## Handoff cleanup and effects

The complete live writer sequence is:

```text
handoff.go:131  os.CreateTemp(dir, ".handoff-*.tmp")
handoff.go:135  tmpName := tmp.Name()
handoff.go:136  defer func() { _ = os.Remove(tmpName) }()
handoff.go:137  tmp.Write(data)
handoff.go:141  tmp.Sync()
handoff.go:145  tmp.Close()
handoff.go:148  os.Rename(tmpName, target)
handoff.go:151  return target, nil
```

On success, rename has already removed the temporary pathname and the deferred
remove is a discarded no-op. On a pre-rename or rename failure, the deferred
call conditionally deletes the temp file. This is both a temporary-file
lifecycle and a conditional deletion.

V8 records `handoff` in both relevant §7.3 rows, carries the lifecycle into
§7.2 and §9.3, and explicitly states that contract v3 has no
`filesystem.delete` kind. Assigning the omission to existing **H6 [P2]** is
consistent with the audit's repair-unit rule: H6 already owns handoff's empty
legacy effect rail, and this added behavioral detail creates no independently
repairable metadata defect. The ledger correctly remains unchanged.

A fresh HEAD-built dry run also reproduced the existing handoff ledger:

```text
schema_errors=3
missing required property: consumed
missing required property: type
fractional-second id violates ^handoff-[0-9]{8}T[0-9]{6}Z$
```

## Prior v6 correction ledger

| Item | Result | Independent check |
|---|---|---|
| H1 — class-A producers | **PASS** | Rows 25 and 26 map exactly to `:1845-1871`, outside `--output-dir` |
| H2 — complete A–D reconciliation | **PASS** | Each table partitions all 27 rows; counts above |
| H3 — row-18 no-archive branch | **PASS** | Fresh `/usr/bin/true` probe produced only `extract.NOOP.md`; `PRIMARY.txt` absent |
| H4 — valid row-20 negative seam | **PASS** | Fresh empty `<tmp>/binary/` direct call returned `False`; no `spec-cli-surface.md` |
| H5 — security scratch class | **PASS** | Source shows `mktemp` + EXIT trap; fresh isolated scan left zero temp entries |
| H6 — comparison write citation | **PASS** | path bind at `:1400`, `write_text` at `:1401` |

Probe roots:

```text
row 18  /tmp/sol-v8-archive.psclfc
row 20  /tmp/sol-v8-row20.C82jKe
H5      /tmp/sol-v8-security-scratch.ub1cJZ
```

The direct seam and helper probes were confined to `/tmp`; no reverse-engineer
write was directed at the repository.

## Exact 70-path manifest

The twelve owned path sets were derived from `git ls-tree -r --name-only HEAD`,
sorted, and compared with all 70 path/digest rows in the v2 appendix.

```text
bootstrap=2
codebase-recon=3
handoff=2
refactor=3
reverse-engineer=40
sbh=1
scaffold=5
shared=1
status=2
test=9
using-gc=1
workflow-builder=1
total=70
```

The sorted path-list SHA-256 is:

```text
e78d803549b6c6c16e21ae5117a54958286e4131881d85a94c5e66c99d14ba19
```

The appendix comparison is:

```text
appendix rows=70
missing=0
extra=0
whole-file SHA-256 mismatch=0
```

Thus the inherited v2 whole-file appendix remains an exact content manifest for
the bound tree, not merely a prior prose claim.

## Authority, effects, splits, and arithmetic

### Authority classes

ADR-0016's accepted closed top-level `.agents/` set is exactly:

```text
ao/
scratch/
projections/
```

The selected twelve have exactly the six C1 locations v8 names:

```text
codebase-recon   .agents/recon
handoff          .agents/handoff
reverse-engineer .agents/research
test             .agents/tests
reverse-engineer .agents/council
reverse-engineer .agents/learnings
```

The named `fm-ws-noncanonical-topdir` detector has zero executable-source hits,
while `cli/internal/initapp/initapp.go:28` itself creates `.agents/handoff`.
V8 correctly treats the six as accepted-contract/source faults, not falsely as
currently firing detector failures.

C2 is limited to reverse-engineer's mechanical council output and canned
learning output. C3 is limited to refactor's commit/revert grants and
scaffold's initial-commit/continuation grants. Handoff's stale schema lifecycle
fields remain data-shape drift, not exercised authority.

### Effects rails

- `skill-frontmatter.v2` defines legacy `metadata.effects` as a unique array of
  arbitrary nonempty strings; it has no enum.
- `skill-contract.v3` defines a separate structured rail with exactly 12 effect
  kinds.
- `filesystem.delete` occurs zero times in the v3 schema.
- Exactly one skill declares `contract_v3`: `skill-builder`, outside this set.
- Eleven selected skills declare `effects: []`.
- `using-gc` declares `[operate_gas_city]`, legal on the legacy rail.

The per-skill real-effect inventory agrees with canonical skill behavior and
live owners. Handoff's environment reads, clock read, Git process, atomic
write, and conditional cleanup are all now included. Reverse-engineer's
network, caller-target execution, scratch, extraction, in-place rewrite, and
out-of-root writes are distinguished. `shared` is correctly effect-free.

### Seven split repair bundles

All seven splits remain independent:

```text
H1  -> H1a missing type / H1b missing consumed / H1c id pattern
RF1 -> RF1a commit / RF1b automatic revert / RF1c header metadata
SC1 -> SC1a initial commit / SC1b next-steps continuation
SC5 -> SC5a exemplar proof / SC5b one-way-sync proof
T4  -> T4a oracle strength / T4b mutation kill / T4c harness health
U1  -> U1a dispatch once / U1b one-wake maximum
H5  -> H5a stale command name / H5b nonexistent hook
```

Each item can be repaired without deciding its sibling: the sources occupy
separate schema requirements, text blocks, metadata, or behavioral proof
obligations.

### Severity arithmetic

Fresh parsing of the twelve explicit ledger rows yields:

```text
P0=13
P1=25
P2=34
total=72
```

The three v8 corrections change audit coverage, not repository repair units.

## Twelve one-by-one judgments

| Skill | Binding audit judgment | Fresh evidence |
|---|---|---|
| bootstrap | **SUPPORTED — 0/0/3.** Pre-loop create-only initializer, no RPI authority. | Canonical skill read in full; `snapshot_intent` creates its store at `validate.py:427`; status treats missing stores as empty. |
| codebase-recon | **SUPPORTED — 0/2/3.** Upstream falsifiable evidence producer, not a verdict writer. | Canonical skill and validator read in full; validator fixes `repo_root` to itself and accepts bare paths/directories; fresh scenarios passed 2/2. |
| handoff | **SUPPORTED — 3/1/5.** Session-boundary evidence with no continuation authority. | Canonical skill, schema, Go writer/reader/init/environment owners; fresh dry run produced exactly three schema errors; cleanup correction verified. |
| refactor | **SUPPORTED — 3/4/1.** Optional Implement method with real C3 contradictions. | Skill denies commit/revert; owned feature grants both; skill also says mismatch may be “explain or revert”; fresh scenarios failed 0/4. |
| reverse-engineer | **SUPPORTED — 4/6/7.** External research feeding Plan; no core phase or verdict ownership. | All 27 producers walked, Witness E replayed, H1–H6 checked, rows 3/19 and 10/20 kept distinct. V8's replacement matrix is now independently supported. |
| sbh | **SUPPORTED — 0/2/1.** Optional caller-authorized destructive host adapter outside RPI. | Canonical skill read in full; mutation/deletion surface and missing witness remain accurately disclosed; no destructive action was run. |
| scaffold | **SUPPORTED — 3/1/3.** Optional Implement method with C3 grants in an owned reference. | `generic-templates.md` separately grants commit and next steps; validator reads only `SKILL.md`; fresh validator passed while scenarios failed 0/4. |
| shared | **SUPPORTED — 0/0/1.** Just-in-time reference policy, not permission or phase authority. | Canonical file read in full; `effects: []` is accurate. |
| status | **SUPPORTED — 0/1/3.** Read-only evidence-store view. | Canonical skill and Go owner read; focused test passed; fresh empty-store JSON contained counts/state/checked/not-checked, not per-artifact digests. |
| test | **SUPPORTED — 0/6/3.** Optional Implement evidence method. | Canonical skill and all three distinct doctrines inspected; fresh scenarios failed 0/3 while prose validator passed 5/5. |
| using-gc | **SUPPORTED — 0/2/2.** Explicit optional runtime adapter; GC close is not AgentOps completion. | Canonical skill read; local annotated `v1.3.5` tag object `aec4c52e…` dereferences to `8ffc009d…`; idempotent single-item and batch no-nudge source confirmed. |
| workflow-builder | **SUPPORTED — 0/0/2.** Meta-authoring for one-shot adapters, no lifecycle authority. | Canonical skill read in full; at-most-once witness remains absent; real output is a filesystem write. |

The five fresh scenario results are exactly:

```text
codebase-recon    rc=0  pass 2/2 errors=0
refactor          rc=1  fail 0/4 errors=4
scaffold          rc=1  fail 0/4 errors=4
test              rc=1  fail 0/3 errors=3
reverse-engineer  rc=1  fail 0/3 errors=3
```

Fresh auxiliary checks also reproduced:

```text
scaffold validator  rc=0  PASS
test validator      rc=0  5 passed / 0 failed
status focused test rc=0  PASS
```

The local Gas City tag identities are:

```text
annotated tag object  aec4c52ef7649e37a6c4795f1b177bce0401d5ea
release commit        8ffc009ded781a2ada2077f3a29bd712b2def0bf
```

Issue #4586 remains prior primary-source evidence, as v8 explicitly discloses;
no live city or current-upstream claim was inferred from it.

## Full provenance

Every available artifact in the audit chain was rehashed. V8's abbreviated
ancestor prefixes resolve to the following full identities:

| Artifact | SHA-256 |
|---|---|
| origin audit `agentops-opus5-skill-audit-workflow-12.md` | `867647e1f776fe723e221c07a2c71997c4252731c905483df9b2f7b2319a070d` |
| origin Sol review | `ae8859fbc57a4e60baf523de8836918c563c18904312009bda90fe4072aaa162` |
| v2 audit | `4ba0d4c339751fbfac48674d33d384f046648d750149dca73a0835ada7318083` |
| v2 Sol review | `fe4c987834886544da82f6522ca75ed609812a65a36c9659431e7e2f44712360` |
| v3 audit | `59c87b6344316ae6149261d12aca739ec7b059a4bc97707072f44c37dfa53e6f` |
| v3 Sol review | `abff70ea33e977b384041c1a08351ef7346e708acec6a96d1250a4a4dd597051` |
| v4 audit | `b93f36c8620a382b251419acb9a9babdca979a667f918e0013971419a6e3de7c` |
| v4 Sol review | `c2e3a0e5cb9d5fd6f578020ace033bb0036748a83f6738b8adff08041b05df86` |
| v5 audit | `71f060d9ab84f76ca3a38eb37b16a05123807afe61bf4c52b4535ed250cdd951` |
| v5 Sol review | `f4cc66ae875464b2e345326ca86e496c59c871388821691467fee5b770d0b753` |
| v6 audit | `0515700b6afda3fbad71b00ed971bce9357a5fcee94c488e80070c935cee61c9` |
| v6 Sol review | `a29ff4a4f1522705ff6b1c5a80f6251b4e9d98750ecf671018fd8f280734e78c` |
| v7 audit | `ec07309d243bf3e86319dc07d22da70eda2bb93a21c0f6732868fd80a200dca8` |
| v7 Sol review | `9e6771e7aecd27d63a3941722d4368a86583dc526dd509194b532ba4fb4a07c9` |
| v8 audit | `b9d6b3d913990509319d86260ce7dd918490d4ecf94ad9b38012208414da357a` |

No artifact in that chain was modified.

## Checked

- Exact v8 subject and v7 Sol review, both in full.
- Exact repository branch, HEAD, tree, and clean status at open and close.
- Root operating contract, Validate skill contract, operating-loop architecture,
  ADR-0016, frontmatter-v2, contract-v3, and handoff-v1.
- All twelve canonical `SKILL.md` files in full.
- All 70 tracked owned path identities and every v2 appendix SHA-256 against
  current Git blob bytes.
- Reverse-engineer main call/write sequence and durable-writer search across
  its script tree.
- Fresh Witness E, all three row-26 states, row-18 NOOP, row-20 direct negative
  seam, and security temp cleanup.
- Handoff Go writer and Git-environment helper; fresh HEAD-built dry-run schema
  validation.
- Codebase-recon validator, refactor feature, scaffold authority-bearing
  reference and validator, test doctrine/feature/validator, status Go owner and
  focused test, and local Gas City v1.3.5 tag source.
- Six C1 locations, C2/C3 membership, both effects rails, all seven split
  bundles, and severity arithmetic.
- Full origin-through-v8 artifact digest chain.

## Not checked

- No destructive SBH command, deletion, tuning, service change, or host
  configuration mutation.
- No live Gas City city/session, dispatch, wake, pane, doctor, or state
  mutation.
- Witnesses A–D were not re-executed in this context; their exact row
  classifications were independently checked against live control flow and
  prior digest-bound Sol observations. Witness E was freshly replayed.
- No external network clone, untrusted binary execution, or real upstream
  repository analysis.
- No row-18 archive-candidate materialization; only the no-openable-ZIP branch
  was freshly observed.
- No end-to-end row-20 dual-suppression package injection; the direct
  empty-evidence seam was freshly executed.
- No successful real-`syft` scan or semantic `go list`; the documented branch
  residuals remain source/prior-witness based.
- No induced handoff failure between `CreateTemp` and `Rename`; conditional
  cleanup is source-established.
- No full Go suite, full repository gates, fuzzing, formatter, generator,
  projection, or synchronization command.
- No other 37 skills, proposed repair implementation, commit, merge, push, tag,
  release, publication, or canonical verdict persistence.

These exclusions do not prevent PASS because v8 names the same residuals and
does not claim execution or proof it does not have.

## Closing seal

```text
subject  b9d6b3d913990509319d86260ce7dd918490d4ecf94ad9b38012208414da357a
prior    9e6771e7aecd27d63a3941722d4368a86583dc526dd509194b532ba4fb4a07c9
HEAD     0088c6e3824da201eabb1e751ac8e976599e0b5c
tree     c0c43eefb8042af5a6a7877c0f7f0de80149ffc6
status   clean
result   PASS
```

The review file's own SHA-256 and line/byte count are computed after its final
byte and reported alongside; a file cannot contain its own digest.
