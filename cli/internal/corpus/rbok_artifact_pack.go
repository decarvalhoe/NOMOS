package corpus

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// RBOKLawbookArtifactPackOptions configures release-gate artifact generation.
type RBOKLawbookArtifactPackOptions struct {
	Now            time.Time
	CorpusID       string
	ProjectID      string
	ScannerVersion string
}

// RBOKLawbookArtifactPackResult reports which artifacts were produced.
type RBOKLawbookArtifactPackResult struct {
	Profile       string           `json:"profile"`
	ArtifactsDir  string           `json:"artifacts_dir"`
	DocumentCount int              `json:"document_count"`
	TotalNodes    int              `json:"total_nodes"`
	Artifacts     []string         `json:"artifacts"`
	Diagnosis     DiagnoseVerdict  `json:"diagnosis"`
	Governance    GovernanceResult `json:"governance"`
	Attestation   string           `json:"attestation"`
}

type lawbookMarkdownSource struct {
	Path           string
	AbsPath        string
	Hash           string
	Classification RBOKSourceClassification
}

// WriteRBOKLawbookArtifactPack builds the canonical rbok-lawbook artifact set
// expected by the release gate without writing inside the corpus root.
func WriteRBOKLawbookArtifactPack(root string, outDir string, opts RBOKLawbookArtifactPackOptions) (RBOKLawbookArtifactPackResult, error) {
	if strings.TrimSpace(root) == "" {
		return RBOKLawbookArtifactPackResult{}, fmt.Errorf("corpus root is required")
	}
	if strings.TrimSpace(outDir) == "" {
		return RBOKLawbookArtifactPackResult{}, fmt.Errorf("artifacts dir is required")
	}

	absRoot, err := filepath.Abs(root)
	if err != nil {
		return RBOKLawbookArtifactPackResult{}, fmt.Errorf("resolve corpus root: %w", err)
	}
	info, err := os.Stat(absRoot)
	if err != nil {
		return RBOKLawbookArtifactPackResult{}, fmt.Errorf("stat corpus root: %w", err)
	}
	if !info.IsDir() {
		return RBOKLawbookArtifactPackResult{}, fmt.Errorf("corpus root must be a directory")
	}

	now := opts.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}

	diagnosis, err := DiagnoseProfile(ProfileRBOKLawbook, absRoot)
	if err != nil {
		return RBOKLawbookArtifactPackResult{}, err
	}
	if diagnosis.Verdict == "blocked" {
		return RBOKLawbookArtifactPackResult{
			Profile:      ProfileRBOKLawbook,
			ArtifactsDir: outDir,
			Diagnosis:    diagnosis,
		}, fmt.Errorf("profile %s blocked: %s", ProfileRBOKLawbook, diagnosis.Summary)
	}

	sources, fingerprints, err := collectRBOKLawbookMarkdownSources(absRoot)
	if err != nil {
		return RBOKLawbookArtifactPackResult{}, err
	}
	if len(sources) == 0 {
		return RBOKLawbookArtifactPackResult{}, fmt.Errorf("no active Markdown lawbook sources found")
	}

	generatedAt := now.Format(time.RFC3339)
	feeds := make([]LawbookFeed, 0, len(sources))
	for _, source := range sources {
		data, err := os.ReadFile(source.AbsPath)
		if err != nil {
			return RBOKLawbookArtifactPackResult{}, fmt.Errorf("read %s: %w", source.Path, err)
		}
		docSlug := lawbookSlugify(strings.TrimSuffix(source.Path, filepath.Ext(source.Path)))
		if docSlug == "" {
			docSlug = "rbok-lawbook"
		}
		extracted := ExtractMarkdown(string(data), docSlug)
		if len(extracted.Nodes) == 0 {
			continue
		}
		defaults := NodeDefaults{
			DocumentID: "DOC-" + toUpperSlug(strings.TrimSuffix(source.Path, filepath.Ext(source.Path))),
			SourcePath: source.Path,
			SourceHash: source.Hash,
			Domain:     rbokDomainFromPath(source.Path),
			Status:     StatusActive,
			Priority:   rbokPriorityFromClassification(source.Classification),
		}
		if defaults.DocumentID == "DOC-" {
			defaults.DocumentID = "DOC-RBOK"
		}
		NormalizeExtractResult(&extracted, defaults)
		feedID := docSlug + "-feed"
		feeds = append(feeds, BuildNormalizedFeed(extracted, feedID, defaults, generatedAt))
	}
	if len(feeds) == 0 {
		return RBOKLawbookArtifactPackResult{}, fmt.Errorf("Markdown sources produced 0 lawbook nodes")
	}

	assembly := AssembleMultiFeed(feeds, MultiAssembleOptions{
		Now:    now,
		OutDir: outDir,
	})
	if err := WriteMultiFeedArtifacts(assembly, outDir); err != nil {
		return RBOKLawbookArtifactPackResult{}, err
	}

	governance := governanceFromDiagnosis(diagnosis)
	governancePath := filepath.Join(outDir, "rbok-governance.json")
	if err := writeJSONFile(governancePath, governance); err != nil {
		return RBOKLawbookArtifactPackResult{}, fmt.Errorf("write rbok-governance.json: %w", err)
	}

	verdict := diagnosisVerdictToCorpusVerdict(diagnosis.Verdict)
	confidence := firstNonEmpty(diagnosis.Confidence, "medium")
	attestation, err := GenerateCorpusAttestation(CorpusAttestationOptions{
		CorpusID:       firstNonEmpty(opts.CorpusID, ProfileRBOKLawbook),
		ProjectID:      firstNonEmpty(opts.ProjectID, "nomos"),
		ScannerVersion: firstNonEmpty(opts.ScannerVersion, "unknown"),
		Scope:          "full_profile",
		Verdict:        verdict,
		Confidence:     confidence,
		FilesScanned:   len(fingerprints),
		UnitsExtracted: assembly.TotalNodes,
		ScannedFiles:   fingerprints,
		Diagnosis:      &diagnosis,
		Now:            now,
		Metadata: map[string]any{
			"profile":     ProfileRBOKLawbook,
			"corpus_root": absRoot,
		},
	})
	if err != nil {
		return RBOKLawbookArtifactPackResult{}, err
	}
	attestationPath := filepath.Join(outDir, "rbok-attestation.json")
	f, err := os.Create(attestationPath)
	if err != nil {
		return RBOKLawbookArtifactPackResult{}, fmt.Errorf("create rbok-attestation.json: %w", err)
	}
	if err := WriteAttestation(f, attestation); err != nil {
		_ = f.Close()
		return RBOKLawbookArtifactPackResult{}, fmt.Errorf("write rbok-attestation.json: %w", err)
	}
	if err := f.Close(); err != nil {
		return RBOKLawbookArtifactPackResult{}, fmt.Errorf("close rbok-attestation.json: %w", err)
	}

	// Emit certified TOC artifact.
	certifiedTOC := BuildCertifiedTOCFromFeeds(feeds, firstNonEmpty(opts.CorpusID, ProfileRBOKLawbook))
	if err := WriteCertifiedTOCArtifact(certifiedTOC, outDir); err != nil {
		return RBOKLawbookArtifactPackResult{}, fmt.Errorf("write rbok-certified-toc.json: %w", err)
	}

	artifacts := []string{
		"rbok-lawbook-feed.json",
		"rbok-lawbook-index.json",
		"rbok-rag-metadata.json",
		"rbok-engine-import.json",
		"rbok-governance.json",
		"rbok-attestation.json",
		"rbok-certified-toc.json",
	}
	sort.Strings(artifacts)

	return RBOKLawbookArtifactPackResult{
		Profile:       ProfileRBOKLawbook,
		ArtifactsDir:  outDir,
		DocumentCount: assembly.DocumentCount,
		TotalNodes:    assembly.TotalNodes,
		Artifacts:     artifacts,
		Diagnosis:     diagnosis,
		Governance:    governance,
		Attestation:   "rbok-attestation.json",
	}, nil
}

func collectRBOKLawbookMarkdownSources(root string) ([]lawbookMarkdownSource, []string, error) {
	var sources []lawbookMarkdownSource
	var fingerprints []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() || !d.Type().IsRegular() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		hash, err := hashFileSHA256Digest(path)
		if err != nil {
			return fmt.Errorf("hash %s: %w", rel, err)
		}
		fingerprints = append(fingerprints, rel+" "+hash)
		classification := ClassifyRBOKSource(rel)
		ext := strings.ToLower(filepath.Ext(rel))
		if classification.Role == RoleLawbook && (ext == ".md" || ext == ".mdx") {
			sources = append(sources, lawbookMarkdownSource{
				Path:           rel,
				AbsPath:        path,
				Hash:           hash,
				Classification: classification,
			})
		}
		return nil
	})
	if err != nil {
		return nil, nil, fmt.Errorf("walk corpus: %w", err)
	}
	sort.Slice(sources, func(i, j int) bool { return sources[i].Path < sources[j].Path })
	sort.Strings(fingerprints)
	return sources, fingerprints, nil
}

func hashFileSHA256Digest(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

func rbokDomainFromPath(path string) string {
	parts := strings.Split(path, "/")
	if len(parts) == 0 {
		return "rbok-lawbook"
	}
	top := strings.ToLower(parts[0])
	switch {
	case strings.HasPrefix(top, "00_meta"):
		return "rbok-meta"
	case strings.HasPrefix(top, "01_referentiel"):
		return "rbok-referentiel"
	case strings.HasPrefix(top, "02_domaines"):
		if len(parts) > 1 {
			return "rbok-" + lawbookSlugify(parts[1])
		}
		return "rbok-domaines"
	case strings.HasPrefix(top, "03_parcours"):
		return "rbok-parcours"
	default:
		return "rbok-lawbook"
	}
}

func rbokPriorityFromClassification(c RBOKSourceClassification) LawbookPriority {
	switch c.Priority {
	case "primary":
		return PriorityHigh
	case "secondary":
		return PriorityMedium
	default:
		return PriorityMedium
	}
}

func governanceFromDiagnosis(d DiagnoseVerdict) GovernanceResult {
	result := GovernanceResult{
		Verdict: diagnosisVerdictToCorpusVerdict(d.Verdict),
	}
	for i, blocker := range d.Blockers {
		result.Findings = append(result.Findings, Finding{
			ID:          fmt.Sprintf("DIAG-%04d", i+1),
			Severity:    "critical",
			Blocking:    true,
			Field:       "profile_diagnosis",
			Message:     blocker,
			Remediation: "Resolve the blocking diagnosis before release admission.",
		})
	}
	for i, warning := range d.Warnings {
		result.Findings = append(result.Findings, Finding{
			ID:          fmt.Sprintf("DIAG-W%04d", i+1),
			Severity:    "medium",
			Blocking:    false,
			Field:       "profile_diagnosis",
			Message:     warning,
			Remediation: "Review and either resolve or explicitly accept this warning.",
		})
	}
	for _, finding := range result.Findings {
		if finding.Blocking {
			result.Blocking++
		}
	}
	result.TotalFindings = len(result.Findings)
	return result
}
