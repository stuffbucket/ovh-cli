package schema

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// newSchemaServer returns an httptest.NewTLSServer that serves the
// fetch_*.json fixtures at /1.0/. /1.0/ -> fetch_index.json; per-service
// /1.0/<svc>?format=json -> fetch_service_<svc>.json. Services in
// badServices return 500.
func newSchemaServer(t *testing.T, badServices ...string) *httptest.Server {
	t.Helper()
	bad := setOf(badServices)
	mux := http.NewServeMux()
	mux.HandleFunc("/1.0/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/1.0")
		if path == "" || path == "/" {
			writeFixture(t, w, "fetch_index.json")
			return
		}
		if _, fail := bad[path]; fail {
			http.Error(w, "fixture: forced failure", http.StatusInternalServerError)
			return
		}
		writeFixture(t, w, "fetch_service"+strings.ReplaceAll(path, "/", "_")+".json")
	})
	return httptest.NewTLSServer(mux)
}

// newSchemaServerWithETag returns a server that responds 200 + ETag the
// first time and 304 when the request's If-None-Match equals etag. Per-
// service GETs always return 200; ETag handling is index-only (PRD-05).
func newSchemaServerWithETag(t *testing.T, etag string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/1.0/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/1.0")
		if path == "" || path == "/" {
			w.Header().Set("ETag", etag)
			if r.Header.Get("If-None-Match") == etag {
				w.WriteHeader(http.StatusNotModified)
				return
			}
			writeFixture(t, w, "fetch_index.json")
			return
		}
		writeFixture(t, w, "fetch_service"+strings.ReplaceAll(path, "/", "_")+".json")
	})
	return httptest.NewTLSServer(mux)
}

func writeFixture(t *testing.T, w http.ResponseWriter, name string) {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(b); err != nil {
		t.Fatalf("write response: %v", err)
	}
}

func TestFetchAPISpace_HappyPath(t *testing.T) {
	srv := newSchemaServer(t)
	defer srv.Close()
	u, _ := url.Parse(srv.URL)
	tr := srv.Client().Transport

	res, err := fetchAPISpace(context.Background(), "test", srv.URL+"/1.0", []string{u.Host}, tr, "")
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if !res.Modified {
		t.Fatal("Modified=false on first fetch")
	}
	if res.Space.Region != "test" {
		t.Errorf("Region=%q want test", res.Space.Region)
	}
	if got, want := len(res.Space.Services), 2; got != want {
		t.Fatalf("services count=%d want %d", got, want)
	}
	cloud, ok := res.Space.Services["/cloud"]
	if !ok {
		t.Fatal("/cloud service missing")
	}
	if cloud.Error != "" {
		t.Errorf("/cloud.Error=%q want empty", cloud.Error)
	}
	if _, ok := cloud.Paths["/cloud/project"]; !ok {
		t.Error("/cloud/project path missing")
	}
}

func TestFetchAPISpace_HostNotAllowed(t *testing.T) {
	srv := newSchemaServer(t)
	defer srv.Close()
	tr := srv.Client().Transport
	_, err := fetchAPISpace(context.Background(), "test", srv.URL+"/1.0", []string{"not-the-host.example"}, tr, "")
	if err == nil || !strings.Contains(err.Error(), "not in allowedHosts") {
		t.Fatalf("got %v; want allowedHosts rejection", err)
	}
}

func TestFetchAPISpace_NonHTTPSEndpointRejected(t *testing.T) {
	_, err := fetchAPISpace(context.Background(), "test", "http://insecure.example/1.0", []string{"insecure.example"}, nil, "")
	if err == nil || !strings.Contains(err.Error(), "not https") {
		t.Fatalf("got %v; want https rejection", err)
	}
}

func TestFetchAPISpace_PerServiceFailureNonFatal(t *testing.T) {
	srv := newSchemaServer(t, "/me")
	defer srv.Close()
	u, _ := url.Parse(srv.URL)
	tr := srv.Client().Transport
	res, err := fetchAPISpace(context.Background(), "test", srv.URL+"/1.0", []string{u.Host}, tr, "")
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if res.Space.Services["/me"].Error == "" {
		t.Error("/me should have error recorded")
	}
	if cloud := res.Space.Services["/cloud"]; cloud.Error != "" {
		t.Errorf("/cloud should have succeeded, got error: %q", cloud.Error)
	}
}

func TestFetchAPISpace_CancelledContext(t *testing.T) {
	srv := newSchemaServer(t)
	defer srv.Close()
	u, _ := url.Parse(srv.URL)
	tr := srv.Client().Transport
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := fetchAPISpace(ctx, "test", srv.URL+"/1.0", []string{u.Host}, tr, "")
	if err == nil {
		t.Fatal("fetch should fail on cancelled ctx")
	}
}

func TestFetchAPISpace_ETagNotModified(t *testing.T) {
	const etag = `W/"v1"`
	srv := newSchemaServerWithETag(t, etag)
	defer srv.Close()
	u, _ := url.Parse(srv.URL)
	tr := srv.Client().Transport

	res, err := fetchAPISpace(context.Background(), "test", srv.URL+"/1.0", []string{u.Host}, tr, etag)
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if res.Modified {
		t.Error("Modified=true on matching If-None-Match; want 304 path")
	}
	if res.Space != nil {
		t.Error("Space non-nil on 304 path")
	}
	if res.ETag != etag {
		t.Errorf("ETag=%q want %q", res.ETag, etag)
	}
}
