package fidelity

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"
)

var envNow = time.Date(2026, 5, 3, 16, 0, 0, 0, time.UTC)

func validOpts() EnvelopeOptions {
	return EnvelopeOptions{
		EnvelopeID:      "ENV-FID-001",
		ArtifactType:    "structure_tree",
		PipelineVersion: "0.1.0",
		Producer:        "nomos-fidelity",
		SourceHash:      "sha256:aabbccdd",
		Inputs: []EnvelopeInput{
			{ID: "input-source", Path: "corpus/doc.md", Hash: "sha256:1111"},
		},
		EnvelopeGates: []EnvelopeGate{
			{GateID: "parse.lossless", Status: "passed", Message: "OK", Severity: "info"},
			{GateID: "hash.stable", Status: "passed", Message: "OK", Severity: "info"},
		},
		Now: envNow,
	}
}

func TestGenerateEnvelopeBasic(t *testing.T) {
	payload := []byte(`{"tree": "data"}`)
	env := GenerateEnvelope(payload, validOpts())

	if env.SchemaVersion != "0.1.0" {
		t.Fatalf("expected 0.1.0, got %s", env.SchemaVersion)
	}
	if env.EnvelopeID != "ENV-FID-001" {
		t.Fatalf("expected ENV-FID-001, got %s", env.EnvelopeID)
	}
	if env.ArtifactType != "structure_tree" {
		t.Fatalf("expected structure_tree, got %s", env.ArtifactType)
	}
	if env.PipelineVersion != "0.1.0" {
		t.Fatalf("expected 0.1.0, got %s", env.PipelineVersion)
	}
	if env.Producer != "nomos-fidelity" {
		t.Fatalf("expected nomos-fidelity, got %s", env.Producer)
	}
	if env.SourceHash != "sha256:aabbccdd" {
		t.Fatalf("expected source hash, got %s", env.SourceHash)
	}
	if env.Status != EnvelopeValid {
		t.Fatalf("expected valid, got %s", env.Status)
	}
}

func TestGenerateEnvelopeContentHash(t *testing.T) {
	payload := []byte(`{"data": "test"}`)
	env := GenerateEnvelope(payload, validOpts())

	if env.ContentHash == "" {
		t.Fatal("expected content hash")
	}
	if env.ContentHash[:7] != "sha256:" {
		t.Fatalf("expected sha256 prefix, got %s", env.ContentHash)
	}
}

func TestGenerateEnvelopeContentHashDeterministic(t *testing.T) {
	payload := []byte(`same content`)
	env1 := GenerateEnvelope(payload, validOpts())
	env2 := GenerateEnvelope(payload, validOpts())

	if env1.ContentHash != env2.ContentHash {
		t.Fatal("expected deterministic content hash")
	}
}

func TestGenerateEnvelopeContentHashChanges(t *testing.T) {
	env1 := GenerateEnvelope([]byte("content A"), validOpts())
	env2 := GenerateEnvelope([]byte("content B"), validOpts())

	if env1.ContentHash == env2.ContentHash {
		t.Fatal("expected different hashes for different content")
	}
}

func TestGenerateEnvelopeStatusFromGates(t *testing.T) {
	// All passed → valid.
	opts := validOpts()
	env := GenerateEnvelope([]byte("x"), opts)
	if env.Status != EnvelopeValid {
		t.Fatalf("expected valid, got %s", env.Status)
	}

	// One failed → invalid.
	opts.EnvelopeGates = append(opts.EnvelopeGates, EnvelopeGate{
		GateID: "check.fail", Status: "failed", Message: "bad", Severity: "high",
	})
	env = GenerateEnvelope([]byte("x"), opts)
	if env.Status != EnvelopeInvalid {
		t.Fatalf("expected invalid, got %s", env.Status)
	}

	// No gates → pending.
	opts.EnvelopeGates = nil
	env = GenerateEnvelope([]byte("x"), opts)
	if env.Status != EnvelopePending {
		t.Fatalf("expected pending, got %s", env.Status)
	}
}

func TestVerifyEnvelopeValid(t *testing.T) {
	env := GenerateEnvelope([]byte("payload"), validOpts())
	errs := VerifyEnvelope(env)
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got %v", errs)
	}
}

func TestVerifyEnvelopeMissingFields(t *testing.T) {
	env := EvidenceEnvelope{}
	errs := VerifyEnvelope(env)
	if len(errs) < 7 {
		t.Fatalf("expected at least 7 errors for empty envelope, got %d: %v", len(errs), errs)
	}
}

func TestVerifyEnvelopeInvalidStatus(t *testing.T) {
	env := GenerateEnvelope([]byte("x"), validOpts())
	env.Status = "bogus"
	errs := VerifyEnvelope(env)
	found := false
	for _, e := range errs {
		if e != "" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected status error")
	}
}

func TestVerifyContentHash(t *testing.T) {
	payload := []byte("verify me")
	env := GenerateEnvelope(payload, validOpts())

	if !VerifyContentHash(env, payload) {
		t.Fatal("expected content hash to verify")
	}
	if VerifyContentHash(env, []byte("tampered")) {
		t.Fatal("expected tampered content to fail verification")
	}
}

func TestChainEnvelopes(t *testing.T) {
	env1 := GenerateEnvelope([]byte("artifact1"), EnvelopeOptions{
		EnvelopeID: "ENV-001", ArtifactType: "parse", PipelineVersion: "0.1.0",
		Producer: "fid", SourceHash: "sha256:aa",
		EnvelopeGates: []EnvelopeGate{{GateID: "g1", Status: "passed"}},
		Now:         envNow,
	})
	env2 := GenerateEnvelope([]byte("artifact2"), EnvelopeOptions{
		EnvelopeID: "ENV-002", ArtifactType: "citations", PipelineVersion: "0.1.0",
		Producer: "fid", SourceHash: "sha256:bb",
		EnvelopeGates: []EnvelopeGate{{GateID: "g2", Status: "passed"}},
		Now:         envNow,
	})

	chained := ChainEnvelopes([]EvidenceEnvelope{env1, env2}, []byte("combined"), EnvelopeOptions{
		EnvelopeID: "ENV-003", ArtifactType: "combined_output", PipelineVersion: "0.1.0",
		Producer: "fid", SourceHash: "sha256:cc",
		EnvelopeGates: []EnvelopeGate{{GateID: "g3", Status: "passed"}},
		Now:         envNow,
	})

	if chained.EnvelopeID != "ENV-003" {
		t.Fatalf("expected ENV-003, got %s", chained.EnvelopeID)
	}
	if len(chained.Inputs) < 2 {
		t.Fatalf("expected at least 2 chained inputs, got %d", len(chained.Inputs))
	}
	// Inputs should reference previous envelopes.
	foundEnv1 := false
	for _, inp := range chained.Inputs {
		if inp.ID == "ENV-001" && inp.Hash == env1.ContentHash {
			foundEnv1 = true
		}
	}
	if !foundEnv1 {
		t.Fatal("expected ENV-001 in chained inputs")
	}
}

func TestWriteEnvelope(t *testing.T) {
	env := GenerateEnvelope([]byte("test"), validOpts())

	var buf bytes.Buffer
	if err := WriteEnvelope(&buf, env); err != nil {
		t.Fatalf("write error: %v", err)
	}

	var decoded EvidenceEnvelope
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if decoded.EnvelopeID != "ENV-FID-001" {
		t.Fatalf("expected ENV-FID-001 after round-trip, got %s", decoded.EnvelopeID)
	}
	if decoded.Status != EnvelopeValid {
		t.Fatalf("expected valid after round-trip, got %s", decoded.Status)
	}
}

func TestEnvelopeTimestamp(t *testing.T) {
	env := GenerateEnvelope([]byte("x"), validOpts())
	if env.GeneratedAt != "2026-05-03T16:00:00Z" {
		t.Fatalf("expected timestamp, got %s", env.GeneratedAt)
	}
}

func TestEnvelopeInputValidation(t *testing.T) {
	env := GenerateEnvelope([]byte("x"), validOpts())
	env.Inputs = append(env.Inputs, EnvelopeInput{ID: "", Hash: ""})
	errs := VerifyEnvelope(env)
	if len(errs) < 2 {
		t.Fatalf("expected input validation errors, got %v", errs)
	}
}
