# Quick Mode (absorbed Vibe trigger)

`/validate --quick <artifact>` performs a bounded fresh-context proof pass:

1. pin the artifact and acceptance surface;
2. verify exact-input deterministic receipts and rerun only missing, stale,
   suspicious, or invalidated facts;
3. have one accountable validator distinct from the author inspect the narrow
   claim set for intent, correctness, and obvious risk;
4. emit the normal immutable verdict with structured observations;
5. record the author and validator identities.

Quick mode changes depth, not authority or independence. It stops at the same
proof boundary as every other mode.
