// practices: [hexagonal-architecture, ddd-bounded-context]

package canon

import (
	"os"
	"strings"
)

// Environment variables through which an orchestrator injects the acting
// actor's identity. The orchestrator (gc/ntm) sets these at agent spawn time,
// sourcing the value from the fleet's agent registry (e.g. Agent Mail
// `whois`/`register_agent`). This keeps `ao` decoupled from any MCP/session
// protocol: the registry is the source of truth, the env var is the transport.
const (
	envActor      = "AGENTOPS_ACTOR"       // "Name" or "Name <email>"
	envActorEmail = "AGENTOPS_ACTOR_EMAIL" // optional explicit email
)

// IdentitySource records which tier of the resolution chain produced an
// identity, so the CLI can show it (the guard is only as trustworthy as the
// attribution behind it — surfacing the source makes a weak one visible).
type IdentitySource string

const (
	SourceExplicit IdentitySource = "explicit" // --as flag / scripted override
	SourceEnv      IdentitySource = "env"      // AGENTOPS_ACTOR, set by the orchestrator
	SourceGit      IdentitySource = "git"      // git config (human fallback)
)

// ResolveIdentity resolves the acting actor (human OR agent) by precedence:
//
//  1. explicit  — an --as override passed by the caller
//  2. env        — AGENTOPS_ACTOR, set by the orchestrator from the agent registry
//  3. git        — git config user.name/email (the human fallback)
//
// The env tier is how a distinct AGENT identity reaches the CLI: a swarm
// orchestrator that already knows each agent's identity exports it at spawn.
// Without it, every agent on one box would attribute to the same git user and
// the cross-actor promotion guard would collapse. The guard defends against
// *accidental* self-certification, not malicious impersonation — a stronger
// (signed) tier is a later concern, not this one.
func ResolveIdentity(explicit string) (Identity, IdentitySource) {
	if id, ok := parseActor(explicit); ok {
		return id, SourceExplicit
	}
	if id, ok := parseActor(os.Getenv(envActor)); ok {
		if id.Email == "" {
			if email := strings.TrimSpace(os.Getenv(envActorEmail)); email != "" {
				id.Email = email
			}
		}
		return id, SourceEnv
	}
	return gitIdentity(), SourceGit
}

// CurrentIdentity resolves the acting actor with no explicit override (env then
// git). Preserved as the zero-argument entry point for callers that do not
// expose an --as flag.
func CurrentIdentity() Identity {
	id, _ := ResolveIdentity("")
	return id
}

// parseActor parses an actor spec in either "Name <email>" or "Name" form,
// returning ok=false for an empty/whitespace spec.
func parseActor(spec string) (Identity, bool) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return Identity{}, false
	}
	if open := strings.LastIndex(spec, "<"); open >= 0 && strings.HasSuffix(spec, ">") {
		name := strings.TrimSpace(spec[:open])
		email := strings.TrimSpace(spec[open+1 : len(spec)-1])
		return Identity{Name: name, Email: email}, true
	}
	return Identity{Name: spec}, true
}
