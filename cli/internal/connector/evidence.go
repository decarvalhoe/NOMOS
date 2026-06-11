package connector

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

// SourceDescriptor documents exactly what an open source is, so evidence is
// self-describing and the no-full-text discipline is auditable.
type SourceDescriptor struct {
	ID          string `json:"id"`
	Jurisdiction string `json:"jurisdiction"`
	Authority   string `json:"authority"`
	Access      string `json:"access"`
	LicenseNote string `json:"license_note"`
	Description string `json:"description"`
}

// KnownSources maps a connector id to its descriptor. These are OPEN Swiss
// sources only — paid norms (SIA/ISO/GAMP) are intentionally excluded.
var KnownSources = map[string]SourceDescriptor{
	"ch-ofs-commune-register": {
		ID:           "ch-ofs-commune-register",
		Jurisdiction: "CH",
		Authority:    "Office fédéral de la statistique (OFS/BFS)",
		Access:       "open_data",
		LicenseNote:  "OFS open government data; redistributed as hash + span coverage only (no full-text copy).",
		Description:  "Historicised register of Swiss communes (CSV).",
	},
	"ch-fedlex-eli": {
		ID:           "ch-fedlex-eli",
		Jurisdiction: "CH",
		Authority:    "Chancellerie fédérale (Fedlex / ELI)",
		Access:       "open_law",
		LicenseNote:  "Fedlex open federal law; redistributed as hash + span coverage only (no full-text copy).",
		Description:  "Swiss federal legislation addressed by ELI.",
	},
	"ch-swisstopo-stac": {
		ID:           "ch-swisstopo-stac",
		Jurisdiction: "CH",
		Authority:    "Office fédéral de topographie (swisstopo)",
		Access:       "open_data",
		LicenseNote:  "swisstopo open geodata (data.geo.admin.ch); redistributed as hash + span coverage only (no full-text copy).",
		Description:  "STAC collection metadata for federal geodata (e.g. swissBUILDINGS3D).",
	},
	"ch-rdppf-oereb": {
		ID:           "ch-rdppf-oereb",
		Jurisdiction: "CH",
		Authority:    "Cadastre RDPPF / ÖREB-Kataster (service cantonal, standard fédéral)",
		Access:       "open_data",
		LicenseNote:  "Official cadastre of public-law restrictions; redistributed as hash + span coverage only (no full-text copy).",
		Description:  "OEREB v2 webservice snapshot (capabilities: themes incl. Nutzungsplanung).",
	},
}

// AtomSample is a small, non-full-text sample of an atomized segment. The
// preview is truncated so evidence never becomes a full-text copy.
type AtomSample struct {
	AtomID      string `json:"atom_id"`
	Line        int    `json:"line"`
	StartByte   int    `json:"start_byte"`
	EndByte     int    `json:"end_byte"`
	ContentHash string `json:"content_hash"`
	Preview     string `json:"preview"`
}

// ConnectorEvidence is the committed, self-describing record of a real fetch.
// It carries the real provenance and the byte-coverage proof — not the source
// text. No full text is redistributed (NoFullText == true).
type ConnectorEvidence struct {
	SchemaVersion string           `json:"schema_version"`
	ConnectorID   string           `json:"connector_id"`
	Source        SourceDescriptor `json:"source"`
	Fetch         FetchResult      `json:"fetch"`
	BodyLedger    BodyLedger       `json:"body_ledger"`
	AtomCount     int              `json:"atom_count"`
	AtomSample    []AtomSample     `json:"atom_sample"`
	NoFullText    bool             `json:"no_full_text"`
	ClaimBoundary string           `json:"claim_boundary"`
}

const evidenceClaimBoundary = "Read-only fetch + real content hash + byte-coverage proof only. " +
	"No full-text redistribution; no correctness, completeness, or licensing claim about the source."

// PreviewMaxRunes bounds the per-atom preview so evidence stays a sample.
const PreviewMaxRunes = 80

// BuildEvidence assembles connector evidence from a real fetch + ledger. sample
// caps how many atom previews are recorded (non-content-bearing or whitespace
// lines are skipped). connectorID resolves a known source descriptor when present.
func BuildEvidence(connectorID string, content []byte, fetch FetchResult, ledger BodyLedger, sample int) ConnectorEvidence {
	desc, ok := KnownSources[connectorID]
	if !ok {
		desc = SourceDescriptor{
			ID:           connectorID,
			Jurisdiction: "unknown",
			Authority:    "unspecified",
			Access:       "unspecified",
			LicenseNote:  "Redistributed as hash + span coverage only (no full-text copy).",
		}
	}

	atomCount := 0
	var samples []AtomSample
	for _, seg := range ledger.Segments {
		text := strings.TrimSpace(string(content[seg.StartByte:seg.EndByte]))
		if text == "" {
			continue // blank/structural line: not a content atom
		}
		atomCount++
		if len(samples) < sample {
			samples = append(samples, AtomSample{
				AtomID:      fmt.Sprintf("CH-ATOM-%04d", seg.Index),
				Line:        seg.Line,
				StartByte:   seg.StartByte,
				EndByte:     seg.EndByte,
				ContentHash: seg.ContentHash,
				Preview:     truncateRunes(text, PreviewMaxRunes),
			})
		}
	}

	return ConnectorEvidence{
		SchemaVersion: EvidenceSchemaVersion,
		ConnectorID:   connectorID,
		Source:        desc,
		Fetch:         fetch,
		BodyLedger:    ledgerWithoutSegments(ledger),
		AtomCount:     atomCount,
		AtomSample:    samples,
		NoFullText:    true,
		ClaimBoundary: evidenceClaimBoundary,
	}
}

// Validate checks the integrity invariants of the evidence: a real (non-synthetic)
// hash and full byte coverage.
func (e ConnectorEvidence) Validate() error {
	if e.SchemaVersion != EvidenceSchemaVersion {
		return fmt.Errorf("connector evidence: wrong schema_version %q", e.SchemaVersion)
	}
	if !strings.HasPrefix(e.Fetch.SHA256, "sha256:") || len(e.Fetch.SHA256) != len("sha256:")+64 {
		return fmt.Errorf("connector evidence: fetch sha256 %q is not a real digest", e.Fetch.SHA256)
	}
	if e.Fetch.ByteCount <= 0 {
		return fmt.Errorf("connector evidence: empty fetch")
	}
	if !e.BodyLedger.IsFullyCovered() {
		return fmt.Errorf("connector evidence: %d byte(s) uncovered by the body ledger", e.BodyLedger.UncoveredBytes)
	}
	return nil
}

// Marshal renders the evidence as indented JSON.
func (e ConnectorEvidence) Marshal() ([]byte, error) {
	return json.MarshalIndent(e, "", "  ")
}

// VerifyContentHash recomputes the sha256 of supplied bytes and checks it against
// the recorded fetch digest — a consumer can re-prove the evidence offline.
func (e ConnectorEvidence) VerifyContentHash(content []byte) error {
	sum := sha256.Sum256(content)
	got := "sha256:" + hex.EncodeToString(sum[:])
	if got != e.Fetch.SHA256 {
		return fmt.Errorf("content hash mismatch: recomputed %s, evidence %s", got, e.Fetch.SHA256)
	}
	return nil
}

// ledgerWithoutSegments keeps the coverage proof but drops the per-line segment
// list so committed evidence stays compact and carries no line-by-line text map.
func ledgerWithoutSegments(l BodyLedger) BodyLedger {
	l.Segments = nil
	return l
}

func truncateRunes(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max]) + "…"
}
