package auth

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// classicCKServer mocks OVH's /auth/credential POST and /me GET. It tracks
// state so the first N /me calls return 403 (pre-validation) and subsequent
// calls return 200, simulating a user clicking "validate" in the browser.
type classicCKServer struct {
	*httptest.Server
	preValidationCalls int32 // /me calls before "validation" succeeds
	mePreCalls         atomic.Int32
}

func newClassicCKServer(t *testing.T, preValidationCalls int32) *classicCKServer {
	t.Helper()
	cks := &classicCKServer{preValidationCalls: preValidationCalls}
	mux := http.NewServeMux()
	mux.HandleFunc("/auth/credential", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method", http.StatusMethodNotAllowed)
			return
		}
		_ = json.NewEncoder(w).Encode(credentialResponse{
			ValidationURL: cks.URL + "/auth/validate",
			ConsumerKey:   "TEST-CK",
			State:         "pendingValidation",
		})
	})
	// /auth/time is what go-ovh fetches before signing every request.
	// It returns the unix timestamp as a bare number.
	mux.HandleFunc("/auth/time", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("1746374400")) // 2026-05-04T12:00:00Z; arbitrary fixed time
	})
	mux.HandleFunc("/me", func(w http.ResponseWriter, _ *http.Request) {
		n := cks.mePreCalls.Add(1)
		if n <= cks.preValidationCalls {
			http.Error(w, `{"errorCode":"NOT_GRANTED_CALL"}`, http.StatusForbidden)
			return
		}
		_ = json.NewEncoder(w).Encode(struct {
			Nichandle string `json:"nichandle"`
		}{Nichandle: "ab12345-ovh"})
	})
	mux.HandleFunc("/auth/validate", func(w http.ResponseWriter, _ *http.Request) {
		// User-facing validation page; not exercised by these tests.
		_, _ = w.Write([]byte("OK"))
	})
	cks.Server = httptest.NewTLSServer(mux)
	return cks
}

// trustedClient swaps http.DefaultClient.Transport so tests can call
// httptest.NewTLSServer without cert errors. Restored on test cleanup.
func trustedClient(t *testing.T, srv *httptest.Server) {
	t.Helper()
	orig := http.DefaultClient.Transport
	t.Cleanup(func() { http.DefaultClient.Transport = orig })
	pool := x509.NewCertPool()
	pool.AddCert(srv.Certificate())
	http.DefaultClient.Transport = &http.Transport{
		TLSClientConfig: &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12},
	}
}

// stubAskAKAS returns a callback that always replies with the given AK+AS.
func stubAskAKAS(ak, as string) AskAKAS {
	return func(_ context.Context) (string, string, error) { return ak, as, nil }
}

// regionFor returns a Region wired to srv's URL.
func regionFor(srv *httptest.Server) Region {
	u, _ := url.Parse(srv.URL)
	return Region{
		ID:                    "test",
		EndpointURL:           srv.URL,
		PortalCreateAppURL:    srv.URL + "/createApp",
		ValidationHostPattern: []string{u.Host},
	}
}

func TestRunConsumerKeyFlow_HappyPath(t *testing.T) {
	// browser.OpenURL is OS-dependent; we can't avoid it triggering, but
	// it'll fail silently on a CI host with no GUI. The test cares only
	// about the network flow, not the browser.
	cks := newClassicCKServer(t, 1) // /me returns 403 once, then 200
	defer cks.Close()
	trustedClient(t, cks.Server)

	region := regionFor(cks.Server)
	creds, err := RunConsumerKeyFlow(context.Background(), region, "default", stubAskAKAS("AK", "AS"))
	if err != nil {
		t.Fatalf("RunConsumerKeyFlow: %v", err)
	}
	if creds.Method != MethodConsumerKey {
		t.Errorf("Method=%v want consumer_key", creds.Method)
	}
	if creds.ApplicationKey != "AK" || creds.ApplicationSecret != "AS" || creds.ConsumerKey != "TEST-CK" {
		t.Errorf("creds=%+v", creds)
	}
	if cks.mePreCalls.Load() < 2 {
		t.Errorf("/me poll should have made >= 2 calls; got %d", cks.mePreCalls.Load())
	}
}

func TestRunConsumerKeyFlow_RejectsEmptyCreds(t *testing.T) {
	region := Region{ID: "test", EndpointURL: "https://127.0.0.1:1", PortalCreateAppURL: "https://127.0.0.1:1/", ValidationHostPattern: []string{"127.0.0.1:1"}}
	// askAKAS returns empty both times -> error after the second prompt.
	calls := 0
	ask := func(_ context.Context) (string, string, error) {
		calls++
		return "", "", nil
	}
	_, err := RunConsumerKeyFlow(context.Background(), region, "default", ask)
	if err == nil || !strings.Contains(err.Error(), "required") {
		t.Fatalf("got %v; want both-required error", err)
	}
	if calls != 2 {
		t.Errorf("askAKAS called %d times; expected 2 (initial + post-portal retry)", calls)
	}
}

func TestRunConsumerKeyFlow_PanicsOnNilCtx(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on nil ctx")
		}
	}()
	region := Region{ID: "test"}
	//nolint:staticcheck // intentionally passing nil ctx
	_, _ = RunConsumerKeyFlow(nil, region, "default", stubAskAKAS("a", "b"))
}

func TestRunConsumerKeyFlow_PanicsOnEmptyRegion(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on empty region")
		}
	}()
	_, _ = RunConsumerKeyFlow(context.Background(), Region{}, "default", stubAskAKAS("a", "b"))
}

func TestRunConsumerKeyFlow_AskAKASRequired(t *testing.T) {
	region := Region{ID: "test"}
	_, err := RunConsumerKeyFlow(context.Background(), region, "default", nil)
	if err == nil || !strings.Contains(err.Error(), "askAKAS") {
		t.Fatalf("got %v; want askAKAS-required error", err)
	}
}

func TestRunConsumerKeyFlow_PostFailureSurfaces(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/auth/credential", func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"error":"bad ak"}`, http.StatusForbidden)
	})
	srv := httptest.NewTLSServer(mux)
	defer srv.Close()
	trustedClient(t, srv)

	region := regionFor(srv)
	_, err := RunConsumerKeyFlow(context.Background(), region, "default", stubAskAKAS("AK", "AS"))
	if err == nil || !strings.Contains(err.Error(), "HTTP 403") {
		t.Fatalf("got %v; want HTTP 403 surfaced", err)
	}
}

func TestRunConsumerKeyFlow_PollTimeout(t *testing.T) {
	// preValidationCalls = high so /me never returns 200 within the test.
	cks := newClassicCKServer(t, 1000)
	defer cks.Close()
	trustedClient(t, cks.Server)

	region := regionFor(cks.Server)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := RunConsumerKeyFlow(ctx, region, "default", stubAskAKAS("AK", "AS"))
	if err == nil {
		t.Fatal("expected timeout / ctx error")
	}
	if !errors.Is(err, context.DeadlineExceeded) && !strings.Contains(err.Error(), "timed out") {
		t.Errorf("got %v; want ctx deadline or timeout error", err)
	}
}
