package schema

import (
	"reflect"
	"testing"
	"time"
)

// stubQuerier is a minimal Querier for diff tests; only Paths matters here.
type stubQuerier struct{ paths []string }

func (s *stubQuerier) Paths() []string                  { return s.paths }
func (s *stubQuerier) Region() string                   { return "stub" }
func (s *stubQuerier) FetchedAt() time.Time             { return time.Time{} }
func (s *stubQuerier) Stale() bool                      { return false }
func (s *stubQuerier) HasPath(string, string) bool      { return false }
func (s *stubQuerier) Describe(string) (PathSpec, bool) { return nil, false }
func (s *stubQuerier) Search(string) []string           { return nil }

func TestCompare(t *testing.T) {
	a := &stubQuerier{paths: []string{"/cloud", "/dedicated", "/vps"}}
	b := &stubQuerier{paths: []string{"/cloud", "/me", "/vps"}}
	got := Compare(a, b)
	want := Diff{
		OnlyInA: []string{"/dedicated"},
		OnlyInB: []string{"/me"},
		Common:  []string{"/cloud", "/vps"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Compare mismatch:\n got=%+v\nwant=%+v", got, want)
	}
}

func TestCompare_BothEmpty(t *testing.T) {
	got := Compare(&stubQuerier{}, &stubQuerier{})
	if got.OnlyInA != nil || got.OnlyInB != nil || got.Common != nil {
		t.Errorf("Compare(empty,empty)=%+v want all nil", got)
	}
}

func TestCompare_DisjointSets(t *testing.T) {
	a := &stubQuerier{paths: []string{"/a"}}
	b := &stubQuerier{paths: []string{"/b"}}
	got := Compare(a, b)
	if len(got.Common) != 0 {
		t.Errorf("Common=%v want empty", got.Common)
	}
	if !reflect.DeepEqual(got.OnlyInA, []string{"/a"}) {
		t.Errorf("OnlyInA=%v want [/a]", got.OnlyInA)
	}
	if !reflect.DeepEqual(got.OnlyInB, []string{"/b"}) {
		t.Errorf("OnlyInB=%v want [/b]", got.OnlyInB)
	}
}

func TestCompare_IdenticalSetsHaveOnlyCommon(t *testing.T) {
	a := &stubQuerier{paths: []string{"/x", "/y"}}
	b := &stubQuerier{paths: []string{"/x", "/y"}}
	got := Compare(a, b)
	if len(got.OnlyInA) != 0 || len(got.OnlyInB) != 0 {
		t.Errorf("expected all common; got %+v", got)
	}
	if !reflect.DeepEqual(got.Common, []string{"/x", "/y"}) {
		t.Errorf("Common=%v want [/x /y]", got.Common)
	}
}
