---
id: learning-2026-05-01-bd-server-mode-closeout-contract
type: learning
date: 2026-05-01
category: process
confidence: high
maturity: provisional
utility: 0.7
---

# Learning: bd Server-Mode Closeout Needs Contract Proof

## What We Learned

When a repo uses bd in server/Dolt mode, successful local tracker writes are
not enough closeout evidence. The session must also prove which database and
project id are active, and whether `bd dolt push` is intentionally skipped,
backed by a real remote, or failing because the workspace is miswired.

## Why It Matters

Without this check, a crank can close every issue correctly in the local
tracker while leaving team/backup sync semantics ambiguous or broken.

## Source

Post-mortem for `soc-o6eb` on 2026-05-01. The crank recovered bd by importing
`bushido` from JSONL and later observed `bd dolt push` skip because no Dolt
remote is configured.
