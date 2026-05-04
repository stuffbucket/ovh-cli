package auth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestNewPKCE_ChallengeIsS256OfVerifier(t *testing.T) {
	v, c := NewPKCE()
	if len(v) < 43 {
		t.Errorf("verifier length=%d (RFC 7636 minimum is 43)", len(v))
	}
	if v == c {
		t.Error("verifier == challenge (S256 should differ)")
	}
	sum := sha256.Sum256([]byte(v))
	want := base64.RawURLEncoding.EncodeToString(sum[:])
	if c != want {
		t.Errorf("challenge=%q want %q (S256(verifier))", c, want)
	}
}

func TestNewPKCE_DistinctVerifiers(t *testing.T) {
	a, _ := NewPKCE()
	b, _ := NewPKCE()
	if a == b {
		t.Error("two NewPKCE calls returned identical verifier (RNG is broken)")
	}
}

func TestRandomState_Distinct(t *testing.T) {
	a := RandomState()
	b := RandomState()
	if a == b {
		t.Error("two RandomState calls returned identical token")
	}
	if len(a) < 32 {
		t.Errorf("state too short: %d", len(a))
	}
}

func TestLoopbackServer_HappyPath(t *testing.T) {
	srv, err := NewLoopbackServer()
	if err != nil {
		t.Fatalf("NewLoopbackServer: %v", err)
	}
	defer func() { _ = srv.Close() }()

	redirectURI := srv.RedirectURI()
	if !strings.HasPrefix(redirectURI, "http://127.0.0.1:") {
		t.Errorf("RedirectURI=%q", redirectURI)
	}
	if !strings.HasSuffix(redirectURI, "/callback") {
		t.Errorf("RedirectURI=%q (no /callback suffix)", redirectURI)
	}

	// Fire callback from a goroutine after a short delay.
	go func() {
		time.Sleep(10 * time.Millisecond)
		fireCallback(t, redirectURI+"?code=CODE&state=STATE")
	}()

	code, state, err := srv.Wait(context.Background(), 5*time.Second)
	if err != nil {
		t.Fatalf("Wait: %v", err)
	}
	if code != "CODE" {
		t.Errorf("code=%q want CODE", code)
	}
	if state != "STATE" {
		t.Errorf("state=%q want STATE", state)
	}
}

func TestLoopbackServer_Timeout(t *testing.T) {
	srv, err := NewLoopbackServer()
	if err != nil {
		t.Fatalf("NewLoopbackServer: %v", err)
	}
	defer func() { _ = srv.Close() }()

	_, _, err = srv.Wait(context.Background(), 50*time.Millisecond)
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("got %v; want timeout", err)
	}
}

func TestLoopbackServer_CtxCancel(t *testing.T) {
	srv, err := NewLoopbackServer()
	if err != nil {
		t.Fatalf("NewLoopbackServer: %v", err)
	}
	defer func() { _ = srv.Close() }()

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()
	_, _, err = srv.Wait(ctx, 5*time.Second)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v; want context.Canceled", err)
	}
}

func TestLoopbackServer_OAuthErrorParam(t *testing.T) {
	srv, err := NewLoopbackServer()
	if err != nil {
		t.Fatalf("NewLoopbackServer: %v", err)
	}
	defer func() { _ = srv.Close() }()

	go func() {
		time.Sleep(10 * time.Millisecond)
		fireCallback(t, srv.RedirectURI()+"?error=access_denied&error_description=user_denied")
	}()
	_, _, err = srv.Wait(context.Background(), 5*time.Second)
	if err == nil || !strings.Contains(err.Error(), "access_denied") {
		t.Fatalf("got %v; want access_denied error", err)
	}
}

func TestLoopbackServer_CloseIdempotent(t *testing.T) {
	srv, err := NewLoopbackServer()
	if err != nil {
		t.Fatalf("NewLoopbackServer: %v", err)
	}
	if err := srv.Close(); err != nil {
		t.Errorf("first Close: %v", err)
	}
	if err := srv.Close(); err != nil {
		t.Errorf("second Close: %v (should be idempotent)", err)
	}
}

func TestResolveTokenEndpoint_NoIssuer(t *testing.T) {
	region := Region{ID: "test"} // empty OAuth2Issuer
	_, err := resolveTokenEndpoint(context.Background(), region)
	if err == nil || !strings.Contains(err.Error(), "no OAuth2Issuer") {
		t.Fatalf("got %v; want no-issuer error", err)
	}
}

func TestResolveTokenEndpoint_FallbackToIssuerSlashToken(t *testing.T) {
	// 127.0.0.1:1 (reserved tcpmux port) is unreachable on test hosts —
	// discovery fails fast (connection refused), fallback returns
	// <issuer>/token.
	region := Region{
		ID:                    "test",
		OAuth2Issuer:          "https://127.0.0.1:1",
		ValidationHostPattern: []string{"127.0.0.1:1"},
	}
	got, err := resolveTokenEndpoint(context.Background(), region)
	if err != nil {
		t.Fatalf("resolveTokenEndpoint: %v", err)
	}
	if got != "https://127.0.0.1:1/token" {
		t.Errorf("got %q want https://127.0.0.1:1/token", got)
	}
}

// fireCallback issues a GET to url and discards the response body so
// bodyclose is satisfied. Test-only helper.
func fireCallback(t *testing.T, target string) {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, target, http.NoBody)
	if err != nil {
		t.Logf("NewRequest: %v", err)
		return
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Logf("http.Do: %v", err)
		return
	}
	_ = resp.Body.Close()
}

func TestResolveTokenEndpoint_HostNotInAllowList(t *testing.T) {
	region := Region{
		ID:                    "test",
		OAuth2Issuer:          "https://example.com",
		ValidationHostPattern: []string{"different-host.example"},
	}
	_, err := resolveTokenEndpoint(context.Background(), region)
	if err == nil || !strings.Contains(err.Error(), "host validation") {
		t.Fatalf("got %v; want host-validation error", err)
	}
}
