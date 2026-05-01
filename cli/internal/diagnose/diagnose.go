package diagnose

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/RBOKproject/Nomos/cli/internal/detect"
	"github.com/RBOKproject/Nomos/cli/internal/output"
)

const (
	metadataKey = "diagnose"

	verdictInScope    = "in_scope"
	verdictPartial    = "partial"
	verdictBlocked    = "blocked"
	verdictOutOfScope = "out_of_scope"

	verdictCorpusAdmissible = "corpus_admissible"
	verdictCorpusPartial    = "corpus_partial"
	verdictCorpusBlocked    = "corpus_blocked"
)

type Options struct {
	Now         time.Time
	ToolVersion string
	Command     []string
	Mode        string
}

type Classification struct {
	PreliminaryVerdict string                  `json:"preliminary_verdict"`
	Confidence         string                  `json:"confidence"`
	Escalation         string                  `json:"escalation"`
	RepositoryMode     string                  `json:"repository_mode,omitempty"`
	Blockers           []Gap                   `json:"blockers,omitempty"`
	MissingEvidence    []Gap                   `json:"missing_evidence,omitempty"`
	Surfaces           []SurfaceClassification `json:"surfaces,omitempty"`
}

type Gap struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Blocking    bool   `json:"blocking"`
	Remediation string `json:"remediation"`
}

type SurfaceClassification struct {
	Name          string `json:"name"`
	Confidence    string `json:"confidence"`
	EvidenceCount int    `json:"evidence_count"`
}

type repositoryEvidence struct {
	projectManifest string
	projectMode     string
	sourceManifest  string
	canonicalMatrix string
	tests           []string
	ci              []string
	decisionRecords []string
}

func Diagnose(root string, options Options) (output.Report, error) {
	detection, err := detect.Detect(root)
	if err != nil {
		return output.Report{}, err
	}
	repoEvidence, err := scanRepositoryEvidence(detection.Root)
	if err != nil {
		return output.Report{}, err
	}

	now := options.Now.UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}
	version := strings.TrimSpace(options.ToolVersion)
	if version == "" {
		version = "dev"
	}

	surfaces := classifySurfaces(detection.Surfaces)
	requestedMode := normalizeMode(options.Mode)
	corpusMode := requestedMode == "canonical_corpus" || repoEvidence.projectMode == "canonical_corpus"
	blockers, missing := classifyGaps(surfaces, repoEvidence, detection.CI, corpusMode)
	classification := classifyRepository(surfaces, blockers, missing, corpusMode)

	evidence := buildEvidence(surfaces, repoEvidence, detection.CI)
	findings := buildFindings(blockers, missing, surfaceEvidenceIDs(productSurfaces(surfaces)))
	checks := buildChecks(surfaces, repoEvidence, detection.CI, findings, evidence)
	verdict := buildVerdict(classification)

	projectID := slug(filepath.Base(detection.Root))
	if projectID == "" || projectID == "." {
		projectID = "repository"
	}

	report := output.Report{
		SchemaVersion: output.SchemaVersion,
		ReportType:    output.ReportType,
		GeneratedAt:   now.Format(time.RFC3339),
		Run: output.Run{
			ID:   "run-" + now.Format("20060102-150405"),
			Mode: reportMode(corpusMode),
			Tool: output.Tool{
				Name:    "nomos",
				Version: version,
			},
			Command: options.Command,
			Environment: &output.Environment{
				CI:     isCI(),
				Runner: "local",
				OS:     runtime.GOOS,
			},
		},
		Project: output.Project{
			ID:           projectID,
			Name:         titleFromSlug(projectID),
			Domain:       "unknown",
			RiskLevel:    "medium",
			ManifestPath: repoEvidence.projectManifest,
		},
		Summary:  buildSummary(surfaces, findings, evidence, classification.PreliminaryVerdict),
		Verdict:  verdict,
		Checks:   checks,
		Findings: findings,
		Evidence: evidence,
		Metadata: output.Metadata{
			metadataKey: classification,
		},
	}

	return output.Normalize(report), nil
}

func classifySurfaces(items []detect.SurfaceFinding) []SurfaceClassification {
	surfaces := make([]SurfaceClassification, 0, len(items))
	for _, item := range items {
		surfaces = append(surfaces, SurfaceClassification{
			Name:          item.Name,
			Confidence:    item.Confidence,
			EvidenceCount: len(item.Evidence),
		})
	}
	sort.Slice(surfaces, func(i, j int) bool {
		return surfaces[i].Name < surfaces[j].Name
	})
	return surfaces
}

func classifyGaps(surfaces []SurfaceClassification, repo repositoryEvidence, ci []detect.CIFinding, corpusMode bool) ([]Gap, []Gap) {
	if corpusMode {
		return classifyCorpusGaps(repo)
	}
	if !hasProductSurface(surfaces) {
		return nil, nil
	}

	var blockers []Gap
	var missing []Gap

	if repo.projectManifest == "" {
		blockers = append(blockers, Gap{
			ID:          "project_manifest",
			Label:       "Nomos project manifest",
			Blocking:    true,
			Remediation: "Add nomos.project.yaml with scope, owners, risk level, and declared surfaces.",
		})
	}
	if repo.sourceManifest == "" {
		blockers = append(blockers, Gap{
			ID:          "source_manifest",
			Label:       "Source manifest",
			Blocking:    true,
			Remediation: "Add source-manifest.yaml with active authoritative sources for the detected product surfaces.",
		})
	}
	if repo.canonicalMatrix == "" {
		missing = append(missing, Gap{
			ID:          "canonical_matrix",
			Label:       "Canonical matrix",
			Blocking:    false,
			Remediation: "Add canonical-matrix.yaml linking source units to product behavior and tests.",
		})
	}
	if len(repo.tests) == 0 {
		missing = append(missing, Gap{
			ID:          "test_evidence",
			Label:       "Test evidence",
			Blocking:    false,
			Remediation: "Add or reference automated tests for each critical detected surface.",
		})
	}
	if len(repo.decisionRecords) == 0 {
		missing = append(missing, Gap{
			ID:          "owner_or_decision_record",
			Label:       "Owner or decision record",
			Blocking:    false,
			Remediation: "Add ADR or decision evidence for accepted scope assumptions and ownership.",
		})
	}
	if len(repo.ci) == 0 && len(ci) == 0 {
		missing = append(missing, Gap{
			ID:          "ci_evidence",
			Label:       "CI evidence",
			Blocking:    false,
			Remediation: "Add CI workflow evidence showing how Nomos checks and tests are run.",
		})
	}

	sortGaps(blockers)
	sortGaps(missing)
	return blockers, missing
}

func classifyCorpusGaps(repo repositoryEvidence) ([]Gap, []Gap) {
	var missing []Gap
	if repo.projectManifest == "" {
		missing = append(missing, Gap{
			ID:          "project_manifest",
			Label:       "Nomos corpus project manifest",
			Blocking:    false,
			Remediation: "Keep the source repository untouched and provide a sidecar nomos.project.yaml declaring mode: canonical_corpus.",
		})
	}
	if repo.sourceManifest == "" {
		missing = append(missing, Gap{
			ID:          "source_manifest",
			Label:       "Corpus source manifest",
			Blocking:    false,
			Remediation: "Run nomos corpus scan and nomos corpus manifest to create a sidecar source manifest outside the source corpus.",
		})
	}
	sortGaps(missing)
	return nil, missing
}

func classifyRepository(surfaces []SurfaceClassification, blockers []Gap, missing []Gap, corpusMode bool) Classification {
	classification := Classification{
		PreliminaryVerdict: verdictInScope,
		Confidence:         "high",
		Escalation:         "none",
		RepositoryMode:     "product",
		Blockers:           blockers,
		MissingEvidence:    missing,
		Surfaces:           surfaces,
	}

	if corpusMode {
		classification.RepositoryMode = "canonical_corpus"
		classification.PreliminaryVerdict = verdictCorpusAdmissible
		if len(blockers) > 0 {
			classification.PreliminaryVerdict = verdictCorpusBlocked
			classification.Confidence = "low"
			classification.Escalation = "product_owner"
		} else if len(missing) > 0 {
			classification.PreliminaryVerdict = verdictCorpusPartial
			classification.Confidence = "medium"
			classification.Escalation = "domain_owner"
		}
		return classification
	}

	switch {
	case !hasProductSurface(surfaces):
		classification.PreliminaryVerdict = verdictOutOfScope
		classification.Confidence = "medium"
	case len(blockers) > 0:
		classification.PreliminaryVerdict = verdictBlocked
		classification.Confidence = "low"
		classification.Escalation = "product_owner"
	case len(missing) > 0:
		classification.PreliminaryVerdict = verdictPartial
		classification.Confidence = "medium"
		classification.Escalation = "domain_owner"
	}

	return classification
}

func buildEvidence(surfaces []SurfaceClassification, repo repositoryEvidence, ci []detect.CIFinding) []output.Evidence {
	var evidence []output.Evidence
	if repo.projectManifest != "" {
		evidence = append(evidence, artifactEvidence("EVIDENCE-PROJECT-MANIFEST", "manual_review", "Nomos project manifest exists.", repo.projectManifest))
	}
	if repo.sourceManifest != "" {
		evidence = append(evidence, artifactEvidence("EVIDENCE-SOURCE-MANIFEST", "source_manifest", "Source manifest exists.", repo.sourceManifest))
	}
	if repo.canonicalMatrix != "" {
		evidence = append(evidence, artifactEvidence("EVIDENCE-CANONICAL-MATRIX", "canonical_matrix", "Canonical matrix exists.", repo.canonicalMatrix))
	}
	if len(repo.tests) > 0 {
		evidence = append(evidence, artifactEvidence("EVIDENCE-TESTS", "test_result", "Automated test evidence exists.", repo.tests[0]))
	}
	if len(repo.decisionRecords) > 0 {
		evidence = append(evidence, artifactEvidence("EVIDENCE-DECISION-RECORD", "decision_record", "Decision or ownership evidence exists.", repo.decisionRecords[0]))
	}
	if len(repo.ci) > 0 {
		evidence = append(evidence, artifactEvidence("EVIDENCE-CI", "ci_run", "CI configuration evidence exists.", repo.ci[0]))
	} else if len(ci) > 0 && len(ci[0].Evidence) > 0 {
		evidence = append(evidence, artifactEvidence("EVIDENCE-CI", "ci_run", "CI configuration evidence exists.", ci[0].Evidence[0].Path))
	}
	for _, surface := range surfaces {
		evidence = append(evidence, output.Evidence{
			ID:          "EVIDENCE-SURFACE-" + idPart(surface.Name),
			Type:        "code_reference",
			Description: fmt.Sprintf("Detected %s surface with %s confidence.", surface.Name, surface.Confidence),
			Target: &output.Target{
				Type: targetTypeForSurface(surface.Name),
				ID:   surface.Name,
			},
			Producer: "nomos",
			Metadata: output.Metadata{
				"evidence_count": surface.EvidenceCount,
			},
		})
	}
	return evidence
}

func buildFindings(blockers []Gap, missing []Gap, evidenceIDs []string) []output.Finding {
	var findings []output.Finding
	index := 1
	for _, gap := range blockers {
		findings = append(findings, findingForGap(index, gap, "high", evidenceIDs))
		index++
	}
	for _, gap := range missing {
		findings = append(findings, findingForGap(index, gap, "medium", evidenceIDs))
		index++
	}
	return findings
}

func findingForGap(index int, gap Gap, severity string, evidenceIDs []string) output.Finding {
	return output.Finding{
		ID:          fmt.Sprintf("NOMOS-DIAG-%03d", index),
		Code:        "NOMOS_EVIDENCE_MISSING",
		Severity:    severity,
		Status:      "open",
		Blocking:    gap.Blocking,
		Message:     fmt.Sprintf("Missing required evidence: %s.", gap.Label),
		Remediation: gap.Remediation,
		Target: output.Target{
			Type: "project",
			ID:   gap.ID,
		},
		EvidenceIDs: evidenceIDs,
	}
}

func buildChecks(
	surfaces []SurfaceClassification,
	repo repositoryEvidence,
	ci []detect.CIFinding,
	findings []output.Finding,
	evidence []output.Evidence,
) []output.Check {
	findingIDs := make([]string, 0, len(findings))
	blocking := false
	for _, finding := range findings {
		findingIDs = append(findingIDs, finding.ID)
		if finding.Blocking {
			blocking = true
		}
	}
	sort.Strings(findingIDs)

	evidenceIDs := make([]string, 0, len(evidence))
	for _, item := range evidence {
		evidenceIDs = append(evidenceIDs, item.ID)
	}
	sort.Strings(evidenceIDs)

	surfaceStatus := "passed"
	surfaceSeverity := "info"
	surfaceMessage := "Product surfaces detected."
	if !hasProductSurface(surfaces) {
		surfaceStatus = "not_applicable"
		surfaceSeverity = "low"
		surfaceMessage = "No product surface detected."
	}

	requiredStatus := "passed"
	requiredSeverity := "info"
	requiredMessage := "Required admission evidence is present."
	if len(findings) > 0 {
		requiredStatus = "warning"
		requiredSeverity = "medium"
		requiredMessage = "Admission evidence is incomplete."
	}
	if blocking {
		requiredStatus = "blocked"
		requiredSeverity = "high"
		requiredMessage = "Admission evidence has blocking gaps."
	}

	ciStatus := "passed"
	ciSeverity := "info"
	ciMessage := "CI evidence detected."
	if len(repo.ci) == 0 && len(ci) == 0 {
		ciStatus = "warning"
		ciSeverity = "medium"
		ciMessage = "No CI evidence detected."
		if !hasProductSurface(surfaces) {
			ciStatus = "not_applicable"
			ciSeverity = "low"
		}
	}

	return []output.Check{
		{
			ID:          "diagnose.ci",
			Name:        "CI evidence",
			Category:    "execution",
			Status:      ciStatus,
			Severity:    ciSeverity,
			EvidenceIDs: evidenceIDsByPrefix(evidence, "EVIDENCE-CI"),
			Message:     ciMessage,
		},
		{
			ID:          "diagnose.required_evidence",
			Name:        "Required admission evidence",
			Category:    "sources",
			Status:      requiredStatus,
			Severity:    requiredSeverity,
			FindingIDs:  findingIDs,
			EvidenceIDs: evidenceIDs,
			Message:     requiredMessage,
		},
		{
			ID:          "diagnose.surfaces",
			Name:        "Detected product surfaces",
			Category:    "scope",
			Status:      surfaceStatus,
			Severity:    surfaceSeverity,
			EvidenceIDs: evidenceIDsByPrefix(evidence, "EVIDENCE-SURFACE-"),
			Message:     surfaceMessage,
		},
	}
}

func buildVerdict(classification Classification) output.Verdict {
	switch classification.PreliminaryVerdict {
	case verdictInScope:
		return output.Verdict{
			Status:   "pass",
			Severity: "info",
			Blocking: false,
			Summary:  "Preliminary verdict in_scope: detected product surfaces have the required baseline Nomos evidence.",
			NextActions: []string{
				"Run validate once schema and canonical gates are available.",
			},
		}
	case verdictPartial:
		return output.Verdict{
			Status:      "warn",
			Severity:    "medium",
			Blocking:    false,
			Summary:     "Preliminary verdict partial: detected product surfaces can enter Nomos with explicit missing evidence.",
			NextActions: nextActions(classification.MissingEvidence),
		}
	case verdictBlocked:
		return output.Verdict{
			Status:      "blocked",
			Severity:    "high",
			Blocking:    true,
			Summary:     "Preliminary verdict blocked: blocking admission evidence is missing.",
			NextActions: nextActions(append(append([]Gap{}, classification.Blockers...), classification.MissingEvidence...)),
		}
	case verdictOutOfScope:
		return output.Verdict{
			Status:   "pass",
			Severity: "low",
			Blocking: false,
			Summary:  "Preliminary verdict out_of_scope: no product surface was detected.",
			NextActions: []string{
				"Document scope boundaries before adding product surfaces.",
			},
		}
	case verdictCorpusAdmissible:
		return output.Verdict{
			Status:   "pass",
			Severity: "info",
			Blocking: false,
			Summary:  "Preliminary verdict corpus_admissible: authoritative corpus can feed the canonical chain.",
			NextActions: []string{
				"Run nomos corpus scan, manifest, feed, and attest with outputs outside the source corpus root.",
			},
		}
	case verdictCorpusPartial:
		return output.Verdict{
			Status:      "warn",
			Severity:    "medium",
			Blocking:    false,
			Summary:     "Preliminary verdict corpus_partial: authoritative corpus is recognized but feed evidence is incomplete.",
			NextActions: nextActions(classification.MissingEvidence),
		}
	case verdictCorpusBlocked:
		return output.Verdict{
			Status:      "blocked",
			Severity:    "high",
			Blocking:    true,
			Summary:     "Preliminary verdict corpus_blocked: corpus cannot feed the canonical chain until blockers are resolved.",
			NextActions: nextActions(append(append([]Gap{}, classification.Blockers...), classification.MissingEvidence...)),
		}
	default:
		return output.Verdict{
			Status:   "warn",
			Severity: "medium",
			Blocking: false,
			Summary:  "Preliminary verdict unknown: diagnose could not classify the repository.",
		}
	}
}

func buildSummary(surfaces []SurfaceClassification, findings []output.Finding, evidence []output.Evidence, verdict string) output.Summary {
	blockingFindings := 0
	for _, finding := range findings {
		if finding.Blocking {
			blockingFindings++
		}
	}

	unitTotal := len(productSurfaces(surfaces))
	coverage := output.Coverage{
		UnitTotal: unitTotal,
	}
	switch {
	case unitTotal == 0:
		coverage.UnitNotApplicable = 1
		coverage.CoverageRatio = 1
	case verdict == verdictInScope:
		coverage.UnitCovered = unitTotal
		coverage.CoverageRatio = 1
	case verdict == verdictPartial:
		coverage.UnitPartial = unitTotal
		coverage.CoverageRatio = 0.5
	case verdict == verdictBlocked:
		coverage.UnitMissing = unitTotal
		coverage.CoverageRatio = 0
	case verdict == verdictCorpusAdmissible:
		coverage.UnitCovered = unitTotal
		coverage.CoverageRatio = 1
	case verdict == verdictCorpusPartial:
		coverage.UnitPartial = unitTotal
		coverage.CoverageRatio = 0.5
	case verdict == verdictCorpusBlocked:
		coverage.UnitMissing = unitTotal
		coverage.CoverageRatio = 0
	}

	return output.Summary{
		CheckCount:           3,
		FindingCount:         len(findings),
		BlockingFindingCount: blockingFindings,
		EvidenceCount:        len(evidence),
		Coverage:             coverage,
	}
}

func scanRepositoryEvidence(root string) (repositoryEvidence, error) {
	var repo repositoryEvidence
	cleanRoot, err := filepath.Abs(root)
	if err != nil {
		return repo, err
	}
	info, err := os.Stat(cleanRoot)
	if err != nil {
		return repo, err
	}
	if !info.IsDir() {
		return repo, errors.New("diagnose root must be a directory")
	}

	err = filepath.WalkDir(cleanRoot, func(filePath string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(cleanRoot, filePath)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		rel = filepath.ToSlash(rel)
		lower := strings.ToLower(rel)
		base := strings.ToLower(path.Base(rel))

		if entry.IsDir() {
			if shouldSkipDir(base) {
				return filepath.SkipDir
			}
			return nil
		}
		if !entry.Type().IsRegular() {
			return nil
		}

		switch {
		case isProjectManifest(lower, base) && repo.projectManifest == "":
			repo.projectManifest = rel
			repo.projectMode = readProjectMode(filepath.Join(cleanRoot, filepath.FromSlash(rel)))
		case isSourceManifest(base) && repo.sourceManifest == "":
			repo.sourceManifest = rel
		case isCanonicalMatrix(base) && repo.canonicalMatrix == "":
			repo.canonicalMatrix = rel
		}
		if isTestEvidence(lower, base) {
			repo.tests = append(repo.tests, rel)
		}
		if isCIEvidence(lower, base) {
			repo.ci = append(repo.ci, rel)
		}
		if isDecisionRecord(lower, base) {
			repo.decisionRecords = append(repo.decisionRecords, rel)
		}
		return nil
	})
	if err != nil {
		return repo, err
	}

	sort.Strings(repo.tests)
	sort.Strings(repo.ci)
	sort.Strings(repo.decisionRecords)
	return repo, nil
}

func isProjectManifest(lower string, base string) bool {
	return base == "nomos.project.yaml" ||
		base == "nomos.project.yml" ||
		lower == ".nomos/project.yaml" ||
		lower == ".nomos/project.yml"
}

func readProjectMode(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	text := strings.ToLower(string(data))
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "mode:") {
			continue
		}
		value := strings.TrimSpace(strings.TrimPrefix(line, "mode:"))
		value = strings.Trim(value, `"'`)
		return normalizeMode(value)
	}
	return ""
}

func normalizeMode(mode string) string {
	mode = strings.ToLower(strings.TrimSpace(mode))
	mode = strings.ReplaceAll(mode, "-", "_")
	switch mode {
	case "", "auto":
		return "auto"
	case "product":
		return "product"
	case "corpus", "canonical_corpus":
		return "canonical_corpus"
	default:
		return mode
	}
}

func reportMode(corpusMode bool) string {
	if corpusMode {
		return "corpus_admission"
	}
	return "admission"
}

func isSourceManifest(base string) bool {
	return strings.Contains(base, "source-manifest") &&
		(strings.HasSuffix(base, ".yaml") || strings.HasSuffix(base, ".yml") || strings.HasSuffix(base, ".json"))
}

func isCanonicalMatrix(base string) bool {
	return strings.Contains(base, "canonical-matrix") &&
		(strings.HasSuffix(base, ".yaml") || strings.HasSuffix(base, ".yml") || strings.HasSuffix(base, ".json"))
}

func isTestEvidence(lower string, base string) bool {
	return strings.HasPrefix(lower, "test/") ||
		strings.HasPrefix(lower, "tests/") ||
		strings.Contains(lower, "/test/") ||
		strings.Contains(lower, "/tests/") ||
		strings.HasSuffix(base, "_test.go") ||
		strings.HasSuffix(base, ".test.ts") ||
		strings.HasSuffix(base, ".test.tsx") ||
		strings.HasSuffix(base, ".spec.ts") ||
		strings.HasSuffix(base, ".spec.tsx") ||
		strings.HasPrefix(base, "test_") && strings.HasSuffix(base, ".py")
}

func isCIEvidence(lower string, base string) bool {
	return strings.HasPrefix(lower, ".github/workflows/") ||
		strings.HasPrefix(lower, ".circleci/") ||
		base == ".gitlab-ci.yml" ||
		base == ".gitlab-ci.yaml"
}

func isDecisionRecord(lower string, base string) bool {
	return strings.HasPrefix(lower, "adr/") ||
		strings.HasPrefix(lower, "docs/adr/") ||
		strings.HasPrefix(lower, "docs/decisions/") ||
		strings.Contains(lower, "/adr/") ||
		strings.Contains(lower, "/decisions/") ||
		strings.Contains(base, "decision") ||
		strings.Contains(base, "owner")
}

func shouldSkipDir(base string) bool {
	switch base {
	case ".git", ".hg", ".svn", ".tools", ".cache", "node_modules", "vendor", "dist", "build", ".next",
		"coverage", ".venv", "venv", "__pycache__", "target", ".terraform":
		return true
	default:
		return false
	}
}

func artifactEvidence(id string, typ string, description string, rel string) output.Evidence {
	return output.Evidence{
		ID:          id,
		Type:        typ,
		Description: description,
		URI:         rel,
		Target: &output.Target{
			Type: "project",
			Path: rel,
		},
		Producer: "nomos",
	}
}

func evidenceIDsByPrefix(evidence []output.Evidence, prefix string) []string {
	var ids []string
	for _, item := range evidence {
		if strings.HasPrefix(item.ID, prefix) {
			ids = append(ids, item.ID)
		}
	}
	sort.Strings(ids)
	return ids
}

func surfaceEvidenceIDs(surfaces []SurfaceClassification) []string {
	ids := make([]string, 0, len(surfaces))
	for _, surface := range surfaces {
		ids = append(ids, "EVIDENCE-SURFACE-"+idPart(surface.Name))
	}
	sort.Strings(ids)
	return ids
}

func hasProductSurface(surfaces []SurfaceClassification) bool {
	return len(productSurfaces(surfaces)) > 0
}

func productSurfaces(surfaces []SurfaceClassification) []SurfaceClassification {
	product := make([]SurfaceClassification, 0, len(surfaces))
	for _, surface := range surfaces {
		switch surface.Name {
		case "api", "ui", "worker", "data", "infra":
			product = append(product, surface)
		}
	}
	return product
}

func targetTypeForSurface(surface string) string {
	switch surface {
	case "api", "ui":
		return surface
	default:
		return "code"
	}
}

func sortGaps(gaps []Gap) {
	sort.Slice(gaps, func(i, j int) bool {
		return gaps[i].ID < gaps[j].ID
	})
}

func nextActions(gaps []Gap) []string {
	actions := make([]string, 0, len(gaps))
	for _, gap := range gaps {
		actions = append(actions, gap.Remediation)
	}
	sort.Strings(actions)
	return actions
}

func idPart(value string) string {
	value = strings.ToUpper(slug(value))
	value = strings.ReplaceAll(value, "-", "_")
	value = strings.ReplaceAll(value, "_", "-")
	if value == "" {
		return "UNKNOWN"
	}
	return value
}

func slug(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	lastDash := false
	for _, r := range value {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash && b.Len() > 0 {
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

func titleFromSlug(value string) string {
	parts := strings.Split(value, "-")
	for index, part := range parts {
		if part == "" {
			continue
		}
		parts[index] = strings.ToUpper(part[:1]) + part[1:]
	}
	return strings.Join(parts, " ")
}

func isCI() bool {
	return strings.TrimSpace(os.Getenv("CI")) != ""
}
