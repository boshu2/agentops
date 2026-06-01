package contextpacket

import "time"

// ContextOptions holds the inputs previously carried as package-level flag
// variables in cmd/ao/context.go. Business logic in this package accepts a
// *ContextOptions explicitly rather than reading globals, which makes the
// context commands testable without mutating process state.
type ContextOptions struct {
	SessionID    string
	Prompt       string
	AgentName    string
	MaxTokens    int
	WriteHandoff bool
	AutoRestart  bool
	Watchdog     time.Duration
}

// DefaultWatchdog is the default watchdog window (20 minutes).
const DefaultWatchdog = 20 * time.Minute
