package auth

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stuffbucket/ovh-cli/internal/xdgpaths"
)

// envSandboxKeys lists the OVH_* vars that may leak in from a developer's
// shell. sandbox() unsets them and restores at test end.
var envSandboxKeys = []string{
	"OVH_REGION",
	"OVH_APPLICATION_KEY",
	"OVH_APPLICATION_SECRET",
	"OVH_CONSUMER_KEY",
	"OVH_CLIENT_ID",
	"OVH_CLIENT_SECRET",
}

// sandbox isolates the test from the user's real $HOME / $XDG_CONFIG_HOME /
// OVH_* env. Returns the path where ovh.conf will be written by
// StoreCredentials.
//
// MUTATES PROCESS-GLOBAL ENV. Tests using sandbox MUST NOT call t.Parallel().
func sandbox(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	xdgConfig := filepath.Join(root, "config")
	t.Setenv("HOME", root)
	t.Setenv("XDG_CONFIG_HOME", xdgConfig)

	// t.Setenv("X", "") sets the var to empty, which is NOT the same as
	// unset for callers that distinguish via os.LookupEnv. Use real unset
	// + t.Cleanup to restore the original (or absence of) value.
	for _, k := range envSandboxKeys {
		if orig, ok := os.LookupEnv(k); ok {
			t.Cleanup(func() { _ = os.Setenv(k, orig) })
		} else {
			t.Cleanup(func() { _ = os.Unsetenv(k) })
		}
		_ = os.Unsetenv(k)
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

func TestLoadCredentials_PanicsOnEmptyRegion(t *testing.T) {
	sandbox(t)
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on empty region")
		}
	}()
	_, _ = LoadCredentials(context.Background(), "", "default")
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

func TestLoadCredentials_EmptySection(t *testing.T) {
	confPath := sandbox(t)
	if err := os.MkdirAll(filepath.Dir(confPath), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// Write a section header with no recognized keys; readCredsFromConf
	// must treat this as "no creds" and LoadCredentials must return
	// ErrNotConfigured rather than a non-zero Credentials with MethodNone.
	if err := os.WriteFile(confPath, []byte("[ovh-eu]\n"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, err := LoadCredentials(context.Background(), "ovh-eu", "default")
	if !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("got %v; want ErrNotConfigured for empty section", err)
	}
}

func TestStoreCredentials_RoundTrip_ClassicCK(t *testing.T) {
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
	if got.Method != MethodConsumerKey {
		t.Errorf("got.Method=%v want consumer_key", got.Method)
	}
}

func TestStoreCredentials_RoundTrip_OAuth2(t *testing.T) {
	sandbox(t)
	want := Credentials{
		Region:       "ovh-eu",
		Profile:      "default",
		Method:       MethodOAuth2,
		ClientID:     "cid",
		ClientSecret: "csecret",
	}
	if err := StoreCredentials(context.Background(), "ovh-eu", "default", want); err != nil {
		t.Fatalf("Store: %v", err)
	}
	got, err := LoadCredentials(context.Background(), "ovh-eu", "default")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Method != MethodOAuth2 {
		t.Errorf("Method=%v want oauth2", got.Method)
	}
	if got.ClientID != "cid" || got.ClientSecret != "csecret" {
		t.Errorf("OAuth2 fields not propagated: %+v", got)
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
