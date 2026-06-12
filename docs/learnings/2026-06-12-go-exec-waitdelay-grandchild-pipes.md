---
title: exec.CommandContext with buffer pipes needs WaitDelay — deadline kill doesn't unblock Run past grandchildren
date: 2026-06-12
tags: [go, os-exec, timeouts, subprocess]
status: draft
source: ag-7ixm9 independent validation residual → ag-1sibx fix (2026-06-12)
---

# `exec.CommandContext` + buffer pipes needs `WaitDelay`

## The gotcha

`exec.CommandContext(ctx, ...)` kills only the **direct child** on deadline.
When `Stdout`/`Stderr` are not `*os.File` (e.g. `bytes.Buffer`), Go creates OS
pipes plus copy goroutines, and `cmd.Run()` blocks until the pipe write-ends
close — which includes **every grandchild** that inherited them. A check
command that forks a background process can therefore block `Run()` long past
the context deadline even though the child was killed.

Reproduced red in `TestCodexImageHealthForkingCheckReturnsDespiteGrandchildPipeHold`:
`sh -c "sleep 60 & wait"` under a 100ms budget hung >5s without the fix.

## The rule

Anywhere we wrap a subprocess in a per-call timeout and capture output via
buffers, set `cmd.WaitDelay` (1–2s) so `Wait` abandons the pipe copy after the
deadline. Fix shape: `cli/cmd/ao/codex.go` `codexImageHealthWaitDelay` at
25d9d1c85.

## Grep surface (if this recurs, promote to skills/standards/references/go.md)

`grep -rn "CommandContext" cli/ | grep -v WaitDelay` — any hit that also sets
buffer Stdout/Stderr and a timeout is a candidate for the same hang.

## Evidence

- RED: forking test hung 5s+ at cf8084d22; GREEN: returns in ~budget+2s at 25d9d1c85.
- Validation residual that found it: .agents/council/2026-06-12-validate-ag-7ixm9-image-health-timeouts.md (mirrored to bead ag-7ixm9 comments).
