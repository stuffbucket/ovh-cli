package schema

import (
	"encoding/json"
	"sort"
	"strings"
	"time"
)

// cachedQuerier satisfies Querier by holding apispace.json in memory and
// answering queries by walking a single decoded map. The decode is one-shot
// at OpenCache time; subsequent queries are pure lookups.
//
// gjson is used elsewhere in this package for streaming reads of large
// fixtures; for runtime queries an explicit decoded map is simpler and the
// spec is small enough (<5 MB) that holding it decoded is cheap.
type cachedQuerier struct {
	meta   Meta
	spec   []byte
	now    func() time.Time
	parsed *parsedSpec // lazy
}

type parsedSpec struct {
	services map[string]Service
}

func (q *cachedQuerier) ensureParsed() error {
	if q.parsed != nil {
		return nil
	}
	var as APISpace
	if err := json.Unmarshal(q.spec, &as); err != nil {
		return ErrCacheCorrupt
	}
	q.parsed = &parsedSpec{services: as.Services}
	return nil
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
	if q.ensureParsed() != nil {
		return nil, false
	}
	for _, svc := range q.parsed.services {
		if ps, ok := svc.Paths[path]; ok {
			return ps, true
		}
	}
	return nil, false
}

func (q *cachedQuerier) Paths() []string {
	if q.ensureParsed() != nil {
		return nil
	}
	out := make([]string, 0, 32)
	for _, svc := range q.parsed.services {
		for p := range svc.Paths {
			out = append(out, p)
		}
	}
	sort.Strings(out)
	return out
}

func (q *cachedQuerier) Search(prefix string) []string {
	out := make([]string, 0)
	for _, p := range q.Paths() {
		if strings.HasPrefix(p, prefix) {
			out = append(out, p)
		}
	}
	return out
}
