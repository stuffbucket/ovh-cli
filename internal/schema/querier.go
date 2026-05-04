package schema

import "time"

// Querier is the read-only view over a cached apispace.json. It is satisfied
// by the result of OpenCache and by test stubs.
//
// PRD-05 §Querier interface; all method contracts live there.
type Querier interface {
	// HasPath reports whether the spec contains the given (method, path).
	HasPath(method, path string) bool

	// Describe returns the methods-by-name map for the given path, or
	// (nil, false) if the path is unknown.
	Describe(path string) (PathSpec, bool)

	// Search returns all paths starting with the given prefix, sorted.
	Search(prefix string) []string

	// Paths returns every path declared in the cache, sorted.
	Paths() []string

	// Region returns the region id this cache covers.
	Region() string

	// FetchedAt is the wall-clock time when Refresh last produced this cache.
	FetchedAt() time.Time

	// Stale reports whether FetchedAt is older than the configured TTL.
	// A stale cache is still served — the runner decides whether to call
	// Refresh based on this signal.
	Stale() bool
}
