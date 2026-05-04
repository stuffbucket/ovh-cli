package auth

import (
	"errors"
	"fmt"
	"strings"

	"github.com/zalando/go-keyring"
)

// keyringService is the OS-keyring service name used by every ovh-cli
// install. PRD-04 §Keyring placeholder grammar.
const keyringService = "ovh-cli"

// placeholderPrefix marks a value in ovh.conf as a keyring reference rather
// than a plaintext secret. PRD-04 §Keyring placeholder grammar.
const placeholderPrefix = "keyring:"

// keyringUser returns the (region, profile, key) tuple as the OS-keyring
// "account" string. The placeholder value in ovh.conf is just
// placeholderPrefix + keyringUser(...) — they share format on purpose so a
// reader can extract the user directly from the placeholder.
func keyringUser(region, profile, key string) string {
	return region + ":" + profile + ":" + key
}

// isPlaceholder reports whether v is a keyring placeholder reference.
func isPlaceholder(v string) bool { return strings.HasPrefix(v, placeholderPrefix) }

// makePlaceholder returns the placeholder value to write into ovh.conf.
func makePlaceholder(region, profile, key string) string {
	return placeholderPrefix + keyringUser(region, profile, key)
}

// keyringSet writes the secret to the OS keyring.
func keyringSet(region, profile, key, secret string) error {
	if err := keyring.Set(keyringService, keyringUser(region, profile, key), secret); err != nil {
		return fmt.Errorf("auth: keyring write: %w", mapKeyringError(err))
	}
	return nil
}

// keyringGet reads the secret from the OS keyring. Maps go-keyring errors
// per PRD-04 §Keyring placeholder grammar:
//   - ErrNotFound (placeholder exists but no keyring entry) -> ErrNotConfigured
//   - other errors -> ErrKeyringUnavailable
func keyringGet(region, profile, key string) (string, error) {
	secret, err := keyring.Get(keyringService, keyringUser(region, profile, key))
	if err != nil {
		return "", mapKeyringError(err)
	}
	return secret, nil
}

// keyringDelete removes the secret from the OS keyring. Idempotent: a missing
// entry is treated as success (already gone).
func keyringDelete(region, profile, key string) error {
	err := keyring.Delete(keyringService, keyringUser(region, profile, key))
	if err == nil || errors.Is(err, keyring.ErrNotFound) {
		return nil
	}
	return mapKeyringError(err)
}

// mapKeyringError translates go-keyring errors to our sentinels.
// PRD-04 §Keyring placeholder grammar.
func mapKeyringError(err error) error {
	if errors.Is(err, keyring.ErrNotFound) {
		return ErrNotConfigured
	}
	// Other errors typically indicate the OS keyring service is unavailable
	// (no dbus on Linux, locked Keychain on macOS, etc.).
	return fmt.Errorf("%w: %v", ErrKeyringUnavailable, err)
}
