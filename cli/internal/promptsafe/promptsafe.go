// Package promptsafe provides dependency-free helpers for safely handing
// untrusted material to an agent: stripping harness delimiter tags out of text
// that gets interpolated into a prompt an agent will read, and stripping
// inherited secret-bearing environment out of the env passed to agent-invoked
// child commands.
//
// It is a leaf package — standard library only — so any layer (cmd/ao,
// internal adapters) can call it without an upward or cyclic dependency.
package promptsafe

import (
	"regexp"
	"strings"
)

// harnessTagNames are the literal delimiter tag names the harness injects
// around agent context. Untrusted text that reconstructs any of them can break
// out of its surrounding block and impersonate the harness or operator, so
// SanitizeLeaf strips them.
var harnessTagNames = []string{
	"system-reminder",
	"command-name",
	"command-message",
	"local-command-stdout",
}

// harnessTagRe matches one open or close harness delimiter tag,
// case-insensitively and tolerant of whitespace after '<', after '/', and
// before '>' (e.g. "</ System-Reminder >"), including an attribute-style tail
// ("<system-reminder role=\"x\">") — a tag variant with junk after the name
// still reads as the tag to a model, so it must not survive stripping. The
// tail excludes '<' and '>' so matching never jumps across an adjacent tag.
// Compiled once at init. RE2 is linear-time, so there is no
// catastrophic-backtracking risk from hostile input.
var harnessTagRe = regexp.MustCompile(`(?i)<\s*/?\s*(?:` + strings.Join(harnessTagNames, "|") + `)(?:\s[^<>]*)?>`)

// SanitizeLeaf strips harness delimiter tags — <system-reminder> and its close,
// plus <command-name>/<command-message>/<local-command-stdout> — from
// untrusted text before it is interpolated into agent-readable output.
// Matching is case-insensitive and whitespace-tolerant inside the tag.
//
// Stripping repeats to a fixpoint. A single pass is not enough: deleting one
// tag can splice its neighbors into a brand-new tag — e.g.
// "<system-remi<system-reminder>nder>" collapses to "<system-reminder>" once
// the inner copy is removed. The loop runs until a full pass deletes nothing,
// so no tag survives by reconstruction. Every changing pass strictly shortens
// the string, so the loop is guaranteed to terminate.
//
// This is not a general HTML escape: it touches only the harness delimiter
// tags and leaves all other text, including lone '<' / '>' characters, intact.
func SanitizeLeaf(s string) string {
	if s == "" {
		return s
	}
	for {
		stripped := harnessTagRe.ReplaceAllString(s, "")
		if stripped == s {
			return stripped
		}
		s = stripped
	}
}

// secretKeyRe matches an environment KEY that looks secret-bearing, as a
// case-insensitive substring. Mirrors the gascity trust-boundary set:
// TOKEN, SECRET, API_KEY, APIKEY, PASSWORD, CREDENTIAL, PRIVATE_KEY, ACCESS_KEY.
var secretKeyRe = regexp.MustCompile(`(?i)(TOKEN|SECRET|API_KEY|APIKEY|PASSWORD|CREDENTIAL|PRIVATE_KEY|ACCESS_KEY)`)

// StripSecretEnv returns env with every entry whose KEY looks secret-bearing
// removed, preserving order and every non-secret entry. Only the key (the text
// before the first '=') is inspected, so a benign key with a secret-looking
// value survives.
//
// Keys named in allowlist are always preserved (case-insensitive exact match),
// representing an explicit operator decision to pass a secret through to an
// agent-invoked child command.
func StripSecretEnv(env []string, allowlist ...string) []string {
	allow := make(map[string]struct{}, len(allowlist))
	for _, k := range allowlist {
		allow[strings.ToUpper(k)] = struct{}{}
	}
	out := make([]string, 0, len(env))
	for _, e := range env {
		key, _, _ := strings.Cut(e, "=")
		if _, ok := allow[strings.ToUpper(key)]; ok {
			out = append(out, e)
			continue
		}
		if secretKeyRe.MatchString(key) {
			continue
		}
		out = append(out, e)
	}
	return out
}
