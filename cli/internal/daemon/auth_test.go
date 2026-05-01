package daemon

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAuthRequiresMutationTokenHeader(t *testing.T) {
	policy := DefaultMutationPolicy("secret-token", []string{"/v1/jobs"})
	req := mutationRequest(http.MethodPost, "/v1/jobs", "127.0.0.1:51111")
	if err := AuthorizeMutation(req, policy); !errors.Is(err, ErrMutationDenied) {
		t.Fatalf("missing token error = %v, want ErrMutationDenied", err)
	}

	req = mutationRequest(http.MethodPost, "/v1/jobs", "127.0.0.1:51111")
	req.Header.Set(DefaultMutationTokenHeader, "secret-token")
	if err := AuthorizeMutation(req, policy); err != nil {
		t.Fatalf("authorize token header: %v", err)
	}

	req = mutationRequest(http.MethodPost, "/v1/jobs", "127.0.0.1:51111")
	req.Header.Set("Authorization", "Bearer secret-token")
	if err := AuthorizeMutation(req, policy); err != nil {
		t.Fatalf("authorize bearer token: %v", err)
	}
}

func TestThreatModelRejectsBrowserOriginStyleAbuse(t *testing.T) {
	policy := DefaultMutationPolicy("secret-token", []string{"/v1/jobs"})
	req := mutationRequest(http.MethodPost, "/v1/jobs", "127.0.0.1:51111")
	req.Header.Set(DefaultMutationTokenHeader, "secret-token")
	req.Header.Set("Origin", "https://evil.example")
	if err := AuthorizeMutation(req, policy); !errors.Is(err, ErrMutationDenied) {
		t.Fatalf("untrusted origin error = %v, want ErrMutationDenied", err)
	}

	req = mutationRequest(http.MethodPost, "/v1/jobs", "127.0.0.1:51111")
	req.Header.Set(DefaultMutationTokenHeader, "secret-token")
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	if err := AuthorizeMutation(req, policy); !errors.Is(err, ErrMutationDenied) {
		t.Fatalf("cross-site fetch error = %v, want ErrMutationDenied", err)
	}

	policy.AllowedOrigins = []string{"http://127.0.0.1:8787"}
	req = mutationRequest(http.MethodPost, "/v1/jobs", "127.0.0.1:51111")
	req.Header.Set(DefaultMutationTokenHeader, "secret-token")
	req.Header.Set("Origin", "http://127.0.0.1:8787")
	if err := AuthorizeMutation(req, policy); err != nil {
		t.Fatalf("trusted local origin with token rejected: %v", err)
	}
}

func TestMutationPolicyRejectsOutOfScopeMutation(t *testing.T) {
	policy := DefaultMutationPolicy("secret-token", []string{"/v1/jobs"})
	req := mutationRequest(http.MethodPost, "/v1/admin/rewrite", "127.0.0.1:51111")
	req.Header.Set(DefaultMutationTokenHeader, "secret-token")
	if err := AuthorizeMutation(req, policy); !errors.Is(err, ErrMutationDenied) {
		t.Fatalf("out-of-scope path error = %v, want ErrMutationDenied", err)
	}

	req = mutationRequest(http.MethodGet, "/v1/jobs", "127.0.0.1:51111")
	req.Header.Set(DefaultMutationTokenHeader, "secret-token")
	if err := AuthorizeMutation(req, policy); !errors.Is(err, ErrMutationDenied) {
		t.Fatalf("out-of-scope method error = %v, want ErrMutationDenied", err)
	}

	req = mutationRequest(http.MethodPost, "/v1/jobs", "192.168.1.10:51111")
	req.Header.Set(DefaultMutationTokenHeader, "secret-token")
	if err := AuthorizeMutation(req, policy); !errors.Is(err, ErrMutationDenied) {
		t.Fatalf("external remote error = %v, want ErrMutationDenied", err)
	}
}

func TestScopedMutationPolicyAllowsSubmitAndRejectsCancel(t *testing.T) {
	policy := MutationPolicy{
		Tokens: []MutationToken{{
			Name:         "phone-readonly-submit",
			Token:        "phone-token",
			Capabilities: []MutationCapability{MutationCapabilitySubmitJob, MutationCapabilityOpenClawTrigger},
		}},
		AllowedPaths:       []string{"/v1/jobs", "/v1/jobs/cancel"},
		AllowedMethods:     []string{http.MethodPost},
		PathCapabilities:   DefaultMutationPathCapabilities(),
		RequireLocalRemote: true,
	}
	req := mutationRequest(http.MethodPost, "/v1/jobs", "127.0.0.1:51111")
	req.Header.Set(DefaultMutationTokenHeader, "phone-token")
	decision, err := AuthorizeMutationDecision(req, policy)
	if err != nil {
		t.Fatalf("submit token rejected: %v", err)
	}
	if decision.TokenName != "phone-readonly-submit" || decision.RequiredCapability != MutationCapabilitySubmitJob {
		t.Fatalf("decision = %#v, want phone submit", decision)
	}
	req = mutationRequest(http.MethodPost, "/v1/jobs/cancel", "127.0.0.1:51111")
	req.Header.Set(DefaultMutationTokenHeader, "phone-token")
	if err := AuthorizeMutation(req, policy); !errors.Is(err, ErrMutationDenied) {
		t.Fatalf("phone cancel error = %v, want ErrMutationDenied", err)
	}
}

func TestScopedMutationPolicyMacExecutorCanCancel(t *testing.T) {
	policy := MutationPolicy{
		Tokens: []MutationToken{{
			Name:         "mac-executor",
			Token:        "mac-token",
			Capabilities: []MutationCapability{MutationCapabilitySubmitJob, MutationCapabilityCancelJob},
		}},
		AllowedPaths:       []string{"/v1/jobs/cancel"},
		AllowedMethods:     []string{http.MethodPost},
		PathCapabilities:   DefaultMutationPathCapabilities(),
		RequireLocalRemote: true,
	}
	req := mutationRequest(http.MethodPost, "/v1/jobs/cancel", "127.0.0.1:51111")
	req.Header.Set(DefaultMutationTokenHeader, "mac-token")
	if err := AuthorizeMutation(req, policy); err != nil {
		t.Fatalf("mac cancel rejected: %v", err)
	}
}

func TestScopedMutationPolicyBushidoAdminLocalOnly(t *testing.T) {
	policy := MutationPolicy{
		Tokens: []MutationToken{{
			Name:         "bushido-admin",
			Token:        "admin-token",
			Capabilities: []MutationCapability{MutationCapabilityAdmin},
			LocalOnly:    true,
		}},
		AllowedPaths:       []string{"/v1/jobs/cancel"},
		AllowedMethods:     []string{http.MethodPost},
		PathCapabilities:   DefaultMutationPathCapabilities(),
		RequireLocalRemote: false,
	}
	req := mutationRequest(http.MethodPost, "/v1/jobs/cancel", "192.168.1.10:51111")
	req.Header.Set(DefaultMutationTokenHeader, "admin-token")
	if err := AuthorizeMutation(req, policy); !errors.Is(err, ErrMutationDenied) {
		t.Fatalf("remote admin error = %v, want ErrMutationDenied", err)
	}
	req = mutationRequest(http.MethodPost, "/v1/jobs/cancel", "127.0.0.1:51111")
	req.Header.Set(DefaultMutationTokenHeader, "admin-token")
	if err := AuthorizeMutation(req, policy); err != nil {
		t.Fatalf("local admin rejected: %v", err)
	}
}

func TestLocalhostBindAddressValidation(t *testing.T) {
	for _, addr := range []string{"127.0.0.1:0", "localhost:8787", "[::1]:8787", "::1"} {
		if err := ValidateLocalBindAddress(addr); err != nil {
			t.Fatalf("loopback bind %q rejected: %v", addr, err)
		}
	}
	for _, addr := range []string{"", ":8787", "0.0.0.0:8787", "192.168.1.10:8787", "[2001:db8::1]:8787"} {
		if err := ValidateLocalBindAddress(addr); !errors.Is(err, ErrUnsafeBindAddress) {
			t.Fatalf("unsafe bind %q error = %v, want ErrUnsafeBindAddress", addr, err)
		}
	}
}

func TestAuthTokenFilePermissions(t *testing.T) {
	dir := t.TempDir()
	secure := filepath.Join(dir, "token")
	if err := os.WriteFile(secure, []byte("secret-token\n"), 0600); err != nil {
		t.Fatalf("write secure token: %v", err)
	}
	token, err := LoadMutationTokenFile(secure)
	if err != nil {
		t.Fatalf("load secure token: %v", err)
	}
	if token != "secret-token" {
		t.Fatalf("token = %q, want secret-token", token)
	}

	insecure := filepath.Join(dir, "token-world-readable")
	if err := os.WriteFile(insecure, []byte("secret-token\n"), 0644); err != nil {
		t.Fatalf("write insecure token: %v", err)
	}
	if err := os.Chmod(insecure, 0644); err != nil {
		t.Fatalf("chmod insecure token: %v", err)
	}
	if _, err := LoadMutationTokenFile(insecure); !errors.Is(err, ErrUnsafeTokenFileMode) {
		t.Fatalf("load insecure token error = %v, want ErrUnsafeTokenFileMode", err)
	}
}

func TestLoadMutationTokensFileSupportsLegacyAndScopedJSON(t *testing.T) {
	dir := t.TempDir()
	legacy := filepath.Join(dir, "legacy-token")
	if err := os.WriteFile(legacy, []byte("legacy-token\n"), 0o600); err != nil {
		t.Fatalf("write legacy token: %v", err)
	}
	tokens, err := LoadMutationTokensFile(legacy)
	if err != nil {
		t.Fatalf("load legacy token: %v", err)
	}
	if len(tokens) != 1 || tokens[0].Name != "legacy" || tokens[0].Token != "legacy-token" || !tokens[0].LocalOnly {
		t.Fatalf("legacy tokens = %#v", tokens)
	}
	scoped := filepath.Join(dir, "scoped-token")
	payload := `{"tokens":[{"name":"phone-readonly-submit","token":"phone-token","capabilities":["submit_job","openclaw_trigger"]},{"name":"bushido-admin","token":"admin-token","capabilities":["admin"],"local_only":true}]}`
	if err := os.WriteFile(scoped, []byte(payload), 0o600); err != nil {
		t.Fatalf("write scoped token: %v", err)
	}
	tokens, err = LoadMutationTokensFile(scoped)
	if err != nil {
		t.Fatalf("load scoped token: %v", err)
	}
	if len(tokens) != 2 || tokens[0].Name != "phone-readonly-submit" || tokens[1].Name != "bushido-admin" || !tokens[1].LocalOnly {
		t.Fatalf("scoped tokens = %#v", tokens)
	}
}

func mutationRequest(method, target, remoteAddr string) *http.Request {
	req := httptest.NewRequest(method, target, nil)
	req.RemoteAddr = remoteAddr
	return req
}

// TestRegisterMutationRoute_AppendsAndWraps verifies the helper:
//  1. tracks the registered path in the mutationRoutes slice (so the parity
//     test can audit coverage), and
//  2. wraps the handler in mutation-auth enforcement (no token = 403, valid
//     token = handler runs and decision is in context).
//
// PER PRE-MORTEM AMENDMENT A2 (soc-8inr.5): registerMutationRoute is the
// single registration choke-point. This test pins the contract.
func TestRegisterMutationRoute_AppendsAndWraps(t *testing.T) {
	resetMutationRoutesForTest()
	t.Cleanup(resetMutationRoutesForTest)

	mux := http.NewServeMux()
	policy := MutationPolicy{
		Token:        "secret-token",
		TokenHeader:  DefaultMutationTokenHeader,
		AllowedPaths: []string{"/v1/test"},
		AllowedMethods: []string{
			http.MethodPost,
		},
		PathCapabilities: map[string]MutationCapability{
			"/v1/test": MutationCapabilityAdmin,
		},
		RequireLocalRemote: true,
	}
	handlerCalls := 0
	var seenDecision MutationDecision
	registerMutationRoute(mux, "/v1/test", func() MutationPolicy { return policy }, func(w http.ResponseWriter, r *http.Request) {
		handlerCalls++
		if d, ok := MutationDecisionFromContext(r.Context()); ok {
			seenDecision = d
		}
		w.WriteHeader(http.StatusAccepted)
	})

	// 1. Path appended to tracked slice.
	tracked := MutationRoutesForTest()
	found := false
	for _, p := range tracked {
		if p == "/v1/test" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("registered path not in MutationRoutesForTest(): %v", tracked)
	}

	// 2a. POST without token → 403.
	req := httptest.NewRequest(http.MethodPost, "/v1/test", nil)
	req.RemoteAddr = "127.0.0.1:51111"
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("missing token status = %d body=%s, want 403", rec.Code, rec.Body.String())
	}
	if handlerCalls != 0 {
		t.Fatalf("handler invoked despite missing token: %d calls", handlerCalls)
	}

	// 2b. POST with valid token → handler runs, decision in context.
	req = httptest.NewRequest(http.MethodPost, "/v1/test", nil)
	req.RemoteAddr = "127.0.0.1:51111"
	req.Header.Set(DefaultMutationTokenHeader, "secret-token")
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("authorized status = %d body=%s, want 202", rec.Code, rec.Body.String())
	}
	if handlerCalls != 1 {
		t.Fatalf("handler call count = %d, want 1", handlerCalls)
	}
	if !seenDecision.Allowed || seenDecision.RequiredCapability != MutationCapabilityAdmin {
		t.Fatalf("decision passed to handler = %#v, want allowed admin", seenDecision)
	}
}

// TestMutation_AllRegisteredRoutesEnforcePolicy is the parity test required
// by amendment A2: every path in MutationRoutesForTest() must reject an
// unauthenticated POST with 403. This catches bypass loopholes where a route
// is registered through the helper but accidentally exposes a code path that
// runs before auth.
func TestMutation_AllRegisteredRoutesEnforcePolicy(t *testing.T) {
	resetMutationRoutesForTest()
	t.Cleanup(resetMutationRoutesForTest)

	store := NewStore(t.TempDir())
	now := time.Now().UTC()
	router := NewDaemonRouter(store, ServerOptions{
		Now:            func() time.Time { return now },
		MutationPolicy: DefaultMutationPolicy("secret-token", nil),
	})
	tracked := MutationRoutesForTest()
	if len(tracked) == 0 {
		t.Fatal("no mutation routes registered; expected NewDaemonRouter to populate the tracked list")
	}
	for _, pattern := range tracked {
		// mux.HandleFunc supports method-prefixed patterns ("POST /v1/foo").
		// Pull the URL path back out so the parity probe hits the same path.
		method := http.MethodPost
		path := pattern
		if idx := strings.IndexByte(pattern, ' '); idx >= 0 {
			method = pattern[:idx]
			path = pattern[idx+1:]
		}
		// Replace mux pattern wildcards ("{name}") with a literal probe value.
		path = strings.ReplaceAll(path, "{name}", "probe-name")

		req := httptest.NewRequest(method, path, nil)
		req.RemoteAddr = "127.0.0.1:51111"
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Errorf("route %q (method %s) without token returned %d (body=%s), want 403",
				pattern, method, rec.Code, rec.Body.String())
		}
	}
}

// TestMutation_GetSchedulesIsReadOnly asserts GET /v1/schedules bypasses
// mutation auth — it is intentionally registered via registerReadOnlyRoute.
func TestMutation_GetSchedulesIsReadOnly(t *testing.T) {
	store := NewStore(t.TempDir())
	now := time.Now().UTC()
	router := NewDaemonRouter(store, ServerOptions{
		Now:            func() time.Time { return now },
		MutationPolicy: DefaultMutationPolicy("secret-token", nil),
	})
	req := httptest.NewRequest(http.MethodGet, "/v1/schedules", nil)
	req.RemoteAddr = "127.0.0.1:51111"
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /v1/schedules without token status = %d body=%s, want 200 (read-only)", rec.Code, rec.Body.String())
	}
	// GET path must NOT be in the tracked mutation slice (it's read-only).
	for _, pattern := range MutationRoutesForTest() {
		if pattern == "GET /v1/schedules" || pattern == "/v1/schedules" {
			t.Fatalf("GET /v1/schedules accidentally registered as a mutation route: %v", MutationRoutesForTest())
		}
	}
}
