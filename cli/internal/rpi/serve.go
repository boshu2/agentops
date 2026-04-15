package rpi

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// ServeRunIDPattern matches persisted run IDs in positional-argument position:
// rpi-<8-12hex> or bare 12-hex. Bare 8-hex is excluded from positional
// classification to avoid false positives with short git SHAs passed as goals.
var ServeRunIDPattern = regexp.MustCompile(`^(rpi-[a-f0-9]{8,12}|[a-f0-9]{12})$`)

// ExplicitServeRunIDPattern matches run IDs supplied via the explicit --run-id
// flag. It is deliberately more permissive than ServeRunIDPattern: because the
// user typed --run-id, there is no ambiguity with goal strings, so bare 8-hex
// (historical run IDs still present on disk at .agents/rpi/runs/<8hex>/) is
// accepted too. Keeping this wider shape on the explicit path fixes the bug
// where legacy 8-hex runs were unreachable via --run-id.
var ExplicitServeRunIDPattern = regexp.MustCompile(`^(rpi-[a-f0-9]{8,12}|[a-f0-9]{8}|[a-f0-9]{12})$`)

// ClassifyServeArg returns (goal, runID) from flags and positional args.
// The explicit --run-id flag wins over positional args and uses the wider
// ExplicitServeRunIDPattern (bare 8-hex allowed). A positional token is
// classified with the stricter ServeRunIDPattern so ambiguous short hex
// tokens are still treated as goal strings.
func ClassifyServeArg(flagRunID string, args []string) (goal, runID string) {
	if tok := strings.TrimSpace(flagRunID); tok != "" {
		if ExplicitServeRunIDPattern.MatchString(tok) {
			return "", tok
		}
		return tok, ""
	}
	if len(args) > 0 {
		tok := strings.TrimSpace(args[0])
		if ServeRunIDPattern.MatchString(tok) {
			return "", tok
		}
		return tok, ""
	}
	return "", ""
}

// ValidateExplicitServeRunID validates an explicit --run-id flag value.
// Returns the trimmed token on success, or an error describing the expected shape.
// Accepts the wider ExplicitServeRunIDPattern so historical bare 8-hex run IDs
// remain reachable via the explicit flag.
func ValidateExplicitServeRunID(flagRunID string) (string, error) {
	tok := strings.TrimSpace(flagRunID)
	if tok == "" {
		return "", nil
	}
	if !ExplicitServeRunIDPattern.MatchString(tok) {
		return "", fmt.Errorf("invalid --run-id %q: expected rpi-<8-12 hex>, <8 hex>, or <12 hex>", tok)
	}
	return tok, nil
}

// ParseServeRunTime parses a persisted RFC3339(Nano) timestamp, returning zero on failure.
func ParseServeRunTime(raw string) time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}
	}
	if ts, err := time.Parse(time.RFC3339Nano, raw); err == nil {
		return ts
	}
	if ts, err := time.Parse(time.RFC3339, raw); err == nil {
		return ts
	}
	return time.Time{}
}

// IsLocalhostOrigin returns true if the given origin URL is a localhost origin.
// Used to gate CORS allow-origin decisions on the dashboard server.
func IsLocalhostOrigin(origin string) bool {
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return false
	}
	host := u.Hostname()
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}
