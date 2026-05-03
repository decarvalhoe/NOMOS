package fidelity

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"time"
)

// EvidenceEnvelope wraps a fidelity pipeline artifact with full provenance.
type EvidenceEnvelope struct {
	SchemaVersion   string          `json:"schema_version"`
	EnvelopeID      string          `json:"envelope_id"`
	ArtifactType    string          `json:"artifact_type"`
	GeneratedAt     string          `json:"generated_at"`
	SourceHash      string          `json:"source_hash"`
	ContentHash     string          `json:"content_hash"`
	PipelineVersion string          `json:"pipeline_version"`
	Producer        string          `json:"producer"`
	EnvelopeGates     []EnvelopeGate    `json:"gate_results"`
	Inputs          []EnvelopeInput `json:"inputs"`
	Status          EnvelopeStatus  `json:"status"`
	Metadata        map[string]any  `json:"metadata,omitempty"`
}

// EnvelopeInput describes one input consumed to produce the artifact.
type EnvelopeInput struct {
	ID   string `json:"id"`
	Path string `json:"path"`
	Hash string `json:"hash"`
}

// EnvelopeGate is a pass/fail check result embedded in the envelope.
type EnvelopeGate struct {
	GateID   string `json:"gate_id"`
	Status   string `json:"status"` // "passed", "failed", "skipped"
	Message  string `json:"message"`
	Severity string `json:"severity"`
}

// EnvelopeStatus indicates the overall validity of the envelope.
type EnvelopeStatus string

const (
	EnvelopeValid   EnvelopeStatus = "valid"
	EnvelopeInvalid EnvelopeStatus = "invalid"
	EnvelopePending EnvelopeStatus = "pending"
)

// EnvelopeOptions configures envelope generation.
type EnvelopeOptions struct {
	EnvelopeID      string
	ArtifactType    string
	PipelineVersion string
	Producer        string
	SourceHash      string
	Inputs          []EnvelopeInput
	EnvelopeGates     []EnvelopeGate
	Metadata        map[string]any
	Now             time.Time
}

// GenerateEnvelope creates an evidence envelope for a fidelity artifact.
// The content hash is computed from the artifact payload bytes.
func GenerateEnvelope(artifactPayload []byte, opts EnvelopeOptions) EvidenceEnvelope {
	now := opts.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}

	contentHash := computeContentHash(artifactPayload)
	status := deriveStatus(opts.EnvelopeGates)

	return EvidenceEnvelope{
		SchemaVersion:   "0.1.0",
		EnvelopeID:      opts.EnvelopeID,
		ArtifactType:    opts.ArtifactType,
		GeneratedAt:     now.Format(time.RFC3339),
		SourceHash:      opts.SourceHash,
		ContentHash:     contentHash,
		PipelineVersion: opts.PipelineVersion,
		Producer:        opts.Producer,
		EnvelopeGates:     opts.EnvelopeGates,
		Inputs:          opts.Inputs,
		Status:          status,
		Metadata:        opts.Metadata,
	}
}

// WriteEnvelope writes the envelope as indented JSON.
func WriteEnvelope(w io.Writer, env EvidenceEnvelope) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(env)
}

// VerifyEnvelope checks that an envelope is structurally valid and consistent.
func VerifyEnvelope(env EvidenceEnvelope) []string {
	var errs []string

	if env.EnvelopeID == "" {
		errs = append(errs, "envelope_id is required")
	}
	if env.ArtifactType == "" {
		errs = append(errs, "artifact_type is required")
	}
	if env.GeneratedAt == "" {
		errs = append(errs, "generated_at is required")
	}
	if env.SourceHash == "" {
		errs = append(errs, "source_hash is required")
	}
	if env.ContentHash == "" {
		errs = append(errs, "content_hash is required")
	}
	if env.PipelineVersion == "" {
		errs = append(errs, "pipeline_version is required")
	}
	if env.Producer == "" {
		errs = append(errs, "producer is required")
	}
	if env.Status == "" {
		errs = append(errs, "status is required")
	} else if env.Status != EnvelopeValid && env.Status != EnvelopeInvalid && env.Status != EnvelopePending {
		errs = append(errs, fmt.Sprintf("status %q must be valid, invalid, or pending", env.Status))
	}

	for i, input := range env.Inputs {
		if input.ID == "" {
			errs = append(errs, fmt.Sprintf("inputs[%d].id is required", i))
		}
		if input.Hash == "" {
			errs = append(errs, fmt.Sprintf("inputs[%d].hash is required", i))
		}
	}

	return errs
}

// VerifyContentHash checks that the envelope's content_hash matches the given payload.
func VerifyContentHash(env EvidenceEnvelope, payload []byte) bool {
	return env.ContentHash == computeContentHash(payload)
}

// ChainEnvelopes creates a new envelope whose inputs are previous envelopes.
func ChainEnvelopes(envelopes []EvidenceEnvelope, artifactPayload []byte, opts EnvelopeOptions) EvidenceEnvelope {
	inputs := make([]EnvelopeInput, 0, len(envelopes))
	for _, env := range envelopes {
		inputs = append(inputs, EnvelopeInput{
			ID:   env.EnvelopeID,
			Path: env.ArtifactType,
			Hash: env.ContentHash,
		})
	}
	sort.Slice(inputs, func(i, j int) bool {
		return inputs[i].ID < inputs[j].ID
	})

	opts.Inputs = append(inputs, opts.Inputs...)
	return GenerateEnvelope(artifactPayload, opts)
}

func computeContentHash(data []byte) string {
	h := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(h[:])
}

func deriveStatus(gates []EnvelopeGate) EnvelopeStatus {
	if len(gates) == 0 {
		return EnvelopePending
	}
	for _, g := range gates {
		if g.Status == "failed" {
			return EnvelopeInvalid
		}
	}
	return EnvelopeValid
}
