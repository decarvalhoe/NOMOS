package app

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// VRC-10 (#556) — the `answer gate` CLI surface: cites a grounded answer,
// abstains + exits 1 on an ungrounded one.

const groundedFixture = `answers:
  - answer_id: A1
    prompt_id: P1
    answer: "Le gabarit retient une hauteur de neuf metres au faite."
    citation_status: source_backed
    policy_outcome: acceptable
    confidence: 0.99
    source_spans:
      - source_id: S1
        source_hash: "sha256:abc"
        span: L1-L2
        chunk_id: c1
        text: "Le gabarit retient une hauteur de neuf metres au faite pour le volume principal."
    retrieved_chunks:
      - chunk_id: c1
        text: "Le gabarit retient une hauteur de neuf metres au faite."
`

const ungroundedFixture = `answers:
  - answer_id: B1
    prompt_id: P1
    answer: "Une affirmation fabriquee sans aucune source pour la verifier."
    citation_status: source_backed
    policy_outcome: acceptable
    confidence: 0.99
    source_spans: []
    retrieved_chunks: []
`

func writeAnswerFixture(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "answers.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestAnswerGate_CitesGroundedAnswer(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"answer", "gate", "--fixtures", writeAnswerFixture(t, groundedFixture)}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("grounded answer must pass, got %d: %s", code, stderr.String())
	}
	var res map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &res); err != nil {
		t.Fatalf("output not JSON: %v", err)
	}
	if res["status"] != "pass" {
		t.Fatalf("expected pass, got %v", res["status"])
	}
}

func TestAnswerGate_AbstainsAndFailsOnUngrounded(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"answer", "gate", "--fixtures", writeAnswerFixture(t, ungroundedFixture)}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("an ungrounded answer must exit 1, got %d", code)
	}
	var res map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &res); err != nil {
		t.Fatalf("output not JSON: %v", err)
	}
	if res["status"] != "fail" {
		t.Fatalf("expected fail, got %v", res["status"])
	}
}

func TestAnswerGate_RequiresFixtures(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"answer", "gate"}, &stdout, &stderr); code != 2 {
		t.Fatalf("missing --fixtures must exit 2, got %d", code)
	}
}
