package fidelity

import (
	"strings"
	"unicode"
)

// BoundaryPolicy defines rules for splitting content into atoms.
type BoundaryPolicy struct {
	MaxWords         int    `json:"max_words"`
	MinWords         int    `json:"min_words"`
	PreferSentence   bool   `json:"prefer_sentence"`   // never split mid-sentence
	PreferParagraph  bool   `json:"prefer_paragraph"`  // prefer paragraph boundaries
	AllowOrphanTail  bool   `json:"allow_orphan_tail"` // allow final fragment below min
	ProfileID        string `json:"profile_id"`
}

// BoundaryResult holds the output of boundary splitting.
type BoundaryResult struct {
	Segments []Segment `json:"segments"`
	Total    int       `json:"total"`
	Splits   int       `json:"splits"`
	Policy   string    `json:"policy"`
}

// Segment is one content unit after boundary splitting.
type Segment struct {
	Index     int    `json:"index"`
	Text      string `json:"text"`
	WordCount int    `json:"word_count"`
	StartChar int    `json:"start_char"`
	EndChar   int    `json:"end_char"`
	Complete  bool   `json:"complete"` // ends at sentence boundary
}

// DefaultBoundaryPolicy returns a safe default policy.
func DefaultBoundaryPolicy() BoundaryPolicy {
	return BoundaryPolicy{
		MaxWords:        200,
		MinWords:        20,
		PreferSentence:  true,
		PreferParagraph: true,
		AllowOrphanTail: true,
		ProfileID:       "default",
	}
}

// RBOKBoundaryPolicy returns a policy tuned for legal/regulatory text.
func RBOKBoundaryPolicy() BoundaryPolicy {
	return BoundaryPolicy{
		MaxWords:        150,
		MinWords:        15,
		PreferSentence:  true,
		PreferParagraph: true,
		AllowOrphanTail: false,
		ProfileID:       "rbok",
	}
}

// GameRulesBoundaryPolicy returns a policy tuned for game rules.
func GameRulesBoundaryPolicy() BoundaryPolicy {
	return BoundaryPolicy{
		MaxWords:        100,
		MinWords:        10,
		PreferSentence:  true,
		PreferParagraph: true,
		AllowOrphanTail: true,
		ProfileID:       "game-rules",
	}
}

// ApplyBoundary splits content according to the policy.
func ApplyBoundary(content string, policy BoundaryPolicy) BoundaryResult {
	if policy.MaxWords <= 0 {
		policy.MaxWords = 200
	}
	if policy.MinWords <= 0 {
		policy.MinWords = 10
	}

	content = strings.TrimSpace(content)
	if content == "" {
		return BoundaryResult{Policy: policy.ProfileID}
	}

	wordCount := len(strings.Fields(content))
	if wordCount <= policy.MaxWords {
		// No split needed.
		return BoundaryResult{
			Segments: []Segment{{
				Index:     0,
				Text:      content,
				WordCount: wordCount,
				StartChar: 0,
				EndChar:   len(content),
				Complete:  endsAtSentence(content),
			}},
			Total:  1,
			Splits: 0,
			Policy: policy.ProfileID,
		}
	}

	// Split needed.
	var segments []Segment
	if policy.PreferParagraph {
		segments = splitByParagraphs(content, policy)
	}
	if len(segments) <= 1 {
		segments = splitBySentences(content, policy)
	}

	// Handle orphan tail.
	if !policy.AllowOrphanTail && len(segments) > 1 {
		last := segments[len(segments)-1]
		if last.WordCount < policy.MinWords {
			// Merge tail into previous segment.
			prev := &segments[len(segments)-2]
			prev.Text = prev.Text + " " + last.Text
			prev.WordCount += last.WordCount
			prev.EndChar = last.EndChar
			prev.Complete = last.Complete
			segments = segments[:len(segments)-1]
		}
	}

	// Re-index.
	for i := range segments {
		segments[i].Index = i
	}

	return BoundaryResult{
		Segments: segments,
		Total:    len(segments),
		Splits:   len(segments) - 1,
		Policy:   policy.ProfileID,
	}
}

func splitByParagraphs(content string, policy BoundaryPolicy) []Segment {
	paragraphs := strings.Split(content, "\n\n")
	if len(paragraphs) <= 1 {
		return nil // no paragraph splits available
	}

	var segments []Segment
	var current strings.Builder
	currentStart := 0
	currentWords := 0
	charOffset := 0

	for _, para := range paragraphs {
		para = strings.TrimSpace(para)
		if para == "" {
			charOffset += 2
			continue
		}
		paraWords := len(strings.Fields(para))

		if currentWords+paraWords > policy.MaxWords && currentWords >= policy.MinWords {
			// Flush current.
			text := strings.TrimSpace(current.String())
			segments = append(segments, Segment{
				Text:      text,
				WordCount: currentWords,
				StartChar: currentStart,
				EndChar:   currentStart + len(text),
				Complete:  endsAtSentence(text),
			})
			current.Reset()
			currentStart = charOffset
			currentWords = 0
		}

		if current.Len() > 0 {
			current.WriteString("\n\n")
		}
		current.WriteString(para)
		currentWords += paraWords
		charOffset += len(para) + 2
	}

	// Flush remainder.
	if current.Len() > 0 {
		text := strings.TrimSpace(current.String())
		segments = append(segments, Segment{
			Text:      text,
			WordCount: currentWords,
			StartChar: currentStart,
			EndChar:   currentStart + len(text),
			Complete:  endsAtSentence(text),
		})
	}

	return segments
}

func splitBySentences(content string, policy BoundaryPolicy) []Segment {
	sentences := splitSentences(content)
	if len(sentences) == 0 {
		return []Segment{{
			Text: content, WordCount: len(strings.Fields(content)),
			StartChar: 0, EndChar: len(content), Complete: true,
		}}
	}

	var segments []Segment
	var current strings.Builder
	currentStart := 0
	currentWords := 0
	charOffset := 0

	for _, sent := range sentences {
		sentWords := len(strings.Fields(sent))

		if currentWords+sentWords > policy.MaxWords && currentWords >= policy.MinWords {
			text := strings.TrimSpace(current.String())
			segments = append(segments, Segment{
				Text:      text,
				WordCount: currentWords,
				StartChar: currentStart,
				EndChar:   currentStart + len(text),
				Complete:  true,
			})
			current.Reset()
			currentStart = charOffset
			currentWords = 0
		}

		if current.Len() > 0 {
			current.WriteByte(' ')
		}
		current.WriteString(sent)
		currentWords += sentWords
		charOffset += len(sent) + 1
	}

	if current.Len() > 0 {
		text := strings.TrimSpace(current.String())
		segments = append(segments, Segment{
			Text:      text,
			WordCount: currentWords,
			StartChar: currentStart,
			EndChar:   currentStart + len(text),
			Complete:  endsAtSentence(text),
		})
	}

	return segments
}

func splitSentences(text string) []string {
	var sentences []string
	var current strings.Builder

	runes := []rune(text)
	for i := 0; i < len(runes); i++ {
		current.WriteRune(runes[i])

		if isSentenceEnd(runes, i) {
			sent := strings.TrimSpace(current.String())
			if sent != "" {
				sentences = append(sentences, sent)
			}
			current.Reset()
		}
	}

	if current.Len() > 0 {
		sent := strings.TrimSpace(current.String())
		if sent != "" {
			sentences = append(sentences, sent)
		}
	}

	return sentences
}

func isSentenceEnd(runes []rune, i int) bool {
	r := runes[i]
	if r != '.' && r != '!' && r != '?' {
		return false
	}
	// Must be followed by whitespace or end of text.
	if i+1 >= len(runes) {
		return true
	}
	next := runes[i+1]
	return unicode.IsSpace(next)
}

func endsAtSentence(text string) bool {
	text = strings.TrimSpace(text)
	if text == "" {
		return true
	}
	last := rune(text[len(text)-1])
	return last == '.' || last == '!' || last == '?'
}
