package daemon

import (
	"crypto/subtle"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
)

const (
	DefaultMutationTokenHeader = "X-AgentOps-Daemon-Token" // #nosec G101 -- HTTP header name, not a credential.
)

var (
	ErrMutationDenied       = errors.New("daemon mutation denied")
	ErrUnsafeBindAddress    = errors.New("daemon bind address is not loopback")
	ErrUnsafeTokenFileMode  = errors.New("daemon mutation token file mode is too permissive")
	ErrMutationTokenMissing = errors.New("daemon mutation token is missing")
)

type MutationPolicy struct {
	Token              string
	TokenHeader        string
	AllowedPaths       []string
	AllowedMethods     []string
	AllowedOrigins     []string
	RequireLocalRemote bool
}

type MutationDecision struct {
	Allowed bool   `json:"allowed"`
	Reason  string `json:"reason,omitempty"`
}

func DefaultMutationPolicy(token string, allowedPaths []string) MutationPolicy {
	return MutationPolicy{
		Token:              token,
		TokenHeader:        DefaultMutationTokenHeader,
		AllowedPaths:       allowedPaths,
		AllowedMethods:     []string{http.MethodPost, http.MethodPatch, http.MethodDelete},
		RequireLocalRemote: true,
	}
}

func LoadMutationTokenFile(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if info.Mode().Perm()&0077 != 0 {
		return "", fmt.Errorf("%w: %s has mode %o", ErrUnsafeTokenFileMode, path, info.Mode().Perm())
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	token := strings.TrimSpace(string(data))
	if token == "" {
		return "", ErrMutationTokenMissing
	}
	return token, nil
}

func ValidateLocalBindAddress(addr string) error {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}
	if strings.TrimSpace(host) == "" {
		return fmt.Errorf("%w: %q", ErrUnsafeBindAddress, addr)
	}
	if isLoopbackHost(host) {
		return nil
	}
	return fmt.Errorf("%w: %q", ErrUnsafeBindAddress, addr)
}

func AuthorizeMutation(r *http.Request, policy MutationPolicy) error {
	decision := EvaluateMutationPolicy(r, policy)
	if decision.Allowed {
		return nil
	}
	return fmt.Errorf("%w: %s", ErrMutationDenied, decision.Reason)
}

func EvaluateMutationPolicy(r *http.Request, policy MutationPolicy) MutationDecision {
	if strings.TrimSpace(policy.TokenHeader) == "" {
		policy.TokenHeader = DefaultMutationTokenHeader
	}
	if len(policy.AllowedMethods) == 0 {
		policy.AllowedMethods = []string{http.MethodPost, http.MethodPatch, http.MethodDelete}
	}
	if strings.TrimSpace(policy.Token) == "" {
		return deny("mutation token not configured")
	}
	if !stringInSet(r.Method, policy.AllowedMethods) {
		return deny("method outside mutation scope")
	}
	if len(policy.AllowedPaths) > 0 && !stringInSet(r.URL.Path, policy.AllowedPaths) {
		return deny("path outside mutation scope")
	}
	if policy.RequireLocalRemote && r.RemoteAddr != "" {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil || !isLoopbackHost(host) {
			return deny("remote address is not local")
		}
	}
	if origin := strings.TrimSpace(r.Header.Get("Origin")); origin != "" && !stringInSet(origin, policy.AllowedOrigins) {
		return deny("browser origin is not trusted")
	}
	if fetchSite := strings.ToLower(strings.TrimSpace(r.Header.Get("Sec-Fetch-Site"))); fetchSite == "cross-site" {
		return deny("browser fetch site is not trusted")
	}
	if !constantTimeTokenEqual(extractMutationToken(r, policy.TokenHeader), policy.Token) {
		return deny("mutation token mismatch")
	}
	return MutationDecision{Allowed: true}
}

func extractMutationToken(r *http.Request, tokenHeader string) string {
	if token := strings.TrimSpace(r.Header.Get(tokenHeader)); token != "" {
		return token
	}
	const bearer = "Bearer "
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(auth, bearer) {
		return strings.TrimSpace(strings.TrimPrefix(auth, bearer))
	}
	return ""
}

func constantTimeTokenEqual(got, want string) bool {
	if got == "" || want == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}

func isLoopbackHost(host string) bool {
	host = strings.Trim(host, "[]")
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func stringInSet(value string, allowed []string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}

func deny(reason string) MutationDecision {
	return MutationDecision{Allowed: false, Reason: reason}
}
