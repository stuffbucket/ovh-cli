package auth

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stuffbucket/ovh-cli/internal/xdgpaths"
)

// sandbox isolates the test from the user's real $HOME / $XDG_CONFIG_HOME.
// Returns the path where ovh.conf will be written by StoreCredentials.
func sandbox(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	xdgConfig := filepath.Join(root, "config")
	t.Setenv("HOME", root)
	t.Setenv("XDG_CONFIG_HOME", xdgConfig)
	// Clear OVH_* vars that might be set in the developer's shell so they
	// don't leak into LoadCredentials. t.Setenv restores at test end.
	for _, k := range []string{"OVH_REGION", "OVH_APPLICATION_KEY", "OVH_APPLICATION_SECRET", "OVH_CONSUMER_KEY", "OVH_CLIENT_ID", "OVH_CLIENT_SECRET"} {
		t.Setenv(k, "")
	}
	xdgpaths.Reload()
	t.Chdir(root)
	return filepath.Join(xdgConfig, "ovh", "ovh.conf")
}

func TestLoadCredentials_NotConfigured(t *testing.T) {
	sandbox(t)
	_, err := LoadCredentials(context.Background(), "ovh-eu", "default")
	if !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("got %v; want ErrNotConfigured", err)
	}
}

func TestLoadCredentials_UnknownRegion(t *testing.T) {
	sandbox(t)
	_, err := LoadCredentials(context.Background(), "ovh-mars", "default")
	if err == nil {
		t.Fatal("expected error for unknown region")
	}
}

func TestLoadCredentials_EmptyRegion(t *testing.T) {
	sandbox(t)
	_, err := LoadCredentials(context.Background(), "", "default")
	if err == nil {
		t.Fatal("expected error for empty region")
	}
}

func TestLoadCredentials_Env_ClassicCK(t *testing.T) {
	sandbox(t)
	t.Setenv("OVH_REGION", "ovh-eu")
	t.Setenv("OVH_APPLICATION_KEY", "ak1")
	t.Setenv("OVH_APPLICATION_SECRET", "as1")
	t.Setenv("OVH_CONSUMER_KEY", "ck1")

	c, err := LoadCredentials(context.Background(), "ovh-eu", "default")
	if err != nil {
		t.Fatalf("LoadCredentials: %v", err)
	}
	if c.Method != MethodConsumerKey {
		t.Errorf("Method=%v want consumer_key", c.Method)
	}
	if c.ApplicationKey != "ak1" || c.ApplicationSecret != "as1" || c.ConsumerKey != "ck1" {
		t.Errorf("env creds not propagated: %+v", c)
	}
}

func TestLoadCredentials_Env_RegionMismatchFallsThrough(t *testing.T) {
	sandbox(t)
	t.Setenv("OVH_REGION", "ovh-us")
	t.Setenv("OVH_APPLICATION_KEY", "ak1")

	_, err := LoadCredentials(context.Background(), "ovh-eu", "default")
	if !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("got %v; want ErrNotConfigured (env region mismatch should fall through)", err)
	}
}

func TestStoreCredentials_RoundTrip(t *testing.T) {
	confPath := sandbox(t)
	want := Credentials{
		Region:            "ovh-eu",
		Profile:           "default",
		Method:            MethodConsumerKey,
		ApplicationKey:    "ak2",
		ApplicationSecret: "as2",
		ConsumerKey:       "ck2",
	}
	if err := StoreCredentials(context.Background(), "ovh-eu", "default", want); err != nil {
		t.Fatalf("StoreCredentials: %v", err)
	}

	info, err := os.Stat(confPath)
	if err != nil {
		t.Fatalf("stat ovh.conf: %v", err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Errorf("ovh.conf mode=%o want 0600 (PRD-04 file-mode registry)", mode)
	}

	got, err := LoadCredentials(context.Background(), "ovh-eu", "default")
	if err != nil {
		t.Fatalf("LoadCredentials: %v", err)
	}
	if got.ApplicationKey != want.ApplicationKey || got.ConsumerKey != want.ConsumerKey {
		t.Errorf("round-trip mismatch:\n got=%+v\nwant=%+v", got, want)
	}
}

func TestStoreCredentials_RegionMismatchRejected(t *testing.T) {
	sandbox(t)
	err := StoreCredentials(context.Background(), "ovh-eu", "default", Credentials{Region: "ovh-us", Method: MethodConsumerKey, ApplicationKey: "ak"})
	if err == nil {
		t.Fatal("expected error when creds.Region mismatches argument region")
	}
}

func TestStoreCredentials_ZeroRejected(t *testing.T) {
	sandbox(t)
	if err := StoreCredentials(context.Background(), "ovh-eu", "default", Credentials{}); err == nil {
		t.Fatal("expected error when storing zero Credentials")
	}
}

func TestDeleteCredentials_RemovesSection(t *testing.T) {
	sandbox(t)
	c := Credentials{Region: "ovh-eu", Method: MethodConsumerKey, ApplicationKey: "ak", ConsumerKey: "ck"}
	if err := StoreCredentials(context.Background(), "ovh-eu", "default", c); err != nil {
		t.Fatalf("Store: %v", err)
	}
	if err := DeleteCredentials(context.Background(), "ovh-eu", "default"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	_, err := LoadCredentials(context.Background(), "ovh-eu", "default")
	if !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("after Delete: got %v; want ErrNotConfigured", err)
	}
}

func TestDeleteCredentials_IdempotentOnEmpty(t *testing.T) {
	sandbox(t)
	if err := DeleteCredentials(context.Background(), "ovh-eu", "default"); err != nil {
		t.Errorf("Delete on empty: got %v; want nil", err)
	}
}
