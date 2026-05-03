package corpus

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/RBOKproject/Nomos/cli/internal/fidelity"
	"gopkg.in/yaml.v3"
)

// RunStrictFidelityGateForPack runs the fidelity gate against the assembled
// feed and writes the result to the artifact output directory.
// Returns the gate result for embedding in the pack result.
func RunStrictFidelityGateForPack(assembly MultiFeedAssembly, outDir string, tocPath string, lexiconPath string) (fidelity.StrictGateResult, error) {
	// Build CAST from the assembled feed nodes.
	cast := buildCASTFromFeedNodes(assembly)

	// Load TOC artifact if present.
	var toc *fidelity.TOCArtifact
	if tocPath != "" {
		loaded, err := loadTOCArtifact(tocPath)
		if err == nil {
			toc = loaded
		}
	}

	// Load lexicon if present.
	var lex *fidelity.Lexicon
	if lexiconPath != "" {
		loaded, err := loadLexiconForGate(lexiconPath)
		if err == nil {
			lex = loaded
		}
	}

	input := fidelity.StrictGateInput{
		CAST:       cast,
		TOC:        toc,
		Lexicon:    lex,
		SourceLen:  assembly.TotalNodes,
		DocumentID: firstDocumentID(assembly),
	}

	result := fidelity.RunStrictFidelityGate(input)

	// Write gate result to output directory.
	gatePath := filepath.Join(outDir, "rbok-strict-fidelity-gate.json")
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return result, err
	}
	if err := os.WriteFile(gatePath, data, 0o644); err != nil {
		return result, err
	}

	return result, nil
}

func buildCASTFromFeedNodes(assembly MultiFeedAssembly) *fidelity.CAST {
	var nodes []fidelity.CNode
	for _, feed := range assembly.Feeds {
		for _, n := range feed.Nodes {
			kind := fidelity.NodeKind(n.NodeType)
			node := fidelity.CNode{
				ID:       n.NodeID,
				Kind:     kind,
				Text:     n.Text,
				RawText:  n.Text,
				ParentID: n.ParentID,
				Hash:     n.SourceHash,
				Span: fidelity.Span{
					StartLine: n.Span.StartLine,
					EndLine:   n.Span.EndLine,
				},
			}
			if n.Metadata != nil {
				node.Props = make(map[string]string)
				for k, v := range n.Metadata {
					if s, ok := v.(string); ok {
						node.Props[k] = s
					}
				}
			}
			nodes = append(nodes, node)
		}
	}

	cast := &fidelity.CAST{
		Root:  "feed-root",
		Nodes: nodes,
	}

	// Compute coverage counts.
	for _, n := range nodes {
		switch n.Kind {
		case fidelity.KindHeading:
			cast.Coverage.Headings++
		case fidelity.KindParagraph:
			cast.Coverage.Paragraphs++
		case fidelity.KindTable:
			cast.Coverage.Tables++
		case fidelity.KindCodeBlock:
			cast.Coverage.CodeBlocks++
		case fidelity.KindLink:
			cast.Coverage.Links++
		case fidelity.KindImage:
			cast.Coverage.Images++
		case fidelity.KindList:
			cast.Coverage.Lists++
		case fidelity.KindBlockquote:
			cast.Coverage.Blockquotes++
		}
	}

	return cast
}

func loadTOCArtifact(path string) (*fidelity.TOCArtifact, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var toc fidelity.TOCArtifact
	if err := json.Unmarshal(data, &toc); err != nil {
		return nil, err
	}
	return &toc, nil
}

func loadLexiconForGate(path string) (*fidelity.Lexicon, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	type lexEntry struct {
		Term       string `yaml:"term" json:"term"`
		Definition string `yaml:"definition" json:"definition"`
		Status     string `yaml:"status" json:"status"`
	}
	type lexDoc struct {
		Terms []lexEntry `yaml:"terms" json:"terms"`
	}
	var doc lexDoc
	// Try JSON first, then YAML.
	if err := json.Unmarshal(data, &doc); err != nil {
		if yerr := yamlUnmarshal(data, &doc); yerr != nil {
			return nil, yerr
		}
	}
	lex := fidelity.NewLexicon()
	for _, entry := range doc.Terms {
		status := fidelity.TermCanonical
		if entry.Status == "synonym" {
			status = fidelity.TermSynonym
		}
		_ = lex.Add(fidelity.Term{
			Canonical:  entry.Term,
			Status:     status,
			Definition: entry.Definition,
		})
	}
	return lex, nil
}

func yamlUnmarshal(data []byte, v interface{}) error {
	return yaml.Unmarshal(data, v)
}

func firstDocumentID(assembly MultiFeedAssembly) string {
	if len(assembly.Feeds) > 0 {
		return assembly.Feeds[0].DocumentID
	}
	return "unknown"
}
