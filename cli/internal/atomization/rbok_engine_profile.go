package atomization

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
)

const EngineProfileVersion = "import-contract-v1"

// --- Document projection ---

// DocumentProjection matches the rbok_documents import schema.
type DocumentProjection struct {
	ExternalID      string `json:"external_id"`
	Title           string `json:"title"`
	DocumentType    string `json:"document_type"`
	Jurisdiction    string `json:"jurisdiction"`
	PublicationDate string `json:"publication_date,omitempty"`
	EffectiveDate   string `json:"effective_date,omitempty"`
	SourcePath      string `json:"source_path"`
	SourceHash      string `json:"source_hash"`
	Version         string `json:"version"`
	Owner           string `json:"owner"`
	Domain          string `json:"domain"`
	Status          string `json:"status"`
}

// --- Node projection ---

// NodeProjection matches the rbok_nodes import schema.
type NodeProjection struct {
	ExternalID         string   `json:"external_id"`
	DocumentExternalID string   `json:"document_external_id"`
	ParentExternalID   string   `json:"parent_external_id,omitempty"`
	NodeType           string   `json:"node_type"`
	DisplayRef         string   `json:"display_ref"`
	CanonicalRef       string   `json:"canonical_ref"`
	Depth              int      `json:"depth"`
	HeadingLevel       int      `json:"heading_level,omitempty"`
	StructureOnly      bool     `json:"structure_only"`
	Priority           int      `json:"priority"`
	Status             string   `json:"status"`
	SourcePath         string   `json:"source_path"`
	SourceHash         string   `json:"source_hash"`
	Content            string   `json:"content,omitempty"`
	Aliases            []string `json:"aliases,omitempty"`
}

// --- Revision projection ---

// RevisionProjection matches the rbok_revisions import schema.
type RevisionProjection struct {
	ExternalID         string `json:"external_id"`
	DocumentExternalID string `json:"document_external_id"`
	RevisionNumber     int    `json:"revision_number"`
	CreatedAt          string `json:"created_at"`
	CreatedBy          string `json:"created_by"`
	ChangeSummary      string `json:"change_summary,omitempty"`
	SourceHash         string `json:"source_hash"`
	NodeCount          int    `json:"node_count"`
}

// --- Engine profile input/output ---

// EngineProfileInput configures the engine projection.
type EngineProfileInput struct {
	SourcePath   string
	Domain       string
	Owner        string
	Version      string
	Jurisdiction string
	DocumentType string
	Status       string
}

// EngineProfileOutput is the full import payload.
type EngineProfileOutput struct {
	ContractVersion string               `json:"contract_version"`
	GeneratedAt     string               `json:"generated_at"`
	Document        DocumentProjection   `json:"document"`
	Nodes           []NodeProjection     `json:"nodes"`
	Revision        RevisionProjection   `json:"revision"`
	GovernanceGate  GovernanceGateResult `json:"governance_gate"`
}

// GovernanceGateResult captures pre-publish checks.
type GovernanceGateResult struct {
	PublishAllowed bool              `json:"publish_allowed"`
	Findings       []GovernanceFinding `json:"findings,omitempty"`
}

// GovernanceFinding is a single governance issue.
type GovernanceFinding struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Blocks  bool   `json:"blocks"`
}

// ProjectAST transforms a parsed Markdown AST into the RBOK Engine
// import format (documents, nodes, revision).
func ProjectAST(ast AST, input EngineProfileInput) EngineProfileOutput {
	now := time.Now().UTC()
	docID := makeDocExternalID(input.SourcePath)

	doc := DocumentProjection{
		ExternalID:   docID,
		Title:        extractTitle(ast),
		DocumentType: defaultString(input.DocumentType, "internal"),
		Jurisdiction: defaultString(input.Jurisdiction, "FR"),
		SourcePath:   input.SourcePath,
		SourceHash:   ast.SourceHash,
		Version:      defaultString(input.Version, "0.1.0"),
		Owner:        defaultString(input.Owner, "unknown"),
		Domain:       defaultString(input.Domain, "unknown"),
		Status:       defaultString(input.Status, "active"),
	}

	nodes := projectNodes(ast, docID, input)

	revision := RevisionProjection{
		ExternalID:         fmt.Sprintf("%s-rev-1", docID),
		DocumentExternalID: docID,
		RevisionNumber:     1,
		CreatedAt:          now.Format(time.RFC3339),
		CreatedBy:          "nomos-cli",
		ChangeSummary:      "Initial import",
		SourceHash:         ast.SourceHash,
		NodeCount:          len(nodes),
	}

	gate := checkGovernanceGate(doc, nodes)

	return EngineProfileOutput{
		ContractVersion: EngineProfileVersion,
		GeneratedAt:     now.Format(time.RFC3339),
		Document:        doc,
		Nodes:           nodes,
		Revision:        revision,
		GovernanceGate:  gate,
	}
}

// WriteEngineJSON serializes the output as indented JSON.
func WriteEngineJSON(w io.Writer, output EngineProfileOutput) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(output)
}

func projectNodes(ast AST, docID string, input EngineProfileInput) []NodeProjection {
	blockMap := make(map[string]*Block, len(ast.Blocks))
	for i := range ast.Blocks {
		blockMap[ast.Blocks[i].ID] = &ast.Blocks[i]
	}

	depthMap := computeDepths(ast)

	nodes := make([]NodeProjection, 0, len(ast.Blocks))
	for _, block := range ast.Blocks {
		if block.Type == BlockBlankLine || block.Type == BlockDocument {
			continue
		}

		nodeType := mapBlockType(block.Type)
		structureOnly := block.Type == BlockHeading && len(block.Children) > 0
		content := ""
		if !structureOnly {
			content = block.Text
		}

		depth := depthMap[block.ID]

		nodes = append(nodes, NodeProjection{
			ExternalID:         block.ID,
			DocumentExternalID: docID,
			ParentExternalID:   block.ParentID,
			NodeType:           nodeType,
			DisplayRef:         makeDisplayRef(block),
			CanonicalRef:       makeCanonicalRef(docID, block),
			Depth:              depth,
			HeadingLevel:       block.Level,
			StructureOnly:      structureOnly,
			Priority:           computePriority(block),
			Status:             defaultString(input.Status, "active"),
			SourcePath:         input.SourcePath,
			SourceHash:         block.Hash,
			Content:            content,
		})
	}

	return nodes
}

func computeDepths(ast AST) map[string]int {
	depths := map[string]int{}
	for _, b := range ast.Blocks {
		if b.ParentID == "" || b.ParentID == ast.Root {
			depths[b.ID] = 0
		}
	}
	// BFS
	changed := true
	for changed {
		changed = false
		for _, b := range ast.Blocks {
			if _, ok := depths[b.ID]; ok {
				continue
			}
			if parentDepth, ok := depths[b.ParentID]; ok {
				depths[b.ID] = parentDepth + 1
				changed = true
			}
		}
	}
	return depths
}

func mapBlockType(bt BlockType) string {
	switch bt {
	case BlockHeading:
		return "section"
	case BlockParagraph:
		return "article"
	case BlockList, BlockListItem:
		return "article"
	case BlockTable, BlockTableRow:
		return "annex"
	case BlockCodeBlock:
		return "annex"
	case BlockMetadata:
		return "definition"
	default:
		return "article"
	}
}

func makeDisplayRef(block Block) string {
	if block.Type == BlockHeading {
		return fmt.Sprintf("Section %d: %s", block.Level, truncate(block.Text, 60))
	}
	return fmt.Sprintf("L%d-%d", block.Span.StartLine, block.Span.EndLine)
}

func makeCanonicalRef(docID string, block Block) string {
	return fmt.Sprintf("%s#%s", docID, block.ID)
}

func computePriority(block Block) int {
	switch block.Type {
	case BlockHeading:
		return 100 - block.Level*10
	case BlockParagraph:
		return 50
	case BlockList, BlockListItem:
		return 40
	case BlockTable:
		return 30
	default:
		return 20
	}
}

func extractTitle(ast AST) string {
	for _, b := range ast.Blocks {
		if b.Type == BlockHeading && b.Level == 1 {
			return b.Text
		}
	}
	return "Untitled"
}

func makeDocExternalID(sourcePath string) string {
	h := sha256.Sum256([]byte(sourcePath))
	slug := strings.ReplaceAll(sourcePath, "/", "-")
	slug = strings.ReplaceAll(slug, ".", "-")
	return fmt.Sprintf("doc-%s-%s", truncate(slug, 40), hex.EncodeToString(h[:4]))
}

func checkGovernanceGate(doc DocumentProjection, nodes []NodeProjection) GovernanceGateResult {
	var findings []GovernanceFinding

	if doc.Owner == "unknown" || doc.Owner == "" {
		findings = append(findings, GovernanceFinding{
			Code: "corpus_partial", Message: "document missing owner", Blocks: true,
		})
	}
	if doc.Version == "" {
		findings = append(findings, GovernanceFinding{
			Code: "corpus_partial", Message: "document missing version", Blocks: true,
		})
	}
	if doc.Domain == "unknown" || doc.Domain == "" {
		findings = append(findings, GovernanceFinding{
			Code: "corpus_partial", Message: "document missing domain", Blocks: true,
		})
	}
	if doc.SourceHash == "" {
		findings = append(findings, GovernanceFinding{
			Code: "corpus_unapproved", Message: "document missing source_hash", Blocks: true,
		})
	}

	// Check for duplicate canonical_refs.
	refs := map[string]int{}
	for _, n := range nodes {
		refs[n.CanonicalRef]++
	}
	for ref, count := range refs {
		if count > 1 {
			findings = append(findings, GovernanceFinding{
				Code:    "corpus_unresolved_ref",
				Message: fmt.Sprintf("duplicate canonical_ref %q (%d occurrences)", ref, count),
				Blocks:  true,
			})
		}
	}

	publishAllowed := true
	for _, f := range findings {
		if f.Blocks {
			publishAllowed = false
			break
		}
	}

	return GovernanceGateResult{
		PublishAllowed: publishAllowed,
		Findings:       findings,
	}
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}
