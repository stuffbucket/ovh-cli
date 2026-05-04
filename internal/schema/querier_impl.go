package schema

import (
	"sort"
	"strings"
	"time"
)

// cachedQuerier satisfies Querier. Constructed only by openCacheAt, which
// guarantees both services and now are non-nil at construction; the methods
// below assume that invariant and do not paper over it with late nil checks.
type cachedQuerier struct {
	meta     Meta
	services map[string]Service
	now      func() time.Time
}

func (q *cachedQuerier) Region() string       { return q.meta.Region }
func (q *cachedQuerier) FetchedAt() time.Time { return q.meta.FetchedAt }

func (q *cachedQuerier) Stale() bool {
	return q.now().Sub(q.meta.FetchedAt) > DefaultCacheTTL
}

func (q *cachedQuerier) HasPath(method, path string) bool {
	spec, ok := q.lookupPath(path)
	if !ok {
		return false
	}
	_, ok = spec[strings.ToUpper(method)]
	return ok
}

func (q *cachedQuerier) Describe(path string) (PathSpec, bool) {
	return q.lookupPath(path)
}

func (q *cachedQuerier) lookupPath(path string) (PathSpec, bool) {
	for _, svc := range q.services {
		if ps, ok := svc.Paths[path]; ok {
			return ps, true
		}
	}
	return nil, false
}

// Paths returns every path declared in the cache, sorted ascending.
func (q *cachedQuerier) Paths() []string { return q.collectPaths("") }

// Search returns paths whose names start with prefix, sorted ascending.
// Empty result => nil. PRD-05 §Querier interface.
func (q *cachedQuerier) Search(prefix string) []string { return q.collectPaths(prefix) }

// collectPaths walks services once, filters by optional prefix, sorts ascending.
// Empty prefix means "all paths".
func (q *cachedQuerier) collectPaths(prefix string) []string {
	var out []string
	for _, svc := range q.services {
		for p := range svc.Paths {
			if prefix == "" || strings.HasPrefix(p, prefix) {
				out = append(out, p)
			}
		}
	}
	sort.Strings(out)
	return out
}
