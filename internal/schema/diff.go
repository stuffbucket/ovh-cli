package schema

import "sort"

// Diff is a path-level diff between two cached schemas. Used by
// `ovh schema diff <region-a> <region-b>` to surface region capability
// divergence (PRD-05 §Commands).
type Diff struct {
	OnlyInA []string
	OnlyInB []string
	Common  []string
}

// Compare returns the path-level differences between two Queriers. Each
// returned slice is sorted ascending; nil for empty results (idiomatic Go).
func Compare(a, b Querier) Diff {
	pa := setOf(a.Paths())
	pb := setOf(b.Paths())
	var d Diff
	for p := range pa {
		if _, ok := pb[p]; ok {
			d.Common = append(d.Common, p)
		} else {
			d.OnlyInA = append(d.OnlyInA, p)
		}
	}
	for p := range pb {
		if _, ok := pa[p]; !ok {
			d.OnlyInB = append(d.OnlyInB, p)
		}
	}
	sort.Strings(d.OnlyInA)
	sort.Strings(d.OnlyInB)
	sort.Strings(d.Common)
	return d
}

func setOf(s []string) map[string]struct{} {
	out := make(map[string]struct{}, len(s))
	for _, v := range s {
		out[v] = struct{}{}
	}
	return out
}
