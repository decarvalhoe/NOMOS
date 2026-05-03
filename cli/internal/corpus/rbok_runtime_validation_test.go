package corpus

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"
)

var valNow = time.Date(2026, 5, 3, 14, 0, 0, 0, time.UTC)

func completeInput() ValidationInput {
	feed := &RuntimeFeed{Format: RuntimeFeedFormat, NodeCount: 5, LayerCount: 2}
	return ValidationInput{
		Feed:          feed,
		NodeCount:     5,
		HasGovernance: true,
		HasRAGMeta:    true,
		HasIndex:      true,
		HasEngine:     true,
		LayerCount:    2,
		ContentHash:   "sha256:abcdef",
	}
}

func TestValidateRuntimeOutputAllPassed(t *testing.T) {
	pack := ValidateRuntimeOutput(completeInput(), ValidationOptions{Now: valNow})

	if pack.Status != ValidationPassed {
		t.Fatalf("expected passed, got %s", pack.Status)
	}
	if pack.Summary.Failed != 0 {
		t.Fatalf("expected 0 failed, got %d", pack.Summary.Failed)
	}
	if pack.Summary.TotalChecks != 8 {
		t.Fatalf("expected 8 checks, got %d", pack.Summary.TotalChecks)
	}
	if pack.Summary.Passed != 8 {
		t.Fatalf("expected 8 passed, got %d", pack.Summary.Passed)
	}
}

func TestValidateRuntimeOutputMissingFeed(t *testing.T) {
	input := completeInput()
	input.Feed = nil
	pack := ValidateRuntimeOutput(input, ValidationOptions{Now: valNow})

	if pack.Status != ValidationFailed {
		t.Fatalf("expected failed, got %s", pack.Status)
	}
	assertCheckStatus(t, pack, "output.feed_present", ValidationFailed)
}

func TestValidateRuntimeOutputBelowMinNodes(t *testing.T) {
	input := completeInput()
	input.NodeCount = 0
	pack := ValidateRuntimeOutput(input, ValidationOptions{MinNodes: 1, Now: valNow})

	if pack.Status != ValidationFailed {
		t.Fatalf("expected failed, got %s", pack.Status)
	}
	assertCheckStatus(t, pack, "output.min_nodes", ValidationFailed)
}

func TestValidateRuntimeOutputCustomMinNodes(t *testing.T) {
	input := completeInput()
	input.NodeCount = 3
	pack := ValidateRuntimeOutput(input, ValidationOptions{MinNodes: 10, Now: valNow})

	assertCheckStatus(t, pack, "output.min_nodes", ValidationFailed)
}

func TestValidateRuntimeOutputNoGovernance(t *testing.T) {
	input := completeInput()
	input.HasGovernance = false
	pack := ValidateRuntimeOutput(input, ValidationOptions{Now: valNow})

	if pack.Status != ValidationFailed {
		t.Fatalf("expected failed, got %s", pack.Status)
	}
	assertCheckStatus(t, pack, "output.governance", ValidationFailed)
}

func TestValidateRuntimeOutputNoRAGRequired(t *testing.T) {
	input := completeInput()
	input.HasRAGMeta = false
	pack := ValidateRuntimeOutput(input, ValidationOptions{RequireRAG: true, Now: valNow})

	if pack.Status != ValidationFailed {
		t.Fatalf("expected failed when RAG required, got %s", pack.Status)
	}
	assertCheckStatus(t, pack, "output.rag_metadata", ValidationFailed)
}

func TestValidateRuntimeOutputNoRAGOptional(t *testing.T) {
	input := completeInput()
	input.HasRAGMeta = false
	pack := ValidateRuntimeOutput(input, ValidationOptions{RequireRAG: false, Now: valNow})

	// Warning, not failure.
	if pack.Status == ValidationFailed {
		t.Fatal("expected non-failure when RAG optional")
	}
	assertCheckStatus(t, pack, "output.rag_metadata", ValidationWarning)
}

func TestValidateRuntimeOutputNoEngine(t *testing.T) {
	input := completeInput()
	input.HasEngine = false
	pack := ValidateRuntimeOutput(input, ValidationOptions{RequireEngine: true, Now: valNow})

	assertCheckStatus(t, pack, "output.engine_import", ValidationFailed)
}

func TestValidateRuntimeOutputNoIndex(t *testing.T) {
	input := completeInput()
	input.HasIndex = false
	pack := ValidateRuntimeOutput(input, ValidationOptions{RequireIndex: true, Now: valNow})

	assertCheckStatus(t, pack, "output.index", ValidationFailed)
}

func TestValidateRuntimeOutputNoContentHash(t *testing.T) {
	input := completeInput()
	input.ContentHash = ""
	pack := ValidateRuntimeOutput(input, ValidationOptions{Now: valNow})

	// Warning only.
	assertCheckStatus(t, pack, "output.content_hash", ValidationWarning)
	if pack.Status == ValidationFailed {
		t.Fatal("missing content hash should be warning, not failure")
	}
}

func TestValidateRuntimeOutputNoLayers(t *testing.T) {
	input := completeInput()
	input.LayerCount = 0
	pack := ValidateRuntimeOutput(input, ValidationOptions{Now: valNow})

	assertCheckStatus(t, pack, "output.layers", ValidationFailed)
}

func TestValidateRuntimeOutputDefaultOptions(t *testing.T) {
	opts := DefaultValidationOptions()
	if opts.MinNodes != 1 {
		t.Fatalf("expected min nodes 1, got %d", opts.MinNodes)
	}
	if !opts.RequireRAG {
		t.Fatal("expected RequireRAG true")
	}
	if !opts.RequireEngine {
		t.Fatal("expected RequireEngine true")
	}
	if !opts.RequireIndex {
		t.Fatal("expected RequireIndex true")
	}
}

func TestValidateRuntimeOutputSchemaVersion(t *testing.T) {
	pack := ValidateRuntimeOutput(completeInput(), ValidationOptions{Now: valNow})
	if pack.SchemaVersion != "0.1.0" {
		t.Fatalf("expected 0.1.0, got %s", pack.SchemaVersion)
	}
}

func TestWriteValidationPackJSON(t *testing.T) {
	pack := ValidateRuntimeOutput(completeInput(), ValidationOptions{Now: valNow})

	var buf bytes.Buffer
	if err := WriteValidationPack(&buf, pack); err != nil {
		t.Fatalf("write error: %v", err)
	}

	var decoded ValidationPack
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if decoded.Status != ValidationPassed {
		t.Fatalf("expected passed after round-trip, got %s", decoded.Status)
	}
	if decoded.Summary.TotalChecks != 8 {
		t.Fatalf("expected 8 checks, got %d", decoded.Summary.TotalChecks)
	}
}

func TestValidateRuntimeOutputMultipleFailures(t *testing.T) {
	input := ValidationInput{
		Feed:          nil,
		NodeCount:     0,
		HasGovernance: false,
		HasRAGMeta:    false,
		HasIndex:      false,
		HasEngine:     false,
		LayerCount:    0,
		ContentHash:   "",
	}
	pack := ValidateRuntimeOutput(input, ValidationOptions{
		RequireRAG: true, RequireEngine: true, RequireIndex: true, Now: valNow,
	})

	if pack.Status != ValidationFailed {
		t.Fatalf("expected failed, got %s", pack.Status)
	}
	if pack.Summary.Failed < 6 {
		t.Fatalf("expected at least 6 failures, got %d", pack.Summary.Failed)
	}
}

func assertCheckStatus(t *testing.T, pack ValidationPack, checkID string, expected ValidationGateStatus) {
	t.Helper()
	for _, c := range pack.Checks {
		if c.ID == checkID {
			if c.Status != expected {
				t.Fatalf("check %s: expected %s, got %s (%s)", checkID, expected, c.Status, c.Message)
			}
			return
		}
	}
	t.Fatalf("check %s not found", checkID)
}
