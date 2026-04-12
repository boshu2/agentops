package llm

import (
	"os"
	"regexp"
	"strings"
)

const redactionDenylistEnv = "AGENTOPS_REDACTION_DENYLIST"

// secretPatterns are the regular expressions applied to every message BEFORE
// chunking (critical per pre-mortem F3). Redaction runs ahead of chunking so
// credentials cannot leak via chunk-boundary truncation.
//
// Sources: .agents/research/2026-04-11-sessions-privacy-policy.md
var secretPatterns = []*regexp.Regexp{
	// AWS secret assignments
	regexp.MustCompile(`(?i)AWS_SECRET_ACCESS_KEY[[:space:]]*[:=][[:space:]]*["']?[^"'\s]+`),
	// AWS access keys
	regexp.MustCompile(`AKIA[A-Z0-9]{16}`),
	// GitHub personal access tokens
	regexp.MustCompile(`ghp_[A-Za-z0-9]{36,}`),
	// GitHub OAuth tokens
	regexp.MustCompile(`gho_[A-Za-z0-9]{36,}`),
	// GitHub session, refresh, and user tokens
	regexp.MustCompile(`gh[rsu]_[A-Za-z0-9]{36,}`),
	// GitLab personal access tokens
	regexp.MustCompile(`glpat-[A-Za-z0-9_\-]{20,}`),
	// Anthropic API keys
	regexp.MustCompile(`sk-ant-[A-Za-z0-9_\-]{40,}`),
	// Generic sk- API keys (OpenAI, etc.)
	regexp.MustCompile(`sk-[A-Za-z0-9]{32,}`),
	// Slack bot/app/user tokens
	regexp.MustCompile(`xox[baprs]-[0-9A-Za-z-]{20,}`),
	// Google OAuth and API keys
	regexp.MustCompile(`ya29\.[A-Za-z0-9_\-]+`),
	regexp.MustCompile(`AIza[A-Za-z0-9_\-]{35}`),
	// JWT and bearer-like tokens
	regexp.MustCompile(`\beyJ[A-Za-z0-9_\-]{8,}\.[A-Za-z0-9_\-]{8,}\.[A-Za-z0-9_\-]{8,}\b`),
	regexp.MustCompile(`(?i)\bbearer[[:space:]]+[A-Za-z0-9._~+/\-=]{20,}`),
	// Credentialed connection strings
	regexp.MustCompile(`(?i)[a-z][a-z0-9+.-]*://[^/\s:@]+:[^@\s]+@[^/\s]+`),
	// Long base64-like opaque values
	regexp.MustCompile(`\b[A-Za-z0-9+]{40,}={0,2}\b`),
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
	for _, literal := range redactionDenylistLiterals() {
		s = strings.ReplaceAll(s, literal, "[REDACTED]")
	}
	if homePathPattern != nil {
		s = homePathPattern.ReplaceAllString(s, "/FIXTURE")
	}
	if home := os.Getenv("HOME"); home != "" {
		s = strings.ReplaceAll(s, home, "/FIXTURE")
	}
	return s
}

// RedactBytes is the []byte-valued variant of Redact. Convenience for the
// parser boundary where raw message bytes are still in hand.
func RedactBytes(b []byte) []byte {
	return []byte(Redact(string(b)))
}

func redactionDenylistLiterals() []string {
	path := strings.TrimSpace(os.Getenv(redactionDenylistEnv))
	if path == "" {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	literals := make([]string, 0)
	for _, line := range strings.Split(string(data), "\n") {
		literal := strings.TrimSpace(line)
		if literal == "" || strings.HasPrefix(literal, "#") {
			continue
		}
		literals = append(literals, literal)
	}
	return literals
}
