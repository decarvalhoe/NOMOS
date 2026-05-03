package corpus

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/RBOKproject/Nomos/cli/internal/atomization"
)

// OutputStructureFidelityReport is the profile section name for the portable
// source AST -> NOMOS node coverage gate.
const StructureFidelityFormat = "nomos.structure-fidelity-report.v1"

// StructureFidelityReport proves whether active source AST blocks are
// represented by generated NOMOS nodes with source spans and hashes.
type StructureFidelityReport struct {
	Format                  string                     `json:"format"`
	Profile                 string                     `json:"profile"`
	GeneratedAt             string                     `json:"generated_at"`
	CheckedSourceCount      int                        `json:"checked_source_count"`
	SourceBlockCount        int                        `json:"source_block_count"`
	CoveredSourceBlockCount int                        `json:"covered_source_block_count"`
	MissingSourceBlockCount int                        `json:"missing_source_block_count"`
	UnsupportedBlockCount   int                        `json:"unsupported_block_count"`
	NodeCount               int                        `json:"node_count"`
	NodesMissingSourceSpan  int                        `json:"nodes_missing_source_span"`
	NodesMissingSourceHash  int                        `json:"nodes_missing_source_hash"`
	Blocking                int                        `json:"blocking"`
	Warnings                int                        `json:"warnings"`
	Verdict                 string                     `json:"verdict"`
	Findings                []StructureFidelityFinding `json:"findings,omitempty"`
}

// StructureFidelityFinding is one gap in source-to-structure fidelity.
type StructureFidelityFinding struct {
	Code       string `json:"code"`
	Severity   string `json:"severity"`
	SourcePath string `json:"source_path,omitempty"`
	NodeID     string `json:"node_id,omitempty"`
	BlockID    string `json:"block_id,omitempty"`
	BlockType  string `json:"block_type,omitempty"`
	Locator    string `json:"locator,omitempty"`
	Message    string `json:"message"`
}

// BuildStructureFidelityReport evaluates supported source adapters for coverage
// by the generated node assembly. The algorithm is profile-neutral: it reasons
// about source blocks, source spans, hashes, and explicit unsupported findings.
func BuildStructureFidelityReport(profileName, generatedAt, corpusRoot string, classifications []RBOKSourceClassification, assembly MultiFeedAssembly) StructureFidelityReport {
	report := StructureFidelityReport{
		Format:      StructureFidelityFormat,
		Profile:     profileName,
		GeneratedAt: generatedAt,
		Verdict:     "pass",
	}

	nodesByPath := nodesBySourcePath(assembly)
	checkedPaths := structureFidelityCheckedPaths(classifications, nodesByPath)
	for _, nodes := range nodesByPath {
		report.NodeCount += len(nodes)
		for _, node := range nodes {
			if !checkedPaths[node.SourcePath] {
				continue
			}
			if node.SourceSpan == nil {
				report.NodesMissingSourceSpan++
				report.addFinding(StructureFidelityFinding{
					Code:       "node.missing_source_span",
					Severity:   "blocking",
					SourcePath: node.SourcePath,
					NodeID:     node.NodeID,
					Message:    "NOMOS node has no source_span.",
				})
			}
			if strings.TrimSpace(node.SourceHash) == "" {
				report.NodesMissingSourceHash++
				report.addFinding(StructureFidelityFinding{
					Code:       "node.missing_source_hash",
					Severity:   "blocking",
					SourcePath: node.SourcePath,
					NodeID:     node.NodeID,
					Message:    "NOMOS node has no source_hash.",
				})
			}
		}
	}

	ordered := append([]RBOKSourceClassification(nil), classifications...)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].Path < ordered[j].Path })
	for _, c := range ordered {
		sourceNodes := nodesByPath[c.Path]
		if !shouldCheckStructureFidelity(c, sourceNodes) {
			continue
		}
		absPath := filepath.Join(corpusRoot, filepath.FromSlash(c.Path))
		data, err := os.ReadFile(absPath)
		if err != nil {
			report.addFinding(StructureFidelityFinding{
				Code:       "source.read_failed",
				Severity:   "blocking",
				SourcePath: c.Path,
				Message:    fmt.Sprintf("read source for fidelity: %v", err),
			})
			continue
		}
		report.CheckedSourceCount++
		if isStructuredFidelitySource(c) {
			checkStructuredFidelitySource(&report, c.Path, sourceNodes)
			_ = data
			continue
		}
		ast := atomization.ParseMarkdown(string(data))
		for _, block := range ast.Blocks {
			if isIgnorableFidelityBlock(block) {
				continue
			}
			report.SourceBlockCount++
			if block.Type == atomization.BlockRawHTML {
				report.UnsupportedBlockCount++
			}
			covered := sourceBlockCovered(c.Path, block, sourceNodes)
			if !covered && block.Type == atomization.BlockMetadata {
				covered = metadataBlockCovered(sourceNodes)
			}
			if covered {
				report.CoveredSourceBlockCount++
				if block.Type == atomization.BlockRawHTML && !unsupportedBlockReviewed(block, sourceNodes) {
					report.addFinding(StructureFidelityFinding{
						Code:       "unsupported_block.needs_review",
						Severity:   "blocking",
						SourcePath: c.Path,
						BlockID:    block.ID,
						BlockType:  string(block.Type),
						Locator:    blockLocator(c.Path, block),
						Message:    "Unsupported raw HTML block is preserved but has no needs_review node.",
					})
				}
				continue
			}
			report.MissingSourceBlockCount++
			report.addFinding(StructureFidelityFinding{
				Code:       "source_block.uncovered",
				Severity:   "blocking",
				SourcePath: c.Path,
				BlockID:    block.ID,
				BlockType:  string(block.Type),
				Locator:    blockLocator(c.Path, block),
				Message:    "Source AST block has no corresponding NOMOS node or explicit metadata projection.",
			})
		}
	}

	if report.Blocking > 0 {
		report.Verdict = "fail"
	}
	return report
}

func structureFidelityCheckedPaths(classifications []RBOKSourceClassification, nodesByPath map[string][]LawbookNode) map[string]bool {
	out := map[string]bool{}
	for _, c := range classifications {
		if shouldCheckStructureFidelity(c, nodesByPath[c.Path]) {
			out[c.Path] = true
		}
	}
	return out
}

func (r *StructureFidelityReport) addFinding(f StructureFidelityFinding) {
	if strings.TrimSpace(f.Severity) == "" {
		f.Severity = "warning"
	}
	if f.Severity == "blocking" {
		r.Blocking++
	} else {
		r.Warnings++
	}
	r.Findings = append(r.Findings, f)
}

func nodesBySourcePath(assembly MultiFeedAssembly) map[string][]LawbookNode {
	out := map[string][]LawbookNode{}
	for _, feed := range assembly.Feeds {
		for _, node := range feed.Nodes {
			if strings.TrimSpace(node.SourcePath) == "" {
				continue
			}
			out[node.SourcePath] = append(out[node.SourcePath], node)
		}
	}
	return out
}

func shouldCheckStructureFidelity(c RBOKSourceClassification, sourceNodes []LawbookNode) bool {
	if !shouldAtomizeProfileSource(c) {
		return false
	}
	switch strings.ToLower(filepath.Ext(c.Path)) {
	case ".md", ".mdx":
		return true
	case ".yaml", ".yml":
		return len(sourceNodes) > 0
	case ".json":
		return len(sourceNodes) > 0
	default:
		return false
	}
}

func isStructuredFidelitySource(c RBOKSourceClassification) bool {
	switch strings.ToLower(filepath.Ext(c.Path)) {
	case ".yaml", ".yml", ".json":
		return true
	default:
		return false
	}
}

func checkStructuredFidelitySource(report *StructureFidelityReport, sourcePath string, nodes []LawbookNode) {
	report.SourceBlockCount += len(nodes)
	for _, node := range nodes {
		if node.SourceSpan == nil || strings.TrimSpace(node.SourceHash) == "" {
			report.MissingSourceBlockCount++
			report.addFinding(StructureFidelityFinding{
				Code:       "structured_node.uncovered",
				Severity:   "blocking",
				SourcePath: sourcePath,
				NodeID:     node.NodeID,
				Message:    "Structured source node is not backed by a source_span and source_hash.",
			})
			continue
		}
		report.CoveredSourceBlockCount++
	}
}

func isIgnorableFidelityBlock(block atomization.Block) bool {
	switch block.Type {
	case atomization.BlockDocument, atomization.BlockBlankLine, atomization.BlockThematicBreak:
		return true
	default:
		return false
	}
}

func sourceBlockCovered(sourcePath string, block atomization.Block, nodes []LawbookNode) bool {
	for _, node := range nodes {
		if node.SourceSpan == nil {
			continue
		}
		if node.SourcePath != "" && node.SourcePath != sourcePath {
			continue
		}
		if spanCoversBlock(node.SourceSpan.StartLine, node.SourceSpan.EndLine, block) {
			return true
		}
	}
	return false
}

func metadataBlockCovered(nodes []LawbookNode) bool {
	for _, node := range nodes {
		if node.NodeType == NodeDocument && len(node.Metadata) > 0 {
			return true
		}
	}
	return false
}

func unsupportedBlockReviewed(block atomization.Block, nodes []LawbookNode) bool {
	for _, node := range nodes {
		if node.NodeType != NodeRawHTML || node.SourceSpan == nil {
			continue
		}
		if !spanCoversBlock(node.SourceSpan.StartLine, node.SourceSpan.EndLine, block) {
			continue
		}
		if fmt.Sprint(node.Metadata["review_state"]) == "needs_review" {
			return true
		}
	}
	return false
}

func spanCoversBlock(startLine *int, endLine *int, block atomization.Block) bool {
	if startLine == nil || endLine == nil {
		return false
	}
	return *startLine <= block.Span.StartLine && *endLine >= block.Span.EndLine
}

func blockLocator(sourcePath string, block atomization.Block) string {
	return fmt.Sprintf("%s#L%d-L%d", sourcePath, block.Span.StartLine, block.Span.EndLine)
}

// WriteStructureFidelityReportJSON writes the report using stable indentation.
func WriteStructureFidelityReportJSON(report StructureFidelityReport) ([]byte, error) {
	return json.MarshalIndent(report, "", "  ")
}
