package schema

import "testing"

func TestNewCmd_HasSubcommands(t *testing.T) {
	cmd := NewCmd()
	for _, name := range []string{"refresh", "show", "path", "diff"} {
		sub, _, err := cmd.Find([]string{name})
		if err != nil {
			t.Fatalf("subcommand %q missing: %v", name, err)
		}
		if sub.Name() != name {
			t.Errorf("Find(%q) returned %q", name, sub.Name())
		}
	}
}

func TestResolveRegion_Unknown(t *testing.T) {
	if _, err := resolveRegion("ovh-mars"); err == nil {
		t.Fatal("expected error for unknown region")
	}
}

func TestResolveRegion_KnownRegionsHaveEndpointAndHosts(t *testing.T) {
	for _, id := range regionIDs() {
		rc, err := resolveRegion(id)
		if err != nil {
			t.Fatalf("resolveRegion(%q): %v", id, err)
		}
		if rc.Endpoint == "" {
			t.Errorf("region %q has empty endpoint", id)
		}
		if len(rc.AllowedHosts) == 0 {
			t.Errorf("region %q has empty AllowedHosts", id)
		}
	}
}

func TestRegionIDs_CoversAllSevenRegions(t *testing.T) {
	if got := len(regionIDs()); got != 7 {
		t.Errorf("regionIDs() returned %d entries; want 7", got)
	}
}
