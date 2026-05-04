package auth

import "testing"

func TestRegions_AllSevenPresent(t *testing.T) {
	if got := len(Regions()); got != 7 {
		t.Errorf("Regions() returned %d entries; want 7", got)
	}
}

func TestRegions_AllRequiredFieldsNonEmpty(t *testing.T) {
	for _, r := range Regions() {
		if r.ID == "" {
			t.Errorf("region with empty ID: %+v", r)
			continue
		}
		if r.DisplayName == "" {
			t.Errorf("region %q has empty DisplayName", r.ID)
		}
		if r.EndpointURL == "" {
			t.Errorf("region %q has empty EndpointURL", r.ID)
		}
		if r.PortalCreateAppURL == "" {
			t.Errorf("region %q has empty PortalCreateAppURL", r.ID)
		}
		if len(r.ValidationHostPattern) == 0 {
			t.Errorf("region %q has empty ValidationHostPattern (PRD-03 invariant)", r.ID)
		}
	}
}

func TestRegions_DefensiveCopy(t *testing.T) {
	a := Regions()
	a[0].ID = "mutated"
	if b := Regions(); b[0].ID == "mutated" {
		t.Error("Regions() did not return a defensive copy")
	}
}

func TestRegionByID_Known(t *testing.T) {
	r, ok := RegionByID("ovh-eu")
	if !ok {
		t.Fatal("ovh-eu not found")
	}
	if r.ID != "ovh-eu" {
		t.Errorf("ID=%q want ovh-eu", r.ID)
	}
}

func TestRegionByID_Unknown(t *testing.T) {
	if _, ok := RegionByID("ovh-mars"); ok {
		t.Error("ovh-mars should not be found")
	}
}

func TestValidateHost_AllowedExactMatch(t *testing.T) {
	if err := ValidateHost("https://eu.api.ovh.com/auth/credential/123", []string{"eu.api.ovh.com"}); err != nil {
		t.Errorf("got %v; want nil", err)
	}
}

func TestValidateHost_Rejections(t *testing.T) {
	cases := []struct {
		name, u string
	}{
		{"http_scheme", "http://eu.api.ovh.com/x"},
		{"wrong_host", "https://attacker.example/x"},
		{"userinfo", "https://user:pass@eu.api.ovh.com/x"},
		{"unparseable", "://broken"},
		{"empty", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if err := ValidateHost(c.u, []string{"eu.api.ovh.com"}); err == nil {
				t.Errorf("ValidateHost(%q) returned nil; want error", c.u)
			}
		})
	}
}

func TestCredentials_IsZero(t *testing.T) {
	if !(Credentials{}).IsZero() {
		t.Error("zero Credentials reports IsZero=false")
	}
	if (Credentials{Region: "ovh-eu"}).IsZero() {
		t.Error("non-zero Credentials reports IsZero=true")
	}
}
