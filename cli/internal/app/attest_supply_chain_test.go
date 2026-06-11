package app

// VRC-08 (#554): CLI-level proofs that the CKM-05 supply-chain predicate has a
// real production caller. Adversarial discipline (doctrine §2.3): digests are
// computed from real artifact bytes; altering one byte of a pipeline artifact
// must turn verification red.

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/RBOKproject/Nomos/cli/internal/attestation"
)

func writeSupplyChainArtifacts(t *testing.T) (dir, snapshot, manifest, feed string) {
	t.Helper()
	dir = t.TempDir()
	snapshot = filepath.Join(dir, "snapshot.json")
	manifest = filepath.Join(dir, "source-manifest.yaml")
	feed = filepath.Join(dir, "feed.json")
	for path, content := range map[string]string{
		snapshot: `{"total_files":1}`,
		manifest: "sources:\n  - id: SRC-1\n",
		feed:     `{"units":[{"id":"u1"}]}`,
	} {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir, snapshot, manifest, feed
}

func emitSupplyChainStatement(t *testing.T, snapshot, manifest, feed, out string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"attest", "supply-chain",
		"--project-id", "rbok",
		"--corpus-id", "rbok-test",
		"--snapshot", snapshot,
		"--manifest", manifest,
		"--feed", feed,
		"--out", out,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("supply-chain emit failed: %d stderr=%q", code, stderr.String())
	}
}

func TestAttestSupplyChainEmitsStatementFromRealArtifacts(t *testing.T) {
	dir, snapshot, manifest, feed := writeSupplyChainArtifacts(t)
	out := filepath.Join(dir, "statement.json")
	emitSupplyChainStatement(t, snapshot, manifest, feed, out)

	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	var stmt attestation.InTotoStatement
	if err := json.Unmarshal(data, &stmt); err != nil {
		t.Fatalf("decode statement: %v", err)
	}
	if stmt.PredicateType != attestation.SupplyChainPredicateType {
		t.Fatalf("predicateType = %q", stmt.PredicateType)
	}
	var pred attestation.SupplyChainPredicate
	if err := json.Unmarshal(stmt.Predicate, &pred); err != nil {
		t.Fatal(err)
	}
	if !pred.HasStep(attestation.StepIngestion) || !pred.HasStep(attestation.StepCanon) {
		t.Fatalf("expected ingestion+canon steps, got %+v", pred.Steps)
	}
	if pred.Signature.Status == attestation.SignatureStatusSigned {
		t.Fatalf("predicate must stay unsigned until keyless Sigstore lands (VRC-40), got %+v", pred.Signature)
	}
	// The feed digest must be the REAL sha256 of the feed bytes.
	feedBytes, _ := os.ReadFile(feed)
	want := attestation.DigestSHA256(feedBytes)
	found := false
	for _, subj := range stmt.Subject {
		if subj.Name == filepath.Base(feed) && subj.Digest["sha256"] == want {
			found = true
		}
	}
	if !found {
		t.Fatalf("feed subject with computed sha256 missing from statement: %s", string(data))
	}
}

func TestAttestSupplyChainVerifyPassesThenFailsOnTamperedArtifact(t *testing.T) {
	dir, snapshot, manifest, feed := writeSupplyChainArtifacts(t)
	out := filepath.Join(dir, "statement.json")
	emitSupplyChainStatement(t, snapshot, manifest, feed, out)

	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"attest", "supply-chain",
		"--verify", out,
		"--artifact", filepath.Base(feed) + "=" + feed,
		"--artifact", filepath.Base(manifest) + "=" + manifest,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("verify on untampered artifacts failed: %d stderr=%q", code, stderr.String())
	}

	// Adversarial proof: flip one byte of the feed AFTER attestation.
	if err := os.WriteFile(feed, []byte(`{"units":[{"id":"u1"}]} `), 0o644); err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	stderr.Reset()
	code = Run([]string{
		"attest", "supply-chain",
		"--verify", out,
		"--artifact", filepath.Base(feed) + "=" + feed,
	}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("tampered feed artifact VERIFIED — the supply-chain caller is not real")
	}
	if !strings.Contains(stderr.String(), "sha256 mismatch") {
		t.Fatalf("expected sha256 mismatch, got stderr=%q", stderr.String())
	}
}

func TestAttestSupplyChainSignedEnvelopeRoundtrip(t *testing.T) {
	// Envelope-level ECDSA DSSE signing must produce an envelope that the
	// existing `attest verify` accepts with the emitted public key.
	dir, snapshot, manifest, feed := writeSupplyChainArtifacts(t)
	out := filepath.Join(dir, "envelope.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"attest", "supply-chain",
		"--project-id", "rbok",
		"--corpus-id", "rbok-test",
		"--snapshot", snapshot,
		"--manifest", manifest,
		"--feed", feed,
		"--sign",
		"--out", out,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("signed emit failed: %d stderr=%q", code, stderr.String())
	}
	if _, err := os.Stat(out + ".pub.pem"); err != nil {
		t.Fatalf("public key missing, signature unverifiable: %v", err)
	}
	stdout.Reset()
	stderr.Reset()
	code = Run([]string{"attest", "verify", "--envelope", out, "--pub", out + ".pub.pem"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("attest verify rejected the signed supply-chain envelope: %d stderr=%q", code, stderr.String())
	}
}

func TestAttestSupplyChainRequiresPipelineArtifacts(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"attest", "supply-chain", "--project-id", "p"}, &stdout, &stderr); code != 2 {
		t.Fatalf("expected exit 2 without required artifacts, got %d", code)
	}
}
