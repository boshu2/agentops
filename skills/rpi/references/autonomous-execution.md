# Autonomous Execution Rules

## The Four-Umbrella Rule

RPI has FOUR mandatory umbrellas: Discovery, Crank, Validate, and Learn. You
MUST run all four in order in a single lifecycle. Do NOT stop after Crank or
Validate, and do not ask the user whether to continue between umbrellas.
Validate owns the immutable proof verdict; Learn consumes that verdict and
records evidence-backed observations without changing proof or delivery state.

## Fully Autonomous by Default

Unless `--interactive` is explicitly set, RPI runs hands-free from start to finish. Do NOT:
- Ask the user for confirmation between phases
- Ask "want me to commit?" or "should I continue?"
- Pause to summarize and wait for input
- Request clarification mid-execution
- Stop to ask about approach or strategy

The human's only routine touchpoint is after Learn completes. If work reaches a
genuinely terminal blocked state under the current breaker contract, stop and
report; ordinary negative evidence re-plans the remaining lifecycle.

## Anti-Patterns (DO NOT)

| Anti-Pattern | Why It's Wrong | Correct Behavior |
|--------------|----------------|------------------|
| Stop after Crank and ask to commit | Skips independent proof and learning | Proceed through Validate and Learn |
| Treat Validate as learning or delivery | Mutates the proof boundary and gives one umbrella too much authority | Keep the verdict immutable; hand it to Learn, then return delivery to the caller |
| Ask "want me to commit?" between umbrellas | Interrupts autonomous flow — user invoked `/rpi` for hands-free execution | Complete all four umbrellas before reporting |
| Ask the user ANY question during execution | RPI is autonomous unless `--interactive` — questions break the flow | Make best judgment and proceed; report at end |
| Run Discovery inline instead of delegating to `/discovery` | Loses its owned intent-shaping sequence and receipt | Invoke the Discovery skill contract |
| Summarize findings and wait after Discovery | Discovery output is an input to Crank, not the terminal deliverable | Proceed immediately to Crank |
| Pause to explain what you're about to do | Narration wastes time — the user wants results, not commentary | Execute, then report at the end |

## Phase Completion Tracking

After each phase, log progress:
```
UMBRELLA 1 COMPLETE ✓ (discovery) — proceeding to Crank
UMBRELLA 2 COMPLETE ✓ (crank) — proceeding to Validate
UMBRELLA 3 COMPLETE ✓ (validate) — proceeding to Learn
UMBRELLA 4 COMPLETE ✓ (learn) — RPI DONE
```
