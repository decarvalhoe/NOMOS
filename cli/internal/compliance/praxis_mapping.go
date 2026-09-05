package compliance

// NRT-017 (#661) — the atom → Praxis check mapping, verified against the atom
// set it claims to map. The mapping is downstream evidence design: every mapped
// atom must exist in the Nomos atom set with the same content hash, canonical
// ref, type, domain and source position, must be approved on BOTH sides, and
// only the fields docs/regulated/customer-integration/praxis-atom-mapping.md
// exposes may appear (block_id, parent_id, depth are refused at load).
// Nomos stays the authority: `authority: nomos` is the only accepted value.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	PraxisMappingSchema        = "nomos-praxis-atom-mapping-v1"
	PraxisMappingClaimBoundary = "A mapping from approved Nomos atoms to Praxis checks, verified against the atom set " +
		"it names. It demonstrates the evidence flow on a fixture; it claims no runtime assurance, and it never " +
		"lets Praxis redefine, certify or replace a Nomos atom."
)

const (
	CodePraxisMapSchema     = "PRAXIS_MAP_SCHEMA_INVALID"
	CodePraxisMapShape      = "PRAXIS_MAP_SHAPE_INVALID"
	CodePraxisMapAuthority  = "PRAXIS_MAP_AUTHORITY_INVERTED"
	CodePraxisMapFeedHash   = "PRAXIS_MAP_FEED_HASH_MISMATCH"
	CodePraxisMapAtomAbsent = "PRAXIS_MAP_ATOM_NOT_IN_SET"
	CodePraxisMapAtomHash   = "PRAXIS_MAP_ATOM_HASH_MISMATCH"
	CodePraxisMapUnapproved = "PRAXIS_MAP_ATOM_NOT_APPROVED"
	CodePraxisMapFieldDrift = "PRAXIS_MAP_EXPOSED_FIELD_DRIFT"
)

// PraxisCheck is one Praxis check bound to an atom.
type PraxisCheck struct {
	ScenarioID         string   `yaml:"scenario_id" json:"scenario_id"`
	TestID             string   `yaml:"test_id" json:"test_id"`
	RuntimeEvidenceIDs []string `yaml:"runtime_evidence_ids" json:"runtime_evidence_ids"`
}

// MappedAtom carries exactly the exposed atom fields plus its checks.
type MappedAtom struct {
	NomosAtomID        string        `yaml:"nomos_atom_id" json:"nomos_atom_id"`
	CanonicalRef       string        `yaml:"canonical_ref" json:"canonical_ref"`
	AtomType           string        `yaml:"atom_type" json:"atom_type"`
	ContentHash        string        `yaml:"content_hash" json:"content_hash"`
	CertificationState string        `yaml:"certification_state" json:"certification_state"`
	Domain             string        `yaml:"domain" json:"domain"`
	SourceFile         string        `yaml:"source_file" json:"source_file"`
	SourceLine         int           `yaml:"source_line" json:"source_line"`
	PraxisChecks       []PraxisCheck `yaml:"praxis_checks" json:"praxis_checks"`
}

// PraxisAtomMapping is the whole document.
type PraxisAtomMapping struct {
	SchemaVersion string `yaml:"schema_version" json:"schema_version"`
	MappingID     string `yaml:"mapping_id" json:"mapping_id"`
	GeneratedAt   string `yaml:"generated_at" json:"generated_at"`
	FeedRef       struct {
		Path   string `yaml:"path" json:"path"`
		Sha256 string `yaml:"sha256" json:"sha256"`
	} `yaml:"feed_ref" json:"feed_ref"`
	Authority     string       `yaml:"authority" json:"authority"`
	Atoms         []MappedAtom `yaml:"atoms" json:"atoms"`
	ClaimBoundary string       `yaml:"claim_boundary" json:"claim_boundary"`
}

// atomSetView is the part of a Nomos atom set the mapping is checked against.
type atomSetView struct {
	DocumentRef string `json:"document_ref"`
	SourceFile  string `json:"source_file"`
	SourceHash  string `json:"source_hash"`
	Atoms       []struct {
		ID           string `json:"id"`
		CanonicalRef string `json:"canonical_ref"`
		Type         string `json:"type"`
		ContentHash  string `json:"content_hash"`
		ReviewState  string `json:"review_state"`
		Domain       string `json:"domain"`
		SourceSpan   struct {
			File      string `json:"file"`
			StartLine int    `json:"start_line"`
		} `json:"source_span"`
	} `json:"atoms"`
}

func praxisMapErr(code, format string, args ...any) error {
	return &PraxisError{Code: code, Message: fmt.Sprintf(format, args...)}
}

// LoadPraxisMapping reads YAML or JSON; unknown fields are refused — that is
// how internal Nomos fields are kept from crossing.
func LoadPraxisMapping(path string) (PraxisAtomMapping, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return PraxisAtomMapping{}, praxisMapErr(CodePraxisMapShape, "read mapping: %v", err)
	}
	var m PraxisAtomMapping
	dec := yaml.NewDecoder(strings.NewReader(string(raw)))
	dec.KnownFields(true)
	if err := dec.Decode(&m); err != nil {
		return PraxisAtomMapping{}, praxisMapErr(CodePraxisMapShape, "parse mapping: %v", err)
	}
	return m, nil
}

// PraxisMappingReport is the verifier's output.
type PraxisMappingReport struct {
	SchemaVersion string   `json:"schema_version"`
	MappingID     string   `json:"mapping_id"`
	AtomSet       string   `json:"atom_set"`
	AtomSetSha256 string   `json:"atom_set_sha256"`
	AtomsInSet    int      `json:"atoms_in_set"`
	AtomsMapped   int      `json:"atoms_mapped"`
	ChecksBound   int      `json:"checks_bound"`
	Checks        []string `json:"checks"`
	MappingDigest string   `json:"mapping_digest"`
	ClaimBoundary string   `json:"claim_boundary"`
}

// VerifyPraxisMapping checks the mapping against the atom set at atomsPath.
func VerifyPraxisMapping(m PraxisAtomMapping, atomsPath string) (PraxisMappingReport, error) {
	rep := PraxisMappingReport{SchemaVersion: "nomos-praxis-mapping-report-v1", MappingID: m.MappingID, AtomSet: atomsPath, ClaimBoundary: PraxisMappingClaimBoundary}
	if m.SchemaVersion != PraxisMappingSchema {
		return rep, praxisMapErr(CodePraxisMapSchema, "schema_version %q, want %q", m.SchemaVersion, PraxisMappingSchema)
	}
	if !praxisIdent.MatchString(m.MappingID) || !praxisTime.MatchString(m.GeneratedAt) {
		return rep, praxisMapErr(CodePraxisMapShape, "mapping_id and generated_at (RFC3339 UTC) are required")
	}
	if m.Authority != "nomos" {
		return rep, praxisMapErr(CodePraxisMapAuthority, "authority %q; Nomos is the only authority for an atom, Praxis is a downstream consumer", m.Authority)
	}
	if len(strings.Fields(m.ClaimBoundary)) < 6 {
		return rep, praxisMapErr(CodePraxisMapShape, "claim_boundary must be a real sentence")
	}
	if len(m.Atoms) == 0 {
		return rep, praxisMapErr(CodePraxisMapShape, "a mapping without atoms maps nothing")
	}
	rep.Checks = append(rep.Checks, "schema", "authority")

	raw, err := os.ReadFile(atomsPath)
	if err != nil {
		return rep, praxisMapErr(CodePraxisMapFeedHash, "atom set: %v", err)
	}
	sum := sha256.Sum256(raw)
	rep.AtomSetSha256 = "sha256:" + hex.EncodeToString(sum[:])
	if strings.TrimSpace(m.FeedRef.Path) == "" || m.FeedRef.Sha256 != rep.AtomSetSha256 {
		return rep, praxisMapErr(CodePraxisMapFeedHash, "feed_ref.sha256 %s does not match the atom set %s (%s)", m.FeedRef.Sha256, atomsPath, rep.AtomSetSha256)
	}
	var set atomSetView
	if err := json.Unmarshal(raw, &set); err != nil {
		return rep, praxisMapErr(CodePraxisMapShape, "atom set is not a Nomos atom set: %v", err)
	}
	byID := map[string]int{}
	for i, a := range set.Atoms {
		byID[a.ID] = i
	}
	rep.AtomsInSet = len(set.Atoms)
	rep.Checks = append(rep.Checks, "feed_hash")

	seen := map[string]bool{}
	checks := 0
	for _, ma := range m.Atoms {
		if !praxisIdent.MatchString(ma.NomosAtomID) || seen[ma.NomosAtomID] {
			return rep, praxisMapErr(CodePraxisMapShape, "nomos_atom_id %q missing, malformed or mapped twice", ma.NomosAtomID)
		}
		seen[ma.NomosAtomID] = true
		if ma.CertificationState != "approved" {
			return rep, praxisMapErr(CodePraxisMapUnapproved, "atom %s: certification_state %q — only approved atoms cross the boundary", ma.NomosAtomID, ma.CertificationState)
		}
		i, ok := byID[ma.NomosAtomID]
		if !ok {
			return rep, praxisMapErr(CodePraxisMapAtomAbsent, "atom %s is not in the atom set %s", ma.NomosAtomID, atomsPath)
		}
		a := set.Atoms[i]
		if a.ReviewState != "approved" {
			return rep, praxisMapErr(CodePraxisMapUnapproved, "atom %s is %q in the Nomos atom set; the mapping cannot certify it", ma.NomosAtomID, a.ReviewState)
		}
		if !praxisSha256.MatchString(ma.ContentHash) || ma.ContentHash != a.ContentHash {
			return rep, praxisMapErr(CodePraxisMapAtomHash, "atom %s: mapping says %s, atom set has %s", ma.NomosAtomID, ma.ContentHash, a.ContentHash)
		}
		for field, pair := range map[string][2]string{
			"canonical_ref": {ma.CanonicalRef, a.CanonicalRef},
			"atom_type":     {ma.AtomType, a.Type},
			"domain":        {ma.Domain, a.Domain},
			"source_file":   {ma.SourceFile, a.SourceSpan.File},
		} {
			if pair[0] != pair[1] {
				return rep, praxisMapErr(CodePraxisMapFieldDrift, "atom %s: %s is %q in the mapping and %q in the atom set", ma.NomosAtomID, field, pair[0], pair[1])
			}
		}
		if ma.SourceLine != a.SourceSpan.StartLine {
			return rep, praxisMapErr(CodePraxisMapFieldDrift, "atom %s: source_line is %d in the mapping and %d in the atom set", ma.NomosAtomID, ma.SourceLine, a.SourceSpan.StartLine)
		}
		if len(ma.PraxisChecks) == 0 {
			return rep, praxisMapErr(CodePraxisMapShape, "atom %s: mapped to no Praxis check", ma.NomosAtomID)
		}
		for _, c := range ma.PraxisChecks {
			if !praxisIdent.MatchString(c.ScenarioID) || !praxisIdent.MatchString(c.TestID) {
				return rep, praxisMapErr(CodePraxisMapShape, "atom %s: a check lacks scenario_id/test_id", ma.NomosAtomID)
			}
			for _, e := range c.RuntimeEvidenceIDs {
				if !praxisIdent.MatchString(e) {
					return rep, praxisMapErr(CodePraxisMapShape, "atom %s: runtime evidence id %q malformed", ma.NomosAtomID, e)
				}
			}
			checks++
		}
	}
	rep.AtomsMapped, rep.ChecksBound = len(m.Atoms), checks
	rep.Checks = append(rep.Checks, "atoms_exist", "atoms_approved_both_sides", "content_hashes", "exposed_fields", "checks")
	canon, _ := json.Marshal(m)
	d := sha256.Sum256(canon)
	rep.MappingDigest = "sha256:" + hex.EncodeToString(d[:])
	sort.Strings(rep.Checks)
	return rep, nil
}
