package compliance

import (
	"fmt"
	"strings"
	"time"
)

// SLSALevel indicates the SLSA build level achieved.
type SLSALevel string

const (
	SLSA1 SLSALevel = "slsa1"
	SLSA2 SLSALevel = "slsa2"
	SLSA3 SLSALevel = "slsa3"
	SLSA4 SLSALevel = "slsa4"
)

// Rank returns the numeric rank of the SLSA level (1-4).
func (l SLSALevel) Rank() int {
	switch l {
	case SLSA1:
		return 1
	case SLSA2:
		return 2
	case SLSA3:
		return 3
	case SLSA4:
		return 4
	default:
		return 0
	}
}

// IsValid returns true if the level is recognized.
func (l SLSALevel) IsValid() bool {
	return l.Rank() > 0
}

// MeetsMinimum returns true if this level meets or exceeds the minimum.
func (l SLSALevel) MeetsMinimum(min SLSALevel) bool {
	return l.Rank() >= min.Rank()
}

// ProvenanceGateStatus is the result of a provenance gate check.
type ProvenanceGateStatus string

const (
	GatePassed  ProvenanceGateStatus = "passed"
	GateFailed  ProvenanceGateStatus = "failed"
	GateSkipped ProvenanceGateStatus = "skipped"
)

// ConfigSource captures the source configuration for a build.
type ConfigSource struct {
	URI        string `json:"uri"`
	Digest     string `json:"digest"`
	EntryPoint string `json:"entry_point"`
}

// ProvenanceInvocation captures build invocation details.
type ProvenanceInvocation struct {
	ConfigSource ConfigSource   `json:"config_source"`
	Parameters   map[string]any `json:"parameters,omitempty"`
	Environment  map[string]any `json:"environment,omitempty"`
}

// ProvenanceSubject is an artifact subject with digest.
type ProvenanceSubject struct {
	Name   string            `json:"name"`
	Digest map[string]string `json:"digest"`
}

// Completeness tracks which build details are fully described.
type Completeness struct {
	Parameters  bool `json:"parameters"`
	Environment bool `json:"environment"`
	Materials   bool `json:"materials"`
}

// ProvenanceMetadata holds build metadata.
type ProvenanceMetadata struct {
	BuildStartedOn  string       `json:"build_started_on,omitempty"`
	BuildFinishedOn string       `json:"build_finished_on,omitempty"`
	Reproducible    bool         `json:"reproducible"`
	Completeness    Completeness `json:"completeness"`
}

// ProvenanceRecord is a single provenance attestation record.
type ProvenanceRecord struct {
	RecordID   string               `json:"record_id"`
	BuilderID  string               `json:"builder_id"`
	BuildType  string               `json:"build_type"`
	SLSALevel  SLSALevel            `json:"slsa_level"`
	Invocation ProvenanceInvocation `json:"invocation"`
	Subjects   []ProvenanceSubject  `json:"subjects"`
	Metadata   ProvenanceMetadata   `json:"metadata"`
	Verified   bool                 `json:"verified"`
	VerifiedAt string               `json:"verified_at,omitempty"`
	Verifier   string               `json:"verifier,omitempty"`
}

// ProvenanceFinding is a gate finding (issue or info).
type ProvenanceFinding struct {
	Code        string `json:"code"`
	Severity    string `json:"severity"`
	Message     string `json:"message"`
	Remediation string `json:"remediation"`
}

// ProvenanceGateSummary provides aggregate counts.
type ProvenanceGateSummary struct {
	TotalRecords    int  `json:"total_records"`
	VerifiedCount   int  `json:"verified_count"`
	UnverifiedCount int  `json:"unverified_count"`
	MinLevelMet     bool `json:"min_level_met"`
}

// ProvenanceGateResult is the output of the provenance gate.
type ProvenanceGateResult struct {
	SchemaVersion string               `json:"schema_version"`
	GateID        string               `json:"gate_id"`
	Status        ProvenanceGateStatus `json:"status"`
	SLSALevel     SLSALevel            `json:"slsa_level"`
	MinLevel      SLSALevel            `json:"min_level"`
	CheckedAt     string               `json:"checked_at"`
	Records       []ProvenanceRecord   `json:"records"`
	Findings      []ProvenanceFinding  `json:"findings"`
	Summary       ProvenanceGateSummary `json:"summary"`
}

// ProvenanceGateOptions configures the provenance gate check.
type ProvenanceGateOptions struct {
	GateID   string
	MinLevel SLSALevel
	Now      time.Time
}

// CheckProvenance runs the provenance gate against a set of records.
func CheckProvenance(records []ProvenanceRecord, opts ProvenanceGateOptions) ProvenanceGateResult {
	now := opts.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	minLevel := opts.MinLevel
	if !minLevel.IsValid() {
		minLevel = SLSA1
	}
	gateID := opts.GateID
	if gateID == "" {
		gateID = "provenance.slsa"
	}

	var findings []ProvenanceFinding
	verifiedCount := 0
	unverifiedCount := 0
	achievedLevel := SLSALevel("")
	allMeetMin := true

	if len(records) == 0 {
		findings = append(findings, ProvenanceFinding{
			Code:        "PROVENANCE_MISSING",
			Severity:    "high",
			Message:     "No provenance records provided.",
			Remediation: "Generate SLSA provenance during CI build and attach to release artifacts.",
		})
		return ProvenanceGateResult{
			SchemaVersion: "0.1.0",
			GateID:        gateID,
			Status:        GateFailed,
			SLSALevel:     "",
			MinLevel:      minLevel,
			CheckedAt:     now.Format(time.RFC3339),
			Records:       records,
			Findings:      findings,
			Summary: ProvenanceGateSummary{
				TotalRecords:    0,
				VerifiedCount:   0,
				UnverifiedCount: 0,
				MinLevelMet:     false,
			},
		}
	}

	for _, r := range records {
		recFindings := validateProvenanceRecord(r, minLevel)
		findings = append(findings, recFindings...)

		if r.Verified {
			verifiedCount++
		} else {
			unverifiedCount++
		}

		if !r.SLSALevel.MeetsMinimum(minLevel) {
			allMeetMin = false
		}

		if achievedLevel == "" || r.SLSALevel.Rank() < achievedLevel.Rank() {
			achievedLevel = r.SLSALevel
		}
	}

	minLevelMet := allMeetMin && unverifiedCount == 0

	status := GatePassed
	if !minLevelMet {
		status = GateFailed
	}

	return ProvenanceGateResult{
		SchemaVersion: "0.1.0",
		GateID:        gateID,
		Status:        status,
		SLSALevel:     achievedLevel,
		MinLevel:      minLevel,
		CheckedAt:     now.Format(time.RFC3339),
		Records:       records,
		Findings:      findings,
		Summary: ProvenanceGateSummary{
			TotalRecords:    len(records),
			VerifiedCount:   verifiedCount,
			UnverifiedCount: unverifiedCount,
			MinLevelMet:     minLevelMet,
		},
	}
}

func validateProvenanceRecord(r ProvenanceRecord, minLevel SLSALevel) []ProvenanceFinding {
	var findings []ProvenanceFinding

	if strings.TrimSpace(r.BuilderID) == "" {
		findings = append(findings, ProvenanceFinding{
			Code:        "PROVENANCE_MISSING_BUILDER",
			Severity:    "high",
			Message:     fmt.Sprintf("Record %s: builder_id is required.", r.RecordID),
			Remediation: "Ensure the build system populates builder_id in provenance.",
		})
	}
	if strings.TrimSpace(r.BuildType) == "" {
		findings = append(findings, ProvenanceFinding{
			Code:        "PROVENANCE_MISSING_BUILD_TYPE",
			Severity:    "high",
			Message:     fmt.Sprintf("Record %s: build_type is required.", r.RecordID),
			Remediation: "Specify build_type (e.g. github-actions, tekton, custom).",
		})
	}
	if strings.TrimSpace(r.Invocation.ConfigSource.URI) == "" {
		findings = append(findings, ProvenanceFinding{
			Code:        "PROVENANCE_MISSING_INVOCATION",
			Severity:    "medium",
			Message:     fmt.Sprintf("Record %s: invocation config_source URI is required.", r.RecordID),
			Remediation: "Populate invocation.config_source with repository URI and entry point.",
		})
	}
	if len(r.Subjects) == 0 {
		findings = append(findings, ProvenanceFinding{
			Code:        "PROVENANCE_NO_SUBJECTS",
			Severity:    "high",
			Message:     fmt.Sprintf("Record %s: at least one subject is required.", r.RecordID),
			Remediation: "List all produced artifacts as subjects with their digests.",
		})
	}
	if !r.Verified {
		findings = append(findings, ProvenanceFinding{
			Code:        "PROVENANCE_UNVERIFIED",
			Severity:    "medium",
			Message:     fmt.Sprintf("Record %s: provenance is not verified.", r.RecordID),
			Remediation: "Verify provenance signature before gate promotion.",
		})
	}
	if !r.SLSALevel.MeetsMinimum(minLevel) {
		findings = append(findings, ProvenanceFinding{
			Code:        "PROVENANCE_LEVEL_INSUFFICIENT",
			Severity:    "high",
			Message:     fmt.Sprintf("Record %s: SLSA level %s does not meet minimum %s.", r.RecordID, r.SLSALevel, minLevel),
			Remediation: fmt.Sprintf("Upgrade build pipeline to meet SLSA %s requirements.", minLevel),
		})
	}

	return findings
}

// ValidateProvenanceRecord checks a record for structural validity.
func ValidateProvenanceRecord(r ProvenanceRecord) []string {
	var errs []string

	if !recordIDPattern.MatchString(r.RecordID) {
		errs = append(errs, fmt.Sprintf("record_id %q must match %s", r.RecordID, recordIDPattern.String()))
	}
	if strings.TrimSpace(r.BuilderID) == "" {
		errs = append(errs, "builder_id is required")
	}
	if strings.TrimSpace(r.BuildType) == "" {
		errs = append(errs, "build_type is required")
	}
	if !r.SLSALevel.IsValid() {
		errs = append(errs, fmt.Sprintf("slsa_level %q is not valid", r.SLSALevel))
	}
	if strings.TrimSpace(r.Invocation.ConfigSource.URI) == "" {
		errs = append(errs, "invocation.config_source.uri is required")
	}
	if !hashPattern.MatchString(r.Invocation.ConfigSource.Digest) {
		errs = append(errs, fmt.Sprintf("invocation.config_source.digest %q must match hash pattern", r.Invocation.ConfigSource.Digest))
	}
	if len(r.Subjects) == 0 {
		errs = append(errs, "at least one subject is required")
	}

	return errs
}
