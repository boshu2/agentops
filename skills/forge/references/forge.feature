# Executable spec for the /forge skill — transcript→learning capture funnel (BC1 Corpus, loop Move 7 capture).
# /forge mines raw transcripts into CANDIDATE learnings and queues them — it is the funnel,
# not the filter: the promotion ratchet (not forge) decides which survive (one-offs die at
# handoff; repeats promote to .agents/learnings/). It consumes transcripts (raw input), not
# skills, so consumes:[] is correct. Hexagon: domain; produces .agents/research/*.md; shared-
# kernel with standards. (soc-qk4b)

Feature: Forge mines transcripts into candidate learnings
  As the capture funnel of the knowledge flywheel
  I want raw transcripts mined into queued candidate learnings
  So that the ratchet has a stream of candidates to promote or let die

  Scenario: a transcript is mined into queued candidates
    When /forge runs over a session transcript
    Then candidate learnings are extracted and queued for review
    And forge does not itself decide which are durable — it is the funnel, not the filter

  Scenario: promotion is the ratchet's call, applied by --promote
    Given a reviewed candidate worth keeping
    When /forge --promote runs
    Then the learning is written to .agents/learnings/ and its pending source is removed
    And one-offs that were not promoted simply die at handoff (correct)

  Scenario: forge consumes transcripts, not skills
    Then /forge's hexagon consumes no other skill (consumes:[] is correct — it mines raw transcripts)
    And it relates to standards as a shared kernel
