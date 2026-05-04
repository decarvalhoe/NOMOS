package corpus

import (
	"fmt"
	"path/filepath"
	"strings"
)

// FSQ-02 (#365): explicit source admission and non-atomization policy.
//
// Every source in the corpus manifest and the consumer feed must carry
// machine-checkable classification answering three questions:
//
//   - WHY was the source admitted (or rejected)?
//   - WHAT format support does the canonical pipeline have for it?
//   - WHETHER it produced atomized units (and if not, an explicit reason)?
//
// The classification is generic; it does not embed any RBOK assumption.

// AdmissionStatus values.
const (
	AdmissionAdmitted = "admitted"
	AdmissionExcluded = "excluded"
	AdmissionBlocked  = "blocked"
)

// AtomizationStatus values.
const (
	AtomizationAtomized     = "atomized"
	AtomizationCoverageOnly = "coverage_only"
	AtomizationNotAtomized  = "not_atomized"
	AtomizationUnsupported  = "unsupported"
	AtomizationDerivative   = "derivative"
	AtomizationExcluded     = "excluded"
)

// Source role values used by the FSQ-02 admission classification. These
// are bare strings (not the RBOK-specific SourceRole enum from
// rbok_source_policy.go); the two namespaces are intentionally separate
// because the RBOK roles (lawbook / schema / derived / out_of_scope)
// describe RBOK lawbook structure, while these describe the generic
// admission policy.
const (
	AdmissionRoleCanonical  = "canonical"
	AdmissionRoleReference  = "reference"
	AdmissionRoleDerivative = "derivative"
	AdmissionRoleMetadata   = "metadata"
	AdmissionRoleBinary     = "binary"
)

// FormatSupport values.
const (
	FormatSupported   = "supported"
	FormatPartial     = "partial"
	FormatUnsupported = "unsupported"
)

// Stable error code substrings emitted by SourceAdmission.Validate. The
// downstream FSQ-06 audit (#369) keys off these strings, so they are part
// of the public contract.
const (
	ErrCodeNoAdmission           = "SOURCE_NO_ADMISSION_STATUS"
	ErrCodeInvalidAdmission      = "SOURCE_INVALID_ADMISSION_STATUS"
	ErrCodeAdmittedNoAtomization = "SOURCE_ADMITTED_NO_ATOMIZATION_STATUS"
	ErrCodeInvalidAtomization    = "SOURCE_INVALID_ATOMIZATION_STATUS"
	ErrCodeInvalidTransition     = "SOURCE_INVALID_TRANSITION"
	ErrCodeNoReason              = "SOURCE_NO_REASON"
	ErrCodeNoDerivativeTarget    = "SOURCE_NO_DERIVATIVE_TARGET"
	ErrCodeAtomizedButZeroUnits  = "SOURCE_ATOMIZED_BUT_ZERO_UNITS"
	ErrCodeInvalidRole           = "SOURCE_INVALID_ROLE"
	ErrCodeInvalidFormatSupport  = "SOURCE_INVALID_FORMAT_SUPPORT"
)

// SourceAdmission groups the FSQ-02 classification fields. ManifestSource
// and FeedSource each carry their own copies of these fields with
// format-specific tags; this struct is the canonical projection used by
// validation and default-derivation helpers.
type SourceAdmission struct {
	AdmissionStatus   string
	AtomizationStatus string
	ExclusionReason   string
	SourceRole        string
	FormatSupport     string
	DerivativeOf      string
}

// IsZero reports whether all six classification fields are empty.
func (s SourceAdmission) IsZero() bool {
	return s.AdmissionStatus == "" &&
		s.AtomizationStatus == "" &&
		s.ExclusionReason == "" &&
		s.SourceRole == "" &&
		s.FormatSupport == "" &&
		s.DerivativeOf == ""
}

var validAdmissions = map[string]struct{}{
	AdmissionAdmitted: {}, AdmissionExcluded: {}, AdmissionBlocked: {},
}

var validAtomizations = map[string]struct{}{
	AtomizationAtomized: {}, AtomizationCoverageOnly: {}, AtomizationNotAtomized: {},
	AtomizationUnsupported: {}, AtomizationDerivative: {}, AtomizationExcluded: {},
}

var validRoles = map[string]struct{}{
	AdmissionRoleCanonical: {}, AdmissionRoleReference: {}, AdmissionRoleDerivative: {},
	AdmissionRoleMetadata: {}, AdmissionRoleBinary: {},
}

var validFormats = map[string]struct{}{
	FormatSupported: {}, FormatPartial: {}, FormatUnsupported: {},
}

// Validate enforces the FSQ-02 transition rules on a SourceAdmission.
// The error messages embed one of the stable Err* codes so audit
// consumers can pattern-match without parsing the prose.
func (s SourceAdmission) Validate() error {
	if s.AdmissionStatus == "" {
		return fmt.Errorf("%s: admission_status required", ErrCodeNoAdmission)
	}
	if _, ok := validAdmissions[s.AdmissionStatus]; !ok {
		return fmt.Errorf("%s: admission_status %q not in {admitted, excluded, blocked}",
			ErrCodeInvalidAdmission, s.AdmissionStatus)
	}
	if s.SourceRole == "" {
		return fmt.Errorf("%s: source_role required", ErrCodeInvalidRole)
	}
	if _, ok := validRoles[s.SourceRole]; !ok {
		return fmt.Errorf("%s: source_role %q not in {canonical, reference, derivative, metadata, binary}",
			ErrCodeInvalidRole, s.SourceRole)
	}
	if s.FormatSupport == "" {
		return fmt.Errorf("%s: format_support required", ErrCodeInvalidFormatSupport)
	}
	if _, ok := validFormats[s.FormatSupport]; !ok {
		return fmt.Errorf("%s: format_support %q not in {supported, partial, unsupported}",
			ErrCodeInvalidFormatSupport, s.FormatSupport)
	}

	if s.AdmissionStatus != AdmissionAdmitted {
		// Excluded or blocked sources must not carry an atomization status.
		if s.AtomizationStatus != "" {
			return fmt.Errorf("%s: atomization_status %q is illegal for admission_status %q",
				ErrCodeInvalidTransition, s.AtomizationStatus, s.AdmissionStatus)
		}
		if strings.TrimSpace(s.ExclusionReason) == "" {
			return fmt.Errorf("%s: exclusion_reason required when admission_status=%q",
				ErrCodeNoReason, s.AdmissionStatus)
		}
		return nil
	}

	// admission_status == "admitted" from here.
	if s.AtomizationStatus == "" {
		return fmt.Errorf("%s: atomization_status required when admission_status=admitted",
			ErrCodeAdmittedNoAtomization)
	}
	if _, ok := validAtomizations[s.AtomizationStatus]; !ok {
		return fmt.Errorf("%s: atomization_status %q not in {atomized, coverage_only, not_atomized, unsupported, derivative, excluded}",
			ErrCodeInvalidAtomization, s.AtomizationStatus)
	}

	switch s.AtomizationStatus {
	case AtomizationUnsupported:
		if s.FormatSupport != FormatPartial && s.FormatSupport != FormatUnsupported {
			return fmt.Errorf("%s: atomization_status=unsupported requires format_support in {partial, unsupported}, got %q",
				ErrCodeInvalidTransition, s.FormatSupport)
		}
		if strings.TrimSpace(s.ExclusionReason) == "" {
			return fmt.Errorf("%s: exclusion_reason required when atomization_status=unsupported",
				ErrCodeNoReason)
		}
	case AtomizationNotAtomized:
		if strings.TrimSpace(s.ExclusionReason) == "" {
			return fmt.Errorf("%s: exclusion_reason required when atomization_status=not_atomized",
				ErrCodeNoReason)
		}
	case AtomizationDerivative:
		if strings.TrimSpace(s.DerivativeOf) == "" {
			return fmt.Errorf("%s: derivative_of required when atomization_status=derivative",
				ErrCodeNoDerivativeTarget)
		}
	case AtomizationCoverageOnly, AtomizationExcluded:
		if strings.TrimSpace(s.ExclusionReason) == "" {
			return fmt.Errorf("%s: exclusion_reason required when atomization_status=%q",
				ErrCodeNoReason, s.AtomizationStatus)
		}
	}

	if s.SourceRole == AdmissionRoleDerivative && strings.TrimSpace(s.DerivativeOf) == "" {
		return fmt.Errorf("%s: derivative_of required when source_role=derivative",
			ErrCodeNoDerivativeTarget)
	}

	return nil
}

// ValidateAtomizedAgainstUnitCount enforces the runtime invariant that a
// source whose atomization_status is "atomized" must yield at least one
// feed unit. This is a feed-time check, not a struct-shape check: the
// unit count is only known after extraction.
func ValidateAtomizedAgainstUnitCount(s SourceAdmission, sourceID string, unitCount int) error {
	if s.AtomizationStatus == AtomizationAtomized && unitCount == 0 {
		return fmt.Errorf("%s: source %q declared atomization_status=atomized but produced 0 feed units",
			ErrCodeAtomizedButZeroUnits, sourceID)
	}
	return nil
}

// DefaultAdmissionForPath returns the default classification for a source
// at the given relative path. The defaults follow the FSQ-02 heuristic
// table; on-disk binary detection is delegated to the existing binary
// policy (see binary_policy.go) when callers want to override the
// extension-based default.
func DefaultAdmissionForPath(path string) SourceAdmission {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".md", ".mdx":
		return SourceAdmission{
			AdmissionStatus:   AdmissionAdmitted,
			AtomizationStatus: AtomizationAtomized,
			SourceRole:        AdmissionRoleCanonical,
			FormatSupport:     FormatSupported,
		}
	case ".yaml", ".yml":
		return SourceAdmission{
			AdmissionStatus:   AdmissionAdmitted,
			AtomizationStatus: AtomizationAtomized,
			SourceRole:        AdmissionRoleCanonical,
			FormatSupport:     FormatSupported,
		}
	case ".json":
		return SourceAdmission{
			AdmissionStatus:   AdmissionAdmitted,
			AtomizationStatus: AtomizationNotAtomized,
			ExclusionReason:   "json metadata not atomized",
			SourceRole:        AdmissionRoleMetadata,
			FormatSupport:     FormatPartial,
		}
	case ".pdf", ".docx", ".html", ".htm", ".doc", ".rtf", ".odt":
		return SourceAdmission{
			AdmissionStatus:   AdmissionAdmitted,
			AtomizationStatus: AtomizationUnsupported,
			ExclusionReason:   "format not yet supported by canonical scanners",
			SourceRole:        AdmissionRoleReference,
			FormatSupport:     FormatUnsupported,
		}
	case ".xlsx", ".xls", ".csv", ".tsv", ".ods":
		// Tabular sources default to reference role: declaring them
		// derivative requires a known parent (DerivativeOf), which the
		// scanner does not infer. Operators upgrade to derivative role
		// + derivative_of when they record the parent explicitly.
		return SourceAdmission{
			AdmissionStatus:   AdmissionAdmitted,
			AtomizationStatus: AtomizationUnsupported,
			ExclusionReason:   "tabular format not yet supported by canonical scanners",
			SourceRole:        AdmissionRoleReference,
			FormatSupport:     FormatPartial,
		}
	}
	// Use the binary policy's extension classifier to catch images, archives,
	// and other binaries without duplicating logic.
	if class, ok := classifyByExt(ext); ok {
		switch class {
		case ClassImage, ClassBinary, ClassOffice:
			return SourceAdmission{
				AdmissionStatus:   AdmissionAdmitted,
				AtomizationStatus: AtomizationUnsupported,
				ExclusionReason:   "binary policy",
				SourceRole:        AdmissionRoleBinary,
				FormatSupport:     FormatUnsupported,
			}
		}
	}
	// Conservative fallback for unknown extensions: admit but flag as
	// not_atomized with an explicit reason, and force the operator to
	// either declare support or downgrade.
	return SourceAdmission{
		AdmissionStatus:   AdmissionAdmitted,
		AtomizationStatus: AtomizationNotAtomized,
		ExclusionReason:   fmt.Sprintf("no canonical scanner registered for extension %q", ext),
		SourceRole:        AdmissionRoleReference,
		FormatSupport:     FormatUnsupported,
	}
}

// BackfillAdmission fills empty fields of dst from the heuristic default
// for the given path. Existing non-empty values are preserved so an
// operator declaration always wins over the heuristic.
func BackfillAdmission(dst *SourceAdmission, path string) {
	if dst == nil {
		return
	}
	def := DefaultAdmissionForPath(path)
	if dst.AdmissionStatus == "" {
		dst.AdmissionStatus = def.AdmissionStatus
	}
	if dst.SourceRole == "" {
		dst.SourceRole = def.SourceRole
	}
	if dst.FormatSupport == "" {
		dst.FormatSupport = def.FormatSupport
	}
	if dst.AdmissionStatus == AdmissionAdmitted && dst.AtomizationStatus == "" {
		dst.AtomizationStatus = def.AtomizationStatus
	}
	if dst.ExclusionReason == "" && def.ExclusionReason != "" {
		// Only inherit a default reason when the inherited atomization
		// status is one that requires a reason; otherwise leave empty.
		switch dst.AtomizationStatus {
		case AtomizationUnsupported, AtomizationNotAtomized,
			AtomizationCoverageOnly, AtomizationExcluded:
			dst.ExclusionReason = def.ExclusionReason
		}
		if dst.AdmissionStatus != AdmissionAdmitted {
			dst.ExclusionReason = def.ExclusionReason
		}
	}
}
