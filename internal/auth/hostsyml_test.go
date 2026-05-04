package auth

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/zalando/go-keyring"

	"github.com/stuffbucket/ovh-cli/internal/xdgpaths"
)

func TestHostsYML_RoundTrip_KeyringMode(t *testing.T) {
	sandbox(t)
	expiry := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)
	want := Credentials{
		Region: "ovh-eu", Profile: "default", Method: MethodOAuth2,
		ClientID: "cid", ClientSecret: "csecret",
		AccessToken: "AT-SECRET", RefreshToken: "RT-SECRET", Expiry: expiry,
	}
	if err := StoreCredentials(context.Background(), "ovh-eu", "default", want); err != nil {
		t.Fatalf("Store: %v", err)
	}

	hostsPath := xdgpaths.HostsFile()
	info, err := os.Stat(hostsPath)
	if err != nil {
		t.Fatalf("stat hosts.yml: %v", err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Errorf("hosts.yml mode=%o want 0600", mode)
	}

	body, _ := os.ReadFile(hostsPath)
	if !contains(body, "access_token: keyring:ovh-eu:default:access_token") {
		t.Errorf("hosts.yml missing access_token placeholder:\n%s", body)
	}
	if contains(body, "AT-SECRET") {
		t.Error("hosts.yml leaked plaintext access_token (keyring mode)")
	}

	got, err := LoadCredentials(context.Background(), "ovh-eu", "default")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.AccessToken != "AT-SECRET" || got.RefreshToken != "RT-SECRET" {
		t.Errorf("OAuth2 round-trip mismatch: got AT=%q RT=%q", got.AccessToken, got.RefreshToken)
	}
	if !got.Expiry.Equal(expiry) {
		t.Errorf("Expiry=%v want %v", got.Expiry, expiry)
	}
}

func TestHostsYML_RoundTrip_FileMode(t *testing.T) {
	confPath := sandbox(t)
	if err := os.MkdirAll(filepath.Dir(confPath), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(confPath, []byte("[default]\nstorage = file\n"), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}

	want := Credentials{
		Region: "ovh-eu", Profile: "default", Method: MethodOAuth2,
		ClientID: "cid", ClientSecret: "csecret",
		AccessToken: "AT-PLAIN", RefreshToken: "RT-PLAIN",
		Expiry: time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC),
	}
	if err := StoreCredentials(context.Background(), "ovh-eu", "default", want); err != nil {
		t.Fatalf("Store: %v", err)
	}

	body, _ := os.ReadFile(xdgpaths.HostsFile())
	if !contains(body, "access_token: AT-PLAIN") {
		t.Errorf("hosts.yml missing plaintext access_token in file mode:\n%s", body)
	}
	if contains(body, "keyring:") {
		t.Errorf("hosts.yml has keyring placeholder in file mode:\n%s", body)
	}

	got, err := LoadCredentials(context.Background(), "ovh-eu", "default")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.AccessToken != "AT-PLAIN" {
		t.Errorf("AT mismatch: got %q want AT-PLAIN", got.AccessToken)
	}
}

func TestHostsYML_RefuseLooserMode(t *testing.T) {
	confPath := sandbox(t)
	if err := os.MkdirAll(filepath.Dir(confPath), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	hostsPath := xdgpaths.HostsFile()
	body := "version: 1\nhosts:\n  ovh-eu:\n    profiles:\n      default:\n        access_token: AT\n"
	if err := os.WriteFile(hostsPath, []byte(body), 0o644); err != nil { // looser than 0600
		t.Fatalf("write hosts: %v", err)
	}
	// Seed an OAuth2 ovh.conf so LoadCredentials reaches hosts.yml.
	confBody := "[default]\nstorage = file\n\n[ovh-eu]\nauth_method = oauth2\nclient_id = cid\nclient_secret = csecret\n"
	if err := os.WriteFile(confPath, []byte(confBody), 0o600); err != nil {
		t.Fatalf("write conf: %v", err)
	}
	_, err := LoadCredentials(context.Background(), "ovh-eu", "default")
	if !errors.Is(err, ErrFileModeUnsafe) {
		t.Fatalf("got %v; want ErrFileModeUnsafe (hosts.yml looser than 0600)", err)
	}
}

func TestHostsYML_ClassicCKDoesNotWriteHosts(t *testing.T) {
	sandbox(t)
	c := Credentials{Region: "ovh-eu", Profile: "default", Method: MethodConsumerKey, ApplicationKey: "ak", ConsumerKey: "ck"}
	if err := StoreCredentials(context.Background(), "ovh-eu", "default", c); err != nil {
		t.Fatalf("Store: %v", err)
	}
	if _, err := os.Stat(xdgpaths.HostsFile()); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("classic-CK Store should not create hosts.yml; stat err=%v", err)
	}
}

func TestHostsYML_PreservesOtherProfiles(t *testing.T) {
	sandbox(t)
	expiry := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)
	a := Credentials{
		Region: "ovh-eu", Profile: "default", Method: MethodOAuth2,
		ClientID: "cid", ClientSecret: "cs", AccessToken: "AT-A", RefreshToken: "RT-A", Expiry: expiry,
	}
	b := Credentials{
		Region: "ovh-eu", Profile: "ci-bot", Method: MethodOAuth2,
		ClientID: "cid", ClientSecret: "cs", AccessToken: "AT-B", RefreshToken: "RT-B", Expiry: expiry,
	}
	if err := StoreCredentials(context.Background(), "ovh-eu", "default", a); err != nil {
		t.Fatalf("Store a: %v", err)
	}
	if err := StoreCredentials(context.Background(), "ovh-eu", "ci-bot", b); err != nil {
		t.Fatalf("Store b: %v", err)
	}
	// Both profiles should round-trip from hosts.yml.
	gotA, err := LoadCredentials(context.Background(), "ovh-eu", "default")
	if err != nil {
		t.Fatalf("Load a: %v", err)
	}
	if gotA.AccessToken != "AT-A" {
		t.Errorf("default.AT=%q want AT-A", gotA.AccessToken)
	}
	gotB, err := LoadCredentials(context.Background(), "ovh-eu", "ci-bot")
	if err != nil {
		t.Fatalf("Load b: %v", err)
	}
	if gotB.AccessToken != "AT-B" {
		t.Errorf("ci-bot.AT=%q want AT-B", gotB.AccessToken)
	}
}

func TestHostsYML_DeleteCredentialsClearsHostsAndKeyring(t *testing.T) {
	sandbox(t)
	c := Credentials{
		Region: "ovh-eu", Profile: "default", Method: MethodOAuth2,
		ClientID: "cid", ClientSecret: "cs", AccessToken: "AT", RefreshToken: "RT",
	}
	if err := StoreCredentials(context.Background(), "ovh-eu", "default", c); err != nil {
		t.Fatalf("Store: %v", err)
	}
	if err := DeleteCredentials(context.Background(), "ovh-eu", "default"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	// hosts.yml: profile entry gone (or whole file removed if last entry)
	if hf, err := loadHostsYML(); err != nil {
		t.Fatalf("loadHostsYML: %v", err)
	} else if hf != nil {
		if _, ok := hf.Hosts["ovh-eu"].Profiles["default"]; ok {
			t.Error("hosts.yml still has [ovh-eu].default after Delete")
		}
	}
	// keyring access_token + refresh_token gone
	if _, err := keyring.Get(keyringService, "ovh-eu:default:access_token"); !errors.Is(err, keyring.ErrNotFound) {
		t.Errorf("keyring access_token still present: %v", err)
	}
}

func TestHostsYML_LoadNoFileReturnsOK(t *testing.T) {
	sandbox(t)
	c := Credentials{Region: "ovh-eu", Profile: "default"}
	ok, err := readHostsCreds(&c, "ovh-eu", "default")
	if err != nil {
		t.Fatalf("readHostsCreds with no file: %v", err)
	}
	if ok {
		t.Error("readHostsCreds(no file) returned ok=true")
	}
}
