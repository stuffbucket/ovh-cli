package schema

import (
	"sort"
	"strings"
	"time"
)

// cachedQuerier satisfies Querier. Constructed only by openCacheAt, which
// guarantees services is non-nil and parsed (any decode failure becomes
// ErrCacheCorrupt at open time, never at query time).
type cachedQuerier struct {
	meta     Meta
	services map[string]Service
	now      func() time.Time
}

func (q *cachedQuerier) Region() string       { return q.meta.Region }
func (q *cachedQuerier) FetchedAt() time.Time { return q.meta.FetchedAt }

func (q *cachedQuerier) Stale() bool {
	now := time.Now
	if q.now != nil {
		now = q.now
	}
	return now().Sub(q.meta.FetchedAt) > CacheTTL
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

func (q *cachedQuerier) Paths() []string {
	out := make([]string, 0, len(q.services)*4) // 4 paths/service is a coarse but reasonable hint
	for _, svc := range q.services {
		for p := range svc.Paths {
			out = append(out, p)
		}
	}
	sort.Strings(out)
	return out
}

func (q *cachedQuerier) Search(prefix string) []string {
	var out []string
	for _, p := range q.Paths() {
		if strings.HasPrefix(p, prefix) {
			out = append(out, p)
		}
	}
	return out
}
