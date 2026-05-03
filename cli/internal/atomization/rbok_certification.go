package atomization

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"
)

// CertificationLevel indicates the atomization quality tier.
type CertificationLevel string

const (
	CertCertified   CertificationLevel = "certified"
	CertProvisional CertificationLevel = "provisional"
	CertFailed      CertificationLevel = "failed"
)

// CertificationFinding is a single quality issue.
type CertificationFinding struct {
	Code     string `json:"code"`
	Severity string `json:"severity"`
	Blocking bool   `json:"blocking"`
	Message  string `json:"message"`
}

// CertificationCertificate is the output of the certification process.
type CertificationCertificate struct {
	Level           CertificationLevel      `json:"level"`
	GeneratedAt     string                  `json:"generated_at"`
	DocumentRef     string                  `json:"document_ref"`
	SourceFile      string                  `json:"source_file"`
	SourceHash      string                  `json:"source_hash"`
	AtomCount       int                     `json:"atom_count"`
	CoveragePercent float64                 `json:"coverage_percent"`
	IsLossless      bool                    `json:"is_lossless"`
	StableIDs       bool                    `json:"stable_ids"`
	SourceSpans     bool                    `json:"source_spans"`
	HashChain       bool                    `json:"hash_chain"`
	TotalFindings   int                     `json:"total_findings"`
	BlockingCount   int                     `json:"blocking_count"`
	Findings        []CertificationFinding  `json:"findings,omitempty"`
	ChainHash       string                  `json:"chain_hash"`
}

// CertificationThresholds configures the quality gates.
type CertificationThresholds struct {
	MinCoveragePercent float64 // minimum source coverage for certified (default: 95)
	MaxLossRatio       float64 // maximum allowed loss ratio (default: 0.05)
	RequireLossless    bool    // require zero loss for certified
}

// DefaultThresholds returns standard certification thresholds.
func DefaultThresholds() CertificationThresholds {
	return CertificationThresholds{
		MinCoveragePercent: 95.0,
		MaxLossRatio:       0.05,
		RequireLossless:    false,
	}
}

// Certify evaluates an AtomSet and its source AST for atomization quality.
func Certify(atomSet AtomSet, ast AST, thresholds CertificationThresholds) CertificationCertificate {
	now := time.Now().UTC()
	var findings []CertificationFinding

	// 1. Coverage
	coveragePct := computeCoverage(ast)
	isLossless := ast.LossReport.IsLossless

	if coveragePct < thresholds.MinCoveragePercent {
		findings = append(findings, CertificationFinding{
			Code: "LOW_COVERAGE", Severity: "high", Blocking: true,
			Message: fmt.Sprintf("coverage %.1f%% is below threshold %.1f%%", coveragePct, thresholds.MinCoveragePercent),
		})
	}

	if ast.LossReport.LossRatio > thresholds.MaxLossRatio {
		findings = append(findings, CertificationFinding{
			Code: "HIGH_LOSS", Severity: "high", Blocking: true,
			Message: fmt.Sprintf("loss ratio %.3f exceeds maximum %.3f (%d lost bytes)", ast.LossReport.LossRatio, thresholds.MaxLossRatio, ast.LossReport.LostBytes),
		})
	}

	if thresholds.RequireLossless && !isLossless {
		findings = append(findings, CertificationFinding{
			Code: "NOT_LOSSLESS", Severity: "critical", Blocking: true,
			Message: fmt.Sprintf("atomization is not lossless: %d bytes lost in %d spans", ast.LossReport.LostBytes, len(ast.LossReport.LostSpans)),
		})
	}

	// 2. Stable IDs
	stableIDs := checkStableIDs(atomSet, &findings)

	// 3. Source spans
	sourceSpans := checkSourceSpans(atomSet, &findings)

	// 4. Hash chain
	chainHash, hashChain := checkHashChain(atomSet, &findings)

	// 5. Empty atoms
	checkEmptyAtoms(atomSet, &findings)

	// 6. Atom count
	if atomSet.AtomCount == 0 {
		findings = append(findings, CertificationFinding{
			Code: "NO_ATOMS", Severity: "critical", Blocking: true,
			Message: "atom set contains zero atoms",
		})
	}

	// Compute level
	blocking := 0
	for _, f := range findings {
		if f.Blocking {
			blocking++
		}
	}

	level := CertCertified
	if blocking > 0 {
		level = CertFailed
	} else if len(findings) > 0 {
		level = CertProvisional
	}

	return CertificationCertificate{
		Level:           level,
		GeneratedAt:     now.Format(time.RFC3339),
		DocumentRef:     atomSet.DocumentRef,
		SourceFile:      atomSet.SourceFile,
		SourceHash:      atomSet.SourceHash,
		AtomCount:       atomSet.AtomCount,
		CoveragePercent: coveragePct,
		IsLossless:      isLossless,
		StableIDs:       stableIDs,
		SourceSpans:     sourceSpans,
		HashChain:       hashChain,
		TotalFindings:   len(findings),
		BlockingCount:   blocking,
		Findings:        findings,
		ChainHash:       chainHash,
	}
}

// WriteCertificateJSON serializes the certificate.
func WriteCertificateJSON(w io.Writer, cert CertificationCertificate) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(cert)
}

func computeCoverage(ast AST) float64 {
	if ast.LossReport.TotalSourceBytes == 0 {
		if len(ast.Blocks) > 0 {
			return 100.0
		}
		return 0.0
	}
	return float64(ast.LossReport.CoveredBytes) / float64(ast.LossReport.TotalSourceBytes) * 100.0
}

func checkStableIDs(atomSet AtomSet, findings *[]CertificationFinding) bool {
	if len(atomSet.Atoms) == 0 {
		return true
	}

	seen := map[string]int{}
	allStable := true

	for _, atom := range atomSet.Atoms {
		if atom.ID == "" {
			allStable = false
			*findings = append(*findings, CertificationFinding{
				Code: "EMPTY_ATOM_ID", Severity: "critical", Blocking: true,
				Message: fmt.Sprintf("atom at %s has empty ID", atom.SourceSpan.String()),
			})
			continue
		}
		seen[atom.ID]++
	}

	for id, count := range seen {
		if count > 1 {
			allStable = false
			*findings = append(*findings, CertificationFinding{
				Code: "DUPLICATE_ATOM_ID", Severity: "critical", Blocking: true,
				Message: fmt.Sprintf("atom ID %q appears %d times", id, count),
			})
		}
	}

	return allStable
}

func checkSourceSpans(atomSet AtomSet, findings *[]CertificationFinding) bool {
	allValid := true
	for _, atom := range atomSet.Atoms {
		if atom.SourceSpan.StartLine == 0 && atom.SourceSpan.EndLine == 0 {
			allValid = false
			*findings = append(*findings, CertificationFinding{
				Code: "MISSING_SOURCE_SPAN", Severity: "medium", Blocking: false,
				Message: fmt.Sprintf("atom %q has no source span", atom.ID),
			})
		}
		if atom.SourceSpan.StartLine > atom.SourceSpan.EndLine && atom.SourceSpan.EndLine > 0 {
			allValid = false
			*findings = append(*findings, CertificationFinding{
				Code: "INVALID_SOURCE_SPAN", Severity: "high", Blocking: true,
				Message: fmt.Sprintf("atom %q has inverted span: start=%d > end=%d", atom.ID, atom.SourceSpan.StartLine, atom.SourceSpan.EndLine),
			})
		}
	}
	return allValid
}

func checkHashChain(atomSet AtomSet, findings *[]CertificationFinding) (string, bool) {
	if len(atomSet.Atoms) == 0 {
		return "", true
	}

	valid := true
	h := sha256.New()

	// Chain: source_hash → atom[0].content_hash → atom[1].content_hash → ...
	h.Write([]byte(atomSet.SourceHash))

	sorted := make([]Atom, len(atomSet.Atoms))
	copy(sorted, atomSet.Atoms)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].ID < sorted[j].ID })

	for _, atom := range sorted {
		if atom.ContentHash == "" {
			valid = false
			*findings = append(*findings, CertificationFinding{
				Code: "MISSING_CONTENT_HASH", Severity: "high", Blocking: true,
				Message: fmt.Sprintf("atom %q has no content hash", atom.ID),
			})
			continue
		}
		h.Write([]byte(atom.ContentHash))
	}

	chainHash := "sha256:" + hex.EncodeToString(h.Sum(nil))
	return chainHash, valid
}

func checkEmptyAtoms(atomSet AtomSet, findings *[]CertificationFinding) {
	emptyCount := 0
	for _, atom := range atomSet.Atoms {
		if strings.TrimSpace(atom.Text) == "" {
			emptyCount++
		}
	}
	if emptyCount > 0 && len(atomSet.Atoms) > 0 {
		ratio := float64(emptyCount) / float64(len(atomSet.Atoms))
		if ratio > 0.5 {
			*findings = append(*findings, CertificationFinding{
				Code: "EXCESSIVE_EMPTY_ATOMS", Severity: "high", Blocking: true,
				Message: fmt.Sprintf("%d/%d atoms (%.0f%%) have empty text", emptyCount, len(atomSet.Atoms), ratio*100),
			})
		} else if emptyCount > 0 {
			*findings = append(*findings, CertificationFinding{
				Code: "EMPTY_ATOMS", Severity: "low", Blocking: false,
				Message: fmt.Sprintf("%d/%d atoms have empty text", emptyCount, len(atomSet.Atoms)),
			})
		}
	}
}
