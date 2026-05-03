package daemon

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
)

// mutationRoutes is the authoritative list of mutation paths registered through
// registerMutationRoute. The build-time guard at
// scripts/check-mutation-route-coverage.sh asserts every mux.HandleFunc call
// site for a mutation path goes through this helper.
//
// PER PRE-MORTEM AMENDMENT A2 (soc-8inr.5): this slice is the parity-test
// surface; the bash script is the bypass guard. Together they close the
// loophole where a developer could call mux.HandleFunc("/v1/foo", handler)
// directly and bypass auth.
var (
	mutationRoutesMu sync.Mutex
	mutationRoutes   []string
)

// MutationRoutesForTest returns a copy of the registered mutation paths.
// Used by TestMutation_AllRegisteredRoutesEnforcePolicy and similar parity
// checks to assert every tracked route enforces auth.
func MutationRoutesForTest() []string {
	mutationRoutesMu.Lock()
	defer mutationRoutesMu.Unlock()
	out := make([]string, len(mutationRoutes))
	copy(out, mutationRoutes)
	return out
}

// resetMutationRoutesForTest is a test-only escape hatch so independent test
// runs see a stable route list. Production code never resets the slice; routes
// are registered once per router build and the guard script asserts the
// authoritative list via static analysis.
func resetMutationRoutesForTest() {
	mutationRoutesMu.Lock()
	defer mutationRoutesMu.Unlock()
	mutationRoutes = nil
}

// MutationPolicyProvider lazily resolves the policy at request time. The
// daemon defaults are evaluated per-request because tests inject options after
// the router is built.
type MutationPolicyProvider func() MutationPolicy

type mutationDecisionKey struct{}

// MutationDecisionFromContext returns the decision attached by
// registerMutationRoute. Handlers that need TokenName for ledger actor
// attribution should call this rather than re-running the policy check.
func MutationDecisionFromContext(ctx context.Context) (MutationDecision, bool) {
	d, ok := ctx.Value(mutationDecisionKey{}).(MutationDecision)
	return d, ok
}

// registerMutationRoute wraps handler in mutation-auth enforcement and
// atomically appends path to the tracked mutationRoutes slice. EVERY mutation
// route MUST be registered through this helper.
//
// The build-time guard (scripts/check-mutation-route-coverage.sh) asserts no
// other call site to mux.HandleFunc registers a mutation path — it greps the
// daemon package and fails if any HandleFunc lives outside auth.go.
func registerMutationRoute(mux *http.ServeMux, path string, policy MutationPolicyProvider, handler http.HandlerFunc) {
	mutationRoutesMu.Lock()
	mutationRoutes = append(mutationRoutes, path)
	mutationRoutesMu.Unlock()
	mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		// Auth only enforces on mutation methods. Non-mutation methods (e.g.,
		// GET on a POST-only route) fall through to the handler so it can
		// return 405 Method Not Allowed via requireMethod. This preserves the
		// pre-helper behavioral contract verified by
		// TestDaemonJobsCancelStaticRouteUsesMutationPolicy.
		p := policy()
		methods := p.AllowedMethods
		if len(methods) == 0 {
			methods = []string{http.MethodPost, http.MethodPatch, http.MethodDelete}
		}
		if !stringInSet(r.Method, methods) {
			handler(w, r)
			return
		}
		decision, err := AuthorizeMutationDecision(r, p)
		if err != nil {
			writeMutationForbidden(w, err)
			return
		}
		ctx := context.WithValue(r.Context(), mutationDecisionKey{}, decision)
		handler(w, r.WithContext(ctx))
	})
}

// registerReadOnlyRoute registers a route that intentionally does not require
// mutation auth. It exists so all mux.HandleFunc call sites in the daemon
// package live in auth.go (the build-time guard rejects HandleFunc anywhere
// else). Read-only by contract: handlers MUST NOT mutate ledger state.
func registerReadOnlyRoute(mux *http.ServeMux, path string, handler http.HandlerFunc) {
	mux.HandleFunc(path, handler)
}

func writeMutationForbidden(w http.ResponseWriter, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}

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
		"/v1/jobs/*/cancel":             MutationCapabilityCancelJob,
		"/openclaw/v1/triggers/jobs":    MutationCapabilityOpenClawTrigger,
		"/v1/openclaw/triggers/jobs":    MutationCapabilityOpenClawTrigger,
		"/v1/openclaw/v1/triggers/jobs": MutationCapabilityOpenClawTrigger,
		// Schedule routes (soc-8inr.5) require admin capability — schedules are
		// privileged operator surface, not per-job submission.
		"/v1/schedules":   MutationCapabilityAdmin,
		"/v1/schedules/*": MutationCapabilityAdmin,
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
	policy = applyMutationPolicyDefaults(policy)
	tokens := normalizedMutationTokens(policy)
	if len(tokens) == 0 {
		return deny("mutation token not configured")
	}
	if reason, ok := checkMutationRequestScope(r, policy); !ok {
		return deny(reason)
	}
	required := mutationCapabilityForPath(r.URL.Path, policy.PathCapabilities)
	if required == "" {
		required = MutationCapabilityAdmin
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

func applyMutationPolicyDefaults(policy MutationPolicy) MutationPolicy {
	if strings.TrimSpace(policy.TokenHeader) == "" {
		policy.TokenHeader = DefaultMutationTokenHeader
	}
	if len(policy.AllowedMethods) == 0 {
		policy.AllowedMethods = []string{http.MethodPost, http.MethodPatch, http.MethodDelete}
	}
	if len(policy.PathCapabilities) == 0 {
		policy.PathCapabilities = DefaultMutationPathCapabilities()
	}
	return policy
}

// checkMutationRequestScope validates method/path/remote/origin/fetch-site
// constraints. Returns the deny reason and false on the first failure;
// returns ("", true) when the request is in scope.
func checkMutationRequestScope(r *http.Request, policy MutationPolicy) (string, bool) {
	if !stringInSet(r.Method, policy.AllowedMethods) {
		return "method outside mutation scope", false
	}
	if len(policy.AllowedPaths) > 0 && !mutationPathInSet(r.URL.Path, policy.AllowedPaths) {
		return "path outside mutation scope", false
	}
	if policy.RequireLocalRemote && r.RemoteAddr != "" && !isLoopbackRemote(r.RemoteAddr) {
		return "remote address is not local", false
	}
	if origin := strings.TrimSpace(r.Header.Get("Origin")); origin != "" && !stringInSet(origin, policy.AllowedOrigins) {
		return "browser origin is not trusted", false
	}
	if fetchSite := strings.ToLower(strings.TrimSpace(r.Header.Get("Sec-Fetch-Site"))); fetchSite == "cross-site" {
		return "browser fetch site is not trusted", false
	}
	return "", true
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

// constantTimeTokenEqual reports whether got equals want using a timing-safe
// comparison for the non-empty case. The empty-input short-circuit is a
// deliberate design choice, not a timing leak worth fixing:
//
//   - When got=="" (the client sent no mutation token header / Bearer auth),
//     EvaluateMutationPolicy already returns a deterministic "mutation token
//     mismatch" deny reason. The "no token presented" signal is observable
//     through the 403 response itself, so leaking it via timing reveals
//     nothing the caller doesn't already know.
//   - When want=="", the token would never have been registered in the first
//     place — LoadMutationTokenFile, LoadMutationTokensFile, and
//     validateMutationToken all reject empty tokens at load time. The check
//     here is defense-in-depth against a future caller mis-constructing a
//     MutationToken{Token: ""} literal.
//   - For the non-empty path that actually authenticates real requests,
//     subtle.ConstantTimeCompare provides timing-safe comparison for
//     equal-length inputs. Length differences are NOT timing-safe, but mutation
//     tokens are operator-issued opaque secrets (no fixed length contract is
//     promised to clients), so a length-based side channel does not narrow
//     the search space meaningfully.
//
// The pad-then-compare / hash-then-compare pattern would only matter if the
// daemon accepted variable-length tokens that needed timing-safe length-
// mismatch comparison against a known target — which is not this surface.
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
	// Legacy tokens (single-token mode, local-only by construction) carry full
	// admin authority. This includes the schedule-management surface added in
	// soc-8inr.5; scoped tokens that need only submit/cancel must be declared
	// explicitly via the JSON token-file format.
	return []MutationCapability{
		MutationCapabilitySubmitJob,
		MutationCapabilityCancelJob,
		MutationCapabilityOpenClawTrigger,
		MutationCapabilityAdmin,
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

// mutationPathInSet matches r.URL.Path against a list of allowed patterns.
// Patterns ending in "/*" match any path under the prefix segment (e.g.,
// "/v1/schedules/*" matches "/v1/schedules/foo" and "/v1/schedules/foo/bar"
// but NOT "/v1/schedules" itself; that requires an exact-match entry). A "*"
// path segment matches exactly one path segment, e.g. "/v1/jobs/*/cancel".
func mutationPathInSet(path string, allowed []string) bool {
	for _, pattern := range allowed {
		if mutationPathMatches(path, pattern) {
			return true
		}
	}
	return false
}

func mutationPathMatches(path, pattern string) bool {
	if pattern == path {
		return true
	}
	if strings.HasSuffix(pattern, "/*") {
		prefix := strings.TrimSuffix(pattern, "/*")
		return strings.HasPrefix(path, prefix+"/")
	}
	if !strings.Contains(pattern, "*") {
		return false
	}
	pathParts := strings.Split(strings.Trim(path, "/"), "/")
	patternParts := strings.Split(strings.Trim(pattern, "/"), "/")
	if len(pathParts) != len(patternParts) {
		return false
	}
	for i, patternPart := range patternParts {
		if patternPart == "*" {
			continue
		}
		if patternPart != pathParts[i] {
			return false
		}
	}
	return true
}

// mutationCapabilityForPath looks up the required capability for r.URL.Path,
// honoring wildcard entries in PathCapabilities the same way
// mutationPathInSet does.
func mutationCapabilityForPath(path string, capabilities map[string]MutationCapability) MutationCapability {
	if cap, ok := capabilities[path]; ok {
		return cap
	}
	for pattern, cap := range capabilities {
		if mutationPathMatches(path, pattern) {
			return cap
		}
	}
	return ""
}

func deny(reason string) MutationDecision {
	return MutationDecision{Allowed: false, Reason: reason}
}
