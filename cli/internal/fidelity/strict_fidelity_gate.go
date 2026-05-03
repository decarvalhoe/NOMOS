package fidelity

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
)

// StrictFinding is a single fidelity blocker.
type StrictFinding struct {
	Code     string `json:"code"`
	Category string `json:"category"`
	Severity string `json:"severity"`
	Blocking bool   `json:"blocking"`
	Message  string `json:"message"`
	NodeID   string `json:"node_id,omitempty"`
	Detail   string `json:"detail,omitempty"`
}

// StrictGateResult is the output of the strict fidelity gate.
type StrictGateResult struct {
	Pass          bool            `json:"pass"`
	GeneratedAt   string          `json:"generated_at"`
	TotalChecks   int             `json:"total_checks"`
	Passed        int             `json:"passed"`
	Failed        int             `json:"failed"`
	BlockingCount int             `json:"blocking_count"`
	Findings      []StrictFinding `json:"findings,omitempty"`
	GateHash      string          `json:"gate_hash"`
}

// StrictGateInput provides all artifacts for strict fidelity checking.
type StrictGateInput struct {
	CAST       *CAST
	TOC        *TOCArtifact
	Lexicon    *Lexicon
	SourceLen  int
	DocumentID string
}

// RunStrictFidelityGate runs all fidelity checks in strict mode.
// Any blocker causes the gate to fail.
func RunStrictFidelityGate(input StrictGateInput) StrictGateResult {
	var findings []StrictFinding

	findings = append(findings, strictCheckTableTyping(input)...)
	findings = append(findings, strictCheckCodeBlocks(input)...)
	findings = append(findings, strictCheckSourceSpans(input)...)
	findings = append(findings, strictCheckTOCCertified(input)...)
	findings = append(findings, strictCheckLexiconGoverned(input)...)
	findings = append(findings, strictCheckImageAltText(input)...)
	findings = append(findings, strictCheckEmptyNodes(input)...)

	totalChecks := 7
	blocking := 0
	failed := 0
	for _, f := range findings {
		if f.Blocking {
			blocking++
		}
		failed++
	}

	result := StrictGateResult{
		Pass:          blocking == 0,
		GeneratedAt:   time.Now().UTC().Format(time.RFC3339),
		TotalChecks:   totalChecks,
		Passed:        totalChecks - countCategories(findings),
		Failed:        countCategories(findings),
		BlockingCount: blocking,
		Findings:      findings,
	}
	result.GateHash = computeGateHash(result)
	return result
}

// VerifyStrictGate checks the gate hash.
func VerifyStrictGate(result StrictGateResult) bool {
	stored := result.GateHash
	result.GateHash = ""
	return stored == computeGateHash(result)
}

// WriteStrictGateJSON serializes the result.
func WriteStrictGateJSON(w io.Writer, result StrictGateResult) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}

// --- Individual checks ---

func strictCheckTableTyping(input StrictGateInput) []StrictFinding {
	if input.CAST == nil {
		return nil
	}
	var findings []StrictFinding
	for _, node := range input.CAST.Nodes {
		if node.Kind != KindTable {
			continue
		}
		if node.Props == nil || node.Props["col_count"] == "" {
			findings = append(findings, StrictFinding{
				Code: "UNTYPED_TABLE", Category: "tables",
				Severity: "high", Blocking: true,
				Message: fmt.Sprintf("table node %s has no col_count metadata", node.ID),
				NodeID:  node.ID,
			})
		}
	}
	return findings
}

func strictCheckCodeBlocks(input StrictGateInput) []StrictFinding {
	if input.CAST == nil {
		return nil
	}
	var findings []StrictFinding
	for _, node := range input.CAST.Nodes {
		if node.Kind != KindCodeBlock {
			continue
		}
		lang := ""
		if node.Props != nil {
			lang = node.Props["language"]
		}
		if lang == "" {
			findings = append(findings, StrictFinding{
				Code: "CODE_BLOCK_NO_LANG", Category: "code_blocks",
				Severity: "medium", Blocking: false,
				Message: fmt.Sprintf("code block %s has no language annotation", node.ID),
				NodeID:  node.ID,
			})
		}
	}
	return findings
}

func strictCheckSourceSpans(input StrictGateInput) []StrictFinding {
	if input.CAST == nil {
		return nil
	}
	var findings []StrictFinding
	for _, node := range input.CAST.Nodes {
		if node.Kind == KindDocument || node.Kind == KindThematicBreak {
			continue
		}
		if node.Span.StartLine == 0 && node.Span.EndLine == 0 {
			findings = append(findings, StrictFinding{
				Code: "MISSING_SPAN", Category: "spans",
				Severity: "high", Blocking: true,
				Message: fmt.Sprintf("node %s (%s) has no source span", node.ID, node.Kind),
				NodeID:  node.ID,
			})
		}
		if node.Span.StartLine > node.Span.EndLine && node.Span.EndLine > 0 {
			findings = append(findings, StrictFinding{
				Code: "INVERTED_SPAN", Category: "spans",
				Severity: "critical", Blocking: true,
				Message: fmt.Sprintf("node %s has inverted span: start=%d > end=%d", node.ID, node.Span.StartLine, node.Span.EndLine),
				NodeID:  node.ID,
			})
		}
	}
	return findings
}

func strictCheckTOCCertified(input StrictGateInput) []StrictFinding {
	if input.TOC == nil {
		return []StrictFinding{{
			Code: "TOC_MISSING", Category: "toc",
			Severity: "high", Blocking: true,
			Message: "no TOC artifact provided for strict gate",
		}}
	}
	var findings []StrictFinding
	if !input.TOC.Certified {
		findings = append(findings, StrictFinding{
			Code: "TOC_NOT_CERTIFIED", Category: "toc",
			Severity: "critical", Blocking: true,
			Message: "TOC artifact is not certified (tree hash verification failed)",
		})
	}
	if !VerifyTOCArtifact(*input.TOC) {
		findings = append(findings, StrictFinding{
			Code: "TOC_HASH_INVALID", Category: "toc",
			Severity: "critical", Blocking: true,
			Message: "TOC artifact hash does not match contents (tampered or corrupted)",
		})
	}
	return findings
}

func strictCheckLexiconGoverned(input StrictGateInput) []StrictFinding {
	if input.Lexicon == nil {
		return nil // lexicon is optional
	}
	var findings []StrictFinding
	terms := input.Lexicon.AllTerms()
	ungoverned := 0
	for _, term := range terms {
		if term.Status != TermCanonical && term.Status != TermSynonym {
			ungoverned++
		}
	}
	if ungoverned > 0 {
		findings = append(findings, StrictFinding{
			Code: "LEXICON_UNGOVERNED", Category: "lexicon",
			Severity: "medium", Blocking: false,
			Message:  fmt.Sprintf("%d/%d lexicon terms are not approved or candidate", ungoverned, len(terms)),
			Detail:   fmt.Sprintf("ungoverned terms should be reviewed before release"),
		})
	}
	return findings
}

func strictCheckImageAltText(input StrictGateInput) []StrictFinding {
	if input.CAST == nil {
		return nil
	}
	var findings []StrictFinding
	for _, node := range input.CAST.Nodes {
		if node.Kind != KindImage {
			continue
		}
		alt := ""
		if node.Props != nil {
			alt = node.Props["alt_text"]
		}
		if strings.TrimSpace(alt) == "" && strings.TrimSpace(node.Text) == "" {
			findings = append(findings, StrictFinding{
				Code: "IMAGE_NO_ALT", Category: "images",
				Severity: "medium", Blocking: false,
				Message: fmt.Sprintf("image node %s has no alt text", node.ID),
				NodeID:  node.ID,
			})
		}
	}
	return findings
}

func strictCheckEmptyNodes(input StrictGateInput) []StrictFinding {
	if input.CAST == nil {
		return nil
	}
	var findings []StrictFinding
	emptyCount := 0
	total := 0
	for _, node := range input.CAST.Nodes {
		if node.Kind == KindDocument || node.Kind == KindThematicBreak {
			continue
		}
		total++
		if strings.TrimSpace(node.Text) == "" && strings.TrimSpace(node.RawText) == "" {
			emptyCount++
		}
	}
	if total > 0 && float64(emptyCount)/float64(total) > 0.5 {
		findings = append(findings, StrictFinding{
			Code: "EXCESSIVE_EMPTY_NODES", Category: "content",
			Severity: "high", Blocking: true,
			Message: fmt.Sprintf("%d/%d nodes (%.0f%%) have no content", emptyCount, total, float64(emptyCount)/float64(total)*100),
		})
	}
	return findings
}

func countCategories(findings []StrictFinding) int {
	cats := map[string]bool{}
	for _, f := range findings {
		cats[f.Category] = true
	}
	return len(cats)
}

func computeGateHash(result StrictGateResult) string {
	h := sha256.New()
	h.Write([]byte(fmt.Sprintf("%v", result.Pass)))
	h.Write([]byte(fmt.Sprintf("%d", result.TotalChecks)))
	for _, f := range result.Findings {
		h.Write([]byte(f.Code + f.Category + f.NodeID))
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}
