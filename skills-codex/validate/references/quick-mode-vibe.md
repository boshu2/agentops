# Quick Mode (absorbed Vibe trigger)

`/validate --quick <artifact>` performs a bounded inline proof pass:

1. pin the artifact and acceptance surface;
2. run the smallest relevant deterministic checks;
3. inspect the diff for intent, correctness, and obvious risk;
4. emit the normal immutable verdict with structured observations;
5. stamp independence as waived because the context is not fresh.

Quick mode changes depth, not authority. It cannot independently certify its
author's work and it stops at the same proof boundary as every other mode.
