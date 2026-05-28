# Executable spec for the /ratchet skill — permanent progress gates (BC3 Loop).
# /ratchet is the Brownian Ratchet: Chaos × Filter → Ratchet. It records passed
# loop-step gates to .agents/ao/chain.jsonl so progress is locked and monotonic —
# you cannot un-ratchet. Hexagon: domain; consumes validation, vibe, post-mortem
# (the filter outputs it locks); produces .agents/rpi/*.md + chain entries. (soc-qk4b.2)

Feature: Ratchet locks loop progress permanently
  As the loop's progress lock
  I want passed gates recorded to a monotonic chain
  So that progress is permanent — chaos is filtered, then ratcheted, never undone

  Scenario: Chaos is filtered, then ratcheted
    Given multiple attempts (chaos) that pass their validation gate (filter)
    When the gate result is ratcheted
    Then the progress is locked permanently in the chain

  Scenario: A passed gate is recorded to the chain
    When /ratchet records a completed step (research, plan, implement, vibe, post-mortem)
    Then a chain entry is appended to .agents/ao/chain.jsonl with step, status, and output

  Scenario: Gate state is checkable before advancing
    When /ratchet checks a step
    Then it reports whether that gate is satisfied
    And the loop does not advance past an unsatisfied required gate

  Scenario: A recorded gate cannot be un-ratcheted
    Given a step already recorded as completed in the chain
    Then progress is monotonic — the recorded gate is not reversed
