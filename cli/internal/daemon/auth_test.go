package daemon

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
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

func mutationRequest(method, target, remoteAddr string) *http.Request {
	req := httptest.NewRequest(method, target, nil)
	req.RemoteAddr = remoteAddr
	return req
}
