# Executable spec for the council mode taxonomy.
# A council convenes N independent reasoners over a shared briefing and returns
# one synthesis. `--mode` selects the deliberation pattern; everything else is an
# orthogonal knob. Derived from the 2026-05-17 cross-vendor taxonomy duel
# (.agents/council/council-taxonomy-2026-05-17/VERDICT.md).

Feature: Council mode taxonomy
  As an operator
  I want one council skill with a small, exhaustive set of deliberation modes
  So that brainstorming, adversarial debate, and validation share one structure

  Background:
    Given a council substrate (an NTM swarm or codex-team) is available
    And a shared briefing on disk

  Rule: There are exactly three modes — brainstorm, debate, verdict

    Scenario: brainstorm mode diverges
      When a council runs with --mode=brainstorm
      Then each agent generates options independently before any cross-talk
      And the synthesis is a ranked set of ideas, perspectives, and risks
      And no PASS/WARN/FAIL verdict is produced

    Scenario: debate mode contends
      When a council runs with --mode=debate
      Then each agent first writes an independent position
      And every agent adversarially cross-scores every rival position 0-1000
      And a reveal round collects concessions and dissent
      And the synthesis is a ranked decision with recorded dissent

    Scenario: verdict mode converges
      When a council runs with --mode=verdict
      Then each agent judges the artifact against the stated bar independently
      And the synthesis is one PASS, WARN, or FAIL with consolidated findings

    Scenario: verdict is the default mode
      When a council runs with no --mode flag
      Then it runs as --mode=verdict

  Rule: mode (pattern) and focus (subject) are orthogonal axes

    Scenario Outline: focus never changes the deliberation pattern
      When a council runs with --mode=<mode> --focus=security
      Then the deliberation pattern is <mode>
      And the subject under deliberation is security

      Examples:
        | mode       |
        | brainstorm |
        | debate     |
        | verdict    |

  Rule: the lifecycle is mode-invariant

    Scenario: every mode runs the same lifecycle
      When a council runs in any mode
      Then it executes convene, brief, deliberate, synthesize, record in order
      And the deliberate phase isolates each agent before any cross-contamination

  Rule: depth, runtime, and roster are knobs, not modes

    Scenario: depth changes rigor, not pattern
      When --depth=quick or --depth=deep is set
      Then judge count and round count change
      And the deliberation pattern is unchanged

    Scenario: a mixed runtime is cross-vendor, not a mode
      When --runtime=mixed is set
      Then the roster spans Claude and Codex panes
      And the deliberation pattern is unchanged

  Rule: legacy skills route into council modes

    Scenario: expert-council is absorbed as debate mode
      When /expert-council is invoked
      Then it routes to council --mode=debate

    Scenario: dueling-idea-wizards maps to a focused debate
      When dueling-idea-wizards is invoked
      Then it maps to council --mode=debate --focus=ideas
