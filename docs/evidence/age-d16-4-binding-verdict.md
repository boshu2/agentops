# Evidence — M3: the gate writes the binding verdict, no self-approval (age-d16-self-hosting-route-nkr.4)

**Scope:** PROVE a bead is accepted ONLY via a fresh-context pawl verdict recorded in the ledger, and a self-approval (refuter context_id == author) is REFUSED at the door. Close any gap that lets self-approval through. Non-goal: new gate ceremony / new schema.

## Verdict: PROVEN. No production code change needed — the refusal logic is correct and the bypass vectors are already closed.

The bead's "never exercised" premise was stale at the test level: `tests/scripts/reconcile-pr.bats` is a **29/29 passing** suite that already exercises the pawl gate end-to-end through the real `reconcile-pr.sh` door (calling the real `pawl-verdict.sh check`).

## Scenario coverage

**Scenario 2 — self-approval REFUSED at the door (already proven, verified passing):**
`reconcile-pr.bats` test *"fresh-context (default): refuter whose context_id == author: HOLD exit 5 (not a fresh red-team)"* seeds a verdict whose `author_context_id` and sole refuter `context_id` are both `author-session-0` → `reconcile-pr.sh` exits 5 (PAWL-HOLD), no merge, no close. The enforcement is `scripts/pawl-verdict.sh` check's fresh-context floor (`>=1 refuter whose context_id != author_context_id`; `fresh_count < 1 → return 1`).

**No open gap that lets self-approval (or any unauthorized verdict) through.** The suite exhaustively closes the bypass vectors — all HOLD exit 5, fail-closed: no verdict, wrong-PR verdict, stale head, schema-invalid, malformed JSON, fake/unknown reviewer family, two aliases of one family (not cross-family), single-family under multi-model, no-evidence self-stamp, missing-evidence-file, unknown mode, missing author_context_id.

**Scenario 1 — accepted ONLY via a fresh-context verdict, recorded in the ledger:**
- *Accepted only via a valid verdict:* `reconcile-pr.bats` tests 23/25 (fresh-context single refuter / multi-model two families → merge exit 0); everything else HOLDs.
- *Recorded in the ledger:* M1 proved `pawl-verdict.sh write` → `ao provenance emit-verdict` → a hash-chained verdict row (`age-d16-self-hosting-route-nkr.1`).
- **The join (this arc's lock):** the SAME artifact that `check` authorizes is the one whose `write` recorded the ledger row. `tests/scripts/pawl-verdict-binding-verdict.bats` proves it on the producer→check path: write a fresh-context verdict → it fires `emit-verdict` (ledger half) AND `check` on the same file exits 0 (accept half); a self-approval variant → `check` refuses; a stale-head variant → `check` refuses. This guards the one seam no existing test covered: if the gate and the ledger ever read/write different artifacts, "accepted" and "recorded" silently decouple.

## What this arc ships (regression lock, not new gate surface)

- `tests/scripts/pawl-verdict-binding-verdict.bats` — the accept-authorize ⟺ ledger-record join + self-approval/stale negatives at the producer→check level (hermetic, `ao` stubbed).

## Note for the live path

Like M1, the production provenance ledger still has 0 verdict rows because no real bead has been driven to acceptance through this door yet (the door is the **unattended D16 self-hosting acceptance path**, not the interactive push-to-main flow). The logic is proven; the live verdict rows accumulate once the D16 unattended loop runs a real bead to accepted (the epic done-test) — which is also what unblocks the EFC pilot's N≥50 (`age-k2w`).
