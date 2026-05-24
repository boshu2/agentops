# Executable spec for the agentopsd daemon lifecycle — BC5 Runtime.
# agentopsd is the always-on lane /quickstart points at (`ao daemon run`): it starts,
# accepts jobs over its control plane, validates payloads before any ledger write, runs
# work with panic-recovery, heartbeats long jobs under a write timeout, and reaps its
# worker goroutines on cancel/shutdown. The daemon-hardening lane (panic-recovery,
# goroutine-reap, heartbeat-timeout, payload-validation, request-hardening) is the
# acceptance surface here. Each scenario is @covered-by an existing daemon test in
# cli/internal/daemon — the executable proof for that behavior. (soc-6kf1t / soc-gqhrz)

Feature: agentopsd daemon lifecycle — start, accept, run, heartbeat, reap
  As the AgentOps runtime (BC5)
  I want the daemon to start, accept and validate jobs, run them with panic-recovery,
    heartbeat under a timeout, and reap its goroutines on shutdown
  So that the always-on job lane is durable, bounded, and crash-resistant

  @covered-by:cli/internal/daemon/lifecycle_test.go::TestLifecycleDryRunPlan
  Scenario: the daemon builds a service-install plan on start
    Given a repo and an ao executable path
    When the daemon lifecycle builds the service-install plan in dry-run mode
    Then the plan names the agentopsd service, carries the daemon run args, and resolves a unit path

  @covered-by:cli/internal/daemon/jobs_test.go::TestQueueSubmitClaimHeartbeatComplete
  Scenario: a job moves through submit, claim, heartbeat, and completion
    Given a started daemon job queue
    When a job is submitted, claimed by a worker, heartbeated, then completed
    Then the queue reflects each transition and lands the job in a completed terminal state

  @covered-by:cli/internal/daemon/validate_payload_test.go::TestSubmitJob_RejectsMalformedPayloadBeforeAppend
  Scenario: a malformed payload is rejected before any ledger append
    Given a job submission with a malformed payload
    When the daemon validates the payload on submit
    Then the job is rejected and nothing is appended to the ledger

  @covered-by:cli/internal/daemon/server_mutation_test.go::TestMutationRouteAcceptsJobAfterLedgerAppend
  Scenario: a valid job is accepted only after the ledger append succeeds
    Given a well-formed job submission over the mutation control plane
    When the daemon appends the accept event to the ledger
    Then the route acknowledges the job as accepted after the append commits

  @covered-by:cli/internal/daemon/supervisor_test.go::TestSafeRunJob_RecoversPanicAsError
  Scenario: a panic inside RunJob is recovered as an error, not a crash
    Given a worker whose RunJob panics
    When the supervisor runs the job through safeRunJob
    Then the panic is recovered into a job error and a zero-value result, not a process crash

  @covered-by:cli/internal/daemon/supervisor_test.go::TestSupervisor_PanickingWorkerDoesNotKillDaemon
  Scenario: a panicking worker does not take down the daemon
    Given a worker that panics on its job and a follow-up job behind it
    When the supervisor processes both jobs
    Then the panicked job is recorded as a recovered failure and the next job still completes

  @covered-by:cli/internal/daemon/supervisor_test.go::TestSupervisor_beat_TimesOutAndSkips
  Scenario: a blocking heartbeat write is bounded by a timeout
    Given a long-running job whose heartbeat write would block indefinitely
    When the supervisor's beat hits its context timeout
    Then the heartbeat is skipped rather than wedging the worker

  @covered-by:cli/internal/daemon/supervisor_test.go::TestSupervisor_HeartbeatGoroutineDoesNotLeak
  Scenario: heartbeat goroutines are reaped on every exit path
    Given a job running with a heartbeat goroutine
    When the job completes, its context is cancelled, or it times out
    Then the spawned heartbeat goroutine is reaped before the run returns

  @covered-by:cli/internal/daemon/supervisor_test.go::TestSupervisor_RunLoopStopsOnCancel
  Scenario: the run loop stops cleanly when its context is cancelled
    Given a running daemon supervisor loop
    When the loop's context is cancelled
    Then the loop stops and returns without leaving work running

  @covered-by:cli/internal/daemon/server_mutation_test.go::TestSubmitJobRejectsBodyOverMaxBytes
  Scenario: an oversized submission body is rejected before any ledger write
    Given a job submission whose body exceeds the max submission bytes
    When the daemon reads the request through its bounded reader
    Then it short-circuits with 413 Request Entity Too Large before appending anything

  @covered-by:cli/internal/daemon/server_mutation_test.go::TestSubmitJobRejectsOverlongIdempotencyKey
  Scenario: an overlong idempotency key is rejected at the boundary
    Given a job submission carrying an idempotency key past the 256-byte bound
    When the daemon validates the request
    Then the submission is rejected as a bad request before it can dedup or append
