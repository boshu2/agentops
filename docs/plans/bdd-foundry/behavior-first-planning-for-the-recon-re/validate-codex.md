# Codex Validation — Proposed Bead Acceptance

Verdict: **15/16 crank-ready**.

Scope: independent read of `beads-manifest.md` against the frozen contract in
`behaviors.md` and the executable map in `acceptance-tests.md`. I did not run the
red-by-construction suite; this is an acceptance-shape and scenario-mapping grade.

## Per-bead Grade

| Bead | Concrete invocable acceptance? | Grade | Notes |
|---|---:|---|---|
| `a0-vendor-govern` | yes | crank-ready | Invokes Bats filters for A0-S1/S2/S5. Remaining A0 hardening/negative-path scenarios are assigned to `a0-govern-harden`. |
| `a0-govern-harden` | yes | crank-ready | Invokes A0-G14, A0-G10, A0-S3, A0-S4. |
| `a1-model-pin` | yes | crank-ready | Invokes A1-S1/S2/S3/G7. |
| `a2-empty-guard` | yes | crank-ready | Invokes A2-S1/S2/S3/G4/G8. |
| `a3-escalate-repair` | yes | crank-ready | Invokes A3-S1/S2/S3. |
| `a4-scope-since` | yes | crank-ready | Invokes A4-S1/S2/S3/S4/G5/G6. |
| `a5-monitor-guidance` | yes | crank-ready | Invokes A5 content/documented-contract assertions S1/S2/G12. |
| `b1-doc-skill-refs` | yes | crank-ready | Invokes B1-S1/S2/S3/S4/S5/G9/G11. |
| `b3-quorum-doctrine` | no | **not crank-ready** | Acceptance invokes `B3-S1 + B5-G3`, but the bead's own behavior is B3-S2/S3/S4: doctrine note, two memories, and consumer reconciliation. Those are not asserted by the referenced test. |
| `b5-forged-ratification` | yes | crank-ready | Invokes the combined liveness Bats line covering B5-G3 plus the B3-S1 guardrail. |
| `b2-changelog-tag` | yes | crank-ready | Invokes B2-S1/S2/S3/S4. B2-S4 is a light ordering/content assertion, but the manifest also carries the `b3-quorum-doctrine` dependency. |
| `b2-remote-tag-proof` | yes | crank-ready | Invokes B2-G15 remote-tag assertion. |
| `b5-codex-trust-boundary` | yes | crank-ready | Direct Go invocation copies generated acceptance tests and runs B5-S1/S2/S4/G1/G2. |
| `b5-symlink-paths` | yes | crank-ready | Direct Go invocation runs B5-G13. |
| `b5-provenance-doc` | yes | crank-ready | Direct Go invocation runs B5-S3. |
| `b4-decompose-cmd-ao` | yes | crank-ready | Invokes B4-S3/S1/S2. |

## Thin Ones

- `b3-quorum-doctrine`: not acceptable as written. It has an invocable command, but the command is for the wrong acceptance surface. It can go green without the doctrine note, memory edits, or consumer reconciliation required by B3-S2/S3/S4.
- `b2-changelog-tag`: counted crank-ready, but B2-S4 is thin. The Bats assertion checks changelog wording, while the behavior also says the bead carries an ordering dependency. The manifest has that dependency, so this is not a blocker for the proposed set, but the executable assertion is weaker than the scenario text.

## Biggest Systemic Gap

The bead set has no mechanical bead-to-scenario coverage gate. A bead can attach a real,
invocable test command that exercises neighboring or guardrail scenarios while skipping its
own frozen behavior. `b3-quorum-doctrine` is the clear example: the harness has B3-S1 and
B5-G3, but no executable/content assertions for B3-S2, B3-S3, or B3-S4. A crank gate should
require each bead's declared scenario refs to resolve to tests that cover the bead's stated
behavior, not merely to any runnable command.
