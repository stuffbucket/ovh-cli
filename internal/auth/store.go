package auth

import (
	"context"
	"errors"
	"fmt"
	"os"

	"gopkg.in/ini.v1"
)

// readEnv returns Credentials populated from OVH_* env vars per PRD-04
// §Canonical env-var registry. Only returns non-zero when OVH_REGION matches
// the requested region AND at least one credential field is set.
func readEnv(region string) Credentials {
	if envRegion := os.Getenv("OVH_REGION"); envRegion != "" && envRegion != region {
		return Credentials{}
	}
	creds := Credentials{Region: region, Profile: "default"}
	creds.ApplicationKey = os.Getenv("OVH_APPLICATION_KEY")
	creds.ApplicationSecret = os.Getenv("OVH_APPLICATION_SECRET")
	creds.ConsumerKey = os.Getenv("OVH_CONSUMER_KEY")
	creds.ClientID = os.Getenv("OVH_CLIENT_ID")
	creds.ClientSecret = os.Getenv("OVH_CLIENT_SECRET")
	switch {
	case creds.ApplicationKey != "" || creds.ConsumerKey != "":
		creds.Method = MethodConsumerKey
	case creds.ClientID != "" && creds.ClientSecret != "":
		creds.Method = MethodOAuth2
	default:
		return Credentials{}
	}
	return creds
}

// LoadCredentials reads credentials per PRD-04 §Read precedence:
// env -> flags(handled by composition root) -> ./ovh.conf ->
// ~/.ovh.conf -> $XDG_CONFIG_HOME/ovh/ovh.conf -> /etc/ovh.conf.
//
// Phase 2b iter 2a: keyring backend not yet wired — file is the only
// persistent backend. Iter 2b adds the keyring path.
//
// Pre/post: see PRD-03 §store.go contract.
func LoadCredentials(_ context.Context, region, profile string) (Credentials, error) {
	if region == "" {
		return Credentials{}, errors.New("auth: region must not be empty")
	}
	if profile == "" {
		profile = "default"
	}
	if _, ok := RegionByID(region); !ok {
		return Credentials{}, fmt.Errorf("auth: unknown region %q", region)
	}

	if c := readEnv(region); !c.IsZero() {
		c.Profile = profile
		return c, nil
	}

	f, _, err := defaultLocator().load()
	if err != nil {
		return Credentials{}, err
	}
	c := readCredsFromConf(f, region)
	if c.IsZero() {
		return Credentials{}, ErrNotConfigured
	}
	c.Profile = profile
	return c, nil
}

// StoreCredentials writes creds into the canonical
// $XDG_CONFIG_HOME/ovh/ovh.conf, merging with any existing sections.
// Atomic; mode 0600 per PRD-04 §Canonical file-mode registry.
//
// Pre/post: see PRD-03 §store.go contract.
func StoreCredentials(_ context.Context, region, profile string, creds Credentials) error {
	if creds.IsZero() {
		return errors.New("auth: cannot store zero Credentials")
	}
	if creds.Region != region {
		return fmt.Errorf("auth: creds.Region=%q does not match region=%q", creds.Region, region)
	}
	if profile == "" {
		profile = "default"
	}
	creds.Profile = profile

	f, _, err := defaultLocator().load()
	if err != nil {
		return err
	}
	if f == nil {
		f = ini.Empty()
	}
	applyCreds(f, creds)
	return writeOvhConf(f)
}

// DeleteCredentials removes creds for region from ovh.conf. Idempotent:
// returns nil when nothing exists for the region.
//
// Pre/post: see PRD-03 §store.go contract.
func DeleteCredentials(_ context.Context, region, _ string) error {
	if region == "" {
		return errors.New("auth: region must not be empty")
	}
	f, _, err := defaultLocator().load()
	if err != nil {
		return err
	}
	if f == nil || !f.HasSection(region) {
		return nil
	}
	f.DeleteSection(region)
	return writeOvhConf(f)
}
