package app

// #612: `corpus attest --external-snapshot` binds the snapshot's identity,
// root, counts and web-source coverage into the attestation — measured from
// the verified envelope and records, never declared by the caller. Adversarial
// discipline: a tampered envelope must refuse the attestation outright (no
// file written, no verdict invented).

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const recursioExport = "../../../tests/fixtures/recursio-e2e/export"

func scanTinyCorpus(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeTestFile(t, root, "captures/index.md", "# Index\n\nBody.\n")
	initGitRepo(t, root)
	snapshotPath := filepath.Join(t.TempDir(), "scan.json")
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"corpus", "scan", "--root", root, "--out", snapshotPath, "--ext", ".md"}, &stdout, &stderr); code != 0 {
		t.Fatalf("scan failed: %d stderr=%q", code, stderr.String())
	}
	return snapshotPath
}

func attestWithSnapshot(t *testing.T, envelope, records, out string) (int, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	args := []string{"corpus", "attest", "--snapshot", scanTinyCorpus(t), "--corpus-id", "c", "--project-id", "p",
		"--external-snapshot", envelope, "--out", out}
	if records != "" {
		args = append(args, "--external-snapshot-records", records)
	}
	return Run(args, &stdout, &stderr), stderr.String()
}

func TestCorpusAttestExternalSnapshotBindsWebCoverage(t *testing.T) {
	out := filepath.Join(t.TempDir(), "attestation.json")
	code, stderr := attestWithSnapshot(t, filepath.Join(recursioExport, "snapshot.json"), "", out)
	if code != 0 {
		t.Fatalf("attest with a valid snapshot exited %d: %s", code, stderr)
	}
	raw, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	var att struct {
		Predicate struct {
			Metadata map[string]any `json:"metadata"`
		} `json:"predicate"`
	}
	if err := json.Unmarshal(raw, &att); err != nil {
		t.Fatal(err)
	}
	meta, ok := att.Predicate.Metadata["external_snapshot"].(map[string]any)
	if !ok {
		t.Fatalf("attestation metadata has no external_snapshot object: %v", att.Predicate.Metadata)
	}
	envRaw, err := os.ReadFile(filepath.Join(recursioExport, "snapshot.json"))
	if err != nil {
		t.Fatal(err)
	}
	var env map[string]any
	if err := json.Unmarshal(envRaw, &env); err != nil {
		t.Fatal(err)
	}
	// Identity and root come from the envelope; counts are measured from the records.
	for _, key := range []string{"snapshot_id", "content_hash_root", "producer", "generated_at"} {
		if meta[key] != env[key] {
			t.Errorf("metadata.%s = %v, envelope has %v", key, meta[key], env[key])
		}
	}
	if got := meta["web_sources"]; got != float64(3) {
		t.Errorf("web_sources = %v, want 3 (every fixture record carries #610 provenance)", got)
	}
	if got := meta["records"]; got != float64(3) {
		t.Errorf("records = %v, want 3", got)
	}
	types, _ := meta["source_types"].(map[string]any)
	if types["html"] != float64(3) {
		t.Errorf("source_types = %v, want html:3 — the attestation must name the WEB source type", types)
	}
	if cb, _ := meta["claim_boundary"].(string); !strings.Contains(cb, "never reads the operational store") {
		t.Errorf("claim boundary must travel with the coverage metadata, got %q", cb)
	}
}

func TestCorpusAttestExternalSnapshotRefusesTamperedRoot(t *testing.T) {
	dir := t.TempDir()
	raw, err := os.ReadFile(filepath.Join(recursioExport, "snapshot.json"))
	if err != nil {
		t.Fatal(err)
	}
	var env map[string]any
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatal(err)
	}
	env["content_hash_root"] = strings.Repeat("0", 64)
	forged, _ := json.Marshal(env)
	envelope := filepath.Join(dir, "snapshot.json")
	if err := os.WriteFile(envelope, forged, 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "attestation.json")
	code, stderr := attestWithSnapshot(t, envelope, filepath.Join(recursioExport, "sources.jsonl"), out)
	if code == 0 {
		t.Fatal("attest accepted a snapshot whose root does not match its records")
	}
	if !strings.Contains(stderr, "REFUSED") || !strings.Contains(stderr, "no attestation written") {
		t.Errorf("refusal must be explicit and name the consequence, got %q", stderr)
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Fatal("an attestation file was written despite the refusal")
	}
}

func TestCorpusAttestExternalSnapshotMissingRecordsIsAnError(t *testing.T) {
	dir := t.TempDir()
	raw, _ := os.ReadFile(filepath.Join(recursioExport, "snapshot.json"))
	envelope := filepath.Join(dir, "snapshot.json") // no sources.jsonl beside it
	if err := os.WriteFile(envelope, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "attestation.json")
	code, stderr := attestWithSnapshot(t, envelope, "", out)
	if code == 0 {
		t.Fatal("attest produced an attestation for a snapshot whose records are missing")
	}
	if !strings.Contains(stderr, "external snapshot") {
		t.Errorf("error must name the external snapshot, got %q", stderr)
	}
}
