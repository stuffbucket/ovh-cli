package auth

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/google/renameio/v2"
	"gopkg.in/ini.v1"

	"github.com/stuffbucket/ovh-cli/internal/xdgpaths"
)

// loadDefaultConf walks PRD-04 §Read precedence (./ovh.conf -> $HOME/.ovh.conf
// -> $XDG_CONFIG_HOME/ovh/ovh.conf -> /etc/ovh.conf) and returns the first
// existing readable file parsed as INI. Returns (nil, "", nil) when no file
// is found.
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

// readCredsFromConf extracts (region) credentials from a parsed ovh.conf.
// Returns the zero Credentials when the section is missing OR when the
// section exists but contains no usable auth values — callers can then rely
// on the IsZero() check to mean "no credentials in this source," matching
// readEnv's contract.
//
// Phase 2b iter 2a reads classic AK/AS/CK and OAuth2 client_id/secret as
// plaintext. Iter 2b adds keyring placeholder (`keyring:<region>:<key>`)
// resolution when default.storage=keyring.
func readCredsFromConf(f *ini.File, region string) Credentials {
	if f == nil || !f.HasSection(region) {
		return Credentials{}
	}
	s := f.Section(region)
	creds := Credentials{Region: region, Profile: "default"}
	creds.ApplicationKey = s.Key("application_key").String()
	creds.ApplicationSecret = s.Key("application_secret").String()
	creds.ConsumerKey = s.Key("consumer_key").String()
	creds.ClientID = s.Key("client_id").String()
	creds.ClientSecret = s.Key("client_secret").String()

	// Prefer an explicit auth_method when present (PRD-04 §Canonical key
	// registry exposes <region>.auth_method); fall back to deriving from
	// which fields populated, so files written by python-ovh — which do
	// not write auth_method — still parse correctly.
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
			return Credentials{} // section present but no usable auth
		}
	}
	return creds
}

// writeOvhConf writes f to $XDG_CONFIG_HOME/ovh/ovh.conf atomically with
// mode 0600. PRD-04 §Atomic writes + §Canonical file-mode registry.
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

// applyCreds writes creds into f's [region] section. Empty fields are
// deleted so partial writes don't leave stale values. auth_method is
// written explicitly (PRD-04 §Canonical key registry: <region>.auth_method)
// so internal/config can read it without re-deriving.
func applyCreds(f *ini.File, creds Credentials) {
	s := f.Section(creds.Region)
	setOrDelete(s, "auth_method", string(creds.Method))
	setOrDelete(s, "application_key", creds.ApplicationKey)
	setOrDelete(s, "application_secret", creds.ApplicationSecret)
	setOrDelete(s, "consumer_key", creds.ConsumerKey)
	setOrDelete(s, "client_id", creds.ClientID)
	setOrDelete(s, "client_secret", creds.ClientSecret)
}

func setOrDelete(s *ini.Section, key, value string) {
	if value == "" {
		s.DeleteKey(key)
		return
	}
	s.Key(key).SetValue(value)
}
