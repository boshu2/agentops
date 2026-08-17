Feature: Learn stays off the critical path
  Scenario: Missing Learn never changes a verdict
    Given a durable verdict collection
    When Learn is not invoked
    Then candidate validity is unchanged

  Scenario: Learning remains advisory
    When Learn detects recurring evidence
    Then it cites distinct verdict and finding digests
    And it does not promote a rule or choose continuation

  Scenario: Expired observations are actually removed
    Given recognized Learn JSON contains an expiry at or before now
    When an authorized Learn invocation prunes its exact scratch root
    Then only expired direct regular artifacts are deleted
    And live files, unknown JSON, directories, symlinks, and external targets remain

  Scenario: Invalid expiry or cleanup failure blocks a durable write
    Given an observation omits expiry, exceeds 30 days, or cleanup cannot finish
    When Learn prepares durable output
    Then it reports explicit failure and writes no new observation artifact
