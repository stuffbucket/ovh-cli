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

// newSchemaServer returns an httptest.Server that serves the fetch_*.json
// fixtures at /1.0/. Path /1.0/?format=json -> fetch_index.json; per-service
// /1.0/<svc>?format=json -> fetch_service_<svc>.json.
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

	as, err := fetchAPISpace(context.Background(), "test", srv.URL+"/1.0", []string{u.Host}, tr)
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if as.Region != "test" {
		t.Errorf("Region=%q want test", as.Region)
	}
	if got, want := len(as.Services), 2; got != want {
		t.Fatalf("services count=%d want %d", got, want)
	}
	cloud, ok := as.Services["/cloud"]
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
	_, err := fetchAPISpace(context.Background(), "test", srv.URL+"/1.0", []string{"not-the-host.example"}, tr)
	if err == nil || !strings.Contains(err.Error(), "not in allowedHosts") {
		t.Fatalf("got %v; want allowedHosts rejection", err)
	}
}

func TestFetchAPISpace_NonHTTPSEndpointRejected(t *testing.T) {
	_, err := fetchAPISpace(context.Background(), "test", "http://insecure.example/1.0", []string{"insecure.example"}, nil)
	if err == nil || !strings.Contains(err.Error(), "not https") {
		t.Fatalf("got %v; want https rejection", err)
	}
}

func TestFetchAPISpace_PerServiceFailureNonFatal(t *testing.T) {
	srv := newSchemaServer(t, "/me")
	defer srv.Close()
	u, _ := url.Parse(srv.URL)
	tr := srv.Client().Transport
	as, err := fetchAPISpace(context.Background(), "test", srv.URL+"/1.0", []string{u.Host}, tr)
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if as.Services["/me"].Error == "" {
		t.Error("/me should have error recorded")
	}
	if cloud := as.Services["/cloud"]; cloud.Error != "" {
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
	_, err := fetchAPISpace(ctx, "test", srv.URL+"/1.0", []string{u.Host}, tr)
	if err == nil {
		t.Fatal("fetch should fail on cancelled ctx")
	}
}
