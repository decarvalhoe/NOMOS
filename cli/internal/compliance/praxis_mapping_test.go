package compliance

// NRT-017 (#661) — the mapping is checked against the atom set, both ways:
// the mapping cannot certify what Nomos has not approved, cannot drift from
// the atom's exposed fields, cannot leak internal fields, and Praxis cannot
// become the authority.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	mappingFixture = "../../../specs/examples/nomos-praxis-mapping.valid.json"
	atomsFixture   = "../../../tests/fixtures/praxis/gxp-atoms.json"
)

func loadMapping(t *testing.T) PraxisAtomMapping {
	t.Helper()
	m, err := LoadPraxisMapping(mappingFixture)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestPraxisMappingFixtureVerifies(t *testing.T) {
	rep, err := VerifyPraxisMapping(loadMapping(t), atomsFixture)
	if err != nil {
		t.Fatal(err)
	}
	if rep.AtomsMapped != 3 || rep.AtomsInSet != 8 || rep.ChecksBound != 4 {
		t.Fatalf("report: %+v", rep)
	}
	if !strings.Contains(rep.ClaimBoundary, "never") || !strings.HasPrefix(rep.MappingDigest, "sha256:") {
		t.Fatalf("claim boundary / digest: %+v", rep)
	}
}

func TestPraxisMappingNegativeFixtures(t *testing.T) {
	// Internal fields are refused at LOAD: the struct is closed.
	_, err := LoadPraxisMapping("../../../specs/examples/nomos-praxis-mapping.invalid-internal-field.json")
	wantPraxis(t, err, CodePraxisMapShape, "block_id")
	for name, want := range map[string][2]string{
		"nomos-praxis-mapping.invalid-praxis-authority.json": {CodePraxisMapAuthority, "downstream consumer"},
		"nomos-praxis-mapping.invalid-unapproved.json":       {CodePraxisMapUnapproved, "only approved atoms cross"},
	} {
		m, err := LoadPraxisMapping(filepath.Join("../../../specs/examples", name))
		if err != nil {
			t.Fatal(err)
		}
		_, err = VerifyPraxisMapping(m, atomsFixture)
		wantPraxis(t, err, want[0], want[1])
	}
}

// atomsWith writes a copy of the atom set with edit applied and returns its path.
func atomsWith(t *testing.T, edit func(set map[string]any)) string {
	t.Helper()
	raw, _ := os.ReadFile(atomsFixture)
	var set map[string]any
	json.Unmarshal(raw, &set)
	edit(set)
	out, _ := json.Marshal(set)
	p := filepath.Join(t.TempDir(), "atoms.json")
	os.WriteFile(p, out, 0o644)
	return p
}

func firstMappedAtom(t *testing.T, set map[string]any, id string) map[string]any {
	t.Helper()
	for _, a := range set["atoms"].([]any) {
		if am := a.(map[string]any); am["id"] == id {
			return am
		}
	}
	t.Fatalf("atom %s not in set", id)
	return nil
}

func TestPraxisMappingRules(t *testing.T) {
	base := loadMapping(t)
	id := base.Atoms[0].NomosAtomID
	rehash := func(m *PraxisAtomMapping, atoms string) {
		raw, _ := os.ReadFile(atoms)
		m.FeedRef.Sha256 = "sha256:" + sha256Hex(raw)[7:]
	}
	cases := []struct {
		name  string
		mut   func(m *PraxisAtomMapping)
		atoms func(t *testing.T) string
		code  string
		frag  string
	}{
		{"schema", func(m *PraxisAtomMapping) { m.SchemaVersion = "v0" }, nil, CodePraxisMapSchema, "schema_version"},
		{"authority praxis", func(m *PraxisAtomMapping) { m.Authority = "praxis" }, nil, CodePraxisMapAuthority, "authority"},
		{"authority shared", func(m *PraxisAtomMapping) { m.Authority = "shared" }, nil, CodePraxisMapAuthority, "authority"},
		{"no atoms", func(m *PraxisAtomMapping) { m.Atoms = nil }, nil, CodePraxisMapShape, "maps nothing"},
		{"feed hash stale", func(m *PraxisAtomMapping) { m.FeedRef.Sha256 = "sha256:" + strings.Repeat("0", 64) }, nil, CodePraxisMapFeedHash, "does not match"},
		{"atom mapped twice", func(m *PraxisAtomMapping) { m.Atoms[1].NomosAtomID = m.Atoms[0].NomosAtomID }, nil, CodePraxisMapShape, "twice"},
		{"atom not in set", func(m *PraxisAtomMapping) { m.Atoms[0].NomosAtomID = "A-0000000000000000" }, nil, CodePraxisMapAtomAbsent, "not in the atom set"},
		{"content hash drift", func(m *PraxisAtomMapping) { m.Atoms[0].ContentHash = "sha256:" + strings.Repeat("a", 64) }, nil, CodePraxisMapAtomHash, "atom set has"},
		{"canonical ref drift", func(m *PraxisAtomMapping) { m.Atoms[0].CanonicalRef = "other/ref" }, nil, CodePraxisMapFieldDrift, "canonical_ref"},
		{"type drift", func(m *PraxisAtomMapping) { m.Atoms[0].AtomType = "definition" }, nil, CodePraxisMapFieldDrift, "atom_type"},
		{"source line drift", func(m *PraxisAtomMapping) { m.Atoms[0].SourceLine = 999 }, nil, CodePraxisMapFieldDrift, "source_line"},
		{"no checks", func(m *PraxisAtomMapping) { m.Atoms[0].PraxisChecks = nil }, nil, CodePraxisMapShape, "no Praxis check"},
		{"check without test id", func(m *PraxisAtomMapping) { m.Atoms[0].PraxisChecks[0].TestID = "" }, nil, CodePraxisMapShape, "scenario_id/test_id"},
		{"mapping approved but atom set says pending", nil, func(t *testing.T) string {
			return atomsWith(t, func(set map[string]any) { firstMappedAtom(t, set, id)["review_state"] = "pending" })
		}, CodePraxisMapUnapproved, "cannot certify"},
		{"atom content changed after mapping", nil, func(t *testing.T) string {
			return atomsWith(t, func(set map[string]any) {
				firstMappedAtom(t, set, id)["content_hash"] = "sha256:" + strings.Repeat("b", 64)
			})
		}, CodePraxisMapAtomHash, id},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := loadMapping(t)
			atoms := atomsFixture
			if tc.atoms != nil {
				atoms = tc.atoms(t)
				rehash(&m, atoms) // the mapping honestly names the (mutated) set; the atom-level rule must still bite
			}
			if tc.mut != nil {
				tc.mut(&m)
			}
			_, err := VerifyPraxisMapping(m, atoms)
			wantPraxis(t, err, tc.code, tc.frag)
		})
	}
}
