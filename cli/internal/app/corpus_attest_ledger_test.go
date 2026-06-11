package app

// VRC-07 (#553): CLI-level proofs that claim_coverage is wired into
// `corpus attest` and that body-ledger Merkle verification is a production
// surface. Adversarial discipline (doctrine §2.3): a tampered ledger must turn
// both `corpus body-ledger --verify` and `corpus attest --corpus-body-ledger`
// red; coverage values are asserted as CALCULATED outputs, never declared.

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/RBOKproject/Nomos/cli/internal/corpus"
)

// buildLedgerFixture runs scan -> manifest -> body-ledger on a real temp
// corpus and returns (snapshotPath, ledgerPath).
func buildLedgerFixture(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	writeTestFile(t, root, "01_rbok/rule.md", "# Rule\n\nBody paragraph.\n")
	initGitRepo(t, root)

	outDir := t.TempDir()
	snapshotPath := filepath.Join(outDir, "snapshot.json")
	manifestPath := filepath.Join(outDir, "source-manifest.yaml")
	ledgerPath := filepath.Join(outDir, "body-ledger.json")

	var stdout, stderr bytes.Buffer
	if code := Run([]string{"corpus", "scan", "--root", root, "--out", snapshotPath, "--ext", ".md"}, &stdout, &stderr); code != 0 {
		t.Fatalf("scan failed: %d stderr=%q", code, stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	if code := Run([]string{
		"corpus", "manifest",
		"--snapshot", snapshotPath,
		"--out", manifestPath,
		"--domain", "rbok",
		"--owner", "domain-owner@example.com",
		"--id-prefix", "RBOK",
	}, &stdout, &stderr); code != 0 {
		t.Fatalf("manifest failed: %d stderr=%q", code, stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	if code := Run([]string{
		"corpus", "body-ledger",
		"--root", root,
		"--manifest", manifestPath,
		"--out", ledgerPath,
	}, &stdout, &stderr); code != 0 {
		t.Fatalf("body-ledger failed: %d stderr=%q", code, stderr.String())
	}
	return snapshotPath, ledgerPath
}

// tamperLedgerFile flips one recorded source hash inside the ledger JSON so
// the stored Merkle proof no longer matches the recomputed leaf.
func tamperLedgerFile(t *testing.T, ledgerPath string) {
	t.Helper()
	var ledger corpus.CorpusBodyLedger
	data, err := os.ReadFile(ledgerPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &ledger); err != nil {
		t.Fatal(err)
	}
	orig := ledger.Sources[0].Hash
	flipped := "0" + orig[1:]
	if flipped == orig {
		flipped = "1" + orig[1:]
	}
	ledger.Sources[0].Hash = flipped
	out, err := json.Marshal(ledger)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ledgerPath, out, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCorpusBodyLedgerVerifyCLI(t *testing.T) {
	_, ledgerPath := buildLedgerFixture(t)

	var stdout, stderr bytes.Buffer
	if code := Run([]string{"corpus", "body-ledger", "--verify", ledgerPath}, &stdout, &stderr); code != 0 {
		t.Fatalf("verify on untampered ledger failed: %d stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "verification: ok") {
		t.Fatalf("expected verification ok output, got %q", stdout.String())
	}

	tamperLedgerFile(t, ledgerPath)
	stdout.Reset()
	stderr.Reset()
	code := Run([]string{"corpus", "body-ledger", "--verify", ledgerPath}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("tampered ledger VERIFIED via CLI — verification is not real")
	}
	if !strings.Contains(stderr.String(), "verification FAILED") {
		t.Fatalf("expected verification FAILED on stderr, got %q", stderr.String())
	}
}

func TestCorpusAttestEmitsCalculatedClaimCoverage(t *testing.T) {
	snapshotPath, ledgerPath := buildLedgerFixture(t)

	feedPath := filepath.Join(t.TempDir(), "feed.json")
	if err := os.WriteFile(feedPath, []byte(`{"units":[{"id":"u1"},{"id":"u2"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"corpus", "attest",
		"--snapshot", snapshotPath,
		"--corpus-id", "rbok-test",
		"--project-id", "rbok",
		"--corpus-body-ledger", ledgerPath,
		"--feed", feedPath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("attest failed: %d stderr=%q", code, stderr.String())
	}

	var statement struct {
		Predicate struct {
			UnitsExtracted int `json:"unitsExtracted"`
			ClaimCoverage  *struct {
				CoversFullSourceBody bool   `json:"covers_full_source_body"`
				CoversCuratedFeed    bool   `json:"covers_curated_feed"`
				SummaryStatus        string `json:"summary_status"`
			} `json:"claim_coverage"`
		} `json:"predicate"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &statement); err != nil {
		t.Fatalf("decode attestation: %v\noutput=%s", err, stdout.String())
	}
	cc := statement.Predicate.ClaimCoverage
	if cc == nil {
		t.Fatalf("claim_coverage missing from attestation predicate: %s", stdout.String())
	}
	if !cc.CoversFullSourceBody {
		t.Fatalf("expected covers_full_source_body=true for fully covered fixture, got %+v", cc)
	}
	if !cc.CoversCuratedFeed || statement.Predicate.UnitsExtracted != 2 {
		t.Fatalf("expected covers_curated_feed=true with 2 units counted from the feed, got %+v units=%d",
			cc, statement.Predicate.UnitsExtracted)
	}
	if cc.SummaryStatus != "feed_and_body" {
		t.Fatalf("expected summary_status=feed_and_body, got %q", cc.SummaryStatus)
	}
}

func TestCorpusAttestOmitsClaimCoverageWithoutLedger(t *testing.T) {
	// Backward compatibility: no --corpus-body-ledger means no claim_coverage
	// field at all — consumers that never saw the field see no change.
	snapshotPath, _ := buildLedgerFixture(t)

	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"corpus", "attest",
		"--snapshot", snapshotPath,
		"--corpus-id", "rbok-test",
		"--project-id", "rbok",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("attest failed: %d stderr=%q", code, stderr.String())
	}
	if strings.Contains(stdout.String(), "claim_coverage") {
		t.Fatalf("claim_coverage must be omitted when no ledger is supplied: %s", stdout.String())
	}
}

func TestCorpusAttestFailsOnTamperedLedger(t *testing.T) {
	// The adversarial proof: a ledger altered after proof generation must make
	// the attestation FAIL, not silently decorate it with false coverage.
	snapshotPath, ledgerPath := buildLedgerFixture(t)
	tamperLedgerFile(t, ledgerPath)

	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"corpus", "attest",
		"--snapshot", snapshotPath,
		"--corpus-id", "rbok-test",
		"--project-id", "rbok",
		"--corpus-body-ledger", ledgerPath,
	}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("attest ACCEPTED a tampered body ledger — claim_coverage wiring is not real")
	}
	if !strings.Contains(stderr.String(), "body ledger verification FAILED") {
		t.Fatalf("expected body ledger verification failure, got stderr=%q", stderr.String())
	}
}
