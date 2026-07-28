---
id: ratchet-sibling-failclosed-intent
proof_epoch: 1
kind: repair
status: frozen
scope: gate-proof-integrity
author_context_id: claude-fable5-ratchet-sibling-failclosed-author-20260728-session01DUtFAN2gC4ryUmkMEhDVuR
subject_commit: 5644f2ba713046130445910578420c958f8f394f
subject_tree: 469ec9412adb204fdffe617ae7e9dae462506ce4
branch: codex/ratchet-sibling-failclosed-20260728
advisory_ref: /tmp/agentops-fable-ratchet-repair-audit-20260727.md
advisory_sha256: c26f7a4e4fa973e7ee1ce8bdd5f6fcafe2e749f30fffb34d843816a4a48a3655
binding_validation_ref: /tmp/agentops-ratchet-review-5644f2ba-sol.md
binding_validation_sha256: 2af64f12fe6bc7398c7cd95ad25f1de6cd513f9c2bf77d45280dfd0fe1738558
binding_validation_result: PASS (for the 5644f2ba7 python-ratchet repair only)
---

# Ratchet sibling fail-closed repair intent — F1/F2 of the Fable advisory

## Context

The base subject `5644f2ba7` carries the Sol-PASSed python-ratchet fail-closed
repair. The Fable advisory audit of that repair (SHA bound above) found two
**same-class fail-opens in sibling consumers** of `scripts/lib/ratchet.sh`,
outside that repair's declared scope:

- **F1 — `scripts/check-new-scripts-use-preamble.sh:137`.**
  `collect_changed_files > "$tmp_changed/raw" 2>/dev/null || true` — the
  byte-identical idiom the python-ratchet repair removed: it swallows the
  library's fail-closed `return 2` AND routes its "refusing to certify" stderr
  to `/dev/null`. A dead Git read becomes an empty changed set and the gate
  certifies green. Gate `shell.preamble-ratchet` is advisory today but its
  registration comment says it flips Blocking after one clean advisory cycle;
  this must land before that flip.
- **F2 — `scripts/check-jsonl-scanner-ratchet.sh:245`.**
  `done < <(collect_changed_files | LC_ALL=C sort -u)` — process substitution
  discards the collector's rc 2 even under `set -euo pipefail`; a dead git
  read becomes an empty loop and the advisory gate certifies. The repository's
  own defense for this exact trap is documented in
  `check-atomic-write-ratchet.sh` (capture with `$(…) || exit 2` under
  pipefail).

## Required criteria

- **RS-1 — preamble gate fail-closed.** A failing changed-scope collector
  makes `check-new-scripts-use-preamble.sh` exit 2, with the collector's
  stderr preserved (no `2>/dev/null`), and the gate's green certification
  line absent.
- **RS-2 — jsonl gate fail-closed.** A failing changed-scope collector makes
  `check-jsonl-scanner-ratchet.sh` exit 2 via the atomic-write capture
  pattern, with stderr preserved and no green certification.
- **RS-3 — discriminating N7-style witnesses.** For EACH gate: (a) a witness
  that seeds a REAL violation and kills the collection git command via a PATH
  shim, so a swallowed failure would be a visibly wrong green — asserting
  exit 2, refusal text visible, green text absent; and (b) a partial-output
  witness whose shim emits at least one plausible row before dying, asserting
  exit 2 and that the gate never certifies over the truncated bytes. Witnesses
  target the collection git command, not either implementation's internals.
- **RS-KEEP — nothing else moves.** No blocking flag, no grandfather list
  byte, no governed regex, no unrelated gate, no `scripts/lib/ratchet.sh`
  change. Both gates' existing suites and all shared-lib consumers stay green;
  the negative-witness Go closure over gates stays green.

## Non-goals

- Do not flip `shell.preamble-ratchet` or `go.jsonl-scanner-ratchet` to
  Blocking; registration files are untouched.
- Do not modify `scripts/lib/ratchet.sh`, `check-skill-python-ratchet.sh`,
  `check-atomic-write-ratchet.sh`, or any other consumer.
- Do not touch F3/F4 observations from the advisory (shallow-clone posture,
  root-commit exit vocabulary).
- Do not claim PASS, merge, or push; fresh Sol remains the binding validator.

## Authorized write scope

Records first (this commit):

- `docs/evidence/proof-epochs/epoch-1/ratchet-sibling-failclosed-intent.md`

then a RED tests-only commit:

- `tests/scripts/check-new-scripts-use-preamble.bats`
- `tests/scripts/check-jsonl-scanner-ratchet.bats`

then a source-only child commit:

- `scripts/check-new-scripts-use-preamble.sh`
- `scripts/check-jsonl-scanner-ratchet.sh`

Nothing else.

## First useful checks

RED: at the tests-only commit, the four new witnesses fail for the advertised
reason (old preamble caller exits 0/1 with green or wrong text; old jsonl loop
exits 0 with green) while every pre-existing test still passes. GREEN: at the
source child, both full sibling suites pass, the two changed scripts pass
`shellcheck -S warning`, all shared ratchet-lib consumers keep their base
behavior on a sane tree, `go test ./internal/gates/...` (negative-witness
closure) passes, and the worktree is clean.
