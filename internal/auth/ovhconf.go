package auth

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/renameio/v2"
	"gopkg.in/ini.v1"

	"github.com/stuffbucket/ovh-cli/internal/xdgpaths"
)

// storageKeyring is the value of [default].storage that selects the keyring
// backend. PRD-04 §Canonical key registry; default when unset.
const storageKeyring = "keyring"

// secretField is one credential key that may be stored either as plaintext
// in ovh.conf or as a keyring placeholder (PRD-04 §Keyring placeholder
// grammar). Iter 2c will append access_token and refresh_token.
type secretField struct {
	key string
	get func(*Credentials) string
	set func(*Credentials, string)
}

var secretFields = []secretField{
	{"application_secret",
		func(c *Credentials) string { return c.ApplicationSecret },
		func(c *Credentials, v string) { c.ApplicationSecret = v }},
	{"consumer_key",
		func(c *Credentials) string { return c.ConsumerKey },
		func(c *Credentials, v string) { c.ConsumerKey = v }},
	{"client_secret",
		func(c *Credentials) string { return c.ClientSecret },
		func(c *Credentials, v string) { c.ClientSecret = v }},
}

// loadDefaultConf walks PRD-04 §Read precedence and returns the first
// existing readable ovh.conf parsed as INI. (nil, "", nil) on no file.
func loadDefaultConf() (*ini.File, string, error) {
	paths := []string{"ovh.conf"}
	if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths, filepath.Join(home, ".ovh.conf"))
	}
	paths = append(paths, xdgpaths.ConfigFile(), "/etc/ovh.conf")
	for _, p := range paths {
		info, err := os.Stat(p)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, "", fmt.Errorf("auth: stat %s: %w", p, err)
		}
		if info.IsDir() {
			continue
		}
		f, err := ini.Load(p)
		if err != nil {
			return nil, "", fmt.Errorf("auth: parse %s: %w", p, err)
		}
		return f, p, nil
	}
	return nil, "", nil
}

// storageMode returns [default].storage from f, defaulting to "keyring" per
// PRD-04 §Canonical key registry. Reading directly from f avoids an
// internal/config import (cycle rule, PRD-06).
func storageMode(f *ini.File) string {
	if f == nil {
		return storageKeyring
	}
	v := f.Section("default").Key("storage").String()
	if v == "" {
		return storageKeyring
	}
	return v
}

// readCredsFromConf extracts (region) credentials from f.
//
// ovh.conf [region] sections are NOT profile-scoped — they hold the long-
// lived application credentials (AK/AS/CK, client_id, client_secret) that
// are shared across profiles in the region. Per-profile session state
// (access_token, refresh_token, expiry) lives in hosts.yml (PRD-04
// §Canonical hosts.yml schema). Therefore the keyring placeholder in
// ovh.conf is always profile="default" regardless of creds.Profile.
func readCredsFromConf(f *ini.File, region string) (Credentials, error) {
	if f == nil || !f.HasSection(region) {
		return Credentials{}, nil
	}
	s := f.Section(region)
	creds := Credentials{Region: region, Profile: "default"}
	creds.ApplicationKey = s.Key("application_key").String()
	creds.ClientID = s.Key("client_id").String()

	for _, sf := range secretFields {
		v := s.Key(sf.key).String()
		if v == "" {
			continue
		}
		// ovh.conf placeholders are always default-profile-keyed (see doc above).
		resolved, err := resolveSecret(v, region, "default", sf.key)
		if err != nil {
			return Credentials{}, err
		}
		sf.set(&creds, resolved)
	}

	switch Method(s.Key("auth_method").String()) {
	case MethodConsumerKey:
		creds.Method = MethodConsumerKey
	case MethodOAuth2:
		creds.Method = MethodOAuth2
	}
	if creds.Method == MethodNone {
		switch {
		case creds.ApplicationKey != "" || creds.ConsumerKey != "":
			creds.Method = MethodConsumerKey
		case creds.ClientID != "" && creds.ClientSecret != "":
			creds.Method = MethodOAuth2
		default:
			return Credentials{}, nil
		}
	}
	return creds, nil
}

// resolveSecret returns v unchanged if it's plaintext, or resolves the
// placeholder via the keyring. Mismatched placeholder values (the embedded
// region/profile/key disagrees with what the caller asked for) are an
// integrity error: somebody hand-edited ovh.conf incorrectly.
func resolveSecret(v, region, profile, key string) (string, error) {
	if !isPlaceholder(v) {
		return v, nil
	}
	expected := keyringUser(region, profile, key)
	got := strings.TrimPrefix(v, placeholderPrefix)
	if got != expected {
		return "", fmt.Errorf("auth: placeholder %q does not match expected %q", v, placeholderPrefix+expected)
	}
	return keyringGet(region, profile, key)
}

// applyCreds writes creds into f's [region] section. Empty fields are
// deleted from both ovh.conf and the keyring (idempotent on missing).
// auth_method is written explicitly per PRD-04 §Canonical key registry.
//
// When storage=keyring, secrets are written to the OS keyring and ovh.conf
// gets placeholders. When storage=file, secrets are written plaintext to
// ovh.conf. Keyring writes happen BEFORE the ovh.conf write — if keyring
// fails, ovh.conf is untouched and the previous state stands.
func applyCreds(f *ini.File, creds Credentials, storage string) error {
	s := f.Section(creds.Region)
	setOrDelete(s, "auth_method", string(creds.Method))
	setOrDelete(s, "application_key", creds.ApplicationKey)
	setOrDelete(s, "client_id", creds.ClientID)

	// ovh.conf is region-scoped (no profile dimension); long-lived
	// application credentials are always keyed under profile="default" so
	// multi-profile users share one app per region. See readCredsFromConf
	// doc for the rationale.
	for _, sf := range secretFields {
		v := sf.get(&creds)
		if v == "" {
			_ = keyringDelete(creds.Region, "default", sf.key)
			s.DeleteKey(sf.key)
			continue
		}
		if storage == storageKeyring {
			if err := keyringSet(creds.Region, "default", sf.key, v); err != nil {
				return err
			}
			s.Key(sf.key).SetValue(makePlaceholder(creds.Region, "default", sf.key))
		} else {
			s.Key(sf.key).SetValue(v)
		}
	}
	return nil
}

// writeOvhConf writes f to $XDG_CONFIG_HOME/ovh/ovh.conf atomically with
// mode 0600 per PRD-04 §Atomic writes + §Canonical file-mode registry.
func writeOvhConf(f *ini.File) error {
	path := xdgpaths.ConfigFile()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("auth: mkdir config dir: %w", err)
	}
	var buf bytes.Buffer
	if _, err := f.WriteTo(&buf); err != nil {
		return fmt.Errorf("auth: marshal ini: %w", err)
	}
	return renameio.WriteFile(path, buf.Bytes(), 0o600)
}

func setOrDelete(s *ini.Section, key, value string) {
	if value == "" {
		s.DeleteKey(key)
		return
	}
	s.Key(key).SetValue(value)
}
