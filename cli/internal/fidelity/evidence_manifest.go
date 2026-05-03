package fidelity

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// EvidenceArtifact describes a single artifact in the evidence pack.
type EvidenceArtifact struct {
	ID          string `json:"id" yaml:"id"`
	Type        string `json:"type" yaml:"type"`
	Path        string `json:"path" yaml:"path"`
	Hash        string `json:"hash" yaml:"hash"`
	Producer    string `json:"producer,omitempty" yaml:"producer,omitempty"`
	GeneratedAt string `json:"generated_at,omitempty" yaml:"generated_at,omitempty"`
	Status      string `json:"status" yaml:"status"` // "present", "missing", "stale"
}

// EvidenceManifest is the top-level evidence pack manifest.
type EvidenceManifest struct {
	SchemaVersion string             `json:"schema_version" yaml:"schema_version"`
	ManifestID    string             `json:"manifest_id" yaml:"manifest_id"`
	GeneratedAt   string             `json:"generated_at" yaml:"generated_at"`
	Domain        string             `json:"domain" yaml:"domain"`
	ManifestHash  string             `json:"manifest_hash" yaml:"manifest_hash"`
	Artifacts     []EvidenceArtifact `json:"artifacts" yaml:"artifacts"`
	GateSummary   ManifestGateSummary `json:"gate_summary" yaml:"gate_summary"`
	Complete      bool               `json:"complete" yaml:"complete"`
}

// ManifestGateSummary aggregates gate results across all artifacts.
type ManifestGateSummary struct {
	TotalArtifacts  int `json:"total_artifacts" yaml:"total_artifacts"`
	PresentCount    int `json:"present_count" yaml:"present_count"`
	MissingCount    int `json:"missing_count" yaml:"missing_count"`
	StaleCount      int `json:"stale_count" yaml:"stale_count"`
}

// ManifestOptions configures manifest generation.
type ManifestOptions struct {
	ManifestID string
	Domain     string
	Now        time.Time
}

// RequiredArtifact defines an expected artifact in the evidence pack.
type RequiredArtifact struct {
	ID       string
	Type     string
	Path     string
	Required bool
}

// DefaultRequiredArtifacts returns the standard evidence pack requirements.
func DefaultRequiredArtifacts() []RequiredArtifact {
	return []RequiredArtifact{
		{ID: "nomos-report", Type: "report", Path: "nomos-report.json", Required: true},
		{ID: "coverage-report", Type: "report", Path: "coverage-report.md", Required: true},
		{ID: "attestation", Type: "attestation", Path: "attestation.json", Required: true},
		{ID: "sbom-spdx", Type: "sbom", Path: "sbom-spdx.json", Required: false},
		{ID: "sbom-cyclonedx", Type: "sbom", Path: "sbom-cyclonedx.json", Required: false},
		{ID: "provenance", Type: "provenance", Path: "provenance.json", Required: false},
		{ID: "gate-results", Type: "gate", Path: "gate-results.json", Required: true},
		{ID: "corpus-feed", Type: "feed", Path: "corpus-feed.json", Required: false},
		{ID: "remediation-backlog", Type: "remediation", Path: "remediation-backlog.json", Required: false},
	}
}

// BuildManifest constructs an evidence manifest from collected artifacts
// and checks completeness against required artifacts.
func BuildManifest(collected []EvidenceArtifact, required []RequiredArtifact, opts ManifestOptions) EvidenceManifest {
	now := opts.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}

	collectedByID := make(map[string]EvidenceArtifact, len(collected))
	for _, a := range collected {
		collectedByID[a.ID] = a
	}

	var artifacts []EvidenceArtifact
	missingCount := 0
	staleCount := 0
	presentCount := 0

	for _, req := range required {
		if art, ok := collectedByID[req.ID]; ok {
			if art.Status == "" {
				art.Status = "present"
			}
			artifacts = append(artifacts, art)
			switch art.Status {
			case "present":
				presentCount++
			case "stale":
				staleCount++
			case "missing":
				missingCount++
			default:
				presentCount++
			}
		} else {
			artifacts = append(artifacts, EvidenceArtifact{
				ID:     req.ID,
				Type:   req.Type,
				Path:   req.Path,
				Status: "missing",
			})
			missingCount++
		}
	}

	// Add any extra collected artifacts not in required list.
	for _, art := range collected {
		found := false
		for _, req := range required {
			if req.ID == art.ID {
				found = true
				break
			}
		}
		if !found {
			if art.Status == "" {
				art.Status = "present"
			}
			artifacts = append(artifacts, art)
			presentCount++
		}
	}

	sort.Slice(artifacts, func(i, j int) bool {
		return artifacts[i].ID < artifacts[j].ID
	})

	complete := isComplete(artifacts, required)
	manifestHash := computeManifestHash(artifacts)

	return EvidenceManifest{
		SchemaVersion: "0.1.0",
		ManifestID:    opts.ManifestID,
		GeneratedAt:   now.Format(time.RFC3339),
		Domain:        opts.Domain,
		ManifestHash:  manifestHash,
		Artifacts:     artifacts,
		GateSummary: ManifestGateSummary{
			TotalArtifacts: len(artifacts),
			PresentCount:   presentCount,
			MissingCount:   missingCount,
			StaleCount:     staleCount,
		},
		Complete: complete,
	}
}

// VerifyManifest checks the manifest for structural completeness.
func VerifyManifest(m EvidenceManifest) []string {
	var errs []string

	if m.ManifestID == "" {
		errs = append(errs, "manifest_id is required")
	}
	if m.GeneratedAt == "" {
		errs = append(errs, "generated_at is required")
	}
	if m.Domain == "" {
		errs = append(errs, "domain is required")
	}
	if m.ManifestHash == "" {
		errs = append(errs, "manifest_hash is required")
	}
	if len(m.Artifacts) == 0 {
		errs = append(errs, "at least one artifact is required")
	}

	ids := map[string]bool{}
	for i, a := range m.Artifacts {
		if a.ID == "" {
			errs = append(errs, fmt.Sprintf("artifacts[%d].id is required", i))
		} else if ids[a.ID] {
			errs = append(errs, fmt.Sprintf("artifacts[%d].id %q is duplicated", i, a.ID))
		} else {
			ids[a.ID] = true
		}
		if a.Status != "present" && a.Status != "missing" && a.Status != "stale" {
			errs = append(errs, fmt.Sprintf("artifacts[%d].status %q must be present, missing, or stale", i, a.Status))
		}
	}

	return errs
}

// VerifyManifestHash recomputes and checks the manifest hash.
func VerifyManifestHash(m EvidenceManifest) bool {
	return m.ManifestHash == computeManifestHash(m.Artifacts)
}

// WriteManifestJSON writes the manifest as indented JSON.
func WriteManifestJSON(w io.Writer, m EvidenceManifest) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(m)
}

// WriteManifestYAMLFidelity writes the manifest as YAML.
func WriteManifestYAMLFidelity(w io.Writer, m EvidenceManifest) error {
	enc := yaml.NewEncoder(w)
	enc.SetIndent(2)
	defer enc.Close()
	return enc.Encode(m)
}

func isComplete(artifacts []EvidenceArtifact, required []RequiredArtifact) bool {
	requiredIDs := map[string]bool{}
	for _, req := range required {
		if req.Required {
			requiredIDs[req.ID] = true
		}
	}

	for _, art := range artifacts {
		if requiredIDs[art.ID] && art.Status != "present" {
			return false
		}
	}

	// Check all required are present in artifacts.
	artIDs := map[string]bool{}
	for _, art := range artifacts {
		artIDs[art.ID] = true
	}
	for id := range requiredIDs {
		if !artIDs[id] {
			return false
		}
	}

	return true
}

func computeManifestHash(artifacts []EvidenceArtifact) string {
	var parts []string
	for _, a := range artifacts {
		parts = append(parts, a.ID+":"+a.Hash+":"+a.Status)
	}
	sort.Strings(parts)

	h := sha256.New()
	for _, p := range parts {
		h.Write([]byte(p))
		h.Write([]byte{'\n'})
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}

// MissingRequired returns IDs of required artifacts that are not present.
func MissingRequired(m EvidenceManifest, required []RequiredArtifact) []string {
	artStatus := map[string]string{}
	for _, a := range m.Artifacts {
		artStatus[a.ID] = a.Status
	}

	var missing []string
	for _, req := range required {
		if !req.Required {
			continue
		}
		status, ok := artStatus[req.ID]
		if !ok || status != "present" {
			missing = append(missing, req.ID)
		}
	}
	sort.Strings(missing)
	return missing
}

// FormatSummary returns a human-readable summary.
func (m EvidenceManifest) FormatSummary() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Evidence manifest: %s\n", m.ManifestID)
	fmt.Fprintf(&b, "  Domain: %s\n", m.Domain)
	fmt.Fprintf(&b, "  Artifacts: %d total, %d present, %d missing, %d stale\n",
		m.GateSummary.TotalArtifacts, m.GateSummary.PresentCount,
		m.GateSummary.MissingCount, m.GateSummary.StaleCount)
	fmt.Fprintf(&b, "  Complete: %t\n", m.Complete)
	return b.String()
}
