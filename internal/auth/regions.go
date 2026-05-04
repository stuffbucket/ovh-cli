// Package auth provides OVH authentication for Layer B.
//
// PRD-03 §Region Go type — the canonical region registry lives here.
// PRD-06 §Canonical package registry — auth's exported symbols.
//
// Cycle rule (PRD-06): this package may NOT import internal/client. The
// auth-bootstrap HTTP calls in phase 2b will use go-ovh primitives directly.
package auth

import (
	"fmt"
	"net/url"
)

// Region is the registry entry for one OVH endpoint. PRD-03 §Region Go type.
type Region struct {
	ID                    string   // e.g. "ovh-eu"
	DisplayName           string   // e.g. "OVHcloud Europe"
	EndpointURL           string   // /1.0/ base URL
	PortalCreateAppURL    string   // where users mint AK/AS pairs
	OAuth2Issuer          string   // empty if OAuth2 not available
	OAuth2DefaultScopes   []string // requested scopes when --scopes not given
	ValidationHostPattern []string // PRD-03 §validation_host_pattern (exact equality)
}

// regions is the static registry. Order matches PRD-03's table.
// Concrete OAuth2Issuer values are tentative until phase-2b RC verifies them
// against the OVH portal.
var regions = []Region{
	{
		ID:                    "ovh-eu",
		DisplayName:           "OVHcloud Europe",
		EndpointURL:           "https://eu.api.ovh.com/1.0",
		PortalCreateAppURL:    "https://eu.api.ovh.com/createApp/",
		OAuth2Issuer:          "https://www.ovh.com/auth",
		OAuth2DefaultScopes:   []string{"account:get"},
		ValidationHostPattern: []string{"eu.api.ovh.com", "www.ovh.com"},
	},
	{
		ID:                    "ovh-us",
		DisplayName:           "OVHcloud US",
		EndpointURL:           "https://api.us.ovhcloud.com/1.0",
		PortalCreateAppURL:    "https://api.us.ovhcloud.com/createApp/",
		ValidationHostPattern: []string{"api.us.ovhcloud.com", "us.ovhcloud.com"},
	},
	{
		ID:                    "ovh-ca",
		DisplayName:           "OVHcloud Canada",
		EndpointURL:           "https://ca.api.ovh.com/1.0",
		PortalCreateAppURL:    "https://ca.api.ovh.com/createApp/",
		ValidationHostPattern: []string{"ca.api.ovh.com", "ca.ovh.com"},
	},
	{
		ID:                    "kimsufi-eu",
		DisplayName:           "Kimsufi Europe",
		EndpointURL:           "https://eu.api.kimsufi.com/1.0",
		PortalCreateAppURL:    "https://eu.api.kimsufi.com/createApp/",
		ValidationHostPattern: []string{"eu.api.kimsufi.com", "www.kimsufi.com"},
	},
	{
		ID:                    "kimsufi-ca",
		DisplayName:           "Kimsufi Canada",
		EndpointURL:           "https://ca.api.kimsufi.com/1.0",
		PortalCreateAppURL:    "https://ca.api.kimsufi.com/createApp/",
		ValidationHostPattern: []string{"ca.api.kimsufi.com", "ca.kimsufi.com"},
	},
	{
		ID:                    "soyoustart-eu",
		DisplayName:           "SoYouStart Europe",
		EndpointURL:           "https://eu.api.soyoustart.com/1.0",
		PortalCreateAppURL:    "https://eu.api.soyoustart.com/createApp/",
		ValidationHostPattern: []string{"eu.api.soyoustart.com", "www.soyoustart.com"},
	},
	{
		ID:                    "soyoustart-ca",
		DisplayName:           "SoYouStart Canada",
		EndpointURL:           "https://ca.api.soyoustart.com/1.0",
		PortalCreateAppURL:    "https://ca.api.soyoustart.com/createApp/",
		ValidationHostPattern: []string{"ca.api.soyoustart.com", "ca.soyoustart.com"},
	},
}

// Regions returns a defensive copy of the static registry.
func Regions() []Region {
	out := make([]Region, len(regions))
	copy(out, regions)
	return out
}

// RegionByID looks up a region; returns (Region{}, false) if unknown.
func RegionByID(id string) (Region, bool) {
	for _, r := range regions {
		if r.ID == id {
			return r, true
		}
	}
	return Region{}, false
}

// ValidateHost returns nil iff u's scheme is "https", its host is exactly
// equal to one of allowedHosts, and it carries no userinfo component.
// PRD-03 §Hardening; cited by the OAuth2 flow before opening the browser
// and by the composition root when injecting allowedHosts into Layer A.
func ValidateHost(u string, allowedHosts []string) error {
	parsed, err := url.Parse(u)
	if err != nil {
		return fmt.Errorf("auth: parse url: %w", err)
	}
	if parsed.Scheme != "https" {
		return fmt.Errorf("auth: url %q is not https", u)
	}
	if parsed.User != nil {
		return fmt.Errorf("auth: url %q has userinfo component", u)
	}
	for _, h := range allowedHosts {
		if parsed.Host == h {
			return nil
		}
	}
	return fmt.Errorf("auth: url host %q not in allowed list", parsed.Host)
}
