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
// Storage backend selection: LoadCredentials reads [default].storage from
// the same ovh.conf it parses for credentials. When storage=keyring,
// secret values matching the placeholder grammar (PRD-04) are resolved
// via the OS keyring; when storage=file (or unset and no file exists),
// values are taken as plaintext.
//
// Pre/post: see PRD-03 §store.go contract. Empty region panics
// (programming error); unknown-but-nonempty region returns an error.
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
	c, err := readCredsFromConf(f, region)
	if err != nil {
		return Credentials{}, err
	}
	if c.IsZero() {
		return Credentials{}, ErrNotConfigured
	}
	c.Profile = profile

	// OAuth2 session state lives in hosts.yml (PRD-04 §Canonical hosts.yml
	// schema) — read it after ovh.conf so hosts.yml wins on overlap per
	// PRD-03 §Storage. Iter 2c only reads OAuth2 fields; classic-CK creds
	// never touch hosts.yml.
	if c.Method == MethodOAuth2 {
		if _, err := readHostsCreds(&c, region, profile); err != nil {
			return Credentials{}, err
		}
	}
	return c, nil
}

// StoreCredentials writes creds into the canonical
// $XDG_CONFIG_HOME/ovh/ovh.conf, merging with any existing sections.
// Atomic; ovh.conf mode 0600 per PRD-04 §Canonical file-mode registry.
//
// When [default].storage=keyring (the default), secrets go to the OS
// keyring and ovh.conf gets placeholders. When [default].storage=file,
// secrets are written plaintext to ovh.conf. Keyring writes happen first
// — if any fail, ovh.conf is left untouched and the previous state stands.
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
	storage := storageMode(f)
	if err := applyCreds(f, creds, storage); err != nil {
		return err
	}
	if err := writeOvhConf(f); err != nil {
		return err
	}
	// OAuth2 session state goes to hosts.yml (PRD-04). Only relevant when
	// the caller actually has OAuth2 token state to persist; classic-CK
	// creds skip this path entirely.
	if creds.Method == MethodOAuth2 && (creds.AccessToken != "" || creds.RefreshToken != "" || !creds.Expiry.IsZero()) {
		if err := writeHostsCreds(creds, storage); err != nil {
			return err
		}
	}
	return nil
}

// DeleteCredentials removes credentials for (region, profile). Removes both
// the keyring entries (per PRD-04 keyring grammar) and the ovh.conf section.
//
// Iter 2b semantics: profile is honored on the keyring side (entries keyed
// by <region>:<profile>:<key> are removed), but the ovh.conf [region]
// section is removed regardless. Multi-profile coexistence in ovh.conf is
// iter 2c — at that point this function will only remove the named
// profile's slice of [region].
//
// Idempotent on missing.
//
// Pre/post: see PRD-03 §store.go contract.
func DeleteCredentials(_ context.Context, region, profile string) error {
	if region == "" {
		panic("auth.DeleteCredentials: region must not be empty (PRD-03 pre-condition)")
	}
	if profile == "" {
		profile = "default"
	}

	for _, sf := range secretFields {
		_ = keyringDelete(region, profile, sf.key)
	}

	if err := deleteHostsCreds(region, profile); err != nil {
		return err
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
