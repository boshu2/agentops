package llm

import (
	"os"
	"regexp"
)

// secretPatterns are the prefix-match regular expressions applied to every
// message BEFORE chunking (critical per pre-mortem F3). Redaction runs ahead
// of chunking so credentials cannot leak via chunk-boundary truncation.
//
// Sources: .agents/research/2026-04-11-sessions-privacy-policy.md
var secretPatterns = []*regexp.Regexp{
	// AWS access keys
	regexp.MustCompile(`AKIA[A-Z0-9]{16}`),
	// GitHub personal access tokens
	regexp.MustCompile(`ghp_[A-Za-z0-9]{36,}`),
	// GitHub OAuth tokens
	regexp.MustCompile(`gho_[A-Za-z0-9]{36,}`),
	// Anthropic API keys
	regexp.MustCompile(`sk-ant-[A-Za-z0-9_\-]{40,}`),
	// Generic sk- API keys (OpenAI, etc.)
	regexp.MustCompile(`sk-[A-Za-z0-9]{32,}`),
	// PEM private key blocks (DOTALL across newlines)
	regexp.MustCompile(`(?s)-----BEGIN [A-Z ]+PRIVATE KEY-----.*?-----END [A-Z ]+PRIVATE KEY-----`),
}

// homePathPattern matches the current user's home directory segment; it is
// scrubbed to /FIXTURE so absolute paths in sessions don't leak across hosts.
// Resolved once at init from $HOME.
var homePathPattern *regexp.Regexp

func init() {
	if home := os.Getenv("HOME"); home != "" {
		homePathPattern = regexp.MustCompile(regexp.QuoteMeta(home))
	}
}

// Redact applies all secret patterns + home-path scrubbing to s. Returns the
// redacted string. Non-sensitive content passes through unchanged.
//
// CRITICAL: callers MUST invoke Redact on every TranscriptMessage.Content
// BEFORE handing the slice to ChunkTurns. The chunker is a pure structural
// transform; it does NOT redact.
func Redact(s string) string {
	for _, pat := range secretPatterns {
		s = pat.ReplaceAllString(s, "[REDACTED]")
	}
	if homePathPattern != nil {
		s = homePathPattern.ReplaceAllString(s, "/FIXTURE")
	}
	return s
}

// RedactBytes is the []byte-valued variant of Redact. Convenience for the
// parser boundary where raw message bytes are still in hand.
func RedactBytes(b []byte) []byte {
	return []byte(Redact(string(b)))
}
