package answer

import "testing"

// VRC-10 (#556, A1) — doctrine §2.3: the cite-or-abstain gate proves itself by
// REFUSING (abstaining / blocking) on forged citations and missing grounding,
// and by computing faithfulness from the spans, not from a declared number.

func groundedAnswer() Answer {
	// A faithful, source-backed answer whose sentence tokens are covered by the
	// cited span text — the happy path that must CITE.
	return Answer{
		AnswerID:       "A1",
		PromptID:       "P1",
		Answer:         "Le gabarit retient une hauteur de neuf metres au faite.",
		CitationStatus: "source_backed",
		PolicyOutcome:  "acceptable",
		Confidence:     0.99,
		SourceSpans: []Span{{
			SourceID: "S1", SourceHash: "sha256:abc", Span: "L1-L2", ChunkID: "c1",
			Text: "Le gabarit retient une hauteur de neuf metres au faite pour le volume principal.",
		}},
		RetrievedChunks: []Chunk{{ChunkID: "c1", Text: "Le gabarit retient une hauteur de neuf metres au faite."}},
	}
}

func TestGate_GroundedAnswerCites(t *testing.T) {
	v := Evaluate(groundedAnswer(), Defaults())
	if v.Decision != "cite" {
		t.Fatalf("a grounded source-backed answer must cite, got %q (findings %+v)", v.Decision, v.Findings)
	}
	if v.Groundedness.Method != methodLexical || !v.Groundedness.RecomputedFromSpans {
		t.Fatalf("faithfulness must be recomputed from spans, got %+v", v.Groundedness)
	}
	if v.Faithfulness < Defaults().FaithfulnessGate {
		t.Fatalf("grounded answer faithfulness below gate: %v", v.Faithfulness)
	}
}

func TestGate_ForgedCitationHashIsBlocked(t *testing.T) {
	// Move the citation off its retrieved chunk by altering the hash: the cited
	// key no longer binds to any retrieved chunk → recall/precision collapse →
	// the answer is forced to abstain.
	a := groundedAnswer()
	a.SourceSpans[0].ChunkID = ""          // force source-span keying
	a.SourceSpans[0].SourceHash = "sha256:TAMPERED"
	a.RetrievedChunks = []Chunk{{SourceID: "S1", SourceHash: "sha256:abc", Span: "L1-L2",
		Text: "Le gabarit retient une hauteur de neuf metres au faite."}}
	v := Evaluate(a, Defaults())
	if v.Decision != "abstain" {
		t.Fatalf("a forged-hash citation must abstain, got %q", v.Decision)
	}
	if !hasFinding(v, "ALCE_CITATION_RECALL_BELOW_GATE") && !hasFinding(v, "ALCE_CITATION_PRECISION_BELOW_GATE") {
		t.Fatalf("expected an ALCE binding finding, got %+v", v.Findings)
	}
	if v.TrustTier != "unverified" {
		t.Fatalf("a blocked answer cannot be trusted, got %q", v.TrustTier)
	}
}

func TestGate_NoSpanForcesAbstention(t *testing.T) {
	a := groundedAnswer()
	a.SourceSpans = nil
	a.RetrievedChunks = nil
	v := Evaluate(a, Defaults())
	if v.Decision != "abstain" {
		t.Fatalf("an answer with no span must abstain, got %q", v.Decision)
	}
	if v.Faithfulness != 0 {
		t.Fatalf("no span must floor faithfulness at 0, got %v", v.Faithfulness)
	}
}

func TestGate_NoSpanTextBypassIsClosed(t *testing.T) {
	// The #538 bypass: valid-looking citations but NO span text. Groundedness
	// must be 0 (unverifiable), not structural ~1.0.
	a := groundedAnswer()
	a.SourceSpans[0].Text = ""
	a.RetrievedChunks = []Chunk{{ChunkID: "c1"}} // chunk present, no text
	v := Evaluate(a, Defaults())
	if v.Groundedness.Method != methodNoText {
		t.Fatalf("no span text must use the no_span_text method, got %q", v.Groundedness.Method)
	}
	if v.Faithfulness != 0 || v.Decision != "abstain" {
		t.Fatalf("no span text must score 0 and abstain, got faith=%v decision=%q", v.Faithfulness, v.Decision)
	}
}

func TestGate_DeclaredScoreCanOnlyLower(t *testing.T) {
	// A producer declaring 0.99 cannot RAISE a recomputed-low score…
	a := groundedAnswer()
	a.Answer = "Cette phrase parle de tout autre chose, completement hors sujet ici."
	high := 0.99
	a.FaithfulnessScore = &high
	v := Evaluate(a, Defaults())
	if v.Faithfulness >= Defaults().FaithfulnessGate {
		t.Fatalf("a declared score must not raise a low recompute, got %v", v.Faithfulness)
	}
	// …and declaring LOW does lower a high recompute.
	b := groundedAnswer()
	low := 0.10
	b.FaithfulnessScore = &low
	vb := Evaluate(b, Defaults())
	if vb.Faithfulness > 0.10 {
		t.Fatalf("a low declared score must lower the gated value, got %v", vb.Faithfulness)
	}
	if vb.Decision != "abstain" {
		t.Fatalf("self-declared-low must abstain, got %q", vb.Decision)
	}
}

func TestGate_ExplicitRefusalAbstainsLegitimately(t *testing.T) {
	a := Answer{
		AnswerID: "R1", PolicyOutcome: "acceptable_refusal",
		RefusalStatus: "refused", CitationStatus: "none",
		Answer: "", Confidence: 0.0,
	}
	v := Evaluate(a, Defaults())
	if v.Decision != "abstain" {
		t.Fatalf("a refusal abstains, got %q", v.Decision)
	}
	if len(v.Findings) != 0 {
		t.Fatalf("a legitimate refusal carries no findings, got %+v", v.Findings)
	}
	if v.Faithfulness != 1.0 {
		t.Fatalf("a refusal asserts nothing → faithfulness 1.0, got %v", v.Faithfulness)
	}
}

func TestGate_BatchFailsClosedOnAnyFinding(t *testing.T) {
	good := groundedAnswer()
	bad := groundedAnswer()
	bad.AnswerID = "A2"
	bad.SourceSpans = nil
	bad.RetrievedChunks = nil
	res := Gate([]Answer{good, bad}, Defaults())
	if res.Status != "fail" {
		t.Fatalf("a batch with one ungrounded answer must fail, got %q", res.Status)
	}
	if res.Cited != 1 || res.Abstained != 1 {
		t.Fatalf("expected 1 cite + 1 abstain, got cited=%d abstained=%d", res.Cited, res.Abstained)
	}
}

func hasFinding(v Verdict, code string) bool {
	for _, f := range v.Findings {
		if f.Code == code {
			return true
		}
	}
	return false
}
