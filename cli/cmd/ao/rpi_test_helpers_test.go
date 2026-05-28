// practices: [agile-manifesto, dora-metrics]
package main

// newTestPhasedState returns a phasedState with sensible test defaults.
// Use the With* builder methods to override specific fields.
func newTestPhasedState() *phasedState {
	return &phasedState{
		SchemaVersion: 1,
		Goal:          "test goal",
		Phase:         1,
		StartPhase:    1,
		Cycle:         1,
		Verdicts:      make(map[string]string),
		Attempts:      make(map[string]int),
		StartedAt:     "2026-02-19T00:00:00Z",
	}
}

// WithGoal sets the Goal field.
func (s *phasedState) WithGoal(g string) *phasedState {
	s.Goal = g
	return s
}

// WithEpicID sets the EpicID field.
func (s *phasedState) WithEpicID(id string) *phasedState {
	s.EpicID = id
	return s
}

// WithPhase sets both Phase and StartPhase.
func (s *phasedState) WithPhase(p int) *phasedState {
	s.Phase = p
	s.StartPhase = p
	return s
}

// WithStartPhase sets only StartPhase.
func (s *phasedState) WithStartPhase(p int) *phasedState {
	s.StartPhase = p
	return s
}

// WithOpts sets the phasedEngineOptions.
func (s *phasedState) WithOpts(opts phasedEngineOptions) *phasedState {
	s.Opts = opts
	return s
}

// WithRunID sets the RunID field.
func (s *phasedState) WithRunID(id string) *phasedState {
	s.RunID = id
	return s
}

// WithFastPath sets the FastPath field.
func (s *phasedState) WithFastPath(fp bool) *phasedState {
	s.FastPath = fp
	return s
}

// WithSwarmFirst sets the SwarmFirst field.
func (s *phasedState) WithSwarmFirst(sf bool) *phasedState {
	s.SwarmFirst = sf
	return s
}

// WithTestFirst sets the TestFirst field.
func (s *phasedState) WithTestFirst(tf bool) *phasedState {
	s.TestFirst = tf
	return s
}

// WithCycle sets the Cycle field.
func (s *phasedState) WithCycle(c int) *phasedState {
	s.Cycle = c
	return s
}

// WithSchemaVersion sets the SchemaVersion field.
func (s *phasedState) WithSchemaVersion(v int) *phasedState {
	s.SchemaVersion = v
	return s
}

// WithStartedAt sets the StartedAt timestamp string.
func (s *phasedState) WithStartedAt(t string) *phasedState {
	s.StartedAt = t
	return s
}

// WithWorktreePath sets the WorktreePath field.
func (s *phasedState) WithWorktreePath(p string) *phasedState {
	s.WorktreePath = p
	return s
}

// WithOrchestratorPID sets the OrchestratorPID field.
func (s *phasedState) WithOrchestratorPID(pid int) *phasedState {
	s.OrchestratorPID = pid
	return s
}

// WithVerdicts sets the Verdicts map.
func (s *phasedState) WithVerdicts(v map[string]string) *phasedState {
	s.Verdicts = v
	return s
}

// WithVerdict adds a single verdict entry to the Verdicts map.
func (s *phasedState) WithVerdict(key, val string) *phasedState {
	if s.Verdicts == nil {
		s.Verdicts = make(map[string]string)
	}
	s.Verdicts[key] = val
	return s
}

// WithAttempts sets the Attempts map.
func (s *phasedState) WithAttempts(a map[string]int) *phasedState {
	s.Attempts = a
	return s
}

// WithAttempt adds a single attempt entry to the Attempts map.
func (s *phasedState) WithAttempt(key string, count int) *phasedState {
	if s.Attempts == nil {
		s.Attempts = make(map[string]int)
	}
	s.Attempts[key] = count
	return s
}

// WithTerminalStatus sets the TerminalStatus field.
func (s *phasedState) WithTerminalStatus(status string) *phasedState {
	s.TerminalStatus = status
	return s
}

// WithTerminalReason sets the TerminalReason field.
func (s *phasedState) WithTerminalReason(reason string) *phasedState {
	s.TerminalReason = reason
	return s
}

// WithMaxRetries sets Opts.MaxRetries.
func (s *phasedState) WithMaxRetries(n int) *phasedState {
	s.Opts.MaxRetries = n
	return s
}
