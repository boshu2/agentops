# Executable spec for the /post-mortem skill — loop closeout (BC3 Loop, Move 7).
# /post-mortem wraps completed work: validates it shipped correctly, extracts
# learnings under the promotion ratchet, processes/activates/retires the backlog,
# and harvests next-work for the flywheel — producing evidence, not status text.
# Hexagon: domain; consumes: implement, vibe, council; produces: result.json.
# (soc-qk4b.2)

Feature: Post-mortem closes the loop with evidence and ratcheted learning
  As the loop's closeout step
  I want completed work validated and its lessons promoted by the ratchet
  So that each turn compounds — evidence captured, learnings durable, next-work surfaced

  Background:
    Given a completed unit of work (a closed bead or epic slice)

  Scenario: Closeout validates the work actually shipped
    When /post-mortem runs
    Then it confirms the work implemented its acceptance (council / did-we-implement-it)
    And it records evidence against the bead, not a status word

  Scenario: Learnings promote only under the ratchet
    When /post-mortem extracts observations
    Then a one-off stays in the handoff, a twice-seen pattern goes to .agents/learnings/,
      a behavior-changing lesson updates a SKILL.md/template, and a must-never-regress
      becomes a gate
    And most observations die at handoff — that is correct

  Scenario: Next-work is harvested for the flywheel
    When closeout finishes
    Then actionable follow-ups are appended to .agents/rpi/next-work.jsonl

  Scenario: The cycle produces a durable result artifact
    Then /post-mortem writes result.json capturing the closeout verdict + evidence
