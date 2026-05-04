package auth

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/google/renameio/v2"
	"gopkg.in/yaml.v3"

	"github.com/stuffbucket/ovh-cli/internal/xdgpaths"
)

// hostsYMLVersion is the current hosts.yml schema version.
const hostsYMLVersion = 1

// hostsFile is the on-disk shape of $XDG_CONFIG_HOME/ovh/hosts.yml.
// PRD-04 §Canonical hosts.yml schema.
type hostsFile struct {
	Version int                   `yaml:"version"`
	Hosts   map[string]hostRegion `yaml:"hosts"`
}

type hostRegion struct {
	Profiles map[string]hostProfile `yaml:"profiles"`
}

type hostProfile struct {
	AccessToken     string    `yaml:"access_token,omitempty"`
	RefreshToken    string    `yaml:"refresh_token,omitempty"`
	Expiry          time.Time `yaml:"expiry,omitempty"`
	Account         string    `yaml:"account,omitempty"`
	LastValidatedAt time.Time `yaml:"last_validated_at,omitempty"`
	ScopesGranted   []string  `yaml:"scopes_granted,omitempty"`
}

// loadHostsYML returns the parsed hosts.yml or (nil, nil) when no file exists.
// Refuses to read on file modes looser than 0600 per PRD-04 file-mode registry.
func loadHostsYML() (*hostsFile, error) {
	path := xdgpaths.HostsFile()
	info, err := os.Stat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("auth: stat hosts.yml: %w", err)
	}
	if info.Mode().Perm()&0o077 != 0 {
		return nil, fmt.Errorf("%w: hosts.yml mode %o", ErrFileModeUnsafe, info.Mode().Perm())
	}
	b, err := os.ReadFile(path) // #nosec G304 -- xdgpaths-derived
	if err != nil {
		return nil, fmt.Errorf("auth: read hosts.yml: %w", err)
	}
	var hf hostsFile
	if err := yaml.Unmarshal(b, &hf); err != nil {
		return nil, fmt.Errorf("auth: parse hosts.yml: %w", err)
	}
	return &hf, nil
}

// writeHostsYML atomically writes f to hosts.yml at mode 0600.
func writeHostsYML(f *hostsFile) error {
	path := xdgpaths.HostsFile()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("auth: mkdir config dir: %w", err)
	}
	b, err := yaml.Marshal(f)
	if err != nil {
		return fmt.Errorf("auth: marshal hosts.yml: %w", err)
	}
	return renameio.WriteFile(path, b, 0o600)
}

// readHostsCreds merges OAuth2 token state from hosts.yml into c. Placeholder
// values are resolved via the keyring. Returns ok=false when hosts.yml is
// missing or has no entry for (region, profile).
func readHostsCreds(c *Credentials, region, profile string) (bool, error) {
	hf, err := loadHostsYML()
	if err != nil {
		return false, err
	}
	if hf == nil {
		return false, nil
	}
	p, ok := hf.Hosts[region].Profiles[profile]
	if !ok {
		return false, nil
	}
	if p.AccessToken != "" {
		v, err := resolveSecret(p.AccessToken, region, profile, "access_token")
		if err != nil {
			return false, err
		}
		c.AccessToken = v
	}
	if p.RefreshToken != "" {
		v, err := resolveSecret(p.RefreshToken, region, profile, "refresh_token")
		if err != nil {
			return false, err
		}
		c.RefreshToken = v
	}
	if !p.Expiry.IsZero() {
		c.Expiry = p.Expiry
	}
	return true, nil
}

// writeHostsCreds persists OAuth2 token state for (region, profile) into
// hosts.yml. Secrets go through the placeholder grammar when storage=keyring.
// Empty fields delete from both keyring and hosts.yml.
func writeHostsCreds(c Credentials, storage string) error {
	hf, err := loadHostsYML()
	if err != nil {
		return err
	}
	if hf == nil {
		hf = &hostsFile{Version: hostsYMLVersion, Hosts: map[string]hostRegion{}}
	}
	if hf.Hosts == nil {
		hf.Hosts = map[string]hostRegion{}
	}
	region := hf.Hosts[c.Region]
	if region.Profiles == nil {
		region.Profiles = map[string]hostProfile{}
	}
	p := region.Profiles[c.Profile]

	for _, sf := range []struct {
		key string
		val *string
		out *string
	}{
		{"access_token", &c.AccessToken, &p.AccessToken},
		{"refresh_token", &c.RefreshToken, &p.RefreshToken},
	} {
		if *sf.val == "" {
			_ = keyringDelete(c.Region, c.Profile, sf.key)
			*sf.out = ""
			continue
		}
		if storage == storageKeyring {
			if err := keyringSet(c.Region, c.Profile, sf.key, *sf.val); err != nil {
				return err
			}
			*sf.out = makePlaceholder(c.Region, c.Profile, sf.key)
		} else {
			*sf.out = *sf.val
		}
	}
	p.Expiry = c.Expiry

	region.Profiles[c.Profile] = p
	hf.Hosts[c.Region] = region
	hf.Version = hostsYMLVersion
	return writeHostsYML(hf)
}

// deleteHostsCreds removes (region, profile) from hosts.yml and clears the
// access_token + refresh_token keyring entries. Idempotent on missing.
func deleteHostsCreds(region, profile string) error {
	_ = keyringDelete(region, profile, "access_token")
	_ = keyringDelete(region, profile, "refresh_token")

	hf, err := loadHostsYML()
	if err != nil {
		return err
	}
	if hf == nil {
		return nil
	}
	r, ok := hf.Hosts[region]
	if !ok {
		return nil
	}
	delete(r.Profiles, profile)
	if len(r.Profiles) == 0 {
		delete(hf.Hosts, region)
	} else {
		hf.Hosts[region] = r
	}
	return writeHostsYML(hf)
}
