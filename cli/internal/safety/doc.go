// Package safety contains pure input and sandbox validators used by retained
// inspection commands.
//
// AgentOps does not use this package to control repository lifecycle. It does
// not authorize pushes, admission, retries, work ownership, release, or
// delivery. Repository policy and the caller own those decisions.
package safety
