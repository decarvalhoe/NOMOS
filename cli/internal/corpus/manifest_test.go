package corpus

import (
	"bytes"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func sampleSnapshot() Snapshot {
	return Snapshot{
		Format:     "nomos.corpus-snapshot.v1",
		CorpusRoot: "/data/insurance-sources",
		TotalFiles: 3,
		TotalBytes: 1024,
		Sources: []SourceEntry{
			{Path: "sources/contracts/home-2026.pdf", Hash: "sha256:aabbcc", SizeBytes: 500, Extension: ".pdf"},
			{Path: "sources/contracts/auto-2026.pdf", Hash: "sha256:ddeeff", SizeBytes: 400, Extension: ".pdf"},
			{Path: "sources/tables/rates.csv", Hash: "sha256:112233", SizeBytes: 124, Extension: ".csv"},
		},
	}
}

func TestGenerateManifestDefaults(t *testing.T) {
	m := GenerateManifest(sampleSnapshot(), ManifestOptions{
		Domain: "insurance",
	})
	if m.SchemaVersion != "0.1.0" {
		t.Fatalf("expected 0.1.0, got %q", m.SchemaVersion)
	}
	if len(m.Sources) != 3 {
		t.Fatalf("expected 3 sources, got %d", len(m.Sources))
	}
	for _, src := range m.Sources {
		if src.Owner != "unknown" {
			t.Fatalf("expected default owner 'unknown', got %q", src.Owner)
		}
		if src.Priority != "primary" {
			t.Fatalf("expected default priority 'primary', got %q", src.Priority)
		}
		if src.Confidentiality != "internal" {
			t.Fatalf("expected default confidentiality 'internal', got %q", src.Confidentiality)
		}
		if src.Status != "active" {
			t.Fatalf("expected status 'active', got %q", src.Status)
		}
		if len(src.AllowedUses) != 2 {
			t.Fatalf("expected 2 default allowed_uses, got %d", len(src.AllowedUses))
		}
		if src.Domain != "insurance" {
			t.Fatalf("expected domain 'insurance', got %q", src.Domain)
		}
	}
}

func TestGenerateManifestCustomOptions(t *testing.T) {
	m := GenerateManifest(sampleSnapshot(), ManifestOptions{
		Domain:          "finance",
		Owner:           "cfo@example.com",
		License:         "proprietary",
		Confidentiality: "restricted",
		Priority:        "secondary",
		AllowedUses:     []string{"human_review_only"},
		IDPrefix:        "FIN",
	})
	src := m.Sources[0]
	if src.Domain != "finance" {
		t.Fatalf("expected finance, got %q", src.Domain)
	}
	if src.Owner != "cfo@example.com" {
		t.Fatalf("expected cfo@example.com, got %q", src.Owner)
	}
	if src.License != "proprietary" {
		t.Fatalf("expected proprietary, got %q", src.License)
	}
	if src.Confidentiality != "restricted" {
		t.Fatalf("expected restricted, got %q", src.Confidentiality)
	}
	if src.Priority != "secondary" {
		t.Fatalf("expected secondary, got %q", src.Priority)
	}
	if len(src.AllowedUses) != 1 || src.AllowedUses[0] != "human_review_only" {
		t.Fatalf("expected [human_review_only], got %v", src.AllowedUses)
	}
	if !strings.HasPrefix(src.ID, "FIN-") {
		t.Fatalf("expected FIN- prefix, got %q", src.ID)
	}
}

func TestGenerateManifestHashesPreserved(t *testing.T) {
	m := GenerateManifest(sampleSnapshot(), ManifestOptions{Domain: "test"})
	if m.Sources[0].Hash != "sha256:aabbcc" {
		t.Fatalf("expected sha256:aabbcc, got %q", m.Sources[0].Hash)
	}
	if m.Sources[2].Hash != "sha256:112233" {
		t.Fatalf("expected sha256:112233, got %q", m.Sources[2].Hash)
	}
}

func TestGenerateManifestPathsPreserved(t *testing.T) {
	m := GenerateManifest(sampleSnapshot(), ManifestOptions{Domain: "test"})
	if m.Sources[0].Path != "sources/contracts/home-2026.pdf" {
		t.Fatalf("expected original path, got %q", m.Sources[0].Path)
	}
}

func TestGenerateManifestTypeInference(t *testing.T) {
	m := GenerateManifest(sampleSnapshot(), ManifestOptions{Domain: "test"})
	if m.Sources[0].Type != "pdf" {
		t.Fatalf("expected pdf, got %q", m.Sources[0].Type)
	}
	if m.Sources[2].Type != "csv" {
		t.Fatalf("expected csv, got %q", m.Sources[2].Type)
	}
}

func TestGenerateManifestIDFormat(t *testing.T) {
	m := GenerateManifest(sampleSnapshot(), ManifestOptions{Domain: "test", IDPrefix: "INS"})
	for _, src := range m.Sources {
		if !strings.HasPrefix(src.ID, "INS-") {
			t.Fatalf("expected INS- prefix, got %q", src.ID)
		}
		// Verify uppercase pattern
		for _, r := range src.ID {
			if !((r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-') {
				t.Fatalf("ID %q contains invalid char %q", src.ID, string(r))
			}
		}
	}
}

func TestGenerateManifestDisambiguatesDuplicateBasenames(t *testing.T) {
	snapshot := Snapshot{
		Sources: []SourceEntry{
			{Path: "01_rbok/README.md", Hash: "sha256:aaaa", Extension: ".md"},
			{Path: "04_marketing/README.md", Hash: "sha256:bbbb", Extension: ".md"},
		},
	}
	m := GenerateManifest(snapshot, ManifestOptions{Domain: "rbok", IDPrefix: "RBOK"})

	if m.Sources[0].ID != "RBOK-README" {
		t.Fatalf("expected first duplicate to keep stable base ID, got %q", m.Sources[0].ID)
	}
	if m.Sources[1].ID == "RBOK-README" {
		t.Fatalf("expected second duplicate to be disambiguated, got %q", m.Sources[1].ID)
	}
	if !strings.HasPrefix(m.Sources[1].ID, "RBOK-README-") {
		t.Fatalf("expected path-hash suffix, got %q", m.Sources[1].ID)
	}
}

func TestGenerateManifestDefaultPrefix(t *testing.T) {
	m := GenerateManifest(sampleSnapshot(), ManifestOptions{Domain: "test"})
	if !strings.HasPrefix(m.Sources[0].ID, "CORPUS-") {
		t.Fatalf("expected CORPUS- default prefix, got %q", m.Sources[0].ID)
	}
}

func TestWriteManifestYAML(t *testing.T) {
	m := GenerateManifest(sampleSnapshot(), ManifestOptions{Domain: "insurance", IDPrefix: "INS"})
	var buf bytes.Buffer
	if err := WriteManifestYAML(&buf, m); err != nil {
		t.Fatalf("write yaml: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "schema_version:") {
		t.Fatalf("expected schema_version in YAML")
	}
	if !strings.Contains(output, "sources:") {
		t.Fatalf("expected sources in YAML")
	}
	if !strings.Contains(output, "sha256:aabbcc") {
		t.Fatalf("expected hash in YAML output")
	}

	// Verify it round-trips
	var decoded SidecarManifest
	if err := yaml.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("decode yaml: %v", err)
	}
	if decoded.SchemaVersion != "0.1.0" {
		t.Fatalf("expected 0.1.0, got %q", decoded.SchemaVersion)
	}
	if len(decoded.Sources) != 3 {
		t.Fatalf("expected 3 sources, got %d", len(decoded.Sources))
	}
}

func TestWriteManifestYAMLMatchesSchema(t *testing.T) {
	m := GenerateManifest(sampleSnapshot(), ManifestOptions{
		Domain:          "insurance",
		Owner:           "actuarial@example.com",
		Confidentiality: "restricted",
		IDPrefix:        "INS",
	})
	var buf bytes.Buffer
	if err := WriteManifestYAML(&buf, m); err != nil {
		t.Fatal(err)
	}
	output := buf.String()
	// All required CUE fields must be present
	for _, field := range []string{"id:", "path:", "type:", "domain:", "priority:", "status:", "hash:", "owner:", "license:", "confidentiality:", "allowed_uses:"} {
		if !strings.Contains(output, field) {
			t.Fatalf("expected field %q in YAML output", field)
		}
	}
}

func TestGenerateManifestEmptySnapshot(t *testing.T) {
	m := GenerateManifest(Snapshot{}, ManifestOptions{Domain: "test"})
	if len(m.Sources) != 0 {
		t.Fatalf("expected 0 sources, got %d", len(m.Sources))
	}
	if m.SchemaVersion != "0.1.0" {
		t.Fatalf("expected 0.1.0, got %q", m.SchemaVersion)
	}
}

func TestInferSourceTypeVariousExtensions(t *testing.T) {
	cases := map[string]string{
		".pdf":  "pdf",
		".md":   "markdown",
		".html": "html",
		".csv":  "csv",
		".xlsx": "spreadsheet",
		".png":  "image",
		".mp3":  "audio",
		".sql":  "database_export",
		".go":   "source_code",
		".py":   "source_code",
		".xyz":  "source_code", // unknown defaults to source_code
	}
	for ext, expected := range cases {
		got := inferSourceType(ext)
		if got != expected {
			t.Errorf("inferSourceType(%q) = %q, want %q", ext, got, expected)
		}
	}
}

func TestEndToEndScanToManifest(t *testing.T) {
	snap, err := Scan(fixtureRoot(), ScanOptions{})
	if err != nil {
		t.Fatal(err)
	}
	m := GenerateManifest(snap, ManifestOptions{
		Domain:          "insurance",
		Owner:           "actuarial@example.com",
		Confidentiality: "restricted",
		IDPrefix:        "INS",
	})
	if len(m.Sources) != snap.TotalFiles {
		t.Fatalf("expected %d sources, got %d", snap.TotalFiles, len(m.Sources))
	}
	for i, src := range m.Sources {
		if src.Hash != snap.Sources[i].Hash {
			t.Fatalf("hash mismatch for %s", src.Path)
		}
		if src.Path != snap.Sources[i].Path {
			t.Fatalf("path mismatch: %q vs %q", src.Path, snap.Sources[i].Path)
		}
	}

	var buf bytes.Buffer
	if err := WriteManifestYAML(&buf, m); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "INS-") {
		t.Fatal("expected INS- prefixed IDs")
	}
}
