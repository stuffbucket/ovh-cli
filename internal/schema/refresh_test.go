package schema

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRefreshFrom_HappyPath(t *testing.T) {
	srv := newSchemaServer(t)
	defer srv.Close()
	u, _ := url.Parse(srv.URL)
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatalf("chmod: %v", err)
	}

	if err := refreshFrom(context.Background(), "test", srv.URL+"/1.0", []string{u.Host}, dir, srv.Client().Transport); err != nil {
		t.Fatalf("refreshFrom: %v", err)
	}

	for _, name := range []string{"apispace.json", "apispace.meta.json"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatalf("missing %s: %v", name, err)
		}
	}

	q, err := openCacheAt(dir, "test", freshClock)
	if err != nil {
		t.Fatalf("openCacheAt: %v", err)
	}
	if !q.HasPath("GET", "/cloud/project") {
		t.Error("expected /cloud/project GET in re-opened cache")
	}
	if !q.HasPath("POST", "/cloud/project/{serviceName}/instance") {
		t.Error("expected /cloud/project/{serviceName}/instance POST")
	}
}

func TestRefreshFrom_PanicsOnEmptyAllowedHosts(t *testing.T) {
	defer expectPanic(t, "allowedHosts")
	_ = refreshFrom(context.Background(), "test", "https://example/1.0", nil, t.TempDir(), nil)
}

func TestRefreshFrom_PanicsOnEmptyRegion(t *testing.T) {
	defer expectPanic(t, "region")
	_ = refreshFrom(context.Background(), "", "https://example/1.0", []string{"example"}, t.TempDir(), nil)
}

func TestRefreshFrom_PanicsOnNilContext(t *testing.T) {
	defer expectPanic(t, "ctx")
	//nolint:staticcheck // intentionally passing nil ctx to trigger the documented panic
	_ = refreshFrom(nil, "test", "https://example/1.0", []string{"example"}, t.TempDir(), nil)
}

func TestRefreshFrom_AtomicOnFetchFailure(t *testing.T) {
	// Set up a cache dir with valid pre-existing files.
	dir := copyFixture(t)

	// Point Refresh at an unreachable endpoint; assertHostAllowed succeeds
	// (host matches) but the actual HTTP call fails. Existing files should
	// be untouched.
	preMeta, _ := os.ReadFile(filepath.Join(dir, "apispace.meta.json"))
	preSpec, _ := os.ReadFile(filepath.Join(dir, "apispace.json"))

	err := refreshFrom(context.Background(), "ovh-eu", "https://127.0.0.1:1/1.0", []string{"127.0.0.1:1"}, dir, nil)
	if err == nil {
		t.Fatal("refreshFrom should fail against unreachable endpoint")
	}

	postMeta, _ := os.ReadFile(filepath.Join(dir, "apispace.meta.json"))
	postSpec, _ := os.ReadFile(filepath.Join(dir, "apispace.json"))

	if string(preMeta) != string(postMeta) {
		t.Error("apispace.meta.json mutated on failed Refresh")
	}
	if string(preSpec) != string(postSpec) {
		t.Error("apispace.json mutated on failed Refresh")
	}
}

func TestRefresh_PanicsOnEmptyEndpoint(t *testing.T) {
	defer expectPanic(t, "endpoint")
	_ = refreshFrom(context.Background(), "test", "", []string{"example"}, t.TempDir(), nil)
}

func expectPanic(t *testing.T, wantSubstr string) {
	t.Helper()
	r := recover()
	if r == nil {
		t.Fatalf("expected panic containing %q", wantSubstr)
	}
	if msg, _ := r.(string); !strings.Contains(msg, wantSubstr) {
		t.Fatalf("panic %q did not contain %q", msg, wantSubstr)
	}
}
