package compliance

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const RCPBundleFormat = "nomos.rcp-evidence-bundle.v1"

var (
	ErrBundleIncomplete = errors.New("evidence bundle incomplete")
	ErrArtifactMissing  = errors.New("required artifact missing")
)

// RCPArtifactStatus tracks resolution state.
type RCPArtifactStatus string

const (
	ArtPresent     RCPArtifactStatus = "present"
	ArtMissing     RCPArtifactStatus = "missing"
	ArtPlanned     RCPArtifactStatus = "planned"
	ArtNotRequired RCPArtifactStatus = "not_required"
)

// RCPCategory groups artifacts by regulated domain.
type RCPCategory string

const (
	CatControlMatrix  RCPCategory = "control_matrix"
	CatValidation     RCPCategory = "validation"
	CatTraining       RCPCategory = "training"
	CatAuditLog       RCPCategory = "audit_log"
	CatAttestation    RCPCategory = "attestation"
	CatQualitySystem  RCPCategory = "quality_system"
	CatSupplyChain    RCPCategory = "supply_chain"
	CatSecurity       RCPCategory = "security"
	CatDataIntegrity  RCPCategory = "data_integrity"
	CatLifecycle      RCPCategory = "lifecycle"
	CatGitHubOps      RCPCategory = "github_ops"
	CatAIGovernance   RCPCategory = "ai_governance"
	CatEvidenceIndex  RCPCategory = "evidence_index"
)

// RCPArtifactSpec declares a required or optional evidence artifact.
type RCPArtifactSpec struct {
	ID          string           `json:"id"`
	Category    RCPCategory `json:"category"`
	Path        string           `json:"path"`
	Required    bool             `json:"required"`
	Description string           `json:"description"`
	ControlRefs []string         `json:"control_refs,omitempty"`
}

// RCPArtifactResult is the resolved state of an artifact.
type RCPArtifactResult struct {
	ID          string           `json:"id"`
	Category    RCPCategory `json:"category"`
	Path        string           `json:"path"`
	Status      RCPArtifactStatus   `json:"status"`
	Hash        string           `json:"hash,omitempty"`
	Size        int64            `json:"size,omitempty"`
	Required    bool             `json:"required"`
	ControlRefs []string         `json:"control_refs,omitempty"`
}

// RCPBundleManifest is the top-level manifest for the evidence bundle.
type RCPBundleManifest struct {
	Format        string           `json:"format"`
	Product       string           `json:"product"`
	Version       string           `json:"version"`
	GeneratedAt   string           `json:"generated_at"`
	GeneratedBy   string           `json:"generated_by"`
	Commit        string           `json:"commit,omitempty"`
	Complete      bool             `json:"complete"`
	TotalArtifacts int             `json:"total_artifacts"`
	PresentCount  int              `json:"present_count"`
	MissingCount  int              `json:"missing_count"`
	Artifacts     []RCPArtifactResult `json:"artifacts"`
	Missing       []string         `json:"missing,omitempty"`
	RCPGateResult    RCPGateResult       `json:"gate_result"`
	ClaimBoundary string           `json:"claim_boundary"`
}

// RCPGateResult summarises the completeness gate.
type RCPGateResult struct {
	Pass     bool     `json:"pass"`
	Blockers []string `json:"blockers,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
}

// RCPBundleInput configures the bundle assembly.
type RCPBundleInput struct {
	Product     string
	Version     string
	Commit      string
	RepoRoot    string
	Artifacts   []RCPArtifactSpec
	GeneratedBy string
	Now         time.Time
}

// RCPBundleOutput holds the assembled bundle.
type RCPBundleOutput struct {
	Manifest RCPBundleManifest
}

// RCPArtifacts returns the standard regulated evidence artifacts.
func RCPArtifacts() []RCPArtifactSpec {
	return []RCPArtifactSpec{
		// Control matrix
		{ID: "control-matrix", Category: CatControlMatrix, Path: "docs/regulated/control-matrix/nomos-control-matrix.yaml", Required: true, Description: "Regulated control matrix", ControlRefs: []string{"ALL"}},
		{ID: "ref-to-control-map", Category: CatControlMatrix, Path: "docs/regulated/control-matrix/reference-to-control-map.yaml", Required: true, Description: "Reference-to-control cross-map", ControlRefs: []string{"ALL"}},

		// Validation
		{ID: "validation-master-plan", Category: CatValidation, Path: "docs/regulated/lifecycle/validation-master-plan.md", Required: true, Description: "Validation master plan", ControlRefs: []string{"CTL-VAL-001", "CTL-DI-001"}},
		{ID: "nomos-report", Category: CatValidation, Path: "nomos-report.json", Required: true, Description: "Nomos execution report", ControlRefs: []string{"CTL-VAL-001", "CTL-VAL-002", "CTL-VAL-003"}},
		{ID: "self-compliance-report", Category: CatValidation, Path: "docs/self-compliance-report.md", Required: false, Description: "Self-compliance dogfood report", ControlRefs: []string{"CTL-VAL-003"}},
		{ID: "requirements-sop", Category: CatValidation, Path: "docs/regulated/lifecycle/requirements-and-traceability-sop.md", Required: true, Description: "Requirements and traceability SOP", ControlRefs: []string{"CTL-VAL-004"}},

		// Training
		{ID: "training-sop", Category: CatTraining, Path: "docs/regulated/quality-system/training-and-competence-sop.md", Required: true, Description: "Training and competence SOP", ControlRefs: []string{"CTL-QS-004"}},

		// Audit log
		{ID: "audit-trail-sop", Category: CatAuditLog, Path: "docs/regulated/security-privacy/access-control-and-audit-trail-sop.md", Required: true, Description: "Access control and audit trail SOP", ControlRefs: []string{"CTL-DI-002", "CTL-AC-001"}},

		// Attestation
		{ID: "alcoa-envelope", Category: CatAttestation, Path: "templates/regulated/alcoa-evidence-envelope.yaml", Required: true, Description: "ALCOA+ evidence envelope template", ControlRefs: []string{"CTL-DI-003"}},

		// Quality system
		{ID: "quality-manual", Category: CatQualitySystem, Path: "docs/regulated/quality-system/quality-manual.md", Required: true, Description: "Quality manual", ControlRefs: []string{"CTL-QS-002"}},
		{ID: "qrm-sop", Category: CatQualitySystem, Path: "docs/regulated/quality-system/quality-risk-management-sop.md", Required: true, Description: "Quality risk management SOP", ControlRefs: []string{"CTL-QS-001"}},
		{ID: "deviation-capa-sop", Category: CatQualitySystem, Path: "docs/regulated/quality-system/deviation-capa-sop.md", Required: true, Description: "Deviation and CAPA SOP", ControlRefs: []string{"CTL-CC-003"}},
		{ID: "management-review-sop", Category: CatQualitySystem, Path: "docs/regulated/quality-system/management-review-sop.md", Required: true, Description: "Management review SOP", ControlRefs: []string{"CTL-QS-003"}},
		{ID: "internal-audit-sop", Category: CatQualitySystem, Path: "docs/regulated/quality-system/internal-audit-sop.md", Required: true, Description: "Internal audit SOP", ControlRefs: []string{"CTL-QS-005"}},
		{ID: "document-control-sop", Category: CatQualitySystem, Path: "docs/regulated/quality-system/document-and-record-control-sop.md", Required: true, Description: "Document and record control SOP", ControlRefs: []string{"CTL-CC-002"}},

		// Supply chain
		{ID: "secure-sdlc-sop", Category: CatSupplyChain, Path: "docs/regulated/security-privacy/secure-sdlc-sop.md", Required: true, Description: "Secure SDLC SOP", ControlRefs: []string{"CTL-SC-002", "CTL-SC-003"}},
		{ID: "sbom", Category: CatSupplyChain, Path: "sbom.json", Required: false, Description: "Software bill of materials", ControlRefs: []string{"CTL-SC-002"}},
		{ID: "slsa-provenance", Category: CatSupplyChain, Path: "slsa-provenance.json", Required: false, Description: "SLSA provenance attestation", ControlRefs: []string{"CTL-SC-003"}},

		// Security
		{ID: "vulnerability-sop", Category: CatSecurity, Path: "docs/regulated/security-privacy/vulnerability-and-incident-management-sop.md", Required: true, Description: "Vulnerability and incident management SOP", ControlRefs: []string{"CTL-SEC-002"}},
		{ID: "backup-bcdr-sop", Category: CatSecurity, Path: "docs/regulated/security-privacy/backup-restore-bcdr-sop.md", Required: false, Description: "Backup/restore and BCDR SOP"},

		// Data integrity
		{ID: "electronic-records-policy", Category: CatDataIntegrity, Path: "docs/regulated/data-integrity/electronic-records-and-signatures-policy.md", Required: true, Description: "Electronic records and signatures policy", ControlRefs: []string{"CTL-DI-001", "CTL-DI-002"}},
		{ID: "alcoa-policy", Category: CatDataIntegrity, Path: "docs/regulated/data-integrity/alcoa-data-integrity-policy.md", Required: true, Description: "ALCOA+ data integrity policy", ControlRefs: []string{"CTL-DI-003", "CTL-DI-004"}},

		// Lifecycle
		{ID: "sdmp", Category: CatLifecycle, Path: "docs/regulated/lifecycle/software-development-management-plan.md", Required: true, Description: "Software development management plan", ControlRefs: []string{"CTL-LC-001"}},
		{ID: "config-change-sop", Category: CatLifecycle, Path: "docs/regulated/lifecycle/configuration-and-change-control-sop.md", Required: true, Description: "Configuration and change control SOP", ControlRefs: []string{"CTL-CC-001"}},
		{ID: "release-retirement-sop", Category: CatLifecycle, Path: "docs/regulated/lifecycle/release-and-retirement-sop.md", Required: true, Description: "Release and retirement SOP", ControlRefs: []string{"CTL-LC-002"}},

		// GitHub ops
		{ID: "github-qms-baseline", Category: CatGitHubOps, Path: "docs/regulated/github-operating-model/github-qms-control-baseline.yaml", Required: true, Description: "GitHub QMS control baseline", ControlRefs: []string{"CTL-AC-001", "CTL-AC-002"}},

		// AI governance
		{ID: "ai-rag-governance", Category: CatAIGovernance, Path: "docs/regulated/ai-rag-governance/README.md", Required: true, Description: "AI/RAG governance policy", ControlRefs: []string{"CTL-AI-001"}},

		// Evidence index
		{ID: "evidence-ledger", Category: CatEvidenceIndex, Path: "docs/regulated/evidence-index/evidence-ledger.yaml", Required: true, Description: "Evidence ledger", ControlRefs: []string{"CTL-EV-001"}},
		{ID: "product-profile", Category: CatEvidenceIndex, Path: "docs/regulated/product-profiles/nomos.yaml", Required: true, Description: "Nomos product profile"},
		{ID: "reference-register", Category: CatEvidenceIndex, Path: "docs/regulated/reference-basis/external-reference-register.yaml", Required: true, Description: "External reference register"},
	}
}

// AssembleRCPBundle resolves all artifacts and checks completeness.
func AssembleRCPBundle(input RCPBundleInput) RCPBundleOutput {
	now := input.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}

	specs := input.Artifacts
	if len(specs) == 0 {
		specs = RCPArtifacts()
	}

	results := make([]RCPArtifactResult, 0, len(specs))
	var missing []string
	presentCount := 0

	for _, spec := range specs {
		r := rcpResolveArtifact(spec, input.RepoRoot)
		results = append(results, r)
		if r.Status == ArtPresent {
			presentCount++
		}
		if r.Required && r.Status == ArtMissing {
			missing = append(missing, spec.ID)
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].ID < results[j].ID
	})
	sort.Strings(missing)

	gate := evaluateGate(results, missing)
	complete := gate.Pass

	claimBoundary := "RCP evidence bundle complete. Ready for regulated review."
	if !complete {
		claimBoundary = fmt.Sprintf(
			"Bundle incomplete: %d required artifact(s) missing. No regulated-grade claim allowed.",
			len(missing),
		)
	}

	manifest := RCPBundleManifest{
		Format:         RCPBundleFormat,
		Product:        input.Product,
		Version:        input.Version,
		GeneratedAt:    now.Format(time.RFC3339),
		GeneratedBy:    input.GeneratedBy,
		Commit:         input.Commit,
		Complete:       complete,
		TotalArtifacts: len(results),
		PresentCount:   presentCount,
		MissingCount:   len(missing),
		Artifacts:      results,
		Missing:        missing,
		RCPGateResult:     gate,
		ClaimBoundary:  claimBoundary,
	}

	return RCPBundleOutput{Manifest: manifest}
}

// WriteRCPZipBundle writes the manifest and present artifacts into a ZIP.
func WriteRCPZipBundle(output RCPBundleOutput, repoRoot string, outPath string) error {
	f, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("create zip: %w", err)
	}
	defer f.Close()

	w := zip.NewWriter(f)
	defer w.Close()

	manifestData, err := json.MarshalIndent(output.Manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}
	mw, err := w.Create("rcp-bundle-manifest.json")
	if err != nil {
		return err
	}
	mw.Write(manifestData)

	for _, art := range output.Manifest.Artifacts {
		if art.Status != ArtPresent {
			continue
		}
		data, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(art.Path)))
		if err != nil {
			continue
		}
		aw, err := w.Create("evidence/" + art.Path)
		if err != nil {
			continue
		}
		aw.Write(data)
	}

	return nil
}

// MarshalRCPManifest writes the manifest as indented JSON.
func MarshalRCPManifest(w io.Writer, manifest RCPBundleManifest) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(manifest)
}

// ValidateRCPCompleteness returns an error if required artifacts are missing.
func ValidateRCPCompleteness(manifest RCPBundleManifest) error {
	if manifest.Complete {
		return nil
	}
	return fmt.Errorf("%w: missing %s", ErrBundleIncomplete, strings.Join(manifest.Missing, ", "))
}

func rcpResolveArtifact(spec RCPArtifactSpec, repoRoot string) RCPArtifactResult {
	r := RCPArtifactResult{
		ID:          spec.ID,
		Category:    spec.Category,
		Path:        spec.Path,
		Required:    spec.Required,
		ControlRefs: spec.ControlRefs,
	}

	if repoRoot == "" {
		if r.Required {
			r.Status = ArtMissing
		} else {
			r.Status = ArtPlanned
		}
		return r
	}

	fullPath := filepath.Join(repoRoot, filepath.FromSlash(spec.Path))
	info, err := os.Stat(fullPath)
	if err != nil {
		if r.Required {
			r.Status = ArtMissing
		} else {
			r.Status = ArtPlanned
		}
		return r
	}

	r.Status = ArtPresent
	r.Size = info.Size()

	if h, err := hashFileForBundle(fullPath); err == nil {
		r.Hash = h
	}

	return r
}

func evaluateGate(results []RCPArtifactResult, missing []string) RCPGateResult {
	gate := RCPGateResult{Pass: len(missing) == 0}

	for _, m := range missing {
		gate.Blockers = append(gate.Blockers, fmt.Sprintf("required artifact %q is missing", m))
	}

	// Warn about optional missing artifacts.
	for _, r := range results {
		if !r.Required && r.Status != ArtPresent {
			gate.Warnings = append(gate.Warnings, fmt.Sprintf("optional artifact %q is %s", r.ID, r.Status))
		}
	}

	return gate
}

func hashFileForBundle(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}
