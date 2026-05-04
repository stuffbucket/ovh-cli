package schema

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
)

// maxConcurrentServiceFetches caps the per-region fan-out (PRD-05 §Fetch).
const maxConcurrentServiceFetches = 8

// maxResponseBytes caps any single OVH response we'll read. PRD-05 §Threat
// model: response size capped via http.MaxBytesReader.
const maxResponseBytes = 32 * 1024 * 1024 // 32 MiB

// fetchResult is the outcome of fetchAPISpace. When Modified is false, the
// upstream returned 304 Not Modified and Space is nil — refresh should leave
// apispace.json unchanged and only refresh apispace.meta.json's FetchedAt.
type fetchResult struct {
	Space    *APISpace
	ETag     string
	Modified bool
}

// fetchAPISpace walks /1.0/?format=json and fetches each service description
// concurrently. Per-service failures are recorded as Service.Error and do not
// fail the whole walk. allowedHosts MUST be non-empty (caller validates).
//
// prevETag, when non-empty, is sent as If-None-Match on the index request.
// On 304 the per-service walk is skipped entirely (PRD-05 §Fetch: "on 304,
// skip merge and bump fetched_at").
//
// Implementation note: we use raw net/http rather than go-ovh's Client — the
// /1.0/ surface is anonymous (no signing) and go-ovh's Client mutates state
// inside NewRequest, racing with errgroup fan-out. Authenticated callers in
// phase 2b's internal/client still use go-ovh.
//
// transport=nil uses http.DefaultTransport (production); tests pass an
// httptest.NewTLSServer's transport to trust the self-signed cert.
func fetchAPISpace(ctx context.Context, region, endpoint string, allowedHosts []string, transport http.RoundTripper, prevETag string) (fetchResult, error) {
	if err := assertHostAllowed(endpoint, allowedHosts); err != nil {
		return fetchResult{}, err
	}

	// CheckRedirect refuses redirects: a malicious response could otherwise
	// pivot the anonymous discovery client to an attacker-controlled host
	// even though the original endpoint passed assertHostAllowed.
	httpClient := &http.Client{
		Transport:     transport,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}

	var index struct {
		APIs []struct {
			Path string `json:"path"`
		} `json:"apis"`
	}
	etag, modified, err := getConditional(ctx, httpClient, endpoint+"/?format=json", prevETag, &index)
	if err != nil {
		return fetchResult{}, fmt.Errorf("schema: list services: %w", err)
	}
	if !modified {
		return fetchResult{ETag: etag, Modified: false}, nil
	}

	services := make(map[string]Service, len(index.APIs))
	var mu sync.Mutex

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(maxConcurrentServiceFetches)
	for _, entry := range index.APIs {
		p := entry.Path
		g.Go(func() error {
			var svc Service
			ferr := getJSON(gctx, httpClient, endpoint+p+"?format=json", &svc)
			mu.Lock()
			defer mu.Unlock()
			if ferr != nil {
				services[p] = Service{Error: ferr.Error()}
				return nil // PRD-05 §Fetch: per-service failures non-fatal
			}
			services[p] = svc
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return fetchResult{}, err
	}

	return fetchResult{
		Space: &APISpace{
			Version:   SpecVersion,
			Region:    region,
			FetchedAt: time.Now().UTC(),
			ETag:      etag,
			Services:  services,
		},
		ETag:     etag,
		Modified: true,
	}, nil
}

// getConditional issues a GET with optional If-None-Match. Returns the
// response ETag, modified=false on 304 (caller should not touch out), or
// modified=true with body decoded into out.
func getConditional(ctx context.Context, client *http.Client, target, ifNoneMatch string, out any) (string, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, http.NoBody)
	if err != nil {
		return "", false, err
	}
	req.Header.Set("Accept", "application/json")
	if ifNoneMatch != "" {
		req.Header.Set("If-None-Match", ifNoneMatch)
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", false, err
	}
	defer func() { _ = resp.Body.Close() }()

	etag := resp.Header.Get("ETag")
	if resp.StatusCode == http.StatusNotModified {
		// On 304 the upstream may echo the previous ETag or omit it; either
		// is fine — fall back to the value the caller already had.
		if etag == "" {
			etag = ifNoneMatch
		}
		return etag, false, nil
	}
	if resp.StatusCode != http.StatusOK {
		return "", false, fmt.Errorf("HTTP %d %s", resp.StatusCode, resp.Status)
	}
	if err := json.NewDecoder(http.MaxBytesReader(nil, resp.Body, maxResponseBytes)).Decode(out); err != nil {
		return "", false, err
	}
	return etag, true, nil
}

// getJSON does an unconditional anonymous GET and decodes a JSON response
// into out. Used for per-service fetches where we don't track ETags.
func getJSON(ctx context.Context, client *http.Client, target string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, http.NoBody)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d %s", resp.StatusCode, resp.Status)
	}
	return json.NewDecoder(http.MaxBytesReader(nil, resp.Body, maxResponseBytes)).Decode(out)
}

// assertHostAllowed enforces PRD-05 §Threat model: the discovery endpoint
// must be HTTPS and exactly match one of the allowed hosts. Mirrors the
// intent of internal/auth.ValidateHost (PRD-03); duplicated rather than
// imported because Layer A may not depend on internal/auth (PRD-05).
func assertHostAllowed(endpoint string, allowed []string) error {
	u, err := url.Parse(endpoint)
	if err != nil {
		return fmt.Errorf("schema: parse endpoint: %w", err)
	}
	if u.Scheme != "https" {
		return fmt.Errorf("schema: endpoint %q is not https", endpoint)
	}
	for _, h := range allowed {
		if u.Host == h {
			return nil
		}
	}
	return fmt.Errorf("schema: endpoint host %q not in allowedHosts", u.Host)
}
