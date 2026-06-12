# Research excerpt — worker-lane retry behavior under provider rate limits

> Golden fixture for /operationalize. Fake but representative deep-research
> output: three sources, stable anchors, one genuine disagreement.

## Findings

[RX-1] (source: provider-docs, 2026-05) Burst retries against a 429 extend the
penalty window. Both surveyed providers document that immediate re-send after a
rate-limit response resets the cooldown clock; clients that waited the full
`retry-after` header value cleared in one cycle.

[RX-2] (source: swarm-telemetry, 2026-06) Across 312 logged rate-limit events in
overnight swarm runs, lanes that retried inside 30 seconds hit a second 429
in 84% of cases. Lanes that rotated to a different account first succeeded on
the next call in 91% of cases.

[RX-3] (source: swarm-telemetry, 2026-06) Re-dispatching the in-flight work item
to a different, already-warm lane resolved 61% of rate-limit stalls faster than
waiting out the cooldown on the original lane — but produced duplicate work in
the 9% of cases where the original lane later resumed on its own.

[RX-4] (source: operator-interview, 2026-06) The operator reports that account
rotation "always beats waiting" and recommends rotating on the first 429,
unconditionally.

[RX-5] (source: provider-docs, 2026-05) One provider's terms flag rapid
credential cycling as abuse-signal behavior; sustained rotation across more
than three accounts in an hour risks a account-level review.

## Noted disagreement

RX-4 (rotate unconditionally, immediately) conflicts with RX-5 (rotation
frequency itself carries account risk). Telemetry (RX-2) supports rotation but
measured it only after a first failed retry, not as a first response.
