# Executable spec for the /red-team skill — usability probing (supporting role).
# /red-team adopts constrained personas and ATTEMPTS REAL TASKS against a surface (a doc or a
# skill) to find where it breaks in actual use — distinct from /council (expert judgment) and
# /vibe (code quality). Hexagon: supporting; consumes repo-context (it probes the repo's docs/
# skills); produces result.json. (soc-qk4b)

Feature: Red-team probes whether docs and skills actually work in use
  As a usability adversary
  I want a surface attempted by a constrained persona doing real tasks
  So that gaps that only appear when someone TRIES to use it are found

  Scenario: a persona attempts real tasks against a surface
    When /red-team --surface=docs <target> runs
    Then it adopts a constrained persona and attempts the real tasks the surface claims to support
    And it reports what breaks (result.json)

  Scenario: it tests usability, not judgment or code quality
    Then /red-team tests whether the surface actually WORKS when used
    And this is distinct from /council (expert judgment) and /vibe (code quality)

  Scenario: the surface selects the probe target
    When --surface is given
    Then the probe targets that surface type (docs, skills) rather than guessing
