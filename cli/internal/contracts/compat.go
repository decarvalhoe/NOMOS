package contracts

// NRT-024 (#677): what the core reads and writes, what the adapters declare,
// and whether they agree — computed from the contract registry and the adapter
// manifests, rendered once into docs/16 and announced by `nomos version --json`.
// Nothing here declares compatibility; it compares declarations and refuses
// the ones that contradict the current core.

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	CodeAdapterUnreadable   = "ADAPTER_MANIFEST_UNREADABLE"
	CodeAdapterIncompatible = "ADAPTER_INCOMPATIBLE"
	CodeDocsDrift           = "COMPATIBILITY_MATRIX_DRIFT"

	AnnouncementSchema = "nomos-version-announcement-v1"
	MatrixDoc          = "docs/16-versioning-policy.md"
	matrixBegin        = "<!-- compatibility-matrix:begin -->"
	matrixEnd          = "<!-- compatibility-matrix:end -->"
	adapterContractID  = "adapter-manifest"
)

// semver is MAJOR.MINOR.PATCH with an optional pre-release tag.
type semver struct {
	major, minor, patch int
	pre                 string
}

var semverRe = regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)(?:-([0-9A-Za-z.-]+))?$`)

func parseSemver(s string) (semver, error) {
	m := semverRe.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return semver{}, fmt.Errorf("%q is not MAJOR.MINOR.PATCH[-pre]", s)
	}
	a, _ := strconv.Atoi(m[1])
	b, _ := strconv.Atoi(m[2])
	c, _ := strconv.Atoi(m[3])
	return semver{a, b, c, m[4]}, nil
}

// compare returns -1, 0, 1. A pre-release sorts below the release of the same
// numbers (0.2.0-ALPHA < 0.2.0); two pre-releases compare as strings.
func (v semver) compare(o semver) int {
	for _, d := range []int{v.major - o.major, v.minor - o.minor, v.patch - o.patch} {
		if d < 0 {
			return -1
		}
		if d > 0 {
			return 1
		}
	}
	switch {
	case v.pre == o.pre:
		return 0
	case v.pre == "":
		return 1
	case o.pre == "":
		return -1
	case v.pre < o.pre:
		return -1
	default:
		return 1
	}
}

// adapterManifest is the slice of adapter.nomos.yaml this check reads.
type adapterManifest struct {
	SchemaVersion string `yaml:"schema_version"`
	Adapter       struct {
		ID      string `yaml:"id"`
		Version string `yaml:"version"`
		Status  string `yaml:"status"`
	} `yaml:"adapter"`
	Compatibility struct {
		NomosCore struct {
			MinVersion     string   `yaml:"min_version"`
			MaxVersion     string   `yaml:"max_version"`
			TestedVersions []string `yaml:"tested_versions"`
		} `yaml:"nomos_core"`
		ManifestContract struct {
			Version string `yaml:"version"`
		} `yaml:"manifest_contract"`
		Schemas map[string]struct {
			MinVersion string `yaml:"min_version"`
			MaxVersion string `yaml:"max_version"`
			Mode       string `yaml:"mode"`
		} `yaml:"schemas"`
	} `yaml:"compatibility"`
}

// schemaKeys maps the adapter manifest's schema keys to registry contract ids.
var schemaKeys = map[string]string{
	"nomos_project":    "nomos-project",
	"source_manifest":  "source-manifest",
	"canonical_matrix": "canonical-matrix",
	"adapter_manifest": adapterContractID,
}

// AdapterCompat is one adapter's verdict against the current core.
type AdapterCompat struct {
	ID               string            `json:"id"`
	Path             string            `json:"path"`
	Version          string            `json:"version"`
	Status           string            `json:"status"`
	MinCore          string            `json:"min_core"`
	MaxCore          string            `json:"max_core,omitempty"`
	TestedCore       []string          `json:"tested_core,omitempty"`
	ManifestContract string            `json:"manifest_contract"`
	Schemas          map[string]string `json:"schemas,omitempty"`
	Verdict          string            `json:"verdict"`
	Reasons          []string          `json:"reasons,omitempty"`
}

// CheckAdapters reads every adapters/*/adapter.nomos.yaml and compares it with
// the core version and the registry. It returns every adapter with a verdict;
// err is non-nil (fail-closed) when any adapter is unreadable or incompatible.
func CheckAdapters(root, coreVersion string, reg Registry) ([]AdapterCompat, error) {
	core, err := parseSemver(coreVersion)
	if err != nil {
		return nil, refuse(CodeAdapterIncompatible, "core version %v", err)
	}
	versions := map[string]string{}
	for _, c := range reg.Contracts {
		versions[c.ID] = c.SchemaVersion
	}
	files, _ := filepath.Glob(filepath.Join(root, "adapters", "*", "adapter.nomos.yaml"))
	sort.Strings(files)
	var out []AdapterCompat
	var firstErr error
	for _, f := range files {
		rel := filepath.ToSlash(strings.TrimPrefix(f, filepath.Clean(root)+string(os.PathSeparator)))
		raw, err := os.ReadFile(f)
		if err != nil {
			return out, refuse(CodeAdapterUnreadable, "%s: %v", rel, err)
		}
		var m adapterManifest
		if err := yaml.Unmarshal(raw, &m); err != nil {
			return out, refuse(CodeAdapterUnreadable, "%s: %v", rel, err)
		}
		a := AdapterCompat{ID: m.Adapter.ID, Path: rel, Version: m.Adapter.Version, Status: m.Adapter.Status,
			MinCore: m.Compatibility.NomosCore.MinVersion, MaxCore: m.Compatibility.NomosCore.MaxVersion,
			TestedCore: m.Compatibility.NomosCore.TestedVersions, ManifestContract: m.Compatibility.ManifestContract.Version, Schemas: map[string]string{}}
		if a.ID == "" {
			a.Reasons = append(a.Reasons, "adapter.id is empty")
		}
		if a.ManifestContract == "" {
			a.ManifestContract = "0.1.0" // the contract's default
		}
		// The adapter conforms to a manifest contract version; a MINOR/PATCH
		// difference below what the core ships is compatible (additive), a newer
		// version or another MAJOR is not.
		if want := versions[adapterContractID]; want != "" {
			shipped, e1 := parseSemver(want)
			declared, e2 := parseSemver(a.ManifestContract)
			switch {
			case e1 != nil || e2 != nil:
				a.Reasons = append(a.Reasons, fmt.Sprintf("manifest_contract.version %q vs the core's %s %q: not comparable as semver", a.ManifestContract, adapterContractID, want))
			case declared.major != shipped.major:
				a.Reasons = append(a.Reasons, fmt.Sprintf("manifest_contract.version %q is another MAJOR than the %s %s the core ships", a.ManifestContract, adapterContractID, want))
			case declared.compare(shipped) > 0:
				a.Reasons = append(a.Reasons, fmt.Sprintf("manifest_contract.version %q is newer than the %s %s the core ships", a.ManifestContract, adapterContractID, want))
			}
		}
		if min, err := parseSemver(a.MinCore); err != nil {
			a.Reasons = append(a.Reasons, "nomos_core.min_version: "+err.Error())
		} else if core.compare(min) < 0 {
			a.Reasons = append(a.Reasons, fmt.Sprintf("requires core >= %s, current core is %s", a.MinCore, coreVersion))
		}
		if a.MaxCore != "" {
			if max, err := parseSemver(a.MaxCore); err != nil {
				a.Reasons = append(a.Reasons, "nomos_core.max_version: "+err.Error())
			} else if core.compare(max) > 0 {
				a.Reasons = append(a.Reasons, fmt.Sprintf("supports core <= %s, current core is %s", a.MaxCore, coreVersion))
			}
		}
		keys := make([]string, 0, len(m.Compatibility.Schemas))
		for k := range m.Compatibility.Schemas {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			s := m.Compatibility.Schemas[k]
			id, ok := schemaKeys[k]
			if !ok {
				a.Reasons = append(a.Reasons, fmt.Sprintf("schemas.%s is not a schema the core knows", k))
				continue
			}
			shipped, okv := parseSemver(versions[id])
			a.Schemas[k] = s.MinVersion
			if s.MaxVersion != "" {
				a.Schemas[k] = s.MinVersion + ".." + s.MaxVersion
			}
			if okv != nil {
				a.Reasons = append(a.Reasons, fmt.Sprintf("schemas.%s: the core's %s version %q is not semver, cannot compare", k, id, versions[id]))
				continue
			}
			if min, err := parseSemver(s.MinVersion); err != nil {
				a.Reasons = append(a.Reasons, fmt.Sprintf("schemas.%s.min_version: %v", k, err))
			} else if shipped.compare(min) < 0 {
				a.Reasons = append(a.Reasons, fmt.Sprintf("schemas.%s requires >= %s, the core ships %s", k, s.MinVersion, versions[id]))
			}
			if s.MaxVersion != "" {
				if max, err := parseSemver(s.MaxVersion); err != nil {
					a.Reasons = append(a.Reasons, fmt.Sprintf("schemas.%s.max_version: %v", k, err))
				} else if shipped.compare(max) > 0 {
					a.Reasons = append(a.Reasons, fmt.Sprintf("schemas.%s supports <= %s, the core ships %s", k, s.MaxVersion, versions[id]))
				}
			}
		}
		a.Verdict = "compatible"
		if len(a.Reasons) > 0 {
			a.Verdict = "incompatible"
			if firstErr == nil {
				firstErr = refuse(CodeAdapterIncompatible, "%s (%s): %s", a.ID, rel, strings.Join(a.Reasons, "; "))
			}
		}
		out = append(out, a)
	}
	return out, firstErr
}

// ContractLine is what the core announces for one contract.
type ContractLine struct {
	ID               string `json:"id"`
	SchemaVersion    string `json:"schema_version"`
	Stability        string `json:"stability"`
	Reads            bool   `json:"reads"`
	Writes           bool   `json:"writes"`
	DeprecatedSince  string `json:"deprecated_since,omitempty"`
	RemovalNotBefore string `json:"removal_not_before,omitempty"`
}

// Announcement is `nomos version --json`.
type Announcement struct {
	SchemaVersion string `json:"schema_version"`
	CoreVersion   string `json:"core_version"`
	Registry      struct {
		Path   string `json:"path"`
		Sha256 string `json:"sha256"`
		Total  int    `json:"total"`
		Stable int    `json:"stable"`
	} `json:"contract_registry"`
	Contracts     []ContractLine    `json:"contracts"`
	Formats       map[string]string `json:"formats"`
	Adapters      []AdapterCompat   `json:"adapters"`
	Warnings      []string          `json:"warnings"`
	ClaimBoundary string            `json:"claim_boundary"`
}

// Formats the core emits besides the registered contracts; the values are the
// literals the Go code writes, listed here so the announcement names them.
var CoreFormats = map[string]string{
	"contract_status":           "nomos-contract-status-v1",
	"version_announcement":      AnnouncementSchema,
	"release_candidate_spec":    "nomos-release-candidate-spec-v1",
	"release_candidate_gates":   "nomos-release-candidate-gates-v1",
	"portfolio_status":          "nomos-portfolio-status-v1",
	"attestation_predicate":     "https://nomos.dev/attestation/v1",
	"claim_boundary_predicate":  "https://nomos.dev/claim-boundary/v1",
	"slsa_provenance_predicate": "https://slsa.dev/provenance/v1",
}

// DeprecationWarnings names every deprecated contract with its removal date.
func DeprecationWarnings(reg Registry) []string {
	var w []string
	for _, c := range reg.Contracts {
		if c.Stability == "deprecated" {
			w = append(w, fmt.Sprintf("contract %s (%s) is deprecated since %s; removal not before %s — migrate readers and writers before that date", c.ID, c.Path, c.DeprecatedSince, c.RemovalNotBefore))
		}
	}
	return w
}

// Announce builds the announcement from the registry and the adapters. The
// adapter verdicts are included even when some are incompatible; the caller
// decides whether that is a refusal (contracts status) or a report (version).
func Announce(root, coreVersion string) (Announcement, error) {
	reg, err := Load(root)
	if err != nil {
		return Announcement{}, err
	}
	raw, _ := os.ReadFile(filepath.Join(root, filepath.FromSlash(RegistryPath)))
	sum := sha256.Sum256(raw)
	ann := Announcement{SchemaVersion: AnnouncementSchema, CoreVersion: coreVersion, Formats: CoreFormats, ClaimBoundary: ClaimBoundary}
	ann.Registry.Path, ann.Registry.Sha256 = RegistryPath, "sha256:"+hex.EncodeToString(sum[:])
	for _, c := range reg.Contracts {
		ann.Contracts = append(ann.Contracts, ContractLine{ID: c.ID, SchemaVersion: c.SchemaVersion, Stability: c.Stability, Reads: len(c.Readers) > 0, Writes: len(c.Writers) > 0, DeprecatedSince: c.DeprecatedSince, RemovalNotBefore: c.RemovalNotBefore})
		if c.Stability == "stable" {
			ann.Registry.Stable++
		}
	}
	sort.Slice(ann.Contracts, func(i, j int) bool { return ann.Contracts[i].ID < ann.Contracts[j].ID })
	ann.Registry.Total = len(ann.Contracts)
	ann.Warnings = DeprecationWarnings(reg)
	if ann.Warnings == nil {
		ann.Warnings = []string{}
	}
	adapters, aerr := CheckAdapters(root, coreVersion, reg)
	ann.Adapters = adapters
	if ann.Adapters == nil {
		ann.Adapters = []AdapterCompat{}
	}
	return ann, aerr
}

// RenderMatrix is the generated section of docs/16: deterministic Markdown.
func RenderMatrix(ann Announcement) string {
	var b strings.Builder
	b.WriteString(matrixBegin + "\n")
	b.WriteString("<!-- GENERATED from specs/contract-registry.yaml and adapters/*/adapter.nomos.yaml by `nomos contracts status --emit-docs`; do not edit by hand, CI fails on drift -->\n\n")
	fmt.Fprintf(&b, "Core `%s` — %d contract(s) registered, %d stable. `reads`/`writes` = a Go reader/writer is declared in the registry.\n\n", ann.CoreVersion, ann.Registry.Total, ann.Registry.Stable)
	b.WriteString("| Contract | Version | Stability | Core reads | Core writes |\n|---|---|---|---|---|\n")
	for _, c := range ann.Contracts {
		if c.Stability == "experimental" && !c.Reads && !c.Writes {
			continue // listed in the registry, not in the compatibility promise
		}
		stab := c.Stability
		if c.Stability == "deprecated" {
			stab = fmt.Sprintf("deprecated (since %s, removal not before %s)", c.DeprecatedSince, c.RemovalNotBefore)
		}
		fmt.Fprintf(&b, "| `%s` | `%s` | %s | %s | %s |\n", c.ID, c.SchemaVersion, stab, yesNo(c.Reads), yesNo(c.Writes))
	}
	b.WriteString("\n| Adapter | Version | Status | Core range | Manifest contract | Schema minimums | Verdict |\n|---|---|---|---|---|---|---|\n")
	for _, a := range ann.Adapters {
		rng := ">= " + a.MinCore
		if a.MaxCore != "" {
			rng += ", <= " + a.MaxCore
		}
		keys := make([]string, 0, len(a.Schemas))
		for k := range a.Schemas {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		parts := make([]string, 0, len(keys))
		for _, k := range keys {
			parts = append(parts, k+" "+a.Schemas[k])
		}
		verdict := a.Verdict
		if len(a.Reasons) > 0 {
			verdict += ": " + strings.Join(a.Reasons, "; ")
		}
		fmt.Fprintf(&b, "| `%s` | `%s` | %s | %s | `%s` | %s | %s |\n", a.ID, a.Version, a.Status, rng, a.ManifestContract, strings.Join(parts, ", "), verdict)
	}
	b.WriteString("\nOther formats the core emits: ")
	fk := make([]string, 0, len(ann.Formats))
	for k := range ann.Formats {
		fk = append(fk, k)
	}
	sort.Strings(fk)
	for i, k := range fk {
		if i > 0 {
			b.WriteString(", ")
		}
		fmt.Fprintf(&b, "%s `%s`", k, ann.Formats[k])
	}
	b.WriteString(".\n")
	for _, w := range ann.Warnings {
		b.WriteString("\n> **Deprecation:** " + w + "\n")
	}
	b.WriteString("\n" + matrixEnd)
	return b.String()
}

func yesNo(v bool) string {
	if v {
		return "yes"
	}
	return "no"
}

// spliceMatrix replaces the generated block, or the hand-written example under
// "## Compatibility Matrix" on first emission.
func spliceMatrix(text, block string) (string, error) {
	if i := strings.Index(text, matrixBegin); i >= 0 {
		j := strings.Index(text, matrixEnd)
		if j < i {
			return "", fmt.Errorf("%s: end marker before begin marker", MatrixDoc)
		}
		return text[:i] + block + text[j+len(matrixEnd):], nil
	}
	head := "## Compatibility Matrix\n"
	i := strings.Index(text, head)
	if i < 0 {
		return "", fmt.Errorf("%s: section %q not found", MatrixDoc, strings.TrimSpace(head))
	}
	start := i + len(head)
	rest := text[start:]
	next := strings.Index(rest, "\n## ")
	if next < 0 {
		next = len(rest)
	}
	return text[:start] + "\n" + block + "\n" + rest[next:], nil
}

// EmitDocs writes the generated section into docs/16.
func EmitDocs(root string, ann Announcement) error {
	p := filepath.Join(root, filepath.FromSlash(MatrixDoc))
	raw, err := os.ReadFile(p)
	if err != nil {
		return err
	}
	out, err := spliceMatrix(string(raw), RenderMatrix(ann))
	if err != nil {
		return err
	}
	return os.WriteFile(p, []byte(out), 0o644)
}

// CheckDocs refuses when docs/16 does not carry exactly the generated section.
func CheckDocs(root string, ann Announcement) error {
	p := filepath.Join(root, filepath.FromSlash(MatrixDoc))
	raw, err := os.ReadFile(p)
	if err != nil {
		return refuse(CodeDocsDrift, "%s: %v", MatrixDoc, err)
	}
	if !strings.Contains(string(raw), RenderMatrix(ann)) {
		return refuse(CodeDocsDrift, "%s: the compatibility matrix is not the one generated from the registry and the adapters — run `nomos contracts status --emit-docs`", MatrixDoc)
	}
	return nil
}
