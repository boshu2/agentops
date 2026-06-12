# BDD Foundry Bead Validation

15/16 crank-ready

Validation basis: `docs/plans/bdd-foundry/beads-manifest.md`, the matching
`br show --json` records from `BEADS_DIR=/Users/bo/dev/agentops/_beads`, and a
non-executing Bats count check (`bats -c -f ...`) against
`docs/plans/bdd-foundry/acceptance-tests`.

I treated a bead as crank-ready only if its `## ACCEPTANCE` carries a concrete
command an implementer can run from the repo root, not just prose or "see spec".
The referenced Bats filters all resolve to real test cases.

| # | Bead | Runnable acceptance? | Verdict |
|---|------|----------------------|---------|
| 1 | `ag-d3-fixture-guard-yk7rq` | No primary Bats/filter command for its B62 harness half; it gives manual setup prose and only checks `B25` as a harness-smoke side condition. | Thin |
| 2 | `ag-m1-m2-skeleton-1thaa` | `bats -f '^B(20|21|22):' docs/plans/bdd-foundry/acceptance-tests` | Ready |
| 3 | `ag-m3-m4-lock-core-yett0` | `bats -f '^B(27|34):' docs/plans/bdd-foundry/acceptance-tests` | Ready |
| 4 | `ag-m5-m6-rebase-spine-sqoec` | `bats -f '^B(15|35|36|40):' docs/plans/bdd-foundry/acceptance-tests` | Ready |
| 5 | `ag-m7-regen-verifiers-ckesa` | `bats -f '^B(43|44|47|48):' docs/plans/bdd-foundry/acceptance-tests` | Ready |
| 6 | `ag-m8-gate-runner-medas` | `bats -f '^B(13|49|50|51):' docs/plans/bdd-foundry/acceptance-tests` | Ready |
| 7 | `ag-m8b-push-spine-ns7zw` | `bats -f '^B(1|2|12|25|52|53|54|55|56):' docs/plans/bdd-foundry/acceptance-tests` | Ready |
| 8 | `ag-queue-fifo-concurrency-5dwxw` | `bats -f '^B(3|4|5|6|26|29|30|33):' docs/plans/bdd-foundry/acceptance-tests` | Ready |
| 9 | `ag-liveness-staleness-signals-mo5as` | `bats -f '^B(7|8|18|28|31|32):' docs/plans/bdd-foundry/acceptance-tests` | Ready |
| 10 | `ag-branch-shapes-e2e-m5ktj` | `bats -f '^B(10|37|38|39|41):' docs/plans/bdd-foundry/acceptance-tests` | Ready |
| 11 | `ag-regen-e2e-guards-4a0f1` | `bats -f '^B(9|11|16|42|45|46|64|65):' docs/plans/bdd-foundry/acceptance-tests` | Ready |
| 12 | `ag-preflight-closeout-u58s1` | `bats -f '^B(14|19|23|24):' docs/plans/bdd-foundry/acceptance-tests` | Ready |
| 13 | `ag-recovery-abort-iodl9` | `bats -f '^B(57|58|59|60|61|70):' docs/plans/bdd-foundry/acceptance-tests` | Ready |
| 14 | `ag-guard-install-86yy3` | `bats -f '^B(17|62|63|66):' docs/plans/bdd-foundry/acceptance-tests` | Ready |
| 15 | `ag-observability-cli-3ngky` | `bats -f '^B(67|68|69|71|72):' docs/plans/bdd-foundry/acceptance-tests` | Ready |
| 16 | `ag-meta-suite-closure-eg2yn` | `bats -f '^B73:' docs/plans/bdd-foundry/acceptance-tests`; also names `bash tests/landing/run-acceptance.sh` as the final suite command. | Ready |

## Thin Ones

- `ag-d3-fixture-guard-yk7rq`: useful and specific, but not crank-ready by the
  same standard as the rest. Its acceptance is a manual recipe: source
  `helpers.bash`, create a sandbox, call `make_bare_remote` and `seed_fixture`,
  fabricate a live holder nonce, then raw-push twice. That is concrete prose, but
  it is not a single runnable acceptance test. The only Bats command in the bead
  is `bats docs/plans/bdd-foundry/acceptance-tests/02-preflight.bats -f '^B25:'`,
  which does not directly prove the B62 fixture-guard behavior the bead owns.

## Biggest Systemic Gap

Acceptance is embedded as prose paragraphs, not normalized as a required
machine-readable `acceptance_command` per bead. Most beads still include a solid
`bats -f ...` command, but regressions often collapse to shorthand like "spine
filter stays green", and the D3 bead slipped into manual procedure. A crank
worker should not have to infer the runnable command from prose or prior beads.
