package corpus

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/RBOKproject/Nomos/cli/internal/fidelity"
)

func TestLoadTOCArtifactAcceptsCertifiedTOC(t *testing.T) {
	dir := t.TempDir()
	certified := fidelity.BuildCertifiedTOC(fidelity.TOCInput{
		DocumentRef: "DOC-RBOK",
		Headings: []fidelity.TOCHeading{
			{Title: "Root", Level: 1, Line: 1, NodeID: "DOC-RBOK"},
			{Title: "Scope", Level: 2, Line: 3, NodeID: "SEC-SCOPE"},
		},
	})
	data, err := json.Marshal(certified)
	if err != nil {
		t.Fatalf("marshal certified toc: %v", err)
	}
	path := filepath.Join(dir, "rbok-certified-toc.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write certified toc: %v", err)
	}

	toc, err := loadTOCArtifact(path)
	if err != nil {
		t.Fatalf("load toc: %v", err)
	}
	if !toc.Certified {
		t.Fatal("expected certified TOC to load as certified strict gate artifact")
	}
	if !fidelity.VerifyTOCArtifact(*toc) {
		t.Fatal("expected converted TOC artifact hash to verify")
	}
}

func TestBuildCASTFromFeedNodesStringifiesMetadata(t *testing.T) {
	assembly := MultiFeedAssembly{
		Feeds: []LawbookFeed{{
			Nodes: []LawbookNode{{
				NodeID:   "TBL-1",
				NodeType: NodeTable,
				Text:     "| A |\n|---|\n| B |",
				Span:     LawbookSourceSpan{StartLine: 1, EndLine: 3},
				Metadata: map[string]any{
					"col_count": 1,
					"row_count": "1",
				},
			}},
		}},
	}

	cast := buildCASTFromFeedNodes(assembly)
	if len(cast.Nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(cast.Nodes))
	}
	if cast.Nodes[0].Props["col_count"] != "1" {
		t.Fatalf("expected col_count stringified to 1, got %q", cast.Nodes[0].Props["col_count"])
	}
}
