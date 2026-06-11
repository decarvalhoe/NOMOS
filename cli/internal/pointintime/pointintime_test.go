package pointintime

import "testing"

// VRC-12 (#558, A3) — the resolver selects the expression in force on the
// project date and REFUSES (not_in_force) for a stale or future date, so the
// engine never cites a version that was not in force.

func atom(id, work, from, to, expr, hash string) Atom {
	a := Atom{AtomID: id}
	a.SourceSpan.Hash = hash
	a.Metadata.Temporal = Temporal{WorkID: work, ExpressionID: expr, EffectiveFrom: from, EffectiveTo: to}
	return a
}

func corpus() []Atom {
	return []Atom{
		atom("v1", "LAT", "2014-05-01", "2019-12-31", "exp-2014", "sha256:aa"),
		atom("v2", "LAT", "2020-01-01", "", "exp-2020", "sha256:bb"),         // open-ended
		atom("other", "RPGA", "2018-01-01", "", "exp-rpga", "sha256:cc"),     // different work
	}
}

func mustResolve(t *testing.T, work, asOf string) Result {
	t.Helper()
	r, err := ResolvePointInTime(corpus(), work, asOf)
	if err != nil {
		t.Fatalf("resolve %s @ %s: %v", work, asOf, err)
	}
	return r
}

func TestPIT_SelectsTheVersionInForce(t *testing.T) {
	r := mustResolve(t, "LAT", "2016-06-15")
	if r.Status != "resolved" || r.SelectedAtomID != "v1" || r.ExpressionID != "exp-2014" {
		t.Fatalf("expected v1 (2014 expression), got %+v", r)
	}
	r2 := mustResolve(t, "LAT", "2024-01-01")
	if r2.Status != "resolved" || r2.SelectedAtomID != "v2" {
		t.Fatalf("expected v2 (open-ended 2020 expression), got %+v", r2)
	}
}

func TestPIT_LatestEffectiveFromWinsOnOverlap(t *testing.T) {
	// Two versions both in force on the date → the later effective_from wins.
	atoms := []Atom{
		atom("old", "LAT", "2014-05-01", "2025-12-31", "exp-old", "sha256:11"),
		atom("new", "LAT", "2020-01-01", "", "exp-new", "sha256:22"),
	}
	r, err := ResolvePointInTime(atoms, "LAT", "2022-03-03")
	if err != nil {
		t.Fatal(err)
	}
	if r.SelectedAtomID != "new" {
		t.Fatalf("the latest in-force expression must win, got %+v", r)
	}
}

func TestPIT_FutureDateIsNotInForce(t *testing.T) {
	r := mustResolve(t, "LAT", "2010-01-01") // before any effective_from
	if r.Status != "not_in_force" {
		t.Fatalf("a date before any version must be not_in_force, got %+v", r)
	}
	if r.SelectedAtomID != "" {
		t.Fatal("not_in_force must select nothing")
	}
}

func TestPIT_StaleDateBetweenVersionsIsNotInForce(t *testing.T) {
	// v1 ends 2019-12-31, v2 starts 2020-01-01 — there is no gap here, so use a
	// corpus with a real gap to prove the stale-window refusal.
	atoms := []Atom{
		atom("v1", "LAT", "2014-05-01", "2018-12-31", "exp-2014", "sha256:aa"),
		atom("v2", "LAT", "2020-01-01", "", "exp-2020", "sha256:bb"),
	}
	r, err := ResolvePointInTime(atoms, "LAT", "2019-06-01") // in the gap
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != "not_in_force" {
		t.Fatalf("a date in a coverage gap must be not_in_force, got %+v", r)
	}
}

func TestPIT_OtherWorkIdsAreIgnored(t *testing.T) {
	r := mustResolve(t, "RPGA", "2024-01-01")
	if r.Status != "resolved" || r.SelectedAtomID != "other" {
		t.Fatalf("must resolve within the requested work id, got %+v", r)
	}
}

func TestPIT_MalformedDateErrors(t *testing.T) {
	if _, err := ResolvePointInTime(corpus(), "LAT", "not-a-date"); err == nil {
		t.Fatal("a malformed as_of must error, never silently resolve")
	}
}
