package agentworker

import "testing"

func TestSupervisorTwoTickPolicy(t *testing.T) {
	s := NewSupervisor(2, 1)

	if got := s.Observe(Observation{Status: StatusRunning, Progress: false}); got.Action != SupervisorSuspect {
		t.Fatalf("first quiet tick = %q, want suspect", got.Action)
	}
	if got := s.Observe(Observation{Status: StatusRunning, Progress: false}); got.Action != SupervisorNudge {
		t.Fatalf("second quiet tick = %q, want one bounded nudge", got.Action)
	}
	if got := s.Observe(Observation{Status: StatusRunning, Progress: false}); got.Action != SupervisorReplace {
		t.Fatalf("post-nudge non-recovery = %q, want replace", got.Action)
	}

	s = NewSupervisor(2, 1)
	_ = s.Observe(Observation{Status: StatusRunning})
	if got := s.Observe(Observation{Status: StatusRunning, Progress: true}); got.Action != SupervisorContinue {
		t.Fatalf("progress should reset suspicion, got %q", got.Action)
	}
	if got := s.Observe(Observation{Status: StatusCompleted, Progress: true}); got.Action != SupervisorComplete {
		t.Fatalf("completed worker = %q, want complete", got.Action)
	}
	if got := s.Observe(Observation{Status: StatusLost}); got.Action == SupervisorComplete {
		t.Fatal("lost worker must never classify as success")
	}
}
