package corpus

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSourceAdmissionValidate(t *testing.T) {
	good := SourceAdmission{
		AdmissionStatus:   AdmissionAdmitted,
		AtomizationStatus: AtomizationAtomized,
		SourceRole:        AdmissionRoleCanonical,
		FormatSupport:     FormatSupported,
	}

	cases := []struct {
		name     string
		s        SourceAdmission
		wantCode string
	}{
		// Stable error codes — one negative case per code.
		{
			name:     "no admission status",
			s:        SourceAdmission{SourceRole: AdmissionRoleCanonical, FormatSupport: FormatSupported},
			wantCode: ErrCodeNoAdmission,
		},
		{
			name: "invalid admission status",
			s: SourceAdmission{
				AdmissionStatus: "weird",
				SourceRole:      AdmissionRoleCanonical, FormatSupport: FormatSupported,
			},
			wantCode: ErrCodeInvalidAdmission,
		},
		{
			name: "admitted no atomization status",
			s: SourceAdmission{
				AdmissionStatus: AdmissionAdmitted,
				SourceRole:      AdmissionRoleCanonical, FormatSupport: FormatSupported,
			},
			wantCode: ErrCodeAdmittedNoAtomization,
		},
		{
			name: "invalid atomization status",
			s: SourceAdmission{
				AdmissionStatus: AdmissionAdmitted, AtomizationStatus: "exploded",
				SourceRole: AdmissionRoleCanonical, FormatSupport: FormatSupported,
			},
			wantCode: ErrCodeInvalidAtomization,
		},
		{
			name: "invalid transition: excluded with atomization",
			s: SourceAdmission{
				AdmissionStatus: AdmissionExcluded, AtomizationStatus: AtomizationAtomized,
				SourceRole: AdmissionRoleCanonical, FormatSupport: FormatSupported,
				ExclusionReason: "out of scope",
			},
			wantCode: ErrCodeInvalidTransition,
		},
		{
			name: "invalid transition: unsupported with format=supported",
			s: SourceAdmission{
				AdmissionStatus: AdmissionAdmitted, AtomizationStatus: AtomizationUnsupported,
				SourceRole: AdmissionRoleReference, FormatSupport: FormatSupported,
				ExclusionReason: "format pending",
			},
			wantCode: ErrCodeInvalidTransition,
		},
		{
			name: "no reason: not_atomized without reason",
			s: SourceAdmission{
				AdmissionStatus: AdmissionAdmitted, AtomizationStatus: AtomizationNotAtomized,
				SourceRole: AdmissionRoleMetadata, FormatSupport: FormatPartial,
			},
			wantCode: ErrCodeNoReason,
		},
		{
			name: "no derivative target",
			s: SourceAdmission{
				AdmissionStatus: AdmissionAdmitted, AtomizationStatus: AtomizationDerivative,
				SourceRole: AdmissionRoleDerivative, FormatSupport: FormatPartial,
			},
			wantCode: ErrCodeNoDerivativeTarget,
		},
		{
			name: "invalid role",
			s: SourceAdmission{
				AdmissionStatus: AdmissionAdmitted, AtomizationStatus: AtomizationAtomized,
				SourceRole: "loudspeaker", FormatSupport: FormatSupported,
			},
			wantCode: ErrCodeInvalidRole,
		},
		{
			name: "invalid format support",
			s: SourceAdmission{
				AdmissionStatus: AdmissionAdmitted, AtomizationStatus: AtomizationAtomized,
				SourceRole: AdmissionRoleCanonical, FormatSupport: "telegram",
			},
			wantCode: ErrCodeInvalidFormatSupport,
		},

		// Positive cases.
		{name: "good admitted+atomized", s: good, wantCode: ""},
		{
			name: "good unsupported pdf",
			s: SourceAdmission{
				AdmissionStatus: AdmissionAdmitted, AtomizationStatus: AtomizationUnsupported,
				SourceRole: AdmissionRoleReference, FormatSupport: FormatUnsupported,
				ExclusionReason: "format not yet supported by canonical scanners",
			},
			wantCode: "",
		},
		{
			name: "good derivative",
			s: SourceAdmission{
				AdmissionStatus: AdmissionAdmitted, AtomizationStatus: AtomizationDerivative,
				SourceRole: AdmissionRoleDerivative, FormatSupport: FormatPartial,
				DerivativeOf: "DOC-PARENT-001",
			},
			wantCode: "",
		},
		{
			name: "good excluded with reason",
			s: SourceAdmission{
				AdmissionStatus: AdmissionExcluded,
				SourceRole:      AdmissionRoleReference, FormatSupport: FormatUnsupported,
				ExclusionReason: "operator decision",
			},
			wantCode: "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.s.Validate()
			if tc.wantCode == "" {
				if err != nil {
					t.Fatalf("expected pass, got error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.wantCode)
			}
			if !strings.Contains(err.Error(), tc.wantCode) {
				t.Fatalf("expected error containing %q, got %q", tc.wantCode, err.Error())
			}
		})
	}
}

func TestValidateAtomizedAgainstUnitCount(t *testing.T) {
	atomized := SourceAdmission{
		AdmissionStatus: AdmissionAdmitted, AtomizationStatus: AtomizationAtomized,
		SourceRole: AdmissionRoleCanonical, FormatSupport: FormatSupported,
	}
	if err := ValidateAtomizedAgainstUnitCount(atomized, "SRC", 0); err == nil {
		t.Fatal("expected SOURCE_ATOMIZED_BUT_ZERO_UNITS, got nil")
	} else if !strings.Contains(err.Error(), ErrCodeAtomizedButZeroUnits) {
		t.Fatalf("expected error code %s, got %q", ErrCodeAtomizedButZeroUnits, err.Error())
	}
	if err := ValidateAtomizedAgainstUnitCount(atomized, "SRC", 3); err != nil {
		t.Fatalf("expected pass with non-zero units, got %v", err)
	}
	other := SourceAdmission{
		AdmissionStatus: AdmissionAdmitted, AtomizationStatus: AtomizationUnsupported,
		SourceRole: AdmissionRoleReference, FormatSupport: FormatUnsupported,
		ExclusionReason: "fmt",
	}
	if err := ValidateAtomizedAgainstUnitCount(other, "SRC", 0); err != nil {
		t.Fatalf("non-atomized status with zero units must pass: %v", err)
	}
}

func TestDefaultClassificationByExtension(t *testing.T) {
	cases := []struct {
		path                  string
		wantRole              string
		wantFormat            string
		wantAtomization       string
		expectExclusionReason bool
	}{
		{"docs/intro.md", AdmissionRoleCanonical, FormatSupported, AtomizationAtomized, false},
		{"profiles/parcours.yaml", AdmissionRoleCanonical, FormatSupported, AtomizationAtomized, false},
		{"00_meta/parcours-template-approfondi.yaml", AdmissionRoleReference, FormatSupported, AtomizationNotAtomized, true},
		{"03_parcours/ai-config/ai-behavior-config-aligned.md", AdmissionRoleReference, FormatSupported, AtomizationNotAtomized, true},
		{"policies/config/rules.json", AdmissionRoleReference, FormatSupported, AtomizationNotAtomized, true},
		{"03_parcours/testdata/PAR_ACC_DEM_EXI_TESTDATA.yaml", AdmissionRoleReference, FormatSupported, AtomizationNotAtomized, true},
		{"03_parcours/generated/md/PAR_ACC_DEM.md", AdmissionRoleReference, FormatSupported, AtomizationNotAtomized, true},
		{"03_parcours/generated/workbooks/PAR_ACC_DEM.md", AdmissionRoleReference, FormatSupported, AtomizationNotAtomized, true},
		{"98_references/archive/previous-standard.md", AdmissionRoleReference, FormatSupported, AtomizationNotAtomized, true},
		{"data/payload.json", AdmissionRoleMetadata, FormatPartial, AtomizationNotAtomized, true},
		{"refs/standard.pdf", AdmissionRoleReference, FormatUnsupported, AtomizationUnsupported, true},
		{"refs/spec.html", AdmissionRoleReference, FormatUnsupported, AtomizationUnsupported, true},
		{"refs/word.docx", AdmissionRoleReference, FormatUnsupported, AtomizationUnsupported, true},
		{"data/grid.csv", AdmissionRoleReference, FormatPartial, AtomizationUnsupported, true},
		{"data/grid.xlsx", AdmissionRoleReference, FormatPartial, AtomizationUnsupported, true},
		{"img/diagram.png", AdmissionRoleBinary, FormatUnsupported, AtomizationUnsupported, true},
		{"weird/thing.xyz", AdmissionRoleReference, FormatUnsupported, AtomizationNotAtomized, true},
	}
	for _, c := range cases {
		t.Run(c.path, func(t *testing.T) {
			adm := DefaultAdmissionForPath(c.path)
			if adm.SourceRole != c.wantRole {
				t.Fatalf("role: want %q, got %q", c.wantRole, adm.SourceRole)
			}
			if adm.FormatSupport != c.wantFormat {
				t.Fatalf("format: want %q, got %q", c.wantFormat, adm.FormatSupport)
			}
			if adm.AtomizationStatus != c.wantAtomization {
				t.Fatalf("atomization: want %q, got %q", c.wantAtomization, adm.AtomizationStatus)
			}
			if c.expectExclusionReason && strings.TrimSpace(adm.ExclusionReason) == "" {
				t.Fatalf("expected non-empty exclusion_reason for %s", c.path)
			}
			if err := adm.Validate(); err != nil {
				t.Fatalf("default admission for %s must validate: %v", c.path, err)
			}
		})
	}
}

func TestBackfillAdmissionPreservesOperatorOverride(t *testing.T) {
	adm := SourceAdmission{
		AdmissionStatus:   AdmissionAdmitted,
		AtomizationStatus: AtomizationCoverageOnly,
		SourceRole:        AdmissionRoleCanonical,
		FormatSupport:     FormatSupported,
		ExclusionReason:   "operator override: structural-only doc",
	}
	BackfillAdmission(&adm, "docs/structure.md")
	if adm.AtomizationStatus != AtomizationCoverageOnly {
		t.Fatalf("operator atomization status overridden: %q", adm.AtomizationStatus)
	}
	if adm.ExclusionReason != "operator override: structural-only doc" {
		t.Fatalf("operator exclusion reason overridden: %q", adm.ExclusionReason)
	}
}

// TestFeedFailsOnSilentZeroUnits — feed generation fails when a .md
// source ends up with AtomizationStatus=atomized but zero units. This
// is the silent-zero-units state the dispatch wants to eliminate.
func TestFeedFailsOnSilentZeroUnits(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "docs"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// An empty markdown file: the typed scanner emits no segments, and
	// extract_md emits no units, but the default heuristic puts it at
	// atomization=atomized. The feed must refuse to generate.
	if err := os.WriteFile(filepath.Join(root, "docs/empty.md"), nil, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	manifestYAML := []byte(`
schema_version: "0.1.0"
sources:
  - id: EMPTY-MD
    path: docs/empty.md
    type: markdown
    domain: rbok
    priority: primary
    status: active
    hash: "sha256:empty"
    owner: Alice
    license: internal
    confidentiality: internal
    allowed_uses:
      - structured_contract
`)
	_, err := GenerateFeed(FeedInput{
		ManifestYAML: manifestYAML,
		Root:         root,
		GeneratedAt:  time.Date(2026, 5, 4, 0, 0, 0, 0, time.UTC),
	})
	if err == nil {
		t.Fatal("expected SOURCE_ATOMIZED_BUT_ZERO_UNITS error, got nil")
	}
	if !strings.Contains(err.Error(), ErrCodeAtomizedButZeroUnits) {
		t.Fatalf("expected error containing %s, got %q", ErrCodeAtomizedButZeroUnits, err.Error())
	}
}

// TestFeedAcceptsZeroUnitsWithReason — feed generation succeeds when a
// .pdf source has AtomizationStatus=unsupported and a reason. That's
// the whole point of the explicit classification.
func TestFeedAcceptsZeroUnitsWithReason(t *testing.T) {
	manifestYAML := []byte(`
schema_version: "0.1.0"
sources:
  - id: PDF-REF-001
    path: refs/standard.pdf
    type: pdf
    domain: rbok
    priority: primary
    status: active
    hash: "sha256:abc"
    owner: Alice
    license: internal
    confidentiality: internal
    allowed_uses:
      - citation_internal
`)
	feed, err := GenerateFeed(FeedInput{
		ManifestYAML: manifestYAML,
		GeneratedAt:  time.Date(2026, 5, 4, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("expected feed to generate; got %v", err)
	}
	if feed.SourceCount != 1 {
		t.Fatalf("expected 1 source, got %d", feed.SourceCount)
	}
	got := feed.Sources[0]
	if got.AdmissionStatus != AdmissionAdmitted {
		t.Fatalf("admission_status: want admitted, got %q", got.AdmissionStatus)
	}
	if got.AtomizationStatus != AtomizationUnsupported {
		t.Fatalf("atomization_status: want unsupported, got %q", got.AtomizationStatus)
	}
	if got.ExclusionReason == "" {
		t.Fatal("expected non-empty exclusion_reason on PDF source")
	}
	if got.SourceRole != AdmissionRoleReference {
		t.Fatalf("source_role: want reference, got %q", got.SourceRole)
	}
	if got.FormatSupport != FormatUnsupported {
		t.Fatalf("format_support: want unsupported, got %q", got.FormatSupport)
	}
}

func TestFeedSkipsNotAtomizedMarkdownSources(t *testing.T) {
	root := t.TempDir()
	sourcePath := filepath.Join("03_parcours", "generated", "md", "PAR_ACC_ADMIN.md")
	if err := os.MkdirAll(filepath.Join(root, filepath.Dir(sourcePath)), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, sourcePath), []byte("# Generated workbook\n\nThis generated derivative text must not become a canonical feed unit.\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	manifestYAML := []byte(`
schema_version: "0.1.0"
sources:
  - id: GENERATED-MD
    path: 03_parcours/generated/md/PAR_ACC_ADMIN.md
    type: markdown
    domain: rbok
    priority: primary
    status: active
    hash: "sha256:generated"
    owner: Alice
    license: internal
    confidentiality: internal
    allowed_uses:
      - citation_internal
`)
	feed, err := GenerateFeed(FeedInput{
		ManifestYAML: manifestYAML,
		Root:         root,
		GeneratedAt:  time.Date(2026, 5, 4, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("expected feed to generate, got %v", err)
	}
	if feed.UnitCount != 0 {
		t.Fatalf("not_atomized markdown must emit 0 units, got %d", feed.UnitCount)
	}
	if len(feed.Sources) != 1 {
		t.Fatalf("expected source entry to remain present, got %d", len(feed.Sources))
	}
	if feed.Sources[0].AtomizationStatus != AtomizationNotAtomized {
		t.Fatalf("atomization_status: want %q, got %q", AtomizationNotAtomized, feed.Sources[0].AtomizationStatus)
	}
}

// TestFeedFailsOnInvalidOperatorOverride — when an operator manually
// declares atomization_status=derivative without DerivativeOf, the
// feed must refuse to generate (the schema rule, not just the runtime
// unit-count rule).
func TestFeedFailsOnInvalidOperatorOverride(t *testing.T) {
	manifestYAML := []byte(`
schema_version: "0.1.0"
sources:
  - id: BROKEN-DERIV
    path: data/payload.csv
    type: csv
    domain: rbok
    priority: primary
    status: active
    hash: "sha256:csv"
    owner: Alice
    license: internal
    confidentiality: internal
    allowed_uses:
      - citation_internal
    admission_status: admitted
    atomization_status: derivative
    source_role: derivative
    format_support: partial
`)
	_, err := GenerateFeed(FeedInput{
		ManifestYAML: manifestYAML,
	})
	if err == nil {
		t.Fatal("expected SOURCE_NO_DERIVATIVE_TARGET error, got nil")
	}
	if !strings.Contains(err.Error(), ErrCodeNoDerivativeTarget) {
		t.Fatalf("expected error containing %s, got %q", ErrCodeNoDerivativeTarget, err.Error())
	}
}
