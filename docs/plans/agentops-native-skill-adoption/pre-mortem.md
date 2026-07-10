# Pre-mortem — AgentOps-native skill adoption

Status: mitigations frozen before tracker creation.

| Failure mode | Earliest signal | Preventive control | Recovery |
|---|---|---|---|
| Acceptance appears green while behavior is absent | a filter runs zero tests, or a missing command is interpreted as a semantic result | one named test per frozen Gherkin scenario; assert executables first; direct GC `CANONICAL` tests | reject the receipt, strengthen the test, rerun red before beads |
| New idea skills steal discovery/plan-pawl ownership | two skills claim the same BDD or decision artifact | versioned `idea-portfolio.v1` and `idea-challenge.v1` boundaries; discovery alone persists BDD, plan-pawl alone decides | revert the leaf routing slice and restore the prior entry-point owner |
| Docs claim ports/adapters that do not exist | mesh references `AgentWorker` or `ReviewLanePort` with no compiling implementation | compile-first Go acceptance for supervisor, NTM adapter, narrow review port, and worker-backed adapter | revert adapter slice and remove dependent routes before regeneration |
| GC writes a convincing but invalid verdict | `DEGRADED`, raw `pass`, or extra failure fields appear in `pawl-verdict.v1` | validate actual finalizer output against the canonical schema; copy contained nonempty lane evidence; degradation writes a separate attempt artifact | revert GC composition claims and finalizer together; retain prior terminal verdict |
| Generated graph becomes another graveyard | two parsers/stores disagree or a node is missing | extend catalog/context-map/`ao skills graph`; frontmatter only; regen/check and adversarial duplicate/dangling/cycle/unreachable fixtures | revert the graph slice and regenerate the existing projections |
| Horizontal slices cannot prove one behavior | a child bead names multiple Given/When/Then scenarios | exactly one frozen scenario per behavior child; the closure child only aggregates already-green evidence | split the bead before implementation; never close on partial proof |
| Clean-room adoption copies distinctive expression | suspicious shared word run or external package prose appears in source | fresh drafting context gets only AgentOps specs/tests; planted similarity test; local corpus receipt and honest exposure disclosure | delete and redraft the affected source skill from frozen behaviors |
| Rollback destroys unrelated runtime/user state | plan says “delete everything” or modifies cities/sessions | per-slice git rollback; generated outputs rebuilt; no runtime session/city/mail mutation during installation | revert only owned paths, regenerate, rerun the slice test |
| ATM removal strands a behavior | active caller still points at `using-atm`/vibing or no owner remains | disposition split targets plus boundary checker and reachability graph | restore the retired source from parent commit until every inbound route has a live owner |
| GC and NTM accidentally become mutual mandatory dependencies | hard dependency edge in either direction or automatic fallback wording | optional typed context edges only; independence acceptance; operator choice remains explicit | remove optional composition routes without changing either substrate core |

The work stops before bead creation if an independent reviewer finds any of
these controls missing from the executable acceptance surface or slice manifest.
