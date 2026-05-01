package corpus

import (
	"strings"
	"testing"
	"time"
)

var ragTestNow = time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)

func validInput() ChunkInput {
	return ChunkInput{
		Content:    "L'assure doit declarer le sinistre dans les 5 jours ouvrables.",
		SourceID:   "RULEBOOK-2026",
		SourcePath: "docs/rulebook-2026.pdf",
		SourceHash: "sha256:abcdef1234567890",
		Domain:     "assurance-auto",
		UnitIDs:    []string{"RBOK-RUL-delai-declaration-v1"},
		Locator:    "p.42 s.3",
		Priority:   "primary",
		Status:     "active",
		Confidence: "high",
		Tags:       []string{"sinistre", "delai", "obligation"},
	}
}

func validConfig() EnrichConfig {
	return EnrichConfig{
		IngestionVersion: "0.1.0",
		Now:              ragTestNow,
	}
}

func TestEnrich_Valid(t *testing.T) {
	meta, err := Enrich(validInput(), validConfig())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(meta.ChunkID, "chunk-") {
		t.Fatalf("expected chunk ID prefix 'chunk-', got %q", meta.ChunkID)
	}
	if len(meta.ChunkID) != len("chunk-")+16 {
		t.Fatalf("expected chunk ID length %d, got %d", len("chunk-")+16, len(meta.ChunkID))
	}
	if meta.SourceID != "RULEBOOK-2026" {
		t.Fatalf("expected source ID RULEBOOK-2026, got %q", meta.SourceID)
	}
	if meta.Domain != "assurance-auto" {
		t.Fatalf("expected domain assurance-auto, got %q", meta.Domain)
	}
	if meta.TokenCount <= 0 {
		t.Fatalf("expected positive token count, got %d", meta.TokenCount)
	}
	if meta.CharCount <= 0 {
		t.Fatalf("expected positive char count, got %d", meta.CharCount)
	}
	if meta.IngestedAt != "2026-05-01T10:00:00Z" {
		t.Fatalf("expected ingested_at timestamp, got %q", meta.IngestedAt)
	}
	if meta.IngestionVersion != "0.1.0" {
		t.Fatalf("expected ingestion version 0.1.0, got %q", meta.IngestionVersion)
	}
	if meta.Confidence != "high" {
		t.Fatalf("expected confidence high, got %q", meta.Confidence)
	}
	if len(meta.SemanticTags) != 3 {
		t.Fatalf("expected 3 tags, got %d", len(meta.SemanticTags))
	}
	if len(meta.UnitIDs) != 1 {
		t.Fatalf("expected 1 unit ID, got %d", len(meta.UnitIDs))
	}
}

func TestEnrich_EmptyContent(t *testing.T) {
	input := validInput()
	input.Content = ""
	_, err := Enrich(input, validConfig())
	if err == nil {
		t.Fatal("expected error for empty content")
	}
}

func TestEnrich_MissingSourceID(t *testing.T) {
	input := validInput()
	input.SourceID = ""
	_, err := Enrich(input, validConfig())
	if err == nil {
		t.Fatal("expected error for missing source_id")
	}
}

func TestEnrich_MissingDomain(t *testing.T) {
	input := validInput()
	input.Domain = ""
	_, err := Enrich(input, validConfig())
	if err == nil {
		t.Fatal("expected error for missing domain")
	}
}

func TestEnrich_InvalidConfidence(t *testing.T) {
	input := validInput()
	input.Confidence = "very-high"
	_, err := Enrich(input, validConfig())
	if err == nil {
		t.Fatal("expected error for invalid confidence")
	}
}

func TestEnrich_EmptyConfidence(t *testing.T) {
	input := validInput()
	input.Confidence = ""
	_, err := Enrich(input, validConfig())
	if err == nil {
		t.Fatal("expected error for empty confidence")
	}
}

func TestEnrich_InvalidPriority(t *testing.T) {
	input := validInput()
	input.Priority = "urgent"
	_, err := Enrich(input, validConfig())
	if err == nil {
		t.Fatal("expected error for invalid priority")
	}
}

func TestEnrich_InvalidStatus(t *testing.T) {
	input := validInput()
	input.Status = "archived"
	_, err := Enrich(input, validConfig())
	if err == nil {
		t.Fatal("expected error for invalid status")
	}
}

func TestEnrich_DeterministicChunkID(t *testing.T) {
	input := validInput()
	cfg := validConfig()
	m1, _ := Enrich(input, cfg)
	m2, _ := Enrich(input, cfg)
	if m1.ChunkID != m2.ChunkID {
		t.Fatalf("expected deterministic chunk ID, got %q and %q", m1.ChunkID, m2.ChunkID)
	}
}

func TestEnrich_DifferentContentDifferentID(t *testing.T) {
	input1 := validInput()
	input2 := validInput()
	input2.Content = "Different content entirely."
	cfg := validConfig()
	m1, _ := Enrich(input1, cfg)
	m2, _ := Enrich(input2, cfg)
	if m1.ChunkID == m2.ChunkID {
		t.Fatal("expected different chunk IDs for different content")
	}
}

func TestEnrich_UnicodeCharCount(t *testing.T) {
	input := validInput()
	input.Content = "café résumé naïve"
	meta, err := Enrich(input, validConfig())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// "café résumé naïve" = 17 runes (including accented chars)
	if meta.CharCount != 17 {
		t.Fatalf("expected 17 rune chars, got %d", meta.CharCount)
	}
}

func TestEnrich_CustomTokenRatio(t *testing.T) {
	input := validInput()
	input.Content = strings.Repeat("a", 100)
	cfg := validConfig()
	cfg.TokenEstimateRatio = 2.0
	meta, err := Enrich(input, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if meta.TokenCount != 50 {
		t.Fatalf("expected 50 tokens with ratio 2.0, got %d", meta.TokenCount)
	}
}

func TestEnrich_NoUnitIDs(t *testing.T) {
	input := validInput()
	input.UnitIDs = nil
	meta, err := Enrich(input, validConfig())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if meta.UnitIDs != nil {
		t.Fatalf("expected nil unit IDs, got %v", meta.UnitIDs)
	}
}

func TestEnrich_NoTags(t *testing.T) {
	input := validInput()
	input.Tags = nil
	meta, err := Enrich(input, validConfig())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if meta.SemanticTags != nil {
		t.Fatalf("expected nil tags, got %v", meta.SemanticTags)
	}
}

func TestEnrich_AllStatuses(t *testing.T) {
	for _, status := range []string{"active", "superseded", "duplicate", "out_of_scope", "needs_review", "blocked"} {
		input := validInput()
		input.Status = status
		if _, err := Enrich(input, validConfig()); err != nil {
			t.Fatalf("status %q should be valid: %v", status, err)
		}
	}
}

func TestEnrich_AllPriorities(t *testing.T) {
	for _, p := range []string{"primary", "secondary", "legacy", "derived", "reference"} {
		input := validInput()
		input.Priority = p
		if _, err := Enrich(input, validConfig()); err != nil {
			t.Fatalf("priority %q should be valid: %v", p, err)
		}
	}
}

func TestEnrich_AllConfidences(t *testing.T) {
	for _, c := range []string{"high", "medium", "low"} {
		input := validInput()
		input.Confidence = c
		if _, err := Enrich(input, validConfig()); err != nil {
			t.Fatalf("confidence %q should be valid: %v", c, err)
		}
	}
}

func TestEnrichBatch_Valid(t *testing.T) {
	inputs := []ChunkInput{validInput(), validInput()}
	inputs[1].Content = "Second chunk."
	results, err := EnrichBatch(inputs, validConfig())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].ChunkID == results[1].ChunkID {
		t.Fatal("expected different chunk IDs")
	}
}

func TestEnrichBatch_StopsOnError(t *testing.T) {
	inputs := []ChunkInput{validInput(), {Content: ""}}
	_, err := EnrichBatch(inputs, validConfig())
	if err == nil {
		t.Fatal("expected error from batch with invalid input")
	}
	if !strings.Contains(err.Error(), "chunk[1]") {
		t.Fatalf("expected error to reference chunk[1], got: %v", err)
	}
}

func TestFilterByConfidence(t *testing.T) {
	chunks := []ChunkMetadata{
		{ChunkID: "a", Confidence: "high"},
		{ChunkID: "b", Confidence: "low"},
		{ChunkID: "c", Confidence: "high"},
	}
	result := FilterByConfidence(chunks, "high")
	if len(result) != 2 {
		t.Fatalf("expected 2 high-confidence chunks, got %d", len(result))
	}
}

func TestFilterByConfidence_NoMatch(t *testing.T) {
	chunks := []ChunkMetadata{{ChunkID: "a", Confidence: "high"}}
	result := FilterByConfidence(chunks, "low")
	if len(result) != 0 {
		t.Fatalf("expected 0 results, got %d", len(result))
	}
}

func TestFilterByTag(t *testing.T) {
	chunks := []ChunkMetadata{
		{ChunkID: "a", SemanticTags: []string{"sinistre", "delai"}},
		{ChunkID: "b", SemanticTags: []string{"prime"}},
		{ChunkID: "c", SemanticTags: []string{"sinistre"}},
	}
	result := FilterByTag(chunks, "sinistre")
	if len(result) != 2 {
		t.Fatalf("expected 2 chunks with tag 'sinistre', got %d", len(result))
	}
}

func TestFilterByTag_CaseInsensitive(t *testing.T) {
	chunks := []ChunkMetadata{
		{ChunkID: "a", SemanticTags: []string{"Sinistre"}},
	}
	result := FilterByTag(chunks, "sinistre")
	if len(result) != 1 {
		t.Fatalf("expected case-insensitive match, got %d", len(result))
	}
}

func TestFilterByTag_NoMatch(t *testing.T) {
	chunks := []ChunkMetadata{
		{ChunkID: "a", SemanticTags: []string{"prime"}},
	}
	result := FilterByTag(chunks, "sinistre")
	if len(result) != 0 {
		t.Fatalf("expected 0 results, got %d", len(result))
	}
}

func TestEstimateTokens(t *testing.T) {
	if estimateTokens(0, 4) != 0 {
		t.Fatal("0 chars should yield 0 tokens")
	}
	if estimateTokens(100, 4) != 25 {
		t.Fatalf("expected 25, got %d", estimateTokens(100, 4))
	}
	if estimateTokens(100, 0) != 0 {
		t.Fatal("0 ratio should yield 0 tokens")
	}
}
