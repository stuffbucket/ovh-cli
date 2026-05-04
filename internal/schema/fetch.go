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

// fetchAPISpace walks /1.0/?format=json and fetches each service description
// concurrently. Per-service failures are recorded as Service.Error and do not
// fail the whole walk. allowedHosts MUST be non-empty (caller validates).
//
// Implementation note: we use raw net/http rather than go-ovh's Client. The
// /1.0/ discovery surface is anonymous (no signing) and go-ovh's Client
// mutates internal state inside NewRequest, which races with errgroup
// fan-out. Authenticated calls in phase 2b's internal/client still use
// go-ovh — this is a Layer A specific choice.
//
// transport=nil uses http.DefaultTransport (production); tests pass an
// httptest.NewTLSServer's transport to trust the self-signed cert.
func fetchAPISpace(ctx context.Context, region, endpoint string, allowedHosts []string, transport http.RoundTripper) (*APISpace, error) {
	if err := assertHostAllowed(endpoint, allowedHosts); err != nil {
		return nil, err
	}

	// CheckRedirect refuses redirects: a malicious response could otherwise
	// pivot the anonymous discovery client to an attacker-controlled host
	// even though the original endpoint passed assertHostAllowed.
	httpClient := &http.Client{
		Transport:     transport, // nil is fine; net/http uses DefaultTransport
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}

	var index struct {
		APIs []struct {
			Path string `json:"path"`
		} `json:"apis"`
	}
	if err := getJSON(ctx, httpClient, endpoint+"/?format=json", &index); err != nil {
		return nil, fmt.Errorf("schema: list services: %w", err)
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
		return nil, err
	}

	return &APISpace{
		Version:   SpecVersion,
		Region:    region,
		FetchedAt: time.Now().UTC(),
		Services:  services,
	}, nil
}

// getJSON does an anonymous GET and decodes a JSON response into out. Caps
// response size per PRD-05 §Threat model.
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
