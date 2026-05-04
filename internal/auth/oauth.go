// OAuth2 PKCE flow primitives. Public surface defined in PRD-03 §oauth.go.
//
// internal/auth/oauth.go is the only package in the codebase that
// source-imports golang.org/x/oauth2; PRD-08 §Canonical depguard ruleset
// confines it via the internal-auth-allowlist.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/sync/singleflight"
)

// pkceVerifierBytes is the byte length of a PKCE verifier before base64url
// encoding. RFC 7636 requires 43–128 bytes encoded; 32 raw bytes -> 43 chars.
const pkceVerifierBytes = 32

// stateBytes is the byte length of OAuth2 state. 32 bytes -> 43 base64url chars.
const stateBytes = 32

// loopbackTimeoutMax caps the per-Wait timeout regardless of caller intent.
// PRD-03 §Hardening: 5-minute single-shot listener.
const loopbackTimeoutMax = 5 * time.Minute

// NewPKCE returns a fresh verifier (cryptographically-random ~43 base64url
// chars) and its derived S256 challenge per RFC 7636.
func NewPKCE() (verifier, challenge string) {
	b := make([]byte, pkceVerifierBytes)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand.Read never returns an error on supported platforms;
		// panic here mirrors stdlib's contract (see crypto/rand docs).
		panic("auth.NewPKCE: rand.Read failed: " + err.Error())
	}
	verifier = base64.RawURLEncoding.EncodeToString(b)
	sum := sha256.Sum256([]byte(verifier))
	challenge = base64.RawURLEncoding.EncodeToString(sum[:])
	return verifier, challenge
}

// RandomState returns a URL-safe random state token for the OAuth2
// authorize step (CSRF guard).
func RandomState() string {
	b := make([]byte, stateBytes)
	if _, err := rand.Read(b); err != nil {
		panic("auth.RandomState: rand.Read failed: " + err.Error())
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// LoopbackServer captures the OAuth2 authorization-code callback on a
// 127.0.0.1 listener with an OS-assigned port. PRD-03 §oauth.go.
type LoopbackServer struct {
	listener  net.Listener
	server    *http.Server
	addr      string
	resultCh  chan loopbackResult
	closeOnce sync.Once
}

type loopbackResult struct {
	code, state string
	err         error
}

// NewLoopbackServer binds the listener and starts serving. Caller must
// invoke Close (typically via defer) to release the port.
func NewLoopbackServer() (*LoopbackServer, error) {
	var lc net.ListenConfig
	ln, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("auth: bind loopback: %w", err)
	}
	s := &LoopbackServer{
		listener: ln,
		addr:     ln.Addr().String(),
		resultCh: make(chan loopbackResult, 1),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/callback", s.handleCallback)
	s.server = &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() { _ = s.server.Serve(ln) }()
	return s, nil
}

// RedirectURI returns the URL the OAuth2 server should redirect to.
func (s *LoopbackServer) RedirectURI() string {
	return "http://" + s.addr + "/callback"
}

// Wait blocks until the callback arrives, the timeout fires, or ctx is
// cancelled. Single-shot: the second call returns an error.
func (s *LoopbackServer) Wait(ctx context.Context, timeout time.Duration) (code, state string, err error) {
	if timeout > loopbackTimeoutMax || timeout <= 0 {
		timeout = loopbackTimeoutMax
	}
	t := time.NewTimer(timeout)
	defer t.Stop()
	select {
	case r := <-s.resultCh:
		return r.code, r.state, r.err
	case <-ctx.Done():
		return "", "", ctx.Err()
	case <-t.C:
		return "", "", fmt.Errorf("auth: loopback callback timed out after %s", timeout)
	}
}

// Close shuts down the loopback HTTP server and releases the listener.
func (s *LoopbackServer) Close() error {
	var err error
	s.closeOnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		err = s.server.Shutdown(ctx)
	})
	return err
}

func (s *LoopbackServer) handleCallback(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	res := loopbackResult{code: q.Get("code"), state: q.Get("state")}
	if e := q.Get("error"); e != "" {
		res.err = fmt.Errorf("auth: OAuth2 callback error %q: %s", e, q.Get("error_description"))
	} else if res.code == "" {
		res.err = errors.New("auth: OAuth2 callback missing code")
	}
	// Send result; non-blocking on buffered channel.
	select {
	case s.resultCh <- res:
	default:
	}
	// Friendly user-facing response. Pure HTML; no scripts.
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	if res.err != nil {
		_, _ = io.WriteString(w, "<html><body><h1>ovh: authentication failed</h1><p>You may close this tab.</p></body></html>")
		return
	}
	_, _ = io.WriteString(w, "<html><body><h1>ovh: authentication complete</h1><p>You may close this tab.</p></body></html>")
}

// ExchangePKCE swaps an authorization code for OAuth2 tokens. redirectURI
// must match what was sent in the authorize step. Token endpoint resolved
// per PRD-03 §OAuth2 implementation note.
func ExchangePKCE(ctx context.Context, region Region, code, verifier, redirectURI string) (Credentials, error) {
	tokenURL, err := resolveTokenEndpoint(ctx, region)
	if err != nil {
		return Credentials{}, err
	}
	cfg := &oauth2.Config{
		Endpoint:    oauth2.Endpoint{TokenURL: tokenURL},
		RedirectURL: redirectURI,
	}
	tok, err := cfg.Exchange(ctx, code, oauth2.SetAuthURLParam("code_verifier", verifier))
	if err != nil {
		return Credentials{}, fmt.Errorf("auth: token exchange: %w", err)
	}
	return Credentials{
		Region:       region.ID,
		Profile:      "default",
		Method:       MethodOAuth2,
		AccessToken:  tok.AccessToken,
		RefreshToken: tok.RefreshToken,
		Expiry:       tok.Expiry,
	}, nil
}

// refreshGroup coalesces concurrent RefreshOAuth2 calls per (region, profile).
// PRD-03 §RefreshOAuth2 concurrency invariant.
var refreshGroup singleflight.Group

// RefreshOAuth2 exchanges the stored refresh token for a fresh access token.
// Idempotent; safe to call concurrently. PRD-03 §RefreshOAuth2.
func RefreshOAuth2(ctx context.Context, region, profile string) (Credentials, error) {
	if region == "" {
		panic("auth.RefreshOAuth2: region must not be empty (PRD-03 pre-condition)")
	}
	if profile == "" {
		profile = "default"
	}
	key := region + ":" + profile
	v, err, _ := refreshGroup.Do(key, func() (any, error) {
		return refreshOnce(ctx, region, profile)
	})
	if err != nil {
		return Credentials{}, err
	}
	return v.(Credentials), nil
}

func refreshOnce(ctx context.Context, region, profile string) (Credentials, error) {
	r, ok := RegionByID(region)
	if !ok {
		return Credentials{}, fmt.Errorf("auth: unknown region %q", region)
	}
	creds, err := LoadCredentials(ctx, region, profile)
	if err != nil {
		return Credentials{}, err
	}
	if creds.RefreshToken == "" {
		return Credentials{}, ErrCredentialsExpired
	}
	tokenURL, err := resolveTokenEndpoint(ctx, r)
	if err != nil {
		return Credentials{}, err
	}
	cfg := &oauth2.Config{Endpoint: oauth2.Endpoint{TokenURL: tokenURL}}
	src := cfg.TokenSource(ctx, &oauth2.Token{RefreshToken: creds.RefreshToken})
	tok, err := src.Token()
	if err != nil {
		return Credentials{}, fmt.Errorf("auth: oauth2 refresh: %w", err)
	}
	creds.AccessToken = tok.AccessToken
	if tok.RefreshToken != "" {
		creds.RefreshToken = tok.RefreshToken
	}
	creds.Expiry = tok.Expiry
	if err := StoreCredentials(ctx, region, profile, creds); err != nil {
		return Credentials{}, fmt.Errorf("auth: store refreshed creds: %w", err)
	}
	return creds, nil
}

// resolveTokenEndpoint returns the OAuth2 token endpoint URL for region.
// Tries OIDC discovery (<issuer>/.well-known/openid-configuration) first,
// falls back to <issuer>/token. Validates the resulting host against
// region.ValidationHostPattern. PRD-03 §Token endpoint resolution.
func resolveTokenEndpoint(ctx context.Context, region Region) (string, error) {
	if region.OAuth2Issuer == "" {
		return "", fmt.Errorf("auth: region %q has no OAuth2Issuer", region.ID)
	}
	if discovered, ok := tryOIDCDiscovery(ctx, region.OAuth2Issuer); ok {
		if err := ValidateHost(discovered, region.ValidationHostPattern); err != nil {
			return "", fmt.Errorf("auth: discovered token_endpoint %q failed host validation: %w", discovered, err)
		}
		return discovered, nil
	}
	fallback, err := url.JoinPath(region.OAuth2Issuer, "/token")
	if err != nil {
		return "", fmt.Errorf("auth: build token URL: %w", err)
	}
	if err := ValidateHost(fallback, region.ValidationHostPattern); err != nil {
		return "", fmt.Errorf("auth: token endpoint %q failed host validation: %w", fallback, err)
	}
	return fallback, nil
}

// tryOIDCDiscovery fetches <issuer>/.well-known/openid-configuration.
// Returns ("", false) on any failure — caller falls back to <issuer>/token.
func tryOIDCDiscovery(ctx context.Context, issuer string) (string, bool) {
	target, err := url.JoinPath(issuer, "/.well-known/openid-configuration")
	if err != nil {
		return "", false
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, http.NoBody)
	if err != nil {
		return "", false
	}
	cli := &http.Client{Timeout: 5 * time.Second}
	resp, err := cli.Do(req)
	if err != nil {
		return "", false
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", false
	}
	var doc struct {
		TokenEndpoint string `json:"token_endpoint"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&doc); err != nil {
		return "", false
	}
	if doc.TokenEndpoint == "" {
		return "", false
	}
	return doc.TokenEndpoint, true
}
