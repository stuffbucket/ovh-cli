package schema

import (
	"fmt"
	"sort"
	"strings"
)

// regionConfig pairs an OVH API endpoint URL with the host allowlist for a
// region. PRD-03 §validation_host_pattern; PRD-05 §Threat model.
//
// Phase 2a hardcodes this map. Phase 2b should delete this file and have the
// cobra command resolve both fields via internal/auth.RegionByID, which
// becomes the canonical region registry per PRD-03.
type regionConfig struct {
	Endpoint     string
	AllowedHosts []string
}

var regions = map[string]regionConfig{
	"ovh-eu":        {"https://eu.api.ovh.com/1.0", []string{"eu.api.ovh.com", "www.ovh.com"}},
	"ovh-us":        {"https://api.us.ovhcloud.com/1.0", []string{"api.us.ovhcloud.com", "us.ovhcloud.com"}},
	"ovh-ca":        {"https://ca.api.ovh.com/1.0", []string{"ca.api.ovh.com", "ca.ovh.com"}},
	"kimsufi-eu":    {"https://eu.api.kimsufi.com/1.0", []string{"eu.api.kimsufi.com", "www.kimsufi.com"}},
	"kimsufi-ca":    {"https://ca.api.kimsufi.com/1.0", []string{"ca.api.kimsufi.com", "ca.kimsufi.com"}},
	"soyoustart-eu": {"https://eu.api.soyoustart.com/1.0", []string{"eu.api.soyoustart.com", "www.soyoustart.com"}},
	"soyoustart-ca": {"https://ca.api.soyoustart.com/1.0", []string{"ca.api.soyoustart.com", "ca.soyoustart.com"}},
}

func resolveRegion(id string) (regionConfig, error) {
	rc, ok := regions[id]
	if !ok {
		return regionConfig{}, fmt.Errorf("unknown region %q (one of: %s)", id, strings.Join(regionIDs(), ", "))
	}
	return rc, nil
}

func regionIDs() []string {
	out := make([]string, 0, len(regions))
	for id := range regions {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}
