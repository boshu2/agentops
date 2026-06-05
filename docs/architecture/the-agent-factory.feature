Feature: The Agent Factory — architecture fitness functions
  The load-bearing invariants of the factory, as measurable acceptance — not prose.
  Each scenario is a fitness function: PASS = the invariant holds in the live system.
  Tags: @enforced = guarded in code/CI today; @to-build = gated by a build-sequence epic.
  Spine: docs/architecture/the-agent-factory.md (ag-yv25, council-reviewed 2026-06-05).

  # ─────────────────────────  The membrane (the novel primitive)  ─────────────────────────

  @enforced @one-way-door-1
  Scenario: No agent grades its own work
    Given a verdict recorded at the ClaimEvidenceBinder
    When the judge_id equals the author_id
    Then the verdict is rejected or stamped self-graded before it lands
    # enforced: liveness kernel (#737) + check-author-judge-convergence.sh (#746)

  @enforced @one-way-door-1
  Scenario: A significant decision requires cross-model quorum
    Given a significant decision (merge to main, north-star change, doc-canonization, or P0 bead)
    When a single orchestrator attempts to commit it
    Then it is blocked until a >=2-of-3 cross-model ACK is recorded in the consensus log
    # the orchestration tier of ag-xdrw

  @to-build @build-step-4
  Scenario: Every workload output passes admission
    Given any agent produces an output destined for main
    When the output is presented for landing
    Then it passes the membrane (author!=judge, judged against the acceptance contract) or is rejected
    # the UNIVERSAL admission path — build step 4 (built at gate sites today, not yet universal)

  @to-build @boundary-6
  Scenario: The contract, not judge-agreement, is ground truth
    Given a cross-model quorum that unanimously ACKs an output
    When the output is checked against the executable acceptance contract
    And the contract fails
    Then the output is rejected despite unanimous judges
    # boundary 6: judges are not independent failure domains; correlated error is a residual risk

  # ─────────────────────────  The control plane (must be HA)  ─────────────────────────

  @to-build @build-step-1 @ag-o2tc
  Scenario: The control plane survives single-host loss
    Given the bd/Dolt control-plane state replicated across at least 2 hosts
    When the primary host dies
    Then bd reads and writes continue from another host within the failover budget
    # ag-o2tc; today FAILS (single-host bushido) — the 2026-06-05 crash is this scenario red

  @enforced @one-way-door-4
  Scenario: Control-plane binaries are version-locked across hosts
    Given the fleet version manifest pins DOLT_VERSION
    When check-fleet-versions runs across all hosts
    Then every host's dolt version equals the pin with zero skew
    # enforced today: fleet-versions.env + check-fleet-versions + bushido-health Version Pins

  @to-build @build-step-2 @one-way-door-2
  Scenario: A declared workload object is a versioned, validated contract
    Given a goal submitted with an acceptance contract
    When it is admitted to the control plane
    Then it is stored as a versioned workload object that controllers and the scheduler read
    # one-way door #2 (the factory's API) — design through the council before coding against it

  # ─────────────────────────  The data plane (fungible)  ─────────────────────────

  @to-build @build-step-6
  Scenario: A dead agent's work reschedules onto live compute
    Given a declared workload running on a compute node
    When the node or agent dies mid-work
    Then the workload reschedules onto a live node without operator intervention
    # self-healing — build step 6; today FAILS (the crash lost all 3 orchestrators, nothing rescheduled)

  @to-build @build-step-5 @ag-xanm
  Scenario: Work schedules across heterogeneous compute
    Given agent-tasks with capability and cost requirements
    And compute nodes across Mac, bushido, and cloud
    When the scheduler places the tasks
    Then each task lands on a node that satisfies its capability, cost, and model-fit
    # ag-xanm

  # ─────────────────────────  The compounding corpus (no k8s analog)  ─────────────────────────

  @to-build
  Scenario: Each session measurably improves on the last
    Given the corpus produced by session N
    When session N+1 runs the same class of work
    Then it uses fewer tokens or fails fewer times than session N
    # the flywheel as a MEASURED layer beneath the membrane, not the headline
