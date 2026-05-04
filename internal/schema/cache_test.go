package schema

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// fixtureTime is the FetchedAt baked into testdata/apispace.meta.json.
var fixtureTime = time.Date(2026, 5, 4, 0, 0, 0, 0, time.UTC)

// freshClock reports a wall-clock 1 second past fixtureTime so the cache is
// fresh (Stale=false) for tests that don't care about staleness.
var freshClock = frozenNow(fixtureTime.Add(time.Second))

// frozenNow returns a clock that always reports t. The shared helper keeps
// every test deterministic — no test should accept time.Now as the clock.
func frozenNow(t time.Time) func() time.Time { return func() time.Time { return t } }

// copyFixture lays the testdata fixtures into a fresh tempdir at mode 0700,
// satisfying PRD-04 §Canonical file-mode registry.
func copyFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatalf("chmod tempdir: %v", err)
	}
	for _, name := range []string{"apispace.json", "apispace.meta.json"} {
		b, err := os.ReadFile(filepath.Join("testdata", name))
		if err != nil {
			t.Fatalf("read fixture %s: %v", name, err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), b, 0o600); err != nil {
			t.Fatalf("write fixture %s: %v", name, err)
		}
	}
	return dir
}

func TestOpenCache_PanicsOnEmptyRegion(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for empty region")
		}
	}()
	_, _ = openCacheAt(t.TempDir(), "", freshClock)
}

func TestOpenCache_NoMetaReturnsErrNoCache(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	_, err := openCacheAt(dir, "ovh-eu", freshClock)
	if !errors.Is(err, ErrNoCache) {
		t.Fatalf("got %v; want ErrNoCache", err)
	}
}

func TestOpenCache_LooserDirModeRefused(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX file mode bits not meaningful on Windows")
	}
	dir := copyFixture(t)
	if err := os.Chmod(dir, 0o755); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	_, err := openCacheAt(dir, "ovh-eu", freshClock)
	if !errors.Is(err, ErrCachePerms) {
		t.Fatalf("got %v; want ErrCachePerms", err)
	}
}

func TestOpenCache_RegionMismatch(t *testing.T) {
	dir := copyFixture(t)
	_, err := openCacheAt(dir, "ovh-us", freshClock)
	if !errors.Is(err, ErrCacheCorrupt) {
		t.Fatalf("got %v; want ErrCacheCorrupt", err)
	}
}

func TestOpenCache_CorruptMeta(t *testing.T) {
	dir := copyFixture(t)
	if err := os.WriteFile(filepath.Join(dir, "apispace.meta.json"), []byte("{ not json"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, err := openCacheAt(dir, "ovh-eu", freshClock)
	if !errors.Is(err, ErrCacheCorrupt) {
		t.Fatalf("got %v; want ErrCacheCorrupt", err)
	}
}

func TestOpenCache_CorruptSpec(t *testing.T) {
	dir := copyFixture(t)
	if err := os.WriteFile(filepath.Join(dir, "apispace.json"), []byte("{ not json"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, err := openCacheAt(dir, "ovh-eu", freshClock)
	if !errors.Is(err, ErrCacheCorrupt) {
		t.Fatalf("got %v; want ErrCacheCorrupt", err)
	}
}

func TestOpenCache_HappyPath(t *testing.T) {
	q := loadFixture(t)
	if got, want := q.Region(), "ovh-eu"; got != want {
		t.Fatalf("Region=%q want %q", got, want)
	}
	if !q.FetchedAt().Equal(fixtureTime) {
		t.Fatalf("FetchedAt=%v want %v", q.FetchedAt(), fixtureTime)
	}
	if q.Stale() {
		t.Fatalf("Stale()=true within 24h")
	}
}

func TestQuerier_HasPath(t *testing.T) {
	q := loadFixture(t)
	cases := []struct {
		method, path string
		want         bool
	}{
		{"GET", "/cloud/project", true},
		{"get", "/cloud/project", true}, // case-insensitive
		{"DELETE", "/cloud/project", false},
		{"GET", "/cloud/project/{serviceName}/instance", true},
		{"POST", "/cloud/project/{serviceName}/instance", true},
		{"GET", "/me", true},
		{"GET", "/nonexistent", false},
	}
	for _, c := range cases {
		if got := q.HasPath(c.method, c.path); got != c.want {
			t.Errorf("HasPath(%q, %q) = %v; want %v", c.method, c.path, got, c.want)
		}
	}
}

func TestQuerier_Describe(t *testing.T) {
	q := loadFixture(t)
	ps, ok := q.Describe("/cloud/project/{serviceName}/instance")
	if !ok {
		t.Fatal("path not found")
	}
	if _, ok := ps["GET"]; !ok {
		t.Fatal("GET method missing")
	}
	if got, want := ps["GET"].Description, "List instances"; got != want {
		t.Errorf("Description=%q want %q", got, want)
	}
	if _, ok := q.Describe("/nonexistent"); ok {
		t.Errorf("Describe('/nonexistent') ok=true; want false")
	}
}

// fixturePathCount is the number of paths declared in
// testdata/apispace.json. Update if the fixture changes.
const fixturePathCount = 3

func TestQuerier_Paths(t *testing.T) {
	q := loadFixture(t)
	paths := q.Paths()
	if len(paths) != fixturePathCount {
		t.Fatalf("Paths len=%d want %d (got %v)", len(paths), fixturePathCount, paths)
	}
}

func TestQuerier_Search(t *testing.T) {
	q := loadFixture(t)
	hits := q.Search("/cloud/")
	if len(hits) != 2 {
		t.Fatalf("Search('/cloud/') len=%d want 2 (got %v)", len(hits), hits)
	}
	if hits := q.Search("/missing"); hits != nil {
		t.Errorf("Search('/missing') = %v; want nil", hits)
	}
}

func TestQuerier_Stale(t *testing.T) {
	dir := copyFixture(t)
	// Cross the TTL by clocking forward 25h past FetchedAt.
	clock := frozenNow(fixtureTime.Add(25 * time.Hour))
	q, err := openCacheAt(dir, "ovh-eu", clock)
	if err != nil {
		t.Fatalf("OpenCache: %v", err)
	}
	if !q.Stale() {
		t.Fatal("Stale()=false past 24h")
	}
}

func loadFixture(t *testing.T) Querier {
	t.Helper()
	dir := copyFixture(t)
	q, err := openCacheAt(dir, "ovh-eu", freshClock)
	if err != nil {
		t.Fatalf("OpenCache: %v", err)
	}
	return q
}
