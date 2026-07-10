package agentworker

type SupervisorAction string

const (
	SupervisorContinue SupervisorAction = "continue"
	SupervisorSuspect  SupervisorAction = "suspect"
	SupervisorNudge    SupervisorAction = "nudge"
	SupervisorReplace  SupervisorAction = "replace"
	SupervisorComplete SupervisorAction = "complete"
)

type Observation struct {
	Status   SessionStatus
	Progress bool
}

type SupervisorDecision struct {
	Action SupervisorAction
}

type Supervisor struct {
	quietBeforeNudge int
	maxNudges        int
	quietTicks       int
	nudges           int
}

func NewSupervisor(quietBeforeNudge, maxNudges int) *Supervisor {
	if quietBeforeNudge < 1 {
		quietBeforeNudge = 1
	}
	if maxNudges < 0 {
		maxNudges = 0
	}
	return &Supervisor{quietBeforeNudge: quietBeforeNudge, maxNudges: maxNudges}
}

func (s *Supervisor) Observe(observation Observation) SupervisorDecision {
	if observation.Status == StatusCompleted {
		return SupervisorDecision{Action: SupervisorComplete}
	}
	if observation.Status.Terminal() {
		return SupervisorDecision{Action: SupervisorReplace}
	}
	if observation.Progress {
		s.quietTicks = 0
		s.nudges = 0
		return SupervisorDecision{Action: SupervisorContinue}
	}
	s.quietTicks++
	if s.quietTicks < s.quietBeforeNudge {
		return SupervisorDecision{Action: SupervisorSuspect}
	}
	if s.nudges < s.maxNudges {
		s.nudges++
		return SupervisorDecision{Action: SupervisorNudge}
	}
	return SupervisorDecision{Action: SupervisorReplace}
}
