package fidelity

import (
	"fmt"
	"strings"
	"testing"
)

const conformanceFixture = `# Title

First paragraph of content here.

## Section A

Section A has important rules.

- Rule one applies.
- Rule two applies.

## Section B

Section B describes conditions.
`

func testAdapter(source string) ([]PortableAtom, error) {
	lines := strings.Split(source, "\n")
	var atoms []PortableAtom
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		atomType := "paragraph"
		if strings.HasPrefix(trimmed, "#") {
			atomType = "heading"
		} else if strings.HasPrefix(trimmed, "- ") {
			atomType = "list_item"
			trimmed = strings.TrimPrefix(trimmed, "- ")
		}
		atoms = append(atoms, PortableAtom{
			ID:           fmt.Sprintf("ATOM-%d", i+1),
			CanonicalRef: fmt.Sprintf("ref-%d", i+1),
			Type:         atomType,
			Text:         trimmed,
			ContentHash:  computeSHA256(trimmed),
			Depth:        0,
			SourceLine:   i + 1,
		})
	}
	return atoms, nil
}

func testRoundtrip(atoms []PortableAtom) (string, error) {
	var sb strings.Builder
	for _, atom := range atoms {
		sb.WriteString(atom.Text)
		sb.WriteByte('\n')
	}
	return sb.String(), nil
}

func TestRunConformanceAllPass(t *testing.T) {
	result, err := RunConformance(HarnessConfig{
		AdapterID:     "test-adapter",
		Source:        conformanceFixture,
		Adapter:       testAdapter,
		Roundtrip:     testRoundtrip,
		LossThreshold: 0.5,
	})
	if err != nil {
		t.Fatalf("harness error: %v", err)
	}
	if !result.Pass {
		for _, r := range result.Results {
			if !r.Pass {
				t.Logf("FAILED: %s — %s (%s)", r.Check, r.Message, r.Detail)
			}
		}
		t.Fatalf("expected pass, got %d/%d failed", result.Failed, result.TotalChecks)
	}
	if result.TotalChecks < 7 {
		t.Fatalf("expected at least 7 checks, got %d", result.TotalChecks)
	}
}

func TestRunConformanceDetectsLosslessFailure(t *testing.T) {
	// Adapter that drops most content.
	sparseAdapter := func(source string) ([]PortableAtom, error) {
		return []PortableAtom{
			{ID: "A1", CanonicalRef: "r1", Text: "x", ContentHash: computeSHA256("x"), SourceLine: 1},
		}, nil
	}

	result, _ := RunConformance(HarnessConfig{
		AdapterID:     "sparse",
		Source:        conformanceFixture,
		Adapter:       sparseAdapter,
		LossThreshold: 0.8,
	})

	found := findCheck(result, CheckLosslessness)
	if found.Pass {
		t.Fatal("expected losslessness to fail for sparse adapter")
	}
}

func TestRunConformanceDetectsHashMismatch(t *testing.T) {
	badHashAdapter := func(source string) ([]PortableAtom, error) {
		return []PortableAtom{
			{ID: "A1", CanonicalRef: "r1", Text: "hello", ContentHash: "sha256:wrong", SourceLine: 1},
		}, nil
	}

	result, _ := RunConformance(HarnessConfig{
		AdapterID: "bad-hash",
		Source:    "hello\n",
		Adapter:   badHashAdapter,
	})

	found := findCheck(result, CheckHashIntegrity)
	if found.Pass {
		t.Fatal("expected hash integrity to fail")
	}
}

func TestRunConformanceDetectsDuplicateIDs(t *testing.T) {
	dupAdapter := func(source string) ([]PortableAtom, error) {
		return []PortableAtom{
			{ID: "DUP", CanonicalRef: "r1", Text: "one", ContentHash: computeSHA256("one"), SourceLine: 1},
			{ID: "DUP", CanonicalRef: "r2", Text: "two", ContentHash: computeSHA256("two"), SourceLine: 2},
		}, nil
	}

	result, _ := RunConformance(HarnessConfig{
		AdapterID: "dup",
		Source:    "one\ntwo\n",
		Adapter:   dupAdapter,
	})

	found := findCheck(result, CheckIDUniqueness)
	if found.Pass {
		t.Fatal("expected ID uniqueness to fail")
	}
}

func TestRunConformanceDetectsOrphanParent(t *testing.T) {
	orphanAdapter := func(source string) ([]PortableAtom, error) {
		return []PortableAtom{
			{ID: "A1", CanonicalRef: "r1", Text: "child", ContentHash: computeSHA256("child"), ParentID: "GHOST", SourceLine: 1},
		}, nil
	}

	result, _ := RunConformance(HarnessConfig{
		AdapterID: "orphan",
		Source:    "child\n",
		Adapter:   orphanAdapter,
	})

	found := findCheck(result, CheckParentValidity)
	if found.Pass {
		t.Fatal("expected parent validity to fail")
	}
}

func TestRunConformanceDetectsDepthViolation(t *testing.T) {
	deepAdapter := func(source string) ([]PortableAtom, error) {
		return []PortableAtom{
			{ID: "A1", CanonicalRef: "r1", Text: "deep", ContentHash: computeSHA256("deep"), Depth: 10, SourceLine: 1},
		}, nil
	}

	result, _ := RunConformance(HarnessConfig{
		AdapterID: "deep",
		Source:    "deep\n",
		Adapter:   deepAdapter,
		MaxDepth:  6,
	})

	found := findCheck(result, CheckDepthBounds)
	if found.Pass {
		t.Fatal("expected depth bounds to fail")
	}
}

func TestRunConformanceDetectsMissingRef(t *testing.T) {
	noRefAdapter := func(source string) ([]PortableAtom, error) {
		return []PortableAtom{
			{ID: "A1", CanonicalRef: "", Text: "no ref", ContentHash: computeSHA256("no ref"), SourceLine: 1},
		}, nil
	}

	result, _ := RunConformance(HarnessConfig{
		AdapterID: "no-ref",
		Source:    "no ref\n",
		Adapter:   noRefAdapter,
	})

	found := findCheck(result, CheckRefPresence)
	if found.Pass {
		t.Fatal("expected ref presence to fail")
	}
}

func TestRunConformanceRoundtripSkipped(t *testing.T) {
	result, _ := RunConformance(HarnessConfig{
		AdapterID: "no-roundtrip",
		Source:    "hello\n",
		Adapter:   testAdapter,
		Roundtrip: nil, // skip
	})

	for _, r := range result.Results {
		if r.Check == CheckRoundtrip {
			t.Fatal("roundtrip check should not appear when Roundtrip is nil")
		}
	}
}

func TestRunConformanceAdapterError(t *testing.T) {
	errAdapter := func(source string) ([]PortableAtom, error) {
		return nil, fmt.Errorf("parse failed")
	}

	result, _ := RunConformance(HarnessConfig{
		AdapterID: "err",
		Source:    "content\n",
		Adapter:   errAdapter,
	})

	if result.Pass {
		t.Fatal("expected fail on adapter error")
	}
}

func TestRunConformanceRequiresAdapter(t *testing.T) {
	_, err := RunConformance(HarnessConfig{Source: "x"})
	if err == nil {
		t.Fatal("expected error when no adapter")
	}
}

func TestRunConformanceRequiresSource(t *testing.T) {
	_, err := RunConformance(HarnessConfig{Adapter: testAdapter})
	if err == nil {
		t.Fatal("expected error when no source")
	}
}

func TestRunConformanceResultCounts(t *testing.T) {
	result, _ := RunConformance(HarnessConfig{
		AdapterID: "test",
		Source:    conformanceFixture,
		Adapter:   testAdapter,
		Roundtrip: testRoundtrip,
	})

	if result.Passed+result.Failed != result.TotalChecks {
		t.Fatalf("passed(%d) + failed(%d) != total(%d)", result.Passed, result.Failed, result.TotalChecks)
	}
}

func findCheck(result HarnessResult, check ConformanceCheck) ConformanceResult {
	for _, r := range result.Results {
		if r.Check == check {
			return r
		}
	}
	return ConformanceResult{Check: check, Pass: true, Message: "not found"}
}
