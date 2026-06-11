// Package answer is the cite-or-abstain gate in the Go engine (VRC-10 #556,
// A1). Faithfulness is RECOMPUTED from the retrieved span text — never trusted
// from a producer's self-declared score — and the verdict is cite or abstain.
//
// The recomputation is the deterministic lexical-entailment proxy ported from
// scripts/regulated_rag_answer_evidence.py (CKM-H6/H6-FU): an answer sentence
// is supported when at least SentenceThreshold of its content tokens appear in
// the union of cited/retrieved span text. Key invariants preserved:
//   - an explicit refusal asserts nothing → faithfulness 1.0, decision abstain
//     (legitimate);
//   - an answer that requires grounding but carries no span text scores 0
//     (the no-text bypass is closed — a producer cannot disarm the gate by
//     withholding span text);
//   - a self-declared faithfulness score may only LOWER the gated value, never
//     raise it;
//   - a forged citation (moved span / altered hash) stops binding to the
//     retrieved chunks → citation recall/precision drop → blocked.
//
// Limitation (documented, same as the sidecar): the lexical proxy is
// negation-blind; neural NLI is the pluggable upgrade, not implemented here.
package answer

import (
	"math"
	"regexp"
	"sort"
	"strings"
)

// Config carries the configurable gate thresholds. Defaults() mirrors the
// sidecar's published gates so the Go verdict and the Python evidence agree.
type Config struct {
	ALCEGate               float64
	FaithfulnessGate       float64
	TrustScoreCertified    float64
	TrustScoreIndicative   float64
	SentenceThreshold      float64
}

// Defaults returns the canonical gate configuration.
func Defaults() Config {
	return Config{
		ALCEGate:             0.95,
		FaithfulnessGate:     0.95,
		TrustScoreCertified:  0.95,
		TrustScoreIndicative: 0.80,
		SentenceThreshold:    0.6,
	}
}

const (
	methodLexical = "lexical_entailment_v1"
	methodNoText  = "no_span_text"
	methodStruct  = "structural_citation_coverage"

	groundednessLimitation = "lexical_entailment_v1 is negation-blind: it matches content-token overlap and cannot distinguish a claim from its negation. NLI is the pluggable upgrade. Spans that require grounding but carry no text score 0 (cannot be verified)."
)

var refusalOutcomes = map[string]bool{
	"acceptable_refusal":         true,
	"unsupported":                true,
	"blocked_prompt_injection":   true,
}

var stopwords = map[string]bool{
	"the": true, "a": true, "an": true, "and": true, "or": true, "of": true,
	"to": true, "in": true, "is": true, "are": true, "be": true, "for": true,
	"that": true, "this": true, "it": true, "as": true, "by": true, "on": true,
	"with": true, "must": true, "not": true, "no": true, "from": true, "its": true,
	"was": true, "were": true, "has": true, "have": true, "had": true, "but": true,
	"any": true, "all": true, "may": true,
}

var (
	tokenRe    = regexp.MustCompile(`[a-z0-9]+`)
	sentenceRe = regexp.MustCompile(`[.!?]+`)
)

// Span is one source-backed citation on the answer.
type Span struct {
	SourceID   string `json:"source_id"`
	SourceHash string `json:"source_hash"`
	Span       string `json:"span"`
	ChunkID    string `json:"chunk_id,omitempty"`
	Text       string `json:"text,omitempty"`
}

// Chunk is one retrieved candidate the answer was grounded against.
type Chunk struct {
	ChunkID    string `json:"chunk_id,omitempty"`
	SourceID   string `json:"source_id,omitempty"`
	SourceHash string `json:"source_hash,omitempty"`
	Span       string `json:"span,omitempty"`
	Text       string `json:"text,omitempty"`
	ChunkText  string `json:"chunk_text,omitempty"`
	Content    string `json:"content,omitempty"`
}

// Answer is the producer's RAG answer record submitted to the gate.
type Answer struct {
	AnswerID          string   `json:"answer_id"`
	PromptID          string   `json:"prompt_id"`
	Answer            string   `json:"answer"`
	CitationStatus    string   `json:"citation_status"`
	RefusalStatus     string   `json:"refusal_status"`
	PolicyOutcome     string   `json:"policy_outcome"`
	Confidence        float64  `json:"confidence"`
	FaithfulnessScore *float64 `json:"faithfulness_score,omitempty"`
	SourceSpans       []Span   `json:"source_spans"`
	RetrievedChunks   []Chunk  `json:"retrieved_chunks"`
}

// Finding is one blocking gate violation.
type Finding struct {
	Code     string `json:"code"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

// Groundedness records how faithfulness was derived.
type Groundedness struct {
	Method             string  `json:"method"`
	Score              float64 `json:"score"`
	SupportedSentences int     `json:"supported_sentences"`
	TotalSentences     int     `json:"total_sentences"`
	RecomputedFromSpans bool   `json:"recomputed_from_spans"`
	SelfDeclared       *float64 `json:"self_declared"`
	SelfDeclaredTrusted bool   `json:"self_declared_trusted"`
	Limitation         string  `json:"limitation"`
}

// Verdict is the gate's decision for one answer.
type Verdict struct {
	AnswerID          string       `json:"answer_id"`
	Decision          string       `json:"decision"` // "cite" | "abstain"
	TrustTier         string       `json:"trust_tier"`
	CitationRecall    float64      `json:"citation_recall"`
	CitationPrecision float64      `json:"citation_precision"`
	Faithfulness      float64      `json:"faithfulness"`
	TrustScore        float64      `json:"trust_score"`
	Groundedness      Groundedness `json:"groundedness"`
	Findings          []Finding    `json:"findings"`
}

func round4(f float64) float64 { return math.Round(f*1e4) / 1e4 }

func contentTokens(text string) []string {
	out := []string{}
	for _, t := range tokenRe.FindAllString(strings.ToLower(text), -1) {
		if len(t) >= 3 && !stopwords[t] {
			out = append(out, t)
		}
	}
	return out
}

func sentences(text string) []string {
	out := []string{}
	for _, part := range sentenceRe.Split(text, -1) {
		if s := strings.TrimSpace(part); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func (a Answer) hasExplicitRefusal() bool {
	rs := strings.TrimSpace(a.RefusalStatus)
	po := strings.TrimSpace(a.PolicyOutcome)
	return (rs == "refused" || rs == "unsupported") && refusalOutcomes[po]
}

func (a Answer) requiresGrounding() bool {
	if a.hasExplicitRefusal() {
		return false
	}
	return strings.TrimSpace(a.Answer) != ""
}

func (a Answer) hasSourceBackedCitation() bool {
	if strings.TrimSpace(a.CitationStatus) != "source_backed" {
		return false
	}
	if len(a.SourceSpans) == 0 {
		return false
	}
	for _, s := range a.SourceSpans {
		if s.SourceID == "" || s.SourceHash == "" || s.Span == "" {
			return false
		}
	}
	return true
}

func chunkKey(c Chunk) string {
	if c.ChunkID != "" {
		return "chunk_id\x00" + c.ChunkID
	}
	return "source_span\x00" + c.SourceID + "\x00" + c.SourceHash + "\x00" + c.Span
}

func spanKey(s Span) string {
	if s.ChunkID != "" {
		return "chunk_id\x00" + s.ChunkID
	}
	return "source_span\x00" + s.SourceID + "\x00" + s.SourceHash + "\x00" + s.Span
}

func (a Answer) citationMetrics() (recall, precision float64) {
	retrieved := map[string]bool{}
	for _, c := range a.RetrievedChunks {
		retrieved[chunkKey(c)] = true
	}
	cited := map[string]bool{}
	for _, s := range a.SourceSpans {
		cited[spanKey(s)] = true
	}
	if len(retrieved) == 0 && a.hasExplicitRefusal() {
		return 1.0, 1.0
	}
	inter := 0
	for k := range cited {
		if retrieved[k] {
			inter++
		}
	}
	if len(retrieved) == 0 {
		if len(cited) == 0 {
			recall = 1.0
		}
	} else {
		recall = float64(inter) / float64(len(retrieved))
	}
	if len(cited) == 0 {
		if len(retrieved) == 0 {
			precision = 1.0
		}
	} else {
		precision = float64(inter) / float64(len(cited))
	}
	return round4(recall), round4(precision)
}

func (a Answer) supportCorpus() []string {
	texts := []string{}
	for _, s := range a.SourceSpans {
		if s.Text != "" {
			texts = append(texts, s.Text)
		}
	}
	for _, c := range a.RetrievedChunks {
		for _, v := range []string{c.Text, c.ChunkText, c.Content} {
			if strings.TrimSpace(v) != "" {
				texts = append(texts, v)
				break
			}
		}
	}
	return texts
}

// recomputeGroundedness mirrors the sidecar: returns (detail, applicable).
// applicable=false means grounding is genuinely not applicable (refusal / no
// answer text), so the caller falls back to structural coverage.
func (a Answer) recomputeGroundedness() (Groundedness, bool) {
	hasAnswerText := strings.TrimSpace(a.Answer) != ""
	support := a.supportCorpus()

	if len(support) == 0 {
		if a.requiresGrounding() {
			total := 0
			if hasAnswerText {
				total = len(sentences(a.Answer))
			}
			return Groundedness{Method: methodNoText, Score: 0, TotalSentences: total}, true
		}
		return Groundedness{}, false
	}
	if !hasAnswerText {
		return Groundedness{}, false
	}
	supportTokens := map[string]bool{}
	for _, text := range support {
		for _, t := range contentTokens(text) {
			supportTokens[t] = true
		}
	}
	sents := sentences(a.Answer)
	if len(sents) == 0 {
		return Groundedness{}, false
	}
	supported := 0
	for _, sentence := range sents {
		toks := contentTokens(sentence)
		if len(toks) == 0 {
			supported++
			continue
		}
		covered := 0
		for _, t := range toks {
			if supportTokens[t] {
				covered++
			}
		}
		if float64(covered)/float64(len(toks)) >= Defaults().SentenceThreshold {
			supported++
		}
	}
	return Groundedness{
		Method:              methodLexical,
		Score:               round4(float64(supported) / float64(len(sents))),
		SupportedSentences:  supported,
		TotalSentences:      len(sents),
		RecomputedFromSpans: true,
	}, true
}

// Evaluate runs the cite-or-abstain gate on one answer.
func Evaluate(a Answer, cfg Config) Verdict {
	recall, precision := a.citationMetrics()
	ground, applicable := a.recomputeGroundedness()

	var base float64
	switch {
	case a.hasExplicitRefusal():
		base = 1.0
	case applicable:
		base = ground.Score
	case a.requiresGrounding():
		base = 0.0
	default:
		base = math.Min(recall, precision)
		ground = Groundedness{Method: methodStruct, Score: round4(math.Min(recall, precision))}
	}
	// A producer may declare itself LESS faithful, never more.
	if a.FaithfulnessScore != nil {
		d := *a.FaithfulnessScore
		if d >= 0 && d <= 1 {
			ground.SelfDeclared = &d
			base = math.Min(base, d)
		}
	}
	faithfulness := round4(base)
	ground.Limitation = groundednessLimitation

	trustScore := round4((recall + precision + faithfulness + clamp01(a.Confidence)) / 4)

	findings := a.validate(cfg, recall, precision, faithfulness)

	v := Verdict{
		AnswerID:          a.AnswerID,
		TrustTier:         trustTier(cfg, recall, precision, faithfulness, trustScore, findings),
		CitationRecall:    recall,
		CitationPrecision: precision,
		Faithfulness:      faithfulness,
		TrustScore:        trustScore,
		Groundedness:      ground,
		Findings:          findings,
	}
	// cite-or-abstain: an explicit refusal abstains legitimately; an acceptable
	// answer with no blocking findings cites; anything else is forced to abstain.
	if a.hasExplicitRefusal() {
		v.Decision = "abstain"
	} else if strings.TrimSpace(a.PolicyOutcome) == "acceptable" && len(findings) == 0 {
		v.Decision = "cite"
	} else {
		v.Decision = "abstain"
	}
	return v
}

func clamp01(f float64) float64 {
	if f < 0 {
		return 0
	}
	if f > 1 {
		return 1
	}
	return f
}

func (a Answer) validate(cfg Config, recall, precision, faithfulness float64) []Finding {
	findings := []Finding{}
	add := func(code, msg string) { findings = append(findings, Finding{Code: code, Severity: "error", Message: msg}) }

	if strings.TrimSpace(a.CitationStatus) == "source_backed" && !a.hasSourceBackedCitation() {
		add("SOURCE_BACKED_CITATION_WITHOUT_SOURCE_SPANS",
			"source_backed citation status requires source_id, source_hash, and span")
	}
	if strings.TrimSpace(a.PolicyOutcome) == "acceptable" {
		if !a.hasSourceBackedCitation() && !a.hasExplicitRefusal() {
			add("ACCEPTABLE_WITHOUT_CITATION_OR_REFUSAL",
				"acceptable answers require source-backed citations or explicit refusal")
		}
		if recall < cfg.ALCEGate {
			add("ALCE_CITATION_RECALL_BELOW_GATE",
				"retrieved chunks are not fully covered by source-backed citations")
		}
		if precision < cfg.ALCEGate {
			add("ALCE_CITATION_PRECISION_BELOW_GATE",
				"citations include spans that do not bind to retrieved chunks")
		}
		if faithfulness < cfg.FaithfulnessGate {
			add("DEEPEVAL_FAITHFULNESS_BELOW_GATE",
				"faithfulness score is below the cite-or-abstain gate")
		}
	}
	return findings
}

func trustTier(cfg Config, recall, precision, faithfulness, trustScore float64, findings []Finding) string {
	if len(findings) > 0 {
		return "unverified"
	}
	if recall >= cfg.ALCEGate && precision >= cfg.ALCEGate &&
		faithfulness >= cfg.FaithfulnessGate && trustScore >= cfg.TrustScoreCertified {
		return "certified"
	}
	if trustScore >= cfg.TrustScoreIndicative {
		return "indicative"
	}
	return "unverified"
}

// GateResult aggregates verdicts over a batch of answers.
type GateResult struct {
	Status   string    `json:"status"` // "pass" | "fail"
	Checked  int       `json:"checked"`
	Cited    int       `json:"cited"`
	Abstained int      `json:"abstained"`
	Findings int       `json:"findings"`
	Verdicts []Verdict `json:"verdicts"`
}

// Gate evaluates a batch and fails closed if any answer carries findings.
func Gate(answers []Answer, cfg Config) GateResult {
	res := GateResult{Status: "pass"}
	for _, a := range answers {
		v := Evaluate(a, cfg)
		res.Verdicts = append(res.Verdicts, v)
		res.Checked++
		if v.Decision == "cite" {
			res.Cited++
		} else {
			res.Abstained++
		}
		res.Findings += len(v.Findings)
	}
	if res.Findings > 0 {
		res.Status = "fail"
	}
	// Deterministic verdict order by answer id.
	sort.SliceStable(res.Verdicts, func(i, j int) bool { return res.Verdicts[i].AnswerID < res.Verdicts[j].AnswerID })
	return res
}
