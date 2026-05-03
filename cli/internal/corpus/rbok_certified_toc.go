package corpus

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/RBOKproject/Nomos/cli/internal/fidelity"
)

// BuildCertifiedTOCFromFeeds constructs a certified TOC from lawbook feeds.
func BuildCertifiedTOCFromFeeds(feeds []LawbookFeed, documentRef string) fidelity.CertifiedTOC {
	var headings []fidelity.TOCHeading

	for _, feed := range feeds {
		for _, node := range feed.Nodes {
			if !isHeadingNodeType(node.NodeType) {
				continue
			}
			headings = append(headings, fidelity.TOCHeading{
				Title:  node.Title,
				Level:  nodeTypeToHeadingLevel(node.NodeType),
				Line:   0, // line not available from feed nodes
				NodeID: node.NodeID,
			})
		}
	}

	return fidelity.BuildCertifiedTOC(fidelity.TOCInput{
		DocumentRef: documentRef,
		Headings:    headings,
	})
}

// WriteCertifiedTOCArtifact writes the TOC artifact to the artifacts directory.
func WriteCertifiedTOCArtifact(toc fidelity.CertifiedTOC, outDir string) error {
	path := filepath.Join(outDir, "rbok-certified-toc.json")
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create rbok-certified-toc.json: %w", err)
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(toc)
}

func isHeadingNodeType(t LawbookNodeType) bool {
	switch t {
	case NodeDocument, NodeChapter, NodeSection, NodeSubsection, NodeArticle:
		return true
	default:
		return false
	}
}

func nodeTypeToHeadingLevel(t LawbookNodeType) int {
	switch t {
	case NodeDocument:
		return 1
	case NodeChapter:
		return 2
	case NodeSection:
		return 3
	case NodeSubsection:
		return 4
	case NodeArticle:
		return 5
	default:
		return 6
	}
}

// buildCertifiedTOCFromFeeds constructs a certified TOC from assembled multi-feed data.
func buildCertifiedTOCFromFeeds(assembly MultiFeedAssembly) fidelity.CertifiedTOC {
	var entries []fidelity.TOCEntry
	for _, feed := range assembly.Feeds {
		for _, node := range feed.Nodes {
			if node.NodeType == "document" || node.NodeType == "chapter" || node.NodeType == "section" {
				entries = append(entries, fidelity.TOCEntry{
					NodeID: node.NodeID,
					Title:  node.Title,
					Depth:  node.Depth,
				})
			}
		}
	}
	h := sha256.Sum256([]byte(fmt.Sprintf("%v", entries)))
	return fidelity.CertifiedTOC{
		Format:        "nomos.certified-toc.v1",
		DocumentRef:   "multi-document",
		StructureHash: hex.EncodeToString(h[:]),
		EntryCount:    len(entries),
		Entries:       entries,
	}
}
