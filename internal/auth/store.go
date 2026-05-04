package auth

import (
	"context"
	"errors"
	"fmt"
	"os"

	"gopkg.in/ini.v1"
)

// readEnv returns Credentials populated from OVH_* env vars per PRD-04
// §Canonical env-var registry. Returns the zero Credentials when no
// actionable auth values are set OR when OVH_REGION is set to a region
// other than the requested one.
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
// Pre/post: see PRD-03 §store.go contract. Empty region is a programming
// error and panics; an unknown-but-nonempty region returns an error so
// config-derived bad values don't crash the process.
func LoadCredentials(_ context.Context, region, profile string) (Credentials, error) {
	if region == "" {
		panic("auth.LoadCredentials: region must not be empty (PRD-03 pre-condition)")
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

	f, _, err := loadDefaultConf()
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
	if region == "" {
		panic("auth.StoreCredentials: region must not be empty (PRD-03 pre-condition)")
	}
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

	f, _, err := loadDefaultConf()
	if err != nil {
		return err
	}
	if f == nil {
		f = ini.Empty()
	}
	applyCreds(f, creds)
	return writeOvhConf(f)
}

// DeleteCredentials removes credentials for region from ovh.conf.
//
// Profile handling: phase 2b iter 2a treats each [region] section as a
// single "default" profile. The profile argument is reserved for the
// multi-profile expansion in iter 2b; today it is ignored, and the call
// removes the entire section. Idempotent: returns nil when no section
// exists.
//
// Pre/post: see PRD-03 §store.go contract.
func DeleteCredentials(_ context.Context, region, _ string) error {
	if region == "" {
		panic("auth.DeleteCredentials: region must not be empty (PRD-03 pre-condition)")
	}
	f, _, err := loadDefaultConf()
	if err != nil {
		return err
	}
	if f == nil || !f.HasSection(region) {
		return nil
	}
	f.DeleteSection(region)
	return writeOvhConf(f)
}
