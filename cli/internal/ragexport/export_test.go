package ragexport

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/RBOKproject/Nomos/cli/internal/corpus"
)

// sampleChunk is a well-formed composed chunk: heading-contextualised, fully
// provenanced. Tests mutate a copy to prove each guard actually bites.
func sampleChunk() corpus.ChunkMetadata {
	return corpus.ChunkMetadata{
		ChunkID:                  "chunk:SRC-1:100-220",
		SourceID:                 "SRC-1",
		SourcePath:               "corpus/rulebook.md",
		SourceHash:               "sha256:aaaa",
		Domain:                   "aec",
		UnitIDs:                  []string{"ART-1382"},
		Locator:                  "corpus/rulebook.md:L10-L14",
		Priority:                 "primary",
		Status:                   "active",
		Confidence:               "high",
		SegmentKind:              "paragraph",
		NormalizedTextHash:       "sha256:bbbb",
		SourceSegmentID:          "seg-1",
		SourceSegmentIDs:         []string{"seg-1"},
		CanonicalUnitID:          "ART-1382",
		StartByte:                100,
		EndByte:                  220,
		StartLine:                10,
		EndLine:                  14,
		ChunkCompositionStrategy: "single_atom",
		ContextHeadingPath:       []string{"Titre I", "Chapitre 3"},
		ContextSourceRole:        "reference",
		IngestedAt:               "2026-06-01T00:00:00Z",
		IngestionVersion:         "rag-v1",
		ChunkText:                "Titre I/Chapitre 3\n\nLe delai court des la notification.",
	}
}

func buildOne(t *testing.T, m corpus.ChunkMetadata) Record {
	t.Helper()
	res := Build([]corpus.ChunkMetadata{m})
	if len(res.Records) != 1 {
		t.Fatalf("expected 1 record, got %d (rejections: %+v)", len(res.Records), res.Rejections)
	}
	return res.Records[0]
}

func TestBuildContextPrefix_RendersStructureInFixedOrder(t *testing.T) {
	m := sampleChunk()
	m.ContextTableID = "T3"
	m.ContextRowIndex = 4
	m.ContextColumnHeaders = []string{"Type", "Valeur"}
	m.ContextYAMLPath = "policy.limits.max"

	got := BuildContextPrefix(m)
	want := "SRC-1 · reference · aec · Titre I › Chapitre 3 · table T3 row 4 · columns Type, Valeur · yaml policy.limits.max"
	if got != want {
		t.Fatalf("context prefix grammar drifted\n got: %q\nwant: %q", got, want)
	}
}

func TestBuildContextPrefix_OmitsEmptyGroups(t *testing.T) {
	m := corpus.ChunkMetadata{SourceID: "SRC-1", ChunkText: "body"}
	if got := BuildContextPrefix(m); got != "SRC-1" {
		t.Fatalf("expected lone source id, got %q", got)
	}
}

// Row 0 is the composer's zero value and is indistinguishable from "no row".
// Rendering it would situate the chunk on a row that may not exist.
func TestBuildContextPrefix_ZeroRowIndexIsNotRendered(t *testing.T) {
	m := sampleChunk()
	m.ContextTableID = "T3"
	m.ContextRowIndex = 0
	if got := BuildContextPrefix(m); strings.Contains(got, "row") {
		t.Fatalf("row 0 must not be rendered, got %q", got)
	}
}

// The context prefix is a retrieval aid that does not exist in the source. If
// it leaked into BodyText, a consumer citing the chunk would quote text no
// source contains — the exact failure the cite-or-abstain gate exists to catch.
func TestBuild_ContextPrefixNeverLeaksIntoCitableBody(t *testing.T) {
	rec := buildOne(t, sampleChunk())

	if strings.Contains(rec.BodyText, "SRC-1") || strings.Contains(rec.BodyText, "›") {
		t.Fatalf("context prefix leaked into citable body: %q", rec.BodyText)
	}
	if rec.BodyText != "Le delai court des la notification." {
		t.Fatalf("unexpected body: %q", rec.BodyText)
	}
	if !strings.HasPrefix(rec.EmbeddingText, rec.ContextPrefix) {
		t.Fatalf("embedding text must open with the context prefix, got %q", rec.EmbeddingText)
	}
	if !strings.HasSuffix(rec.EmbeddingText, rec.BodyText) {
		t.Fatalf("embedding text must close with the citable body, got %q", rec.EmbeddingText)
	}
}

// A body that itself contains the composer's heading separator must not be
// truncated: trusting the separator blindly would silently drop source text.
func TestBuild_BodyContainingSeparatorIsNotTruncated(t *testing.T) {
	m := sampleChunk()
	body := "Premier alinea.\n\nSecond alinea."
	m.ChunkText = "Titre I/Chapitre 3\n\n" + body

	rec := buildOne(t, m)
	if rec.BodyText != body {
		t.Fatalf("body truncated at separator\n got: %q\nwant: %q", rec.BodyText, body)
	}
}

func TestBuild_IsByteDeterministic(t *testing.T) {
	chunks := []corpus.ChunkMetadata{sampleChunk()}
	first, err := Encode(Build(chunks).Records, FormatJSONL)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	second, err := Encode(Build(chunks).Records, FormatJSONL)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if string(first) != string(second) {
		t.Fatal("export is not byte-deterministic: a CI gate could not diff it")
	}
}

// Feed ordering must not reach the index: a reordered feed that churned every
// chunk would force a full re-embed for no semantic change.
func TestBuild_OrderIsIndependentOfInputOrder(t *testing.T) {
	a := sampleChunk()
	b := sampleChunk()
	b.ChunkID = "chunk:SRC-1:000-099"
	b.ChunkText = "Titre I/Chapitre 3\n\nAutre alinea."

	forward, err := Encode(Build([]corpus.ChunkMetadata{a, b}).Records, FormatJSONL)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	reversed, err := Encode(Build([]corpus.ChunkMetadata{b, a}).Records, FormatJSONL)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if string(forward) != string(reversed) {
		t.Fatal("export order follows feed order: reordering the feed would churn the index")
	}
}

// The export is fail-closed. Each of these chunks would produce a retrieval hit
// that could not be cited or could not be proved fresh, so none may be indexed.
func TestBuild_RejectsChunksThatCouldNotBeCited(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*corpus.ChunkMetadata)
		code   string
	}{
		{"no chunk id", func(m *corpus.ChunkMetadata) { m.ChunkID = "" }, RejectMissingChunkID},
		{"no source id", func(m *corpus.ChunkMetadata) { m.SourceID = "" }, RejectMissingSourceID},
		{"no source hash", func(m *corpus.ChunkMetadata) { m.SourceHash = "" }, RejectMissingSourceHash},
		{"empty body", func(m *corpus.ChunkMetadata) { m.ChunkText = "Titre I/Chapitre 3\n\n   " }, RejectEmptyBody},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := sampleChunk()
			tc.mutate(&m)
			res := Build([]corpus.ChunkMetadata{m})
			if len(res.Records) != 0 {
				t.Fatalf("chunk was indexed anyway: %+v", res.Records)
			}
			if len(res.Rejections) != 1 || res.Rejections[0].Code != tc.code {
				t.Fatalf("expected rejection %s, got %+v", tc.code, res.Rejections)
			}
		})
	}
}

// Guard against the guard: the well-formed chunk must pass, or the rejection
// tests above would be green for the wrong reason.
func TestBuild_AcceptsWellFormedChunk(t *testing.T) {
	res := Build([]corpus.ChunkMetadata{sampleChunk()})
	if len(res.Records) != 1 || len(res.Rejections) != 0 {
		t.Fatalf("well-formed chunk was refused: %+v", res.Rejections)
	}
}

func TestEncode_LangChainKeepsBodySeparateFromPageContent(t *testing.T) {
	rec := buildOne(t, sampleChunk())
	data, err := Encode([]Record{rec}, FormatLangChain)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	var doc struct {
		PageContent string         `json:"page_content"`
		Metadata    map[string]any `json:"metadata"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if doc.PageContent != rec.EmbeddingText {
		t.Fatal("page_content must be the contextualised text that gets embedded")
	}
	if doc.Metadata["body_text"] != rec.BodyText {
		t.Fatal("citable body must survive into metadata, or consumers can only cite the prefix")
	}
	if doc.Metadata["source_hash"] != "sha256:aaaa" {
		t.Fatalf("provenance must stay filterable, got %v", doc.Metadata["source_hash"])
	}
}

func TestEncode_LlamaIndexCarriesStableID(t *testing.T) {
	rec := buildOne(t, sampleChunk())
	data, err := Encode([]Record{rec}, FormatLlamaIndex)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	var doc struct {
		ID   string `json:"id_"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if doc.ID != rec.ChunkID {
		t.Fatalf("node id must be the stable chunk id, got %q", doc.ID)
	}
	if doc.Text != rec.EmbeddingText {
		t.Fatal("node text must be the contextualised text")
	}
}

func TestParseFormat_RejectsUnknown(t *testing.T) {
	if _, err := ParseFormat("faiss"); err == nil {
		t.Fatal("an unknown format must fail loudly, not fall back to a default")
	}
	if f, err := ParseFormat(""); err != nil || f != FormatJSONL {
		t.Fatalf("empty format must default to jsonl, got %q / %v", f, err)
	}
}
