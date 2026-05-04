package schema

import (
	"bytes"
	"context"
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

func TestRefreshFrom_PanicsOnEmptyEndpoint(t *testing.T) {
	defer expectPanic(t, "endpoint")
	_ = refreshFrom(context.Background(), "test", "", []string{"example"}, t.TempDir(), nil)
}

func TestRefreshFrom_PanicsOnNilContext(t *testing.T) {
	defer expectPanic(t, "ctx")
	//nolint:staticcheck // intentionally passing nil ctx to trigger the documented panic
	_ = refreshFrom(nil, "test", "https://example/1.0", []string{"example"}, t.TempDir(), nil)
}

func TestRefreshFrom_AtomicOnFetchFailure(t *testing.T) {
	dir := copyFixture(t)
	preMeta, _ := os.ReadFile(filepath.Join(dir, "apispace.meta.json"))
	preSpec, _ := os.ReadFile(filepath.Join(dir, "apispace.json"))

	err := refreshFrom(context.Background(), "ovh-eu", "https://127.0.0.1:1/1.0", []string{"127.0.0.1:1"}, dir, nil)
	if err == nil {
		t.Fatal("refreshFrom should fail against unreachable endpoint")
	}

	postMeta, _ := os.ReadFile(filepath.Join(dir, "apispace.meta.json"))
	postSpec, _ := os.ReadFile(filepath.Join(dir, "apispace.json"))
	if !bytes.Equal(preMeta, postMeta) {
		t.Error("apispace.meta.json mutated on failed Refresh")
	}
	if !bytes.Equal(preSpec, postSpec) {
		t.Error("apispace.json mutated on failed Refresh")
	}
}

// TestRefreshFrom_ETagRoundTrip exercises the PRD-05 §Fetch contract:
// "Honor If-None-Match from previous etag; on 304, skip merge and bump
// fetched_at." The first refresh primes the cache with a server-supplied
// ETag; the second refresh should hit the 304 path, leave apispace.json
// byte-identical, and only advance FetchedAt in apispace.meta.json.
func TestRefreshFrom_ETagRoundTrip(t *testing.T) {
	const etag = `W/"v1"`
	srv := newSchemaServerWithETag(t, etag)
	defer srv.Close()
	u, _ := url.Parse(srv.URL)
	tr := srv.Client().Transport
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatalf("chmod: %v", err)
	}

	if err := refreshFrom(context.Background(), "test", srv.URL+"/1.0", []string{u.Host}, dir, tr); err != nil {
		t.Fatalf("first refresh: %v", err)
	}
	firstSpec, _ := os.ReadFile(filepath.Join(dir, "apispace.json"))
	firstMeta := readMetaForTest(t, dir)
	if firstMeta.ETag != etag {
		t.Fatalf("first.ETag=%q want %q", firstMeta.ETag, etag)
	}

	// Sleep a beat so FetchedAt can advance even on coarse clocks.
	time.Sleep(10 * time.Millisecond)

	if err := refreshFrom(context.Background(), "test", srv.URL+"/1.0", []string{u.Host}, dir, tr); err != nil {
		t.Fatalf("second refresh: %v", err)
	}
	secondSpec, _ := os.ReadFile(filepath.Join(dir, "apispace.json"))
	secondMeta := readMetaForTest(t, dir)

	if !bytes.Equal(firstSpec, secondSpec) {
		t.Error("apispace.json mutated on 304 response")
	}
	if !secondMeta.FetchedAt.After(firstMeta.FetchedAt) {
		t.Errorf("FetchedAt did not advance: first=%v second=%v", firstMeta.FetchedAt, secondMeta.FetchedAt)
	}
	if secondMeta.ETag != etag {
		t.Errorf("second.ETag=%q want %q (304 path should preserve)", secondMeta.ETag, etag)
	}
}

func readMetaForTest(t *testing.T, dir string) Meta {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, "apispace.meta.json"))
	if err != nil {
		t.Fatalf("read meta: %v", err)
	}
	var m Meta
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("parse meta: %v", err)
	}
	return m
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
