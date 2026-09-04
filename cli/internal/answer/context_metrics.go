package answer

import "strings"

// Context metrics (VRC-13 follow-up, #620). The harness measured what the
// answer did with its citations; these measure what retrieval handed it, and
// whether the answer was contaminated by what it should not have used.
//
// They need ground truth: the golden corpus declares, per answer, the
// expected_chunk_ids relevant to the prompt. Gate answers declare none and
// get no context block.
//
//   - context_recall     = |expected ∩ retrieved| / |expected|
//   - context_precision  = Σ_k precision@k · rel_k / |relevant retrieved|
//     over the retrieved order (RAGAS context_precision@K): a distractor
//     ranked above the relevant chunk lowers it, a distractor ranked below
//     does not.
//   - noise_sensitivity  = answer sentences supported ONLY by distractor
//     chunks / all sentences. The lexical faithfulness proxy counts such a
//     sentence as supported — its tokens are in the support corpus — so this
//     is the metric that catches contamination faithfulness cannot see.
const (
	methodContext = "context_metrics_v1"

	contextLimitation = "context_metrics_v1 uses the golden corpus' expected_chunk_ids as ground truth and the same negation-blind lexical support as faithfulness; noise_sensitivity only detects sentences supported by distractor chunks alone."
)

// ContextMetrics is the retrieval-side view of one evaluated answer.
type ContextMetrics struct {
	Method                string  `json:"method"`
	ExpectedChunks        int     `json:"expected_chunks"`
	RetrievedChunks       int     `json:"retrieved_chunks"`
	RelevantRetrieved     int     `json:"relevant_retrieved"`
	DistractorChunks      int     `json:"distractor_chunks"`
	ContextRecall         float64 `json:"context_recall"`
	ContextPrecision      float64 `json:"context_precision"`
	NoiseSensitivity      float64 `json:"noise_sensitivity"`
	NoiseInducedSentences int     `json:"noise_induced_sentences"`
	TotalSentences        int     `json:"total_sentences"`
	Limitation            string  `json:"limitation"`
}

func expectedKey(id string) string { return "chunk_id\x00" + id }

// chunkText is the first non-empty text carrier of a chunk (same rule as
// supportCorpus).
func chunkText(c Chunk) string {
	for _, v := range []string{c.Text, c.ChunkText, c.Content} {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func tokenSet(texts []string) map[string]bool {
	set := map[string]bool{}
	for _, text := range texts {
		for _, t := range contentTokens(text) {
			set[t] = true
		}
	}
	return set
}

// coverage is the share of toks present in set.
func coverage(toks []string, set map[string]bool) float64 {
	if len(toks) == 0 {
		return 0
	}
	covered := 0
	for _, t := range toks {
		if set[t] {
			covered++
		}
	}
	return float64(covered) / float64(len(toks))
}

// contextMetrics computes the retrieval-side metrics against the declared
// expected chunks; nil when the answer declares none.
func (a Answer) contextMetrics(cfg Config) *ContextMetrics {
	expected := map[string]bool{}
	for _, id := range a.ExpectedChunkIDs {
		if id = strings.TrimSpace(id); id != "" {
			expected[expectedKey(id)] = true
		}
	}
	if len(expected) == 0 {
		return nil
	}
	m := &ContextMetrics{Method: methodContext, ExpectedChunks: len(expected), Limitation: contextLimitation}

	seenRelevant := map[string]bool{}
	relevantHits := 0
	var precisionSum float64
	var relevantTexts, distractorTexts []string
	for i, c := range a.RetrievedChunks {
		m.RetrievedChunks++
		if key := chunkKey(c); expected[key] {
			relevantHits++
			seenRelevant[key] = true
			precisionSum += float64(relevantHits) / float64(i+1)
			relevantTexts = append(relevantTexts, chunkText(c))
		} else {
			m.DistractorChunks++
			distractorTexts = append(distractorTexts, chunkText(c))
		}
	}
	// Cited span text follows its chunk: relevant if the span cites an
	// expected chunk, distractor otherwise.
	for _, s := range a.SourceSpans {
		if strings.TrimSpace(s.Text) == "" {
			continue
		}
		if expected[spanKey(s)] {
			relevantTexts = append(relevantTexts, s.Text)
		} else {
			distractorTexts = append(distractorTexts, s.Text)
		}
	}
	m.RelevantRetrieved = len(seenRelevant)
	m.ContextRecall = round4(float64(len(seenRelevant)) / float64(len(expected)))
	if relevantHits > 0 {
		m.ContextPrecision = round4(precisionSum / float64(relevantHits))
	}

	// A refusal (or an empty answer) asserts nothing: no sentence can be
	// noise-induced.
	if !a.requiresGrounding() {
		return m
	}
	sents := sentences(a.Answer)
	m.TotalSentences = len(sents)
	relevantTokens := tokenSet(relevantTexts)
	distractorTokens := tokenSet(distractorTexts)
	for _, sentence := range sents {
		toks := contentTokens(sentence)
		if len(toks) == 0 {
			continue
		}
		if coverage(toks, distractorTokens) >= cfg.SentenceThreshold && coverage(toks, relevantTokens) < cfg.SentenceThreshold {
			m.NoiseInducedSentences++
		}
	}
	if m.TotalSentences > 0 {
		m.NoiseSensitivity = round4(float64(m.NoiseInducedSentences) / float64(m.TotalSentences))
	}
	return m
}
