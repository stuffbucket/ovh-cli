// Classic AK/AS/CK consumer-key validation flow per PRD-03 §Classic CK flow.
package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/cli/browser"
	"github.com/ovh/go-ovh/ovh"
)

// pollInterval is the initial delay between /me poll attempts. Doubles up
// to maxPollInterval; total bounded by the caller's ctx deadline.
const (
	pollInterval       = 2 * time.Second
	maxPollInterval    = 30 * time.Second
	defaultFlowTimeout = 10 * time.Minute
)

// AskAKAS is the prompt callback signature passed to RunConsumerKeyFlow.
// Production (cobra) supplies a huh-backed prompt; tests supply a stub.
type AskAKAS func(ctx context.Context) (ak, as string, err error)

// openBrowser is the browser-open hook. Production: browser.OpenURL.
// Tests swap to a no-op via t.Cleanup (CI runners are headless and
// xdg-open would fail). Package-level mutable var keeps the production
// signature simple at the cost of one shared test seam.
var openBrowser = browser.OpenURL

// RunConsumerKeyFlow drives the classic AK/AS/CK validation flow.
// PRD-03 §flow.go for the full contract.
func RunConsumerKeyFlow(ctx context.Context, region Region, profile string, askAKAS AskAKAS) (Credentials, error) {
	if ctx == nil {
		panic("auth.RunConsumerKeyFlow: ctx must not be nil")
	}
	if region.ID == "" {
		panic("auth.RunConsumerKeyFlow: empty Region (PRD-03 pre-condition)")
	}
	if profile == "" {
		profile = "default"
	}
	if askAKAS == nil {
		return Credentials{}, errors.New("auth: askAKAS callback required")
	}

	ak, as, err := askAKAS(ctx)
	if err != nil {
		return Credentials{}, err
	}
	if ak == "" && as == "" {
		// PRD-03 §flow.go step 1: open portal first so user can mint creds,
		// then re-prompt.
		if err := openBrowser(region.PortalCreateAppURL); err != nil {
			return Credentials{}, fmt.Errorf("auth: open portal: %w", err)
		}
		ak, as, err = askAKAS(ctx)
		if err != nil {
			return Credentials{}, err
		}
	}
	if ak == "" || as == "" {
		return Credentials{}, errors.New("auth: both ApplicationKey and ApplicationSecret are required")
	}

	credResp, err := postAuthCredential(ctx, region, ak, defaultRules())
	if err != nil {
		return Credentials{}, err
	}
	// PRD-03 §Hardening: validate validationUrl host before opening browser.
	if err := ValidateHost(credResp.ValidationURL, region.ValidationHostPattern); err != nil {
		return Credentials{}, fmt.Errorf("auth: validation URL rejected: %w", err)
	}
	if err := openBrowser(credResp.ValidationURL); err != nil {
		return Credentials{}, fmt.Errorf("auth: open validation URL: %w", err)
	}

	if err := pollUntilValidated(ctx, region, ak, as, credResp.ConsumerKey, defaultFlowTimeout); err != nil {
		return Credentials{}, err
	}
	return Credentials{
		Region:            region.ID,
		Profile:           profile,
		Method:            MethodConsumerKey,
		ApplicationKey:    ak,
		ApplicationSecret: as,
		ConsumerKey:       credResp.ConsumerKey,
	}, nil
}

// credentialResponse is the /auth/credential reply shape.
type credentialResponse struct {
	ValidationURL string `json:"validationUrl"`
	ConsumerKey   string `json:"consumerKey"`
	State         string `json:"state"`
}

// rule is one entry in the /auth/credential request's accessRules.
type rule struct {
	Method string `json:"method"`
	Path   string `json:"path"`
}

// defaultRules requests full access (all methods on all paths). Phase 2b
// iter 3 keeps this maximally permissive; later iterations may narrow per
// the user's --scopes flag.
func defaultRules() []rule {
	return []rule{
		{Method: "GET", Path: "/*"},
		{Method: "POST", Path: "/*"},
		{Method: "PUT", Path: "/*"},
		{Method: "DELETE", Path: "/*"},
	}
}

// postAuthCredential POSTs /auth/credential (anonymous; no signing). Uses
// raw net/http rather than go-ovh because go-ovh requires non-empty creds
// at construction and its anonymous POST path mutates internal state under
// concurrent use (we hit this in internal/schema/fetch.go).
func postAuthCredential(ctx context.Context, region Region, appKey string, rules []rule) (*credentialResponse, error) {
	body, err := json.Marshal(struct {
		AccessRules []rule `json:"accessRules"`
	}{AccessRules: rules})
	if err != nil {
		return nil, fmt.Errorf("auth: marshal credential request: %w", err)
	}
	target := strings.TrimRight(region.EndpointURL, "/") + "/auth/credential"
	if err := ValidateHost(target, region.ValidationHostPattern); err != nil {
		return nil, fmt.Errorf("auth: endpoint %q: %w", target, err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Ovh-Application", appKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("auth: post /auth/credential: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<14))
		return nil, fmt.Errorf("auth: /auth/credential HTTP %d: %s", resp.StatusCode, string(b))
	}
	var out credentialResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<16)).Decode(&out); err != nil {
		return nil, fmt.Errorf("auth: decode /auth/credential: %w", err)
	}
	if out.ValidationURL == "" || out.ConsumerKey == "" {
		return nil, errors.New("auth: /auth/credential response missing validationUrl or consumerKey")
	}
	return &out, nil
}

// pollUntilValidated calls /me with the new CK on a backoff schedule until
// it returns 200 (validation succeeded) or ctx/timeout fires. Uses go-ovh
// for the request because /me requires the OVH AK/AS/CK signature; go-ovh's
// signing isn't worth re-implementing.
func pollUntilValidated(ctx context.Context, region Region, ak, as, ck string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	delay := pollInterval

	client, err := ovh.NewClient(region.EndpointURL, ak, as, ck)
	if err != nil {
		return fmt.Errorf("auth: build go-ovh client: %w", err)
	}
	// go-ovh's NewClient creates a fresh *http.Client; replace it with
	// http.DefaultClient so any process-wide transport swap (e.g., test
	// TLS-trust override) is inherited. We add a per-call timeout so a
	// hung /auth/time or /me doesn't block the polling loop's ctx-check.
	timed := *http.DefaultClient
	timed.Timeout = 10 * time.Second
	client.Client = &timed

	for {
		var nichandle struct {
			Nichandle string `json:"nichandle"`
		}
		err := client.Get("/me", &nichandle)
		if err == nil {
			return nil
		}
		// Pre-validation OVH returns 403 "This Credential is not allowed";
		// other errors (network, etc.) also keep polling until deadline.
		if time.Now().After(deadline) {
			return fmt.Errorf("auth: classic CK validation timed out after %s; last error: %w", timeout, err)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
		if delay < maxPollInterval {
			delay *= 2
			if delay > maxPollInterval {
				delay = maxPollInterval
			}
		}
	}
}
