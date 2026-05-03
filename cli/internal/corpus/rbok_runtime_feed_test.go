package corpus

import (
	"testing"
)

func validRuntimeNode() RuntimeFeedNode {
	return RuntimeFeedNode{
		NodeID:         "ART-L113-1",
		DocumentID:     "DOC-CODE-ASSURANCES",
		CanonicalRef:   "code-assurances/l113-1",
		DisplayRef:     "Art. L113-1",
		SourcePath:     "01_rbok/code-assurances/l113-1.md",
		SourceHash:     "sha256:aaaa1111aaaa1111aaaa1111aaaa1111aaaa1111aaaa1111aaaa1111aaaa1111",
		Status:         StatusActive,
		Priority:       PriorityCritical,
		Domain:         "insurance-regulation",
		Layer:          LayerRBOK,
		AuthorityLevel: AuthBinding,
		NodeType:       "article",
		Depth:          4,
	}
}

func validRuntimeFeed() RuntimeFeed {
	lawNode := validRuntimeNode()
	parcoursNode := RuntimeFeedNode{
		NodeID:         "PARCOURS-SINISTRE",
		DocumentID:     "DOC-PARCOURS-SINISTRE",
		CanonicalRef:   "parcours/sinistre",
		DisplayRef:     "Parcours sinistre",
		SourcePath:     "02_parcours/sinistre/index.md",
		SourceHash:     "sha256:bbbb2222bbbb2222bbbb2222bbbb2222bbbb2222bbbb2222bbbb2222bbbb2222",
		Status:         StatusActive,
		Priority:       PriorityHigh,
		Domain:         "insurance-regulation",
		Layer:          LayerParcours,
		AuthorityLevel: AuthInternal,
		NodeType:       "parcours",
		Depth:          0,
	}
	wbNode := RuntimeFeedNode{
		NodeID:         "WB-FORM-SINISTRE",
		DocumentID:     "DOC-WORKBOOKS",
		CanonicalRef:   "workbooks/form-sinistre",
		DisplayRef:     "Formulaire sinistre",
		SourcePath:     "03_workbooks/forms/sinistre.md",
		SourceHash:     "sha256:cccc3333cccc3333cccc3333cccc3333cccc3333cccc3333cccc3333cccc3333",
		Status:         StatusActive,
		Priority:       PriorityMedium,
		Domain:         "insurance-regulation",
		Layer:          LayerWorkbooks,
		AuthorityLevel: AuthInformational,
		NodeType:       "form",
		Depth:          1,
		RefType:        "form",
	}
	return RuntimeFeed{
		SchemaVersion: "0.1.0",
		FeedFormat:    RuntimeFeedFormat,
		FeedID:        "rbok-runtime-test",
		CorpusID:      "realisons-business",
		Domain:        "insurance-regulation",
		GeneratedAt:   "2026-05-03T10:00:00Z",
		Layers:        []CorpusLayer{LayerRBOK, LayerParcours, LayerWorkbooks},
		NodeCount:     3,
		Nodes:         []RuntimeFeedNode{lawNode, parcoursNode, wbNode},
	}
}

func TestValidateRuntimeFeedNodeValid(t *testing.T) {
	errs := ValidateRuntimeFeedNode(validRuntimeNode())
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got %v", errs)
	}
}

func TestValidateRuntimeFeedNodeInvalidLayer(t *testing.T) {
	n := validRuntimeNode()
	n.Layer = "05_unknown"
	errs := ValidateRuntimeFeedNode(n)
	if len(errs) == 0 {
		t.Fatal("expected error for invalid layer")
	}
}

func TestValidateRuntimeFeedNodeInvalidAuthority(t *testing.T) {
	n := validRuntimeNode()
	n.AuthorityLevel = "supreme"
	errs := ValidateRuntimeFeedNode(n)
	if len(errs) == 0 {
		t.Fatal("expected error for invalid authority")
	}
}

func TestValidateRuntimeFeedValid(t *testing.T) {
	f := validRuntimeFeed()
	f.LayerSummary = ComputeLayerSummary(f.Nodes)
	errs := ValidateRuntimeFeed(f)
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got %v", errs)
	}
}

func TestValidateRuntimeFeedWrongFormat(t *testing.T) {
	f := validRuntimeFeed()
	f.FeedFormat = "wrong"
	errs := ValidateRuntimeFeed(f)
	found := false
	for _, e := range errs {
		if e != "" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected error for wrong feed_format")
	}
}

func TestValidateRuntimeFeedCountMismatch(t *testing.T) {
	f := validRuntimeFeed()
	f.NodeCount = 99
	errs := ValidateRuntimeFeed(f)
	found := false
	for _, e := range errs {
		if len(e) > 0 {
			found = true
		}
	}
	if !found {
		t.Fatal("expected error for count mismatch")
	}
}

func TestComputeLayerSummary(t *testing.T) {
	f := validRuntimeFeed()
	summary := ComputeLayerSummary(f.Nodes)

	if summary["01_rbok"].NodeCount != 1 {
		t.Fatalf("expected 1 rbok node, got %d", summary["01_rbok"].NodeCount)
	}
	if summary["02_parcours"].NodeCount != 1 {
		t.Fatalf("expected 1 parcours node, got %d", summary["02_parcours"].NodeCount)
	}
	if summary["03_workbooks"].NodeCount != 1 {
		t.Fatalf("expected 1 workbook node, got %d", summary["03_workbooks"].NodeCount)
	}
	if summary["01_rbok"].AuthorityBreakdown["binding"] != 1 {
		t.Fatalf("expected 1 binding in rbok layer")
	}
	if summary["02_parcours"].AuthorityBreakdown["internal"] != 1 {
		t.Fatalf("expected 1 internal in parcours layer")
	}
	if summary["01_rbok"].DocumentCount != 1 {
		t.Fatalf("expected 1 document in rbok, got %d", summary["01_rbok"].DocumentCount)
	}
}

func TestComputeLayerSummaryMultipleDocsPerLayer(t *testing.T) {
	nodes := []RuntimeFeedNode{
		{NodeID: "A1", DocumentID: "DOC-1", Layer: LayerRBOK, AuthorityLevel: AuthBinding,
			CanonicalRef: "a", DisplayRef: "a", SourceHash: "sha256:aa", Domain: "d"},
		{NodeID: "A2", DocumentID: "DOC-2", Layer: LayerRBOK, AuthorityLevel: AuthRegulatory,
			CanonicalRef: "b", DisplayRef: "b", SourceHash: "sha256:bb", Domain: "d"},
		{NodeID: "A3", DocumentID: "DOC-1", Layer: LayerRBOK, AuthorityLevel: AuthBinding,
			CanonicalRef: "c", DisplayRef: "c", SourceHash: "sha256:cc", Domain: "d"},
	}
	summary := ComputeLayerSummary(nodes)

	if summary["01_rbok"].NodeCount != 3 {
		t.Fatalf("expected 3 nodes, got %d", summary["01_rbok"].NodeCount)
	}
	if summary["01_rbok"].DocumentCount != 2 {
		t.Fatalf("expected 2 documents, got %d", summary["01_rbok"].DocumentCount)
	}
	if summary["01_rbok"].AuthorityBreakdown["binding"] != 2 {
		t.Fatalf("expected 2 binding, got %d", summary["01_rbok"].AuthorityBreakdown["binding"])
	}
}

func TestCorpusLayerIsValid(t *testing.T) {
	valid := []CorpusLayer{LayerRBOK, LayerParcours, LayerWorkbooks, LayerDoctrine, LayerArchive}
	for _, l := range valid {
		if !l.IsValid() {
			t.Fatalf("expected %s valid", l)
		}
	}
	if CorpusLayer("05_bogus").IsValid() {
		t.Fatal("expected bogus invalid")
	}
}

func TestAuthorityLevelIsValid(t *testing.T) {
	valid := []AuthorityLevel{AuthBinding, AuthRegulatory, AuthGuidance, AuthInformational, AuthInternal, AuthDeprecated}
	for _, a := range valid {
		if !a.IsValid() {
			t.Fatalf("expected %s valid", a)
		}
	}
	if AuthorityLevel("supreme").IsValid() {
		t.Fatal("expected bogus invalid")
	}
}

func TestRuntimeFeedFormat(t *testing.T) {
	if RuntimeFeedFormat != "nomos.rbok-runtime-feed.v1" {
		t.Fatalf("expected nomos.rbok-runtime-feed.v1, got %s", RuntimeFeedFormat)
	}
}

func TestParcoursNodeFields(t *testing.T) {
	n := RuntimeFeedNode{
		NodeID: "STEP-001", DocumentID: "DOC-P",
		CanonicalRef: "p/step1", DisplayRef: "Step 1",
		SourcePath: "02_parcours/step.md",
		SourceHash: "sha256:dddd4444dddd4444dddd4444dddd4444dddd4444dddd4444dddd4444dddd4444",
		Status: StatusActive, Priority: PriorityHigh,
		Domain: "insurance", Layer: LayerParcours,
		AuthorityLevel: AuthInternal, NodeType: "step", Depth: 2,
		PredecessorIDs: []string{"STEP-000"},
		SuccessorIDs:   []string{"STEP-002"},
		GateCriteria:   "Documents complets.",
	}
	errs := ValidateRuntimeFeedNode(n)
	if len(errs) != 0 {
		t.Fatalf("expected no errors for parcours node, got %v", errs)
	}
}

func TestWorkbookNodeFields(t *testing.T) {
	n := RuntimeFeedNode{
		NodeID: "WB-TPL-001", DocumentID: "DOC-WB",
		CanonicalRef: "wb/template", DisplayRef: "Template X",
		SourcePath: "03_workbooks/tpl.md",
		SourceHash: "sha256:eeee5555eeee5555eeee5555eeee5555eeee5555eeee5555eeee5555eeee5555",
		Status: StatusActive, Priority: PriorityLow,
		Domain: "insurance", Layer: LayerWorkbooks,
		AuthorityLevel: AuthInformational, NodeType: "template", Depth: 1,
		RefType:    "template",
		TargetURL:  "https://intranet/templates/x.docx",
		TargetHash: "sha256:ffff6666ffff6666ffff6666ffff6666ffff6666ffff6666ffff6666ffff6666",
	}
	errs := ValidateRuntimeFeedNode(n)
	if len(errs) != 0 {
		t.Fatalf("expected no errors for workbook node, got %v", errs)
	}
}
