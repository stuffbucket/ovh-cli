package auth

import (
	"errors"
	"strings"
	"testing"

	"github.com/zalando/go-keyring"
)

func TestKeyring_RoundTrip(t *testing.T) {
	keyring.MockInit()
	if err := keyringSet("ovh-eu", "default", "application_secret", "secret-value"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got, err := keyringGet("ovh-eu", "default", "application_secret")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != "secret-value" {
		t.Errorf("got %q want %q", got, "secret-value")
	}
}

func TestKeyring_GetMissingMapsToErrNotConfigured(t *testing.T) {
	keyring.MockInit()
	_, err := keyringGet("ovh-eu", "default", "application_secret")
	if !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("got %v; want ErrNotConfigured (placeholder/keyring desync)", err)
	}
}

func TestKeyring_DeleteIdempotent(t *testing.T) {
	keyring.MockInit()
	if err := keyringDelete("ovh-eu", "default", "application_secret"); err != nil {
		t.Errorf("Delete on missing: got %v; want nil", err)
	}
	if err := keyringSet("ovh-eu", "default", "application_secret", "x"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if err := keyringDelete("ovh-eu", "default", "application_secret"); err != nil {
		t.Errorf("Delete on present: got %v; want nil", err)
	}
	if err := keyringDelete("ovh-eu", "default", "application_secret"); err != nil {
		t.Errorf("Delete on second-call: got %v; want nil", err)
	}
}

func TestPlaceholderRoundTrip(t *testing.T) {
	p := makePlaceholder("ovh-eu", "default", "application_secret")
	if !isPlaceholder(p) {
		t.Errorf("isPlaceholder(%q)=false", p)
	}
	if got, want := p, "keyring:ovh-eu:default:application_secret"; got != want {
		t.Errorf("placeholder=%q want %q", got, want)
	}
	if isPlaceholder("plain-secret-value") {
		t.Error("plain value should not match isPlaceholder")
	}
}

func TestResolveSecret_Plaintext(t *testing.T) {
	got, err := resolveSecret("plain-secret", "ovh-eu", "default", "application_secret")
	if err != nil {
		t.Fatalf("resolveSecret: %v", err)
	}
	if got != "plain-secret" {
		t.Errorf("got %q want plain-secret", got)
	}
}

func TestResolveSecret_PlaceholderHits(t *testing.T) {
	keyring.MockInit()
	_ = keyringSet("ovh-eu", "default", "application_secret", "S3CRET")
	p := makePlaceholder("ovh-eu", "default", "application_secret")
	got, err := resolveSecret(p, "ovh-eu", "default", "application_secret")
	if err != nil {
		t.Fatalf("resolveSecret: %v", err)
	}
	if got != "S3CRET" {
		t.Errorf("got %q want S3CRET", got)
	}
}

func TestResolveSecret_MismatchedPlaceholder(t *testing.T) {
	// Hand-edited ovh.conf with a placeholder pointing at a different
	// region/profile/key — integrity violation; resolveSecret refuses.
	bogus := "keyring:ovh-us:default:application_secret"
	_, err := resolveSecret(bogus, "ovh-eu", "default", "application_secret")
	if err == nil || !strings.Contains(err.Error(), "does not match expected") {
		t.Fatalf("got %v; want mismatched-placeholder error", err)
	}
}
