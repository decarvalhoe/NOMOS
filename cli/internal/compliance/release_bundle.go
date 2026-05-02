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

const BundleFormat = "nomos.release-bundle.v1"

var (
	ErrMissingRequired = errors.New("required evidence artifact missing")
	ErrIncomplete      = errors.New("bundle is incomplete for target quality level")
)

// ArtifactStatus tracks the state of each evidence artifact.
type ArtifactStatus string

const (
	StatusPresent      ArtifactStatus = "present"
	StatusMissing      ArtifactStatus = "missing"
	ArtifactPlanned    ArtifactStatus = "planned"
	StatusNotRequired  ArtifactStatus = "not_required"
)

// QualityLevel is the target qualification level.
type QualityLevel string

const (
	LevelNQ0 QualityLevel = "NQ-0"
	LevelNQ1 QualityLevel = "NQ-1"
	LevelNQ3 QualityLevel = "NQ-3"
	LevelNQ5 QualityLevel = "NQ-5"
)

// ArtifactSpec declares a required or optional evidence artifact.
type ArtifactSpec struct {
	ID          string       `json:"id"`
	Category    string       `json:"category"`
	Path        string       `json:"path"`
	Required    bool         `json:"required"`
	MinLevel    QualityLevel `json:"min_level"`
	Description string       `json:"description"`
}

// ArtifactResult is the resolved state of an artifact in the bundle.
type ArtifactResult struct {
	ID       string         `json:"id"`
	Category string         `json:"category"`
	Path     string         `json:"path"`
	Status   ArtifactStatus `json:"status"`
	Hash     string         `json:"hash,omitempty"`
	Size     int64          `json:"size,omitempty"`
	Required bool           `json:"required"`
}

// BundleManifest is the top-level manifest written into the bundle.
type BundleManifest struct {
	Format       string           `json:"format"`
	Product      string           `json:"product"`
	Version      string           `json:"version"`
	TargetLevel  QualityLevel     `json:"target_level"`
	GeneratedAt  string           `json:"generated_at"`
	GeneratedBy  string           `json:"generated_by"`
	Commit       string           `json:"commit,omitempty"`
	Complete     bool             `json:"complete"`
	Artifacts    []ArtifactResult `json:"artifacts"`
	Missing      []string         `json:"missing,omitempty"`
	ClaimBoundary string          `json:"claim_boundary"`
}

// BundleInput configures a release bundle generation.
type BundleInput struct {
	Product     string
	Version     string
	TargetLevel QualityLevel
	Commit      string
	RepoRoot    string
	Artifacts   []ArtifactSpec
	GeneratedBy string
	Now         time.Time
}

// BundleOutput holds the result of bundle assembly.
type BundleOutput struct {
	Manifest BundleManifest
	ZipPath  string
}

// NQ5Artifacts returns the standard evidence artifacts required for NQ-5.
func NQ5Artifacts() []ArtifactSpec {
	return []ArtifactSpec{
		{ID: "nomos-report", Category: "validation", Path: "nomos-report.json", Required: true, MinLevel: LevelNQ1, Description: "Nomos execution report"},
		{ID: "coverage-report", Category: "validation", Path: "coverage-report.md", Required: true, MinLevel: LevelNQ3, Description: "Coverage report markdown"},
		{ID: "control-matrix", Category: "controls", Path: "docs/regulated/control-matrix/nomos-control-matrix.yaml", Required: true, MinLevel: LevelNQ3, Description: "Regulated control matrix"},
		{ID: "evidence-ledger", Category: "evidence", Path: "docs/regulated/evidence-index/evidence-ledger.yaml", Required: true, MinLevel: LevelNQ1, Description: "Evidence ledger"},
		{ID: "source-manifest", Category: "sources", Path: "docs/canonical/source-manifest.yaml", Required: true, MinLevel: LevelNQ1, Description: "Source manifest"},
		{ID: "validation-master-plan", Category: "validation", Path: "docs/regulated/lifecycle/validation-master-plan.md", Required: true, MinLevel: LevelNQ3, Description: "Validation master plan"},
		{ID: "quality-manual", Category: "quality", Path: "docs/regulated/quality-system/quality-manual.md", Required: true, MinLevel: LevelNQ3, Description: "Quality manual"},
		{ID: "sbom", Category: "supply-chain", Path: "sbom.json", Required: false, MinLevel: LevelNQ5, Description: "Software bill of materials"},
		{ID: "slsa-provenance", Category: "supply-chain", Path: "slsa-provenance.json", Required: false, MinLevel: LevelNQ5, Description: "SLSA provenance attestation"},
		{ID: "product-profile", Category: "product", Path: "docs/regulated/product-profiles/nomos.yaml", Required: true, MinLevel: LevelNQ1, Description: "Product profile"},
		{ID: "self-compliance", Category: "validation", Path: "docs/self-compliance-report.md", Required: false, MinLevel: LevelNQ3, Description: "Self-compliance report"},
		{ID: "reference-registry", Category: "references", Path: "docs/regulated/reference-basis/reference-registry.yaml", Required: true, MinLevel: LevelNQ3, Description: "Reference registry"},
	}
}

// AssembleBundle resolves artifacts, checks completeness, and optionally
// writes a ZIP archive.
func AssembleBundle(input BundleInput) (BundleOutput, error) {
	now := input.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}

	specs := input.Artifacts
	if len(specs) == 0 {
		specs = NQ5Artifacts()
	}

	results := make([]ArtifactResult, 0, len(specs))
	var missing []string

	for _, spec := range specs {
		result := resolveArtifact(spec, input.RepoRoot, input.TargetLevel)
		results = append(results, result)
		if result.Required && result.Status == StatusMissing {
			missing = append(missing, spec.ID)
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].ID < results[j].ID
	})
	sort.Strings(missing)

	complete := len(missing) == 0

	claimBoundary := "Release bundle complete for " + string(input.TargetLevel) + "."
	if !complete {
		claimBoundary = fmt.Sprintf(
			"Bundle incomplete: %d required artifact(s) missing. No regulated-grade claim allowed.",
			len(missing),
		)
	}

	manifest := BundleManifest{
		Format:        BundleFormat,
		Product:       input.Product,
		Version:       input.Version,
		TargetLevel:   input.TargetLevel,
		GeneratedAt:   now.Format(time.RFC3339),
		GeneratedBy:   input.GeneratedBy,
		Commit:        input.Commit,
		Complete:      complete,
		Artifacts:     results,
		Missing:       missing,
		ClaimBoundary: claimBoundary,
	}

	return BundleOutput{Manifest: manifest}, nil
}

// WriteZipBundle writes the bundle manifest and all present artifacts
// into a ZIP archive at outPath.
func WriteZipBundle(output BundleOutput, repoRoot string, outPath string) error {
	f, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("create zip: %w", err)
	}
	defer f.Close()

	w := zip.NewWriter(f)
	defer w.Close()

	// Write manifest.
	manifestData, err := json.MarshalIndent(output.Manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}
	mw, err := w.Create("bundle-manifest.json")
	if err != nil {
		return err
	}
	if _, err := mw.Write(manifestData); err != nil {
		return err
	}

	// Write present artifacts.
	for _, art := range output.Manifest.Artifacts {
		if art.Status != StatusPresent {
			continue
		}
		srcPath := filepath.Join(repoRoot, filepath.FromSlash(art.Path))
		data, err := os.ReadFile(srcPath)
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

// MarshalManifest writes the manifest as indented JSON.
func MarshalManifest(w io.Writer, manifest BundleManifest) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(manifest)
}

// ValidateCompleteness checks if the bundle meets the target quality level.
func ValidateCompleteness(manifest BundleManifest) error {
	if manifest.Complete {
		return nil
	}
	return fmt.Errorf("%w: missing %s",
		ErrIncomplete, strings.Join(manifest.Missing, ", "))
}

func resolveArtifact(spec ArtifactSpec, repoRoot string, targetLevel QualityLevel) ArtifactResult {
	result := ArtifactResult{
		ID:       spec.ID,
		Category: spec.Category,
		Path:     spec.Path,
		Required: spec.Required && levelGTE(targetLevel, spec.MinLevel),
	}

	if repoRoot == "" {
		if result.Required {
			result.Status = StatusMissing
		} else {
			result.Status = ArtifactPlanned
		}
		return result
	}

	fullPath := filepath.Join(repoRoot, filepath.FromSlash(spec.Path))
	info, err := os.Stat(fullPath)
	if err != nil {
		if result.Required {
			result.Status = StatusMissing
		} else {
			result.Status = ArtifactPlanned
		}
		return result
	}

	result.Status = StatusPresent
	result.Size = info.Size()

	hash, err := hashFileContent(fullPath)
	if err == nil {
		result.Hash = hash
	}

	return result
}

func hashFileContent(path string) (string, error) {
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

func levelGTE(target, minimum QualityLevel) bool {
	order := map[QualityLevel]int{
		LevelNQ0: 0,
		LevelNQ1: 1,
		LevelNQ3: 3,
		LevelNQ5: 5,
	}
	return order[target] >= order[minimum]
}
