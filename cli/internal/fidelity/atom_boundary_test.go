package fidelity

import (
	"strings"
	"testing"
)

func TestApplyBoundaryShortContent(t *testing.T) {
	result := ApplyBoundary("Short sentence here.", DefaultBoundaryPolicy())

	if result.Total != 1 {
		t.Fatalf("expected 1 segment for short content, got %d", result.Total)
	}
	if result.Splits != 0 {
		t.Fatalf("expected 0 splits, got %d", result.Splits)
	}
	if !result.Segments[0].Complete {
		t.Fatal("expected complete segment")
	}
}

func TestApplyBoundaryEmptyContent(t *testing.T) {
	result := ApplyBoundary("", DefaultBoundaryPolicy())
	if result.Total != 0 {
		t.Fatalf("expected 0 segments for empty, got %d", result.Total)
	}
}

func TestApplyBoundarySplitsBySentence(t *testing.T) {
	// Generate content exceeding max words.
	sentences := make([]string, 30)
	for i := range sentences {
		sentences[i] = "This is sentence number that has about ten words in it."
	}
	content := strings.Join(sentences, " ")

	policy := BoundaryPolicy{MaxWords: 50, MinWords: 10, PreferSentence: true, ProfileID: "test"}
	result := ApplyBoundary(content, policy)

	if result.Total <= 1 {
		t.Fatalf("expected multiple segments, got %d", result.Total)
	}
	// Each segment should end at a sentence boundary.
	for _, seg := range result.Segments {
		if !seg.Complete {
			t.Fatalf("segment %d does not end at sentence boundary: %q", seg.Index, seg.Text[max(0, len(seg.Text)-20):])
		}
	}
}

func TestApplyBoundaryNeverSplitMidSentence(t *testing.T) {
	content := "First sentence here. Second sentence continues. Third one ends."
	policy := BoundaryPolicy{MaxWords: 4, MinWords: 2, PreferSentence: true, ProfileID: "strict"}
	result := ApplyBoundary(content, policy)

	for _, seg := range result.Segments {
		trimmed := strings.TrimSpace(seg.Text)
		if trimmed == "" {
			continue
		}
		// Should end with sentence terminator.
		last := trimmed[len(trimmed)-1]
		if last != '.' && last != '!' && last != '?' {
			t.Fatalf("segment %d split mid-sentence: %q", seg.Index, trimmed)
		}
	}
}

func TestApplyBoundarySplitsByParagraph(t *testing.T) {
	para1 := "First paragraph with enough words to be meaningful on its own here."
	para2 := "Second paragraph also has sufficient words to stand alone as content."
	para3 := "Third paragraph completes the set with additional meaningful content."
	content := para1 + "\n\n" + para2 + "\n\n" + para3

	policy := BoundaryPolicy{MaxWords: 15, MinWords: 5, PreferParagraph: true, ProfileID: "para"}
	result := ApplyBoundary(content, policy)

	if result.Total < 2 {
		t.Fatalf("expected >= 2 segments from paragraphs, got %d", result.Total)
	}
}

func TestApplyBoundaryOrphanTailMerged(t *testing.T) {
	content := "Long sentence one. Long sentence two. Long sentence three. Short."
	policy := BoundaryPolicy{MaxWords: 8, MinWords: 5, PreferSentence: true, AllowOrphanTail: false, ProfileID: "no-orphan"}
	result := ApplyBoundary(content, policy)

	// Last segment should have been merged if below min.
	for _, seg := range result.Segments {
		if seg.WordCount < policy.MinWords && seg.Index < result.Total-1 {
			t.Fatalf("segment %d has %d words, below min %d", seg.Index, seg.WordCount, policy.MinWords)
		}
	}
	// With AllowOrphanTail=false, "Short." (1 word) should merge into previous.
	last := result.Segments[result.Total-1]
	if last.WordCount < policy.MinWords {
		t.Fatalf("orphan tail not merged: last segment has %d words", last.WordCount)
	}
}

func TestApplyBoundaryOrphanTailAllowed(t *testing.T) {
	content := "Long sentence one. Long sentence two. Long sentence three. End."
	policy := BoundaryPolicy{MaxWords: 8, MinWords: 5, PreferSentence: true, AllowOrphanTail: true, ProfileID: "allow-orphan"}
	result := ApplyBoundary(content, policy)

	// Last segment may be below min — that's OK with AllowOrphanTail.
	if result.Total == 0 {
		t.Fatal("expected segments")
	}
}

func TestApplyBoundaryWordCounts(t *testing.T) {
	content := "One. Two. Three. Four. Five."
	policy := BoundaryPolicy{MaxWords: 2, MinWords: 1, PreferSentence: true, ProfileID: "tiny"}
	result := ApplyBoundary(content, policy)

	for _, seg := range result.Segments {
		actual := len(strings.Fields(seg.Text))
		if actual != seg.WordCount {
			t.Fatalf("segment %d: reported %d words, actual %d", seg.Index, seg.WordCount, actual)
		}
	}
}

func TestApplyBoundaryRBOKPolicy(t *testing.T) {
	// Generate ~200 words of legal text.
	sentences := make([]string, 20)
	for i := range sentences {
		sentences[i] = "L'assure est tenu de repondre aux questions exactement."
	}
	content := strings.Join(sentences, " ")

	result := ApplyBoundary(content, RBOKBoundaryPolicy())

	if result.Total <= 1 {
		t.Fatalf("expected splits for RBOK policy (max 150), got %d segments", result.Total)
	}
	if result.Policy != "rbok" {
		t.Fatalf("expected policy rbok, got %s", result.Policy)
	}
}

func TestApplyBoundaryGameRulesPolicy(t *testing.T) {
	result := ApplyBoundary("Short rule.", GameRulesBoundaryPolicy())
	if result.Policy != "game-rules" {
		t.Fatalf("expected game-rules policy, got %s", result.Policy)
	}
}

func TestApplyBoundaryIndexSequential(t *testing.T) {
	sentences := make([]string, 15)
	for i := range sentences {
		sentences[i] = "Sentence with several words here."
	}
	content := strings.Join(sentences, " ")

	result := ApplyBoundary(content, BoundaryPolicy{MaxWords: 20, MinWords: 5, PreferSentence: true, ProfileID: "idx"})

	for i, seg := range result.Segments {
		if seg.Index != i {
			t.Fatalf("expected index %d, got %d", i, seg.Index)
		}
	}
}

func TestApplyBoundaryPreservesAllContent(t *testing.T) {
	content := "First sentence. Second sentence. Third sentence. Fourth sentence. Fifth sentence."
	policy := BoundaryPolicy{MaxWords: 5, MinWords: 2, PreferSentence: true, AllowOrphanTail: true, ProfileID: "preserve"}
	result := ApplyBoundary(content, policy)

	// Reassemble all segments and check no content lost.
	var reassembled strings.Builder
	for i, seg := range result.Segments {
		if i > 0 {
			reassembled.WriteByte(' ')
		}
		reassembled.WriteString(seg.Text)
	}

	original := strings.Join(strings.Fields(content), " ")
	rebuilt := strings.Join(strings.Fields(reassembled.String()), " ")
	if original != rebuilt {
		t.Fatalf("content lost:\noriginal: %s\nrebuilt:  %s", original, rebuilt)
	}
}

func TestDefaultBoundaryPolicyValues(t *testing.T) {
	p := DefaultBoundaryPolicy()
	if p.MaxWords != 200 {
		t.Fatalf("expected max 200, got %d", p.MaxWords)
	}
	if p.MinWords != 20 {
		t.Fatalf("expected min 20, got %d", p.MinWords)
	}
	if !p.PreferSentence {
		t.Fatal("expected PreferSentence true")
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
