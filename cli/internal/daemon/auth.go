package daemon

import (
	"crypto/subtle"
	"encoding/json"
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
	Tokens             []MutationToken
	TokenHeader        string
	AllowedPaths       []string
	AllowedMethods     []string
	PathCapabilities   map[string]MutationCapability
	AllowedOrigins     []string
	RequireLocalRemote bool
}

type MutationDecision struct {
	Allowed            bool                 `json:"allowed"`
	Reason             string               `json:"reason,omitempty"`
	TokenName          string               `json:"token_name,omitempty"`
	Capabilities       []MutationCapability `json:"capabilities,omitempty"`
	RequiredCapability MutationCapability   `json:"required_capability,omitempty"`
	LocalOnly          bool                 `json:"local_only,omitempty"`
}

type MutationCapability string

const (
	MutationCapabilitySubmitJob       MutationCapability = "submit_job"
	MutationCapabilityCancelJob       MutationCapability = "cancel_job"
	MutationCapabilityOpenClawTrigger MutationCapability = "openclaw_trigger"
	MutationCapabilityAdmin           MutationCapability = "admin"
)

type MutationToken struct {
	Name         string               `json:"name"`
	Token        string               `json:"token"` // #nosec G101 -- config field name, not a hard-coded credential.
	Capabilities []MutationCapability `json:"capabilities"`
	LocalOnly    bool                 `json:"local_only,omitempty"`
}

func DefaultMutationPolicy(token string, allowedPaths []string) MutationPolicy {
	return MutationPolicy{
		Token:              token,
		TokenHeader:        DefaultMutationTokenHeader,
		AllowedPaths:       allowedPaths,
		AllowedMethods:     []string{http.MethodPost, http.MethodPatch, http.MethodDelete},
		PathCapabilities:   DefaultMutationPathCapabilities(),
		RequireLocalRemote: true,
	}
}

func DefaultMutationPathCapabilities() map[string]MutationCapability {
	return map[string]MutationCapability{
		"/jobs":                         MutationCapabilitySubmitJob,
		"/v1/jobs":                      MutationCapabilitySubmitJob,
		"/jobs/cancel":                  MutationCapabilityCancelJob,
		"/v1/jobs/cancel":               MutationCapabilityCancelJob,
		"/openclaw/v1/triggers/jobs":    MutationCapabilityOpenClawTrigger,
		"/v1/openclaw/triggers/jobs":    MutationCapabilityOpenClawTrigger,
		"/v1/openclaw/v1/triggers/jobs": MutationCapabilityOpenClawTrigger,
	}
}

func LoadMutationTokenFile(path string) (string, error) {
	data, err := readMutationTokenFile(path)
	if err != nil {
		return "", err
	}
	token := strings.TrimSpace(string(data))
	if token == "" {
		return "", ErrMutationTokenMissing
	}
	return token, nil
}

func LoadMutationTokensFile(path string) ([]MutationToken, error) {
	data, err := readMutationTokenFile(path)
	if err != nil {
		return nil, err
	}
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return nil, ErrMutationTokenMissing
	}
	if !strings.HasPrefix(trimmed, "{") {
		return []MutationToken{legacyMutationToken(trimmed)}, nil
	}
	var parsed struct {
		Tokens []MutationToken `json:"tokens"`
	}
	if err := json.Unmarshal([]byte(trimmed), &parsed); err != nil {
		return nil, fmt.Errorf("parse scoped mutation token file: %w", err)
	}
	if len(parsed.Tokens) == 0 {
		return nil, ErrMutationTokenMissing
	}
	for i := range parsed.Tokens {
		normalizeMutationToken(&parsed.Tokens[i], fmt.Sprintf("token-%d", i+1))
		if err := validateMutationToken(parsed.Tokens[i]); err != nil {
			return nil, err
		}
	}
	return parsed.Tokens, nil
}

func readMutationTokenFile(path string) ([]byte, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if info.Mode().Perm()&0077 != 0 {
		return nil, fmt.Errorf("%w: %s has mode %o", ErrUnsafeTokenFileMode, path, info.Mode().Perm())
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return data, nil
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
	_, err := AuthorizeMutationDecision(r, policy)
	return err
}

func AuthorizeMutationDecision(r *http.Request, policy MutationPolicy) (MutationDecision, error) {
	decision := EvaluateMutationPolicy(r, policy)
	if decision.Allowed {
		return decision, nil
	}
	return decision, fmt.Errorf("%w: %s", ErrMutationDenied, decision.Reason)
}

func EvaluateMutationPolicy(r *http.Request, policy MutationPolicy) MutationDecision {
	if strings.TrimSpace(policy.TokenHeader) == "" {
		policy.TokenHeader = DefaultMutationTokenHeader
	}
	if len(policy.AllowedMethods) == 0 {
		policy.AllowedMethods = []string{http.MethodPost, http.MethodPatch, http.MethodDelete}
	}
	if len(policy.PathCapabilities) == 0 {
		policy.PathCapabilities = DefaultMutationPathCapabilities()
	}
	tokens := normalizedMutationTokens(policy)
	if len(tokens) == 0 {
		return deny("mutation token not configured")
	}
	if !stringInSet(r.Method, policy.AllowedMethods) {
		return deny("method outside mutation scope")
	}
	if len(policy.AllowedPaths) > 0 && !stringInSet(r.URL.Path, policy.AllowedPaths) {
		return deny("path outside mutation scope")
	}
	required := policy.PathCapabilities[r.URL.Path]
	if required == "" {
		required = MutationCapabilityAdmin
	}
	if policy.RequireLocalRemote && r.RemoteAddr != "" {
		if !isLoopbackRemote(r.RemoteAddr) {
			return deny("remote address is not local")
		}
	}
	if origin := strings.TrimSpace(r.Header.Get("Origin")); origin != "" && !stringInSet(origin, policy.AllowedOrigins) {
		return deny("browser origin is not trusted")
	}
	if fetchSite := strings.ToLower(strings.TrimSpace(r.Header.Get("Sec-Fetch-Site"))); fetchSite == "cross-site" {
		return deny("browser fetch site is not trusted")
	}
	credential, ok := matchMutationToken(extractMutationToken(r, policy.TokenHeader), tokens)
	if !ok {
		return deny("mutation token mismatch")
	}
	if credential.LocalOnly && r.RemoteAddr != "" && !isLoopbackRemote(r.RemoteAddr) {
		return deny("token requires local remote")
	}
	if !mutationTokenAllows(credential, required) {
		return deny("token capability outside mutation scope")
	}
	return MutationDecision{
		Allowed:            true,
		TokenName:          credential.Name,
		Capabilities:       append([]MutationCapability{}, credential.Capabilities...),
		RequiredCapability: required,
		LocalOnly:          credential.LocalOnly,
	}
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

func normalizedMutationTokens(policy MutationPolicy) []MutationToken {
	tokens := append([]MutationToken{}, policy.Tokens...)
	if strings.TrimSpace(policy.Token) != "" {
		tokens = append(tokens, legacyMutationToken(policy.Token))
	}
	for i := range tokens {
		normalizeMutationToken(&tokens[i], fmt.Sprintf("token-%d", i+1))
	}
	return tokens
}

func legacyMutationToken(token string) MutationToken {
	return MutationToken{
		Name:         "legacy",
		Token:        token,
		Capabilities: defaultMutationCapabilities(),
		LocalOnly:    true,
	}
}

func defaultMutationCapabilities() []MutationCapability {
	return []MutationCapability{
		MutationCapabilitySubmitJob,
		MutationCapabilityCancelJob,
		MutationCapabilityOpenClawTrigger,
	}
}

func normalizeMutationToken(token *MutationToken, fallbackName string) {
	token.Name = strings.TrimSpace(token.Name)
	if token.Name == "" {
		token.Name = fallbackName
	}
	token.Token = strings.TrimSpace(token.Token)
	for i, capability := range token.Capabilities {
		token.Capabilities[i] = MutationCapability(strings.TrimSpace(string(capability)))
	}
}

func validateMutationToken(token MutationToken) error {
	if strings.TrimSpace(token.Token) == "" {
		return ErrMutationTokenMissing
	}
	if len(token.Capabilities) == 0 {
		return fmt.Errorf("mutation token %q has no capabilities", token.Name)
	}
	for _, capability := range token.Capabilities {
		switch capability {
		case MutationCapabilitySubmitJob, MutationCapabilityCancelJob, MutationCapabilityOpenClawTrigger, MutationCapabilityAdmin:
		default:
			return fmt.Errorf("mutation token %q has unsupported capability %q", token.Name, capability)
		}
	}
	return nil
}

func matchMutationToken(raw string, tokens []MutationToken) (MutationToken, bool) {
	for _, token := range tokens {
		if constantTimeTokenEqual(raw, token.Token) {
			return token, true
		}
	}
	return MutationToken{}, false
}

func mutationTokenAllows(token MutationToken, required MutationCapability) bool {
	for _, capability := range token.Capabilities {
		if capability == MutationCapabilityAdmin || capability == required {
			return true
		}
	}
	return false
}

func isLoopbackRemote(remoteAddr string) bool {
	host, _, err := net.SplitHostPort(remoteAddr)
	return err == nil && isLoopbackHost(host)
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
