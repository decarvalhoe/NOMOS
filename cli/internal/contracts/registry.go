// Package contracts verifies the contract stability registry (NRT-023 #676):
// every machine contract under specs/ is registered with a declared stability
// and version; a stable contract cannot change without an accepted bump, must
// have a valid fixture, and its compatibility fixtures must be READ by the Go
// reader that owns them — the read is the proof. Nothing here decides what is
// stable; it refuses what contradicts the declaration.
package contracts

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/RBOKproject/Nomos/cli/internal/atomization"
	"github.com/RBOKproject/Nomos/cli/internal/bundle"
	"github.com/RBOKproject/Nomos/cli/internal/canon"
	"github.com/RBOKproject/Nomos/cli/internal/checks"
	"github.com/RBOKproject/Nomos/cli/internal/compliance"
	"github.com/RBOKproject/Nomos/cli/internal/corpus"
	"github.com/RBOKproject/Nomos/cli/internal/domainpack"
	"github.com/RBOKproject/Nomos/cli/internal/output"
	"github.com/RBOKproject/Nomos/cli/internal/pointintime"
	"github.com/RBOKproject/Nomos/cli/internal/validate"
)

const (
	RegistrySchema = "nomos-contract-registry-v1"
	RegistryPath   = "specs/contract-registry.yaml"
	ClaimBoundary  = "Declared stability of the machine contracts under specs/, verified: registration, hash-at-version, " +
		"fixtures, deprecation dates and compatibility reads. It says nothing about the semantic correctness of a contract."
)

const (
	CodeRegistryInvalid       = "CONTRACT_REGISTRY_INVALID"
	CodeUnregistered          = "CONTRACT_UNREGISTERED"
	CodeFileMissing           = "CONTRACT_FILE_MISSING"
	CodeChangedWithoutBump    = "CONTRACT_CHANGED_WITHOUT_BUMP"
	CodeVersionMismatch       = "CONTRACT_VERSION_MISMATCH"
	CodeStableWithoutVersion  = "CONTRACT_STABLE_WITHOUT_VERSION"
	CodeStableWithoutFixture  = "CONTRACT_STABLE_WITHOUT_FIXTURE"
	CodeFixtureMissing        = "CONTRACT_FIXTURE_MISSING"
	CodeDeprecatedWithoutDate = "CONTRACT_DEPRECATED_WITHOUT_DATES"
	CodeDeprecatedPastRemoval = "CONTRACT_DEPRECATED_PAST_REMOVAL"
	CodeCompatUnread          = "CONTRACT_COMPAT_FIXTURE_UNREADABLE"
	CodeCompatUnknownReader   = "CONTRACT_COMPAT_READER_UNKNOWN"
	CodeBumpRefused           = "CONTRACT_BUMP_REFUSED"
)

// Error is a named refusal.
type Error struct {
	Code    string
	Message string
}

func (e *Error) Error() string { return e.Code + ": " + e.Message }

func refuse(code, format string, args ...any) *Error {
	return &Error{Code: code, Message: fmt.Sprintf(format, args...)}
}

// CompatFixture is a fixture of a given schema version that a Go reader must still read.
type CompatFixture struct {
	Path          string `yaml:"path" json:"path"`
	Reader        string `yaml:"reader" json:"reader"`
	SchemaVersion string `yaml:"schema_version" json:"schema_version"`
	// BaseDir resolves relative paths inside the fixture (source manifests);
	// repo-relative, default: the fixture's directory.
	BaseDir string `yaml:"base_dir,omitempty" json:"base_dir,omitempty"`
}

// ReaderFixture is a fixture a named reader (script or engine) accepts or
// rejects; it is not vetted with cue because its contract is the reader's.
type ReaderFixture struct {
	Path   string `yaml:"path" json:"path"`
	Reader string `yaml:"reader" json:"reader"`
}

// Contract is one registry entry.
type Contract struct {
	ID            string `yaml:"id" json:"id"`
	Path          string `yaml:"path" json:"path"`
	Stability     string `yaml:"stability" json:"stability"`
	VersionKind   string `yaml:"version_kind" json:"version_kind"`
	SchemaVersion string `yaml:"schema_version" json:"schema_version"`
	Pattern       string `yaml:"schema_version_pattern,omitempty" json:"schema_version_pattern,omitempty"`
	VersionField  string `yaml:"version_field,omitempty" json:"version_field,omitempty"`
	Sha256        string `yaml:"sha256" json:"sha256"`
	Definition    string `yaml:"definition" json:"definition"`
	Fixtures      struct {
		Valid         []string        `yaml:"valid" json:"valid"`
		Invalid       []string        `yaml:"invalid" json:"invalid"`
		ReaderValid   []ReaderFixture `yaml:"reader_valid,omitempty" json:"reader_valid,omitempty"`
		ReaderInvalid []ReaderFixture `yaml:"reader_invalid,omitempty" json:"reader_invalid,omitempty"`
	} `yaml:"fixtures" json:"fixtures"`
	DefinitionOverrides map[string]string `yaml:"definition_overrides,omitempty" json:"definition_overrides,omitempty"`
	Readers             []string          `yaml:"readers" json:"readers"`
	Writers             []string          `yaml:"writers,omitempty" json:"writers,omitempty"`
	CompatFixtures      []CompatFixture   `yaml:"compat_fixtures" json:"compat_fixtures"`
	DeprecatedSince     string            `yaml:"deprecated_since,omitempty" json:"deprecated_since,omitempty"`
	RemovalNotBefore    string            `yaml:"removal_not_before,omitempty" json:"removal_not_before,omitempty"`
	Note                string            `yaml:"note,omitempty" json:"note,omitempty"`
}

// Registry is the whole file.
type Registry struct {
	SchemaVersion string          `yaml:"schema_version" json:"schema_version"`
	ClaimBoundary string          `yaml:"claim_boundary" json:"claim_boundary"`
	Rules         map[string]bool `yaml:"rules" json:"rules"`
	Contracts     []Contract      `yaml:"contracts" json:"contracts"`
}

// Load reads the registry.
func Load(root string) (Registry, error) {
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(RegistryPath)))
	if err != nil {
		return Registry{}, refuse(CodeRegistryInvalid, "read registry: %v", err)
	}
	var r Registry
	dec := yaml.NewDecoder(strings.NewReader(string(raw)))
	dec.KnownFields(true)
	if err := dec.Decode(&r); err != nil {
		return Registry{}, refuse(CodeRegistryInvalid, "parse registry: %v", err)
	}
	if r.SchemaVersion != RegistrySchema {
		return r, refuse(CodeRegistryInvalid, "schema_version %q, want %q", r.SchemaVersion, RegistrySchema)
	}
	return r, nil
}

// Row is the verified state of one contract.
type Row struct {
	ID              string `json:"id"`
	Stability       string `json:"stability"`
	SchemaVersion   string `json:"schema_version"`
	Sha256          string `json:"sha256"`
	FileVersion     string `json:"file_version,omitempty"`
	ValidFixtures   int    `json:"valid_fixtures"`
	InvalidFixtures int    `json:"invalid_fixtures"`
	Readers         int    `json:"readers"`
	CompatReads     int    `json:"compat_reads"`
	Deprecated      bool   `json:"deprecated"`
}

// Report is the verifier's output.
type Report struct {
	SchemaVersion string         `json:"schema_version"`
	GeneratedAt   string         `json:"generated_at"`
	Registry      string         `json:"registry"`
	RegistrySha   string         `json:"registry_sha256"`
	Total         int            `json:"total"`
	ByStability   map[string]int `json:"by_stability"`
	CompatReads   int            `json:"compat_reads"`
	Rows          []Row          `json:"contracts"`
	Warnings      []string       `json:"warnings"`
	ClaimBoundary string         `json:"claim_boundary"`
}

var (
	defaultRe  = regexp.MustCompile(`schema_version:\s*string \| \*"([^"]+)"`)
	pinnedRe   = regexp.MustCompile(`schema_version:\s*"([^"]+)"`)
	constRefRe = regexp.MustCompile(`schema_version:\s*(#\w+)`)
	dateRe     = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
)

// fileVersion extracts the schema_version literal a CUE file declares, for
// pinned/default kinds; "" when the file carries none.
func fileVersion(text string, c Contract) string {
	if c.VersionKind == "pinned" && c.VersionField != "" {
		re := regexp.MustCompile(c.VersionField + `:\s*"([^"]+)"`)
		if m := re.FindStringSubmatch(text); m != nil {
			return m[1]
		}
	}
	if m := defaultRe.FindStringSubmatch(text); m != nil {
		return m[1]
	}
	if m := pinnedRe.FindStringSubmatch(text); m != nil {
		return m[1]
	}
	if m := constRefRe.FindStringSubmatch(text); m != nil {
		re := regexp.MustCompile(regexp.QuoteMeta(m[1]) + `:\s*"([^"]+)"`)
		if mm := re.FindStringSubmatch(text); mm != nil {
			return mm[1]
		}
	}
	return ""
}

// Verify applies every rule at repoRoot. It returns the first refusal.
func Verify(root string, now time.Time) (Report, error) {
	r, err := Load(root)
	if err != nil {
		return Report{}, err
	}
	rawReg, _ := os.ReadFile(filepath.Join(root, filepath.FromSlash(RegistryPath)))
	rep := Report{SchemaVersion: "nomos-contract-status-v1", GeneratedAt: now.UTC().Format("2006-01-02T15:04:05Z"), Registry: RegistryPath, RegistrySha: hashOf(rawReg), ByStability: map[string]int{}, ClaimBoundary: ClaimBoundary}
	seen := map[string]bool{}
	registered := map[string]bool{}
	for _, c := range r.Contracts {
		if c.ID == "" || seen[c.ID] {
			return rep, refuse(CodeRegistryInvalid, "contract id %q missing or duplicated", c.ID)
		}
		seen[c.ID] = true
		switch c.Stability {
		case "stable", "experimental", "deprecated":
		default:
			return rep, refuse(CodeRegistryInvalid, "%s: stability %q is not stable|experimental|deprecated", c.ID, c.Stability)
		}
		registered[c.Path] = true
		full := filepath.Join(root, filepath.FromSlash(c.Path))
		raw, err := os.ReadFile(full)
		if err != nil {
			return rep, refuse(CodeFileMissing, "%s: %s: %v", c.ID, c.Path, err)
		}
		row := Row{ID: c.ID, Stability: c.Stability, SchemaVersion: c.SchemaVersion, Sha256: c.Sha256, Readers: len(c.Readers), Deprecated: c.Stability == "deprecated"}
		// Hash at version: any change to the file is evidence-affecting and needs an accepted bump.
		if got := hashOf(raw); got != c.Sha256 {
			return rep, refuse(CodeChangedWithoutBump, "%s: %s changed (%s, registry has %s) without an accepted schema_version bump — run `nomos contracts status --accept %s --new-version <v>` after bumping the contract", c.ID, c.Path, got[:19], c.Sha256[:19], c.ID)
		}
		// Declared version must agree with the file where the file declares one.
		if fv := fileVersion(string(raw), c); fv != "" {
			row.FileVersion = fv
			if c.VersionKind == "pinned" || c.VersionKind == "default" {
				if fv != c.SchemaVersion {
					return rep, refuse(CodeVersionMismatch, "%s: file declares schema_version %q, registry says %q", c.ID, fv, c.SchemaVersion)
				}
			}
		}
		if c.Stability == "stable" && strings.TrimSpace(c.SchemaVersion) == "" {
			return rep, refuse(CodeStableWithoutVersion, "%s: a stable contract must carry a schema_version (pinned, default or registry_declared)", c.ID)
		}
		if c.Stability == "stable" && len(c.Fixtures.Valid) == 0 {
			return rep, refuse(CodeStableWithoutFixture, "%s: a stable contract must have at least one valid fixture", c.ID)
		}
		all := append(append([]string{}, c.Fixtures.Valid...), c.Fixtures.Invalid...)
		for _, rf := range append(append([]ReaderFixture{}, c.Fixtures.ReaderValid...), c.Fixtures.ReaderInvalid...) {
			if rf.Reader == "" {
				return rep, refuse(CodeRegistryInvalid, "%s: reader fixture %s names no reader", c.ID, rf.Path)
			}
			all = append(all, rf.Path, rf.Reader)
		}
		for f := range c.DefinitionOverrides {
			all = append(all, f)
		}
		all = append(all, c.Writers...)
		for _, f := range all {
			if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(f))); err != nil {
				return rep, refuse(CodeFixtureMissing, "%s: fixture %s: %v", c.ID, f, err)
			}
		}
		row.ValidFixtures, row.InvalidFixtures = len(c.Fixtures.Valid), len(c.Fixtures.Invalid)
		if c.Stability == "deprecated" {
			if !dateRe.MatchString(c.DeprecatedSince) || !dateRe.MatchString(c.RemovalNotBefore) {
				return rep, refuse(CodeDeprecatedWithoutDate, "%s: deprecated contracts need deprecated_since and removal_not_before (YYYY-MM-DD)", c.ID)
			}
			if c.RemovalNotBefore <= c.DeprecatedSince {
				return rep, refuse(CodeDeprecatedWithoutDate, "%s: removal_not_before must be after deprecated_since", c.ID)
			}
			if rm, err := time.Parse("2006-01-02", c.RemovalNotBefore); err == nil && now.After(rm) {
				return rep, refuse(CodeDeprecatedPastRemoval, "%s: removal_not_before %s has passed; remove the contract with a MAJOR bump or extend the date deliberately", c.ID, c.RemovalNotBefore)
			}
		}
		for _, cf := range c.CompatFixtures {
			if err := readCompat(root, cf); err != nil {
				return rep, err
			}
			row.CompatReads++
			rep.CompatReads++
		}
		rep.ByStability[c.Stability]++
		rep.Rows = append(rep.Rows, row)
	}
	// Every contract file under specs/ must be registered.
	matches, _ := filepath.Glob(filepath.Join(root, "specs", "*.cue"))
	more, _ := filepath.Glob(filepath.Join(root, "specs", "*.schema.json"))
	for _, m := range append(matches, more...) {
		rel := filepath.ToSlash(strings.TrimPrefix(m, filepath.Clean(root)+string(os.PathSeparator)))
		if !registered[rel] {
			return rep, refuse(CodeUnregistered, "%s is a contract file with no registry entry — register it with its stability and version", rel)
		}
	}
	sort.Slice(rep.Rows, func(i, j int) bool { return rep.Rows[i].ID < rep.Rows[j].ID })
	rep.Total = len(rep.Rows)
	rep.Warnings = DeprecationWarnings(r)
	if rep.Warnings == nil {
		rep.Warnings = []string{}
	}
	return rep, nil
}

// readCompat exercises the Go reader that owns the fixture's contract — the
// engine's real loader, never an ad hoc decoder — and compares the version it
// read with the registry's declared version where the document carries one.
func readCompat(root string, cf CompatFixture) error {
	full := filepath.Join(root, filepath.FromSlash(cf.Path))
	unread := func(err error) error { return refuse(CodeCompatUnread, "%s via %s: %v", cf.Path, cf.Reader, err) }
	version := func(got string) error {
		if got != cf.SchemaVersion {
			return refuse(CodeCompatUnread, "%s: read version %q, registry expects %q", cf.Path, got, cf.SchemaVersion)
		}
		return nil
	}
	switch cf.Reader {
	case "external-snapshot":
		env, records, err := corpus.LoadExternalSnapshot(full, "")
		if err != nil {
			return unread(err)
		}
		if err := version(env.Format); err != nil {
			return err
		}
		if _, err := corpus.VerifyExternalSnapshot(env, records); err != nil {
			return refuse(CodeCompatUnread, "%s: %v", cf.Path, err)
		}
	case "nomos-praxis-evidence":
		ex, err := compliance.LoadPraxisExchange(full)
		if err != nil {
			return unread(err)
		}
		return version(ex.SchemaVersion)
	case "nomos-praxis-mapping":
		m, err := compliance.LoadPraxisMapping(full)
		if err != nil {
			return unread(err)
		}
		return version(m.SchemaVersion)
	case "portfolio-status":
		raw, err := os.ReadFile(full)
		if err != nil {
			return refuse(CodeCompatUnread, "%s: %v", cf.Path, err)
		}
		// Decoded structurally (no import of the portfolio package: it imports this one).
		var st struct {
			SchemaVersion string `json:"schema_version"`
			StatusDigest  string `json:"status_digest"`
		}
		if err := json.Unmarshal(raw, &st); err != nil || st.SchemaVersion != cf.SchemaVersion || st.StatusDigest == "" {
			return refuse(CodeCompatUnread, "%s via %s: not a %s document (%v)", cf.Path, cf.Reader, cf.SchemaVersion, err)
		}
	case "canon-promotion":
		b, err := canon.LoadPromotionBundle(full)
		if err != nil {
			return unread(err)
		}
		if len(b.Atoms) == 0 {
			return unread(fmt.Errorf("bundle carries no atoms"))
		}
	case "canonical-knowledge-bundle":
		b, _, err := bundle.LoadFile(full)
		if err != nil {
			return unread(err)
		}
		if err := version(b.SchemaVersion); err != nil {
			return err
		}
		if err := b.Validate(); err != nil {
			return unread(err)
		}
	case "canonical-matrix":
		m, err := checks.ParseMatrixFile(full)
		if err != nil {
			return unread(err)
		}
		return version(m.SchemaVersion)
	case "corpus-body-ledger":
		l, err := corpus.LoadCorpusBodyLedger(full)
		if err != nil {
			return unread(err)
		}
		if err := corpus.VerifyCorpusBodyLedgerProofs(l); err != nil {
			return unread(err)
		}
	case "corpus-integrity-report":
		si, fq, err := corpus.LoadIntegrityReportFile(full)
		if err != nil {
			return unread(err)
		}
		if si == nil && fq == nil {
			return unread(fmt.Errorf("neither a source-integrity nor a feed-quality report"))
		}
	case "domain-pack":
		m, err := domainpack.LoadManifest(full)
		if err != nil {
			return unread(err)
		}
		return version(m.SchemaVersion)
	case "facets":
		if _, err := atomization.LoadFacetedDocument(full); err != nil {
			return unread(err)
		}
	case "knowledge-lens":
		if _, err := atomization.LoadKnowledgeLens(full); err != nil {
			return unread(err)
		}
	case "nomos-project":
		res := validate.ValidateFile(full)
		if !res.Valid || res.ManifestType != "nomos-project" {
			return unread(fmt.Errorf("validate: type %q valid=%v errors=%d", res.ManifestType, res.Valid, len(res.Errors)))
		}
	case "nomos-report":
		r, err := output.LoadReport(full)
		if err != nil {
			return unread(err)
		}
		return version(r.SchemaVersion)
	case "point-in-time":
		doc, err := pointintime.LoadAtomSet(full)
		if err != nil {
			return unread(err)
		}
		return version(doc.SchemaVersion)
	case "source-manifest":
		base := filepath.Dir(full)
		if cf.BaseDir != "" {
			base = filepath.Join(root, filepath.FromSlash(cf.BaseDir))
		}
		res, err := checks.CheckSources(full, base)
		if err != nil {
			return unread(err)
		}
		if !res.Valid {
			return unread(fmt.Errorf("source manifest does not pass the sources check"))
		}
	case "verdicts":
		doc, err := output.LoadVerdictCases(full)
		if err != nil {
			return unread(err)
		}
		return version(doc.SchemaVersion)
	default:
		return refuse(CodeCompatUnknownReader, "%s: reader %q is not a known Go reader (%s)", cf.Path, cf.Reader, strings.Join(KnownReaders, ", "))
	}
	return nil
}

// KnownReaders names every engine loader readCompat can exercise.
var KnownReaders = []string{"canon-promotion", "canonical-knowledge-bundle", "canonical-matrix", "corpus-body-ledger", "corpus-integrity-report",
	"domain-pack", "external-snapshot", "facets", "knowledge-lens", "nomos-praxis-evidence", "nomos-praxis-mapping", "nomos-project",
	"nomos-report", "point-in-time", "portfolio-status", "source-manifest", "verdicts"}

// AcceptBump records a deliberate contract change: the registry entry takes the
// new version and the file's current hash. For pinned/default kinds the file
// must already declare the new version; a bump to the same version is refused.
func AcceptBump(root, id, newVersion string) error {
	r, err := Load(root)
	if err != nil {
		return err
	}
	idx := -1
	for i, c := range r.Contracts {
		if c.ID == id {
			idx = i
		}
	}
	if idx < 0 {
		return refuse(CodeBumpRefused, "%s is not registered", id)
	}
	c := &r.Contracts[idx]
	if strings.TrimSpace(newVersion) == "" || newVersion == c.SchemaVersion {
		return refuse(CodeBumpRefused, "%s: a bump needs a NEW schema_version (registry has %q)", id, c.SchemaVersion)
	}
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(c.Path)))
	if err != nil {
		return refuse(CodeFileMissing, "%s: %v", id, err)
	}
	if c.VersionKind == "pinned" || c.VersionKind == "default" {
		if fv := fileVersion(string(raw), *c); fv != newVersion {
			return refuse(CodeBumpRefused, "%s: the file declares schema_version %q, not the requested %q — bump the contract first", id, fv, newVersion)
		}
	}
	c.SchemaVersion = newVersion
	c.Sha256 = hashOf(raw)
	out, err := yaml.Marshal(r)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(root, filepath.FromSlash(RegistryPath)), out, 0o644)
}

func hashOf(raw []byte) string {
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:])
}
